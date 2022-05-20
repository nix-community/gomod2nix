package main

import (
	"fmt"

	martian "github.com/google/martian/v3"
	"github.com/tharsis/ethermint/crypto/hd"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println(hd.NewExtendedKey())
	fmt.Println(grpc.Version)
	fmt.Println(martian.Noop(""))
}
