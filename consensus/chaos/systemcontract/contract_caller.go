package systemcontract

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type CallContext struct {
	Statedb      *state.StateDB
	Header       *types.Header
	ChainContext core.ChainContext
	ChainConfig  *params.ChainConfig
}

// CallContract executes transaction sent to system contracts.
func CallContract(ctx *CallContext, to *common.Address, data []byte) (ret []byte, err error) {
	msg := types.NewMessage(ctx.Header.Coinbase, to, ctx.Statedb.GetNonce(ctx.Header.Coinbase), big.NewInt(0), math.MaxUint64, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, false)
	blockContext := core.NewEVMBlockContext(ctx.Header, ctx.ChainContext, nil)
	vmenv := vm.NewEVM(blockContext, core.NewEVMTxContext(msg), ctx.Statedb, ctx.ChainConfig, vm.Config{})

	ret, _, err = vmenv.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	// Finalise the statedb so any changes can take effect,
	// and especially if the `from` account is empty, it can be finally deleted.
	ctx.Statedb.Finalise(true)
	if err != nil {
		log.Error("ExecuteMsg failed", "err", err, "ret", string(ret))
	}
	return ret, err
}

// VMCallContract executes transaction sent to system contracts with given EVM.
func VMCallContract(evm *vm.EVM, from common.Address, to *common.Address, data []byte) (ret []byte, err error) {
	state, ok := evm.StateDB.(*state.StateDB)
	if !ok {
		log.Crit("Unknown statedb type")
	}
	msg := types.NewMessage(from, to, state.GetNonce(from), big.NewInt(0), math.MaxUint64, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, false)
	ret, _, err = evm.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	// Finalise the statedb so any changes can take effect,
	// and especially if the `from` account is empty, it can be finally deleted.
	state.Finalise(true)
	if err != nil {
		log.Error("ExecuteMsg failed", "err", err, "ret", string(ret))
	}
	return ret, err
}
