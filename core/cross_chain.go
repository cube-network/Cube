package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ct "github.com/tendermint/tendermint/types"
)

type BlockAndCosmosHeader struct {
	Block        *types.Block
	CosmosHeader *ct.SignedHeader
}

type CosmosHeader struct {
	Hash         common.Hash
	CosmosHeader *ct.SignedHeader
}
