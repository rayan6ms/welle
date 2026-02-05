package semantics_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"welle/internal/compiler"
	"welle/internal/evaluator"
	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
	"welle/internal/vm"
)

type runResult struct {
	exports *object.Dict
	errMsg  string
}

func captureRun(run func() runResult) (runResult, string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return runResult{}, "", err
	}
	os.Stdout = w

	res := run()

	_ = w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		return res, "", err
	}
	return res, string(out), nil
}

func runInterpreter(input string) runResult {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return runResult{errMsg: fmt.Sprintf("parse errors: %s", strings.Join(p.Errors(), "; "))}
	}

	env := object.NewEnvironment()
	res := evaluator.Eval(program, env)
	if errObj, ok := res.(*object.Error); ok {
		return runResult{errMsg: errObj.Message}
	}

	exports := snapshotExports(env)
	return runResult{exports: exports}
}

func runVM(input string) runResult {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return runResult{errMsg: fmt.Sprintf("parse errors: %s", strings.Join(p.Errors(), "; "))}
	}

	c := compiler.NewWithFile("parity.wll")
	if err := c.Compile(program); err != nil {
		return runResult{errMsg: err.Error()}
	}
	bc := c.Bytecode()

	m := vm.New(bc)
	if err := m.Run(); err != nil {
		return runResult{exports: m.Exports(), errMsg: vmErrorMessage(err)}
	}
	return runResult{exports: m.Exports()}
}

func vmErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "error: ") {
		line := strings.SplitN(msg, "\n", 2)[0]
		return strings.TrimPrefix(line, "error: ")
	}
	return msg
}

func snapshotExports(env *object.Environment) *object.Dict {
	snap := env.Snapshot()
	exports := env.ExportedNames()
	out := &object.Dict{Pairs: map[string]object.DictPair{}}
	for k, v := range snap {
		if k == object.ExportSetName {
			continue
		}
		if len(exports) == 0 || !exports[k] {
			continue
		}
		key := &object.String{Value: k}
		hk, _ := object.HashKeyOf(key)
		out.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: v}
	}
	return out
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

func assertExport(t *testing.T, exports *object.Dict, name string, wantType object.Type, wantInspect string) {
	t.Helper()
	val, ok := exportValue(exports, name)
	if !ok {
		t.Fatalf("expected export %s", name)
	}
	if val.Type() != wantType {
		t.Fatalf("expected %s to be %s, got %s", name, wantType, val.Type())
	}
	if val.Inspect() != wantInspect {
		t.Fatalf("expected %s=%q, got %q", name, wantInspect, val.Inspect())
	}
}

func assertParity(t *testing.T, input string, expected map[string]struct {
	typ     object.Type
	inspect string
}) {
	t.Helper()
	intRes, intOut, err := captureRun(func() runResult { return runInterpreter(input) })
	if err != nil {
		t.Fatalf("interpreter capture error: %v", err)
	}
	vmRes, vmOut, err := captureRun(func() runResult { return runVM(input) })
	if err != nil {
		t.Fatalf("vm capture error: %v", err)
	}
	if intOut != vmOut {
		t.Fatalf("stdout mismatch: interpreter %q, vm %q", intOut, vmOut)
	}
	if intRes.errMsg != vmRes.errMsg {
		t.Fatalf("error mismatch: interpreter %q, vm %q", intRes.errMsg, vmRes.errMsg)
	}
	if intRes.errMsg != "" {
		return
	}
	for name, exp := range expected {
		assertExport(t, intRes.exports, name, exp.typ, exp.inspect)
		assertExport(t, vmRes.exports, name, exp.typ, exp.inspect)
	}
}

func TestSemanticsParity_Values(t *testing.T) {
	input := `export t0 = not false
export t1 = not nil
export t2 = not 0
export t3 = not ""
export b0 = !false
export b1 = !nil
export b2 = !0
export b3 = !""
export eq1 = 1 == 1.0
export neq1 = 1 != 1.5
export gt1 = 1.5 > 1
export lt1 = 1.5 < 2
export gte1 = 2.0 >= 2
export lte1 = 2.0 <= 1
export add = 1 + 2.5
export divi = 5 / 2
export divf = 5.0 / 2
export str = "a" + "b"`

	expected := map[string]struct {
		typ     object.Type
		inspect string
	}{
		"t0":   {object.BOOLEAN_OBJ, "true"},
		"t1":   {object.BOOLEAN_OBJ, "true"},
		"t2":   {object.BOOLEAN_OBJ, "false"},
		"t3":   {object.BOOLEAN_OBJ, "false"},
		"b0":   {object.BOOLEAN_OBJ, "true"},
		"b1":   {object.BOOLEAN_OBJ, "true"},
		"b2":   {object.BOOLEAN_OBJ, "false"},
		"b3":   {object.BOOLEAN_OBJ, "false"},
		"eq1":  {object.BOOLEAN_OBJ, "true"},
		"neq1": {object.BOOLEAN_OBJ, "true"},
		"gt1":  {object.BOOLEAN_OBJ, "true"},
		"lt1":  {object.BOOLEAN_OBJ, "true"},
		"gte1": {object.BOOLEAN_OBJ, "true"},
		"lte1": {object.BOOLEAN_OBJ, "false"},
		"add":  {object.FLOAT_OBJ, "3.5"},
		"divi": {object.INTEGER_OBJ, "2"},
		"divf": {object.FLOAT_OBJ, "2.5"},
		"str":  {object.STRING_OBJ, "ab"},
	}

	assertParity(t, input, expected)
}

