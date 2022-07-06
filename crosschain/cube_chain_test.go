package crosschain

//
//import (
//	"fmt"
//	"github.com/ethereum/go-ethereum/core/types"
//	"math/big"
//	"testing"
//
//	ibctesting "github.com/cosmos/ibc-go/v4/testing"
//)
//
//type TestCubeChain struct {
//	*testing.T
//
//	Coordinator *ibctesting.Coordinator
//	App         ibctesting.TestingApp
//	ChainID     string
//	//LastHeader    *ibctmtypes.Header // header for last block height committed
//	LastHeader    *types.Header
//	CurrentHeader *types.Header // header for current block height
//
//	//Vals     *tmtypes.ValidatorSet
//	//NextVals *tmtypes.ValidatorSet
//}
//
//// NewTestChain initializes a new test chain with a default of 4 validators
//// Use this function if the tests do not need custom control over the validator set
//func NewTestCubeChain(t *testing.T, coord *ibctesting.Coordinator, chainID string) *TestCubeChain {
//	app := NewCubeApp()
//	chain := &TestCubeChain{
//		T:             t,
//		Coordinator:   coord,
//		ChainID:       app.ChainID,
//		App:           app,
//		CurrentHeader: app.eth.BlockChain().CurrentHeader(),
//	}
//
//	//coord.CommitBlock(chain)
//
//	return chain
//}
//
//func (chain *TestCubeChain) NextBlock() {
//	app := chain.App.(*CubeApp)
//	state, err := app.eth.BlockChain().State()
//	if err != nil {
//		fmt.Println("get state failed when generate next block")
//		return
//	}
//
//	chain.LastHeader = chain.CurrentHeader
//	height := big.NewInt(chain.LastHeader.Number.Int64() + 1)
//	chain.CurrentHeader = &types.Header{
//		Number:     height,
//		Root:       state.IntermediateRoot(app.eth.BlockChain().Config().IsEIP158(height)),
//		ParentHash: chain.LastHeader.Hash(),
//	}
//
//	app.eth.BlockChain().InsertChain([]*types.Block{})
//}
