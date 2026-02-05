package spectest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type StdoutMode int

const (
	StdoutNone StdoutMode = iota
	StdoutExact
	StdoutContains
	StdoutFile
)

type StdoutExpectation struct {
	Mode  StdoutMode
	Value string
}

var stdoutMu sync.Mutex

func CaptureStdout(run func()) (string, error) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	run()

	_ = w.Close()
	os.Stdout = oldStdout
	stdout := <-outCh
	_ = r.Close()

	return stdout, nil
}

func NormalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func MatchStdout(got string, exp StdoutExpectation, baseDir string) (bool, string, error) {
	normalizedGot := NormalizeNewlines(got)
	switch exp.Mode {
	case StdoutNone:
		return true, "", nil
	case StdoutExact:
		want := NormalizeNewlines(exp.Value)
		if normalizedGot != want {
			return false, fmt.Sprintf("stdout mismatch: expected %q, got %q", want, normalizedGot), nil
		}
		return true, "", nil
	case StdoutContains:
		want := NormalizeNewlines(exp.Value)
		if !strings.Contains(normalizedGot, want) {
			return false, fmt.Sprintf("stdout mismatch: expected to contain %q, got %q", want, normalizedGot), nil
		}
		return true, "", nil
	case StdoutFile:
		path := exp.Value
		if path == "" {
			return false, "stdout file path is empty", nil
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return false, "", err
		}
		want := NormalizeNewlines(string(b))
		if normalizedGot != want {
			return false, fmt.Sprintf("stdout mismatch: expected file %q to match, got %q", exp.Value, normalizedGot), nil
		}
		return true, "", nil
	default:
		return false, "unknown stdout expectation", nil
	}
}