func TestSemanticsParity_Re24Features(t *testing.T) {
	input := `export in_arr = 2 in [1, 2, 3]
export in_str = "ell" in "hello"
export in_dict = "a" in #{"a": 1}
export sqrt_val = sqrt(9)
export sqrt_eq = sqrt(9) == math_sqrt(9)
try { input() } catch (e) { }
try { getpass("pw: ") } catch (e) { }
export io_done = true`

	expected := map[string]struct {
		typ     object.Type
		inspect string
	}{
		"in_arr":   {object.BOOLEAN_OBJ, "true"},
		"in_str":   {object.BOOLEAN_OBJ, "true"},
		"in_dict":  {object.BOOLEAN_OBJ, "true"},
		"sqrt_val": {object.FLOAT_OBJ, "3"},
		"sqrt_eq":  {object.BOOLEAN_OBJ, "true"},
		"io_done":  {object.BOOLEAN_OBJ, "true"},
	}

	assertParity(t, input, expected)
}

func TestSemanticsParity_Bitwise(t *testing.T) {
	input := `export orv = 5 | 2
export andv = 5 & 2
export xorv = 5 ^ 2
export notv = ~0
export shl = 1 << 3
export shr = 8 >> 2
print(5 | 2)
print(5 & 2)
`

	expected := map[string]struct {
		typ     object.Type
		inspect string
	}{
		"orv":  {object.INTEGER_OBJ, "7"},
		"andv": {object.INTEGER_OBJ, "0"},
		"xorv": {object.INTEGER_OBJ, "7"},
		"notv": {object.INTEGER_OBJ, "-1"},
		"shl":  {object.INTEGER_OBJ, "8"},
		"shr":  {object.INTEGER_OBJ, "2"},
	}

	assertParity(t, input, expected)
}

func TestSemanticsParity_BitwiseErrors(t *testing.T) {
	tests := []string{
		"print(1 << 64)\n",
		"print(1 >> -1)\n",
		"print(1 | 1.0)\n",
		"print(~1.5)\n",
	}

	for i, input := range tests {
		intRes, intOut, err := captureRun(func() runResult { return runInterpreter(input) })
		if err != nil {
			t.Fatalf("case %d interpreter capture error: %v", i, err)
		}
		vmRes, vmOut, err := captureRun(func() runResult { return runVM(input) })
		if err != nil {
			t.Fatalf("case %d vm capture error: %v", i, err)
		}
		if intOut != vmOut {
			t.Fatalf("case %d stdout mismatch: interpreter %q, vm %q", i, intOut, vmOut)
		}
		if intRes.errMsg != vmRes.errMsg {
			t.Fatalf("case %d error mismatch: interpreter %q, vm %q", i, intRes.errMsg, vmRes.errMsg)
		}
		if intRes.errMsg == "" {
			t.Fatalf("case %d expected error, got none", i)
		}
	}
}

func TestSemanticsParity_FunctionLiterals(t *testing.T) {
	input := `export basic = func(x) { return x + 1 }(2)
func makeAdder(x) {
  return func(y) { return x + y }
}
add2 = makeAdder(2)
export closure = add2(5)
export immediate = (func(x) { return x * 2 })(21)
a = [func(x) { return x + 1 }, func(x) { return x + 2 }]
export nested = a[0](10) + a[1](10)`

	expected := map[string]struct {
		typ     object.Type
		inspect string
	}{
		"basic":     {object.INTEGER_OBJ, "3"},
		"closure":   {object.INTEGER_OBJ, "7"},
		"immediate": {object.INTEGER_OBJ, "42"},
		"nested":    {object.INTEGER_OBJ, "23"},
	}

	assertParity(t, input, expected)
}

func TestSemanticsParity_CallSpread(t *testing.T) {
	input := `func f(a, b, c, d) { return a + b + c + d }
t = (1, 2)
export sum = f(0, ...t, 3)
a = [4, 5]
export sum2 = f(1, ...a, 2)`

	expected := map[string]struct {
		typ     object.Type
		inspect string
	}{
		"sum":  {object.INTEGER_OBJ, "6"},
		"sum2": {object.INTEGER_OBJ, "12"},
	}

	assertParity(t, input, expected)
}

func TestSemanticsParity_Errors(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{`export x = 1 < "a"`, "type mismatch: INTEGER < STRING"},
		{`export x = "a" < "b"`, "unknown operator for strings: <"},
		{`export x = true < false`, "unknown operator for booleans: <"},
		{`export x = nil > 1`, "cannot compare nil with INTEGER using >"},
		{`export x = 1.0 % 2`, "modulo requires INTEGER operands"},
		{`export x = 1 / 0`, "division by zero"},
		{`export x = 1 % 0`, "modulo by zero"},
		{`export x = nil + nil`, "invalid operator for nil: +"},
		{`export x = func(a) { return a }(...1)`, "cannot spread INTEGER in call arguments"},
	}
	for i, tt := range tests {
		intRes, intOut, err := captureRun(func() runResult { return runInterpreter(tt.input) })
		if err != nil {
			t.Fatalf("tests[%d] interpreter capture error: %v", i, err)
		}
		vmRes, vmOut, err := captureRun(func() runResult { return runVM(tt.input) })
		if err != nil {
			t.Fatalf("tests[%d] vm capture error: %v", i, err)
		}
		if intOut != vmOut {
			t.Fatalf("tests[%d] stdout mismatch: interpreter %q, vm %q", i, intOut, vmOut)
		}
		if intRes.errMsg != vmRes.errMsg {
			t.Fatalf("tests[%d] error mismatch: interpreter %q, vm %q", i, intRes.errMsg, vmRes.errMsg)
		}
		if intRes.errMsg != tt.wantErr {
			t.Fatalf("tests[%d] expected error %q, got %q", i, tt.wantErr, intRes.errMsg)
		}
	}
}
