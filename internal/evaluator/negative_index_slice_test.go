package evaluator

import "testing"

func TestNegativeIndexingAndSlicing(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`a=[10,20,30]; a[-1]`, "30"},
		{`a=[10,20,30,40]; a[-3:-1]`, "[20, 30]"},
		{`a=[1,2,3]; a[2:100]`, "[3]"},
		{`a=[1,2,3]; a[100:]`, "[]"},
		{`s="café"; s[-1]`, "é"},
		{`s="café"; s[-3:-1]`, "af"},
		{`s="café"; s[:100]`, "café"},
	}

	for _, tt := range tests {
		got := evalProgramInTest(t, tt.in).Inspect()
		if got != tt.want {
			t.Fatalf("input=%q expected=%q got=%q", tt.in, tt.want, got)
		}
	}
}
