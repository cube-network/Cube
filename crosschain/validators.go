package crosschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crosschain/systemcontract"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
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

type ValidatorsMgr struct {
	//// LastValidators is used to validate block.LastCommit.
	//// Validators are persisted to the database separately every time they change,
	//// so we can query for historical validator sets.
	//// Note that if s.LastBlockHeight causes a valset change,
	//// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1 + 1
	//// Extra +1 due to nextValSet delay.
	NextValidators              *types.ValidatorSet
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	AddrValMap map[common.Address]*types.Validator // cube address => cosmos validator
}

func (vmgr *ValidatorsMgr) initGenesisValidators(evm *vm.EVM, height int64) error {
	var vals []Validator
	if err := tmjson.Unmarshal([]byte(ValidatorsConfig), &vals); err != nil {
		panic(err)
	}

	validators := make([]*types.Validator, len(vals))
	vmgr.AddrValMap = make(map[common.Address]*types.Validator, len(vals))
	ctx := sdk.Context{}.WithEvm(evm)
	for i, val := range vals {
		sVal := &SimplifiedValidator{PubKey: val.PubKey, VotingPower: val.VotingPower}
		valBytes, err := tmjson.Marshal(sVal)
		if err != nil {
			panic("Marshal validator failed")
		}
		log.Info("Marshal", "result", string(valBytes))

		_, err = systemcontract.RegisterValidator(ctx, val.CubeAddr, string(valBytes))
		if err != nil {
			log.Error("RegisterValidator failed", "err", err)
		}
		result, err := systemcontract.GetValidator(ctx, val.CubeAddr)
		if err != nil {
			log.Error("GetValidator failed", "err", err)
		}
		log.Info("GetValidator", "result", result)
		var tmpVal types.Validator
		err = tmjson.Unmarshal([]byte(result), &tmpVal)
		if err != nil {
			panic("Unmarshal validator failed")
		}
		if !tmpVal.PubKey.Equals(val.PubKey) {
			panic("Conversion failed")
		}

		tVal := types.NewValidator(val.PubKey, val.VotingPower)
		validators[i] = tVal
		vmgr.AddrValMap[val.CubeAddr] = tVal
	}
	vmgr.Validators = types.NewValidatorSet(validators)
	vmgr.NextValidators = types.NewValidatorSet(validators)
	//vmgr.NextValidators = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	vmgr.LastValidators = types.NewValidatorSet(nil)
	vmgr.LastHeightValidatorsChanged = height

	return nil
}

func (vmgr *ValidatorsMgr) updateValidators(h *et.Header, height int64) {
	vmgr.LastValidators = types.NewValidatorSet(vmgr.Validators.Validators)
	_, vmgr.Validators = vmgr.getValidators(h)
	vmgr.NextValidators = types.NewValidatorSet(vmgr.Validators.Validators)
	vmgr.LastHeightValidatorsChanged = height
}

func (vmgr *ValidatorsMgr) getValidators(h *et.Header) ([]common.Address, *types.ValidatorSet) {
	addrs := getAddressesFromHeader(h) // make([]common.Address, 1) //
	count := len(addrs)
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		val := vmgr.AddrValMap[addrs[i]]
		if val == nil {
			panic("validator is nil")
		}
		tVal := types.NewValidator(val.PubKey, val.VotingPower)
		validators[i] = tVal
		log.Info("getValidators", "index", i, "cubeAddr", addrs[i].String(), "cosmosAddr", val.PubKey.Address().String())
	}
	return addrs, types.NewValidatorSet(validators)
}

func (vmgr *ValidatorsMgr) getValidator(addr common.Address) *types.Validator {
	return vmgr.AddrValMap[addr]
}

func getAddressesFromHeader(h *et.Header) []common.Address {
	extraVanity := 32                   // Fixed number of extra-data prefix bytes reserved for validator vanity
	extraSeal := crypto.SignatureLength // Fixed number of extra-data suffix bytes reserved for validator seal

	validatorsBytes := len(h.Extra) - extraVanity - extraSeal
	count := validatorsBytes / common.AddressLength

	addresses := make([]common.Address, count)
	for i := 0; i < count; i++ {
		copy(addresses[i][:], h.Extra[extraVanity+i*common.AddressLength:])
	}
	return addresses
}
