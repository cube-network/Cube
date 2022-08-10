package crosschain

import (
	"bytes"
	"encoding/hex"
	"math/big"
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

type GetHeaderByNumber func(number uint64) *types.Header

var (
	state_block_number  = common.Hash{0x01, 0x01}
	state_app_hash_last = common.Hash{0x01, 0x02}
	state_root_last     = common.Hash{0x01, 0x03}
	state_app_hash_cur  = common.Hash{0x01, 0x04}
	state_root_cur      = common.Hash{0x01, 0x05}
)

func (app *CosmosApp) SetState(statedb vm.StateDB, app_hash common.Hash, state_root common.Hash, block_number int64) {
	app_hash_last := statedb.GetState(vm.CrossChainContractAddr, state_app_hash_cur)
	root_last := statedb.GetState(vm.CrossChainContractAddr, state_root_cur)

	statedb.SetState(vm.CrossChainContractAddr, state_app_hash_last, app_hash_last)
	statedb.SetState(vm.CrossChainContractAddr, state_root_last, root_last)

	statedb.SetState(vm.CrossChainContractAddr, state_app_hash_cur, app_hash)
	statedb.SetState(vm.CrossChainContractAddr, state_root_cur, state_root)

	cn := common.BigToHash(big.NewInt(block_number))
	statedb.SetState(vm.CrossChainContractAddr, state_block_number, cn)
}

func (app *CosmosApp) IsDuplicateBlock(statedb vm.StateDB, block_number int64) bool {
	tcn := common.BigToHash(big.NewInt(block_number))
	cn := statedb.GetState(vm.CrossChainContractAddr, state_block_number)

	return tcn == cn
}

func (app *CosmosApp) GetLastStateRoot(statedb vm.StateDB) common.Hash {
	// app_hash_cur := statedb.GetState(vm.CrossChainContractAddr, state_app_hash_cur)
	// root_cur := statedb.GetState(vm.CrossChainContractAddr, state_root_cur)
	// app_hash_last := statedb.GetState(vm.CrossChainContractAddr, state_app_hash_last)
	// root_last := statedb.GetState(vm.CrossChainContractAddr, state_root_last)
	// println("xxxx last ", app_hash_last.Hex(), " ", root_last.Hex())
	// println("xxxx cur ", app_hash_cur.Hex(), " ", root_cur.Hex())

	if app.is_duplicate_block {
		return statedb.GetState(vm.CrossChainContractAddr, state_root_last)
	} else {
		return statedb.GetState(vm.CrossChainContractAddr, state_root_cur)
	}
}

func (app *CosmosApp) GetAppHash(statedb vm.StateDB, block_number int64) common.Hash {
	return statedb.GetState(vm.CrossChainContractAddr, state_app_hash_cur)
}

func (app *CosmosApp) InitGenesis(evm *vm.EVM) {
	if app.is_genesis_init {
		return
	}

	init_block_height := evm.Context.BlockNumber.Int64()
	app.SetState(evm.StateDB, common.Hash{}, common.Hash{}, init_block_height)

	// Module Account
	evm.StateDB.CreateAccount(common.HexToAddress(ModuleAccount))
	// deploy erc20 factory contract
	evm.StateDB.CreateAccount(system.ERC20FactoryContract)
	code, _ := hex.DecodeString(ERC20FactoryCode)
	evm.StateDB.SetCode(system.ERC20FactoryContract, code)

	// deploy validators' register contract
	evm.StateDB.CreateAccount(system.AddrToPubkeyMapContract)
	code, _ = hex.DecodeString(AddrToPubkeyMapCode)
	evm.StateDB.SetCode(system.AddrToPubkeyMapContract, code)

	//// register validator
	//app.cc.RegisterValidator(evm)

	// crosschain
	app.LoadVersion2(0)
	var genesisState GenesisState
	if err := tmjson.Unmarshal([]byte(IBCConfig), &genesisState); err != nil {
		panic(err)
	}

	app.InitChain(abci.RequestInitChain{Time: time.Time{}, ChainId: app.cc.ChainID, InitialHeight: init_block_height})
	app.mm.InitGenesis(app.GetContextForDeliverTx([]byte{}), app.codec.Marshaler, genesisState)

	app.is_start_crosschain = true
	app.is_genesis_init = true

	app.cc.valsMgr.initGenesisValidators(evm, init_block_height)

	hdr := app.cc.MakeCosmosSignedHeader(app.header, common.Hash{})
	//hdr := app.cc.makeCosmosSignedHeader(app.db.header, common.Hash{})
	log.Info("===============initGenesis OnBlockStart", "number", evm.Context.BlockNumber.Int64())
	app.BeginBlock(abci.RequestBeginBlock{Header: *hdr.ToProto().Header})
}

func (app *CosmosApp) SetGetHeaderFn(getHeaderFn GetHeaderByNumber) {
	app.cc.SetGetHeaderFn(getHeaderFn)
}

// TODO get cube block header instead
func (app *CosmosApp) Load() {
	init_block_height := app.header.Number.Int64() - 1
	if !app.is_start_crosschain || app.is_duplicate_block {
		println("load version... ", init_block_height)
		app.LoadVersion2(init_block_height)
		app.is_start_crosschain = true
	}
}

func (app *CosmosApp) OnBlockBegin(config *params.ChainConfig, blockContext vm.BlockContext, statedb *state.StateDB, header *types.Header, cfg vm.Config, fromMine bool) {
	// app.bapp_mu.Lock()
	// defer app.bapp_mu.Unlock()

	app.header = header
	app.is_duplicate_block = app.IsDuplicateBlock(statedb, header.Number.Int64())
	state_root := app.GetLastStateRoot(statedb)
	app.is_genesis_init = app.db.SetEVM(config, blockContext, state_root, cfg)
	println("begin block height", header.Number.Int64(), " genesis init ", app.is_genesis_init, " duplicat block ", app.is_duplicate_block, " stateroot ", state_root.Hex(), " ts ", time.Now().UTC().String())

	if !app.is_genesis_init {
		// app.InitGenesis(app.db.evm)
		return
	} else {
		app.Load()
		var hdr *ct.SignedHeader
		if fromMine {
			hdr = app.cc.makeCosmosSignedHeader(header, common.Hash{})
		} else {
			hdr = app.cc.getSignedHeader(header.Hash())
		}
		//hdr := app.cc.MakeCosmosSignedHeader(header, common.Hash{})
		app.BeginBlock(abci.RequestBeginBlock{Header: *hdr.ToProto().Header})
	}
}

func (app *CosmosApp) CommitIBC(statedb *state.StateDB) {
	// app.bapp_mu.Lock()
	// defer app.bapp_mu.Unlock()

	if !app.is_genesis_init {
		return
	}
	app.db.Commit(statedb)
}

func (app *CosmosApp) OnBlockEnd(statedb *state.StateDB) *state.StateDB {
	// app.bapp_mu.Lock()
	// defer app.bapp_mu.Unlock()

	if !app.is_genesis_init {
		return nil
	}

	c := app.BaseApp.Commit()
	state_root := app.db.IntermediateRoot()

	app.SetState(statedb, common.BytesToHash(c.Data[:]), state_root, app.header.Number.Int64())

	println("OnBlockEnd ibc hash", hex.EncodeToString(c.Data[:]), " state root ", state_root.Hex(), " ts ", time.Now().UTC().String())

	// TODO statedb lock??
	return app.db.statedb
}

func (app *CosmosApp) MakeHeader(h *et.Header, statedb *state.StateDB) *ct.Header {
	if !app.is_genesis_init {
		return nil
	}
	// TODO
	app_hash := statedb.GetState(vm.CrossChainContractAddr, state_app_hash_cur)
	app.cc.MakeLightBlockAndSign(h, app_hash)
	println("header ", app.cc.GetLightBlock(h.Number.Int64()).Header.AppHash.String(), " ", time.Now().UTC().String())
	return app.cc.GetLightBlock(h.Number.Int64()).Header
}

func (app *CosmosApp) MakeSignedHeader(h *et.Header) *ct.SignedHeader {
	if !app.is_genesis_init {
		return nil
	}
	header := app.cc.makeCosmosSignedHeader(h, app.app_hash)
	return header
}

func (app *CosmosApp) GetSignedHeader(hash common.Hash) *ct.SignedHeader {
	if !app.is_genesis_init {
		return nil
	}
	header := app.cc.getSignedHeader(hash)
	return header
}

func (app *CosmosApp) HandleHeader(h *et.Header, header *ct.SignedHeader) error {
	if !app.is_genesis_init {
		return nil
	}

	return app.cc.handleSignedHeader(h, header)
}

func (app *CosmosApp) SetCubeAddress(addr common.Address) {
	app.cc.cubeAddr = addr
}

// TODO validator set pubkey, config for demo, register in contract later
// only one validator now, read more validator addr2pubkey mapping from conf/contract later
// validator index,pubkey

type CosmosChain struct {
	ChainID       string
	light_block   map[int64]*ct.LightBlock         // cache only for demo, write/read db instead later
	signed_header map[common.Hash]*ct.SignedHeader // cache only for demo, write/read db instead later
	// light_block    *lru.ARCCache
	//valsMgr     []*ct.Validator // fixed for demo; full validator set, fixed validator set for demo,
	valsMgr *ValidatorsMgr
	//priv_addr_idx  uint32
	priv_validator *privval.FilePV // use ed2559 for demo, secp256k1 support later;
	cubeAddr       common.Address

	blockID           ct.BlockID // load best block height later
	best_block_height uint64

	getHeaderByNumber GetHeaderByNumber
	cube_cosmos_header map[string][]byte
}

// priv_validator_addr: chaos.validator
func MakeCosmosChain(chainID string, priv_validator_key_file, priv_validator_state_file string) *CosmosChain {
	log.Debug("MakeCosmosChain")
	c := &CosmosChain{}
	// TODO chainID
	c.ChainID = "ibc-1"
	c.light_block = make(map[int64]*ct.LightBlock)
	c.signed_header = make(map[common.Hash]*ct.SignedHeader)
	c.priv_validator = privval.LoadOrGenFilePV(priv_validator_key_file, priv_validator_state_file) //privval.GenFilePV(priv_validator_key_file, priv_validator_state_file /*"secp256k1"*/)
	c.priv_validator.Save()

	pubkey, _ := c.priv_validator.GetPubKey()
	log.Info("init validator", "pubAddr", pubkey.Address().String(), "privAddr", c.priv_validator.GetAddress().String())

	// TODO load validator set, should use contract to deal with validators getting changed in the future
	c.valsMgr = &ValidatorsMgr{}

	// TODO load best block
	psh := ct.PartSetHeader{Total: 1, Hash: make([]byte, 32)}
	c.blockID = ct.BlockID{Hash: make([]byte, 32), PartSetHeader: psh}
	c.best_block_height = 0

	return c
}

func (c *CosmosChain) SetGetHeaderFn(getHeaderFn GetHeaderByNumber) {
	c.getHeaderByNumber = getHeaderFn
}

func (c *CosmosChain) String() string {
	return fmt.Sprintf(
		"Cosmos{\n ChainID:%v \n len(light_block):%v \n priv_validator:%v \n blockID:%v}",
		c.ChainID,
		len(c.light_block),
		//len(c.valsMgr),
		//c.priv_addr_idx,
		c.priv_validator,
		c.blockID,
	)
}

func (c *CosmosChain) SetCubeAddress(addr common.Address) {
	c.cubeAddr = addr
}

//func (c *CosmosChain) RegisterValidator(evm *vm.EVM) error {
//	ctx := sdk.Context{}.WithEvm(evm)
//	pubkey, err := c.priv_validator.GetPubKey()
//	if err != nil {
//		panic("GetPubKey failed")
//	}
//	// todo: pubkey to string
//	val := Validator{PubKey: pubkey, VotingPower: 100}
//	valBytes, err := tmjson.Marshal(val)
//	if err != nil {
//		panic("Marshal validator failed")
//	}
//	log.Info("Marshal", "result", string(valBytes))
//
//	_, err = systemcontract.RegisterValidator(ctx, c.cubeAddr, string(valBytes))
//	if err != nil {
//		log.Error("RegisterValidator failed", "err", err)
//	}
//	result, err := systemcontract.GetValidator(ctx, c.cubeAddr)
//	if err != nil {
//		log.Error("GetValidator failed", "err", err)
//	}
//	log.Info("GetValidator", "result", string(result))
//	var tmpVal Validator
//	err = tmjson.Unmarshal([]byte(result), &tmpVal)
//	if err != nil {
//		panic("Unmarshal validator failed")
//	}
//	if !tmpVal.PubKey.Equals(val.PubKey) {
//		panic("Conversion failed")
//	}
//
//	return err
//}

func (c *CosmosChain) getAllValidators() {

}

func (c *CosmosChain) makeCosmosSignedHeader(h *et.Header, app_hash common.Hash) *ct.SignedHeader {
	log.Info("makeCosmosSignedHeader", "height", h.Number, "hash", h.Hash())
	// TODO find_cosmos_parent_header(h.parent_hash) {return c.cube_cosmos_header[parent_hash]}
	// todo: cannot use header to update validators as validators are only updated every Epoch length to reset votes and checkpoint. see more info from chaos.Prepare()
	pubkey, _ := c.priv_validator.GetPubKey()
	addr := pubkey.Address()
	//c.valsMgr.updateValidators(h, h.Number.Int64())

	// make header
	header := &ct.Header{
		Version:            version.Consensus{Block: 11, App: 0},
		ChainID:            c.ChainID,
		Height:             h.Number.Int64(),
		Time:               time.Unix(int64(h.Time), 0),
		LastCommitHash:     make([]byte, 32), // todo: to be changed
		LastBlockID:        c.blockID,
		DataHash:           h.TxHash[:],
		ValidatorsHash:     c.valsMgr.Validators.Hash(),
		NextValidatorsHash: c.valsMgr.NextValidators.Hash(),
		ConsensusHash:      make([]byte, 32), // todo: to be changed
		AppHash:            app_hash[:],
		LastResultsHash:    make([]byte, 32), // todo: to be changed
		EvidenceHash:       make([]byte, 32), // todo: to be changed
		ProposerAddress:    addr,             //c.valsMgr.Validators.GetProposer().Address,
	}

	// TODO
	// c.cube_cosmos_header[h.hash] = header.hash
	// save leveldb

	psh := ct.PartSetHeader{Total: 1, Hash: header.Hash()}
	c.blockID = ct.BlockID{Hash: header.Hash(), PartSetHeader: psh}
	signatures := make([]ct.CommitSig, c.valsMgr.Validators.Size())

	commit := &ct.Commit{Height: header.Height, Round: 1, BlockID: c.blockID, Signatures: signatures}
	signedHeader := &ct.SignedHeader{Header: header, Commit: commit}

	c.voteSignedHeader(signedHeader)
	// store header
	c.storeSignedHeader(h.Hash(), signedHeader)

	return signedHeader
}

func (c *CosmosChain) voteSignedHeader(header *ct.SignedHeader) {
	pubkey, _ := c.priv_validator.GetPubKey()
	addr := pubkey.Address()
	idx, val := c.valsMgr.Validators.GetByAddress(addr)
	if val == nil {
		log.Error("voteSignedHeader", "cosmosAddr", addr.String())
		panic("not a validator")
	}
	vote := &ct.Vote{
		Type:             tmproto.PrecommitType,
		Height:           header.Height,
		Round:            header.Commit.Round,
		BlockID:          header.Commit.BlockID,
		Timestamp:        header.Time,
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
	}
	v := vote.ToProto()
	c.priv_validator.SignVote(c.ChainID, v)

	cc := ct.CommitSig{}
	cc.BlockIDFlag = ct.BlockIDFlagCommit
	cc.ValidatorAddress = addr
	cc.Timestamp = v.Timestamp
	cc.Signature = v.Signature

	commit := header.Commit
	commit.Signatures[idx] = cc
	header.Commit = commit
}

func (c *CosmosChain) handleSignedHeader(h *et.Header, header *ct.SignedHeader) error {
	log.Info("handleSignedHeader", "height", h.Number, "hash", h.Hash())

	if err := header.ValidateBasic(c.ChainID); err != nil {
		return err
	}

	// check state_root
	var stateRoot common.Hash
	copy(stateRoot[:], h.Extra[:32])

	// check validators
	_, vals := c.valsMgr.getValidators(h)
	if !bytes.Equal(header.ValidatorsHash, vals.Hash()) {
		return fmt.Errorf("Verify validatorsHash failed. number=%f hash=%s\n", h.Number, h.Hash())
	}
	// check proposer
	proposer := c.valsMgr.getValidator(h.Coinbase)
	if !bytes.Equal(proposer.Address, header.ProposerAddress) {
		return fmt.Errorf("Verify proposer failed. number=%f hash=%s\n", h.Number, h.Hash())
	}

	// check votes
	sigs := header.Commit.Signatures
	if len(sigs) < 1 {
		return fmt.Errorf("Commit signatures are wrong. number=%f hash=%s\n", h.Number, h.Hash())
	}

	//voteSet := ct.CommitToVoteSet(c.ChainID, header.Commit, vals)
	// check proposer's signature
	idx, val := vals.GetByAddress(proposer.Address)
	vote := header.Commit.GetByIndex(idx) //voteSet.GetByIndex(idx)
	if err := vote.Verify(c.ChainID, val.PubKey); err != nil {
		return fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", c.ChainID, val.PubKey, err)
	}

	// todo: check other signatures

	// store header
	c.storeSignedHeader(h.Hash(), header)

	return nil
}

func (c *CosmosChain) storeSignedHeader(hash common.Hash, header *ct.SignedHeader) {
	c.signed_header[hash] = header
	log.Info("store signed header", "hash", hash, "header", header.Hash())
}

//func (c *CosmosChain) verifySignature(validators *ct.ValidatorSet) error {
//	addr := c.priv_validator.GetAddress()
//	idx, val := validators.GetByAddress(addr)
//	if val == nil {
//		panic("not a validator")
//	}
//
//}
//
//func (c *CosmosChain) Vote(block_height int64, cs ct.CommitSig, light_block *ct.LightBlock) {
//	// light_block := c.GetLightBlockInternal(block_height)
//	val_idx := 0
//	// TODO get val idx from c.valaditors (cs.ValidatorAddress)
//	// todo: add other signatures
//	light_block.Commit.Signatures[val_idx] = cs
//	c.SetLightBlock(light_block)
//	if c.IsLightBlockValid(light_block) {
//		c.best_block_height = uint64(light_block.Height)
//	}
//}
//
//func (c *CosmosChain) MakeLightBlock(h *et.Header, app_hash common.Hash) *ct.LightBlock {
//	// TODO load validator set from h.Extra, fixed for demo
//	light_block := &ct.LightBlock{SignedHeader: c.makeCosmosSignedHeader(h, app_hash), ValidatorSet: c.valsMgr.Validators}
//	// c.SetLightBlock(light_block)
//	return light_block
//}
//
//// todo: light block need to be signed with most valsMgr
//func (c *CosmosChain) MakeLightBlockAndSign(h *et.Header, app_hash common.Hash) *ct.LightBlock {
//
//	println("make crosschain block, height --  ", h.Number.Int64(), time.Now().UTC().String())
//
//	light_block := c.MakeLightBlock(h, app_hash)
//	addr := c.priv_validator.GetAddress()
//	idx, val := c.valsMgr.Validators.GetByAddress(addr)
//	if val == nil {
//		panic("not a validator")
//	}
//	vote := &ct.Vote{
//		Type:             tmproto.PrecommitType,
//		Height:           light_block.Height,
//		Round:            light_block.Commit.Round,
//		BlockID:          light_block.Commit.BlockID,
//		Timestamp:        light_block.Time,
//		ValidatorAddress: addr,
//		ValidatorIndex:   idx,
//	}
//	v := vote.ToProto()
//	c.priv_validator.SignVote(c.ChainID, v)
//
//	cc := ct.CommitSig{}
//	cc.BlockIDFlag = ct.BlockIDFlagCommit
//	cc.Timestamp = vote.Timestamp
//	cc.ValidatorAddress = addr
//	cc.Timestamp = v.Timestamp
//	cc.Signature = v.Signature
//
//	c.Vote(light_block.Height, cc, light_block)
//
//	// todo: broadcast this light block
//
//	return light_block
//}
//
//func (c *CosmosChain) SetLightBlock(light_block *ct.LightBlock) {
//	if len(c.light_block) > 1200*24 {
//		delete(c.light_block, light_block.Header.Height-100)
//	}
//	c.light_block[light_block.Height] = light_block
//}

func (c *CosmosChain) getSignedHeader(hash common.Hash) *ct.SignedHeader {
	log.Info("============getSignedHeader", "hash", hash)
	return c.signed_header[hash]
}

func (c *CosmosChain) GetLightBlock(block_height int64) *ct.LightBlock {
	h := c.getHeaderByNumber(uint64(block_height))
	if h == nil {
		log.Error("Cannot get block header", "number", block_height)
		return nil
	}
	header := c.getSignedHeader(h.Hash())
	if header == nil {
		log.Error("Cannot get cosmos signed header", "number", block_height)
		return nil
	}

	// make light block
	_, validators := c.valsMgr.getValidators(h)
	return &ct.LightBlock{SignedHeader: header, ValidatorSet: validators}

	//light_block, ok := c.light_block[block_height]
	//if ok {
	//	if c.IsLightBlockValid(light_block) {
	//		return light_block
	//	} else {
	//		return nil
	//	}
	//} else {
	//	return nil
	//}
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
