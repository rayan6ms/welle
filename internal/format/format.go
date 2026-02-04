package format

import (
	"bytes"
	"strings"

	"welle/internal/lexer"
	"welle/internal/token"
)

type Options struct {
	Indent string // "  " or "\t"
}

func Format(src string, opt Options) (string, error) {
	if opt.Indent == "" {
		opt.Indent = "  "
	}

	l := lexer.New(src)
	var out bytes.Buffer

	indent := 0
	atLineStart := true
	justBrokeLine := false
	pendingNewlines := 0

	emitIndent := func() {
		if atLineStart {
			for i := 0; i < indent; i++ {
				out.WriteString(opt.Indent)
			}
			atLineStart = false
		}
	}

	newline := func() {
		out.WriteByte('\n')
		atLineStart = true
		justBrokeLine = true
	}

	space := func() {
		if out.Len() == 0 || atLineStart {
			return
		}
		b := out.Bytes()
		if len(b) > 0 && b[len(b)-1] != ' ' && b[len(b)-1] != '\n' {
			out.WriteByte(' ')
		}
	}

	write := func(s string) {
		emitIndent()
		out.WriteString(s)
		justBrokeLine = false
	}

	trimTrailingSpace := func() {
		for out.Len() > 0 {
			b := out.Bytes()
			last := b[len(b)-1]
			if last != ' ' && last != '\t' {
				return
			}
			out.Truncate(out.Len() - 1)
		}
	}

	tokenText := func(tok token.Token) string {
		if tok.Type == token.STRING && tok.Raw != "" {
			return tok.Raw
		}
		return tok.Literal
	}

	needsSpaceBeforeParen := func(t token.Type) bool {
		switch t {
		case token.IF, token.WHILE, token.FOR, token.SWITCH, token.MATCH:
			return true
		default:
			return false
		}
	}

	isBinaryOperator := func(t token.Type) bool {
		switch t {
		case token.ASSIGN, token.PLUS, token.STAR, token.SLASH,
			token.PERCENT, token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
			token.AND, token.OR, token.IN:
			return true
		default:
			return false
		}
	}

	isExprEnd := func(t token.Type) bool {
		switch t {
		case token.IDENT, token.INT, token.FLOAT, token.STRING,
			token.TRUE, token.FALSE, token.NIL,
			token.RPAREN, token.RBRACKET, token.RBRACE:
			return true
		default:
			return false
		}
	}

	shouldSpaceBeforeLBrace := func(t token.Type) bool {
		return isExprEnd(t)
	}

	shouldTrimBeforeLParen := func(prev token.Token, prevUnaryMinus bool) bool {
		switch prev.Type {
		case token.IDENT, token.INT, token.FLOAT, token.STRING,
			token.TRUE, token.FALSE, token.NIL,
			token.RPAREN, token.RBRACKET, token.RBRACE, token.DOT:
			return true
		case token.MINUS:
			return prevUnaryMinus
		default:
			if isBinaryOperator(prev.Type) {
				return false
			}
			return true
		}
	}

	prev := token.Token{}
	prevUnaryMinus := false
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}

		if tok.Type == token.NEWLINE {
			pendingNewlines++
			continue
		}

		if pendingNewlines > 0 {
			trimTrailingSpace()
			breaks := 1
			if pendingNewlines > 1 {
				breaks = 2
			}
			if justBrokeLine && breaks > 0 {
				breaks--
			}
			for i := 0; i < breaks; i++ {
				newline()
			}
			pendingNewlines = 0
		}

		if tok.Type == token.RBRACE {
			if indent > 0 {
				indent--
			}
		}

		switch tok.Type {
		case token.COMMA:
			trimTrailingSpace()
			write(",")
			space()

		case token.SEMICOLON:
			trimTrailingSpace()
			write(";")
			newline()

		case token.COLON:
			trimTrailingSpace()
			write(":")
			space()

		case token.LPAREN:
			if needsSpaceBeforeParen(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
			} else {
				if shouldTrimBeforeLParen(prev, prevUnaryMinus) {
					// Do not trim if a binary operator already emitted a trailing space.
					trimTrailingSpace()
				}
			}
			write("(")

		case token.RPAREN:
			trimTrailingSpace()
			write(")")

		case token.LBRACKET:
			trimTrailingSpace()
			write("[")

		case token.RBRACKET:
			trimTrailingSpace()
			write("]")

		case token.LBRACE:
			trimTrailingSpace()
			if shouldSpaceBeforeLBrace(prev.Type) && !atLineStart {
				space()
			}
			write("{")
			indent++

		case token.RBRACE:
			if !atLineStart {
				b := out.Bytes()
				if len(b) > 0 {
					last := b[len(b)-1]
					if last != ' ' && last != '\t' && last != '\n' && last != '{' {
						out.WriteByte(' ')
					}
				}
			}
			write("}")

		case token.DOT:
			trimTrailingSpace()
			write(".")

		case token.MINUS:
			if isExprEnd(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
				write("-")
				space()
				prevUnaryMinus = false
			} else {
				trimTrailingSpace()
				if !atLineStart && prev.Type != token.LPAREN && prev.Type != token.LBRACKET && prev.Type != token.DOT {
					space()
				}
				write("-")
				prevUnaryMinus = true
			}

		case token.ASSIGN, token.PLUS, token.STAR, token.SLASH,
			token.PERCENT, token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
			token.AND, token.OR, token.IN:
			trimTrailingSpace()
			space()
			write(tok.Literal)
			space()
			prevUnaryMinus = false

		case token.IF, token.WHILE, token.FOR, token.SWITCH, token.MATCH:
			trimTrailingSpace()
			if !atLineStart {
				space()
			}
			write(tok.Literal)
			prevUnaryMinus = false

		case token.CASE, token.DEFAULT, token.ELSE, token.TRY, token.CATCH, token.FINALLY,
			token.THROW, token.DEFER, token.RETURN, token.FUNC, token.BREAK, token.CONTINUE,
			token.IMPORT, token.FROM, token.AS, token.EXPORT, token.NOT:
			trimTrailingSpace()
			if !atLineStart {
				space()
			}
			write(tok.Literal)
			prevUnaryMinus = false

		default:
			if !atLineStart {
				if prev.Type != token.LPAREN &&
					prev.Type != token.DOT &&
					prev.Type != token.LBRACKET &&
					prev.Type != token.COMMA &&
					prev.Type != token.COLON &&
					!(prev.Type == token.MINUS && prevUnaryMinus) {
					space()
				}
			}
			write(tokenText(tok))
			prevUnaryMinus = false
		}

		prev = tok
	}

	trimTrailingSpace()
	s := out.String()
	s = strings.TrimRight(s, " \t\n") + "\n"
	return s, nil
}
