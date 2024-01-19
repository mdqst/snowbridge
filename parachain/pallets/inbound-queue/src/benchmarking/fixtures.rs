// Generated, do not edit!
// See ethereum client README.md for instructions to generate
use hex_literal::hex;
use snowbridge_beacon_primitives::CompactExecutionHeader;
use snowbridge_core::inbound::{Log, Message, Proof};
use sp_std::vec;

pub struct InboundQueueTest {
	pub execution_header: CompactExecutionHeader,
	pub message: Message,
}

pub fn make_create_message() -> InboundQueueTest {
	InboundQueueTest {
        execution_header: CompactExecutionHeader{
            parent_hash: hex!("088df21dc48b1ef18b6df9ef35dc3b21eda78f943813436d5059fb3b8248c74a").into(),
            block_number: 210,
            state_root: hex!("c1e042d99f2e5d21f4be14cca504ce8bd961db18084a1908431686ef918900bd").into(),
            receipts_root: hex!("7b1f61b9714c080ef0be014e01657a15f45f0304b477beebc7ca5596c8033095").into(),
        },
        message: Message {
            event_log: 	Log {
                address: hex!("eda338e4dc46038493b885327842fd3e301cab39").into(),
                topics: vec![
                    hex!("7153f9357c8ea496bba60bf82e67143e27b64462b49041f8e689e1b05728f84f").into(),
                    hex!("c173fac324158e77fb5840738a1a541f633cbec8884c6a601c567d2b376a0539").into(),
                    hex!("5f7060e971b0dc81e63f0aa41831091847d97c1a4693ac450cc128c7214e65e0").into(),
                ],
                data: hex!("00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000002e00a736aa00000000000087d1f7fdfee7f651fabc8bfcb6e086c278b77a7d00e40b54020000000000000000000000000000000000000000000000000000000000").into(),
            },
            proof: Proof {
                block_hash: hex!("b8b9d2d1cba781dee0b344c8102fa02fc94aefe92dd2d7e154f3eb98a3c6288f").into(),
                tx_index: 0,
                data: (vec![
                    hex!("7b1f61b9714c080ef0be014e01657a15f45f0304b477beebc7ca5596c8033095").to_vec(),
                ], vec![
                    hex!("f9028e822080b9028802f90284018301d205b9010000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000080000000000000000000000000000004000000000080000000000000000000000000000000000010100000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000040004000000000000002000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000200000000000010f90179f85894eda338e4dc46038493b885327842fd3e301cab39e1a0f78bb28d4b1d7da699e5c0bc2be29c2b04b5aab6aacf6298fe5304f9db9c6d7ea000000000000000000000000087d1f7fdfee7f651fabc8bfcb6e086c278b77a7df9011c94eda338e4dc46038493b885327842fd3e301cab39f863a07153f9357c8ea496bba60bf82e67143e27b64462b49041f8e689e1b05728f84fa0c173fac324158e77fb5840738a1a541f633cbec8884c6a601c567d2b376a0539a05f7060e971b0dc81e63f0aa41831091847d97c1a4693ac450cc128c7214e65e0b8a000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000002e00a736aa00000000000087d1f7fdfee7f651fabc8bfcb6e086c278b77a7d00e40b54020000000000000000000000000000000000000000000000000000000000").to_vec(),
                ]),
            },
        },
    }
}
