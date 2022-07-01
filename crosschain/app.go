package crosschain

import (
	"encoding/hex"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/version"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ica "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v3/modules/apps/27-interchain-accounts/types"
	ibcfee "github.com/cosmos/ibc-go/v3/modules/apps/29-fee"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v3/modules/core/05-port/types"
	ibchost "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"
	ibcmock "github.com/cosmos/ibc-go/v3/testing/mock"
	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/expectedkeepers"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/tendermint/libs/bytes"
	tl "github.com/tendermint/tendermint/libs/log"
	tc "github.com/tendermint/tendermint/rpc/client"
	ct "github.com/tendermint/tendermint/rpc/core/types"
	tt "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var (
	// ModuleBasics defines the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		ica.AppModuleBasic{},
	)

	// Add module account permissions
	maccPerms = map[string][]string{
		icatypes.ModuleName: nil,
	}
)

type CosmosApp struct {
	*baseapp.BaseApp

	codec        EncodingConfig
	mm           *module.Manager
	configurator module.Configurator

	// keys to access the substores
	keys    map[string]*sdk.KVStoreKey
	tkeys   map[string]*sdk.TransientStoreKey
	memKeys map[string]*sdk.MemoryStoreKey

	// keepers
	ParamsKeeper     paramskeeper.Keeper
	AccountKeeper    icatypes.AccountKeeper //authkeeper.AccountKeeper
	CapabilityKeeper *capabilitykeeper.Keeper
	IBCKeeper        *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly

	// todo: to be replaced
	StakingKeeper clienttypes.StakingKeeper
	UpgradeKeeper upgradekeeper.Keeper // todo: clienttypes.UpgradeKeeper

	mockModule ibcmock.AppModule // acts as the interchain accounts authentication module
	// Add Interchain Accounts Keepers for each submodule used and the authentication module
	// If a submodule is being statically disabled, the associated Keeper does not need to be added.
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper
	//ICAAuthKeeper       icaauthkeeper.Keeper

	cc *CosmosChain
}

// TODO level db/mpt wrapper
func NewCosmosApp(skipUpgradeHeights map[int64]bool) *CosmosApp {
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

	app.setupBasicKeepers(skipUpgradeHeights, path)

	// Create IBC Router
	ibcRouter := porttypes.NewRouter()
	// setup for the interchain account module
	app.setupICAKeepers(ibcRouter)

	// Seal the IBC Router
	app.IBCKeeper.SetRouter(ibcRouter)

	app.mm = module.NewManager( /* TODO add ibc module here*/
		ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper), // Create Interchain Accounts AppModule
	)
	app.configurator = module.NewConfigurator(app.codec.Marshaler, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	return app
}

func (app *CosmosApp) setupBasicKeepers(skipUpgradeHeights map[int64]bool, homePath string) {
	app.keys = sdk.NewKVStoreKeys(
		icacontrollertypes.StoreKey, // Create store keys for each submodule Keeper and the authentication module
		icahosttypes.StoreKey,
		//icaauthtypes.StoreKey,	// todo: we need to create our own authentication module
	)
	app.memKeys = sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	app.tkeys = sdk.NewTransientStoreKeys(paramstypes.TStoreKey)

	appCodec := app.codec.Marshaler
	legacyAmino := app.codec.Amino
	//interfaceRegistry := app.codec.InterfaceRegistry

	// todo: for consensus parameters. Do we need to create our own ParamsKeeper?
	app.ParamsKeeper = initParamsKeeper(appCodec, legacyAmino, app.keys[paramstypes.StoreKey], app.tkeys[paramstypes.TStoreKey])
	// set the BaseApp's parameter store
	app.BaseApp.SetParamStore(app.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramskeeper.ConsensusParamsKeyTable()))

	app.UpgradeKeeper = upgradekeeper.NewKeeper(skipUpgradeHeights, app.keys[upgradetypes.StoreKey], appCodec, homePath, app.BaseApp)
	app.StakingKeeper = expectedkeepers.CubeStakingKeeper{}
}

func (app *CosmosApp) setupMockModule(ibcRouter *porttypes.Router) {
	scopedIBCMockKeeper := app.CapabilityKeeper.ScopeToModule(ibcmock.ModuleName)

	// Mock Module Stack
	// todo: Mock Module setup for testing IBC and also acts as the interchain accounts authentication module
	app.mockModule = ibcmock.NewAppModule(&app.IBCKeeper.PortKeeper)

	mockIBCModule := ibcmock.NewIBCModule(&app.mockModule, ibcmock.NewMockIBCApp(ibcmock.ModuleName, scopedIBCMockKeeper))
	ibcRouter.AddRoute(ibcmock.ModuleName, mockIBCModule)
}

