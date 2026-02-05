package module

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestResolveMissingModuleError(t *testing.T) {
	tmp := t.TempDir()
	res := NewResolver(tmp, nil)
	fromFile := filepath.Join(tmp, "main.wll")
	_, err := res.Resolve(fromFile, "missing_mod")
	if err == nil {
		t.Fatal("expected error")
	}
	var re *ResolveError
	if !errors.As(err, &re) {
		t.Fatalf("expected ResolveError, got %T", err)
	}
	if !strings.Contains(err.Error(), "missing module") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "missing_mod") {
		t.Fatalf("expected spec in error, got: %s", err.Error())
	}
}

func TestDuplicateExportError(t *testing.T) {
	tmp := t.TempDir()
	modPath := filepath.Join(tmp, "dup.wll")
	src := "export x = 1\nexport x = 2\n"
	if err := os.WriteFile(modPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res := NewResolver(tmp, nil)
	loader := NewLoader(res)
	_, _, err := loader.LoadBytecode(modPath, modPath, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate export") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "dup.wll:1") || !strings.Contains(err.Error(), "dup.wll:2") {
		t.Fatalf("expected locations in error, got: %s", err.Error())
	}
}
