package lsp

import (
	"fmt"
	"os"
	"path/filepath"

	"welle/internal/ast"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type SymbolKeyKind int

const (
	SymKeyLocal SymbolKeyKind = iota
	SymKeyExport
)

type SymbolKey struct {
	Kind       SymbolKeyKind
	Name       string
	URI        string
	ScopeStart Pos
	ScopeEnd   Pos
	ModulePath string
}

type OccurrenceKind int

const (
	OccurrenceDecl OccurrenceKind = iota
	OccurrenceRef
	OccurrenceImportName
	OccurrenceAliasUse
)

type Occurrence struct {
	URI   string
	Range protocol.Range
	Kind  OccurrenceKind
}

type WorkspaceIndex struct {
	ByKey map[SymbolKey][]Occurrence
}

type importBindingInfo struct {
	Spec      string
	Member    string
	AliasUsed bool
}

type importNameOccurrence struct {
	Spec   string
	Member string
	Ident  *ast.Identifier
}

func BuildWorkspaceIndex(ws *Workspace) (*WorkspaceIndex, error) {
	idx := &WorkspaceIndex{ByKey: map[SymbolKey][]Occurrence{}}
	if ws == nil {
		return idx, nil
	}
	files, err := ws.WorkspaceFiles()
	if err != nil {
		return idx, err
	}
	seen := map[string]bool{}
	addFile := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	for _, pth := range ws.OpenDocPaths() {
		addFile(pth)
	}

	for _, pth := range files {
		absPath, _ := filepath.Abs(pth)
		uri := PathToURI(absPath)
		text, ok := ws.TextForPath(absPath)
		if !ok {
			b, err := os.ReadFile(absPath)
			if err != nil {
				continue
			}
			text = string(b)
		}
		docOcc := buildDocOccurrences(ws, uri, absPath, text)
		for key, occs := range docOcc {
			idx.ByKey[key] = append(idx.ByKey[key], occs...)
		}
	}
	return idx, nil
}

func buildDocOccurrences(ws *Workspace, uri string, absPath string, text string) map[SymbolKey][]Occurrence {
	out := map[SymbolKey][]Occurrence{}
	an, _ := Analyze(text)
	if an == nil || an.Program == nil {
		return out
	}

	exports := exportedNames(an.Program)
	importInfo, importNames := collectFromImportInfo(an.Program)

	modulePath := ""
	if absPath != "" {
		modulePath, _ = filepath.Abs(absPath)
	}
	resolve := func(spec string) (string, bool) {
		if ws == nil || spec == "" || absPath == "" {
			return "", false
		}
		resolved, err := ws.ResolveImport(absPath, spec)
		if err != nil {
			return "", false
		}
		resolvedAbs, _ := filepath.Abs(resolved)
		return resolvedAbs, true
	}

	seenRanges := map[SymbolKey]map[string]bool{}
	addOcc := func(key SymbolKey, occ Occurrence) {
		if key.Name == "" {
			return
		}
		if _, ok := seenRanges[key]; !ok {
			seenRanges[key] = map[string]bool{}
		}
		rk := rangeKey(occ.Range, occ.URI)
		if seenRanges[key][rk] {
			return
		}
		seenRanges[key][rk] = true
		out[key] = append(out[key], occ)
	}

	for _, item := range importNames {
		resolved, ok := resolve(item.Spec)
		if !ok {
			continue
		}
		key := SymbolKey{Kind: SymKeyExport, ModulePath: resolved, Name: item.Member}
		r := rangeFromPosLenUTF16(text, item.Ident.Token.Line, item.Ident.Token.Col, identText(item.Ident))
		addOcc(key, Occurrence{URI: uri, Range: r, Kind: OccurrenceImportName})
	}

	bindingKeys := map[*Binding]SymbolKey{}
	for _, b := range an.Defs {
		if b == nil {
			continue
		}
		if b.Kind == SymImport {
			info, ok := importInfo[b.Decl]
			if !ok {
				continue
			}
			resolved, ok := resolve(info.Spec)
			if !ok {
				continue
			}
			key := SymbolKey{Kind: SymKeyExport, ModulePath: resolved, Name: info.Member}
			bindingKeys[b] = key
			if info.AliasUsed && b.Decl != nil {
				r := rangeFromPosLenUTF16(text, b.Decl.Token.Line, b.Decl.Token.Col, identText(b.Decl))
				addOcc(key, Occurrence{URI: uri, Range: r, Kind: OccurrenceAliasUse})
			}
			continue
		}
		key, ok := keyForBinding(uri, modulePath, exports, an.Root, b)
		if !ok {
			continue
		}
		bindingKeys[b] = key
		if b.Decl != nil {
			r := rangeFromPosLenUTF16(text, b.Decl.Token.Line, b.Decl.Token.Col, identText(b.Decl))
			addOcc(key, Occurrence{URI: uri, Range: r, Kind: OccurrenceDecl})
		}
	}

	for _, r := range an.Refs {
		if r == nil || r.Ident == nil {
			continue
		}
		switch r.Kind {
		case SymModuleMember:
			resolved, ok := resolve(r.ModulePath)
			if !ok {
				continue
			}
			key := SymbolKey{Kind: SymKeyExport, ModulePath: resolved, Name: r.Member}
			rng := rangeFromPosLenUTF16(text, r.Ident.Token.Line, r.Ident.Token.Col, identText(r.Ident))
			addOcc(key, Occurrence{URI: uri, Range: rng, Kind: OccurrenceRef})
		case SymBuiltin, SymKeyword:
			continue
		default:
			if r.Binding == nil {
				continue
			}
			key, ok := bindingKeys[r.Binding]
			if !ok {
				key, ok = keyForBinding(uri, modulePath, exports, an.Root, r.Binding)
				if ok {
					bindingKeys[r.Binding] = key
				}
			}
			if !ok {
				continue
			}
			kind := OccurrenceRef
			if r.Binding.Kind == SymImport {
				if info, ok := importInfo[r.Binding.Decl]; ok && info.AliasUsed {
					kind = OccurrenceAliasUse
				}
			}
			rng := rangeFromPosLenUTF16(text, r.Ident.Token.Line, r.Ident.Token.Col, identText(r.Ident))
			addOcc(key, Occurrence{URI: uri, Range: rng, Kind: kind})
		}
	}

	return out
}

func keyForBinding(uri string, modulePath string, exports map[string]bool, root *Scope, b *Binding) (SymbolKey, bool) {
	if b == nil || b.Scope == nil {
		return SymbolKey{}, false
	}
	switch b.Kind {
	case SymFunc, SymVar:
		if root != nil && b.Scope == root && exports[b.Name] && modulePath != "" {
			return SymbolKey{Kind: SymKeyExport, ModulePath: modulePath, Name: b.Name}, true
		}
		return localKey(uri, b)
	case SymParam, SymNamespace:
		return localKey(uri, b)
	default:
		return localKey(uri, b)
	}
}

func localKey(uri string, b *Binding) (SymbolKey, bool) {
	if uri == "" || b == nil || b.Scope == nil {
		return SymbolKey{}, false
	}
	return SymbolKey{
		Kind:       SymKeyLocal,
		Name:       b.Name,
		URI:        uri,
		ScopeStart: b.Scope.Start,
		ScopeEnd:   b.Scope.End,
	}, true
}

func exportedNames(prog *ast.Program) map[string]bool {
	out := map[string]bool{}
	if prog == nil {
		return out
	}
	for _, st := range prog.Statements {
		exp, ok := st.(*ast.ExportStatement)
		if !ok || exp.Stmt == nil {
			continue
		}
		switch inner := exp.Stmt.(type) {
		case *ast.FuncStatement:
			if inner.Name != nil {
				out[identText(inner.Name)] = true
			}
		case *ast.AssignStatement:
			if inner.Name != nil {
				out[identText(inner.Name)] = true
			}
		}
	}
	return out
}

func collectFromImportInfo(prog *ast.Program) (map[*ast.Identifier]importBindingInfo, []importNameOccurrence) {
	info := map[*ast.Identifier]importBindingInfo{}
	names := []importNameOccurrence{}
	var walkStmt func(ast.Statement)
	walkStmt = func(st ast.Statement) {
		switch n := st.(type) {
		case *ast.FromImportStatement:
			spec := ""
			if n.Path != nil {
				spec = n.Path.Value
			}
			for _, item := range n.Items {
				if item.Name == nil {
					continue
				}
				member := identText(item.Name)
				names = append(names, importNameOccurrence{
					Spec:   spec,
					Member: member,
					Ident:  item.Name,
				})
				id := item.Name
				aliasUsed := false
				if item.Alias != nil {
					id = item.Alias
					aliasUsed = true
				}
				if id != nil {
					info[id] = importBindingInfo{
						Spec:      spec,
						Member:    member,
						AliasUsed: aliasUsed,
					}
				}
			}
		case *ast.BlockStatement:
			for _, st := range n.Statements {
				walkStmt(st)
			}
		case *ast.IfStatement:
			walkStmt(n.Consequence)
			if n.Alternative != nil {
				walkStmt(n.Alternative)
			}
		case *ast.WhileStatement:
			walkStmt(n.Body)
		case *ast.ForStatement:
			if n.Init != nil {
				walkStmt(n.Init)
			}
			if n.Post != nil {
				walkStmt(n.Post)
			}
			walkStmt(n.Body)
		case *ast.ForInStatement:
			walkStmt(n.Body)
		case *ast.SwitchStatement:
			for _, c := range n.Cases {
				if c != nil && c.Body != nil {
					walkStmt(c.Body)
				}
			}
			if n.Default != nil {
				walkStmt(n.Default)
			}
		case *ast.TryStatement:
			if n.TryBlock != nil {
				walkStmt(n.TryBlock)
			}
			if n.CatchBlock != nil {
				walkStmt(n.CatchBlock)
			}
			if n.FinallyBlock != nil {
				walkStmt(n.FinallyBlock)
			}
		case *ast.ExportStatement:
			if n.Stmt != nil {
				walkStmt(n.Stmt)
			}
		}
	}
	for _, st := range prog.Statements {
		walkStmt(st)
	}
	return info, names
}

func rangeKey(r protocol.Range, uri string) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", uri, r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
}
