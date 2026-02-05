package module

import (
	"fmt"
	"os"
	"strings"

	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/parser"
	"welle/internal/vm"
)

type Loader struct {
	Resolver  *Resolver
	Cache     map[string]*compiler.Bytecode // key: abs path
	loadStack []string
	loadIndex map[string]int
}

func NewLoader(res *Resolver) *Loader {
	return &Loader{
		Resolver:  res,
		Cache:     map[string]*compiler.Bytecode{},
		loadStack: []string{},
		loadIndex: map[string]int{},
	}
}

func (l *Loader) LoadBytecode(fromFile, spec string, optimize bool) (*compiler.Bytecode, string, error) {
	path, err := l.Resolver.Resolve(fromFile, spec)
	if err != nil {
		return nil, "", err
	}

	if bc, ok := l.Cache[path]; ok {
		return bc, path, nil
	}

	if idx, ok := l.loadIndex[path]; ok {
		chain := append([]string{}, l.loadStack[idx:]...)
		chain = append(chain, path)
		return nil, "", fmt.Errorf("WM0001 import cycle: %s", strings.Join(chain, " -> "))
	}

	l.loadIndex[path] = len(l.loadStack)
	l.loadStack = append(l.loadStack, path)
	defer func() {
		delete(l.loadIndex, path)
		if len(l.loadStack) > 0 {
			l.loadStack = l.loadStack[:len(l.loadStack)-1]
		}
	}()

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	lex := lexer.New(string(src))
	p := parser.New(lex)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, "", fmt.Errorf("parse error in %s:\n%v", path, p.Errors())
	}

	if err := CheckDuplicateExports(prog, path); err != nil {
		return nil, "", err
	}

	c := compiler.NewWithFile(path)
	if err := c.Compile(prog); err != nil {
		return nil, "", fmt.Errorf("compile error in %s: %v", path, err)
	}
	bc := c.Bytecode()

	if optimize {
		opt := &compiler.Optimizer{}
		var err error
		bc, err = opt.Optimize(bc)
		if err != nil {
			return nil, "", fmt.Errorf("optimize error in %s: %v", path, err)
		}
	}

	l.Cache[path] = bc
	return bc, path, nil
}

// Create a VM that can import using this loader.
func (l *Loader) NewVM(entry *compiler.Bytecode, entryPath string) *vm.VM {
	importer := func(fromPath, spec string) (*compiler.Bytecode, string, error) {
		return l.LoadBytecode(fromPath, spec, false)
	}
	return vm.NewWithImporter(entry, entryPath, importer)
}
