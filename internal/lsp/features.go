package lsp

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/token"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type CompletionContext struct {
	Alias    string
	Prefix   string
	InString bool
}

func CompletionItems(ws *Workspace, uri string, text string, pos protocol.Position) []protocol.CompletionItem {
	an, _ := Analyze(text)
	posByte, ok := positionToByte(text, pos)
	if !ok {
		return nil
	}
	ctx := completionContext(text, posByte)
	if ctx.Alias != "" {
		return completionForModule(ws, uri, an, posByte, ctx.Alias, ctx.Prefix)
	}
	if ctx.InString {
		return completionForStdModules(ws, ctx.Prefix)
	}

	scope := an.ScopeAt(posByte)
	items := []completionCandidate{}
	seen := map[string]bool{}

	for sc := scope; sc != nil; sc = sc.Parent {
		for name, b := range sc.Bindings {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			items = append(items, completionCandidate{name: name, kind: b.Kind})
		}
	}

	for name := range builtinDocs {
		if !seen[name] {
			seen[name] = true
			items = append(items, completionCandidate{name: name, kind: SymBuiltin})
		}
	}

	for _, kw := range tokenKeywords() {
		if !seen[kw] {
			seen[kw] = true
			items = append(items, completionCandidate{name: kw, kind: SymKeyword})
		}
	}

	for _, mod := range stdModules(ws) {
		name := strings.TrimSuffix(mod, ".wll")
		if !seen[name] {
			seen[name] = true
			items = append(items, completionCandidate{name: name, kind: SymNamespace})
		}
	}

	return buildCompletionItems(items)
}

func HoverAt(ws *Workspace, uri string, text string, pos protocol.Position) (*protocol.Hover, error) {
	an, _ := Analyze(text)
	posByte, ok := positionToByte(text, pos)
	if !ok {
		return nil, nil
	}
	ref, def := an.FindOccurrence(posByte)
	var method *MethodInfo
	if member := MemberAt(text, pos); member.Ok {
		method = methodInfo(member.Member)
	}
	if ref == nil && def == nil && method == nil {
		return nil, nil
	}

	var kindLabel string
	var signature string
	var doc string
	var name string

	switch {
	case ref != nil && ref.Kind == SymModuleMember:
		name = ref.Member
		kindLabel = "module member"
		signature, doc = moduleSignatureAndDoc(ws, uri, ref.ModulePath, ref.Member)
		if signature == "" {
			signature = fmt.Sprintf("%s.%s", ref.ModuleAlias, ref.Member)
		}
	case ref != nil && ref.Kind == SymBuiltin:
		info := builtinInfo(ref.Name)
		name = ref.Name
		kindLabel = "builtin"
		if info != nil {
			signature = info.Signature
			doc = info.Doc
		}
	case ref != nil && ref.Binding != nil:
		name = ref.Binding.Name
		kindLabel = kindLabelFor(ref.Binding.Kind)
		if ref.Binding.Kind == SymFunc {
			signature = fmt.Sprintf("%s(%s)", ref.Binding.Name, strings.Join(ref.Binding.Params, ", "))
		}
		if ref.Binding.Kind == SymImport {
			signature = fmt.Sprintf("%s.%s", ref.Binding.ModulePath, ref.Binding.Member)
		}
	case def != nil:
		name = def.Name
		kindLabel = kindLabelFor(def.Kind)
		if def.Kind == SymFunc {
			signature = fmt.Sprintf("%s(%s)", def.Name, strings.Join(def.Params, ", "))
		}
		if def.Kind == SymImport {
			signature = fmt.Sprintf("%s.%s", def.ModulePath, def.Member)
		}
	}

	if name == "" && method != nil {
		name = method.Name
		kindLabel = "method"
		signature = method.Signature
		doc = method.Doc
	}

	if name == "" {
		return nil, nil
	}

	lines := []string{}
	if kindLabel != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", kindLabel, name))
	}
	if signature != "" {
		lines = append(lines, signature)
	}
	if doc != "" {
		lines = append(lines, "", doc)
	}

	contents := protocol.MarkupContent{Kind: "markdown", Value: strings.Join(lines, "\n")}
	return &protocol.Hover{Contents: contents}, nil
}

