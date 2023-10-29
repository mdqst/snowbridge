// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
use crate as snowbridge_control;
use frame_support::{
	parameter_types,
	traits::{tokens::fungible::Mutate, ConstU128, ConstU16, ConstU64, Contains, GenesisBuild},
	weights::IdentityFee,
	PalletId,
};
use sp_core::H256;
use xcm_executor::traits::ConvertLocation;

use snowbridge_core::{
	outbound::{ConstantGasMeter, ParaId},
	sibling_sovereign_account, AgentId,
};
use sp_runtime::{
	testing::Header,
	traits::{AccountIdConversion, BlakeTwo256, IdentityLookup, Keccak256},
	AccountId32,
};
use xcm::prelude::*;
use xcm_builder::{DescribeAllTerminal, DescribeFamily, HashedDescription};

#[cfg(feature = "runtime-benchmarks")]
use crate::BenchmarkHelper;

type UncheckedExtrinsic = frame_system::mocking::MockUncheckedExtrinsic<Test>;
type Block = frame_system::mocking::MockBlock<Test>;
type Balance = u128;

pub type AccountId = AccountId32;

// A stripped-down version of pallet-xcm that only inserts an XCM origin into the runtime
#[allow(dead_code)]
#[frame_support::pallet]
mod pallet_xcm_origin {
	use frame_support::{
		pallet_prelude::*,
		traits::{Contains, OriginTrait},
	};
	use xcm::latest::prelude::*;

	#[pallet::pallet]
	pub struct Pallet<T>(_);

	#[pallet::config]
	pub trait Config: frame_system::Config {
		type RuntimeOrigin: From<Origin> + From<<Self as frame_system::Config>::RuntimeOrigin>;
	}

	// Insert this custom Origin into the aggregate RuntimeOrigin
	#[pallet::origin]
	#[derive(PartialEq, Eq, Clone, Encode, Decode, RuntimeDebug, TypeInfo, MaxEncodedLen)]
	pub struct Origin(pub MultiLocation);

	impl From<MultiLocation> for Origin {
		fn from(location: MultiLocation) -> Origin {
			Origin(location)
		}
	}

	/// `EnsureOrigin` implementation succeeding with a `MultiLocation` value to recognize and
	/// filter the contained location
	pub struct EnsureXcm<F>(PhantomData<F>);
	impl<O: OriginTrait + From<Origin>, F: Contains<MultiLocation>> EnsureOrigin<O> for EnsureXcm<F>
	where
		O::PalletsOrigin: From<Origin> + TryInto<Origin, Error = O::PalletsOrigin>,
	{
		type Success = MultiLocation;

		fn try_origin(outer: O) -> Result<Self::Success, O> {
			outer.try_with_caller(|caller| {
				caller.try_into().and_then(|o| match o {
					Origin(location) if F::contains(&location) => Ok(location),
					o => Err(o.into()),
				})
			})
		}

		#[cfg(feature = "runtime-benchmarks")]
		fn try_successful_origin() -> Result<O, ()> {
			Ok(O::from(Origin(MultiLocation { parents: 1, interior: X1(Parachain(2000)) })))
		}
	}
}

// Configure a mock runtime to test the pallet.
frame_support::construct_runtime!(
	pub enum Test where
		Block = Block,
		NodeBlock = Block,
		UncheckedExtrinsic = UncheckedExtrinsic,
	{
		System: frame_system,
		Balances: pallet_balances::{Pallet, Call, Storage, Config<T>, Event<T>},
		XcmOrigin: pallet_xcm_origin::{Pallet, Origin},
		OutboundQueue: snowbridge_outbound_queue::{Pallet, Call, Storage, Config, Event<T>},
		EthereumControl: snowbridge_control,
		MessageQueue: pallet_message_queue::{Pallet, Call, Storage, Event<T>}
	}
);

impl frame_system::Config for Test {
	type BaseCallFilter = frame_support::traits::Everything;
	type BlockWeights = ();
	type BlockLength = ();
	type DbWeight = ();
	type RuntimeOrigin = RuntimeOrigin;
	type RuntimeCall = RuntimeCall;
	type Index = u64;
	type BlockNumber = u64;
	type Hash = H256;
	type Hashing = BlakeTwo256;
	type AccountId = AccountId;
	type Lookup = IdentityLookup<Self::AccountId>;
	type Header = Header;
	type RuntimeEvent = RuntimeEvent;
	type BlockHashCount = ConstU64<250>;
	type Version = ();
	type PalletInfo = PalletInfo;
	type AccountData = pallet_balances::AccountData<Balance>;
	type OnNewAccount = ();
	type OnKilledAccount = ();
	type SystemWeightInfo = ();
	type SS58Prefix = ConstU16<42>;
	type OnSetCode = ();
	type MaxConsumers = frame_support::traits::ConstU32<16>;
}

