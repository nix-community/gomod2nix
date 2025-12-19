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

			// Capture stderr to detect specific errors
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err = cmd.Run()
			if err != nil {
				stderrStr := stderr.String()
				// Check if this is a "path not in store" error (common with private repos)
				// These can be safely skipped - the sources parameter can be used instead
				if strings.Contains(stderrStr, "did not exist in the store") ||
					strings.Contains(stderrStr, "does not exist") {
					log.WithFields(log.Fields{
						"goPackagePath": dl.Path,
					}).Warn("Skipping import (path not accessible, use 'sources' parameter for private repos)")
					return nil
				}
				// For other errors, print and return
				fmt.Fprint(os.Stderr, stderrStr)
				return err
			}

			return nil
		})
	}

	return executor.Wait()
}

// Convert Go module path to git SSH URL
// e.g., "github.com/org/repo/v2" -> "git@github.com:org/repo.git"
func modulePathToGitURL(goPackagePath string) string {
	parts := strings.Split(goPackagePath, "/")
	if len(parts) < 2 {
		return ""
	}
	host := parts[0]

	// Remove version suffix (e.g., /v2, /v3) from path
	var pathParts []string
	for _, p := range parts[1:] {
		// Skip version directories like v2, v3, etc.
		if len(p) > 1 && p[0] == 'v' {
			if _, err := fmt.Sscanf(p, "v%d", new(int)); err == nil {
				continue
			}
		}
		pathParts = append(pathParts, p)
	}

	if len(pathParts) < 2 {
		// Need at least org/repo
		pathParts = parts[1:]
	}

	// For most hosts, we need org/repo (first two path components)
	repoPath := strings.Join(pathParts, "/")
	if len(pathParts) > 2 {
		// Take only first two components for the repo path
		repoPath = strings.Join(pathParts[:2], "/")
	}

	return fmt.Sprintf("git@%s:%s.git", host, repoPath)
}

// Extract git revision from version string
// For pseudo-versions like v0.0.0-20240823115631-b4344a4c2550, extract the commit hash
func extractGitRev(version string) string {
	// Pseudo-version format: vX.Y.Z-yyyymmddhhmmss-abcdef123456
	if strings.HasPrefix(version, "v0.0.0-") || strings.Count(version, "-") >= 2 {
		parts := strings.Split(version, "-")
		if len(parts) >= 3 {
			return parts[len(parts)-1]
		}
	}
	return ""
}

// Check if version is a pseudo-version (commit-based)
func isPseudoVersion(version string) bool {
	return strings.HasPrefix(version, "v0.0.0-") || strings.Count(version, "-") >= 2
}

// gitFilter filters out .git directory and .DS_Store files from NAR hash computation
func gitFilter(name string, nodeType nar.NodeType) bool {
	base := strings.ToLower(filepath.Base(name))
	// Filter out .git directory and .DS_Store files
	if base == ".git" || base == ".ds_store" {
		return false
	}
	return true
}

// fetchGitHash clones a git repo and computes the NAR hash directly
// This only requires git to be available in the PATH
func fetchGitHash(gitURL, version string) (hash string, rev string, err error) {
	// Create a temporary directory for the clone
	tmpDir, err := os.MkdirTemp("", "gomod2nix-git-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")
	var stderr bytes.Buffer

	if isPseudoVersion(version) {
		// For pseudo-versions, extract the commit hash and fetch that specific commit
		commitHash := extractGitRev(version)
		if commitHash == "" {
			return "", "", fmt.Errorf("could not extract commit hash from version %s", version)
		}

		// Clone the repo (shallow first, then deepen if needed to find the commit)
		cloneCmd := exec.Command("git", "clone", "--filter=blob:none", gitURL, cloneDir)
		stderr.Reset()
		cloneCmd.Stderr = &stderr
		if err := cloneCmd.Run(); err != nil {
			return "", "", fmt.Errorf("git clone failed: %w\n%s", err, stderr.String())
		}

		// Checkout the specific commit (short hash should work now)
		checkoutCmd := exec.Command("git", "-C", cloneDir, "checkout", commitHash)
		stderr.Reset()
		checkoutCmd.Stderr = &stderr
		if err := checkoutCmd.Run(); err != nil {
			return "", "", fmt.Errorf("git checkout failed for commit %s: %w\n%s", commitHash, err, stderr.String())
		}

		rev = commitHash
	} else {
		// For regular versions (tags), clone with the specific tag
		cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, gitURL, cloneDir)
		stderr.Reset()
		cloneCmd.Stderr = &stderr
		if err := cloneCmd.Run(); err != nil {
			return "", "", fmt.Errorf("git clone failed for tag %s: %w\n%s", version, err, stderr.String())
		}
	}

	// Get the full commit hash (rev)
	revCmd := exec.Command("git", "-C", cloneDir, "rev-parse", "HEAD")
	revOutput, err := revCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	rev = strings.TrimSpace(string(revOutput))

	// Compute NAR hash of the checkout (excluding .git directory)
	h := sha256.New()
	if err := nar.DumpPathFilter(h, cloneDir, gitFilter); err != nil {
		return "", "", fmt.Errorf("failed to compute NAR hash: %w", err)
	}
	digest := h.Sum(nil)
	hash = "sha256-" + base64.StdEncoding.EncodeToString(digest)

	return hash, rev, nil
}

func GeneratePkgs(directory string, goMod2NixPath string, numWorkers int) ([]*schema.Package, error) {
	return GeneratePkgsWithPrivate(directory, goMod2NixPath, numWorkers, nil)
}

func GeneratePkgsWithPrivate(directory string, goMod2NixPath string, numWorkers int, privateRepoPrefixes []string) ([]*schema.Package, error) {
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

	// Helper to check if path is private
	isPrivate := func(path string) bool {
		for _, prefix := range privateRepoPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return false
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

			// For private repos, also calculate git hash
			if isPrivate(goPackagePath) {
				gitURL := modulePathToGitURL(goPackagePath)
				if gitURL != "" {
					log.WithFields(log.Fields{
						"goPackagePath": goPackagePath,
						"gitURL":        gitURL,
					}).Info("Fetching git hash for private repo")

					gitHash, gitRev, err := fetchGitHash(gitURL, dl.Version)
					if err != nil {
						log.WithFields(log.Fields{
							"goPackagePath": goPackagePath,
							"error":         err,
						}).Warn("Failed to fetch git hash, skipping git fields")
					} else {
						pkg.GitHash = gitHash
						pkg.GitRev = gitRev
					}
				}
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
