package crosschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v4/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	"github.com/cosmos/ibc-go/v4/modules/core/keeper"
)

type CubeAnteHandler struct {
	k *keeper.Keeper
}

func NewCubeAnteHandler(k *keeper.Keeper) *CubeAnteHandler {
	return &CubeAnteHandler{k: k}
}

// todo: be called when checking or executing a tx received from an IBC relayer
func (cah *CubeAnteHandler) checkTxMsgs(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
	redundancies := 0
	packetMsgs := 0
	for _, m := range tx.GetMsgs() {
		switch msg := m.(type) {
		case *channeltypes.MsgRecvPacket:
			response, err := cah.k.RecvPacket(sdk.WrapSDKContext(ctx), msg)
			if err != nil {
				return ctx, err
			}
			if response.Result == channeltypes.NOOP {
				redundancies += 1
			}
			packetMsgs += 1

		case *channeltypes.MsgAcknowledgement:
			response, err := cah.k.Acknowledgement(sdk.WrapSDKContext(ctx), msg)
			if err != nil {
				return ctx, err
			}
			if response.Result == channeltypes.NOOP {
				redundancies += 1
			}
			packetMsgs += 1

		case *channeltypes.MsgTimeout:
			response, err := cah.k.Timeout(sdk.WrapSDKContext(ctx), msg)
			if err != nil {
				return ctx, err
			}
			if response.Result == channeltypes.NOOP {
				redundancies += 1
			}
			packetMsgs += 1

		case *channeltypes.MsgTimeoutOnClose:
			response, err := cah.k.TimeoutOnClose(sdk.WrapSDKContext(ctx), msg)
			if err != nil {
				return ctx, err
			}
			if response.Result == channeltypes.NOOP {
				redundancies += 1
			}
			packetMsgs += 1

		case *clienttypes.MsgUpdateClient:
			_, err := cah.k.UpdateClient(sdk.WrapSDKContext(ctx), msg)
			if err != nil {
				return ctx, err
			}

		default:
			// if the multiMsg tx has a msg that is not a packet msg or update msg, then we will not return error
			// regardless of if all packet messages are redundant. This ensures that non-packet messages get processed
			// even if they get batched with redundant packet messages.
			return ctx, nil
		}
	}

	// only return error if all packet messages are redundant
	if redundancies == packetMsgs && packetMsgs > 0 {
		return ctx, channeltypes.ErrRedundantTx
	}
	return ctx, nil
}
