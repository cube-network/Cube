package systemcontract

import (
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"

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
	return CallContractWithValue(ctx, ctx.Header.Coinbase, to, data, big.NewInt(0))
}

// CallContract executes transaction sent to system contracts.
func CallContractWithValue(ctx *CallContext, from common.Address, to *common.Address, data []byte, value *big.Int) (ret []byte, err error) {
	evm := vm.NewEVM(core.NewEVMBlockContext(ctx.Header, ctx.ChainContext, nil), vm.TxContext{
		Origin:   from,
		GasPrice: big.NewInt(0),
	}, ctx.Statedb, ctx.ChainConfig, vm.Config{})

	ret, _, err = evm.Call(vm.AccountRef(from), *to, data, math.MaxUint64, value)
	// Finalise the statedb so any changes can take effect,
	// and especially if the `from` account is empty, it can be finally deleted.
	ctx.Statedb.Finalise(true)

	return ret, WrapVMError(err, ret)
}

// VMCallContract executes transaction sent to system contracts with given EVM.
func VMCallContract(evm *vm.EVM, from common.Address, to *common.Address, data []byte, gas uint64) (ret []byte, err error) {
	state, ok := evm.StateDB.(*state.StateDB)
	if !ok {
		log.Crit("Unknown statedb type")
	}
	ret, _, err = evm.Call(vm.AccountRef(from), *to, data, gas, big.NewInt(0))
	// Finalise the statedb so any changes can take effect,
	// and especially if the `from` account is empty, it can be finally deleted.
	state.Finalise(true)

	return ret, WrapVMError(err, ret)
}

// WrapVMError wraps vm error with readable reason
func WrapVMError(err error, ret []byte) error {
	if err == vm.ErrExecutionReverted {
		reason, errUnpack := abi.UnpackRevert(common.CopyBytes(ret))
		if errUnpack != nil {
			reason = "internal error"
		}
		return fmt.Errorf("%s: %s", err.Error(), reason)
	}
	return err
}
