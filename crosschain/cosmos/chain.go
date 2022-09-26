package cosmos

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/tempfile"
	"math/big"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/types"
	ct "github.com/tendermint/tendermint/types"
)

type CosmosChain struct {
	config        *params.ChainConfig
	ethdb         ethdb.Database
	ChainID       string
	blockContext  vm.BlockContext
	mu            sync.Mutex
	signedHeader  *lru.ARCCache
	valsMgr       *ValidatorsMgr
	privValidator *privval.FilePV // use ed2559 for demo, secp256k1 support later;
	cubeAddr      common.Address

	blockID           ct.BlockID // load best block height later
	best_block_height uint64

	getHeaderByNumber cccommon.GetHeaderByNumberFn
	getHeaderByHash   cccommon.GetHeaderByHashFn
	statefn           cccommon.StateFn
	vote_cache        *lru.ARCCache
	sigs_cache        *lru.ARCCache
	handledVotesCache *lru.ARCCache
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(config *params.ChainConfig, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, ethdb ethdb.Database, blockContext vm.BlockContext, statefn cccommon.StateFn) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	c.config = config
	c.ethdb = ethdb
	c.ChainID = config.ChainID.String()
	c.blockContext = blockContext
	c.statefn = statefn
	c.vote_cache, _ = lru.NewARC(1024)
	c.sigs_cache, _ = lru.NewARC(1024)
	c.handledVotesCache, _ = lru.NewARC(1024)
	c.signedHeader, _ = lru.NewARC(1024)

	c.initPrivValAndState(priv_validator_key_file, priv_validator_state_file)

	c.getHeaderByNumber = headerfn
	c.getHeaderByHash = headerhashfn

	// TODO load validator set, should use contract to deal with validators getting changed in the future
	c.valsMgr = NewValidatorsMgr(ethdb, blockContext, config, c.privValidator, headerfn, headerhashfn, statefn)

	// TODO load best block
	psh := ct.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = ct.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

func (c *CosmosChain) initPrivValAndState(priv_validator_key_file, priv_validator_state_file string) {
	_, err := os.Stat(priv_validator_state_file)
	if err != nil && !os.IsExist(err) {
		fp, err := os.Create(priv_validator_state_file)
		if err != nil {
			panic(err)
		}
		fp.Close()

		initState := privval.FilePVLastSignState{}
		jsonBytes, err := tmjson.MarshalIndent(&initState, "", "  ")
		if err != nil {
			panic(err)
		}
		err = tempfile.WriteFileAtomic(priv_validator_state_file, jsonBytes, 0600)
		if err != nil {
			panic(err)
		}
	}
	c.privValidator = privval.LoadOrGenFilePV(priv_validator_key_file, priv_validator_state_file)
	c.privValidator.Save()

	pubkey, _ := c.privValidator.GetPubKey()
	log.Info("init validator", "pubAddr", pubkey.Address().String(), "privAddr", c.privValidator.GetAddress().String())
}

func (c *CosmosChain) makeCosmosSignedHeader(h *et.Header) (*ct.SignedHeader, *et.CosmosVote) {
	log.Info("makeCosmosSignedHeader", "height", strconv.Itoa(int(h.Number.Int64())), "hash", h.Hash())

	c.valsMgr.storeValidatorSet(h)

	var app_hash common.Hash
	app_hash.SetBytes(h.Extra[32:64])

	val := c.valsMgr.getValidator(h.Coinbase, h)
	if val == nil {
		log.Warn("makeCosmosSignedHeader getValidator is nil")
		return nil, nil //, -1, ct.CommitSig{}
	}

	addr := val.Address

	_, valset := c.valsMgr.getValidators(h.Number.Uint64())
	var valsetHash []byte
	valsetSize := 0
	if valset == nil {
		log.Warn("failed to get validator set")
	} else {
		valsetHash = valset.Hash()
		valsetSize = valset.Size()
	}

	_, nextValset := c.valsMgr.getNextValidators(h.Number.Uint64())
	var nextValsetHash []byte
	if nextValset == nil {
		log.Warn("failed to get next validator set")
		//return nil
	} else {
		nextValsetHash = nextValset.Hash()
	}

	log.Debug("cosmos", "height", strconv.Itoa(int(h.Number.Int64())), "validator hash ", hex.EncodeToString(valsetHash), " next ", hex.EncodeToString(nextValsetHash))

	parentHeader := c.getSignedHeader(h.ParentHash)
	var lastBlockID types.BlockID
	if parentHeader != nil {
		lastpsh := ct.PartSetHeader{Total: 1, Hash: parentHeader.Hash()}
		lastBlockID = ct.BlockID{Hash: parentHeader.Hash(), PartSetHeader: lastpsh}
	}

	// make header
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32),
		LastBlockID:        lastBlockID, //c.blockID,
		DataHash:           h.TxHash[:],
		ValidatorsHash:     valsetHash,     //valset.Hash(),
		NextValidatorsHash: nextValsetHash, //valset.Hash(),
		ConsensusHash:      make([]byte, 32),
		AppHash:            app_hash[:],
		LastResultsHash:    make([]byte, 32),
		EvidenceHash:       make([]byte, 32),
		ProposerAddress:    addr, // use coinbase's cosmos address
	}

	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}

	signatures := make([]ct.CommitSig, valsetSize)
	for i := 0; i < len(signatures); i++ {
		signatures[i].BlockIDFlag = ct.BlockIDFlagAbsent
	}

	commit := &ct.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	signedHeader := &ct.SignedHeader{Header: header, Commit: commit}

	var cv *et.CosmosVote = nil
	index, vote, _ := c.voteSignedHeader(signedHeader, valset)
	if index >= 0 {
		cv = &et.CosmosVote{
			Number:     h.Number,
			HeaderHash: h.Hash(),
			Index:      uint32(index),
			Vote:       vote,
		}
	}

	var vote_cache []*et.CosmosVote = nil
	{
		c.mu.Lock()
		if vc, ok := c.vote_cache.Get(h.Hash()); ok {
			vote_cache = vc.([]*et.CosmosVote)
			c.vote_cache.Remove(h.Hash())
		}
		c.mu.Unlock()
	}

	if vote_cache != nil {
		for i := 0; i < len(vote_cache); i++ {
			c.handleVote(vote_cache[i], signedHeader)
		}
	}

	var sigs_cache []ct.CommitSig
	{
		c.mu.Lock()
		if s, ok := c.sigs_cache.Get(h.Hash()); ok {
			sigs_cache = s.([]ct.CommitSig)
			c.sigs_cache.Remove(h.Hash())
		}
		c.mu.Unlock()
	}
	if len(sigs_cache) > 0 {
		c.handleSignatures(h, sigs_cache, signedHeader)
	}

	// store header
	c.storeSignedHeader(h.Hash(), signedHeader)

	return signedHeader, cv //, index, vote
}

