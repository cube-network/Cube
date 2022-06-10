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

func HeliocentrismHardFork() []IUpgradeAction {
	return []IUpgradeAction{&StakingV1{}}
}

type StakingV1 struct {
}

func (h *StakingV1) GetName() string {
	return "StakingV1"
}

func (s *StakingV1) DoUpdate(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	contractCode := common.FromHex(system.StakingV1Code)
	//write code to sys contract
	state.SetCode(system.StakingContract, contractCode)
	log.Debug("Write code to system contract account", "addr", system.StakingContract.String(), "code", system.StakingV1Code)
	return
}
