package cosmos

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"

	"container/list"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	lru "github.com/hashicorp/golang-lru"
	ct "github.com/tendermint/tendermint/types"
)

type Cosmos struct {
	datadir string
	ethdb   ethdb.Database
	sdb     state.Database
	config  *params.ChainConfig
	header  *types.Header

	coinbase common.Address

	codec           EncodingConfig
	blockContext    vm.BlockContext
	statefn         cccommon.StateFn
	headerfn        cccommon.GetHeaderByNumberFn
	headerhashfn    cccommon.GetHeaderByHashFn
	finalizeblockfn cccommon.GetLastFinalizedBlockNumberFn

	callmu         sync.Mutex
	chainmu        sync.Mutex
	querymu        sync.Mutex
	queryExecutors *lru.ARCCache

	chain *CosmosChain
}

var once sync.Once

func (c *Cosmos) Init(datadir string,
	ethdb ethdb.Database,
	sdb state.Database,
	config *params.ChainConfig,
	blockContext vm.BlockContext,
	statefn cccommon.StateFn,
	headerfn cccommon.GetHeaderByNumberFn,
	headerhashfn cccommon.GetHeaderByHashFn,
	finalizeblockfn cccommon.GetLastFinalizedBlockNumberFn,
	header *types.Header) {

	// c.callmu.Lock()
	// defer c.callmu.Unlock()

	once.Do(func() {
		c.datadir = datadir
		c.ethdb = ethdb
		c.sdb = sdb
		c.config = config
		c.blockContext = blockContext
		c.statefn = statefn
		c.headerfn = headerfn
		c.headerhashfn = headerhashfn
		c.finalizeblockfn = finalizeblockfn
		c.header = header

		c.codec = MakeEncodingConfig()

		c.queryExecutors, _ = lru.NewARC(128)
		c.chain = MakeCosmosChain(config, datadir+"/priv_validator_key.json", datadir+"/priv_validator_state.json", headerfn, headerhashfn, ethdb, blockContext, statefn)
		qe, _ := c.makeQueryExecutorByHeader(header, ExecutorModeQuery)
		c.freeExecutorWithoutLock(qe)
	})
}

func (c *Cosmos) SetCoinbase(addr common.Address) {
	c.coinbase = addr
	c.chain.cubeAddr = addr
}

func (c *Cosmos) APIs() []rpc.API {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	return APIs(c)
}

func IsEnable(config *params.ChainConfig, block_height *big.Int) bool {
	return config.IsCrosschainCosmos(block_height)
}

func (c *Cosmos) NewExecutor(header *types.Header, statedb *state.StateDB, checkMode bool) vm.CrossChain {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	if !IsEnable(c.config, header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(header.Number.Uint64(), 10))
		return nil
	}

	if checkMode {
		c.querymu.Lock()
		defer c.querymu.Unlock()
		executor, _ := c.getQueryExecutorByHeaderAndStatedb(header, statedb, ExecutorModeCheck)
		log.Debug("get query  block height ", header.Number.String(), " Executor  ", fmt.Sprintf("%p", executor))
		return executor
	} else {
		executor, _ := c.makeExecutor(header, statedb, ExecutorModeExec)
		log.Debug("get exec ", " block height ", header.Number.String(), " Executor ", fmt.Sprintf("%p", executor))

		return executor
	}
}

func (c *Cosmos) FreeExecutor(exec vm.CrossChain) {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	c.querymu.Lock()
	defer c.querymu.Unlock()

	c.freeExecutorWithoutLock(exec)
}

