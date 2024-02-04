package beefy

import (
	"context"
	"fmt"

	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"golang.org/x/sync/errgroup"

	"github.com/snowfork/snowbridge/relayer/chain/relaychain"
	"github.com/snowfork/snowbridge/relayer/substrate"

	log "github.com/sirupsen/logrus"
)

type PolkadotListener struct {
	config              *SourceConfig
	conn                *relaychain.Connection
	beefyAuthoritiesKey types.StorageKey
}

func NewPolkadotListener(
	config *SourceConfig,
	conn *relaychain.Connection,
) *PolkadotListener {
	return &PolkadotListener{
		config: config,
		conn:   conn,
	}
}

func (li *PolkadotListener) Start(
	ctx context.Context,
	eg *errgroup.Group,
	currentState BeefyState,
) (<-chan Request, error) {
	storageKey, err := types.CreateStorageKey(li.conn.Metadata(), "Beefy", "Authorities", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create storage key: %w", err)
	}
	li.beefyAuthoritiesKey = storageKey

	requests := make(chan Request)

	eg.Go(func() error {
		defer close(requests)
		err := li.scanCommitments(ctx, currentState, requests)
		if err != nil {
			return err
		}
		return nil
	})

	return requests, nil
}

func (li *PolkadotListener) scanCommitments(
	ctx context.Context,
	currentState BeefyState,
	requests chan<- Request,
) error {
	lastSyncedBeefyBlock := currentState.LatestBeefyBlock
	currentValidatorSet := currentState.CurrentValidatorSetId
	in, err := ScanSafeCommitments(ctx, li.conn.Metadata(), li.conn.API(), lastSyncedBeefyBlock+1)
	if err != nil {
		return fmt.Errorf("scan commitments: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case result, ok := <-in:
			if !ok {
				return nil
			}
			if result.Error != nil {
				return fmt.Errorf("scan safe commitments: %w", result.Error)
			}

			committedBeefyBlock := uint64(result.SignedCommitment.Commitment.BlockNumber)
			validatorSetID := result.SignedCommitment.Commitment.ValidatorSetID
			nextValidatorSetID := uint64(result.MMRProof.Leaf.BeefyNextAuthoritySet.ID)

			logEntry := log.WithFields(log.Fields{
				"commitment": log.Fields{
					"blockNumber":        committedBeefyBlock,
					"validatorSetID":     validatorSetID,
					"nextValidatorSetID": nextValidatorSetID,
				},
				"validatorSetID":       currentValidatorSet,
				"IsHandover":           validatorSetID > currentValidatorSet,
				"lastSyncedBeefyBlock": lastSyncedBeefyBlock,
			})

			validators, err := li.queryBeefyAuthorities(result.BlockHash)
			if err != nil {
				return fmt.Errorf("fetch beefy authorities at block %v: %w", result.BlockHash, err)
			}
			currentAuthoritySet, err := li.queryBeefyAuthoritySet(result.BlockHash)
			if err != nil {
				return fmt.Errorf("fetch beefy authoritie set at block %v: %w", result.BlockHash, err)
			}
			task := Request{
				Validators:       validators,
				SignedCommitment: result.SignedCommitment,
				Proof:            result.MMRProof,
			}

			if validatorSetID > currentValidatorSet {
				if currentAuthoritySet.Root == currentState.NextValidatorSetRoot && committedBeefyBlock < lastSyncedBeefyBlock+li.config.UpdatePeriod {
					logEntry.Info("Discarded commitment with beefy authorities not change")
					continue
				}
				task.IsHandover = true
				select {
				case <-ctx.Done():
					return ctx.Err()
				case requests <- task:
					logEntry.Info("New commitment with handover added to channel")
					currentValidatorSet = validatorSetID
					lastSyncedBeefyBlock = committedBeefyBlock
				}
			} else if validatorSetID == currentValidatorSet {
				if result.Depth > li.config.FastForwardDepth {
					logEntry.Warn("Discarded commitment with depth not fast forward")
					continue
				}
				if committedBeefyBlock < lastSyncedBeefyBlock+li.config.UpdatePeriod {
					logEntry.Info("Discarded commitment with sampling")
					continue
				}

				// drop task if it can't be processed immediately
				select {
				case <-ctx.Done():
					return ctx.Err()
				case requests <- task:
					lastSyncedBeefyBlock = committedBeefyBlock
					logEntry.Info("New commitment added to channel")
				default:
					logEntry.Warn("Discarded commitment fail adding to channel")
				}
			} else {
				logEntry.Warn("Discarded invalid commitment")
			}
		}
	}
}

func (li *PolkadotListener) queryBeefyAuthorities(blockHash types.Hash) ([]substrate.Authority, error) {
	var authorities []substrate.Authority
	ok, err := li.conn.API().RPC.State.GetStorage(li.beefyAuthoritiesKey, &authorities, blockHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("beefy authorities not found")
	}

	return authorities, nil
}

func (li *PolkadotListener) queryBeefyNextAuthoritySet(blockHash types.Hash) (types.BeefyNextAuthoritySet, error) {
	var nextAuthoritySet types.BeefyNextAuthoritySet
	storageKey, err := types.CreateStorageKey(li.conn.Metadata(), "MmrLeaf", "BeefyNextAuthorities", nil, nil)
	ok, err := li.conn.API().RPC.State.GetStorage(storageKey, &nextAuthoritySet, blockHash)
	if err != nil {
		return nextAuthoritySet, err
	}
	if !ok {
		return nextAuthoritySet, fmt.Errorf("beefy nextAuthoritySet not found")
	}

	return nextAuthoritySet, nil
}

type BeefyAuthoritySet = types.BeefyNextAuthoritySet

func (li *PolkadotListener) queryBeefyAuthoritySet(blockHash types.Hash) (BeefyAuthoritySet, error) {
	var authoritySet BeefyAuthoritySet
	storageKey, err := types.CreateStorageKey(li.conn.Metadata(), "MmrLeaf", "BeefyAuthorities", nil, nil)
	ok, err := li.conn.API().RPC.State.GetStorage(storageKey, &authoritySet, blockHash)
	if err != nil {
		return authoritySet, err
	}
	if !ok {
		return authoritySet, fmt.Errorf("beefy AuthoritySet not found")
	}

	return authoritySet, nil
}
