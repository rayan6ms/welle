package evaluator

import "testing"

func TestUnicodeStringIndexAndSlice(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`s="café"; len(s)`, "4"},
		{`s="café"; s[3]`, "é"},
		{`s="café"; s[1:3]`, "af"},
		{`s="🤖ok"; len(s)`, "3"},
		{`s="🤖ok"; s[0]`, "🤖"},
		{`s="🤖ok"; s[0:2]`, "🤖o"},
	}

	for _, tt := range tests {
		got := evalProgramInTest(t, tt.in).Inspect()
		if got != tt.want {
			t.Fatalf("input=%q expected=%q got=%q", tt.in, tt.want, got)
		}
	}
}
