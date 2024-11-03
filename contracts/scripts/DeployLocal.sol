// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
pragma solidity 0.8.25;

import {WETH9} from "canonical-weth/WETH9.sol";
import {Script} from "forge-std/Script.sol";
import {BeefyClient} from "../src/BeefyClient.sol";

import {IGatewayV2} from "../src/v2/IGateway.sol";
import {GatewayProxy} from "../src/GatewayProxy.sol";
import {Gateway} from "../src/Gateway.sol";
import {MockGatewayV2} from "../test/mocks/MockGatewayV2.sol";
import {Agent} from "../src/Agent.sol";
import {AgentExecutor} from "../src/AgentExecutor.sol";
import {Constants} from "../src/Constants.sol";
import {ChannelID, ParaID, OperatingMode} from "../src/Types.sol";
import {Initializer} from "../src/Initializer.sol";
import {SafeNativeTransfer} from "../src/utils/SafeTransfer.sol";
import {stdJson} from "forge-std/StdJson.sol";
import {UD60x18, ud60x18} from "prb/math/src/UD60x18.sol";

contract DeployLocal is Script {
    using SafeNativeTransfer for address payable;
    using stdJson for string;

    function setUp() public {}

    function run() public {
        uint256 privateKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.rememberKey(privateKey);
        vm.startBroadcast(deployer);

        // BeefyClient
        // Seems `fs_permissions` explicitly configured as absolute path does not work and only allowed from project root
        string memory root = vm.projectRoot();
        string memory beefyCheckpointFile = string.concat(root, "/beefy-state.json");
        string memory beefyCheckpointRaw = vm.readFile(beefyCheckpointFile);
        uint64 startBlock = uint64(beefyCheckpointRaw.readUint(".startBlock"));

        BeefyClient.ValidatorSet memory current = BeefyClient.ValidatorSet(
            uint128(beefyCheckpointRaw.readUint(".current.id")),
            uint128(beefyCheckpointRaw.readUint(".current.length")),
            beefyCheckpointRaw.readBytes32(".current.root")
        );
        BeefyClient.ValidatorSet memory next = BeefyClient.ValidatorSet(
            uint128(beefyCheckpointRaw.readUint(".next.id")),
            uint128(beefyCheckpointRaw.readUint(".next.length")),
            beefyCheckpointRaw.readBytes32(".next.root")
        );

        uint256 randaoCommitDelay = vm.envUint("RANDAO_COMMIT_DELAY");
        uint256 randaoCommitExpiration = vm.envUint("RANDAO_COMMIT_EXP");
        uint256 minimumSignatures = vm.envUint("MINIMUM_REQUIRED_SIGNATURES");
        BeefyClient beefyClient = new BeefyClient(
            randaoCommitDelay,
            randaoCommitExpiration,
            minimumSignatures,
            startBlock,
            current,
            next
        );

        uint8 foreignTokenDecimals = uint8(vm.envUint("FOREIGN_TOKEN_DECIMALS"));
        uint128 maxDestinationFee =
            uint128(vm.envUint("RESERVE_TRANSFER_MAX_DESTINATION_FEE"));

        AgentExecutor executor = new AgentExecutor();
        Gateway gatewayLogic = new Gateway(address(beefyClient), address(executor));

        bool rejectOutboundMessages = vm.envBool("REJECT_OUTBOUND_MESSAGES");
        OperatingMode defaultOperatingMode;
        if (rejectOutboundMessages) {
            defaultOperatingMode = OperatingMode.RejectingOutboundMessages;
        } else {
            defaultOperatingMode = OperatingMode.Normal;
        }

        Initializer.Config memory config = Initializer.Config({
            mode: defaultOperatingMode,
            deliveryCost: uint128(vm.envUint("DELIVERY_COST")),
            registerTokenFee: uint128(vm.envUint("REGISTER_TOKEN_FEE")),
            assetHubCreateAssetFee: uint128(vm.envUint("CREATE_ASSET_FEE")),
            assetHubReserveTransferFee: uint128(vm.envUint("RESERVE_TRANSFER_FEE")),
            exchangeRate: ud60x18(vm.envUint("EXCHANGE_RATE")),
            multiplier: ud60x18(vm.envUint("FEE_MULTIPLIER")),
            rescueOperator: address(0),
            foreignTokenDecimals: foreignTokenDecimals,
            maxDestinationFee: maxDestinationFee
        });

        GatewayProxy gateway =
            new GatewayProxy(address(gatewayLogic), abi.encode(config));

        // Deploy WETH for testing
        new WETH9();

        // Fund the sovereign account for the BridgeHub parachain. Used to reward relayers
        // of messages originating from BridgeHub
        uint256 initialDeposit = vm.envUint("BRIDGE_HUB_INITIAL_DEPOSIT");

        address bridgeHubAgent =
            IGatewayV2(address(gateway)).agentOf(Constants.BRIDGE_HUB_AGENT_ID);
        address assetHubAgent =
            IGatewayV2(address(gateway)).agentOf(Constants.ASSET_HUB_AGENT_ID);

        payable(bridgeHubAgent).safeNativeTransfer(initialDeposit);
        payable(assetHubAgent).safeNativeTransfer(initialDeposit);

        // Deploy MockGatewayV2 for testing
        new MockGatewayV2();

        vm.stopBroadcast();
    }
}
