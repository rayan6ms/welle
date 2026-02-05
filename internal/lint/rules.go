package lint

import (
	"fmt"

	"welle/internal/ast"
	"welle/internal/diag"
	"welle/internal/token"
)

type symKind int

const (
	kindVar symKind = iota
	kindParam
	kindFunc
	kindImport
)

type sym struct {
	name string
	tok  token.Token
	used bool
	kind symKind
}

type scope struct {
	parent *scope
	syms   map[string]*sym
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, syms: map[string]*sym{}}
}

func (s *scope) lookup(name string) *sym {
	for sc := s; sc != nil; sc = sc.parent {
		if v, ok := sc.syms[name]; ok {
			return v
		}
	}
	return nil
}

func (s *scope) lookupHere(name string) *sym {
	return s.syms[name]
}

type Runner struct {
	diags []diag.Diagnostic
	sc    *scope
	opts  Options
}

func (r *Runner) warn(tok token.Token, code string, msg string) {
	r.diags = append(r.diags, diag.Diagnostic{
		Code:     code,
		Message:  msg,
		Severity: diag.SeverityWarning,
		Range: diag.Range{
			Line:   tok.Line,
			Col:    tok.Col,
			Length: tokLength(tok),
		},
	})
}

func tokLength(tok token.Token) int {
	if tok.Literal == "" {
		return 1
	}
	return len([]rune(tok.Literal))
}

func (r *Runner) push() { r.sc = newScope(r.sc) }

func (r *Runner) pop() {
	for name, sm := range r.sc.syms {
		if name == "_" {
			continue
		}
		switch sm.kind {
		case kindVar:
			if !sm.used {
				r.warn(sm.tok, "WL0001", fmt.Sprintf("unused variable: %s", name))
			}
		case kindParam:
			if !sm.used {
				r.warn(sm.tok, "WL0002", fmt.Sprintf("unused parameter: %s", name))
			}
		}
	}
	r.sc = r.sc.parent
}

func (r *Runner) declare(name string, tok token.Token, k symKind) {
	if name == "" {
		return
	}
	if r.opts.CheckShadowing && r.sc.parent != nil && r.sc.parent.lookup(name) != nil && r.sc.lookupHere(name) == nil {
		r.warn(tok, "WL0004", fmt.Sprintf("variable '%s' shadows outer variable", name))
	}
	r.sc.syms[name] = &sym{name: name, tok: tok, kind: k}
}

func (r *Runner) use(name string) {
	if name == "" {
		return
	}
	if sm := r.sc.lookup(name); sm != nil {
		sm.used = true
	}
}

func (r *Runner) walkProgram(p *ast.Program) {
	for _, st := range p.Statements {
		r.walkStmt(st)
	}
}

func (r *Runner) walkBlock(b *ast.BlockStatement) {
	r.push()
	r.walkBlockWithScope(b)
	r.pop()
}

func (r *Runner) walkBlockWithScope(b *ast.BlockStatement) {
	if b == nil {
		return
	}
	terminated := false
	for _, st := range b.Statements {
		if terminated {
			r.warn(firstTokenOfStmt(st), "WL0003", "unreachable code")
		}
		r.walkStmt(st)
		if isTerminator(st) {
			terminated = true
		}
	}
}

func isTerminator(st ast.Statement) bool {
	switch st.(type) {
	case *ast.ReturnStatement:
		return true
	case *ast.ThrowStatement:
		return true
	}
	return false
}

func firstTokenOfStmt(st ast.Statement) token.Token {
	switch n := st.(type) {
	case *ast.ExpressionStatement:
		return n.Token
	case *ast.AssignStatement:
		return n.Token
	case *ast.IndexAssignStatement:
		return n.Token
	case *ast.MemberAssignStatement:
		return n.Token
	case *ast.ReturnStatement:
		return n.Token
	case *ast.DeferStatement:
		return n.Token
	case *ast.ThrowStatement:
		return n.Token
	case *ast.BreakStatement:
		return n.Token
	case *ast.ContinueStatement:
		return n.Token
	case *ast.PassStatement:
		return n.Token
	case *ast.ImportStatement:
		return n.Token
	case *ast.FromImportStatement:
		return n.Token
	case *ast.ExportStatement:
		return n.Token
	case *ast.BlockStatement:
		return n.Token
	case *ast.TryStatement:
		return n.Token
	case *ast.IfStatement:
		return n.Token
	case *ast.WhileStatement:
		return n.Token
	case *ast.ForStatement:
		return n.Token
	case *ast.ForInStatement:
		return n.Token
	case *ast.SwitchStatement:
		return n.Token
	case *ast.FuncStatement:
		return n.Token
	default:
		return token.Token{Line: 1, Col: 1, Literal: ""}
	}
}

