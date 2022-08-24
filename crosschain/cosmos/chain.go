package cosmos

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
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
	"github.com/tendermint/tendermint/types"
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

	getHeaderByNumber cccommon.GetHeaderByNumberFn
	vote_cache        map[string][]*et.CosmosVote // TODO clean later，avoid OOM
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(config *params.ChainConfig, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	c.config = config
	// c.ChainID = "ibc-1"
	c.ChainID = config.ChainID.String()
	c.vote_cache = make(map[string][]*et.CosmosVote)
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

func (c *CosmosChain) makeCosmosSignedHeader(h *et.Header) *ct.SignedHeader {
	log.Info("makeCosmosSignedHeader", "height", h.Number, "hash", h.Hash())
	// TODO find_cosmos_parent_header(h.parent_hash) {return c.cube_cosmos_header[parent_hash]}
	// todo: cannot use header to update validators as validators are only updated every Epoch length to reset votes and checkpoint. see more info from chaos.Prepare()

	var app_hash common.Hash
	app_hash.SetBytes(h.Extra[32:64])

	pubkey, _ := c.privValidator.GetPubKey()
	addr := pubkey.Address()
	//c.valsMgr.updateValidators(h, h.Number.Int64())

	//lastpsh := ct.PartSetHeader{Total: 1, Hash: h.ParentHash}
	//lastBlockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}

	_, valset := c.valsMgr.getValidators(h.Number.Uint64(), h)
	// TODO NextValidatorsHash N%200 -1,
	// chaos.gettopvalidators(h.number)

	// make header
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32), // todo: to be changed
		LastBlockID:        c.blockID,        // todo: need to get parent header's hash
		DataHash:           h.TxHash[:],
		ValidatorsHash:     valset.Hash(),
		NextValidatorsHash: valset.Hash(),
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
	signatures := make([]ct.CommitSig, valset.Size())
	for i := 0; i < len(signatures); i++ {
		signatures[i].BlockIDFlag = ct.BlockIDFlagAbsent
	}

	commit := &ct.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	signedHeader := &ct.SignedHeader{Header: header, Commit: commit}

	// vals, _ := c.valsMgr.getValidators(h.Number.Uint64(), h)
	c.voteSignedHeader(h, signedHeader)

	// store header
	c.storeSignedHeader(h.Hash(), signedHeader)

	var vote_cache []*et.CosmosVote = nil
	{
		c.mu.Lock()
		if _, ok := c.vote_cache[h.Hash().Hex()]; ok {
			vote_cache = c.vote_cache[h.Hash().Hex()]
			delete(c.vote_cache, h.Hash().Hex())
		}
		c.mu.Unlock()
	}

	if vote_cache != nil {
		for i := 0; i < len(vote_cache); i++ {
			c.handleVote(vote_cache[i])
		}
	}

	return signedHeader
}

func (c *CosmosChain) getValidatorIndex(vals []common.Address) int {
	for i, addr := range vals {
		if bytes.Equal(addr.Bytes(), c.cubeAddr.Bytes()) {
			return i
		}
	}

	return -1
}

