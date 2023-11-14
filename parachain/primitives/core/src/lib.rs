// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
//! # Core
//!
//! Common traits and types
#![cfg_attr(not(feature = "std"), no_std)]

#[cfg(test)]
mod tests;

pub mod inbound;
pub mod operating_mode;
pub mod outbound;
pub mod ringbuffer;

pub use polkadot_parachain_primitives::primitives::{
	Id as ParaId, IsSystem, Sibling as SiblingParaId,
};
pub use ringbuffer::{RingBufferMap, RingBufferMapImpl};

use codec::{Decode, Encode, MaxEncodedLen};
use frame_support::traits::Contains;
use hex_literal::hex;
use scale_info::TypeInfo;
use sp_core::H256;
use sp_io::hashing::keccak_256;
use sp_runtime::{traits::AccountIdConversion, RuntimeDebug};
use sp_std::prelude::*;
use xcm::prelude::{Junction::Parachain, Junctions::X1, MultiLocation};

/// The ID of an agent contract
pub type AgentId = H256;
pub use operating_mode::BasicOperatingMode;

pub fn sibling_sovereign_account<T>(para_id: ParaId) -> T::AccountId
where
	T: frame_system::Config,
{
	SiblingParaId::from(para_id).into_account_truncating()
}

pub fn sibling_sovereign_account_raw(para_id: ParaId) -> [u8; 32] {
	SiblingParaId::from(para_id).into_account_truncating()
}

pub struct AllowSiblingsOnly;
impl Contains<MultiLocation> for AllowSiblingsOnly {
	fn contains(location: &MultiLocation) -> bool {
		matches!(location, MultiLocation { parents: 1, interior: X1(Parachain(_)) })
	}
}

pub const GWEI: u128 = 1_000_000_000;
pub const METH: u128 = 1_000_000_000_000_000;
pub const ETH: u128 = 1_000_000_000_000_000_000;

/// Identifier for a message channel
#[derive(
	Clone, Copy, Encode, Decode, PartialEq, Eq, Default, RuntimeDebug, MaxEncodedLen, TypeInfo,
)]
pub struct ChannelId([u8; 32]);

/// Deterministically derive a ChannelId for a sibling parachain
/// Generator: keccak256("para" + big_endian_bytes(para_id))
///
/// The equivalent generator on the Solidity side is in
/// contracts/src/Types.sol:into().
fn derive_channel_id_for_sibling(para_id: ParaId) -> ChannelId {
	let para_id: u32 = para_id.into();
	let para_id_bytes: [u8; 4] = para_id.to_be_bytes();
	let prefix: [u8; 4] = *b"para";
	let preimage: Vec<u8> = prefix.into_iter().chain(para_id_bytes.into_iter()).collect();
	keccak_256(&preimage).into()
}

impl ChannelId {
	pub const fn new(id: [u8; 32]) -> Self {
		ChannelId(id)
	}
}

impl From<ParaId> for ChannelId {
	fn from(value: ParaId) -> Self {
		derive_channel_id_for_sibling(value)
	}
}

impl From<[u8; 32]> for ChannelId {
	fn from(value: [u8; 32]) -> Self {
		ChannelId(value)
	}
}

impl<'a> From<&'a [u8; 32]> for ChannelId {
	fn from(value: &'a [u8; 32]) -> Self {
		ChannelId(*value)
	}
}

impl From<H256> for ChannelId {
	fn from(value: H256) -> Self {
		ChannelId(value.into())
	}
}

impl AsRef<[u8]> for ChannelId {
	fn as_ref(&self) -> &[u8] {
		&self.0
	}
}

#[derive(Clone, Encode, Decode, RuntimeDebug, MaxEncodedLen, TypeInfo)]
pub struct Channel {
	/// ID of the agent contract deployed on Ethereum
	pub agent_id: AgentId,
	/// ID of the parachain who will receive or send messages using this channel
	pub para_id: ParaId,
}

pub trait ChannelLookup {
	fn lookup(channel_id: ChannelId) -> Option<Channel>;
}

/// Channel for high-priority governance commands
pub const PRIMARY_GOVERNANCE_CHANNEL: ChannelId =
	ChannelId::new(hex!("0000000000000000000000000000000000000000000000000000000000000001"));

/// Channel for lower-priority governance commands
pub const SECONDARY_GOVERNANCE_CHANNEL: ChannelId =
	ChannelId::new(hex!("0000000000000000000000000000000000000000000000000000000000000002"));

/// Agent ID for BridgeHub
pub const BRIDGE_HUB_AGENT_ID: AgentId =
	H256(hex!("0000000000000000000000000000000000000000000000000000000000000001"));
