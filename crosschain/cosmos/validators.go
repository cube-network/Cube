package cosmos

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/system"
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

type Validator struct {
	CubeAddr common.Address `json:"address"`
	//CosmosAddr  types.Address   `json:"cosmos_address"`
	PubKey      tmcrypto.PubKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
}

type SimplifiedValidator struct {
	PubKey      tmcrypto.PubKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
}

// MsgRegisterValidator defines a msg to register cosmos validator
type MsgRegisterValidator struct {
	//common.Address
	Address     []byte          `json:"address"`
	PubKey      tmcrypto.PubKey `json:"pub_key"`
	VotingPower int64           `json:"voting_power"`
}

type ValidatorsMgr struct {
	ethdb        ethdb.Database
	blockContext vm.BlockContext
	statefn      cccommon.StateFn
	// AddrValMap        map[common.Address]*types.Validator // cube address => cosmos validator
	AddrValMapCache   *lru.ARCCache
	config            *params.ChainConfig
	getHeaderByNumber cccommon.GetHeaderByNumberFn
	getHeaderByHash   cccommon.GetHeaderByHashFn
	getNonce          cccommon.GetNonceFn
	getPrice          cccommon.GetPriceFn
	signTx            cccommon.SignTxFn
	addLocalTx        cccommon.AddLocalTxFn

	privVal *privval.FilePV
	//registered bool
}

func NewValidatorsMgr(ethdb ethdb.Database, blockContext vm.BlockContext, config *params.ChainConfig, privVal *privval.FilePV, headerfn cccommon.GetHeaderByNumberFn, headerhashfn cccommon.GetHeaderByHashFn, statefn cccommon.StateFn) *ValidatorsMgr {
	valMgr := &ValidatorsMgr{
		ethdb:        ethdb,
		blockContext: blockContext,
		statefn:      statefn,
		// AddrValMap:        make(map[common.Address]*types.Validator, 0),
		config:            config,
		privVal:           privVal,
		getHeaderByNumber: headerfn,
		getHeaderByHash:   headerhashfn,
	}

	valMgr.AddrValMapCache, _ = lru.NewARC(32)

	return valMgr
}

//func (vmgr *ValidatorsMgr) initGenesisValidators(evm *vm.EVM, height int64) error {
//	var vals []Validator
//	if err := tmjson.Unmarshal([]byte(ValidatorsConfig), &vals); err != nil {
//		panic(err)
//	}
//
//	validators := make([]*types.Validator, len(vals))
//	vmgr.AddrValMap = make(map[common.Address]*types.Validator, len(vals))
//	//ctx := sdk.Context{}.WithEvm(evm)
//	for i, val := range vals {
//		sVal := &SimplifiedValidator{PubKey: val.PubKey, VotingPower: val.VotingPower}
//		valBytes, err := tmjson.Marshal(sVal)
//		if err != nil {
//			panic("Marshal validator failed")
//		}
//		log.Info("Marshal", "result", string(valBytes))
//
//		_, err = systemcontract.RegisterValidator(evm, val.CubeAddr, string(valBytes))
//		if err != nil {
//			log.Error("RegisterValidator failed", "err", err)
//		}
//		result, err := systemcontract.GetValidator(evm, val.CubeAddr)
//		if err != nil {
//			log.Error("GetValidator failed", "err", err)
//		}
//		log.Info("GetValidator", "result", result)
//		var tmpVal types.Validator
//		err = tmjson.Unmarshal([]byte(result), &tmpVal)
//		if err != nil {
//			panic("Unmarshal validator failed")
//		}
//		if !tmpVal.PubKey.Equals(val.PubKey) {
//			panic("Conversion failed")
//		}
//
//		tVal := types.NewValidator(val.PubKey, val.VotingPower)
//		validators[i] = tVal
//		vmgr.AddrValMap[val.CubeAddr] = tVal
//	}
//
//	return nil
//}

