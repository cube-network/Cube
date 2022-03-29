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
	"math/big"

	"github.com/ethereum/go-ethereum/consensus/chaos/systemcontract/sysabi"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type genesisVMEnv struct {
	state   *state.StateDB
	header  *types.Header
	genesis *Genesis
}

func (env *genesisVMEnv) exec(contract, method string, args ...interface{}) ([]byte, error) {
	// Get contract abi
	contractABIs := sysabi.GetInteractiveABI()
	contractABI, ok := contractABIs[contract]
	if !ok {
		return nil, errors.New("Unknown contract abi: " + contract)
	}
	// Pack method and args for data seg
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	// Get contract address
	contractAddress, ok := sysabi.GetContractAddress()[contract]
	if !ok {
		return nil, errors.New("Unknown contract address: " + contract)
	}
	// Create EVM calling message
	msg := types.NewMessage(env.genesis.Coinbase, &contractAddress, 0, big.NewInt(0), 0, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, true)
	// Create EVM
	evm := vm.NewEVM(NewEVMBlockContext(env.header, nil, &env.header.Coinbase), NewEVMTxContext(msg), env.state, env.genesis.Config, vm.Config{})
	// Run evm call
	ret, _, err := evm.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	if err != nil {
		log.Error("ExecuteMsg failed", "err", err, "ret", string(ret))
	}
	return ret, err
}

func initStaking(env *genesisVMEnv) error {
	env.exec("contract", "method")
	return nil
}
