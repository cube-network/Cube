package cosmos

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	gogotypes "github.com/gogo/protobuf/types"
	dbm "github.com/tendermint/tm-db"
)

var MAX_IAVL_COUNT int64 = 2

type DB struct {
	impl dbm.DB
}

func (db *DB) SetImpl(impl dbm.DB) {
	db.impl = impl
}

func (db *DB) Get(key []byte) ([]byte, error) {
	return db.impl.Get(key)
}

func (db *DB) Has(key []byte) (bool, error) {
	return db.impl.Has(key)
}

func (db *DB) Set(key []byte, val []byte) error {
	return db.impl.Set(key, val)
}

func (db *DB) SetSync(key []byte, val []byte) error {
	return db.impl.SetSync(key, val)
}

func (db *DB) Delete(key []byte) error {
	return db.impl.Delete(key)
}

func (db *DB) DeleteSync(key []byte) error {
	return db.impl.DeleteSync(key)
}

func (db *DB) Iterator(start, end []byte) (dbm.Iterator, error) {
	return db.impl.Iterator(start, end)
}

func (db *DB) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	return db.impl.ReverseIterator(start, end)
}

func (db *DB) Close() error {
	return db.impl.Close()
}

func (db *DB) NewBatch() dbm.Batch {
	b := &DBBatch{}
	b.db = db
	b.cache = make(map[string][]byte)
	return b
}

func (db *DB) Print() error {
	return db.impl.Print()
}

func (db *DB) Stats() map[string]string {
	return db.impl.Stats()
}

type DBBatch struct {
	db    dbm.DB
	cache map[string][]byte
}

func (b *DBBatch) Set(key, value []byte) error {
	// b.cache[hex.EncodeToString(key)] = value
	b.cache[string(key)] = value
	return nil
}

func (b *DBBatch) Delete(key []byte) error {
	// delete(b.cache, hex.EncodeToString(key))
	delete(b.cache, string(key))
	return nil
}

func (b *DBBatch) Write() error {
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
		// k, _ := hex.DecodeString(key[i])
		k := []byte(key[i])
		b.db.Set(k, b.cache[key[i]])
	}
	return nil
}

func (b *DBBatch) WriteSync() error {
	return b.Write()
}

func (b *DBBatch) Close() error {
	return nil
}

type CosmosStateDB struct {
	mu      sync.Mutex
	evm     *vm.EVM
	counter int64
	config  *params.ChainConfig

	hasher crypto.KeccakState
}

func NewCosmosStateDB(evm *vm.EVM) *CosmosStateDB {
	csdb := &CosmosStateDB{}

	csdb.hasher = crypto.NewKeccakState()
	csdb.SetContext(evm, nil)

	return csdb
}

