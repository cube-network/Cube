// Copyright 2014 The go-ethereum Authors
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

package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go
//go:generate gencodec -type Init -field-override initMarshaling -out gen_genesis_init.go
//go:generate gencodec -type LockedAccount -field-override lockedAccountMarshaling -out gen_genesis_locked_account.go
//go:generate gencodec -type ValidatorInfo -field-override validatorInfoMarshaling -out gen_genesis_validator_info.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

// Genesis specifies the header fields, state of a genesis block. It also defines hard
// fork switch-over blocks through the chain configuration.
type Genesis struct {
	Config     *params.ChainConfig `json:"config"`
	Nonce      uint64              `json:"nonce"`
	Timestamp  uint64              `json:"timestamp"`
	ExtraData  []byte              `json:"extraData"`
	GasLimit   uint64              `json:"gasLimit"   gencodec:"required"`
	Difficulty *big.Int            `json:"difficulty" gencodec:"required"`
	Mixhash    common.Hash         `json:"mixHash"`
	Coinbase   common.Address      `json:"coinbase"`
	Alloc      GenesisAlloc        `json:"alloc"      gencodec:"required"`
	Validators []ValidatorInfo     `json:"validators"`

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64      `json:"number"`
	GasUsed    uint64      `json:"gasUsed"`
	ParentHash common.Hash `json:"parentHash"`
	BaseFee    *big.Int    `json:"baseFeePerGas"`
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance"            gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	Init       *Init                       `json:"init,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

// InitArgs represents the args of system contracts inital args
type Init struct {
	Admin           common.Address  `json:"admin,omitempty"`
	FirstLockPeriod *big.Int        `json:"firstLockPeriod,omitempty"`
	ReleasePeriod   *big.Int        `json:"releasePeriod,omitempty"`
	ReleaseCnt      *big.Int        `json:"releaseCnt,omitempty"`
	RewardsVer      *big.Int        `json:"rewardsVer,omitempty"`
	RuEpoch         *big.Int        `json:"ruEpoch,omitempty"`
	PeriodTime      *big.Int        `json:"periodTime,omitempty"`
	LockedAccounts  []LockedAccount `json:"lockedAccounts,omitempty"`
}

// LockedAccount represents the info of the locked account
type LockedAccount struct {
	UserAddress  common.Address `json:"userAddress,omitempty"`
	TypeId       *big.Int       `json:"typeId,omitempty"`
	LockedAmount *big.Int       `json:"lockedAmount,omitempty"`
	LockedTime   *big.Int       `json:"lockedTime,omitempty"`
	PeriodAmount *big.Int       `json:"periodAmount,omitempty"`
}

// ValidatorInfo represents the info of inital validators
type ValidatorInfo struct {
	Address          common.Address `json:"address"         gencodec:"required"`
	Manager          common.Address `json:"manager"         gencodec:"required"`
	Rate             *big.Int       `json:"rate,omitempty"`
	Stake            *big.Int       `json:"stake,omitempty"`
	AcceptDelegation bool           `json:"acceptDelegation,omitempty"`
}

// makeValidator creates ValidatorInfo
func makeValidator(address, manager, rate, stake string, acceptDelegation bool) ValidatorInfo {
	rateNum, ok := new(big.Int).SetString(rate, 10)
	if !ok {
		panic("Failed to make validator info due to invalid rate")
	}
	stakeNum, ok := new(big.Int).SetString(stake, 10)
	if !ok {
		panic("Failed to make validator info due to invalid stake")
	}

	return ValidatorInfo{
		Address:          common.HexToAddress(address),
		Manager:          common.HexToAddress(manager),
		Rate:             rateNum,
		Stake:            stakeNum,
		AcceptDelegation: acceptDelegation,
	}
}

// field type overrides for gencodec
type genesisSpecMarshaling struct {
	Nonce      math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
	ExtraData  hexutil.Bytes
	GasLimit   math.HexOrDecimal64
	GasUsed    math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Difficulty *math.HexOrDecimal256
	BaseFee    *math.HexOrDecimal256
	Alloc      map[common.UnprefixedAddress]GenesisAccount
}

type genesisAccountMarshaling struct {
	Code       hexutil.Bytes
	Balance    *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	Storage    map[storageJSON]storageJSON
	PrivateKey hexutil.Bytes
}

type initMarshaling struct {
	FirstLockPeriod *math.HexOrDecimal256
	ReleasePeriod   *math.HexOrDecimal256
	ReleaseCnt      *math.HexOrDecimal256
	RewardsVer      *math.HexOrDecimal256
	RuEpoch         *math.HexOrDecimal256
	PeriodTime      *math.HexOrDecimal256
}

type lockedAccountMarshaling struct {
	TypeId       *math.HexOrDecimal256
	LockedAmount *math.HexOrDecimal256
	LockedTime   *math.HexOrDecimal256
	PeriodAmount *math.HexOrDecimal256
}

type validatorInfoMarshaling struct {
	Rate  *math.HexOrDecimal256
	Stake *math.HexOrDecimal256
}

// storageJSON represents a 256 bit byte array, but allows less than 256 bits when
// unmarshaling from hex.
type storageJSON common.Hash

func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 // pad on the left
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}

