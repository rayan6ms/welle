package module

import (
	"fmt"
	"os"

	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/parser"
	"welle/internal/vm"
)

type Loader struct {
	Resolver *Resolver
	Cache    map[string]*compiler.Bytecode // key: abs path
}

func NewLoader(res *Resolver) *Loader {
	return &Loader{Resolver: res, Cache: map[string]*compiler.Bytecode{}}
}

func (l *Loader) LoadBytecode(fromFile, spec string, optimize bool) (*compiler.Bytecode, string, error) {
	path, err := l.Resolver.Resolve(fromFile, spec)
	if err != nil {
		return nil, "", err
	}

	if bc, ok := l.Cache[path]; ok {
		return bc, path, nil
	}

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
