// package cosmos

// import (
// 	"time"

// 	"github.com/ethereum/go-ethereum/common"
// 	cubetypes "github.com/ethereum/go-ethereum/core/types"
// 	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
// 	"github.com/ethereum/go-ethereum/log"
// 	"github.com/tendermint/tendermint/privval"
// 	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
// 	"github.com/tendermint/tendermint/proto/tendermint/version"
// 	tenderminttypes "github.com/tendermint/tendermint/types"
// )

// // TODO validator set pubkey, config for demo, register in contract later
// // only one validator now, read more validator addr2pubkey mapping from conf/contract later
// // validator index,pubkey

// type CosmosChain struct {
// 	ChainID string
// 	// TODO lock
// 	light_block map[int64]map[string]*tenderminttypes.LightBlock // cache only for demo, write/read db instead later
// 	// light_block    *lru.ARCCache
// 	validators     []*tenderminttypes.Validator // fixed for demo; full validator set, fixed validator set for demo,
// 	priv_addr_idx  uint32
// 	priv_validator *privval.FilePV // use ed2559 for demo, secp256k1 support later;

// 	blockID           tenderminttypes.BlockID // load best block height later
// 	best_block_height uint64

// 	cube_cosmos_header map[int64]map[string]string
// 	headerfn           cccommon.GetHeaderByNumberFn
// }

// // priv_validator_addr: chaos.validator
// func MakeCosmosChain(chainID string, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn) *CosmosChain {
// 	log.Debug("MakeCosmosChain")
// 	c := &CosmosChain{}
// 	// TODO chainID
// 	c.ChainID = "ibc-1"
// 	c.headerfn = headerfn
// 	c.light_block = make(map[int64]map[string]*tenderminttypes.LightBlock)
// 	c.cube_cosmos_header = make(map[int64]map[string]string)
// 	c.priv_validator = privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
// 	c.priv_validator.Save()
// 	// TODO load validator set
// 	c.priv_addr_idx = 0
// 	val_size := 1
// 	c.validators = make([]*tenderminttypes.Validator, val_size)
// 	priv_val_pubkey, _ := c.priv_validator.GetPubKey()
// 	c.validators[c.priv_addr_idx] = tenderminttypes.NewValidator(priv_val_pubkey, 1)
// 	// TODO load best block
// 	psh := tenderminttypes.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
// 	c.blockID = tenderminttypes.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
// 	c.best_block_height = 0

// 	return c
// }

// func (c *CosmosChain) MakeValidatorSet() *tenderminttypes.ValidatorSet {
// 	vs := &tenderminttypes.ValidatorSet{}
// 	vs.Validators = c.validators
// 	// TODO cube.header.coinbase
// 	vs.Proposer = c.validators[0]

// 	return vs
// }

// func (c *CosmosChain) MakeValidatorshash() []byte {
// 	return c.MakeValidatorSet().Hash()
// }

// func (c *CosmosChain) MakeCosmosSignedHeader(h *cubetypes.Header) *tenderminttypes.SignedHeader {
// 	log.Debug("MakeCosmosSignedHeader")

// 	var app_hash common.Hash
// 	app_hash.SetBytes(h.Extra[32:64])
// 	validator_hash := c.MakeValidatorshash()
// 	header := &tenderminttypes.Header{
// 		Version:            version.Consensus{Block: 11, App: 0},
// 		ChainID:            c.ChainID,
// 		Height:             h.Number.Int64(),
// 		Time:               time.Unix(int64(h.Time), 0),
// 		LastCommitHash:     make([]byte, 32),
// 		LastBlockID:        c.blockID,
// 		DataHash:           h.TxHash[:],
// 		ValidatorsHash:     validator_hash,
// 		NextValidatorsHash: validator_hash,
// 		ConsensusHash:      make([]byte, 32),
// 		AppHash:            app_hash[:],
// 		LastResultsHash:    make([]byte, 32),
// 		EvidenceHash:       make([]byte, 32),
// 		ProposerAddress:    c.validators[0].Address,
// 	}

// 	// TODO
// 	// c.cube_cosmos_header[h.hash] = header.hash
// 	// save leveldb

// 	psh := tenderminttypes.PartSetHeader{Total: 1, Hash: header.Hash()}
// 	c.blockID = tenderminttypes.BlockID{Hash: header.Hash(), PartSetHeader: psh}
// 	signatures := make([]tenderminttypes.CommitSig, len(c.validators))

