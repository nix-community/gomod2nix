package types

import (
	"bytes"
	"os"

	"github.com/BurntSushi/toml"
)

const SchemaVersion = 4

type Package struct {
	GoPackagePath string `toml:"-"`
	Version       string `toml:"version"`
	Hash          string `toml:"hash"`
	ReplacedPath  string `toml:"replaced,omitempty"`
}

type Output struct {
	SchemaVersion int                 `toml:"schema"`
	Mod           map[string]*Package `toml:"mod"`
	VendorModules string              `toml:"vendorModulesTxt"`

	// Packages with passed import paths trigger `go install` based on this list
	SubPackages []string `toml:"subPackages,omitempty"`

	// Packages with passed import paths has a "default package" which pname & version is inherit from
	GoPackagePath string `toml:"goPackagePath,omitempty"`
}

type GeneratePkgsResult struct {
	Packages   []*Package
	ModulesTxt string
}

func Marshal(generated *GeneratePkgsResult, goPackagePath string, subPackages []string) ([]byte, error) {
	out := &Output{
		SchemaVersion: SchemaVersion,
		Mod:           make(map[string]*Package),
		SubPackages:   subPackages,
		GoPackagePath: goPackagePath,
		VendorModules: generated.ModulesTxt,
	}

	for _, pkg := range generated.Packages {
		out.Mod[pkg.GoPackagePath] = pkg
	}

	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)
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
	_, err = toml.Decode(string(b), &output)
	if err != nil {
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
