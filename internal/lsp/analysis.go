package lsp

import (
	"fmt"

	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/parser"
	"welle/internal/token"
)

type SymbolKind int

const (
	SymVar SymbolKind = iota
	SymParam
	SymFunc
	SymNamespace
	SymImport
	SymBuiltin
	SymKeyword
	SymModuleMember
)

type Binding struct {
	Name       string
	Kind       SymbolKind
	Decl       *ast.Identifier
	Scope      *Scope
	Params     []string
	ModulePath string
	Member     string
}

type Reference struct {
	Name        string
	Ident       *ast.Identifier
	Binding     *Binding
	Kind        SymbolKind
	ModuleAlias string
	ModulePath  string
	Member      string
}

type Scope struct {
	Parent   *Scope
	Children []*Scope
	Start    Pos
	End      Pos
	Bindings map[string]*Binding
}

type Analysis struct {
	Program *ast.Program
	Text    string
	Root    *Scope
	Refs    []*Reference
	Defs    []*Binding
}

type blockRange struct {
	Start Pos
	End   Pos
}

type posKey struct {
	Line int
	Col  int
}

func Analyze(text string) (*Analysis, error) {
	lx := lexer.New(text)
	p := parser.New(lx)
	prog := p.ParseProgram()

	an := &Analysis{Program: prog, Text: text}
	if prog == nil {
		return an, nil
	}

	blockRanges := buildBlockRanges(text, prog)
	root := &Scope{Start: Pos{Line: 1, Col: 1}, End: endPosByte(text), Bindings: map[string]*Binding{}}
	an.Root = root

	resolve := func(sc *Scope, name string) *Binding {
		for s := sc; s != nil; s = s.Parent {
			if b, ok := s.Bindings[name]; ok {
				return b
			}
		}
		return nil
	}

	declare := func(sc *Scope, name string, kind SymbolKind, id *ast.Identifier) *Binding {
		if name == "" || sc == nil {
			return nil
		}
		if existing, ok := sc.Bindings[name]; ok {
			return existing
		}
		b := &Binding{Name: name, Kind: kind, Decl: id, Scope: sc}
		sc.Bindings[name] = b
		an.Defs = append(an.Defs, b)
		return b
	}

	addRef := func(id *ast.Identifier, b *Binding) {
		if id == nil {
			return
		}
		ref := &Reference{Ident: id, Binding: b, Kind: b.Kind, Name: b.Name}
		an.Refs = append(an.Refs, ref)
	}

	addBuiltinRef := func(id *ast.Identifier, name string) {
		if id == nil {
			return
		}
		an.Refs = append(an.Refs, &Reference{Ident: id, Kind: SymBuiltin, Name: name})
	}

	addModuleMemberRef := func(id *ast.Identifier, alias string, modulePath string, member string) {
		if id == nil {
			return
		}
		an.Refs = append(an.Refs, &Reference{Ident: id, Kind: SymModuleMember, ModuleAlias: alias, ModulePath: modulePath, Member: member, Name: member})
	}

	var walkStmt func(sc *Scope, st ast.Statement)
	var walkExpr func(sc *Scope, e ast.Expression)

	walkStmt = func(sc *Scope, st ast.Statement) {
		switch n := st.(type) {
		case *ast.BlockStatement:
			r := blockRanges[n]
			child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
			sc.Children = append(sc.Children, child)
			for _, st := range n.Statements {
				walkStmt(child, st)
			}

		case *ast.FuncStatement:
			if n.Name != nil {
				b := declare(sc, identText(n.Name), SymFunc, n.Name)
				if b != nil {
					b.Params = paramsFromIdents(n.Parameters)
				}
			}
			if n.Body != nil {
				r := blockRanges[n.Body]
				child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
				sc.Children = append(sc.Children, child)
				for _, p := range n.Parameters {
					if p == nil {
						continue
					}
					declare(child, identText(p), SymParam, p)
					addRef(p, child.Bindings[identText(p)])
				}
				for _, st := range n.Body.Statements {
					walkStmt(child, st)
				}
			}

		case *ast.AssignStatement:
			if n.Name != nil {
				name := identText(n.Name)
				b := sc.Bindings[name]
				if b == nil {
					kind := SymVar
					if fl, ok := n.Value.(*ast.FunctionLiteral); ok {
						kind = SymFunc
						b = declare(sc, name, kind, n.Name)
						if b != nil {
							b.Params = paramsFromIdents(fl.Parameters)
						}
					} else {
						b = declare(sc, name, kind, n.Name)
					}
				} else if b.Kind == SymFunc {
					// keep function kind
				}
				if b != nil {
					addRef(n.Name, b)
				}
			}
			walkExpr(sc, n.Value)

		case *ast.IndexAssignStatement:
			walkExpr(sc, n.Left)
			walkExpr(sc, n.Value)

		case *ast.MemberAssignStatement:
			walkExpr(sc, n.Object)
			walkExpr(sc, n.Value)

		case *ast.DestructureAssignStatement:
			for _, t := range n.Targets {
				if t == nil || t.Name == nil {
					continue
				}
				name := identText(t.Name)
				b := sc.Bindings[name]
				if b == nil {
					b = declare(sc, name, SymVar, t.Name)
				}
				if b != nil {
					addRef(t.Name, b)
				}
			}
			walkExpr(sc, n.Value)

		case *ast.ReturnStatement:
			for _, rv := range n.ReturnValues {
				walkExpr(sc, rv)
			}

		case *ast.DeferStatement:
			walkExpr(sc, n.Call)

		case *ast.ThrowStatement:
			walkExpr(sc, n.Value)

		case *ast.ExpressionStatement:
			walkExpr(sc, n.Expression)

		case *ast.ImportStatement:
			if n.Alias != nil {
				b := declare(sc, identText(n.Alias), SymNamespace, n.Alias)
				if b != nil {
					if n.Path != nil {
						b.ModulePath = n.Path.Value
					}
					addRef(n.Alias, b)
				}
			}

		case *ast.FromImportStatement:
			for _, it := range n.Items {
				id := it.Name
				if it.Alias != nil {
					id = it.Alias
				}
				if id == nil {
					continue
				}
				b := declare(sc, identText(id), SymImport, id)
				if b != nil {
					if n.Path != nil {
						b.ModulePath = n.Path.Value
					}
					if it.Name != nil {
						b.Member = identText(it.Name)
					}
					addRef(id, b)
				}
			}

		case *ast.ExportStatement:
			if n.Stmt != nil {
				walkStmt(sc, n.Stmt)
			}

		case *ast.IfStatement:
			walkExpr(sc, n.Condition)
			if n.Consequence != nil {
				walkStmt(sc, n.Consequence)
			}
			if n.Alternative != nil {
				walkStmt(sc, n.Alternative)
			}

		case *ast.WhileStatement:
			walkExpr(sc, n.Condition)
			if n.Body != nil {
				walkStmt(sc, n.Body)
			}

		case *ast.ForStatement:
			r := blockRanges[n.Body]
			child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
			sc.Children = append(sc.Children, child)
			if n.Init != nil {
				walkStmt(child, n.Init)
			}
			if n.Cond != nil {
				walkExpr(child, n.Cond)
			}
			if n.Post != nil {
				walkStmt(child, n.Post)
			}
			if n.Body != nil {
				for _, st := range n.Body.Statements {
					walkStmt(child, st)
				}
			}

		case *ast.ForInStatement:
			r := blockRanges[n.Body]
			child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
			sc.Children = append(sc.Children, child)
			if n.Destruct {
				if n.Key != nil && n.Key.Value != "_" {
					b := declare(child, identText(n.Key), SymVar, n.Key)
					if b != nil {
						addRef(n.Key, b)
					}
				}
				if n.Value != nil && n.Value.Value != "_" {
					b := declare(child, identText(n.Value), SymVar, n.Value)
					if b != nil {
						addRef(n.Value, b)
					}
				}
			} else if n.Var != nil {
				b := declare(child, identText(n.Var), SymVar, n.Var)
				if b != nil {
					addRef(n.Var, b)
				}
			}
			walkExpr(child, n.Iterable)
			if n.Body != nil {
				for _, st := range n.Body.Statements {
					walkStmt(child, st)
				}
			}

		case *ast.SwitchStatement:
			walkExpr(sc, n.Value)
			for _, c := range n.Cases {
				if c == nil {
					continue
				}
				for _, v := range c.Values {
					walkExpr(sc, v)
				}
				if c.Body != nil {
					walkStmt(sc, c.Body)
				}
			}
			if n.Default != nil {
				walkStmt(sc, n.Default)
			}

		case *ast.TryStatement:
			if n.TryBlock != nil {
				walkStmt(sc, n.TryBlock)
			}
			if n.CatchBlock != nil {
				r := blockRanges[n.CatchBlock]
				child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
				sc.Children = append(sc.Children, child)
				if n.CatchName != nil {
					b := declare(child, identText(n.CatchName), SymVar, n.CatchName)
					if b != nil {
						addRef(n.CatchName, b)
					}
				}
				for _, st := range n.CatchBlock.Statements {
					walkStmt(child, st)
				}
			}
			if n.FinallyBlock != nil {
				walkStmt(sc, n.FinallyBlock)
			}
		}
	}

	walkExpr = func(sc *Scope, e ast.Expression) {
		switch n := e.(type) {
		case *ast.Identifier:
			name := identText(n)
			if b := resolve(sc, name); b != nil {
				addRef(n, b)
				return
			}
			if builtinInfo(name) != nil {
				addBuiltinRef(n, name)
				return
			}

		case *ast.CallExpression:
			walkExpr(sc, n.Function)
			for _, a := range n.Arguments {
				walkExpr(sc, a)
			}

		case *ast.MemberExpression:
			if id, ok := n.Object.(*ast.Identifier); ok {
				b := resolve(sc, identText(id))
				if b != nil && b.Kind == SymNamespace {
					addRef(id, b)
					member := identText(n.Property)
					addModuleMemberRef(n.Property, b.Name, b.ModulePath, member)
					return
				}
			}
			walkExpr(sc, n.Object)

		case *ast.InfixExpression:
			walkExpr(sc, n.Left)
			walkExpr(sc, n.Right)

		case *ast.ConditionalExpression:
			walkExpr(sc, n.Cond)
			walkExpr(sc, n.Then)
			walkExpr(sc, n.Else)

		case *ast.CondExpr:
			walkExpr(sc, n.Cond)
			walkExpr(sc, n.Then)
			walkExpr(sc, n.Else)

		case *ast.PrefixExpression:
			walkExpr(sc, n.Right)

		case *ast.SpreadExpression:
			walkExpr(sc, n.Value)

		case *ast.IndexExpression:
			walkExpr(sc, n.Left)
			walkExpr(sc, n.Index)

		case *ast.SliceExpression:
			walkExpr(sc, n.Left)
			if n.Low != nil {
				walkExpr(sc, n.Low)
			}
			if n.High != nil {
				walkExpr(sc, n.High)
			}
			if n.Step != nil {
				walkExpr(sc, n.Step)
			}

		case *ast.ListLiteral:
			for _, el := range n.Elements {
				walkExpr(sc, el)
			}

		case *ast.ListComprehension:
			walkExpr(sc, n.Seq)
			comp := &Scope{Parent: sc, Bindings: map[string]*Binding{}}
			if n.Var != nil {
				b := declare(comp, identText(n.Var), SymVar, n.Var)
				if b != nil {
					addRef(n.Var, b)
				}
			}
			if n.Filter != nil {
				walkExpr(comp, n.Filter)
			}
			walkExpr(comp, n.Elem)

		case *ast.DictLiteral:
			for _, p := range n.Pairs {
				if p.Shorthand != nil {
					walkExpr(sc, p.Shorthand)
					continue
				}
				walkExpr(sc, p.Key)
				walkExpr(sc, p.Value)
			}

		case *ast.MatchExpression:
			walkExpr(sc, n.Value)
			for _, c := range n.Cases {
				for _, val := range c.Values {
					walkExpr(sc, val)
				}
				walkExpr(sc, c.Result)
			}
			if n.Default != nil {
				walkExpr(sc, n.Default)
			}

		case *ast.FunctionLiteral:
			if n.Body != nil {
				r := blockRanges[n.Body]
				child := &Scope{Parent: sc, Start: r.Start, End: r.End, Bindings: map[string]*Binding{}}
				sc.Children = append(sc.Children, child)
				for _, p := range n.Parameters {
					if p == nil {
						continue
					}
					declare(child, identText(p), SymParam, p)
					addRef(p, child.Bindings[identText(p)])
				}
				for _, st := range n.Body.Statements {
					walkStmt(child, st)
				}
			}

		case *ast.TemplateLiteral:
			if n.Tag != nil {
				walkExpr(sc, n.Tag)
			}
			for _, ex := range n.Exprs {
				walkExpr(sc, ex)
			}

		case *ast.AssignExpression:
			switch left := n.Left.(type) {
			case *ast.Identifier:
				name := identText(left)
				b := sc.Bindings[name]
				if b == nil {
					kind := SymVar
					if fl, ok := n.Value.(*ast.FunctionLiteral); ok {
						kind = SymFunc
						b = declare(sc, name, kind, left)
						if b != nil {
							b.Params = paramsFromIdents(fl.Parameters)
						}
					} else {
						b = declare(sc, name, kind, left)
					}
				} else if b.Kind == SymFunc {
					// keep function kind
				}
				if b != nil {
					addRef(left, b)
				}
			case *ast.IndexExpression:
				walkExpr(sc, left.Left)
				walkExpr(sc, left.Index)
			case *ast.MemberExpression:
				walkExpr(sc, left.Object)
			}
			walkExpr(sc, n.Value)
		}
	}

	for _, st := range prog.Statements {
		walkStmt(root, st)
	}

	return an, nil
}

