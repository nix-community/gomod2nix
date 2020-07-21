package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"github.com/tweag/gomod2nix/fetch"
	"github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"io/ioutil"
	"path/filepath"
)

func main() {

	var keepGoing = flag.Bool("keep-going", false, "Whether to panic or not if a rev cannot be resolved (default \"false\")")
	var directory = flag.String("dir", "./", "Go project directory")
	var maxJobs = flag.Int("jobs", 10, "Number of max parallel jobs")
	var outFile = flag.String("outfile", "gomod2nix.toml", "output file")
	var format = flag.String("format", "gomod2nix", "output format (gomod2nix, buildgopackage)")
	flag.Parse()

	goSumPath := filepath.Join(*directory, "go.sum")
	goModPath := filepath.Join(*directory, "go.mod")

	wrongFormatError := fmt.Errorf("Format not supported")

	goMod2NixPath := ""
	depsNixPath := ""
	switch *format {
	case "gomod2nix":
		goMod2NixPath = filepath.Join(*directory, "gomod2nix.toml")
	case "buildgopackage":
		depsNixPath = filepath.Join(*directory, "deps.nix")
	default:
		panic(wrongFormatError)
	}

	pkgs, err := fetch.FetchPackages(goModPath, goSumPath, goMod2NixPath, depsNixPath, *maxJobs, *keepGoing)
	if err != nil {
		panic(err)
	}

	var output []byte
	switch *format {
	case "gomod2nix":
		output, err = gomod2nix.Marshal(pkgs)
		if err != nil {
			panic(err)
		}
	case "buildgopackage":
		output, err = buildgopackage.Marshal(pkgs)
		if err != nil {
			panic(err)
		}
	default:
		panic(wrongFormatError)
	}

	err = ioutil.WriteFile(*outFile, output, 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf("Wrote: %s", *outFile))

}
