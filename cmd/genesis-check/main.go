// Copyright 2022 The Cube Authors
// This file is part of the Cube library.
//
// The Cube library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The Cube library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the Cube library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "<genesis_file>")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, `Parses the given genesis file and tries to generate genesis block hash.`)
	}
}

// checkGenesis uses the genesis file to generate genesis block
func checkGenesis(file *os.File) {
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		die(err)
	}
	genesisHash := genesis.ToBlock(nil).Hash()
	fmt.Printf("Genesis Hash: %v\nIs Mainnet: %v\nIs Testnet: %v\n", genesisHash,
		genesisHash == params.MainnetGenesisHash,
		genesisHash == params.TestnetGenesisHash)
}

// Example
// ./genesis-check genesis.json
func main() {
	flag.Parse()

	switch {
	case flag.NArg() == 1:
		genesisFile := flag.Arg(0)
		file, err := os.Open(genesisFile)
		if err != nil {
			die(err)
		}
		defer file.Close()
		checkGenesis(file)
	default:
		fmt.Fprintln(os.Stderr, "Error: one argument needed")
		flag.Usage()
		os.Exit(2)
	}
}

func die(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
