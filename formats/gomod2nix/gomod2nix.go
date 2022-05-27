package gomod2nix

import (
	"bytes"
	"github.com/BurntSushi/toml"
	"github.com/tweag/gomod2nix/types"
)

const schemaVersion = 1

type packageT struct {
	Version      string `toml:"version"`
	Hash         string `toml:"hash"`
	ReplacedPath string `toml:"replaced,omitempty"`
}

type output struct {
	SchemaVersion int                  `toml:"schema"`
	Mod           map[string]*packageT `toml:"mod"`
}

func Marshal(pkgs []*types.Package) ([]byte, error) {
	out := &output{
		SchemaVersion: schemaVersion,
		Mod:           make(map[string]*packageT),
	}

	for _, pkg := range pkgs {
		out.Mod[pkg.GoPackagePath] = &packageT{
			Version:      pkg.Version,
			Hash:         pkg.Hash,
			ReplacedPath: pkg.ReplacedPath,
		}
	}

	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)
	err := e.Encode(out)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
