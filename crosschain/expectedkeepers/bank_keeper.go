package expectedkeepers

import (
	"encoding/hex"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/systemcontract"
	"math/big"
)

// StateFn gets state by the state root hash.
type StateFn func(hash common.Hash) (*state.StateDB, error)
type CurrentHeaderFn func() *types.Header

// BankKeeper defines the expected bank keeper
type CubeBankKeeper struct {
	moduleAccs map[string]sdk.AccAddress
	mintAcc    sdk.AccAddress

	// list of addresses that are restricted from receiving transactions
	blockedAddrs map[string]bool

	stateFn           StateFn // Function to get state by state root
	stateFnResult     bool
	currentHeaderFn   CurrentHeaderFn // Function to get current header
	curHeaderFnResult bool
	ctx               *systemcontract.BankContext
}

func NewBankKeeper(moduleAccs map[string]sdk.AccAddress, mintAcc sdk.AccAddress, blockedAddrs map[string]bool) *CubeBankKeeper {
	c := &CubeBankKeeper{
		moduleAccs:   moduleAccs,
		mintAcc:      mintAcc,
		blockedAddrs: blockedAddrs,
		ctx:          &systemcontract.BankContext{},
	}
	return c
}

func (cbk *CubeBankKeeper) SetStateFn(fn StateFn) {
	cbk.stateFn = fn
	cbk.stateFnResult = true
}

func (cbk *CubeBankKeeper) SetCurrentHeaderFn(fn CurrentHeaderFn) {
	cbk.currentHeaderFn = fn
	cbk.curHeaderFnResult = true
}

func (cbk *CubeBankKeeper) HasBalance(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Coin) bool {
	println("HasBalance addr ", addr.String(), " ", amt.String())
	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return false
	}
	balance, err := systemcontract.GetBalance(cbk.ctx, addr, amt.String())
	if err != nil {
		println("Failed to perform HasBalance", "coin", amt.String(), "err", err)
		return false
	}
	return balance.Int64() > 0
}

func (cbk *CubeBankKeeper) updateContext(evm *vm.EVM) error {
	header := cbk.currentHeaderFn()
	statedb, err := cbk.stateFn(header.Root)
	if err != nil {
		println("updateContext failed ", "height", header.Number, " ", err)
		return err
	}
	cbk.ctx.Header = header
	cbk.ctx.Statedb = statedb
	cbk.ctx.Evm = evm

	return nil
}

func (cbk *CubeBankKeeper) SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", " ", senderAddr.String(), " ", amt.String())
	recipientAcc := cbk.moduleAccs[recipientModule]
	if recipientAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", recipientModule)
		return fmt.Errorf("SendCoinsFromAccountToModule failed as module account %s does not exist", recipientModule)
	}
	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return err
	}
	if _, err := systemcontract.SendCoin(cbk.ctx, senderAddr, recipientAcc, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}

func (cbk *CubeBankKeeper) SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoinsFromModuleToAccount ", senderModule, " ", hex.EncodeToString(recipientAddr[2:]), " ", amt.String())
	senderAcc := cbk.moduleAccs[senderModule]
	if senderAcc.Empty() {
		println("Failed to perform SendCoin", "coin", amt.String(), "module account not exist", senderAcc)
		return fmt.Errorf("SendCoinsFromModuleToAccount failed as module account %s does not exist", senderAcc)
	}

	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return err
	}
	if _, err := systemcontract.SendCoin(cbk.ctx, senderAcc, recipientAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}
	return nil
}

func (cbk *CubeBankKeeper) SendCoins(ctx sdk.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	println("SendCoins fromAddr ", fromAddr.String(), " ", toAddr.String(), " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("send coins failed as no coin's info provided")
	}

	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return err
	}
	if _, err := systemcontract.SendCoin(cbk.ctx, fromAddr, toAddr, amt[0]); err != nil {
		println("Failed to perform SendCoin", "coin", amt.String(), "err", err)
		return err
	}

	balance := cbk.GetBalance(ctx, fromAddr, amt[0].Denom)
	println("Balance after SendCoins ", balance.Int64(), " fromAddr ", fromAddr.String())
	balance = cbk.GetBalance(ctx, toAddr, amt[0].Denom)
	println("Balance after SendCoins ", balance.Int64(), " toAddr ", toAddr.String())

	return nil
}

func (cbk *CubeBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	println("BlockedAddr ", addr.String())
	return cbk.blockedAddrs[addr.String()]
}

func (cbk *CubeBankKeeper) MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("MintCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("mint coins failed as no coin's info provided")
	}

	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return err
	}
	if _, err := systemcontract.MintCoin(cbk.ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform MintCoins", "coin", amt.String(), "err", err)
		return err
	}
	//if _, err := systemcontract.GetTokenInfo(ctx, amt[0].Denom); err != nil {
	//	println("Failed to perform GetTokenInfo", "coin", amt.String(), "err", err)
	//	return err
	//}

	balance := cbk.GetBalanceOfModuleAccount(ctx, moduleName, amt[0].Denom)
	println("Balance after MintCoins ", balance.Int64(), " account ", cbk.mintAcc.String())

	return nil
}

func (cbk *CubeBankKeeper) BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error {
	println("BurnCoins ", moduleName, " ", amt.String())
	if amt.Empty() {
		return fmt.Errorf("burn coins failed as no coin's info provided")
	}

	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return err
	}
	if _, err := systemcontract.BurnCoin(cbk.ctx, cbk.mintAcc, amt[0]); err != nil {
		println("Failed to perform BurnCoins", "coin", amt.String(), "err", err)
		return err
	}

	return nil
}

func (cbk *CubeBankKeeper) GetBalanceOfModuleAccount(ctx sdk.Context, moduleName string, denom string) *big.Int {
	senderAcc := cbk.moduleAccs[moduleName]
	if senderAcc.Empty() {
		println("Failed to perform SendCoin", "coin", denom, "module account not exist", senderAcc)
		return big.NewInt(0)
	}
	return cbk.GetBalance(ctx, senderAcc, denom)
}

func (cbk *CubeBankKeeper) GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) *big.Int {
	println("GetBalance ", denom, " ", addr.String())

	if err := cbk.updateContext(ctx.EVM()); err != nil {
		return big.NewInt(0)
	}

	balance, err := systemcontract.GetBalance(cbk.ctx, addr, denom)
	if err != nil {
		println("Failed to perform BurnCoins", "coin", denom, "err", err)
		return big.NewInt(0)
	}
	return balance
}
