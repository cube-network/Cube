package cosmos

import (
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/cosmos/systemcontract"
	"github.com/ethereum/go-ethereum/log"
	dbm "github.com/tendermint/tm-db"
)

// call contract

type CosmosStateDB struct {
	mu      sync.Mutex
	evm     *vm.EVM
	counter int64
}

func NewCosmosStateDB(evm *vm.EVM) *CosmosStateDB {
	csdb := &CosmosStateDB{}
	csdb.SetContext(evm)

	return csdb
}

func (csdb *CosmosStateDB) SetContext(evm *vm.EVM) bool {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()

	csdb.evm = evm
	csdb.counter = 0

	return true
}

func (csdb *CosmosStateDB) Get(key []byte) ([]byte, error) {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()

	if csdb.evm == nil {
		return nil, errors.New("IBCStateDB not init")
	}
	ctx := sdk.Context{}.WithEvm(csdb.evm)
	is_exist, val, err := systemcontract.GetState(ctx, key)
	if err != nil {
		log.Debug("Failed to Get, err", err.Error())
		return nil, err
	}

	if is_exist {
		// println("store. get ", csdb.counter, " batch counter ", csdb.counter, " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val (", len(val), ") ")
		return val, nil
	} else {
		// println("store. get ", csdb.counter, " batch counter ", csdb.counter, " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val ( nil ")

		return nil, nil
	}
}

func (csdb *CosmosStateDB) Has(key []byte) (bool, error) {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()

	if csdb.evm == nil {
		return false, errors.New("IBCStateDB not init")
	}
	ctx := sdk.Context{}.WithEvm(csdb.evm)
	is_exist, _, err := systemcontract.GetState(ctx, key)
	if err != nil {
		log.Debug("Failed to Has, err", err.Error())
		return false, err
	}
	// println("store. has ", csdb.counter, " key (", len(key), ")", string(key), " is exist ", is_exist)

	return is_exist, nil
}

func (csdb *CosmosStateDB) Set(key []byte, val []byte) error {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()
	csdb.counter++

	if csdb.evm == nil {
		return errors.New("IBCStateDB not init")
	}

	var prefix string
	skey := string(key)
	dict := map[string]bool{"s/k:bank/r": true, "s/k:capability/r": true, "s/k:feeibc/r": true, "s/k:ibc/r": true, "s/k:icacontroller/r": true, "s/k:icahost/r": true, "s/k:params/r": true, "s/k:staking/r": true, "s/k:transfer/r": true, "s/k:upgrade/r": true,
		"s/k:bank/o": true, "s/k:capability/o": true, "s/k:feeibc/o": true, "s/k:ibc/o": true, "s/k:icacontroller/o": true, "s/k:icahost/o": true, "s/k:params/o": true, "s/k:staking/o": true, "s/k:transfer/o": true, "s/k:upgrade/o": true,
		"s/k:bank/n": true, "s/k:capability/n": true, "s/k:feeibc/n": true, "s/k:ibc/n": true, "s/k:icacontroller/n": true, "s/k:icahost/n": true, "s/k:params/n": true, "s/k:staking/n": true, "s/k:transfer/n": true, "s/k:upgrade/n": true}
	for k := range dict {
		if strings.Contains(skey, k) {
			prefix = k
			break
		}
	}

	ctx := sdk.Context{}.WithEvm(csdb.evm)
	_, err := systemcontract.SetState(ctx, key, val, prefix)
	if err != nil {
		log.Debug("Failed to Set, err", err.Error())
		return err
	}
	// h := csdb.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex()
	// log.Debug("store. set state root ", h)
	// log.Debug("store. set ", strconv.Itoa(int(csdb.counter)), " key (", len(key), ")", string(key), " hex key ", hex.EncodeToString(key), " val (", len(val), ") ", hex.EncodeToString(val))

	return nil
}

func (csdb *CosmosStateDB) SetSync(key []byte, val []byte) error {
	return csdb.Set(key, val)
}

func (csdb *CosmosStateDB) Delete(key []byte) error {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()
	if csdb.evm == nil {
		return errors.New("CosmosStateDB not init")
	}
	// println("store. del ", len(key), " key ", string(key))
	ctx := sdk.Context{}.WithEvm(csdb.evm)
	_, err := systemcontract.DelState(ctx, key)
	if err != nil {
		log.Debug("Failed to Del, err", err.Error())
		return err
	}

	return nil
}

func (csdb *CosmosStateDB) DeleteSync(key []byte) error {
	return csdb.Delete(key)
}

func (csdb *CosmosStateDB) Iterator(start, end []byte) (dbm.Iterator, error) {
	return csdb.NewCosmosStateIterator(false, start, end)
}

func (csdb *CosmosStateDB) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	return csdb.NewCosmosStateIterator(true, start, end)
}

func (csdb *CosmosStateDB) Close() error {
	return nil
}

type CosmosStateDBBatch struct {
	csdb  *CosmosStateDB
	cache map[string][]byte
}

func (b *CosmosStateDBBatch) Set(key, value []byte) error {
	b.cache[hex.EncodeToString(key)] = value
	return nil
}

func (b *CosmosStateDBBatch) Delete(key []byte) error {
	delete(b.cache, hex.EncodeToString(key))
	return nil
}

