package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"path/filepath"
)

type packageJob struct {
	importPath    string
	goPackagePath string
	rev           string
}

type packageResult struct {
	pkg *Package
	err error
}

func worker(id int, replace map[string]string, jobs <-chan *packageJob, results chan<- *packageResult) {
	for j := range jobs {
		pkg, err := fetchPackage(j.importPath, j.goPackagePath, j.rev)
		results <- &packageResult{
			err: err,
			pkg: pkg,
		}
	}
}

func main() {

	flag.Parse()

	numWorkers := 20
	keepGoing := true
	directory := "./"

	pkgs, err := FetchPackages(filepath.Join(directory, "go.mod"), filepath.Join(directory, "go.sum"), numWorkers, keepGoing)
	if err != nil {
		panic(err)
	}

	for _, pkg := range pkgs {
		fmt.Println(pkg)
	}

}
