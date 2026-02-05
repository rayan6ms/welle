package astfmt

import (
	"bytes"
	"strings"

	"welle/internal/ast"
	"welle/internal/token"
)

type Printer struct {
	indent      string
	level       int
	atLineStart bool
	buf         bytes.Buffer
	lines       []string
	index       *scopeIndex
}

func newPrinter(indent string, lines []string, index *scopeIndex) *Printer {
	return &Printer{
		indent:      indent,
		level:       0,
		atLineStart: true,
		lines:       lines,
		index:       index,
	}
}

func (p *Printer) bytes() []byte {
	return p.buf.Bytes()
}

func (p *Printer) write(s string) {
	if s == "" {
		return
	}
	if p.atLineStart {
		for i := 0; i < p.level; i++ {
			p.buf.WriteString(p.indent)
		}
		p.atLineStart = false
	}
	p.buf.WriteString(s)
}

func (p *Printer) newline() {
	p.buf.WriteByte('\n')
	p.atLineStart = true
}

func (p *Printer) blankLine() {
	if !p.atLineStart {
		p.newline()
	}
	p.newline()
}

func (p *Printer) ensureLineStart() {
	if !p.atLineStart {
		p.newline()
	}
}

func (p *Printer) hasBlankLineBetween(a, b int) bool {
	if b <= a+1 {
		return false
	}
	if a < 0 {
		a = 0
	}
	if b > len(p.lines) {
		b = len(p.lines)
	}
	for i := a + 1; i <= b-1 && i <= len(p.lines); i++ {
		if strings.TrimSpace(p.lines[i-1]) == "" {
			return true
		}
	}
	return false
}

func (p *Printer) printProgram(program *ast.Program) {
	p.printStatementList(program.Statements, p.index.root)
}

func (p *Printer) printStatementList(stmts []ast.Statement, scope *blockScope) {
	if scope == nil {
		for _, stmt := range stmts {
			p.printStatement(stmt, nil)
		}
		return
	}

	infos := make([]stmtInfo, 0, len(stmts))
	for _, st := range stmts {
		infos = append(infos, stmtInfo{stmt: st, startLine: startLineStatement(st), endLine: endLineStatement(st)})
	}

	endLineToStmt := map[int]ast.Statement{}
	startLineToStmt := map[int]ast.Statement{}
	headerLineToStmt := map[int]ast.Statement{}
	for _, info := range infos {
		endLineToStmt[info.endLine] = info.stmt
		if _, ok := startLineToStmt[info.startLine]; !ok {
			startLineToStmt[info.startLine] = info.stmt
		}
		for _, line := range headerLinesForStatement(info.stmt) {
			if line <= 0 {
				continue
			}
			if _, ok := headerLineToStmt[line]; !ok {
				headerLineToStmt[line] = info.stmt
			}
		}
	}

	trailing := map[ast.Statement][]Comment{}
	leading := make([]Comment, 0, len(scope.comments))
	for _, c := range scope.comments {
		if isInlineComment(c, p.lines) {
			if st := headerLineToStmt[c.StartLine]; st != nil {
				trailing[st] = append(trailing[st], c)
				continue
			}
			if st := startLineToStmt[c.StartLine]; st != nil {
				trailing[st] = append(trailing[st], c)
				continue
			}
			if st := endLineToStmt[c.StartLine]; st != nil {
				trailing[st] = append(trailing[st], c)
				continue
			}
		}
		leading = append(leading, c)
	}

	commentIdx := 0
	lastOrigLine := scope.startLine - 1

	for _, info := range infos {
		for commentIdx < len(leading) && leading[commentIdx].StartLine < info.startLine {
			c := leading[commentIdx]
			p.prepareItem(lastOrigLine, c.StartLine)
			p.printComment(c)
			lastOrigLine = c.EndLine
			commentIdx++
		}

		p.prepareItem(lastOrigLine, info.startLine)
		p.printStatement(info.stmt, trailing[info.stmt])
		lastOrigLine = info.endLine
	}

	for commentIdx < len(leading) {
		c := leading[commentIdx]
		p.prepareItem(lastOrigLine, c.StartLine)
		p.printComment(c)
		lastOrigLine = c.EndLine
		commentIdx++
	}
}

func (p *Printer) prepareItem(lastOrigLine, nextLine int) {
	if p.hasBlankLineBetween(lastOrigLine, nextLine) {
		p.blankLine()
		return
	}
	p.ensureLineStart()
}

type stmtInfo struct {
	stmt      ast.Statement
	startLine int
	endLine   int
}

func (p *Printer) printComment(c Comment) {
	lines := strings.Split(c.Raw, "\n")
	for i, line := range lines {
		if i > 0 {
			p.newline()
		}
		p.write(line)
	}
	p.newline()
}

func isInlineComment(c Comment, lines []string) bool {
	if c.StartLine <= 0 || c.StartLine > len(lines) {
		return false
	}
	line := lines[c.StartLine-1]
	if c.StartCol <= 0 {
		return false
	}
	col := c.StartCol - 1
	if col > len(line) {
		col = len(line)
	}
	prefix := line[:col]
	return strings.TrimSpace(prefix) != ""
}

