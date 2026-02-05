package vm

import (
	"strings"
	"testing"

	"welle/internal/object"
)

func TestVMErrorMembersOnThrow(t *testing.T) {
	input := `try { throw "boom" } catch (e) { export msg = e.message export code = e.code export stack = e.stack }`

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
	val, ok = exportValue(exports, "code")
	if !ok {
		t.Fatal("expected export code to be set")
	}
	intObj, ok := val.(*object.Integer)
	if !ok {
		t.Fatalf("expected code to be integer, got %T", val)
	}
	if intObj.Value != 0 {
		t.Fatalf("expected code 0, got %d", intObj.Value)
	}
	val, ok = exportValue(exports, "stack")
	if !ok {
		t.Fatal("expected export stack to be set")
	}
	stackObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string, got %T", val)
	}
	if stackObj.Value == "" {
		t.Fatal("expected non-empty stack")
	}
}

func TestVMThrowErrorPreservesCode(t *testing.T) {
	input := `try { throw error("x", 123) } catch (e) { export msg = e.message export code = e.code }`

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
	if strObj.Value != "x" {
		t.Fatalf("expected %q, got %q", "x", strObj.Value)
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
		t.Fatalf("expected code 123, got %d", intObj.Value)
	}
}

func TestVMTryFinallyAndDeferStack(t *testing.T) {
	input := `
flag = ""
func mark() { flag = flag + "d" }
func f() { defer mark(); throw "boom" }
try { f() } catch (e) { export stack = e.stack } finally { flag = flag + "f" }
export flag = flag`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "flag")
	if !ok {
		t.Fatal("expected export flag to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected flag to be string, got %T", val)
	}
	if strObj.Value != "df" {
		t.Fatalf("expected flag %q, got %q", "df", strObj.Value)
	}
	val, ok = exportValue(exports, "stack")
	if !ok {
		t.Fatal("expected export stack to be set")
	}
	stackObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string, got %T", val)
	}
	if !strings.Contains(stackObj.Value, "f") {
		t.Fatalf("expected stack to mention function f, got %q", stackObj.Value)
	}
}

func TestVMDeferRunsOnReturn(t *testing.T) {
	input := `
flag = ""
func mark() { flag = flag + "r" }
func g() { defer mark(); return 1 }
g()
export flag = flag`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "flag")
	if !ok {
		t.Fatal("expected export flag to be set")
	}
	strObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected flag to be string, got %T", val)
	}
	if strObj.Value != "r" {
		t.Fatalf("expected flag %q, got %q", "r", strObj.Value)
	}
}

func TestVMAnonymousFunctionStackName(t *testing.T) {
	input := `stack = ""
func outer() {
  f = func(x) { throw "boom" }
  try { f(1) } catch (e) { export stack = e.stack }
}
outer()`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "stack")
	if !ok {
		t.Fatal("expected export stack to be set")
	}
	stackObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string, got %T", val)
	}
	if !strings.Contains(stackObj.Value, "<anon@3:7>") {
		t.Fatalf("expected stack to mention anonymous function name, got %q", stackObj.Value)
	}
}
