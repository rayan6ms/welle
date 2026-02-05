package format

import (
	"strings"
	"testing"

	"welle/internal/lexer"
	"welle/internal/token"
)

const palindromeInput = `func abs_int(n) {
  if (n <  0) {  return -n }
  return n
}

func is_palindrome(n) {
  if (n < 0) { return false }

  if (n >=  0 and n < 10) { return true }

  original =  n
  rev =    0

  while (n > 0) {
    digit = n % 10
    rev = (rev * 10) + digit
    n = n / 10
  }

  return rev == original
}

func assert_eq(label, got, want) {
  if (got != want) {
    throw "assert failed: " + label
  }
}

assert_eq("121", is_palindrome(121), true)
assert_eq("123", is_palindrome(123), false)
assert_eq("0", is_palindrome(0), true)
assert_eq("7", is_palindrome(7), true)
assert_eq("10", is_palindrome(10), false)
assert_eq("1221", is_palindrome(1221), true)
assert_eq("1001", is_palindrome(1001), true)
assert_eq("-121", is_palindrome(-121), false)

print("121: ", is_palindrome(121))
print("123: ", is_palindrome(123))
print("1221: ", is_palindrome(1221))
print("10: ", is_palindrome(10))
`

func TestFormat_StringPreservationAndIdempotence(t *testing.T) {
	formatted, err := Format(palindromeInput, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(formatted, `"121: "`) {
		t.Fatalf("formatted output lost string punctuation: %q", formatted)
	}
	if !strings.Contains(formatted, `throw "assert failed: " + label`) {
		t.Fatalf("formatted output lost throw string: %q", formatted)
	}
	if !strings.Contains(formatted, `assert_eq("121", is_palindrome(121), true)`) {
		t.Fatalf("formatted output lost quotes around arguments: %q", formatted)
	}

	reformatted, err := Format(formatted, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != formatted {
		t.Fatalf("format not idempotent")
	}
}

func TestFormat_TokenPreservation(t *testing.T) {
	formatted, err := Format(palindromeInput, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := compactTokens(lexTokens(palindromeInput))
	got := compactTokens(lexTokens(formatted))
	if len(want) != len(got) {
		t.Fatalf("token count mismatch: %d vs %d", len(want), len(got))
	}
	for i := range want {
		if want[i].Type != got[i].Type || want[i].Literal != got[i].Literal {
			t.Fatalf("token mismatch at %d: want %s %q, got %s %q", i, want[i].Type, want[i].Literal, got[i].Type, got[i].Literal)
		}
	}
}

func TestFormat_UnaryMinusAndCommas(t *testing.T) {
	input := "x=-n\ny=-121\nz = a-1\nprint(\"121: \", x)\n"
	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(formatted, "x = -n") {
		t.Fatalf("expected unary minus to stay attached: %q", formatted)
	}
	if !strings.Contains(formatted, "y = -121") {
		t.Fatalf("expected unary minus to stay attached to int: %q", formatted)
	}
	if !strings.Contains(formatted, "z = a - 1") {
		t.Fatalf("expected binary minus to be spaced: %q", formatted)
	}
	if !strings.Contains(formatted, `print("121: ", x)`) {
		t.Fatalf("expected string with colon to remain quoted: %q", formatted)
	}
}

func TestFormat_UnaryBang(t *testing.T) {
	input := "x=!a\ny=!!a\nz=!(a and b)\nw=!f(x)\nv=!a==b\n"
	want := "x = !a\ny = !!a\nz = !(a and b)\nw = !f(x)\nv = !a == b\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_BitwiseOperators(t *testing.T) {
	input := "x=1|2\ny=1&2\nz=1^2\ns=1<<2\nt=1>>2\nu=~x\nv=~(1|2)\n"
	want := "x = 1 | 2\ny = 1 & 2\nz = 1 ^ 2\ns = 1 << 2\nt = 1 >> 2\nu = ~x\nv = ~(1 | 2)\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_InOperator(t *testing.T) {
	input := "x=1 in a\ny=1 in a and b\nz=1 in a or b\n"
	want := "x = 1 in a\ny = 1 in a and b\nz = 1 in a or b\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_IsOperatorAndTemplate(t *testing.T) {
	input := "x=1 is 1\ny=tag t\"a=${x}\"\n"
	want := "x = 1 is 1\ny = tag t\"a=${x}\"\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}
}

func TestFormat_DictUpdateAssign(t *testing.T) {
	input := "d|=other\n"
	want := "d |= other\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_SpacingAroundParensAndBraces(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "operator_lparen_spacing",
			input: "rev = (rev * 10) + digit\n",
			want:  "rev = (rev * 10) + digit\n",
		},
		{
			name:  "operator_lparen_missing_space",
			input: "phase = phase +(dt * SPEED)\n",
			want:  "phase = phase + (dt * SPEED)\n",
		},
		{
			name:  "inline_block_spacing",
			input: "func abs_int(n) {\n  if (n < 0) { return -n }\n  return n\n}\n",
			want:  "func abs_int(n) {\n  if (n < 0) { return -n }\n  return n\n}\n",
		},
		{
			name:  "space_before_rbrace_inline",
			input: "while (x) { c2 = rand_color()}\n",
			want:  "while (x) { c2 = rand_color() }\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted, err := Format(tt.input, Options{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if formatted != tt.want {
				t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", tt.want, formatted)
			}

			reformatted, err := Format(tt.want, Options{})
			if err != nil {
				t.Fatalf("unexpected error on reformat: %v", err)
			}
			if reformatted != tt.want {
				t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", tt.want, reformatted)
			}
		})
	}
}

