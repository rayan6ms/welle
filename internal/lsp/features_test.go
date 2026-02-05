package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCompletionOrdering(t *testing.T) {
	ws := testWorkspace(t)
	text := `import "std:math" as math

func f(a) {
  local = 1
  |
}
`
	clean, pos := extractPos(t, text)
	items := CompletionItems(ws, "file:///test.wll", clean, pos)
	idxLocal := indexOfCompletion(items, "local")
	idxParam := indexOfCompletion(items, "a")
	idxImport := indexOfCompletion(items, "math")
	idxBuiltin := indexOfCompletion(items, "print")
	if idxLocal == -1 || idxParam == -1 || idxImport == -1 || idxBuiltin == -1 {
		t.Fatalf("missing expected completions: local=%d param=%d import=%d builtin=%d", idxLocal, idxParam, idxImport, idxBuiltin)
	}
	if !(idxLocal < idxParam && idxParam < idxImport && idxImport < idxBuiltin) {
		t.Fatalf("unexpected completion ordering: local=%d param=%d import=%d builtin=%d", idxLocal, idxParam, idxImport, idxBuiltin)
	}
}

func TestCompletionModuleMembers(t *testing.T) {
	ws := testWorkspace(t)
	text := `import "std:math" as math

math.|
`
	clean, pos := extractPos(t, text)
	items := CompletionItems(ws, "file:///test.wll", clean, pos)
	if indexOfCompletion(items, "add") == -1 {
		t.Fatalf("expected module completion to include add")
	}
	if indexOfCompletion(items, "sub") == -1 {
		t.Fatalf("expected module completion to include sub")
	}
}

func TestCompletionUnicodeIdentifier(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f() {
  π = 1
  |
}
`
	clean, pos := extractPos(t, text)
	items := CompletionItems(ws, "file:///test.wll", clean, pos)
	if indexOfCompletion(items, "π") == -1 {
		t.Fatalf("expected unicode identifier to appear in completion list")
	}
}

func TestHoverVarFuncBuiltin(t *testing.T) {
	ws := testWorkspace(t)
	text := `func add(a, b) { return a + b }

x = 1
print(|x)
print(|add(1, 2))
print(|len([1,2,3]))
`
	clean, posVar := extractPos(t, text)
	hover, err := HoverAt(ws, "file:///test.wll", clean, posVar)
	if err != nil || hover == nil {
		t.Fatalf("expected hover for var, err=%v", err)
	}
	content := hoverContents(hover)
	if !strings.Contains(content, "var") {
		t.Fatalf("expected hover content to include var, got %q", content)
	}

	clean2, posFunc := extractPos(t, clean)
	hover, err = HoverAt(ws, "file:///test.wll", clean2, posFunc)
	if err != nil || hover == nil {
		t.Fatalf("expected hover for func, err=%v", err)
	}
	content = hoverContents(hover)
	if !strings.Contains(content, "add(a, b)") {
		t.Fatalf("expected hover content to include signature, got %q", content)
	}

	clean3, posBuiltin := extractPos(t, clean2)
	hover, err = HoverAt(ws, "file:///test.wll", clean3, posBuiltin)
	if err != nil || hover == nil {
		t.Fatalf("expected hover for builtin, err=%v", err)
	}
	content = hoverContents(hover)
	if !strings.Contains(content, "builtin") || !strings.Contains(content, "len(x) -> int") {
		t.Fatalf("expected hover content to include builtin signature, got %q", content)
	}
}

func TestRenameLocalNestedBlocks(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f() {
  x = 1
  if (true) { x = x + 1 }
  func g() { x = 2 }
  return x
}
`
	pos := protocol.Position{Line: 1, Character: 2} // line 2 col 3? but utf16 char index in "  x"
	edit, err := RenameAt(ws, "file:///test.wll", text, pos, "y")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}
	edits := edit.Changes[protocol.DocumentUri("file:///test.wll")]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits for outer x, got %d", len(edits))
	}
}

