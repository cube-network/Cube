package systemcontract

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const topValidatorNum uint8 = 21

var (
	rewardsByMonth = []*big.Int{
		bigInt("89363425930000000000000"), bigInt("97388213730000000000000"), bigInt("194843300700000000000000"), bigInt("211027180100000000000000"),
		bigInt("316709351000000000000000"), bigInt("341188873400000000000000"), bigInt("465189521400000000000000"), bigInt("508140174800000000000000"),
		bigInt("650765880000000000000000"), bigInt("712496799300000000000000"), bigInt("874059272700000000000000"), bigInt("954884766600000000000000"),
		bigInt("1133513436000000000000000"), bigInt("1231547344000000000000000"), bigInt("1427527831000000000000000"), bigInt("1543058156000000000000000"),
		bigInt("1756680863000000000000000"), bigInt("1890000425000000000000000"), bigInt("2121560614000000000000000"), bigInt("2272967138000000000000000"),
		bigInt("2522765012000000000000000"), bigInt("2692561202000000000000000"), bigInt("2960901990000000000000000"), bigInt("3149395618000000000000000"),
		bigInt("3440061877000000000000000"), bigInt("3651067023000000000000000"), bigInt("3964432396000000000000000"), bigInt("4198325814000000000000000"),
		bigInt("4534770196000000000000000"), bigInt("4791934947000000000000000"), bigInt("5141427924000000000000000"), bigInt("5411750008000000000000000"),
		bigInt("5774509962000000000000000"), bigInt("6058209582000000000000000"), bigInt("6434458551000000000000000"), bigInt("6731759594000000000000000"),
		bigInt("7119755739000000000000000"), bigInt("7428901852000000000000000"), bigInt("7828841775000000000000000"), bigInt("8150031197000000000000000"),
		bigInt("8562114790000000000000000"), bigInt("8895549080000000000000000"), bigInt("9316970322000000000000000"), bigInt("9659820075000000000000000"),
		bigInt("10090735240000000000000000"), bigInt("10443158040000000000000000"), bigInt("10883726020000000000000000"), bigInt("11245882070000000000000000"),
	}

	blocksPerMonth = big.NewInt(60 * 60 * 24 / 3 * 30)
)

// AddrAscend implements the sort interface to allow sorting a list of addresses
type AddrAscend []common.Address

func (s AddrAscend) Len() int           { return len(s) }
func (s AddrAscend) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s AddrAscend) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// bigInt converts a string to big.Int
func bigInt(n string) *big.Int {
	if bint, ok := new(big.Int).SetString(n, 10); !ok {
		panic("Failed to convert to big int:" + n)
	} else {
		return bint
	}
}

// GetTopValidators return the result of calling method `getTopValidators` in Staking contract
func GetTopValidators(ctx *CallContext) ([]common.Address, error) {
	method := "getTopValidators"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, topValidatorNum)
	if err != nil {
		log.Error("Can't pack data for getTopValidators", "error", err)
		return []common.Address{}, err
	}

	result, err := CallContract(ctx, &system.StakingContract, data)
	if err != nil {
		return []common.Address{}, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return []common.Address{}, err
	}
	if len(ret) != 1 {
		return []common.Address{}, errors.New("invalid result length")
	}
	validators, ok := ret[0].([]common.Address)
	if !ok {
		return []common.Address{}, errors.New("invalid validator format")
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
		return 0, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return 0, err
	}
	if len(ret) != 1 {
		return 0, errors.New("invalid result length")
	}
	rewardsUpdateEpoch, ok := ret[0].(*big.Int)
	if !ok {
		return 0, errors.New("invalid result format")
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
		rewardsPerBlock = new(big.Int).Div(rewardsByMonth[month], blocksPerMonth)
	}

	// Execute contract
	data, err := abi.Pack(method, rewardsPerBlock)
	if err != nil {
		log.Error("Can't pack data for updateRewardsInfo", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

// DistributeBlockFee return the result of calling method `distributeBlockFee` in Staking contract
func DistributeBlockFee(ctx *CallContext) error {
	method := "distributeBlockFee"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for distributeBlockFee", "error", err)
		return err
	}

	if _, err := CallContract(ctx, &system.StakingContract, data); err != nil {
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
		return true, err
	}

	// unpack data
	ret, err := abi.Unpack(method, result)
	if err != nil {
		return true, err
	}
	if len(ret) != 1 {
		return true, errors.New("invalid result length")
	}
	punished, ok := ret[0].(bool)
	if !ok {
		return true, errors.New("invalid result format")
	}
	return punished, nil
}

// IsCallingDoubleSignPunish return whether the tx data is calling method `decreaseMissedBlocksCounter` in Staking contract
func IsCallingDoubleSignPunish(header *types.Header, config *params.ChainConfig, data []byte) bool {
	abi := system.GetStakingABI(header.Number, config)
	if method, err := abi.MethodById(data[:4]); err == nil {
		if method != nil && method.Name == "doubleSignPunish" {
			return true
		}
	} else {
		log.Error("Get method ID failed", "err", err)
	}
	return false
}
