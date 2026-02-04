package lexer

import (
	"testing"

	"welle/internal/token"
)

func TestComments(t *testing.T) {
	input := "\n" +
		"x = 1 // comment\n" +
		"/* block\n" +
		"comment */ y = 2\n"
	l := New(input)
	types := []token.Type{
		token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.EOF,
	}

	for i, tt := range types {
		tok := l.NextToken()
		if tok.Type != tt {
			t.Fatalf("i=%d expected=%q got=%q (%q)", i, tt, tok.Type, tok.Literal)
		}
	}
}

func TestMultiLineStrings(t *testing.T) {
	input := "a = `hi\nthere`\n" +
		"b = \"\"\"line1\nline2\"\"\"\n"

	l := New(input)

	// a = `...`
	if l.NextToken().Type != token.IDENT {
		t.Fatal("expected IDENT")
	}
	if l.NextToken().Type != token.ASSIGN {
		t.Fatal("expected =")
	}
	s1 := l.NextToken()
	if s1.Type != token.STRING || s1.Literal != "hi\nthere" {
		t.Fatalf("bad raw string: %v %q", s1.Type, s1.Literal)
	}
	_ = l.NextToken() // NEWLINE

	// b = """..."""
	if l.NextToken().Type != token.IDENT {
		t.Fatal("expected IDENT")
	}
	if l.NextToken().Type != token.ASSIGN {
		t.Fatal("expected =")
	}
	s2 := l.NextToken()
	if s2.Type != token.STRING || s2.Literal != "line1\nline2" {
		t.Fatalf("bad triple string: %v %q", s2.Type, s2.Literal)
	}
}
