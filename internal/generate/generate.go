// Package generate provides functions to import Go package sources and generate package metadata for Nix.
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
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/nix-community/gomod2nix/internal/lib"
	schema "github.com/nix-community/gomod2nix/internal/schema"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
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
	Origin   *goModOrigin
}

type goModOrigin struct {
	VCS    string
	URL    string
	Subdir string
	Hash   string
	Ref    string
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
			if exiterr, ok := err.(*exec.ExitError); ok {
				return nil, nil, fmt.Errorf("failed to run 'go mod download --json: %s\n%s", exiterr, exiterr.Stderr)
			} else {
				return nil, nil, fmt.Errorf("failed to run 'go mod download --json': %s", err)
			}
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

func GeneratePkgs(directory string, goMod2NixPath string, numWorkers int, privateViaGit bool) ([]*schema.Package, error) {
	modDownloads, replace, err := common(directory)
	if err != nil {
		return nil, err
	}

	var noProxy string
	if privateViaGit {
		noProxy, err = goEnv(directory, "GONOPROXY")
		if err != nil {
			return nil, err
		}
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
		goPackagePath, hasReplace := replace[dl.Path]
		if !hasReplace {
			goPackagePath = dl.Path
		}

		var git *schema.GitOrigin
		if noProxy != "" && module.MatchPrefixPatterns(noProxy, dl.Path) {
			if dl.Origin == nil {
				dl.Origin, err = freshOrigin(directory, dl)
				if err != nil {
					return nil, err
				}
			}
			git = gitOrigin(dl)
			if git == nil {
				log.WithFields(log.Fields{
					"goPackagePath": dl.Path,
				}).Warn("Private module was not fetched from git, keeping the module proxy fetch")
			}
		}

		cached, ok := cache[goPackagePath]
		if ok && cached.Version == dl.Version && reflect.DeepEqual(cached.Git, git) {
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
				Git:           git,
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

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].GoPackagePath < packages[j].GoPackagePath
	})

	return packages, nil
}

// freshOrigin re-downloads one module into a throwaway module cache: Go only records the origin
// on a fetch, so a module cached by an older Go has none.
func freshOrigin(directory string, dl *goModDownload) (*goModOrigin, error) {
	tmp, err := os.MkdirTemp("", "gomod2nix-origin-")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := os.RemoveAll(tmp)
		if err != nil {
			log.WithFields(log.Fields{
				"tmpDir": tmp,
				"err":    err,
			}).Warn("failed to remove temp dir")
		}
	}()

	cmd := exec.Command("go", "mod", "download", "-json", dl.Path+"@"+dl.Version)
	cmd.Dir = directory
	cmd.Env = append(os.Environ(), "GOMODCACHE="+tmp, "GOFLAGS=-modcacherw")
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to re-download %s@%s for its origin: %w", dl.Path, dl.Version, err)
	}

	var fresh goModDownload
	if err := json.Unmarshal(stdout, &fresh); err != nil {
		return nil, err
	}
	return fresh.Origin, nil
}

// gitOrigin returns the git origin of a module, or nil if Go did not fetch it from git.
func gitOrigin(dl *goModDownload) *schema.GitOrigin {
	o := dl.Origin
	if o == nil || o.VCS != "git" || o.Hash == "" {
		return nil
	}

	return &schema.GitOrigin{
		URL:    o.URL,
		Ref:    o.Ref,
		Rev:    o.Hash,
		Subdir: o.Subdir,
	}
}

func goEnv(directory string, key string) (string, error) {
	cmd := exec.Command("go", "env", key)
	cmd.Dir = directory
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run 'go env %s': %w", key, err)
	}
	return strings.TrimSpace(string(stdout)), nil
}

// GenerateCacheDeps generates a list of all imported packages
// (excluding standard library and the current module's packages) for cache optimization.
func GenerateCacheDeps(directory string) ([]string, error) {
	goModPath := filepath.Join(directory, "go.mod")

	log.Info("Parsing go.mod to get current module path")

	// Read and parse go.mod to get current module path
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	mod, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	currentModule := mod.Module.Mod.Path

	log.WithFields(log.Fields{
		"currentModule": currentModule,
	}).Debug("Generating cache dependencies")

	// Run go list to get all imported packages
	cmd := exec.Command("go", "list", "-mod=readonly", "-f", "{{.ImportPath}}", "all")
	cmd.Dir = directory
	stdout, err := cmd.Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to run 'go list': %s\n%s", exiterr, exiterr.Stderr)
		}
		return nil, fmt.Errorf("failed to run 'go list': %w", err)
	}

	// Parse and filter packages
	lines := strings.Split(string(stdout), "\n")
	var filteredPackages []string
	seen := make(map[string]bool)

	for _, line := range lines {
		pkg := strings.TrimSpace(line)
		if pkg == "" {
			continue
		}

		// Skip standard library
		if pkg == "std" {
			continue
		}

		// Skip current module packages
		if pkg == currentModule || strings.HasPrefix(pkg, currentModule+"/") {
			continue
		}

		// Skip vendor packages
		if strings.HasPrefix(pkg, "vendor/") {
			continue
		}

		// Skip internal packages (cannot be imported from outside)
		if strings.Contains(pkg, "/internal") {
			continue
		}

		// Keep only external packages (contain '.')
		if !strings.Contains(pkg, ".") {
			continue
		}

		// Deduplicate
		if !seen[pkg] {
			seen[pkg] = true
			filteredPackages = append(filteredPackages, pkg)
		}
	}

	// Sort for determinism
	sort.Strings(filteredPackages)

	return filteredPackages, nil
}
