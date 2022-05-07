// Copyright 2020 The go-ethereum Authors
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

package eth

import (
	"errors"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/protocols/cons"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"sync/atomic"
)

// consHandler implements the cons.Backend interface to handle the various network
// packets that are sent as replies or broadcasts.
type consHandler handler

func (h *consHandler) Chain() *core.BlockChain { return h.chain }

// RunPeer is invoked when a peer joins on the `cons` protocol.
func (h *consHandler) RunPeer(peer *cons.Peer, hand cons.Handler) error {
	return (*handler)(h).runConsExtension(peer, hand)
}

// PeerInfo retrieves all known `cons` information about a peer.
func (h *consHandler) PeerInfo(id enode.ID) interface{} {
	if p := h.peers.peer(id.String()); p != nil {
		if p.consExt != nil {
			return p.consExt.info()
		}
	}
	return nil
}

// Handle is invoked from a peer's message handler when it receives a new remote
// message that the handler couldn't consume and serve itself.
func (h *consHandler) Handle(peer *cons.Peer, packet cons.Packet) error {
	//TODO: deliver AttestationsPacket to downloader
	return errors.New("unimplemented")
}

// AcceptAttestation retrieves whether attestation processing is enabled on the node
// or if inbound attestations should simply be dropped.
//
// Notice: we use the acceptTxs flag here, because it's very the same control logic.
func (h *consHandler) AcceptAttestation() bool {
	return atomic.LoadUint32(&h.acceptTxs) == 1
}
