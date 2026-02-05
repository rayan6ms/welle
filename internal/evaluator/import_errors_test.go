package evaluator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"welle/internal/module"
	"welle/internal/object"
)

func TestFromImportMissingExport(t *testing.T) {
	tmp := t.TempDir()
	modPath := filepath.Join(tmp, "mod.wll")
	if err := os.WriteFile(modPath, []byte("export foo = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(tmp, "main.wll")
	src := "from \"./mod.wll\" import bar\n"
	if err := os.WriteFile(entryPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	res := module.NewResolver(tmp, []string{tmp})
	r := NewRunner()
	r.SetResolver(res)
	r.EnableImports()
	out := r.RunFile(entryPath)
	if out == nil || out.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error, got %v", out)
	}
	msg := out.Inspect()
	if !strings.Contains(msg, "missing export") || !strings.Contains(msg, "bar") || !strings.Contains(msg, "./mod.wll") {
		t.Fatalf("unexpected error message: %s", msg)
	}
}

func TestImportCycleDetection(t *testing.T) {
	root := filepath.Join("..", "module", "testdata")
	cycleA := filepath.Join(root, "cycle_a.wll")

	res := module.NewResolver(root, []string{root})
	r := NewRunner()
	r.SetResolver(res)
	r.EnableImports()
	out := r.RunFile(cycleA)
	if out == nil || out.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error, got %v", out)
	}
	msg := out.Inspect()
	if !strings.Contains(msg, "WM0001") || !strings.Contains(msg, "cycle_a.wll") || !strings.Contains(msg, "cycle_b.wll") {
		t.Fatalf("unexpected cycle error message: %s", msg)
	}
}
