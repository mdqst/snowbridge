// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
pragma solidity 0.8.25;

import {IERC20} from "../interfaces/IERC20.sol";
import {IGateway} from "../interfaces/IGateway.sol";
import {SafeTokenTransferFrom} from "../utils/SafeTransfer.sol";
import {AssetsStorage, TokenInfo} from "../storage/AssetsStorage.sol";
import {CoreStorage} from "../storage/CoreStorage.sol";
import {PricingStorage} from "../storage/PricingStorage.sol";
import {SubstrateTypes} from "../SubstrateTypes.sol";
import {MultiAddress} from "../types/Common.sol";
import {Address} from "../utils/Address.sol";
import {AgentExecutor} from "../AgentExecutor.sol";
import {Agent} from "../Agent.sol";
import {Call} from "../utils/Call.sol";
import {Token} from "../Token.sol";
import {Upgrade} from "../Upgrade.sol";
import {Functions} from "../Functions.sol";
import {Constants} from "../Constants.sol";

import {
    TransferKind,
    UpgradeParams,
    SetOperatingModeParams,
    UnlockNativeTokenParams,
    RegisterForeignTokenParams,
    MintForeignTokenParams,
    CallContractParams
} from "./Types.sol";

library HandlersV2 {
    using Address for address;
    using SafeTokenTransferFrom for IERC20;

    function createAgent(bytes32 origin) external {
        Functions.createAgent(origin);
    }

    function upgrade(bytes calldata data) external {
        UpgradeParams memory params = abi.decode(data, (UpgradeParams));
        Upgrade.upgrade(params.impl, params.implCodeHash, params.initParams);
    }

    function setOperatingMode(bytes calldata data) external {
        SetOperatingModeParams memory params = abi.decode(data, (SetOperatingModeParams));
        CoreStorage.Layout storage $ = CoreStorage.layout();
        $.mode = params.mode;
        emit IGateway.OperatingModeChanged(params.mode);
    }

    // @dev Register a new fungible Polkadot token for an agent
    function registerForeignToken(bytes calldata data) external {
        RegisterForeignTokenParams memory params =
            abi.decode(data, (RegisterForeignTokenParams));
        Functions.registerForeignToken(
            params.foreignTokenID, params.name, params.symbol, params.decimals
        );
    }

    function unlockNativeToken(address executor, bytes calldata data) external {
        UnlockNativeTokenParams memory params =
            abi.decode(data, (UnlockNativeTokenParams));
        address agent = Functions.ensureAgent(Constants.ASSET_HUB_AGENT_ID);
        Functions.withdrawNativeToken(
            executor, agent, params.token, params.recipient, params.amount
        );
    }

    function mintForeignToken(bytes calldata data) external {
        MintForeignTokenParams memory params = abi.decode(data, (MintForeignTokenParams));
        Functions.mintForeignToken(
            params.foreignTokenID, params.recipient, params.amount
        );
    }

    function callContract(bytes32 origin, address executor, bytes calldata data)
        external
    {
        CallContractParams memory params = abi.decode(data, (CallContractParams));
        address agent = Functions.ensureAgent(origin);
        bytes memory call =
            abi.encodeCall(AgentExecutor.callContract, (params.target, params.data));
        Functions.invokeOnAgent(agent, executor, call);
    }
}
