package systemcontract

import (
	"errors"
	"fmt"
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

func GetBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) (*big.Int, error) {
	contract := system.ERC20FactoryContract
	method := "getBalance"
	owner := common.BytesToAddress(addr)
	result, err := callContract(ctx.EVM(), contract, method, amt.Denom, owner)

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

func GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) (sdk.Coins, error) {
	contract := system.ERC20FactoryContract
	method := "allBalances"
	owner := common.BytesToAddress(addr)
	result, err := callContract(ctx.EVM(), contract, method, owner)

	// unpack data
	ret, err := system.ABIUnpack(contract, method, result)
	if err != nil {
		return nil, err
	}
	if len(ret) != 2 {
		return nil, errors.New("GetAllBalances: invalid result length")
	}

	tokens, ok := ret[0].([]string)
	if !ok {
		return nil, errors.New("GetAllBalances: invalid result format")
	}
	balances, ok := ret[1].([]*big.Int)
	if !ok || len(tokens) != len(balances) {
		return nil, errors.New("GetAllBalances: invalid result format")
	}
	coins := make(sdk.Coins, len(tokens))
	for i := 0; i < len(tokens); i++ {
		coins[i] = sdk.Coin{Denom: tokens[i], Amount: sdk.NewInt(balances[i].Int64())}
	}
	return coins, nil
}

func SendCoin(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx.EVM(), system.ERC20FactoryContract, "transferFrom", amt.Denom, common.BytesToAddress(fromAddr), common.BytesToAddress(toAddr), amt.Amount.BigInt())
}

func MintCoin(ctx sdk.Context, moduleAcc sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx.EVM(), system.ERC20FactoryContract, "mintCoin", amt.Denom, common.BytesToAddress(moduleAcc), amt.Amount.BigInt(), amt.Denom)
}

func BurnCoin(ctx sdk.Context, moduleAcc sdk.AccAddress, amt sdk.Coin) ([]byte, error) {
	return callContract(ctx.EVM(), system.ERC20FactoryContract, "burnCoin", amt.Denom, common.BytesToAddress(moduleAcc), amt.Amount.BigInt())
}

