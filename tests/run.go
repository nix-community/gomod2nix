package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
)

func runProcess(prefix string, env []string, command string, args ...string) error {
	fmt.Printf("%s: Executing %s %v\n", prefix, command, args)
	cmd := exec.Command(command, args...)
	if env != nil {
		cmd.Env = env
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Printf("%s: %s\n", prefix, scanner.Text())
		}
		wg.Done()
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("%s: %s\n", prefix, scanner.Text())
		}
		wg.Done()
	}()
	wg.Wait()
	return cmd.Wait()
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}

func runTest(rootDir string, testDir string) error {
	prefix := testDir
	testDir = filepath.Join(rootDir, testDir)
	cmdPath := filepath.Join(rootDir, "..", "gomod2nix")

	if _, err := os.Stat(filepath.Join(testDir, "script")); err == nil {
		env := append(os.Environ(), "GOMOD2NIX="+cmdPath)
		err := runProcess(prefix, env, filepath.Join(testDir, "script"))
		if err != nil {
			return err
		}

	} else {
		if _, err := os.Stat(filepath.Join(testDir, "go.mod")); err == nil {
			err := runProcess(prefix, nil, cmdPath, "--dir", testDir, "--outdir", testDir)
			if err != nil {
				return err
			}
		}
	}

	buildExpr := fmt.Sprintf("with (import <nixpkgs> { overlays = [ (import %s/../overlay.nix) ]; }); callPackage %s {}", rootDir, testDir)
	err := runProcess(prefix, nil, "nix-build", "-v", "--no-out-link", "--expr", buildExpr)
	if err != nil {
		return err
	}

	return nil
}

func getTestDirs(rootDir string) ([]string, error) {
	// Takes too long for Github Actions
	var blacklist []string
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		blacklist = []string{
			"helm",
			"minikube",
			"cross",
		}
	}

	files, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	testDirs := []string{}
	for _, f := range files {
		if f.IsDir() && !contains(blacklist, f.Name()) {
			testDirs = append(testDirs, f.Name())
		}
	}

	return testDirs, nil
}

func runTests(rootDir string, testDirs []string) error {
	var wg sync.WaitGroup
	cmdErrChan := make(chan error)
	for _, testDir := range testDirs {
		fmt.Printf("Running test for: '%s'\n", testDir)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runTest(rootDir, testDir)
			if err != nil {
				cmdErrChan <- fmt.Errorf("test for '%s' failed: %w", testDir, err)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(cmdErrChan)
	}()

	for cmdErr := range cmdErrChan {
		return cmdErr
	}

	return nil
}

func main() {
	var rootDir string
	{
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			panic("Couldn't get test directory")
		}
		rootDir = path.Dir(file)
	}

	flag.Parse()

	nArgs := flag.NArg()

	action := "run"
	if nArgs >= 1 {
		action = flag.Arg(0)
	}

	switch action {
	case "list":
		testDirs, err := getTestDirs(rootDir)
		if err != nil {
			panic(err)
		}

		for _, testDir := range testDirs {
			fmt.Println(testDir)
		}

		return

	case "run":
		var testDirs []string
		var err error
		if nArgs > 1 {
			args := flag.Args()
			testDirs = args[1:nArgs]
		} else {
			testDirs, err = getTestDirs(rootDir)
			if err != nil {
				panic(err)
			}
		}

		err = runTests(rootDir, testDirs)
		if err != nil {
			panic(err)
		}

		return

	default:
		panic(fmt.Errorf("unhandled action: %s", flag.Arg(0)))
	}
}
