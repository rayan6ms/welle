package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaxMemFromManifest(t *testing.T) {
	root := repoRoot(t)
	project := t.TempDir()

	manifest := strings.Join([]string{
		`entry = "main.wll"`,
		`std_root = ` + quote(filepath.Join(root, "std")),
		`max_mem = 10`,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(project, "welle.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "main.wll"), []byte(`print("ok")`+"\n"), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	out, err := runWelle(root, "run", project)
	if err == nil {
		t.Fatalf("expected error, got output: %s", out)
	}
	if !strings.Contains(out, "max memory exceeded (10 bytes)") {
		t.Fatalf("expected memory error, got: %s", out)
	}
}

func TestMaxMemCLIOverridesManifest(t *testing.T) {
	root := repoRoot(t)
	project := t.TempDir()

	manifest := strings.Join([]string{
		`entry = "main.wll"`,
		`std_root = ` + quote(filepath.Join(root, "std")),
		`max_mem = 10`,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(project, "welle.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "main.wll"), []byte(`print("ok")`+"\n"), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	out, err := runWelle(root, "-max-mem", "1000", "run", project)
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected ok output, got: %s", out)
	}
}

func runWelle(root string, args ...string) (string, error) {
	allArgs := append([]string{"run", "./cmd/welle"}, args...)
	cmd := exec.Command("go", allArgs...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