func TestFormat_CompoundAssignments(t *testing.T) {
	input := "x+=1\na[i]-=2\nd.x*=3\n"
	want := "x += 1\na[i] -= 2\nd.x *= 3\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_ForInDestructure(t *testing.T) {
	input := "for(k,_ )in d{print(k)}\n"
	want := "for (k, _) in d { print(k) }\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_StringLinesPreserved(t *testing.T) {
	input := "print(\"121: \", x)\nthrow \"assert failed: \" + label\n"
	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != input {
		t.Fatalf("expected string lines to remain identical:\nwant: %q\ngot:  %q", input, formatted)
	}
}

func TestFormat_FunctionLiterals(t *testing.T) {
	input := "f=func (x,y){return x+1}\nprint((func(x){return x*2})(21))\n"
	want := "f = func(x, y) { return x + 1 }\nprint((func(x) { return x * 2 })(21))\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_TuplesAndReturnList(t *testing.T) {
	input := "t=(1,2)\nreturn 1,2\n(a,b)=(3,4)\n"
	want := "t = (1, 2)\nreturn 1, 2\n(a, b) = (3, 4)\n"

	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatted != want {
		t.Fatalf("unexpected formatting:\nwant: %q\ngot:  %q", want, formatted)
	}

	reformatted, err := Format(want, Options{})
	if err != nil {
		t.Fatalf("unexpected error on reformat: %v", err)
	}
	if reformatted != want {
		t.Fatalf("format not idempotent:\nwant: %q\ngot:  %q", want, reformatted)
	}
}

func TestFormat_NumberLiteralsPreserved(t *testing.T) {
	input := "x=0xFF_FF\ny=1_2.3_4\nz=1e3\nw=0b1010\n"
	formatted, err := Format(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(formatted, "x = 0xFF_FF") {
		t.Fatalf("expected hex literal preserved: %q", formatted)
	}
	if !strings.Contains(formatted, "y = 1_2.3_4") {
		t.Fatalf("expected float literal preserved: %q", formatted)
	}
	if !strings.Contains(formatted, "z = 1e3") {
		t.Fatalf("expected exponent literal preserved: %q", formatted)
	}
	if !strings.Contains(formatted, "w = 0b1010") {
		t.Fatalf("expected binary literal preserved: %q", formatted)
	}
}

func lexTokens(src string) []token.Token {
	l := lexer.New(src)
	var toks []token.Token
	for {
		tok := l.NextToken()
		toks = append(toks, tok)
		if tok.Type == token.EOF {
			break
		}
	}
	return toks
}

func compactTokens(toks []token.Token) []token.Token {
	out := make([]token.Token, 0, len(toks))
	for _, tok := range toks {
		if tok.Type == token.NEWLINE || tok.Type == token.EOF {
			continue
		}
		out = append(out, token.Token{
			Type:    tok.Type,
			Literal: tokenValue(tok),
		})
	}
	return out
}

func tokenValue(tok token.Token) string {
	if tok.Type == token.STRING && tok.Raw != "" {
		return tok.Raw
	}
	return tok.Literal
}