func RenameAt(ws *Workspace, uri string, text string, pos protocol.Position, newName string) (*protocol.WorkspaceEdit, error) {
	if token.LookupIdent(newName) != token.IDENT {
		return nil, fmt.Errorf("cannot rename to keyword")
	}
	if builtinInfo(newName) != nil {
		return nil, fmt.Errorf("cannot rename to builtin")
	}

	an, _ := Analyze(text)
	posByte, ok := positionToByte(text, pos)
	if !ok {
		return nil, nil
	}
	ref, def := an.FindOccurrence(posByte)

	if ref == nil && def == nil {
		return nil, nil
	}
	if ref != nil && ref.Kind == SymBuiltin {
		return nil, fmt.Errorf("cannot rename builtin")
	}

	if key, ok, err := exportKeyForTarget(ws, uri, an, ref, def, true); ok {
		if err != nil {
			return nil, err
		}
		ix, err := BuildWorkspaceIndex(ws)
		if err != nil {
			return nil, err
		}
		occ := ix.ByKey[key]
		if len(occ) == 0 {
			return nil, nil
		}
		changes := map[protocol.DocumentUri][]protocol.TextEdit{}
		seen := map[string]bool{}
		for _, o := range occ {
			if o.Kind == OccurrenceAliasUse {
				continue
			}
			keyStr := fmt.Sprintf("%s:%d:%d:%d:%d", o.URI, o.Range.Start.Line, o.Range.Start.Character, o.Range.End.Line, o.Range.End.Character)
			if seen[keyStr] {
				continue
			}
			seen[keyStr] = true
			uriDoc := protocol.DocumentUri(o.URI)
			changes[uriDoc] = append(changes[uriDoc], protocol.TextEdit{Range: o.Range, NewText: newName})
		}
		if len(changes) == 0 {
			return nil, nil
		}
		return &protocol.WorkspaceEdit{Changes: changes}, nil
	} else if err != nil {
		return nil, err
	}

	edits := []protocol.TextEdit{}
	seen := map[string]bool{}

	addEdit := func(id *ast.Identifier) {
		if id == nil {
			return
		}
		r := rangeFromPosLenUTF16(text, id.Token.Line, id.Token.Col, identText(id))
		key := fmt.Sprintf("%d:%d:%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		if seen[key] {
			return
		}
		seen[key] = true
		edits = append(edits, protocol.TextEdit{Range: r, NewText: newName})
	}

	if ref != nil && ref.Kind == SymModuleMember {
		for _, r := range an.Refs {
			if r.Kind != SymModuleMember {
				continue
			}
			if r.Member == ref.Member && r.ModuleAlias == ref.ModuleAlias {
				addEdit(r.Ident)
			}
		}
	} else {
		var target *Binding
		if ref != nil {
			target = ref.Binding
		} else {
			target = def
		}
		if target == nil {
			return nil, nil
		}
		addEdit(target.Decl)
		for _, r := range an.Refs {
			if r.Binding == target {
				addEdit(r.Ident)
			}
		}
	}

	if len(edits) == 0 {
		return nil, nil
	}

	changes := map[protocol.DocumentUri][]protocol.TextEdit{
		protocol.DocumentUri(uri): edits,
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func ReferencesAt(ws *Workspace, uri string, text string, pos protocol.Position, includeDecl bool) ([]protocol.Location, error) {
	an, _ := Analyze(text)
	posByte, ok := positionToByte(text, pos)
	if !ok {
		return nil, nil
	}
	ref, def := an.FindOccurrence(posByte)
	if ref == nil && def == nil {
		return nil, nil
	}

	if key, ok, err := exportKeyForTarget(ws, uri, an, ref, def, false); ok {
		if err != nil {
			return nil, err
		}
		ix, err := BuildWorkspaceIndex(ws)
		if err != nil {
			return nil, err
		}
		occ := ix.ByKey[key]
		if len(occ) == 0 {
			return nil, nil
		}
		locs := []protocol.Location{}
		seen := map[string]bool{}
		for _, o := range occ {
			if !includeDecl && o.Kind == OccurrenceDecl {
				continue
			}
			keyStr := fmt.Sprintf("%s:%d:%d:%d:%d", o.URI, o.Range.Start.Line, o.Range.Start.Character, o.Range.End.Line, o.Range.End.Character)
			if seen[keyStr] {
				continue
			}
			seen[keyStr] = true
			locs = append(locs, protocol.Location{URI: protocol.DocumentUri(o.URI), Range: o.Range})
		}
		return locs, nil
	} else if err != nil {
		return nil, err
	}

	locs := []protocol.Location{}
	seen := map[string]bool{}
	addLoc := func(id *ast.Identifier) {
		if id == nil {
			return
		}
		r := rangeFromPosLenUTF16(text, id.Token.Line, id.Token.Col, identText(id))
		key := fmt.Sprintf("%d:%d:%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		if seen[key] {
			return
		}
		seen[key] = true
		locs = append(locs, protocol.Location{URI: protocol.DocumentUri(uri), Range: r})
	}

	if ref != nil && ref.Kind == SymModuleMember {
		for _, r := range an.Refs {
			if r.Kind == SymModuleMember && r.Member == ref.Member && r.ModuleAlias == ref.ModuleAlias {
				addLoc(r.Ident)
			}
		}
		return locs, nil
	}

	var target *Binding
	if ref != nil {
		target = ref.Binding
	} else {
		target = def
	}
	if target == nil {
		return nil, nil
	}
	if includeDecl {
		addLoc(target.Decl)
	}
	for _, r := range an.Refs {
		if r.Binding == target {
			addLoc(r.Ident)
		}
	}
	return locs, nil
}

func SignatureHelpAt(ws *Workspace, uri string, text string, pos protocol.Position) (*protocol.SignatureHelp, error) {
	an, _ := Analyze(text)
	posByte, ok := positionToByte(text, pos)
	if !ok || an.Program == nil {
		return nil, nil
	}

	call, active := findCallAt(text, an.Program, posByte)
	if call == nil {
		return nil, nil
	}

	label, params := signatureForCall(ws, uri, an, call)
	if label == "" {
		return nil, nil
	}

	paramInfos := make([]protocol.ParameterInformation, 0, len(params))
	for _, p := range params {
		paramInfos = append(paramInfos, protocol.ParameterInformation{Label: p})
	}

	sig := protocol.SignatureInformation{Label: label, Parameters: paramInfos}
	activeParam := protocol.UInteger(active)
	return &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{sig}, ActiveSignature: ptrUinteger(0), ActiveParameter: &activeParam}, nil
}

func signatureForCall(ws *Workspace, uri string, an *Analysis, call *ast.CallExpression) (string, []string) {
	switch fn := call.Function.(type) {
	case *ast.Identifier:
		name := identText(fn)
		if info := builtinInfo(name); info != nil {
			return info.Signature, info.Params
		}
		if an != nil {
			pos := Pos{Line: fn.Token.Line, Col: fn.Token.Col}
			if b, _ := an.ResolveAt(pos, name); b != nil {
				if b.Kind == SymFunc {
					return fmt.Sprintf("%s(%s)", b.Name, strings.Join(b.Params, ", ")), b.Params
				}
				if b.Kind == SymImport {
					return moduleSignature(ws, uri, b.ModulePath, b.Member)
				}
			}
		}
	case *ast.MemberExpression:
		if id, ok := fn.Object.(*ast.Identifier); ok {
			name := identText(id)
			if an != nil {
				pos := Pos{Line: id.Token.Line, Col: id.Token.Col}
				if b, _ := an.ResolveAt(pos, name); b != nil && b.Kind == SymNamespace {
					member := identText(fn.Property)
					return moduleSignature(ws, uri, b.ModulePath, member)
				}
			}
		}
		member := identText(fn.Property)
		if info := methodInfo(member); info != nil {
			return info.Signature, info.Params
		}
	}
	return "", nil
}

func moduleSignature(ws *Workspace, uri string, spec string, member string) (string, []string) {
	info, err := LoadModuleInfo(ws, uri, spec)
	if err != nil || info == nil {
		return "", nil
	}
	if ex, ok := info.Exports[member]; ok {
		label := ex.Name
		if ex.Kind == SymFunc {
			label = fmt.Sprintf("%s(%s)", ex.Name, strings.Join(ex.Params, ", "))
		}
		return label, ex.Params
	}
	return "", nil
}

func moduleSignatureAndDoc(ws *Workspace, uri string, spec string, member string) (string, string) {
	label, params := moduleSignature(ws, uri, spec, member)
	if label == "" {
		return "", ""
	}
	if len(params) == 0 {
		return label, ""
	}
	return label, ""
}

func completionContext(text string, pos Pos) CompletionContext {
	ctx := CompletionContext{}
	lines := splitLines(text)
	if pos.Line <= 0 || pos.Line > len(lines) {
		return ctx
	}

	ctx.InString = positionInStringToken(text, pos)

	alias, prefix := memberCompletionAlias(text, pos)
	ctx.Alias = alias
	ctx.Prefix = prefix
	return ctx
}

func memberCompletionAlias(text string, pos Pos) (string, string) {
	lx := lexer.New(text)
	var prev2, prev1 token.Token
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			return "", ""
		}
		if tok.Line > pos.Line || (tok.Line == pos.Line && tok.Col > pos.Col) {
			break
		}
		prev2 = prev1
		prev1 = tok
	}

	if prev1.Type == token.DOT && prev2.Type == token.IDENT {
		return prev2.Literal, ""
	}
	if prev1.Type == token.IDENT && prev2.Type == token.DOT {
		return prev2.Literal, prev1.Literal
	}
	return "", ""
}

