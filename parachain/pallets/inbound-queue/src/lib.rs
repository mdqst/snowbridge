// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
#![cfg_attr(not(feature = "std"), no_std)]

mod envelope;

#[cfg(feature = "runtime-benchmarks")]
mod benchmarking;

#[cfg(feature = "runtime-benchmarks")]
use snowbridge_beacon_primitives::CompactExecutionHeader;
#[cfg(feature = "runtime-benchmarks")]
use snowbridge_ethereum::H256;

pub mod weights;

#[cfg(test)]
mod test;

use codec::{Decode, DecodeAll, Encode};
use frame_support::{
	traits::fungible::{Inspect, Mutate},
	CloneNoBound, EqNoBound, PartialEqNoBound,
};
use frame_system::ensure_signed;
use scale_info::TypeInfo;
use sp_core::H160;
use sp_runtime::traits::AccountIdConversion;
use sp_std::convert::TryFrom;
use xcm::v3::{send_xcm, Junction::*, Junctions::*, MultiLocation, SendError};

use envelope::Envelope;
use snowbridge_core::{
	inbound::{Message, Verifier},
	ParaId,
};
use snowbridge_router_primitives::inbound;
pub use weights::WeightInfo;

type BalanceOf<T> =
	<<T as pallet::Config>::Token as Inspect<<T as frame_system::Config>::AccountId>>::Balance;

#[derive(CloneNoBound, EqNoBound, PartialEqNoBound, Encode, Decode, Debug, TypeInfo)]
pub enum MessageDispatchResult {
	InvalidPayload,
	Dispatched,
	NotDispatched(SendError),
}

pub use pallet::*;

pub const LOG_TARGET: &str = "snowbridge-inbound-queue";

#[frame_support::pallet]
pub mod pallet {

	use super::*;

	use frame_support::{pallet_prelude::*, traits::tokens::Preservation};
	use frame_system::pallet_prelude::*;
	use xcm::v3::SendXcm;

	use bp_runtime::{BasicOperatingMode, OwnedBridgeModule};

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[cfg(feature = "runtime-benchmarks")]
	pub trait BenchmarkHelper<T> {
		fn initialize_storage(block_hash: H256, header: CompactExecutionHeader);
	}

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeEvent: From<Event<Self>> + IsType<<Self as frame_system::Config>::RuntimeEvent>;

		/// The verifier for inbound messages from Ethereum
		type Verifier: Verifier;

		/// Message relayers are rewarded with this asset
		type Token: Mutate<Self::AccountId>;

		/// The amount to reward message relayers
		type Reward: Get<BalanceOf<Self>>;

		/// XCM message sender
		type XcmSender: SendXcm;

		type WeightInfo: WeightInfo;

		// Gateway contract address
		#[pallet::constant]
		type GatewayAddress: Get<H160>;

