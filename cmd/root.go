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
	RunE: func(cmd *cobra.Command, args []string) error {

		return generateCmd(flagDirectory, flagOutDir, flagMaxJobs)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDirectory, "dir", "", "Go project directory")
	rootCmd.PersistentFlags().StringVar(&flagOutDir, "outdir", "", "Output directory (defaults to project directory)")
	rootCmd.PersistentFlags().IntVar(&flagMaxJobs, "jobs", 10, "Max number of concurrent jobs")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func generateCmd(directory string, outDir string, maxJobs int) error {
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
