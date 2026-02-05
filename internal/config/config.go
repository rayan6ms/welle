package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Manifest struct {
	Name         string
	Entry        string
	StdRoot      string
	ModulePaths  []string
	MaxRecursion int
	MaxSteps     int64
	MaxMem       int64
}

func LoadManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := &Manifest{}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}

		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s:%d: invalid line", path, lineNo)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			str, err := parseString(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			m.Name = str
		case "entry":
			str, err := parseString(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			m.Entry = str
		case "std_root":
			str, err := parseString(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			m.StdRoot = str
		case "module_paths":
			list, err := parseStringList(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			m.ModulePaths = list
		case "max_recursion":
			n, err := parseInt(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			if n < 0 {
				return nil, fmt.Errorf("%s:%d: max_recursion must be >= 0", path, lineNo)
			}
			if n > int64(^uint(0)>>1) {
				return nil, fmt.Errorf("%s:%d: max_recursion too large", path, lineNo)
			}
			m.MaxRecursion = int(n)
		case "max_steps":
			n, err := parseInt(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			if n < 0 {
				return nil, fmt.Errorf("%s:%d: max_steps must be >= 0", path, lineNo)
			}
			m.MaxSteps = n
		case "max_mem":
			n, err := parseInt(path, lineNo, val)
			if err != nil {
				return nil, err
			}
			if n < 0 {
				return nil, fmt.Errorf("%s:%d: max_mem must be >= 0", path, lineNo)
			}
			m.MaxMem = n
		default:
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manifest) ResolvePaths(projectRoot, defaultStdRoot string) (string, []string, error) {
	stdRoot := defaultStdRoot
	if m != nil && strings.TrimSpace(m.StdRoot) != "" {
		stdRoot = m.StdRoot
		if !filepath.IsAbs(stdRoot) {
			stdRoot = filepath.Join(projectRoot, stdRoot)
		}
	}
	if abs, err := filepath.Abs(stdRoot); err == nil {
		stdRoot = abs
	} else {
		return "", nil, err
	}

	modulePaths := []string{}
	if m != nil && len(m.ModulePaths) > 0 {
		modulePaths = make([]string, 0, len(m.ModulePaths))
		for _, p := range m.ModulePaths {
			if strings.TrimSpace(p) == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(projectRoot, p)
			}
			abs, err := filepath.Abs(p)
			if err != nil {
				return "", nil, err
			}
			modulePaths = append(modulePaths, abs)
		}
	}

	return stdRoot, modulePaths, nil
}

func parseString(path string, lineNo int, val string) (string, error) {
	var out string
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return "", fmt.Errorf("%s:%d: value must be a quoted string", path, lineNo)
	}
	return out, nil
}

func parseStringList(path string, lineNo int, val string) ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, fmt.Errorf("%s:%d: value must be a list of quoted strings", path, lineNo)
	}
	return out, nil
}

func parseInt(path string, lineNo int, val string) (int64, error) {
	var out int64
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return 0, fmt.Errorf("%s:%d: value must be an integer", path, lineNo)
	}
	return out, nil
}
