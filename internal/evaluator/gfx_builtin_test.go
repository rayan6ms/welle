package evaluator

import (
	"testing"

	"welle/internal/object"
)

func TestGfxBuiltinsHeadless(t *testing.T) {
	tests := []string{
		`gfx_open(100, 100, "x")`,
		`gfx_time()`,
		`gfx_beginFrame()`,
	}
	for i, input := range tests {
		got := testEval(t, input)
		errObj, ok := got.(*object.Error)
		if !ok {
			t.Fatalf("tests[%d] - expected *object.Error, got %T (%v)", i, got, got)
		}
		if errObj.Message == "" {
			t.Fatalf("tests[%d] - expected error message", i)
		}
	}
}
