package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/tools/go/vcs"
	"os/exec"
)

type Package struct {
	GoPackagePath string
	URL           string
	Rev           string
	Hash          string
}

func fetchPackage(replace map[string]string, goPackagePath string, rev string) (*Package, error) {

	// Check for replacement path (only original goPackagePath is recorded in go.sum)
	repo := goPackagePath
	v, ok := replace[goPackagePath]
	if ok {
		repo = v
	}

	repoRoot, err := vcs.RepoRootForImportPath(repo, false)
	if err != nil {
		return nil, err
	}

	if repoRoot.VCS.Name != "Git" {
		return nil, fmt.Errorf("Only git repositories are supported")
	}

	type prefetchOutput struct {
		URL    string `json:"url"`
		Rev    string `json:"rev"`
		Sha256 string `json:"sha256"`
		// path   string
		// date   string
		// fetchSubmodules bool
		// deepClone       bool
		// leaveDotGit     bool
	}
	stdout, err := exec.Command(
		"nix-prefetch-git",
		"--quiet",
		"--fetch-submodules",
		"--url", repoRoot.Repo,
		"--rev", rev).Output()
	if err != nil {
		return nil, err
	}

	var output *prefetchOutput

	err = json.Unmarshal(stdout, &output)
	if err != nil {
		return nil, err
	}

	return &Package{
		GoPackagePath: goPackagePath,
		URL:           repoRoot.Repo,
		// It may feel appealing to use output.Rev to get the full git hash
		// However, this has the major downside of not being able to be checked against an
		// older output file (as the revs) don't match
		//
		// This is used to skip fetching where the previous package path & rev are still the same
		Rev:  rev,
		Hash: fmt.Sprintf("sha256:%s", output.Sha256),
	}, nil

}
