// Package schema defines the schema for the package metadata used in caching and serialization.
package schema

import (
	"bytes"
	"os"

	"github.com/pelletier/go-toml/v2"
)

const SchemaVersion = 3

type Package struct {
	GoPackagePath string `toml:"-"`
	Version       string `toml:"version"`
	Hash          string `toml:"hash"`
	ReplacedPath  string `toml:"replaced,omitempty"`
}

type Output struct {
	SchemaVersion int                 `toml:"schema"`
	Mod           map[string]*Package `toml:"mod"`

	// Packages with passed import paths trigger `go install` based on this list
	SubPackages []string `toml:"subPackages,omitempty"`

	// Packages with passed import paths has a "default package" which pname & version is inherit from
	GoPackagePath string `toml:"goPackagePath,omitempty"`

	// List of packages to pre-compile in build cache for faster builds
	CachePackages []string `toml:"cachePackages,multiline,omitempty"`
}

func Marshal(pkgs []*Package, goPackagePath string, subPackages []string, cachePackages []string) ([]byte, error) {
	out := &Output{
		SchemaVersion: SchemaVersion,
		Mod:           make(map[string]*Package),
		SubPackages:   subPackages,
		GoPackagePath: goPackagePath,
		CachePackages: cachePackages,
	}

	for _, pkg := range pkgs {
		out.Mod[pkg.GoPackagePath] = pkg
	}

	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)
	e.SetIndentTables(true)
	err := e.Encode(out)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func ReadCache(filePath string) map[string]*Package {
	ret := make(map[string]*Package)

	if filePath == "" {
		return ret
	}

	b, err := os.ReadFile(filePath)
	if err != nil {
		return ret
	}

	var output Output
	if err := toml.Unmarshal(b, &output); err != nil {
		return ret
	}

	if output.SchemaVersion != SchemaVersion {
		return ret
	}

	for k, v := range output.Mod {
		v.GoPackagePath = k
		ret[k] = v
	}

	return ret
}
