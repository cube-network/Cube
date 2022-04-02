package system

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	// StakingABI contains methods to interactive with Staking contract.
	StakingABI = `
	[
		
	]
	`

	// CommunityPoolABI contains methods to interactive with CommunityPool contract.
	CommunityPoolABI = `
	[
		
	]
	`

	// BonusPoolABI contains methods to interactive with BonusPool contract.
	BonusPoolABI = `
	[
		
	]
	`

	// GenesisLockABI contains methods to interactive with GenesisLock contract.
	GenesisLockABI = `
	[
		
	]
	`
)

var (
	StakingContract       = common.HexToAddress("0x000000000000000000000000000000000000F000")
	CommunityPoolContract = common.HexToAddress("0x000000000000000000000000000000000000F001")
	BonusPoolContract     = common.HexToAddress("0x000000000000000000000000000000000000F002")
	GenesisLockContract   = common.HexToAddress("0x000000000000000000000000000000000000F003")

	abiMap map[common.Address]abi.ABI
)

// init the abiMap
func init() {
	abiMap = make(map[common.Address]abi.ABI, 0)

	stakingABI, _ := abi.JSON(strings.NewReader(StakingABI))
	abiMap[StakingContract] = stakingABI

	communityPoolABI, _ := abi.JSON(strings.NewReader(CommunityPoolABI))
	abiMap[CommunityPoolContract] = communityPoolABI

	bonusPoolABI, _ := abi.JSON(strings.NewReader(BonusPoolABI))
	abiMap[BonusPoolContract] = bonusPoolABI

	genesisLockABI, _ := abi.JSON(strings.NewReader(GenesisLockABI))
	abiMap[GenesisLockContract] = genesisLockABI
}

// ABI return abi for given contract calling
func ABI(contract common.Address) abi.ABI {
	contractABI, ok := abiMap[contract]
	if !ok {
		log.Crit("Unknown system contract: " + contract.String())
	}
	return contractABI
}

// ABIPack generates the data field for given contract calling
func ABIPack(contract common.Address, method string, args ...interface{}) ([]byte, error) {
	return ABI(contract).Pack(method, args...)
}

// StakingABIPack return abi for staking contract calling,
// blockNum, config are used for hard fork contract uprading
func GetStakingABI(blockNum *big.Int, config *params.ChainConfig) abi.ABI {
	return ABI(StakingContract)
}
