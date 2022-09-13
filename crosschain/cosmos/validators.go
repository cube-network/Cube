package cosmos

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/crosschain/cosmos/systemcontract"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	lru "github.com/hashicorp/golang-lru"
	tmcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type ValidatorPubkey struct {
	//CubeAddr    common.Address  `json:"address"`
	PubKey tmcrypto.PubKey `json:"pub_key"`
	//VotingPower int64           `json:"voting_power"`
}

type SimplifiedValidator struct {
	PubKey      tmcrypto.PubKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
}

// MsgRegisterValidator defines a msg to register cosmos validator
type MsgRegisterValidator struct {
	Address     []byte          `json:"address"`
	PubKey      tmcrypto.PubKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
}

type ValidatorsMgr struct {
	ethdb             ethdb.Database
	blockContext      vm.BlockContext
	statefn           cccommon.StateFn
	AddrValMapCache   *lru.ARCCache
	valsetCache       *lru.ARCCache
	config            *params.ChainConfig
	getHeaderByNumber cccommon.GetHeaderByNumberFn
	getHeaderByHash   cccommon.GetHeaderByHashFn

	privVal *privval.FilePV
}

func NewValidatorsMgr(ethdb ethdb.Database, blockContext vm.BlockContext, config *params.ChainConfig, privVal *privval.FilePV, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, statefn cccommon.StateFn) *ValidatorsMgr {
	valMgr := &ValidatorsMgr{
		ethdb:             ethdb,
		blockContext:      blockContext,
		statefn:           statefn,
		config:            config,
		privVal:           privVal,
		getHeaderByNumber: headerfn,
		getHeaderByHash:   headerhashfn,
	}

	valMgr.AddrValMapCache, _ = lru.NewARC(32)
	valMgr.valsetCache, _ = lru.NewARC(16)

	return valMgr
}

func (vmgr *ValidatorsMgr) initGenesisValidators(evm *vm.EVM, vals []params.CosmosValidator) error {
	if len(vals) == 0 {
		log.Warn("vals is empty")
		return nil
	}

	validators := make([]*types.Validator, len(vals))
	AddrValMap := make(map[common.Address]*types.Validator, len(vals))

	for i, val := range vals {
		var valPubkey ValidatorPubkey
		pubkeyStr := fmt.Sprintf("{\n    \"pub_key\":{\n        \"type\":\"tendermint/PubKeyEd25519\",\n        \"value\":\"%s\"\n    }\n}", val.PubKey)
		if err := tmjson.Unmarshal([]byte(pubkeyStr), &valPubkey); err != nil {
			panic(err)
		}
		sVal := &SimplifiedValidator{PubKey: valPubkey.PubKey, VotingPower: val.VotingPower.Int64()}
		valBytes, err := tmjson.Marshal(sVal)
		if err != nil {
			panic("Marshal validator failed")
		}
		log.Info("Marshal", "result", string(valBytes))

		_, err = systemcontract.RegisterValidator(evm, val.CubeAddr, string(valBytes))
		if err != nil {
			log.Error("RegisterValidator failed", "err", err)
		}
		result, err := systemcontract.GetValidator(evm, val.CubeAddr)
		if err != nil {
			log.Error("GetValidator failed", "err", err)
		}
		log.Info("GetValidator", "result", result)
		var tmpVal types.Validator
		err = tmjson.Unmarshal([]byte(result), &tmpVal)
		if err != nil {
			panic("Unmarshal validator failed")
		}
		if !tmpVal.PubKey.Equals(valPubkey.PubKey) {
			panic("Conversion failed")
		}

		tVal := types.NewValidator(valPubkey.PubKey, val.VotingPower.Int64())
		validators[i] = tVal
		AddrValMap[val.CubeAddr] = tVal
	}

	return nil
}

func (vmgr *ValidatorsMgr) getNextValidators(height uint64) ([]common.Address, *types.ValidatorSet) {
	if height%vmgr.config.Chaos.Epoch != vmgr.config.Chaos.Epoch-1 {
		return vmgr.getValidators(height)
	}

	var vheight uint64 = 0
	if height == 0 {
		vheight = 0
	} else {
		vheight = height - 199
	}
	return vmgr.getValidatorsImpl(vheight)
}

func (vmgr *ValidatorsMgr) getValidators(height uint64) ([]common.Address, *types.ValidatorSet) {
	//log.Debug("getValidators", "height", strconv.Itoa(int(height)))
	var vheight uint64 = 0
	if height < vmgr.config.Chaos.Epoch*2 {
		vheight = 0
	} else {
		vheight = height - vmgr.config.Chaos.Epoch - height%vmgr.config.Chaos.Epoch
	}
	return vmgr.getValidatorsImpl(vheight)
}

