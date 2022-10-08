package cosmos

import (
	"errors"
	"strconv"

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

	executor, ok := c.getQueryExecutor(opts.Height, ExecutorModeQuery)
	if !ok {
		return nil, errors.New("not support crosschain")
	}
	defer c.freeExecutorWithoutLock(executor)

	q := abci.RequestQuery{
		Data: data, Path: path, Height: opts.Height, Prove: opts.Prove,
	}

	log.Debug("query height ", strconv.FormatInt(q.Height, 10), " executor height ", executor.header.Number.String(), " hash ", executor.header.Hash().Hex(), " path ", path)

	r := executor.app.BaseApp.Query(q)
	resp := &ct.ResultABCIQuery{Response: r}
	return resp, nil
}

func (c *Cosmos) CosmosTxsSearch(height uint64, page, limit int, events []string) (*tt.ResultTxSearch, error) {
	c.querymu.Lock()
	defer c.querymu.Unlock()

	executor, ok := c.getQueryExecutor(int64(height), ExecutorModeQuery)
	if !ok {
		return nil, errors.New("not support crosschain")
	}
	defer c.freeExecutorWithoutLock(executor)

	key := events[0] + "/" + events[1]
	data, err := executor.db.Get([]byte(key)[:])
	if err != nil {
		log.Debug("tx seach packet fail ", key, " ", err.Error())
		return nil, err
	}
	log.Debug("tx seach packet success ", key, " height ", strconv.FormatInt(int64(height), 10), " executor height ", executor.header.Number.String(), " hash ", executor.header.Hash().Hex())

	var rdt abci.ResponseDeliverTx
	rdt.Unmarshal(data)
	rts := &ttt.ResultTxSearch{
		TotalCount: 1,
	}
	rts.Txs = make([]*ttt.ResultTx, 1)
	rts.Txs[0] = &ttt.ResultTx{TxResult: rdt}
	return rts, err
}
