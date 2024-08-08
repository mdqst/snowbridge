package parachain

import (
	"bytes"
	"context"
	"fmt"

	"github.com/snowfork/go-substrate-rpc-client/v4/scale"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	gsrpc "github.com/snowfork/go-substrate-rpc-client/v4"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/parachain"
	"github.com/snowfork/snowbridge/relayer/chain/relaychain"
	"github.com/snowfork/snowbridge/relayer/contracts"
)

type Scanner struct {
	config    *SourceConfig
	ethConn   *ethereum.Connection
	relayConn *relaychain.Connection
	paraConn  *parachain.Connection
	paraID    uint32
	tasks     chan<- *Task
}

// Scans for all parachain message commitments for the configured parachain channelID that need to be relayed and can be
// proven using the MMR root at the specified beefyBlockNumber of the relay chain.
//
// The algorithm works roughly like this:
//  1. Fetch channel nonce on both sides of the bridge and compare them
//  2. If the nonce on the parachain side is larger that means messages need to be relayed. If not then exit early.
//  3. Scan parachain blocks to figure out exactly which commitments need to be relayed.
//  4. For all the parachain blocks with unsettled commitments, determine the relay chain block number in which the
//     parachain block was included.
func (s *Scanner) Scan(ctx context.Context, beefyBlockNumber uint64) ([]*Task, error) {
	// fetch last parachain header that was finalized *before* the BEEFY block
	beefyBlockMinusOneHash, err := s.relayConn.API().RPC.Chain.GetBlockHash(uint64(beefyBlockNumber - 1))
	if err != nil {
		return nil, fmt.Errorf("fetch block hash for block %v: %w", beefyBlockNumber, err)
	}
	var paraHead types.Header
	ok, err := s.relayConn.FetchParachainHead(beefyBlockMinusOneHash, s.paraID, &paraHead)
	if err != nil {
		return nil, fmt.Errorf("fetch head for parachain %v at block %v: %w", s.paraID, beefyBlockMinusOneHash.Hex(), err)
	}
	if !ok {
		return nil, fmt.Errorf("parachain %v is not registered", s.paraID)
	}

	paraBlockNumber := uint64(paraHead.Number)
	paraBlockHash, err := s.paraConn.API().RPC.Chain.GetBlockHash(paraBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("fetch parachain block hash for block %v: %w", paraBlockNumber, err)
	}

	tasks, err := s.findTasks(ctx, paraBlockNumber, paraBlockHash)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// findTasks finds all the message commitments which need to be relayed
func (s *Scanner) findTasks(
	ctx context.Context,
	paraBlock uint64,
	paraHash types.Hash,
) ([]*Task, error) {
	// Fetch latest nonce in ethereum gateway
	ethInboundNonce, err := s.findLatestNonce(ctx)
	log.WithFields(log.Fields{
		"nonce":     ethInboundNonce,
		"channelID": s.config.ChannelID,
	}).Info("Checked latest nonce delivered to ethereum gateway")

	// Fetch latest nonce in parachain outbound queue
	paraNonceKey, err := types.CreateStorageKey(s.paraConn.Metadata(), "EthereumOutboundQueue", "Nonce", s.config.ChannelID[:], nil)
	if err != nil {
		return nil, fmt.Errorf("create storage key for parachain outbound queue nonce with channelID '%v': %w", s.config.ChannelID, err)
	}
	var paraNonce types.U64
	ok, err := s.paraConn.API().RPC.State.GetStorage(paraNonceKey, &paraNonce, paraHash)
	if err != nil {
		return nil, fmt.Errorf("fetch nonce from parachain outbound queue with key '%v' and hash '%v': %w", paraNonceKey, paraHash, err)
	}
	if !ok {
		log.WithFields(log.Fields{
			"nonceKey":  paraNonceKey,
			"blockHash": paraHash,
		}).Info("Fetched empty nonce from parachain outbound queue")
		paraNonce = 0
	}
	log.WithFields(log.Fields{
		"nonce":     uint64(paraNonce),
		"channelID": s.config.ChannelID,
	}).Info("Checked latest nonce generated by parachain outbound queue")

	if !(uint64(paraNonce) > ethInboundNonce) {
		return nil, nil
	}

	log.Info("Nonces are mismatched, scanning for commitments that need to be relayed")

	tasks, err := s.findTasksImpl(
		ctx,
		paraBlock,
		types.H256(s.config.ChannelID),
		ethInboundNonce+1,
	)
	if err != nil {
		return nil, err
	}

	s.gatherProofInputs(tasks)

	return tasks, nil
}

// Searches from the given parachain block number backwards on the given channel (landID) for all outstanding
// commitments until it finds the given startingNonce
func (s *Scanner) findTasksImpl(
	_ context.Context,
	lastParaBlockNumber uint64,
	channelID types.H256,
	startingNonce uint64,
) ([]*Task, error) {
	log.WithFields(log.Fields{
		"channelID":         channelID,
		"nonce":             startingNonce,
		"latestBlockNumber": lastParaBlockNumber,
	}).Debug("Searching backwards from latest block on parachain to find block with nonce")

	messagesKey, err := types.CreateStorageKey(s.paraConn.Metadata(), "EthereumOutboundQueue", "Messages", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create storage key: %w", err)
	}

	scanOutboundQueueDone := false
	var tasks []*Task

	for currentBlockNumber := lastParaBlockNumber; currentBlockNumber > 0; currentBlockNumber-- {
		if scanOutboundQueueDone {
			break
		}

		log.WithFields(log.Fields{
			"blockNumber": currentBlockNumber,
		}).Debug("Checking header")

		blockHash, err := s.paraConn.API().RPC.Chain.GetBlockHash(currentBlockNumber)
		if err != nil {
			return nil, fmt.Errorf("fetch block hash for block %v: %w", currentBlockNumber, err)
		}

		header, err := s.paraConn.API().RPC.Chain.GetHeader(blockHash)
		if err != nil {
			return nil, fmt.Errorf("fetch header for block hash %v: %w", blockHash.Hex(), err)
		}

		commitmentHash, err := ExtractCommitmentFromDigest(header.Digest)
		if err != nil {
			return nil, err
		}
		if commitmentHash == nil {
			continue
		}

		var messages []OutboundQueueMessage
		raw, err := s.paraConn.API().RPC.State.GetStorageRaw(messagesKey, blockHash)
		if err != nil {
			return nil, fmt.Errorf("fetch committed messages for block %v: %w", blockHash.Hex(), err)
		}
		decoder := scale.NewDecoder(bytes.NewReader(*raw))
		n, err := decoder.DecodeUintCompact()
		if err != nil {
			return nil, fmt.Errorf("decode message length error: %w", err)
		}
		for i := uint64(0); i < n.Uint64(); i++ {
			m := OutboundQueueMessage{}
			err = decoder.Decode(&m)
			if err != nil {
				return nil, fmt.Errorf("decode message error: %w", err)
			}
			messages = append(messages, m)
		}

		// For the outbound channel, the commitment hash is the merkle root of the messages
		// https://github.com/Snowfork/snowbridge/blob/75a475cbf8fc8e13577ad6b773ac452b2bf82fbb/parachain/pallets/basic-channel/src/outbound/mod.rs#L275-L277
		// To verify it we fetch the message proof from the parachain
		result, err := scanForOutboundQueueProofs(
			s.paraConn.API(),
			blockHash,
			*commitmentHash,
			startingNonce,
			channelID,
			messages,
		)
		if err != nil {
			return nil, err
		}

		scanOutboundQueueDone = result.scanDone

		if len(result.proofs) > 0 {
			task := Task{
				Header:        header,
				MessageProofs: &result.proofs,
				ProofInput:    nil,
				ProofOutput:   nil,
			}
			tasks = append(tasks, &task)
		}
	}

	// Reverse tasks, effectively sorting by ascending block number
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}

	return tasks, nil
}