func positionInStringToken(text string, pos Pos) bool {
	lx := lexer.New(text)
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			return false
		}
		if tok.Type != token.STRING {
			continue
		}
		start := Pos{Line: tok.Line, Col: tok.Col}
		end := Pos{Line: tok.Line, Col: tok.Col + max(1, len(tok.Literal))}
		if posWithin(pos, start, end) {
			return true
		}
	}
}

func completionForModule(ws *Workspace, uri string, an *Analysis, pos Pos, alias string, prefix string) []protocol.CompletionItem {
	if ws == nil || an == nil {
		return nil
	}
	b, _ := an.ResolveAt(pos, alias)
	if b == nil || b.Kind != SymNamespace {
		return nil
	}

	info, err := LoadModuleInfo(ws, uri, b.ModulePath)
	if err != nil || info == nil {
		return nil
	}

	items := []completionCandidate{}
	for name, ex := range info.Exports {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		kind := SymVar
		if ex.Kind == SymFunc {
			kind = SymFunc
		}
		items = append(items, completionCandidate{name: name, kind: kind})
	}
	return buildCompletionItems(items)
}

func completionForStdModules(ws *Workspace, prefix string) []protocol.CompletionItem {
	items := []completionCandidate{}
	for _, mod := range stdModules(ws) {
		name := strings.TrimSuffix(mod, ".wll")
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		items = append(items, completionCandidate{name: name, kind: SymNamespace})
	}
	return buildCompletionItems(items)
}