func (c *CosmosChain) getValidatorIndex(vals []common.Address) int {
	for i, addr := range vals {
		if bytes.Equal(addr.Bytes(), c.cubeAddr.Bytes()) {
			return i
		}
	}

	return -1
}

func (c *CosmosChain) voteSignedHeader(header *ct.SignedHeader, valset *ct.ValidatorSet) (int, ct.CommitSig, error) {
	if header == nil || header.Commit == nil {
		log.Error("voteSignedHeader unknown data")
		return -1, ct.CommitSig{}, errors.New("voteSignedHeader unknown data")
	}

	if valset == nil {
		//log.Error("getValidators fail")
		return -1, ct.CommitSig{}, errors.New("getValidatorIndex failed")
	}

	pubkey, _ := c.privValidator.GetPubKey()
	addr := pubkey.Address()
	idx, _ := valset.GetByAddress(addr)
	if idx < 0 {
		log.Error("getValidatorIndex failed", "cubeAddr", c.cubeAddr, "cosmosAddr", addr)
		for i := 0; i < valset.Size(); i++ {
			log.Error("getValidatorIndex valset", "index", i, "addr", valset.Validators[i].Address.String())
		}
		return -1, ct.CommitSig{}, errors.New("getValidatorIndex failed")
	}
	if int(idx) >= len(header.Commit.Signatures) {
		// todo:
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

func (c *CosmosChain) getSignatures(hash common.Hash) []ct.CommitSig {
	sh := c.getSignedHeader(hash)
	if sh == nil {
		return []ct.CommitSig{}
	}
	return sh.Commit.Signatures
}

func (c *CosmosChain) handleSignaturesFromBroadcast(h *et.Header, sigs []ct.CommitSig) error {
	header := c.getSignedHeader(h.Hash())
	err := c.handleSignatures(h, sigs, header)
	if err != nil {
		c.storeSignedHeader(h.Hash(), header)
	}
	return err
}

func (c *CosmosChain) handleSignatures(h *et.Header, sigs []ct.CommitSig, header *types.SignedHeader) error { //(*et.CosmosVote, error) {
	log.Info("handleSignatures", "height", h.Number, "hash", h.Hash())

	cacheSigsFn := func() {
		c.mu.Lock()
		if !c.sigs_cache.Contains(h.Hash()) {
			for _, sig := range sigs {
				if sig.BlockIDFlag != ct.BlockIDFlagCommit {
					continue
				}
				if err := sig.ValidateBasic(); err != nil {
					return
				}
			}
			c.sigs_cache.Add(h.Hash(), sigs)
		} else {
			if oldsigsI, ok := c.sigs_cache.Get(h.Hash()); ok {
				oldsigs := oldsigsI.([]ct.CommitSig)
				newsigs := make([]ct.CommitSig, 0)
				// todo: need more validation
				for i := 0; i < len(oldsigs) || i < len(sigs); i++ {
					if i < len(oldsigs) && oldsigs[i].BlockIDFlag == ct.BlockIDFlagCommit {
						newsigs = append(newsigs, oldsigs[i])
					} else if i < len(sigs) && sigs[i].BlockIDFlag == ct.BlockIDFlagCommit {
						if err := sigs[i].ValidateBasic(); err != nil {
							return
						}
						newsigs = append(newsigs, sigs[i])
					}
				}
				c.sigs_cache.Add(h.Hash(), newsigs)
			} else {
				c.sigs_cache.Add(h.Hash(), sigs)
			}
		}
		c.mu.Unlock()
	}

	_, valset := c.valsMgr.getValidators(h.Number.Uint64())
	if valset == nil || header == nil {
		cacheSigsFn()
		return nil
	}

	// check signatures
	commit := header.Commit
	for i, sig := range sigs {
		if sig.BlockIDFlag != ct.BlockIDFlagCommit {
			continue
		}
		if err := sig.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid commit: %w", err)
		}

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
			return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, val.PubKey, err)
		}
		sig.Timestamp = header.Time
		commit.Signatures[i] = sig
		log.Debug("UpdateVote", "index", i)
	}

	//// store header
	//if updated && fromBroadcast {
	//	c.storeSignedHeader(h.Hash(), header)
	//}
	return nil
}

