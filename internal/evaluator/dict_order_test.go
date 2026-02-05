package evaluator

import (
	"testing"

	"welle/internal/object"
)

func TestDictIterationOrder(t *testing.T) {
	input := `d = #{true: "t", false: "f", 2: "two", 1: "one", "b": "B", "a": "A"}
keys = []
for (k in d) {
  keys = append(keys, k)
}
keys`

	got := testEval(t, input)
	arr, ok := got.(*object.Array)
	if !ok {
		t.Fatalf("expected array, got %T (%v)", got, got)
	}

	assertArray(t, arr, []object.Object{
		&object.Boolean{Value: false},
		&object.Boolean{Value: true},
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.String{Value: "a"},
		&object.String{Value: "b"},
	})
}

func TestDictKeysValuesOrder(t *testing.T) {
	input := `d = #{true: "t", false: "f", 2: "two", 1: "one", "b": "B", "a": "A"}
keys(d)`
	got := testEval(t, input)
	arr, ok := got.(*object.Array)
	if !ok {
		t.Fatalf("expected array, got %T (%v)", got, got)
	}
	assertArray(t, arr, []object.Object{
		&object.Boolean{Value: false},
		&object.Boolean{Value: true},
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.String{Value: "a"},
		&object.String{Value: "b"},
	})

	input = `d = #{true: "t", false: "f", 2: "two", 1: "one", "b": "B", "a": "A"}
values(d)`
	got = testEval(t, input)
	arr, ok = got.(*object.Array)
	if !ok {
		t.Fatalf("expected array, got %T (%v)", got, got)
	}
	assertArray(t, arr, []object.Object{
		&object.String{Value: "f"},
		&object.String{Value: "t"},
		&object.String{Value: "one"},
		&object.String{Value: "two"},
		&object.String{Value: "A"},
		&object.String{Value: "B"},
	})
}

func assertArray(t *testing.T, arr *object.Array, want []object.Object) {
	t.Helper()
	if len(arr.Elements) != len(want) {
		t.Fatalf("expected array len %d, got %d", len(want), len(arr.Elements))
	}
	for i, w := range want {
		g := arr.Elements[i]
		switch w := w.(type) {
		case *object.Boolean:
			gv, ok := g.(*object.Boolean)
			if !ok || gv.Value != w.Value {
				t.Fatalf("index %d: expected bool %v, got %T (%v)", i, w.Value, g, g)
			}
		case *object.Integer:
			gv, ok := g.(*object.Integer)
			if !ok || gv.Value != w.Value {
				t.Fatalf("index %d: expected int %d, got %T (%v)", i, w.Value, g, g)
			}
		case *object.String:
			gv, ok := g.(*object.String)
			if !ok || gv.Value != w.Value {
				t.Fatalf("index %d: expected string %q, got %T (%v)", i, w.Value, g, g)
			}
		default:
			t.Fatalf("unsupported want type %T", w)
		}
	}
}
