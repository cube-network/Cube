package cosmos

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	cubetypes "github.com/ethereum/go-ethereum/core/types"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	tenderminttypes "github.com/tendermint/tendermint/types"
)

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type CosmosChain struct {
	ChainID string
	// TODO lock
	light_block map[int64]map[string]*tenderminttypes.LightBlock // cache only for demo, write/read db instead later
	// light_block    *lru.ARCCache
	validators     []*tenderminttypes.Validator // fixed for demo; full validator set, fixed validator set for demo,
	priv_addr_idx  uint32
	priv_validator *privval.FilePV // use ed2559 for demo, secp256k1 support later;

	blockID           tenderminttypes.BlockID // load best block height later
	best_block_height uint64

	cube_cosmos_header map[int64]map[string]string
	headerfn           cccommon.GetHeaderByNumberFn
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(chainID string, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	// TODO chainID
	c.ChainID = "ibc-1"
	c.headerfn = headerfn
	c.light_block = make(map[int64]map[string]*tenderminttypes.LightBlock)
	c.cube_cosmos_header = make(map[int64]map[string]string)
	c.priv_validator = privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
	c.priv_validator.Save()
	// TODO load validator set
	c.priv_addr_idx = 0
	val_size := 1
	c.validators = make([]*tenderminttypes.Validator, val_size)
	priv_val_pubkey, _ := c.priv_validator.GetPubKey()
	c.validators[c.priv_addr_idx] = tenderminttypes.NewValidator(priv_val_pubkey, 1)
	// TODO load best block
	psh := tenderminttypes.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = tenderminttypes.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

func (c *CosmosChain) MakeValidatorSet() *tenderminttypes.ValidatorSet {
	vs := &tenderminttypes.ValidatorSet{}
	vs.Validators = c.validators
	// TODO cube.header.coinbase
	vs.Proposer = c.validators[0]

	return vs
}

func (c *CosmosChain) MakeValidatorshash() []byte {
	return c.MakeValidatorSet().Hash()
}

func (c *CosmosChain) MakeCosmosSignedHeader(h *cubetypes.Header) *tenderminttypes.SignedHeader {
	log.Debug("MakeCosmosSignedHeader")

	var app_hash common.Hash
	app_hash.SetBytes(h.Extra[32:64])
	validator_hash := c.MakeValidatorshash()
	header := &tenderminttypes.Header{
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

	// TODO
	// c.cube_cosmos_header[h.hash] = header.hash
	// save leveldb

	psh := tenderminttypes.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = tenderminttypes.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]tenderminttypes.CommitSig, len(c.validators))

	commit := &tenderminttypes.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	return &tenderminttypes.SignedHeader{Header: header, Commit: commit}
	// Tendermint light.Verify()
}

func (c *CosmosChain) Vote(block_height int64, cs tenderminttypes.CommitSig, light_block *tenderminttypes.LightBlock) {
	// light_block := c.GetLightBlockInternal(block_height)
	val_idx := 0
	// TODO get val idx from c.valaditors (cs.ValidatorAddress)
	light_block.Commit.Signatures[val_idx] = cs
	c.SetLightBlock(light_block)
	if c.IsLightBlockValid(light_block) {
		c.best_block_height = uint64(light_block.Height)
	}
}

func (c *CosmosChain) MakeLightBlock(h *cubetypes.Header) *tenderminttypes.LightBlock {
	// TODO load validator set from h.Extra, fixed for demo
	light_block := &tenderminttypes.LightBlock{SignedHeader: c.MakeCosmosSignedHeader(h), ValidatorSet: c.MakeValidatorSet()}
	// c.SetLightBlock(light_block)
	return light_block
}

func (c *CosmosChain) MakeLightBlockAndSign(h *cubetypes.Header) *tenderminttypes.LightBlock {

	println("make crosschain block, height --  ", h.Number.Int64(), time.Now().UTC().String())

	light_block := c.MakeLightBlock(h)
	vote := &tenderminttypes.Vote{
		Type:             tmproto.PrecommitType,
		Height:           light_block.Height,
		Round:            light_block.Commit.Round,
		BlockID:          light_block.Commit.BlockID,
		Timestamp:        light_block.Time,
		ValidatorAddress: c.priv_validator.GetAddress(),
		ValidatorIndex:   int32(c.priv_addr_idx),
	}
	v := vote.ToProto()
	c.priv_validator.SignVote(c.ChainID, v)

	cc := tenderminttypes.CommitSig{}
	cc.BlockIDFlag = tenderminttypes.BlockIDFlagCommit
	cc.Timestamp = vote.Timestamp
	cc.ValidatorAddress = c.priv_validator.GetAddress()
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	_, ok := c.cube_cosmos_header[light_block.Height]
	if !ok {
		c.cube_cosmos_header[light_block.Height] = make(map[string]string)
	}
	c.cube_cosmos_header[light_block.Height][h.Hash().Hex()] = light_block.Hash().String()
	println("make header mapping ", h.Hash().Hex(), " ==> ", light_block.Hash().String(), " height ", light_block.Height)
	c.Vote(light_block.Height, cc, light_block)

	return light_block
}

// func (c *CosmosChain) GetLightBlockInternal(block_height int64) *tenderminttypes.LightBlock {
// 	light_block, ok := c.light_block[block_height]
// 	if ok {
// 		return light_block
// 	} else {
// 		return nil
// 	}
// }

func (c *CosmosChain) SetLightBlock(light_block *tenderminttypes.LightBlock) {
	if len(c.light_block) > 1024 {
		delete(c.light_block, light_block.Header.Height-1024)
		delete(c.cube_cosmos_header, light_block.Header.Height-1024)
	}
	_, ok := c.light_block[light_block.Height]
	if !ok {
		c.light_block[light_block.Height] = make(map[string]*tenderminttypes.LightBlock)
	}
	println("set header ", light_block.Hash().String(), " height ", light_block.Height)
	c.light_block[light_block.Height][light_block.Hash().String()] = light_block
}

func (c *CosmosChain) GetLightBlock(block_height int64) *tenderminttypes.LightBlock {
	h := c.headerfn(uint64(block_height))
	hs := h.Hash().Hex()

	if _, ok := c.cube_cosmos_header[block_height][hs]; !ok {
		return nil
	}

	cs := c.cube_cosmos_header[block_height][hs]

	println("get header ", block_height, " cube hash ", hs, " cosmos hs ", cs)
	light_block, ok := c.light_block[block_height][cs]
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

func (c *CosmosChain) IsLightBlockValid(light_block *tenderminttypes.LightBlock) bool {
	// TODO make sure cube block is stable
	votingPowerNeeded := light_block.ValidatorSet.TotalVotingPower() * 2 / 3
	var talliedVotingPower int64
	for idx, commitSig := range light_block.Commit.Signatures {
		if commitSig.BlockIDFlag == tenderminttypes.BlockIDFlagCommit {
			talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
		}
	}

	return talliedVotingPower > votingPowerNeeded
}
