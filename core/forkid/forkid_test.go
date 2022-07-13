// Copyright 2019 The go-ethereum Authors
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

package forkid

import (
	"bytes"
	"math"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// TestCreation tests that different genesis and fork rule combinations result in
// the correct fork ID.
func TestCreation(t *testing.T) {
	type testcase struct {
		head uint64
		want ID
	}
	tests := []struct {
		config  *params.ChainConfig
		genesis common.Hash
		cases   []testcase
	}{
		// Mainnet test cases
		{
			params.MainnetChainConfig,
			params.MainnetGenesisHash,
			[]testcase{
				{0, ID{Hash: checksumToBytes(0x08e4e845), Next: 829915}},      // Unsynced
				{829914, ID{Hash: checksumToBytes(0x08e4e845), Next: 829915}}, // Last unsynced block
				{829915, ID{Hash: checksumToBytes(0xee6b1c5d), Next: 0}},      // First Gravitation block
				{900000, ID{Hash: checksumToBytes(0xee6b1c5d), Next: 0}},      // Future Gravitation block
			},
		},
	}
	for i, tt := range tests {
		for j, ttt := range tt.cases {
			if have := NewID(tt.config, tt.genesis, ttt.head); have != ttt.want {
				t.Errorf("test %d, case %d: fork ID mismatch: have %x, want %x", i, j, have, ttt.want)
			}
		}
	}
}

// TestValidation tests that a local peer correctly validates and accepts a remote
// fork ID.
func TestValidation(t *testing.T) {
	tests := []struct {
		head uint64
		id   ID
		err  error
	}{
		// local is Gravitation, remote is the same, and no future fork is announced
		{900000, ID{Hash: checksumToBytes(0xee6b1c5d), Next: 0}, nil},
		// local is before Gravitation, and with Gravitation fork, remote is the same, but it's not yet aware of Gravitation fork
		{829914, ID{Hash: checksumToBytes(0x08e4e845), Next: 0}, nil},
		// local is before Gravitation, and with Gravitation fork, remote is the same, and with the same Gravitation fork
		{829914, ID{Hash: checksumToBytes(0x08e4e845), Next: 829915}, nil},
		// local is Gravitation, remote is before Gravitation + known of Gravitation fork, so remote is not sync, accept it
		{900000, ID{Hash: checksumToBytes(0x08e4e845), Next: 829915}, nil},
		// Local is mainnet Gravitation. remote announces before any hard-fork, and is not aware of further forks.
		// Remote needs software update.
		{829915, ID{Hash: checksumToBytes(0x08e4e845), Next: 0}, ErrRemoteStale},
		// Local is before any hard-fork, and remote is Gravitation, Local needs to sync.
		{829914, ID{Hash: checksumToBytes(0xee6b1c5d), Next: 0}, nil},
	}
	for i, tt := range tests {
		filter := newFilter(params.MainnetChainConfig, params.MainnetGenesisHash, func() uint64 { return tt.head })
		if err := filter(tt.id); err != tt.err {
			t.Errorf("test %d: validation error mismatch: have %v, want %v", i, err, tt.err)
		}
	}
}

// Tests that IDs are properly RLP encoded (specifically important because we
// use uint32 to store the hash, but we need to encode it as [4]byte).
func TestEncoding(t *testing.T) {
	tests := []struct {
		id   ID
		want []byte
	}{
		{ID{Hash: checksumToBytes(0), Next: 0}, common.Hex2Bytes("c6840000000080")},
		{ID{Hash: checksumToBytes(0xdeadbeef), Next: 0xBADDCAFE}, common.Hex2Bytes("ca84deadbeef84baddcafe,")},
		{ID{Hash: checksumToBytes(math.MaxUint32), Next: math.MaxUint64}, common.Hex2Bytes("ce84ffffffff88ffffffffffffffff")},
	}
	for i, tt := range tests {
		have, err := rlp.EncodeToBytes(tt.id)
		if err != nil {
			t.Errorf("test %d: failed to encode forkid: %v", i, err)
			continue
		}
		if !bytes.Equal(have, tt.want) {
			t.Errorf("test %d: RLP mismatch: have %x, want %x", i, have, tt.want)
		}
	}
}