func makeSignedHeaderKey(hash common.Hash) []byte {
	key := "crosschain_cosmos_header_"
	key += hash.Hex()
	return []byte(key)
}

func makeValidatorKey(hash common.Hash) []byte {
	key := "crosschain_cosmos_validator_"
	key += hash.Hex()
	return []byte(key)
}

func (c *CosmosChain) storeSignedHeader(hash common.Hash, header *ct.SignedHeader) {
	if header == nil {
		log.Warn("nil header for hash ", hash.Hex())
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ph := header.ToProto()
	bz, _ := ph.Marshal()
	err := c.ethdb.Put(makeSignedHeaderKey(hash)[:], bz)
	if err != nil {
		log.Error("storeSignedHeader db put fail ", hash.Hex())
	}

	// c.signedHeader[hash] = header
	c.signedHeader.Add(hash, header)

	counter := 0
	for _, commitSig := range header.Commit.Signatures {
		if commitSig.BlockIDFlag == ct.BlockIDFlagCommit {
			counter++
		}
	}
	if counter == len(header.Commit.Signatures) {
		log.Debug("CosmosVotesAllCollected", "number", header.Height, "hash", hash)
	}

	log.Info("storeSignedHeader", "vote", strconv.Itoa(counter), "number", strconv.Itoa(int(header.Height)), "hash", hash, "header", header.Hash(), "validatorHash", hex.EncodeToString(header.ValidatorsHash), "nextValHash", hex.EncodeToString(header.NextValidatorsHash))
}

func (c *CosmosChain) getSignedHeader(hash common.Hash) *ct.SignedHeader {
	c.mu.Lock()
	defer c.mu.Unlock()

	h, ok := c.signedHeader.Get(hash)

	if !ok {
		bz, err := c.ethdb.Get(makeSignedHeaderKey(hash)[:])
		if err == nil {
			tsh := &tmproto.SignedHeader{}
			// TODO handler unmarshal error
			tsh.Unmarshal(bz)
			sh, _ := types.SignedHeaderFromProto(tsh)
			c.signedHeader.Add(hash, sh)
			return sh
		} else {
			return nil
		}
	} else {
		return h.(*ct.SignedHeader)
	}
}

func (c *CosmosChain) getHeader(block_height int64) *ct.Header {
	// c.mu.Lock()
	// defer c.mu.Unlock()
	h := c.getHeaderByNumber(uint64(block_height))
	if h == nil {
		log.Error("Cannot get block header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	header := c.getSignedHeader(h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	log.Debug("getlightblock height ", strconv.Itoa(int(block_height)), h.Hash().Hex())
	return header.Header
}

func (c *CosmosChain) GetValidators(block_height int64) *types.ValidatorSet {
	_, validators := c.valsMgr.getValidators(uint64(block_height))
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
	header := c.getSignedHeader(h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", strconv.Itoa(int(block_height)))
		return nil
	}
	log.Debug("getlightblock height ", strconv.Itoa(int(block_height)), h.Hash().Hex())

	// make light block
	vals, validators := c.valsMgr.getValidators(h.Number.Uint64())
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
			log.Warn("IsLightBlockValid wrong signature (#%d): %X", idx, commitSig.Signature)
			return false
		}

		talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
	}

	// ？？
	return talliedVotingPower > votingPowerNeeded
}

func (c *CosmosChain) handleVoteFromBroadcast(vote *et.CosmosVote) error {
	if _, ok := c.handledVotesCache.Get(vote.Hash()); ok {
		return et.ErrHandledVote
	}
	c.handledVotesCache.Add(vote.Hash(), true)

	header := c.getSignedHeader(vote.HeaderHash)
	err := c.handleVote(vote, header)
	if header != nil && err == nil {
		c.storeSignedHeader(vote.HeaderHash, header)
	}
	return err
}

// TODO voting power check
func (c *CosmosChain) handleVote(vote *et.CosmosVote, header *types.SignedHeader) error {
	log.Info("handleVote", "number", vote.Number, "headerHash", vote.HeaderHash)
	if header == nil {
		header = c.getSignedHeader(vote.HeaderHash)
		if header == nil {
			c.mu.Lock()
			if !c.vote_cache.Contains(vote.HeaderHash) {
				vc := make([]*et.CosmosVote, 1)
				vc[0] = vote
				c.vote_cache.Add(vote.HeaderHash, vc)
			} else {
				vci, _ := c.vote_cache.Get(vote.HeaderHash)
				vc := vci.([]*et.CosmosVote)
				vc = append(vc, vote)
				c.vote_cache.Add(vote.HeaderHash, vc)
			}
			c.mu.Unlock()

			log.Error("get signed header failed, cache vote, ", "number", vote.Number, "hash", vote.HeaderHash.Hex())
			return nil //errors.New("get signed header failed")
		}
	}

	if len(header.Commit.Signatures) <= int(vote.Index) {
		log.Error("signatures' count is wrong", "origin", len(header.Commit.Signatures), "index", vote.Index)
		log.Error("P2P ERROR!!!!", "func", "handleVote")
		return fmt.Errorf("get signed header failed")
	}

	oriSig := header.Commit.Signatures[vote.Index]
	if oriSig.BlockIDFlag == ct.BlockIDFlagCommit {
		if bytes.Equal(oriSig.Signature, vote.Vote.Signature) {
			return nil
		}
		log.Error("P2P ERROR!!!!", "func", "handleVote")
		return errors.New("already exist signature which is not equal the new one")
	}

	vals, validators := c.valsMgr.getValidators(vote.Number.Uint64())
	if len(vals) <= int(vote.Index) {
		log.Error("P2P ERROR!!!!", "func", "handleVote")
		return fmt.Errorf("invalid address. validators' count is %d, vote index is %d", len(vals), vote.Index)
	}

	// cubeAddr := vals[vote.Index]
	// validator := c.valsMgr.getValidator(cubeAddr)
	// if validator == nil {
	// 	return fmt.Errorf("getValidator failed. cube address is %w", cubeAddr)
	// }

	validator := validators.Validators[vote.Index]
	if validator == nil {
		log.Error("P2P ERROR!!!!", "func", "handleVote")
		return fmt.Errorf("unregister validator. validators' count is %d, vote index is %d", len(vals), vote.Index)
	}

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
		log.Warn("vote fail", "err", err.Error(), " index ", strconv.Itoa(int(vote.Index)), " hash ", vote.HeaderHash.Hex())
		log.Error("P2P ERROR!!!!", "func", "handleVote")
		return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, validator.PubKey, err)
	}
	vote.Vote.Timestamp = realVote.Timestamp
	header.Commit.Signatures[vote.Index] = vote.Vote

	//log.Debug("try storeSignedHeader", "index", strconv.Itoa(int(vote.Index)), "number", vote.Number, "hash", vote.HeaderHash.Hex())
	//c.storeSignedHeader(vote.HeaderHash, header)

	return nil
}

