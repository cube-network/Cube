package crosschain

import (
	"encoding/hex"
	"fmt"
	"github.com/status-im/keycard-go/hexutils"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx"
	chant "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
)

func (app *CosmosApp) RequiredGas(input []byte) uint64 {
	// TODO fixed gas cost for demo test
	return 100000
}

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

func rcvPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_src_channel='%s'", spTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", spTag, seq)}
}

func ackPacketQuery(channelID string, seq int) []string {
	return []string{fmt.Sprintf("%s.packet_dst_channel='%s'", waTag, channelID),
		fmt.Sprintf("%s.packet_sequence='%d'", waTag, seq)}
}

func (app *CosmosApp) Run(simulateMode bool, evm *vm.EVM, input []byte) ([]byte, error) {
	if simulateMode {
		return nil, nil
	} else {
	}

	// app.bapp_mu.Lock()
	// defer app.bapp_mu.Unlock()

	// TODO estimate gas ??
	_, arg, err := UnpackInput(input)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	argbin, err := hex.DecodeString(arg)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}
	if string(argbin) == "InitCosmosGenesis" {
		app.InitGenesis(evm)
		txMsgData := &sdk.TxMsgData{}
		data, _ := proto.Marshal(txMsgData)
		return data, nil
	}

	// TODO check

	msgs, err := app.GetMsgs(argbin)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	msgLogs := make(sdk.ABCIMessageLogs, 0, len(msgs))
	events := sdk.EmptyEvents()
	txMsgData := &sdk.TxMsgData{
		Data: make([]*sdk.MsgData, 0, len(msgs)),
	}
	for i, msg := range msgs {
		if handler := app.MsgServiceRouter().Handler(msg); handler != nil {
			//log.Info("============Run Msg", "index", i, "msg", msg.String())
			msgResult, err := handler(app.GetContextForTx(simulateMode).WithEvm(evm), msg) /*TODO statedb stateobject wrapper */
			eventMsgName := sdk.MsgTypeURL(msg)
			if err != nil {
				log.Info("eventMsgName ", eventMsgName, "run tx err ", err.Error())
				return nil, vm.ErrExecutionReverted
			}

			msgEvents := sdk.Events{sdk.NewEvent(sdk.EventTypeMessage, sdk.NewAttribute(sdk.AttributeKeyAction, eventMsgName))}
			msgEvents = msgEvents.AppendEvents(msgResult.GetEvents())
			events = events.AppendEvents(msgEvents)

			log.Info("==========Handler Msg", "index", i, "name", eventMsgName, "result", msgResult.String())

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
		Address:     vm.CrossChainContractAddr,
		Topics:      topics,
		Data:        rdtxd,
		BlockNumber: evm.Context.BlockNumber.Uint64(),
	}
	evm.StateDB.AddLog(evLog)
	//log.Info("==========AddLog Run", "number", evLog.BlockNumber, "Address", evLog.Address.Hex(), "data", hexutils.BytesToHex(evLog.Data))
	log.Info("==========AddLog Run", "log", rdtx.Log, "data", hexutils.BytesToHex(rdtx.Data))
	abciEvents := events.ToABCIEvents()
	for i, event := range abciEvents {
		log.Info("==========AddLog Run", "index", i, "event", event.String())
	}

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
				app.db.Set([]byte(key)[:], rdtxd[:])
				println("write pkt ", key)
			}
			dstchan, okdstchan := attributes[chant.AttributeKeyDstChannel]
			if okdstchan && event.Type == waTag {
				s, _ := strconv.Atoi(seq)
				keys := ackPacketQuery(dstchan, s)
				key := keys[0] + "/" + keys[1]
				app.db.Set([]byte(key)[:], rdtxd[:])
				println("write pkt ", key)
			}
		}
	}

	return data, nil
}

func (app *CosmosApp) GetMsgs(argbin []byte) ([]sdk.Msg, error) {
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