func (c *Cosmos) freeExecutorWithoutLock(exec vm.CrossChain) {
	if exec == nil {
		return
	}
	executor := exec.(*Executor)
	if executor == nil || !IsEnable(c.config, executor.header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(executor.header.Number.Uint64(), 10))
		return
	}

	if executor.mode == ExecutorModeQuery {
		executors, ok := c.queryExecutors.Get(executor.header.Number.Int64())
		if !ok {
			executors = list.New()
		}
		executors.(*list.List).PushBack(executor)
		c.queryExecutors.Add(executor.header.Number.Int64(), executors)
		log.Debug("freeExecutor reuse ", " block height ", executor.header.Number.String(), fmt.Sprintf("%p", executor), " return key ", executor.header.Root.Hex(), " val ", fmt.Sprintf("%p", executor.app))
	} else {
		log.Debug("freeExecutor ", " block height ", executor.header.Number.String(), fmt.Sprintf("%p", executor), " return key ", executor.header.Root.Hex(), " val ", fmt.Sprintf("%p", executor.app))
	}

	// log.Debug("freeExecutor ", fmt.Sprintf("%p", executor), " return key ", executor.header.Root.Hex(), " val ", fmt.Sprintf("%p", executor.app))
}

func (c *Cosmos) Seal(exec vm.CrossChain) {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	if exec == nil {
		return
	}
	log.Debug("seal exec", "executor", exec)
	executor := exec.(*Executor)
	if executor == nil || !IsEnable(c.config, executor.header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(executor.header.Number.Uint64(), 10))
		return
	}

	executor.EndBlock()
}

func (c *Cosmos) makeExecutor(header *types.Header, statedb *state.StateDB, mode ExecutorMode) (*Executor, bool) {
	executor := NewCosmosExecutor(c.datadir, c.config, c.codec, c.chain.getHeader, c.blockContext, statedb, c.header, c.coinbase, c.chain, mode)
	executor.BeginBlock(header, statedb)

	log.Debug("newExecutor ", fmt.Sprintf("%p", executor), " block height ", header.Number.String())
	return executor, true
}

func (c *Cosmos) makeQueryExecutorByHeader(header *types.Header, mode ExecutorMode) (*Executor, bool) {
	var statedb *state.StateDB
	var err error
	if c.statefn != nil {
		statedb, err = c.statefn(header.Root)
	} else {
		statedb, err = state.New(header.Root, c.sdb, nil)
	}
	if err != nil {
		log.Warn("cosmos event header state root not found, ", err.Error())
		return nil, false
	}

	return c.makeExecutor(header, statedb, mode)
}

func (c *Cosmos) getQueryExecutorByHeaderAndStatedb(header *types.Header, statedb *state.StateDB, mode ExecutorMode) (*Executor, bool) {
	v, ok := c.queryExecutors.Get(header.Number.Int64())
	if ok {
		executors := v.(*list.List)
		for it := executors.Front(); it != nil; it = it.Next() {
			executor := it.Value.(*Executor)
			if executor.header.Hash() == header.Hash() {
				executors.Remove(it)
				if executors.Len() != 0 {
					c.queryExecutors.Add(header.Number.Int64(), executors)
				} else {
					c.queryExecutors.Remove(header.Number.Int64())
				}
				log.Debug("remvoe query executor ", header.Number.String())
				return executor, true
			}
		}

	}
	// make new executor
	return c.makeExecutor(header, statedb, mode)
}

func (c *Cosmos) getQueryExecutorByHeader(header *types.Header, mode ExecutorMode) (*Executor, bool) {
	var statedb *state.StateDB
	var err error
	if c.statefn != nil {
		statedb, err = c.statefn(header.Root)
	} else {
		statedb, err = state.New(header.Root, c.sdb, nil)
	}
	if err != nil {
		log.Warn("cosmos event header state root not found, ", err.Error())
		return nil, false
	}
	return c.getQueryExecutorByHeaderAndStatedb(header, statedb, mode)
}

func (c *Cosmos) getQueryExecutorByHeight(height int64, mode ExecutorMode) (*Executor, bool) {
	header := c.headerfn(uint64(height))
	return c.getQueryExecutorByHeader(header, mode)
}

