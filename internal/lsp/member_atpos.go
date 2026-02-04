package lsp

import (
	"welle/internal/lexer"
	"welle/internal/token"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type MemberRef struct {
	Alias  string
	Member string
	Ok     bool
}

func MemberAt(text string, pos protocol.Position) MemberRef {
	line := int(pos.Line) + 1
	col := int(pos.Character) + 1

	lx := lexer.New(text)
	var prev2, prev1 token.Token

	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			return MemberRef{}
		}
		if tok.Type == token.NEWLINE {
			continue
		}

		if tok.Type == token.IDENT {
			startCol := tok.Col
			endCol := tok.Col + max(1, len(tok.Literal))
			if tok.Line == line && col >= startCol && col < endCol {
				if prev1.Type == token.DOT && prev2.Type == token.IDENT {
					return MemberRef{Alias: prev2.Literal, Member: tok.Literal, Ok: true}
				}
				return MemberRef{Member: tok.Literal, Ok: true}
			}
		}

		prev2 = prev1
		prev1 = tok
	}
}
