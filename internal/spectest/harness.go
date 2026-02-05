package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"welle/internal/ast"
	"welle/internal/compiler"
	"welle/internal/evaluator"
	"welle/internal/lexer"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/parser"
)

type Mode string

const (
	ModeInterpreter Mode = "interp"
	ModeVM          Mode = "vm"
)

type Options struct {
	Mode      Mode
	Source    string
	Files     map[string]string
	Entry     string
	MaxMemory int64
}

type Expectation struct {
	Stdout      string
	ErrCode     string
	ErrContains string
}

type Result struct {
	Stdout  string
	ErrCode string
	ErrMsg  string
}

func Run(t *testing.T, opts Options) Result {
	t.Helper()

	entryPath, tempDir := writeFiles(t, opts)
	var res Result
	stdout, err := CaptureStdout(func() {
		res = runWithOptions(t, opts, entryPath, tempDir)
	})
	if err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}
	res.Stdout = stdout
	return res
}

func Assert(t *testing.T, res Result, exp Expectation) {
	t.Helper()

	ok, reason, err := MatchStdout(res.Stdout, StdoutExpectation{
		Mode:  StdoutExact,
		Value: exp.Stdout,
	}, "")
	if err != nil {
		t.Fatalf("stdout check failed: %v", err)
	}
	if !ok {
		t.Fatal(reason)
	}

	wantErr := exp.ErrCode != "" || exp.ErrContains != ""
	gotErr := res.ErrCode != "" || res.ErrMsg != ""

	if wantErr && !gotErr {
		t.Fatalf("expected error %q/%q, got none", exp.ErrCode, exp.ErrContains)
	}
	if !wantErr && gotErr {
		t.Fatalf("unexpected error: code=%q msg=%q", res.ErrCode, res.ErrMsg)
	}

	if exp.ErrCode != "" && res.ErrCode != exp.ErrCode {
		t.Fatalf("error code mismatch: expected %q, got %q", exp.ErrCode, res.ErrCode)
	}
	if exp.ErrContains != "" && !strings.Contains(res.ErrMsg, exp.ErrContains) {
		t.Fatalf("error message mismatch: expected to contain %q, got %q", exp.ErrContains, res.ErrMsg)
	}
}

func runWithOptions(t *testing.T, opts Options, entryPath, tempDir string) Result {
	switch opts.Mode {
	case ModeInterpreter:
		return runInterpreter(t, entryPath, tempDir, opts)
	case ModeVM:
		return runVM(t, entryPath, tempDir, opts)
	default:
		t.Fatalf("unknown mode: %q", opts.Mode)
	}
	return Result{}
}

func runInterpreter(t *testing.T, entryPath, tempDir string, opts Options) Result {
	res := Result{}

	_, parseErr := parseFile(entryPath)
	if parseErr != "" {
		res.ErrCode = "WP0001"
		res.ErrMsg = parseErr
		return res
	}

	runner := evaluator.NewRunner()
	runner.SetMaxMemory(opts.MaxMemory)
	resolver := module.NewResolver(stdRoot(t), []string{tempDir})
	runner.SetResolver(resolver)
	runner.EnableImports()

	obj := runner.RunFile(entryPath)
	if errObj, ok := obj.(*object.Error); ok {
		res.ErrMsg = errObj.Message
	}

	return res
}

func runVM(t *testing.T, entryPath, tempDir string, opts Options) Result {
	res := Result{}

	program, parseErr := parseFile(entryPath)
	if parseErr != "" {
		res.ErrCode = "WP0001"
		res.ErrMsg = parseErr
		return res
	}

	c := compiler.NewWithFile(entryPath)
	if err := c.Compile(program); err != nil {
		res.ErrMsg = err.Error()
		return res
	}
	bc := c.Bytecode()

	resolver := module.NewResolver(stdRoot(t), []string{tempDir})
	loader := module.NewLoader(resolver)
	m := loader.NewVM(bc, entryPath)
	m.SetMaxMemory(opts.MaxMemory)
	if err := m.Run(); err != nil {
		res.ErrMsg = err.Error()
	}
	return res
}

func parseFile(path string) (*ast.Program, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err.Error()
	}
	l := lexer.New(string(b))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, p.Errors()[0]
	}
	return program, ""
}

func writeFiles(t *testing.T, opts Options) (string, string) {
	t.Helper()

	entry := opts.Entry
	if entry == "" {
		entry = "main.wll"
	}
	if filepath.IsAbs(entry) {
		t.Fatalf("entry path must be relative, got %q", entry)
	}

	root := t.TempDir()
	if opts.Source != "" {
		path := filepath.Join(root, entry)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("failed to create entry dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(opts.Source), 0o644); err != nil {
			t.Fatalf("failed to write entry: %v", err)
		}
	}

	for rel, contents := range opts.Files {
		if filepath.IsAbs(rel) {
			t.Fatalf("file path must be relative, got %q", rel)
		}
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("failed to create file dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", rel, err)
		}
	}

	return filepath.Join(root, entry), root
}

func stdRoot(t *testing.T) string {
	t.Helper()

	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "std")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", start)
		}
		dir = parent
	}
}

func ExpectBoth(exp Expectation) map[Mode]Expectation {
	return map[Mode]Expectation{
		ModeInterpreter: exp,
		ModeVM:          exp,
	}
}

func Expect(mode Mode, exp Expectation) map[Mode]Expectation {
	return map[Mode]Expectation{mode: exp}
}

func FormatError(code, msg string) string {
	if code == "" {
		return msg
	}
	return fmt.Sprintf("%s: %s", code, msg)
}
