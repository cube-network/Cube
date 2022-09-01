package cosmos

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/tendermint/tendermint/privval"

	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	cccommon "github.com/ethereum/go-ethereum/crosschain/common"
	"github.com/ethereum/go-ethereum/crosschain/cosmos/systemcontract"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	tmcrypto "github.com/tendermint/tendermint/crypto"
	tmjson "github.com/tendermint/tendermint/libs/json"
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
	AddrValMap map[common.Address]*types.Validator // cube address => cosmos validator

	config            *params.ChainConfig
	getHeaderByNumber cccommon.GetHeaderByNumberFn
	getNonce          cccommon.GetNonceFn
	getPrice          cccommon.GetPriceFn
	signTx            cccommon.SignTxFn
	addLocalTx        cccommon.AddLocalTxFn

	privVal *privval.FilePV
	//registered bool
}

func NewValidatorsMgr(config *params.ChainConfig, privVal *privval.FilePV, headerfn cccommon.GetHeaderByNumberFn) *ValidatorsMgr {
	valMgr := &ValidatorsMgr{
		AddrValMap:        make(map[common.Address]*types.Validator, 0),
		config:            config,
		privVal:           privVal,
		getHeaderByNumber: headerfn,
	}

	// TODO initAddrValMap from contract
	// TODO initAddrValMap with version(cubeheader.hash)

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
	var vheight uint64 = 0
	if height < 400 {
		vheight = 0
	} else {
		vheight = height - 200 - height%200
	}
	return vmgr.getValidatorsImpl(vheight)
}

func (vmgr *ValidatorsMgr) getValidatorsImpl(vheight uint64) ([]common.Address, *types.ValidatorSet) {
	vh := vmgr.getHeaderByNumber(vheight)
	if vh == nil {
		log.Warn("get header is nil ", strconv.Itoa(int(vheight)))
		return []common.Address{}, nil
	}
	addrs := getAddressesFromHeader(vh, IsEnable(vmgr.config, big.NewInt(int64(vheight)))) // make([]common.Address, 1) //
	count := len(addrs)
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		// val := vmgr.AddrValMap[addrs[i]]
		vals := vmgr.getAddrValMap(vheight)
		val := vals[addrs[i]]
		if val == nil {
			// log.Info("count ", strconv.Itoa(count))
			// log.Info("header extra ", hex.EncodeToString(vh.Extra), " height ", strconv.Itoa(int(vheight)), " addr ", addrs[i].Hex(), " index ", strconv.Itoa(i))
			// //panic("validator is nil")
			// return []common.Address{}, nil
			log.Debug("getValidators val is nil", "index", i, "cubeAddr", addrs[i].String(), "cosmosAddr", val.PubKey.Address().String(), " pk ", val.PubKey.Address().String())
			validators[i] = nil
		} else {
			tVal := types.NewValidator(val.PubKey, val.VotingPower)
			validators[i] = tVal
			//log.Debug("getValidators", "index", i, "cubeAddr", addrs[i].String(), "cosmosAddr", val.PubKey.Address().String(), " pk ", val.PubKey.Address().String())
		}
	}
	return addrs, types.NewValidatorSet(validators)
}

func (vmgr *ValidatorsMgr) getValidator(cubeAddr common.Address) *types.Validator {
	return vmgr.AddrValMap[cubeAddr]
}

func (vmgr *ValidatorsMgr) getAddrValMap(height uint64) map[common.Address]*types.Validator {
	return vmgr.AddrValMap
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

func (vmgr *ValidatorsMgr) registerValidator(cubeAddr common.Address, privVal *privval.FilePV, chainID *big.Int) {
	// todo: check whether this node is registered
	if vmgr.getValidator(cubeAddr) != nil {
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

	tVal := types.NewValidator(rvMsg.PubKey, rvMsg.VotingPower)
	vmgr.AddrValMap[addr] = tVal
}
