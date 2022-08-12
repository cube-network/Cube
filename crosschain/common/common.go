package common

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
)

type StateFn func(root common.Hash) (*state.StateDB, error)

type CrossChainSignature struct {
	valAddr   []byte
	signature []byte
}
