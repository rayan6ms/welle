package main

import (
	"strings"

	"welle/internal/format"
	"welle/internal/lsp"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func textDocumentFormatting(ctx *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	uri := string(params.TextDocument.URI)
	if !strings.HasSuffix(strings.ToLower(uri), ".wll") {
		return []protocol.TextEdit{}, nil
	}

	text, ok := store.Get(uri)
	if !ok {
		return []protocol.TextEdit{}, nil
	}

	indent := formatIndentFromOptions(params.Options)
	formatted, err := format.Format(text, format.Options{Indent: indent})
	if err != nil || formatted == text {
		return []protocol.TextEdit{}, nil
	}

	edit := protocol.TextEdit{
		Range:   lsp.FullDocumentRange(text),
		NewText: formatted,
	}
	return []protocol.TextEdit{edit}, nil
}

func formatIndentFromOptions(opts protocol.FormattingOptions) string {
	insertSpaces := true
	if v, ok := opts[protocol.FormattingOptionInsertSpaces]; ok {
		if b, ok := v.(bool); ok {
			insertSpaces = b
		}
	}

	tabSize := 2
	if v, ok := opts[protocol.FormattingOptionTabSize]; ok {
		switch n := v.(type) {
		case int:
			tabSize = n
		case int32:
			tabSize = int(n)
		case int64:
			tabSize = int(n)
		case uint:
			tabSize = int(n)
		case uint32:
			tabSize = int(n)
		case uint64:
			tabSize = int(n)
		case float32:
			tabSize = int(n)
		case float64:
			tabSize = int(n)
		}
	}
	if tabSize <= 0 {
		tabSize = 2
	}

	if insertSpaces {
		return strings.Repeat(" ", tabSize)
	}
	return "\t"
}