type PersistedValidationData struct {
	ParentHead             []byte
	RelayParentNumber      uint32
	RelayParentStorageRoot types.Hash
	MaxPOVSize             uint32
}

// For each task, gatherProofInputs will search to find the relay chain block
// in which that header was included as well as the parachain heads for that block.
func (s *Scanner) gatherProofInputs(
	tasks []*Task,
) error {
	for _, task := range tasks {

		log.WithFields(log.Fields{
			"ParaBlockNumber": task.Header.Number,
		}).Debug("Gathering proof inputs for parachain header")

		relayBlockNumber, err := s.findInclusionBlockNumber(uint64(task.Header.Number))
		if err != nil {
			return fmt.Errorf("find inclusion block number for parachain block %v: %w", task.Header.Number, err)
		}

		relayBlockHash, err := s.relayConn.API().RPC.Chain.GetBlockHash(relayBlockNumber)
		if err != nil {
			return fmt.Errorf("fetch relaychain block hash: %w", err)
		}

		parachainHeads, err := s.relayConn.FetchParachainHeads(relayBlockHash)
		if err != nil {
			return fmt.Errorf("fetch parachain heads: %w", err)
		}

		task.ProofInput = &ProofInput{
			ParaID:           s.paraID,
			RelayBlockNumber: relayBlockNumber,
			ParaHeads:        parachainHeads,
		}
	}

	return nil
}

