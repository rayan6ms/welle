package vm

import (
	"testing"

	"welle/internal/object"
)

func TestVMFloatArithmetic(t *testing.T) {
	input := `export a = 1 + 2.5
export b = 5 / 2
export c = 5.0 / 2
export d = 2.0 * 3`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := exportValue(exports, "a")
	if !ok {
		t.Fatal("expected export a")
	}
	floatObj, ok := val.(*object.Float)
	if !ok || floatObj.Value != 3.5 {
		t.Fatalf("expected a=3.5 float, got %T (%v)", val, val)
	}

	val, ok = exportValue(exports, "b")
	if !ok {
		t.Fatal("expected export b")
	}
	intObj, ok := val.(*object.Integer)
	if !ok || intObj.Value != 2 {
		t.Fatalf("expected b=2 integer, got %T (%v)", val, val)
	}

	val, ok = exportValue(exports, "c")
	if !ok {
		t.Fatal("expected export c")
	}
	floatObj, ok = val.(*object.Float)
	if !ok || floatObj.Value != 2.5 {
		t.Fatalf("expected c=2.5 float, got %T (%v)", val, val)
	}

	val, ok = exportValue(exports, "d")
	if !ok {
		t.Fatal("expected export d")
	}
	floatObj, ok = val.(*object.Float)
	if !ok || floatObj.Value != 6.0 {
		t.Fatalf("expected d=6.0 float, got %T (%v)", val, val)
	}
}

func TestVMFloatComparisons(t *testing.T) {
	input := `export a = 1 == 1.0
export b = 1.5 > 1
export c = 1.5 < 2
export d = 2.0 >= 2
export e = 2.0 <= 1`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := map[string]bool{
		"a": true,
		"b": true,
		"c": true,
		"d": true,
		"e": false,
	}
	for name, want := range tests {
		val, ok := exportValue(exports, name)
		if !ok {
			t.Fatalf("expected export %s", name)
		}
		boolObj, ok := val.(*object.Boolean)
		if !ok || boolObj.Value != want {
			t.Fatalf("expected %s=%v bool, got %T (%v)", name, want, val, val)
		}
	}
}

func TestVMDivisionByZeroErrors(t *testing.T) {
	tests := []string{
		`export x = 1 / 0`,
		`export x = 1.0 / 0`,
	}
	for i, input := range tests {
		_, err := runVM(input)
		if err == nil {
			t.Fatalf("tests[%d] expected error, got nil", i)
		}
	}
}

func TestVMRangeRejectsFloat(t *testing.T) {
	_, err := runVM(`range(1.5)`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
