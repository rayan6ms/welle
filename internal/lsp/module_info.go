package lsp

import (
	"os"
	"path/filepath"

	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/parser"
)

type ModuleInfo struct {
	Exports map[string]ModuleExport
}

type ModuleExport struct {
	Name   string
	Params []string
	Kind   SymbolKind
}

func LoadModuleInfo(ws *Workspace, fromURI string, spec string) (*ModuleInfo, error) {
	if ws == nil {
		return nil, nil
	}
	fromPath := UriToPath(fromURI)
	if fromPath == "" {
		return nil, nil
	}
	resolved, err := ws.ResolveImport(fromPath, spec)
	if err != nil {
		return nil, err
	}
	resolvedAbs, _ := filepath.Abs(resolved)
	b, err := os.ReadFile(resolvedAbs)
	if err != nil {
		return nil, err
	}
	lx := lexer.New(string(b))
	p := parser.New(lx)
	prog := p.ParseProgram()
	info := &ModuleInfo{Exports: map[string]ModuleExport{}}
	if prog == nil {
		return info, nil
	}

	var addExport func(st ast.Statement)
	addExport = func(st ast.Statement) {
		switch n := st.(type) {
		case *ast.FuncStatement:
			if n.Name == nil {
				return
			}
			info.Exports[n.Name.Value] = ModuleExport{
				Name:   n.Name.Value,
				Params: paramsFromIdents(n.Parameters),
				Kind:   SymFunc,
			}
		case *ast.AssignStatement:
			if n.Name == nil {
				return
			}
			kind := SymVar
			if fl, ok := n.Value.(*ast.FunctionLiteral); ok {
				kind = SymFunc
				info.Exports[n.Name.Value] = ModuleExport{
					Name:   n.Name.Value,
					Params: paramsFromIdents(fl.Parameters),
					Kind:   kind,
				}
				return
			}
			info.Exports[n.Name.Value] = ModuleExport{
				Name: n.Name.Value,
				Kind: kind,
			}
		case *ast.ExportStatement:
			if n.Stmt != nil {
				addExport(n.Stmt)
			}
		}
	}

	for _, st := range prog.Statements {
		addExport(st)
	}

	return info, nil
}
