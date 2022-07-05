package expectedkeepers

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// for core/02-client
// keeper of the staking store
type CubeStakingKeeper struct {
	Stub int
}

// todo: to be implemented
func (c CubeStakingKeeper) GetHistoricalInfo(ctx sdk.Context, height int64) (stakingtypes.HistoricalInfo, bool) {
	return stakingtypes.HistoricalInfo{}, true
}

// todo: to be implemented
func (c CubeStakingKeeper) UnbondingTime(ctx sdk.Context) time.Duration {
	return 0
}