// The process for finalizing a backed parachain header times out after these many blocks:
const FinalizationTimeout = 4

// Find the relaychain block in which a parachain header was included (finalized). This usually happens
// 2-3 blocks after the relaychain block in which the parachain header was backed.
func (s *Scanner) findInclusionBlockNumber(
	paraBlockNumber uint64,
) (uint64, error) {
	validationDataKey, err := types.CreateStorageKey(s.paraConn.Metadata(), "ParachainSystem", "ValidationData", nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create storage key: %w", err)
	}

	paraBlockHash, err := s.paraConn.API().RPC.Chain.GetBlockHash(paraBlockNumber)
	if err != nil {
		return 0, fmt.Errorf("fetch parachain block hash: %w", err)
	}

	var validationData PersistedValidationData
	ok, err := s.paraConn.API().RPC.State.GetStorage(validationDataKey, &validationData, paraBlockHash)
	if err != nil {
		return 0, fmt.Errorf("fetch PersistedValidationData for block %v: %w", paraBlockHash.Hex(), err)
	}
	if !ok {
		return 0, fmt.Errorf("PersistedValidationData not found for block %v", paraBlockHash.Hex())
	}

	startBlock := validationData.RelayParentNumber + 1
	for i := validationData.RelayParentNumber + 1; i < startBlock+FinalizationTimeout; i++ {
		relayBlockHash, err := s.relayConn.API().RPC.Chain.GetBlockHash(uint64(i))
		if err != nil {
			return 0, fmt.Errorf("fetch relaychain block hash: %w", err)
		}

		var paraHead types.Header
		ok, err := s.relayConn.FetchParachainHead(relayBlockHash, s.paraID, &paraHead)
		if err != nil {
			return 0, fmt.Errorf("fetch head for parachain %v at block %v: %w", s.paraID, relayBlockHash.Hex(), err)
		}
		if !ok {
			return 0, fmt.Errorf("parachain %v is not registered", s.paraID)
		}

		if paraBlockNumber == uint64(paraHead.Number) {
			return uint64(i), nil
		}
	}

	return 0, fmt.Errorf("scan terminated")
}

