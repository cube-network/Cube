package expectedkeepers

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// for core/02-client
// UpgradeKeeper expected upgrade keeper
type CubeUpgradeKeeper struct {
	Stub int
	//ClearIBCState(ctx sdk.Context, lastHeight int64)
	//GetUpgradePlan(ctx sdk.Context) (plan upgradetypes.Plan, havePlan bool)
	//GetUpgradedClient(ctx sdk.Context, height int64) ([]byte, bool)
	//SetUpgradedClient(ctx sdk.Context, planHeight int64, bz []byte) error
	//GetUpgradedConsensusState(ctx sdk.Context, lastHeight int64) ([]byte, bool)
	//SetUpgradedConsensusState(ctx sdk.Context, planHeight int64, bz []byte) error
	//ScheduleUpgrade(ctx sdk.Context, plan upgradetypes.Plan) error
}

// ClearIBCState clears any planned IBC state
func (k CubeUpgradeKeeper) ClearIBCState(ctx sdk.Context, lastHeight int64) {
	// delete IBC client and consensus state from store if this is IBC plan
}

// ClearUpgradePlan clears any schedule upgrade and associated IBC states.
func (k CubeUpgradeKeeper) ClearUpgradePlan(ctx sdk.Context) {
	// clear IBC states everytime upgrade plan is removed
}