func (r *Runner) walkStmt(st ast.Statement) {
	if st == nil {
		return
	}
	switch n := st.(type) {
	case *ast.BlockStatement:
		r.walkBlock(n)

	case *ast.FuncStatement:
		if n.Name != nil {
			r.declare(n.Name.Value, n.Name.Token, kindFunc)
		}
		r.push()
		for _, p := range n.Parameters {
			if p != nil {
				r.declare(p.Value, p.Token, kindParam)
			}
		}
		r.walkBlockWithScope(n.Body)
		r.pop()

	case *ast.AssignStatement:
		if n.Name != nil && r.sc.lookupHere(n.Name.Value) == nil {
			r.declare(n.Name.Value, n.Name.Token, kindVar)
		}
		r.walkExpr(n.Value)

	case *ast.IndexAssignStatement:
		r.walkExpr(n.Left)
		r.walkExpr(n.Value)

	case *ast.MemberAssignStatement:
		r.walkExpr(n.Object)
		r.walkExpr(n.Value)

	case *ast.DestructureAssignStatement:
		for _, t := range n.Targets {
			if t == nil || t.Name == nil || t.Name.Value == "_" {
				continue
			}
			if r.sc.lookupHere(t.Name.Value) == nil {
				r.declare(t.Name.Value, t.Name.Token, kindVar)
			}
		}
		r.walkExpr(n.Value)

	case *ast.ReturnStatement:
		for _, rv := range n.ReturnValues {
			r.walkExpr(rv)
		}

	case *ast.DeferStatement:
		r.walkExpr(n.Call)

	case *ast.ThrowStatement:
		r.walkExpr(n.Value)

	case *ast.ExpressionStatement:
		r.walkExpr(n.Expression)

	case *ast.PassStatement:
		return

	case *ast.IfStatement:
		r.walkExpr(n.Condition)
		if n.Consequence != nil {
			r.walkStmt(n.Consequence)
		}
		if n.Alternative != nil {
			r.walkStmt(n.Alternative)
		}

	case *ast.WhileStatement:
		r.walkExpr(n.Condition)
		r.walkBlock(n.Body)

	case *ast.ForStatement:
		r.push()
		if n.Init != nil {
			r.walkStmt(n.Init)
		}
		if n.Cond != nil {
			r.walkExpr(n.Cond)
		}
		if n.Post != nil {
			r.walkStmt(n.Post)
		}
		r.walkBlock(n.Body)
		r.pop()

	case *ast.ForInStatement:
		r.push()
		if n.Destruct {
			if n.Key != nil && n.Key.Value != "_" {
				r.declare(n.Key.Value, n.Key.Token, kindVar)
			}
			if n.Value != nil && n.Value.Value != "_" {
				r.declare(n.Value.Value, n.Value.Token, kindVar)
			}
		} else if n.Var != nil {
			r.declare(n.Var.Value, n.Var.Token, kindVar)
		}
		r.walkExpr(n.Iterable)
		r.walkBlock(n.Body)
		r.pop()

	case *ast.SwitchStatement:
		r.walkExpr(n.Value)
		for _, c := range n.Cases {
			if c == nil {
				continue
			}
			for _, v := range c.Values {
				r.walkExpr(v)
			}
			r.walkBlock(c.Body)
		}
		if n.Default != nil {
			r.walkBlock(n.Default)
		}

	case *ast.TryStatement:
		r.walkBlock(n.TryBlock)
		if n.CatchBlock != nil {
			r.push()
			if n.CatchName != nil {
				r.declare(n.CatchName.Value, n.CatchName.Token, kindVar)
			}
			r.walkBlockWithScope(n.CatchBlock)
			r.pop()
		}
		if n.FinallyBlock != nil {
			r.walkBlock(n.FinallyBlock)
		}

	case *ast.ImportStatement:
		if n.Alias != nil {
			r.declare(n.Alias.Value, n.Alias.Token, kindImport)
		}

	case *ast.FromImportStatement:
		for _, it := range n.Items {
			if it.Alias != nil {
				r.declare(it.Alias.Value, it.Alias.Token, kindImport)
				continue
			}
			if it.Name != nil {
				r.declare(it.Name.Value, it.Name.Token, kindImport)
			}
		}

	case *ast.ExportStatement:
		if n.Stmt != nil {
			r.walkStmt(n.Stmt)
		}

	default:
	}
}