// 	commit := &tenderminttypes.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
// 	return &tenderminttypes.SignedHeader{Header: header, Commit: commit}
// 	// Tendermint light.Verify()
// }

// func (c *CosmosChain) Vote(block_height int64, cs tenderminttypes.CommitSig, light_block *tenderminttypes.LightBlock) {
// 	// light_block := c.GetLightBlockInternal(block_height)
// 	val_idx := 0
// 	// TODO get val idx from c.valaditors (cs.ValidatorAddress)
// 	light_block.Commit.Signatures[val_idx] = cs
// 	c.SetLightBlock(light_block)
// 	if c.IsLightBlockValid(light_block) {
// 		c.best_block_height = uint64(light_block.Height)
// 	}
// }

// func (c *CosmosChain) MakeLightBlock(h *cubetypes.Header) *tenderminttypes.LightBlock {
// 	// TODO load validator set from h.Extra, fixed for demo
// 	light_block := &tenderminttypes.LightBlock{SignedHeader: c.MakeCosmosSignedHeader(h), ValidatorSet: c.MakeValidatorSet()}
// 	// c.SetLightBlock(light_block)
// 	return light_block
// }

// func (c *CosmosChain) MakeLightBlockAndSign(h *cubetypes.Header) *tenderminttypes.LightBlock {

// 	println("make crosschain block, height --  ", h.Number.Int64(), time.Now().UTC().String())

// 	light_block := c.MakeLightBlock(h)
// 	vote := &tenderminttypes.Vote{
// 		Type:             tmproto.PrecommitType,
// 		Height:           light_block.Height,
// 		Round:            light_block.Commit.Round,
// 		BlockID:          light_block.Commit.BlockID,
// 		Timestamp:        light_block.Time,
// 		ValidatorAddress: c.priv_validator.GetAddress(),
// 		ValidatorIndex:   int32(c.priv_addr_idx),
// 	}
// 	v := vote.ToProto()
// 	c.priv_validator.SignVote(c.ChainID, v)

// 	cc := tenderminttypes.CommitSig{}
// 	cc.BlockIDFlag = tenderminttypes.BlockIDFlagCommit
// 	cc.Timestamp = vote.Timestamp
// 	cc.ValidatorAddress = c.priv_validator.GetAddress()
// 	cc.Timestamp = v.Timestamp
// 	cc.Signature = v.Signature

// 	_, ok := c.cube_cosmos_header[light_block.Height]
// 	if !ok {
// 		c.cube_cosmos_header[light_block.Height] = make(map[string]string)
// 	}
// 	c.cube_cosmos_header[light_block.Height][h.Hash().Hex()] = light_block.Hash().String()
// 	println("make header mapping ", h.Hash().Hex(), " ==> ", light_block.Hash().String(), " height ", light_block.Height)
// 	c.Vote(light_block.Height, cc, light_block)

// 	return light_block
// }

// // func (c *CosmosChain) GetLightBlockInternal(block_height int64) *tenderminttypes.LightBlock {
// // 	light_block, ok := c.light_block[block_height]
// // 	if ok {
// // 		return light_block
// // 	} else {
// // 		return nil
// // 	}
// // }

// func (c *CosmosChain) SetLightBlock(light_block *tenderminttypes.LightBlock) {
// 	if len(c.light_block) > 1024 {
// 		delete(c.light_block, light_block.Header.Height-1024)
// 		delete(c.cube_cosmos_header, light_block.Header.Height-1024)
// 	}
// 	_, ok := c.light_block[light_block.Height]
// 	if !ok {
// 		c.light_block[light_block.Height] = make(map[string]*tenderminttypes.LightBlock)
// 	}
// 	println("set header ", light_block.Hash().String(), " height ", light_block.Height)
// 	c.light_block[light_block.Height][light_block.Hash().String()] = light_block
// }

// func (c *CosmosChain) GetLightBlock(block_height int64) *tenderminttypes.LightBlock {
// 	h := c.headerfn(uint64(block_height))
// 	hs := h.Hash().Hex()

// 	if _, ok := c.cube_cosmos_header[block_height][hs]; !ok {
// 		return nil
// 	}

// 	cs := c.cube_cosmos_header[block_height][hs]

