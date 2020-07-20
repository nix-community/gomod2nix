package main // import "github.com/tweag/gomod2nix"

import (
	"flag"
	"fmt"
	"golang.org/x/mod/modfile"
	"io/ioutil"
	"path/filepath"
)

type packageJob struct {
	goPackagePath string
	rev           string
}

type packageResult struct {
	pkg *Package
	err error
}

func worker(id int, replace map[string]string, jobs <-chan *packageJob, results chan<- *packageResult) {
	for j := range jobs {
		pkg, err := fetchPackage(replace, j.goPackagePath, j.rev)
		results <- &packageResult{
			err: err,
			pkg: pkg,
		}
	}
}

func main() {

	// var jobs = flag.Int("jobs", 20, "Number of parallel jobs")
	flag.Parse()

	numWorkers := 10

	// directory := "testdata"
	directory := "./"

	// Read go.mod
	data, err := ioutil.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		panic(err)
	}

	// Parse go.mod
	mod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	// // Parse require
	// require := make(map[string]module.Version)
	// for _, req := range mod.Require {
	// 	require[req.Mod.Path] = req.Mod
	// }

	// Map repos -> replacement repo
	replace := make(map[string]string)
	for _, repl := range mod.Replace {
		replace[repl.Old.Path] = repl.New.Path
	}

	revs, err := parseGoSum(filepath.Join(directory, "go.sum"))
	if err != nil {
		panic(err)
	}

	numJobs := len(revs)
	if numJobs < numWorkers {
		numWorkers = numJobs
	}

	jobs := make(chan *packageJob, numJobs)
	results := make(chan *packageResult, numJobs)
	for i := 0; i <= numWorkers; i++ {
		go worker(i, replace, jobs, results)
	}

	for goPackagePath, rev := range revs {
		jobs <- &packageJob{
			goPackagePath: goPackagePath,
			rev:           rev,
		}
	}
	close(jobs)

	for i := 1; i <= numJobs; i++ {
		result := <-results
		fmt.Println(result)
	}

}
