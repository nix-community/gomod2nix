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
		cmd.Env = append(os.Environ(), "GOWORK=off")
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

func hashPackages(modDownloads []*goModDownload, replace map[string]string, cache map[string]*schema.Package, numWorkers int) ([]*schema.Package, error) {
	executor := lib.NewParallelExecutor(numWorkers)
	var mux sync.Mutex

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

	err := executor.Wait()
	if err != nil {
		return nil, err
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].GoPackagePath < packages[j].GoPackagePath
	})

	return packages, nil
}

func GeneratePkgs(directory string, goMod2NixPath string, numWorkers int) ([]*schema.Package, error) {
	modDownloads, replace, err := common(directory)
	if err != nil {
		return nil, err
	}

	cache := schema.ReadCache(goMod2NixPath)
	return hashPackages(modDownloads, replace, cache, numWorkers)
}

func stripModVersion(s string) string {
	if idx := strings.Index(s, "@"); idx != -1 {
		return s[:idx]
	}
	return s
}

// commonWorkspace downloads all external modules for the workspace and collects
// non-local replace directives from go.work and all module go.mod files.
func commonWorkspace(workspace *WorkspaceInfo) ([]*goModDownload, map[string]string, error) {
	log.WithFields(log.Fields{
		"rootDir": workspace.RootDir,
	}).Info("Processing workspace dependencies")

	replace := make(map[string]string)
	for _, repl := range workspace.WorkFile.Replace {
		if repl.New.Version == "" {
			continue
		}
		replace[repl.New.Path] = repl.Old.Path
	}

	for _, moduleDir := range workspace.ModuleDirs {
		goModPath := filepath.Join(moduleDir, "go.mod")
		data, err := os.ReadFile(goModPath)
		if err != nil {
			continue
		}
		mod, err := modfile.Parse(goModPath, data, nil)
		if err != nil {
			continue
		}
		for _, repl := range mod.Replace {
			if repl.New.Version == "" {
				continue
			}
			if _, exists := replace[repl.New.Path]; !exists {
				replace[repl.New.Path] = repl.Old.Path
			}
		}
	}

	log.Info("Downloading workspace dependencies")

	cmd := exec.Command("go", "mod", "download", "--json")
	cmd.Dir = workspace.RootDir
	stdout, err := cmd.Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return nil, nil, fmt.Errorf("failed to run 'go mod download --json': %s\n%s", exiterr, exiterr.Stderr)
		}
		return nil, nil, fmt.Errorf("failed to run 'go mod download --json': %s", err)
	}

	var modDownloads []*goModDownload
	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var dl *goModDownload
		err := dec.Decode(&dl)
		if err == io.EOF {
			break
		}
		modDownloads = append(modDownloads, dl)
	}

	log.Info("Done downloading workspace dependencies")

	return modDownloads, replace, nil
}

// computeWorkspaceDeps uses `go mod graph` to compute transitive external
// dependencies per workspace module. Keys in the returned map are Go module
// paths (e.g. "example.com/hello"), values contain the relative directory
// and the list of external dependency module paths.
func computeWorkspaceDeps(workspace *WorkspaceInfo) (map[string]*schema.WorkspaceModuleInfo, error) {
	modulePathMap, err := workspace.ModulePathMap()
	if err != nil {
		return nil, err
	}

	workspaceModPaths := make(map[string]bool)
	for modPath := range modulePathMap {
		workspaceModPaths[modPath] = true
	}

	log.Info("Computing per-module dependency graph")

	cmd := exec.Command("go", "mod", "graph")
	cmd.Dir = workspace.RootDir
	stdout, err := cmd.Output()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to run 'go mod graph': %s\n%s", exiterr, exiterr.Stderr)
		}
		return nil, fmt.Errorf("failed to run 'go mod graph': %w", err)
	}

	edges := make(map[string][]string)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		from := stripModVersion(parts[0])
		to := stripModVersion(parts[1])
		edges[from] = append(edges[from], to)
	}

	result := make(map[string]*schema.WorkspaceModuleInfo)
	for modPath, relDir := range modulePathMap {
		visited := make(map[string]bool)
		queue := []string{modPath}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if visited[current] {
				continue
			}
			visited[current] = true
			for _, dep := range edges[current] {
				if !visited[dep] {
					queue = append(queue, dep)
				}
			}
		}
		var deps []string
		for dep := range visited {
			if !workspaceModPaths[dep] && dep != "go" && dep != "toolchain" {
				deps = append(deps, dep)
			}
		}
		sort.Strings(deps)
		result[modPath] = &schema.WorkspaceModuleInfo{
			Dir:  relDir,
			Deps: deps,
		}
	}

	log.Info("Done computing per-module dependency graph")

	return result, nil
}

// GenerateWorkspacePkgs generates packages and a per-module dependency map
// for an entire Go workspace, producing a single unified lock file.
func GenerateWorkspacePkgs(workspace *WorkspaceInfo, goMod2NixPath string, numWorkers int) ([]*schema.Package, map[string]*schema.WorkspaceModuleInfo, error) {
	modDownloads, replace, err := commonWorkspace(workspace)
	if err != nil {
		return nil, nil, err
	}

	workspaceDeps, err := computeWorkspaceDeps(workspace)
	if err != nil {
		return nil, nil, err
	}

	cache := schema.ReadCache(goMod2NixPath)
	packages, err := hashPackages(modDownloads, replace, cache, numWorkers)
	if err != nil {
		return nil, nil, err
	}

	return packages, workspaceDeps, nil
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
	cmd.Env = append(os.Environ(), "GOWORK=off")
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
