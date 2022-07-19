package systemcontract

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/core/state"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//// ChainReader defines a small collection of methods needed to access the local
//// blockchain during header and/or uncle verification.
//type ChainReader interface {
//	// Config retrieves the blockchain's chain configuration.
//	Config() *params.ChainConfig
//
//	// CurrentHeader retrieves the current header from the local chain.
//	CurrentHeader() *types.Header
//
//	// GetHeader retrieves a block header from the database by hash and number.
//	GetHeader(hash common.Hash, number uint64) *types.Header
//
//	// State returns a new mutable state based on the current HEAD block.
//	State() (*state.StateDB, error)
//}
//
//type chainContext struct {
//	chainReader ChainReader
//	engine      consensus.Engine
//}
//
//func NewChainContext(chainReader ChainReader, engine consensus.Engine) *chainContext {
//	return &chainContext{
//		chainReader: chainReader,
//		engine:      engine,
//	}
//}
//
//// Engine retrieves the chain's consensus engine.
//func (cc *chainContext) Engine() consensus.Engine {
//	return cc.engine
//}
//
//// GetHeader returns the hash corresponding to their hash.
//func (cc *chainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
//	return cc.chainReader.GetHeader(hash, number)
//}

type BankContext struct {
	Statedb *state.StateDB
	Header  *types.Header
	Evm     *vm.EVM
	//ChainContext core.ChainContext
	//ChainConfig  *params.ChainConfig
}

func GetBalance(ctx *BankContext, addr sdk.AccAddress, amt sdk.Coin) (*big.Int, error) {
	contract := system.ERC20FactoryContract
	method := "getBalance"
	owner := common.BytesToAddress(addr)
	result, err := callContract(ctx, contract, method, amt.Denom, owner)

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
