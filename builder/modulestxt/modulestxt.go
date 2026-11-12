package main

import (
	"encoding/json"
	"fmt"
	"go/version"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func main() {
	manifest := readManifest("goModulesManifest")
	mainMod := readGoMod("go.mod")

	modules := resolveBuildList(manifest, mainMod)
	unused := unusedReplacements(mainMod, modules)

	modulesTxt := render(mainMod.Go, modules, unused)

	writeModulesTxt(modulesTxt)
}

// manifestEntry is one module of the JSON manifest, keyed by module path.
type manifestEntry struct {
	Version      string `json:"version"`
	ReplacedPath string `json:"replaced,omitempty"`
	Source       string `json:"source"`
}

// ModVersion is a module path and version as printed by `go mod edit -json`.
type ModVersion struct {
	Path    string
	Version string
}

// Replace is a replace directive; an empty Old.Version matches every version.
type Replace struct {
	Old ModVersion
	New ModVersion
}

// GoMod is the subset of `go mod edit -json` output used here.
type GoMod struct {
	Go      string
	Require []ModVersion
	Replace []Replace
}

// vendoredModule is one module block of vendor/modules.txt.
type vendoredModule struct {
	ModVersion
	Replacement *ModVersion
	Explicit    bool
	GoVersion   string
	Packages    []string
}

// Inputs

func readManifest(env string) map[string]manifestEntry {
	path := os.Getenv(env)
	if path == "" {
		panic(fmt.Sprintf("env var '%s' was unset", env))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var manifest map[string]manifestEntry
	if err := json.Unmarshal(b, &manifest); err != nil {
		panic(err)
	}
	return manifest
}

func readGoMod(path string) GoMod {
	mod, err := parseGoMod(path)
	if err != nil {
		panic(err)
	}
	return mod
}

func parseGoMod(path string) (GoMod, error) {
	out, err := exec.Command("go", "mod", "edit", "-json", path).Output()
	if err != nil {
		return GoMod{}, fmt.Errorf("go mod edit -json %s: %w", path, err)
	}
	var mod GoMod
	err = json.Unmarshal(out, &mod)
	return mod, err
}

// Build list

// resolveBuildList joins the manifest with the requires and replaces of mainMod.
func resolveBuildList(manifest map[string]manifestEntry, mainMod GoMod) []vendoredModule {
	required := make(map[string]string, len(mainMod.Require))
	for _, r := range mainMod.Require {
		required[r.Path] = r.Version
	}
	replaces := indexReplaces(mainMod.Replace)

	inBuildList := make(map[string]bool, len(manifest)+len(required))
	sources := make(map[string]string, len(manifest))
	for path, m := range manifest {
		inBuildList[path] = true
		sources[path] = m.Source
	}
	for path := range required {
		inBuildList[path] = true
	}
	for _, r := range mainMod.Replace {
		if r.New.Version == "" {
			// A local replacement path is relative to go.mod, which is in the working directory.
			sources[r.Old.Path] = r.New.Path
		}
	}

	modules := make([]vendoredModule, 0, len(inBuildList))
	for _, path := range slices.Sorted(maps.Keys(inBuildList)) {
		mod := ModVersion{Path: path, Version: selectedVersion(path, required, manifest, replaces)}
		_, explicit := required[path]
		m := vendoredModule{
			ModVersion:  mod,
			Replacement: replaces.lookup(mod),
			Explicit:    explicit,
		}
		if dir, ok := sources[path]; ok {
			m.GoVersion = goDirective(dir)
			m.Packages = packages(path, dir, inBuildList)
		}
		modules = append(modules, m)
	}
	return modules
}

// selectedVersion returns the version selected for path before replacement, or "" if unknown.
func selectedVersion(
	path string,
	required map[string]string,
	manifest map[string]manifestEntry,
	replaces replaceIndex,
) string {
	// go.mod wins over gomod2nix.toml so a stale toml still builds, as it does without modules.txt.
	if v, ok := required[path]; ok {
		return v
	}
	if m, ok := manifest[path]; ok && m.ReplacedPath == "" {
		return m.Version
	}
	return replaces.oldVersion[path]
}

type replaceIndex struct {
	exact      map[ModVersion]ModVersion
	wildcard   map[string]ModVersion
	oldVersion map[string]string
}

func indexReplaces(replaces []Replace) replaceIndex {
	idx := replaceIndex{
		exact:      make(map[ModVersion]ModVersion),
		wildcard:   make(map[string]ModVersion),
		oldVersion: make(map[string]string),
	}
	for _, r := range replaces {
		if r.Old.Version == "" {
			idx.wildcard[r.Old.Path] = r.New
			continue
		}
		idx.exact[r.Old] = r.New
		if _, ok := idx.oldVersion[r.Old.Path]; !ok {
			idx.oldVersion[r.Old.Path] = r.Old.Version
		}
	}
	return idx
}

// lookup returns the replacement of mod, or nil if it is not replaced.
func (idx replaceIndex) lookup(mod ModVersion) *ModVersion {
	if r, ok := idx.exact[mod]; ok {
		return &r
	}
	if r, ok := idx.wildcard[mod.Path]; ok {
		return &r
	}
	return nil
}

// goDirective returns the go directive of the go.mod in dir, or "" if there is none.
func goDirective(dir string) string {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return ""
	}
	mod, err := parseGoMod(goModPath)
	if err != nil {
		// best effort: an unparsable dependency go.mod compiles at the default language version.
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	return mod.Go
}

// packages lists every directory with non-test Go files of the module modPath at dir, not only
// the imported ones. Directories owned by another module (a nested go.mod, or a path in
// inBuildList) are skipped.
func packages(modPath string, dir string, inBuildList map[string]bool) []string {
	root, err := filepath.EvalSymlinks(dir)
	if os.IsNotExist(err) {
		// best effort: a local replacement is absent where only go.mod is copied, as in `mkGoCacheEnv`.
		return nil
	}
	if err != nil {
		panic(err)
	}

	importPath := func(path string) string {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			panic(err)
		}
		if rel == "." {
			return modPath
		}
		return modPath + "/" + filepath.ToSlash(rel)
	}

	found := make(map[string]bool)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") ||
				strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
			if inBuildList[importPath(path)] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			found[importPath(filepath.Dir(path))] = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return slices.Sorted(maps.Keys(found))
}

// unusedReplacements returns the replaces of mainMod whose Old matches no module in modules.
func unusedReplacements(mainMod GoMod, modules []vendoredModule) []Replace {
	seen := make(map[ModVersion]bool, len(modules))
	for _, m := range modules {
		seen[m.ModVersion] = true
	}
	var unused []Replace
	for _, r := range mainMod.Replace {
		if seen[r.Old] {
			continue
		}
		seen[r.Old] = true
		unused = append(unused, r)
	}
	return unused
}

// Output

// render returns vendor/modules.txt as `go mod vendor` writes it for a main module at goVersion.
func render(goVersion string, modules []vendoredModule, unused []Replace) string {
	// Annotations are gated on the main module's go version like `go mod vendor` does.
	annotateExplicit := version.Compare("go"+goVersion, "go1.14") >= 0
	annotateGo := version.Compare("go"+goVersion, "go1.17") >= 0

	var b strings.Builder
	for _, m := range modules {
		b.WriteString(moduleLine(m.ModVersion, m.Replacement))
		b.WriteString(annotation(m, annotateExplicit, annotateGo))
		for _, pkg := range m.Packages {
			b.WriteString(pkg)
			b.WriteByte('\n')
		}
	}
	if annotateExplicit {
		for _, r := range unused {
			b.WriteString(moduleLine(r.Old, &r.New))
		}
	}
	return b.String()
}

func moduleLine(mod ModVersion, repl *ModVersion) string {
	line := "# " + mod.Path
	if mod.Version != "" {
		line += " " + mod.Version
	}
	if repl != nil {
		line += " => " + repl.Path
		if repl.Version != "" {
			line += " " + repl.Version
		}
	}
	return line + "\n"
}

func annotation(m vendoredModule, explicit, goVersion bool) string {
	var notes []string
	if explicit && m.Explicit {
		notes = append(notes, "explicit")
	}
	if goVersion && m.GoVersion != "" {
		notes = append(notes, "go "+m.GoVersion)
	}
	if len(notes) == 0 {
		return ""
	}
	return "## " + strings.Join(notes, "; ") + "\n"
}

func writeModulesTxt(content string) {
	if content == "" {
		return
	}
	if err := os.WriteFile(
		filepath.Join("vendor", "modules.txt"),
		[]byte(content),
		0o644,
	); err != nil {
		panic(err)
	}
}
