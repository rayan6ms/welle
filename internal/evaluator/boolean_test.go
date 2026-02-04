package evaluator

import (
	"testing"

	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
)

func evalInput(t *testing.T, input string) object.Object {
	t.Helper()
	l := lexer.New(input + "\n")
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	env := object.NewEnvironment()
	return Eval(prog, env)
}

func TestBooleanOps(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"true and false", "false"},
		{"true or false", "true"},
		{"not true", "false"},
		{"not false", "true"},
		{"1 < 2 and 3 < 4", "true"},
		{"1 < 2 and 3 > 4", "false"},
		{"false and (10 / 0)", "false"},
		{"true or (10 / 0)", "true"},
	}

	for _, tt := range tests {
		got := evalInput(t, tt.in).Inspect()
		if got != tt.want {
			t.Fatalf("input=%q expected=%q got=%q", tt.in, tt.want, got)
		}
	}
}
