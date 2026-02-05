package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"welle/internal/evaluator"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/spectest"
)

type expectMode int

const (
	expectOK expectMode = iota
	expectError
	expectErrorContains
)

type expectation struct {
	mode        expectMode
	substring   string
	hasExplicit bool
	stdout      spectest.StdoutExpectation
	hasStdout   bool
}

func runTest(args []string) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	useVM := fs.Bool("vm", false, "run tests using bytecode VM")
	if err := fs.Parse(args); err != nil {
		fmt.Println("usage: welle test [--vm] [path|dir]...")
		os.Exit(1)
	}

	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	files, err := collectTestFiles(targets)
	if err != nil {
		fmt.Println("test error:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("no tests found")
		return
	}
	sort.Strings(files)

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	projectRoot, man, err := findManifest(cwd)
	if err != nil {
		fmt.Println("test error:", err)
		os.Exit(1)
	}
	resolver, err := buildResolver(cwd, projectRoot, man)
	if err != nil {
		fmt.Println("test error:", err)
		os.Exit(1)
	}

	passed := 0
	failed := 0
	for _, path := range files {
		ok, reason := runTestFile(path, resolver, *useVM)
		if ok {
			passed++
			continue
		}
		failed++
		fmt.Printf("FAIL %s: %s\n", path, reason)
	}
	fmt.Printf("passed %d, failed %d\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func runTestFile(path string, resolver *module.Resolver, useVM bool) (bool, string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, "invalid path"
	}
	exp, err := parseExpectation(abs)
	if err != nil {
		return false, err.Error()
	}

	var gotErr string
	stdout, err := spectest.CaptureStdout(func() {
		if useVM {
			loader := module.NewLoader(resolver)
			bc, entryPath, err := loader.LoadBytecode(abs, abs, false)
			if err != nil {
				gotErr = err.Error()
			} else {
				vm := loader.NewVM(bc, entryPath)
				if err := vm.Run(); err != nil {
					gotErr = err.Error()
				}
			}
		} else {
			runner := evaluator.NewRunner()
			runner.SetResolver(resolver)
			runner.EnableImports()
			res := runner.RunFile(abs)
			if res != nil && res.Type() == object.ERROR_OBJ {
				gotErr = res.Inspect()
			}
		}
	})
	if err != nil {
		return false, "failed to capture stdout: " + err.Error()
	}

	switch exp.mode {
	case expectOK:
		if gotErr != "" {
			return false, "expected ok, got error: " + gotErr
		}
	case expectError:
		if gotErr == "" {
			return false, "expected error, got ok"
		}
	case expectErrorContains:
		if gotErr == "" {
			return false, "expected error, got ok"
		}
		if !strings.Contains(gotErr, exp.substring) {
			return false, fmt.Sprintf("error mismatch: expected to contain %q, got %q", exp.substring, gotErr)
		}
	default:
		return false, "unknown expectation"
	}

	if exp.stdout.Mode != spectest.StdoutNone {
		ok, reason, err := spectest.MatchStdout(stdout, exp.stdout, filepath.Dir(abs))
		if err != nil {
			return false, err.Error()
		}
		if !ok {
			return false, reason
		}
	}
	return true, ""
}

