package main // import "github.com/tweag/gomod2nix"

import (
	"crypto/sha1"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/fetch"
	"github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	var prefetch = flag.String("prefetch", "", "Prefetch module in the form 'path' or 'path@version'")

	var keepGoing = flag.Bool("keep-going", false, "Whether to panic or not if a rev cannot be resolved (default \"false\")")
	var directoryFlag = flag.String("dir", "", "Go project directory")
	var maxJobs = flag.Int("jobs", 10, "Number of max parallel jobs")
	var outDirFlag = flag.String("outdir", "", "output directory (if different from project directory)")
	var format = flag.String("format", "gomod2nix", "output format (gomod2nix, buildgopackage)")
	flag.Parse()

	directory := *directoryFlag
	outDir := *outDirFlag

	if *prefetch == "" {
		if directory == "" {
			directory = "./"
		}
		if outDir == "" {
			outDir = directory
		}
	} else {
		if directory == "" {
			hash := sha1.Sum([]byte(*prefetch))
			directory = filepath.Join(os.TempDir(), fmt.Sprintf("gomod2nix.%x", hash))
		}
		if outDir == "" {
			outDir = "./"
		}

		log.WithFields(log.Fields{
			"directory": directory,
			"outDir": outDir,
		}).Info(fmt.Sprintf("Prefetching '%s'", *prefetch))

		packagePath, err := fetch.FetchRepo(fetch.ModuleVersion(*prefetch), directory)
		if err != nil {
			panic(err)
		}
		directory = packagePath
	}

	goSumPath := filepath.Join(directory, "go.sum")
	goModPath := filepath.Join(directory, "go.mod")

	wrongFormatError := fmt.Errorf("Format not supported")

	goMod2NixPath := ""
	depsNixPath := ""
	outFile := ""
	switch *format {
	case "gomod2nix":
		goMod2NixPath = filepath.Join(outDir, "gomod2nix.toml")
		outFile = goMod2NixPath
	case "buildgopackage":
		depsNixPath = filepath.Join(outDir, "deps.nix")
		outFile = depsNixPath
	default:
		panic(wrongFormatError)
	}
	log.Info(fmt.Sprintf("Using output format '%s'", *format))

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

	err = ioutil.WriteFile(outFile, output, 0644)
	if err != nil {
		panic(err)
	}
	log.Info(fmt.Sprintf("Wrote: %s", outFile))

}
