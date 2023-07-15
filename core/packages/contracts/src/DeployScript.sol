// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
pragma solidity 0.8.20;

import {WETH9} from "canonical-weth/WETH9.sol";
import {Script} from "forge-std/Script.sol";
import {BeefyClient} from "./BeefyClient.sol";
import {ParachainClient} from "./ParachainClient.sol";
import {IParachainClient} from "./ParachainClient.sol";

import {IGateway} from "./IGateway.sol";
import {GatewayProxy} from "./GatewayProxy.sol";
import {Gateway} from "./Gateway.sol";
import {Agent} from "./Agent.sol";
import {AgentExecutor} from "./AgentExecutor.sol";
import {Features} from "./Features.sol";
import {ParaID} from "./Types.sol";

contract DeployScript is Script {
    function setUp() public {}

    function run() public {
        uint256 privateKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.rememberKey(privateKey);
        vm.startBroadcast(deployer);

        // BeefyClient
        uint256 randaoCommitDelay = vm.envUint("RANDAO_COMMIT_DELAY");
        uint256 randaoCommitExpiration = vm.envUint("RANDAO_COMMIT_EXP");
        BeefyClient beefyClient = new BeefyClient(randaoCommitDelay, randaoCommitExpiration);

        // ParachainClient
        uint32 paraId = uint32(vm.envUint("BRIDGE_HUB_PARAID"));
        ParachainClient parachainClient = new ParachainClient(beefyClient, paraId);

        // Agent Executor
        AgentExecutor executor = new AgentExecutor();

        Gateway.InitParams memory initParams = Gateway.InitParams({
            parachainClient: IParachainClient(parachainClient),
            agentExecutor: address(executor),
            fee: vm.envUint("RELAYER_FEE"),
            reward: vm.envUint("RELAYER_REWARD"),
            bridgeHubParaID: ParaID.wrap(uint32(vm.envUint("BRIDGE_HUB_PARAID"))),
            bridgeHubAgentID: keccak256("bridgeHub"),
            assetHubParaID: ParaID.wrap(uint32(vm.envUint("ASSET_HUB_PARAID"))),
            assetHubAgentID: keccak256("assetHub"),
            createTokenFee: vm.envUint("CREATE_TOKEN_FEE"),
            createTokenCallId: bytes2(vm.envBytes("CREATE_CALL_INDEX")),
            gasToForward: vm.envUint("GAS_TO_FORWARD")
        });

        Gateway gatewayLogic = new Gateway();
        //gatewayLogic.initialize(initParams);

        GatewayProxy gateway = new GatewayProxy(address(gatewayLogic), hex"");

        // Deploy WETH for testing
        new WETH9();

        // Fund the sovereign account for the BridgeHub parachain. Used to reward relayers
        // of messages originating from BridgeHub
        uint256 initialDeposit = vm.envUint("BRIDGE_HUB_INITIAL_DEPOSIT");

        address bridgeHubAgent = IGateway(address(gateway)).agentOf(initParams.bridgeHubAgentID);
        address assetHubAgent = IGateway(address(gateway)).agentOf(initParams.assetHubAgentID);

        (bool success,) = bridgeHubAgent.call{value: initialDeposit}("");
        if (!success) {
            revert("failed to deposit");
        }

        (success,) = assetHubAgent.call{value: initialDeposit}("");
        if (!success) {
            revert("failed to deposit");
        }

        vm.stopBroadcast();
    }
}
