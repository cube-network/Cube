package systemcontract

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

const TopValidatorNum uint8 = 21

var (
	blocksPerMonth = big.NewInt(60 * 60 * 24 / 3 * 30)
)

// AddrAscend implements the sort interface to allow sorting a list of addresses
type AddrAscend []common.Address

func (s AddrAscend) Len() int           { return len(s) }
func (s AddrAscend) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s AddrAscend) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetTopValidators return the result of calling method `getTopValidators` in Staking contract
func GetTopValidators(ctx *CallContext) ([]common.Address, error) {
	method := "getTopValidators"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, TopValidatorNum)
	if err != nil {
		log.Error("Can't pack data for getTopValidators", "error", err)
		return []common.Address{}, err
	}

	result, err := CallContract(ctx, &system.StakingContract, data)
	if err != nil {
		log.Error("Failed to perform GetTopValidators", "err", err)
		return []common.Address{}, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return []common.Address{}, err
	}
	if len(ret) != 1 {
		return []common.Address{}, errors.New("GetTopValidators: invalid result length")
	}
	validators, ok := ret[0].([]common.Address)
	if !ok {
		return []common.Address{}, errors.New("GetTopValidators: invalid validator format")
	}
	sort.Sort(AddrAscend(validators))
	return validators, nil
}

// UpdateActiveValidatorSet return the result of calling method `updateActiveValidatorSet` in Staking contract
func UpdateActiveValidatorSet(ctx *CallContext, newValidators []common.Address) error {
	method := "updateActiveValidatorSet"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, newValidators)
	if err != nil {
		log.Error("Can't pack data for updateActiveValidatorSet", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform UpdateActiveValidatorSet", "newValidators", newValidators, "err", err)
		return err
	}
	return nil
}

// DecreaseMissedBlocksCounter return the result of calling method `decreaseMissedBlocksCounter` in Staking contract
func DecreaseMissedBlocksCounter(ctx *CallContext) error {
	method := "decreaseMissedBlocksCounter"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for decreaseMissedBlocksCounter", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform DecreaseMissedBlocksCounter", "err", err)
		return err
	}
	return nil
}

// GetRewardsUpdatePeroid return the blocks to update the reward in Staking contract
func GetRewardsUpdatePeroid(ctx *CallContext) (uint64, error) {
	method := "rewardsUpdateEpoch"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for rewardsUpdateEpoch", "error", err)
		return 0, err
	}
	result, err := CallContract(ctx, &system.StakingContract, data)
	if err != nil {
		log.Error("Failed to perform GetRewardsUpdatePeroid", "err", err)
		return 0, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return 0, err
	}
	if len(ret) != 1 {
		return 0, errors.New("GetRewardsUpdatePeroid: invalid result length")
	}
	rewardsUpdateEpoch, ok := ret[0].(*big.Int)
	if !ok {
		return 0, errors.New("GetRewardsUpdatePeroid: invalid result format")
	}
	return rewardsUpdateEpoch.Uint64(), nil
}

// UpdateRewardsInfo return the result of calling method `updateRewardsInfo` in Staking contract
func UpdateRewardsInfo(ctx *CallContext) error {
	method := "updateRewardsInfo"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)

	// Calculate rewards
	rewardsPerBlock := big.NewInt(0)
	month := new(big.Int).Div(ctx.Header.Number, blocksPerMonth).Int64()
	// After 4 years, rewards is 0
	if month < 48 {
		rewardsPerBlock = new(big.Int).Div(core.RewardsByMonth[month], blocksPerMonth)
	}

	// Execute contract
	data, err := abi.Pack(method, rewardsPerBlock)
	if err != nil {
		log.Error("Can't pack data for updateRewardsInfo", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform UpdateRewardsInfo", "err", err)
		return err
	}
	return nil
}

// DistributeBlockFee return the result of calling method `distributeBlockFee` in Staking contract
func DistributeBlockFee(ctx *CallContext, fee *big.Int) error {
	method := "distributeBlockFee"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for distributeBlockFee", "error", err)
		return err
	}

	if _, err := CallContractWithValue(ctx, &system.StakingContract, data, fee); err != nil {
		log.Error("Failed to perform DistributeBlockFee", "fee", fee, "err", err)
		return err
	}
	return nil
}

// LazyPunish return the result of calling method `lazyPunish` in Staking contract
func LazyPunish(ctx *CallContext, validator common.Address) error {
	method := "lazyPunish"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, validator)
	if err != nil {
		log.Error("Can't pack data for lazyPunish", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform LazyPunish", "validator", validator, "err", err)
		return err
	}
	return nil
}

// DoubleSignPunish return the result of calling method `doubleSignPunish` in Staking contract
func DoubleSignPunish(ctx *CallContext, punishHash common.Hash, validator common.Address) error {
	method := "doubleSignPunish"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, punishHash, validator)
	if err != nil {
		log.Error("Can't pack data for doubleSignPunish", "error", err)
		return err
	}
	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform DoubleSignPunish", "validator", validator, "punishHash", punishHash, "err", err)
		return err
	}
	return nil
}

// DoubleSignPunishGivenEVM return the result of calling method `doubleSignPunish` in Staking contract with given EVM
func DoubleSignPunishGivenEVM(evm *vm.EVM, from common.Address, punishHash common.Hash, validator common.Address) error {
	method := "doubleSignPunish"
	abi := system.GetStakingABI(evm.Context.BlockNumber, evm.ChainConfig())
	// execute contract
	data, err := abi.Pack(method, punishHash, validator)
	if err != nil {
		log.Error("Can't pack data for doubleSignPunish", "error", err)
		return err
	}
	if _, err := VMCallContract(evm, from, &system.StakingContract, data); err != nil {
		log.Error("Failed to perform DoubleSignPunishGivenEVM", "validator", validator, "punishHash", punishHash, "err", err)
		return err
	}
	return nil
}

// IsDoubleSignPunished return the result of calling method `isDoubleSignPunished` in Staking contract
func IsDoubleSignPunished(ctx *CallContext, punishHash common.Hash) (bool, error) {
	method := "isDoubleSignPunished"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, punishHash)
	if err != nil {
		log.Error("Can't pack data for isDoubleSignPunished", "error", err)
		return true, err
	}

	result, err := CallContract(ctx, &system.StakingContract, data)
	if err != nil {
		log.Error("Failed to perform IsDoubleSignPunished", "punishHash", punishHash, "err", err)
		return true, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return true, err
	}
	if len(ret) != 1 {
		return true, errors.New("IsDoubleSignPunished: invalid result length")
	}
	punished, ok := ret[0].(bool)
	if !ok {
		return true, errors.New("IsDoubleSignPunished: invalid result format")
	}
	return punished, nil
}
