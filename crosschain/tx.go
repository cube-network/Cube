package crosschain

import (
	"encoding/hex"
	"fmt"
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
	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
)

func (app *CosmosApp) RequiredGas(input []byte) uint64 {
	// TODO fixed gas cost for demo test
	return 20000
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

func (app *CosmosApp) Run(simulateMode bool, block_ctx vm.BlockContext, stdb vm.StateDB, input []byte) ([]byte, error) {
	if simulateMode {
		return nil, nil
	} else {
	}
	// TODO estimate gas ??
	_, arg, err := UnpackInput(input)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	msgs, err := app.GetMsgs(arg)
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
			msgResult, err := handler(app.GetContextForTx(simulateMode), msg) /*TODO statedb stateobject wrapper */
			eventMsgName := sdk.MsgTypeURL(msg)
			if err != nil {
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
		Address:     vm.CrossChainContractAddr,
		Topics:      topics,
		Data:        rdtxd,
		BlockNumber: block_ctx.BlockNumber.Uint64(),
	}
	stdb.AddLog(evLog)

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
			}
			dstchan, okdstchan := attributes[chant.AttributeKeyDstChannel]
			if okdstchan && event.Type == waTag {
				s, _ := strconv.Atoi(seq)
				keys := ackPacketQuery(dstchan, s)
				key := keys[0] + "/" + keys[1]
				app.db.Set([]byte(key)[:], rdtxd[:])
			}

		}
	}

	return data, nil
}

func (app *CosmosApp) GetMsgs(arg string) ([]sdk.Msg, error) {
	argbin, err := hex.DecodeString(arg)
	if err != nil {
		return nil, vm.ErrExecutionReverted
	}

	var body tx.TxBody
	err = app.codec.Marshaler.Unmarshal(argbin, &body)
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
