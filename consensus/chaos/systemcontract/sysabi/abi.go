package sysabi

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// ValidatorsInteractiveABI contains all methods to interactive with validator contracts.
const ValidatorsInteractiveABI = `
[
	
]
`

const PunishInteractiveABI = `
[
	{
		"inputs": [],
		"name": "initialize",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
		  {
			"internalType": "address",
			"name": "val",
			"type": "address"
		  }
		],
		"name": "punish",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
		  {
			"internalType": "uint256",
			"name": "epoch",
			"type": "uint256"
		  }
		],
		"name": "decreaseMissedBlocksCounter",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	  }
]
`

const ValidatorsV1InteractiveABI = `[
    {
        "inputs": [
            {
                "internalType": "uint256",
                "name": "",
                "type": "uint256"
            }
        ],
        "name": "activeValidators",
        "outputs": [
            {
                "internalType": "address",
                "name": "",
                "type": "address"
            }
        ],
        "stateMutability": "view",
        "type": "function"
    },
    {
        "inputs": [],
        "name": "distributeBlockReward",
        "outputs": [],
        "stateMutability": "payable",
        "type": "function"
    },
    {
        "inputs": [],
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
                "name": "_candidates",
                "type": "address[]"
            },
            {
                "internalType": "address[]",
                "name": "_manager",
                "type": "address[]"
            },
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
    },
    {
        "inputs": [
            {
                "internalType": "address[]",
                "name": "newSet",
                "type": "address[]"
            },
            {
                "internalType": "uint256",
                "name": "epoch",
                "type": "uint256"
            }
        ],
        "name": "updateActiveValidatorSet",
        "outputs": [],
        "stateMutability": "nonpayable",
        "type": "function"
    }
]`

const PunishV1InteractiveABI = `[
    {
      "inputs": [],
      "name": "initialize",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
]`

var (
	AdminForDevelopChain common.Address
)

var (
	ValidatorsContractName   = "validators"
	PunishContractName       = "punish"
	ValidatorsV1ContractName = "validators_v1"
	PunishV1ContractName     = "punish_v1"
	ValidatorsContractAddr   = common.HexToAddress("0x000000000000000000000000000000000000f000")
	PunishContractAddr       = common.HexToAddress("0x000000000000000000000000000000000000f001")
	ValidatorsV1ContractAddr = common.HexToAddress("0x000000000000000000000000000000000000F005")
	PunishV1ContractAddr     = common.HexToAddress("0x000000000000000000000000000000000000F006")
	DoubleSignPunishToAddr   = common.HexToAddress("0x00000000000000000000000000000000000fffff")
	punishContractAddr       = common.HexToAddress("0x000000000000000000000000000000000000F006")

	abiMap  map[string]abi.ABI
	addrMap map[string]common.Address
)

func init() {
	abiMap = make(map[string]abi.ABI, 0)
	tmpABI, _ := abi.JSON(strings.NewReader(ValidatorsInteractiveABI))
	abiMap[ValidatorsContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(PunishInteractiveABI))
	abiMap[PunishContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(ValidatorsV1InteractiveABI))
	abiMap[ValidatorsV1ContractName] = tmpABI
	tmpABI, _ = abi.JSON(strings.NewReader(PunishV1InteractiveABI))
	abiMap[PunishV1ContractName] = tmpABI

	addrMap = make(map[string]common.Address, 0)
	addrMap[ValidatorsV1ContractName] = ValidatorsV1ContractAddr
}

func GetInteractiveABI() map[string]abi.ABI {
	return abiMap
}

func GetContractAddress() map[string]common.Address {
	return addrMap
}

func GetValidatorAddr(blockNum *big.Int, config *params.ChainConfig) *common.Address {
	return &ValidatorsV1ContractAddr
}

func GetPunishAddr(blockNum *big.Int, config *params.ChainConfig) *common.Address {
	return &punishContractAddr
}
