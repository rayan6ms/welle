package vm

import (
	"testing"

	"welle/internal/object"
)

func TestVMDictIterationOrder(t *testing.T) {
	input := `d = #{true: "t", false: "f", 2: "two", 1: "one", "b": "B", "a": "A"}
iter = []
for (k in d) {
  iter = append(iter, k)
}
export iter = iter
export ks = keys(d)
export vs = values(d)`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	iterObj, ok := exportValue(exports, "iter")
	if !ok {
		t.Fatal("expected export iter")
	}
	iterArr, ok := iterObj.(*object.Array)
	if !ok {
		t.Fatalf("expected iter array, got %T (%v)", iterObj, iterObj)
	}
	assertArray(t, iterArr, []object.Object{
		&object.Boolean{Value: false},
		&object.Boolean{Value: true},
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.String{Value: "a"},
		&object.String{Value: "b"},
	})

	keysObj, ok := exportValue(exports, "ks")
	if !ok {
		t.Fatal("expected export ks")
	}
	keysArr, ok := keysObj.(*object.Array)
	if !ok {
		t.Fatalf("expected ks array, got %T (%v)", keysObj, keysObj)
	}
	assertArray(t, keysArr, []object.Object{
		&object.Boolean{Value: false},
		&object.Boolean{Value: true},
		&object.Integer{Value: 1},
		&object.Integer{Value: 2},
		&object.String{Value: "a"},
		&object.String{Value: "b"},
	})

	valsObj, ok := exportValue(exports, "vs")
	if !ok {
		t.Fatal("expected export vs")
	}
	valsArr, ok := valsObj.(*object.Array)
	if !ok {
		t.Fatalf("expected vs array, got %T (%v)", valsObj, valsObj)
	}
	assertArray(t, valsArr, []object.Object{
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
