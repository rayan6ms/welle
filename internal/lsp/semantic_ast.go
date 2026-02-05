package lsp

import (
	"welle/internal/ast"
	"welle/internal/token"
)

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
	"print":          true,
	"len":            true,
	"str":            true,
	"join":           true,
	"keys":           true,
	"values":         true,
	"range":          true,
	"append":         true,
	"push":           true,
	"count":          true,
	"remove":         true,
	"get":            true,
	"pop":            true,
	"hasKey":         true,
	"sort":           true,
	"max":            true,
	"abs":            true,
	"sum":            true,
	"reverse":        true,
	"any":            true,
	"all":            true,
	"error":          true,
	"writeFile":      true,
	"sqrt":           true,
	"input":          true,
	"getpass":        true,
	"group_digits":   true,
	"format_float":   true,
	"format_percent": true,
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
				if n.Op == token.WALRUS {
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n.Name, ttVariable, mods)
				} else {
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
			for _, rv := range n.ReturnValues {
				walkExpr(rv)
			}

		case *ast.DeferStatement:
			walkExpr(n.Call)

		case *ast.ThrowStatement:
			walkExpr(n.Value)

		case *ast.ExpressionStatement:
			walkExpr(n.Expression)

		case *ast.DestructureAssignStatement:
			for _, t := range n.Targets {
				if t == nil || t.Name == nil {
					continue
				}
				name := identText(t.Name)
				cur().locals[name] = true
				mods := modDecl
				if isAllCapsIdent(name) {
					mods |= modReadonly
				}
				markIdent(t.Name, ttVariable, mods)
			}
			walkExpr(n.Value)

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
			if n.Destruct {
				if n.Key != nil && n.Key.Value != "_" {
					name := identText(n.Key)
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n.Key, ttVariable, mods)
				}
				if n.Value != nil && n.Value.Value != "_" {
					name := identText(n.Value)
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(n.Value, ttVariable, mods)
				}
			} else if n.Var != nil {
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

		case *ast.ConditionalExpression:
			walkExpr(n.Cond)
			walkExpr(n.Then)
			walkExpr(n.Else)

		case *ast.CondExpr:
			walkExpr(n.Cond)
			walkExpr(n.Then)
			walkExpr(n.Else)

		case *ast.PrefixExpression:
			walkExpr(n.Right)

		case *ast.SpreadExpression:
			walkExpr(n.Value)

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
			if n.Step != nil {
				walkExpr(n.Step)
			}

		case *ast.ListLiteral:
			for _, el := range n.Elements {
				walkExpr(el)
			}

		case *ast.ListComprehension:
			walkExpr(n.Seq)
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
			if n.Filter != nil {
				walkExpr(n.Filter)
			}
			walkExpr(n.Elem)
			pop()

		case *ast.DictLiteral:
			for _, p := range n.Pairs {
				if p.Shorthand != nil {
					walkExpr(p.Shorthand)
					continue
				}
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

		case *ast.FunctionLiteral:
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

		case *ast.TemplateLiteral:
			if n.Tag != nil {
				walkExpr(n.Tag)
			}
			for _, ex := range n.Exprs {
				walkExpr(ex)
			}

		case *ast.AssignExpression:
			switch left := n.Left.(type) {
			case *ast.Identifier:
				name := identText(left)
				if n.Op == token.WALRUS {
					cur().locals[name] = true
					mods := modDecl
					if isAllCapsIdent(name) {
						mods |= modReadonly
					}
					markIdent(left, ttVariable, mods)
				} else {
					switch kind, ok := resolveBinding(name); {
					case ok && kind == bindParam:
						markIdent(left, ttParameter, 0)
					case ok && kind == bindLocal:
						markIdent(left, ttVariable, 0)
					case ok && kind == bindNamespace:
						markIdent(left, ttNamespace, 0)
					default:
						cur().locals[name] = true
						mods := modDecl
						if isAllCapsIdent(name) {
							mods |= modReadonly
						}
						markIdent(left, ttVariable, mods)
					}
				}
			case *ast.IndexExpression:
				walkExpr(left.Left)
				walkExpr(left.Index)
			case *ast.MemberExpression:
				walkExpr(left.Object)
			}
			walkExpr(n.Value)
		}
	}

	for _, st := range program.Statements {
		walkStmt(st)
	}
	return out
}
