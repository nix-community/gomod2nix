package gomod2nix

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/tweag/gomod2nix/types"
	"io/ioutil"
)

type fetchInfo struct {
	Type   string `toml:"type"`
	URL    string `toml:"url"`
	Rev    string `toml:"rev"`
	Sha256 string `toml:"sha256"`
}

type packageT struct {
	SumVersion string     `toml:"sumVersion"`
	RelPath    string     `toml:"relPath,omitempty"`
	VendorPath string     `toml:"vendorPath,omitempty"`
	Fetch      *fetchInfo `toml:"fetch"`
}

func Marshal(pkgs []*types.Package) ([]byte, error) {
	result := make(map[string]*packageT)

	for _, pkg := range pkgs {
		result[pkg.GoPackagePath] = &packageT{
			VendorPath: pkg.VendorPath,
			SumVersion: pkg.SumVersion,
			RelPath:    pkg.RelPath,
			Fetch: &fetchInfo{
				Type:   "git",
				URL:    pkg.URL,
				Rev:    pkg.Rev,
				Sha256: pkg.Sha256,
			},
		}
	}

	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)
	err := e.Encode(result)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func LoadGomod2Nix(filePath string) map[string]*types.Package {
	ret := make(map[string]*types.Package)

	if filePath == "" {
		return ret
	}

	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println(err)
		return ret
	}

	result := make(map[string]*packageT)
	_, err = toml.Decode(string(b), &result)
	if err != nil {
		fmt.Println(err)
		return ret
	}

	for k, v := range result {
		ret[k] = &types.Package{
			GoPackagePath: k,
			URL:           v.Fetch.URL,
			Rev:           v.Fetch.Rev,
			Sha256:        v.Fetch.Sha256,
			SumVersion:    v.SumVersion,
			RelPath:       v.RelPath,
			VendorPath:    v.VendorPath,
		}
	}

	return ret
}
