package lsp

import (
	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/token"
)

type callRange struct {
	start Pos
	end   Pos
}

func findCallAt(text string, prog *ast.Program, pos Pos) (*ast.CallExpression, int) {
	calls := map[posKey]*ast.CallExpression{}
	collectCalls(prog, func(c *ast.CallExpression) {
		if c == nil {
			return
		}
		calls[posKey{Line: c.Token.Line, Col: c.Token.Col}] = c
	})

	ranges := map[*ast.CallExpression]callRange{}
	lx := lexer.New(text)
	type frame struct {
		isCall bool
		call   *ast.CallExpression
	}
	stack := []frame{}
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		switch tok.Type {
		case token.LPAREN:
			if c, ok := calls[posKey{Line: tok.Line, Col: tok.Col}]; ok {
				stack = append(stack, frame{isCall: true, call: c})
			} else {
				stack = append(stack, frame{isCall: false})
			}
		case token.RPAREN:
			if len(stack) > 0 {
				fr := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if fr.isCall && fr.call != nil {
					ranges[fr.call] = callRange{start: Pos{Line: fr.call.Token.Line, Col: fr.call.Token.Col}, end: Pos{Line: tok.Line, Col: tok.Col}}
				}
			}
		}
	}

	var best *ast.CallExpression
	var bestRange callRange
	for c, r := range ranges {
		if posWithin(pos, r.start, r.end) {
			if best == nil {
				best = c
				bestRange = r
				continue
			}
			if posWithin(r.start, bestRange.start, bestRange.end) && posWithin(r.end, bestRange.start, bestRange.end) {
				best = c
				bestRange = r
			}
		}
	}

	if best == nil {
		return nil, 0
	}

	active := activeParamIndex(text, bestRange.start, pos)
	return best, active
}

func activeParamIndex(text string, start Pos, pos Pos) int {
	lx := lexer.New(text)
	depth := 0
	count := 0
	for {
		tok := lx.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Line < start.Line || (tok.Line == start.Line && tok.Col < start.Col) {
			continue
		}
		if tok.Line > pos.Line || (tok.Line == pos.Line && tok.Col > pos.Col) {
			break
		}
		switch tok.Type {
		case token.LPAREN, token.LBRACKET, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACKET, token.RBRACE:
			if depth > 0 {
				depth--
			}
		case token.COMMA:
			if depth == 1 {
				count++
			}
		}
	}
	return count
}

func collectCalls(node ast.Node, fn func(*ast.CallExpression)) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.Program:
		for _, st := range n.Statements {
			collectCalls(st, fn)
		}
	case *ast.ExpressionStatement:
		collectCalls(n.Expression, fn)
	case *ast.CallExpression:
		fn(n)
		collectCalls(n.Function, fn)
		for _, a := range n.Arguments {
			collectCalls(a, fn)
		}
	case *ast.FuncStatement:
		collectCalls(n.Body, fn)
	case *ast.FunctionLiteral:
		collectCalls(n.Body, fn)
	case *ast.BlockStatement:
		for _, st := range n.Statements {
			collectCalls(st, fn)
		}
	case *ast.AssignStatement:
		collectCalls(n.Value, fn)
	case *ast.IndexAssignStatement:
		collectCalls(n.Left, fn)
		collectCalls(n.Value, fn)
	case *ast.MemberAssignStatement:
		collectCalls(n.Object, fn)
		collectCalls(n.Value, fn)
	case *ast.DestructureAssignStatement:
		collectCalls(n.Value, fn)
	case *ast.AssignExpression:
		collectCalls(n.Left, fn)
		collectCalls(n.Value, fn)
	case *ast.ReturnStatement:
		for _, rv := range n.ReturnValues {
			collectCalls(rv, fn)
		}
	case *ast.DeferStatement:
		collectCalls(n.Call, fn)
	case *ast.ThrowStatement:
		collectCalls(n.Value, fn)
	case *ast.IfStatement:
		collectCalls(n.Condition, fn)
		collectCalls(n.Consequence, fn)
		collectCalls(n.Alternative, fn)
	case *ast.WhileStatement:
		collectCalls(n.Condition, fn)
		collectCalls(n.Body, fn)
	case *ast.ForStatement:
		collectCalls(n.Init, fn)
		collectCalls(n.Cond, fn)
		collectCalls(n.Post, fn)
		collectCalls(n.Body, fn)
	case *ast.ForInStatement:
		collectCalls(n.Iterable, fn)
		collectCalls(n.Body, fn)
	case *ast.SwitchStatement:
		collectCalls(n.Value, fn)
		for _, c := range n.Cases {
			if c != nil {
				for _, v := range c.Values {
					collectCalls(v, fn)
				}
				collectCalls(c.Body, fn)
			}
		}
		collectCalls(n.Default, fn)
	case *ast.MatchExpression:
		collectCalls(n.Value, fn)
		for _, c := range n.Cases {
			for _, v := range c.Values {
				collectCalls(v, fn)
			}
			collectCalls(c.Result, fn)
		}
		collectCalls(n.Default, fn)
	case *ast.InfixExpression:
		collectCalls(n.Left, fn)
		collectCalls(n.Right, fn)
	case *ast.ConditionalExpression:
		collectCalls(n.Cond, fn)
		collectCalls(n.Then, fn)
		collectCalls(n.Else, fn)
	case *ast.CondExpr:
		collectCalls(n.Cond, fn)
		collectCalls(n.Then, fn)
		collectCalls(n.Else, fn)
	case *ast.PrefixExpression:
		collectCalls(n.Right, fn)
	case *ast.SpreadExpression:
		collectCalls(n.Value, fn)
	case *ast.IndexExpression:
		collectCalls(n.Left, fn)
		collectCalls(n.Index, fn)
	case *ast.SliceExpression:
		collectCalls(n.Left, fn)
		collectCalls(n.Low, fn)
		collectCalls(n.High, fn)
		collectCalls(n.Step, fn)
	case *ast.ListLiteral:
		for _, el := range n.Elements {
			collectCalls(el, fn)
		}
	case *ast.ListComprehension:
		collectCalls(n.Seq, fn)
		collectCalls(n.Filter, fn)
		collectCalls(n.Elem, fn)
	case *ast.DictLiteral:
		for _, p := range n.Pairs {
			if p.Shorthand != nil {
				collectCalls(p.Shorthand, fn)
				continue
			}
			collectCalls(p.Key, fn)
			collectCalls(p.Value, fn)
		}
	case *ast.MemberExpression:
		collectCalls(n.Object, fn)
	case *ast.TemplateLiteral:
		collectCalls(n.Tag, fn)
		for _, ex := range n.Exprs {
			collectCalls(ex, fn)
		}
	}
}
