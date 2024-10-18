// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.25;

import {Gateway} from "../../src/Gateway.sol";
import {ParaID, OperatingMode} from "../../src/Types.sol";
import {CoreStorage} from "../../src/storage/CoreStorage.sol";
import {Verification} from "../../src/Verification.sol";
import {IInitializable} from "../../src/interfaces/IInitializable.sol";

import {UD60x18} from "prb/math/src/UD60x18.sol";

contract MockGateway is Gateway {
    bool public commitmentsAreVerified;

    constructor(address beefyClient, address agentExecutor)
        Gateway(beefyClient, agentExecutor)
    {}

    function v1_handleAgentExecute_public(bytes calldata params) external {
        this.v1_handleAgentExecute(params);
    }

    function v1_handleCreateAgent_public(bytes calldata params) external {
        this.v1_handleCreateAgent(params);
    }

    function v1_handleUpgrade_public(bytes calldata params) external {
        this.v1_handleUpgrade(params);
    }

    function v1_handleSetOperatingMode_public(bytes calldata params) external {
        this.v1_handleSetOperatingMode(params);
    }

    function v1_handleTransferNativeFromAgent_public(bytes calldata params) external {
        this.v1_handleTransferNativeFromAgent(params);
    }

    function v1_handleSetTokenTransferFees_public(bytes calldata params) external {
        this.v1_handleSetTokenTransferFees(params);
    }

    function v1_handleSetPricingParameters_public(bytes calldata params) external {
        this.v1_handleSetPricingParameters(params);
    }

    function v1_handleUnlockNativeToken_public(bytes calldata params) external {
        this.v1_handleUnlockNativeToken(params);
    }

    function v1_handleRegisterForeignToken_public(bytes calldata params) external {
        this.v1_handleRegisterForeignToken(params);
    }

    function v1_handleMintForeignToken_public(bytes calldata params) external {
        this.v1_handleMintForeignToken(params);
    }

    function setCommitmentsAreVerified(bool value) external {
        commitmentsAreVerified = value;
    }

    function _verifyCommitment(bytes32 commitment, Verification.Proof calldata proof)
        internal
        view
        override
        returns (bool)
    {
        if (BEEFY_CLIENT != address(0)) {
            return super._verifyCommitment(commitment, proof);
        } else {
            // for unit tests, verification is set with commitmentsAreVerified
            return commitmentsAreVerified;
        }
    }
}
