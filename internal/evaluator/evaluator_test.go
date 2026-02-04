package evaluator

import (
	"fmt"
	"strings"
	"testing"

	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
)

func TestErrorHasLocation(t *testing.T) {
	input := "x = 10\nz\n"

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	got := Eval(prog, env)

	errObj, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%v)", got, got)
	}
	if !strings.Contains(errObj.Stack, "<unknown>:2:1") {
		t.Fatalf("expected stack to contain %q, got %q", "<unknown>:2:1", errObj.Stack)
	}
}

func TestDictLiteralAndIndexing(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`m = #{"a": 1, "b": 2}
m["a"]`, 1},
		{`#{"x": 10}["x"]`, 10},
		{`m = #{"a": 1, 1: 2, true: 3}
m[1]`, 2},
		{`m = #{"a": 1, 1: 2, true: 3}
m[true]`, 3},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		intObj, ok := got.(*object.Integer)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Integer, got %T (%v)", i, got, got)
		}
		if intObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %d, got %d", i, tt.want, intObj.Value)
		}
	}
}

func TestDictErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`#{"a": 1, [1]: 3}`, "unusable as dict key: ARRAY"},
		{`m = #{"a": 1}
m[[1]]`, "unusable as dict key: ARRAY"},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Error, got %T (%v)", i, got, got)
		}
		if errObj.Message != tt.want {
			t.Fatalf("tests[%d] - expected %q, got %q", i, tt.want, errObj.Message)
		}
	}
}

func TestDictMissingKeyReturnsNil(t *testing.T) {
	got := testEval(t, `m = #{"a": 1}
m["b"]`)
	if got.Type() != object.NIL_OBJ {
		t.Fatalf("expected nil, got %T (%v)", got, got)
	}
}

func TestIfNilIsFalsy(t *testing.T) {
	got := testEval(t, `if (nil) { 1 } else { 2 }`)
	intObj, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("expected *object.Integer, got %T (%v)", got, got)
	}
	if intObj.Value != 2 {
		t.Fatalf("expected 2, got %d", intObj.Value)
	}
}

func TestForLoop_ArrayAndDict(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`nums = [1, 2, 3]
sum = 0
for (x in nums) {
  sum = sum + x
}
sum`, 6},
		{`m = #{"a": 1, "b": 2}
count = 0
for (k in m) {
  count = count + 1
}
count`, 2},
		{`sum = 0
for x in range(5) {
  sum = sum + x
}
sum`, 10},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		intObj, ok := got.(*object.Integer)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Integer, got %T (%v)", i, got, got)
		}
		if intObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %d, got %d", i, tt.want, intObj.Value)
		}
	}
}

func TestRangeBuiltin(t *testing.T) {
	tests := []struct {
		input string
		want  []int64
	}{
		{`range(5)`, []int64{0, 1, 2, 3, 4}},
		{`range(2, 6)`, []int64{2, 3, 4, 5}},
		{`range(10, 0, -3)`, []int64{10, 7, 4, 1}},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		arr, ok := got.(*object.Array)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Array, got %T (%v)", i, got, got)
		}
		if len(arr.Elements) != len(tt.want) {
			t.Fatalf("tests[%d] - expected len %d, got %d", i, len(tt.want), len(arr.Elements))
		}
		for j, want := range tt.want {
			intObj, ok := arr.Elements[j].(*object.Integer)
			if !ok {
				t.Fatalf("tests[%d] - expected *object.Integer at %d, got %T (%v)", i, j, arr.Elements[j], arr.Elements[j])
			}
			if intObj.Value != want {
				t.Fatalf("tests[%d] - expected %d at %d, got %d", i, want, j, intObj.Value)
			}
		}
	}
}

func TestRangeBuiltinErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`range()`, "wrong number of arguments: expected 1, 2, or 3, got 0"},
		{`range(1, 2, 3, 4)`, "wrong number of arguments: expected 1, 2, or 3, got 4"},
		{`range("a")`, "range() expects INTEGER arguments"},
		{`range(1, "b")`, "range() expects INTEGER arguments"},
		{`range(1, 2, "c")`, "range() expects INTEGER arguments"},
		{`range(1, 2, 0)`, "range() step cannot be 0"},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Error, got %T (%v)", i, got, got)
		}
		if errObj.Message != tt.want {
			t.Fatalf("tests[%d] - expected %q, got %q", i, tt.want, errObj.Message)
		}
	}
}

func TestBreakAndContinue(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`i = 0
while (true) {
  i = i + 1
  if (i == 3) { break }
}
i`, 3},
		{`sum = 0
for (x in [1, 2, 3, 4, 5]) {
  if (x % 2 == 0) { continue }
  sum = sum + x
}
sum`, 9},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		intObj, ok := got.(*object.Integer)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Integer, got %T (%v)", i, got, got)
		}
		if intObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %d, got %d", i, tt.want, intObj.Value)
		}
	}
}

func TestSwitchStatement(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`x = 2
out = ""
switch (x) {
  case 1 { out = "one" }
  case 2 { out = "two" }
  default { out = "other" }
}
out`, "two"},
		{`x = 2
out = ""
switch (x) {
  case 1, 2, 3 { out = "small" }
  default { out = "other" }
}
out`, "small"},
		{`s = "b"
out = ""
switch (s) {
  case "a", "b" { out = "hit" }
  default { out = "miss" }
}
out`, "hit"},
		{`x = 9
out = ""
switch (x) {
  case 1 { out = "one" }
  default { out = "other" }
}
out`, "other"},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		strObj, ok := got.(*object.String)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.String, got %T (%v)", i, got, got)
		}
		if strObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %q, got %q", i, tt.want, strObj.Value)
		}
	}
}

func TestFloatArithmetic(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{`1 + 2.5`, 3.5},
		{`2.0 * 3`, 6.0},
		{`5.0 / 2`, 2.5},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		floatObj, ok := got.(*object.Float)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Float, got %T (%v)", i, got, got)
		}
		if floatObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %v, got %v", i, tt.want, floatObj.Value)
		}
	}
}

func TestFloatComparisons(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`1 == 1.0`, true},
		{`1 != 1.5`, true},
		{`1.5 > 1`, true},
		{`1.5 < 2`, true},
		{`2.0 >= 2`, true},
		{`2.0 <= 1`, false},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		boolObj, ok := got.(*object.Boolean)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Boolean, got %T (%v)", i, got, got)
		}
		if boolObj.Value != tt.want {
			t.Fatalf("tests[%d] - expected %v, got %v", i, tt.want, boolObj.Value)
		}
	}
}

func TestDivisionByZeroErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`1 / 0`, "division by zero"},
		{`1.0 / 0`, "division by zero"},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Error, got %T (%v)", i, got, got)
		}
		if errObj.Message != tt.want {
			t.Fatalf("tests[%d] - expected %q, got %q", i, tt.want, errObj.Message)
		}
	}
}

func TestRangeRejectsFloat(t *testing.T) {
	got := testEval(t, `range(1.5)`)
	errObj, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%v)", got, got)
	}
	if errObj.Message != "range() expects INTEGER arguments" {
		t.Fatalf("expected %q, got %q", "range() expects INTEGER arguments", errObj.Message)
	}
}

func TestDeferLIFO(t *testing.T) {
	input := `x = 0
func add(n) { x = x + n }
func f() {
  defer add(1)
  defer add(10)
  return 1
}
f()
x`

	got := testEval(t, input)
	intObj, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("expected *object.Integer, got %T (%v)", got, got)
	}
	if intObj.Value != 11 {
		t.Fatalf("expected 11, got %d", intObj.Value)
	}
}

func TestDeferRunsOnThrow(t *testing.T) {
	input := `out = ""
func add(s) { out = out + s }
func f() {
  defer add("cleanup")
  throw "boom"
}
try { f() } catch (e) { add("caught") }
out`

	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected *object.String, got %T (%v)", got, got)
	}
	if strObj.Value != "cleanupcaught" {
		t.Fatalf("expected %q, got %q", "cleanupcaught", strObj.Value)
	}
}

