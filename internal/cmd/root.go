// Package cmd implements the command line interface for gomod2nix
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	generate "github.com/nix-community/gomod2nix/internal/generate"
	schema "github.com/nix-community/gomod2nix/internal/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const directoryDefault = "./"

var (
	flagDirectory string
	flagOutDir    string
	maxJobs       int
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

	// Write gomod2nix.toml
	{
		goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
		outFile := goMod2NixPath
		pkgs, err := generate.GeneratePkgs(directory, goMod2NixPath, maxJobs)
		if err != nil {
			panic(fmt.Errorf("error generating pkgs: %v", err))
		}

		var goPackagePath string
		var subPackages []string

		if tmpProj != nil {
			subPackages = tmpProj.SubPackages
			goPackagePath = tmpProj.GoPackagePath
		}

		output, err := schema.Marshal(pkgs, goPackagePath, subPackages)
		if err != nil {
			panic(fmt.Errorf("error marshaling output: %v", err))
		}

		err = os.WriteFile(outFile, output, 0644)
		if err != nil {
			panic(fmt.Errorf("error writing file: %v", err))
		}
		log.Info(fmt.Sprintf("Wrote: %s", outFile))
	}
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

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(importCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