func (csdb *CosmosStateDB) SetContext(evm *vm.EVM, config *params.ChainConfig) bool {
	csdb.mu.Lock()
	defer csdb.mu.Unlock()

	csdb.evm = evm
	csdb.config = config

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
	is_exist, val, err := csdb.GetState(ctx, key)

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
	is_exist, _, err := csdb.GetState(ctx, key)
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

	ctx := sdk.Context{}.WithEvm(csdb.evm)
	_, err := csdb.SetState(ctx, key, val)
	if err != nil {
		log.Debug("Failed to Set, err", err.Error())
		return err
	}

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
	_, err := csdb.DelState(ctx, key)
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
	csdb  dbm.DB
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

	b := &CosmosStateDBBatch{}
	b.csdb = csdb
	b.cache = make(map[string][]byte)
	return b
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

func (csdb *CosmosStateDB) getLatestVersion() int64 {
	ctx := sdk.Context{}.WithEvm(csdb.evm)
	ok, bz, err := csdb.GetState(ctx, []byte("s/latest"))
	if !ok {
		return 0
	}
	if err != nil {
		panic(err)
	} else if bz == nil {
		return 0
	}

	var latestVersion int64

	if err := gogotypes.StdInt64Unmarshal(&latestVersion, bz); err != nil {
		panic(err)
	}

	return latestVersion
}

func (csdb *CosmosStateDB) NewCosmosStateIterator(is_reverse bool, start []byte, end []byte) (*CosmosStateIterator, error) {
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

	keys := make([][]byte, 0)
	vals := make([][]byte, 0)

	ver := csdb.getLatestVersion()
	sver := ver - MAX_IAVL_COUNT
	if sver < 0 {
		sver = 0
	}

	for {
		key := make([]byte, len(dictkey)+8)
		copy(key[:len(dictkey)], start[:len(dictkey)])
		binary.BigEndian.PutUint64(key[len(key)-8:], uint64(ver))
		ok, val, _ := csdb.GetState(ctx, key)
		if !ok {
			break
		} else {
			keys = append(keys, key)
			vals = append(vals, val)
		}

		ver = ver - 1
		if ver <= sver {
			break
		}
	}

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

func makeSliceCount(len int) (int, int) {
	count := len / common.HashLength
	if len%common.HashLength != 0 {
		return count + 1, len % common.HashLength
	} else {
		return count, common.HashLength
	}
}

func hashData(hasher crypto.KeccakState, data []byte) common.Hash {
	// w := sha3.NewLegacyKeccak256()
	// w.Write(data[:])
	// var h common.Hash
	// w.Sum(h[:0])
	return crypto.HashDataWithCache(hasher, data[:])
}

func makeInt64(len int64) common.Hash {
	//padding left
	slen := strconv.FormatInt(len, 10)
	return common.BytesToHash([]byte(slen))
}

func makeLength(len int) common.Hash {
	return makeInt64(int64(len))
}

func readInt64(val common.Hash) int64 {
	i := 0
	for ; i < common.HashLength; i++ {
		if val[i] != 0 {
			break
		}
	}
	if i == common.HashLength {
		return -1
	}

	slen := string(val[i:])
	len, err := strconv.ParseInt(slen, 10, 64)
	if err != nil {
		panic("unexpected length " + slen)
	}
	return len
}
func readLength(val common.Hash) int {
	return int(readInt64(val))
}
func makeHashKeys(hasher crypto.KeccakState, key []byte, valLen int) []common.Hash {
	sliceCount, _ := makeSliceCount(valLen)
	keys := make([]common.Hash, sliceCount)

	for i := 0; i < sliceCount; i++ {
		k := hashData(hasher, key).Bytes()
		k = append(k, []byte(strconv.Itoa(i))...)
		hk := hashData(hasher, k)
		keys[i] = hk
	}

	return keys
}

func makeHashVals(val []byte) []common.Hash {
	sliceCount, _ := makeSliceCount(len(val))
	vals := make([]common.Hash, sliceCount)

	for i := 0; i < sliceCount; i++ {
		if i < sliceCount-1 {
			b := val[common.HashLength*i : common.HashLength*i+common.HashLength]
			vals[i] = common.BytesToHash(b)
		} else {
			b := val[common.HashLength*i:]
			vals[i] = common.BytesToHash(b)
		}
	}
	return vals
}

func makeMasterKV(hasher crypto.KeccakState, key []byte, val []byte) (common.Hash, common.Hash) {
	return hashData(hasher, key), makeLength(len(val))
}

func (csdb *CosmosStateDB) SetState(ctx sdk.Context, key []byte, val []byte) ([]byte, error) {
	statedb := ctx.EVM().StateDB.(*state.StateDB)

	mk, mv := makeMasterKV(csdb.hasher, key, val)
	statedb.SetState(system.CrossChainCosmosStateContract, mk, mv)

	keys := makeHashKeys(csdb.hasher, key, len(val))
	vals := makeHashVals(val)
	if len(keys) != len(vals) {
		panic("make key val not match")
	}

	for i := 0; i < len(keys); i++ {
		statedb.SetState(system.CrossChainCosmosStateContract, keys[i], vals[i])
	}
	// log.Debug(fmt.Sprintf("%p", csdb), "set state counter ", strconv.Itoa(int(csdb.counter)), " key ", string(key), " ", hashData(key).Hex(), " ", hashData(val).Hex())

	return nil, nil
}

func (csdb *CosmosStateDB) GetState(ctx sdk.Context, key []byte) (bool, []byte, error) {
	statedb := ctx.EVM().StateDB.(*state.StateDB)

	masterKey := hashData(csdb.hasher, key)
	masterVal := statedb.GetState(system.CrossChainCosmosStateContract, masterKey)
	valLen := readLength(masterVal)
	if valLen < 0 {
		return false, nil, nil
	}
	if valLen == 0 {
		return true, make([]byte, 0), nil
	}

	keys := makeHashKeys(csdb.hasher, key, valLen)
	vals := make([]byte, valLen)
	kvSliceCount, last_val_slice_len := makeSliceCount(valLen)
	for i := 0; i < kvSliceCount; i++ {
		vali := statedb.GetState(system.CrossChainCosmosStateContract, keys[i])
		if i == kvSliceCount-1 {
			copy(vals[common.HashLength*i:], vali.Bytes()[common.HashLength-last_val_slice_len:])
		} else {
			copy(vals[common.HashLength*i:common.HashLength*i+common.HashLength], vali.Bytes())
		}
	}

	// log.Debug("GetState key ", hashData(key).Hex(), " val ", hashData(vals).Hex())

	return true, vals, nil
}

func (csdb *CosmosStateDB) DelState(ctx sdk.Context, key []byte) ([]byte, error) {
	statedb := ctx.EVM().StateDB.(*state.StateDB)

	masterKey := hashData(csdb.hasher, key)
	masterVal := statedb.GetState(system.CrossChainCosmosStateContract, masterKey)
	valLen := readLength(masterVal)
	keys := makeHashKeys(csdb.hasher, key, valLen)

	statedb.SetState(system.CrossChainCosmosStateContract, masterKey, common.Hash{})
	for i := 0; i < len(keys); i++ {
		statedb.SetState(system.CrossChainCosmosStateContract, keys[i], common.Hash{})
	}

	// log.Debug("DelState key ", hashData(key).Hex())

	return nil, nil
}
