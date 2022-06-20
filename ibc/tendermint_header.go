package ibc

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tt "github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type Cosmos struct {
	ChainID        string
	light_block    map[int64]*tt.LightBlock // cache only for demo, write/read db instead later
	validators     []*tt.Validator          // fixed for demo; full validator set, fixed validator set for demo,
	priv_addr_idx  uint32
	priv_validator *privval.FilePV // use ed2559 for demo, secp256k1 support later;

	blockID           tt.BlockID // load best block height later
	best_block_height uint64
}

// priv_validator_addr: chaos.validator
func MakeCosmos(priv_validator_key_file, priv_validator_state_file string) *Cosmos {
	c := &Cosmos{}
	c.ChainID = "Cube"
	c.light_block = make(map[int64]*tt.LightBlock)
	c.priv_validator, _ = privval.GenFilePV(priv_validator_key_file, priv_validator_state_file, "" /*"secp256k1"*/)
	c.priv_validator.Save()
	// TODO load validator set
	c.priv_addr_idx = 0
	val_size := 1
	c.validators = make([]*tt.Validator, val_size)
	priv_val_pubkey, _ := c.priv_validator.GetPubKey(context.TODO())
	c.validators[c.priv_addr_idx] = tt.NewValidator(priv_val_pubkey, 1)
	// TODO load best block
	psh := tt.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = tt.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

func (c *Cosmos) String() string {
	return fmt.Sprintf(
		"Cosmos{\n ChainID:%v \n len(light_block):%v \n len(validators):%v \npriv_addr_idx:%v \n priv_validator:%v \n blockID:%v}",
		c.ChainID,
		len(c.light_block),
		len(c.validators),
		c.priv_addr_idx,
		c.priv_validator,
		c.blockID,
	)
}

func (c *Cosmos) MakeValidatorSet() *tt.ValidatorSet {
	vs := &tt.ValidatorSet{}
	vs.Validators = c.validators
	// TODO cube.header.coinbase
	vs.Proposer = c.validators[0]

	return vs
}

func (c *Cosmos) MakeValidatorshash() []byte {
	return c.MakeValidatorSet().Hash()
}

func (c *Cosmos) MakeCosmosSignedHeader(h *et.Header, app_hash common.Hash) *tt.SignedHeader {
	validator_hash := c.MakeValidatorshash()
	header := &tt.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32),
		LastBlockID:        c.blockID,
		DataHash:           h.TxHash[:],
		ValidatorsHash:     validator_hash,
		NextValidatorsHash: validator_hash,
		ConsensusHash:      make([]byte, 32),
		AppHash:            app_hash[:],
		LastResultsHash:    make([]byte, 32),
		EvidenceHash:       make([]byte, 32),
		ProposerAddress:    c.validators[0].Address,
	}

	psh := tt.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = tt.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]tt.CommitSig, len(c.validators))

	commit := &tt.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	return &tt.SignedHeader{Header: header, Commit: commit}
	// Tendermint light.Verify()
}

func (c *Cosmos) Vote(block_height int64, cs tt.CommitSig) {
	light_block := c.GetCosmosLightBlockInternal(block_height)
	val_idx := 0
	// TODO get val idx from c.valaditors (cs.ValidatorAddress)
	light_block.Commit.Signatures[val_idx] = cs
	c.SetCosmosLightBlock(light_block)
	if c.IsLightBlockValid(light_block) {
		c.best_block_height = uint64(light_block.Height)
	}
}

func (c *Cosmos) MakeCosmosLightBlock(h *et.Header, app_hash common.Hash) *tt.LightBlock {
	// TODO load validator set from h.Extra, fixed for demo
	light_block := &tt.LightBlock{SignedHeader: c.MakeCosmosSignedHeader(h, app_hash), ValidatorSet: c.MakeValidatorSet()}
	c.SetCosmosLightBlock(light_block)
	return light_block
}

func (c *Cosmos) MakeCosmosLightBlockAndSign(h *et.Header, app_hash common.Hash) *tt.LightBlock {
	light_block := c.MakeCosmosLightBlock(h, app_hash)
	vote := &tt.Vote{
		Type:             tmproto.PrecommitType,
		Height:           light_block.Height,
		Round:            light_block.Commit.Round,
		BlockID:          light_block.Commit.BlockID,
		Timestamp:        light_block.Time,
		ValidatorAddress: c.priv_validator.GetAddress(),
		ValidatorIndex:   int32(c.priv_addr_idx),
	}
	v := vote.ToProto()
	c.priv_validator.SignVote(context.TODO(), c.ChainID, v)

	cc := tt.CommitSig{}
	cc.BlockIDFlag = tt.BlockIDFlagCommit
	cc.Timestamp = vote.Timestamp
	cc.ValidatorAddress = c.priv_validator.GetAddress()
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	c.Vote(light_block.Height, cc)

	return light_block
}

func (c *Cosmos) GetCosmosLightBlockInternal(block_height int64) *tt.LightBlock {
	light_block, ok := c.light_block[block_height]
	if ok {
		return light_block
	} else {
		return nil
	}
}

func (c *Cosmos) SetCosmosLightBlock(light_block *tt.LightBlock) {
	if len(c.light_block) > 100 {
		delete(c.light_block, light_block.Header.Height-100)
	}
	c.light_block[light_block.Height] = light_block
}

func (c *Cosmos) GetCosmosLightBlock(block_height int64) *tt.LightBlock {
	light_block, ok := c.light_block[block_height]
	if ok {
		if c.IsLightBlockValid(light_block) {
			return light_block
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (c *Cosmos) IsLightBlockValid(light_block *tt.LightBlock) bool {
	votingPowerNeeded := light_block.ValidatorSet.TotalVotingPower() * 2 / 3
	var talliedVotingPower int64
	for idx, commitSig := range light_block.Commit.Signatures {
		if commitSig.BlockIDFlag == tt.BlockIDFlagCommit {
			talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
		}
	}

	return talliedVotingPower > votingPowerNeeded
}