func buildBlockRanges(text string, prog *ast.Program) map[*ast.BlockStatement]blockRange {
	blocks := map[posKey]*ast.BlockStatement{}
	collectBlocks(prog, func(b *ast.BlockStatement) {
		if b == nil {
			return
		}
		blocks[posKey{Line: b.Token.Line, Col: b.Token.Col}] = b
	})

	ranges := map[*ast.BlockStatement]blockRange{}
	lx := lexer.New(text)
	stack := []*ast.BlockStatement{}
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		switch tok.Type {
		case token.LBRACE:
			if b, ok := blocks[posKey{Line: tok.Line, Col: tok.Col}]; ok {
				stack = append(stack, b)
			}
		case token.RBRACE:
			if len(stack) > 0 {
				b := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				ranges[b] = blockRange{Start: Pos{Line: b.Token.Line, Col: b.Token.Col}, End: Pos{Line: tok.Line, Col: tok.Col}}
			}
		}
	}
	end := endPosByte(text)
	for _, b := range blocks {
		if _, ok := ranges[b]; !ok {
			ranges[b] = blockRange{Start: Pos{Line: b.Token.Line, Col: b.Token.Col}, End: end}
		}
	}
	return ranges
}

func collectBlocks(node ast.Node, fn func(*ast.BlockStatement)) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.Program:
		for _, st := range n.Statements {
			collectBlocks(st, fn)
		}
	case *ast.BlockStatement:
		fn(n)
		for _, st := range n.Statements {
			collectBlocks(st, fn)
		}
	case *ast.FuncStatement:
		collectBlocks(n.Body, fn)
	case *ast.FunctionLiteral:
		collectBlocks(n.Body, fn)
	case *ast.IfStatement:
		collectBlocks(n.Consequence, fn)
		if n.Alternative != nil {
			collectBlocks(n.Alternative, fn)
		}
	case *ast.WhileStatement:
		collectBlocks(n.Body, fn)
	case *ast.ForStatement:
		collectBlocks(n.Body, fn)
	case *ast.ForInStatement:
		collectBlocks(n.Body, fn)
	case *ast.TryStatement:
		collectBlocks(n.TryBlock, fn)
		collectBlocks(n.CatchBlock, fn)
		collectBlocks(n.FinallyBlock, fn)
	case *ast.SwitchStatement:
		for _, c := range n.Cases {
			if c != nil {
				collectBlocks(c.Body, fn)
			}
		}
		if n.Default != nil {
			collectBlocks(n.Default, fn)
		}
	case *ast.ExpressionStatement:
		collectBlocks(n.Expression, fn)
	case *ast.CallExpression:
		collectBlocks(n.Function, fn)
		for _, a := range n.Arguments {
			collectBlocks(a, fn)
		}
	case *ast.MatchExpression:
		for _, c := range n.Cases {
			for _, v := range c.Values {
				collectBlocks(v, fn)
			}
			collectBlocks(c.Result, fn)
		}
		collectBlocks(n.Default, fn)
	case *ast.ListLiteral:
		for _, el := range n.Elements {
			collectBlocks(el, fn)
		}
	case *ast.ListComprehension:
		collectBlocks(n.Seq, fn)
		collectBlocks(n.Filter, fn)
		collectBlocks(n.Elem, fn)
	case *ast.DictLiteral:
		for _, p := range n.Pairs {
			if p.Shorthand != nil {
				collectBlocks(p.Shorthand, fn)
				continue
			}
			collectBlocks(p.Key, fn)
			collectBlocks(p.Value, fn)
		}
	case *ast.MemberExpression:
		collectBlocks(n.Object, fn)
	case *ast.IndexExpression:
		collectBlocks(n.Left, fn)
		collectBlocks(n.Index, fn)
	case *ast.SliceExpression:
		collectBlocks(n.Left, fn)
		collectBlocks(n.Low, fn)
		collectBlocks(n.High, fn)
		collectBlocks(n.Step, fn)
	case *ast.InfixExpression:
		collectBlocks(n.Left, fn)
		collectBlocks(n.Right, fn)
	case *ast.ConditionalExpression:
		collectBlocks(n.Cond, fn)
		collectBlocks(n.Then, fn)
		collectBlocks(n.Else, fn)
	case *ast.CondExpr:
		collectBlocks(n.Cond, fn)
		collectBlocks(n.Then, fn)
		collectBlocks(n.Else, fn)
	case *ast.PrefixExpression:
		collectBlocks(n.Right, fn)
	case *ast.SpreadExpression:
		collectBlocks(n.Value, fn)
	case *ast.AssignExpression:
		collectBlocks(n.Left, fn)
		collectBlocks(n.Value, fn)
	case *ast.AssignStatement:
		collectBlocks(n.Value, fn)
	case *ast.IndexAssignStatement:
		collectBlocks(n.Left, fn)
		collectBlocks(n.Value, fn)
	case *ast.MemberAssignStatement:
		collectBlocks(n.Object, fn)
		collectBlocks(n.Value, fn)
	case *ast.DestructureAssignStatement:
		collectBlocks(n.Value, fn)
	case *ast.ReturnStatement:
		for _, rv := range n.ReturnValues {
			collectBlocks(rv, fn)
		}
	case *ast.DeferStatement:
		collectBlocks(n.Call, fn)
	case *ast.ThrowStatement:
		collectBlocks(n.Value, fn)
	}
}

