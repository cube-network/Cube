package vm

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	CrossChainContractAddr = common.Address{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 'C', 'U', 'B', 'E', '-', 'I', 'B', 'C'}
)

type CrossChainContract interface {
	RequiredGas(input []byte) uint64                               // RequiredPrice calculates the contract gas use
	Run(simulateMode bool, evm *EVM, input []byte) ([]byte, error) // Run runs the precompiled contract
}

func IsCrossChainContract(addr common.Address) bool {
	return addr.String() == CrossChainContractAddr.String()
}

func RunCrossChainContract(cc CrossChainContract, simulateMode bool, evm *EVM, input []byte, suppliedGas uint64) (ret []byte, remainingGas uint64, err error) {
	gasCost := cc.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, ErrOutOfGas
	}
	suppliedGas -= gasCost
	output, err := cc.Run(simulateMode, evm, input)
	return output, suppliedGas, err
}

// TODO: evm binary 只需要指定一个区块哈希和交易就可以模拟执行交易；IBC目前需要把整个模块加载起来才可以；
// demo暂时不支持evm binary单独执行交易；
