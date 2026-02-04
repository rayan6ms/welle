package evaluator

import (
	"path/filepath"
	"testing"

	"welle/internal/lexer"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/parser"
)

func evalWithImports(t *testing.T, input string) object.Object {
	t.Helper()

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	stdRoot, err := filepath.Abs(filepath.Join("..", "..", "std"))
	if err != nil {
		t.Fatalf("failed to resolve std root: %v", err)
	}

	r := NewRunner()
	r.SetResolver(module.NewResolver(stdRoot, nil))
	r.EnableImports()
	return r.Eval(prog)
}

func TestRandDeterministic(t *testing.T) {
	input := `import "std:rand" as rand
rand.seed(1)
a = rand.int(10)
b = rand.int(10)
c = rand.range(5, 12)
[a, b, c]`

	got := evalWithImports(t, input)
	arr, ok := got.(*object.Array)
	if !ok {
		t.Fatalf("expected *object.Array, got %T (%v)", got, got)
	}
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr.Elements))
	}
	expected := []int64{1, 4, 10}
	for i, want := range expected {
		intObj, ok := arr.Elements[i].(*object.Integer)
		if !ok {
			t.Fatalf("expected integer at %d, got %T", i, arr.Elements[i])
		}
		if intObj.Value != want {
			t.Fatalf("expected %d at %d, got %d", want, i, intObj.Value)
		}
	}
}

func TestNoiseDeterministic(t *testing.T) {
	input := `import "std:noise" as noise
noise.noise2(10, 20, 8, 0)`

	got := evalWithImports(t, input)
	intObj, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("expected *object.Integer, got %T (%v)", got, got)
	}
	if intObj.Value != 710 {
		t.Fatalf("expected 710, got %d", intObj.Value)
	}
}
