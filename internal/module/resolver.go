package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Resolver struct {
	StdRoot string
	Paths   []string
}

type ResolveError struct {
	Spec     string
	FromFile string
	Attempts []string
}

func (e *ResolveError) Error() string {
	from := e.FromFile
	if strings.TrimSpace(from) == "" {
		from = "<unknown>"
	}
	attempts := strings.Join(e.Attempts, ", ")
	return fmt.Sprintf("missing module %q from %s (attempted: %s)", e.Spec, from, attempts)
}

func NewResolver(stdRoot string, extraPaths []string) *Resolver {
	return &Resolver{StdRoot: stdRoot, Paths: extraPaths}
}

func (r *Resolver) Resolve(fromFile string, spec string) (string, error) {
	addExt := func(p string) string {
		if filepath.Ext(p) == "" {
			return p + ".wll"
		}
		return p
	}

	attempts := []string{}
	addAttempt := func(p string) string {
		attempts = append(attempts, p)
		return p
	}

	if strings.HasPrefix(spec, "std:") {
		name := strings.TrimPrefix(spec, "std:")
		if name == "" {
			return "", fmt.Errorf("invalid std import: %q", spec)
		}
		p := filepath.Join(r.StdRoot, addExt(name))
		p = addAttempt(p)
		if ok, _ := exists(p); ok {
			p, _ = filepath.Abs(p)
			return p, nil
		}
		return "", &ResolveError{Spec: spec, FromFile: fromFile, Attempts: attempts}
	}

	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || filepath.IsAbs(spec) {
		p := spec
		if !filepath.IsAbs(p) {
			base := filepath.Dir(fromFile)
			p = filepath.Join(base, p)
		}
		p = addExt(p)
		p, _ = filepath.Abs(p)
		p = addAttempt(p)
		if ok, _ := exists(p); ok {
			return p, nil
		}
		return "", &ResolveError{Spec: spec, FromFile: fromFile, Attempts: attempts}
	}

	p := filepath.Join(r.StdRoot, addExt(spec))
	p = addAttempt(p)
	if ok, _ := exists(p); ok {
		p, _ = filepath.Abs(p)
		return p, nil
	}
	for _, root := range r.Paths {
		pp := filepath.Join(root, addExt(spec))
		addAttempt(pp)
		if ok, _ := exists(pp); ok {
			pp, _ = filepath.Abs(pp)
			return pp, nil
		}
	}

	return "", &ResolveError{Spec: spec, FromFile: fromFile, Attempts: attempts}
}

func exists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