func (p *Printer) inlineCommentAfterBrace(c Comment) bool {
	if c.StartLine <= 0 || c.StartLine > len(p.lines) {
		return false
	}
	if c.StartCol <= 0 {
		return false
	}
	line := p.lines[c.StartLine-1]
	col := c.StartCol - 1
	if col > len(line) {
		col = len(line)
	}
	prefix := line[:col]
	lastOpen := strings.LastIndex(prefix, "{")
	lastClose := strings.LastIndex(prefix, "}")
	if lastClose == -1 {
		return false
	}
	if lastOpen == -1 {
		return true
	}
	return lastClose > lastOpen
}

func (p *Printer) printStatement(stmt ast.Statement, trailing []Comment) {
	trailingAfter := trailing
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if m, ok := s.Expression.(*ast.MatchExpression); ok {
			var header []Comment
			var footer []Comment
			for _, c := range trailing {
				if c.StartLine == m.Token.Line {
					header = append(header, c)
				} else {
					footer = append(footer, c)
				}
			}
			p.printMatchExpressionWithHeader(m, header)
			trailingAfter = footer
		} else {
			p.formatExpr(s.Expression, precLowest)
		}
	case *ast.AssignStatement:
		p.formatAssign(s.Name, s.OpToken, s.Value)
	case *ast.IndexAssignStatement:
		p.formatExpr(s.Left, precLowest)
		p.write(" ")
		p.write(assignOpLiteral(s.Token, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.MemberAssignStatement:
		p.formatExpr(s.Object, precCall)
		p.write(".")
		p.write(s.Property.Value)
		p.write(" ")
		p.write(assignOpLiteral(s.Token, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.ReturnStatement:
		p.write("return")
		if len(s.ReturnValues) > 0 {
			p.write(" ")
			for i, v := range s.ReturnValues {
				if i > 0 {
					p.write(", ")
				}
				p.formatExpr(v, precLowest)
			}
		}
	case *ast.DestructureAssignStatement:
		p.write("(")
		for i, t := range s.Targets {
			if i > 0 {
				p.write(", ")
			}
			if t != nil {
				if t.Star {
					p.write("*")
				}
				if t.Name != nil {
					p.write(t.Name.Value)
				}
			}
		}
		p.write(") ")
		p.write(assignOpLiteral(s.OpToken, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.DeferStatement:
		p.write("defer ")
		p.formatExpr(s.Call, precLowest)
	case *ast.ThrowStatement:
		p.write("throw ")
		p.formatExpr(s.Value, precLowest)
	case *ast.BreakStatement:
		p.write("break")
	case *ast.ContinueStatement:
		p.write("continue")
	case *ast.PassStatement:
		p.write("pass")
	case *ast.ImportStatement:
		p.write("import ")
		p.write(stringLiteralText(s.Path))
		if s.Alias != nil {
			p.write(" as ")
			p.write(s.Alias.Value)
		}
	case *ast.FromImportStatement:
		p.write("from ")
		p.write(stringLiteralText(s.Path))
		p.write(" import ")
		for i, it := range s.Items {
			if i > 0 {
				p.write(", ")
			}
			p.write(it.Name.Value)
			if it.Alias != nil {
				p.write(" as ")
				p.write(it.Alias.Value)
			}
		}
	case *ast.ExportStatement:
		p.write("export ")
		p.printStatementInline(s.Stmt)
	case *ast.BlockStatement:
		p.printBlock(s)
	case *ast.IfStatement:
		var headerIf []Comment
		var headerElse []Comment
		var footer []Comment
		altLine := 0
		if s.Alternative != nil {
			altLine = startLineStatement(s.Alternative)
		}
		for _, c := range trailing {
			switch {
			case c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c):
				headerIf = append(headerIf, c)
			case altLine > 0 && c.StartLine == altLine && !p.inlineCommentAfterBrace(c):
				headerElse = append(headerElse, c)
			default:
				footer = append(footer, c)
			}
		}
		p.write("if (")
		p.formatExpr(s.Condition, precLowest)
		p.write(") ")
		if !p.printIfBranchWithHeaderComments(s.Consequence, headerIf) && len(headerIf) > 0 {
			footer = append(footer, headerIf...)
		}
		if s.Alternative != nil {
			p.write(" else ")
			if !p.printIfBranchWithHeaderComments(s.Alternative, headerElse) && len(headerElse) > 0 {
				footer = append(footer, headerElse...)
			}
		}
		trailingAfter = footer
	case *ast.WhileStatement:
		var header []Comment
		var footer []Comment
		for _, c := range trailing {
			if c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c) {
				header = append(header, c)
			} else {
				footer = append(footer, c)
			}
		}
		p.write("while (")
		p.formatExpr(s.Condition, precLowest)
		p.write(") ")
		if !p.printBlockWithHeaderComments(s.Body, header) && len(header) > 0 {
			footer = append(footer, header...)
		}
		trailingAfter = footer
	case *ast.ForStatement:
		var header []Comment
		var footer []Comment
		for _, c := range trailing {
			if c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c) {
				header = append(header, c)
			} else {
				footer = append(footer, c)
			}
		}
		p.write("for (")
		if s.Init != nil {
			p.printStatementInline(s.Init)
		}
		p.write("; ")
		if s.Cond != nil {
			p.formatExpr(s.Cond, precLowest)
		}
		p.write("; ")
		if s.Post != nil {
			p.printStatementInline(s.Post)
		}
		p.write(") ")
		if !p.printBlockWithHeaderComments(s.Body, header) && len(header) > 0 {
			footer = append(footer, header...)
		}
		trailingAfter = footer
	case *ast.ForInStatement:
		var header []Comment
		var footer []Comment
		for _, c := range trailing {
			if c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c) {
				header = append(header, c)
			} else {
				footer = append(footer, c)
			}
		}
		p.write("for (")
		if s.Destruct {
			if s.Key != nil {
				p.write(s.Key.Value)
			}
			p.write(", ")
			if s.Value != nil {
				p.write(s.Value.Value)
			}
		} else {
			p.write(s.Var.Value)
		}
		p.write(" in ")
		p.formatExpr(s.Iterable, precLowest)
		p.write(") ")
		if !p.printBlockWithHeaderComments(s.Body, header) && len(header) > 0 {
			footer = append(footer, header...)
		}
		trailingAfter = footer
	case *ast.SwitchStatement:
		var header []Comment
		var footer []Comment
		for _, c := range trailing {
			if c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c) {
				header = append(header, c)
			} else {
				footer = append(footer, c)
			}
		}
		p.printSwitch(s, header)
		trailingAfter = footer
	case *ast.TryStatement:
		var headerTry []Comment
		var headerCatch []Comment
		var headerFinally []Comment
		var footer []Comment
		catchLine := 0
		if s.CatchBlock != nil {
			if s.CatchToken.Line > 0 {
				catchLine = s.CatchToken.Line
			} else {
				catchLine = s.CatchBlock.Token.Line
			}
		}
		finallyLine := 0
		if s.FinallyBlock != nil {
			if s.FinallyToken.Line > 0 {
				finallyLine = s.FinallyToken.Line
			} else {
				finallyLine = s.FinallyBlock.Token.Line
			}
		}
		for _, c := range trailing {
			switch {
			case c.StartLine == s.Token.Line && !p.inlineCommentAfterBrace(c):
				headerTry = append(headerTry, c)
			case catchLine > 0 && c.StartLine == catchLine && !p.inlineCommentAfterBrace(c):
				headerCatch = append(headerCatch, c)
			case finallyLine > 0 && c.StartLine == finallyLine && !p.inlineCommentAfterBrace(c):
				headerFinally = append(headerFinally, c)
			default:
				footer = append(footer, c)
			}
		}
		p.write("try ")
		if !p.printBlockWithHeaderComments(s.TryBlock, headerTry) && len(headerTry) > 0 {
			footer = append(footer, headerTry...)
		}
		if s.CatchBlock != nil {
			p.write(" catch (")
			if s.CatchName != nil {
				p.write(s.CatchName.Value)
			}
			p.write(") ")
			if !p.printBlockWithHeaderComments(s.CatchBlock, headerCatch) && len(headerCatch) > 0 {
				footer = append(footer, headerCatch...)
			}
		}
		if s.FinallyBlock != nil {
			p.write(" finally ")
			if !p.printBlockWithHeaderComments(s.FinallyBlock, headerFinally) && len(headerFinally) > 0 {
				footer = append(footer, headerFinally...)
			}
		}
		trailingAfter = footer
	default:
		p.write("/* unsupported */")
	}

	if len(trailingAfter) > 0 {
		for _, c := range trailingAfter {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
}

func (p *Printer) printStatementInline(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		p.formatExpr(s.Expression, precLowest)
	case *ast.AssignStatement:
		p.formatAssign(s.Name, s.OpToken, s.Value)
	case *ast.IndexAssignStatement:
		p.formatExpr(s.Left, precLowest)
		p.write(" ")
		p.write(assignOpLiteral(s.Token, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.MemberAssignStatement:
		p.formatExpr(s.Object, precCall)
		p.write(".")
		p.write(s.Property.Value)
		p.write(" ")
		p.write(assignOpLiteral(s.Token, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.DestructureAssignStatement:
		p.write("(")
		for i, t := range s.Targets {
			if i > 0 {
				p.write(", ")
			}
			if t != nil {
				if t.Star {
					p.write("*")
				}
				if t.Name != nil {
					p.write(t.Name.Value)
				}
			}
		}
		p.write(") ")
		p.write(assignOpLiteral(s.OpToken, s.Op))
		p.write(" ")
		p.formatExpr(s.Value, precLowest)
	case *ast.ReturnStatement:
		p.write("return")
		if len(s.ReturnValues) > 0 {
			p.write(" ")
			for i, v := range s.ReturnValues {
				if i > 0 {
					p.write(", ")
				}
				p.formatExpr(v, precLowest)
			}
		}
	case *ast.DeferStatement:
		p.write("defer ")
		p.formatExpr(s.Call, precLowest)
	case *ast.ThrowStatement:
		p.write("throw ")
		p.formatExpr(s.Value, precLowest)
	case *ast.BreakStatement:
		p.write("break")
	case *ast.ContinueStatement:
		p.write("continue")
	case *ast.PassStatement:
		p.write("pass")
	case *ast.ImportStatement:
		p.write("import ")
		p.write(stringLiteralText(s.Path))
	case *ast.FromImportStatement:
		p.write("from ")
		p.write(stringLiteralText(s.Path))
		p.write(" import ...")
	case *ast.ExportStatement:
		p.write("export ")
		p.printStatementInline(s.Stmt)
	case *ast.BlockStatement:
		p.printBlock(s)
	case *ast.IfStatement:
		p.write("if (")
		p.formatExpr(s.Condition, precLowest)
		p.write(") ")
		p.printIfBranchInline(s.Consequence)
		if s.Alternative != nil {
			p.write(" else ")
			p.printIfBranchInline(s.Alternative)
		}
	default:
		p.write("/* unsupported */")
	}
}

func (p *Printer) printBlock(block *ast.BlockStatement) {
	if block == nil {
		p.write("{}")
		return
	}
	scope := p.index.byBlock[block]
	inline := p.canInlineBlock(block, scope)
	if inline {
		p.write("{")
		p.write(" ")
		if len(block.Statements) == 1 {
			p.printStatementInline(block.Statements[0])
		}
		p.write(" }")
		return
	}

	p.write("{")
	p.newline()
	p.level++
	p.printStatementList(block.Statements, scope)
	p.level--
	p.write("}")
}

func (p *Printer) printBlockWithHeaderComments(block *ast.BlockStatement, header []Comment) bool {
	if block == nil {
		p.write("{}")
		return false
	}
	scope := p.index.byBlock[block]
	inline := p.canInlineBlock(block, scope)
	if len(header) > 0 {
		inline = false
	}
	if inline {
		p.write("{")
		p.write(" ")
		if len(block.Statements) == 1 {
			p.printStatementInline(block.Statements[0])
		}
		p.write(" }")
		return false
	}

	p.write("{")
	if len(header) > 0 {
		for _, c := range header {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
	p.level++
	p.printStatementList(block.Statements, scope)
	p.level--
	p.write("}")
	return len(header) > 0
}

func (p *Printer) printIfBranchWithHeaderComments(stmt ast.Statement, header []Comment) bool {
	if stmt == nil {
		p.write("{}")
		return false
	}
	if block, ok := stmt.(*ast.BlockStatement); ok {
		return p.printBlockWithHeaderComments(block, header)
	}
	p.printStatementInline(stmt)
	return false
}

func (p *Printer) printIfBranchInline(stmt ast.Statement) {
	if stmt == nil {
		p.write("{}")
		return
	}
	if block, ok := stmt.(*ast.BlockStatement); ok {
		p.printBlock(block)
		return
	}
	p.printStatementInline(stmt)
}

func (p *Printer) canInlineBlock(block *ast.BlockStatement, scope *blockScope) bool {
	if block == nil {
		return true
	}
	if len(block.Statements) != 1 {
		return false
	}
	if scope != nil && len(scope.comments) > 0 {
		return false
	}
	return isSimpleStatement(block.Statements[0])
}

func isSimpleStatement(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement,
		*ast.AssignStatement,
		*ast.IndexAssignStatement,
		*ast.MemberAssignStatement,
		*ast.ReturnStatement,
		*ast.DestructureAssignStatement,
		*ast.DeferStatement,
		*ast.ThrowStatement,
		*ast.BreakStatement,
		*ast.ContinueStatement,
		*ast.PassStatement,
		*ast.ImportStatement,
		*ast.FromImportStatement:
		return true
	case *ast.ExportStatement:
		return isSimpleStatement(s.Stmt)
	default:
		return false
	}
}

func (p *Printer) formatAssign(name *ast.Identifier, opTok token.Token, value ast.Expression) {
	if name != nil {
		p.write(name.Value)
	}
	p.write(" ")
	p.write(assignOpLiteral(opTok, opTok.Type))
	p.write(" ")
	p.formatExpr(value, precLowest)
}

func assignOpLiteral(tok token.Token, op token.Type) string {
	if tok.Literal != "" {
		return tok.Literal
	}
	switch op {
	case token.PLUS_ASSIGN:
		return "+="
	case token.WALRUS:
		return ":="
	case token.MINUS_ASSIGN:
		return "-="
	case token.STAR_ASSIGN:
		return "*="
	case token.SLASH_ASSIGN:
		return "/="
	case token.PERCENT_ASSIGN:
		return "%="
	case token.BITOR_ASSIGN:
		return "|="
	default:
		return "="
	}
}

func stringLiteralText(lit *ast.StringLiteral) string {
	if lit == nil {
		return "\"\""
	}
	if lit.Token.Raw != "" {
		return lit.Token.Raw
	}
	return "\"" + lit.Value + "\""
}

func templateLiteralText(lit *ast.TemplateLiteral) string {
	if lit == nil {
		return `t""`
	}
	if lit.Token.Raw != "" {
		return lit.Token.Raw
	}
	return lit.String()
}

// Expressions and precedence
const (
	precLowest = iota
	precAssign
	precCoalesce
	precTernary
	precOr
	precAnd
	precBitOr
	precBitXor
	precBitAnd
	precEquals
	precLessGreater
	precShift
	precSum
	precProduct
	precPrefix
	precCall
)

func (p *Printer) formatExpr(expr ast.Expression, parentPrec int) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		p.write(e.Value)
	case *ast.IntegerLiteral:
		p.write(e.Token.Literal)
	case *ast.FloatLiteral:
		p.write(e.Token.Literal)
	case *ast.StringLiteral:
		p.write(stringLiteralText(e))
	case *ast.TemplateLiteral:
		if e.Tagged && e.Tag != nil {
			p.formatExpr(e.Tag, precCall)
			p.write(" ")
		}
		p.write(templateLiteralText(e))
	case *ast.BooleanLiteral:
		p.write(e.Token.Literal)
	case *ast.NilLiteral:
		p.write("nil")
	case *ast.TupleLiteral:
		p.write("(")
		for i, el := range e.Elements {
			if i > 0 {
				p.write(", ")
			}
			p.formatExpr(el, precLowest)
		}
		if len(e.Elements) == 1 {
			p.write(",")
		}
		p.write(")")
	case *ast.ListLiteral:
		p.write("[")
		for i, el := range e.Elements {
			if i > 0 {
				p.write(", ")
			}
			p.formatExpr(el, precLowest)
		}
		p.write("]")
	case *ast.ListComprehension:
		p.write("[")
		p.formatExpr(e.Elem, precLowest)
		p.write(" for ")
		if e.Var != nil {
			p.write(e.Var.Value)
		}
		p.write(" in ")
		p.formatExpr(e.Seq, precLowest)
		if e.Filter != nil {
			p.write(" if ")
			p.formatExpr(e.Filter, precLowest)
		}
		p.write("]")
	case *ast.DictLiteral:
		p.write("#{")
		for i, pair := range e.Pairs {
			if i > 0 {
				p.write(", ")
			}
			if pair.Shorthand != nil {
				p.write(pair.Shorthand.Value)
				continue
			}
			p.formatExpr(pair.Key, precLowest)
			p.write(": ")
			p.formatExpr(pair.Value, precLowest)
		}
		p.write("}")
	case *ast.PrefixExpression:
		prec := precPrefix
		if parentPrec > prec {
			p.write("(")
		}
		if e.Operator == "not" {
			p.write("not ")
		} else {
			p.write(e.Operator)
		}
		p.formatExpr(e.Right, prec)
		if parentPrec > prec {
			p.write(")")
		}
	case *ast.InfixExpression:
		prec := infixPrec(e.Operator)
		if parentPrec > prec {
			p.write("(")
		}
		p.formatExpr(e.Left, prec)
		p.write(" ")
		p.write(e.Operator)
		p.write(" ")
		p.formatExpr(e.Right, prec)
		if parentPrec > prec {
			p.write(")")
		}
	case *ast.ConditionalExpression:
		prec := precTernary
		if parentPrec > prec {
			p.write("(")
		}
		p.formatExpr(e.Cond, prec)
		p.write(" ? ")
		p.formatExpr(e.Then, prec)
		p.write(" : ")
		p.formatExpr(e.Else, prec)
		if parentPrec > prec {
			p.write(")")
		}
	case *ast.CondExpr:
		prec := precTernary
		if parentPrec > prec {
			p.write("(")
		}
		p.formatExpr(e.Then, prec)
		p.write(" if ")
		p.formatExpr(e.Cond, prec)
		p.write(" else ")
		p.formatExpr(e.Else, prec)
		if parentPrec > prec {
			p.write(")")
		}
	case *ast.AssignExpression:
		prec := precAssign
		if parentPrec > prec {
			p.write("(")
		}
		p.formatExpr(e.Left, prec)
		p.write(" ")
		p.write(assignOpLiteral(e.Token, e.Op))
		p.write(" ")
		p.formatExpr(e.Value, prec)
		if parentPrec > prec {
			p.write(")")
		}
	case *ast.MemberExpression:
		p.formatExpr(e.Object, precCall)
		p.write(".")
		p.write(e.Property.Value)
	case *ast.SpreadExpression:
		p.write("...")
		if e.Value != nil {
			p.formatExpr(e.Value, precLowest)
		}
	case *ast.CallExpression:
		p.formatExpr(e.Function, precCall)
		p.write("(")
		for i, a := range e.Arguments {
			if i > 0 {
				p.write(", ")
			}
			p.formatExpr(a, precLowest)
		}
		p.write(")")
	case *ast.IndexExpression:
		p.formatExpr(e.Left, precCall)
		p.write("[")
		p.formatExpr(e.Index, precLowest)
		p.write("]")
	case *ast.SliceExpression:
		p.formatExpr(e.Left, precCall)
		p.write("[")
		if e.Low != nil {
			p.formatExpr(e.Low, precLowest)
		}
		p.write(":")
		if e.High != nil {
			p.formatExpr(e.High, precLowest)
		}
		if e.Step != nil {
			p.write(":")
			p.formatExpr(e.Step, precLowest)
		}
		p.write("]")
	case *ast.FunctionLiteral:
		p.write("func(")
		for i, pident := range e.Parameters {
			if i > 0 {
				p.write(", ")
			}
			p.write(pident.Value)
		}
		p.write(") ")
		p.printBlock(e.Body)
	case *ast.MatchExpression:
		p.printMatchExpression(e)
	default:
		p.write("/* unsupported */")
	}
}

func infixPrec(op string) int {
	switch op {
	case "??":
		return precCoalesce
	case "or":
		return precOr
	case "and":
		return precAnd
	case "|":
		return precBitOr
	case "^":
		return precBitXor
	case "&":
		return precBitAnd
	case "==", "!=", "is":
		return precEquals
	case "<", "<=", ">", ">=":
		return precLessGreater
	case "in":
		return precLessGreater
	case "<<", ">>":
		return precShift
	case "+", "-":
		return precSum
	case "*", "/", "%":
		return precProduct
	default:
		return precLowest
	}
}

func (p *Printer) printSwitch(stmt *ast.SwitchStatement, header []Comment) {
	scope := p.index.bySwitch[stmt]
	p.write("switch (")
	p.formatExpr(stmt.Value, precLowest)
	p.write(") {")
	if len(header) > 0 {
		for _, c := range header {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
	p.level++
	p.printCaseList(stmt, scope)
	p.level--
	p.write("}")
}

func (p *Printer) printCaseList(stmt *ast.SwitchStatement, scope *blockScope) {
	cases := buildSwitchCases(stmt)
	if scope == nil {
		for _, c := range cases {
			p.printSwitchCase(c, nil)
		}
		return
	}

	trailing := map[int][]Comment{}
	comments := make([]Comment, 0, len(scope.comments))
	for _, c := range scope.comments {
		if isInlineComment(c, p.lines) {
			trailing[c.StartLine] = append(trailing[c.StartLine], c)
			continue
		}
		comments = append(comments, c)
	}

	commentIdx := 0
	lastOrigLine := scope.startLine
	for _, c := range cases {
		for commentIdx < len(comments) && comments[commentIdx].StartLine < c.startLine {
			cm := comments[commentIdx]
			p.prepareItem(lastOrigLine, cm.StartLine)
			p.printComment(cm)
			lastOrigLine = cm.EndLine
			commentIdx++
		}

		p.prepareItem(lastOrigLine, c.startLine)
		p.printSwitchCase(c, trailing[c.startLine])
		lastOrigLine = c.endLine
	}
	for commentIdx < len(comments) {
		cm := comments[commentIdx]
		p.prepareItem(lastOrigLine, cm.StartLine)
		p.printComment(cm)
		lastOrigLine = cm.EndLine
		commentIdx++
	}
}

type switchCaseItem struct {
	kind      string
	clause    *ast.CaseClause
	defBlock  *ast.BlockStatement
	startLine int
	endLine   int
}

func buildSwitchCases(stmt *ast.SwitchStatement) []switchCaseItem {
	items := make([]switchCaseItem, 0, len(stmt.Cases)+1)
	for _, c := range stmt.Cases {
		items = append(items, switchCaseItem{
			kind:      "case",
			clause:    c,
			startLine: c.Token.Line,
			endLine:   endLineStatement(c.Body),
		})
	}
	if stmt.Default != nil {
		items = append(items, switchCaseItem{
			kind:      "default",
			defBlock:  stmt.Default,
			startLine: stmt.Default.Token.Line,
			endLine:   endLineStatement(stmt.Default),
		})
	}
	return items
}

func (p *Printer) printSwitchCase(item switchCaseItem, trailing []Comment) {
	var header []Comment
	var footer []Comment
	for _, c := range trailing {
		if p.inlineCommentAfterBrace(c) {
			footer = append(footer, c)
		} else {
			header = append(header, c)
		}
	}
	switch item.kind {
	case "case":
		p.write("case ")
		for i, v := range item.clause.Values {
			if i > 0 {
				p.write(", ")
			}
			p.formatExpr(v, precLowest)
		}
		p.write(" ")
		if !p.printBlockWithHeaderComments(item.clause.Body, header) && len(header) > 0 {
			footer = append(footer, header...)
		}
	case "default":
		p.write("default ")
		if !p.printBlockWithHeaderComments(item.defBlock, header) && len(header) > 0 {
			footer = append(footer, header...)
		}
	}
	if len(footer) > 0 {
		for _, c := range footer {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
}

func (p *Printer) printMatchExpression(expr *ast.MatchExpression) {
	p.printMatchExpressionWithHeader(expr, nil)
}

func (p *Printer) printMatchExpressionWithHeader(expr *ast.MatchExpression, header []Comment) {
	scope := p.index.byMatch[expr]
	p.write("match (")
	p.formatExpr(expr.Value, precLowest)
	p.write(") {")
	if len(header) > 0 {
		for _, c := range header {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
	p.level++
	p.printMatchCases(expr, scope)
	p.level--
	p.write("}")
}

func (p *Printer) printMatchCases(expr *ast.MatchExpression, scope *blockScope) {
	cases := buildMatchCases(expr)
	if scope == nil {
		for _, c := range cases {
			p.printMatchCase(c, nil)
		}
		return
	}

	trailing := map[int][]Comment{}
	comments := make([]Comment, 0, len(scope.comments))
	for _, c := range scope.comments {
		if isInlineComment(c, p.lines) {
			trailing[c.StartLine] = append(trailing[c.StartLine], c)
			continue
		}
		comments = append(comments, c)
	}

	commentIdx := 0
	lastOrigLine := scope.startLine
	for _, c := range cases {
		for commentIdx < len(comments) && comments[commentIdx].StartLine < c.startLine {
			cm := comments[commentIdx]
			p.prepareItem(lastOrigLine, cm.StartLine)
			p.printComment(cm)
			lastOrigLine = cm.EndLine
			commentIdx++
		}

		p.prepareItem(lastOrigLine, c.startLine)
		p.printMatchCase(c, trailing[c.startLine])
		lastOrigLine = c.endLine
	}
	for commentIdx < len(comments) {
		cm := comments[commentIdx]
		p.prepareItem(lastOrigLine, cm.StartLine)
		p.printComment(cm)
		lastOrigLine = cm.EndLine
		commentIdx++
	}
}

type matchCaseItem struct {
	kind      string
	clause    *ast.MatchCase
	defExpr   ast.Expression
	startLine int
	endLine   int
}

func buildMatchCases(expr *ast.MatchExpression) []matchCaseItem {
	items := make([]matchCaseItem, 0, len(expr.Cases)+1)
	for _, c := range expr.Cases {
		items = append(items, matchCaseItem{
			kind:      "case",
			clause:    c,
			startLine: c.Token.Line,
			endLine:   endLineExpr(c.Result),
		})
	}
	if expr.Default != nil {
		items = append(items, matchCaseItem{
			kind:      "default",
			defExpr:   expr.Default,
			startLine: startLineExpr(expr.Default),
			endLine:   endLineExpr(expr.Default),
		})
	}
	return items
}

func (p *Printer) printMatchCase(item matchCaseItem, trailing []Comment) {
	switch item.kind {
	case "case":
		p.write("case ")
		for i, v := range item.clause.Values {
			if i > 0 {
				p.write(", ")
			}
			p.formatExpr(v, precLowest)
		}
		p.write(" { ")
		p.formatExpr(item.clause.Result, precLowest)
		p.write(" }")
	case "default":
		p.write("default { ")
		p.formatExpr(item.defExpr, precLowest)
		p.write(" }")
	}
	if len(trailing) > 0 {
		for _, c := range trailing {
			p.write(" ")
			p.write(c.Raw)
		}
	}
	p.newline()
}

// Source positions
func startLineStatement(stmt ast.Statement) int {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		return startLineExpr(s.Expression)
	case *ast.AssignStatement:
		return s.Token.Line
	case *ast.IndexAssignStatement:
		return s.Token.Line
	case *ast.MemberAssignStatement:
		return s.Token.Line
	case *ast.ReturnStatement:
		return s.Token.Line
	case *ast.DestructureAssignStatement:
		return s.Token.Line
	case *ast.DeferStatement:
		return s.Token.Line
	case *ast.ThrowStatement:
		return s.Token.Line
	case *ast.BreakStatement:
		return s.Token.Line
	case *ast.ContinueStatement:
		return s.Token.Line
	case *ast.PassStatement:
		return s.Token.Line
	case *ast.ImportStatement:
		return s.Token.Line
	case *ast.FromImportStatement:
		return s.Token.Line
	case *ast.ExportStatement:
		return s.Token.Line
	case *ast.BlockStatement:
		return s.Token.Line
	case *ast.IfStatement:
		return s.Token.Line
	case *ast.WhileStatement:
		return s.Token.Line
	case *ast.ForStatement:
		return s.Token.Line
	case *ast.ForInStatement:
		return s.Token.Line
	case *ast.SwitchStatement:
		return s.Token.Line
	case *ast.TryStatement:
		return s.Token.Line
	case *ast.FuncStatement:
		return s.Token.Line
	default:
		return 1
	}
}

func headerLinesForStatement(stmt ast.Statement) []int {
	switch s := stmt.(type) {
	case *ast.IfStatement:
		lines := []int{s.Token.Line}
		if s.Alternative != nil {
			lines = append(lines, startLineStatement(s.Alternative))
		}
		return lines
	case *ast.WhileStatement:
		return []int{s.Token.Line}
	case *ast.ForStatement:
		return []int{s.Token.Line}
	case *ast.ForInStatement:
		return []int{s.Token.Line}
	case *ast.SwitchStatement:
		return []int{s.Token.Line}
	case *ast.TryStatement:
		lines := []int{s.Token.Line}
		if s.CatchBlock != nil {
			if s.CatchToken.Line > 0 {
				lines = append(lines, s.CatchToken.Line)
			} else {
				lines = append(lines, s.CatchBlock.Token.Line)
			}
		}
		if s.FinallyBlock != nil {
			if s.FinallyToken.Line > 0 {
				lines = append(lines, s.FinallyToken.Line)
			} else {
				lines = append(lines, s.FinallyBlock.Token.Line)
			}
		}
		return lines
	default:
		return nil
	}
}

func endLineStatement(stmt ast.Statement) int {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		return endLineExpr(s.Expression)
	case *ast.AssignStatement:
		return endLineExpr(s.Value)
	case *ast.IndexAssignStatement:
		return endLineExpr(s.Value)
	case *ast.MemberAssignStatement:
		return endLineExpr(s.Value)
	case *ast.ReturnStatement:
		if len(s.ReturnValues) > 0 {
			return endLineExpr(s.ReturnValues[len(s.ReturnValues)-1])
		}
		return s.Token.Line
	case *ast.DestructureAssignStatement:
		return endLineExpr(s.Value)
	case *ast.DeferStatement:
		return endLineExpr(s.Call)
	case *ast.ThrowStatement:
		return endLineExpr(s.Value)
	case *ast.BreakStatement:
		return s.Token.Line
	case *ast.ContinueStatement:
		return s.Token.Line
	case *ast.PassStatement:
		return s.Token.Line
	case *ast.ImportStatement:
		return s.Token.Line
	case *ast.FromImportStatement:
		return s.Token.Line
	case *ast.ExportStatement:
		return endLineStatement(s.Stmt)
	case *ast.BlockStatement:
		if len(s.Statements) == 0 {
			return s.Token.Line
		}
		return endLineStatement(s.Statements[len(s.Statements)-1])
	case *ast.IfStatement:
		if s.Alternative != nil {
			return endLineStatement(s.Alternative)
		}
		return endLineStatement(s.Consequence)
	case *ast.WhileStatement:
		return endLineStatement(s.Body)
	case *ast.ForStatement:
		return endLineStatement(s.Body)
	case *ast.ForInStatement:
		return endLineStatement(s.Body)
	case *ast.SwitchStatement:
		endLine := s.Token.Line
		if len(s.Cases) > 0 {
			endLine = endLineStatement(s.Cases[len(s.Cases)-1].Body)
		}
		if s.Default != nil {
			defaultEnd := endLineStatement(s.Default)
			if defaultEnd > endLine {
				endLine = defaultEnd
			}
		}
		return endLine
	case *ast.TryStatement:
		if s.FinallyBlock != nil {
			return endLineStatement(s.FinallyBlock)
		}
		if s.CatchBlock != nil {
			return endLineStatement(s.CatchBlock)
		}
		return endLineStatement(s.TryBlock)
	case *ast.FuncStatement:
		return endLineStatement(s.Body)
	default:
		return 1
	}
}

func startLineExpr(expr ast.Expression) int {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Token.Line
	case *ast.IntegerLiteral:
		return e.Token.Line
	case *ast.FloatLiteral:
		return e.Token.Line
	case *ast.StringLiteral:
		return e.Token.Line
	case *ast.BooleanLiteral:
		return e.Token.Line
	case *ast.NilLiteral:
		return e.Token.Line
	case *ast.PrefixExpression:
		return e.Token.Line
	case *ast.InfixExpression:
		return startLineExpr(e.Left)
	case *ast.ConditionalExpression:
		return startLineExpr(e.Cond)
	case *ast.CondExpr:
		return startLineExpr(e.Then)
	case *ast.AssignExpression:
		return startLineExpr(e.Left)
	case *ast.MemberExpression:
		return startLineExpr(e.Object)
	case *ast.CallExpression:
		return startLineExpr(e.Function)
	case *ast.IndexExpression:
		return startLineExpr(e.Left)
	case *ast.SliceExpression:
		return startLineExpr(e.Left)
	case *ast.ListLiteral:
		return e.Token.Line
	case *ast.ListComprehension:
		return e.Token.Line
	case *ast.TupleLiteral:
		return e.Token.Line
	case *ast.DictLiteral:
		return e.Token.Line
	case *ast.FunctionLiteral:
		return e.Token.Line
	case *ast.MatchExpression:
		return e.Token.Line
	default:
		return 1
	}
}

func endLineExpr(expr ast.Expression) int {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Token.Line
	case *ast.IntegerLiteral:
		return e.Token.Line
	case *ast.FloatLiteral:
		return e.Token.Line
	case *ast.StringLiteral:
		return e.Token.Line + literalNewlineCount(e.Token.Raw, e.Token.Literal)
	case *ast.BooleanLiteral:
		return e.Token.Line
	case *ast.NilLiteral:
		return e.Token.Line
	case *ast.PrefixExpression:
		return endLineExpr(e.Right)
	case *ast.InfixExpression:
		return endLineExpr(e.Right)
	case *ast.ConditionalExpression:
		return endLineExpr(e.Else)
	case *ast.CondExpr:
		return endLineExpr(e.Else)
	case *ast.AssignExpression:
		return endLineExpr(e.Value)
	case *ast.MemberExpression:
		return e.Property.Token.Line
	case *ast.SpreadExpression:
		if e.Value != nil {
			return endLineExpr(e.Value)
		}
		return e.Token.Line
	case *ast.CallExpression:
		if len(e.Arguments) > 0 {
			return endLineExpr(e.Arguments[len(e.Arguments)-1])
		}
		return endLineExpr(e.Function)
	case *ast.IndexExpression:
		return endLineExpr(e.Index)
	case *ast.SliceExpression:
		if e.Step != nil {
			return endLineExpr(e.Step)
		}
		if e.High != nil {
			return endLineExpr(e.High)
		}
		if e.Low != nil {
			return endLineExpr(e.Low)
		}
		return endLineExpr(e.Left)
	case *ast.ListLiteral:
		if len(e.Elements) > 0 {
			return endLineExpr(e.Elements[len(e.Elements)-1])
		}
		return e.Token.Line
	case *ast.ListComprehension:
		if e.Filter != nil {
			return endLineExpr(e.Filter)
		}
		if e.Elem != nil {
			return endLineExpr(e.Elem)
		}
		return e.Token.Line
	case *ast.TupleLiteral:
		if len(e.Elements) > 0 {
			return endLineExpr(e.Elements[len(e.Elements)-1])
		}
		return e.Token.Line
	case *ast.DictLiteral:
		if len(e.Pairs) > 0 {
			last := e.Pairs[len(e.Pairs)-1]
			if last.Shorthand != nil {
				return endLineExpr(last.Shorthand)
			}
			return endLineExpr(last.Value)
		}
		return e.Token.Line
	case *ast.FunctionLiteral:
		return endLineStatement(e.Body)
	case *ast.MatchExpression:
		endLine := e.Token.Line
		if len(e.Cases) > 0 {
			endLine = endLineExpr(e.Cases[len(e.Cases)-1].Result)
		}
		if e.Default != nil {
			defaultEnd := endLineExpr(e.Default)
			if defaultEnd > endLine {
				endLine = defaultEnd
			}
		}
		return endLine
	default:
		return 1
	}
}

func literalNewlineCount(raw, literal string) int {
	if raw == "" {
		raw = literal
	}
	if raw == "" {
		return 0
	}
	return strings.Count(raw, "\n")
}
