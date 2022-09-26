package types

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	ct "github.com/tendermint/tendermint/types"
	"math/big"
)

type BlockAndCosmosVotes struct {
	Block      *Block
	Signatures []ct.CommitSig `json:"signatures"`
}

type CubeAndCosmosVotes struct {
	Header     *Header
	Signatures []ct.CommitSig `json:"signatures"`
}

type CosmosVote struct {
	Number     *big.Int
	HeaderHash common.Hash // cube header's hash
	Index      uint32
	Vote       ct.CommitSig
}

func (h *CosmosVote) Hash() common.Hash {
	return rlpHash(h)
}

type CosmosLackedVoteIndexs struct {
	Number *big.Int
	Hash   common.Hash
	Indexs []*big.Int
}

type CosmosVoteCommit struct {
	Index *big.Int
	Vote  ct.CommitSig
}

type CosmosVotesList struct {
	Number  *big.Int
	Hash    common.Hash
	Commits []CosmosVoteCommit
}

var (
	ErrHandledVote = errors.New("vote already handled")
)
