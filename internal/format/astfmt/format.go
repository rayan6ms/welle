package astfmt

import (
	"fmt"
	"strings"

	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/parser"
)

// FormatAST formats source using the AST-aware formatter.
// indent is the number of spaces per indentation level.
func FormatAST(src []byte, indent int) ([]byte, error) {
	if indent <= 0 {
		indent = 2
	}
	return FormatASTWithIndent(src, strings.Repeat(" ", indent))
}

// FormatASTWithIndent formats source using the AST-aware formatter and an explicit indent string.
func FormatASTWithIndent(src []byte, indent string) ([]byte, error) {
	if indent == "" {
		indent = "  "
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse errors: %s", strings.Join(p.Errors(), "; "))
	}

	lines := splitLines(string(src))
	comments := scanComments(string(src))
	index := buildScopeIndex(program, lines)
	assignComments(index.root, comments)

	printer := newPrinter(indent, lines, index)
	printer.printProgram(program)
	out := printer.bytes()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out, nil
}

func splitLines(src string) []string {
	// Normalize CRLF to LF for consistent line math.
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return strings.Split(src, "\n")
}

// scopeIndex maps block statements to their formatting scopes.
// The root scope corresponds to the program.
type scopeIndex struct {
	root     *blockScope
	byBlock  map[*ast.BlockStatement]*blockScope
	bySwitch map[*ast.SwitchStatement]*blockScope
	byMatch  map[*ast.MatchExpression]*blockScope
}

func buildScopeIndex(program *ast.Program, lines []string) *scopeIndex {
	root := &blockScope{startLine: 1, endLine: len(lines), statements: program.Statements}
	idx := &scopeIndex{
		root:     root,
		byBlock:  map[*ast.BlockStatement]*blockScope{},
		bySwitch: map[*ast.SwitchStatement]*blockScope{},
		byMatch:  map[*ast.MatchExpression]*blockScope{},
	}
	for _, stmt := range program.Statements {
		idx.addScopesForStatement(root, stmt)
	}
	return idx
}

func (s *scopeIndex) addScopesForStatement(parent *blockScope, stmt ast.Statement) {
	switch st := stmt.(type) {
	case *ast.BlockStatement:
		s.addBlockScope(parent, st)
	case *ast.IfStatement:
		if st.Consequence != nil {
			s.addScopesForStatement(parent, st.Consequence)
		}
		if st.Alternative != nil {
			s.addScopesForStatement(parent, st.Alternative)
		}
	case *ast.WhileStatement:
		if st.Body != nil {
			s.addBlockScope(parent, st.Body)
		}
	case *ast.ForStatement:
		if st.Body != nil {
			s.addBlockScope(parent, st.Body)
		}
	case *ast.ForInStatement:
		if st.Body != nil {
			s.addBlockScope(parent, st.Body)
		}
	case *ast.SwitchStatement:
		s.addSwitchScope(parent, st)
	case *ast.TryStatement:
		if st.TryBlock != nil {
			s.addBlockScope(parent, st.TryBlock)
		}
		if st.CatchBlock != nil {
			s.addBlockScope(parent, st.CatchBlock)
		}
		if st.FinallyBlock != nil {
			s.addBlockScope(parent, st.FinallyBlock)
		}
	case *ast.FuncStatement:
		if st.Body != nil {
			s.addBlockScope(parent, st.Body)
		}
	case *ast.ExpressionStatement:
		s.addScopesForExpression(parent, st.Expression)
	case *ast.AssignStatement:
		s.addScopesForExpression(parent, st.Value)
	case *ast.IndexAssignStatement:
		s.addScopesForExpression(parent, st.Left)
		s.addScopesForExpression(parent, st.Value)
	case *ast.MemberAssignStatement:
		s.addScopesForExpression(parent, st.Object)
		s.addScopesForExpression(parent, st.Value)
	case *ast.ReturnStatement:
		for _, v := range st.ReturnValues {
			s.addScopesForExpression(parent, v)
		}
	case *ast.DestructureAssignStatement:
		s.addScopesForExpression(parent, st.Value)
	case *ast.DeferStatement:
		s.addScopesForExpression(parent, st.Call)
	case *ast.ThrowStatement:
		s.addScopesForExpression(parent, st.Value)
	case *ast.ExportStatement:
		if st.Stmt != nil {
			s.addScopesForStatement(parent, st.Stmt)
		}
	case *ast.ImportStatement, *ast.FromImportStatement, *ast.BreakStatement, *ast.ContinueStatement, *ast.PassStatement:
		return
	}
}

