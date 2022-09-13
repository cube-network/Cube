package expectedkeepers

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ct "github.com/tendermint/tendermint/types"
)

type HeaderFn func(height int64) *ct.Header

// for core/02-client
// keeper of the staking store
type CubeStakingKeeper struct {
	Stub     int
	HeaderFn HeaderFn
}

// todo: to be implemented
func (c CubeStakingKeeper) GetHistoricalInfo(ctx sdk.Context, height int64) (stakingtypes.HistoricalInfo, bool) {
	header := c.HeaderFn(height)
	// TODO header is nil
	return stakingtypes.HistoricalInfo{Header: *header.ToProto()}, true
}

// todo: to be implemented
func (c CubeStakingKeeper) UnbondingTime(ctx sdk.Context) time.Duration {
	return time.Hour * 24 * 14
}
