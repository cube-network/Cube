package crosschain

import (
	"github.com/ethereum/go-ethereum/rpc"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	tc "github.com/tendermint/tendermint/rpc/client"
	tt "github.com/tendermint/tendermint/rpc/core/types"
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
func (api *API) ABCIInfo() (*tt.ResultABCIInfo, error) {
	res := &tt.ResultABCIInfo{Response: abci.ResponseInfo{Data: "a", Version: "b", AppVersion: 1, LastBlockHeight: 1, LastBlockAppHash: []byte{'a', 'b'}}}
	return res, nil
}

func (api *API) ABCIQuery(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*tt.ResultABCIQuery, error) {
	return api.app.Query(path, data, opts)
}

func (api *API) Block(height *int64) (*tt.ResultBlock, error) {
	// resultBlock.Block.Time.UnixNano()
	return nil, nil
}

func (api *API) Commit(height *int64) (*tt.ResultCommit, error) {
	// time
	// apphash
	return nil, nil
}

func (api *API) Validators(height *int64, page, perPage *int) (*tt.ResultValidators, error) {
	// validator set
	return nil, nil
}

// cmd
func (api *API) Tx(hash []byte, prove bool) (*tt.ResultTx, error) {
	// proof
	// TODO tendermint
	return nil, nil
}

func (api *API) TxSearch(
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*tt.ResultTxSearch, error) {
	return nil, nil
}

func (api *API) Status() (*tt.ResultStatus, error) {
	//h.SyncInfo.CatchingUp
	//h.SyncInfo.LatestBlockHeight
	return nil, nil
}
