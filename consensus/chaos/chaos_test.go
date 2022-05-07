// Copyright 2017 The go-ethereum Authors
// Copyright 2021 the Cube Authors

package chaos

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/chaos/systemcontract"
	"github.com/ethereum/go-ethereum/contracts/system"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const (
	validatorAdd validatorOp = iota
	validatorInc
	validatorExit
)

var (
	bigHT = big.NewInt(1e18)
)

var (
	validatorV2abi abi.ABI
	votepoolV2abi  abi.ABI
)

type validatorOp byte

func init() {
	file, err := os.Open("testdata/validators_v2_abi.json")
	if err != nil {
		panic(err)
	}
	validatorV2abi, err = abi.JSON(file)
	if err != nil {
		panic(err)
	}
	file, err = os.Open("testdata/votepool_v2_abi.json")
	if err != nil {
		panic(err)
	}
	votepoolV2abi, err = abi.JSON(file)
	if err != nil {
		panic(err)
	}

}

// testerAccountPool is a pool to maintain currently active tester accounts,
// mapped from textual names used in the tests below to actual Ethereum private
// keys capable of signing transactions.
type testerAccountPool struct {
	accounts  map[string]*ecdsa.PrivateKey
	accounts2 map[common.Address]*ecdsa.PrivateKey
	admin     *ecdsa.PrivateKey
	adminAddr common.Address
	votePools map[string]common.Address
}

func newTesterAccountPool() *testerAccountPool {
	adm, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(adm.PublicKey)
	return &testerAccountPool{
		accounts:  make(map[string]*ecdsa.PrivateKey),
		accounts2: make(map[common.Address]*ecdsa.PrivateKey),
		admin:     adm,
		adminAddr: addr,
		votePools: make(map[string]common.Address),
	}
}

// checkpoint creates a Chaos checkpoint signer section from the provided list
// of authorized signers and embeds it into the provided header.
func (ap *testerAccountPool) checkpoint(header *types.Header, signers []string) {
	auths := make([]common.Address, len(signers))
	for i, signer := range signers {
		auths[i] = ap.address(signer)
	}
	sort.Sort(systemcontract.AddrAscend(auths))
	for i, auth := range auths {
		copy(header.Extra[extraVanity+i*common.AddressLength:], auth.Bytes())
	}
}

// address retrieves the Ethereum address of a tester account by label, creating
// a new account if no previous one exists yet.
func (ap *testerAccountPool) address(account string) common.Address {
	// Return the zero account for non-addresses
	if account == "" {
		return common.Address{}
	}
	// Ensure we have a persistent key for the account
	if ap.accounts[account] == nil {
		ap.accounts[account], _ = crypto.GenerateKey()
	}
	// Resolve and return the Ethereum address
	addr := crypto.PubkeyToAddress(ap.accounts[account].PublicKey)
	ap.accounts2[addr] = ap.accounts[account]
	return addr
}

// sign calculates a Chaos digital signature for the given block and embeds it
// back into the header.
func (ap *testerAccountPool) sign(header *types.Header) {
	priv := ap.accounts2[header.Coinbase]
	if priv == nil {
		panic(fmt.Sprintf("account not exist, %s", header.Coinbase))
	}
	// Sign the header and embed the signature in extra data
	sig, _ := crypto.Sign(SealHash(header).Bytes(), priv)
	copy(header.Extra[len(header.Extra)-extraSeal:], sig)
}

