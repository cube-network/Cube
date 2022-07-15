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
