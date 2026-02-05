package object

import "testing"

func TestMemCostBasics(t *testing.T) {
	if got := CostStringBytes(5); got != memStringHead+5 {
		t.Fatalf("CostStringBytes mismatch: got %d", got)
	}
	if got := CostArray(3); got != memArrayHead+3*memPtrSize {
		t.Fatalf("CostArray mismatch: got %d", got)
	}
	if got := CostArrayElements(3); got != 3*memPtrSize {
		t.Fatalf("CostArrayElements mismatch: got %d", got)
	}
	if got := CostTuple(2); got != memTupleHead+2*memPtrSize {
		t.Fatalf("CostTuple mismatch: got %d", got)
	}
	if got := CostDict(4); got != memDictHead+4*memDictEntry {
		t.Fatalf("CostDict mismatch: got %d", got)
	}
	if got := CostImage(2, 3); got != memImageHead+2*3*memImagePixel {
		t.Fatalf("CostImage mismatch: got %d", got)
	}
}
