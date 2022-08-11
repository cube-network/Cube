package cosmos

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/contracts/system"
)

const (
	method = "submit"
)

func PackInput(args ...interface{}) ([]byte, error) {
	return system.ABI(system.CrossChainCosmosContract).Pack(method, args...)
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
	method, exist := system.ABI(system.CrossChainCosmosContract).Methods[method]
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
	if method, ok := system.ABI(system.CrossChainCosmosContract).Methods[name]; ok {
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
