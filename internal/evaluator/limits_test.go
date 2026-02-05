package evaluator

import (
	"strings"
	"testing"

	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
)

func TestRecursionLimitInterpreterCatchable(t *testing.T) {
	input := `func f() { return f() }
try { f() } catch (e) { e.message }`

	runner := NewRunner()
	runner.SetMaxRecursion(5)

	got := testEvalWithRunner(t, input, runner)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", got, got)
	}
	if !strings.HasPrefix(strObj.Value, "max recursion depth exceeded") {
		t.Fatalf("expected recursion error message, got %q", strObj.Value)
	}
}

func TestMemoryLimitInterpreterCatchable(t *testing.T) {
	input := `try { s = "hello" } catch (e) { e.message }`

	runner := NewRunner()
	runner.SetMaxMemory(10)

	got := testEvalWithRunner(t, input, runner)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", got, got)
	}
	if strObj.Value != "max memory exceeded (10 bytes)" {
		t.Fatalf("expected memory error message, got %q", strObj.Value)
	}
}

func TestMemoryLimitInterpreterImage(t *testing.T) {
	input := `try { image_new(20, 20) } catch (e) { e.message }`

	runner := NewRunner()
	runner.SetMaxMemory(100)

	got := testEvalWithRunner(t, input, runner)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", got, got)
	}
	if strObj.Value != "max memory exceeded (100 bytes)" {
		t.Fatalf("expected memory error message, got %q", strObj.Value)
	}
}

func testEvalWithRunner(t *testing.T, input string, runner *Runner) object.Object {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	env := object.NewEnvironment()
	return eval(prog, env, runner, 0, 0)
}
