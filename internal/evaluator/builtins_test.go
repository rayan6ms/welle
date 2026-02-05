package evaluator

import (
	"testing"

	"welle/internal/object"
)

func TestBuiltinMaxEmptySequence(t *testing.T) {
	fn := builtins["max"]
	if fn == nil {
		t.Fatal("missing max builtin")
	}

	res := fn.Fn(&object.Array{Elements: []object.Object{}})
	errObj, ok := res.(*object.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", res)
	}
	if errObj.Message != "max() arg is an empty sequence" {
		t.Fatalf("unexpected error message: %q", errObj.Message)
	}
}

func TestBuiltinMeanEmptySequence(t *testing.T) {
	fn := builtins["mean"]
	if fn == nil {
		t.Fatal("missing mean builtin")
	}

	res := fn.Fn(&object.Array{Elements: []object.Object{}})
	errObj, ok := res.(*object.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", res)
	}
	if errObj.Message != "mean() arg is an empty sequence" {
		t.Fatalf("unexpected error message: %q", errObj.Message)
	}
}
