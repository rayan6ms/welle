package lsp

import (
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// EndPositionUTF16 returns the LSP position at the end of text, using UTF-16 code units.
func EndPositionUTF16(text string) protocol.Position {
	var line uint32
	var col uint32
	for _, r := range text {
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
	return protocol.Position{Line: line, Character: col}
}

// FullDocumentRange returns an LSP range covering the entire document.
func FullDocumentRange(text string) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   EndPositionUTF16(text),
	}
}