func (c *CosmosChain) checkVotes(height uint64, hash common.Hash) *et.CosmosLackedVoteIndexs { //(*et.CosmosVotesList, *et.CosmosLackedVoteIndexs) {
	sh := c.getSignedHeader(hash)
	if sh == nil {
		log.Info("checkVotes empty", "number", height, "hash", hash)
		return nil
		//sh, _, _ = c.makeCosmosSignedHeader(h)
	}

	// check votes
	_, valset := c.valsMgr.getValidators(height)
	lackIdx := make([]*big.Int, 0)
	//votes := make([]et.CosmosVoteCommit, 0)
	for idx, commitSig := range sh.Commit.Signatures {
		if commitSig.BlockIDFlag != ct.BlockIDFlagCommit {
			lackIdx = append(lackIdx, big.NewInt(int64(idx)))
			log.Info("checkVotes", "number", height, "lackIndex", idx, "hash", hash)
			continue
		}

		val := valset.Validators[idx]
		// Validate signature.
		voteSignBytes := sh.Commit.VoteSignBytes(c.config.ChainID.String(), int32(idx))
		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			log.Warn("checkVotes wrong signature (#%d): %X", idx, commitSig.Signature)
			// todo: remove signature
			lackIdx = append(lackIdx, big.NewInt(int64(idx)))
			continue
		}
		//commit := et.CosmosVoteCommit{
		//	Index: uint32(idx),
		//	Vote:  commitSig,
		//}
		//votes = append(votes, commit)
	}
	if len(lackIdx) > 0 {
		lackVotes := &et.CosmosLackedVoteIndexs{
			Number: big.NewInt(int64(height)),
			Hash:   hash,
			Indexs: lackIdx,
		}
		return lackVotes
		//} else {
		//	commit := &et.CosmosVotesList{
		//		Number:  big.NewInt(int64(height)),
		//		Hash:    hash,
		//		Commits: votes,
		//	}
		//	return commit, nil
	}
	return nil
}

