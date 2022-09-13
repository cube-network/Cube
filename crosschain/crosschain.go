package crosschain

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/crosschain/cosmos"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	ct "github.com/tendermint/tendermint/types"
)

type CrossChain interface {
	Init(datadir string, ethdb ethdb.Database, statedb state.Database, chainConfig *params.ChainConfig, blockContext vm.BlockContext, statefn cccommon.StateFn, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, finalizeblockfn cccommon.GetLastFinalizedBlockNumberFn, header *types.Header)
	SetCoinbase(addr common.Address)

	APIs() []rpc.API

	NewExecutor(header *types.Header, statedb *state.StateDB, layLoadVersion bool) vm.CrossChain
	FreeExecutor(exec vm.CrossChain)
	Seal(exec vm.CrossChain)

	EventHeader(header *types.Header) *types.CosmosVote

	// vote
	GetSignatures(hash common.Hash) []ct.CommitSig
	HandleSignatures(h *types.Header, sigs []ct.CommitSig) error //(*types.CosmosVote, error)

	HandleVote(vote *types.CosmosVote) error

	// collect
	CheckVotes(height uint64, hash common.Hash) *types.CosmosLackedVoteIndexs //(*types.CosmosVotesList, *types.CosmosLackedVoteIndexs)
	HandleVotesQuery(idxs *types.CosmosLackedVoteIndexs) (*types.CosmosVotesList, error)
	HandleVotesList(votes *types.CosmosVotesList) error
}

var cc CrossChain
var once sync.Once

func GetCrossChain() CrossChain {
	once.Do(func() {
		cc = &cosmos.Cosmos{}
	})
	return cc
}
