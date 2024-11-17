package generate

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/nix-community/gomod2nix/internal/lib"
	schema "github.com/nix-community/gomod2nix/internal/schema"
	log "github.com/sirupsen/logrus"
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

func sourceFilter(name string, nodeType nar.NodeType) bool {
	return strings.ToLower(filepath.Base(name)) != ".ds_store"
}

func common(directory string) ([]*goModDownload, map[string]string, error) {
	goModPath := filepath.Join(directory, "go.mod")

	log.WithFields(log.Fields{
		"modPath": goModPath,
	}).Info("Parsing go.mod")

	// Read go.mod
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, nil, err
	}

	// Parse go.mod
	mod, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, nil, err
	}

	// Map repos -> replacement repo
	replace := make(map[string]string)
	for _, repl := range mod.Replace {
		replace[repl.New.Path] = repl.Old.Path
	}

	var modDownloads []*goModDownload
	{
		log.Info("Downloading dependencies")

		cmd := exec.Command(
			"go", "mod", "download", "--json",
		)
		cmd.Dir = directory
		stdout, err := cmd.Output()
		if err != nil {
			return nil, nil, err
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

	return modDownloads, replace, nil
}

func ImportPkgs(directory string, numWorkers int) error {
	modDownloads, _, err := common(directory)
	if err != nil {
		return err
	}

	executor := lib.NewParallelExecutor(numWorkers)
	for _, dl := range modDownloads {
		dl := dl
		executor.Add(func() error {
			log.WithFields(log.Fields{
				"goPackagePath": dl.Path,
			}).Info("Importing sources")

			pathName := filepath.Base(dl.Path) + "_" + dl.Version

			cmd := exec.Command(
				"nix-instantiate",
				"--eval",
				"--expr",
				fmt.Sprintf(`
builtins.filterSource (name: type: baseNameOf name != ".DS_Store") (
  builtins.path {
    path = "%s";
    name = "%s";
  }
)
`, dl.Dir, pathName),
			)
			cmd.Stderr = os.Stderr

			err = cmd.Start()
			if err != nil {
				fmt.Println(cmd)
				return err
			}

			err = cmd.Wait()
			if err != nil {
				fmt.Println(cmd)
				return err
			}

			return nil
		})
	}

	return executor.Wait()
}

func GeneratePkgs(directory string, goMod2NixPath string, numWorkers int) (*schema.GeneratePkgsResult, error) {
	modDownloads, replace, err := common(directory)
	if err != nil {
		return nil, err
	}

	executor := lib.NewParallelExecutor(numWorkers)
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
			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
			}).Info("Calculating NAR hash")

			h := sha256.New()
			err := nar.DumpPathFilter(h, dl.Dir, sourceFilter)
			if err != nil {
				return err
			}
			digest := h.Sum(nil)

			pkg := &schema.Package{
				GoPackagePath: goPackagePath,
				Version:       dl.Version,
				Hash:          "sha256-" + base64.StdEncoding.EncodeToString(digest),
			}
			if hasReplace {
				pkg.ReplacedPath = dl.Path
			}

			addPkg(pkg)

			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
			}).Info("Done calculating NAR hash")

			return nil
		})
	}

	err = executor.Wait()
	if err != nil {
		return nil, err
	}

	// Dependencies are downloaded, run `go mod vendor` to obtain `vendor/modules.txt` without reverse engineering
	const tmpVendorEnvRelative = "vendor-gomod2nix"
	tmpVendorEnvAbsolute := filepath.Join(directory, tmpVendorEnvRelative)
	err = os.RemoveAll(tmpVendorEnvAbsolute)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(tmpVendorEnvAbsolute)
	}()

	var modulesTxt string
	{
		log.Info("Obtaining modules.txt")
		cmd := exec.Command(
			"go", "mod", "vendor", "-o", tmpVendorEnvRelative,
		)
		cmd.Dir = directory
		err = cmd.Run()
		if err != nil {
			return nil, err
		}
		modulesTxtBytes, err := os.ReadFile(filepath.Join(tmpVendorEnvAbsolute, "modules.txt"))
		if err != nil {
			return nil, err
		}
		if len(modulesTxtBytes) == 0 {
			return nil, fmt.Errorf("modules.txt has no content")
		}
		modulesTxt = string(modulesTxtBytes)
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].GoPackagePath < packages[j].GoPackagePath
	})

	return &schema.GeneratePkgsResult{
		Packages:   packages,
		ModulesTxt: string(modulesTxt),
	}, nil

}
