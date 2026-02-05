package ast

import (
	"fmt"

	"welle/internal/token"
)

// AnonymousFuncName returns a stable synthetic name for anonymous functions.
func AnonymousFuncName(tok token.Token) string {
	if tok.Line > 0 && tok.Col > 0 {
		return fmt.Sprintf("<anon@%d:%d>", tok.Line, tok.Col)
	}
	return "<anon>"
}