func stdModules(ws *Workspace) []string {
	if ws == nil {
		return nil
	}
	return ws.StdModules()
}

type completionCandidate struct {
	name string
	kind SymbolKind
}

func buildCompletionItems(items []completionCandidate) []protocol.CompletionItem {
	sort.Slice(items, func(i, j int) bool {
		wi := completionWeight(items[i].kind)
		wj := completionWeight(items[j].kind)
		if wi != wj {
			return wi < wj
		}
		return items[i].name < items[j].name
	})

	out := make([]protocol.CompletionItem, 0, len(items))
	for _, it := range items {
		out = append(out, protocol.CompletionItem{
			Label: it.name,
			Kind:  completionItemKindPtr(it.kind),
		})
	}
	return out
}

func completionWeight(kind SymbolKind) int {
	switch kind {
	case SymVar, SymFunc:
		return 0
	case SymParam:
		return 1
	case SymNamespace, SymImport:
		return 2
	case SymBuiltin:
		return 3
	case SymKeyword:
		return 4
	default:
		return 5
	}
}

func completionItemKind(kind SymbolKind) protocol.CompletionItemKind {
	switch kind {
	case SymFunc:
		return protocol.CompletionItemKindFunction
	case SymParam:
		return protocol.CompletionItemKindVariable
	case SymNamespace:
		return protocol.CompletionItemKindModule
	case SymImport:
		return protocol.CompletionItemKindVariable
	case SymBuiltin:
		return protocol.CompletionItemKindFunction
	case SymKeyword:
		return protocol.CompletionItemKindKeyword
	default:
		return protocol.CompletionItemKindVariable
	}
}