func (ap *testerAccountPool) genTx(change testerValidatorChange, nonce uint64, signer types.Signer) (*types.Transaction, error) {
	// Ensure we have a persistent key for the account
	if ap.accounts[change.account] == nil {
		ap.accounts[change.account], _ = crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(ap.accounts[change.account].PublicKey)
		ap.accounts2[addr] = ap.accounts[change.account]
	}

	switch change.op {
	case validatorAdd:
		method := "addValidator"
		valAddr := ap.address(change.account)
		// args: validator, manager, percent(base on 10000), valType(0: pos, 1 poa)
		data, err := validatorV2abi.Pack(method, valAddr, ap.adminAddr, big.NewInt(10000), uint8(1))
		if err != nil {
			return nil, err
		}
		return types.SignTx(types.NewTransaction(nonce, system.StakingContract, nil, 3000000, big.NewInt(params.GWei), data), signer, ap.admin)
	case validatorInc:
		method := "addMargin"
		data, err := votepoolV2abi.Pack(method)
		if err != nil {
			return nil, err
		}
		return types.SignTx(types.NewTransaction(nonce, ap.votePools[change.account], htToWei(change.value), 1000000, big.NewInt(params.GWei), data), signer, ap.admin)
	case validatorExit:
		method := "exit"
		data, err := votepoolV2abi.Pack(method)
		if err != nil {
			return nil, err
		}
		return types.SignTx(types.NewTransaction(nonce, ap.votePools[change.account], nil, 1000000, big.NewInt(params.GWei), data), signer, ap.admin)
	}
	return nil, fmt.Errorf("unsupported op: %v", change.op)
}

// testerValidatorChange represents a single transaction that changes the validators status.
type testerValidatorChange struct {
	account  string
	blockNum int
	op       validatorOp
	value    uint64
}

func htToWei(ht uint64) *big.Int {
	v := new(big.Int).SetUint64(ht)
	return v.Mul(v, bigHT)
}

