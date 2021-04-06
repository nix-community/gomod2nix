package main // import "github.com/tweag/gomod2nix"

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/fetch"
	"github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"golang.org/x/oauth2"
)

func main() {
	// optional support for using GH API
	// try using GH_TOKEN, GITHUB_TOKEN env vars in order (same as GH's official CLI).
	ghToken := os.Getenv("GH_TOKEN")
	if len(ghToken) == 0 {
		ghToken = os.Getenv("GITHUB_TOKEN")
	}
	if len(ghToken) > 0 {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: ghToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		client := github.NewClient(tc)
		fetch.GithubClient = client
	}

	var keepGoing = flag.Bool("keep-going", false, "Whether to panic or not if a rev cannot be resolved (default \"false\")")
	var directory = flag.String("dir", "./", "Go project directory")
	var maxJobs = flag.Int("jobs", 10, "Number of max parallel jobs")
	var outDirFlag = flag.String("outdir", "", "output directory (if different from project directory)")
	var format = flag.String("format", "gomod2nix", "output format (gomod2nix, buildgopackage)")
	flag.Parse()

	outDir := *outDirFlag
	if outDir == "" {
		outDir = *directory
	}

	goSumPath := filepath.Join(*directory, "go.sum")
	goModPath := filepath.Join(*directory, "go.mod")

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