func (c *Cosmos) getQueryExecutor(height int64, mode ExecutorMode) (*Executor, bool) {
	var fh int64 = 0
	if c.finalizeblockfn != nil {
		fh = int64(c.finalizeblockfn())
	} else {
		return nil, false
	}

	if !IsEnable(c.config, big.NewInt(fh)) {
		return nil, false
	}

	if height < 0 || height > fh {
		return nil, false
	}

	if height == 0 {
		height = fh
	}

	executor, _ := c.getQueryExecutorByHeight(height, mode)
	log.Debug("get executor height ", executor.header.Number.String(), " hash ", executor.header.Hash().Hex())
	return executor, true
}

func (c *Cosmos) EventHeader(header *types.Header) *types.CosmosVote {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	if !IsEnable(c.config, big.NewInt(header.Number.Int64())) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(header.Number.Uint64()-1, 10))
		return nil
	}

	log.Info("event header", "number", header.Number.Int64(), " hash ", header.Hash().Hex(), " root ", header.Root.Hex(), " coinbase ", header.Coinbase.Hex(), " diffculty ", header.Difficulty.Int64())

	csh, vote := c.chain.makeCosmosSignedHeader(header)
	if csh == nil {
		log.Warn("make cosmos signed header fail!")
		return nil
	}

	// c.chain.LogCosmosVote("EventHeader", vote)
	return vote
}

func (c *Cosmos) GetSignedHeader(height uint64, hash common.Hash) *ct.SignedHeader {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	if !IsEnable(c.config, big.NewInt(int64(height))) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(height, 10))
		return nil
	}
	return c.chain.getSignedHeader(hash)
}

func (c *Cosmos) GetSignatures(hash common.Hash) []ct.CommitSig {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	return c.chain.getSignatures(hash)
}

func (c *Cosmos) HandleSignatures(h *types.Header, sigs []ct.CommitSig) error { //(*types.CosmosVote, error) {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	return c.chain.handleSignaturesFromBroadcast(h, sigs)
}

func (c *Cosmos) HandleVote(vote *et.CosmosVote) error {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	// c.chain.LogCosmosVote("HandleVote", vote)
	return c.chain.handleVoteFromBroadcast(vote)
}

func (c *Cosmos) CheckVotes(height uint64, hash common.Hash) *types.CosmosLackedVoteIndexs {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	return c.chain.checkVotes(height, hash)
}

func (c *Cosmos) HandleVotesQuery(idxs *types.CosmosLackedVoteIndexs) (*types.CosmosVotesList, error) {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	votes, err := c.chain.handleVotesQuery(idxs)

	// if err != nil {
	// 	c.chain.LogCosmosVotesList("HandleVotesQuery", votes)
	// }

	return votes, err
}

func (c *Cosmos) HandleVotesList(votes *types.CosmosVotesList) error {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	// if votes != nil {
	// 	c.chain.LogCosmosVotesList("HandleVotesList", votes)
	// }
	return c.chain.handleVotesList(votes)
}

func (c *Cosmos) CosmosValidators(height *int64, page, perPage *int) ([]byte, error) {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	vals := c.chain.GetValidators(*height)
	if vals == nil {
		return nil, errors.New("invalid validators")
	}

	vt, _ := vals.ToProto()
	return vt.Marshal()
}

func (c *Cosmos) CosmosLightBlock(height *int64) ([]byte, error) {
	c.chainmu.Lock()
	defer c.chainmu.Unlock()

	if c.finalizeblockfn != nil {
		fh := int64(c.finalizeblockfn())
		if *height > fh {
			return nil, errors.New("invalid height")
		}
	}

	lb := c.chain.GetLightBlock(*height)
	if lb != nil {
		// log.Debug("get cosmos ligth block ", strconv.FormatInt(lb.Height, 10), " apphash ", lb.Header.AppHash.String())
		tlb, _ := lb.ToProto()
		return tlb.Marshal()
	} else {
		return nil, errors.New("invalid height")
	}
}
