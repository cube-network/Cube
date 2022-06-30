package crosschain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

const (
	IBCContractABI = `[
		{
			"inputs": [
				{
					"internalType": "string",
					"name": "selector",
					"type": "string"
				},
				{
					"internalType": "string",
					"name": "args",
					"type": "string"
				}
			],
			"name": "submit",
			"outputs": [
				{
					"internalType": "string",
					"name": "",
					"type": "string"
				}
			],
			"stateMutability": "",
			"type": "function"
		}
	]`
)
const (
	method = "submit"
)

var (
	IBCABI abi.ABI
)

func init() {
	if abi, err := abi.JSON(strings.NewReader(IBCContractABI)); err != nil {
		panic(err)
	} else {
		IBCABI = abi
	}
}

func PackInput(args ...interface{}) ([]byte, error) {
	return IBCABI.Pack(method, args...)
}

func UnpackInput(data []byte) (string, string, error) {
	res, err := Unpack(method, data[4:], true)
	if err != nil {
		return "", "", err
	} else {
		return res[0].(string), res[1].(string), err
	}
}

func PackOutput(args ...interface{}) ([]byte, error) {
	method, exist := IBCABI.Methods[method]
	if !exist {
		return nil, fmt.Errorf("method '%s' not found", method)
	}
	arguments, err := method.Outputs.Pack(args...)
	if err != nil {
		return nil, err
	}
	return arguments, nil
}

func UnpackOutput(data []byte) (string, error) {
	res, err := Unpack(method, data, false)
	if err != nil {
		return "", err
	} else {
		return res[0].(string), nil
	}
}

func getArguments(name string, data []byte, is_input bool) (abi.Arguments, error) {
	// since there can't be naming collisions with contracts and events,
	// we need to decide whether we're calling a method or an event
	var args abi.Arguments
	if method, ok := IBCABI.Methods[name]; ok {
		if len(data)%32 != 0 {
			return nil, fmt.Errorf("abi: improperly formatted output: %s - Bytes: [%+v]", string(data), data)
		}
		if is_input {
			args = method.Inputs
		} else {
			args = method.Outputs
		}

	}
	if args == nil {
		return nil, errors.New("abi: could not locate named method or event")
	}
	return args, nil
}

func Unpack(name string, data []byte, is_input bool) ([]interface{}, error) {
	args, err := getArguments(name, data, is_input)
	if err != nil {
		return nil, err
	}
	return args.Unpack(data)
}
