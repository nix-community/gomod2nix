package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/lib"
	"github.com/tweag/gomod2nix/types"
	"golang.org/x/mod/modfile"
)

type packageJob struct {
	importPath    string
	goPackagePath string
	sumVersion    string
}

type packageResult struct {
	pkg *types.Package
	err error
}

type goModDownload struct {
	Path     string
	Version  string
	Info     string
	GoMod    string
	Zip      string
	Dir      string
	Sum      string
	GoModSum string
}

func FetchPackages(goModPath string, goSumPath string, goMod2NixPath string, numWorkers int, keepGoing bool) ([]*types.Package, error) {

	log.WithFields(log.Fields{
		"modPath": goModPath,
	}).Info("Parsing go.mod")

	// Read go.mod
	data, err := ioutil.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	// Parse go.mod
	mod, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, err
	}

	// Map repos -> replacement repo
	replace := make(map[string]string)
	for _, repl := range mod.Replace {
		replace[repl.New.Path] = repl.Old.Path
	}

	var modDownloads []*goModDownload
	{
		log.WithFields(log.Fields{
			"sumPath": goSumPath,
		}).Info("Downloading dependencies")

		stdout, err := exec.Command(
			"go", "mod", "download", "--json",
		).Output()
		if err != nil {
			return nil, err
		}

		dec := json.NewDecoder(bytes.NewReader(stdout))
		for {
			var dl *goModDownload
			err := dec.Decode(&dl)
			if err == io.EOF {
				break
			}
			modDownloads = append(modDownloads, dl)
		}

		log.WithFields(log.Fields{
			"sumPath": goSumPath,
		}).Info("Done downloading dependencies")
	}

	executor := lib.NewParallellExecutor(numWorkers)
	var mux sync.Mutex

	packages := []*types.Package{}
	for _, dl := range modDownloads {
		dl := dl

		executor.Add(func() error {

			goPackagePath, hasReplace := replace[dl.Path]
			if !hasReplace {
				goPackagePath = dl.Path
			}

			var storePath string
			{
				stdout, err := exec.Command(
					"nix", "eval", "--impure", "--expr",
					fmt.Sprintf("builtins.path { name = \"%s_%s\"; path = \"%s\"; }", path.Base(goPackagePath), dl.Version, dl.Dir),
				).Output()
				if err != nil {
					return err
				}
				storePath = string(stdout)[1 : len(stdout)-2]
			}

			stdout, err := exec.Command(
				"nix-store", "--query", "--hash", storePath,
			).Output()
			if err != nil {
				return err
			}
			hash := strings.TrimSpace(string(stdout))

			pkg := &types.Package{
				GoPackagePath: goPackagePath,
				Version:       dl.Version,
				Hash:          hash,
			}
			if hasReplace {
				pkg.ReplacedPath = dl.Path
			}

			mux.Lock()
			packages = append(packages, pkg)
			mux.Unlock()

			return nil
		})
	}

	err = executor.Wait()
	if err != nil {
		return nil, err
	}

	return packages, nil

}
