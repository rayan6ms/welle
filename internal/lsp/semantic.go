package lsp

import "welle/internal/token"

// semantic token type indices (must match legend order in server)
const (
	ttKeyword   = 0
	ttString    = 1
	ttNumber    = 2
	ttOperator  = 3
	ttFunction  = 4
	ttVariable  = 5
	ttParameter = 6
	ttNamespace = 7
	ttType      = 8
	ttComment   = 9
)

const (
	modDecl     = 1 << 0
	modReadonly = 1 << 1
)

type SemTok struct {
	Line   int // 1-based
	Col    int // 1-based
	Length int
	Type   int
	Mods   int
}

// Classify returns (typeIndex, ok).
func Classify(tok token.Token) (int, bool) {
	switch tok.Type {
	// keywords
	case token.FUNC, token.RETURN, token.IF, token.ELSE, token.WHILE, token.FOR,
		token.SWITCH, token.CASE, token.DEFAULT, token.MATCH,
		token.TRY, token.CATCH, token.FINALLY, token.THROW, token.DEFER,
		token.BREAK, token.CONTINUE, token.IMPORT, token.EXPORT,
		token.IN, token.TRUE, token.FALSE, token.NIL, token.AND, token.OR, token.NOT,
		token.FROM, token.AS:
		return ttKeyword, true

	// literals
	case token.STRING:
		return ttString, true
	case token.INT, token.FLOAT:
		return ttNumber, true

	// operators & punctuation that you want colored as operator
	case token.ASSIGN, token.PLUS, token.MINUS, token.STAR, token.SLASH,
		token.PERCENT, token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
		token.DOT:
		return ttOperator, true

	// identifiers
	case token.IDENT:
		// For v0.1: treat all as variable.
		// Later: use parser/symbol info to mark function/type.
		return ttVariable, true
	}

	return 0, false
}
