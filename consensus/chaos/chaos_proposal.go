// Copyright 2021 The Cube Authors
// This file is part of the Cube library.
//
// The Cube library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Cube library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Cube library. If not, see <http://www.gnu.org/licenses/>.

package chaos

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/chaos/systemcontract"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

// processProposalTx process tx of system proposal
// Due to the logics of the finish operation of contract `OnChainDao`, when finishing a proposal which
// is not the last passed proposal, it will change the sequence. So in here we must first executes all
// passed proposals, and then finish then all.
func (c *Chaos) processProposalTx(chain consensus.ChainHeaderReader, header *types.Header,
	state *state.StateDB, txs *[]*types.Transaction, receipts *[]*types.Receipt, proposalTxs []*types.Transaction, mined bool) error {
	// Skip unauthorized validator mining
	if mined && c.signTxFn == nil {
		return nil
	}

	var (
		proposalCount uint32
		i             uint32
		err           error
	)

	if proposalCount, err = c.getPassedProposalCount(chain, header, state); err != nil {
		return err
	}

	if !mined && proposalCount != uint32(len(proposalTxs)) {
		return errInvalidProposalCount
	}

	pIds := make([]*big.Int, 0, proposalCount)
	for i = 0; i < proposalCount; i++ {
		var (
			prop    *systemcontract.Proposal
			tx      *types.Transaction
			receipt *types.Receipt
		)

		if prop, err = c.getPassedProposalByIndex(chain, header, state, i); err != nil {
			return err
		}
		// execute the system Proposal
		if !mined {
			tx = proposalTxs[int(i)]
			if receipt, err = c.replayProposal(chain, header, state, prop, len(*txs), tx); err != nil {
				return err
			}
		} else if tx, receipt, err = c.executeProposal(chain, header, state, prop, len(*txs)); err != nil {
			return err
		}
		*txs = append(*txs, tx)
		*receipts = append(*receipts, receipt)
		// set
		pIds = append(pIds, prop.Id)
	}
	// Finish all proposal
	for i = 0; i < proposalCount; i++ {
		err = c.finishProposalById(chain, header, state, pIds[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Chaos) getPassedProposalCount(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) (uint32, error) {
	return systemcontract.GetPassedProposalCount(&systemcontract.CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: newChainContext(chain, c),
		ChainConfig:  c.chainConfig,
	})
}

func (c *Chaos) getPassedProposalByIndex(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, idx uint32) (*systemcontract.Proposal, error) {
	return systemcontract.GetPassedProposalByIndex(&systemcontract.CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: newChainContext(chain, c),
		ChainConfig:  c.chainConfig,
	}, idx)
}

//finishProposalById
func (c *Chaos) finishProposalById(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, id *big.Int) error {
	return systemcontract.FinishProposalById(&systemcontract.CallContext{
		Statedb:      state,
		Header:       header,
		ChainContext: newChainContext(chain, c),
		ChainConfig:  c.chainConfig,
	}, id)
}

func (c *Chaos) executeProposal(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, prop *systemcontract.Proposal, totalTxIndex int) (*types.Transaction, *types.Receipt, error) {
	// Even if the miner is not `running`, it's still working,
	// the 'miner.worker' will try to FinalizeAndAssemble a block,
	// in this case, the signTxFn is not set. A `non-miner node` can't execute system governance proposal.
	if c.signTxFn == nil {
		return nil, nil, errors.New("signTxFn not set")
	}

	propRLP, err := rlp.EncodeToBytes(prop)
	if err != nil {
		return nil, nil, err
	}
	//make system governance transaction
	nonce := state.GetNonce(c.validator)
	tx := types.NewTransaction(nonce, system.OnChainDaoContract, common.Big0, header.GasLimit, new(big.Int), propRLP)
	if tx, err = c.signTxFn(accounts.Account{Address: c.validator}, tx, chain.Config().ChainID); err != nil {
		return nil, nil, err
	}
	//add nonce for validator
	state.SetNonce(c.validator, nonce+1)
	receipt := c.executeProposalMsg(chain, header, state, prop, totalTxIndex, tx.Hash(), common.Hash{})

	return tx, receipt, nil
}

func (c *Chaos) replayProposal(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, prop *systemcontract.Proposal, totalTxIndex int, tx *types.Transaction) (*types.Receipt, error) {
	sender, err := types.Sender(c.signer, tx)
	if err != nil {
		return nil, err
	}
	if sender != header.Coinbase {
		return nil, errors.New("invalid sender for system governance transaction")
	}
	propRLP, err := rlp.EncodeToBytes(prop)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(propRLP, tx.Data()) {
		return nil, fmt.Errorf("data missmatch, proposalID: %s, rlp: %s, txHash:%s, txData:%s", prop.Id.String(), hexutil.Encode(propRLP), tx.Hash().String(), hexutil.Encode(tx.Data()))
	}
	//make system governance transaction
	//add nonce for validator
	state.SetNonce(sender, state.GetNonce(sender)+1)
	receipt := c.executeProposalMsg(chain, header, state, prop, totalTxIndex, tx.Hash(), header.Hash())

	return receipt, nil
}

func (c *Chaos) executeProposalMsg(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, prop *systemcontract.Proposal, totalTxIndex int, txHash, bHash common.Hash) *types.Receipt {
	var receipt *types.Receipt
	action := prop.Action.Uint64()
	switch action {
	case 0:
		// evm action.
		state.Prepare(txHash, totalTxIndex)
		err := systemcontract.ExecuteProposal(&systemcontract.CallContext{
			Statedb:      state,
			Header:       header,
			ChainContext: newChainContext(chain, c),
			ChainConfig:  c.chainConfig,
		}, prop)
		receipt = types.NewReceipt([]byte{}, err != nil, header.GasUsed)
		// Set the receipt logs and create a bloom for filtering
		receipt.Logs = state.GetLogs(txHash, bHash)
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		log.Info("executeProposalMsg", "action", "evmCall", "id", prop.Id.String(), "from", prop.From, "to", prop.To, "value", prop.Value.String(), "data", hexutil.Encode(prop.Data), "txHash", txHash.String(), "err", err)

	case 1:
		// delete code action
		ok := state.Erase(prop.To)
		receipt = types.NewReceipt([]byte{}, ok != true, header.GasUsed)
		log.Info("executeProposalMsg", "action", "erase", "id", prop.Id.String(), "to", prop.To, "txHash", txHash.String(), "success", ok)
	default:
		receipt = types.NewReceipt([]byte{}, true, header.GasUsed)
		log.Warn("executeProposalMsg failed, unsupported action", "action", action, "id", prop.Id.String(), "from", prop.From, "to", prop.To, "value", prop.Value.String(), "data", hexutil.Encode(prop.Data), "txHash", txHash.String())
	}

	receipt.TxHash = txHash
	receipt.BlockHash = bHash
	receipt.BlockNumber = header.Number
	receipt.TransactionIndex = uint(state.TxIndex())

	return receipt
}

// IsSysTransaction checks whether a specific transaction is a system transaction.
func (c *Chaos) IsSysTransaction(sender common.Address, tx *types.Transaction, header *types.Header) bool {
	if tx.To() == nil {
		return false
	}
	to := tx.To()
	if sender == header.Coinbase && *to == system.OnChainDaoContract && tx.GasPrice().Sign() == 0 {
		return true
	}
	// Make sure the miner can NOT call the system contract through a normal transaction.
	if sender == header.Coinbase && *to == system.OnChainDaoContract {
		return true
	}
	return false
}

// Methods for debug trace

// ApplyProposalTx applies a system-transaction using a given evm,
// the main purpose of this method is for tracing a system-transaction.
func (c *Chaos) ApplyProposalTx(evm *vm.EVM, state *state.StateDB, txIndex int, sender common.Address, tx *types.Transaction) (ret []byte, vmerr error, err error) {
	var prop = &systemcontract.Proposal{}
	if err = rlp.DecodeBytes(tx.Data(), prop); err != nil {
		return
	}
	evm.Context.AccessFilter = nil
	//add nonce for validator
	evm.StateDB.SetNonce(sender, evm.StateDB.GetNonce(sender)+1)

	action := prop.Action.Uint64()
	switch action {
	case 0:
		// evm action.
		// actually run the governance message
		state.Prepare(tx.Hash(), txIndex)
		evm.TxContext = vm.TxContext{
			Origin:   prop.From,
			GasPrice: common.Big0,
		}
		ret, vmerr = systemcontract.ExecuteProposalWithGivenEVM(evm, prop, tx.Gas())
		state.Finalise(true)
	case 1:
		// delete code action
		_ = state.Erase(prop.To)
	default:
		vmerr = errors.New("unsupported action")
	}
	return
}
