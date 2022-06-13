package generate

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/go/vcs"
)

type TempProject struct {
	Dir           string
	Install       []string
	GoPackagePath string
}

func NewTempProject(packages []string) (*TempProject, error) {
	// Imports without version suffix
	install := make([]string, len(packages))
	for i, imp := range packages {
		idx := strings.Index(imp, "@")
		if idx == -1 {
			idx = len(imp)
		}

		install[i] = imp[:idx]
	}

	var goPackagePath string

	{
		path := install[0]

		log.WithFields(log.Fields{
			"path": path,
		}).Info("Finding repo root for import path")

		repoRoot, err := vcs.RepoRootForImportPath(path, false)
		if err != nil {
			return nil, err
		}

		goPackagePath = repoRoot.Root
	}

	log.Info("Setting up temporary project")

	dir, err := os.MkdirTemp("", "gomod2nix-proj")
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"dir": dir,
	}).Info("Created temporary directory")

	// Create tools.go
	{
		log.WithFields(log.Fields{
			"dir": dir,
		}).Info("Creating tools.go")

		astFile := &ast.File{
			Name: ast.NewIdent("main"),
			Decls: []ast.Decl{
				&ast.GenDecl{
					Tok: token.IMPORT,
					Specs: func() []ast.Spec {
						specs := make([]ast.Spec, len(install))

						i := 0
						for _, imp := range install {
							specs[i] = &ast.ImportSpec{
								Name: ast.NewIdent("_"),
								Path: &ast.BasicLit{
									ValuePos: token.NoPos,
									Kind:     token.STRING,
									Value:    strconv.Quote(imp),
								},
							}

							i++
						}

						return specs
					}(),
				},
			},
		}

		f, err := os.Create(filepath.Join(dir, "tools.go"))
		if err != nil {
			return nil, fmt.Errorf("Error creating tools.go: %v", err)
		}
		defer f.Close()

		fset := token.NewFileSet()
		err = printer.Fprint(f, fset, astFile)
		if err != nil {
			return nil, fmt.Errorf("error writing tools.go: %v", err)
		}

		log.WithFields(log.Fields{
			"dir": dir,
		}).Info("Created tools.go")
	}

	// Set up go module
	{
		log.WithFields(log.Fields{
			"dir": dir,
		}).Info("Initializing go.mod")

		cmd := exec.Command("go", "mod", "init", "gomod2nix/dummy/package")
		cmd.Dir = dir
		cmd.Stderr = os.Stderr

		_, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("error creating go module: %v", err)
		}

		log.WithFields(log.Fields{
			"dir": dir,
		}).Info("Done initializing go.mod")

		// For every dependency fetch it
		for _, imp := range packages {
			log.WithFields(log.Fields{
				"dir": dir,
				"dep": imp,
			}).Info("Getting dependency")

			cmd := exec.Command("go", "get", "-d", imp)
			cmd.Dir = dir
			cmd.Stderr = os.Stderr

			_, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("error fetching '%s': %v", imp, err)
			}

			log.WithFields(log.Fields{
				"dir": dir,
				"dep": imp,
			}).Info("Done getting dependency")
		}
	}

	return &TempProject{
		Dir:           dir,
		Install:       install,
		GoPackagePath: goPackagePath,
	}, nil
}

func (t *TempProject) Remove() error {
	return os.RemoveAll(t.Dir)
}
