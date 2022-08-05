package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ct "github.com/tendermint/tendermint/types"
)

func GetAddressesFromHeader(h *types.Header) []common.Address {
	validatorsBytes := len(h.Extra) - extraVanity - extraSeal
	count := validatorsBytes / common.AddressLength

	addresses := make([]common.Address, count)
	for i := 0; i < count; i++ {
		copy(addresses[i][:], h.Extra[extraVanity+i*common.AddressLength:])
	}
	return addresses
}

func GetValidators(h *types.Header, addrValMap map[common.Address]*ct.Validator) ([]common.Address, *ct.ValidatorSet) {
	addrs := GetAddressesFromHeader(h)
	count := len(addrs)
	validators := make([]*ct.Validator, count)
	for i := 0; i < count; i++ {
		val := addrValMap[addrs[i]]
		if val == nil {
			panic("validator is nil")
		}
		tVal := ct.NewValidator(val.PubKey, val.VotingPower)
		validators[i] = tVal
	}
	return addrs, ct.NewValidatorSet(validators)
}

type BlockAndCosmosHeader struct {
	Block        *types.Block
	CosmosHeader *ct.SignedHeader
}