func (b *CosmosStateDBBatch) Write() error {
	if len(b.cache) == 0 {
		return nil
	}
	key := make([]string, len(b.cache))
	i := 0
	for k := range b.cache {
		key[i] = k
		i++
	}
	sort.Strings(key)
	for i := 0; i < len(key); i++ {
		k, _ := hex.DecodeString(key[i])
		b.csdb.Set(k, b.cache[key[i]])
	}
	return nil
}

func (b *CosmosStateDBBatch) WriteSync() error {
	return b.Write()
}

func (b *CosmosStateDBBatch) Close() error {
	return nil
}

func (csdb *CosmosStateDB) NewBatch() dbm.Batch {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()
	h := csdb.evm.StateDB.(*state.StateDB).IntermediateRoot(true).Hex()
	log.Debug("newbatch", "state root", h)

	b := &CosmosStateDBBatch{}
	b.csdb = csdb
	b.cache = make(map[string][]byte)
	return b

	// return csdb
}

func (csdb *CosmosStateDB) Print() error {
	return nil
}

func (csdb *CosmosStateDB) Write() error {
	return nil
}
func (csdb *CosmosStateDB) WriteSync() error {
	return nil
}

func (csdb *CosmosStateDB) Stats() map[string]string {
	return map[string]string{}
}

type CosmosStateIterator struct {
	start []byte
	end   []byte

	keys [][]byte
	vals [][]byte
	cur  int
}

func (csdb *CosmosStateDB) NewCosmosStateIterator(is_reverse bool, start []byte, end []byte) (*CosmosStateIterator, error) {
	// println("Iterator reverse ", is_reverse, " start ", string(start), "  ", hex.EncodeToString(start), " end ", string(end), " ", hex.EncodeToString(end))
	is_rootkey := false
	skey := string(start)
	var dictkey string
	dict := map[string]bool{"s/k:bank/r": true, "s/k:capability/r": true, "s/k:feeibc/r": true, "s/k:ibc/r": true, "s/k:icacontroller/r": true, "s/k:icahost/r": true, "s/k:params/r": true, "s/k:staking/r": true, "s/k:transfer/r": true, "s/k:upgrade/r": true,
		"s/k:bank/o": true, "s/k:capability/o": true, "s/k:feeibc/o": true, "s/k:ibc/o": true, "s/k:icacontroller/o": true, "s/k:icahost/o": true, "s/k:params/o": true, "s/k:staking/o": true, "s/k:transfer/o": true, "s/k:upgrade/o": true,
		"s/k:bank/n": true, "s/k:capability/n": true, "s/k:feeibc/n": true, "s/k:ibc/n": true, "s/k:icacontroller/n": true, "s/k:icahost/n": true, "s/k:params/n": true, "s/k:staking/n": true, "s/k:transfer/n": true, "s/k:upgrade/n": true}
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

	csdb.mu.Lock()
	defer csdb.mu.Unlock()
	if csdb.evm == nil {
		return nil, errors.New("CosmosStateDB not init")
	}

	ctx := sdk.Context{}.WithEvm(csdb.evm)
	keys, vals, err := systemcontract.GetRoot(ctx, dictkey)
	if err != nil {
		log.Debug("Failed to Get, err", err.Error())
		return nil, err
	}
	// // 10, 9, 8
	// if len(keys) > 0 && len(vals) > 0 {
	// 	// for i := 0; i < len(keys); i++ {
	// 	// 	println("keys ", i, " ", hex.EncodeToString(keys[i]))
	// 	// }
	// 	si, ei := 0, len(keys)-1
	// 	for i := 0; i < len(keys); i++ {
	// 		cmp := bytes.Compare(keys[i], end)
	// 		if cmp >= 0 {
	// 			si = i
	// 		} else {
	// 			break
	// 		}
	// 	}
	// 	for i := 0; i < len(keys); i++ {
	// 		cmp := bytes.Compare(keys[i], start)
	// 		if cmp >= 0 {
	// 			ei = i
	// 		} else {
	// 			break
	// 		}
	// 	}
	// 	// println("len keys ", len(keys), " ", si, " ei ", ei)
	// 	if si == ei {
	// 		si = ei + 1
	// 	}
	// 	keys = keys[ei:si]
	// 	vals = vals[ei:si]
	// 	// println(len(keys), " ")
	// }
	if !is_reverse {
		for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
			keys[i], keys[j] = keys[j], keys[i]
		}
		for i, j := 0, len(vals)-1; i < j; i, j = i+1, j-1 {
			vals[i], vals[j] = vals[j], vals[i]
		}
	}

	it := &CosmosStateIterator{start: start, end: end, cur: 0, keys: keys, vals: vals}
	return it, nil
}

func (it *CosmosStateIterator) Domain() (start []byte, end []byte) {
	return it.start, it.end
}

func (it *CosmosStateIterator) Valid() bool {
	return it.cur < len(it.keys)
}

func (it *CosmosStateIterator) Next() {
	it.cur++
}

func (it *CosmosStateIterator) Key() (key []byte) {
	return it.keys[it.cur]
}

func (it *CosmosStateIterator) Value() (value []byte) {
	return it.vals[it.cur]
}

func (it *CosmosStateIterator) Error() error {
	return nil
}

func (it *CosmosStateIterator) Close() error {
	return nil
}
