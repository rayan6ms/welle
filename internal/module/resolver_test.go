package module

import (
	"os"
	"path/filepath"
	"testing"

	"welle/internal/config"
)

func TestResolverUsesManifestPaths(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	stdRoot := filepath.Join(projectRoot, "custom_std")
	if err := os.MkdirAll(stdRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stdRoot, "math.wll"), []byte("export PI = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	modRoot := filepath.Join(projectRoot, "modules")
	if err := os.MkdirAll(modRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modRoot, "util.wll"), []byte("export answer = 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(projectRoot, "welle.toml")
	manifest := "entry = \"main.wll\"\nstd_root = \"custom_std\"\nmodule_paths = [\"modules\"]\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	man, err := config.LoadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	defaultStd := filepath.Join(projectRoot, "std")
	stdPath, modulePaths, err := man.ResolvePaths(projectRoot, defaultStd)
	if err != nil {
		t.Fatal(err)
	}

	res := NewResolver(stdPath, modulePaths)
	fromFile := filepath.Join(projectRoot, "main.wll")

	stdResolved, err := res.Resolve(fromFile, "std:math")
	if err != nil {
		t.Fatal(err)
	}
	if stdResolved != filepath.Join(stdRoot, "math.wll") {
		t.Fatalf("unexpected std resolve: %s", stdResolved)
	}

	utilResolved, err := res.Resolve(fromFile, "util")
	if err != nil {
		t.Fatal(err)
	}
	if utilResolved != filepath.Join(modRoot, "util.wll") {
		t.Fatalf("unexpected module path resolve: %s", utilResolved)
	}
}
