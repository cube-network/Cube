package cosmos

import (
	"container/list"
	"errors"
	"math/big"
	"strconv"
	"sync"

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
	ct "github.com/tendermint/tendermint/types"
)

type Cosmos struct {
	datadir string
	ethdb   ethdb.Database
	sdb     state.Database
	config  *params.ChainConfig
	header  *types.Header

	coinbase common.Address

	codec        EncodingConfig
	blockContext vm.BlockContext
	statefn      cccommon.StateFn
	headerfn     cccommon.GetHeaderByNumberFn

	querymu       sync.Mutex
	queryExecutor *Executor
	callmu        sync.Mutex
	callExectors  *list.List

	chain *CosmosChain

	newExecutorCounter  int64
	freeExecutorCounter int64
}

var once sync.Once

func (c *Cosmos) Init(datadir string,
	ethdb ethdb.Database,
	sdb state.Database,
	config *params.ChainConfig,
	blockContext vm.BlockContext,
	statefn cccommon.StateFn,
	headerfn cccommon.GetHeaderByNumberFn,
	header *types.Header) {

	c.callmu.Lock()
	defer c.callmu.Unlock()

	once.Do(func() {
		c.datadir = datadir
		c.ethdb = ethdb
		c.sdb = sdb
		c.config = config
		c.blockContext = blockContext
		c.statefn = statefn
		c.headerfn = headerfn
		c.header = header

		c.codec = MakeEncodingConfig()

		c.callExectors = list.New()

		statedb, err := state.New(header.Root, c.sdb, nil)
		if err != nil {
			panic("cosmos init state root not found")
		}
		c.chain = MakeCosmosChain(config, datadir+"/priv_validator_key.json", datadir+"/priv_validator_state.json", headerfn)
		c.queryExecutor = NewCosmosExecutor(c.datadir, c.config, c.codec, c.chain.getHeader, c.blockContext, statedb, c.header, common.Address{}, nil, true)
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

func (c *Cosmos) NewExecutor(header *types.Header, statedb *state.StateDB) vm.CrossChain {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	if !IsEnable(c.config, header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(header.Number.Uint64(), 10))
		return nil
	}

	var exector *Executor
	// if c.callExectors.Len() > 0 {
	// 	// TODO max list len
	// 	elem := c.callExectors.Front()
	// 	exector = elem.Value.(*Executor)
	// 	c.callExectors.Remove(elem)
	// } else {
	exector = NewCosmosExecutor(c.datadir, c.config, c.codec, c.chain.getHeader, c.blockContext, statedb, header, c.coinbase, c.chain, false)
	// }

	c.newExecutorCounter++
	log.Debug("new exec", "exector", exector)
	log.Debug("newExecutorCounter ", "counter", c.newExecutorCounter, " block height ", header.Number.Int64())

	exector.BeginBlock(header, statedb)

	return exector
}

func (c *Cosmos) FreeExecutor(exec vm.CrossChain) {
	c.callmu.Lock()
	defer c.callmu.Unlock()
	if exec == nil {
		return
	}
	executor := exec.(*Executor)
	if executor == nil || !IsEnable(c.config, executor.header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(executor.header.Number.Uint64(), 10))
		return
	}

	// c.callExectors.PushFront(exec)
	c.freeExecutorCounter++
	log.Debug("freeExecutorCounter", "counter", c.freeExecutorCounter)
	log.Debug("free exec", "executor", exec)
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

func (c *Cosmos) EventHeader(header *types.Header) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, header.Number) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(header.Number.Uint64(), 10))
		return
	}

	log.Info("event header", "number", header.Number.Int64(), " hash ", header.Hash().Hex(), " root ", header.Root.Hex(), " coinbase ", header.Coinbase.Hex(), " diffculty ", header.Difficulty.Int64())

	sh := c.chain.getSignedHeader(header.Number.Uint64(), header.Hash())
	if sh == nil {
		c.chain.makeCosmosSignedHeader(header)
	}

	var statedb *state.StateDB
	var err error
	if c.statefn != nil {
		statedb, err = c.statefn(header.Root)
	} else {
		statedb, err = state.New(header.Root, c.sdb, nil)
	}

	if err != nil {
		log.Warn("cosmos event header state root not found, maybe reorg...")
		return
	}
	c.queryExecutor.BeginBlock(header, statedb)
}

func (c *Cosmos) GetSignedHeader(height uint64, hash common.Hash) *ct.SignedHeader {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, big.NewInt(int64(height))) {
		log.Debug("cosmos not enable yet", "number", strconv.FormatUint(height, 10))
		return nil
	}
	return c.chain.getSignedHeader(height, hash)
}

// func (c *Cosmos) GetSignedHeaderWithSealHash(height uint64, sealHash common.Hash, hash common.Hash) *ct.SignedHeader {
// 	c.querymu.Lock()
// 	defer c.querymu.Unlock()
// 	if !IsEnable(c.config, big.NewInt(int64(height))) {
// 		log.Debug("cosmos not enable yet", strconv.FormatUint(height, 10))
// 		return nil
// 	}
// 	return c.chain.getSignedHeaderWithSealHash(height, sealHash, hash)
// }

func (c *Cosmos) HandleHeader(h *et.Header, header *ct.SignedHeader) (*types.CosmosVote, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	//if !IsEnable(c.config, h.Number) {
	//	log.Debug("cosmos not enable yet", "number", strconv.FormatUint(h.Number.Uint64(), 10))
	//	return nil, nil
	//}

	if header == nil {
		return nil, errors.New("missing cosmos header")
	}

	return c.chain.handleSignedHeader(h, header)
}

func (c *Cosmos) HandleVote(vote *et.CosmosVote) error {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	return c.chain.handleVote(vote)
}
