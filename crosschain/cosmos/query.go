package cosmos

import (
	"errors"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum/go-ethereum/rpc"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/bytes"
	tc "github.com/tendermint/tendermint/rpc/client"
	ct "github.com/tendermint/tendermint/rpc/core/types"
	tt "github.com/tendermint/tendermint/rpc/core/types"
	ttt "github.com/tendermint/tendermint/rpc/core/types"
)

func APIs(c *Cosmos) []rpc.API {
	return []rpc.API{{
		Namespace: "crosschain",
		Version:   "1.0",
		Service:   c,
		Public:    true,
	}}
}

// Query interface
func (c *Cosmos) CosmosABCIQuery(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*tt.ResultABCIQuery, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, c.queryExecutor.header.Number) {
		return nil, errors.New("Not Support")
	}

	q := abci.RequestQuery{
		Data: data, Path: path, Height: opts.Height, Prove: opts.Prove,
	}

	r := c.queryExecutor.app.BaseApp.Query(q)

	resp := &ct.ResultABCIQuery{Response: r}
	return resp, nil
}

func (c *Cosmos) CosmosTxsSearch(page, limit int, events []string) (*tt.ResultTxSearch, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, c.queryExecutor.header.Number) {
		return nil, errors.New("Not Support")
	}

	key := events[0] + "/" + events[1]
	data, err := c.queryExecutor.db.Get([]byte(key)[:])
	if err != nil {
		log.Debug("tx seach packet fail ", key, " ", err.Error())
		return nil, err
	}
	log.Debug("tx seach packet success ", key)

	var rdt abci.ResponseDeliverTx
	rdt.Unmarshal(data)
	rts := &ttt.ResultTxSearch{
		TotalCount: 1,
	}
	rts.Txs = make([]*ttt.ResultTx, 1)
	rts.Txs[0] = &ttt.ResultTx{TxResult: rdt}
	return rts, err
}

func (c *Cosmos) CosmosValidators(height *int64, page, perPage *int) ([]byte, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, c.queryExecutor.header.Number) {
		return nil, errors.New("Not Support")
	}

	vals := c.chain.GetValidators(*height)
	if vals == nil {
		return nil, errors.New("invalid validators")
	}

	// val := &tt.ResultValidators{BlockHeight: *height, Count: len(vals.Validators), Total: len(vals.Validators)}
	// // copy(val.Validators, vals.Validators)
	// val.Validators = vals.Validators

	vt, _ := vals.ToProto()
	return vt.Marshal()
}

func (c *Cosmos) CosmosLightBlock(height *int64) ([]byte, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	if !IsEnable(c.config, c.queryExecutor.header.Number) {
		return nil, errors.New("Not Support")
	}

	lb := c.chain.GetLightBlock(*height)
	if lb != nil {
		tlb, _ := lb.ToProto()
		return tlb.Marshal()
	} else {
		return nil, errors.New("invalid height")
	}
}
