package evaluator

import (
	"testing"

	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
)

func evalProgramInTest(t *testing.T, input string) object.Object {
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

func TestClosureReadsOuter(t *testing.T) {
	input := `
 x = 5
 func f() { return x }
 f()
 `
	got := evalProgramInTest(t, input)
	if got.Inspect() != "5" {
		t.Fatalf("expected 5, got %s", got.Inspect())
	}
}

func TestClosureMutatesOuter(t *testing.T) {
	input := `
 x = 0
 func inc() { x = x + 1; return x }
 inc()
 inc()
 x
 `
	got := evalProgramInTest(t, input)
	if got.Inspect() != "2" {
		t.Fatalf("expected 2, got %s", got.Inspect())
	}
}

func TestNestedClosureState(t *testing.T) {
	input := `
 func makeCounter() {
   n = 0
   func inc() { n = n + 1; return n }
   return inc
 }

 c = makeCounter()
 c()
 c()
 c()
 `
	got := evalProgramInTest(t, input)
	if got.Inspect() != "3" {
		t.Fatalf("expected 3, got %s", got.Inspect())
	}
}