func scanForOutboundQueueProofs(
	api *gsrpc.SubstrateAPI,
	blockHash types.Hash,
	commitmentHash types.H256,
	startingNonce uint64,
	channelID types.H256,
	messages []OutboundQueueMessage,
) (*struct {
	proofs   []MessageProof
	scanDone bool
}, error) {
	var scanDone bool
	proofs := []MessageProof{}

	// There are 4 cases here:
	// 1. There are no messages to relay, continue
	// 2. All messages have been relayed, halt
	// 3. There are messages to relay and *none* have been sent, continue
	// 4. There are messages to relay and *some* have been sent, continue

	// Messages are sorted by nonce ascending. Traverse them backwards to get nonce descending.
	// This allows us to distinguish between cases 2 & 4 above:
	// - When nonce is ascending, we find a message where messageNonce < startingNonce but later messages may have a
	// higher nonce.
	// - When nonce is descending, we either find the first message has messageNonce < startingNonce (all messages have
	// been relayed) or we reach messageNonce == startingNonce, potentially in an earlier block.
	//
	// eg. m1 has nonce 1 and has been relayed. We're looking for messages from nonce 2 upwards in [m1, m2, m3] (m2 and
	// m3). With nonce ascending, m1.nonce < 2 but we can't assume case 2 yet (where all messages have been relayed).
	// With nonce descending, we find m3, then m2 where m2.nonce == 2.

	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]

		if message.ChannelID != channelID {
			continue
		}

		messageNonce := message.Nonce

		// This case will be hit when there are no new messages to relay.
		if messageNonce < startingNonce {
			log.Debugf(
				"Halting scan for channelID '%v'. Messages not committed yet on outbound channel",
				message.ChannelID.Hex(),
			)
			scanDone = true
			break
		}

		messageProof, err := fetchMessageProof(api, blockHash, uint64(i), message)
		if err != nil {
			return nil, err
		}
		// Check that the merkle root in the proof is the same as the digest hash from the header
		if messageProof.Proof.Root != commitmentHash {
			return nil, fmt.Errorf(
				"Halting scan for channelID '%v'. Outbound queue proof root '%v' doesn't match digest item's commitment hash '%v'",
				message.ChannelID.Hex(),
				messageProof.Proof.Root,
				commitmentHash,
			)
		}

		// Collect these commitments
		proofs = append(proofs, messageProof)

		if messageNonce == startingNonce {
			// Terminate scan
			scanDone = true
		}
	}

	// Reverse proofs, effectively sorting by nonce ascending
	for i, j := 0, len(proofs)-1; i < j; i, j = i+1, j-1 {
		proofs[i], proofs[j] = proofs[j], proofs[i]
	}

	return &struct {
		proofs   []MessageProof
		scanDone bool
	}{
		proofs:   proofs,
		scanDone: scanDone,
	}, nil
}

func fetchMessageProof(
	api *gsrpc.SubstrateAPI,
	blockHash types.Hash,
	messageIndex uint64,
	message OutboundQueueMessage,
) (MessageProof, error) {
	var proofHex string

	params, err := types.EncodeToHexString(messageIndex)
	if err != nil {
		return MessageProof{}, fmt.Errorf("encode params: %w", err)
	}

	err = api.Client.Call(&proofHex, "state_call", "OutboundQueueApi_prove_message", params, blockHash.Hex())
	if err != nil {
		return MessageProof{}, fmt.Errorf("call RPC OutboundQueueApi_prove_message(%v, %v): %w", messageIndex, blockHash, err)
	}

	var optionRawMerkleProof OptionRawMerkleProof
	err = types.DecodeFromHexString(proofHex, &optionRawMerkleProof)
	if err != nil {
		return MessageProof{}, fmt.Errorf("decode merkle proof: %w", err)
	}

	if !optionRawMerkleProof.HasValue {
		return MessageProof{}, fmt.Errorf("retrieve proof failed")
	}

	proof, err := NewMerkleProof(optionRawMerkleProof.Value)
	if err != nil {
		return MessageProof{}, fmt.Errorf("decode merkle proof: %w", err)
	}

	return MessageProof{Message: message, Proof: proof}, nil
}

func (s *Scanner) findLatestNonce(ctx context.Context) (uint64, error) {
	// Fetch latest nonce in ethereum gateway
	gatewayAddress := common.HexToAddress(s.config.Contracts.Gateway)
	gatewayContract, err := contracts.NewGateway(
		gatewayAddress,
		s.ethConn.Client(),
	)
	if err != nil {
		return 0, fmt.Errorf("create gateway contract for address '%v': %w", gatewayAddress, err)
	}

	options := bind.CallOpts{
		Pending: true,
		Context: ctx,
	}
	ethInboundNonce, _, err := gatewayContract.ChannelNoncesOf(&options, s.config.ChannelID)
	if err != nil {
		return 0, fmt.Errorf("fetch nonce from gateway contract for channelID '%v': %w", s.config.ChannelID, err)
	}
	return ethInboundNonce, err
}
