package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Package struct {
	GoPackagePath string `json:"-"`
	Version       string `json:"version"`
	Hash          string `json:"hash"`
	ReplacedPath  string `json:"replaced,omitempty"`
}

func main() {
	sources := make(map[string]string)
	pkgs := make(map[string]*Package)

	{
		b, err := os.ReadFile(os.Getenv("sourcesPath"))
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(b, &sources)
		if err != nil {
			panic(err)
		}
	}

	{
		b, err := os.ReadFile(os.Getenv("jsonPath"))
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(b, &pkgs)
		if err != nil {
			panic(err)
		}
	}

	keys := make([]string, 0, len(pkgs))
	for key := range pkgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate, in reverse order
	for i := len(keys) - 1; i >= 0; i-- {
		key := keys[i]
		src := sources[key]

		paths := []string{key}

		for _, path := range paths {

			vendorDir := filepath.Join("vendor", filepath.Dir(path))
			if err := os.MkdirAll(vendorDir, 0755); err != nil {
				panic(err)
			}

			if _, err := os.Stat(filepath.Join("vendor", path)); err == nil {
				files, err := os.ReadDir(src)
				if err != nil {
					panic(err)
				}

				for _, f := range files {
					innerSrc := filepath.Join(src, f.Name())
					dst := filepath.Join("vendor", path, f.Name())
					if err := os.Symlink(innerSrc, dst); err != nil {
						// assume it's an existing directory, try to link the directory content instead.
						// TODO should we do this recursively
						files, err := os.ReadDir(innerSrc)
						if err != nil {
							panic(err)
						}
						for _, f := range files {
							if err := os.Symlink(filepath.Join(innerSrc, f.Name()), filepath.Join(dst, f.Name())); err != nil {
								fmt.Println("ignore symlink error", filepath.Join(innerSrc, f.Name()), filepath.Join(dst, f.Name()))
							}
						}
					}
				}

				continue
			}

			// If the file doesn't already exist, just create a simple symlink
			err := os.Symlink(src, filepath.Join("vendor", path))
			if err != nil {
				panic(err)
			}

		}
	}

}
