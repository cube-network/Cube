package systemcontract

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

type TestChainContext struct {
	header *types.Header
	engine consensus.Engine
}

func (c *TestChainContext) Engine() consensus.Engine {
	return c.engine
}

func (c *TestChainContext) GetHeader(common.Hash, uint64) *types.Header {
	return c.header
}

type TestEngine struct {
}

func (c *TestEngine) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

func (c *TestEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return nil
}

func (c *TestEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return make(chan struct{}), make(chan error, len(headers))
}

func (c *TestEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (c *TestEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (c *TestEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction, uncles []*types.Header, receipts *[]*types.Receipt, punishTxs []*types.Transaction) error {
	return nil
}

func (c *TestEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, []*types.Receipt, error) {
	return nil, receipts, nil
}

func (c *TestEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}

func (c *TestEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return common.Big2
}

func (c *TestEngine) SealHash(header *types.Header) common.Hash {
	return common.Hash{}
}

func (c *TestEngine) Close() error {
	return nil
}

func (c *TestEngine) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{}
}

func initCallContext() (*CallContext, error) {
	file, err := os.Open("../../../core/testdata/test-genesis.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		return nil, err
	}

	db := rawdb.NewMemoryDatabase()

	genesisBlock := genesis.ToBlock(db)

	header := &types.Header{
		ParentHash: genesisBlock.Hash(),
		Number:     big.NewInt(200),
		Difficulty: common.Big2,
		Time:       uint64(time.Now().Unix()),
	}

	var statedb *state.StateDB
	if statedb, err = state.New(genesisBlock.Root(), state.NewDatabase(db), nil); err != nil {
		return nil, err
	}

	return &CallContext{
		Statedb:      statedb,
		Header:       header,
		ChainContext: &TestChainContext{header, &TestEngine{}},
		ChainConfig:  genesis.Config,
	}, nil
}

func TestGetTopValidators(t *testing.T) {
	ctx, err := initCallContext()
	if err != nil {
		t.Fatal("Init call context error", "err", err)
	}
	vals, err := GetTopValidators(ctx)

	if err != nil {
		t.Error(err)
	}
	t.Log(vals)
}

func TestUpdateActiveValidatorSet(t *testing.T) {
	ctx, err := initCallContext()
	if err != nil {
		t.Fatal("Init call context error", "err", err)
	}

	if err := UpdateActiveValidatorSet(ctx, []common.Address{common.BigToAddress(big.NewInt(8))}); err != nil {
		t.Error(err)
	}
}
