package lsp

import (
	"welle/internal/lexer"
	"welle/internal/parser"
	"welle/internal/token"
)

// SemanticTokensForText returns unencoded semantic tokens for the given source text.
func SemanticTokensForText(text string) []SemTok {
	p := parser.New(lexer.New(text))
	prog := p.ParseProgram()
	classified := CollectSemantic(prog)

	type posKey struct {
		Line int
		Col  int
	}
	byPos := make(map[posKey]Classified, len(classified))
	for k, v := range classified {
		byPos[posKey{Line: k.Line, Col: k.Col}] = v
	}

	lx := lexer.New(text)
	sem := make([]SemTok, 0, 1024)

	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Type == token.NEWLINE {
			continue
		}

		if tok.Type == token.IDENT {
			length := len(tok.Literal)
			key := Key{Line: tok.Line, Col: tok.Col, Len: length}
			if cls, ok := classified[key]; ok {
				sem = append(sem, SemTok{
					Line:   tok.Line,
					Col:    tok.Col,
					Length: length,
					Type:   cls.Type,
					Mods:   cls.Mods,
				})
				continue
			}
			if cls, ok := byPos[posKey{Line: tok.Line, Col: tok.Col}]; ok {
				sem = append(sem, SemTok{
					Line:   tok.Line,
					Col:    tok.Col,
					Length: length,
					Type:   cls.Type,
					Mods:   cls.Mods,
				})
				continue
			}
		}

		tt, ok := Classify(tok)
		if !ok {
			continue
		}

		length := 1
		if tok.Literal != "" {
			length = len(tok.Literal)
		}

		sem = append(sem, SemTok{
			Line:   tok.Line,
			Col:    tok.Col,
			Length: length,
			Type:   tt,
			Mods:   0,
		})
	}

	return sem
}
