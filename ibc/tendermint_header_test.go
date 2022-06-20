package ibc

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	et "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/light"
)

func TestMakeCosmos(t *testing.T) {
	c := MakeCosmos("/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
	println(c.String())
}

func TestMakeValidatorshash(t *testing.T) {
	c := MakeCosmos("/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
	h := c.MakeValidatorshash()
	println(hex.EncodeToString(h))
}

func TestMakeCosmosLightBlockAndSign(t *testing.T) {
	c := MakeCosmos("/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
	headers := MockCubeHeader()
	lb1 := c.MakeCosmosLightBlockAndSign(headers[0], common.Hash{})
	lb2 := c.MakeCosmosLightBlockAndSign(headers[1], common.Hash{})

	err := light.Verify(lb1.SignedHeader, lb1.ValidatorSet, lb2.SignedHeader, lb2.ValidatorSet, 1024000000000, time.Now(), 512000000000, math.Fraction{2, 3})
	if err == nil {
		println("verify pass")
	} else {
		println("verify fail! ", err)
	}

	lb := c.GetCosmosLightBlock(2)
	print(lb.String())
}

func MockCubeHeader() []*et.Header {
	headers := make([]*et.Header, 2)
	var height1 big.Int
	height1.SetInt64(1)
	h1 := &et.Header{Number: &height1, Time: uint64(time.Now().Unix() - 3)}
	var height2 big.Int
	height2.SetInt64(2)
	h2 := &et.Header{ParentHash: h1.Hash(), Number: &height2, Time: uint64(time.Now().Unix() - 2)}

	headers[0] = h1
	headers[1] = h2
	return headers
}

//  type Header struct {
// 	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
// 	UncleHash   common.Hash    `json:"sha3Uncles"       gencodec:"required"`
// 	Coinbase    common.Address `json:"miner"            gencodec:"required"`
// 	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
// 	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
// 	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
// 	Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
// 	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
// 	Number      *big.Int       `json:"number"           gencodec:"required"`
// 	GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
// 	GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
// 	Time        uint64         `json:"timestamp"        gencodec:"required"`
// 	Extra       []byte         `json:"extraData"        gencodec:"required"`
// 	MixDigest   common.Hash    `json:"mixHash"`
// 	Nonce       BlockNonce     `json:"nonce"`

// 	// BaseFee was added by EIP-1559 and is ignored in legacy headers.
// 	BaseFee *big.Int `json:"baseFeePerGas" rlp:"optional"`
// }
