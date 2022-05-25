package systemcontract

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	SysContractV1 SysContractVersion = iota + 1
)

var sysContracts map[SysContractVersion][]IUpgradeAction = map[SysContractVersion][]IUpgradeAction{
	SysContractV1: HardFork1(),
}

type SysContractVersion int

type IUpgradeAction interface {
	GetName() string
	DoUpdate(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) error
}

func ApplySystemContractUpgrade(version SysContractVersion, state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	if config == nil || header == nil || state == nil {
		return
	}
	height := header.Number
	contracts, ok := sysContracts[version]
	if !ok {
		log.Crit("unsupported SysContractVersion", "version", version)
	}

	for _, contract := range contracts {
		log.Info("system contract upgrade", "version", version, "name", contract.GetName(), "height", height, "chainId", config.ChainID.String())
		err = contract.DoUpdate(state, header, chainContext, config)
		if err != nil {
			log.Error("Upgrade system contract execute error", "version", version, "name", contract.GetName(), "err", err)
			return
		}
	}
	return
}