func (s *scopeIndex) addScopesForExpression(parent *blockScope, expr ast.Expression) {
	switch e := expr.(type) {
	case *ast.FunctionLiteral:
		if e.Body != nil {
			s.addBlockScope(parent, e.Body)
		}
	case *ast.InfixExpression:
		s.addScopesForExpression(parent, e.Left)
		s.addScopesForExpression(parent, e.Right)
	case *ast.ConditionalExpression:
		s.addScopesForExpression(parent, e.Cond)
		s.addScopesForExpression(parent, e.Then)
		s.addScopesForExpression(parent, e.Else)
	case *ast.CondExpr:
		s.addScopesForExpression(parent, e.Cond)
		s.addScopesForExpression(parent, e.Then)
		s.addScopesForExpression(parent, e.Else)
	case *ast.PrefixExpression:
		s.addScopesForExpression(parent, e.Right)
	case *ast.AssignExpression:
		s.addScopesForExpression(parent, e.Left)
		s.addScopesForExpression(parent, e.Value)
	case *ast.SpreadExpression:
		s.addScopesForExpression(parent, e.Value)
	case *ast.CallExpression:
		s.addScopesForExpression(parent, e.Function)
		for _, a := range e.Arguments {
			s.addScopesForExpression(parent, a)
		}
	case *ast.MemberExpression:
		s.addScopesForExpression(parent, e.Object)
	case *ast.IndexExpression:
		s.addScopesForExpression(parent, e.Left)
		s.addScopesForExpression(parent, e.Index)
	case *ast.SliceExpression:
		s.addScopesForExpression(parent, e.Left)
		s.addScopesForExpression(parent, e.Low)
		s.addScopesForExpression(parent, e.High)
		s.addScopesForExpression(parent, e.Step)
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			s.addScopesForExpression(parent, el)
		}
	case *ast.ListComprehension:
		s.addScopesForExpression(parent, e.Seq)
		s.addScopesForExpression(parent, e.Filter)
		s.addScopesForExpression(parent, e.Elem)
	case *ast.TupleLiteral:
		for _, el := range e.Elements {
			s.addScopesForExpression(parent, el)
		}
	case *ast.DictLiteral:
		for _, p := range e.Pairs {
			if p.Shorthand != nil {
				s.addScopesForExpression(parent, p.Shorthand)
				continue
			}
			s.addScopesForExpression(parent, p.Key)
			s.addScopesForExpression(parent, p.Value)
		}
	case *ast.MatchExpression:
		s.addMatchScope(parent, e)
	case *ast.TemplateLiteral:
		if e.Tag != nil {
			s.addScopesForExpression(parent, e.Tag)
		}
		for _, ex := range e.Exprs {
			s.addScopesForExpression(parent, ex)
		}
	}
}

func (s *scopeIndex) addBlockScope(parent *blockScope, block *ast.BlockStatement) {
	if block == nil {
		return
	}
	scope := &blockScope{startLine: block.Token.Line, startCol: block.Token.Col, endLine: endLineStatement(block), statements: block.Statements}
	parent.children = append(parent.children, scope)
	s.byBlock[block] = scope
	for _, st := range block.Statements {
		s.addScopesForStatement(scope, st)
	}
}

func (s *scopeIndex) addSwitchScope(parent *blockScope, stmt *ast.SwitchStatement) {
	if stmt == nil {
		return
	}
	scope := &blockScope{startLine: stmt.Token.Line, startCol: stmt.Token.Col, endLine: endLineStatement(stmt)}
	parent.children = append(parent.children, scope)
	s.bySwitch[stmt] = scope

	for _, cc := range stmt.Cases {
		if cc.Body != nil {
			s.addBlockScope(scope, cc.Body)
		}
	}
	if stmt.Default != nil {
		s.addBlockScope(scope, stmt.Default)
	}
}

func (s *scopeIndex) addMatchScope(parent *blockScope, expr *ast.MatchExpression) {
	if expr == nil {
		return
	}
	scope := &blockScope{startLine: expr.Token.Line, startCol: expr.Token.Col, endLine: endLineExpr(expr)}
	parent.children = append(parent.children, scope)
	s.byMatch[expr] = scope

	for _, c := range expr.Cases {
		for _, v := range c.Values {
			s.addScopesForExpression(scope, v)
		}
		s.addScopesForExpression(scope, c.Result)
	}
	if expr.Default != nil {
		s.addScopesForExpression(scope, expr.Default)
	}
}

// blockScope represents a list of statements and the comments that belong to it.
type blockScope struct {
	startLine  int
	startCol   int
	endLine    int
	statements []ast.Statement
	children   []*blockScope
	comments   []Comment
}

func (b *blockScope) contains(line int) bool {
	return line >= b.startLine && line <= b.endLine
}

func assignComments(scope *blockScope, comments []Comment) {
	for i := range comments {
		c := comments[i]
		assignCommentToScope(scope, c)
	}
}

func assignCommentToScope(scope *blockScope, c Comment) {
	for _, child := range scope.children {
		if child.contains(c.StartLine) {
			if c.StartLine == child.startLine {
				break
			}
			assignCommentToScope(child, c)
			return
		}
	}
	scope.comments = append(scope.comments, c)
}