func (h storageJSON) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database contains incompatible genesis (have %x, new %x)", e.Stored, e.New)
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The stored chain configuration will be updated if it is compatible (i.e. does not
// specify a fork block below the local head block). In case of a conflict, the
// error is a *params.ConfigCompatError and the new, unwritten config is returned.
//
// The returned chain configuration is never nil.
func SetupGenesisBlock(db ethdb.Database, genesis *Genesis) (*params.ChainConfig, common.Hash, error) {
	return SetupGenesisBlockWithOverride(db, genesis, nil)
}

func SetupGenesisBlockWithOverride(db ethdb.Database, genesis *Genesis, overrideArrowGlacier *big.Int) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.AllEthashProtocolChanges, common.Hash{}, errGenesisNoConfig
	}
	// Just commit the new block if there is no stored genesis block.
	stored := rawdb.ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		if err != nil {
			return genesis.Config, common.Hash{}, err
		}
		return genesis.Config, block.Hash(), nil
	}
	// We have the genesis block in database(perhaps in ancient database)
	// but the corresponding state is missing.
	header := rawdb.ReadHeader(db, stored, 0)
	if _, err := state.New(header.Root, state.NewDatabaseWithConfig(db, nil), nil); err != nil {
		if genesis == nil {
			genesis = DefaultGenesisBlock()
		}
		// Ensure the stored genesis matches with the given one.
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
		block, err := genesis.Commit(db)
		if err != nil {
			return genesis.Config, hash, err
		}
		return genesis.Config, block.Hash(), nil
	}
	// Check whether the genesis block is already written.
	if genesis != nil {
		hash := genesis.ToBlock(nil).Hash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}
	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	if overrideArrowGlacier != nil {
		newcfg.ArrowGlacierBlock = overrideArrowGlacier
	}
	if err := newcfg.CheckConfigForkOrder(); err != nil {
		return newcfg, common.Hash{}, err
	}
	storedcfg := rawdb.ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		rawdb.WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return storedcfg, stored, nil
	}
	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := rawdb.ReadHeaderNumber(db, rawdb.ReadHeadHeaderHash(db))
	if height == nil {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	// Check whether consensus config of Chaos is changed
	if (storedcfg.Chaos != nil || newcfg.Chaos != nil) && (storedcfg.Chaos == nil ||
		newcfg.Chaos == nil || *storedcfg.Chaos != *newcfg.Chaos) {
		return nil, common.Hash{}, errors.New("ChaosConfig is not compatiable with stored")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, *height)
	if compatErr != nil && *height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	rawdb.WriteChainConfig(db, stored, newcfg)
	return newcfg, stored, nil
}

