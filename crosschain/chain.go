package crosschain

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proto/tendermint/version"
	ct "github.com/tendermint/tendermint/types"
)

func (app *CosmosApp) InitGenesis(evm *vm.EVM) {
	if app.is_genesis_init {
		return
	}

	// Module Account
	evm.StateDB.CreateAccount(common.HexToAddress(ModuleAccount))
	// deploy erc20 factory contract
	evm.StateDB.CreateAccount(system.ERC20FactoryContract)
	code, _ := hex.DecodeString(ERC20FactoryCode)
	evm.StateDB.SetCode(system.ERC20FactoryContract, code)

	// todo: deploy validators' register contract

	// crosschain
	init_block_height := evm.Context.BlockNumber.Int64()

	app.LoadVersion2(0)
	var genesisState GenesisState
	if err := tmjson.Unmarshal([]byte(IBCConfig), &genesisState); err != nil {
		panic(err)
	}

	app.InitChain(abci.RequestInitChain{Time: time.Time{}, ChainId: app.cc.ChainID, InitialHeight: init_block_height})
	app.mm.InitGenesis(app.GetContextForDeliverTx([]byte{}), app.codec.Marshaler, genesisState)

	app.last_begin_block_height = init_block_height
	app.is_genesis_init = true

	app.cc.validators.initGenesisValidators(init_block_height)

	hdr := app.cc.MakeCosmosSignedHeader(app.db.header, common.Hash{})
	app.BeginBlock(abci.RequestBeginBlock{Header: *hdr.ToProto().Header})
}

// TODO get cube block header instead
func (app *CosmosApp) Load(init_block_height int64) {
	if app.last_begin_block_height == 0 {
		app.LoadVersion2(init_block_height)
		app.last_begin_block_height = init_block_height
	}

	if app.last_begin_block_height != init_block_height {
		println("load version... ", init_block_height)
		app.LoadVersion2(init_block_height)
	}
	app.last_begin_block_height = init_block_height + 1
}

func (app *CosmosApp) OnBlockBegin(config *params.ChainConfig, blockContext vm.BlockContext, statedb *state.StateDB, header *types.Header, parent_header *types.Header, cfg vm.Config) {
	app.bapp_mu.Lock()
	defer app.bapp_mu.Unlock()

	app.is_genesis_init = app.db.SetEVM(config, blockContext, statedb, header, parent_header, cfg)

	println("begin block height", header.Number.Int64(), " genesis init ", app.is_genesis_init, " ts ", time.Now().UTC().String())

	if !app.is_genesis_init {
		// app.InitGenesis(app.db.evm)
		return
	} else {
		app.Load(parent_header.Number.Int64())
		hdr := app.cc.MakeCosmosSignedHeader(header, common.Hash{})
		app.BeginBlock(abci.RequestBeginBlock{Header: *hdr.ToProto().Header})
	}
}

func (app *CosmosApp) CommitIBC(statedb *state.StateDB) {
	if !app.is_genesis_init {
		return
	}
	app.db.Commit(statedb)
}

func (app *CosmosApp) OnBlockEnd() (common.Hash, *state.StateDB) {
	if !app.is_genesis_init {
		return common.Hash{}, nil
	}

	app.bapp_mu.Lock()
	defer app.bapp_mu.Unlock()

	c := app.BaseApp.Commit()
	app.db.Set([]byte("cosmos_app_hash"), c.Data[:])
	app.app_hash.SetBytes(c.Data[:])

	state_root := app.db.statedb.IntermediateRoot(false)
	app.state_root = state_root

	println("OnBlockEnd ibc hash", hex.EncodeToString(c.Data[:]), " state root ", state_root.Hex(), " ts ", time.Now().UTC().String())

	return state_root, app.db.statedb
}

func (app *CosmosApp) MakeHeader(h *et.Header) *ct.Header {
	if !app.is_genesis_init || !app.cc.isProposer() {
		return nil
	}

	app.cc.MakeLightBlockAndSign(h, app.app_hash)
	light_block := app.cc.GetLightBlock(h.Number.Int64())
	if light_block != nil {
		println("header ", light_block.Header.AppHash.String(), " ", time.Now().UTC().String())
		return light_block.Header
	}
	return nil
}

