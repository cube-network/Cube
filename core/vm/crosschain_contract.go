package vm

import (
	"github.com/ethereum/go-ethereum/common"
)

type CrossChain interface {
	IsCrossChainContract(addr common.Address) bool
	RunCrossChainContract(evm *EVM, input []byte, suppliedGas uint64) (ret []byte, remainingGas uint64, err error)
}
