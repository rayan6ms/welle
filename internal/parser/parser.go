package parser

import (
	"fmt"
	"strconv"

	"welle/internal/ast"
	"welle/internal/diag"
	"welle/internal/lexer"
	"welle/internal/token"
)

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string
	diags  []diag.Diagnostic

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.Type]prefixParseFn
	infixParseFns  map[token.Type]infixParseFn
}

/* -------------------- precedence -------------------- */

const (
	_ int = iota
	LOWEST
	ORPREC      // or
	ANDPREC     // and
	EQUALS      // == !=
	LESSGREATER // < <= > >=
	SUM         // + -
	PRODUCT     // * / %
	PREFIX      // -X, not X
	INDEX       // array[index]
	CALL        // fn(X)
)

var precedences = map[token.Type]int{
	token.OR:       ORPREC,
	token.AND:      ANDPREC,
	token.EQ:       EQUALS,
	token.NE:       EQUALS,
	token.LT:       LESSGREATER,
	token.LE:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.GE:       LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.STAR:     PRODUCT,
	token.SLASH:    PRODUCT,
	token.PERCENT:  PRODUCT,
	token.LBRACKET: INDEX,
	token.LPAREN:   CALL,
	token.DOT:      CALL,
}

/* -------------------- constructor -------------------- */

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:              l,
		errors:         []string{},
		diags:          []diag.Diagnostic{},
		prefixParseFns: map[token.Type]prefixParseFn{},
		infixParseFns:  map[token.Type]infixParseFn{},
	}

	// read two tokens, so cur and peek are set
	p.nextToken()
	p.nextToken()

	// Prefix parsers
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(token.FALSE, p.parseBooleanLiteral)
	p.registerPrefix(token.NIL, p.parseNilLiteral)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.HASH, p.parseDictLiteral)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.MATCH, p.parseMatchExpression)

	// Infix parsers
	for _, tt := range []token.Type{
		token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT,
		token.EQ, token.NE, token.LT, token.LE, token.GT, token.GE,
		token.AND, token.OR,
	} {
		p.registerInfix(tt, p.parseInfixExpression)
	}
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.DOT, p.parseMemberExpression)

	return p
}

func (p *Parser) Diagnostics() []diag.Diagnostic { return p.diags }
func (p *Parser) Errors() []string               { return p.errors }

/* -------------------- program -------------------- */

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{Statements: []ast.Statement{}}

	for p.curToken.Type != token.EOF {
		if p.isSeparator(p.curToken.Type) {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}

		p.nextToken()
	}

	return program
}

