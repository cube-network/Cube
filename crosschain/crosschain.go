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
	Init(datadir string, ethdb ethdb.Database, statedb state.Database, chainConfig *params.ChainConfig, blockContext vm.BlockContext, statefn cccommon.StateFn, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, header *types.Header)
	SetCoinbase(addr common.Address)
	SetFuncs(getNonce cccommon.GetNonceFn, getPrice cccommon.GetPriceFn, addLocalTx cccommon.AddLocalTxFn)
	SetSignTx(signTx cccommon.SignTxFn)

	APIs() []rpc.API

	NewExecutor(header *types.Header, statedb *state.StateDB) vm.CrossChain
	FreeExecutor(exec vm.CrossChain)
	Seal(exec vm.CrossChain)

	EventHeader(header *types.Header) *types.CosmosVote

	//// TODO remove cosmos info
	//GetSignedHeader(height uint64, hash common.Hash) *ct.SignedHeader
	//HandleHeader(h *types.Header, header *ct.SignedHeader) (*types.CosmosVote, error)

	GetSignatures(hash common.Hash) []ct.CommitSig
	HandleSignatures(h *types.Header, sigs []ct.CommitSig) error //(*types.CosmosVote, error)

	//SignHeader(h *types.Header) (*types.CosmosVote, error)
	HandleVote(vote *types.CosmosVote) error

	// collect cosmos votes
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

// // TODO CrossChains for multi crosschain later
// type CrossChains struct {
// 	modules map[string]*Module
// }

// func (cc *CrossChain) Init(datadir string,
// 	ethdb ethdb.Database,
// 	config *params.ChainConfig,
// 	header *types.Header) CrossChain {

// 	cc := &CrossChain{}
// 	cc.modules = make(map[string]*Module)
// 	cc.modules["cosmos"] = cosmos.NewCrossChain(datadir,ethdb, config, header)

// 	return nil
// }

// func (cc *CrossChain) APIs() []rpc.API {
// 	var api []rpc.API
// 	for _, v := range cc.modules {
// 		api = append(api, v.APIs())
// 	}

// 	return api
// }

// func (cc *CrossChain) BeginBlock(statedb *state.StateDB, header *types.Header) {
// 	for _, v := range cc.modules {
// 		v.BeginBlock(statedb, header)
// 	}
// }

// func (cc *CrossChain) EndBlock(statedb *state.StateDB, header *types.Header) map[string]*state.StateDB {
// 	stdbs = make(map[string]*state.StateDB)
// 	for k, v := range cc.modules {
// 		stdbs[k] = v.EndBlock(statedb, header)
// 	}

// 	return stdbs
// }

// func (cc *CrossChain) CommitBlock(statedb *state.StateDB, header *types.Header) {
// 	for _, v := range cc.modules {
// 		v.CommitBlock(statedb, header)
// 	}
// }

// func (cc *CrossChain) EventHeader(header *types.Header) {
// 	for _, v := range cc.modules {
// 		v.EventHeader(header)
// 	}
// }

// func (cc *CrossChain) SignHeader(header *types.Header) map[string]*CrossChainSignature {
// 	signatures := make(map[string]*CrossChainSignature)
// 	for k, v := range cc.modules {
// 		signatures[k] = v.SignHeader(header)
// 	}

// 	return signatures
// }

// func (cc *CrossChain) VoteHeader(header *types.Header, signature CrossChainSignature) {
// 	for k, v := range cc.modules {
// 		v.VoteHeader(header, signature)
// 	}
// }
