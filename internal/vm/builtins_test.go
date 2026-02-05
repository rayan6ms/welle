package vm

import (
	"testing"

	"welle/internal/object"
)

func TestBuiltinReverseUnicodeString(t *testing.T) {
	res := builtinReverse(&object.String{Value: "ab😊"})
	str, ok := res.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", res)
	}
	if str.Value != "😊ba" {
		t.Fatalf("unexpected reversed string: %q", str.Value)
	}
}

func TestBuiltinMeanEmptySequence(t *testing.T) {
	res := builtinMean(&object.Array{Elements: []object.Object{}})
	errObj, ok := res.(*object.Error)
	if !ok {
		t.Fatalf("expected Error, got %T", res)
	}
	if errObj.Message != "mean() arg is an empty sequence" {
		t.Fatalf("unexpected error message: %q", errObj.Message)
	}
}
