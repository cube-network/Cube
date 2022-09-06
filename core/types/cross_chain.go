package types

import (
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	ct "github.com/tendermint/tendermint/types"
)

type BlockAndCosmosVotes struct {
	Block      *Block
	Signatures []ct.CommitSig `json:"signatures"`
	//CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
}

//type CosmosHeader struct {
//	Hash         common.Hash
//	CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
//}

type CubeAndCosmosVotes struct {
	Header     *Header
	Signatures []ct.CommitSig `json:"signatures"`
	//CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
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

type CosmosHeaderForP2P struct {
	// basic block info
	Version tmversion.Consensus `json:"version"`
	ChainID string              `json:"chain_id"`
	Height  uint64              `json:"height"`
	// Time    time.Time           `json:"time"`
	Time uint64 `json:"time"`
	// prev block info
	LastBlockID ct.BlockID `json:"last_block_id"`
	// hashes of block data
	LastCommitHash tmbytes.HexBytes `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       tmbytes.HexBytes `json:"data_hash"`        // transactions

	// hashes from the app output from the prev block
	ValidatorsHash     tmbytes.HexBytes `json:"validators_hash"`      // validators for the current block
	NextValidatorsHash tmbytes.HexBytes `json:"next_validators_hash"` // validators for the next block
	ConsensusHash      tmbytes.HexBytes `json:"consensus_hash"`       // consensus params for current block
	AppHash            tmbytes.HexBytes `json:"app_hash"`             // state after txs from the previous block
	// root hash of all results from the txs from the previous block
	// see `deterministicResponseDeliverTx` to understand which parts of a tx is hashed into here
	LastResultsHash tmbytes.HexBytes `json:"last_results_hash"`

	// consensus info
	EvidenceHash    tmbytes.HexBytes `json:"evidence_hash"`    // evidence included in the block
	ProposerAddress ct.Address       `json:"proposer_address"` // original proposer of the block

	Round      uint32         `json:"round"`
	BlockID    ct.BlockID     `json:"block_id"`
	Signatures []ct.CommitSig `json:"signatures"`
	Sigtime    []uint64       `json:"sigtime"`
}

func CosmosHeaderFromSignedHeader(h *ct.SignedHeader) *CosmosHeaderForP2P {
	if h == nil {
		log.Error("CosmosHeaderFromSignedHeader failed: CosmosHeaderForP2P is nil")
		return nil
	}
	sigtime := make([]uint64, len(h.Commit.Signatures))
	for i := 0; i < len(sigtime); i++ {
		sigtime[i] = uint64(h.Commit.Signatures[i].Timestamp.Unix())
	}
	return &CosmosHeaderForP2P{
		Version: h.Version,
		ChainID: h.ChainID,
		Height:  uint64(h.Height),
		// Time:               h.Time,
		Time:               uint64(h.Time.Unix()),
		LastBlockID:        h.LastBlockID,
		LastCommitHash:     h.LastCommitHash,
		DataHash:           h.DataHash,
		ValidatorsHash:     h.ValidatorsHash,
		NextValidatorsHash: h.NextValidatorsHash,
		ConsensusHash:      h.ConsensusHash,
		AppHash:            h.AppHash,
		LastResultsHash:    h.LastResultsHash,
		EvidenceHash:       h.EvidenceHash,
		ProposerAddress:    h.ProposerAddress,
		Round:              uint32(h.Commit.Round),
		BlockID:            h.Commit.BlockID,
		Signatures:         h.Commit.Signatures,
		Sigtime:            sigtime,
	}
}

func SignedHeaderFromCosmosHeader(h *CosmosHeaderForP2P) *ct.SignedHeader {
	if h == nil {
		log.Error("SignedHeaderFromCosmosHeader failed: CosmosHeaderForP2P is nil")
		return nil
	}
	header := &ct.Header{
		Version: h.Version,
		ChainID: h.ChainID,
		Height:  int64(h.Height),
		// Time:               h.Time,
		Time:               time.Unix(int64(h.Time), 0),
		LastBlockID:        h.LastBlockID,
		LastCommitHash:     h.LastCommitHash,
		DataHash:           h.DataHash,
		ValidatorsHash:     h.ValidatorsHash,
		NextValidatorsHash: h.NextValidatorsHash,
		ConsensusHash:      h.ConsensusHash,
		AppHash:            h.AppHash,
		LastResultsHash:    h.LastResultsHash,
		EvidenceHash:       h.EvidenceHash,
		ProposerAddress:    h.ProposerAddress,
	}
	commit := &ct.Commit{
		Height:     int64(h.Height),
		Round:      int32(h.Round),
		BlockID:    h.BlockID,
		Signatures: h.Signatures,
	}
	sigtime := h.Sigtime
	for i := 0; i < len(sigtime); i++ {
		commit.Signatures[i].Timestamp = time.Unix(int64(sigtime[i]), 0)
	}
	return &ct.SignedHeader{
		Header: header,
		Commit: commit,
	}
}