func (app *CosmosApp) Vote(block_height uint64, Address ct.Address) {
	if !app.is_genesis_init {
		return
	}
	// app.cc.MakeCosmosSignedHeader(h, nil)
}

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type CosmosChain struct {
	ChainID     string
	light_block map[int64]*ct.LightBlock // cache only for demo, write/read db instead later
	// light_block    *lru.ARCCache
	//validators     []*ct.Validator // fixed for demo; full validator set, fixed validator set for demo,
	validators *ValidatorsMgr
	//priv_addr_idx  uint32
	priv_validator *privval.FilePV // use ed2559 for demo, secp256k1 support later;

	blockID           ct.BlockID // load best block height later
	best_block_height uint64
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(chainID string, priv_validator_key_file, priv_validator_state_file string) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	// TODO chainID
	c.ChainID = "ibc-1"
	c.light_block = make(map[int64]*ct.LightBlock)
	c.priv_validator = privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
	c.priv_validator.Save()

	// TODO load validator set, should use contract to deal with validators getting changed in the future
	c.validators = &ValidatorsMgr{}

	// TODO load best block
	psh := ct.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = ct.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

//func MarshalPubKeyToAmino(cdc *amino.Codec, key crypto.PubKey) (data []byte, err error) {
//	switch key.(type) {
//	case secp256k1.PubKeySecp256k1:
//		data = make([]byte, 0, secp256k1.PubKeySecp256k1Size+typePrefixAndSizeLen)
//		data = append(data, typePubKeySecp256k1Prefix...)
//		data = append(data, byte(secp256k1.PubKeySecp256k1Size))
//		keyData := key.(secp256k1.PubKeySecp256k1)
//		data = append(data, keyData[:]...)
//		return data, nil
//	case ed25519.PubKeyEd25519:
//		data = make([]byte, 0, ed25519.PubKeyEd25519Size+typePrefixAndSizeLen)
//		data = append(data, typePubKeyEd25519Prefix...)
//		data = append(data, byte(ed25519.PubKeyEd25519Size))
//		keyData := key.(ed25519.PubKeyEd25519)
//		data = append(data, keyData[:]...)
//		return data, nil
//	case sr25519.PubKeySr25519:
//		data = make([]byte, 0, sr25519.PubKeySr25519Size+typePrefixAndSizeLen)
//		data = append(data, typePubKeySr25519Prefix...)
//		data = append(data, byte(sr25519.PubKeySr25519Size))
//		keyData := key.(sr25519.PubKeySr25519)
//		data = append(data, keyData[:]...)
//		return data, nil
//	}
//	data, err = cdc.MarshalBinaryBare(key)
//	if err != nil {
//		return nil, err
//	}
//	return data, nil
//}

//func (c *CosmosChain) initValidators() error {
//	c.validators = &ValidatorsMgr{}
//
//	var validators []ct.Validator
//	if err := tmjson.Unmarshal([]byte(ValidatorsConfig), &validators); err != nil {
//		panic(err)
//	}
//
//	//validators := make([]*ct.Validator, len(validators))
//	validators := make([]*ct.Validator, len(validators))
//	priv_val_pubkey, _ := c.priv_validator.GetPubKey()
//	for index, val := range validators {
//		//validators[index] = &val
//		if val.PubKey.Equals(priv_val_pubkey) {
//			c.priv_addr_idx = uint32(index)
//			fmt.Printf("val.addr: %s, val.index: %d\n", val.Address.String(), c.priv_addr_idx)
//		}
//		//fmt.Printf("val.addr: %s, val.pubkey: %d\n", val.Address.String(), val.PubKey.Address().String())
//	}
//
//	//c.state.Validators = ct.NewValidatorSet(c.validators)
//
//	//publicKey, _ := c.priv_validator.GetPubKey()
//	//publicKeyBytes := make([]byte, ed25519.PubKeySize)
//	//copy(publicKeyBytes, publicKey.Bytes())
//	//restoredPubkey := ed25519.PubKey{Key: publicKeyBytes}
//	//println(restoredPubkey.Address().String())
//	//println(publicKey.Address().String())
//	//println(restoredPubkey.String())
//	//println(string(publicKey.Bytes()))
//
//	return nil
//}

func (c *CosmosChain) String() string {
	return fmt.Sprintf(
		"Cosmos{\n ChainID:%v \n len(light_block):%v \n priv_validator:%v \n blockID:%v}",
		c.ChainID,
		len(c.light_block),
		//len(c.validators),
		//c.priv_addr_idx,
		c.priv_validator,
		c.blockID,
	)
}

func (c *CosmosChain) isProposer() bool {
	if c.priv_validator == nil {
		return false
	}
	return c.validators.isProposer(c.priv_validator.GetAddress())
}

//func (c *CosmosChain) MakeValidatorSet() *ct.ValidatorSet {
//	vs := &ct.ValidatorSet{}
//	vs.Validators = c.validators
//	// TODO cube.header.coinbase
//	vs.Proposer = c.validators[c.priv_addr_idx]
//
//	return vs
//}
//
//func (c *CosmosChain) MakeValidatorshash() []byte {
//	return c.MakeValidatorSet().Hash()
//}

func (c *CosmosChain) MakeCosmosSignedHeader(h *et.Header, app_hash common.Hash) *ct.SignedHeader {
	log.Debug("MakeCosmosSignedHeader")
	//validator_hash := c.MakeValidatorshash()
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32), // todo: to be replaced with
		LastBlockID:        c.blockID,
		DataHash:           h.TxHash[:],
		ValidatorsHash:     c.validators.Validators.Hash(),
		NextValidatorsHash: c.validators.NextValidators.Hash(),
		ConsensusHash:      make([]byte, 32), // todo: to be replaced with
		AppHash:            app_hash[:],      // todo: to be replaced with
		LastResultsHash:    make([]byte, 32), // todo: to be replaced with
		EvidenceHash:       make([]byte, 32), // todo: to be replaced with
		ProposerAddress:    c.validators.Validators.GetProposer().Address,
	}

	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]ct.CommitSig, c.validators.Validators.Size())

	commit := &ct.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	return &ct.SignedHeader{Header: header, Commit: commit}
	// Tendermint light.Verify()
}