impl pallet_balances::Config for Test {
	type MaxLocks = ();
	type MaxReserves = ();
	type ReserveIdentifier = [u8; 8];
	type Balance = Balance;
	type RuntimeEvent = RuntimeEvent;
	type DustRemoval = ();
	type ExistentialDeposit = ConstU128<1>;
	type AccountStore = System;
	type WeightInfo = ();
	type FreezeIdentifier = ();
	type MaxFreezes = ();
	type RuntimeHoldReason = ();
	type MaxHolds = ();
}

impl pallet_xcm_origin::Config for Test {
	type RuntimeOrigin = RuntimeOrigin;
}

parameter_types! {
	pub const HeapSize: u32 = 32 * 1024;
	pub const MaxStale: u32 = 32;
	pub static ServiceWeight: Option<Weight> = Some(Weight::from_parts(100, 100));
}

impl pallet_message_queue::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type WeightInfo = ();
	type MessageProcessor = OutboundQueue;
	type Size = u32;
	type QueueChangeHandler = ();
	type HeapSize = HeapSize;
	type MaxStale = MaxStale;
	type ServiceWeight = ServiceWeight;
}

parameter_types! {
	pub const MaxMessagePayloadSize: u32 = 1024;
	pub const MaxMessagesPerBlock: u32 = 20;
	pub const OwnParaId: ParaId = ParaId::new(1013);
}

impl snowbridge_outbound_queue::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type Hashing = Keccak256;
	type MessageQueue = MessageQueue;
	type MaxMessagePayloadSize = MaxMessagePayloadSize;
	type MaxMessagesPerBlock = MaxMessagesPerBlock;
	type OwnParaId = OwnParaId;
	type GasMeter = ConstantGasMeter;
	type Balance = u128;
	type WeightToFee = IdentityFee<u128>;
	type WeightInfo = ();
}

parameter_types! {
	pub const SS58Prefix: u8 = 42;
	pub const AnyNetwork: Option<NetworkId> = None;
	pub const RelayNetwork: Option<NetworkId> = Some(NetworkId::Kusama);
	pub const RelayLocation: MultiLocation = MultiLocation::parent();
	pub UniversalLocation: InteriorMultiLocation =
		X2(GlobalConsensus(RelayNetwork::get().unwrap()), Parachain(1013));
}

parameter_types! {
	pub TreasuryAccount: AccountId = PalletId(*b"py/trsry").into_account_truncating();
	pub Fee: u64 = 1000;
	pub const RococoNetwork: NetworkId = NetworkId::Rococo;
	pub const InitialFunding: u128 = 1_000_000_000_000;
	pub TestParaId: u32 = 2000;
}

#[cfg(feature = "runtime-benchmarks")]
impl BenchmarkHelper<RuntimeOrigin> for () {
	fn make_xcm_origin(location: MultiLocation) -> RuntimeOrigin {
		RuntimeOrigin::from(pallet_xcm_origin::Origin(location))
	}
}

pub struct AllowSiblingsOnly;
impl Contains<MultiLocation> for AllowSiblingsOnly {
	fn contains(location: &MultiLocation) -> bool {
		if let MultiLocation { parents: 1, interior: X1(Parachain(_)) } = location {
			true
		} else {
			false
		}
	}
}

impl crate::Config for Test {
	type RuntimeEvent = RuntimeEvent;
	type OwnParaId = OwnParaId;
	type OutboundQueue = OutboundQueue;
	type MessageHasher = BlakeTwo256;
	type SiblingOrigin = pallet_xcm_origin::EnsureXcm<AllowSiblingsOnly>;
	type AgentIdOf = HashedDescription<AgentId, DescribeFamily<DescribeAllTerminal>>;
	type TreasuryAccount = TreasuryAccount;
	type Token = Balances;
	type WeightInfo = ();
	#[cfg(feature = "runtime-benchmarks")]
	type Helper = ();
}

// Build genesis storage according to the mock runtime.
pub fn new_test_ext() -> sp_io::TestExternalities {
	let mut storage = frame_system::GenesisConfig::default().build_storage::<Test>().unwrap();

	let config = snowbridge_outbound_queue::GenesisConfig {
		operating_mode: Default::default(),
		fee_config: Default::default(),
	};

	GenesisBuild::<Test>::assimilate_storage(&config, &mut storage).unwrap();

	let mut ext: sp_io::TestExternalities = storage.into();
	let initial_amount = InitialFunding::get().into();
	let test_para_id = TestParaId::get();
	let sovereign_account = sibling_sovereign_account::<Test>(test_para_id.into());
	let treasury_account = TreasuryAccount::get();
	ext.execute_with(|| {
		System::set_block_number(1);
		Balances::mint_into(&AccountId32::from([0; 32]), initial_amount).unwrap();
		Balances::mint_into(&sovereign_account, initial_amount).unwrap();
		Balances::mint_into(&treasury_account, initial_amount).unwrap();
	});
	ext
}

// Test helpers

pub fn make_xcm_origin(location: MultiLocation) -> RuntimeOrigin {
	pallet_xcm_origin::Origin(location).into()
}

pub fn make_agent_id(location: MultiLocation) -> AgentId {
	HashedDescription::<AgentId, DescribeFamily<DescribeAllTerminal>>::convert_location(&location)
		.expect("convert location")
}
