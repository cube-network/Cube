// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import "github.com/ethereum/go-ethereum/common"

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Cube network.
var MainnetBootnodes = []string{
	// Cube Foundation Go Bootnodes
	"enode://38d2c886ed86379a88c58bf024b7575d3b89e9239ba380bc0c49c4d14d24f147b429f89553106351600f35efa835680ff96fc4220c1d6f5e3fb8b109e36f2574@43.133.189.105:33688",
	"enode://576751034af9c7fbc958af162318e5d962b962821b55e6533967a29dec83bb2c486baf8808e020ca7e1ffb3658b1669ccfe25513f73058fca93a9296cfa15b7c@43.133.23.39:33688",
	"enode://2219a8c4cefc5c31ce4c744de46ca70f436344342327baad82d83fe3d64e679fdd631380e1c7ae1033d12ec9a8a1ae75ecae2e10313b95108ec1791955d5291f@43.128.80.123:33688",
	"enode://4e158ecd8cdc6857461f8358768b37bdab94ba934fd3b0982eb1d85c0b3d354bdcd21ed0ca22a58ef56437dc74781f71884af166e0673b97aabad42c0b6e55b8@43.134.69.100:33688",
	"enode://ae8f91e34062152d483f45999bee323bde7c230f3a4d79d3fc67cff83027ecaa5cfde5a538e3aacf7266b064825ee90ca9c8ddd6192cdc8d83b8363c7dc82777@50.18.45.58:33688",
	"enode://193a049e1f76ef6202a5e1a24b33492fd2f343e3ae527c6582a5afe7e7158502b44ed524939d126b2cc37fa7354dfad7b594591f532755dd16f54155964ee3dc@50.18.102.76:33688",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the Cube testnet.
var TestnetBootnodes = []string{
	"enode://e0c19e130f2d95e764357901c9624479e5653deb8ba66c2695848c3a0f16cca1950cf5c19f790c75383282d03f4aba95a7596c45ee48a63ef593339db0c772ab@43.134.93.113:33688",
	"enode://142fb9df46ac243f4443b047a6d3478f9ae38e4ba8ad5b9da056756df7f00178c5e7995c1c092b39be47cdfe67b6054e5a225e82bf1f4e0cffa63c5141afb325@43.133.170.69:33688",
	"enode://9406f9eb812043fdafd09d000f5983cd5dfa5ac2ea3e9187c7d1d7adc0af89578ba43486648858f35962da44e45889610b7970c14ae0a7761233bfd7c523395c@52.9.153.50:33688",
}

var V5Bootnodes = []string{}

// KnownDNSNetwork returns the address of a public DNS-based node list for the given
// genesis hash and protocol. See https://github.com/ethereum/discv4-dns-lists for more
// information.
func KnownDNSNetwork(genesis common.Hash, protocol string) string {
	return ""
}
