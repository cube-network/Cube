package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

// Maximize performance, space for time

func (bc *BlockChain) UpdateBlockStatus(num *big.Int, hash common.Hash, status uint8) error {
	bc.lockBlockStatusCache.Lock()
	defer bc.lockBlockStatusCache.Unlock()

	s, h := rawdb.ReadBlockStatusByNum(bc.db, num)
	if s == status && h == hash {
		return nil
	}
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
	if num.Cmp(last) > 0 {
		rawdb.WriteLastBlockStatusNumber(bc.db, num)
		bc.currentBlockStatusNumber.Store(new(big.Int).Set(num))
	}

	firstCatchup := bc.firstCatchUpNumber.Load().(*big.Int)
	if firstCatchup.Uint64() > 0 && num.Uint64() > firstCatchup.Uint64() && bc.ChaosEngine.AttestationStatus() == types.AttestationPending {
		bc.ChaosEngine.StartAttestation()
		log.Info("StartAttestation", "firstCatchup", firstCatchup.Uint64(), "latestJustifiedNumber", num.Uint64())
	}
	return nil
}
