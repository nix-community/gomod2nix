package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"github.com/tweag/gomod2nix/fetch"
	"path/filepath"
)

func main() {

	flag.Parse()

	numWorkers := 20
	keepGoing := true
	directory := "./"

	pkgs, err := fetch.FetchPackages(filepath.Join(directory, "go.mod"), filepath.Join(directory, "go.sum"), numWorkers, keepGoing)
	if err != nil {
		panic(err)
	}

	for _, pkg := range pkgs {
		fmt.Println(pkg)
	}

}
