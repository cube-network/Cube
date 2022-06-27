// Copyright 2021 The Cube Authors
// This file is part of the Cube library.
//
// The Cube library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Cube library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Cube library. If not, see <http://www.gnu.org/licenses/>.

package systemcontract

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	addressListAdmin        = common.HexToAddress("0x6f326b326fea7d1786e45ff25adb67c8532b5234")
	addressListAdminTestnet = common.HexToAddress("0x148c4f33de145c11bdcfbb0f188dfc8fa75d3568")

	onChainDaoAdmin        = common.HexToAddress("0x93b54cbed48d92fb891ded0d02336dd1e2faa627")
	onChainDaoAdminTestnet = common.HexToAddress("0x91e8bafe89c485645beed0c26fabc79a98a0ce2e")

	AdminDevnet common.Address
)

func GravitationHardFork() []IUpgradeAction {
	return []IUpgradeAction{
		&StakingV2{},
		&AddressList{},
		&OnChainDao{},
	}
}

type StakingV2 struct {
}

func (h *StakingV2) GetName() string {
	return "StakingV2"
}

func (s *StakingV2) DoUpdate(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	contractCode := common.FromHex(system.StakingV2Code)
	//write code to sys contract
	state.SetCode(system.StakingContract, contractCode)
	log.Debug("Write code to system contract account", "addr", system.StakingContract, "code", system.StakingV2Code)
	return
}

// AddressList is used to manage tx by address
type AddressList struct {
}

func (s *AddressList) GetName() string {
	return "AddressList"
}

func (s *AddressList) DoUpdate(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	contractCode := common.FromHex(system.AddressListCode)
	//write addressListCode to sys contract
	state.SetCode(system.AddressListContract, contractCode)
	log.Debug("Write code to system contract account", "addr", system.AddressListContract, "code", system.AddressListCode)

	method := "initialize"

	admin := addressListAdminTestnet
	if config.ChainID.Cmp(params.MainnetChainConfig.ChainID) == 0 {
		admin = addressListAdmin
	} else if config.ChainID.Cmp(params.TestnetChainConfig.ChainID) != 0 && (AdminDevnet != common.Address{}) {
		admin = AdminDevnet
	}

	data, err := system.ABIPack(system.AddressListContract, method, admin)
	if err != nil {
		log.Error("Can't pack data for initialize", "error", err)
		return err
	}

	_, err = CallContract(&CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: chainContext,
		ChainConfig:  config,
	}, &system.AddressListContract, data)
	return err
}

// OnChainDao is used to manage proposal
type OnChainDao struct {
}

func (s *OnChainDao) GetName() string {
	return "OnChainDao"
}

func (s *OnChainDao) DoUpdate(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	contractCode := common.FromHex(system.OnChainDaoCode)
	//write Code to sys contract
	state.SetCode(system.OnChainDaoContract, contractCode)
	log.Debug("Write code to system contract account", "addr", system.OnChainDaoContract, "code", system.OnChainDaoCode)

	method := "initialize"

	admin := onChainDaoAdminTestnet
	if config.ChainID.Cmp(params.MainnetChainConfig.ChainID) == 0 {
		admin = onChainDaoAdmin
	} else if config.ChainID.Cmp(params.TestChainConfig.ChainID) != 0 && (AdminDevnet != common.Address{}) {
		admin = AdminDevnet
	}
	data, err := system.ABIPack(system.OnChainDaoContract, method, admin)
	if err != nil {
		log.Error("Can't pack data for initialize", "error", err)
		return err
	}
	_, err = CallContract(&CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: chainContext,
		ChainConfig:  config,
	}, &system.OnChainDaoContract, data)
	return err
}
