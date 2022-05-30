package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	generate "github.com/tweag/gomod2nix/generate"
	schema "github.com/tweag/gomod2nix/schema"
	"io/ioutil"
	"path/filepath"
)

func main() {

	var directory = flag.String("dir", "./", "Go project directory")
	var maxJobs = flag.Int("jobs", 10, "Number of max parallel jobs")
	var outDirFlag = flag.String("outdir", "", "output directory (if different from project directory)")
	flag.Parse()

	outDir := *outDirFlag
	if outDir == "" {
		outDir = *directory
	}

	goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
	outFile := goMod2NixPath
	pkgs, err := generate.GeneratePkgs(*directory, goMod2NixPath, *maxJobs)
	if err != nil {
		panic(err)
	}

	output, err := schema.Marshal(pkgs)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(outFile, output, 0644)
	if err != nil {
		panic(err)
	}
	log.Info(fmt.Sprintf("Wrote: %s", outFile))

}
