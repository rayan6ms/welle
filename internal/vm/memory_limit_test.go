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

func runVMWithMaxMemory(input string, maxMem int64) (*object.Dict, error) {
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
	m.SetMaxMemory(maxMem)
	if err := m.Run(); err != nil {
		return m.Exports(), err
	}
	return m.Exports(), nil
}

func TestMemoryLimitVMCatchable(t *testing.T) {
	input := `try { s = "hello" } catch (e) { export msg = e.message }`
	exports, err := runVMWithMaxMemory(input, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatalf("missing export msg")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", val, val)
	}
	if strObj.Value != "max memory exceeded (10 bytes)" {
		t.Fatalf("expected memory error message, got %q", strObj.Value)
	}
}

func TestMemoryLimitVMImage(t *testing.T) {
	input := `try { image_new(20, 20) } catch (e) { export msg = e.message }`
	exports, err := runVMWithMaxMemory(input, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatalf("missing export msg")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T (%v)", val, val)
	}
	if strObj.Value != "max memory exceeded (100 bytes)" {
		t.Fatalf("expected memory error message, got %q", strObj.Value)
	}
}
