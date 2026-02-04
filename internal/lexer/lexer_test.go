package lexer

import (
	"testing"

	"welle/internal/token"
)

func TestLexer_TourProgram(t *testing.T) {
	input := `func add(a, b) {
  return a + b
}

x = add(2, 3)
print(x)

if (x > 3) {
  print("big")
} else {
  print("small")
}

i = 0
	while (i < 3) {
  print(i)
  i = i + 1
}`

	tests := []struct {
		typ token.Type
		lit string
	}{
		{token.FUNC, "func"},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.RETURN, "return"},
		{token.IDENT, "a"},
		{token.PLUS, "+"},
		{token.IDENT, "b"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.INT, "2"},
		{token.COMMA, ","},
		{token.INT, "3"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},

		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.GT, ">"},
		{token.INT, "3"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.STRING, "big"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.STRING, "small"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "i"},
		{token.ASSIGN, "="},
		{token.INT, "0"},
		{token.NEWLINE, "\n"},

		{token.WHILE, "while"},
		{token.LPAREN, "("},
		{token.IDENT, "i"},
		{token.LT, "<"},
		{token.INT, "3"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.IDENT, "i"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},

		{token.IDENT, "i"},
		{token.ASSIGN, "="},
		{token.IDENT, "i"},
		{token.PLUS, "+"},
		{token.INT, "1"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.typ {
			t.Fatalf("tests[%d] - wrong type. expected=%q got=%q (lit=%q line=%d col=%d)",
				i, tt.typ, tok.Type, tok.Literal, tok.Line, tok.Col)
		}

		if tok.Literal != tt.lit {
			t.Fatalf("tests[%d] - wrong literal. expected=%q got=%q (type=%q line=%d col=%d)",
				i, tt.lit, tok.Literal, tok.Type, tok.Line, tok.Col)
		}
	}
}

func TestLexer_DictTokens(t *testing.T) {
	input := `#{"a": 1}`

	tests := []struct {
		typ token.Type
		lit string
	}{
		{token.HASH, "#"},
		{token.LBRACE, "{"},
		{token.STRING, "a"},
		{token.COLON, ":"},
		{token.INT, "1"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.typ {
			t.Fatalf("tests[%d] - wrong type. expected=%q got=%q (lit=%q)", i, tt.typ, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.lit {
			t.Fatalf("tests[%d] - wrong literal. expected=%q got=%q (type=%q)", i, tt.lit, tok.Literal, tok.Type)
		}
	}
}

func TestLexer_Dot(t *testing.T) {
	input := `a.b()`

	tests := []struct {
		typ token.Type
		lit string
	}{
		{token.IDENT, "a"},
		{token.DOT, "."},
		{token.IDENT, "b"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.typ {
			t.Fatalf("tests[%d] - wrong type. expected=%q got=%q (lit=%q)", i, tt.typ, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.lit {
			t.Fatalf("tests[%d] - wrong literal. expected=%q got=%q (type=%q)", i, tt.lit, tok.Literal, tok.Type)
		}
	}
}

func TestLexer_FloatLiteral(t *testing.T) {
	input := `x = 1.0
y = 0.5
z = 10.25
q = 1.2 + 3`

	tests := []struct {
		typ token.Type
		lit string
	}{
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.FLOAT, "1.0"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.FLOAT, "0.5"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "z"},
		{token.ASSIGN, "="},
		{token.FLOAT, "10.25"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "q"},
		{token.ASSIGN, "="},
		{token.FLOAT, "1.2"},
		{token.PLUS, "+"},
		{token.INT, "3"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.typ {
			t.Fatalf("tests[%d] - wrong type. expected=%q got=%q (lit=%q)", i, tt.typ, tok.Type, tok.Literal)
		}
		if tok.Literal != tt.lit {
			t.Fatalf("tests[%d] - wrong literal. expected=%q got=%q (type=%q)", i, tt.lit, tok.Literal, tok.Type)
		}
	}
}
