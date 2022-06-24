package systemcontract

import (
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
)

const testAbi = `[
	{
		"inputs": [],
		"name": "getActiveValidators",
		"outputs": [
			{
				"internalType": "address[]",
				"name": "",
				"type": "address[]"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "currRewardsPerBlock",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	  },
	  {
		"inputs": [
		  {
			"internalType": "address",
			"name": "",
			"type": "address"
		  }
		],
		"name": "valMaps",
		"outputs": [
		  {
			"internalType": "contract IValidator",
			"name": "",
			"type": "address"
		  }
		],
		"stateMutability": "view",
		"type": "function"
	  },
	  {
		"inputs": [
			{
				"internalType": "address",
				"name": "_val",
				"type": "address"
			}
		],
		"name": "getPunishRecord",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "currFeeRewards",
		"outputs": [
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var GenesisValidators = []common.Address{
	common.HexToAddress("0x1B5813bAA493742CEe5d2Eb7410b3014fe3cf2b6"),
	common.HexToAddress("0x8Cc5A1a0802DB41DB826C2FcB72423744338DcB0"),
}

func TestGetTopValidators(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	vals, err := GetTopValidators(ctx)
	if assert.NoError(t, err) {
		assert.Equal(t, GenesisValidators, vals)
	}
}

func TestUpdateActiveValidatorSet(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	getActiveValidators := func(ctx *CallContext) []common.Address {
		validators, ok := readSystemContract(t, ctx, "getActiveValidators").([]common.Address)
		assert.True(t, ok, "invalid validator format")
		return validators
	}

	newSet := []common.Address{
		common.BigToAddress(big.NewInt(111)),
		common.BigToAddress(big.NewInt(222)),
	}

	if assert.NoError(t, UpdateActiveValidatorSet(ctx, newSet)) {
		assert.Equal(t, newSet, getActiveValidators(ctx))
	}
}

func TestDecreaseMissedBlocksCounter(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	getPunishRecord := func(val common.Address) int {
		count, ok := readSystemContract(t, ctx, "getPunishRecord", val).(*big.Int)
		assert.True(t, ok, "invalid result format")
		return int(count.Int64())
	}

	LazyPunish(ctx, GenesisValidators[0])

	assert.Equal(t, 1, getPunishRecord(GenesisValidators[0]))

	assert.NoError(t, DecreaseMissedBlocksCounter(ctx))

	assert.Equal(t, 0, getPunishRecord(GenesisValidators[0]))
}

func TestGetRewardsUpdatePeroid(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	period, err := GetRewardsUpdatePeroid(ctx)
	if assert.NoError(t, err) {
		assert.Equal(t, 28800, int(period), "Read from system contract mismatch")
	}
}

func TestUpdateRewardsInfo(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	currRewardsPerBlock := func(ctx *CallContext) *big.Int {
		ret, ok := readSystemContract(t, ctx, "currRewardsPerBlock").(*big.Int)
		assert.True(t, ok, "invalid result format")
		return ret
	}

	period, err := GetRewardsUpdatePeroid(ctx)
	assert.NoError(t, err)

	ctx.Header.Number.SetUint64(period)

	assert.Equal(t, new(big.Int).Div(core.RewardsByMonth(0)[0], blocksPerMonth), currRewardsPerBlock(ctx))

	ctx.Header.Number.SetUint64(period * 33)

	assert.NoError(t, UpdateRewardsInfo(ctx))

	assert.Equal(t, new(big.Int).Div(core.RewardsByMonth(0)[1], blocksPerMonth), currRewardsPerBlock(ctx))
}

func TestDistributeBlockFee(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	getValidatorFee := func(val common.Address) *big.Int {
		contract, ok := readSystemContract(t, ctx, "valMaps", val).(common.Address)
		assert.True(t, ok, "invalid contract format")
		fee, ok := readContract(t, ctx, &contract, "currFeeRewards").(*big.Int)
		assert.True(t, ok, "invalid fee format")
		return fee
	}

	assert.NoError(t, UpdateActiveValidatorSet(ctx, GenesisValidators))

	origin := ctx.Statedb.GetBalance(ctx.Header.Coinbase)
	fee := big.NewInt(1000000000000000000)

	assert.NoError(t, DistributeBlockFee(ctx, fee))

	assert.Equal(t, new(big.Int).Sub(origin, fee), ctx.Statedb.GetBalance(ctx.Header.Coinbase))

	assert.Equal(t, big.NewInt(fee.Int64()/5), ctx.Statedb.GetBalance(system.CommunityPoolContract))

	valAmount := big.NewInt(fee.Int64() / 5 * 4 / 2)
	assert.Equal(t, valAmount, getValidatorFee(GenesisValidators[0]))
	assert.Equal(t, valAmount, getValidatorFee(GenesisValidators[1]))

}

func TestLazyPunish(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")
	getPunishRecord := func(val common.Address) int {
		count, ok := readSystemContract(t, ctx, "getPunishRecord", val).(*big.Int)
		assert.True(t, ok, "invalid validator format")
		return int(count.Int64())
	}

	assert.Equal(t, 0, getPunishRecord(GenesisValidators[0]))

	assert.NoError(t, LazyPunish(ctx, GenesisValidators[0]))

	assert.Equal(t, 1, getPunishRecord(GenesisValidators[0]))

}

func TestDoubleSignPunish(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	punishHash := common.BigToHash(big.NewInt(886))

	punished, err := IsDoubleSignPunished(ctx, punishHash)
	assert.NoError(t, err)
	assert.False(t, punished)

	assert.NoError(t, DoubleSignPunish(ctx, punishHash, GenesisValidators[0]))

	punished, err = IsDoubleSignPunished(ctx, punishHash)
	assert.NoError(t, err)
	assert.True(t, punished)
}

func TestDoubleSignPunishGivenEVM(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	punishHash := common.BigToHash(big.NewInt(886))

	punished, err := IsDoubleSignPunished(ctx, punishHash)
	assert.NoError(t, err)
	assert.False(t, punished)

	blockContext := core.NewEVMBlockContext(ctx.Header, ctx.ChainContext, nil)
	evm := vm.NewEVM(blockContext, vm.TxContext{}, ctx.Statedb, ctx.ChainConfig, vm.Config{})

	assert.NoError(t, DoubleSignPunishWithGivenEVM(evm, ctx.Header.Coinbase, punishHash, GenesisValidators[0]))

	punished, err = IsDoubleSignPunished(ctx, punishHash)
	assert.NoError(t, err)
	assert.True(t, punished)
}

func TestIsDoubleSignPunished(t *testing.T) {
	ctx, err := initCallContext()
	assert.NoError(t, err, "Init call context error")

	punishHash := common.BigToHash(big.NewInt(886))

	assert.NoError(t, DoubleSignPunish(ctx, punishHash, GenesisValidators[0]))

	check, err := IsDoubleSignPunished(ctx, punishHash)
	if assert.NoError(t, err) {
		assert.True(t, check)
	}
}

// Utils function to create call context
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
		Coinbase:   common.HexToAddress("0x352BbF453fFdcba6b126a73eD684260D7968dDc8"),
	}

	var statedb *state.StateDB
	if statedb, err = state.New(genesisBlock.Root(), state.NewDatabase(db), nil); err != nil {
		return nil, err
	}

	return &CallContext{
		Statedb:      statedb,
		Header:       header,
		ChainContext: &MockChainContext{header, &MockConsensusEngine{}},
		ChainConfig:  genesis.Config,
	}, nil
}

func readSystemContract(t *testing.T, ctx *CallContext, method string, args ...interface{}) interface{} {
	return readContract(t, ctx, &system.StakingContract, method, args...)
}

func readContract(t *testing.T, ctx *CallContext, contract *common.Address, method string, args ...interface{}) interface{} {
	abi, err := abi.JSON(strings.NewReader(testAbi))
	// execute contract
	data, err := abi.Pack(method, args...)
	assert.NoError(t, err)

	result, err := CallContract(ctx, contract, data)
	assert.NoError(t, err)

	// unpack data
	ret, err := abi.Unpack(method, result)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ret), "invalid result length")
	return ret[0]
}

// MockChainContext implements ChainContext for unit test
type MockChainContext struct {
	header *types.Header
	engine consensus.Engine
}

func (c *MockChainContext) Engine() consensus.Engine {
	return c.engine
}

func (c *MockChainContext) GetHeader(common.Hash, uint64) *types.Header {
	return c.header
}

// MockConsensusEngine implements Engine for unit test
type MockConsensusEngine struct {
}

func (c *MockConsensusEngine) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

func (c *MockConsensusEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return nil
}

func (c *MockConsensusEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return make(chan struct{}), make(chan error, len(headers))
}

func (c *MockConsensusEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (c *MockConsensusEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (c *MockConsensusEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction, uncles []*types.Header, receipts *[]*types.Receipt, punishTxs []*types.Transaction, proposalTxs []*types.Transaction) error {
	return nil
}

func (c *MockConsensusEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, []*types.Receipt, error) {
	return nil, receipts, nil
}

func (c *MockConsensusEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}

func (c *MockConsensusEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return common.Big2
}

func (c *MockConsensusEngine) SealHash(header *types.Header) common.Hash {
	return common.Hash{}
}

func (c *MockConsensusEngine) Close() error {
	return nil
}

func (c *MockConsensusEngine) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{}
}
