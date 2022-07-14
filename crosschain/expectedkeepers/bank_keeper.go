package expectedkeepers

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank keeper
type CubeBankKeeper struct {
}

func (cbk CubeBankKeeper) HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool {
	println("HasBalance addr ", addr.String(), " ", amt.String())
	return true
}

func (cbk CubeBankKeeper) SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", " ", senderAddr.String(), " ", amt.String())
	return nil
}

func (cbk CubeBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", senderModule, " ", hex.EncodeToString(recipientAddr[2:]), " ", amt.String())
	return nil
}

func (cbk CubeBankKeeper) SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoins fromAddr ", fromAddr.String(), " ", toAddr.String(), " ", amt.String())
	return nil
}

func (cbk CubeBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	println("BlockedAddr ", addr.String())
	return false
}

func (cbk CubeBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("MintCoins ", moduleName, " ", amt.String())
	return nil
}

func (cbk CubeBankKeeper) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("BurnCoins ", moduleName, " ", amt.String())
	return nil
}
