package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	generate "github.com/tweag/gomod2nix/generate"
	schema "github.com/tweag/gomod2nix/schema"
)

var (
	flagDirectory string
	flagOutDir    string
	flagMaxJobs   int
)

var rootCmd = &cobra.Command{
	Use:   "gomod2nix",
	Short: "Convert applications using Go modules -> Nix",
	Run: func(cmd *cobra.Command, args []string) {
		err := generateInternal(flagDirectory, flagOutDir, flagMaxJobs)
		if err != nil {
			panic(err)
		}
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Run gomod2nix.toml generator",
	Run: func(cmd *cobra.Command, args []string) {
		err := generateInternal(flagDirectory, flagOutDir, flagMaxJobs)
		if err != nil {
			panic(err)
		}
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import Go sources into the Nix store",
	Run: func(cmd *cobra.Command, args []string) {
		err := generate.ImportPkgs(flagDirectory, flagMaxJobs)
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDirectory, "dir", "", "Go project directory")
	rootCmd.PersistentFlags().StringVar(&flagOutDir, "outdir", "", "Output directory (defaults to project directory)")
	rootCmd.PersistentFlags().IntVar(&flagMaxJobs, "jobs", 10, "Max number of concurrent jobs")

	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(importCmd)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func generateInternal(directory string, outDir string, maxJobs int) error {
	if outDir == "" {
		outDir = directory
	}

	goMod2NixPath := filepath.Join(outDir, "gomod2nix.toml")
	outFile := goMod2NixPath
	pkgs, err := generate.GeneratePkgs(directory, goMod2NixPath, maxJobs)
	if err != nil {
		return fmt.Errorf("error generating pkgs: %v", err)
	}

	output, err := schema.Marshal(pkgs)
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
