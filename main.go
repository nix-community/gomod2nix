package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"github.com/tweag/gomod2nix/fetch"
	// "github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
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

	// output, err := buildgopackage.Marshal(pkgs)
	output, err := gomod2nix.Marshal(pkgs)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(output))

}
