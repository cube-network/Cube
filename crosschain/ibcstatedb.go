package crosschain

import (
	"errors"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/systemcontract"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	dbm "github.com/tendermint/tm-db"
)

// call contract

type IBCStateDB struct {
	mu      sync.Mutex
	ethdb   ethdb.Database
	statedb *state.StateDB
	evm     *vm.EVM
	counter int

	header        *types.Header
	parent_header *types.Header
}

func NewIBCStateDB(ethdb ethdb.Database) *IBCStateDB {
	ibcstatedb := &IBCStateDB{ethdb: ethdb}
	return ibcstatedb
}

func (mdb *IBCStateDB) SetEVM(config *params.ChainConfig, blockContext vm.BlockContext, cube_statedb *state.StateDB, header *types.Header, parent_header *types.Header, cfg vm.Config) bool {
	mdb.mu.Lock()
	defer mdb.mu.Unlock()

	mdb.counter = 0
	mdb.header = header
	mdb.parent_header = parent_header

	var state_root, empty_state_root common.Hash
	state_root.SetBytes(parent_header.Extra[:32])

	println("cosmos restore state root ", state_root.Hex())
	var statedb *state.StateDB
	statedb, err := state.New(state_root, state.NewDatabase(mdb.ethdb), nil)
	if err != nil {
		state_root = empty_state_root
		statedb, _ = state.New(state_root, state.NewDatabase(mdb.ethdb), nil)
	}

	mdb.statedb = statedb
	mdb.evm = vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, cfg)

	if state_root.Hex() == empty_state_root.Hex() {
		println("init statedb with code/account")
		statedb.CreateAccount(system.IBCStateContract)
		statedb.SetCode(system.IBCStateContract, cube_statedb.GetCode(system.IBCStateContract))

		return false
	}

	return true
}

func (mdb *IBCStateDB) Commit(statedb *state.StateDB) common.Hash {
	mdb.mu.Lock()
	defer mdb.mu.Unlock()

	var ws sync.WaitGroup
	ws.Add(1)
	var hash common.Hash
	afterCommit := func(root common.Hash) {
		hash = root
		ws.Done()
	}
	err := statedb.AsyncCommit(false, afterCommit)
	if err != nil {
		println("err ", err.Error())
	}
	ws.Wait()
	statedb.Database().TrieDB().Commit(hash, false, nil)
	println("ibc state commit, hash ", hash.Hex(), " set counter ", mdb.counter, time.Now().UTC().String())

	return hash
}

// TODO cache
func (mdb *IBCStateDB) Get(key []byte) ([]byte, error) {
	if mdb.evm == nil {
		return nil, errors.New("IBCStateDB not init")
	}
	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	ctx := sdk.Context{}.WithEvm(mdb.evm)
	is_exist, val, err := systemcontract.GetState(ctx, key)
	if err != nil {
		println("Failed to Get, err", err.Error())
		return nil, err
	}

	if is_exist {
		// println("store. get ", mdb.counter, " batch counter ", mdb.counter, " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val (", len(val), ") ")
		return val, nil
	} else {
		// println("store. get ", mdb.counter, " batch counter ", mdb.counter, " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val ( nil ")

		return nil, nil
	}
}

func (mdb *IBCStateDB) Has(key []byte) (bool, error) {
	if mdb.evm == nil {
		return false, errors.New("IBCStateDB not init")
	}
	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	ctx := sdk.Context{}.WithEvm(mdb.evm)
	is_exist, _, err := systemcontract.GetState(ctx, key)
	if err != nil {
		println("Failed to Get, err", err.Error())
		return false, err
	}
	// println("store. has ", mdb.counter, " batch counter ", mdb.counter, " key (", len(key), ")", string(key), " is exist ", is_exist)

	return is_exist, nil
}

