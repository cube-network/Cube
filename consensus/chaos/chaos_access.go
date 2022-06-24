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
	"encoding/binary"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/chaos/systemcontract"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

const (
	DirectionFrom accessDirection = iota
	DirectionTo
	DirectionBoth
)

var (
	refreshAccessTimer = metrics.NewRegisteredTimer("chaos/accesslist/get", nil)
	getRulesTimer      = metrics.NewRegisteredTimer("chaos/eventcheckrules/get", nil)
)

type EventCheckRule struct {
	EventSig common.Hash
	Checks   map[int]common.AddressCheckType
}

type accessDirection uint

type chaosAccessFilter struct {
	accesses map[common.Address]accessDirection
	rules    map[common.Hash]*EventCheckRule
}

func (b *chaosAccessFilter) IsAddressDenied(address common.Address, cType common.AddressCheckType) (hit bool) {
	d, exist := b.accesses[address]
	if exist {
		switch cType {
		case common.CheckFrom:
			hit = d != DirectionTo // equals to : d == DirectionFrom || d == DirectionBoth
		case common.CheckTo:
			hit = d != DirectionFrom
		case common.CheckBothInAny:
			hit = true
		default:
			log.Warn("access filter, unsupported AddressCheckType", "type", cType)
			// Unsupported value, not denied by default
			hit = false
		}
	}
	if hit {
		log.Trace("Hit access filter", "addr", address.String(), "direction", d, "checkType", cType)
	}
	return
}

func (b *chaosAccessFilter) IsLogDenied(evLog *types.Log) bool {
	if nil == evLog || len(evLog.Topics) <= 1 {
		return false
	}
	if rule, exist := b.rules[evLog.Topics[0]]; exist {
		for idx, checkType := range rule.Checks {
			// do a basic check
			if idx >= len(evLog.Topics) {
				log.Error("check index in rule out to range", "sig", rule.EventSig.String(), "checkIdx", idx, "topicsLen", len(evLog.Topics))
				continue
			}
			addr := common.BytesToAddress(evLog.Topics[idx].Bytes())
			if b.IsAddressDenied(addr, checkType) {
				return true
			}
		}
	}
	return false
}

// CanCreate determines where a given address can create a new contract.
//
// This will queries the system Developers contract, by DIRECTLY to get the target slot value of the contract,
// it means that it's strongly relative to the layout of the Developers contract's state variables
func (c *Chaos) CanCreate(state consensus.StateReader, addr common.Address, isContract bool, height *big.Int) bool {
	if c.chainConfig.IsGravitation(height) && c.config.EnableDevVerification {
		devVerifyEnabled, checkInnerCreation := systemcontract.IsDeveloperVerificationEnabled(state)
		if devVerifyEnabled {
			if checkInnerCreation ||
				(!checkInnerCreation && !isContract) {
				slot := calcSlotOfDevMappingKey(addr)
				valueHash := state.GetState(system.AddressListContract, slot)
				// none zero value means true
				return valueHash.Big().Sign() > 0
			}
		}
	}
	return true
}

// FilterTx do a consensus-related validation on the given transaction at the given header and state.
// the parentState must be the state of the header's parent block.
func (c *Chaos) FilterTx(sender common.Address, tx *types.Transaction, header *types.Header, parentState *state.StateDB) error {
	// Must use the parent state for current validation,
	// so we must starting the validation after GravitationBlock
	if c.chainConfig.GravitationBlock != nil && c.chainConfig.GravitationBlock.Cmp(header.Number) < 0 {
		m, err := c.getAccessList(header, parentState)
		if err != nil {
			return err
		}
		if d, exist := m[sender]; exist && (d != DirectionTo) {
			log.Trace("Hit access filter", "tx", tx.Hash().String(), "addr", sender.String(), "direction", d)
			return types.ErrAddressDenied
		}
		if to := tx.To(); to != nil {
			if d, exist := m[*to]; exist && (d != DirectionFrom) {
				log.Trace("Hit access filter", "tx", tx.Hash().String(), "addr", to.String(), "direction", d)
				return types.ErrAddressDenied
			}
		}
	}
	return nil
}