func (r *Runner) walkExpr(e ast.Expression) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.Identifier:
		r.use(n.Value)

	case *ast.InfixExpression:
		r.walkExpr(n.Left)
		r.walkExpr(n.Right)

	case *ast.ConditionalExpression:
		r.walkExpr(n.Cond)
		r.walkExpr(n.Then)
		r.walkExpr(n.Else)

	case *ast.CondExpr:
		r.walkExpr(n.Cond)
		r.walkExpr(n.Then)
		r.walkExpr(n.Else)

	case *ast.PrefixExpression:
		r.walkExpr(n.Right)

	case *ast.CallExpression:
		r.walkExpr(n.Function)
		for _, a := range n.Arguments {
			r.walkExpr(a)
		}

	case *ast.SpreadExpression:
		r.walkExpr(n.Value)

	case *ast.MemberExpression:
		r.walkExpr(n.Object)

	case *ast.IndexExpression:
		r.walkExpr(n.Left)
		r.walkExpr(n.Index)

	case *ast.SliceExpression:
		r.walkExpr(n.Left)
		r.walkExpr(n.Low)
		r.walkExpr(n.High)
		r.walkExpr(n.Step)

	case *ast.ListLiteral:
		for _, el := range n.Elements {
			r.walkExpr(el)
		}

	case *ast.ListComprehension:
		r.walkExpr(n.Seq)
		r.push()
		if n.Var != nil {
			r.declare(n.Var.Value, n.Var.Token, kindVar)
		}
		r.walkExpr(n.Filter)
		r.walkExpr(n.Elem)
		r.pop()

	case *ast.DictLiteral:
		for _, p := range n.Pairs {
			if p.Shorthand != nil {
				r.walkExpr(p.Shorthand)
				continue
			}
			r.walkExpr(p.Key)
			r.walkExpr(p.Value)
		}

	case *ast.MatchExpression:
		r.walkExpr(n.Value)
		for _, c := range n.Cases {
			if c == nil {
				continue
			}
			for _, v := range c.Values {
				r.walkExpr(v)
			}
			r.walkExpr(c.Result)
		}
		r.walkExpr(n.Default)

	case *ast.FunctionLiteral:
		r.push()
		for _, p := range n.Parameters {
			if p != nil {
				r.declare(p.Value, p.Token, kindParam)
			}
		}
		r.walkBlockWithScope(n.Body)
		r.pop()

	case *ast.TemplateLiteral:
		r.walkExpr(n.Tag)
		for _, ex := range n.Exprs {
			r.walkExpr(ex)
		}

	case *ast.BooleanLiteral, *ast.IntegerLiteral, *ast.StringLiteral:
		return

	case *ast.AssignExpression:
		switch left := n.Left.(type) {
		case *ast.Identifier:
			if r.sc.lookupHere(left.Value) == nil {
				r.declare(left.Value, left.Token, kindVar)
			}
		case *ast.IndexExpression:
			r.walkExpr(left.Left)
			r.walkExpr(left.Index)
		case *ast.MemberExpression:
			r.walkExpr(left.Object)
		default:
			// ignore
		}
		r.walkExpr(n.Value)

	default:
	}
}
