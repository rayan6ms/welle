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
		if (tok.Type == token.STRING || tok.Type == token.TEMPLATE) && tok.Raw != "" {
			return tok.Raw
		}
		if tok.Type == token.NIL {
			return "nil"
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
		case token.ASSIGN, token.WALRUS, token.PLUS, token.STAR, token.SLASH,
			token.PERCENT, token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
			token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN, token.PERCENT_ASSIGN, token.BITOR_ASSIGN,
			token.AND, token.OR, token.IN, token.IS, token.QUESTION, token.NULLISH,
			token.BITOR, token.BITAND, token.BITXOR, token.SHL, token.SHR:
			return true
		default:
			return false
		}
	}

	isExprEnd := func(t token.Type) bool {
		switch t {
		case token.IDENT, token.INT, token.FLOAT, token.STRING, token.TEMPLATE,
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

	shouldTrimBeforeLParen := func(prev token.Token, prevUnaryMinus, prevUnaryBang, prevUnaryTilde bool) bool {
		switch prev.Type {
		case token.IDENT, token.INT, token.FLOAT, token.STRING,
			token.TRUE, token.FALSE, token.NIL,
			token.RPAREN, token.RBRACKET, token.RBRACE, token.DOT:
			return true
		case token.MINUS:
			return prevUnaryMinus
		case token.BANG:
			return prevUnaryBang
		case token.BITNOT:
			return prevUnaryTilde
		default:
			if isBinaryOperator(prev.Type) {
				return false
			}
			return true
		}
	}

	prev := token.Token{}
	prevUnaryMinus := false
	prevUnaryBang := false
	prevUnaryTilde := false
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	type ternaryDepth struct {
		paren   int
		bracket int
		brace   int
	}
	var ternaryStack []ternaryDepth
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
			isTernary := false
			if len(ternaryStack) > 0 {
				top := ternaryStack[len(ternaryStack)-1]
				if top.paren == parenDepth && top.bracket == bracketDepth && top.brace == braceDepth {
					isTernary = true
					ternaryStack = ternaryStack[:len(ternaryStack)-1]
				}
			}
			if isTernary {
				space()
				write(":")
				space()
			} else {
				write(":")
				space()
			}

		case token.LPAREN:
			if needsSpaceBeforeParen(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
			} else {
				if shouldTrimBeforeLParen(prev, prevUnaryMinus, prevUnaryBang, prevUnaryTilde) {
					// Do not trim if a binary operator already emitted a trailing space.
					trimTrailingSpace()
				}
			}
			write("(")
			parenDepth++

		case token.RPAREN:
			trimTrailingSpace()
			write(")")
			if parenDepth > 0 {
				parenDepth--
			}

		case token.LBRACKET:
			trimTrailingSpace()
			write("[")
			bracketDepth++

		case token.RBRACKET:
			trimTrailingSpace()
			write("]")
			if bracketDepth > 0 {
				bracketDepth--
			}

		case token.LBRACE:
			trimTrailingSpace()
			if shouldSpaceBeforeLBrace(prev.Type) && !atLineStart {
				space()
			}
			write("{")
			indent++
			braceDepth++

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
			if braceDepth > 0 {
				braceDepth--
			}

		case token.DOT:
			trimTrailingSpace()
			write(".")

		case token.ELLIPSIS:
			if isExprEnd(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
				write("...")
				prevUnaryMinus = false
				prevUnaryBang = false
				break
			}
			trimTrailingSpace()
			if !atLineStart && prev.Type != token.LPAREN && prev.Type != token.LBRACKET && prev.Type != token.DOT {
				space()
			}
			write("...")
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false

		case token.MINUS:
			if isExprEnd(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
				write("-")
				space()
				prevUnaryMinus = false
				prevUnaryBang = false
				prevUnaryTilde = false
			} else {
				trimTrailingSpace()
				if !atLineStart && prev.Type != token.LPAREN && prev.Type != token.LBRACKET && prev.Type != token.DOT && prev.Type != token.ELLIPSIS {
					space()
				}
				write("-")
				prevUnaryMinus = true
				prevUnaryBang = false
				prevUnaryTilde = false
			}

		case token.BANG:
			if isExprEnd(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
				write("!")
				space()
				prevUnaryBang = false
				prevUnaryMinus = false
				prevUnaryTilde = false
			} else {
				trimTrailingSpace()
				if !atLineStart &&
					prev.Type != token.LPAREN &&
					prev.Type != token.LBRACKET &&
					prev.Type != token.DOT &&
					prev.Type != token.ELLIPSIS &&
					!(prev.Type == token.BANG && prevUnaryBang) {
					space()
				}
				write("!")
				prevUnaryBang = true
				prevUnaryMinus = false
				prevUnaryTilde = false
			}

		case token.BITNOT:
			if isExprEnd(prev.Type) {
				trimTrailingSpace()
				if !atLineStart {
					space()
				}
				write("~")
				space()
				prevUnaryBang = false
				prevUnaryMinus = false
				prevUnaryTilde = false
			} else {
				trimTrailingSpace()
				if !atLineStart &&
					prev.Type != token.LPAREN &&
					prev.Type != token.LBRACKET &&
					prev.Type != token.DOT &&
					prev.Type != token.ELLIPSIS &&
					!(prev.Type == token.BITNOT && prevUnaryTilde) {
					space()
				}
				write("~")
				prevUnaryTilde = true
				prevUnaryBang = false
				prevUnaryMinus = false
			}

		case token.ASSIGN, token.WALRUS, token.PLUS, token.STAR, token.SLASH,
			token.PERCENT, token.EQ, token.NE, token.LT, token.GT, token.LE, token.GE,
			token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN, token.PERCENT_ASSIGN,
			token.AND, token.OR, token.IN, token.QUESTION,
			token.BITOR, token.BITAND, token.BITXOR, token.SHL, token.SHR:
			trimTrailingSpace()
			space()
			write(tok.Literal)
			space()
			if tok.Type == token.QUESTION {
				ternaryStack = append(ternaryStack, ternaryDepth{
					paren:   parenDepth,
					bracket: bracketDepth,
					brace:   braceDepth,
				})
			}
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false

		case token.IF, token.WHILE, token.FOR, token.SWITCH, token.MATCH:
			trimTrailingSpace()
			if !atLineStart {
				space()
			}
			write(tok.Literal)
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false

		case token.CASE, token.DEFAULT, token.ELSE, token.TRY, token.CATCH, token.FINALLY,
			token.THROW, token.DEFER, token.RETURN, token.BREAK, token.CONTINUE, token.PASS,
			token.IMPORT, token.FROM, token.AS, token.EXPORT, token.NOT:
			trimTrailingSpace()
			if !atLineStart {
				space()
			}
			write(tok.Literal)
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false

		case token.FUNC:
			trimTrailingSpace()
			if !atLineStart &&
				prev.Type != token.LPAREN &&
				prev.Type != token.LBRACKET &&
				prev.Type != token.COMMA &&
				prev.Type != token.COLON {
				space()
			}
			write(tok.Literal)
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false

		default:
			if !atLineStart {
				if prev.Type != token.LPAREN &&
					prev.Type != token.DOT &&
					prev.Type != token.ELLIPSIS &&
					prev.Type != token.LBRACKET &&
					prev.Type != token.COMMA &&
					prev.Type != token.COLON &&
					!(prev.Type == token.MINUS && prevUnaryMinus) &&
					!(prev.Type == token.BANG && prevUnaryBang) &&
					!(prev.Type == token.BITNOT && prevUnaryTilde) {
					space()
				}
			}
			write(tokenText(tok))
			prevUnaryMinus = false
			prevUnaryBang = false
			prevUnaryTilde = false
		}

		prev = tok
	}

	trimTrailingSpace()
	s := out.String()
	s = strings.TrimRight(s, " \t\n") + "\n"
	return s, nil
}