func (g *Genesis) configOrDefault(ghash common.Hash) *params.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	case ghash == params.MainnetGenesisHash:
		return params.MainnetChainConfig
	case ghash == params.TestnetGenesisHash:
		return params.TestnetChainConfig
	default:
		return params.AllChaosProtocolChanges
	}
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (g *Genesis) ToBlock(db ethdb.Database) *types.Block {
	if db == nil {
		db = rawdb.NewMemoryDatabase()
	}
	statedb, err := state.New(common.Hash{}, state.NewDatabase(db), nil)
	if err != nil {
		panic(err)
	}
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	head := &types.Header{
		Number:     new(big.Int).SetUint64(g.Number),
		Nonce:      types.EncodeNonce(g.Nonce),
		Time:       g.Timestamp,
		ParentHash: g.ParentHash,
		Extra:      g.ExtraData,
		GasLimit:   g.GasLimit,
		GasUsed:    g.GasUsed,
		BaseFee:    g.BaseFee,
		Difficulty: g.Difficulty,
		MixDigest:  g.Mixhash,
		Coinbase:   g.Coinbase,
	}
	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	if g.Config != nil && g.Config.IsLondon(common.Big0) {
		if g.BaseFee != nil {
			head.BaseFee = g.BaseFee
		} else {
			head.BaseFee = new(big.Int).SetUint64(params.InitialBaseFee)
		}
	}

	// Handle the Chaos related
	if g.Config != nil && g.Config.Chaos != nil {
		// init system contract
		gInit := &genesisInit{statedb, head, g}
		for name, initSystemContract := range map[string]func() error{
			"Staking":       gInit.initStaking,
			"CommunityPool": gInit.initCommunityPool,
			"BonusPool":     gInit.initBonusPool,
			"GenesisLock":   gInit.initGenesisLock,
		} {
			if err = initSystemContract(); err != nil {
				log.Crit("Failed to init system contract", "contract", name, "err", err)
			}
		}
		// Set validoter info
		if head.Extra, err = gInit.initValidators(); err != nil {
			log.Crit("Failed to init Validators", "err", err)
		}
	}

	// Update root after execution
	head.Root = statedb.IntermediateRoot(false)

	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(head.Root, true, nil)

	return types.NewBlock(head, nil, nil, nil, trie.NewStackTrie(nil))
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db ethdb.Database) (*types.Block, error) {
	block := g.ToBlock(db)
	if block.Number().Sign() != 0 {
		return nil, errors.New("can't commit genesis block with number > 0")
	}
	config := g.Config
	if config == nil {
		config = params.AllEthashProtocolChanges
	}
	if err := config.CheckConfigForkOrder(); err != nil {
		return nil, err
	}
	if config.Clique != nil && len(block.Extra()) == 0 {
		return nil, errors.New("can't start clique chain without signers")
	}
	rawdb.WriteTd(db, block.Hash(), block.NumberU64(), block.Difficulty())
	rawdb.WriteBlock(db, block)
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), nil)
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(db, block.Hash())
	rawdb.WriteHeadFastBlockHash(db, block.Hash())
	rawdb.WriteHeadHeaderHash(db, block.Hash())
	rawdb.WriteChainConfig(db, block.Hash(), config)
	return block, nil
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommit(db ethdb.Database) *types.Block {
	block, err := g.Commit(db)
	if err != nil {
		panic(err)
	}
	return block
}

