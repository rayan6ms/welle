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

func runVM(input string) (*object.Dict, error) {
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
	if err := m.Run(); err != nil {
		return m.Exports(), err
	}
	return m.Exports(), nil
}

func exportValue(exports *object.Dict, name string) (object.Object, bool) {
	key := &object.String{Value: name}
	hk, ok := object.HashKeyOf(key)
	if !ok {
		return nil, false
	}
	pair, ok := exports.Pairs[object.HashKeyString(hk)]
	if !ok {
		return nil, false
	}
	return pair.Value, true
}

func TestVMThrowStringCaught(t *testing.T) {
	input := `try { throw "nope" } catch (e) { export msg = e.message }`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string, got %T", val)
	}
	if strObj.Value != "nope" {
		t.Fatalf("expected %q, got %q", "nope", strObj.Value)
	}
}

func TestVMThrowFinallyOverrides(t *testing.T) {
	input := `try { throw "a" } catch (e) { export out = "caught" } finally { throw "b" }`

	exports, err := runVM(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "b") {
		t.Fatalf("expected error to contain %q, got %q", "b", err.Error())
	}
	val, ok := exportValue(exports, "out")
	if !ok {
		t.Fatal("expected export out to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected out to be string, got %T", val)
	}
	if strObj.Value != "caught" {
		t.Fatalf("expected %q, got %q", "caught", strObj.Value)
	}
}

func TestVMErrorBuiltinCreatesObject(t *testing.T) {
	input := `e = error("bad", 123)
export msg = e.message
export code = e.code`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string, got %T", val)
	}
	if strObj.Value != "bad" {
		t.Fatalf("expected %q, got %q", "bad", strObj.Value)
	}
	val, ok = exportValue(exports, "code")
	if !ok {
		t.Fatal("expected export code to be set")
	}
	intObj, ok := val.(*object.Integer)
	if !ok {
		t.Fatalf("expected code to be integer, got %T", val)
	}
	if intObj.Value != 123 {
		t.Fatalf("expected %d, got %d", 123, intObj.Value)
	}
}

func TestVMErrorStackOnThrow(t *testing.T) {
	input := `
func b() { throw error("boom") }
func a() { b() }
try { a() } catch (e) { export msg = e.message export stack = e.stack }`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string, got %T", val)
	}
	if strObj.Value != "boom" {
		t.Fatalf("expected %q, got %q", "boom", strObj.Value)
	}
	val, ok = exportValue(exports, "stack")
	if !ok {
		t.Fatal("expected export stack to be set")
	}
	stackObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string, got %T", val)
	}
	if !strings.Contains(stackObj.Value, "a") || !strings.Contains(stackObj.Value, "b") {
		t.Fatalf("expected stack to contain function names, got %q", stackObj.Value)
	}
	if !strings.Contains(stackObj.Value, "test.wll") {
		t.Fatalf("expected stack to contain file name, got %q", stackObj.Value)
	}
}
