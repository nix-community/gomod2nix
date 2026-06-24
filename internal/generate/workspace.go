// Package generate provides functions to import Go package sources and generate package metadata for Nix.
package generate

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
)

// WorkspaceInfo holds parsed go.work data.
type WorkspaceInfo struct {
	WorkFilePath string
	RootDir      string
	ModuleDirs   []string // absolute paths resolved from use directives
	GoVersion    string
	WorkFile     *modfile.WorkFile // parsed go.work, avoids re-reading
}

// DetectWorkspace checks if the directory contains a go.work file and parses it.
// Returns nil if no go.work file is found.
func DetectWorkspace(directory string) (*WorkspaceInfo, error) {
	workFilePath := filepath.Join(directory, "go.work")

	data, err := os.ReadFile(workFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	log.WithFields(log.Fields{
		"workFile": workFilePath,
	}).Info("Detected go.work file")

	workFile, err := modfile.ParseWork(workFilePath, data, nil)
	if err != nil {
		return nil, err
	}

	absDir, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}

	var moduleDirs []string
	for _, use := range workFile.Use {
		moduleDir := filepath.Join(absDir, use.Path)
		goModPath := filepath.Join(moduleDir, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			log.WithFields(log.Fields{
				"path": use.Path,
			}).Warn("Workspace use directive points to directory without go.mod, skipping")
			continue
		}
		moduleDirs = append(moduleDirs, moduleDir)
	}

	goVersion := ""
	if workFile.Go != nil {
		goVersion = workFile.Go.Version
	}

	log.WithFields(log.Fields{
		"modules": len(moduleDirs),
	}).Info("Parsed workspace modules")

	return &WorkspaceInfo{
		WorkFilePath: workFilePath,
		RootDir:      absDir,
		ModuleDirs:   moduleDirs,
		GoVersion:    goVersion,
		WorkFile:     workFile,
	}, nil
}

// ModulePathMap returns a mapping from Go module path to relative directory path
// for all workspace modules.
func (w *WorkspaceInfo) ModulePathMap() (map[string]string, error) {
	result := make(map[string]string)
	for _, dir := range w.ModuleDirs {
		goModPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", goModPath, err)
		}
		mod, err := modfile.Parse(goModPath, data, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}
		rel, err := filepath.Rel(w.RootDir, dir)
		if err != nil {
			return nil, err
		}
		result[mod.Module.Mod.Path] = rel
	}
	return result, nil
}