// GenesisBlockForTesting creates and writes a block in which addr has the given wei balance.
func GenesisBlockForTesting(db ethdb.Database, addr common.Address, balance *big.Int) *types.Block {
	g := Genesis{
		Alloc:   GenesisAlloc{addr: {Balance: balance}},
		BaseFee: big.NewInt(params.InitialBaseFee),
	}
	return g.MustCommit(db)
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.MainnetChainConfig,
		Timestamp:  0x629d4380,
		ExtraData:  hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		GasLimit:   0x05f5e100,
		Difficulty: big.NewInt(1),
		Mixhash:    common.Hash{},
		Alloc:      decodePrealloc(mainnetAllocData),
		Validators: []ValidatorInfo{
			makeValidator("0x2dA7ac30e0a5349d9F80ff9Fc5fE1fB8f4d4600F", "0xfa2d29414dcaa64d4b6752b20c4fc7e6f42ea8fc", "26", "50000", true),
			makeValidator("0xEaC2409dBd931da5EabD46e6c92983d2A3699615", "0x797df3ca242510d46e50cd2bc2be16ede0c61612", "26", "50000", true),
			makeValidator("0xa47646e4B454C108bBBD4FF834101790A244f10C", "0x85c7d77e6475f9a5a3469a8070e372c46badca7b", "26", "50000", true),
			makeValidator("0x98b667B77cBdf71E16bd075bdBA5c23d269a456a", "0xc93ab14a99d6af2f4b42a46ee066e3900ecefcdd", "26", "50000", true),
			makeValidator("0x3633af1DC4640AC9794aB7871319D510aDC5D10e", "0x7e079e9aba7b1d61a0fe7b4cca596709872149d4", "26", "50000", true),
			makeValidator("0x8f6A0444779Ec4969525C310B94DdA765A7608e9", "0x3e03b18571816414bb0c9e063ca9335df92572a0", "26", "50000", true),
			makeValidator("0x48f75A302e2A59EB72d352cd31ed4c13cC9c6346", "0xa1e425bef641b90458409b94a9e591a76a9b5c38", "26", "50000", true),
			makeValidator("0x09dc73a5E3D435683f9e09807783D0b3a3110E25", "0x02a2a59c0d3ada0ebbfb166a0e41a0ebaece17bc", "26", "50000", true),
			makeValidator("0xd50D3a121d929B8f566F5cAd48899Aad01500a26", "0x50d6692414a1794f06b4350c2183257e76926a70", "26", "50000", true),
			makeValidator("0xC3cF65c7c9ebb75E87ece96268e470b9493d2434", "0xfcdd32feec5744aa2bee6a45be87b5831b392f34", "26", "50000", true),
			makeValidator("0x17aB522447155309010dc7beD34259246AF76255", "0xc78600e89efa7cfe96194dddc0217305f5e19738", "26", "50000", true),
			makeValidator("0xdCAe089A400a9Cb1B8DD25b1eDbBd3e08d341961", "0xe5123fc9f63c3e6d684ab9aa2eea30f8566c466f", "26", "50000", true),
			makeValidator("0x68F7ceF23d298d377D52A42510ebe219D24A8C40", "0x6d17ac943cf2fb55a6e119a24f42f72f738698d6", "26", "50000", true),
			makeValidator("0xE5e87B30b923B2066C95f02939F00dA629FAA071", "0x1ae2bd63990c0a973cd126051349f19e5c72d1c4", "26", "50000", true),
			makeValidator("0x5E2deC6C368F98925548bb0F64DAd556E981F92f", "0x5f95f4ced9f0158de27f9bf8680223c8eb36e9f2", "26", "50000", true),
			makeValidator("0xc79947171f262A8ff61D43f6465A923cDD58dA12", "0x4fc05b7b507d5e466fe269ca07efe82df0fd706a", "26", "50000", true),
			makeValidator("0x1232435BF931C0927a8d7D749d936abF63693E68", "0x8f837e522d0956fc0290289e3692e4dfc03a9f58", "26", "50000", true),
			makeValidator("0xF3c9dF9C8a53b456dcdf2125Bc6E36dB9b69FA66", "0xf63c57dc142576eb6332cdac6ef31a38ea55a97f", "26", "50000", true),
			makeValidator("0x2eec7Fa4d11D9515076063Baa319E216Ee69355e", "0x445325716934af82d9995b0d9054b080399a96be", "26", "50000", true),
			makeValidator("0xd0c91a95d6a230A734afb6Ee04749B5A550a7A4E", "0xfcd3e993e09ccd0c89c48414bd8942aff5afaba0", "26", "50000", true),
			makeValidator("0x5CF35483feDc4f77909df1Dac04050807Ff216c1", "0x9f1f9ee5301d57055ebf3afe7e1ebae345f45d9e", "26", "50000", true),
		},
	}
}

func DefaultTestnetGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.TestnetChainConfig,
		Timestamp:  0x6279c720,
		ExtraData:  hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		GasLimit:   0x05f5e100,
		Difficulty: big.NewInt(1),
		Alloc:      decodePrealloc(testnetAllocData),
		Mixhash:    common.Hash{},
		Validators: []ValidatorInfo{
			makeValidator("0x4c5a5ec1258fb56E476dA4fA137928b278351CB0", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "0", "1000000", false),
			makeValidator("0xA6F3A3031208a603727B303A4DbC4e19c0A99508", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "0", "1000000", true),
			makeValidator("0x59FB3eFe94d0F332200DB0Cf2161F2083e6EeCbf", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "40", "1000000", true),
			makeValidator("0xA28B9860780A7D25827519379654222A6cBa187C", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "60", "1000000", true),
			makeValidator("0xC9a91dd01E6EFCf73aD8412f698a7A50a3225437", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "80", "1000000", true),
			makeValidator("0x6277ed2DBe70Ad3436cF97A7C2018bd38d11A7BE", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "100", "1000000", true),
			makeValidator("0xD7040cF7Eb9E23eAa4b92A6fa0BeD0eAD0277d5d", "0x2488c99EEdEf8a842079951792865911C34BDFfb", "100", "1000000", false),
		},
	}
}

// DefaultRopstenGenesisBlock returns the Ropsten network genesis block.
func DefaultRopstenGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.RopstenChainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0x3535353535353535353535353535353535353535353535353535353535353535"),
		GasLimit:   16777216,
		Difficulty: big.NewInt(1048576),
		Alloc:      decodePrealloc(ropstenAllocData),
	}
}

