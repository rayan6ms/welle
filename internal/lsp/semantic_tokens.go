package lsp

import (
	"strings"

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
		if tok.Raw != "" {
			length = len(tok.Raw)
		} else if tok.Literal != "" {
			length = len(tok.Literal)
		}

		sem = append(sem, SemTok{
			Line:   tok.Line,
			Col:    tok.Col,
			Length: length,
			Type:   tt,
			Mods:   0,
		})

		if tok.Type == token.TEMPLATE {
			sem = append(sem, templateInterpolationSemanticTokens(tok)...)
		}
	}

	return sem
}

func templateInterpolationSemanticTokens(tok token.Token) []SemTok {
	out := []SemTok{}
	raw := tok.Literal
	i := 0
	for i < len(raw) {
		if raw[i] == '$' && i+1 < len(raw) && raw[i+1] == '{' {
			exprStart := i + 2
			exprEnd, ok := findTemplateExprEnd(raw, exprStart)
			if !ok {
				return out
			}
			expr := strings.TrimSpace(raw[exprStart:exprEnd])
			if expr != "" {
				lx := lexer.New(expr)
				for {
					subTok := lx.NextToken()
					if subTok.Type == token.EOF || subTok.Type == token.NEWLINE {
						if subTok.Type == token.EOF {
							break
						}
						continue
					}
					tt, ok := Classify(subTok)
					if !ok {
						continue
					}
					length := 1
					if subTok.Raw != "" {
						length = len(subTok.Raw)
					} else if subTok.Literal != "" {
						length = len(subTok.Literal)
					}
					out = append(out, SemTok{
						Line:   tok.Line,
						Col:    tok.Col + 2 + exprStart + (subTok.Col - 1),
						Length: length,
						Type:   tt,
						Mods:   0,
					})
				}
			}
			i = exprEnd + 1
			continue
		}
		i++
	}
	return out
}

func findTemplateExprEnd(raw string, start int) (int, bool) {
	depth := 0
	i := start
	for i < len(raw) {
		switch raw[i] {
		case '"':
			if i+2 < len(raw) && raw[i+1] == '"' && raw[i+2] == '"' {
				i += 3
				for i < len(raw) {
					if i+2 < len(raw) && raw[i] == '"' && raw[i+1] == '"' && raw[i+2] == '"' {
						i += 3
						break
					}
					i++
				}
				continue
			}
			i++
			for i < len(raw) {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		case '`':
			i++
			for i < len(raw) && raw[i] != '`' {
				i++
			}
			if i < len(raw) {
				i++
			}
			continue
		case '/':
			if i+1 < len(raw) && raw[i+1] == '/' {
				i += 2
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(raw) && raw[i+1] == '*' {
				i += 2
				for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
					i++
				}
				if i+1 < len(raw) {
					i += 2
				}
				continue
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i, true
			}
			depth--
		}
		i++
	}
	return 0, false
}