func parseExpectation(path string) (*expectation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	exp := &expectation{
		mode:   expectOK,
		stdout: spectest.StdoutExpectation{Mode: spectest.StdoutNone},
	}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break
		}
		comment := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		lowerComment := strings.ToLower(comment)
		if !strings.HasPrefix(lowerComment, "expect:") {
			continue
		}
		body := strings.TrimSpace(comment[len("expect:"):])
		bodyLower := strings.ToLower(body)
		switch {
		case bodyLower == "ok":
			if exp.hasExplicit {
				return nil, fmt.Errorf("%s:%d: multiple outcome expect directives", path, lineNo)
			}
			exp.hasExplicit = true
			exp.mode = expectOK
		case bodyLower == "error":
			if exp.hasExplicit {
				return nil, fmt.Errorf("%s:%d: multiple outcome expect directives", path, lineNo)
			}
			exp.hasExplicit = true
			exp.mode = expectError
		case strings.HasPrefix(bodyLower, "error contains"):
			if exp.hasExplicit {
				return nil, fmt.Errorf("%s:%d: multiple outcome expect directives", path, lineNo)
			}
			exp.hasExplicit = true
			exp.mode = expectErrorContains
			rest := strings.TrimSpace(body[len("error contains"):])
			if rest == "" {
				return nil, fmt.Errorf("%s:%d: missing error substring", path, lineNo)
			}
			sub, err := parseQuoted(rest)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, lineNo, err)
			}
			exp.substring = sub
		case strings.HasPrefix(bodyLower, "stdout file"):
			if exp.hasStdout {
				return nil, fmt.Errorf("%s:%d: multiple stdout expect directives", path, lineNo)
			}
			exp.hasStdout = true
			rest := strings.TrimSpace(body[len("stdout file"):])
			if rest == "" {
				return nil, fmt.Errorf("%s:%d: missing stdout file path", path, lineNo)
			}
			pathVal, err := parseQuoted(rest)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, lineNo, err)
			}
			exp.stdout = spectest.StdoutExpectation{Mode: spectest.StdoutFile, Value: pathVal}
		case strings.HasPrefix(bodyLower, "stdout contains"):
			if exp.hasStdout {
				return nil, fmt.Errorf("%s:%d: multiple stdout expect directives", path, lineNo)
			}
			exp.hasStdout = true
			rest := strings.TrimSpace(body[len("stdout contains"):])
			if rest == "" {
				return nil, fmt.Errorf("%s:%d: missing stdout substring", path, lineNo)
			}
			sub, err := parseQuoted(rest)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, lineNo, err)
			}
			exp.stdout = spectest.StdoutExpectation{Mode: spectest.StdoutContains, Value: sub}
		case strings.HasPrefix(bodyLower, "stdout"):
			if exp.hasStdout {
				return nil, fmt.Errorf("%s:%d: multiple stdout expect directives", path, lineNo)
			}
			exp.hasStdout = true
			rest := strings.TrimSpace(body[len("stdout"):])
			if rest == "" {
				return nil, fmt.Errorf("%s:%d: missing stdout string", path, lineNo)
			}
			val, err := parseQuoted(rest)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, lineNo, err)
			}
			exp.stdout = spectest.StdoutExpectation{Mode: spectest.StdoutExact, Value: val}
		default:
			return nil, fmt.Errorf("%s:%d: invalid expect directive", path, lineNo)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return exp, nil
}

func parseQuoted(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("expected quoted string")
	}
	if raw[0] != '"' {
		return "", fmt.Errorf("expected quoted string")
	}
	out, err := strconv.Unquote(raw)
	if err != nil {
		return "", err
	}
	return out, nil
}

func collectTestFiles(targets []string) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isTestFile(target) {
				abs, err := filepath.Abs(target)
				if err != nil {
					return nil, err
				}
				if !seen[abs] {
					seen[abs] = true
					files = append(files, abs)
				}
			}
			continue
		}

		err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" || base == "fixtures" {
					return filepath.SkipDir
				}
				return nil
			}
			if isTestFile(path) {
				abs, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				if !seen[abs] {
					seen[abs] = true
					files = append(files, abs)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func isTestFile(path string) bool {
	if strings.HasSuffix(path, ".test.wll") {
		return true
	}
	if !strings.HasSuffix(path, ".wll") {
		return false
	}
	sep := string(os.PathSeparator)
	if strings.Contains(path, sep+"tests"+sep+"fixtures"+sep) {
		return false
	}
	if strings.Contains(path, sep+"tests"+sep) || strings.HasPrefix(path, "tests"+sep) {
		return true
	}
	return false
}
