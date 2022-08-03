package crosschain

import (
	"bytes"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/types"
)

// State is a short description of the latest committed block of the Tendermint consensus.
// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or updateState(...).
// NOTE: not goroutine-safe.
type ValidatorsMgr struct {
	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1 + 1
	// Extra +1 due to nextValSet delay.
	NextValidators              *types.ValidatorSet
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64
}

func (vmgr *ValidatorsMgr) initGenesisValidators(height int64) error {
	var vals []types.Validator
	if err := tmjson.Unmarshal([]byte(ValidatorsConfig), &vals); err != nil {
		panic(err)
	}

	//validators := make([]*ct.Validator, len(validators))
	validators := make([]*types.Validator, len(vals))
	//priv_val_pubkey, _ := c.priv_validator.GetPubKey()
	for i, val := range vals {
		validators[i] = types.NewValidator(val.PubKey, val.VotingPower)
		//validators[index] = &val
		//if val.PubKey.Equals(priv_val_pubkey) {
		//	c.priv_addr_idx = uint32(index)
		//	fmt.Printf("val.addr: %s, val.index: %d\n", val.Address.String(), c.priv_addr_idx)
		//}
		//fmt.Printf("val.addr: %s, val.pubkey: %d\n", val.Address.String(), val.PubKey.Address().String())
	}
	vmgr.Validators = types.NewValidatorSet(validators)
	vmgr.NextValidators = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	vmgr.LastValidators = types.NewValidatorSet(nil)
	vmgr.LastHeightValidatorsChanged = height

	return nil

	//c.state.Validators = ct.NewValidatorSet(c.validators)

	//publicKey, _ := c.priv_validator.GetPubKey()
	//publicKeyBytes := make([]byte, ed25519.PubKeySize)
	//copy(publicKeyBytes, publicKey.Bytes())
	//restoredPubkey := ed25519.PubKey{Key: publicKeyBytes}
	//println(restoredPubkey.Address().String())
	//println(publicKey.Address().String())
	//println(restoredPubkey.String())
	//println(string(publicKey.Bytes()))
}

func (vmgr *ValidatorsMgr) isProposer(address []byte) bool {
	return bytes.Equal(vmgr.Validators.GetProposer().Address, address)
}
