package vm

import (
	"testing"

	"welle/internal/object"
)

func TestVMDeferLIFO(t *testing.T) {
	input := `out = 0
func add(n) { out = out * 10 + n }
func t() {
  defer add(1)
  defer add(2)
  add(3)
}
t()
export out = out`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "out")
	if !ok {
		t.Fatal("expected export out to be set")
	}
	intObj, ok := val.(*object.Integer)
	if !ok {
		t.Fatalf("expected out to be integer, got %T", val)
	}
	if intObj.Value != 321 {
		t.Fatalf("expected %d, got %d", 321, intObj.Value)
	}
}

func TestVMDeferRunsOnThrow(t *testing.T) {
	input := `out = 0
func add(n) { out = out * 10 + n }
func t() {
  defer add(1)
  throw "boom"
}
try { t() } catch (e) { add(2) }
export out = out`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "out")
	if !ok {
		t.Fatal("expected export out to be set")
	}
	intObj, ok := val.(*object.Integer)
	if !ok {
		t.Fatalf("expected out to be integer, got %T", val)
	}
	if intObj.Value != 12 {
		t.Fatalf("expected %d, got %d", 12, intObj.Value)
	}
}

func TestVMDeferErrorPropagates(t *testing.T) {
	input := `out = 0
func add(n) { out = out * 10 + n }
func boom() { throw "defer boom" }
func t() {
  defer boom()
  add(1)
}
try { t() } catch (e) { export msg = e.message }
export out = out`

	exports, err := runVM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, ok := exportValue(exports, "out")
	if !ok {
		t.Fatal("expected export out to be set")
	}
	intObj, ok := val.(*object.Integer)
	if !ok {
		t.Fatalf("expected out to be integer, got %T", val)
	}
	if intObj.Value != 1 {
		t.Fatalf("expected %d, got %d", 1, intObj.Value)
	}
	val, ok = exportValue(exports, "msg")
	if !ok {
		t.Fatal("expected export msg to be set")
	}
	msgObj, ok := val.(*object.String)
	if !ok {
		t.Fatalf("expected msg to be string, got %T", val)
	}
	if msgObj.Value != "defer boom" {
		t.Fatalf("expected %q, got %q", "defer boom", msgObj.Value)
	}
}