func (c *CosmosChain) Vote(block_height int64, cs ct.CommitSig, light_block *ct.LightBlock) {
	// light_block := c.GetLightBlockInternal(block_height)
	val_idx := 0
	// TODO get val idx from c.valaditors (cs.ValidatorAddress)
	// todo: add other signatures
	light_block.Commit.Signatures[val_idx] = cs
	c.SetLightBlock(light_block)
	if c.IsLightBlockValid(light_block) {
		c.best_block_height = uint64(light_block.Height)
	}
}

func (c *CosmosChain) MakeLightBlock(h *et.Header, app_hash common.Hash) *ct.LightBlock {
	// TODO load validator set from h.Extra, fixed for demo
	light_block := &ct.LightBlock{SignedHeader: c.MakeCosmosSignedHeader(h, app_hash), ValidatorSet: c.validators.Validators}
	// c.SetLightBlock(light_block)
	return light_block
}

// todo: light block need to be signed with most validators
func (c *CosmosChain) MakeLightBlockAndSign(h *et.Header, app_hash common.Hash) *ct.LightBlock {

	println("make crosschain block, height --  ", h.Number.Int64(), time.Now().UTC().String())

	light_block := c.MakeLightBlock(h, app_hash)
	addr := c.priv_validator.GetAddress()
	idx, val := c.validators.Validators.GetByAddress(addr)
	if val == nil {
		panic("not a validator")
	}
	vote := &ct.Vote{
		Type:             tmproto.PrecommitType,
		Height:           light_block.Height,
		Round:            light_block.Commit.Round,
		BlockID:          light_block.Commit.BlockID,
		Timestamp:        light_block.Time,
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
	}
	v := vote.ToProto()
	c.priv_validator.SignVote(c.ChainID, v)

	cc := ct.CommitSig{}
	cc.BlockIDFlag = ct.BlockIDFlagCommit
	cc.Timestamp = vote.Timestamp
	cc.ValidatorAddress = addr
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	c.Vote(light_block.Height, cc, light_block)

	// todo: broadcast this light block

	return light_block
}

// func (c *CosmosChain) GetLightBlockInternal(block_height int64) *ct.LightBlock {
// 	light_block, ok := c.light_block[block_height]
// 	if ok {
// 		return light_block
// 	} else {
// 		return nil
// 	}
// }

func (c *CosmosChain) SetLightBlock(light_block *ct.LightBlock) {
	if len(c.light_block) > 1200*24 {
		delete(c.light_block, light_block.Header.Height-100)
	}
	c.light_block[light_block.Height] = light_block
}

func (c *CosmosChain) GetLightBlock(block_height int64) *ct.LightBlock {
	light_block, ok := c.light_block[block_height]
	if ok {
		if c.IsLightBlockValid(light_block) {
			return light_block
		} else {
			return nil
		}
	} else {
		return nil
	}
}

func (c *CosmosChain) IsLightBlockValid(light_block *ct.LightBlock) bool {
	// TODO make sure cube block is stable
	votingPowerNeeded := light_block.ValidatorSet.TotalVotingPower() * 2 / 3
	var talliedVotingPower int64
	for idx, commitSig := range light_block.Commit.Signatures {
		if commitSig.BlockIDFlag == ct.BlockIDFlagCommit {
			talliedVotingPower += int64(light_block.ValidatorSet.Validators[idx].VotingPower)
		}
	}

	return talliedVotingPower > votingPowerNeeded
}

func (c *CosmosChain) LastBlockHeight() int64 {
	return int64(c.best_block_height)
}
