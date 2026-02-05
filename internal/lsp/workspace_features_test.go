package lsp

import (
	"os"
	"path/filepath"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestRenameExportWorkspace(t *testing.T) {
	root := t.TempDir()
	paths := writeWorkspaceFiles(t, root, map[string]string{
		"mod.wll":       "export func greet() { return 1 }\n",
		"use_from.wll":  "from \"./mod\" import greet\nx = greet()\n",
		"use_alias.wll": "import \"./mod\" as m\nx = m.greet()\n",
	})
	ws := NewWorkspace(root)
	updateWorkspaceDocs(t, ws, paths)

	clean, pos := extractPos(t, "export func |greet() { return 1 }\n")
	edit, err := RenameAt(ws, PathToURI(paths["mod.wll"]), clean, pos, "hello")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}

	modURI := protocol.DocumentUri(PathToURI(paths["mod.wll"]))
	fromURI := protocol.DocumentUri(PathToURI(paths["use_from.wll"]))
	aliasURI := protocol.DocumentUri(PathToURI(paths["use_alias.wll"]))

	if len(edit.Changes) != 3 {
		t.Fatalf("expected edits in 3 files, got %d", len(edit.Changes))
	}
	if len(edit.Changes[modURI]) != 1 {
		t.Fatalf("expected 1 edit in mod file, got %d", len(edit.Changes[modURI]))
	}
	if len(edit.Changes[fromURI]) != 2 {
		t.Fatalf("expected 2 edits in from-import file, got %d", len(edit.Changes[fromURI]))
	}
	if len(edit.Changes[aliasURI]) != 1 {
		t.Fatalf("expected 1 edit in alias-member file, got %d", len(edit.Changes[aliasURI]))
	}
}

func TestReferencesExportWorkspace(t *testing.T) {
	root := t.TempDir()
	paths := writeWorkspaceFiles(t, root, map[string]string{
		"mod.wll":       "export func greet() { return 1 }\n",
		"use_from.wll":  "from \"./mod\" import greet\nx = greet()\n",
		"use_alias.wll": "import \"./mod\" as m\nx = m.greet()\n",
	})
	ws := NewWorkspace(root)
	updateWorkspaceDocs(t, ws, paths)

	clean, pos := extractPos(t, "export func |greet() { return 1 }\n")
	locs, err := ReferencesAt(ws, PathToURI(paths["mod.wll"]), clean, pos, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locs) != 4 {
		t.Fatalf("expected 4 references across workspace, got %d", len(locs))
	}
	seen := map[string]bool{}
	for _, loc := range locs {
		seen[string(loc.URI)] = true
	}
	if !seen[PathToURI(paths["mod.wll"])] || !seen[PathToURI(paths["use_from.wll"])] || !seen[PathToURI(paths["use_alias.wll"])] {
		t.Fatalf("expected references in all workspace files, got %v", seen)
	}
}

func TestRenameLocalStaysInFile(t *testing.T) {
	root := t.TempDir()
	paths := writeWorkspaceFiles(t, root, map[string]string{
		"main.wll":  "func f() { x = 1; x = x + 1 }\n",
		"other.wll": "x = 2\n",
	})
	ws := NewWorkspace(root)
	updateWorkspaceDocs(t, ws, paths)

	clean, pos := extractPos(t, "func f() { |x = 1; x = x + 1 }\n")
	edit, err := RenameAt(ws, PathToURI(paths["main.wll"]), clean, pos, "y")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}
	if len(edit.Changes) != 1 {
		t.Fatalf("expected edits in 1 file, got %d", len(edit.Changes))
	}
	if _, ok := edit.Changes[protocol.DocumentUri(PathToURI(paths["other.wll"]))]; ok {
		t.Fatalf("unexpected edits in other file")
	}
}

func TestRenameUnicodeUTF16Workspace(t *testing.T) {
	root := t.TempDir()
	paths := writeWorkspaceFiles(t, root, map[string]string{
		"mod.wll":      "export func 😀() { return 1 }\n",
		"use_from.wll": "from \"./mod\" import 😀\n😀()\n",
	})
	ws := NewWorkspace(root)
	updateWorkspaceDocs(t, ws, paths)

	clean, pos := extractPos(t, "export func |😀() { return 1 }\n")
	edit, err := RenameAt(ws, PathToURI(paths["mod.wll"]), clean, pos, "smile")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}

	for _, edits := range edit.Changes {
		for _, e := range edits {
			if int(e.Range.End.Character-e.Range.Start.Character) != 2 {
				t.Fatalf("expected UTF-16 length 2 for emoji, got %d", e.Range.End.Character-e.Range.Start.Character)
			}
		}
	}
}

func TestRenameExportModuleIdentity(t *testing.T) {
	root := t.TempDir()
	paths := writeWorkspaceFiles(t, root, map[string]string{
		"a.wll":    "export func dup() { return 1 }\n",
		"b.wll":    "export func dup() { return 2 }\n",
		"main.wll": "from \"./a\" import dup\nfrom \"./b\" import dup as dupB\nx = dup()\ny = dupB()\n",
	})
	ws := NewWorkspace(root)
	updateWorkspaceDocs(t, ws, paths)

	clean, pos := extractPos(t, "export func |dup() { return 1 }\n")
	edit, err := RenameAt(ws, PathToURI(paths["a.wll"]), clean, pos, "dupA")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}
	if _, ok := edit.Changes[protocol.DocumentUri(PathToURI(paths["b.wll"]))]; ok {
		t.Fatalf("unexpected edits in module b")
	}
	mainEdits := edit.Changes[protocol.DocumentUri(PathToURI(paths["main.wll"]))]
	if len(mainEdits) != 2 {
		t.Fatalf("expected 2 edits in main file, got %d", len(mainEdits))
	}
}

func writeWorkspaceFiles(t *testing.T, root string, files map[string]string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for rel, contents := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(contents), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		out[rel] = abs
	}
	return out
}

func updateWorkspaceDocs(t *testing.T, ws *Workspace, paths map[string]string) {
	t.Helper()
	for _, abs := range paths {
		b, err := os.ReadFile(abs)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if _, err := ws.UpdateOpenDoc(PathToURI(abs), string(b)); err != nil {
			t.Fatalf("update open doc: %v", err)
		}
	}
}
