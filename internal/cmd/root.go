// Package cmd implements the command line interface for gomod2nix
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	generate "github.com/nix-community/gomod2nix/internal/generate"
	schema "github.com/nix-community/gomod2nix/internal/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const directoryDefault = "./"

var (
	flagDirectory   string
	flagOutDir      string
	maxJobs         int
	flagWithDeps    bool
	flagNoWorkspace bool
)

func generateFunc(cmd *cobra.Command, args []string) {
	directory := flagDirectory
	outDir := flagOutDir

	// If we are dealing with a project packaged by passing packages on the command line
	// we need to create a temporary project.
	var tmpProj *generate.TempProject
	if len(args) > 0 {
		var err error

		if directory != directoryDefault {
			panic(fmt.Errorf("directory flag not supported together with import arguments"))
		}
		if outDir == "" {
			pwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}

			outDir = pwd
		}

		tmpProj, err = generate.NewTempProject(args)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := tmpProj.Remove()
			if err != nil {
				panic(err)
			}
		}()

		directory = tmpProj.Dir
	} else if outDir == "" {
		// Default out to current working directory if we are developing some software in the current repo.
		outDir = directory
	}

	// Workspace-aware generation: if no CLI args and go.work exists, generate single workspace toml
	if len(args) == 0 && !flagNoWorkspace {
		workspace, err := generate.DetectWorkspace(directory)
		if err != nil {
			panic(fmt.Errorf("error detecting workspace: %v", err))
		}
		if workspace != nil {
			wsOutDir := outDir
			if wsOutDir == "" {
				wsOutDir = workspace.RootDir
			}
			if err := generateWorkspace(workspace, wsOutDir); err != nil {
				panic(err)
			}
			return
		}
	}

	// Single-module generation (original path)
	if err := generateModule(directory, outDir, tmpProj); err != nil {
		panic(err)
	}
}

func generateWorkspace(workspace *generate.WorkspaceInfo, outDir string) error {
	goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")

	pkgs, workspaceModules, err := generate.GenerateWorkspacePkgs(workspace, goMod2NixPath, maxJobs)
	if err != nil {
		return fmt.Errorf("error generating workspace pkgs: %v", err)
	}

	var cachePackages []string
	if flagWithDeps {
		for _, moduleDir := range workspace.ModuleDirs {
			deps, err := generate.GenerateCacheDeps(moduleDir)
			if err != nil {
				log.WithFields(log.Fields{
					"dir": moduleDir,
				}).Warn(fmt.Sprintf("Failed to generate cache deps: %v", err))
				continue
			}
			cachePackages = append(cachePackages, deps...)
		}
		seen := make(map[string]bool)
		var unique []string
		for _, dep := range cachePackages {
			if !seen[dep] {
				seen[dep] = true
				unique = append(unique, dep)
			}
		}
		sort.Strings(unique)
		cachePackages = unique
	}

	output, err := schema.MarshalWorkspace(pkgs, workspaceModules, cachePackages)
	if err != nil {
		return fmt.Errorf("error marshaling workspace output: %v", err)
	}

	err = os.WriteFile(goMod2NixPath, output, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	log.Info(fmt.Sprintf("Wrote workspace lock: %s", goMod2NixPath))
	return nil
}

func generateModule(directory string, outDir string, tmpProj *generate.TempProject) error {
	goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
	outFile := goMod2NixPath
	pkgs, err := generate.GeneratePkgs(directory, goMod2NixPath, maxJobs)
	if err != nil {
		return fmt.Errorf("error generating pkgs: %v", err)
	}

	var goPackagePath string
	var subPackages []string

	if tmpProj != nil {
		subPackages = tmpProj.SubPackages
		goPackagePath = tmpProj.GoPackagePath
	}

	var cachePackages []string
	if flagWithDeps {
		cachePackages, err = generate.GenerateCacheDeps(directory)
		if err != nil {
			return fmt.Errorf("error generating cache packages: %v", err)
		}
	}

	output, err := schema.Marshal(pkgs, goPackagePath, subPackages, cachePackages)
	if err != nil {
		return fmt.Errorf("error marshaling output: %v", err)
	}

	err = os.WriteFile(outFile, output, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	log.Info(fmt.Sprintf("Wrote: %s", outFile))
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "gomod2nix",
	Short: "Convert applications using Go modules -> Nix",
	Run:   generateFunc,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Run gomod2nix.toml generator",
	Run:   generateFunc,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import Go sources into the Nix store",
	Run: func(cmd *cobra.Command, args []string) {
		err := generate.ImportPkgs(flagDirectory, maxJobs)
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDirectory, "dir", "./", "Go project directory")
	rootCmd.PersistentFlags().StringVar(&flagOutDir, "outdir", "", "Output directory (defaults to project directory)")
	rootCmd.PersistentFlags().IntVar(&maxJobs, "jobs", 10, "Max number of concurrent jobs")
	rootCmd.PersistentFlags().BoolVar(&flagWithDeps, "with-deps", false, "Include dependencies in gomod2nix.toml for build cache priming")
	rootCmd.PersistentFlags().BoolVar(&flagNoWorkspace, "no-workspace", false, "Disable automatic go.work workspace detection")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(importCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