func TestRenameParam(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f(a) {
  return a + 1
}
`
	pos := protocol.Position{Line: 0, Character: 7}
	edit, err := RenameAt(ws, "file:///test.wll", text, pos, "b")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}
	edits := edit.Changes[protocol.DocumentUri("file:///test.wll")]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits for param, got %d", len(edits))
	}
}

func TestRenameUnicodeRange(t *testing.T) {
	ws := testWorkspace(t)
	text := "π = 1\nprint(π)\n"
	pos := protocol.Position{Line: 0, Character: 0}
	edit, err := RenameAt(ws, "file:///test.wll", text, pos, "tau")
	if err != nil || edit == nil {
		t.Fatalf("expected rename edit, err=%v", err)
	}
	edits := edit.Changes[protocol.DocumentUri("file:///test.wll")]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d", len(edits))
	}
	if edits[0].Range.Start.Character != 0 || edits[0].Range.End.Character != 1 {
		t.Fatalf("expected utf16 range length 1, got %d..%d", edits[0].Range.Start.Character, edits[0].Range.End.Character)
	}
}

func TestReferencesLocal(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f() {
  x = 1
  x = x + 1
  return x
}
`
	pos := protocol.Position{Line: 1, Character: 2}
	locs, err := ReferencesAt(ws, "file:///test.wll", text, pos, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locs) != 4 {
		t.Fatalf("expected 4 references including decl, got %d", len(locs))
	}
}

func TestReferencesWalrusLocal(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f() {
  x := 1
  return x
}
`
	pos := protocol.Position{Line: 1, Character: 2}
	locs, err := ReferencesAt(ws, "file:///test.wll", text, pos, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("expected 2 references including declaration, got %d", len(locs))
	}
}

func TestSignatureHelpActiveParam(t *testing.T) {
	ws := testWorkspace(t)
	text := `func f(a, b, c) { return a }
func g(x, y) { return x }

f(1, g(2, 3), 4)
`
	posA := protocol.Position{Line: 3, Character: 2}
	help, err := SignatureHelpAt(ws, "file:///test.wll", text, posA)
	if err != nil || help == nil {
		t.Fatalf("expected signature help at param 0, err=%v", err)
	}
	if help.ActiveParameter == nil || *help.ActiveParameter != 0 {
		t.Fatalf("expected active parameter 0, got %v", help.ActiveParameter)
	}

	posB := protocol.Position{Line: 3, Character: 10}
	help, err = SignatureHelpAt(ws, "file:///test.wll", text, posB)
	if err != nil || help == nil {
		t.Fatalf("expected signature help at param 1, err=%v", err)
	}
	if help.ActiveParameter == nil || *help.ActiveParameter != 1 {
		t.Fatalf("expected active parameter 1, got %v", help.ActiveParameter)
	}

	posC := protocol.Position{Line: 3, Character: 14}
	help, err = SignatureHelpAt(ws, "file:///test.wll", text, posC)
	if err != nil || help == nil {
		t.Fatalf("expected signature help at param 2, err=%v", err)
	}
	if help.ActiveParameter == nil || *help.ActiveParameter != 2 {
		t.Fatalf("expected active parameter 2, got %v", help.ActiveParameter)
	}
}

func TestSignatureHelpStringMethod(t *testing.T) {
	ws := testWorkspace(t)
	text, pos := extractPos(t, `s = "hello"
s.slice(1, |2)
`)
	help, err := SignatureHelpAt(ws, "file:///test.wll", text, pos)
	if err != nil || help == nil {
		t.Fatalf("expected signature help for string method, err=%v", err)
	}
	if len(help.Signatures) == 0 || !strings.Contains(help.Signatures[0].Label, "slice(") {
		t.Fatalf("expected slice signature, got %v", help.Signatures)
	}
	if help.ActiveParameter == nil || *help.ActiveParameter != 1 {
		t.Fatalf("expected active parameter 1, got %v", help.ActiveParameter)
	}
}

func extractPos(t *testing.T, text string) (string, protocol.Position) {
	idx := strings.Index(text, "|")
	if idx == -1 {
		t.Fatalf("missing cursor marker")
	}
	before := text[:idx]
	after := text[idx+1:]
	clean := before + after
	line := uint32(0)
	col := uint32(0)
	for _, r := range before {
		if r == '\n' {
			line++
			col = 0
			continue
		}
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		col += uint32(n)
	}
	return clean, protocol.Position{Line: line, Character: col}
}

func indexOfCompletion(items []protocol.CompletionItem, label string) int {
	for i, it := range items {
		if it.Label == label {
			return i
		}
	}
	return -1
}

func hoverContents(h *protocol.Hover) string {
	if h == nil {
		return ""
	}
	switch v := h.Contents.(type) {
	case protocol.MarkupContent:
		return v.Value
	default:
		return ""
	}
}

func testWorkspace(t *testing.T) *Workspace {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Dir(filepath.Dir(wd))
	return NewWorkspace(root)
}
