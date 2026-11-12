package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate testdata/*/modules.txt with `go mod vendor`")

func TestModulesTxt(t *testing.T) {
	for _, fixture := range []string{"go1.24", "go1.16", "go1.13"} {
		t.Run(fixture, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", fixture))
			if *update {
				vendorGolden(t)
			}

			mainMod := readGoMod("go.mod")
			modules := resolveBuildList(nil, mainMod)
			got := render(mainMod.Go, modules, unusedReplacements(mainMod, modules))

			want, err := os.ReadFile("modules.txt")
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("modules.txt mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func vendorGolden(t *testing.T) {
	out := t.TempDir()
	cmd := exec.Command("go", "mod", "vendor", "-o", out)
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GOPROXY=off")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod vendor: %v\n%s", err, b)
	}
	golden, err := os.ReadFile(filepath.Join(out, "modules.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("modules.txt", golden, 0o644); err != nil {
		t.Fatal(err)
	}
}
