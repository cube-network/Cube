package crosschain

import (
	"testing"
)

func TestPackUnpack(t *testing.T) {
	a, _ := PackInput("ABC", "def")

	a1, a2, _ := UnpackInput(a)
	println("a1", a1)
	println("a2", a2)

	b, _ := PackOutput("ABC")
	b1, _ := UnpackOutput(b)
	println(b1)

}
