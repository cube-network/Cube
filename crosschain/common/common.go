package common

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type StateFn func(root common.Hash) (*state.StateDB, error)
type GetHeaderByNumberFn func(number uint64) *types.Header

type CrossChainSignature struct {
	valAddr   []byte
	signature []byte
}
