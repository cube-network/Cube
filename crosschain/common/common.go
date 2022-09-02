package common

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

type StateFn func(root common.Hash) (*state.StateDB, error)
type GetHeaderByNumberFn func(number uint64) *types.Header
type GetHeaderByHashFn func(hash common.Hash) *types.Header
type GetNonceFn func(addr common.Address) uint64
type GetPriceFn func() *big.Int
type SignTxFn func(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
type AddLocalTxFn func(tx *types.Transaction) error

type CrossChainSignature struct {
	valAddr   []byte
	signature []byte
}
