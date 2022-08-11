package expectedkeepers

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ct "github.com/tendermint/tendermint/types"
)

type BlockFn func(height int64) *ct.LightBlock
type HistoricalInfo interface {
	GetLightBlock(block_height int64) *ct.LightBlock
}

// for core/02-client
// keeper of the staking store
type CubeStakingKeeper struct {
	Stub    int
	BlockFn BlockFn
}

// todo: to be implemented
func (c CubeStakingKeeper) GetHistoricalInfo(ctx sdk.Context, height int64) (stakingtypes.HistoricalInfo, bool) {
	lb := c.BlockFn(height)
	return stakingtypes.HistoricalInfo{Header: *lb.Header.ToProto()}, true
}

// todo: to be implemented
func (c CubeStakingKeeper) UnbondingTime(ctx sdk.Context) time.Duration {
	return time.Hour * 24 * 14
}
