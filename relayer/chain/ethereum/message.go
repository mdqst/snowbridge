// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package ethereum

import (
	"bytes"
	gethCommon "github.com/ethereum/go-ethereum/common"

	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	etrie "github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/parachain"

	log "github.com/sirupsen/logrus"
)

func MakeMessageFromEvent(event *etypes.Log, receiptsTrie *etrie.Trie) (*parachain.Message, error) {
	// RLP encode event log's Address, Topics, and Data
	var buf bytes.Buffer
	err := event.EncodeRLP(&buf)
	if err != nil {
		return nil, err
	}

	receiptKey, err := rlp.EncodeToBytes(event.TxIndex)
	if err != nil {
		return nil, err
	}

	proof := parachain.NewProofData()
	err = receiptsTrie.Prove(receiptKey, proof)
	if err != nil {
		return nil, err
	}

	var convertedTopics []types.H256
	for _, topic := range event.Topics {
		convertedTopics = append(convertedTopics, types.H256(topic))
	}

	m := parachain.Message{
		EventLog: parachain.EventLog{
			Address: types.H160(event.Address),
			Topics:  convertedTopics,
			Data:    event.Data,
		},
		Proof: parachain.Proof{
			BlockHash: types.NewH256(event.BlockHash.Bytes()),
			TxIndex:   types.NewU32(uint32(event.TxIndex)),
			Data:      proof,
		},
	}

	keys := []gethCommon.Hash{}
	for _, key := range m.Proof.Data.Keys {
		keys = append(keys, gethCommon.BytesToHash(key))
	}
	values := []gethCommon.Hash{}
	for _, value := range m.Proof.Data.Values {
		values = append(values, gethCommon.BytesToHash(value))
	}
	log.WithFields(logrus.Fields{
		"Keys": keys,
	}).Debug("Keys")
	log.WithFields(logrus.Fields{
		"Values": values,
	}).Debug("Values")

	log.WithFields(logrus.Fields{
		"EventLog": m.EventLog,
		"Proof":    m.Proof,
		"txHash":   event.TxHash.Hex(),
	}).Debug("Generated message from Ethereum log")

	return &m, nil
}
