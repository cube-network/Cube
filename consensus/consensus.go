// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package consensus implements different Ethereum consensus engines.
package consensus

import (
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	FeeRecoder = common.HexToAddress("0xffffffffffffffffffffffffffffffffffffffff")
)

// ChainHeaderReader defines a small collection of methods needed to access the local
// blockchain during header verification.
type ChainHeaderReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *params.ChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() *types.Header

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) *types.Header

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) *types.Header

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) *types.Header
}

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {
	ChainHeaderReader

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) *types.Block
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header *types.Header) (common.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainHeaderReader, header *types.Header, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error)

	// VerifyUncles verifies that the given block's uncles conform to the consensus
	// rules of a given engine.
	VerifyUncles(chain ChainReader, block *types.Block) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainHeaderReader, header *types.Header) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// but does not assemble the block.
	//
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	Finalize(chain ChainHeaderReader, header *types.Header, state *state.StateDB, txs *[]*types.Transaction,
		uncles []*types.Header, receipts *[]*types.Receipt, punishTxs []*types.Transaction, proposalTxs []*types.Transaction) error

	// FinalizeAndAssemble runs any post-transaction state modifications (e.g. block
	// rewards) and assembles the final block.
	//
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	FinalizeAndAssemble(chain ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
		uncles []*types.Header, receipts []*types.Receipt) (*types.Block, []*types.Receipt, error)

	// Seal generates a new sealing request for the given input block and pushes
	// the result into the given channel.
	//
	// Note, the method returns immediately and will send the result async. More
	// than one result may also be returned depending on the consensus algorithm.
	Seal(chain ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error

	// SealHash returns the hash of a block prior to it being sealed.
	SealHash(header *types.Header) common.Hash

	// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
	// that a new block should have.
	// Notice: this is currently only used and should be only used for test-case,
	// and it's mainly for the ethash engine.
	CalcDifficulty(chain ChainHeaderReader, time uint64, parent *types.Header) *big.Int

	// APIs returns the RPC APIs this consensus engine provides.
	APIs(chain ChainHeaderReader) []rpc.API

	// Close terminates any background threads maintained by the consensus engine.
	Close() error
}

// PoW is a consensus engine based on proof-of-work.
type PoW interface {
	Engine

	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
}

// ChaosEngine is a consensus engine based on delegate proof-of-stake and BFT.
type ChaosEngine interface {
	Engine

	// PreHandle runs any pre-transaction state modifications (e.g. apply hard fork rules).
	//
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at pre-handling.
	PreHandle(chain ChainHeaderReader, header *types.Header, state *state.StateDB) error

	// VerifyAttestation checks whether an attestation is valid,
	// and if it's valid, return the signer,
	// and a threshold that indicates how many attestations can finalize a block.
	VerifyAttestation(chain ChainHeaderReader, a *types.Attestation) (common.Address, int, error)

	// CurrentValidator Get the verifier address in the current consensus
	CurrentValidator() common.Address
	MaxValidators() uint8

	// Attest trys to give an attestation on current chain when a ChainHeadEvent is fired.
	Attest(chain ChainHeaderReader, headerNum *big.Int, source, target *types.RangeEdge) (*types.Attestation, error)
	CurrentNeedHandleHeight(headerNum uint64) (uint64, error)
	AttestationDelay() uint64

	// IsReadyAttest Whether it meets the conditions for executing interest
	IsReadyAttest() bool
	AttestationStatus() uint8
	StartAttestation()

	// AttestationThreshold Get the attestation threshold at the specified height
	AttestationThreshold(chain ChainHeaderReader, hash common.Hash, number uint64) (int, error)

	Validators(chain ChainHeaderReader, hash common.Hash, number uint64) ([]common.Address, error)

	// CalculateGasPool calculate the expected max gas used for a block
	CalculateGasPool(header *types.Header) uint64

	GetDb() ethdb.Database

	VerifyCasperFFGRule(beforeSourceNum uint64, beforeTargetNum uint64, afterSourceNum uint64, afterTargetNum uint64) int
	// IsDoubleSignPunishTransaction checks whether a specific transaction is a system transaction.
	IsDoubleSignPunishTransaction(sender common.Address, tx *types.Transaction, header *types.Header) bool

	// ExtraValidateOfTx do some consensus related validation to a given transaction.
	ExtraValidateOfTx(sender common.Address, tx *types.Transaction, header *types.Header) error

	ApplyDoubleSignPunishTx(evm *vm.EVM, sender common.Address, tx *types.Transaction) (ret []byte, vmerr error, err error)

	// IsSysTransaction checks whether a specific transaction is a system transaction.
	IsSysTransaction(sender common.Address, tx *types.Transaction, header *types.Header) bool

	// CanCreate determines where a given address can create a new contract.
	CanCreate(state StateReader, addr common.Address, isContract bool, height *big.Int) bool

	// FilterTx do a consensus-related validation on the given transaction at the given header and state.
	FilterTx(sender common.Address, tx *types.Transaction, header *types.Header, parentState *state.StateDB) error

	// CreateEvmAccessFilter returns a EvmAccessFilter if necessary.
	CreateEvmAccessFilter(header *types.Header, parentState *state.StateDB) vm.EvmAccessFilter

	//Methods for debug trace

	// ApplyProposalTx applies a system-transaction using a given evm,
	// the main purpose of this method is for tracing a system-transaction.
	ApplyProposalTx(evm *vm.EVM, state *state.StateDB, txIndex int, sender common.Address, tx *types.Transaction) (ret []byte, vmerr error, err error)
}

type StateReader interface {
	GetState(addr common.Address, hash common.Hash) common.Hash
}
