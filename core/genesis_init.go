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

package core

import (
	"errors"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

const (
	initBatch   = 30
	extraVanity = 32                     // Fixed number of extra-data prefix bytes reserved for validator vanity
	extraSeal   = crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for validator seal
)

// genesisInit is tools to init system contracts in genesis
type genesisInit struct {
	state   *state.StateDB
	header  *types.Header
	genesis *Genesis
}

// callContract executes contract in EVM
func (env *genesisInit) callContract(contract common.Address, method string, args ...interface{}) ([]byte, error) {
	// Pack method and args for data seg
	data, err := system.ABIPack(contract, method, args...)
	if err != nil {
		return nil, err
	}
	// Create EVM calling message
	msg := types.NewMessage(env.genesis.Coinbase, &contract, 0, big.NewInt(0), math.MaxUint64, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, false)
	// Set up the initial access list.
	if rules := env.genesis.Config.Rules(env.header.Number); rules.IsBerlin {
		env.state.PrepareAccessList(msg.From(), msg.To(), vm.ActivePrecompiles(rules), msg.AccessList())
	}
	// Create EVM
	evm := vm.NewEVM(NewEVMBlockContext(env.header, nil, &env.header.Coinbase), NewEVMTxContext(msg), env.state, env.genesis.Config, vm.Config{})
	// Run evm call
	ret, _, err := evm.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	if err != nil {
		log.Error("ExecuteMsg failed", "err", err, "ret", string(ret))
	}
	env.state.Finalise(true)
	return ret, err
}

// initStaking initializes Staking Contract
func (env *genesisInit) initStaking() error {
	contract, ok := env.genesis.Alloc[system.StakingContract]
	if !ok {
		return errors.New("Staking Contract is missing in genesis!")
	}

	if len(env.genesis.Validators) <= 0 {
		return errors.New("validators are missing in genesis!")
	}

	totalValidatorStake := big.NewInt(0)
	for _, validator := range env.genesis.Validators {
		totalValidatorStake = new(big.Int).Add(totalValidatorStake, new(big.Int).Mul(validator.Stake, big.NewInt(1000000000000000000)))
	}

	if contract.Balance.Cmp(new(big.Int).Add(totalValidatorStake, contract.Init.TotalRewards)) != 0 {
		return errors.New("Balance of staking contract must be equal to total validator stake plus total staking rewards")
	}

	_, err := env.callContract(system.StakingContract, "initialize",
		contract.Init.Admin,
		contract.Init.FirstLockPeriod,
		contract.Init.ReleasePeriod,
		contract.Init.ReleaseCnt,
		contract.Init.TotalRewards,
		contract.Init.RewardsPerBlock,
		contract.Init.Epoch,
		contract.Init.RuEpoch,
		contract.Init.CommunityPool,
		contract.Init.BonusPool)
	return err
}

// initCommunityPool initializes CommunityPool Contract
func (env *genesisInit) initCommunityPool() error {
	contract, ok := env.genesis.Alloc[system.CommunityPoolContract]
	if !ok {
		return errors.New("CommunityPool Contract is missing in genesis!")
	}
	_, err := env.callContract(system.CommunityPoolContract, "initialize", contract.Init.Admin)
	return err
}

// initBonusPool initializes BonusPool Contract
func (env *genesisInit) initBonusPool() error {
	contract, ok := env.genesis.Alloc[system.BonusPoolContract]
	if !ok {
		return errors.New("BonusPool Contract is missing in genesis!")
	}
	_, err := env.callContract(system.BonusPoolContract, "initialize", contract.Init.StakingContract)
	return err
}

// initGenesisLock initializes GenesisLock Contract
func (env *genesisInit) initGenesisLock() error {
	contract, ok := env.genesis.Alloc[system.GenesisLockContract]
	if !ok {
		return errors.New("GenesisLock Contract is missing in genesis!")
	}

	totalLocked := big.NewInt(0)
	for _, account := range contract.Init.LockedAccounts {
		totalLocked = new(big.Int).Add(totalLocked, account.LockedAmount)
	}
	if contract.Balance.Cmp(totalLocked) != 0 {
		return errors.New("Balance of GenesisLock must be equal to total locked amount")
	}

	var (
		address      = make([]common.Address, 0, initBatch)
		typeId       = make([]*big.Int, 0, initBatch)
		lockedAmount = make([]*big.Int, 0, initBatch)
		lockedTime   = make([]*big.Int, 0, initBatch)
		periodAmount = make([]*big.Int, 0, initBatch)
	)
	sum := 0
	for _, account := range contract.Init.LockedAccounts {
		address = append(address, account.UserAddress)
		typeId = append(typeId, account.TypeId)
		lockedAmount = append(lockedAmount, account.LockedAmount)
		lockedTime = append(lockedTime, account.LockedTime)
		periodAmount = append(periodAmount, account.PeriodAmount)
		sum++
		if sum == initBatch {
			if _, err := env.callContract(system.GenesisLockContract, "init",
				address, typeId, lockedAmount, lockedTime, periodAmount); err != nil {
				return err
			}
			sum = 0
			address = make([]common.Address, 0, initBatch)
			typeId = make([]*big.Int, 0, initBatch)
			lockedAmount = make([]*big.Int, 0, initBatch)
			lockedTime = make([]*big.Int, 0, initBatch)
			periodAmount = make([]*big.Int, 0, initBatch)
		}
	}
	if len(address) > 0 {
		_, err := env.callContract(system.GenesisLockContract, "init",
			address, typeId, lockedAmount, lockedTime, periodAmount)
		return err
	}
	return nil
}

// initValidators add validators into Staking contracts
// and set validator addresses to header extra data
// and return new header extra data
func (env *genesisInit) initValidators() ([]byte, error) {
	if len(env.genesis.Validators) <= 0 {
		return env.header.Extra, errors.New("validators are missing in genesis!")
	}
	extra := make([]byte, 0, extraVanity+common.AddressLength*len(env.genesis.Validators)+extraSeal)
	extra = append(extra, env.header.Extra[:extraVanity]...)
	for _, v := range env.genesis.Validators {
		if _, err := env.callContract(system.StakingContract, "initValidator",
			v.Address, v.Manager, v.Rate, v.Stake, v.AcceptDelegation); err != nil {
			return env.header.Extra, err
		}
		extra = append(extra, v.Address[:]...)
	}
	extra = append(extra, env.header.Extra[len(env.header.Extra)-extraSeal:]...)
	env.header.Extra = extra
	return env.header.Extra, nil
}