func TestTryFinallyNoError(t *testing.T) {
	input := `out = ""
try { out = out + "try" } catch (e) { out = out + "catch" } finally { out = out + "finally" }
out`

	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected *object.String, got %T (%v)", got, got)
	}
	if strObj.Value != "tryfinally" {
		t.Fatalf("expected %q, got %q", "tryfinally", strObj.Value)
	}
}

func TestTryFinallyCaught(t *testing.T) {
	input := `out = ""
try { 10 / 0 } catch (e) { out = out + "caught" } finally { out = out + "finally" }
out`

	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected *object.String, got %T (%v)", got, got)
	}
	if strObj.Value != "caughtfinally" {
		t.Fatalf("expected %q, got %q", "caughtfinally", strObj.Value)
	}
}

func TestTryFinallyUncaught(t *testing.T) {
	input := `out = ""
try {
  try { 10 / 0 } finally { out = out + "finally" }
} catch (e) { out = out + "caught" }
out`

	got := testEval(t, input)
	strObj, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected *object.String, got %T (%v)", got, got)
	}
	if strObj.Value != "finallycaught" {
		t.Fatalf("expected %q, got %q", "finallycaught", strObj.Value)
	}
}

func TestTryFinallyErrorWins(t *testing.T) {
	input := `try { 10 / 0 } catch (e) { out = "caught" } finally { 1 / 0 }`

	got := testEval(t, input)
	errObj, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%v)", got, got)
	}
	if errObj.Message != "division by zero" {
		t.Fatalf("expected %q, got %q", "division by zero", errObj.Message)
	}
}

func TestDeferOutsideFunction(t *testing.T) {
	input := `defer print("x")`

	got := testEval(t, input)
	errObj, ok := got.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%v)", got, got)
	}
	if errObj.Message != "defer used outside of a function" {
		t.Fatalf("expected %q, got %q", "defer used outside of a function", errObj.Message)
	}
}

func TestBreakContinueOutsideLoop(t *testing.T) {
	tests := []struct {
		input string
		want  string
		line  int
		col   int
	}{
		{"break", "break used outside of a loop or switch", 1, 1},
		{"continue", "continue used outside of a loop", 1, 1},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Error, got %T (%v)", i, got, got)
		}
		if errObj.Message != tt.want {
			t.Fatalf("tests[%d] - expected %q, got %q", i, tt.want, errObj.Message)
		}
		loc := fmt.Sprintf("<unknown>:%d:%d", tt.line, tt.col)
		if !strings.Contains(errObj.Stack, loc) {
			t.Fatalf("tests[%d] - expected stack to contain %q, got %q", i, loc, errObj.Stack)
		}
	}
}

func TestMemberCalls(t *testing.T) {
	tests := []struct {
		input string
		want  interface{}
	}{
		{`a = [1, 2]
a = a.append(3)
a.len()`, int64(3)},
		{`m = #{"a": 1}
m.hasKey("a")`, true},
		{`m = #{"a": 1}
m.hasKey("b")`, false},
	}

	for i, tt := range tests {
		got := testEval(t, tt.input)
		switch want := tt.want.(type) {
		case int64:
			intObj, ok := got.(*object.Integer)
			if !ok {
				t.Fatalf("tests[%d] - expected *object.Integer, got %T (%v)", i, got, got)
			}
			if intObj.Value != want {
				t.Fatalf("tests[%d] - expected %d, got %d", i, want, intObj.Value)
			}
		case bool:
			boolObj, ok := got.(*object.Boolean)
			if !ok {
				t.Fatalf("tests[%d] - expected *object.Boolean, got %T (%v)", i, got, got)
			}
			if boolObj.Value != want {
				t.Fatalf("tests[%d] - expected %v, got %v", i, want, boolObj.Value)
			}
		default:
			t.Fatalf("tests[%d] - unsupported want type %T", i, tt.want)
		}
	}
}

func testEval(t *testing.T, input string) object.Object {
	t.Helper()

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	return Eval(prog, env)
}
