package vm

import "testing"

func TestVMGfxBuiltinsHeadless(t *testing.T) {
	input := `gfx_time()`

	_, err := runVM(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
