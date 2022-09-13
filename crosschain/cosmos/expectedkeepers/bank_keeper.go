package expectedkeepers

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/cosmos/systemcontract"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
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
	log.Debug("HasBalance addr ", addr.String(), " ", amt.String())
	balance, err := systemcontract.GetBalance(ctx, addr, amt)
	if err != nil {
		log.Debug("Failed to perform HasBalance", "coin", amt.String(), "err", err.Error())
		return false
	}
	return balance.Cmp(big.NewInt(0)) == 1
}

func (cbk CubeBankKeeper) SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	log.Debug("SendCoinsFromAccountToModule ", " ", senderAddr.String(), " ", amt.String())
	recipientAcc := cbk.moduleAccs[recipientModule]

	if recipientAcc.Empty() {
		log.Debug("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", recipientModule)
		return fmt.Errorf("SendCoinsFromAccountToModule failed as module account %s does not exist", recipientModule)
	}

	if err := cbk.checkBalance(ctx, senderAddr, amt); err != nil {
		log.Debug("SendCoinsFromAccountToModule failed", "coin", amt.String(), "err", err.Error())
		return err
	}
	// cbk.DumpCoins(ctx, senderAddr, recipientAcc, amt)
	if _, err := systemcontract.SendCoin(ctx, senderAddr, recipientAcc, amt[0]); err != nil {
		log.Debug("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
		return err
	}
	// cbk.DumpCoins(ctx, senderAddr, recipientAcc, amt)

	return nil
}

func (cbk CubeBankKeeper) checkBalance(ctx sdk.Context, senderAddr sdk.AccAddress, amt sdk.Coins) error {
	bal, err := systemcontract.GetBalance(ctx, senderAddr, amt[0])
	if err != nil {
		log.Debug("DumpCoins sender err ", err.Error(), " addr ", hex.EncodeToString(senderAddr.Bytes()))
		return err
	}
	coin := amt[0]
	balAmt := sdk.NewCoin(coin.Denom, sdk.NewIntFromBigInt(bal))

	_, hasNeg := sdk.Coins{balAmt}.SafeSub(amt)
	if hasNeg {
		return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, "%s is smaller than %s", balAmt, amt)
	}
	return nil
}

func (cbk CubeBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	log.Debug("SendCoinsFromModuleToAccount ", senderModule, " ", hex.EncodeToString(recipientAddr[2:]), " ", amt.String())
	senderAcc := cbk.moduleAccs[senderModule]

	if senderAcc.Empty() {
		log.Debug("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", senderAcc)
		return fmt.Errorf("SendCoinsFromModuleToAccount failed as module account %s does not exist", senderAcc)
	}
	if err := cbk.checkBalance(ctx, senderAcc, amt); err != nil {
		log.Debug("SendCoinsFromAccountToModule failed", "coin", amt.String(), "err", err.Error())
		return err
	}
	// cbk.DumpCoins(ctx, senderAcc, recipientAddr, amt)
	if _, err := systemcontract.SendCoin(ctx, senderAcc, recipientAddr, amt[0]); err != nil {
		log.Debug("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
		return err
	}
	// cbk.DumpCoins(ctx, senderAcc, recipientAddr, amt)

	return nil
}

func (cbk CubeBankKeeper) SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	if fromAddr == nil {
		fromAddr = cbk.moduleAccs["transfer"]
	}
	if toAddr == nil {
		toAddr = cbk.moduleAccs["transfer"]
	}
	log.Debug("SendCoins ", "Denom ", amt[0].Denom, "amt ", amt.String(), "from ", fromAddr, "to ", toAddr)
	isWei, _, amount := params.AmountAfterConvert(amt[0].Denom, amt[0].Amount.BigInt())
	if isWei {
		if !ctx.EVM().Context.CanTransfer(ctx.EVM().StateDB, common.BytesToAddress(fromAddr), amount) {
			return errors.New("insufficient balance")
		}
		ctx.EVM().Context.Transfer(ctx.EVM().StateDB, common.BytesToAddress(fromAddr), common.BytesToAddress(toAddr), amount)
		// Handle tracer events for entering and exiting a call frame
		gas := ctx.Gas()
		if ctx.EVM().Config.Debug {
			ctx.EVM().Config.Tracer.CaptureEnter(vm.CALL, common.BytesToAddress(fromAddr), common.BytesToAddress(toAddr), []byte{}, gas, amount)
			defer func(startGas uint64) {
				ctx.EVM().Config.Tracer.CaptureExit([]byte{}, 0, nil)
			}(gas)
		}
	} else {
		// cbk.DumpCoins(ctx, fromAddr, toAddr, amt)
		if _, err := systemcontract.SendCoin(ctx, fromAddr, toAddr, amt[0]); err != nil {
			log.Debug("Failed to perform SendCoin", "coin", amt.String(), "err", err.Error())
			return err
		}
		// cbk.DumpCoins(ctx, fromAddr, toAddr, amt)
	}

	return nil
}

func (cbk CubeBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	log.Debug("BlockedAddr ", addr.String())
	return cbk.blockedAddrs[addr.String()]
}

func (cbk CubeBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	log.Debug("MintCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("mint coins failed as no coin's info provided")
	}
	// cbk.DumpCoins(ctx, cbk.mintAcc, cbk.mintAcc, amt)
	if _, err := systemcontract.MintCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		log.Debug("Failed to perform MintCoins", "coin", amt.String(), "err", err.Error())
		return err
	}

	// cbk.DumpCoins(ctx, cbk.mintAcc, cbk.mintAcc, amt)

	return nil
}

func (cbk CubeBankKeeper) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	log.Debug("BurnCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("burn coins failed as no coin's info provided")
	}
	// cbk.DumpCoins(ctx, cbk.mintAcc, cbk.mintAcc, amt)
	if _, err := systemcontract.BurnCoin(ctx, cbk.mintAcc, amt[0]); err != nil {
		log.Debug("Failed to perform BurnCoins", "coin", amt.String(), "err", err.Error())
		return err
	}
	// cbk.DumpCoins(ctx, cbk.mintAcc, cbk.mintAcc, amt)
	return nil
}

func (cbk CubeBankKeeper) DumpCoins(ctx sdk.Context, sender, receiver sdk.AccAddress, amt sdk.Coins) {
	{
		sb, err := systemcontract.GetBalance(ctx, sender, amt[0])
		if err != nil {
			log.Debug("DumpCoins sender err ", err.Error(), " addr ", hex.EncodeToString(receiver.Bytes()))
		} else {
			log.Debug("sender bal  ", sb.String(), " addr ", hex.EncodeToString(sender.Bytes()))
		}
		rb, err := systemcontract.GetBalance(ctx, receiver, amt[0])
		if err != nil {
			log.Debug("DumpCoins receiver err ", err.Error(), " addr ", hex.EncodeToString(receiver.Bytes()))
		} else {
			log.Debug("receiver bal ", rb.String(), " addr ", hex.EncodeToString(receiver.Bytes()))
		}
	}
}