func (vmgr *ValidatorsMgr) getNextValidators(height uint64) ([]common.Address, *types.ValidatorSet) {
	if height%200 != 199 {
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
	log.Debug("getValidators ", strconv.Itoa(int(height)))
	var vheight uint64 = 0
	if height < 400 {
		vheight = 0
	} else {
		vheight = height - 200 - height%200
	}
	return vmgr.getValidatorsImpl(vheight)
}

func (vmgr *ValidatorsMgr) getValidatorsImpl(vheight uint64) ([]common.Address, *types.ValidatorSet) {
	log.Debug("getValidatorsImpl ", strconv.Itoa(int(vheight)))
	vh := vmgr.getHeaderByNumber(vheight)
	if vh == nil {
		log.Warn("getValidatorsImpl get header is nil ", strconv.Itoa(int(vheight)))
		return []common.Address{}, nil
	}
	vals := vmgr.getAddrValMap(vh)
	if vals == nil {
		log.Warn("getValidatorsImpl getAddrValMap is nil", strconv.Itoa(int(vheight)))
		return []common.Address{}, nil
	}

	addrs := getAddressesFromHeader(vh, IsEnable(vmgr.config, big.NewInt(int64(vheight)))) // make([]common.Address, 1) //
	count := len(addrs)
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		val := vals[addrs[i]]
		if val == nil {
			log.Debug("getValidatorsImpl getValidators val is nil, fill with default, height ", strconv.Itoa(int(vheight)), " index ", i, "cubeAddr", addrs[i].String())

			pubkeyBytes := make([]byte, ed25519.PubKeySize)
			copy(pubkeyBytes, []byte(strconv.Itoa(i)))
			pk := ed25519.PubKey(pubkeyBytes)
			tVal := types.NewValidator(pk, 100)
			validators[i] = tVal
		} else {
			tVal := types.NewValidator(val.PubKey, val.VotingPower)
			validators[i] = tVal
			log.Debug("getValidators height ", strconv.Itoa(int(vheight)), " index ", i, "cubeAddr", addrs[i].String(), "cosmosAddr", val.PubKey.Address().String(), " pk ", val.PubKey.Address().String())
		}
	}
	// return addrs, types.NewValidatorSet(validators)

	vs := &types.ValidatorSet{}
	vs.Validators = validators
	vs.Proposer = validators[0]
	return addrs, vs
}

