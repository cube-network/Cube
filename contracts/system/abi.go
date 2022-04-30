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
	StakingABI = `[
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "_admin",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "_firstLockPeriod",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_releasePeriod",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_releaseCnt",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_totalRewards",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_rewardsPerBlock",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_epoch",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_ruEpoch",
					"type": "uint256"
				},
				{
					"internalType": "address payable",
					"name": "_communityPool",
					"type": "address"
				},
				{
					"internalType": "contract IBonusPool",
					"name": "_bonusPool",
					"type": "address"
				}
			],
			"name": "initialize",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "_val",
					"type": "address"
				},
				{
					"internalType": "address",
					"name": "_manager",
					"type": "address"
				},
				{
					"internalType": "uint256",
					"name": "_rate",
					"type": "uint256"
				},
				{
					"internalType": "uint256",
					"name": "_stakeEth",
					"type": "uint256"
				},
				{
					"internalType": "bool",
					"name": "_acceptDelegation",
					"type": "bool"
				}
			],
			"name": "initValidator",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint8",
					"name": "_count",
					"type": "uint8"
				}
			],
			"name": "getTopValidators",
			"outputs": [
				{
					"internalType": "address[]",
					"name": "",
					"type": "address[]"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address[]",
					"name": "newSet",
					"type": "address[]"
				}
			],
			"name": "updateActiveValidatorSet",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "decreaseMissedBlocksCounter",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_rewardsPerBlock",
					"type": "uint256"
				}
			],
			"name": "updateRewardsInfo",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "distributeBlockFee",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "bytes32",
					"name": "punishHash",
					"type": "bytes32"
				}
			],
			"name": "isDoubleSignPunished",
			"outputs": [
				{
					"internalType": "bool",
					"name": "",
					"type": "bool"
				}
			],
			"stateMutability": "view",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "_val",
					"type": "address"
				}
			],
			"name": "lazyPunish",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "bytes32",
					"name": "_punishHash",
					"type": "bytes32"
				},
				{
					"internalType": "address",
					"name": "_val",
					"type": "address"
				}
			],
			"name": "doubleSignPunish",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [],
			"name": "rewardsUpdateEpoch",
			"outputs": [
				{
					"internalType": "uint256",
					"name": "",
					"type": "uint256"
				}
			],
			"stateMutability": "view",
			"type": "function"
		}
	]`

	// CommunityPoolABI contains methods to interactive with CommunityPool contract.
	CommunityPoolABI = `[
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "_admin",
					"type": "address"
				}
			],
			"name": "initialize",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`

	// BonusPoolABI contains methods to interactive with BonusPool contract.
	BonusPoolABI = `[
		{
			"inputs": [
				{
					"internalType": "address",
					"name": "_stakingContract",
					"type": "address"
				}
			],
			"name": "initialize",
			"outputs": [],
			"stateMutability": "payable",
			"type": "function"
		}
	]`

	// GenesisLockABI contains methods to interactive with GenesisLock contract.
	GenesisLockABI = `[
		{
			"inputs": [
				{
					"internalType": "uint256",
					"name": "_periodTime",
					"type": "uint256"
				}
			],
			"name": "initialize",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		},
		{
			"inputs": [
				{
					"internalType": "address[]",
					"name": "userAddress",
					"type": "address[]"
				},
				{
					"internalType": "uint256[]",
					"name": "typeId",
					"type": "uint256[]"
				},
				{
					"internalType": "uint256[]",
					"name": "lockedAmount",
					"type": "uint256[]"
				},
				{
					"internalType": "uint256[]",
					"name": "lockedTime",
					"type": "uint256[]"
				},
				{
					"internalType": "uint256[]",
					"name": "periodAmount",
					"type": "uint256[]"
				}
			],
			"name": "init",
			"outputs": [],
			"stateMutability": "nonpayable",
			"type": "function"
		}
	]`
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

	rawAbiMap := map[common.Address]string{StakingContract: StakingABI, CommunityPoolContract: CommunityPoolABI,
		BonusPoolContract: BonusPoolABI, GenesisLockContract: GenesisLockABI}

	for addr, rawAbi := range rawAbiMap {
		if abi, err := abi.JSON(strings.NewReader(rawAbi)); err != nil {
			panic(err)
		} else {
			abiMap[addr] = abi
		}
	}
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
