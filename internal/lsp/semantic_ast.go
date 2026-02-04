package lsp

import "welle/internal/ast"

type Key struct {
	Line int
	Col  int
	Len  int
}

type Classified struct {
	Type int
	Mods int
}

type scope struct {
	params     map[string]bool
	locals     map[string]bool
	funcs      map[string]bool
	namespaces map[string]bool
}

var builtinFunctions = map[string]bool{
	"print":     true,
	"len":       true,
	"str":       true,
	"keys":      true,
	"values":    true,
	"range":     true,
	"append":    true,
	"push":      true,
	"hasKey":    true,
	"sort":      true,
	"error":     true,
	"writeFile": true,
}

func identText(id *ast.Identifier) string {
	if id == nil {
		return ""
	}
	if id.Value != "" {
		return id.Value
	}
	return id.Token.Literal
}

func isAllCapsIdent(s string) bool {
	if s == "" {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		} else if (r >= '0' && r <= '9') || r == '_' {
			// ok
		} else {
			return false
		}
	}
	return hasLetter
}

func CollectSemantic(program *ast.Program) map[Key]Classified {
	out := map[Key]Classified{}
	if program == nil {
		return out
	}

	scopes := []scope{{params: map[string]bool{}, locals: map[string]bool{}, funcs: map[string]bool{}, namespaces: map[string]bool{}}}

	push := func() {
		scopes = append(scopes, scope{
			params:     map[string]bool{},
			locals:     map[string]bool{},
			funcs:      map[string]bool{},
			namespaces: map[string]bool{},
		})
	}
	pop := func() { scopes = scopes[:len(scopes)-1] }
	cur := func() *scope { return &scopes[len(scopes)-1] }

	markIdent := func(id *ast.Identifier, typ int, mods int) {
		if id == nil {
			return
		}
		name := identText(id)
		if name == "" {
			return
		}
		k := Key{
			Line: id.Token.Line,
			Col:  id.Token.Col,
			Len:  len(name),
		}
		out[k] = Classified{Type: typ, Mods: mods}
	}

	type bindingKind int
	const (
		bindNone bindingKind = iota
		bindLocal
		bindParam
		bindNamespace
		bindFunc
	)

	resolveBinding := func(name string) (bindingKind, bool) {
		if name == "" {
			return bindNone, false
		}
		for i := len(scopes) - 1; i >= 0; i-- {
			if scopes[i].locals[name] {
				return bindLocal, true
			}
			if scopes[i].params[name] {
				return bindParam, true
			}
			if scopes[i].funcs[name] {
				return bindFunc, true
			}
			if scopes[i].namespaces[name] {
				return bindNamespace, true
			}
		}
		return bindNone, false
	}

	var walkStmt func(s ast.Statement)
	var walkExpr func(e ast.Expression)

	walkStmt = func(s ast.Statement) {
		switch n := s.(type) {
		case *ast.FuncStatement:
			markIdent(n.Name, ttFunction, modDecl)
			if n.Name != nil {
				cur().funcs[identText(n.Name)] = true
			}

			push()
			for _, p := range n.Parameters {
				pName := identText(p)
				if pName != "" {
					cur().params[pName] = true
				}
				markIdent(p, ttParameter, modDecl)
			}
			if n.Body != nil {
				for _, st := range n.Body.Statements {
					walkStmt(st)
				}
			}
			pop()

		case *ast.AssignStatement:
			if n.Name != nil {
				name := identText(n.Name)
				switch kind, ok := resolveBinding(name); {
				case ok && kind == bindParam:
					markIdent(n.Name, ttParameter, 0)
				case ok && kind == bindLocal:
					markIdent(n.Name, ttVariable, 0)
				case ok && kind == bindNamespace:
					markIdent(n.Name, ttNamespace, 0)
				default:
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n.Name, ttVariable, mods)
				}
			}
			walkExpr(n.Value)

		case *ast.IndexAssignStatement:
			walkExpr(n.Left)
			walkExpr(n.Value)

		case *ast.MemberAssignStatement:
			walkExpr(n.Object)
			walkExpr(n.Value)

		case *ast.ImportStatement:
			if n.Alias != nil {
				a := identText(n.Alias)
				if a != "" {
					cur().namespaces[a] = true
				}
				markIdent(n.Alias, ttNamespace, modDecl)
			}

		case *ast.FromImportStatement:
			for _, it := range n.Items {
				if it.Alias != nil {
					name := identText(it.Alias)
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(it.Alias, ttVariable, mods)
				} else if it.Name != nil {
					name := identText(it.Name)
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(it.Name, ttVariable, mods)
				}
			}

		case *ast.ExportStatement:
			if n.Stmt != nil {
				walkStmt(n.Stmt)
			}

		case *ast.ReturnStatement:
			walkExpr(n.ReturnValue)

		case *ast.DeferStatement:
			walkExpr(n.Call)

		case *ast.ThrowStatement:
			walkExpr(n.Value)

		case *ast.ExpressionStatement:
			walkExpr(n.Expression)

		case *ast.BlockStatement:
			push()
			for _, st := range n.Statements {
				walkStmt(st)
			}
			pop()

		case *ast.TryStatement:
			if n.TryBlock != nil {
				walkStmt(n.TryBlock)
			}
			if n.CatchBlock != nil {
				push()
				if n.CatchName != nil {
					name := identText(n.CatchName)
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n.CatchName, ttVariable, mods)
				}
				walkStmt(n.CatchBlock)
				pop()
			}
			if n.FinallyBlock != nil {
				walkStmt(n.FinallyBlock)
			}

		case *ast.IfStatement:
			walkExpr(n.Condition)
			walkStmt(n.Consequence)
			if n.Alternative != nil {
				walkStmt(n.Alternative)
			}

		case *ast.WhileStatement:
			walkExpr(n.Condition)
			walkStmt(n.Body)

		case *ast.ForStatement:
			push()
			if n.Init != nil {
				walkStmt(n.Init)
			}
			if n.Cond != nil {
				walkExpr(n.Cond)
			}
			if n.Post != nil {
				walkStmt(n.Post)
			}
			if n.Body != nil {
				walkStmt(n.Body)
			}
			pop()

		case *ast.ForInStatement:
			push()
			if n.Var != nil {
				name := identText(n.Var)
				cur().locals[name] = true
				mods := modDecl
				if isAllCapsIdent(name) {
					mods |= modReadonly
				}
				markIdent(n.Var, ttVariable, mods)
			}
			walkExpr(n.Iterable)
			walkStmt(n.Body)
			pop()

		case *ast.SwitchStatement:
			walkExpr(n.Value)
			for _, c := range n.Cases {
				for _, val := range c.Values {
					walkExpr(val)
				}
				if c.Body != nil {
					walkStmt(c.Body)
				}
			}
			if n.Default != nil {
				walkStmt(n.Default)
			}
		}
	}

	walkExpr = func(e ast.Expression) {
		switch n := e.(type) {
		case *ast.Identifier:
			name := identText(n)
			if kind, ok := resolveBinding(name); ok {
				switch kind {
				case bindLocal:
					mods := 0
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n, ttVariable, mods)
				case bindParam:
					markIdent(n, ttParameter, 0)
				case bindNamespace:
					markIdent(n, ttNamespace, 0)
				case bindFunc:
					markIdent(n, ttFunction, 0)
				}
				return
			}
			if builtinFunctions[name] {
				markIdent(n, ttFunction, 0)
				return
			}
			markIdent(n, ttVariable, 0)

		case *ast.CallExpression:
			if id, ok := n.Function.(*ast.Identifier); ok {
				name := identText(id)
				if kind, ok := resolveBinding(name); ok {
					switch kind {
					case bindLocal:
						mods := 0
						if isAllCapsIdent(name) {
							mods |= modReadonly
						}
						markIdent(id, ttVariable, mods)
					case bindParam:
						markIdent(id, ttParameter, 0)
					case bindNamespace:
						markIdent(id, ttNamespace, 0)
					case bindFunc:
						markIdent(id, ttFunction, 0)
					}
				} else {
					markIdent(id, ttFunction, 0)
				}
			} else {
				walkExpr(n.Function)
			}
			for _, a := range n.Arguments {
				walkExpr(a)
			}

		case *ast.MemberExpression:
			if id, ok := n.Object.(*ast.Identifier); ok {
				if kind, ok := resolveBinding(identText(id)); ok && kind == bindNamespace {
					markIdent(id, ttNamespace, 0)
				} else {
					walkExpr(n.Object)
				}
			} else {
				walkExpr(n.Object)
			}

		case *ast.InfixExpression:
			walkExpr(n.Left)
			walkExpr(n.Right)

		case *ast.PrefixExpression:
			walkExpr(n.Right)

		case *ast.IndexExpression:
			walkExpr(n.Left)
			walkExpr(n.Index)

		case *ast.SliceExpression:
			walkExpr(n.Left)
			if n.Low != nil {
				walkExpr(n.Low)
			}
			if n.High != nil {
				walkExpr(n.High)
			}

		case *ast.ListLiteral:
			for _, el := range n.Elements {
				walkExpr(el)
			}

		case *ast.DictLiteral:
			for _, p := range n.Pairs {
				walkExpr(p.Key)
				walkExpr(p.Value)
			}

		case *ast.MatchExpression:
			walkExpr(n.Value)
			for _, c := range n.Cases {
				for _, val := range c.Values {
					walkExpr(val)
				}
				walkExpr(c.Result)
			}
			if n.Default != nil {
				walkExpr(n.Default)
			}
		}
	}

	for _, st := range program.Statements {
		walkStmt(st)
	}
	return out
}
