package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/fetch"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"io/ioutil"
	"path/filepath"
)

func main() {

	var keepGoing = flag.Bool("keep-going", false, "Whether to panic or not if a rev cannot be resolved (default \"false\")")
	var directory = flag.String("dir", "./", "Go project directory")
	var maxJobs = flag.Int("jobs", 10, "Number of max parallel jobs")
	var outDirFlag = flag.String("outdir", "", "output directory (if different from project directory)")
	flag.Parse()

	outDir := *outDirFlag
	if outDir == "" {
		outDir = *directory
	}

	goSumPath := filepath.Join(*directory, "go.sum")
	goModPath := filepath.Join(*directory, "go.mod")

	goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
	outFile := goMod2NixPath
	pkgs, err := fetch.FetchPackages(goModPath, goSumPath, goMod2NixPath, *maxJobs, *keepGoing)
	if err != nil {
		panic(err)
	}

	output, err := gomod2nix.Marshal(pkgs)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(outFile, output, 0644)
	if err != nil {
		panic(err)
	}
	log.Info(fmt.Sprintf("Wrote: %s", outFile))

}
