package state

import (
	"github.com/ethereum/go-ethereum/common"
)

type ExecutionHeader struct {
	BeaconBlockRoot common.Hash
	BeaconSlot      uint64
	BlockHash       common.Hash
	BlockNumber     uint64
}

type FinalizedHeader struct {
	BeaconBlockRoot       common.Hash
	BeaconSlot            uint64
	InitialCheckpointRoot common.Hash
	InitialCheckpointSlot uint64
}

type CompactExecutionHeaderState struct {
	ParentHash   common.Hash
	BlockNumber  uint64
	StateRoot    common.Hash
	ReceiptsRoot common.Hash
}
