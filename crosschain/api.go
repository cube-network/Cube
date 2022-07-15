package crosschain

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
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

// Query interface

// for test only
func (api *API) CosmosABCIInfo() (*tt.ResultABCIInfo, error) {
	res := &tt.ResultABCIInfo{Response: abci.ResponseInfo{Data: "Hello", Version: "CosmosABCIInfo", AppVersion: 1, LastBlockHeight: 1, LastBlockAppHash: []byte{'a', 'b'}}}
	return res, nil
}

func (api *API) CosmosABCIQuery(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*tt.ResultABCIQuery, error) {
	// println("data ", hex.EncodeToString(data), " path ", path, " height ", opts.Height, " prove ", opts.Prove)
	return api.app.Query(path, data, opts)
}

func (api *API) CosmosValidators(height *int64, page, perPage *int) (*tt.ResultValidators, error) {
	lb := api.app.cc.GetLightBlock(*height)
	if lb == nil {
		return nil, errors.New("invalid validators")
	}

	val := &tt.ResultValidators{BlockHeight: *height, Count: 1, Total: 1}
	copy(val.Validators, lb.ValidatorSet.Validators)
	return val, nil
}

func (api *API) CosmosTxsSearch(page, limit int, events []string) (*tt.ResultTxSearch, error) {
	return api.app.TxsSearch(page, limit, events)
}

func (api *API) CosmosLightBlock(height *int64) ([]byte, error) {
	lb := api.app.cc.GetLightBlock(*height)
	if lb != nil {
		tlb, _ := lb.ToProto()
		return tlb.Marshal()
	} else {
		return nil, errors.New("invalid height")
	}
}

func (api *API) CosmosBalances(account common.Address) (*sdk.Coins, error) {
	// TODO
	return nil, nil
}

func (api *API) CosmosLastBlockHeight() int64 {
	return api.app.cc.LastBlockHeight()
}