/* -------------------- statements -------------------- */

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.FUNC:
		return p.parseFuncStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.DEFER:
		return p.parseDeferStatement()
	case token.THROW:
		return p.parseThrowStatement()
	case token.BREAK:
		return &ast.BreakStatement{Token: p.curToken}
	case token.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}
	case token.IF:
		return p.parseIfStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.FOR:
		return p.parseForStatement()
	case token.SWITCH:
		return p.parseSwitchStatement()
	case token.TRY:
		return p.parseTryStatement()
	case token.IMPORT:
		return p.parseImportStatement()
	case token.FROM:
		return p.parseFromImportStatement()
	case token.EXPORT:
		return p.parseExportStatement()
	default:
		// assignment lookahead: IDENT '=' ...
		if p.curToken.Type == token.IDENT && p.peekToken.Type == token.ASSIGN {
			return p.parseAssignStatement()
		}
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseFuncStatement() ast.Statement {
	stmt := &ast.FuncStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	stmt.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseExportStatement() ast.Statement {
	stmt := &ast.ExportStatement{Token: p.curToken}

	// Move to the statement after 'export'
	p.nextToken()

	inner := p.parseStatement()
	if inner == nil {
		p.errorAt(p.curToken, "expected statement after export")
		return nil
	}

	stmt.Stmt = inner
	return stmt
}

func (p *Parser) parseTryStatement() ast.Statement {
	stmt := &ast.TryStatement{Token: p.curToken}

	// try { ... }
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.TryBlock = p.parseBlockStatement()

	hasCatch := false
	hasFinally := false

	// catch (optional)
	if p.peekToken.Type == token.CATCH {
		hasCatch = true
		if !p.expectPeek(token.CATCH) {
			return nil
		}
		stmt.CatchToken = p.curToken

		// (e)
		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.CatchName = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		if !p.expectPeek(token.RPAREN) {
			return nil
		}

		// { ... }
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		stmt.CatchBlock = p.parseBlockStatement()
	}

	// finally (optional)
	if p.peekToken.Type == token.FINALLY {
		hasFinally = true
		if !p.expectPeek(token.FINALLY) {
			return nil
		}
		stmt.FinallyToken = p.curToken

		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		stmt.FinallyBlock = p.parseBlockStatement()
	}

	if !hasCatch && !hasFinally {
		p.errorAt(p.peekToken, "expected catch or finally after try block")
		return nil
	}

	return stmt
}

func (p *Parser) parseThrowStatement() ast.Statement {
	stmt := &ast.ThrowStatement{Token: p.curToken}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseDeferStatement() ast.Statement {
	stmt := &ast.DeferStatement{Token: p.curToken}

	p.nextToken()
	stmt.Call = p.parseExpression(LOWEST)

	if _, ok := stmt.Call.(*ast.CallExpression); !ok {
		p.errorAt(stmt.Token, "defer expects a call expression")
		return nil
	}
	return stmt
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	params := []*ast.Identifier{}

	// curToken is '('
	if p.peekToken.Type == token.RPAREN {
		p.nextToken() // consume ')'
		return params
	}

	p.nextToken() // first param
	params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

	for p.peekToken.Type == token.COMMA {
		p.nextToken() // consume ','
		p.nextToken() // next ident
		params = append(params, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseReturnStatement() ast.Statement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseImportStatement() ast.Statement {
	stmt := &ast.ImportStatement{Token: p.curToken}

	if !p.expectPeek(token.STRING) {
		return nil
	}
	stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekToken.Type == token.AS {
		p.nextToken() // consume 'as'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	return stmt
}

func (p *Parser) parseFromImportStatement() ast.Statement {
	stmt := &ast.FromImportStatement{Token: p.curToken}

	// from "path"
	if !p.expectPeek(token.STRING) {
		return nil
	}
	stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}

	// import
	if !p.expectPeek(token.IMPORT) {
		return nil
	}

	items := []ast.ImportItem{}

	for {
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		name := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

		var alias *ast.Identifier
		if p.peekToken.Type == token.AS {
			p.nextToken() // consume 'as'
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}

		items = append(items, ast.ImportItem{Name: name, Alias: alias})

		if p.peekToken.Type != token.COMMA {
			break
		}
		p.nextToken() // consume comma
	}

	stmt.Items = items
	return stmt
}

func (p *Parser) parseAssignStatement() ast.Statement {
	// curToken is IDENT, peek is '='
	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	p.nextToken() // now '='
	if p.curToken.Type != token.ASSIGN {
		p.errorAt(p.curToken, "expected '=' in assignment")
		return nil
	}

	p.nextToken() // start of value expression
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseSimpleAssignStatement() ast.Statement {
	if p.curToken.Type != token.IDENT {
		p.errorAt(p.curToken, "expected identifier in assignment")
		return nil
	}
	if p.peekToken.Type != token.ASSIGN {
		p.errorAt(p.peekToken, "expected '=' in assignment")
		return nil
	}

	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	p.nextToken() // now '='
	p.nextToken() // start of value expression
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if idx, ok := stmt.Expression.(*ast.IndexExpression); ok && p.peekToken.Type == token.ASSIGN {
		p.nextToken() // now '='
		assign := &ast.IndexAssignStatement{Token: p.curToken, Left: idx}

		p.nextToken() // start of value expression
		assign.Value = p.parseExpression(LOWEST)
		return assign
	}
	if me, ok := stmt.Expression.(*ast.MemberExpression); ok && p.peekToken.Type == token.ASSIGN {
		p.nextToken() // now '='
		assign := &ast.MemberAssignStatement{
			Token:    p.curToken,
			Object:   me.Object,
			Property: me.Property,
		}

		p.nextToken() // start of value expression
		assign.Value = p.parseExpression(LOWEST)
		return assign
	}
	return stmt
}

func (p *Parser) parseIfStatement() ast.Statement {
	stmt := &ast.IfStatement{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Consequence = p.parseBlockStatement()

	// Optional else
	p.skipSeparatorsPeek()
	if p.peekToken.Type == token.ELSE {
		p.nextToken() // move to ELSE
		p.skipSeparatorsPeek()
		if p.peekToken.Type == token.IF {
			p.nextToken() // move to IF
			elseIfStmt := p.parseIfStatement()
			if elseIfStmt == nil {
				return nil
			}
			stmt.Alternative = &ast.BlockStatement{
				Token: token.Token{
					Type:    token.LBRACE,
					Literal: "{",
					Line:    p.curToken.Line,
					Col:     p.curToken.Col,
				},
				Statements: []ast.Statement{elseIfStmt},
			}
			return stmt
		}
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		stmt.Alternative = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseForStatement() ast.Statement {
	forTok := p.curToken
	if p.peekToken.Type == token.LPAREN {
		p.nextToken() // consume '('
		p.nextToken() // move to first token inside '('

		if p.curToken.Type == token.IDENT && p.peekToken.Type == token.IN {
			return p.parseForInStatementFromCur(forTok, true)
		}
		return p.parseCForStatementFromCur(forTok)
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	return p.parseForInStatementFromCur(forTok, false)
}

func (p *Parser) parseForInStatementFromCur(forTok token.Token, hasParens bool) ast.Statement {
	stmt := &ast.ForInStatement{Token: forTok}

	if p.curToken.Type != token.IDENT {
		p.errorAt(p.curToken, "expected identifier after 'for'")
		return nil
	}
	stmt.Var = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)

	if hasParens {
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseCForStatementFromCur(forTok token.Token) ast.Statement {
	stmt := &ast.ForStatement{Token: forTok}

	// init
	if p.curToken.Type != token.SEMICOLON {
		stmt.Init = p.parseSimpleAssignStatement()
		if stmt.Init == nil {
			return nil
		}
		if !p.expectPeekNoSkip(token.SEMICOLON) {
			p.errorAt(p.peekToken, "expected ';' after for init")
			return nil
		}
	}
	if p.curToken.Type != token.SEMICOLON {
		p.errorAt(p.curToken, "expected ';' after for init")
		return nil
	}
	p.nextToken() // move to condition or ';'

	// cond
	if p.curToken.Type != token.SEMICOLON {
		stmt.Cond = p.parseExpression(LOWEST)
		if !p.expectPeekNoSkip(token.SEMICOLON) {
			p.errorAt(p.peekToken, "expected ';' after for condition")
			return nil
		}
	}
	if p.curToken.Type != token.SEMICOLON {
		p.errorAt(p.curToken, "expected ';' after for condition")
		return nil
	}
	p.nextToken() // move to post or ')'

	// post
	if p.curToken.Type != token.RPAREN {
		stmt.Post = p.parseSimpleAssignStatement()
		if stmt.Post == nil {
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			p.errorAt(p.peekToken, "expected ')' after for post")
			return nil
		}
	}
	if p.curToken.Type != token.RPAREN {
		p.errorAt(p.curToken, "expected ')' after for post")
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseSwitchStatement() ast.Statement {
	stmt := &ast.SwitchStatement{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Cases = []*ast.CaseClause{}

	p.nextToken()
	for p.curToken.Type != token.RBRACE && p.curToken.Type != token.EOF {
		if p.isSeparator(p.curToken.Type) {
			p.nextToken()
			continue
		}

		if p.curToken.Type == token.CASE {
			cc := &ast.CaseClause{Token: p.curToken}

			p.nextToken()
			values := []ast.Expression{}
			values = append(values, p.parseExpression(LOWEST))
			for p.peekToken.Type == token.COMMA {
				p.nextToken()
				p.nextToken()
				values = append(values, p.parseExpression(LOWEST))
			}
			cc.Values = values

			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			cc.Body = p.parseBlockStatement()
			stmt.Cases = append(stmt.Cases, cc)

			p.nextToken()
			continue
		}

		if p.curToken.Type == token.DEFAULT {
			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			stmt.Default = p.parseBlockStatement()
			p.nextToken()
			continue
		}

		p.errorAt(p.curToken, "unexpected token in switch: "+string(p.curToken.Type))
		return nil
	}

	return stmt
}

func (p *Parser) parseMatchExpression() ast.Expression {
	exp := &ast.MatchExpression{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	p.nextToken()
	exp.Value = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	exp.Cases = []*ast.MatchCase{}

	p.nextToken()
	for p.curToken.Type != token.RBRACE && p.curToken.Type != token.EOF {
		if p.isSeparator(p.curToken.Type) {
			p.nextToken()
			continue
		}

		if p.curToken.Type == token.CASE {
			cc := &ast.MatchCase{Token: p.curToken}

			p.nextToken()
			values := []ast.Expression{}
			values = append(values, p.parseExpression(LOWEST))
			for p.peekToken.Type == token.COMMA {
				p.nextToken()
				p.nextToken()
				values = append(values, p.parseExpression(LOWEST))
			}
			cc.Values = values

			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			p.nextToken()
			for p.isSeparator(p.curToken.Type) {
				p.nextToken()
			}
			cc.Result = p.parseExpression(LOWEST)
			if cc.Result == nil {
				return nil
			}
			p.skipSeparatorsPeek()
			if !p.expectPeek(token.RBRACE) {
				return nil
			}
			exp.Cases = append(exp.Cases, cc)

			p.nextToken()
			continue
		}

		if p.curToken.Type == token.DEFAULT {
			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			p.nextToken()
			for p.isSeparator(p.curToken.Type) {
				p.nextToken()
			}
			exp.Default = p.parseExpression(LOWEST)
			if exp.Default == nil {
				return nil
			}
			p.skipSeparatorsPeek()
			if !p.expectPeek(token.RBRACE) {
				return nil
			}
			p.nextToken()
			continue
		}

		p.errorAt(p.curToken, "unexpected token in match: "+string(p.curToken.Type))
		return nil
	}

	return exp
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	// curToken is '{'
	block := &ast.BlockStatement{Token: p.curToken, Statements: []ast.Statement{}}

	p.nextToken()

	for p.curToken.Type != token.RBRACE && p.curToken.Type != token.EOF {
		if p.isSeparator(p.curToken.Type) {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}

		p.nextToken()
	}

	return block
}

/* -------------------- expressions (Pratt) -------------------- */

func (p *Parser) parseExpression(precedence int) ast.Expression {
	// stop on statement terminators / block end
	if p.isTerminator(p.curToken.Type) {
		return nil
	}

	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}

	leftExp := prefix()

	for !p.peekIsTerminator() && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken() // advance to infix operator (or '(' for call)
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	v, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.errorAt(p.curToken, fmt.Sprintf("could not parse int %q", p.curToken.Literal))
		return nil
	}
	lit.Value = v
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	v, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.errorAt(p.curToken, fmt.Sprintf("could not parse float %q", p.curToken.Literal))
		return nil
	}
	lit.Value = v
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curToken.Type == token.TRUE}
}

func (p *Parser) parseNilLiteral() ast.Expression {
	return &ast.NilLiteral{Token: p.curToken}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	// curToken is '('
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	exp := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	exp.Right = p.parseExpression(PREFIX)
	return exp
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	exp := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	prec := p.curPrecedence()
	p.nextToken()
	exp.Right = p.parseExpression(prec)
	return exp
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	// curToken is '('
	exp := &ast.CallExpression{
		Token:    p.curToken,
		Function: function,
	}
	exp.Arguments = p.parseCallArguments()
	return exp
}

func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	exp := &ast.MemberExpression{Token: p.curToken, Object: left}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	exp.Property = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	return exp
}

func (p *Parser) parseDictLiteral() ast.Expression {
	lit := &ast.DictLiteral{Token: p.curToken, Pairs: []ast.DictPair{}}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	if p.peekToken.Type == token.RBRACE {
		p.nextToken()
		return lit
	}

	p.nextToken()
	key := p.parseExpression(LOWEST)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	val := p.parseExpression(LOWEST)
	lit.Pairs = append(lit.Pairs, ast.DictPair{Key: key, Value: val})

	for p.peekToken.Type == token.COMMA {
		p.nextToken()
		p.nextToken()

		key := p.parseExpression(LOWEST)
		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		val := p.parseExpression(LOWEST)
		lit.Pairs = append(lit.Pairs, ast.DictPair{Key: key, Value: val})
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return lit
}

func (p *Parser) parseListLiteral() ast.Expression {
	lit := &ast.ListLiteral{Token: p.curToken}
	lit.Elements = p.parseExpressionList(token.RBRACKET)
	return lit
}

func (p *Parser) parseExpressionList(end token.Type) []ast.Expression {
	list := []ast.Expression{}

	if p.peekToken.Type == end {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekToken.Type == token.COMMA {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	tok := p.curToken

	if p.peekToken.Type == token.COLON {
		p.nextToken()
		var high ast.Expression
		if p.peekToken.Type != token.RBRACKET {
			p.nextToken()
			high = p.parseExpression(LOWEST)
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return &ast.SliceExpression{Token: tok, Left: left, Low: nil, High: high}
	}

	p.nextToken()
	first := p.parseExpression(LOWEST)

	if p.peekToken.Type == token.COLON {
		p.nextToken()

		var high ast.Expression
		if p.peekToken.Type != token.RBRACKET {
			p.nextToken()
			high = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(token.RBRACKET) {
			return nil
		}

		return &ast.SliceExpression{Token: tok, Left: left, Low: first, High: high}
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return &ast.IndexExpression{Token: tok, Left: left, Index: first}
}

func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}

	// If next token is ')', no args
	if p.peekToken.Type == token.RPAREN {
		p.nextToken() // consume ')'
		return args
	}

	p.nextToken() // first arg
	args = append(args, p.parseExpression(LOWEST))

	for p.peekToken.Type == token.COMMA {
		p.nextToken() // consume ','
		p.nextToken() // next arg
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

/* -------------------- helpers -------------------- */

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) registerPrefix(t token.Type, fn prefixParseFn) {
	p.prefixParseFns[t] = fn
}

func (p *Parser) registerInfix(t token.Type, fn infixParseFn) {
	p.infixParseFns[t] = fn
}

func (p *Parser) expectPeek(t token.Type) bool {
	p.skipSeparatorsPeek()
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) expectPeekNoSkip(t token.Type) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) errorAt(tok token.Token, msg string) {
	length := 1
	if tok.Literal != "" {
		length = len([]rune(tok.Literal))
	}
	p.diags = append(p.diags, diag.Diagnostic{
		Code:     "WP0001",
		Message:  msg,
		Severity: diag.SeverityError,
		Range: diag.Range{
			Line:   tok.Line,
			Col:    tok.Col,
			Length: length,
		},
	})
	p.errors = append(p.errors, msg)
}

func (p *Parser) peekError(t token.Type) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errorAt(p.peekToken, msg)
}

func (p *Parser) noPrefixParseFnError(t token.Type) {
	msg := fmt.Sprintf("no prefix parse function for %s", t)
	p.errorAt(p.curToken, msg)
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) isSeparator(t token.Type) bool {
	return t == token.NEWLINE || t == token.SEMICOLON
}

func (p *Parser) skipSeparatorsPeek() {
	for p.peekToken.Type == token.NEWLINE || p.peekToken.Type == token.SEMICOLON {
		p.nextToken()
	}
}

func (p *Parser) isTerminator(t token.Type) bool {
	return t == token.NEWLINE || t == token.SEMICOLON || t == token.RBRACE || t == token.RPAREN || t == token.EOF
}

func (p *Parser) peekIsTerminator() bool {
	return p.isTerminator(p.peekToken.Type)
}