func (vmgr *ValidatorsMgr) getValidator(cubeAddr common.Address, header *et.Header) *types.Validator {
	var vheight uint64 = 0
	if header.Number.Uint64() < 400 {
		vheight = 0
	} else {
		vheight = header.Number.Uint64() - 200 - header.Number.Uint64()%200
	}

	vh := vmgr.getHeaderByNumber(vheight)
	if vh == nil {
		log.Warn("getValidatorsImpl get header is nil ", strconv.Itoa(int(vheight)))
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

	log.Debug("getAddrValMap height ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex())

	addrs := getAddressesFromHeader(header, IsEnable(vmgr.config, header.Number))

	key := makeValidatorKey(header.Hash())
	bz, err := vmgr.ethdb.Get(key)
	if err != nil {
		log.Warn("getAddrValMap read addr map fail ", err.Error(), strconv.Itoa(int(header.Number.Int64())), " key ", string(key))
		return nil
	}
	if len(bz) == 0 {
		log.Warn("getAddrValMap read addr map is nil", strconv.Itoa(int(header.Number.Int64())), " key ", string(key))
		return nil
	}

	tvs := &tmproto.ValidatorSet{}
	err = tvs.Unmarshal(bz)
	if err != nil {
		log.Warn("getAddrValMap unmarshal validator set fail ", err.Error(), strconv.Itoa(int(header.Number.Int64())), " key ", string(key))
		return nil
	}

	vs, _ := types.ValidatorSetFromProto(tvs)
	if len(addrs) != len(vs.Validators) {
		log.Warn("getAddrValMap addr/ validator size not match", strconv.Itoa(int(header.Number.Int64())), " key ", string(key))
		return nil
	}

	log.Debug("ethdb validator set size ", strconv.Itoa(len(addrs)), " key ", string(key))
	AddrValMap := make(map[common.Address]*types.Validator, 0)
	for i := 0; i < len(addrs); i++ {
		AddrValMap[addrs[i]] = vs.Validators[i]
		log.Info("getAddrValMap from ethdb height ", strconv.Itoa(int(header.Number.Uint64())), " addr ", addrs[i].Hex(), " cosmos addr ", vs.Validators[i].Address.String(), "cosmosAddr", vs.Validators[i].PubKey.Address().String())
	}

	vmgr.AddrValMapCache.Add(header.Number.Uint64(), AddrValMap)
	return AddrValMap
}

func (vmgr *ValidatorsMgr) getAddrValMapFromContract(h *et.Header) map[common.Address]*types.Validator {
	header := vmgr.getHeaderByHash(h.ParentHash)
	if header == nil {
		log.Warn("getAddrValMap header is nil ", h.ParentHash.Hex(), strconv.Itoa(int(h.Number.Int64())))
		return nil
	}

	var statedb *state.StateDB = nil
	var err error = nil
	statedb, err = vmgr.statefn(header.Root)
	if err != nil {
		log.Warn("getAddrValMap make statedb fail! ", header.Root.Hex(), strconv.Itoa(int(header.Number.Int64())))
		return nil
	}

	ctx := vmgr.blockContext
	vm := makeContext(ctx, vmgr.config, header, statedb)
	addrs, vals, err := systemcontract.GetAllValidators(vm)
	if err != nil {
		log.Warn("getAddrValMap read cosmos pk from contract fail!", strconv.Itoa(int(header.Number.Int64())))
		return nil
	}

	log.Debug("contract val size ", strconv.Itoa(len(addrs)), strconv.Itoa(int(header.Number.Int64())))
	AddrValMap := make(map[common.Address]*types.Validator, 0)
	for i := 0; i < len(addrs); i++ {
		tmpVal := &types.Validator{}
		err = tmjson.Unmarshal([]byte(vals[i]), tmpVal)
		if err != nil {
			log.Error("getAddrValMap Unmarshal validator failed", strconv.Itoa(int(header.Number.Int64())))
			return nil
		}
		AddrValMap[addrs[i]] = tmpVal
		log.Info("getAddrValMapFromContract register validator height ", strconv.Itoa(int(header.Number.Uint64())), " addr ", addrs[i].Hex(), " cosmos addr ", tmpVal.Address.String(), "cosmosAddr", tmpVal.PubKey.Address().String())
	}

	for k, v := range AddrValMap {
		log.Debug("k ", k.Hex(), " v ", v.Address.String())
	}

	return AddrValMap
}

func (vmgr *ValidatorsMgr) storeValidatorSet(header *et.Header) {
	if header.Number.Uint64()%200 != 0 {
		return
	}

	vals := vmgr.getAddrValMapFromContract(header)
	if vals == nil {
		log.Warn("storeValidatorSet getAddrValMap is nil ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex())
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
			log.Debug("storeValidatorSet height ", strconv.Itoa(int(header.Number.Uint64())), " index ", i, "cubeAddr", addrs[i].String(), " cosmos addr ", val.Address.String(), "cosmosAddr", val.PubKey.Address().String(), " pk ", val.PubKey.Address().String())
		}
	}
	vs := &types.ValidatorSet{}
	vs.Validators = validators
	vs.Proposer = validators[0]
	// vs := types.NewValidatorSet(validators)
	tpvs, err := vs.ToProto()
	if err != nil {
		log.Error("store validator set to proto ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex(), " err ", err.Error())
		return
	}
	bz, err := tpvs.Marshal()
	if err != nil {
		log.Error("store validator set marshal ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex(), " err ", err.Error())
		return
	}
	key := makeValidatorKey(header.Hash())

	err = vmgr.ethdb.Put(key, bz)
	if err != nil {
		log.Error("store validator fail ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex(), " err ", err.Error())
		return
	}

	log.Debug("store validator number ", strconv.Itoa(int(header.Number.Int64())), " hash ", header.Hash().Hex(), " key ", string(key), " val ", hex.EncodeToString(bz))

	// // // TODO prune validator set
	// if header.Number.Int64() > 200*5+vmgr.config.CrosschainCosmosBlock.Int64() {
	// 	hd := vmgr.getHeaderByNumber(header.Number.Uint64() - 1000)
	// 	if hd == nil {
	// 		log.Warn("getAddrValMap header hd is nil ", header.Hash().Hex())
	// 		return
	// 	}
	// 	key = makeValidatorKey(hd.Hash())
	// 	vmgr.ethdb.Delete(key)
	// }
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

func (vmgr *ValidatorsMgr) registerValidator(cubeAddr common.Address, privVal *privval.FilePV, chainID *big.Int, header *et.Header) {
	// todo: check whether this node is registered
	if vmgr.getValidator(cubeAddr, header) != nil {
		return
	}

	if vmgr.getNonce == nil || vmgr.getPrice == nil || vmgr.signTx == nil || vmgr.addLocalTx == nil {
		log.Error("Basic functions are missing")
		return
	}

	log.Info("generate register validator tx", "cubeAddr", cubeAddr.Hex(), "cosmosAddr", privVal.GetAddress().String())

	// make cosmos tx
	pubkey, _ := privVal.GetPubKey()
	sVal := &MsgRegisterValidator{Address: cubeAddr.Bytes(), PubKey: pubkey, VotingPower: 100}
	valBytes, err := tmjson.Marshal(sVal)
	if err != nil {
		log.Error("Marshal validator failed", "err", err)
		return
	}

	//msg := &MsgRegisterValidator{
	//	address: cubeAddr.Bytes(), // common.BytesToAddress()
	//	pubkey:  string(valBytes),
	//}
	//msgBytes, err := json.Marshal(msg)
	//if err != nil {
	//	log.Error("Marshal msg failed", "err", err)
	//}
	//log.Info("after marshal", hex.EncodeToString(valBytes))

	// make geth tx
	nonce := vmgr.getNonce(cubeAddr)

	value := big.NewInt(0) // in wei (0 eth)
	gasPrice := vmgr.getPrice()

	toAddress := system.AddrToPubkeyMapContract

	data, _ := PackInput("", hex.EncodeToString(valBytes))
	//log.Info("after pack", "tx.data", string(data))
	//gasLimit, err := cc.Client.EstimateGas(context.Background(), ethereum.CallMsg{
	//	To:   &toAddress,
	//	Data: data,
	//})
	//if err != nil {
	//	return //nil, err
	//}
	gasLimit := uint64(3000000)

	log.Debug("NewTransaction", "gasPrice", gasPrice.Int64())
	tx := et.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

	// sign tx
	signedTx, err := vmgr.signTx(accounts.Account{Address: cubeAddr}, tx, chainID)
	if err != nil {
		log.Error("signTx failed", "err", err)
		return //nil, err
	}

	// put tx into tx pool
	if err := vmgr.addLocalTx(signedTx); err != nil {
		log.Error("addLocalTx failed", "err", err)
	}
}

func (vmgr *ValidatorsMgr) doRegisterValidator(evm *vm.EVM, data []byte) {
	log.Info("doRegisterValidator")

	var rvMsg MsgRegisterValidator
	if err := tmjson.Unmarshal(data, &rvMsg); err != nil {
		log.Error("unmarshal data failed", "err", err)
		return
	}

	sVal := &SimplifiedValidator{PubKey: rvMsg.PubKey, VotingPower: rvMsg.VotingPower}
	valBytes, err := tmjson.Marshal(sVal)
	if err != nil {
		panic("Marshal validator failed")
	}

	addr := common.BytesToAddress(rvMsg.Address)
	_, err = systemcontract.RegisterValidator(evm, addr, string(valBytes))
	if err != nil {
		log.Error("RegisterValidator failed", "err", err)
	}
	result, err := systemcontract.GetValidator(evm, addr)
	if err != nil {
		log.Error("GetValidator failed", "err", err)
	}
	log.Info("GetValidator", "result", result)
	var tmpVal types.Validator
	err = tmjson.Unmarshal([]byte(result), &tmpVal)
	if err != nil {
		log.Error("Unmarshal validator failed")
	}
	if !tmpVal.PubKey.Equals(rvMsg.PubKey) {
		panic("Conversion failed")
	}
	log.Info("register validator succeed", "cubeAddr", common.BytesToAddress(rvMsg.Address).Hex(), "cosmosAddr", tmpVal.PubKey.Address().String())

	// tVal := types.NewValidator(rvMsg.PubKey, rvMsg.VotingPower)
	// vmgr.AddrValMap[addr] = tVal
}