func exportKeyForTarget(ws *Workspace, uri string, an *Analysis, ref *Reference, def *Binding, strict bool) (SymbolKey, bool, error) {
	if ws == nil || an == nil {
		return SymbolKey{}, false, nil
	}
	absPath := UriToPath(uri)
	if absPath != "" {
		absPath, _ = filepath.Abs(absPath)
	}
	resolve := func(spec string) (string, bool, error) {
		if spec == "" || absPath == "" {
			if strict {
				return "", false, fmt.Errorf("cannot resolve module import without file path")
			}
			return "", false, nil
		}
		resolved, err := ws.ResolveImport(absPath, spec)
		if err != nil {
			if strict {
				return "", false, fmt.Errorf("cannot resolve module import: %w", err)
			}
			return "", false, nil
		}
		resolvedAbs, _ := filepath.Abs(resolved)
		return resolvedAbs, true, nil
	}

	exports := exportedNames(an.Program)

	if ref != nil && ref.Kind == SymModuleMember {
		resolved, ok, err := resolve(ref.ModulePath)
		if err != nil {
			return SymbolKey{}, true, err
		}
		if !ok {
			return SymbolKey{}, false, nil
		}
		return SymbolKey{Kind: SymKeyExport, ModulePath: resolved, Name: ref.Member}, true, nil
	}

	var target *Binding
	if ref != nil {
		target = ref.Binding
	} else {
		target = def
	}
	if target == nil {
		return SymbolKey{}, false, nil
	}

	if target.Kind == SymImport {
		resolved, ok, err := resolve(target.ModulePath)
		if err != nil {
			return SymbolKey{}, true, err
		}
		if !ok {
			return SymbolKey{}, false, nil
		}
		if target.Member == "" && strict {
			return SymbolKey{}, true, fmt.Errorf("cannot resolve import member for rename")
		}
		return SymbolKey{Kind: SymKeyExport, ModulePath: resolved, Name: target.Member}, true, nil
	}

	if (target.Kind == SymFunc || target.Kind == SymVar) && an.Root != nil && target.Scope == an.Root && exports[target.Name] {
		if absPath == "" {
			if strict {
				return SymbolKey{}, true, fmt.Errorf("cannot resolve module path for rename")
			}
			return SymbolKey{}, false, nil
		}
		return SymbolKey{Kind: SymKeyExport, ModulePath: absPath, Name: target.Name}, true, nil
	}

	return SymbolKey{}, false, nil
}

func completionItemKindPtr(kind SymbolKind) *protocol.CompletionItemKind {
	k := completionItemKind(kind)
	return &k
}

func kindLabelFor(kind SymbolKind) string {
	switch kind {
	case SymVar:
		return "var"
	case SymParam:
		return "param"
	case SymFunc:
		return "func"
	case SymNamespace:
		return "import"
	case SymImport:
		return "import"
	default:
		return "symbol"
	}
}

func tokenKeywords() []string {
	return []string{
		"func", "return", "break", "continue", "if", "else", "while", "for", "in", "true", "false", "nil", "null",
		"and", "or", "not", "import", "from", "as", "try", "catch", "finally", "throw", "defer", "export",
		"switch", "match", "case", "default",
	}
}

func ptrUinteger(v protocol.UInteger) *protocol.UInteger { return &v }
