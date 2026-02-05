package parser

import (
	"fmt"
	"strings"

	"welle/internal/ast"
	"welle/internal/diag"
	"welle/internal/lexer"
	"welle/internal/numlit"
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
	ASSIGNPREC
	COALESCEPREC
	TERNARYPREC
	ORPREC      // or
	ANDPREC     // and
	BITORPREC   // |
	BITXORPREC  // ^
	BITANDPREC  // &
	EQUALS      // == !=
	LESSGREATER // < <= > >=
	SHIFT       // << >>
	SUM         // + -
	PRODUCT     // * / %
	PREFIX      // -X, not X
	INDEX       // array[index]
	CALL        // fn(X)
)

var precedences = map[token.Type]int{
	token.ASSIGN:         ASSIGNPREC,
	token.WALRUS:         ASSIGNPREC,
	token.PLUS_ASSIGN:    ASSIGNPREC,
	token.MINUS_ASSIGN:   ASSIGNPREC,
	token.STAR_ASSIGN:    ASSIGNPREC,
	token.SLASH_ASSIGN:   ASSIGNPREC,
	token.PERCENT_ASSIGN: ASSIGNPREC,
	token.NULLISH:        COALESCEPREC,
	token.QUESTION:       TERNARYPREC,
	token.IF:             TERNARYPREC,
	token.OR:             ORPREC,
	token.AND:            ANDPREC,
	token.BITOR:          BITORPREC,
	token.BITXOR:         BITXORPREC,
	token.BITAND:         BITANDPREC,
	token.EQ:             EQUALS,
	token.NE:             EQUALS,
	token.IS:             EQUALS,
	token.LT:             LESSGREATER,
	token.LE:             LESSGREATER,
	token.GT:             LESSGREATER,
	token.GE:             LESSGREATER,
	token.IN:             LESSGREATER,
	token.SHL:            SHIFT,
	token.SHR:            SHIFT,
	token.PLUS:           SUM,
	token.MINUS:          SUM,
	token.STAR:           PRODUCT,
	token.SLASH:          PRODUCT,
	token.PERCENT:        PRODUCT,
	token.LBRACKET:       INDEX,
	token.LPAREN:         CALL,
	token.DOT:            CALL,
	token.TEMPLATE:       CALL,
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
	p.registerPrefix(token.TEMPLATE, p.parseTemplateLiteral)
	p.registerPrefix(token.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(token.FALSE, p.parseBooleanLiteral)
	p.registerPrefix(token.NIL, p.parseNilLiteral)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseListLiteral)
	p.registerPrefix(token.HASH, p.parseDictLiteral)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.BITNOT, p.parsePrefixExpression)
	p.registerPrefix(token.MATCH, p.parseMatchExpression)
	p.registerPrefix(token.FUNC, p.parseFunctionLiteral)

	// Infix parsers
	for _, tt := range []token.Type{
		token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT,
		token.EQ, token.NE, token.LT, token.LE, token.GT, token.GE,
		token.IS,
		token.AND, token.OR,
		token.BITOR, token.BITXOR, token.BITAND, token.SHL, token.SHR,
		token.IN,
	} {
		p.registerInfix(tt, p.parseInfixExpression)
	}
	for _, tt := range []token.Type{
		token.ASSIGN, token.WALRUS, token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN, token.PERCENT_ASSIGN,
	} {
		p.registerInfix(tt, p.parseAssignmentExpression)
	}
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.DOT, p.parseMemberExpression)
	p.registerInfix(token.TEMPLATE, p.parseTaggedTemplate)
	p.registerInfix(token.NULLISH, p.parseNullishExpression)
	p.registerInfix(token.QUESTION, p.parseConditionalExpression)
	p.registerInfix(token.IF, p.parseCondExpression)

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
	case token.PASS:
		return &ast.PassStatement{Token: p.curToken}
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
		if p.curToken.Type == token.IDENT && isAssignOperator(p.peekToken.Type) {
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

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()

	return lit
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
	if p.peekIsTerminator() {
		return stmt
	}

	p.nextToken()
	stmt.ReturnValues = append(stmt.ReturnValues, p.parseExpression(LOWEST))
	for p.peekToken.Type == token.COMMA {
		p.nextToken()
		p.nextToken()
		stmt.ReturnValues = append(stmt.ReturnValues, p.parseExpression(LOWEST))
	}
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
	// curToken is IDENT, peek is assignment operator
	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	p.nextToken() // now assignment operator
	if !isAssignOperator(p.curToken.Type) {
		p.errorAt(p.curToken, "expected assignment operator")
		return nil
	}
	stmt.OpToken = p.curToken
	stmt.Op = p.curToken.Type

	p.nextToken() // start of value expression
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseSimpleAssignStatement() ast.Statement {
	if p.curToken.Type != token.IDENT {
		p.errorAt(p.curToken, "expected identifier in assignment")
		return nil
	}
	if !isAssignOperator(p.peekToken.Type) {
		p.errorAt(p.peekToken, "expected assignment operator")
		return nil
	}

	stmt := &ast.AssignStatement{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	p.nextToken() // now assignment operator
	stmt.OpToken = p.curToken
	stmt.Op = p.curToken.Type
	p.nextToken() // start of value expression
	stmt.Value = p.parseExpression(LOWEST)

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	if p.curToken.Type == token.LPAREN && p.peekDestructureAssign() {
		return p.parseDestructureAssignStatement()
	}
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if tl, ok := stmt.Expression.(*ast.TupleLiteral); ok && p.peekToken.Type == token.ASSIGN {
		if len(tl.Elements) == 0 {
			p.errorAt(tl.Token, "destructuring assignment requires at least one target")
			return nil
		}
		targets := make([]*ast.DestructureTarget, 0, len(tl.Elements))
		for _, el := range tl.Elements {
			ident, ok := el.(*ast.Identifier)
			if !ok {
				p.errorAt(tl.Token, "destructuring assignment targets must be identifiers or '_'")
				return nil
			}
			targets = append(targets, &ast.DestructureTarget{Token: ident.Token, Name: ident})
		}
		p.nextToken() // now assignment operator
		assign := &ast.DestructureAssignStatement{
			Token:   tl.Token,
			OpToken: p.curToken,
			Op:      p.curToken.Type,
			Targets: targets,
		}
		p.nextToken() // start of value expression
		assign.Value = p.parseExpression(LOWEST)
		return assign
	}
	if ae, ok := stmt.Expression.(*ast.AssignExpression); ok {
		switch left := ae.Left.(type) {
		case *ast.Identifier:
			return &ast.AssignStatement{
				Token:   left.Token,
				OpToken: ae.Token,
				Op:      ae.Op,
				Name:    left,
				Value:   ae.Value,
			}
		case *ast.IndexExpression:
			return &ast.IndexAssignStatement{Token: ae.Token, Op: ae.Op, Left: left, Value: ae.Value}
		case *ast.MemberExpression:
			return &ast.MemberAssignStatement{
				Token:    ae.Token,
				Op:       ae.Op,
				Object:   left.Object,
				Property: left.Property,
				Value:    ae.Value,
			}
		}
	}
	if idx, ok := stmt.Expression.(*ast.IndexExpression); ok && isAssignOperator(p.peekToken.Type) {
		if p.peekToken.Type == token.WALRUS {
			p.errorAt(p.peekToken, "invalid walrus target")
			return nil
		}
		p.nextToken() // now assignment operator
		assign := &ast.IndexAssignStatement{Token: p.curToken, Op: p.curToken.Type, Left: idx}

		p.nextToken() // start of value expression
		assign.Value = p.parseExpression(LOWEST)
		return assign
	}
	if me, ok := stmt.Expression.(*ast.MemberExpression); ok && isAssignOperator(p.peekToken.Type) {
		if p.peekToken.Type == token.WALRUS {
			p.errorAt(p.peekToken, "invalid walrus target")
			return nil
		}
		p.nextToken() // now assignment operator
		assign := &ast.MemberAssignStatement{
			Token:    p.curToken,
			Op:       p.curToken.Type,
			Object:   me.Object,
			Property: me.Property,
		}

		p.nextToken() // start of value expression
		assign.Value = p.parseExpression(LOWEST)
		return assign
	}
	return stmt
}

func (p *Parser) peekDestructureAssign() bool {
	if p.curToken.Type != token.LPAREN {
		return false
	}
	lex := *p.l
	first := true
	next := func() token.Token {
		if first {
			first = false
			return p.peekToken
		}
		tok := lex.NextToken()
		for tok.Type == token.NEWLINE {
			tok = lex.NextToken()
		}
		return tok
	}

	seenStar := false
	seenComma := false
	seenTarget := false
	for {
		tok := next()
		if tok.Type == token.RPAREN {
			if !seenTarget {
				return false
			}
			tok = next()
			return isAssignOperator(tok.Type) && (seenStar || seenComma)
		}
		if tok.Type == token.COMMA {
			seenComma = true
			continue
		}
		if tok.Type == token.STAR {
			if seenStar {
				return false
			}
			seenStar = true
			tok = next()
		}
		if tok.Type != token.IDENT {
			return false
		}
		seenTarget = true
	}
}

func (p *Parser) parseDestructureAssignStatement() ast.Statement {
	stmt := &ast.DestructureAssignStatement{Token: p.curToken}
	if p.peekToken.Type == token.RPAREN {
		p.errorAt(p.curToken, "destructuring assignment requires at least one target")
		return nil
	}

	seenStar := false
	p.nextToken()
	for {
		switch p.curToken.Type {
		case token.STAR:
			if seenStar {
				p.errorAt(p.curToken, "destructuring assignment allows only one starred target")
				return nil
			}
			seenStar = true
			starTok := p.curToken
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			stmt.Targets = append(stmt.Targets, &ast.DestructureTarget{Token: starTok, Name: ident, Star: true})
		case token.IDENT:
			ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			stmt.Targets = append(stmt.Targets, &ast.DestructureTarget{Token: p.curToken, Name: ident})
		default:
			p.errorAt(p.curToken, "destructuring assignment targets must be identifiers or '_'")
			return nil
		}

		if p.peekToken.Type != token.COMMA {
			if !p.expectPeek(token.RPAREN) {
				return nil
			}
			break
		}
		p.nextToken() // consume ','
		if p.peekToken.Type == token.RPAREN {
			p.nextToken() // consume ')'
			break
		}
		p.nextToken()
	}

	if !isAssignOperator(p.peekToken.Type) {
		p.errorAt(p.peekToken, "expected assignment operator")
		return nil
	}
	p.nextToken() // now assignment operator
	stmt.OpToken = p.curToken
	stmt.Op = p.curToken.Type

	p.nextToken() // start of value expression
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

func isAssignOperator(tt token.Type) bool {
	switch tt {
	case token.ASSIGN, token.WALRUS, token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.STAR_ASSIGN, token.SLASH_ASSIGN, token.PERCENT_ASSIGN, token.BITOR_ASSIGN:
		return true
	default:
		return false
	}
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
	p.skipSeparatorsPeek()
	if p.peekToken.Type == token.LBRACE {
		p.nextToken()
		stmt.Consequence = p.parseBlockStatement()
	} else {
		p.nextToken()
		stmt.Consequence = p.parseStatement()
		if stmt.Consequence == nil {
			return nil
		}
	}

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
			stmt.Alternative = elseIfStmt
			return stmt
		}
		p.skipSeparatorsPeek()
		if p.peekToken.Type == token.LBRACE {
			p.nextToken()
			stmt.Alternative = p.parseBlockStatement()
		} else {
			p.nextToken()
			stmt.Alternative = p.parseStatement()
			if stmt.Alternative == nil {
				return nil
			}
		}
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
		if p.peekForInParensDestructure() || p.peekForInParensSingle() {
			p.nextToken() // consume '('
			p.nextToken() // move to first token inside '('
			return p.parseForInStatementFromCur(forTok, true)
		}
		if p.peekForInParensTooMany() {
			p.nextToken() // consume '('
			p.nextToken() // move to first token inside '('
			p.errorAt(p.curToken, "for-in destructuring requires exactly two targets")
			return nil
		}
		if p.peekForInParensGrouped() {
			p.nextToken() // consume '('
			p.nextToken() // move to first token inside '('
			p.errorAt(p.curToken, "invalid for-in binding pattern: use identifier or '(k, v)'")
			return nil
		}
		p.nextToken() // consume '('
		p.nextToken() // move to first token inside '('
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
	firstIdent := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if hasParens && p.peekToken.Type == token.COMMA {
		stmt.Destruct = true
		stmt.Key = firstIdent
		p.nextToken() // consume ','
		p.nextToken() // move to second identifier
		if p.curToken.Type != token.IDENT {
			p.errorAt(p.curToken, "expected identifier in for-in destructuring")
			return nil
		}
		stmt.Value = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if p.peekToken.Type == token.COMMA {
			p.errorAt(p.peekToken, "for-in destructuring requires exactly two targets")
			return nil
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		if !p.expectPeek(token.IN) {
			return nil
		}
		p.nextToken()
		stmt.Iterable = p.parseExpression(LOWEST)
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		stmt.Body = p.parseBlockStatement()
		return stmt
	}
	stmt.Var = firstIdent

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

func (p *Parser) peekForInParensDestructure() bool {
	if p.peekToken.Type != token.LPAREN {
		return false
	}
	lex := *p.l
	next := func() token.Token {
		tok := lex.NextToken()
		for tok.Type == token.NEWLINE {
			tok = lex.NextToken()
		}
		return tok
	}

	t1 := next()
	if t1.Type != token.IDENT {
		return false
	}
	t2 := next()
	if t2.Type != token.COMMA {
		return false
	}
	t3 := next()
	if t3.Type != token.IDENT {
		return false
	}
	t4 := next()
	if t4.Type != token.RPAREN {
		return false
	}
	t5 := next()
	return t5.Type == token.IN
}

func (p *Parser) peekForInParensSingle() bool {
	if p.peekToken.Type != token.LPAREN {
		return false
	}
	lex := *p.l
	next := func() token.Token {
		tok := lex.NextToken()
		for tok.Type == token.NEWLINE {
			tok = lex.NextToken()
		}
		return tok
	}

	t1 := next()
	if t1.Type != token.IDENT {
		return false
	}
	t2 := next()
	return t2.Type == token.IN
}

func (p *Parser) peekForInParensGrouped() bool {
	if p.peekToken.Type != token.LPAREN {
		return false
	}
	lex := *p.l
	next := func() token.Token {
		tok := lex.NextToken()
		for tok.Type == token.NEWLINE {
			tok = lex.NextToken()
		}
		return tok
	}

	t1 := next()
	if t1.Type != token.IDENT {
		return false
	}
	t2 := next()
	if t2.Type != token.RPAREN {
		return false
	}
	t3 := next()
	return t3.Type == token.IN
}

func (p *Parser) peekForInParensTooMany() bool {
	if p.peekToken.Type != token.LPAREN {
		return false
	}
	lex := *p.l
	next := func() token.Token {
		tok := lex.NextToken()
		for tok.Type == token.NEWLINE {
			tok = lex.NextToken()
		}
		return tok
	}

	t1 := next()
	if t1.Type != token.IDENT {
		return false
	}
	t2 := next()
	if t2.Type != token.COMMA {
		return false
	}
	t3 := next()
	if t3.Type != token.IDENT {
		return false
	}
	t4 := next()
	if t4.Type != token.COMMA {
		return false
	}

	for {
		tok := next()
		if tok.Type == token.RPAREN {
			tok = next()
			return tok.Type == token.IN
		}
		if tok.Type == token.EOF {
			return false
		}
	}
}

func (p *Parser) parseCForStatementFromCur(forTok token.Token) ast.Statement {
	stmt := &ast.ForStatement{Token: forTok}

	// init
	if p.curToken.Type != token.SEMICOLON {
		startTok := p.curToken
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil
		}
		stmt.Init = &ast.ExpressionStatement{Token: startTok, Expression: expr}
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
		startTok := p.curToken
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			return nil
		}
		stmt.Post = &ast.ExpressionStatement{Token: startTok, Expression: expr}
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
	if p.curToken.Type == token.EOF {
		p.errorAt(p.curToken, "unterminated block")
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
		if isAssignOperator(p.peekToken.Type) {
			if _, ok := leftExp.(*ast.TupleLiteral); ok {
				return leftExp
			}
		}
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken() // advance to infix operator (or '(' for call)
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseAssignmentExpression(left ast.Expression) ast.Expression {
	if !isAssignOperator(p.curToken.Type) {
		p.errorAt(p.curToken, "expected assignment operator")
		return nil
	}

	if p.curToken.Type == token.WALRUS {
		if _, ok := left.(*ast.Identifier); !ok {
			p.errorAt(p.curToken, "invalid walrus target")
			return nil
		}
	} else {
		switch left.(type) {
		case *ast.Identifier, *ast.IndexExpression, *ast.MemberExpression:
		default:
			p.errorAt(p.curToken, "invalid assignment target")
			return nil
		}
	}

	opTok := p.curToken
	prec := p.curPrecedence() - 1
	if prec < LOWEST {
		prec = LOWEST
	}

	p.nextToken()
	value := p.parseExpression(prec)
	if value == nil {
		return nil
	}

	return &ast.AssignExpression{
		Token: opTok,
		Op:    opTok.Type,
		Left:  left,
		Value: value,
	}
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	v, err := numlit.ParseIntLiteral(p.curToken.Literal)
	if err != nil {
		p.errorAt(p.curToken, err.Error())
		return nil
	}
	lit.Value = v
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	v, err := numlit.ParseFloatLiteral(p.curToken.Literal)
	if err != nil {
		p.errorAt(p.curToken, err.Error())
		return nil
	}
	lit.Value = v
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseTemplateLiteral() ast.Expression {
	parts, exprs, ok := p.parseTemplateParts(p.curToken)
	if !ok {
		return nil
	}
	return &ast.TemplateLiteral{
		Token: p.curToken,
		Parts: parts,
		Exprs: exprs,
	}
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curToken.Type == token.TRUE}
}

func (p *Parser) parseNilLiteral() ast.Expression {
	return &ast.NilLiteral{Token: p.curToken}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	// curToken is '('
	tok := p.curToken
	if p.peekToken.Type == token.RPAREN {
		p.nextToken()
		return &ast.TupleLiteral{Token: tok, Elements: []ast.Expression{}}
	}

	p.nextToken()
	first := p.parseExpression(LOWEST)
	if first == nil {
		return nil
	}

	if p.peekToken.Type != token.COMMA {
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return first
	}

	lit := &ast.TupleLiteral{Token: tok, Elements: []ast.Expression{first}}
	for p.peekToken.Type == token.COMMA {
		p.nextToken() // consume ','
		if p.peekToken.Type == token.RPAREN {
			p.nextToken() // consume ')'
			return lit
		}
		p.nextToken()
		elem := p.parseExpression(LOWEST)
		if elem == nil {
			return nil
		}
		lit.Elements = append(lit.Elements, elem)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return lit
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

func (p *Parser) parseNullishExpression(left ast.Expression) ast.Expression {
	exp := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	prec := p.curPrecedence() - 1
	if prec < LOWEST {
		prec = LOWEST
	}
	p.nextToken()
	exp.Right = p.parseExpression(prec)
	return exp
}

func (p *Parser) parseConditionalExpression(cond ast.Expression) ast.Expression {
	exp := &ast.ConditionalExpression{
		Token: p.curToken,
		Cond:  cond,
	}

	prec := p.curPrecedence() - 1
	if prec < LOWEST {
		prec = LOWEST
	}

	p.nextToken()
	exp.Then = p.parseExpression(prec)
	if exp.Then == nil {
		return nil
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	exp.Else = p.parseExpression(prec)
	if exp.Else == nil {
		return nil
	}

	return exp
}

func (p *Parser) parseCondExpression(then ast.Expression) ast.Expression {
	exp := &ast.CondExpr{
		Token: p.curToken,
		Then:  then,
	}

	prec := p.curPrecedence() - 1
	if prec < LOWEST {
		prec = LOWEST
	}

	p.nextToken()
	exp.Cond = p.parseExpression(prec)
	if exp.Cond == nil {
		return nil
	}

	if !p.expectPeek(token.ELSE) {
		return nil
	}

	p.nextToken()
	exp.Else = p.parseExpression(prec)
	if exp.Else == nil {
		return nil
	}

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

func (p *Parser) parseTaggedTemplate(tag ast.Expression) ast.Expression {
	parts, exprs, ok := p.parseTemplateParts(p.curToken)
	if !ok {
		return nil
	}
	return &ast.TemplateLiteral{
		Token:  p.curToken,
		Parts:  parts,
		Exprs:  exprs,
		Tagged: true,
		Tag:    tag,
	}
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
	pair := p.parseDictPair()
	if pair == nil {
		return nil
	}
	lit.Pairs = append(lit.Pairs, *pair)

	for p.peekToken.Type == token.COMMA {
		p.nextToken()
		p.nextToken()

		pair := p.parseDictPair()
		if pair == nil {
			return nil
		}
		lit.Pairs = append(lit.Pairs, *pair)
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return lit
}

func (p *Parser) parseDictPair() *ast.DictPair {
	if p.curToken.Type == token.IDENT && (p.peekToken.Type == token.COMMA || p.peekToken.Type == token.RBRACE) {
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		return &ast.DictPair{Shorthand: ident}
	}

	key := p.parseExpression(LOWEST)
	if key == nil {
		return nil
	}
	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	val := p.parseExpression(LOWEST)
	if val == nil {
		return nil
	}
	return &ast.DictPair{Key: key, Value: val}
}

func (p *Parser) parseListLiteral() ast.Expression {
	tok := p.curToken
	if p.peekToken.Type == token.RBRACKET {
		p.nextToken()
		return &ast.ListLiteral{Token: tok, Elements: []ast.Expression{}}
	}

	p.nextToken()
	first := p.parseExpression(LOWEST)
	if first == nil {
		return nil
	}

	if p.peekToken.Type == token.FOR {
		p.nextToken() // consume 'for'
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		varIdent := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		if !p.expectPeek(token.IN) {
			return nil
		}
		p.nextToken()
		var seq ast.Expression
		savedIf, hadIf := p.infixParseFns[token.IF]
		if hadIf {
			delete(p.infixParseFns, token.IF)
		}
		seq = p.parseExpression(LOWEST)
		if hadIf {
			p.infixParseFns[token.IF] = savedIf
		}
		if seq == nil {
			return nil
		}
		var filter ast.Expression
		if p.peekToken.Type == token.IF {
			p.nextToken()
			p.nextToken()
			filter = p.parseExpression(LOWEST)
			if filter == nil {
				return nil
			}
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return &ast.ListComprehension{
			Token:  tok,
			Elem:   first,
			Var:    varIdent,
			Seq:    seq,
			Filter: filter,
		}
	}

	lit := &ast.ListLiteral{Token: tok, Elements: []ast.Expression{first}}
	for p.peekToken.Type == token.COMMA {
		p.nextToken()
		p.nextToken()
		elem := p.parseExpression(LOWEST)
		if elem == nil {
			return nil
		}
		lit.Elements = append(lit.Elements, elem)
	}
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}
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
		var step ast.Expression
		if p.peekToken.Type != token.COLON && p.peekToken.Type != token.RBRACKET {
			p.nextToken()
			high = p.parseExpression(LOWEST)
		}
		if p.peekToken.Type == token.COLON {
			p.nextToken()
			if p.peekToken.Type != token.RBRACKET {
				p.nextToken()
				step = p.parseExpression(LOWEST)
			}
		}
		if !p.expectPeek(token.RBRACKET) {
			return nil
		}
		return &ast.SliceExpression{Token: tok, Left: left, Low: nil, High: high, Step: step}
	}

	p.nextToken()
	first := p.parseExpression(LOWEST)

	if p.peekToken.Type == token.COLON {
		p.nextToken()

		var high ast.Expression
		var step ast.Expression
		if p.peekToken.Type != token.RBRACKET {
			p.nextToken()
			high = p.parseExpression(LOWEST)
		}

		if p.peekToken.Type == token.COLON {
			p.nextToken()
			if p.peekToken.Type != token.RBRACKET {
				p.nextToken()
				step = p.parseExpression(LOWEST)
			}
		}

		if !p.expectPeek(token.RBRACKET) {
			return nil
		}

		return &ast.SliceExpression{Token: tok, Left: left, Low: first, High: high, Step: step}
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
	args = append(args, p.parseCallArgument())

	for p.peekToken.Type == token.COMMA {
		p.nextToken() // consume ','
		p.nextToken() // next arg
		args = append(args, p.parseCallArgument())
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

func (p *Parser) parseCallArgument() ast.Expression {
	if p.curToken.Type != token.ELLIPSIS {
		return p.parseExpression(LOWEST)
	}

	tok := p.curToken
	p.nextToken()
	value := p.parseExpression(LOWEST)
	if value == nil {
		return nil
	}
	return &ast.SpreadExpression{Token: tok, Value: value}
}

func (p *Parser) parseTemplateParts(tok token.Token) ([]string, []ast.Expression, bool) {
	raw := tok.Literal
	parts := make([]string, 0, 4)
	exprs := make([]ast.Expression, 0, 4)
	partStart := 0
	i := 0

	for i < len(raw) {
		if raw[i] == '$' && i+1 < len(raw) && raw[i+1] == '{' {
			part, err := decodeTemplatePart(raw[partStart:i])
			if err != nil {
				p.errorAt(tok, err.Error())
				return nil, nil, false
			}
			parts = append(parts, part)

			exprStart := i + 2
			exprEnd, ok := findTemplateExprEnd(raw, exprStart)
			if !ok {
				p.errorAt(tok, "malformed template interpolation: missing '}'")
				return nil, nil, false
			}
			exprRaw := raw[exprStart:exprEnd]
			expr, ok := p.parseTemplateInterpolation(tok, exprRaw)
			if !ok {
				return nil, nil, false
			}
			exprs = append(exprs, expr)
			i = exprEnd + 1
			partStart = i
			continue
		}
		i++
	}

	part, err := decodeTemplatePart(raw[partStart:])
	if err != nil {
		p.errorAt(tok, err.Error())
		return nil, nil, false
	}
	parts = append(parts, part)
	return parts, exprs, true
}

func (p *Parser) parseTemplateInterpolation(tok token.Token, exprRaw string) (ast.Expression, bool) {
	if strings.TrimSpace(exprRaw) == "" {
		p.errorAt(tok, "empty template interpolation")
		return nil, false
	}
	sub := New(lexer.New(exprRaw))
	program := sub.ParseProgram()
	if len(sub.Errors()) > 0 {
		p.errorAt(tok, "invalid template interpolation: "+sub.Errors()[0])
		return nil, false
	}
	if len(program.Statements) != 1 {
		p.errorAt(tok, "template interpolation must contain exactly one expression")
		return nil, false
	}
	es, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok || es.Expression == nil {
		p.errorAt(tok, "template interpolation must be an expression")
		return nil, false
	}
	return es.Expression, true
}

func decodeTemplatePart(raw string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			b.WriteByte(raw[i])
			continue
		}
		if i+1 >= len(raw) {
			b.WriteByte('\\')
			break
		}
		switch raw[i+1] {
		case '"':
			b.WriteByte('"')
			i++
		case '\\':
			b.WriteByte('\\')
			i++
		case 'n':
			b.WriteByte('\n')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		default:
			b.WriteByte('\\')
		}
	}
	return b.String(), nil
}

func findTemplateExprEnd(raw string, start int) (int, bool) {
	depth := 0
	i := start
	for i < len(raw) {
		switch raw[i] {
		case '"':
			if i+2 < len(raw) && raw[i+1] == '"' && raw[i+2] == '"' {
				i += 3
				for i < len(raw) {
					if i+2 < len(raw) && raw[i] == '"' && raw[i+1] == '"' && raw[i+2] == '"' {
						i += 3
						break
					}
					i++
				}
				continue
			}
			i++
			for i < len(raw) {
				if raw[i] == '\\' {
					i += 2
					continue
				}
				if raw[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		case '`':
			i++
			for i < len(raw) && raw[i] != '`' {
				i++
			}
			if i < len(raw) {
				i++
			}
			continue
		case '/':
			if i+1 < len(raw) && raw[i+1] == '/' {
				i += 2
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(raw) && raw[i+1] == '*' {
				i += 2
				for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
					i++
				}
				if i+1 < len(raw) {
					i += 2
				}
				continue
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i, true
			}
			depth--
		}
		i++
	}
	return 0, false
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
