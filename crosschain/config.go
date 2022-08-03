package crosschain

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
)

type GenesisState map[string]json.RawMessage

// NewDefaultGenesisState generates the default state for the application.
func NewDefaultGenesisState(cdc codec.JSONCodec) GenesisState {
	return ModuleBasics.DefaultGenesis(cdc)
}

const IBCConfig = `
{
	"ibc": {
	"client_genesis": {
	  "clients": [],
	  "clients_consensus": [],
	  "clients_metadata": [],
	  "params": {
		"allowed_clients": [
		  "06-solomachine",
		  "07-tendermint"
		]
	  },
	  "create_localhost": false,
	  "next_client_sequence": "0"
	},
	"connection_genesis": {
	  "connections": [],
	  "client_connection_paths": [],
	  "next_connection_sequence": "0",
	  "params": {
		"max_expected_time_per_block": "30000000000"
	  }
	},
	"channel_genesis": {
	  "channels": [],
	  "acknowledgements": [],
	  "commitments": [],
	  "receipts": [],
	  "send_sequences": [],
	  "recv_sequences": [],
	  "ack_sequences": [],
	  "next_channel_sequence": "0"
	}
  },
  "interchainaccounts": {
	"controller_genesis_state": {
	  "active_channels": [],
	  "interchain_accounts": [],
	  "ports": [],
	  "params": {
		"controller_enabled": true
	  }
	},
	"host_genesis_state": {
	  "active_channels": [],
	  "interchain_accounts": [],
	  "port": "icahost",
	  "params": {
		"host_enabled": true,
		"allow_messages": []
	  }
	}
  },
	"packetfowardmiddleware": {
		"params": {
		  "fee_percentage": "0.000000000000000000"
		}
	  },
	"params": null,
	"transfer": {
		"port_id": "transfer",
		"denom_traces": [],
		"params": {
		  "send_enabled": true,
		  "receive_enabled": true
		}
	  }
}
`

const ValidatorsConfig = `
[
    {
        "address":"2C4CD6E199A377D856F6D2BF9EE646865AE5D36C",
        "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"nOPLfpaB6YOhTZu/pulwcM/FLj1u3L5aB6myGsd6re8="
        },
        "voting_power":"100",
        "name":""
    },
    {
        "address":"695EA7EA41E50B71AD75D51ECEC63FA75BD475E2",
        "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"o10GuR9a9Q+TR+LvlslCLv2OZu/8uLJHiL2POAIlIRg="
        },
        "voting_power":"100",
        "name":""
    },
    {
        "address":"F7786E81F204A40DCA679664E46E3CE72A28387B",
        "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"JWEePRroDSKDrcYqlcGe2u2F0xgKozCLMuOQtLOI1ro="
        },
        "voting_power":"100",
        "name":""
    },
    {
        "address":"6262EDC062A5C72A6DB9371B2F2040A945F9A1CE",
        "pub_key":{
            "type":"tendermint/PubKeyEd25519",
            "value":"iYU6fzPxOrMTkCGvfMjZh4sKVOLPWy5Uk9SRrp9+E+Y="
        },
        "voting_power":"100",
        "name":""
    }
]
`