// Tests that Chaos signer voting is evaluated correctly for various simple and
// complex scenarios, as well as that a few special corner cases fail correctly.
func TestChaos(t *testing.T) {
	// Define the various voting scenarios to test
	tests := []struct {
		epoch          uint64 // default: 2
		waterdropBlock uint64 // default: nil
		chainLen       int    // default: 2*epoch
		signers        []string
		miners         []string
		batches        []int
		changes        []testerValidatorChange

		// Since the validator's state is changed by system contracts, we need to mock checkpoints manually
		// for the sake of simplicity,
		// (that is: the GenerateChain will not call the `engine.Prepare` process, so we need to mock the prepare process),
		// only need to set the first and the changed ones.
		checkpoints map[int][]string
		results     []string
		failure     error
	}{
		{
			// Single signer, no votes cast
			signers:     []string{"A"},
			results:     []string{"A"},
			miners:      []string{"A", "A", "A", "A"},
			checkpoints: map[int][]string{2: {"A"}},
		}, {
			// Single signer, add one other, not effective until next epoch
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 4, op: validatorInc, value: 1},
			},
			miners:   []string{"A", "A", "A", "A", "A"},
			epoch:    6,
			chainLen: 5,
			results:  []string{"A"},
		}, {
			// Single signer, add one other, effective on next epoch
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 3, op: validatorInc, value: 1},
			},
			miners:      []string{"A", "A", "A", "A", "B"},
			epoch:       2,
			chainLen:    5,
			results:     []string{"A", "B"},
			checkpoints: map[int][]string{2: {"A"}, 4: {"A", "B"}},
		}, {
			// Single signer, add one another, use the same validators set before the first epoch after waterdropBlock
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 4, op: validatorInc, value: 1},
			},
			miners:         []string{"A", "A", "A", "A", "A"},
			epoch:          6,
			chainLen:       5,
			waterdropBlock: 4,
			results:        []string{"A"},
		},
		{
			// Single signer, add one another, use look-back validators set after the first epoch after waterdropBlock
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 4, op: validatorInc, value: 1},
			},
			miners:         []string{"A", "A", "A", "A", "A", "A", "A"},
			epoch:          3,
			chainLen:       7,
			waterdropBlock: 4,
			results:        []string{"A"},
			checkpoints: map[int][]string{
				3: {"A"},
				6: {"A", "B"},
			},
		},
		{
			// Single signer, add two others, use look-back validators set after the first epoch after waterdropBlock
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 4, op: validatorInc, value: 1},
				{account: "C", blockNum: 6, op: validatorAdd},
				{account: "C", blockNum: 7, op: validatorInc, value: 1},
			},
			miners:         []string{"A", "A", "A", "A", "A", "A", "A", "A", "A", "B"},
			epoch:          3,
			chainLen:       10,
			waterdropBlock: 4,
			results:        []string{"A", "B"},
			checkpoints: map[int][]string{
				3: {"A"},
				6: {"A", "B"},
				9: {"A", "B", "C"},
			},
		},
		{
			// Single signer, add two others, use look-back validators set after the first epoch after waterdropBlock
			signers: []string{"A"},
			changes: []testerValidatorChange{
				{account: "B", blockNum: 3, op: validatorAdd},
				{account: "B", blockNum: 4, op: validatorInc, value: 1},
				{account: "C", blockNum: 6, op: validatorAdd},
				{account: "C", blockNum: 7, op: validatorInc, value: 1},
			},
			miners:         []string{"A", "A", "A", "A", "A", "A", "A", "A", "A", "B", "B", "B", "C"},
			epoch:          3,
			chainLen:       13,
			waterdropBlock: 4,
			results:        []string{"A", "B", "C"},
			checkpoints: map[int][]string{
				3: {"A"},
				6: {"A", "B"},
				9: {"A", "B", "C"},
			},
		},
		{
			// two signers, add two others, use look-back validators set after the first epoch after waterdropBlock
			signers: []string{"A", "B", "C"},
			changes: []testerValidatorChange{
				{account: "D", blockNum: 3, op: validatorAdd},
				{account: "D", blockNum: 4, op: validatorInc, value: 1},
				{account: "E", blockNum: 4, op: validatorAdd},
				{account: "E", blockNum: 5, op: validatorInc, value: 1},
			},
			miners:         []string{"A", "B", "C", "A", "B", "C", "A", "B", "B", "B", "B", "C", "C", "C", "C", "D", "D"},
			epoch:          3,
			chainLen:       13,
			waterdropBlock: 8,
			results:        []string{"A", "B", "C", "D", "E"},
			checkpoints: map[int][]string{
				3: {"A", "B", "C"},
				6: {"A", "B", "C", "D", "E"},
			},
		},
	}
	// Run through the scenarios and test them
	for i, tt := range tests {
		// Create the account pool and generate the initial set of signers
		accounts := newTesterAccountPool()

		signers := make([]common.Address, len(tt.signers))
		for j, signer := range tt.signers {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
		config := *params.AllChaosProtocolChanges
		epoch := tt.epoch
		if epoch == 0 {
			epoch = 2
		}
		config.Chaos = &params.ChaosConfig{
			Period: 1,
			Epoch:  epoch,
		}

		// Create the genesis block with the initial set of signers
		genesis := core.BasicChaosGenesisBlock(&config, signers, accounts.adminAddr)
		// Create a pristine blockchain with the genesis injected
		db := rawdb.NewMemoryDatabase()
		genesis.Commit(db)

		// Assemble a chain of headers from the cast votes
		engine := New(&config, db)
		engine.fakeDiff = true
		// Pass all the headers through chaos and ensure tallying succeeds
		chain, err := core.NewBlockChain(db, nil, &config, engine, vm.Config{}, nil, nil)
		if err != nil {
			t.Errorf("test %d: failed to create test chain: %v", i, err)
			continue
		}
		// do some extra work for chaos.
		// set state fn
		engine.SetStateFn(func(hash common.Hash) (*state.StateDB, error) {
			statedb, err := state.New(hash, state.NewDatabase(db), nil)
			if err != nil {
				panic(fmt.Sprintf("can't get statedb for %s : %v", hash.String(), err))
			}
			return statedb, nil
		})
		// chaos need the chain for extra_validate feature
		engine.SetChain(chain)

		chainLen := tt.chainLen
		if chainLen == 0 {
			chainLen = int(2 * epoch)
		}
		//tx signer
		signer := types.LatestSigner(&config)

		blocks, _ := core.GenerateChain(&config, genesis.ToBlock(db), engine, db, chainLen, func(j int, gen *core.BlockGen) {

			// j is not block number, but index which starts from 0.
			// Cast the vote contained in this block
			gen.SetCoinbase(accounts.address(tt.miners[j]))
			// Since the `validator` field is empty in engine, so the difficulty from chainMaker is not correct.
			gen.SetDifficulty(diffInTurn)

			for _, change := range tt.changes {
				if change.blockNum == (j + 1) {
					tx, err := accounts.genTx(change, gen.TxNonce(accounts.adminAddr), signer)
					if err != nil {
						panic("genTx: " + err.Error())
					}
					receipt := gen.AddTxWithChain(chain, tx)
					if change.op == validatorAdd {
						//get vote pool address
						if receipt.Status == 0 {
							panic("add validator failed")
						}
						votePool := common.BytesToAddress(receipt.Logs[0].Data)
						accounts.votePools[change.account] = votePool
						//t.Logf("votePool address %s:%v\n", change.account, votePool.String())
					}
					//t.Logf("info: op=%v, gasUesd=%d\n", change.op, receipt.GasUsed)
				}
			}
		})
		// Iterate through the blocks and seal them individually
		var lastCheckpointExtra []byte
		failed := false
		for j, block := range blocks {
			// Get the header and prepare it for signing
			header := block.Header()
			if j > 0 {
				header.ParentHash = blocks[j-1].Hash()
			}
			header.Extra = make([]byte, extraVanity+extraSeal)
			if uint64(j+1)%epoch == 0 {
				if tt.checkpoints != nil {
					auths, exist := tt.checkpoints[j+1]
					if exist {
						header.Extra = make([]byte, extraVanity+len(auths)*common.AddressLength+extraSeal)
						accounts.checkpoint(header, auths)
						lastCheckpointExtra = make([]byte, len(header.Extra))
						copy(lastCheckpointExtra, header.Extra)
					} else if len(lastCheckpointExtra) > 0 {
						header.Extra = make([]byte, len(lastCheckpointExtra))
						copy(header.Extra, lastCheckpointExtra)
					} else {
						t.Errorf("need to set checkpoints correctly")
						failed = true
						break
					}
				}
			}

			header.Difficulty = diffInTurn // Ignored, we just need a valid number

			// Generate the signature, embed it into the header and the block
			accounts.sign(header)
			blocks[j] = block.WithSeal(header)
		}
		if failed {
			continue
		}
		// Split the blocks up into individual import batches (corner case testing)
		batches := [][]*types.Block{nil}
		idx := 0
		for _, batch := range tt.batches {
			if idx >= chainLen {
				break
			}
			batches = append(batches, nil)
			n := idx + batch
			if n > chainLen {
				n = chainLen
			}
			for ; idx < n; idx++ {
				batches[len(batches)-1] = append(batches[len(batches)-1], blocks[idx])
			}
		}
		if idx < chainLen {
			batches = append(batches, nil)
			n := chainLen
			for ; idx < n; idx++ {
				batches[len(batches)-1] = append(batches[len(batches)-1], blocks[idx])
			}
		}

		for j := 0; j < len(batches)-1; j++ {
			if k, err := chain.InsertChain(batches[j]); err != nil {
				t.Errorf("test %d: failed to import batch %d, block %d: %v", i, j, k, err)
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		if _, err = chain.InsertChain(batches[len(batches)-1]); err != tt.failure {
			t.Errorf("test %d: failure mismatch: have %v, want %v", i, err, tt.failure)
		}
		if tt.failure != nil {
			continue
		}
		// No failure was produced or requested, generate the final voting snapshot
		head := blocks[len(blocks)-1]

		snap, err := engine.snapshot(chain, head.NumberU64(), head.Hash(), nil)
		if err != nil {
			t.Errorf("test %d: failed to retrieve voting snapshot: %v", i, err)
			continue
		}
		// Verify the final list of signers against the expected ones
		signers = make([]common.Address, len(tt.results))
		for j, signer := range tt.results {
			signers[j] = accounts.address(signer)
		}
		for j := 0; j < len(signers); j++ {
			for k := j + 1; k < len(signers); k++ {
				if bytes.Compare(signers[j][:], signers[k][:]) > 0 {
					signers[j], signers[k] = signers[k], signers[j]
				}
			}
		}
		result := snap.validators()
		if len(result) != len(signers) {
			t.Errorf("test %d: signers mismatch: have %x, want %x", i, result, signers)
			continue
		}
		for j := 0; j < len(result); j++ {
			if !bytes.Equal(result[j][:], signers[j][:]) {
				t.Errorf("test %d, signer %d: signer mismatch: have %x, want %x", i, j, result[j], signers[j])
			}
		}
	}
}
