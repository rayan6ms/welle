package main

import (
	"strings"
	"testing"
)

func TestFormatWithMode_ASTToggle(t *testing.T) {
	input := []byte("x=1 // keep\n")

	outToken, err := formatWithMode(input, "  ", false)
	if err != nil {
		t.Fatalf("token format error: %v", err)
	}
	if strings.Contains(outToken, "// keep") {
		t.Fatalf("token formatter should drop comments, got: %q", outToken)
	}

	outAST, err := formatWithMode(input, "  ", true)
	if err != nil {
		t.Fatalf("ast format error: %v", err)
	}
	if !strings.Contains(outAST, "// keep") {
		t.Fatalf("ast formatter should preserve comments, got: %q", outAST)
	}
}
