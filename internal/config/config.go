package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Manifest struct {
	Name  string
	Entry string
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

		if len(val) < 2 || val[0] != '"' || val[len(val)-1] != '"' {
			return nil, fmt.Errorf("%s:%d: value must be a quoted string", path, lineNo)
		}
		val = val[1 : len(val)-1]

		switch key {
		case "name":
			m.Name = val
		case "entry":
			m.Entry = val
		default:
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
