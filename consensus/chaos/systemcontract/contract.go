package systemcontract

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/chaos/vmcaller"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/log"
)

const topValidatorNum uint8 = 21

var (
	rewardsByMonth = []*big.Int{
		toBigInt("89363425930000000000000"), toBigInt("97388213730000000000000"), toBigInt("194843300700000000000000"), toBigInt("211027180100000000000000"), toBigInt("316709351000000000000000"), toBigInt("341188873400000000000000"),
		toBigInt("465189521400000000000000"), toBigInt("508140174800000000000000"), toBigInt("650765880000000000000000"), toBigInt("712496799300000000000000"), toBigInt("874059272700000000000000"), toBigInt("954884766600000000000000"),
		toBigInt("1133513436000000000000000"), toBigInt("1231547344000000000000000"), toBigInt("1427527831000000000000000"), toBigInt("1543058156000000000000000"), toBigInt("1756680863000000000000000"), toBigInt("1890000425000000000000000"),
		toBigInt("2121560614000000000000000"), toBigInt("2272967138000000000000000"), toBigInt("2522765012000000000000000"), toBigInt("2692561202000000000000000"), toBigInt("2960901990000000000000000"), toBigInt("3149395618000000000000000"),
		toBigInt("3440061877000000000000000"), toBigInt("3651067023000000000000000"), toBigInt("3964432396000000000000000"), toBigInt("4198325814000000000000000"), toBigInt("4534770196000000000000000"), toBigInt("4791934947000000000000000"),
		toBigInt("5141427924000000000000000"), toBigInt("5411750008000000000000000"), toBigInt("5774509962000000000000000"), toBigInt("6058209582000000000000000"), toBigInt("6434458551000000000000000"), toBigInt("6731759594000000000000000"),
		toBigInt("7119755739000000000000000"), toBigInt("7428901852000000000000000"), toBigInt("7828841775000000000000000"), toBigInt("8150031197000000000000000"), toBigInt("8562114790000000000000000"), toBigInt("8895549080000000000000000"),
		toBigInt("9316970322000000000000000"), toBigInt("9659820075000000000000000"), toBigInt("10090735240000000000000000"), toBigInt("10443158040000000000000000"), toBigInt("10883726020000000000000000"), toBigInt("11245882070000000000000000"),
	}

	blocksPerDay   = big.NewInt(60 * 60 * 24 / 3)
	blocksPerMonth = big.NewInt(60 * 60 * 24 / 3 * 30)
)

func toBigInt(n string) *big.Int {
	if bint, ok := new(big.Int).SetString(n, 10); !ok {
		panic("Failed to convert to big int:" + n)
	} else {
		return bint
	}
}

func GetTopValidators(ctx *vmcaller.VMContext) ([]common.Address, error) {
	method := "getTopValidators"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, topValidatorNum)
	if err != nil {
		log.Error("Can't pack data for getTopValidators", "error", err)
		return []common.Address{}, err
	}

	result, err := vmcaller.Execute(ctx, &system.StakingContract, data)
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
	sort.Slice(validators, func(i, j int) bool {
		return bytes.Compare(validators[i][:], validators[j][:]) < 0
	})
	return validators, err
}

func UpdateActiveValidatorSet(ctx *vmcaller.VMContext, newValidators []common.Address) error {
	method := "updateActiveValidatorSet"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, newValidators)
	if err != nil {
		log.Error("Can't pack data for updateActiveValidatorSet", "error", err)
		return err
	}

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

func DecreaseMissedBlocksCounter(ctx *vmcaller.VMContext) error {
	method := "decreaseMissedBlocksCounter"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for decreaseMissedBlocksCounter", "error", err)
		return err
	}

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

func UpdateRewardsInfo(ctx *vmcaller.VMContext) error {
	// Only update once a day
	if new(big.Int).Mod(ctx.Header.Number, blocksPerDay).Int64() != 0 {
		return nil
	}
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

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

func DistributeBlockFee(ctx *vmcaller.VMContext) error {
	method := "distributeBlockFee"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method)
	if err != nil {
		log.Error("Can't pack data for distributeBlockFee", "error", err)
		return err
	}

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

func LazyPunish(ctx *vmcaller.VMContext, validator common.Address) error {
	method := "lazyPunish"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, validator)
	if err != nil {
		log.Error("Can't pack data for lazyPunish", "error", err)
		return err
	}

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}

func DoubleSignPunish(ctx *vmcaller.VMContext, punishHashhash common.Hash, validator common.Address) error {
	method := "doubleSignPunish"
	abi := system.GetStakingABI(ctx.Header.Number, ctx.ChainConfig)
	// execute contract
	data, err := abi.Pack(method, punishHashhash, validator)
	if err != nil {
		log.Error("Can't pack data for doubleSignPunish", "error", err)
		return err
	}

	if _, err := vmcaller.Execute(ctx, &system.StakingContract, data); err != nil {
		return err
	}
	return nil
}
