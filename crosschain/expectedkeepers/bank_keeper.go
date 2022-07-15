package expectedkeepers

import (
	"encoding/hex"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/crosschain/systemcontract"
)

// BankKeeper defines the expected bank keeper
type CubeBankKeeper struct {
	moduleAccs map[string]sdk.AccAddress
	mintAcc    sdk.AccAddress

	// list of addresses that are restricted from receiving transactions
	blockedAddrs map[string]bool
}

func NewBankKeeper(moduleAccs map[string]sdk.AccAddress, mintAcc sdk.AccAddress, blockedAddrs map[string]bool) CubeBankKeeper {
	c := CubeBankKeeper{
		moduleAccs:   moduleAccs,
		mintAcc:      mintAcc,
		blockedAddrs: blockedAddrs,
	}
	return c
}

func (cbk CubeBankKeeper) HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool {
	println("HasBalance addr ", addr.String(), " ", amt.String())
	balance, err := systemcontract.GetBalance(ctx, addr, amt)
	if err != nil {
		println("Failed to perform HasBalance", "coin", amt.String(), "err", err)
		return false
	}
	return balance.Int64() > 0
}

func (cbk CubeBankKeeper) SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", " ", senderAddr.String(), " ", amt.String())
	recipientAcc := cbk.moduleAccs[recipientModule]
	if recipientAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", recipientModule)
		return fmt.Errorf("SendCoinsFromAccountToModule failed as module account %s does not exist", recipientModule)
	}
	if _, err := systemcontract.SendCoin(ctx, senderAddr, recipientAcc, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}

func (cbk CubeBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", senderModule, " ", hex.EncodeToString(recipientAddr[2:]), " ", amt.String())
	senderAcc := cbk.moduleAccs[senderModule]
	if senderAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", senderAcc)
		return fmt.Errorf("SendCoinsFromModuleToAccount failed as module account %s does not exist", senderAcc)
	}
	if _, err := systemcontract.SendCoin(ctx, senderAcc, recipientAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}
	return nil
}

func (cbk CubeBankKeeper) SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoins fromAddr ", fromAddr.String(), " ", toAddr.String(), " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("send coins failed as no coin's info provided")
	}
	if _, err := systemcontract.SendCoin(ctx, fromAddr, toAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}

func (cbk CubeBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	println("BlockedAddr ", addr.String())
	return cbk.blockedAddrs[addr.String()]
}

func (cbk CubeBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("MintCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("mint coins failed as no coin's info provided")
	}
	// call evm contract with ctx.EVM()
	if _, err := systemcontract.MintCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform MintCoins", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}

func (cbk CubeBankKeeper) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("BurnCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("burn coins failed as no coin's info provided")
	}
	if _, err := systemcontract.BurnCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform BurnCoins", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}
