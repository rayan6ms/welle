package main

import (
	"os"
	"path/filepath"
	"testing"

	"welle/internal/spectest"
)

func TestParseExpectationStdoutExact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout_exact.test.wll")
	src := "// expect: ok\n// expect: stdout \"alpha\\n\"\nprint(\"alpha\")\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	exp, err := parseExpectation(path)
	if err != nil {
		t.Fatalf("parseExpectation failed: %v", err)
	}
	if exp.mode != expectOK {
		t.Fatalf("expected mode ok, got %v", exp.mode)
	}
	if exp.stdout.Mode != spectest.StdoutExact {
		t.Fatalf("expected stdout exact, got %v", exp.stdout.Mode)
	}
	if exp.stdout.Value != "alpha\n" {
		t.Fatalf("expected stdout value %q, got %q", "alpha\\n", exp.stdout.Value)
	}
}

func TestParseExpectationStdoutContains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout_contains.test.wll")
	src := "// EXPECT: error contains \"boom\"\n// expect: stdout contains \"part\\n\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	exp, err := parseExpectation(path)
	if err != nil {
		t.Fatalf("parseExpectation failed: %v", err)
	}
	if exp.mode != expectErrorContains {
		t.Fatalf("expected mode error contains, got %v", exp.mode)
	}
	if exp.substring != "boom" {
		t.Fatalf("expected error substring %q, got %q", "boom", exp.substring)
	}
	if exp.stdout.Mode != spectest.StdoutContains {
		t.Fatalf("expected stdout contains, got %v", exp.stdout.Mode)
	}
	if exp.stdout.Value != "part\n" {
		t.Fatalf("expected stdout value %q, got %q", "part\\n", exp.stdout.Value)
	}
}

func TestParseExpectationDuplicateStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout_dup.test.wll")
	src := "// expect: stdout \"a\\n\"\n// expect: stdout contains \"b\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := parseExpectation(path)
	if err == nil {
		t.Fatalf("expected duplicate stdout error, got nil")
	}
}

func TestRunnerStdoutExpectations(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	projectRoot := findRepoRoot(t)
	resolver, err := buildResolver(cwd, projectRoot, nil)
	if err != nil {
		t.Fatalf("buildResolver failed: %v", err)
	}

	paths := []string{
		filepath.Join(projectRoot, "tests/stdout_exact.test.wll"),
		filepath.Join(projectRoot, "tests/stdout_contains.test.wll"),
		filepath.Join(projectRoot, "tests/stdout_golden.test.wll"),
	}
	for _, useVM := range []bool{false, true} {
		for _, path := range paths {
			ok, reason := runTestFile(path, resolver, useVM)
			if !ok {
				t.Fatalf("runTestFile failed (vm=%v) for %s: %s", useVM, path, reason)
			}
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", start)
		}
		dir = parent
	}
}