func (mdb *IBCStateDB) Set(key []byte, val []byte) error {
	if mdb.evm == nil {
		return errors.New("IBCStateDB not init")
	}

	// println("store. set ", mdb.counter, " batch counter ", mdb.counter, " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val (", len(val), ") ", hex.EncodeToString(val))
	mdb.counter++

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	var prefix string
	skey := string(key)
	dict := map[string]bool{"s/k:bank/r": true, "s/k:capability/r": true, "s/k:feeibc/r": true, "s/k:ibc/r": true, "s/k:icacontroller/r": true, "s/k:icahost/r": true, "s/k:params/r": true, "s/k:staking/r": true, "s/k:transfer/r": true, "s/k:upgrade/r": true}
	for k := range dict {
		if strings.Contains(skey, k) {
			prefix = k
			break
		}
	}

	ctx := sdk.Context{}.WithEvm(mdb.evm)
	_, err := systemcontract.SetState(ctx, key, val, prefix)
	if err != nil {
		println("Failed to Set, err", err.Error())
		return err
	}

	return nil
}

func (mdb *IBCStateDB) SetSync(key []byte, val []byte) error {
	if mdb.evm == nil {
		return errors.New("IBCStateDB not init")
	}
	mdb.counter++
	return mdb.Set(key, val)
}

func (mdb *IBCStateDB) Delete(key []byte) error {
	if mdb.evm == nil {
		return errors.New("IBCStateDB not init")
	}

	// TODO delete contract
	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	ctx := sdk.Context{}.WithEvm(mdb.evm)
	_, err := systemcontract.DelState(ctx, key)
	if err != nil {
		println("Failed to Get, err", err.Error())
		return err
	}

	return nil
}

func (mdb *IBCStateDB) DeleteSync(key []byte) error {
	return mdb.Delete(key)
}

func (mdb *IBCStateDB) Iterator(start, end []byte) (dbm.Iterator, error) {
	return mdb.NewIBCStateIterator(false, start, end)
}

func (mdb *IBCStateDB) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	return mdb.NewIBCStateIterator(true, start, end)
}

func (mdb *IBCStateDB) Close() error {
	return nil
}

func (mdb *IBCStateDB) NewBatch() dbm.Batch {
	return mdb
}

func (mdb *IBCStateDB) Print() error {
	return nil
}

func (mdb *IBCStateDB) Write() error {
	return nil
}
func (mdb *IBCStateDB) WriteSync() error {
	return nil
}

func (mdb *IBCStateDB) Stats() map[string]string {
	return map[string]string{}
}

type IBCStateIterator struct {
	start []byte
	end   []byte

	keys [][]byte
	vals [][]byte
	cur  int
}

func (mdb *IBCStateDB) NewIBCStateIterator(is_reverse bool, start []byte, end []byte) (*IBCStateIterator, error) {
	if mdb.evm == nil {
		return nil, errors.New("IBCStateDB not init")
	}

	is_rootkey := false
	skey := string(start)
	var dictkey string
	dict := map[string]bool{"s/k:bank/r": true, "s/k:capability/r": true, "s/k:feeibc/r": true, "s/k:ibc/r": true, "s/k:icacontroller/r": true, "s/k:icahost/r": true, "s/k:params/r": true, "s/k:staking/r": true, "s/k:transfer/r": true, "s/k:upgrade/r": true}
	for k := range dict {
		if strings.Contains(skey, k) {
			is_rootkey = true
			dictkey = k
			break
		}
	}

	if !is_rootkey {
		return nil, errors.New("not support iterator")
	}

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	ctx := sdk.Context{}.WithEvm(mdb.evm)
	keys, vals, err := systemcontract.GetRoot(ctx, dictkey)
	if err != nil {
		println("Failed to Get, err", err.Error())
		return nil, err
	}

	if !is_reverse {
		for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
			keys[i], keys[j] = keys[j], keys[i]
		}
		for i, j := 0, len(vals)-1; i < j; i, j = i+1, j-1 {
			vals[i], vals[j] = vals[j], vals[i]
		}

	}

	it := &IBCStateIterator{start: start, end: end, cur: 0, keys: keys, vals: vals}
	return it, nil
}

func (it *IBCStateIterator) Domain() (start []byte, end []byte) {
	return it.start, it.end
}

func (it *IBCStateIterator) Valid() bool {
	return it.cur < len(it.keys)
}

func (it *IBCStateIterator) Next() {
	it.cur++
}

func (it *IBCStateIterator) Key() (key []byte) {
	return it.keys[it.cur]
}

func (it *IBCStateIterator) Value() (value []byte) {
	return it.vals[it.cur]
}

func (it *IBCStateIterator) Error() error {
	return nil
}

func (it *IBCStateIterator) Close() error {
	return nil
}
