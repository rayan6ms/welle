package spectest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchStdoutExactNormalize(t *testing.T) {
	got := "a\r\nb\r\n"
	exp := StdoutExpectation{Mode: StdoutExact, Value: "a\nb\n"}
	ok, reason, err := MatchStdout(got, exp, "")
	if err != nil {
		t.Fatalf("MatchStdout error: %v", err)
	}
	if !ok {
		t.Fatalf("expected match, got mismatch: %s", reason)
	}
}

func TestMatchStdoutContains(t *testing.T) {
	got := "hello\nworld\n"
	exp := StdoutExpectation{Mode: StdoutContains, Value: "world\n"}
	ok, reason, err := MatchStdout(got, exp, "")
	if err != nil {
		t.Fatalf("MatchStdout error: %v", err)
	}
	if !ok {
		t.Fatalf("expected match, got mismatch: %s", reason)
	}
}

func TestMatchStdoutFile(t *testing.T) {
	dir := t.TempDir()
	golden := filepath.Join(dir, "golden.txt")
	if err := os.WriteFile(golden, []byte("golden\n42\n"), 0o644); err != nil {
		t.Fatalf("failed to write golden: %v", err)
	}

	got := "golden\n42\n"
	exp := StdoutExpectation{Mode: StdoutFile, Value: "golden.txt"}
	ok, reason, err := MatchStdout(got, exp, dir)
	if err != nil {
		t.Fatalf("MatchStdout error: %v", err)
	}
	if !ok {
		t.Fatalf("expected match, got mismatch: %s", reason)
	}
}
