package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/lib"
	schema "github.com/tweag/gomod2nix/schema"
	"golang.org/x/mod/modfile"
)

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

func GeneratePkgs(goModPath string, goMod2NixPath string, numWorkers int) ([]*schema.Package, error) {

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
		log.Info("Downloading dependencies")

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

		log.Info("Done downloading dependencies")
	}

	executor := lib.NewParallellExecutor(numWorkers)
	var mux sync.Mutex

	cache := schema.ReadCache(goMod2NixPath)

	packages := []*schema.Package{}
	addPkg := func(pkg *schema.Package) {
		mux.Lock()
		packages = append(packages, pkg)
		mux.Unlock()
	}

	for _, dl := range modDownloads {
		dl := dl

		goPackagePath, hasReplace := replace[dl.Path]
		if !hasReplace {
			goPackagePath = dl.Path
		}

		cached, ok := cache[goPackagePath]
		if ok && cached.Version == dl.Version {
			addPkg(cached)
			continue
		}

		executor.Add(func() error {
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

			pkg := &schema.Package{
				GoPackagePath: goPackagePath,
				Version:       dl.Version,
				Hash:          hash,
			}
			if hasReplace {
				pkg.ReplacedPath = dl.Path
			}

			addPkg(pkg)

			return nil
		})
	}

	err = executor.Wait()
	if err != nil {
		return nil, err
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].GoPackagePath < packages[j].GoPackagePath
	})

	return packages, nil

}
