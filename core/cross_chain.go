package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	ct "github.com/tendermint/tendermint/types"
)

type BlockAndCosmosHeader struct {
	Block        *types.Block
	CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
}

type CosmosHeader struct {
	Hash         common.Hash
	CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
}

type CubeAndCosmosHeader struct {
	Header       *types.Header
	CosmosHeader *CosmosHeaderForP2P `rlp:"nil"` //ct.SignedHeader
}

type CosmosHeaderForP2P struct {
	// basic block info
	Version tmversion.Consensus `json:"version"`
	ChainID string              `json:"chain_id"`
	Height  uint64              `json:"height"`
	Time    time.Time           `json:"time"`

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

	//BlockID    ct.BlockID     `json:"block_id"`
	Signatures []ct.CommitSig `json:"signatures";rlp:"nil"`
}

func CosmosHeaderFromSignedHeader(h *ct.SignedHeader) *CosmosHeaderForP2P {
	if h == nil {
		return nil
	}
	return &CosmosHeaderForP2P{
		Version:            h.Version,
		ChainID:            h.ChainID,
		Height:             uint64(h.Height),
		Time:               h.Time,
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
		//BlockID:            h.Commit.BlockID,
		Signatures: h.Commit.Signatures,
	}
}

func SignedHeaderFromCosmosHeader(h *CosmosHeaderForP2P) *ct.SignedHeader {
	if h == nil {
		return nil
	}
	header := &ct.Header{
		Version:            h.Version,
		ChainID:            h.ChainID,
		Height:             int64(h.Height),
		Time:               h.Time,
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
	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	commit := &ct.Commit{
		Height:     int64(h.Height),
		Round:      int32(1),
		BlockID:    ct.BlockID{Hash: header.Hash(), PartSetHeader: psh},
		Signatures: h.Signatures,
	}
	return &ct.SignedHeader{
		Header: header,
		Commit: commit,
	}
}
