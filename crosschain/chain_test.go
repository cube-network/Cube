package crosschain

import (
	"testing"
)

func TestMakeCosmosChain(t *testing.T) {
	c := MakeCosmosChain("", "/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
	println(c.String())
}

//func TestMakeValidatorshash(t *testing.T) {
//	c := MakeCosmosChain("", "/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
//	h := c.MakeValidatorshash()
//	println(hex.EncodeToString(h))
//}
//
//func TestMakeCosmosLightBlockAndSign(t *testing.T) {
//	c := MakeCosmosChain("", "/Users/xieguilu/code/Cube/ibc/key.json", "/Users/xieguilu/code/Cube/ibc/state.json")
//	headers := MockCubeHeader()
//	lb1 := c.MakeLightBlockAndSign(headers[0], common.Hash{})
//	lb2 := c.MakeLightBlockAndSign(headers[1], common.Hash{})
//
//	err := light.Verify(lb1.SignedHeader, lb1.ValidatorSet, lb2.SignedHeader, lb2.ValidatorSet, 1024000000000, time.Now(), 512000000000, math.Fraction{2, 3})
//	if err == nil {
//		println("verify pass")
//	} else {
//		println("verify fail! ", err)
//	}
//
//	lb := c.GetLightBlock(2)
//	print(lb.String())
//}
//
//func MockCubeHeader() []*et.Header {
//	headers := make([]*et.Header, 2)
//	var height1 big.Int
//	height1.SetInt64(1)
//	h1 := &et.Header{Number: &height1, Time: uint64(time.Now().Unix() - 3)}
//	var height2 big.Int
//	height2.SetInt64(2)
//	h2 := &et.Header{ParentHash: h1.Hash(), Number: &height2, Time: uint64(time.Now().Unix() - 2)}
//
//	headers[0] = h1
//	headers[1] = h2
//	return headers
//}
