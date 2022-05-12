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
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

var RewardsByMonth = []*big.Int{
	fromGwei(460590277777778), fromGwei(516685474537037), fromGwei(573248131269290), fromGwei(630282143474312), fromGwei(687791439114376), fromGwei(745779978884773),
	fromGwei(838973978708813), fromGwei(932944595198053), fromGwei(1027698300158040), fromGwei(1123241619326020), fromGwei(1219581132820400), fromGwei(1316723475593910),
	fromGwei(1449397560112750), fromGwei(1583177262002570), fromGwei(1718071794741480), fromGwei(1854090448586550), fromGwei(1991242591213660), fromGwei(2129537668362660),
	fromGwei(2268985204487910), fromGwei(2409594803414200), fromGwei(2551376148998200), fromGwei(2694339005795410), fromGwei(2838493219732590), fromGwei(2983848718785920),
	fromGwei(3154721069220250), fromGwei(3327017355908200), fromGwei(3500749444985210), fromGwei(3675929301471200), fromGwei(3852568990094570), fromGwei(4030680676123140),
	fromGwei(4210276626201940), fromGwei(4391369209198070), fromGwei(4573970897052500), fromGwei(4758094265639040), fromGwei(4943751995630480), fromGwei(5130956873371840),
	fromGwei(5295416236205500), fromGwei(5461246093729430), fromGwei(5628457866732730), fromGwei(5797063071177730), fromGwei(5967073318993100), fromGwei(6138500318873600),
	fromGwei(6276633654864210), fromGwei(6415918101988080), fromGwei(6556363252837980), fromGwei(6697978779944960), fromGwei(6840774436444510), fromGwei(6984760056748220),
}

// fromGwei convert amount from gwei to wei
func fromGwei(gwei int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(gwei), big.NewInt(1000000000))
}

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

	if err == vm.ErrExecutionReverted {
		reason, errUnpack := abi.UnpackRevert(common.CopyBytes(ret))
		if errUnpack != nil {
			reason = "internal error"
		}
		err = fmt.Errorf("%s: %s", err.Error(), reason)
	}

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

	totalRewards := big.NewInt(0)
	for _, rewards := range RewardsByMonth {
		totalRewards = new(big.Int).Add(totalRewards, rewards)
	}

	contract.Balance = new(big.Int).Add(totalValidatorStake, totalRewards)
	env.state.SetBalance(system.StakingContract, contract.Balance)

	blocksPerMonth := big.NewInt(60 * 60 * 24 / 3 * 30)
	rewardsPerBlock := new(big.Int).Div(RewardsByMonth[0], blocksPerMonth)

	_, err := env.callContract(system.StakingContract, "initialize",
		contract.Init.Admin,
		contract.Init.FirstLockPeriod,
		contract.Init.ReleasePeriod,
		contract.Init.ReleaseCnt,
		totalRewards,
		rewardsPerBlock,
		big.NewInt(int64(env.genesis.Config.Chaos.Epoch)),
		contract.Init.RuEpoch,
		system.CommunityPoolContract,
		system.BonusPoolContract)
	if err != nil {
		return err
	}
	if env.genesis.Config != nil && env.genesis.Config.IsHardfork1(common.Big0) {
		_, err = env.callContract(system.StakingContract, "initializeV2",
			system.DAOCharityFoundationContract)
		if err != nil {
			return err
		}
	}
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

// initDAOCharityFoundation initializes DAOCharityFoundation Contract
func (env *genesisInit) initDAOCharityFoundation() error {
	if env.genesis.Config != nil && env.genesis.Config.IsHardfork1(common.Big0) {
		contract, ok := env.genesis.Alloc[system.DAOCharityFoundationContract]
		if !ok {
			return errors.New("DAOCharityFoundation Contract is missing in genesis")
		}
		_, err := env.callContract(system.DAOCharityFoundationContract, "initialize", contract.Init.Admin)
		return err
	}
	return nil
}

// initBonusPool initializes BonusPool Contract
func (env *genesisInit) initBonusPool() error {
	if _, ok := env.genesis.Alloc[system.BonusPoolContract]; !ok {
		return errors.New("BonusPool Contract is missing in genesis!")
	}
	_, err := env.callContract(system.BonusPoolContract, "initialize", system.StakingContract)
	return err
}

// initGenesisLock initializes GenesisLock Contract, including:
// 1. initialize PeriodTime
// 2. init locked accounts
func (env *genesisInit) initGenesisLock() error {
	contract, ok := env.genesis.Alloc[system.GenesisLockContract]
	if !ok {
		return errors.New("GenesisLock Contract is missing in genesis!")
	}

	contract.Balance = big.NewInt(0)
	for _, account := range contract.Init.LockedAccounts {
		contract.Balance = new(big.Int).Add(contract.Balance, account.LockedAmount)
	}
	env.state.SetBalance(system.GenesisLockContract, contract.Balance)

	if _, err := env.callContract(system.GenesisLockContract, "initialize",
		contract.Init.PeriodTime); err != nil {
		return err
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
	activeSet := make([]common.Address, 0, len(env.genesis.Validators))
	extra := make([]byte, 0, extraVanity+common.AddressLength*len(env.genesis.Validators)+extraSeal)
	extra = append(extra, env.header.Extra[:extraVanity]...)
	for _, v := range env.genesis.Validators {
		if _, err := env.callContract(system.StakingContract, "initValidator",
			v.Address, v.Manager, v.Rate, v.Stake, v.AcceptDelegation); err != nil {
			return env.header.Extra, err
		}
		extra = append(extra, v.Address[:]...)
		activeSet = append(activeSet, v.Address)
	}
	extra = append(extra, env.header.Extra[len(env.header.Extra)-extraSeal:]...)
	env.header.Extra = extra
	if _, err := env.callContract(system.StakingContract, "updateActiveValidatorSet", activeSet); err != nil {
		return extra, err
	}
	return env.header.Extra, nil
}
