package generate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectWorkspace_NoGoWork(t *testing.T) {
	dir := t.TempDir()

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != nil {
		t.Fatal("expected nil workspace when no go.work exists")
	}
}

func TestDetectWorkspace_WithGoWork(t *testing.T) {
	dir := t.TempDir()

	// Create module directories with go.mod files
	for _, mod := range []string{"app", "lib"} {
		modDir := filepath.Join(dir, mod)
		if err := os.MkdirAll(modDir, 0755); err != nil {
			t.Fatal(err)
		}
		goMod := filepath.Join(modDir, "go.mod")
		content := "module example.com/" + mod + "\n\ngo 1.21\n"
		if err := os.WriteFile(goMod, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create go.work
	goWork := filepath.Join(dir, "go.work")
	content := "go 1.21\n\nuse (\n\t./app\n\t./lib\n)\n"
	if err := os.WriteFile(goWork, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws == nil {
		t.Fatal("expected non-nil workspace")
	}

	if ws.GoVersion != "1.21" {
		t.Errorf("expected Go version 1.21, got %s", ws.GoVersion)
	}

	if len(ws.ModuleDirs) != 2 {
		t.Fatalf("expected 2 module dirs, got %d", len(ws.ModuleDirs))
	}

	// Verify module directories are absolute paths
	for _, d := range ws.ModuleDirs {
		if !filepath.IsAbs(d) {
			t.Errorf("expected absolute path, got %s", d)
		}
	}
}

func TestDetectWorkspace_SkipsMissingGoMod(t *testing.T) {
	dir := t.TempDir()

	// Create only one module directory with go.mod
	modDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module example.com/app\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create directory without go.mod
	if err := os.MkdirAll(filepath.Join(dir, "missing"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create go.work referencing both
	goWork := filepath.Join(dir, "go.work")
	content := "go 1.21\n\nuse (\n\t./app\n\t./missing\n)\n"
	if err := os.WriteFile(goWork, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ws, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws == nil {
		t.Fatal("expected non-nil workspace")
	}

	// Only the app module should be included
	if len(ws.ModuleDirs) != 1 {
		t.Fatalf("expected 1 module dir (missing should be skipped), got %d", len(ws.ModuleDirs))
	}
}
