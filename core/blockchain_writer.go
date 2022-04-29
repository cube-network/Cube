package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Maximize performance, space for time

func (bc *BlockChain) UpdateBlockStatus(num *big.Int, hash common.Hash, status uint8) error {
	bc.lockBlockStatusCache.Lock()
	defer bc.lockBlockStatusCache.Unlock()

	err := rawdb.WriteBlockStatus(bc.db, num, hash, status)
	if err != nil {
		return err
	}
	bc.BlockStatusCache.Add(num.Uint64(), &types.BlockStatus{
		BlockNumber: num,
		Hash:        hash,
		Status:      status,
	})
	last := bc.currentBlockStatusNumber.Load().(*big.Int)
	if num.Cmp(last) <= 0 {
		return nil
	}
	rawdb.WriteLastBlockStatusNumber(bc.db, num)
	bc.currentBlockStatusNumber.Store(new(big.Int).Set(num))
	return nil
}