func (app *CosmosApp) setupICAKeepers(ibcRouter *porttypes.Router) {
	appCodec := app.codec.Marshaler

	// add capability keeper and ScopeToModule for ibc module
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(appCodec, app.keys[capabilitytypes.StoreKey], app.memKeys[capabilitytypes.MemStoreKey])

	// Create the scoped keepers for each submodule keeper and authentication keeper
	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibchost.ModuleName)
	scopedICAControllerKeeper := app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	scopedICAHostKeeper := app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	scopedICAMockKeeper := app.CapabilityKeeper.ScopeToModule(ibcmock.ModuleName + icacontrollertypes.SubModuleName)
	//scopedICAAuthKeeper := app.CapabilityKeeper.ScopeToModule(icaauthtypes.ModuleName)

	// SDK module keepers
	app.AccountKeeper = expectedkeepers.CubeAccountKeeper{}
	// authkeeper.NewAccountKeeper(
	//	appCodec, app.keys[authtypes.StoreKey], app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, maccPerms,
	//)

	// IBC Keepers
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec, app.keys[ibchost.StoreKey], app.GetSubspace(ibchost.ModuleName), app.StakingKeeper, app.UpgradeKeeper, scopedIBCKeeper,
	)

	// Create the Keeper for each submodule
	// ICA Controller keeper
	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		appCodec, app.keys[icacontrollertypes.StoreKey], app.GetSubspace(icacontrollertypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper, // todo: may be replaced with middleware such as ics29 fee
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		scopedICAControllerKeeper, app.MsgServiceRouter(),
	)
	// ICA Host keeper
	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec, app.keys[icahosttypes.StoreKey], app.GetSubspace(icahosttypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		app.AccountKeeper, scopedICAHostKeeper, app.MsgServiceRouter(),
	)

	//// todo: Create your Interchain Accounts authentication module
	//app.ICAAuthKeeper = icaauthkeeper.NewKeeper(appCodec, keys[icaauthtypes.StoreKey], app.ICAControllerKeeper, scopedICAAuthKeeper)

	// Create Interchain Accounts Stack
	// SendPacket, since it is originating from the application to core IBC:
	// icaAuthModuleKeeper.SendTx -> icaController.SendPacket -> fee.SendPacket -> channel.SendPacket

	// initialize ICA module with mock module as the authentication module on the controller side
	var icaControllerStack porttypes.IBCModule
	icaControllerStack = ibcmock.NewIBCModule(&app.mockModule, ibcmock.NewMockIBCApp("", scopedICAMockKeeper))
	//app.ICAAuthModule = icaControllerStack.(ibcmock.IBCModule)
	icaControllerStack = icacontroller.NewIBCMiddleware(icaControllerStack, app.ICAControllerKeeper)
	icaControllerStack = ibcfee.NewIBCMiddleware(icaControllerStack, app.IBCFeeKeeper)

	// RecvPacket, message that originates from core IBC and goes down to app, the flow is:
	// channel.RecvPacket -> fee.OnRecvPacket -> icaHost.OnRecvPacket

	var icaHostStack porttypes.IBCModule
	icaHostStack = icahost.NewIBCModule(app.ICAHostKeeper)
	icaHostStack = ibcfee.NewIBCMiddleware(icaHostStack, app.IBCFeeKeeper)

	// todo: ICA auth AppModule
	//icaAuthModule := icaauth.NewAppModule(appCodec, app.ICAAuthKeeper)
	//// ICA auth IBC Module
	//icaAuthIBCModule := icaauth.NewIBCModule(app.ICAAuthKeeper)
	// Create host and controller IBC Modules as desired
	//icaControllerIBCModule := icacontroller.NewIBCModule(app.ICAControllerKeeper, icaAuthIBCModule)
	//icaHostIBCModule := icahost.NewIBCModule(app.ICAHostKeeper)

	// Add host, controller & ica auth modules to IBC router
	ibcRouter.
		// the ICA Controller middleware needs to be explicitly added to the IBC Router because the
		// ICA controller module owns the port capability for ICA. The ICA authentication module
		// owns the channel capability.
		AddRoute(icacontrollertypes.SubModuleName, icaControllerStack).
		AddRoute(icahosttypes.SubModuleName, icaHostStack).
		AddRoute(ibcmock.ModuleName+icacontrollertypes.SubModuleName, icaControllerStack) // ica with mock auth module stack route to ica (top level of middleware stack)
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

// GetSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *CosmosApp) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey sdk.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	//paramsKeeper.Subspace(banktypes.ModuleName)
	//paramsKeeper.Subspace(stakingtypes.ModuleName)
	//paramsKeeper.Subspace(minttypes.ModuleName)
	//paramsKeeper.Subspace(distrtypes.ModuleName)
	//paramsKeeper.Subspace(slashingtypes.ModuleName)
	//paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable())
	//paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)

	return paramsKeeper
}
