package cosmos

import (
	"encoding/hex"
	"fmt"

	"math/big"
	"strconv"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx"
	chant "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	cubetypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/cosmos/expectedkeepers"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	tenderminttypes "github.com/tendermint/tendermint/types"
)

var (
	state_block_number  = common.Hash{0x01, 0x01}
	state_app_hash_last = common.Hash{0x01, 0x02}
	state_app_hash_cur  = common.Hash{0x01, 0x04}
)

var (
	spTag       = "send_packet"
	waTag       = "write_acknowledgement"
	srcChanTag  = "packet_src_channel"
	dstChanTag  = "packet_dst_channel"
	srcPortTag  = "packet_src_port"
	dstPortTag  = "packet_dst_port"
	dataTag     = "packet_data"
	ackTag      = "packet_ack"
	toHeightTag = "packet_timeout_height"
	toTSTag     = "packet_timeout_timestamp"
	seqTag      = "packet_sequence"
)

type Executor struct {
	app *CosmosApp

	coinbase  common.Address
	chain     *CosmosChain
	queryMode bool

	db           *CosmosStateDB
	blockContext vm.BlockContext
	config       *params.ChainConfig
	codec        EncodingConfig
	header       *types.Header
	statedb      *state.StateDB
	// is_start_crosschain bool
}

func makeContext(blockContext vm.BlockContext, config *params.ChainConfig, header *types.Header, statedb *state.StateDB) *vm.EVM {
	blockContext.BlockNumber = header.Number
	blockContext.Time = new(big.Int).SetUint64(header.Time)
	blockContext.Difficulty = new(big.Int).Set(header.Difficulty)
	return vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, vm.Config{NoBaseFee: true})
}

func makeCosmosHeader(cubeheader *cubetypes.Header, config *params.ChainConfig) *tenderminttypes.Header {
	empty_hash := common.Hash{}
	header := &tenderminttypes.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            config.ChainID.String(),
		Height:             cubeheader.Number.Int64(),
		Time:               time.Unix(int64(cubeheader.Time), 0),
		LastCommitHash:     empty_hash[:],
		LastBlockID:        tenderminttypes.BlockID{},
		DataHash:           cubeheader.TxHash[:],
		ValidatorsHash:     empty_hash[:],
		NextValidatorsHash: empty_hash[:],
		ConsensusHash:      empty_hash[:],
		AppHash:            empty_hash[:],
		LastResultsHash:    empty_hash[:],
		EvidenceHash:       empty_hash[:],
		ProposerAddress:    empty_hash[:],
	}

	return header
}

func NewCosmosExecutor(datadir string,
	config *params.ChainConfig,
	codec EncodingConfig,
	blockFn expectedkeepers.BlockFn,
	blockContext vm.BlockContext,
	statedb *state.StateDB,
	header *types.Header,
	coinbase common.Address,
	chain *CosmosChain,
	queryMode bool) *Executor {

	db := NewCosmosStateDB(makeContext(blockContext, config, header, statedb))
	app := NewCosmosApp(datadir, db, config, codec, blockFn)

	executor := &Executor{app: app}
	executor.queryMode = queryMode
	executor.db = db
	executor.blockContext = blockContext
	executor.config = config
	executor.codec = codec
	executor.header = header
	executor.statedb = statedb
	executor.coinbase = coinbase
	executor.chain = chain
	return executor
}

func (c *Executor) IsCrossChainContract(addr common.Address) bool {
	return addr.String() == system.CrossChainCosmosContract.String()
}

func (c *Executor) RunCrossChainContract(evm *vm.EVM, input []byte, suppliedGas uint64) (ret []byte, remainingGas uint64, err error) {
	gasCost := c.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, vm.ErrOutOfGas
	}
	suppliedGas -= gasCost
	output, err := c.Run(evm, input)
	return output, suppliedGas, err
}

func (c *Executor) BeginBlock(header *types.Header, statedb *state.StateDB) {
	log.Debug("begin block height ", header.Number.Int64(), " root ", header.Root.Hex())
	log.Debug("begin block  state root ", statedb.IntermediateRoot(true).Hex())

	c.header = header
	c.statedb = statedb

	ctx := makeContext(c.blockContext, c.config, c.header, c.statedb)
	c.db.SetContext(ctx)

	if c.queryMode {
		c.Load(header.Number.Int64())
	} else {
		if header.Number.Cmp(c.config.CrosschainCosmosBlock) == 0 {
			c.InitGenesis(ctx)
		} else {
			c.Load(header.Number.Int64() - 1)
		}
	}

	hdr := makeCosmosHeader(header, c.config)
	c.app.BeginBlock(abci.RequestBeginBlock{Header: *hdr.ToProto()})
	log.Debug("begin block done state root ", statedb.IntermediateRoot(true).Hex())
	log.Debug("begin block done 2 state root ", c.db.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())
}

