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

	if strings.HasPrefix(spec, "std:") {
		name := strings.TrimPrefix(spec, "std:")
		if name == "" {
			return "", fmt.Errorf("invalid std import: %q", spec)
		}
		p := filepath.Join(r.StdRoot, addExt(name))
		return mustExist(p)
	}

	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || filepath.IsAbs(spec) {
		p := spec
		if !filepath.IsAbs(p) {
			base := filepath.Dir(fromFile)
			p = filepath.Join(base, p)
		}
		p = addExt(p)
		p, _ = filepath.Abs(p)
		return mustExist(p)
	}

	p := filepath.Join(r.StdRoot, addExt(spec))
	if ok, _ := exists(p); ok {
		p, _ = filepath.Abs(p)
		return p, nil
	}
	for _, root := range r.Paths {
		pp := filepath.Join(root, addExt(spec))
		if ok, _ := exists(pp); ok {
			pp, _ = filepath.Abs(pp)
			return pp, nil
		}
	}

	return "", fmt.Errorf("cannot resolve import %q from %s", spec, fromFile)
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

func mustExist(p string) (string, error) {
	ok, err := exists(p)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("module not found: %s", p)
	}
	return p, nil
}
