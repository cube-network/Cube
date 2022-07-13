package crosschain

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	tc "github.com/tendermint/tendermint/rpc/client"
	ct "github.com/tendermint/tendermint/rpc/core/types"
	ttt "github.com/tendermint/tendermint/rpc/core/types"
)

// ABCI Query
func (app *CosmosApp) Query(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*ct.ResultABCIQuery, error) {
	// TODO check base app query
	q := abci.RequestQuery{
		Data: data, Path: path, Height: opts.Height, Prove: opts.Prove,
	}
	// if q.Height == 0 {
	// 	q.Height = app.cc.LastBlockHeight()
	// }
	r := app.BaseApp.Query(q)

	resp := &ct.ResultABCIQuery{Response: r}
	return resp, nil
}

func (app *CosmosApp) TxsSearch(page, limit int, events []string) (*ttt.ResultTxSearch, error) {
	key := events[0] + "/" + events[1]
	data, err := app.db.Get([]byte(key)[:])
	var rdt abci.ResponseDeliverTx
	rdt.Unmarshal(data)
	rts := &ttt.ResultTxSearch{
		TotalCount: 1,
	}
	rts.Txs = make([]*ttt.ResultTx, 1)
	rts.Txs[0].TxResult = rdt
	return rts, err
}
