package crosschain

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
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

	AddrValMap map[common.Address]*types.Validator
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

func (vmgr *ValidatorsMgr) updateValidators(data []byte, count int, height int64) {
	validators := make([]*types.Validator, count)
	for i := 0; i < count; i++ {
		var addr common.Address
		copy(addr[:], data[i*common.AddressLength:])
		val := vmgr.AddrValMap[addr]
		if val == nil {
			panic("validator is nil")
		}
		tVal := types.NewValidator(val.PubKey, val.VotingPower)
		validators[i] = tVal
	}
	vmgr.LastValidators = types.NewValidatorSet(vmgr.Validators.Validators)
	vmgr.Validators = types.NewValidatorSet(validators)
	vmgr.LastHeightValidatorsChanged = height
}
