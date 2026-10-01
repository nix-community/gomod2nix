package main

import (
	"bytes"
	"testing"
)

func TestHello(t *testing.T) {
	var out bytes.Buffer
	cmd := newCmd()
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello\n" {
		t.Fatalf("unexpected output %q", out.String())
	}
}
