package evaluator

import (
	"strings"
	"testing"

	"welle/internal/object"
)

func dictValue(t *testing.T, obj object.Object, key string) object.Object {
	t.Helper()
	d, ok := obj.(*object.Dict)
	if !ok {
		t.Fatalf("expected dict, got %T (%v)", obj, obj)
	}
	keyObj := &object.String{Value: key}
	hk, ok := object.HashKeyOf(keyObj)
	if !ok {
		t.Fatalf("invalid key %q", key)
	}
	pair, ok := d.Pairs[object.HashKeyString(hk)]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	return pair.Value
}

func TestInterpreterErrorMembersOnThrow(t *testing.T) {
	input := `
try { throw "boom" } catch (e) {
  #{"msg": e.message, "code": e.code, "stack": e.stack}
}`
	got := testEval(t, input)
	msgObj, ok := dictValue(t, got, "msg").(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string")
	}
	if msgObj.Value != "boom" {
		t.Fatalf("expected msg %q, got %q", "boom", msgObj.Value)
	}
	codeObj, ok := dictValue(t, got, "code").(*object.Integer)
	if !ok {
		t.Fatalf("expected code to be integer")
	}
	if codeObj.Value != 0 {
		t.Fatalf("expected code 0, got %d", codeObj.Value)
	}
	stackObj, ok := dictValue(t, got, "stack").(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string")
	}
	if stackObj.Value == "" {
		t.Fatalf("expected non-empty stack")
	}
}

func TestInterpreterThrowErrorPreservesCode(t *testing.T) {
	input := `
try { throw error("x", 123) } catch (e) {
  #{"msg": e.message, "code": e.code}
}`
	got := testEval(t, input)
	msgObj, ok := dictValue(t, got, "msg").(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string")
	}
	if msgObj.Value != "x" {
		t.Fatalf("expected msg %q, got %q", "x", msgObj.Value)
	}
	codeObj, ok := dictValue(t, got, "code").(*object.Integer)
	if !ok {
		t.Fatalf("expected code to be integer")
	}
	if codeObj.Value != 123 {
		t.Fatalf("expected code 123, got %d", codeObj.Value)
	}
}

func TestInterpreterTryFinallyAndDeferStack(t *testing.T) {
	input := `
flag = ""
stack = ""
func mark() { flag = flag + "d" }
func f() { defer mark(); throw "boom" }
try { f() } catch (e) { stack = e.stack } finally { flag = flag + "f" }
#{"flag": flag, "stack": stack}`
	got := testEval(t, input)
	flagObj, ok := dictValue(t, got, "flag").(*object.String)
	if !ok {
		t.Fatalf("expected flag to be string")
	}
	if flagObj.Value != "df" {
		t.Fatalf("expected flag %q, got %q", "df", flagObj.Value)
	}
	stackObj, ok := dictValue(t, got, "stack").(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string")
	}
	if !strings.Contains(stackObj.Value, "f") {
		t.Fatalf("expected stack to mention function f, got %q", stackObj.Value)
	}
}

func TestInterpreterDeferRunsOnReturn(t *testing.T) {
	input := `
flag = ""
func mark() { flag = flag + "r" }
func g() { defer mark(); return 1 }
g()
flag`
	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected flag to be string, got %T (%v)", got, got)
	}
	if strObj.Value != "r" {
		t.Fatalf("expected flag %q, got %q", "r", strObj.Value)
	}
}

func TestInterpreterAnonymousFunctionStackName(t *testing.T) {
	input := `stack = ""
func outer() {
  f = func(x) { throw "boom" }
  try { f(1) } catch (e) { stack = e.stack }
}
outer()
stack`
	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected stack to be string, got %T", got)
	}
	if !strings.Contains(strObj.Value, "<anon@3:7>") {
		t.Fatalf("expected stack to mention anonymous function name, got %q", strObj.Value)
	}
}
