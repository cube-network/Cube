package cosmos

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
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

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type CosmosChain struct {
	config       *params.ChainConfig
	ethdb        ethdb.Database
	ChainID      string
	blockContext vm.BlockContext
	mu           sync.Mutex
	// signedHeader map[common.Hash]*ct.SignedHeader // cache only for demo, write/read db instead later
	signedHeader *lru.ARCCache
	//valsMgr     []*ct.Validator // fixed for demo; full validator set, fixed validator set for demo,
	valsMgr *ValidatorsMgr
	//priv_addr_idx  uint32
	privValidator *privval.FilePV // use ed2559 for demo, secp256k1 support later;
	cubeAddr      common.Address

	blockID           ct.BlockID // load best block height later
	best_block_height uint64

	getHeaderByNumber cccommon.GetHeaderByNumberFn
	getHeaderByHash   cccommon.GetHeaderByHashFn
	statefn           cccommon.StateFn
	// vote_cache        map[string][]*et.CosmosVote // TODO clean later，avoid OOM
	vote_cache *lru.ARCCache
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(config *params.ChainConfig, priv_validator_key_file, priv_validator_state_file string, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, ethdb ethdb.Database, blockContext vm.BlockContext, statefn cccommon.StateFn) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	c.config = config
	c.ethdb = ethdb
	// c.ChainID = "ibc-1"
	c.ChainID = config.ChainID.String()
	c.blockContext = blockContext
	c.statefn = statefn
	// c.vote_cache = make(map[string][]*et.CosmosVote)
	c.vote_cache, _ = lru.NewARC(4096)
	// c.signedHeader = make(map[common.Hash]*ct.SignedHeader)
	c.signedHeader, _ = lru.NewARC(4096)
	c.privValidator = privval.LoadOrGenFilePV(priv_validator_key_file, priv_validator_state_file) //privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
	c.privValidator.Save()

	pubkey, _ := c.privValidator.GetPubKey()
	log.Info("init validator", "pubAddr", pubkey.Address().String(), "privAddr", c.privValidator.GetAddress().String())

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

func (c *CosmosChain) generateRegisterValidatorTx(header *et.Header) {
	if len(c.cubeAddr.Bytes()) > 0 {
		chainid := new(big.Int)
		chainid.SetString(c.ChainID, 10)

		p := c.getHeaderByHash(header.ParentHash)
		if p != nil {
			c.valsMgr.registerValidator(c.cubeAddr, c.privValidator, chainid, p)
		}
	}
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

	c.valsMgr.storeValidatorSet(h)

	// TODO 200 check?
	// v := c.valsMgr.getValidator(c.cubeAddr, h)
	// if v == nil {
	// 	c.generateRegisterValidatorTx(h)
	// }

	_, valset := c.valsMgr.getValidators(h.Number.Uint64())
	var valsetHash []byte
	valsetSize := 0
	if valset == nil {
		log.Warn("failed to get validator set")
		//return nil
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

	// make header
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32),
		LastBlockID:        c.blockID, // todo: need to get parent header's hash
		DataHash:           h.TxHash[:],
		ValidatorsHash:     valsetHash,     //valset.Hash(),
		NextValidatorsHash: nextValsetHash, //valset.Hash(),
		ConsensusHash:      make([]byte, 32),
		AppHash:            app_hash[:],
		LastResultsHash:    make([]byte, 32),
		EvidenceHash:       make([]byte, 32),
		ProposerAddress:    addr, // todo: use coinbase's cosmos address
	}

	// TODO
	// c.cube_cosmos_header[h.hash] = header.hash
	// save leveldb

	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]ct.CommitSig, valsetSize)
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
		if vc, ok := c.vote_cache.Get(h.Hash()); ok {
			vote_cache = vc.([]*et.CosmosVote)
			c.vote_cache.Remove(h.Hash())
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
	vals, valset := c.valsMgr.getValidators(uint64(header.Height))
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
	vals, valset := c.valsMgr.getValidators(h.Number.Uint64())
	valsetSize := len(vals)
	var valsetHash []byte
	if valset != nil {
		valsetHash = valset.Hash()
		//return nil, fmt.Errorf("Verify getValidators failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}
	if !bytes.Equal(header.ValidatorsHash, valsetHash) {
		return nil, fmt.Errorf("Verify validatorsHash failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}
	if valsetSize != len(header.Commit.Signatures) {
		return nil, fmt.Errorf("Verify signatures' count failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
	}

	// check proposer
	if valsetSize > 0 {
		proposer := c.valsMgr.getValidator(h.Coinbase, h)
		if proposer == nil {
			return nil, fmt.Errorf("Cannot get proposer. number=%d coinbase=%s hash=%s\n", h.Number.Int64(), h.Coinbase, h.Hash())
		}
		if !bytes.Equal(proposer.Address, header.ProposerAddress) {
			return nil, fmt.Errorf("Verify proposer failed. number=%d hash=%s\n", h.Number.Int64(), h.Hash())
		}

		// check votes
		sigs := header.Commit.Signatures
		if len(sigs) < 1 {
			return nil, fmt.Errorf("Commit signatures are wrong. number=%f hash=%s\n", h.Number, h.Hash())
		}
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

	if c.getSignedHeader(h.Hash()) == nil {
		c.valsMgr.storeValidatorSet(h)
	}

	// store header
	c.storeSignedHeader(h.Hash(), header)

	// vote
	if valsetSize > 0 {
		// todo: should check whether this node is a validator
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

	return nil, nil
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
	// c.mu.Lock()
	// defer c.mu.Unlock()
	if header == nil {
		log.Warn("nil header for hash ", hash.Hex())
		return
	}

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
		log.Info("CosmosVotesAllCollected", "number", header.Height, "hash", hash)
	}

	log.Info("storeSignedHeader", "number", header.Height, "hash", hash, "header", header.Hash())

}

func (c *CosmosChain) getSignedHeader(hash common.Hash) *ct.SignedHeader {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	// h := c.signedHeader[hash]
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
	header := c.getSignedHeader(vote.HeaderHash)
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
	if len(header.Commit.Signatures) <= int(vote.Index) {
		log.Error("signatures' count is wrong", "origin", len(header.Commit.Signatures), "index", vote.Index)
		return fmt.Errorf("get signed header failed")
	}

	oriSig := header.Commit.Signatures[vote.Index]
	if oriSig.BlockIDFlag == ct.BlockIDFlagCommit {
		if bytes.Equal(oriSig.Signature, vote.Vote.Signature) {
			return nil
		}
		return errors.New("already exist signature which is not equal the new one")
	}

	vals, validators := c.valsMgr.getValidators(vote.Number.Uint64())
	if len(vals) <= int(vote.Index) {
		return fmt.Errorf("invalid address. validators' count is %d, vote index is %d", len(vals), vote.Index)
	}

	// cubeAddr := vals[vote.Index]
	// validator := c.valsMgr.getValidator(cubeAddr)
	// if validator == nil {
	// 	return fmt.Errorf("getValidator failed. cube address is %w", cubeAddr)
	// }

	validator := validators.Validators[vote.Index]
	if validator == nil {
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
		return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, validator.PubKey, err)
	}
	vote.Vote.Timestamp = realVote.Timestamp
	header.Commit.Signatures[vote.Index] = vote.Vote
	log.Debug("try storeSignedHeader", "index", strconv.Itoa(int(vote.Index)), "number", vote.Number, "hash", vote.HeaderHash.Hex())
	c.storeSignedHeader(vote.HeaderHash, header)

	return nil
}

func (c *CosmosChain) checkVotes(height uint64, hash common.Hash, h *et.Header) *et.CosmosLackedVoteIndexs { //(*et.CosmosVotesList, *et.CosmosLackedVoteIndexs) {
	sh := c.getSignedHeader(hash)
	if sh == nil {
		log.Info("checkVotes empty", "number", height, "hash", hash)
		return nil
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
			log.Warn("wrong signature (#%d): %X", idx, commitSig.Signature)
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
	sh := c.getSignedHeader(votes.Hash)
	if sh == nil {
		return errors.New("cannot get signedheader")
	}

	for _, v := range votes.Commits {
		nv := &et.CosmosVote{
			Number:     votes.Number,
			HeaderHash: votes.Hash,
			Index:      uint32(v.Index.Int64()),
			Vote:       v.Vote,
		}
		log.Info("handleVotesList", "number", votes.Number, "index", v.Index.Int64(), "hash", votes.Hash)
		if err := c.handleVote(nv); err != nil {
			return err
		}
	}

	return nil
}
