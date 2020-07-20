package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

func parseGoSum(file string) (map[string]string, error) {
	commitShaRev := regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:\d+\.)?[0-9]{14}-(.*?)$`)

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

		rev := strings.TrimSuffix(strings.TrimSuffix(f[1], "/go.mod"), "+incompatible")
		if commitShaRev.MatchString(rev) {
			rev = commitShaRev.FindAllStringSubmatch(rev, -1)[0][1]
		}

		pkgs[f[0]] = rev
	}

	return pkgs, nil

}
