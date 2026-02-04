package lsp

import (
	"welle/internal/ast"
	"welle/internal/token"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type DocIndex struct {
	Defs    map[string]protocol.Location
	Exports map[string]protocol.Location
	Imports map[string]string
	Symbols []protocol.DocumentSymbol
}

func BuildIndex(uri string, prog *ast.Program) *DocIndex {
	ix := &DocIndex{
		Defs:    map[string]protocol.Location{},
		Exports: map[string]protocol.Location{},
		Imports: map[string]string{},
		Symbols: []protocol.DocumentSymbol{},
	}
	if prog == nil {
		return ix
	}

	nameRange := func(tok token.Token) protocol.Range {
		start := protocol.Position{Line: uint32(tok.Line - 1), Character: uint32(tok.Col - 1)}
		end := start
		l := len(tok.Literal)
		if l <= 0 {
			l = 1
		}
		end.Character = start.Character + uint32(l)
		return protocol.Range{Start: start, End: end}
	}

	fullRange := func(tok token.Token) protocol.Range {
		return nameRange(tok)
	}

	addSymbol := func(name string, tok token.Token, kind protocol.SymbolKind) {
		loc := protocol.Location{
			URI:   protocol.DocumentUri(uri),
			Range: nameRange(tok),
		}
		ix.Defs[name] = loc
		ix.Symbols = append(ix.Symbols, protocol.DocumentSymbol{
			Name:           name,
			Kind:           kind,
			Range:          fullRange(tok),
			SelectionRange: nameRange(tok),
		})
	}

	addExport := func(name string, tok token.Token, kind protocol.SymbolKind) {
		addSymbol(name, tok, kind)
		ix.Exports[name] = ix.Defs[name]
	}

	var indexStatement func(ast.Statement)
	indexStatement = func(st ast.Statement) {
		switch n := st.(type) {
		case *ast.FuncStatement:
			if n.Name == nil {
				return
			}
			addSymbol(n.Name.Value, n.Name.Token, protocol.SymbolKindFunction)
		case *ast.ImportStatement:
			if n.Alias == nil {
				return
			}
			if n.Path != nil && n.Path.Value != "" {
				ix.Imports[n.Alias.Value] = n.Path.Value
			}
			addSymbol(n.Alias.Value, n.Alias.Token, protocol.SymbolKindNamespace)
		case *ast.FromImportStatement:
			for _, item := range n.Items {
				id := item.Alias
				if id == nil {
					id = item.Name
				}
				if id == nil {
					continue
				}
				addSymbol(id.Value, id.Token, protocol.SymbolKindNamespace)
			}
		case *ast.AssignStatement:
			if n.Name == nil {
				return
			}
			addSymbol(n.Name.Value, n.Name.Token, protocol.SymbolKindVariable)
		case *ast.ExportStatement:
			if n.Stmt == nil {
				return
			}
			switch inner := n.Stmt.(type) {
			case *ast.FuncStatement:
				if inner.Name == nil {
					return
				}
				addExport(inner.Name.Value, inner.Name.Token, protocol.SymbolKindFunction)
			case *ast.AssignStatement:
				if inner.Name == nil {
					return
				}
				addExport(inner.Name.Value, inner.Name.Token, protocol.SymbolKindVariable)
			default:
				indexStatement(inner)
			}
		}
	}

	for _, st := range prog.Statements {
		indexStatement(st)
	}

	return ix
}
