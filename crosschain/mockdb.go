package crosschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tm-db"
)

type MockDB struct {
	db      dbm.DB
	counter int
}

func NewMockDB(name string, path string) *MockDB {
	db, _ := sdk.NewLevelDB(name, path)
	mdb := &MockDB{db: db}
	return mdb
}

func (mdb *MockDB) Get(b []byte) ([]byte, error) {
	return mdb.db.Get(b)
}

func (mdb *MockDB) Has(key []byte) (bool, error) {
	return mdb.db.Has(key)
}

func (mdb *MockDB) Set(s []byte, b []byte) error {
	mdb.counter++
	// println("set ", mdb.counter, " key ", string(s), " val ", b, hex.EncodeToString(b))
	return mdb.db.Set(s, b)
}

func (mdb *MockDB) SetSync(s []byte, b []byte) error {
	mdb.counter++
	// println("set sync ", mdb.counter, " key ", string(s), " val ", b, hex.EncodeToString(b))
	return mdb.db.SetSync(s, b)
}

func (mdb *MockDB) Delete(b []byte) error {
	return mdb.db.Delete(b)
}

func (mdb *MockDB) DeleteSync(b []byte) error {
	return mdb.db.DeleteSync(b)
}

func (mdb *MockDB) Iterator(start, end []byte) (dbm.Iterator, error) {
	return mdb.db.Iterator(start, end)
}

func (mdb *MockDB) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	return mdb.db.ReverseIterator(start, end)
}

func (mdb *MockDB) Close() error {
	return mdb.db.Close()
}

func (mdb *MockDB) NewBatch() dbm.Batch {
	return &MockBatch{batch: mdb.db.NewBatch(), db: mdb}
}

func (mdb *MockDB) Print() error {
	return mdb.db.Print()
}

func (mdb *MockDB) Stats() map[string]string {
	return mdb.db.Stats()
}

func (mdb *MockDB) Clean() error {
	mdb.counter = 0
	return nil
}

type MockBatch struct {
	batch   dbm.Batch
	db      *MockDB
	counter int
}

func (mdb *MockBatch) Set(key, value []byte) error {
	mdb.db.counter++
	mdb.counter++
	// println("set ", mdb.db.counter, " batch counter ", mdb.counter, " key (", len(key), ")", string(key), " val (", len(value), ") ", hex.EncodeToString(value))
	return mdb.batch.Set(key, value)
}

func (mdb *MockBatch) Delete(key []byte) error {
	return mdb.batch.Delete(key)
}

func (mdb *MockBatch) Write() error {
	return mdb.batch.Write()
}

func (mdb *MockBatch) WriteSync() error {
	return mdb.batch.WriteSync()
}
func (mdb *MockBatch) Close() error {
	return mdb.batch.Close()
}
