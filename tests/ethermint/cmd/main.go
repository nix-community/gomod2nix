package main

import (
	"fmt"

	"github.com/tharsis/ethermint/crypto/hd"
)

func main() {
	fmt.Println(hd.NewExtendedKey())
}