func (c *Executor) EndBlock() {
	log.Debug("end block  state root ", c.db.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())
	rc := c.app.BaseApp.Commit()
	// TODO hardfork cosmos block height
	// if c.header.Number.Int64() > 128 {
	// 	key := fmt.Sprintf("s/%d", c.header.Number.Int64()-128)
	// 	c.db.Delete([]byte(key))
	// }

	log.Debug("end block 2 state root ", c.db.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())

	copy(c.header.Extra[32:64], rc.Data[:])
	c.SetState(c.statedb, common.BytesToHash(rc.Data[:]), c.header.Number.Int64())
	// c.app.EndBlock(abci.RequestEndBlock{Height: c.header.Number.Int64()})

	c.chain.makeCosmosSignedHeader(c.header)
	log.Debug("end block done state root ", c.db.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())

	log.Debug("EndBlock ibc hash", hex.EncodeToString(rc.Data[:]))
}

func (c *Executor) SetState(statedb vm.StateDB, app_hash common.Hash, block_number int64) {
	statedb.SetNonce(system.CrossChainCosmosContract, statedb.GetNonce(system.CrossChainCosmosContract)+1)
	app_hash_last := statedb.GetState(system.CrossChainCosmosContract, state_app_hash_cur)
	statedb.SetState(system.CrossChainCosmosContract, state_app_hash_last, app_hash_last)
	statedb.SetState(system.CrossChainCosmosContract, state_app_hash_cur, app_hash)

	log.Debug("setstate ", app_hash_last.Hex(), " ", app_hash.Hex())

	cn := common.BigToHash(big.NewInt(block_number))
	statedb.SetState(system.CrossChainCosmosContract, state_block_number, cn)
}

func (c *Executor) InitGenesis(evm *vm.EVM) {
	log.Debug("init genesis state root ", evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())
	init_block_height := evm.Context.BlockNumber.Int64()
	c.SetState(evm.StateDB, common.Hash{}, init_block_height)

	// cosmos state contract
	log.Debug("init statedb with code/account")
	evm.StateDB.CreateAccount(system.CrossChainCosmosStateContract)
	code, _ := hex.DecodeString(StateContractCode)
	evm.StateDB.SetCode(system.CrossChainCosmosStateContract, code)

	// Module Account
	evm.StateDB.CreateAccount(common.HexToAddress(system.CrossChainCosmosModuleAccount))
	// deploy erc20 factory contract
	evm.StateDB.CreateAccount(system.ERC20FactoryContract)
	erc20code, _ := hex.DecodeString(ERC20FactoryCode)
	evm.StateDB.SetCode(system.ERC20FactoryContract, erc20code)

	// Addr2Pk
	evm.StateDB.CreateAccount(system.AddrToPubkeyMapContract)
	code, _ = hex.DecodeString(AddrToPubkeyMapCode)
	evm.StateDB.SetCode(system.AddrToPubkeyMapContract, code)

	// crosschain
	c.app.LoadVersion2(0)
	var genesisState GenesisState
	if err := tmjson.Unmarshal([]byte(IBCConfig), &genesisState); err != nil {
		panic(err)
	}

	c.app.InitChain(abci.RequestInitChain{Time: time.Time{}, ChainId: c.config.ChainID.String(), InitialHeight: init_block_height})
	c.app.mm.InitGenesis(c.app.GetContextForDeliverTx([]byte{}), c.codec.Marshaler, genesisState)

	// if c.coinbase == evm.Context.Coinbase {
	c.chain.valsMgr.initGenesisValidators(evm, init_block_height)
	// }

	// c.is_start_crosschain = true
	log.Debug("init genesis done state root ", evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex())
}

// TODO get cube block header instead
func (c *Executor) Load(init_block_height int64) {
	// if !c.is_start_crosschain {
	log.Debug("load version... ", init_block_height)
	c.app.LoadVersion2(init_block_height)
	// c.is_start_crosschain = true
	// }
}

func rcvPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_src_channel='%s'", spTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", spTag, seq)}
}

func ackPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_dst_channel='%s'", waTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", waTag, seq)}
}

func (app *Executor) RequiredGas(input []byte) uint64 {
	// TODO fixed gas cost, change later
	return 100000
}

func (c *Executor) Run(evm *vm.EVM, input []byte) ([]byte, error) {
	if evm.SimulateMode {
		return nil, nil
	} else {
	}

	// TODO estimate gas ??
	_, arg, err := UnpackInput(input)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	argbin, err := hex.DecodeString(arg)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	msgs, err := c.GetMsgs(argbin)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	msgLogs := make(sdk.ABCIMessageLogs, 0, len(msgs))
	events := sdk.EmptyEvents()
	txMsgData := &sdk.TxMsgData{
		Data: make([]*sdk.MsgData, 0, len(msgs)),
	}
	for i, msg := range msgs {
		if handler := c.app.MsgServiceRouter().Handler(msg); handler != nil {
			msgResult, err := handler(c.app.GetContextForTx(evm.SimulateMode).WithEvm(evm), msg) /*TODO statedb stateobject wrapper */
			eventMsgName := sdk.MsgTypeURL(msg)
			log.Debug("process tx ", eventMsgName)
			if err != nil {
				fmt.Println("process tx fail, eventMsgName ", eventMsgName, "run tx err ", err.Error())
				return nil, vm.ErrExecutionReverted
			}

			msgEvents := sdk.Events{sdk.NewEvent(sdk.EventTypeMessage, sdk.NewAttribute(sdk.AttributeKeyAction, eventMsgName))}
			msgEvents = msgEvents.AppendEvents(msgResult.GetEvents())
			events = events.AppendEvents(msgEvents)

			txMsgData.Data = append(txMsgData.Data, &sdk.MsgData{MsgType: sdk.MsgTypeURL(msg), Data: msgResult.Data})
			msgLogs = append(msgLogs, sdk.NewABCIMessageLog(uint32(i), msgResult.Log, msgEvents))
		} else {
			return nil, vm.ErrExecutionReverted
		}
	}

	data, err := proto.Marshal(txMsgData)
	if err != nil {
		return nil, sdkerrors.Wrap(err, "failed to marshal tx data")
	}

	rdtx := abci.ResponseDeliverTx{
		GasWanted: 0,
		GasUsed:   0,
		Log:       strings.TrimSpace(msgLogs.String()),
		Data:      data,
		Events:    sdk.MarkEventsToIndex(events.ToABCIEvents(), map[string]struct{}{}),
	}

	rdtxd, _ := rdtx.Marshal()

	// log
	topics := make([]common.Hash, 1)
	crypto.Keccak256Hash([]byte("submit(string,string)"))
	evLog := &types.Log{
		Address:     system.CrossChainCosmosContract,
		Topics:      topics,
		Data:        rdtxd,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	}
	evm.StateDB.AddLog(evLog)

	// index
	for _, event := range events {
		attributes := make(map[string]string)
		for _, attribute := range event.Attributes {
			attributes[string(attribute.Key)] = string(attribute.Value)
		}
		seq, ok := attributes[chant.AttributeKeySequence]
		if ok {
			srcchan, oksrcchan := attributes[chant.AttributeKeySrcChannel]
			if oksrcchan && event.Type == spTag {
				s, _ := strconv.Atoi(seq)
				keys := rcvPacketQuery(srcchan, s)
				key := keys[0] + "/" + keys[1]
				c.db.Set([]byte(key)[:], rdtxd[:])
				log.Debug("write pkt ", key)
			}
			dstchan, okdstchan := attributes[chant.AttributeKeyDstChannel]
			if okdstchan && event.Type == waTag {
				s, _ := strconv.Atoi(seq)
				keys := ackPacketQuery(dstchan, s)
				key := keys[0] + "/" + keys[1]
				c.db.Set([]byte(key)[:], rdtxd[:])
				log.Debug("write pkt ", key)
			}
		}
	}
	return data, nil
}

func (app *Executor) GetMsgs(argbin []byte) ([]sdk.Msg, error) {
	var body tx.TxBody
	err := app.codec.Marshaler.Unmarshal(argbin, &body)
	body.UnpackInterfaces(app.codec.InterfaceRegistry)
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
