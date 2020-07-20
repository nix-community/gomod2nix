package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"github.com/tweag/gomod2nix/fetch"
	// "github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"io/ioutil"
	"path/filepath"
)

func main() {

	flag.Parse()

	numWorkers := 1
	keepGoing := false
	// directory := "./"
	directory := "./testdata/vuls"
	outFile := "gomod2nix.toml"

	goModPath := filepath.Join(directory, "go.mod")
	goSumPath := filepath.Join(directory, "go.sum")
	goMod2NixPath := "./gomod2nix.toml"

	pkgs, err := fetch.FetchPackages(goModPath, goSumPath, goMod2NixPath, numWorkers, keepGoing)
	if err != nil {
		panic(err)
	}

	if true {
		panic("Success")
	}

	// output, err := buildgopackage.Marshal(pkgs)
	output, err := gomod2nix.Marshal(pkgs)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(outFile, output, 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf("Wrote: %s", outFile))

}