		#[cfg(feature = "runtime-benchmarks")]
		type Helper: BenchmarkHelper<Self>;
	}

	#[pallet::hooks]
	impl<T: Config> Hooks<BlockNumberFor<T>> for Pallet<T> {}

	#[pallet::event]
	#[pallet::generate_deposit(pub(super) fn deposit_event)]
	pub enum Event<T> {
		/// A message was received from Ethereum
		MessageReceived {
			/// The destination parachain
			dest: ParaId,
			/// The message nonce
			nonce: u64,
			/// The result of the message dispatch
			result: MessageDispatchResult,
		},
	}

	#[pallet::error]
	pub enum Error<T> {
		/// Message came from an invalid outbound channel on the Ethereum side.
		InvalidGateway,
		/// Message has an invalid envelope.
		InvalidEnvelope,
		/// Message has an unexpected nonce.
		InvalidNonce,
		/// The max nonce for the type has been reached
		MaxNonceReached,
		/// Error generated by the `OwnedBridgeModule` trait.
		BridgeModule(bp_runtime::OwnedBridgeModuleError),
	}

	/// The current nonce for each parachain
	#[pallet::storage]
	pub type Nonce<T: Config> = StorageMap<_, Twox64Concat, ParaId, u64, ValueQuery>;

	/// Optional pallet owner.
	///
	/// Pallet owner has a right to halt all pallet operations and then resume them. If it is
	/// `None`, then there are no direct ways to halt/resume pallet operations, but other
	/// runtime methods may still be used to do that (i.e. democracy::referendum to update halt
	/// flag directly or call the `halt_operations`).
	#[pallet::storage]
	pub type PalletOwner<T: Config> = StorageValue<_, T::AccountId, OptionQuery>;

	/// The current operating mode of the pallet.
	///
	/// Depending on the mode either all, or no transactions will be allowed.
	#[pallet::storage]
	pub type PalletOperatingMode<T: Config> = StorageValue<_, BasicOperatingMode, ValueQuery>;

	impl<T: Config> OwnedBridgeModule<T> for Pallet<T> {
		const LOG_TARGET: &'static str = LOG_TARGET;
		type OwnerStorage = PalletOwner<T>;
		type OperatingMode = BasicOperatingMode;
		type OperatingModeStorage = PalletOperatingMode<T>;
	}

	#[pallet::call]
	impl<T: Config> Pallet<T> {
		/// Submit an inbound message originating from the Gateway contract on Ethereum
		#[pallet::call_index(0)]
		#[pallet::weight(T::WeightInfo::submit())]
		pub fn submit(origin: OriginFor<T>, message: Message) -> DispatchResult {
			Self::ensure_not_halted().map_err(Error::<T>::BridgeModule)?;
			let who = ensure_signed(origin)?;
			// submit message to verifier for verification
			let log = T::Verifier::verify(&message)?;

			// Decode log into an Envelope
			let envelope = Envelope::try_from(log).map_err(|_| Error::<T>::InvalidEnvelope)?;

			// Verify that the message was submitted from the known Gateway contract
			ensure!(T::GatewayAddress::get() == envelope.gateway, Error::<T>::InvalidGateway,);

			// Verify message nonce
			<Nonce<T>>::try_mutate(envelope.dest, |nonce| -> DispatchResult {
				if *nonce == u64::MAX {
					return Err(Error::<T>::MaxNonceReached.into())
				}
				if envelope.nonce != nonce.saturating_add(1) {
					Err(Error::<T>::InvalidNonce.into())
				} else {
					*nonce = nonce.saturating_add(1);
					Ok(())
				}
			})?;

			// Reward relayer from the sovereign account of the destination parachain
			// Expected to fail if sovereign account has no funds
			let sovereign_account = envelope.dest.into_account_truncating();
			T::Token::transfer(&sovereign_account, &who, T::Reward::get(), Preservation::Preserve)?;

			// From this point, any errors are masked, i.e the extrinsic will
			// succeed even if the message was not successfully decoded or dispatched.

			// Attempt to decode message
			let xcm = match inbound::VersionedMessage::decode_all(&mut envelope.payload.as_ref()) {
				Ok(inbound::VersionedMessage::V1(message_v1)) => message_v1.into(),
				Err(_) => {
					Self::deposit_event(Event::MessageReceived {
						dest: envelope.dest,
						nonce: envelope.nonce,
						result: MessageDispatchResult::InvalidPayload,
					});
					return Ok(())
				},
			};

			// Attempt to convert to XCM
			let sibling_para =
				MultiLocation { parents: 1, interior: X1(Parachain(envelope.dest.into())) };

			// Attempt to send XCM to a sibling parachain
			match send_xcm::<T::XcmSender>(sibling_para, xcm) {
				Ok(_) => Self::deposit_event(Event::MessageReceived {
					dest: envelope.dest,
					nonce: envelope.nonce,
					result: MessageDispatchResult::Dispatched,
				}),
				Err(err) => Self::deposit_event(Event::MessageReceived {
					dest: envelope.dest,
					nonce: envelope.nonce,
					result: MessageDispatchResult::NotDispatched(err),
				}),
			}

			Ok(())
		}

		/// Change `PalletOwner`.
		/// May only be called either by root, or by `PalletOwner`.
		#[pallet::call_index(3)]
		#[pallet::weight((T::DbWeight::get().reads_writes(1, 1), DispatchClass::Operational))]
		pub fn set_owner(origin: OriginFor<T>, new_owner: Option<T::AccountId>) -> DispatchResult {
			<Self as OwnedBridgeModule<_>>::set_owner(origin, new_owner)
		}

		/// Halt or resume all pallet operations.
		/// May only be called either by root, or by `PalletOwner`.
		#[pallet::call_index(4)]
		#[pallet::weight((T::DbWeight::get().reads_writes(1, 1), DispatchClass::Operational))]
		pub fn set_operating_mode(
			origin: OriginFor<T>,
			operating_mode: BasicOperatingMode,
		) -> DispatchResult {
			<Self as OwnedBridgeModule<_>>::set_operating_mode(origin, operating_mode)
		}
	}
}
