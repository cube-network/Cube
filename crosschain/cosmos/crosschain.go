package cosmos

import (
	"container/list"
	"sync"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

type Cosmos struct {
	datadir string
	ethdb   ethdb.Database
	sdb     state.Database
	config  *params.ChainConfig
	header  *types.Header

	codec        EncodingConfig
	blockContext vm.BlockContext

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
	header *types.Header) {

	c.callmu.Lock()
	defer c.callmu.Unlock()

	once.Do(func() {
		c.datadir = datadir
		c.ethdb = ethdb
		c.sdb = sdb
		c.config = config
		c.blockContext = blockContext
		c.header = header

		c.codec = MakeEncodingConfig()

		c.callExectors = list.New()

		statedb, err := state.New(header.Root, c.sdb, nil)
		if err != nil {
			panic("cosmos init state root not found")
		}
		c.queryExecutor = NewCosmosExecutor(c.datadir, c.config, c.codec, c.chain.GetLightBlock, c.blockContext, statedb, c.header)
		c.chain = MakeCosmosChain(config.ChainID.String(), datadir+"priv_validator_key.json", datadir+"priv_validator_state.json")
	})

}

func (c *Cosmos) APIs() []rpc.API {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	return APIs(c)
}

func IsEnable(config *params.ChainConfig, header *types.Header) bool {
	return config.IsCrosschainCosmos(header.Number)
}

func (c *Cosmos) NewExecutor(header *types.Header, statedb *state.StateDB) vm.CrossChain {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	if !IsEnable(c.config, header) {
		return nil
	}

	var exector *Executor
	if c.callExectors.Len() > 0 {
		// TODO max list len
		elem := c.callExectors.Front()
		exector = elem.Value.(*Executor)
		c.callExectors.Remove(elem)
	} else {
		exector = NewCosmosExecutor(c.datadir, c.config, c.codec, c.chain.GetLightBlock, c.blockContext, statedb, header)
	}
	exector.BeginBlock(header, statedb)
	c.newExecutorCounter++
	println("newExecutorCounter ", c.newExecutorCounter)
	return exector
}

func (c *Cosmos) FreeExecutor(exec vm.CrossChain) {
	c.callmu.Lock()
	defer c.callmu.Unlock()
	if exec == nil {
		return
	}
	executor := exec.(*Executor)
	if executor == nil || !IsEnable(c.config, executor.header) {
		return
	}

	c.callExectors.PushFront(exec)
	c.freeExecutorCounter++
	println("freeExecutorCounter ", c.freeExecutorCounter)
}

func (c *Cosmos) Seal(exec vm.CrossChain) {
	c.callmu.Lock()
	defer c.callmu.Unlock()

	if exec == nil {
		return
	}

	executor := exec.(*Executor)
	if executor == nil || !IsEnable(c.config, executor.header) {
		return
	}

	executor.EndBlock()
}

func (c *Cosmos) EventHeader(header *types.Header) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, header) {
		return
	}

	c.chain.MakeLightBlock(header)

	statedb, err := state.New(header.Root, c.sdb, nil)
	if err != nil {
		panic("cosmos event header state root not found")
	}
	c.queryExecutor.BeginBlock(header, statedb)
}

func (c *Cosmos) SignHeader(header *types.Header) *cccommon.CrossChainSignature {
	// TODO
	return nil
}

func (c *Cosmos) VoteHeader(header *types.Header, signature *cccommon.CrossChainSignature) {
	// TODO
}

// func (app *CosmosApp) MakeHeader(h *cubetypes.Header) *tenderminttypes.Header {
// 	app_hash := h.Extra[32:64]
// 	app.cc.MakeLightBlockAndSign(h, common.BytesToHash(app_hash))
// 	println("header ", app.cc.GetLightBlock(h.Number.Int64()).Header.AppHash.String(), " ", time.Now().UTC().String())
// 	return app.cc.GetLightBlock(h.Number.Int64()).Header
// }
