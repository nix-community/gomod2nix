package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	got := "Hello flake"
	if got != "Hello flake" {
		t.Errorf("main: %s; want Hello flake", got)
	}
}
