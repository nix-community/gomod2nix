package fetch

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/tweag/gomod2nix/formats/buildgopackage"
	"github.com/tweag/gomod2nix/formats/gomod2nix"
	"github.com/tweag/gomod2nix/types"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/tools/go/vcs"
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

var GithubClient *github.Client
var githubRepoRegexp *regexp.Regexp = regexp.MustCompile(`^https://github.com/([^/]+)/([^/]+)`)

func resolveFullRev(repoRoot *vcs.RepoRoot, rev string) (string, error) {
	// try using github api
	if GithubClient != nil {
		repoParts := githubRepoRegexp.FindStringSubmatch(repoRoot.Repo)
		if len(repoParts) > 0 {
			owner := repoParts[1]
			repo := repoParts[2]

			commit, _, _ := GithubClient.Repositories.GetCommit(context.Background(), owner, repo, rev)
			if commit != nil && commit.SHA != nil {
				return *commit.SHA, nil
			}
		}
	}

	// fallback to cloning repo
	dirTop, err := ioutil.TempDir("", "vcstop-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dirTop)

	dir := filepath.Join(dirTop, "repo")

	err = repoRoot.VCS.CreateAtRev(dir, repoRoot.Repo, rev)
	if err != nil {
		return "", err
	}

	stdout, err := exec.Command(
		"git",
		"-C",
		dir,
		"rev-parse",
		rev,
	).Output()

	if err != nil {
		return "", err
	}

	if len(stdout) > 0 {
		// remove ending new line
		return string(stdout[:len(stdout)-1]), nil
	} else {
		return "", errors.New("git rev-parse output was empty")
	}
}

func fetchGitExprForRev(repo, rev string) string {
	return fmt.Sprintf(
		"builtins.fetchGit { url = %q; allRefs = true; rev = %q; submodules = true; }",
		repo,
		rev,
	)
}

func fetchGitExprForTag(repo, tag string) string {
	return fmt.Sprintf(
		"builtins.fetchGit { url = %q; ref = %q; submodules = true; }",
		repo,
		"refs/tags/"+tag,
	)
}

func fetchPackage(caches []map[string]*types.Package, importPath string, goPackagePath string, sumVersion string) (*types.Package, error) {
	repoRoot, err := vcs.RepoRootForImportPath(importPath, false)
	if err != nil {
		return nil, err
	}

	commitShaRev := regexp.MustCompile(`v\d+\.\d+\.\d+-[\d+\.a-zA-Z]*?[0-9]{14}-(.*?)$`)
	rev := strings.TrimSuffix(sumVersion, "+incompatible")
	var makeFetchExpr func(string, string) string
	if commitShaRev.MatchString(rev) {
		makeFetchExpr = fetchGitExprForRev
		rev = commitShaRev.FindAllStringSubmatch(rev, -1)[0][1]
		rev, err = resolveFullRev(repoRoot, rev)
		if err != nil {
			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
			}).Error("Fetching failed")
			return nil, err
		}
	} else {
		makeFetchExpr = fetchGitExprForTag
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
		LastModified     int    `json:"lastModified"`
		LastModifiedDate string `json:"lastModifiedDate"`
		NarHash          string `json:"narHash"`
		Path             string `json:"path"`
		Rev              string `json:"rev"`
		RevCount         int    `json:"revCount"`
		ShortRev         string `json:"shortRev"`
		Submodules       bool   `json:"submodules"`
	}

	log.WithFields(log.Fields{
		"goPackagePath": goPackagePath,
		"rev":           rev,
	}).Info("Cache miss, fetching")

	stdout, err := exec.Command(
		"nix",
		"eval",
		"--impure",
		"--json",
		"--expr",
		fmt.Sprintf(
			// we need to rename the outPath attr because nix's JSON output format
			// logic is such that an attrset with an outPath attr will be printed
			// as a solitary JSON string consisting of the outPath.
			"let r = (%s); in builtins.removeAttrs r [\"outPath\"] // { path = r.outPath; }",
			makeFetchExpr(repoRoot.Repo, rev),
		),
	).Output()

	if err != nil {
		if relPath != "" {
			// handle cases like cloud.google.com/go/datastore where rev is v1.1.0
			// the ref to fetch is refs/tags/datastore/v1.1.0,
			// not refs/tags/v1.1.0.
			//
			// TODO: look through go source code and briefly document how this logic
			// is supposed to work precisely.
			newRev := fmt.Sprintf("%s/%s", relPath, rev)

			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
				"rev":           newRev,
			}).Info("Fetching failed, retrying with different rev format")
			originalErr := err

			stdout, err = exec.Command(
				"nix",
				"eval",
				"--impure",
				"--json",
				"--expr",
				fmt.Sprintf(
					"let r = (%s); in builtins.removeAttrs r [\"outPath\"] // { path = r.outPath; }",
					makeFetchExpr(repoRoot.Repo, newRev),
				),
			).Output()
			if err != nil {
				log.WithFields(log.Fields{
					"goPackagePath": goPackagePath,
				}).Error("Fetching failed")
				return nil, originalErr
			}

			rev = newRev
		} else {
			// no relative path to try: propagate original error
			log.WithFields(log.Fields{
				"goPackagePath": goPackagePath,
			}).Error("Fetching failed")
			return nil, err
		}
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

	// need to convert SRI sha256 hash to sha256 hex
	if !strings.HasPrefix(output.NarHash, "sha256-") {
		log.WithFields(log.Fields{
			"goPackagePath": goPackagePath,
		}).Error("Fetching failed")
		return nil, fmt.Errorf("Error: NarHash didn't begin with sha256- prefix: %s", output.NarHash)
	}

	b, err := base64.StdEncoding.DecodeString(output.NarHash[7:])
	if err != nil {
		log.WithFields(log.Fields{
			"goPackagePath": goPackagePath,
		}).Error("Fetching failed")
		return nil, err
	}

	sha256 := hex.EncodeToString(b)

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
		Sha256:        sha256,
		// This is used to skip fetching where the previous package path & versions are still the same
		// It's also used to construct the vendor directory in the Nix build
		SumVersion: sumVersion,
		RelPath:    relPath,
		VendorPath: vendorPath,
	}, nil

}