// 	println("get header ", block_height, " cube hash ", hs, " cosmos hs ", cs)
// 	light_block, ok := c.light_block[block_height][cs]
// 	if ok {
// 		if c.IsLightBlockValid(light_block) {
// 			return light_block
// 		} else {
// 			return nil
// 		}
// 	} else {
// 		return nil
// 	}
// }

// func (c *CosmosChain) IsLightBlockValid(light_block *tenderminttypes.LightBlock) bool {
// 	// TODO make sure cube block is stable
// 	votingPowerNeeded := light_block.ValidatorSet.TotalVotingPower() * 2 / 3
// 	var talliedVotingPower int64
// 	for idx, commitSig := range light_block.Commit.Signatures {
// 		if commitSig.BlockIDFlag == tenderminttypes.BlockIDFlagCommit {
// 			talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
// 		}
// 	}

// 	return talliedVotingPower > votingPowerNeeded
// }

package cosmos

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	ct "github.com/tendermint/tendermint/types"
)

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type CosmosChain struct {
	config       *params.ChainConfig
	ChainID      string
	mu           sync.Mutex
	signedHeader map[common.Hash]*ct.SignedHeader // cache only for demo, write/read db instead later
	// light_block    *lru.ARCCache
	//valsMgr     []*ct.Validator // fixed for demo; full validator set, fixed validator set for demo,
	valsMgr *ValidatorsMgr
	//priv_addr_idx  uint32
	privValidator *privval.FilePV // use ed2559 for demo, secp256k1 support later;
	cubeAddr      common.Address

	blockID           ct.BlockID // load best block height later
	best_block_height uint64

	getHeaderByNumber  cccommon.GetHeaderByNumberFn
	cube_cosmos_header map[string][]byte
	latestSignedHeight uint64
	latestSignedHash   common.Hash
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(config *params.ChainConfig, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	c.config = config
	// TODO chainID
	c.ChainID = "ibc-1"
	c.signedHeader = make(map[common.Hash]*ct.SignedHeader)
	c.privValidator = privval.LoadOrGenFilePV(priv_validator_key_file, priv_validator_state_file) //privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
	c.privValidator.Save()

	pubkey, _ := c.privValidator.GetPubKey()
	log.Info("init validator", "pubAddr", pubkey.Address().String(), "privAddr", c.privValidator.GetAddress().String())

	c.getHeaderByNumber = headerfn
	// TODO load validator set, should use contract to deal with validators getting changed in the future
	c.valsMgr = &ValidatorsMgr{config: c.config, getHeaderByNumber: headerfn}

	// TODO load best block
	psh := ct.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = ct.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

func (c *CosmosChain) getAllValidators() {

}

func (c *CosmosChain) makeCosmosSignedHeader(h *et.Header) *ct.SignedHeader {
	log.Info("makeCosmosSignedHeader", "height", h.Number, "hash", h.Hash())
	// TODO find_cosmos_parent_header(h.parent_hash) {return c.cube_cosmos_header[parent_hash]}
	// todo: cannot use header to update validators as validators are only updated every Epoch length to reset votes and checkpoint. see more info from chaos.Prepare()

	var app_hash common.Hash
	app_hash.SetBytes(h.Extra[32:64])

	pubkey, _ := c.privValidator.GetPubKey()
	addr := pubkey.Address()
	//c.valsMgr.updateValidators(h, h.Number.Int64())

	// make header
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32), // todo: to be changed
		LastBlockID:        c.blockID,
		DataHash:           h.TxHash[:],
		ValidatorsHash:     c.valsMgr.Validators.Hash(),
		NextValidatorsHash: c.valsMgr.NextValidators.Hash(),
		ConsensusHash:      make([]byte, 32), // todo: to be changed
		AppHash:            app_hash[:],
		LastResultsHash:    make([]byte, 32), // todo: to be changed
		EvidenceHash:       make([]byte, 32), // todo: to be changed
		ProposerAddress:    addr,             //c.valsMgr.Validators.GetProposer().Address,
	}

	// TODO
	// c.cube_cosmos_header[h.hash] = header.hash
	// save leveldb

	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]ct.CommitSig, c.valsMgr.Validators.Size())

	commit := &ct.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	signedHeader := &ct.SignedHeader{Header: header, Commit: commit}

	c.voteSignedHeader(signedHeader)
	// store header
	c.storeSignedHeader(h.Hash(), signedHeader)
	c.latestSignedHeight = h.Number.Uint64()
	c.latestSignedHash = h.Hash()

	return signedHeader
}

