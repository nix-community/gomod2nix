package fetch

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
)

func parseGoSum(file string) (map[string]string, error) {

	// Read go.mod
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	pkgs := make(map[string]string) // goPackagepath -> rev
	for lineno, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}

		f := strings.Fields(string(line))
		if len(f) != 3 {
			return nil, fmt.Errorf("malformed go.sum:\n%s:%d: wrong number of fields %v", file, lineno, len(f))
		}

		pkgs[f[0]] = strings.TrimSuffix(f[1], "/go.mod")
	}

	return pkgs, nil

}
