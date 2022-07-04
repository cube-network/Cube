package crosschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	tc "github.com/tendermint/tendermint/rpc/client"
	tt "github.com/tendermint/tendermint/rpc/core/types"
	ttt "github.com/tendermint/tendermint/types"
)

type API struct {
	app *CosmosApp
}

func APIs(app *CosmosApp) []rpc.API {
	return []rpc.API{{
		Namespace: "crosschain",
		Version:   "1.0",
		Service:   &API{app: app},
		Public:    true,
	}}
}

// Query interface //

// for test only
func (api *API) CosmosABCIInfo() (*tt.ResultABCIInfo, error) {
	res := &tt.ResultABCIInfo{Response: abci.ResponseInfo{Data: "Hello", Version: "CosmosABCIInfo", AppVersion: 1, LastBlockHeight: 1, LastBlockAppHash: []byte{'a', 'b'}}}
	return res, nil
}

func (api *API) CosmosABCIQuery(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*tt.ResultABCIQuery, error) {
	// println("data ", hex.EncodeToString(data), " path ", path, " height ", opts.Height, " prove ", opts.Prove)
	return api.app.Query(path, data, opts)
}

// func (api *API) CosmosBlock(height *int64) (*tt.ResultBlock, error) {
// 	// resultBlock.Block.Time.UnixNano()
// 	return nil, nil
// }

// func (api *API) CosmosCommit(height *int64) (*tt.ResultCommit, error) {
// 	// time
// 	// apphash
// 	return nil, nil
// }

func (api *API) CosmosValidators(height *int64, page, perPage *int) (*tt.ResultValidators, error) {
	// validator set
	return nil, nil
}

// // cmd
// func (api *API) CosmosTx(hash []byte, prove bool) (*tt.ResultTx, error) {
// 	// proof
// 	// TODO tendermint
// 	return nil, nil
// }

func (api *API) CosmosTxsSearch(page, limit int, events []string) (*tt.ResultTxSearch, error) {
	return nil, nil
}

// func (api *API) CosmosStatus() (*tt.ResultStatus, error) {
// 	//h.SyncInfo.CatchingUp
// 	//h.SyncInfo.LatestBlockHeight
// 	return nil, nil
// }

func (api *API) CosmosLightBlock(height *int64) (*ttt.LightBlock, error) {
	return nil, nil
}

func (api *API) CosmosBalances(account common.Address) (*sdk.Coins, error) {
	return nil, nil
}
