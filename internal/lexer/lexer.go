package lexer

import (
	"strings"
	"unicode"

	"welle/internal/token"
)

type Lexer struct {
	input string

	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination

	line int // 1-based
	col  int // 1-based column of current char
}

func New(input string) *Lexer {
	l := &Lexer{
		input: input,
		line:  1,
		col:   0, // readChar() will advance to col=1 for first char
	}
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	// Skip spaces/tabs and comments, but NOT newlines.
	for {
		l.skipWhitespace()

		// Skip // comments
		if l.ch == '/' && l.peekChar() == '/' {
			l.skipLineComment()
			// After skipping comment, we may be at newline or EOF. Loop to handle it.
			continue
		}
		if l.ch == '/' && l.peekChar() == '*' {
			l.skipBlockComment()
			continue
		}

		break
	}

	// NEWLINE is a real token (statement separator)
	if l.ch == '\n' {
		tok := l.newToken(token.NEWLINE, "\n", l.line, l.col)
		l.readChar()
		return tok
	}

	// EOF
	if l.ch == 0 {
		return l.newToken(token.EOF, "", l.line, l.col)
	}

	startLine, startCol := l.line, l.col
	startIdx := l.position

	switch l.ch {
	case ';':
		tok := l.newToken(token.SEMICOLON, ";", startLine, startCol)
		l.readChar()
		return tok

	case '(':
		tok := l.newToken(token.LPAREN, "(", startLine, startCol)
		l.readChar()
		return tok
	case ')':
		tok := l.newToken(token.RPAREN, ")", startLine, startCol)
		l.readChar()
		return tok
	case '{':
		tok := l.newToken(token.LBRACE, "{", startLine, startCol)
		l.readChar()
		return tok
	case '}':
		tok := l.newToken(token.RBRACE, "}", startLine, startCol)
		l.readChar()
		return tok
	case '#':
		tok := l.newToken(token.HASH, "#", startLine, startCol)
		l.readChar()
		return tok
	case '[':
		tok := l.newToken(token.LBRACKET, "[", startLine, startCol)
		l.readChar()
		return tok
	case ']':
		tok := l.newToken(token.RBRACKET, "]", startLine, startCol)
		l.readChar()
		return tok
	case ',':
		tok := l.newToken(token.COMMA, ",", startLine, startCol)
		l.readChar()
		return tok
	case ':':
		tok := l.newToken(token.COLON, ":", startLine, startCol)
		l.readChar()
		return tok
	case '.':
		tok := l.newToken(token.DOT, ".", startLine, startCol)
		l.readChar()
		return tok

	case '+':
		tok := l.newToken(token.PLUS, "+", startLine, startCol)
		l.readChar()
		return tok
	case '-':
		tok := l.newToken(token.MINUS, "-", startLine, startCol)
		l.readChar()
		return tok
	case '*':
		tok := l.newToken(token.STAR, "*", startLine, startCol)
		l.readChar()
		return tok
	case '%':
		tok := l.newToken(token.PERCENT, "%", startLine, startCol)
		l.readChar()
		return tok
	case '/':
		// We already handled // comments above
		tok := l.newToken(token.SLASH, "/", startLine, startCol)
		l.readChar()
		return tok

	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			lit := string([]byte{ch, l.ch})
			tok := l.newToken(token.EQ, lit, startLine, startCol)
			l.readChar()
			return tok
		}
		tok := l.newToken(token.ASSIGN, "=", startLine, startCol)
		l.readChar()
		return tok

	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			lit := string([]byte{ch, l.ch})
			tok := l.newToken(token.NE, lit, startLine, startCol)
			l.readChar()
			return tok
		}
		// '!' alone not supported in v0.1
		tok := l.newToken(token.ILLEGAL, "!", startLine, startCol)
		l.readChar()
		return tok

	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			lit := string([]byte{ch, l.ch})
			tok := l.newToken(token.LE, lit, startLine, startCol)
			l.readChar()
			return tok
		}
		tok := l.newToken(token.LT, "<", startLine, startCol)
		l.readChar()
		return tok

	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			lit := string([]byte{ch, l.ch})
			tok := l.newToken(token.GE, lit, startLine, startCol)
			l.readChar()
			return tok
		}
		tok := l.newToken(token.GT, ">", startLine, startCol)
		l.readChar()
		return tok

	case '"':
		if l.startsTripleQuote() {
			return l.readTripleStringToken(startLine, startCol, startIdx)
		}
		return l.readStringToken(startLine, startCol, startIdx)
	case '`':
		lit := l.readRawString()
		tok := l.newToken(token.STRING, lit, startLine, startCol)
		if l.ch == '`' {
			l.readChar()
		}
		tok.Raw = l.input[startIdx:l.position]
		return tok
	}

	// Identifiers / keywords
	if isIdentStart(l.ch) {
		lit := l.readIdentifier()
		tt := token.LookupIdent(lit)
		return l.newToken(tt, lit, startLine, startCol)
	}

	// Numbers (int or float)
	if isDigit(l.ch) {
		lit, isFloat := l.readNumber()
		if isFloat {
			return l.newToken(token.FLOAT, lit, startLine, startCol)
		}
		return l.newToken(token.INT, lit, startLine, startCol)
	}

	// Unknown character
	illegal := string(l.ch)
	tok := l.newToken(token.ILLEGAL, illegal, startLine, startCol)
	l.readChar()
	return tok
}

