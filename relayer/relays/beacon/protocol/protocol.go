package protocol

import (
	"encoding/hex"
	"strings"

	"github.com/snowfork/snowbridge/relayer/relays/beacon/config"
)

type Protocol struct {
	Settings               config.SpecSettings
	SlotsPerHistoricalRoot uint64
}

func New(setting config.SpecSettings) *Protocol {
	return &Protocol{
		Settings:               setting,
		SlotsPerHistoricalRoot: setting.SlotsInEpoch * setting.EpochsPerSyncCommitteePeriod,
	}
}

func (p *Protocol) ComputeSyncPeriodAtSlot(slot uint64) uint64 {
	return slot / (p.Settings.SlotsInEpoch * p.Settings.EpochsPerSyncCommitteePeriod)
}

func (p *Protocol) ComputeEpochAtSlot(slot uint64) uint64 {
	return slot / p.Settings.SlotsInEpoch
}

func (p *Protocol) IsStartOfEpoch(slot uint64) bool {
	return slot%p.Settings.SlotsInEpoch == 0
}

func (p *Protocol) CalculateNextCheckpointSlot(slot uint64) uint64 {
	syncPeriod := p.ComputeSyncPeriodAtSlot(slot)

	// on new period boundary
	if syncPeriod*p.Settings.SlotsInEpoch*p.Settings.EpochsPerSyncCommitteePeriod == slot {
		return slot
	}

	return (syncPeriod + 1) * p.Settings.SlotsInEpoch * p.Settings.EpochsPerSyncCommitteePeriod
}

func (p *Protocol) SyncPeriodLength() uint64 {
	return p.Settings.SlotsInEpoch * p.Settings.EpochsPerSyncCommitteePeriod
}

func (p *Protocol) SyncCommitteeSuperMajority(syncCommitteeHex string) (bool, error) {
	bytes, err := hex.DecodeString(strings.Replace(syncCommitteeHex, "0x", "", 1))
	if err != nil {
		return false, err
	}

	var bits []int

	// Convert each byte to bits
	for _, b := range bytes {
		for i := 7; i >= 0; i-- {
			bit := (b >> i) & 1
			bits = append(bits, int(bit))
		}
	}
	sum := 0
	for _, bit := range bits {
		sum += bit
	}
	if sum*3 < int(p.Settings.SyncCommitteeSize)*2 {
		return false, nil
	}
	return true, nil
}

// ForkVersion is a custom type for Ethereum fork versions.
type ForkVersion string

const (
	Deneb   ForkVersion = "Deneb"
	Capella ForkVersion = "Capella"
	Electra ForkVersion = "Electra"
)

func (p *Protocol) ForkVersion(slot uint64) ForkVersion {
	epoch := p.ComputeEpochAtSlot(slot)
	if epoch >= p.Settings.ForkVersions.Electra {
		return Electra
	}
	if epoch >= p.Settings.ForkVersions.Deneb {
		return Deneb
	}
	return Capella
}
