package systemcontract

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
	"github.com/ethereum/go-ethereum/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BankContext struct {
	Statedb *state.StateDB
	Header  *types.Header
	Evm     *vm.EVM
}

func GetBalance(ctx *BankContext, addr sdk.AccAddress, denom string) (*big.Int, error) {
	contract := system.ERC20FactoryContract
	method := "getBalance"
	owner := common.BytesToAddress(addr)
	result, err := callContract(ctx, contract, method, denom, owner)

	// unpack data
	ret, err := system.ABIUnpack(contract, method, result)
	if err != nil {
		return nil, err
	}
	if len(ret) != 1 {
		return nil, errors.New("GetBalance: invalid result length")
	}
	balance, ok := ret[0].(*big.Int)
	if !ok {
		return balance, errors.New("GetBalance: invalid result format")
	}
	return balance, nil
}

func GetAllTokens(ctx *BankContext) ([]byte, error) {
	ret, err := callContract(ctx, system.ERC20FactoryContract, "allTokens")
	if err != nil {
		return nil, err
	}
	if len(ret) != 1 {
		return nil, errors.New("GetAllTokens: invalid result length")
	}
	return ret, nil
}

func GetTokenInfo(ctx *BankContext, token string) ([]byte, error) {
	ret, err := callContract(ctx, system.ERC20FactoryContract, "getERC20Info", token)
	if err != nil {
		return nil, err
	}
	if len(ret) != 1 {
		return nil, errors.New("GetTokenInfo: invalid result length")
	}
	return ret, nil
}

func SendCoin(ctx *BankContext, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx, system.ERC20FactoryContract, "transferFrom", amt.Denom, common.BytesToAddress(fromAddr), common.BytesToAddress(toAddr), amt.Amount.BigInt())
}

func MintCoin(ctx *BankContext, moduleAcc sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx, system.ERC20FactoryContract, "mintCoin", amt.Denom, common.BytesToAddress(moduleAcc), amt.Amount.BigInt(), amt.Denom)
}

func BurnCoin(ctx *BankContext, moduleAcc sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx, system.ERC20FactoryContract, "burnCoin", amt.Denom, common.BytesToAddress(moduleAcc), amt.Amount.BigInt())
}

// callContract executes contract in EVM
func callContract(ctx *BankContext, contract common.Address, method string, args ...interface{}) ([]byte, error) {
	// Pack method and args for data seg
	data, err := system.ABIPack(contract, method, args...)
	if err != nil {
		return nil, err
	}

	// Create EVM calling message
	msg := types.NewMessage(ctx.Header.Coinbase, &contract, ctx.Statedb.GetNonce(ctx.Header.Coinbase), big.NewInt(0), math.MaxUint64, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, false)

	//// Create EVM
	//blockContext := core.NewEVMBlockContext(ctx.Header, ctx.ChainContext, nil)
	//vmenv := vm.NewEVM(blockContext, core.NewEVMTxContext(msg), ctx.Statedb, nil, vm.Config{})

	// Run evm call
	ret, _, err := ctx.Evm.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())

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

	// Finalise the statedb so any changes can take effect
	// and especially if the `from` account is empty, it can be finally deleted.
	ctx.Statedb.Finalise(true)

	return ret, err
}
