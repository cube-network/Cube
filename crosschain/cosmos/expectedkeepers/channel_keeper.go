package expectedkeepers

// for apps/27-interchain-accounts
// ChannelKeeper defines the expected IBC channel keeper
type CubeChannelKeeper struct {

	//GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool)
	//GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool)
	//GetConnection(ctx sdk.Context, connectionID string) (ibcexported.ConnectionI, error)
}
