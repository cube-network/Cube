package crosschain

//
//import (
//	"compress/gzip"
//	"encoding/json"
//	"fmt"
//	"github.com/cosmos/cosmos-sdk/client"
//	"github.com/cosmos/cosmos-sdk/version"
//	"github.com/cosmos/cosmos-sdk/x/auth"
//	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
//	"github.com/cosmos/cosmos-sdk/x/auth/tx"
//	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
//	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
//	"github.com/cosmos/cosmos-sdk/x/authz"
//	"github.com/cosmos/cosmos-sdk/x/bank"
//	"github.com/cosmos/cosmos-sdk/x/capability"
//	distr "github.com/cosmos/cosmos-sdk/x/distribution"
//	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
//	"github.com/cosmos/cosmos-sdk/x/feegrant"
//	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
//	"github.com/cosmos/cosmos-sdk/x/params"
//	"github.com/cosmos/cosmos-sdk/x/slashing"
//	"github.com/cosmos/cosmos-sdk/x/staking"
//	"github.com/cosmos/cosmos-sdk/x/upgrade"
//	ica "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts"
//	icacontrollerkeeper "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/controller/keeper"
//	icahostkeeper "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/host/keeper"
//	ibcfee "github.com/cosmos/ibc-go/v4/modules/apps/29-fee"
//	ibcfeekeeper "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/keeper"
//	"github.com/cosmos/ibc-go/v4/modules/apps/transfer"
//	ibc "github.com/cosmos/ibc-go/v4/modules/core"
//	porttypes "github.com/cosmos/ibc-go/v4/modules/core/05-port/types"
//	"github.com/ethereum/go-ethereum/core"
//	"github.com/ethereum/go-ethereum/core/types"
//	"github.com/ethereum/go-ethereum/eth/ethconfig"
//	"github.com/ethereum/go-ethereum/node"
//	"github.com/ethereum/go-ethereum/p2p"
//	"github.com/ethereum/go-ethereum/rlp"
//	"github.com/tendermint/tendermint/libs/log"
//	dbm "github.com/tendermint/tm-db"
//	"io"
//	"io/ioutil"
//	"os"
//	"path/filepath"
//	"strings"
//	"time"
//
//	"github.com/cosmos/cosmos-sdk/baseapp"
//	"github.com/cosmos/cosmos-sdk/codec"
//	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
//	sdk "github.com/cosmos/cosmos-sdk/types"
//	"github.com/cosmos/cosmos-sdk/types/module"
//	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
//	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
//	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
//	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
//	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
//	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
//	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
//	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
//	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
//	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
//	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
//	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
//	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
//	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
//	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
//	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
//	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
//	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
//	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
//	icacontrollertypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/controller/types"
//	icahosttypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/host/types"
//	icatypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/types"
//	ibcfeetypes "github.com/cosmos/ibc-go/v4/modules/apps/29-fee/types"
//	ibctransferkeeper "github.com/cosmos/ibc-go/v4/modules/apps/transfer/keeper"
//	ibctransfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
//	ibchost "github.com/cosmos/ibc-go/v4/modules/core/24-host"
//	ibckeeper "github.com/cosmos/ibc-go/v4/modules/core/keeper"
//	ibcmock "github.com/cosmos/ibc-go/v4/testing/mock"
//	"github.com/ethereum/go-ethereum/eth"
//)
//
//const appName = "CubeApp"
//
//var (
//	genesisFile   = "./cubetestdata/genesis.json"
//	halfchainFile = "./cubetestdata/halfchain.rlp"
//	fullchainFile = "./cubetestdata/chain.rlp"
//
//	// DefaultNodeHome default home directories for the application daemon
//	DefaultNodeHome string
//
//	//// module account permissions
//	//maccPerms = map[string][]string{
//	//	authtypes.FeeCollectorName:     nil,
//	//	distrtypes.ModuleName:          nil,
//	//	minttypes.ModuleName:           {authtypes.Minter},
//	//	stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
//	//	stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
//	//	govtypes.ModuleName:            {authtypes.Burner},
//	//	ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
//	//	ibcfeetypes.ModuleName:         nil,
//	//	icatypes.ModuleName:            nil,
//	//	ibcmock.ModuleName:             nil,
//	//}
//)
//
//type CubeApp struct {
//	eth *eth.Ethereum
//	*baseapp.BaseApp
//	ChainID string
//
//	// application's protocol version that increments on every upgrade
//	// if BaseApp is passed to the upgrade keeper's NewKeeper method.
//	appVersion uint64
//
//	legacyAmino       *codec.LegacyAmino
//	appCodec          codec.Codec
//	interfaceRegistry codectypes.InterfaceRegistry
//
//	// keepers
//	AccountKeeper authkeeper.AccountKeeper
//	BankKeeper    bankkeeper.Keeper
//
//	DistrKeeper    distrkeeper.Keeper
//	SlashingKeeper slashingkeeper.Keeper
//	StakingKeeper  stakingkeeper.Keeper
//
//	UpgradeKeeper upgradekeeper.Keeper
//	ParamsKeeper  paramskeeper.Keeper
//
//	CapabilityKeeper    *capabilitykeeper.Keeper
//	IBCKeeper           *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
//	IBCFeeKeeper        ibcfeekeeper.Keeper
//	ICAControllerKeeper icacontrollerkeeper.Keeper
//	ICAHostKeeper       icahostkeeper.Keeper
//	TransferKeeper      ibctransferkeeper.Keeper // for cross-chain fungible token transfers
//
//	// make scoped keepers public for test purposes
//	ScopedIBCKeeper capabilitykeeper.ScopedKeeper
//
//	// the module manager
//	mm *module.Manager
//
//	// simulation manager
//	sm *module.SimulationManager
//
//	// the configurator
//	configurator module.Configurator
//}
//
//func init() {
//	userHomeDir, err := os.UserHomeDir()
//	if err != nil {
//		panic(err)
//	}
//
//	DefaultNodeHome = filepath.Join(userHomeDir, ".cubeapp")
//}
//
//func NewCubeApp() *CubeApp {
//	legacyAmino := codec.NewLegacyAmino()
//	interfaceRegistry := codectypes.NewInterfaceRegistry()
//	appCodec := codec.NewProtoCodec(interfaceRegistry)
//
//	bApp := baseapp.NewBaseApp(appName, log.NewNopLogger(), dbm.NewMemDB(), tx.NewTxConfig(appCodec, tx.DefaultSignModes).TxDecoder())
//	//bApp.SetCommitMultiStoreTracer(traceStore)
//	bApp.SetVersion(version.Version)
//	bApp.SetInterfaceRegistry(interfaceRegistry)
//
//	app := &CubeApp{
//		BaseApp:           bApp,
//		legacyAmino:       legacyAmino,
//		appCodec:          appCodec,
//		interfaceRegistry: interfaceRegistry,
//	}
//
//	app.setupGeth()
//
//	app.setupKeeper()
//
//	app.setupRouterAndManagers()
//
//	return app
//}
//
//func (app *CubeApp) setupGeth() {
//	stack, err := node.New(&node.Config{
//		P2P: p2p.Config{
//			ListenAddr:  "127.0.0.1:0",
//			NoDiscovery: true,
//			MaxPeers:    10, // in case a test requires multiple connections, can be changed in the future
//			NoDial:      true,
//		},
//	})
//	if err != nil {
//		return
//	}
//
//	genesis, blocks, err := loadGenesisAndBlocks(halfchainFile, genesisFile)
//	if err != nil {
//		return
//	}
//
//	backend, err := eth.New(stack, &ethconfig.Config{
//		Genesis:                 &genesis,
//		NetworkId:               genesis.Config.ChainID.Uint64(), // 19763
//		DatabaseCache:           10,
//		TrieCleanCache:          10,
//		TrieCleanCacheJournal:   "",
//		TrieCleanCacheRejournal: 60 * time.Minute,
//		TrieDirtyCache:          16,
//		TrieTimeout:             60 * time.Minute,
//		SnapshotCache:           10,
//	})
//	if err != nil {
//		return
//	} else {
//		app.eth = backend
//		app.ChainID = genesis.Config.ChainID.String()
//	}
//
//	_, err = backend.BlockChain().InsertChain(blocks[1:])
//	return
//}
//
//func loadGenesisAndBlocks(chainfile string, genesis string) (core.Genesis, []*types.Block, error) {
//	gen, err := loadGenesis(genesis)
//	if err != nil {
//		return core.Genesis{}, nil, err
//	}
//	gblock := gen.ToBlock(nil)
//
//	blocks, err := blocksFromFile(chainfile, gblock)
//	if err != nil {
//		return core.Genesis{}, nil, err
//	}
//
//	return gen, blocks, nil
//}
//
//func loadGenesis(genesisFile string) (core.Genesis, error) {
//	chainConfig, err := ioutil.ReadFile(genesisFile)
//	if err != nil {
//		return core.Genesis{}, err
//	}
//	var gen core.Genesis
//	if err := json.Unmarshal(chainConfig, &gen); err != nil {
//		return core.Genesis{}, err
//	}
//	return gen, nil
//}
//
//func blocksFromFile(chainfile string, gblock *types.Block) ([]*types.Block, error) {
//	// Load chain.rlp.
//	fh, err := os.Open(chainfile)
//	if err != nil {
//		return nil, err
//	}
//	defer fh.Close()
//	var reader io.Reader = fh
//	if strings.HasSuffix(chainfile, ".gz") {
//		if reader, err = gzip.NewReader(reader); err != nil {
//			return nil, err
//		}
//	}
//	stream := rlp.NewStream(reader, 0)
//	var blocks = make([]*types.Block, 1)
//	blocks[0] = gblock
//	for i := 0; ; i++ {
//		var b types.Block
//		if err := stream.Decode(&b); err == io.EOF {
//			break
//		} else if err != nil {
//			return nil, fmt.Errorf("at block index %d: %v", i, err)
//		}
//		if b.NumberU64() != uint64(i+1) {
//			return nil, fmt.Errorf("block at index %d has wrong number %d", i, b.NumberU64())
//		}
//		blocks = append(blocks, &b)
//	}
//	return blocks, nil
//}
//
//func (app *CubeApp) setupKeeper() {
//	keys := sdk.NewKVStoreKeys(
//		ibchost.StoreKey, ibctransfertypes.StoreKey, capabilitytypes.StoreKey,
//		ibcfeetypes.StoreKey,
//	)
//	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
//	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
//
//	app.ParamsKeeper = initParamsKeeper(app.appCodec, app.legacyAmino, keys[paramstypes.StoreKey], tkeys[paramstypes.TStoreKey])
//	//// set the BaseApp's parameter store
//	//bApp.SetParamStore(app.ParamsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramskeeper.ConsensusParamsKeyTable()))
//
//	// add capability keeper and ScopeToModule for ibc module
//	app.AccountKeeper = authkeeper.NewAccountKeeper(
//		app.appCodec, keys[authtypes.StoreKey], app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, maccPerms,
//	)
//	app.BankKeeper = bankkeeper.NewBaseKeeper(
//		app.appCodec, keys[banktypes.StoreKey], app.AccountKeeper, app.GetSubspace(banktypes.ModuleName), app.ModuleAccountAddrs(),
//	)
//
//	// register the staking hooks
//	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
//	stakingKeeper := stakingkeeper.NewKeeper(
//		app.appCodec, keys[stakingtypes.StoreKey], app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName),
//	)
//	app.DistrKeeper = distrkeeper.NewKeeper(
//		app.appCodec, keys[distrtypes.StoreKey], app.GetSubspace(distrtypes.ModuleName), app.AccountKeeper, app.BankKeeper,
//		&stakingKeeper, authtypes.FeeCollectorName, app.ModuleAccountAddrs(),
//	)
//	app.SlashingKeeper = slashingkeeper.NewKeeper(
//		app.appCodec, keys[slashingtypes.StoreKey], &stakingKeeper, app.GetSubspace(slashingtypes.ModuleName),
//	)
//	app.StakingKeeper = *stakingKeeper.SetHooks(
//		stakingtypes.NewMultiStakingHooks(app.DistrKeeper.Hooks(), app.SlashingKeeper.Hooks()),
//	)
//
//	app.CapabilityKeeper = capabilitykeeper.NewKeeper(app.appCodec, keys[capabilitytypes.StoreKey], memKeys[capabilitytypes.MemStoreKey])
//
//	app.UpgradeKeeper = upgradekeeper.NewKeeper(make(map[int64]bool), keys[upgradetypes.StoreKey], app.appCodec, DefaultNodeHome, app) // todo: SetProtocolVersion()
//
//	// grant capabilities for the ibc and ibc-transfer modules
//	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibchost.ModuleName)
//	scopedTransferKeeper := app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
//	scopedICAControllerKeeper := app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
//	scopedICAHostKeeper := app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
//
//	app.ScopedIBCKeeper = scopedIBCKeeper
//
//	// Create IBC Keeper
//	app.IBCKeeper = ibckeeper.NewKeeper(
//		app.appCodec, keys[ibchost.StoreKey], app.GetSubspace(ibchost.ModuleName), app.StakingKeeper, app.UpgradeKeeper, scopedIBCKeeper,
//	)
//
//	// Create Transfer Keepers
//	app.TransferKeeper = ibctransferkeeper.NewKeeper(
//		app.appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
//		app.IBCKeeper.ChannelKeeper, app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
//		app.AccountKeeper, app.BankKeeper, scopedTransferKeeper,
//	)
//
//	// IBC Fee Module keeper
//	app.IBCFeeKeeper = ibcfeekeeper.NewKeeper(
//		app.appCodec, keys[ibcfeetypes.StoreKey], app.GetSubspace(ibcfeetypes.ModuleName),
//		app.IBCKeeper.ChannelKeeper, // may be replaced with IBC middleware
//		app.IBCKeeper.ChannelKeeper,
//		&app.IBCKeeper.PortKeeper, app.AccountKeeper, app.BankKeeper,
//	)
//
//	// ICA Controller keeper
//	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
//		app.appCodec, keys[icacontrollertypes.StoreKey], app.GetSubspace(icacontrollertypes.SubModuleName),
//		app.IBCFeeKeeper, // use ics29 fee as ics4Wrapper in middleware stack
//		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
//		scopedICAControllerKeeper, app.MsgServiceRouter(),
//	)
//
//	// ICA Host keeper
//	app.ICAHostKeeper = icahostkeeper.NewKeeper(
//		app.appCodec, keys[icahosttypes.StoreKey], app.GetSubspace(icahosttypes.SubModuleName),
//		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
//		app.AccountKeeper, scopedICAHostKeeper, app.MsgServiceRouter(),
//	)
//}
//
//func (app *CubeApp) setupRouterAndManagers() {
//	// Create static IBC router, add ibc-transfer module route
//
//	// Mock Module Stack
//
//	// Mock Module setup for testing IBC and also acts as the interchain accounts authentication module
//	// NOTE: the IBC mock keeper and application module is used only for testing core IBC. Do
//	// not replicate if you do not need to test core IBC or light clients.
//	mockModule := ibcmock.NewAppModule(&app.IBCKeeper.PortKeeper)
//
//	// The mock module is used for testing IBC
//	// NOTE: the IBC mock keeper and application module is used only for testing core IBC. Do
//	// not replicate if you do not need to test core IBC or light clients.
//	scopedIBCMockKeeper := app.CapabilityKeeper.ScopeToModule(ibcmock.ModuleName)
//	mockIBCModule := ibcmock.NewIBCModule(&mockModule, ibcmock.NewMockIBCApp(ibcmock.ModuleName, scopedIBCMockKeeper))
//	ibcRouter := porttypes.NewRouter()
//	ibcRouter.AddRoute(ibcmock.ModuleName, mockIBCModule) // todo: handle OnChainOpenInit, OnChainOpenTry, OnChainOpenAck, OnChainOpenConfirm...etc
//	// Setting Router will finalize all routes by sealing router
//	// No more routes can be added
//	app.IBCKeeper.SetRouter(ibcRouter)
//
//	// NOTE: Any module instantiated in the module manager that is later modified
//	// must be passed by reference here.
//	app.mm = module.NewManager(
//		// SDK app modules
//		//genutil.NewAppModule(
//		//	app.AccountKeeper, app.StakingKeeper, app.BaseApp.DeliverTx,
//		//	encodingConfig.TxConfig,
//		//),
//		auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts),
//		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
//		bank.NewAppModule(app.appCodec, app.BankKeeper, app.AccountKeeper),
//		capability.NewAppModule(app.appCodec, *app.CapabilityKeeper),
//		//crisis.NewAppModule(&app.CrisisKeeper, skipGenesisInvariants),
//		//feegrantmodule.NewAppModule(app.appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
//		//gov.NewAppModule(app.appCodec, app.GovKeeper, app.AccountKeeper, app.BankKeeper),
//		//mint.NewAppModule(app.appCodec, app.MintKeeper, app.AccountKeeper),
//		slashing.NewAppModule(app.appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
//		distr.NewAppModule(app.appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper),
//		staking.NewAppModule(app.appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper),
//		upgrade.NewAppModule(app.UpgradeKeeper),
//		//evidence.NewAppModule(app.EvidenceKeeper),
//		ibc.NewAppModule(app.IBCKeeper),
//		params.NewAppModule(app.ParamsKeeper),
//		//authzmodule.NewAppModule(app.appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
//
//		// IBC modules
//		transfer.NewAppModule(app.TransferKeeper),
//		ibcfee.NewAppModule(app.IBCFeeKeeper),
//		ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper),
//		mockModule,
//	)
//
//	// During begin block slashing happens after distr.BeginBlocker so that
//	// there is nothing left over in the validator fee pool, so as to keep the
//	// CanWithdrawInvariant invariant.
//	// NOTE: staking module is required if HistoricalEntries param > 0
//	// NOTE: capability module's beginblocker must come before any modules using capabilities (e.g. IBC)
//	app.mm.SetOrderBeginBlockers(
//		upgradetypes.ModuleName, capabilitytypes.ModuleName, minttypes.ModuleName, distrtypes.ModuleName, slashingtypes.ModuleName,
//		evidencetypes.ModuleName, stakingtypes.ModuleName, ibchost.ModuleName, ibctransfertypes.ModuleName, authtypes.ModuleName,
//		banktypes.ModuleName, govtypes.ModuleName, crisistypes.ModuleName, genutiltypes.ModuleName, authz.ModuleName, feegrant.ModuleName,
//		paramstypes.ModuleName, vestingtypes.ModuleName, icatypes.ModuleName, ibcfeetypes.ModuleName, ibcmock.ModuleName,
//	)
//	app.mm.SetOrderEndBlockers(
//		crisistypes.ModuleName, govtypes.ModuleName, stakingtypes.ModuleName, ibchost.ModuleName, ibctransfertypes.ModuleName,
//		capabilitytypes.ModuleName, authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName, slashingtypes.ModuleName,
//		minttypes.ModuleName, genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName, feegrant.ModuleName, paramstypes.ModuleName,
//		upgradetypes.ModuleName, vestingtypes.ModuleName, icatypes.ModuleName, ibcfeetypes.ModuleName, ibcmock.ModuleName,
//	)
//
//	// NOTE: The genutils module must occur after staking so that pools are
//	// properly initialized with tokens from genesis accounts.
//	// NOTE: Capability module must occur first so that it can initialize any capabilities
//	// so that other modules that want to create or claim capabilities afterwards in InitChain
//	// can do so safely.
//	app.mm.SetOrderInitGenesis(
//		capabilitytypes.ModuleName, authtypes.ModuleName, banktypes.ModuleName, distrtypes.ModuleName, stakingtypes.ModuleName,
//		slashingtypes.ModuleName, govtypes.ModuleName, minttypes.ModuleName, crisistypes.ModuleName,
//		ibchost.ModuleName, genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName, ibctransfertypes.ModuleName,
//		icatypes.ModuleName, ibcfeetypes.ModuleName, ibcmock.ModuleName, feegrant.ModuleName, paramstypes.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName,
//	)
//
//	app.mm.RegisterRoutes(app.Router(), app.QueryRouter(), app.legacyAmino)
//	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
//	app.mm.RegisterServices(app.configurator)
//}
//
//// GetSubspace returns a param subspace for a given module name.
////
//// NOTE: This is solely to be used for testing purposes.
//func (app *CubeApp) GetSubspace(moduleName string) paramstypes.Subspace {
//	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
//	return subspace
//}
//
//// ModuleAccountAddrs returns all the app's module account addresses.
//func (app *CubeApp) ModuleAccountAddrs() map[string]bool {
//	modAccAddrs := make(map[string]bool)
//	for acc := range maccPerms {
//		// do not add mock module to blocked addresses
//		// this is only used for testing
//		if acc == ibcmock.ModuleName {
//			continue
//		}
//
//		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
//	}
//
//	return modAccAddrs
//}
//
//// SetProtocolVersion sets the application's protocol version
//func (app *CubeApp) SetProtocolVersion(v uint64) {
//	app.appVersion = v
//}
//
//// TestingApp functions
//
//// GetBaseApp implements the TestingApp interface.
//func (app *CubeApp) GetBaseApp() *baseapp.BaseApp {
//	return app.BaseApp
//}
//
//// GetStakingKeeper implements the TestingApp interface.
//func (app *CubeApp) GetStakingKeeper() stakingkeeper.Keeper {
//	return app.StakingKeeper
//}
//
//// GetIBCKeeper implements the TestingApp interface.
//func (app *CubeApp) GetIBCKeeper() *ibckeeper.Keeper {
//	return app.IBCKeeper
//}
//
//// GetScopedIBCKeeper implements the TestingApp interface.
//func (app *CubeApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
//	return app.ScopedIBCKeeper
//}
//
//// GetTxConfig implements the TestingApp interface.
//func (app *CubeApp) GetTxConfig() client.TxConfig {
//	marshaler := codec.NewProtoCodec(app.interfaceRegistry)
//	return tx.NewTxConfig(marshaler, tx.DefaultSignModes)
//}
//
//// AppCodec returns CubeApp's app codec.
////
//// NOTE: This is solely to be used for testing purposes as it may be desirable
//// for modules to register their own custom testing types.
//func (app *CubeApp) AppCodec() codec.Codec {
//	return app.appCodec
//}
//
////// initParamsKeeper init params keeper and its subspaces
////func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey sdk.StoreKey) paramskeeper.Keeper {
////	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)
////
////	paramsKeeper.Subspace(authtypes.ModuleName)
////	//paramsKeeper.Subspace(banktypes.ModuleName)
////	//paramsKeeper.Subspace(stakingtypes.ModuleName)
////	//paramsKeeper.Subspace(minttypes.ModuleName)
////	//paramsKeeper.Subspace(distrtypes.ModuleName)
////	//paramsKeeper.Subspace(slashingtypes.ModuleName)
////	//paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable())
////	//paramsKeeper.Subspace(crisistypes.ModuleName)
////	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
////	paramsKeeper.Subspace(ibchost.ModuleName)
////	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
////	paramsKeeper.Subspace(icahosttypes.SubModuleName)
////
////	return paramsKeeper
////}