func (c *Chaos) getAccessList(header *types.Header, parentState *state.StateDB) (map[common.Address]accessDirection, error) {
	defer func(start time.Time) {
		refreshAccessTimer.UpdateSince(start)
	}(time.Now())

	if v, ok := c.accesslist.Get(header.ParentHash); ok {
		return v.(map[common.Address]accessDirection), nil
	}

	c.accessLock.Lock()
	defer c.accessLock.Unlock()
	if v, ok := c.accesslist.Get(header.ParentHash); ok {
		return v.(map[common.Address]accessDirection), nil
	}

	// if the last updates is long ago, we don't need to get accesslist from the contract.
	num := header.Number.Uint64()
	lastUpdated := systemcontract.LastBlackUpdatedNumber(parentState)
	if num >= 2 && num > lastUpdated+1 {
		parent := c.chain.GetHeader(header.ParentHash, num-1)
		if parent != nil {
			if v, ok := c.accesslist.Get(parent.ParentHash); ok {
				m := v.(map[common.Address]accessDirection)
				c.accesslist.Add(header.ParentHash, m)
				return m, nil
			}
		} else {
			log.Error("Unexpected error when getAccessList, can not get parent from chain", "number", num, "blockHash", header.Hash(), "parentHash", header.ParentHash)
		}
	}

	// can't get access list from cache, try to call the contract
	ctx := &systemcontract.CallContext{
		Statedb:      parentState,
		Header:       header,
		ChainContext: newMinimalChainContext(c),
		ChainConfig:  c.chainConfig,
	}

	froms, err := systemcontract.GetBlacksFrom(ctx)
	if err != nil {
		return nil, err
	}
	tos, err := systemcontract.GetBlacksTo(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[common.Address]accessDirection)
	for _, from := range froms {
		m[from] = DirectionFrom
	}
	for _, to := range tos {
		if _, exist := m[to]; exist {
			m[to] = DirectionBoth
		} else {
			m[to] = DirectionTo
		}
	}
	c.accesslist.Add(header.ParentHash, m)
	return m, nil
}

func (c *Chaos) CreateEvmAccessFilter(header *types.Header, parentState *state.StateDB) vm.EvmAccessFilter {
	if c.chainConfig.GravitationBlock != nil && c.chainConfig.GravitationBlock.Cmp(header.Number) < 0 {
		accesses, err := c.getAccessList(header, parentState)
		if err != nil {
			log.Error("getAccessList failed", "err", err)
			return nil
		}
		rules, err := c.getEventCheckRules(header, parentState)
		if err != nil {
			log.Error("getEventCheckRules failed", "err", err)
			return nil
		}
		return &chaosAccessFilter{
			accesses: accesses,
			rules:    rules,
		}
	}
	return nil
}

func (c *Chaos) getEventCheckRules(header *types.Header, parentState *state.StateDB) (map[common.Hash]*EventCheckRule, error) {
	defer func(start time.Time) {
		getRulesTimer.UpdateSince(start)
	}(time.Now())

	if v, ok := c.eventCheckRules.Get(header.ParentHash); ok {
		return v.(map[common.Hash]*EventCheckRule), nil
	}

	c.rulesLock.Lock()
	defer c.rulesLock.Unlock()
	if v, ok := c.eventCheckRules.Get(header.ParentHash); ok {
		return v.(map[common.Hash]*EventCheckRule), nil
	}

	// if the last updates is long ago, we don't need to get access list from the contract.
	num := header.Number.Uint64()
	lastUpdated := systemcontract.LastRulesUpdatedNumber(parentState)
	if num >= 2 && num > lastUpdated+1 {
		parent := c.chain.GetHeader(header.ParentHash, num-1)
		if parent != nil {
			if v, ok := c.eventCheckRules.Get(parent.ParentHash); ok {
				m := v.(map[common.Hash]*EventCheckRule)
				c.eventCheckRules.Add(header.ParentHash, m)
				return m, nil
			}
		} else {
			log.Error("Unexpected error when getEventCheckRules, can not get parent from chain", "number", num, "blockHash", header.Hash(), "parentHash", header.ParentHash)
		}
	}

	// can't get access list from cache, try to call the contract
	ctx := &systemcontract.CallContext{
		Statedb:      parentState,
		Header:       header,
		ChainContext: newMinimalChainContext(c),
		ChainConfig:  c.chainConfig,
	}

	cnt, err := systemcontract.GetRulesLen(ctx)
	if err != nil {
		return nil, err
	}
	rules := make(map[common.Hash]*EventCheckRule)
	var i uint32 = 0
	for ; i < cnt; i++ {
		sig, idx, ct, err := systemcontract.GetRuleByIndex(ctx, i)
		if err != nil {
			log.Error("getRuleByIndex failed", "index", i, "number", num, "blockHash", header.Hash(), "err", err)
			return nil, err
		}
		rule, exist := rules[sig]
		if !exist {
			rule = &EventCheckRule{
				EventSig: sig,
				Checks:   make(map[int]common.AddressCheckType),
			}
			rules[sig] = rule
		}
		rule.Checks[idx] = ct
	}

	c.eventCheckRules.Add(header.ParentHash, rules)
	return rules, nil
}

func calcSlotOfDevMappingKey(addr common.Address) common.Hash {
	p := make([]byte, common.HashLength)
	binary.BigEndian.PutUint16(p[common.HashLength-2:], uint16(system.DevMappingPosition))
	return crypto.Keccak256Hash(addr.Hash().Bytes(), p)
}
