package config

import (
	"errors"
	"math"

	"github.com/snowfork/snowbridge/relayer/config"
)

type Config struct {
	Source SourceConfig `mapstructure:"source"`
	Sink   SinkConfig   `mapstructure:"sink"`
}

type SpecSettings struct {
	SlotsInEpoch                 uint64 `mapstructure:"slotsInEpoch"`
	EpochsPerSyncCommitteePeriod uint64 `mapstructure:"epochsPerSyncCommitteePeriod"`
	MaxSlotsPerHistoricalRoot    int    `mapstructure:"maxSlotsPerHistoricalRoot"`
}

type Spec struct {
	Minimal SpecSettings `mapstructure:"minimal"`
	Mainnet SpecSettings `mapstructure:"mainnet"`
}

type SourceConfig struct {
	Beacon BeaconConfig `mapstructure:"beacon"`
}

type BeaconConfig struct {
	Endpoint   string `mapstructure:"endpoint"`
	Spec       Spec   `mapstructure:"spec"`
	ActiveSpec string `mapstructure:"activeSpec"`
}

type SinkConfig struct {
	Parachain config.ParachainConfig `mapstructure:"parachain"`
}

func (c Config) GetSpecSettings() SpecSettings {
	if c.Source.Beacon.ActiveSpec == "minimal" {
		return c.Source.Beacon.Spec.Minimal
	}

	return c.Source.Beacon.Spec.Mainnet
}

func (c Config) GetSpecSettingsBySpec(spec ActiveSpec) SpecSettings {
	if spec.IsMinimal() {
		return c.Source.Beacon.Spec.Minimal
	}

	return c.Source.Beacon.Spec.Mainnet
}

type ActiveSpec string

const (
	Mainnet ActiveSpec = "mainnet"
	Minimal ActiveSpec = "minimal"
	GOERLI  ActiveSpec = "goerli"
)

const (
	// For fork upgrading test locally
	Minimal_CapellaForkEpoch uint64 = 20
	Goerli_CapellaForkEpoch  uint64 = 162304
	// TODO: change when timeline of Capella upgrade in mainnet finalized
	Mainnet_CapellaForkEpoch uint64 = math.MaxUint64
)

func (c Config) GetActiveSpec() ActiveSpec {
	switch c.Source.Beacon.ActiveSpec {
	case string(GOERLI):
		return GOERLI
	case string(Mainnet):
		return Mainnet
	case string(Minimal):
		return Minimal
	default:
		return Mainnet
	}
}

func (a ActiveSpec) IsMainnet() bool {
	return a == Mainnet
}

func (a ActiveSpec) IsMinimal() bool {
	return a == Minimal
}

func (a ActiveSpec) IsGoerli() bool {
	return a == GOERLI
}

func ToSpec(spec string) (ActiveSpec, error) {
	switch spec {
	case string(Mainnet):
		return Mainnet, nil
	case string(Minimal):
		return Minimal, nil
	case string(GOERLI):
		return GOERLI, nil
	default:
		return Minimal, errors.New("spec is not a valid value")
	}
}

func (a ActiveSpec) ToString() string {
	return string(a)
}
