package vm

import (
	"fmt"
	"strings"
	"testing"

	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
)

func buildVMLimited(input string, maxRecursion int, maxSteps int64) (*VM, error) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %s", strings.Join(p.Errors(), "; "))
	}

	c := compiler.NewWithFile("test.wll")
	if err := c.Compile(program); err != nil {
		return nil, err
	}
	bc := c.Bytecode()

	m := New(bc)
	m.SetMaxRecursion(maxRecursion)
	m.SetMaxSteps(maxSteps)
	return m, nil
}

func TestVMRecursionLimitCatchable(t *testing.T) {
	input := `func f6() { return 1 }
func f5() { return f6() }
func f4() { return f5() }
func f3() { return f4() }
func f2() { return f3() }
func f1() { return f2() }
try { f1() } catch (e) { export msg = e.message }`

	m, err := buildVMLimited(input, 3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exports := m.Exports()
	msgObj, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg")
	}
	strObj, ok := msgObj.(*object.String)
	if !ok {
		t.Fatalf("expected msg string, got %T (%v)", msgObj, msgObj)
	}
	if !strings.HasPrefix(strObj.Value, "max recursion depth exceeded") {
		t.Fatalf("expected recursion error message, got %q", strObj.Value)
	}
}

func TestVMStepLimitCatchable(t *testing.T) {
	input := `try { while (true) { } } catch (e) { export msg = e.message }`

	m, err := buildVMLimited(input, 0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := m.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exports := m.Exports()
	msgObj, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg")
	}
	strObj, ok := msgObj.(*object.String)
	if !ok {
		t.Fatalf("expected msg string, got %T (%v)", msgObj, msgObj)
	}
	if !strings.HasPrefix(strObj.Value, "max instruction count exceeded") {
		t.Fatalf("expected step limit error message, got %q", strObj.Value)
	}
}
