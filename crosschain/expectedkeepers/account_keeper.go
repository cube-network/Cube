package expectedkeepers

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// for apps/27-interchain-accounts
type CubeAccountKeeper struct {
}

func (ack *CubeAccountKeeper) NewAccount(ctx sdk.Context, acc authtypes.AccountI) authtypes.AccountI {
	return nil
}

func (ack *CubeAccountKeeper) GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI {
	return nil
}

func (ack *CubeAccountKeeper) SetAccount(ctx sdk.Context, acc authtypes.AccountI) {

}

func (ack *CubeAccountKeeper) GetModuleAccount(ctx sdk.Context, name string) authtypes.ModuleAccountI {
	return nil
}

func (ack *CubeAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return nil
}
