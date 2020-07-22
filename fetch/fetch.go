package fetch

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"github.com/tweag/gomod2nix/types"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/tools/go/vcs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

func worker(id int, caches []map[string]*types.Package, jobs <-chan *packageJob, results chan<- *packageResult) {
	log.WithField("workerId", id).Info("Starting worker process")

	for j := range jobs {
		log.WithFields(log.Fields{
			"workerId":      id,
			"goPackagePath": j.goPackagePath,
		}).Info("Worker received job")

		pkg, err := fetchPackage(caches, j.importPath, j.goPackagePath, j.sumVersion)
		results <- &packageResult{
			err: err,
			pkg: pkg,
		}
	}
}

func FetchPackages(goModPath string, goSumPath string, goMod2NixPath string, depsNixPath string, numWorkers int, keepGoing bool) ([]*types.Package, error) {

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

	caches := []map[string]*types.Package{}
	goModCache := gomod2nix.LoadGomod2Nix(goMod2NixPath)
	if len(goModCache) > 0 {
		caches = append(caches, goModCache)
	}
	buildGoCache := buildgopackage.LoadDepsNix(depsNixPath)
	if len(buildGoCache) > 0 {
		caches = append(caches, buildGoCache)
	}

	// Map repos -> replacement repo
	replace := make(map[string]string)
	for _, repl := range mod.Replace {
		replace[repl.New.Path] = repl.Old.Path
	}

	log.WithFields(log.Fields{
		"sumPath": goSumPath,
	}).Info("Parsing go.sum")

	sumVersions, err := parseGoSum(goSumPath)
	if err != nil {
		return nil, err
	}

	numJobs := len(sumVersions)
	if numJobs < numWorkers {
		numWorkers = numJobs
	}

	log.WithFields(log.Fields{
		"numWorkers": numWorkers,
	}).Info("Starting worker processes")
	jobs := make(chan *packageJob, numJobs)
	results := make(chan *packageResult, numJobs)
	for i := 0; i <= numWorkers; i++ {
		go worker(i, caches, jobs, results)
	}

	log.WithFields(log.Fields{
		"numJobs": numJobs,
	}).Info("Queuing jobs")
	for importPath, sumVersion := range sumVersions {
		// Check for replacement path (only original goPackagePath is recorded in go.sum)
		goPackagePath := importPath
		v, ok := replace[goPackagePath]
		if ok {
			goPackagePath = v
		}

		jobs <- &packageJob{
			importPath:    importPath,
			goPackagePath: goPackagePath,
			sumVersion:    sumVersion,
		}
	}
	close(jobs)

	var pkgs []*types.Package
	for i := 1; i <= numJobs; i++ {
		result := <-results

		log.WithFields(log.Fields{
			"current": i,
			"total":   numJobs,
		}).Info("Received finished job")

		if result.err != nil {
			if keepGoing {
				fmt.Println(result.err)
				continue
			} else {
				return nil, result.err
			}
		}

		pkgs = append(pkgs, result.pkg)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].GoPackagePath < pkgs[j].GoPackagePath
	})

	return pkgs, nil
}

func fetchPackage(caches []map[string]*types.Package, importPath string, goPackagePath string, sumVersion string) (*types.Package, error) {
	repoRoot, err := vcs.RepoRootForImportPath(importPath, false)
	if err != nil {
		return nil, err
	}

	commitShaRev := regexp.MustCompile(`v\d+\.\d+\.\d+-[\d+\.a-zA-Z]*?[0-9]{14}-(.*?)$`)
	rev := strings.TrimSuffix(sumVersion, "+incompatible")
	if commitShaRev.MatchString(rev) {
		rev = commitShaRev.FindAllStringSubmatch(rev, -1)[0][1]
	}

	goPackagePathPrefix, pathMajor, _ := module.SplitPathVersion(goPackagePath)

	// Relative path within the repo
	relPath := strings.TrimPrefix(goPackagePathPrefix, repoRoot.Root+"/")
	if relPath == goPackagePathPrefix {
		relPath = ""
	}

	if len(caches) > 0 {
		log.WithFields(log.Fields{
			"goPackagePath": goPackagePath,
		}).Info("Checking previous invocation cache")

		for _, cache := range caches {
			cached, ok := cache[goPackagePath]
			if ok {
				if cached.SumVersion == sumVersion {
					log.WithFields(log.Fields{
						"goPackagePath": goPackagePath,
					}).Info("Returning cached entry")
					return cached, nil
				}
			}
		}
	}

	if repoRoot.VCS.Name != "Git" {
		return nil, fmt.Errorf("Only git repositories are supported")
	}

	type prefetchOutput struct {
		URL    string `json:"url"`
		Rev    string `json:"rev"`
		Sha256 string `json:"sha256"`
		Path   string `json:"path"`
		// date   string
		// fetchSubmodules bool
		// deepClone       bool
		// leaveDotGit     bool
	}

	log.WithFields(log.Fields{
		"goPackagePath": goPackagePath,
		"rev":           rev,
	}).Info("Cache miss, fetching")
	stdout, err := exec.Command(
		"nix-prefetch-git",
		"--quiet",
		"--fetch-submodules",
		"--url", repoRoot.Repo,
		"--rev", rev).Output()
	if err != nil {
		newRev := fmt.Sprintf("%s/%s", relPath, rev)

		log.WithFields(log.Fields{
			"goPackagePath": goPackagePath,
			"rev":           newRev,
		}).Info("Fetching failed, retrying with different rev format")
		originalErr := err
		stdout, err = exec.Command(
			"nix-prefetch-git",
			"--quiet",
			"--fetch-submodules",
			"--url", repoRoot.Repo,
			"--rev", newRev).Output()
		if err != nil {
			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
			}).Error("Fetching failed")
			return nil, originalErr
		}

		rev = newRev
	}

	var output *prefetchOutput
	err = json.Unmarshal(stdout, &output)
	if err != nil {
		return nil, err
	}

	vendorPath := ""
	if importPath != goPackagePath {
		vendorPath = importPath
	}

	if relPath == "" && pathMajor != "" {
		p := filepath.Join(output.Path, pathMajor)
		_, err := os.Stat(p)
		if err == nil {
			fmt.Println(pathMajor)
			relPath = strings.TrimPrefix(pathMajor, "/")
		}
	}

	return &types.Package{
		GoPackagePath: goPackagePath,
		URL:           repoRoot.Repo,
		Rev:           output.Rev,
		Sha256:        output.Sha256,
		// This is used to skip fetching where the previous package path & versions are still the same
		// It's also used to construct the vendor directory in the Nix build
		SumVersion: sumVersion,
		RelPath:    relPath,
		VendorPath: vendorPath,
	}, nil

}
