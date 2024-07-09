// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2023 Snowfork <hello@snowfork.com>
pragma solidity 0.8.25;

import {Channel, OperatingMode, ChannelID, ParaID} from "../Types.sol";
import {PayMaster} from "../PayMaster.sol";

library CoreStorage {
    struct Layout {
        // Operating mode:
        OperatingMode mode;
        // Message channels
        mapping(ChannelID channelID => Channel) channels;
        // Agents
        mapping(bytes32 agentID => address) agents;
        // V2
        mapping(bytes32 messageHash => bool) messageHashes;
        uint64 nonce;
        PayMaster payMaster;
    }

    bytes32 internal constant SLOT = keccak256("org.snowbridge.storage.core");

    function layout() internal pure returns (Layout storage $) {
        bytes32 slot = SLOT;
        assembly {
            $.slot := slot
        }
    }
}