func (c *CosmosChain) handleVotesQuery(idxs *et.CosmosLackedVoteIndexs) (*et.CosmosVotesList, error) {
	sh := c.getSignedHeader(idxs.Hash)
	if sh == nil {
		return nil, nil //errors.New("cannot get signedheader")
	}
	if idxs.Indexs == nil || len(idxs.Indexs) == 0 {
		log.Error("P2P ERROR!!!!", "func", "handleVotesQuery")
		return nil, errors.New("indexs is empty")
	}

	votes := make([]et.CosmosVoteCommit, 0)
	commit := sh.Commit.Signatures
	for _, idx := range idxs.Indexs {
		if len(commit) <= int(idx.Int64()) {
			return nil, errors.New("signatures' count is wrong")
		}
		sig := commit[idx.Int64()]
		if sig.BlockIDFlag == ct.BlockIDFlagCommit {
			v := et.CosmosVoteCommit{
				Index: idx,
				Vote:  sig,
			}
			votes = append(votes, v)
		}
	}
	if len(votes) == 0 {
		return nil, nil
	}

	vl := &et.CosmosVotesList{
		Number:  idxs.Number,
		Hash:    idxs.Hash,
		Commits: votes,
	}
	return vl, nil
}

func (c *CosmosChain) handleVotesList(votes *et.CosmosVotesList) error {
	if len(votes.Commits) == 0 {
		return nil
	}
	header := c.getSignedHeader(votes.Hash)
	var err error
	for _, v := range votes.Commits {
		nv := &et.CosmosVote{
			Number:     votes.Number,
			HeaderHash: votes.Hash,
			Index:      uint32(v.Index.Int64()),
			Vote:       v.Vote,
		}
		log.Info("handleVotesList", "number", votes.Number, "index", v.Index.Int64(), "hash", votes.Hash)
		if err = c.handleVote(nv, header); err != nil {
			log.Error("P2P ERROR!!!!", "func", "handleVotesList")
			return err
		}
	}
	if header != nil {
		log.Debug("try storeSignedHeader", "number", votes.Number, "hash", votes.Hash.Hex())
		c.storeSignedHeader(votes.Hash, header)
	}

	return nil
}