// callContract executes contract in EVM
func callContract(evm *vm.EVM, contract common.Address, method string, args ...interface{}) (ret []byte, err error) {
	// Pack method and args for data seg
	data, err := system.ABIPack(contract, method, args...)
	if err != nil {
		return nil, err
	}

	// Create EVM calling message
	// header := ctx.BlockHeader()
	nonce := uint64(1) // todo: get proposer's sequence from account module
	msg := types.NewMessage(system.CrossChainCosmosContract, &contract, nonce, big.NewInt(0), math.MaxUint64, big.NewInt(0), big.NewInt(0), big.NewInt(0), data, nil, false)

	//// Create EVM
	//blockContext := core.NewEVMBlockContext(header, ctx.ChainContext, nil)
	//vmenv := vm.NewEVM(blockContext, core.NewEVMTxContext(msg), ctx.Statedb, ctx.ChainConfig, vm.Config{})

	// Run evm call
	if msg.To().String() == system.AddrToPubkeyMapContract.String() {
		ret, _, err = evm.RunInterpreter(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	} else {
		ret, _, err = evm.Call(vm.AccountRef(msg.From()), *msg.To(), msg.Data(), msg.Gas(), msg.Value())
	}

	if err == vm.ErrExecutionReverted {
		reason, errUnpack := abi.UnpackRevert(common.CopyBytes(ret))
		if errUnpack != nil {
			reason = "internal error: " + errUnpack.Error()
		}
		err = fmt.Errorf("%s: %s", err.Error(), reason)
	}

	if err != nil {
		log.Error("ExecuteMsg failed", "err", err, "ret", string(ret))
	}

	// todo: should be finished in transfer keeper. Finalise the statedb so any changes can take effect
	// and especially if the `from` account is empty, it can be finally deleted.
	//ctx.Statedb.Finalise(true)

	return ret, err
}

func SetState(ctx sdk.Context, key []byte, val []byte, prefix string) ([]byte, error) {
	if val == nil {
		return nil, nil
	}

	method := "set"
	_, err := callContract(ctx.EVM(), system.CrossChainCosmosStateContract, method, key, val, ctx.EVM().Context.BlockNumber.Uint64(), prefix)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func GetRoot(ctx sdk.Context, prefix string) ([][]byte, [][]byte, error) {
	method := "getroot"
	result, err := callContract(ctx.EVM(), system.CrossChainCosmosStateContract, method, prefix)
	if err != nil {
		return nil, nil, err
	}
	ret, err := system.ABIUnpack(system.CrossChainCosmosStateContract, method, result)
	if err != nil {
		return nil, nil, err
	}
	if len(ret) != 2 {
		return nil, nil, errors.New("length")
	}
	k, ok := ret[0].([][]byte)
	if !ok {
		return nil, nil, errors.New("GetRoot result format, length")
	}

	v, ok := ret[1].([][]byte)
	if !ok {
		return nil, nil, errors.New("GetRoot result format, val")
	}
	return k, v, nil
}

func GetState(ctx sdk.Context, key []byte) (bool, []byte, error) {
	method := "get"
	result, err := callContract(ctx.EVM(), system.CrossChainCosmosStateContract, method, key)
	if err != nil {
		return false, nil, err
	}
	ret, err := system.ABIUnpack(system.CrossChainCosmosStateContract, method, result)
	if err != nil {
		return false, nil, err
	}
	if len(ret) != 2 {
		return false, nil, errors.New("length")
	}
	is_exist, ok := ret[0].(bool)
	if !ok {
		return false, nil, errors.New("GetState result format, length")
	}
	if !is_exist {
		return false, nil, nil
	}

	val, ok := ret[1].([]byte)
	if !ok {
		return false, nil, errors.New("GetState result format, val")
	}
	return true, val, nil
}

func DelState(ctx sdk.Context, key []byte) ([]byte, error) {
	method := "del"
	_, err := callContract(ctx.EVM(), system.CrossChainCosmosStateContract, method, key)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func RegisterValidator(evm *vm.EVM, address common.Address, pubkey string) ([]byte, error) {
	return callContract(evm, system.AddrToPubkeyMapContract, "registerValidator", address, pubkey)
}

func GetAllValidators(ctx sdk.Context) ([]common.Address, []string, error) { //map[common.Address]*crosschain.Validator
	contract := system.AddrToPubkeyMapContract
	method := "getAllValidators"
	result, err := callContract(ctx.EVM(), contract, method)
	if err != nil {
		return nil, nil, err
	}

	// unpack data
	ret, err := system.ABIUnpack(contract, method, result)
	if err != nil {
		return nil, nil, err
	}
	if len(ret) != 2 {
		return nil, nil, errors.New("GetAllValidators: invalid result length")
	}
	addrs, ok := ret[0].([]common.Address)
	if !ok {
		return nil, nil, errors.New("GetAllValidators: invalid result format")
	}
	pubkeys, ok := ret[1].([]string)
	if !ok || len(addrs) != len(pubkeys) {
		return nil, nil, errors.New("GetAllValidators: invalid result format")
	}

	return addrs, pubkeys, nil
}

func GetValidator(evm *vm.EVM, address common.Address) (string, error) { // *crosschain.Validator
	contract := system.AddrToPubkeyMapContract
	method := "getValidator"
	result, err := callContract(evm, contract, method, address)
	if err != nil {
		return string(0), err
	}

	// unpack data
	ret, err := system.ABIUnpack(contract, method, result)
	if err != nil {
		return string(0), err
	}
	if len(ret) != 1 {
		return string(0), errors.New("GetValidator: invalid result length")
	}
	pubkey, ok := ret[0].(string)
	if !ok {
		return string(0), errors.New("GetValidator: invalid result format")
	}
	return pubkey, nil
}
