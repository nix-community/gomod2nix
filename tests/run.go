package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var cwd = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}()

type testError struct {
	testDir string
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func runProcess(prefix string, command string, args ...string) error {
	fmt.Println(fmt.Sprintf("%s: Executing %s %s", prefix, command, args))

	cmd := exec.Command(command, args...)

	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	done := make(chan struct{})

	go func() {
		reader := io.MultiReader(stdoutReader, stderrReader)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Bytes()
			fmt.Println(fmt.Sprintf("%s: %s", prefix, line))
		}
		done <- struct{}{}
	}()

	err = cmd.Start()
	if err != nil {
		return err
	}

	<-done

	return cmd.Wait()
}

func runTest(testDir string) error {
	rootDir := filepath.Join(cwd, "..")

	cmdPath := filepath.Join(rootDir, "gomod2nix")
	err := runProcess(testDir, cmdPath, "--dir", testDir, "--outdir", testDir)
	if err != nil {
		return err
	}

	buildExpr := fmt.Sprintf("with (import <nixpkgs> { overlays = [ (import %s/overlay.nix) ]; }); callPackage ./%s {}", rootDir, testDir)
	err = runProcess(testDir, "nix-build", "--no-out-link", "--expr", buildExpr)
	if err != nil {
		return err
	}

	return nil
}

func main() {

	files, err := os.ReadDir(".")
	if err != nil {
		panic(err)
	}

	testDirs := []string{}
	for _, f := range files {
		if f.IsDir() {
			testDirs = append(testDirs, f.Name())
		}
	}

	var wg sync.WaitGroup
	cmdErrChan := make(chan error)
	for _, testDir := range testDirs {
		testDir := testDir
		fmt.Println(fmt.Sprintf("Running test for: '%s'", testDir))
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runTest(testDir)
			if err != nil {
				cmdErrChan <- err
			}
		}()
	}

	go func() {
		wg.Wait()
		close(cmdErrChan)
	}()

	for cmdErr := range cmdErrChan {
		fmt.Println(fmt.Sprintf("Test for '%s' failed:", cmdErr))
		os.Exit(1)
	}

}
