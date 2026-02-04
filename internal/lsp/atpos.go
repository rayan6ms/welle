package lsp

import (
	"welle/internal/lexer"
	"welle/internal/token"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func IdentAt(text string, pos protocol.Position) (string, bool) {
	line := int(pos.Line) + 1
	col := int(pos.Character) + 1

	lx := lexer.New(text)
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			return "", false
		}
		if tok.Type != token.IDENT {
			continue
		}
		startCol := tok.Col
		endCol := tok.Col + max(1, len(tok.Literal))
		if tok.Line == line && col >= startCol && col < endCol {
			return tok.Literal, true
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
