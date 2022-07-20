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
		println("Failed to perform HasBalance", "coin", amt.String(), "err", err.Error())
		return false
	}
	return balance.Int64() > 0
}

func (cbk CubeBankKeeper) SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	println("SendCoinsFromAccountToModule ", " ", senderAddr.String(), " ", amt.String())
	recipientAcc := cbk.moduleAccs[recipientModule]
	{
		sb, err := systemcontract.GetBalance(ctx, senderAddr, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", sb.Int64(), " addr ", hex.EncodeToString(senderAddr.Bytes()))
		}
		rb, err := systemcontract.GetBalance(ctx, recipientAcc, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", rb.Int64(), " addr ", hex.EncodeToString(recipientAcc.Bytes()))
		}
	}
	if recipientAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", recipientModule)
		return fmt.Errorf("SendCoinsFromAccountToModule failed as module account %s does not exist", recipientModule)
	}
	if _, err := systemcontract.SendCoin(ctx, senderAddr, recipientAcc, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
		return err
	}

	sb, err := systemcontract.GetBalance(ctx, senderAddr, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", sb.Int64(), " addr ", hex.EncodeToString(senderAddr.Bytes()))
	}
	rb, err := systemcontract.GetBalance(ctx, recipientAcc, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", rb.Int64(), " addr ", hex.EncodeToString(recipientAcc.Bytes()))
	}
	return nil
}

func (cbk CubeBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", senderModule, " ", hex.EncodeToString(recipientAddr[2:]), " ", amt.String())
	senderAcc := cbk.moduleAccs[senderModule]
	{

		sb, err := systemcontract.GetBalance(ctx, senderAcc, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", sb.Int64(), " addr ", hex.EncodeToString(senderAcc.Bytes()))
		}
		rb, err := systemcontract.GetBalance(ctx, recipientAddr, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", rb.Int64(), " addr ", hex.EncodeToString(recipientAddr.Bytes()))
		}
	}
	if senderAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", senderAcc)
		return fmt.Errorf("SendCoinsFromModuleToAccount failed as module account %s does not exist", senderAcc)
	}
	if _, err := systemcontract.SendCoin(ctx, senderAcc, recipientAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
		return err
	}

	sb, err := systemcontract.GetBalance(ctx, senderAcc, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", sb.Int64(), " addr ", hex.EncodeToString(senderAcc.Bytes()))
	}
	rb, err := systemcontract.GetBalance(ctx, recipientAddr, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", rb.Int64(), " addr ", hex.EncodeToString(recipientAddr.Bytes()))
	}
	return nil
}

func (cbk CubeBankKeeper) SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoins fromAddr ", fromAddr.String(), " ", toAddr.String(), " ", amt.String())
	{

		sb, err := systemcontract.GetBalance(ctx, fromAddr, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", sb.Int64(), " addr ", hex.EncodeToString(fromAddr.Bytes()))
		}
		rb, err := systemcontract.GetBalance(ctx, toAddr, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", rb.Int64(), " addr ", hex.EncodeToString(toAddr.Bytes()))
		}
	}
	if amt.Empty() {
		return fmt.Errorf("send coins failed as no coin's info provided")
	}
	if _, err := systemcontract.SendCoin(ctx, fromAddr, toAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
		return err
	}

	sb, err := systemcontract.GetBalance(ctx, fromAddr, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", sb.Int64(), " addr ", hex.EncodeToString(fromAddr.Bytes()))
	}
	rb, err := systemcontract.GetBalance(ctx, toAddr, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", rb.Int64(), " addr ", hex.EncodeToString(toAddr.Bytes()))
	}
	return nil
}

func (cbk CubeBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	println("BlockedAddr ", addr.String())
	return cbk.blockedAddrs[addr.String()]
}

func (cbk CubeBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("MintCoins ", moduleName, " ", amt.String())
	{

		sb, err := systemcontract.GetBalance(ctx, cbk.mintAcc, amt[0])

		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", sb.Int64(), " addr ", hex.EncodeToString(cbk.mintAcc.Bytes()))
		}

	}
	if amt.Empty() {
		return fmt.Errorf("mint coins failed as no coin's info provided")
	}
	// call evm contract with ctx.EVM()
	if _, err := systemcontract.MintCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform MintCoins", "coin", amt.String(), "err", err.Error())
		return err
	}

	sb, err := systemcontract.GetBalance(ctx, cbk.mintAcc, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", sb.Int64(), " addr ", hex.EncodeToString(cbk.mintAcc.Bytes()))
	}

	allbalances, err := systemcontract.GetAllBalances(ctx, cbk.mintAcc)
	if err != nil {
		println("err ", err.Error())
	} else {
		for token, balance := range allbalances {
			println("token ", token, " balance ", balance)
		}
	}

	return nil
}

func (cbk CubeBankKeeper) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("BurnCoins ", moduleName, " ", amt.String())
	{
		sb, err := systemcontract.GetBalance(ctx, cbk.mintAcc, amt[0])
		if err != nil {
			println("err ", err.Error())
		} else {
			println("sb ", sb.Int64(), " addr ", hex.EncodeToString(cbk.mintAcc.Bytes()))
		}
	}
	if amt.Empty() {
		return fmt.Errorf("burn coins failed as no coin's info provided")
	}
	if _, err := systemcontract.BurnCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform BurnCoins", "coin", amt.String(), "err", err.Error())
		return err
	}

	sb, err := systemcontract.GetBalance(ctx, cbk.mintAcc, amt[0])
	if err != nil {
		println("err ", err.Error())
	} else {
		println("sb ", sb.Int64(), " addr ", hex.EncodeToString(cbk.mintAcc.Bytes()))
	}

	return nil
}