func (vmgr *ValidatorsMgr) getValidatorsImpl(vheight uint64) ([]common.Address, *types.ValidatorSet) {
	//log.Debug("getValidatorsImpl", "height", strconv.Itoa(int(vheight)))
	vh := vmgr.getHeaderByNumber(vheight)
	if vh == nil {
		log.Warn("getValidatorsImpl get header is nil ", "height", strconv.Itoa(int(vheight)))
		return []common.Address{}, nil
	}
	vals := vmgr.getAddrValMap(vh)
	if vals == nil {
		log.Warn("getValidatorsImpl getAddrValMap is nil", "height", strconv.Itoa(int(vheight)))
		return []common.Address{}, nil
	}
	addrs := getAddressesFromHeader(vh, IsEnable(vmgr.config, big.NewInt(int64(vheight)))) // make([]common.Address, 1) //
	if m, ok := vmgr.valsetCache.Get(vheight); ok {
		return addrs, m.(*types.ValidatorSet)
	}

	count := len(addrs)
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		val := vals[addrs[i]]
		if val == nil {
			log.Debug("getValidatorsImpl getValidators val is nil, fill with default", "height", strconv.Itoa(int(vheight)), "index", i, "cubeAddr", addrs[i].String())

			pubkeyBytes := make([]byte, ed25519.PubKeySize)
			copy(pubkeyBytes, strconv.Itoa(i))
			pk := ed25519.PubKey(pubkeyBytes)
			tVal := types.NewValidator(pk, 100)
			validators[i] = tVal
		} else {
			tVal := types.NewValidator(val.PubKey, val.VotingPower)
			validators[i] = tVal
			// log.Debug("getValidators height ", strconv.Itoa(int(vheight)), " index ", i, "cubeAddr", addrs[i].String(), "cosmosAddr", val.PubKey.Address().String(), " pk ", val.PubKey.Address().String())
		}
	}
	// return addrs, types.NewValidatorSet(validators)

	vs := &types.ValidatorSet{}
	vs.Validators = validators
	vs.Proposer = validators[0]
	vmgr.valsetCache.Add(vheight, vs)
	return addrs, vs
}

func (vmgr *ValidatorsMgr) getValidator(cubeAddr common.Address, header *et.Header) *types.Validator {
	var vheight uint64 = 0
	if header.Number.Uint64() < vmgr.config.Chaos.Epoch*2 {
		vheight = 0
	} else {
		vheight = header.Number.Uint64() - vmgr.config.Chaos.Epoch - header.Number.Uint64()%vmgr.config.Chaos.Epoch
	}

	vh := vmgr.getHeaderByNumber(vheight)
	if vh == nil {
		log.Warn("getValidatorsImpl get header is nil", "height", strconv.Itoa(int(vheight)))
		return nil
	}
	m := vmgr.getAddrValMap(vh)
	if m != nil {
		return m[cubeAddr]
	} else {
		return nil
	}
}

func (vmgr *ValidatorsMgr) getAddrValMap(header *et.Header) map[common.Address]*types.Validator {
	// TODO lock ???
	if m, ok := vmgr.AddrValMapCache.Get(header.Number.Uint64()); ok {
		return m.(map[common.Address]*types.Validator)
	}

	log.Debug("getAddrValMap", "height", strconv.Itoa(int(header.Number.Int64())), "hash", header.Hash().Hex())

	addrs := getAddressesFromHeader(header, IsEnable(vmgr.config, header.Number))

	key := makeValidatorKey(header.Hash())
	bz, err := vmgr.ethdb.Get(key)
	if err != nil {
		log.Warn("getAddrValMap read addr map fail", "err", err.Error(), strconv.Itoa(int(header.Number.Int64())), "key", string(key))
		return nil
	}
	if len(bz) == 0 {
		log.Warn("getAddrValMap read addr map is nil", "height", strconv.Itoa(int(header.Number.Int64())), "key", string(key))
		return nil
	}

	tvs := &tmproto.ValidatorSet{}
	err = tvs.Unmarshal(bz)
	if err != nil {
		log.Warn("getAddrValMap unmarshal validator set fail", "err", err.Error(), strconv.Itoa(int(header.Number.Int64())), "key", string(key))
		return nil
	}

	vs, _ := types.ValidatorSetFromProto(tvs)
	if len(addrs) != len(vs.Validators) {
		log.Warn("getAddrValMap addr/ validator size not match", "height", strconv.Itoa(int(header.Number.Int64())), "key", string(key))
		return nil
	}

	log.Debug("ethdb validator set", "size", strconv.Itoa(len(addrs)), "key", string(key))
	AddrValMap := make(map[common.Address]*types.Validator, 0)
	for i := 0; i < len(addrs); i++ {
		AddrValMap[addrs[i]] = vs.Validators[i]
		log.Info("getAddrValMap from ethdb", "height", strconv.Itoa(int(header.Number.Uint64())), "addr", addrs[i].Hex(), "cosmosAddr", vs.Validators[i].Address.String(), "cosmosAddr", vs.Validators[i].PubKey.Address().String())
	}

	vmgr.AddrValMapCache.Add(header.Number.Uint64(), AddrValMap)
	return AddrValMap
}

