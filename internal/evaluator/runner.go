package evaluator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"welle/internal/ast"
	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/limits"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/parser"
	"welle/internal/token"
	"welle/internal/vm"
)

type Runner struct {
	Env          *object.Environment
	modules      map[string]*object.Dict
	baseDir      string
	resolver     *module.Resolver
	loader       *module.Loader
	loadStack    []string
	loadIndex    map[string]int
	maxRecursion int
	recursion    int
	maxMemory    int64
	budget       *limits.Budget
}

func NewRunner() *Runner {
	ctx.Budget = nil
	return &Runner{
		Env:       object.NewEnvironment(),
		modules:   map[string]*object.Dict{},
		loadStack: []string{},
		loadIndex: map[string]int{},
	}
}

func (r *Runner) SetMaxRecursion(max int) {
	if max < 0 {
		max = 0
	}
	r.maxRecursion = max
}

func (r *Runner) SetMaxMemory(max int64) {
	if max < 0 {
		max = 0
	}
	r.maxMemory = max
	r.budget = limits.NewBudget(max)
	ctx.Budget = r.budget
}

func (r *Runner) SetBudget(b *limits.Budget) {
	r.budget = b
	ctx.Budget = b
}

func (r *Runner) Eval(node ast.Node) object.Object {
	return eval(node, r.Env, r, 0, 0)
}

func (r *Runner) EnableImports() {
	if r.resolver == nil {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		stdRoot, err := filepath.Abs(filepath.Join(cwd, "std"))
		if err != nil {
			stdRoot = filepath.Join(cwd, "std")
		}
		r.resolver = module.NewResolver(stdRoot, nil)
	}
	if r.loader == nil {
		r.loader = module.NewLoader(r.resolver)
	}
	if r.baseDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			r.baseDir = cwd
		}
	}

	importResolver = func(fromFile, spec string) (string, error) {
		if fromFile == "" && r.baseDir != "" {
			fromFile = filepath.Join(r.baseDir, "repl.wll")
		}
		return r.resolver.Resolve(fromFile, spec)
	}

	importHook = func(path string) object.Object {
		return r.RunFile(path)
	}
}

func (r *Runner) SetResolver(resolver *module.Resolver) {
	r.resolver = resolver
	if resolver != nil {
		r.loader = module.NewLoader(resolver)
	}
}

func (r *Runner) RunFile(path string) object.Object {
	abs, err := filepath.Abs(path)
	if err != nil {
		return &object.Error{Message: "import/run: invalid path"}
	}

	if mod, ok := r.modules[abs]; ok {
		return mod
	}

	if idx, ok := r.loadIndex[abs]; ok {
		chain := append([]string{}, r.loadStack[idx:]...)
		chain = append(chain, abs)
		return &object.Error{Message: fmt.Sprintf("WM0001 import cycle: %s", strings.Join(chain, " -> "))}
	}

	r.loadIndex[abs] = len(r.loadStack)
	r.loadStack = append(r.loadStack, abs)
	defer func() {
		delete(r.loadIndex, abs)
		if len(r.loadStack) > 0 {
			r.loadStack = r.loadStack[:len(r.loadStack)-1]
		}
	}()

	prevFile := ctx.File
	ctx.File = abs
	defer func() { ctx.File = prevFile }()

	b, err := os.ReadFile(abs)
	if err != nil {
		return &object.Error{Message: "import/run: cannot read file: " + abs}
	}

	prev := r.baseDir
	r.baseDir = filepath.Dir(abs)
	defer func() { r.baseDir = prev }()

	l := lexer.New(string(b))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return &object.Error{Message: fmt.Sprintf("parse error in %s: %s", abs, p.Errors()[0])}
	}

	if err := module.CheckDuplicateExports(program, abs); err != nil {
		return &object.Error{Message: err.Error()}
	}

	modEnv := object.NewEnvironment()
	res := eval(program, modEnv, r, 0, 0)
	if res != nil && res.Type() == object.ERROR_OBJ {
		return res
	}

	snap := modEnv.Snapshot()
	exports := modEnv.ExportedNames()
	mod := &object.Dict{Pairs: map[string]object.DictPair{}}
	for k, v := range snap {
		if k == object.ExportSetName {
			continue
		}
		if len(exports) == 0 {
			continue
		}
		if exports[k] {
			key := &object.String{Value: k}
			hk, _ := object.HashKeyOf(key)
			mod.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: v}
		}
	}

	r.modules[abs] = mod
	return mod
}

func (r *Runner) RunFileEnv(path string) (*object.Environment, object.Object) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, &object.Error{Message: "import/run: invalid path"}
	}

	prevFile := ctx.File
	ctx.File = abs
	defer func() { ctx.File = prevFile }()

	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, &object.Error{Message: "import/run: cannot read file: " + abs}
	}

	prev := r.baseDir
	r.baseDir = filepath.Dir(abs)
	defer func() { r.baseDir = prev }()

	l := lexer.New(string(b))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, &object.Error{Message: fmt.Sprintf("parse error in %s: %s", abs, p.Errors()[0])}
	}

	if err := module.CheckDuplicateExports(program, abs); err != nil {
		return nil, &object.Error{Message: err.Error()}
	}

	modEnv := object.NewEnvironment()
	res := eval(program, modEnv, r, 0, 0)
	if res != nil && res.Type() == object.ERROR_OBJ {
		return modEnv, res
	}
	return modEnv, res
}

func (r *Runner) Call(fn object.Object, args ...object.Object) object.Object {
	return applyFunction(token.Token{Literal: "<gfx>", Line: 1, Col: 1}, fn, args, r)
}

func (r *Runner) LoadModuleVMAsObject(path string) (object.Object, error) {
	mod, err := r.LoadModuleVM(path)
	if err != nil {
		return nil, err
	}
	return mod, nil
}

func (r *Runner) LoadModuleVM(path string) (*object.Dict, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("import/run: invalid path")
	}

	if mod, ok := r.modules[abs]; ok {
		return mod, nil
	}

	if r.loader == nil {
		if r.resolver == nil {
			r.EnableImports()
		} else {
			r.loader = module.NewLoader(r.resolver)
		}
	}
	bc, absPath, err := r.loader.LoadBytecode(abs, abs, false)
	if err != nil {
		return nil, err
	}

	importer := func(fromPath, spec string) (*compiler.Bytecode, string, error) {
		return r.loader.LoadBytecode(fromPath, spec, false)
	}
	mvm := vm.NewWithImporter(bc, absPath, importer)
	if r.budget != nil {
		mvm.SetBudget(r.budget)
	}
	mvm.SetModuleCache(r.modules)
	if err := mvm.Run(); err != nil {
		return nil, fmt.Errorf("vm error in %s: %v", absPath, err)
	}

	exports := mvm.Exports()
	r.modules[absPath] = exports
	return exports, nil
}
