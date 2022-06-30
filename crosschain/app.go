package crosschain

import (
	"encoding/hex"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/tendermint/libs/bytes"
	tl "github.com/tendermint/tendermint/libs/log"
	tc "github.com/tendermint/tendermint/rpc/client"
	ct "github.com/tendermint/tendermint/rpc/core/types"
	tt "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

type CosmosApp struct {
	*baseapp.BaseApp

	codec        EncodingConfig
	mm           *module.Manager
	configurator module.Configurator

	cc *CosmosChain
}

// TODO level db/mpt wrapper
func NewCosmosApp() *CosmosApp {
	log.Debug("new cosmos app...")
	// TODO make db
	var db dbm.DB
	codec := MakeEncodingConfig()
	bApp := baseapp.NewBaseApp("Cube", tl.NewNopLogger(), db, codec.TxConfig.TxDecoder())
	// bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(codec.InterfaceRegistry)

	// TODO read path from cmdline/conf
	path := "./data/"
	cc := MakeCosmosChain(path+"priv_validator_key.json", path+"priv_validator_state.json")
	app := &CosmosApp{BaseApp: bApp, codec: codec, cc: cc}

	app.mm = module.NewManager( /* TODO add ibc module here*/ )
	app.configurator = module.NewConfigurator(app.codec.Marshaler, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	return app
}

//called before mpt.commit
func (app *CosmosApp) CommitIBC() common.Hash {
	// app.cc.map[height] = app_hash;
	return common.Hash{}
}

func (app *CosmosApp) MakeHeader(h *et.Header, app_hash common.Hash) {
	log.Debug("log make header test")
	app.cc.MakeLightBlockAndSign(h, app_hash)

}

func (app *CosmosApp) Vote(block_height uint64, Address tt.Address) {
	// app.cc.MakeCosmosSignedHeader(h, nil)

}

// ABCI Query
func (app *CosmosApp) Query(path string, data bytes.HexBytes, opts tc.ABCIQueryOptions) (*ct.ResultABCIQuery, error) {
	return nil, nil
}

func (app *CosmosApp) RequiredGas(input []byte) uint64 {
	// TODO fixed gas cost for demo test
	return 20000
}

func (app *CosmosApp) Run(block_ctx vm.BlockContext, stdb vm.StateDB, input []byte) ([]byte, error) {
	_, arg, err := UnpackInput(input)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	msgs, err := app.GetMsgs(arg)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	for _, msg := range msgs {
		if handler := app.MsgServiceRouter().Handler(msg); handler != nil {
			/*msgResult*/ _, err := handler( /*TODO statedb stateobject wrapper */ sdk.Context{}, msg)
			if err != nil {
				return nil, vm.ErrExecutionReverted
			}
			// TODO make result, save ??
		} else {
			return nil, vm.ErrExecutionReverted
		}
	}

	return nil, nil
}

func (app *CosmosApp) GetMsgs(arg string) ([]sdk.Msg, error) {
	argbin, err := hex.DecodeString(arg)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	var body tx.TxBody
	err = app.codec.Marshaler.Unmarshal(argbin, &body)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	anys := body.Messages
	res := make([]sdk.Msg, len(anys))
	for i, any := range anys {
		cached := any.GetCachedValue()
		if cached == nil {
			panic("Any cached value is nil. Transaction messages must be correctly packed Any values.")
		}
		res[i] = cached.(sdk.Msg)
	}
	return res, nil
}
