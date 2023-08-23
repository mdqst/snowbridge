use ethers::prelude::U256;
use ethers::{core::types::Address, utils::parse_units};
use snowbridge_smoketest::constants::{
    ASSET_HUB_AGENT_ID, GATEWAY_PROXY_CONTRACT, TEMPLATE_AGENT_ID,
};
use snowbridge_smoketest::helper::{get_agent_address, initial_clients, wait_for_substrate_event};
use snowbridge_smoketest::parachains::template;
use snowbridge_smoketest::{
    contracts::i_gateway, parachains::template::api::template_pallet::events::SomethingStored,
};
use subxt::tx::TxPayload;
use subxt::{OnlineClient, PolkadotConfig};

#[tokio::test]
async fn do_something_through_sovereign() {
    let test_clients = initial_clients().await.expect("initialize clients");

    let gateway_addr: Address = GATEWAY_PROXY_CONTRACT.into();
    let eth_client = *test_clients.ethereum_signed_client;
    let gateway = i_gateway::IGateway::new(gateway_addr, eth_client.clone());

    let template_agent = get_agent_address(gateway.clone(), TEMPLATE_AGENT_ID)
        .await
        .unwrap();
    println!("{:?}", template_agent.0);
    println!("{:?}", template_agent.1.to_string());

    let assethub_agent = get_agent_address(gateway.clone(), ASSET_HUB_AGENT_ID)
        .await
        .unwrap();
    println!("{:?}", assethub_agent.0);
    println!("{:?}", assethub_agent.1.to_string());

    let template_client: OnlineClient<PolkadotConfig> = *(test_clients.template_client).clone();

    let call = template::api::template_pallet::calls::TransactionApi
        .do_something(1)
        .encode_call_data(&template_client.metadata())
        .expect("create call");

    let fee = parse_units("0.0002", "ether").unwrap();

    let dynamic_fee = parse_units("100000000000000", "wei").unwrap().into();

    let receipt = gateway
        .transact_through_sovereign_with_destination_chain_and_origin_kind(
            U256::from(1001),
            [1],
            call.into(),
            dynamic_fee,
            400_000_000,
            8_000,
        )
        // Or just use default
        // .send_transact(U256::from(1000), create_call.into())
        .value(fee)
        .send()
        .await
        .unwrap()
        .await
        .unwrap()
        .unwrap();

    println!("receipt: {:#?}", hex::encode(receipt.transaction_hash));

    assert_eq!(receipt.status.unwrap().as_u64(), 1u64);

    wait_for_substrate_event::<SomethingStored>(&test_clients.template_client).await;
}