func (c *CosmosChain) voteSignedHeader(h *et.Header, header *ct.SignedHeader) (int, ct.CommitSig, error) {
	if header == nil || header.Commit == nil {
		log.Error("voteSignedHeader unknown data")
		return -1, ct.CommitSig{}, errors.New("voteSignedHeader unknown data")
	}

	pubkey, _ := c.privValidator.GetPubKey()
	addr := pubkey.Address()
	//idx := c.getValidatorIndex(vals)
	vals, valset := c.valsMgr.getValidators(uint64(header.Height), h)
	if valset == nil {
		log.Error("getValidators fail")
		return -1, ct.CommitSig{}, errors.New("getValidatorIndex failed")
	}
	idx, _ := valset.GetByAddress(addr)
	if idx < 0 {
		log.Error("getValidatorIndex failed", "cubeAddr", c.cubeAddr, "vals", vals)
		return -1, ct.CommitSig{}, errors.New("getValidatorIndex failed")
	}
	if len(header.Commit.Signatures[idx].Signature) > 0 {
		//log.Debug("voteSignedHeader terminated", "hash", header.Hash())
		return -1, ct.CommitSig{}, nil //errors.New("duplicate vote ")
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
	{
		height, round := vote.Height, vote.Round

		signBytes := types.VoteSignBytes(c.ChainID, v)

		// It passed the checks. Sign the vote
		sig, err := c.privValidator.Key.PrivKey.Sign(signBytes)
		if err != nil {
			log.Debug("sign error! ", err.Error())
			panic("unexpected sign error ")
		}
		c.privValidator.LastSignState.Height = height
		c.privValidator.LastSignState.Round = round
		c.privValidator.LastSignState.Step = 3 /*step*/
		c.privValidator.LastSignState.Signature = sig
		c.privValidator.LastSignState.SignBytes = signBytes
		c.privValidator.LastSignState.Save()

		v.Signature = sig
	}

	cc := ct.CommitSig{}
	cc.BlockIDFlag = ct.BlockIDFlagCommit
	cc.ValidatorAddress = addr
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	commit := header.Commit
	commit.Signatures[idx] = cc
	header.Commit = commit

	return int(idx), cc, nil
}

func (c *CosmosChain) handleSignedHeader(h *et.Header, header *ct.SignedHeader) (*et.CosmosVote, error) {
	log.Info("handleSignedHeader", "height", h.Number, "hash", h.Hash())

	if header.Header == nil {
		return nil, errors.New("missing header")
	}
	if header.Commit == nil {
		return nil, errors.New("missing commit")
	}

	if err := header.Header.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid header: %w", err)
	}
	for _, sig := range header.Commit.Signatures {
		if sig.BlockIDFlag == ct.BlockIDFlagCommit {
			if err := sig.ValidateBasic(); err != nil {
				return nil, fmt.Errorf("invalid commit: %w", err)
			}
		}
	}
	if header.ChainID != c.ChainID {
		return nil, fmt.Errorf("header belongs to another chain %q, not %q", header.ChainID, c.ChainID)
	}

	// Make sure the header is consistent with the commit.
	if header.Commit.Height != header.Height {
		return nil, fmt.Errorf("header and commit height mismatch: %d vs %d", header.Height, header.Commit.Height)
	}
	if hhash, chash := header.Header.Hash(), header.Commit.BlockID.Hash; !bytes.Equal(hhash, chash) {
		return nil, fmt.Errorf("commit signs block %X, header is block %X", chash, hhash)
	}
	//if err := header.ValidateBasic(c.ChainID); err != nil {
	//	return err
	//}

	// todo:need to be verified
	//// check state_root
	//var stateRoot common.Hash
	//copy(stateRoot[:], h.Extra[:32])

	// check validators
	vals, valset := c.valsMgr.getValidators(h.Number.Uint64(), h)
	if valset == nil {
		return nil, fmt.Errorf("Verify getValidators failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}
	if !bytes.Equal(header.ValidatorsHash, valset.Hash()) {
		return nil, fmt.Errorf("Verify validatorsHash failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}
	if len(vals) != len(header.Commit.Signatures) {
		return nil, fmt.Errorf("Verify signatures' count failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}

	// check proposer
	proposer := c.valsMgr.getValidator(h.Coinbase)
	if !bytes.Equal(proposer.Address, header.ProposerAddress) {
		return nil, fmt.Errorf("Verify proposer failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}

	// check votes
	sigs := header.Commit.Signatures
	if len(sigs) < 1 {
		return nil, fmt.Errorf("Commit signatures are wrong. number=%f hash=%s\n", h.Number, h.Hash())
	}

	// check signatures
	commit := header.Commit
	for i, sig := range commit.Signatures {
		if sig.BlockIDFlag == ct.BlockIDFlagCommit {
			realVote := &ct.Vote{
				Type:             tmproto.PrecommitType,
				Height:           header.Height,
				Round:            commit.Round,
				BlockID:          commit.BlockID,
				Timestamp:        header.Time,
				ValidatorAddress: sig.ValidatorAddress,
				ValidatorIndex:   int32(i),
				Signature:        sig.Signature,
			}
			_, val := valset.GetByIndex(int32(i))
			if err := realVote.Verify(c.ChainID, val.PubKey); err != nil {
				return nil, fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, val.PubKey, err)
			}
		}
	}

	// store header
	c.storeSignedHeader(h.Hash(), header)

	// vote
	index, vote, err := c.voteSignedHeader(h, header)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, nil
	}

	// store header
	c.storeSignedHeader(h.Hash(), header)

	cv := &et.CosmosVote{
		Number:     h.Number,
		Vote:       vote,
		Index:      uint32(index),
		HeaderHash: h.Hash(),
	}

	return cv, nil
}

func (c *CosmosChain) storeSignedHeader(hash common.Hash, header *ct.SignedHeader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if header == nil {
		log.Warn("nil header for hash ", hash.Hex())
		return
	}

	// h := c.signedHeader[hash]
	// if h == nil {
	// 	log.Debug("new signed header")
	c.signedHeader[hash] = header
	// } else {
	// 	log.Debug("update signed header")
	// 	for i := 0; i < len(header.Commit.Signatures); i++ {
	// 		if header.Commit.Signatures[i].BlockIDFlag == ct.BlockIDFlagCommit {
	// 			log.Debug("update sig idx ", strconv.Itoa(i), " ", c.signedHeader[hash].Commit.Signatures[i].ValidatorAddress.String(), " ", header.Commit.Signatures[i].ValidatorAddress.String())
	// 			c.signedHeader[hash].Commit.Signatures[i] = header.Commit.Signatures[i]
	// 		}
	// 	}
	// }

	counter := 0
	for i := 0; i < len(c.signedHeader[hash].Commit.Signatures); i++ {
		if c.signedHeader[hash].Commit.Signatures[i].BlockIDFlag == ct.BlockIDFlagCommit {
			counter++
		}
	}
	log.Info("storeSignedHeader", "votes", strconv.Itoa(counter), "number", header.Height, "hash", hash, "header", header.Hash())
}

func (c *CosmosChain) getSignedHeader(height uint64, hash common.Hash) *ct.SignedHeader {
	c.mu.Lock()
	defer c.mu.Unlock()
	//log.Info("getSignedHeader", "number", height, "hash", hash)
	h := c.signedHeader[hash]
	//if h == nil {
	//	log.Error("getSignedHeader failed")
	//}
	return h
}

func (c *CosmosChain) getHeader(block_height int64) *ct.Header {
	// c.mu.Lock()
	// defer c.mu.Unlock()
	h := c.getHeaderByNumber(uint64(block_height))
	if h == nil {
		log.Error("Cannot get block header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	header := c.getSignedHeader(h.Number.Uint64(), h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	log.Debug("getlightblock height ", strconv.Itoa(int(block_height)), h.Hash().Hex())
	return header.Header
}

func (c *CosmosChain) GetValidators(block_height int64) *types.ValidatorSet {
	_, validators := c.valsMgr.getValidators(uint64(block_height), nil)
	if validators == nil {
		log.Warn("Cannot get validator set, number ", strconv.Itoa(int(block_height)))
		return nil
	}

	return validators
}

func (c *CosmosChain) GetLightBlock(block_height int64) *ct.LightBlock {
	h := c.getHeaderByNumber(uint64(block_height))
	if h == nil {
		log.Error("Cannot get block header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	header := c.getSignedHeader(h.Number.Uint64(), h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	log.Debug("getlightblock height ", strconv.Itoa(int(block_height)), h.Hash().Hex())

	// make light block
	vals, validators := c.valsMgr.getValidators(h.Number.Uint64(), nil)
	if validators == nil {
		log.Warn("Cannot get validator set, number ", strconv.Itoa(int(block_height)))
		return nil
	}

	light_block := &ct.LightBlock{SignedHeader: header, ValidatorSet: validators}
	if c.IsLightBlockValid(light_block, vals) {
		return light_block
	} else {
		log.Warn("light block invalid, number ", strconv.Itoa(int(block_height)))
		return nil
	}
}

func (c *CosmosChain) IsLightBlockValid(light_block *ct.LightBlock, vals []common.Address) bool {
	votingPowerNeeded := light_block.ValidatorSet.TotalVotingPower() * 2 / 3
	var talliedVotingPower int64
	for idx, commitSig := range light_block.Commit.Signatures {
		if commitSig.BlockIDFlag != ct.BlockIDFlagCommit {
			continue
		}

		val := light_block.ValidatorSet.Validators[idx]
		// Validate signature.
		voteSignBytes := light_block.Commit.VoteSignBytes(c.config.ChainID.String(), int32(idx))
		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			log.Warn("wrong signature (#%d): %X", idx, commitSig.Signature)
			return false
		}

		talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
	}

	// ？？
	return talliedVotingPower > votingPowerNeeded
}

// TODO voting power check
func (c *CosmosChain) handleVote(vote *et.CosmosVote) error {
	log.Info("handleVote", "number", vote.Number, "headerHash", vote.HeaderHash)
	header := c.getSignedHeader(vote.Number.Uint64(), vote.HeaderHash)
	if header == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, ok := c.vote_cache[vote.HeaderHash.Hex()]; !ok {
			c.vote_cache[vote.HeaderHash.Hex()] = make([]*et.CosmosVote, 1)
			c.vote_cache[vote.HeaderHash.Hex()][0] = vote
		} else {
			c.vote_cache[vote.HeaderHash.Hex()] = append(c.vote_cache[vote.HeaderHash.Hex()], vote)
		}
		log.Error("get signed header failed", "number", vote.Number, "hash", vote.HeaderHash.Hex())
		return errors.New("get signed header failed")
	}
	if len(header.Commit.Signatures) <= int(vote.Index) {
		log.Error("signatures' count is wrong", "origin", len(header.Commit.Signatures), "index", vote.Index)
		return fmt.Errorf("get signed header failed")
	}

	vals, validators := c.valsMgr.getValidators(vote.Number.Uint64(), nil)
	if len(vals) <= int(vote.Index) {
		return fmt.Errorf("invalid address. validators' count is %d, vote index is %d", len(vals), vote.Index)
	}

	// cubeAddr := vals[vote.Index]
	// validator := c.valsMgr.getValidator(cubeAddr)
	// if validator == nil {
	// 	return fmt.Errorf("getValidator failed. cube address is %w", cubeAddr)
	// }

	validator := validators.Validators[vote.Index]

	commit := header.Commit
	commitSig := vote.Vote
	realVote := &ct.Vote{
		Type:             tmproto.PrecommitType,
		Height:           header.Height,
		Round:            commit.Round,
		BlockID:          commit.BlockID,
		Timestamp:        header.Time,
		ValidatorAddress: commitSig.ValidatorAddress,
		ValidatorIndex:   int32(vote.Index),
		Signature:        commitSig.Signature,
	}

	if err := realVote.Verify(c.ChainID, validator.PubKey); err != nil {
		log.Warn("vote fail ", err.Error(), " index ", strconv.Itoa(int(vote.Index)), " hash ", vote.HeaderHash.Hex())
		return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, validator.PubKey, err)
	}
	vote.Vote.Timestamp = realVote.Timestamp
	header.Commit.Signatures[vote.Index] = vote.Vote
	log.Debug("try storeSignedHeader index ", strconv.Itoa(int(vote.Index)), " hash ", vote.HeaderHash.Hex())
	c.storeSignedHeader(vote.HeaderHash, header)

	return nil
}
