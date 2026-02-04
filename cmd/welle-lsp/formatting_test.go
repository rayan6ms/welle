package main

import (
	"testing"

	"welle/internal/lsp"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestTextDocumentFormatting_NoEditsWhenFormatted(t *testing.T) {
	store = lsp.NewStore()
	uri := "file:///test.wll"
	text := "x = 1\n"
	store.Set(uri, text)

	params := formattingParams(uri, true, 2)
	edits, err := textDocumentFormatting(nil, &params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edits) != 0 {
		t.Fatalf("expected no edits, got %d", len(edits))
	}
}

func TestTextDocumentFormatting_FullDocumentEdit(t *testing.T) {
	store = lsp.NewStore()
	uri := "file:///test.wll"
	text := "x=1"
	store.Set(uri, text)

	params := formattingParams(uri, true, 2)
	edits, err := textDocumentFormatting(nil, &params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	edit := edits[0]
	if edit.NewText != "x = 1\n" {
		t.Fatalf("unexpected formatted text: %q", edit.NewText)
	}
	if edit.Range.Start.Line != 0 || edit.Range.Start.Character != 0 {
		t.Fatalf("expected range start 0:0, got %d:%d", edit.Range.Start.Line, edit.Range.Start.Character)
	}
	if edit.Range.End.Line != 0 || edit.Range.End.Character != 3 {
		t.Fatalf("expected range end 0:3, got %d:%d", edit.Range.End.Line, edit.Range.End.Character)
	}
}

func TestTextDocumentFormatting_RangeEndPositions(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		expected protocol.Position
	}{
		{
			name:     "trailing_newline",
			text:     "x=1\n",
			expected: protocol.Position{Line: 1, Character: 0},
		},
		{
			name:     "no_trailing_newline",
			text:     "x=1",
			expected: protocol.Position{Line: 0, Character: 3},
		},
		{
			name:     "unicode",
			text:     "x=😀",
			expected: protocol.Position{Line: 0, Character: 4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store = lsp.NewStore()
			uri := "file:///range.wll"
			store.Set(uri, tc.text)

			params := formattingParams(uri, true, 2)
			edits, err := textDocumentFormatting(nil, &params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(edits) != 1 {
				t.Fatalf("expected 1 edit, got %d", len(edits))
			}
			end := edits[0].Range.End
			if end.Line != tc.expected.Line || end.Character != tc.expected.Character {
				t.Fatalf("expected range end %d:%d, got %d:%d", tc.expected.Line, tc.expected.Character, end.Line, end.Character)
			}
		})
	}
}

func TestFormatIndentFromOptions(t *testing.T) {
	opts := protocol.FormattingOptions{
		protocol.FormattingOptionInsertSpaces: true,
		protocol.FormattingOptionTabSize:      float64(4),
	}
	if got := formatIndentFromOptions(opts); got != "    " {
		t.Fatalf("expected 4 spaces, got %q", got)
	}

	opts = protocol.FormattingOptions{
		protocol.FormattingOptionInsertSpaces: false,
		protocol.FormattingOptionTabSize:      float64(8),
	}
	if got := formatIndentFromOptions(opts); got != "\t" {
		t.Fatalf("expected tab, got %q", got)
	}
}

func formattingParams(uri string, insertSpaces bool, tabSize int) protocol.DocumentFormattingParams {
	return protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentUri(uri),
		},
		Options: protocol.FormattingOptions{
			protocol.FormattingOptionInsertSpaces: insertSpaces,
			protocol.FormattingOptionTabSize:      float64(tabSize),
		},
	}
}