// BasicChaosGenesisBlock returns a genesis containing basic allocation for Chais engine,
func BasicChaosGenesisBlock(config *params.ChainConfig, initialValidators []common.Address, faucet common.Address) *Genesis {
	extraVanity := 32
	extraData := make([]byte, extraVanity+65)
	alloc := decodePrealloc(basicAllocForChaos)
	if (faucet != common.Address{}) {
		// 100M
		b, _ := new(big.Int).SetString("100000000000000000000000000", 10)
		alloc[faucet] = GenesisAccount{Balance: b}
	}
	validators := make([]ValidatorInfo, 0, len(initialValidators))
	for _, val := range initialValidators {
		validators = append(validators, ValidatorInfo{val, faucet, big.NewInt(20), big.NewInt(50000), true})
	}
	alloc[system.StakingContract].Init.Admin = faucet
	return &Genesis{
		Config:     config,
		ExtraData:  extraData,
		GasLimit:   0x280de80,
		Difficulty: big.NewInt(2),
		Alloc:      alloc,
		Validators: validators,
	}
}

// DeveloperGenesisBlock returns the 'geth --dev' genesis block.
func DeveloperGenesisBlock(period uint64, gasLimit uint64, faucet common.Address) *Genesis {
	// Override the default period to the user requested one
	config := *params.AllCliqueProtocolChanges
	config.Clique = &params.CliqueConfig{
		Period: period,
		Epoch:  config.Clique.Epoch,
	}

	// Assemble and return the genesis with the precompiles and faucet pre-funded
	return &Genesis{
		Config:     &config,
		ExtraData:  append(append(make([]byte, 32), faucet[:]...), make([]byte, crypto.SignatureLength)...),
		GasLimit:   gasLimit,
		BaseFee:    big.NewInt(params.InitialBaseFee),
		Difficulty: big.NewInt(1),
		Alloc: map[common.Address]GenesisAccount{
			common.BytesToAddress([]byte{1}): {Balance: big.NewInt(1)}, // ECRecover
			common.BytesToAddress([]byte{2}): {Balance: big.NewInt(1)}, // SHA256
			common.BytesToAddress([]byte{3}): {Balance: big.NewInt(1)}, // RIPEMD
			common.BytesToAddress([]byte{4}): {Balance: big.NewInt(1)}, // Identity
			common.BytesToAddress([]byte{5}): {Balance: big.NewInt(1)}, // ModExp
			common.BytesToAddress([]byte{6}): {Balance: big.NewInt(1)}, // ECAdd
			common.BytesToAddress([]byte{7}): {Balance: big.NewInt(1)}, // ECScalarMul
			common.BytesToAddress([]byte{8}): {Balance: big.NewInt(1)}, // ECPairing
			common.BytesToAddress([]byte{9}): {Balance: big.NewInt(1)}, // BLAKE2b
			faucet:                           {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(9))},
		},
	}
}

func decodePrealloc(data string) GenesisAlloc {
	type locked struct {
		UserAddress  *big.Int
		TypeId       *big.Int
		LockedAmount *big.Int
		LockedTime   *big.Int
		PeriodAmount *big.Int
	}

	type initArgs struct {
		Admin           *big.Int
		FirstLockPeriod *big.Int
		ReleasePeriod   *big.Int
		ReleaseCnt      *big.Int
		RewardsVer      *big.Int
		RuEpoch         *big.Int
		PeriodTime      *big.Int
		LockedAccounts  []locked
	}

	var p []struct {
		Addr    *big.Int
		Balance *big.Int
		Code    []byte
		Init    *initArgs
	}

	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := make(GenesisAlloc, len(p))
	for _, account := range p {
		var init *Init
		if account.Init != nil {
			init = &Init{
				Admin:           common.BigToAddress(account.Init.Admin),
				FirstLockPeriod: account.Init.FirstLockPeriod,
				ReleasePeriod:   account.Init.ReleasePeriod,
				ReleaseCnt:      account.Init.ReleaseCnt,
				RewardsVer:      account.Init.RewardsVer,
				RuEpoch:         account.Init.RuEpoch,
				PeriodTime:      account.Init.PeriodTime,
			}
			if len(account.Init.LockedAccounts) > 0 {
				init.LockedAccounts = make([]LockedAccount, 0, len(account.Init.LockedAccounts))
				for _, locked := range account.Init.LockedAccounts {
					init.LockedAccounts = append(init.LockedAccounts,
						LockedAccount{
							UserAddress:  common.BigToAddress(locked.UserAddress),
							TypeId:       locked.TypeId,
							LockedAmount: locked.LockedAmount,
							LockedTime:   locked.LockedTime,
							PeriodAmount: locked.PeriodAmount,
						})
				}
			}
		}
		ga[common.BigToAddress(account.Addr)] = GenesisAccount{Balance: account.Balance, Code: account.Code, Init: init}
	}
	return ga
}