func paramsFromIdents(ids []*ast.Identifier) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == nil {
			continue
		}
		out = append(out, identText(id))
	}
	return out
}

func (a *Analysis) ScopeAt(pos Pos) *Scope {
	if a == nil || a.Root == nil {
		return nil
	}
	return scopeAt(a.Root, pos)
}

func scopeAt(sc *Scope, pos Pos) *Scope {
	if sc == nil {
		return nil
	}
	for _, child := range sc.Children {
		if posWithin(pos, child.Start, child.End) {
			if inner := scopeAt(child, pos); inner != nil {
				return inner
			}
			return child
		}
	}
	if posWithin(pos, sc.Start, sc.End) {
		return sc
	}
	return nil
}

func (a *Analysis) FindOccurrence(pos Pos) (*Reference, *Binding) {
	if a == nil {
		return nil, nil
	}
	match := func(id *ast.Identifier) bool {
		if id == nil {
			return false
		}
		name := identText(id)
		start := Pos{Line: id.Token.Line, Col: id.Token.Col}
		end := Pos{Line: id.Token.Line, Col: id.Token.Col + max(1, len(name))}
		return posWithin(pos, start, end)
	}

	for _, ref := range a.Refs {
		if ref != nil && match(ref.Ident) {
			return ref, ref.Binding
		}
	}
	for _, b := range a.Defs {
		if b != nil && match(b.Decl) {
			return nil, b
		}
	}
	return nil, nil
}

func (a *Analysis) ResolveAt(pos Pos, name string) (*Binding, error) {
	sc := a.ScopeAt(pos)
	for s := sc; s != nil; s = s.Parent {
		if b, ok := s.Bindings[name]; ok {
			return b, nil
		}
	}
	return nil, fmt.Errorf("unresolved: %s", name)
}