func (l *Lexer) newToken(t token.Type, lit string, line, col int) token.Token {
	return token.Token{
		Type:    t,
		Literal: lit,
		Line:    line,
		Col:     col,
	}
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
		l.position = l.readPosition
		return
	}

	l.ch = l.input[l.readPosition]
	l.position = l.readPosition
	l.readPosition++

	// Track line/col for current char
	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) peekSecondChar() byte {
	if l.readPosition+1 >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition+1]
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipLineComment() {
	// We are at first '/' and next is '/'
	// Consume both and then everything until newline or EOF.
	l.readChar() // consume first '/'
	l.readChar() // consume second '/'

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	// Do not consume newline here — NextToken will emit NEWLINE token.
}

func (l *Lexer) skipBlockComment() {
	// We are at first '/' and next is '*'
	l.readChar() // consume '/'
	l.readChar() // consume '*'

	for l.ch != 0 {
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			l.readChar() // move to next char after */
			return
		}
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	start := l.position
	for isIdentPart(l.ch) {
		l.readChar()
	}
	return l.input[start:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	start := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	isFloat := false
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[start:l.position], isFloat
}

func (l *Lexer) readRawString() string {
	l.readChar() // move past opening backtick
	start := l.position
	for l.ch != 0 && l.ch != '`' {
		l.readChar()
	}
	return l.input[start:l.position]
}

func (l *Lexer) startsTripleQuote() bool {
	return l.ch == '"' && l.peekChar() == '"' && l.peekSecondChar() == '"'
}

func (l *Lexer) readTripleString() string {
	// Consume opening """
	l.readChar()
	l.readChar()
	l.readChar()

	start := l.position
	for l.ch != 0 {
		if l.ch == '"' && l.peekChar() == '"' && l.peekSecondChar() == '"' {
			out := l.input[start:l.position]
			l.readChar()
			l.readChar()
			l.readChar()
			return out
		}
		l.readChar()
	}

	return l.input[start:l.position]
}

func (l *Lexer) readTripleStringToken(startLine, startCol, startIdx int) token.Token {
	lit := l.readTripleString()
	tok := l.newToken(token.STRING, lit, startLine, startCol)
	tok.Raw = l.input[startIdx:l.position]
	return tok
}

func (l *Lexer) readStringToken(startLine, startCol, startIdx int) token.Token {
	// Current l.ch == '"'
	l.readChar() // move past opening quote

	var b strings.Builder
	for {
		if l.ch == 0 || l.ch == '\n' {
			// Unterminated string
			return l.newToken(token.ILLEGAL, "unterminated string", startLine, startCol)
		}
		if l.ch == '"' {
			// closing quote
			break
		}

		if l.ch == '\\' {
			// Minimal escapes for v0.1
			switch l.peekChar() {
			case '"':
				l.readChar()
				b.WriteByte('"')
				l.readChar()
				continue
			case '\\':
				l.readChar()
				b.WriteByte('\\')
				l.readChar()
				continue
			case 'n':
				l.readChar()
				b.WriteByte('\n')
				l.readChar()
				continue
			case 't':
				l.readChar()
				b.WriteByte('\t')
				l.readChar()
				continue
			default:
				// Unknown escape: keep the backslash literally
				b.WriteByte(l.ch)
				l.readChar()
				continue
			}
		}

		b.WriteByte(l.ch)
		l.readChar()
	}

	// l.ch == '"' (closing quote)
	l.readChar() // consume closing quote
	tok := l.newToken(token.STRING, b.String(), startLine, startCol)
	tok.Raw = l.input[startIdx:l.position]
	return tok
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	// ASCII letters, underscore; allow unicode letters too if you want (Go can, but byte-based lexer doesn’t).
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= 128 && unicode.IsLetter(rune(ch)))
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}
