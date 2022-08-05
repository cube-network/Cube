package crosschain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/tendermint/crypto"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/types"
)

type Validator struct {
	CubeAddr common.Address `json:"address"`
	//CosmosAddr  types.Address  `json:"cosmos_address"`
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower int64         `json:"voting_power"`
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

func (vmgr *ValidatorsMgr) initGenesisValidators(height int64) error {
	var vals []Validator
	if err := tmjson.Unmarshal([]byte(ValidatorsConfig), &vals); err != nil {
		panic(err)
	}

	validators := make([]*types.Validator, len(vals))
	vmgr.AddrValMap = make(map[common.Address]*types.Validator, len(vals))
	for i, val := range vals {
		tVal := types.NewValidator(val.PubKey, val.VotingPower)
		vmgr.AddrValMap[val.CubeAddr] = tVal
		validators[i] = tVal
		fmt.Printf("val.addr: %s, val.pubkey: %s\n", val.CubeAddr.String(), val.PubKey.Address().String())
	}
	vmgr.Validators = types.NewValidatorSet(validators)
	//vmgr.NextValidators = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	vmgr.LastValidators = types.NewValidatorSet(nil)
	vmgr.LastHeightValidatorsChanged = height

	return nil
}

func (vmgr *ValidatorsMgr) updateValidators(h *et.Header, height int64) {
	vmgr.LastValidators = types.NewValidatorSet(vmgr.Validators.Validators)
	_, vmgr.Validators = vmgr.getValidators(h)
	vmgr.LastHeightValidatorsChanged = height
}

func (vmgr *ValidatorsMgr) getValidators(h *et.Header) ([]common.Address, *types.ValidatorSet) {
	addrs := core.GetAddressesFromHeader(h)
	count := len(addrs)
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		val := vmgr.AddrValMap[addrs[i]]
		if val == nil {
			panic("validator is nil")
		}
		tVal := types.NewValidator(val.PubKey, val.VotingPower)
		validators[i] = tVal
	}
	return addrs, types.NewValidatorSet(validators)
}

func (vmgr *ValidatorsMgr) getValidator(addr common.Address) *types.Validator {
	return vmgr.AddrValMap[addr]
}