func (vmgr *ValidatorsMgr) getAddrValMapFromContract(h *et.Header) map[common.Address]*types.Validator {
	header := vmgr.getHeaderByHash(h.ParentHash)
	if header == nil {
		log.Warn("getAddrValMap header is nil", "hash", h.ParentHash.Hex(), "height", strconv.Itoa(int(h.Number.Int64())))
		return nil
	}

	var statedb *state.StateDB = nil
	var err error = nil
	statedb, err = vmgr.statefn(header.Root)
	if err != nil {
		log.Warn("getAddrValMap make statedb fail! ", "root", header.Root.Hex(), "height", strconv.Itoa(int(header.Number.Int64())))
		return nil
	}

	ctx := vmgr.blockContext
	vm := makeContext(ctx, vmgr.config, header, statedb)
	addrs, vals, err := systemcontract.GetAllValidators(vm)
	if err != nil {
		log.Warn("getAddrValMap read cosmos pk from contract fail!", "height", strconv.Itoa(int(header.Number.Int64())))
		return nil
	}

	log.Debug("contract", "valSize", strconv.Itoa(len(addrs)), "height", strconv.Itoa(int(header.Number.Int64())))
	AddrValMap := make(map[common.Address]*types.Validator, 0)
	for i := 0; i < len(addrs); i++ {
		tmpVal := &types.Validator{}
		err = tmjson.Unmarshal([]byte(vals[i]), tmpVal)
		if err != nil {
			log.Error("getAddrValMap Unmarshal validator failed", "height", strconv.Itoa(int(header.Number.Int64())))
			return nil
		}
		AddrValMap[addrs[i]] = tmpVal
		log.Info("getAddrValMapFromContract register validator", "height", strconv.Itoa(int(header.Number.Uint64())), "addr", addrs[i].Hex(), "cosmos addr", tmpVal.Address.String(), "cosmosAddr", tmpVal.PubKey.Address().String())
	}

	for k, v := range AddrValMap {
		log.Debug("AddrValMap", "k", k.Hex(), "v", v.Address.String())
	}

	return AddrValMap
}

func (vmgr *ValidatorsMgr) storeValidatorSet(header *et.Header) {
	if header.Number.Uint64()%vmgr.config.Chaos.Epoch != 0 {
		return
	}

	vals := vmgr.getAddrValMapFromContract(header)
	if vals == nil {
		log.Warn("storeValidatorSet getAddrValMap is nil", "height", strconv.Itoa(int(header.Number.Int64())), "hash", header.Hash().Hex())
		return
	}

	addrs := getAddressesFromHeader(header, IsEnable(vmgr.config, header.Number))
	count := len(addrs)
	validators := make([]*types.Validator, count)

	for i := 0; i < count; i++ {
		val := vals[addrs[i]]
		if val == nil {
			log.Debug("getValidators val is nil, fill with default", "index", i, "cubeAddr", addrs[i].String())

			pubkeyBytes := make([]byte, ed25519.PubKeySize)
			copy(pubkeyBytes, []byte(strconv.Itoa(i)))
			pk := ed25519.PubKey(pubkeyBytes)
			tVal := types.NewValidator(pk, 100)
			validators[i] = tVal
		} else {
			tVal := types.NewValidator(val.PubKey, val.VotingPower)
			validators[i] = tVal
			log.Debug("storeValidatorSet", "height", strconv.Itoa(int(header.Number.Uint64())), "index", i, "cubeAddr", addrs[i].String(), " cosmosAddr ", val.Address.String(), "cosmosAddr", val.PubKey.Address().String(), "pk", val.PubKey.Address().String())
		}
	}
	vs := &types.ValidatorSet{}
	vs.Validators = validators
	vs.Proposer = validators[0]
	// vs := types.NewValidatorSet(validators)
	tpvs, err := vs.ToProto()
	if err != nil {
		log.Error("store validator set to proto", "height", strconv.Itoa(int(header.Number.Int64())), "hash", header.Hash().Hex(), "err", err.Error())
		return
	}
	bz, err := tpvs.Marshal()
	if err != nil {
		log.Error("store validator set marshal", "height", strconv.Itoa(int(header.Number.Int64())), "hash", header.Hash().Hex(), "err", err.Error())
		return
	}
	key := makeValidatorKey(header.Hash())

	err = vmgr.ethdb.Put(key, bz)
	if err != nil {
		log.Error("store validator fail", "height", strconv.Itoa(int(header.Number.Int64())), "hash", header.Hash().Hex(), "err", err.Error())
		return
	}

	log.Info("store validator number ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex(), " key ", string(key), " val ", hex.EncodeToString(bz))
}

func getAddressesFromHeader(h *et.Header, isCosmosEnable bool) []common.Address {
	extraVanity := 32                   // Fixed number of extra-data prefix bytes reserved for validator vanity
	extraSeal := crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for validator seal
	extraCosmosAppHash := 0
	if isCosmosEnable {
		extraCosmosAppHash = 32
	}
	validatorsBytes := len(h.Extra) - extraVanity - extraSeal - extraCosmosAppHash

	count := validatorsBytes / common.AddressLength

	addresses := make([]common.Address, count)
	for i := 0; i < count; i++ {
		copy(addresses[i][:], h.Extra[extraVanity+extraCosmosAppHash+i*common.AddressLength:])
	}
	return addresses
}