func (c *CosmosChain) voteSignedHeader(header *ct.SignedHeader) {
	pubkey, _ := c.privValidator.GetPubKey()
	addr := pubkey.Address()
	idx, val := c.valsMgr.Validators.GetByAddress(addr)
	if val == nil {
		log.Error("voteSignedHeader", "cosmosAddr", addr.String())
		panic("not a validator")
	}
	vote := &ct.Vote{
		Type:             tmproto.PrecommitType,
		Height:           header.Height,
		Round:            header.Commit.Round,
		BlockID:          header.Commit.BlockID,
		Timestamp:        header.Time,
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
	}
	v := vote.ToProto()
	c.privValidator.SignVote(c.ChainID, v)

	cc := ct.CommitSig{}
	cc.BlockIDFlag = ct.BlockIDFlagCommit
	cc.ValidatorAddress = addr
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	commit := header.Commit
	commit.Signatures[idx] = cc
	header.Commit = commit
}

func (c *CosmosChain) handleSignedHeader(h *et.Header, header *ct.SignedHeader) error {
	log.Info("handleSignedHeader", "height", h.Number, "hash", h.Hash())

	if err := header.ValidateBasic(c.ChainID); err != nil {
		return err
	}

	// check state_root
	var app_hash common.Hash
	copy(app_hash[:], h.Extra[32:64])

	// check validators
	_, vals := c.valsMgr.getValidators(h)
	if !bytes.Equal(header.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("Verify validatorsHash failed. number=%f hash=%s\n", h.Number, h.Hash())
	}
	// check proposer
	proposer := c.valsMgr.getValidator(h.Coinbase)
	if !bytes.Equal(proposer.Address, header.ProposerAddress) {
		return fmt.Errorf("Verify proposer failed. number=%f hash=%s\n", h.Number, h.Hash())
	}

	// TODO 2/3 signature
	// check votes
	sigs := header.Commit.Signatures
	if len(sigs) < 1 {
		return fmt.Errorf("Commit signatures are wrong. number=%f hash=%s\n", h.Number, h.Hash())
	}

	//voteSet := ct.CommitToVoteSet(c.ChainID, header.Commit, vals)
	// check proposer's signature
	idx, val := vals.GetByAddress(proposer.Address)
	vote := header.Commit.GetByIndex(idx) //voteSet.GetByIndex(idx)
	if err := vote.Verify(c.ChainID, val.PubKey); err != nil {
		return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, val.PubKey, err)
	}

	// todo: check other signatures

	// TODO merge signature, not replace header
	// store header
	c.storeSignedHeader(h.Hash(), header)

	return nil
}

func (c *CosmosChain) storeSignedHeader(hash common.Hash, header *ct.SignedHeader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.signedHeader[hash] = header
	log.Info("storeSignedHeader", "hash", hash, "header", header.Hash())
}

func (c *CosmosChain) getSignedHeader(height uint64, hash common.Hash) *ct.SignedHeader {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Info("getSignedHeader", "number", height, "hash", hash)
	return c.signedHeader[hash]
}

func (c *CosmosChain) getSignedHeaderWithSealHash(height uint64, sealHash common.Hash, hash common.Hash) *ct.SignedHeader {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Info("getSignedHeaderWithSealHash", "number", height, "sealHash", sealHash, "hash", hash)
	header := c.signedHeader[sealHash]
	if header == nil && height == c.latestSignedHeight {
		header = c.signedHeader[c.latestSignedHash]
		if header != nil {
			log.Info("getHeaderInstead", "number", height, "hash", c.latestSignedHash)
		} else {
			log.Info("getHeaderInstead failed", "number", height, "hash", c.latestSignedHash)
		}
	}
	c.signedHeader[hash] = header
	return header
}

func (c *CosmosChain) GetLightBlock(block_height int64) *ct.LightBlock {
	h := c.getHeaderByNumber(uint64(block_height))
	if h == nil {
		log.Error("Cannot get block header", "number", block_height)
		return nil
	}
	header := c.getSignedHeader(h.Number.Uint64(), h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", block_height)
		return nil
	}

	// make light block
	_, validators := c.valsMgr.getValidators(h)
	return &ct.LightBlock{SignedHeader: header, ValidatorSet: validators}
}
