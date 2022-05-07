package chaos

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/core/rawdb"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/chaos/systemcontract"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	attestationThresholdNumerator   = 2
	attestationThresholdDenominator = 3

	maxOldBlockToAttest = 4
)

var (
	doubleSignIdentity = common.HexToAddress("0xfffffffffffffffffffffffffffffffffffffffe")
	uint256Max, _      = new(big.Int).SetString("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 0)

	// event ExecutedDoubleSignPunish(address indexed plaintiff, address indexed defendant, uint8 punishType value);
	executedDoubleSignPunishTopic = common.HexToHash("0x1874a4becdbc3c81b2409d3af931b783d9f5a7c77cb4a75fb2986b452b447688") // TODO
)

// VerifyAttestation checks whether an attestation is valid,
// and if it's valid, return the signer,
// and a threshold that indicates how many attestations can justify a block.
func (c *Chaos) VerifyAttestation(chain consensus.ChainHeaderReader, a *types.Attestation) (common.Address, int, error) {
	header := chain.GetHeader(a.TargetRangeEdge.Hash, a.TargetRangeEdge.Number.Uint64())
	if header == nil {
		return common.Address{}, 0, errUnknownBlock
	}
	// verify signature
	signer, err := a.RecoverSigner()
	if err != nil {
		return common.Address{}, 0, err
	}
	// check if it is an authorized validator?
	snap, err := c.snapshot(chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return common.Address{}, 0, err
	}

	if !snap.IsAuthorized(signer) {
		return common.Address{}, 0, errIsNotValidator
	}
	return signer, attestationThreshold(snap.Len()), nil
}

func attestationThreshold(valsCnt int) int {
	return valsCnt*attestationThresholdNumerator/attestationThresholdDenominator + 1
}

func (c *Chaos) CurrentValidator() common.Address {
	return c.validator
}

func (c *Chaos) MaxValidators() uint8 {
	return systemcontract.TopValidatorNum
}

func (c *Chaos) Attest(chain consensus.ChainHeaderReader, headerNum *big.Int, source, target *types.RangeEdge) (*types.Attestation, error) {
	if !c.IsReadyAttest() {
		return nil, errIsNotReadyAttest
	}
	if !c.IsAuthorizedAtHeight(chain, c.validator, target.Number.Uint64()) {
		return nil, errIsNotAuthorizedAtHeight
	}
	return c.makeNewAttestation(source, target)
}

// keccak256(abi.encode(s,t,h(s),h(t)) , where s is the hash of the last justified block,
//t is the hash of the current block to vote, and h(s) h(T) are the corresponding block numbers respectively.
func (c *Chaos) makeNewAttestation(sourceRangeEdge *types.RangeEdge, targetRangeEdge *types.RangeEdge) (*types.Attestation, error) {
	// because the sign function is `Wallet.SignData`，so we should pass the data to it, not the hash.
	sig, err := c.signFn(accounts.Account{Address: c.validator}, "", types.AttestationData(sourceRangeEdge, targetRangeEdge))
	if err != nil {
		return nil, errSignFailed
	}
	return types.NewAttestation(sourceRangeEdge, targetRangeEdge, sig), nil
}

func (c *Chaos) IsAuthorizedAtHeightAndHash(chain consensus.ChainHeaderReader, val common.Address, hash common.Hash, number uint64) bool {
	h := chain.GetHeader(hash, number)
	return c.IsAuthorizedByHeader(chain, val, h)
}

func (c *Chaos) IsAuthorizedAtHeight(chain consensus.ChainHeaderReader, val common.Address, height uint64) bool {
	h := chain.GetHeaderByNumber(height)
	return c.IsAuthorizedByHeader(chain, val, h)
}

func (c *Chaos) IsAuthorizedByHeader(chain consensus.ChainHeaderReader, val common.Address, h *types.Header) bool {
	if h == nil {
		log.Error("can not find the header when attesting")
		return false
	}
	height := h.Number.Uint64()
	snap, err := c.snapshot(chain, height, h.Hash(), nil)
	if err != nil {
		log.Error("get snapshot failed when attesting", "height", height, "err", err)
		return false
	}
	// if not an authorized validator for that block, skip it
	return snap.IsAuthorized(val)
}

func (c *Chaos) CurrentNeedHandleHeight(headerNum uint64) (uint64, error) {
	// Witness voting is postponed for two heights(config.AttestationDelay).
	if headerNum <= c.config.AttestationDelay {
		// WaterdropBlock must be greater than AttestationDelay
		return 0, errors.New("execution height not reached")
	}
	return headerNum - c.config.AttestationDelay, nil
}

func (c *Chaos) AttestationDelay() uint64 {
	return c.config.AttestationDelay
}

func (c *Chaos) IsReadyAttest() bool {
	return c.isReady && c.attestationStatus == types.AttestationStart
}

func (c *Chaos) AttestationThreshold(chain consensus.ChainHeaderReader, hash common.Hash, number uint64) (int, error) {
	header := chain.GetHeader(hash, number)
	if header == nil {
		return 0, errUnknownBlock
	}
	snap, err := c.snapshot(chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return 0, err
	}
	return attestationThreshold(snap.Len()), nil
}

func (c *Chaos) Validators(chain consensus.ChainHeaderReader, hash common.Hash, number uint64) ([]common.Address, error) {
	header := chain.GetHeader(hash, number)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := c.snapshot(chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.validators(), nil
}

// VerifyCasperFFGRule Judge whether there is a violation of the rules before the two proofs according to the CasperFFG rules
// Duplicate data has been filtered out before arriving here
// No matter the same branch or different branches, the same height can only be cast once
// For fully inclusive relationships, penalties will be imposed
// Other cross relationships or coupling relationships will not be violated
func (c *Chaos) VerifyCasperFFGRule(beforeSourceNum uint64, beforeTargetNum uint64, afterSourceNum uint64, afterTargetNum uint64) int {
	if beforeTargetNum == afterTargetNum {
		return types.PunishMultiSig
	} else if (beforeSourceNum < afterSourceNum && beforeTargetNum > afterTargetNum) ||
		(afterSourceNum < beforeSourceNum && afterTargetNum > beforeTargetNum) {
		return types.PunishInclusive
	}
	return types.PunishNone
}

// Assembly of penalty transactions in violation of CasperFFG rules
func (c *Chaos) executeDoubleSignPunish(chain consensus.ChainHeaderReader, header *types.Header,
	state *state.StateDB, p *types.ViolateCasperFFGPunish, totalTxIndex int) (*types.Transaction, *types.Receipt, error) {
	if c.signTxFn == nil {
		return nil, nil, errors.New("signTxFn not set")
	}

	p.PunishAddr = system.StakingContract
	p.Plaintiff = c.validator
	signer, err := p.RecoverSigner()
	if err != nil {
		return nil, nil, err
	}
	p.Defendant = signer

	pRLP, err := rlp.EncodeToBytes(p)
	if err != nil {
		return nil, nil, err
	}
	copy(p.Data, pRLP)
	//make system governance transaction
	nonce := state.GetNonce(c.validator)

	// Special to address for filtering transactions
	tx := types.NewTransaction(nonce, doubleSignIdentity, uint256Max, 0, common.Big0, pRLP)
	tx, err = c.signTxFn(accounts.Account{Address: c.validator}, tx, chain.Config().ChainID)
	if err != nil {
		return nil, nil, err
	}

	//add nonce for validator
	state.SetNonce(c.validator, nonce+1)
	receipt, err := c.executeDoubleSignPunishMsg(chain, header, state, p, totalTxIndex, tx.Hash(), common.Hash{})

	return tx, receipt, err
}

// After receiving a block containing multiple signed penalty transactions, execute the penalty transactions in it.
// If the execution fails, discard the whole block. BAD BLOCK
func (c *Chaos) replayDoubleSignPunish(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, totalTxIndex int, tx *types.Transaction) (*types.Receipt, error) {
	log.Debug("replayDoubleSignPunish", "Number", header.Number.Uint64())
	sender, err := types.Sender(c.signer, tx)
	if err != nil {
		return nil, err
	}
	if sender != header.Coinbase {
		return nil, errors.New("invalid sender for system transaction")
	}
	var p types.ViolateCasperFFGPunish
	if err := rlp.DecodeBytes(tx.Data(), &p); err != nil {
		return nil, err
	}
	// Clear your own records at the first time after receiving them to avoid data error accumulation
	rawdb.DeleteViolateCasperFFGPunish(c.db, &p)
	copy(p.Data, tx.Data())
	if b, err := c.IsDoubleSignPunished(chain, header, state, p.Hash()); err != nil || b {
		return nil, errors.New("is double sign punished")
	}
	// Verify that the data is valid
	signer, err := p.RecoverSigner()
	if err != nil {
		return nil, err
	}
	if signer != p.Defendant {
		return nil, errors.New("transaction signature does not match")
	}
	if types.PunishNone == c.VerifyCasperFFGRule(p.Before.SourceRangeEdge.Number.Uint64(), p.Before.TargetRangeEdge.Number.Uint64(),
		p.After.SourceRangeEdge.Number.Uint64(), p.After.TargetRangeEdge.Number.Uint64()) {
		return nil, errors.New("the transaction did not violate CasperFFG rules")
	}
	nonce := state.GetNonce(sender)
	//add nonce for validator
	state.SetNonce(sender, nonce+1)
	return c.executeDoubleSignPunishMsg(chain, header, state, &p, totalTxIndex, tx.Hash(), header.Hash())
}

// IsDoubleSignPunished Execute the query of punishment contract to judge whether the punishment hash of the current query has been punished
func (c *Chaos) IsDoubleSignPunished(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, punishHash common.Hash) (bool, error) {
	// isDoubleSignPunished(bytes32 punishHash) public view returns (bool)
	return systemcontract.IsDoubleSignPunished(&systemcontract.CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: newChainContext(chain, c),
		ChainConfig:  c.chainConfig,
	}, punishHash)
}

// Execute multi sign penalty transaction in EVM
func (c *Chaos) executeDoubleSignPunishMsg(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, p *types.ViolateCasperFFGPunish, totalTxIndex int, txHash, bHash common.Hash) (*types.Receipt, error) {
	var receipt *types.Receipt

	state.Prepare(txHash, totalTxIndex)
	pLog := &types.Log{
		Address:     system.StakingContract,
		Topics:      []common.Hash{executedDoubleSignPunishTopic, p.Plaintiff.Hash(), p.Defendant.Hash(), common.BigToHash(p.PunishType)},
		Data:        p.Data,
		BlockNumber: header.Number.Uint64(),
	}
	state.AddLog(pLog)

	// must succeed
	err := systemcontract.DoubleSignPunish(&systemcontract.CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: newChainContext(chain, c),
		ChainConfig:  c.chainConfig,
	}, p.Hash(), p.Defendant)
	if err != nil {
		return nil, err
	}

	receipt = types.NewReceipt([]byte{}, err != nil, header.GasUsed)
	log.Info("executeDoubleSignPunishMsg", "Plaintiff", p.Plaintiff, "Defendant", p.Defendant, "pushHash", p.Hash().String(), "success", true)

	receipt.Logs = state.GetLogs(txHash, bHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	receipt.TxHash = txHash
	receipt.BlockHash = bHash
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(state.TxIndex())
	return receipt, nil
}

// IsDoubleSignPunishTransaction Judge whether the transaction is a multi sign penalty transaction.
// Due to the particularity of transaction data, a special to address is used to distinguish
func (c *Chaos) IsDoubleSignPunishTransaction(sender common.Address, tx *types.Transaction, header *types.Header) (bool, error) {
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false, nil
	}
	to := tx.To()
	if sender == header.Coinbase &&
		*to == doubleSignIdentity &&
		tx.Value().Cmp(uint256Max) == 0 &&
		tx.Gas() == 0 &&
		tx.GasPrice().Sign() == 0 {
		return true, nil
	}
	return false, nil
}

// ApplyDoubleSignPunishTx TODO
func (c *Chaos) ApplyDoubleSignPunishTx(evm *vm.EVM, sender common.Address, tx *types.Transaction) (ret []byte, vmerr error, err error) {
	p := &types.ViolateCasperFFGPunish{}
	if err = rlp.DecodeBytes(tx.Data(), p); err != nil {
		return
	}
	nonce := evm.StateDB.GetNonce(sender)
	//add nonce for validator
	evm.StateDB.SetNonce(sender, nonce+1)
	evm.TxContext = vm.TxContext{
		Origin:   p.Plaintiff,
		GasPrice: new(big.Int).Set(big.NewInt(0)),
	}
	err = systemcontract.DoubleSignPunishGivenEVM(evm, p.Plaintiff, p.Hash(), p.Defendant)
	return nil, nil, err
}
