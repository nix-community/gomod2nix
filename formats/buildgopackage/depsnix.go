package buildgopackage

import (
	"fmt"
	"github.com/orivej/go-nix/nix/eval"
	"github.com/orivej/go-nix/nix/parser"
	"github.com/tweag/gomod2nix/types"
	"log"
	"os"
	"strings"
)

const depNixFormat = `  {
    goPackagePath = "%s";
    fetch = {
      type = "%s";
      url = "%s";
      rev = "%s";
      sha256 = "%s";
    };
  }`

func Marshal(pkgs []*types.Package) ([]byte, error) {
	var result []string

	result = append(result, "[")
	for _, pkg := range pkgs {
		result = append(result,
			fmt.Sprintf(depNixFormat,
				pkg.GoPackagePath, "git", pkg.URL,
				pkg.Rev, pkg.Sha256))
	}
	result = append(result, "]")

	return []byte(strings.Join(result, "\n")), nil
}

// Load the contents of deps.nix into a struct
// This is mean to achieve re-use of previous invocations using the deps.nix (buildGoPackage) output format
func LoadDepsNix(filePath string) map[string]*types.Package {
	ret := make(map[string]*types.Package)

	stat, err := os.Stat(filePath)
	if err != nil {
		return ret
	}
	if stat.Size() == 0 {
		return ret
	}

	p, err := parser.ParseFile(filePath)
	if err != nil {
		log.Println("Failed reading deps.nix")
		return ret
	}

	evalResult := eval.ParseResult(p)
	for _, pkgAttrsExpr := range evalResult.(eval.List) {
		pkgAttrs, ok := pkgAttrsExpr.Eval().(eval.Set)
		if !ok {
			continue
		}
		fetch, ok := pkgAttrs[eval.Intern("fetch")].Eval().(eval.Set)
		if !ok {
			continue
		}

		goPackagePath, ok := pkgAttrs[eval.Intern("goPackagePath")].Eval().(string)
		if !ok {
			continue
		}

		url, ok := fetch[eval.Intern("url")].Eval().(string)
		if !ok {
			continue
		}
		rev, ok := fetch[eval.Intern("rev")].Eval().(string)
		if !ok {
			continue
		}
		sha256, ok := fetch[eval.Intern("sha256")].Eval().(string)
		if !ok {
			continue
		}

		ret[goPackagePath] = &types.Package{
			GoPackagePath: goPackagePath,
			URL:           url,
			Rev:           rev,
			Sha256:        sha256,
		}
	}

	return ret
}
