package lsp

import "testing"

func TestSemanticTokensParameters(t *testing.T) {
	text := `func is_palindrome(n) {
  if (n < 0) { return false }
  if (n >= 0 and n < 10) { return true }
  original = n
  rev = 0
  while (n > 0) {
    digit = n % 10
    rev = (rev * 10) + digit
    n = n / 10
  }
  return rev == original
}
`

	toks := SemanticTokensForText(text)

	if !hasToken(toks, 1, 20, ttParameter, modDecl) {
		t.Fatalf("expected parameter declaration token at 1:20")
	}
	if !hasToken(toks, 2, 7, ttParameter, 0) {
		t.Fatalf("expected parameter usage token at 2:7")
	}
	if !hasToken(toks, 9, 5, ttParameter, 0) {
		t.Fatalf("expected parameter usage token at 9:5")
	}
	if !hasToken(toks, 9, 9, ttParameter, 0) {
		t.Fatalf("expected parameter usage token at 9:9")
	}
}

func TestSemanticTokensParameterShadowing(t *testing.T) {
	text := `func f(n) {
  n = n + 1
  func g(n) {
    n = n + 2
  }
  n = n + 3
}
`

	toks := SemanticTokensForText(text)

	if !hasToken(toks, 1, 8, ttParameter, modDecl) {
		t.Fatalf("expected outer parameter declaration token at 1:8")
	}
	if !hasToken(toks, 2, 3, ttParameter, 0) || !hasToken(toks, 2, 7, ttParameter, 0) {
		t.Fatalf("expected outer parameter usage tokens at 2:3 and 2:7")
	}
	if !hasToken(toks, 3, 10, ttParameter, modDecl) {
		t.Fatalf("expected inner parameter declaration token at 3:10")
	}
	if !hasToken(toks, 4, 5, ttParameter, 0) || !hasToken(toks, 4, 9, ttParameter, 0) {
		t.Fatalf("expected inner parameter usage tokens at 4:5 and 4:9")
	}
	if !hasToken(toks, 6, 3, ttParameter, 0) || !hasToken(toks, 6, 7, ttParameter, 0) {
		t.Fatalf("expected outer parameter usage tokens at 6:3 and 6:7")
	}
}

func hasToken(toks []SemTok, line, col, typ, mods int) bool {
	for _, tok := range toks {
		if tok.Line == line && tok.Col == col && tok.Type == typ && tok.Mods == mods {
			return true
		}
	}
	return false
}
