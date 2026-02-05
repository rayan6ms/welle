package ast

import (
	"bytes"
	"strings"

	"welle/internal/token"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
		out.WriteString("\n")
	}
	return out.String()
}

/* -------------------- Statements -------------------- */

type ExpressionStatement struct {
	Token      token.Token // first token of expression
	Expression Expression
}

func (*ExpressionStatement) statementNode()          {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression == nil {
		return ""
	}
	return es.Expression.String()
}

type AssignStatement struct {
	Token   token.Token // identifier token
	OpToken token.Token // assignment operator token
	Op      token.Type
	Name    *Identifier
	Value   Expression
}

func (*AssignStatement) statementNode()          {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(as.Name.String())
	if as.OpToken.Literal != "" {
		out.WriteString(" ")
		out.WriteString(as.OpToken.Literal)
		out.WriteString(" ")
	} else {
		out.WriteString(" = ")
	}
	if as.Value != nil {
		out.WriteString(as.Value.String())
	}
	return out.String()
}

type IndexAssignStatement struct {
	Token token.Token // assignment operator
	Op    token.Type
	Left  Expression // *IndexExpression
	Value Expression
}

func (*IndexAssignStatement) statementNode()         {}
func (s *IndexAssignStatement) TokenLiteral() string { return s.Token.Literal }
func (s *IndexAssignStatement) String() string {
	op := s.Token.Literal
	if op == "" {
		op = "="
	}
	return s.Left.String() + " " + op + " " + s.Value.String()
}

type MemberAssignStatement struct {
	Token    token.Token // assignment operator
	Op       token.Type
	Object   Expression
	Property *Identifier
	Value    Expression
}

func (*MemberAssignStatement) statementNode()         {}
func (s *MemberAssignStatement) TokenLiteral() string { return s.Token.Literal }
func (s *MemberAssignStatement) String() string {
	op := s.Token.Literal
	if op == "" {
		op = "="
	}
	return s.Object.String() + "." + s.Property.String() + " " + op + " " + s.Value.String()
}

type ReturnStatement struct {
	Token        token.Token // 'return'
	ReturnValues []Expression
}

func (*ReturnStatement) statementNode()          {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString("return")
	if len(rs.ReturnValues) > 0 {
		out.WriteString(" ")
		for i, v := range rs.ReturnValues {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(v.String())
		}
	}
	return out.String()
}

type DestructureAssignStatement struct {
	Token   token.Token // '('
	OpToken token.Token // assignment operator token
	Op      token.Type
	Targets []*DestructureTarget
	Value   Expression
}

func (*DestructureAssignStatement) statementNode()          {}
func (ds *DestructureAssignStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DestructureAssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	for i, t := range ds.Targets {
		if i > 0 {
			out.WriteString(", ")
		}
		if t != nil {
			out.WriteString(t.String())
		}
	}
	out.WriteString(")")
	if ds.OpToken.Literal != "" {
		out.WriteString(" ")
		out.WriteString(ds.OpToken.Literal)
		out.WriteString(" ")
	} else {
		out.WriteString(" = ")
	}
	if ds.Value != nil {
		out.WriteString(ds.Value.String())
	}
	return out.String()
}

type DestructureTarget struct {
	Token token.Token // identifier or '*'
	Name  *Identifier
	Star  bool
}

func (dt *DestructureTarget) String() string {
	if dt == nil || dt.Name == nil {
		return ""
	}
	if dt.Star {
		return "*" + dt.Name.String()
	}
	return dt.Name.String()
}

type DeferStatement struct {
	Token token.Token // 'defer'
	Call  Expression
}

func (*DeferStatement) statementNode()          {}
func (ds *DeferStatement) TokenLiteral() string { return ds.Token.Literal }
func (ds *DeferStatement) String() string {
	var out bytes.Buffer
	out.WriteString("defer ")
	if ds.Call != nil {
		out.WriteString(ds.Call.String())
	}
	return out.String()
}

type ThrowStatement struct {
	Token token.Token // 'throw'
	Value Expression
}

func (*ThrowStatement) statementNode()          {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) String() string {
	var out bytes.Buffer
	out.WriteString("throw ")
	if ts.Value != nil {
		out.WriteString(ts.Value.String())
	}
	return out.String()
}

type BreakStatement struct {
	Token token.Token // 'break'
}

func (*BreakStatement) statementNode()          {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return "break" }

type ContinueStatement struct {
	Token token.Token // 'continue'
}

func (*ContinueStatement) statementNode()          {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return "continue" }

type PassStatement struct {
	Token token.Token // 'pass'
}

func (*PassStatement) statementNode()          {}
func (ps *PassStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PassStatement) String() string       { return "pass" }

type ImportStatement struct {
	Token token.Token // 'import'
	Path  *StringLiteral
	Alias *Identifier
}

func (*ImportStatement) statementNode()          {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	if is.Alias != nil {
		return "import " + is.Path.String() + " as " + is.Alias.String()
	}
	return "import " + is.Path.String()
}

type ImportItem struct {
	Name  *Identifier
	Alias *Identifier // optional
}

type FromImportStatement struct {
	Token token.Token // 'from'
	Path  *StringLiteral
	Items []ImportItem
}

func (*FromImportStatement) statementNode()          {}
func (fs *FromImportStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FromImportStatement) String() string {
	var out bytes.Buffer
	out.WriteString("from ")
	out.WriteString(fs.Path.String())
	out.WriteString(" import ")
	for i, it := range fs.Items {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(it.Name.String())
		if it.Alias != nil {
			out.WriteString(" as ")
			out.WriteString(it.Alias.String())
		}
	}
	return out.String()
}

type ExportStatement struct {
	Token token.Token // 'export'
	Stmt  Statement
}

func (*ExportStatement) statementNode()          {}
func (es *ExportStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExportStatement) String() string {
	return "export " + es.Stmt.String()
}

type BlockStatement struct {
	Token      token.Token // '{'
	Statements []Statement
}

func (*BlockStatement) statementNode()          {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	out.WriteString("{\n")
	for _, s := range bs.Statements {
		out.WriteString("  ")
		out.WriteString(s.String())
		out.WriteString("\n")
	}
	out.WriteString("}")
	return out.String()
}

type TryStatement struct {
	Token        token.Token // 'try'
	TryBlock     *BlockStatement
	CatchToken   token.Token // 'catch' (optional)
	CatchName    *Identifier
	CatchBlock   *BlockStatement
	FinallyToken token.Token // 'finally' (optional)
	FinallyBlock *BlockStatement
}

func (*TryStatement) statementNode()          {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TryStatement) String() string {
	var out bytes.Buffer
	out.WriteString("try ")
	out.WriteString(ts.TryBlock.String())
	if ts.CatchBlock != nil {
		out.WriteString(" catch (")
		out.WriteString(ts.CatchName.String())
		out.WriteString(") ")
		out.WriteString(ts.CatchBlock.String())
	}
	if ts.FinallyBlock != nil {
		out.WriteString(" finally ")
		out.WriteString(ts.FinallyBlock.String())
	}
	return out.String()
}

type IfStatement struct {
	Token       token.Token // 'if'
	Condition   Expression
	Consequence Statement
	Alternative Statement
}

func (*IfStatement) statementNode()          {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) String() string {
	var out bytes.Buffer
	out.WriteString("if (")
	out.WriteString(is.Condition.String())
	out.WriteString(") ")
	if is.Consequence != nil {
		out.WriteString(is.Consequence.String())
	}
	if is.Alternative != nil {
		out.WriteString(" else ")
		out.WriteString(is.Alternative.String())
	}
	return out.String()
}

type WhileStatement struct {
	Token     token.Token // 'while'
	Condition Expression
	Body      *BlockStatement
}

func (*WhileStatement) statementNode()          {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while (")
	out.WriteString(ws.Condition.String())
	out.WriteString(") ")
	out.WriteString(ws.Body.String())
	return out.String()
}

type ForStatement struct {
	Token token.Token // 'for'
	Init  Statement   // may be nil
	Cond  Expression  // may be nil (treated as true)
	Post  Statement   // may be nil
	Body  *BlockStatement
}

func (*ForStatement) statementNode()          {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	return "for (...) " + fs.Body.String()
}

type ForInStatement struct {
	Token    token.Token // 'for'
	Var      *Identifier
	Key      *Identifier
	Value    *Identifier
	Destruct bool
	Iterable Expression
	Body     *BlockStatement
}

func (*ForInStatement) statementNode()          {}
func (fs *ForInStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForInStatement) String() string {
	var out bytes.Buffer
	out.WriteString("for (")
	if fs.Destruct {
		if fs.Key != nil {
			out.WriteString(fs.Key.String())
		}
		out.WriteString(", ")
		if fs.Value != nil {
			out.WriteString(fs.Value.String())
		}
	} else if fs.Var != nil {
		out.WriteString(fs.Var.String())
	}
	out.WriteString(" in ")
	out.WriteString(fs.Iterable.String())
	out.WriteString(") ")
	out.WriteString(fs.Body.String())
	return out.String()
}

type CaseClause struct {
	Token  token.Token // 'case'
	Values []Expression
	Body   *BlockStatement
}

type SwitchStatement struct {
	Token   token.Token // 'switch'
	Value   Expression
	Cases   []*CaseClause
	Default *BlockStatement // optional
}

func (*SwitchStatement) statementNode()          {}
func (ss *SwitchStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *SwitchStatement) String() string {
	var out bytes.Buffer
	out.WriteString("switch (")
	out.WriteString(ss.Value.String())
	out.WriteString(") {")
	for _, c := range ss.Cases {
		out.WriteString(" case ")
		for i, val := range c.Values {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(val.String())
		}
		out.WriteString(" ")
		out.WriteString(c.Body.String())
	}
	if ss.Default != nil {
		out.WriteString(" default ")
		out.WriteString(ss.Default.String())
	}
	out.WriteString(" }")
	return out.String()
}

type FuncStatement struct {
	Token      token.Token // 'func'
	Name       *Identifier
	Parameters []*Identifier
	Body       *BlockStatement
}

func (*FuncStatement) statementNode()          {}
func (fs *FuncStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FuncStatement) String() string {
	var out bytes.Buffer
	out.WriteString("func ")
	out.WriteString(fs.Name.String())
	out.WriteString("(")
	for i, p := range fs.Parameters {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(p.String())
	}
	out.WriteString(") ")
	out.WriteString(fs.Body.String())
	return out.String()
}

/* -------------------- Expressions -------------------- */

type FunctionLiteral struct {
	Token      token.Token // 'func'
	Parameters []*Identifier
	Body       *BlockStatement
}

func (*FunctionLiteral) expressionNode()         {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("func(")
	for i, p := range fl.Parameters {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(p.String())
	}
	out.WriteString(") ")
	out.WriteString(fl.Body.String())
	return out.String()
}

type MatchCase struct {
	Token  token.Token // 'case'
	Values []Expression
	Result Expression
}

type MatchExpression struct {
	Token   token.Token // 'match'
	Value   Expression
	Cases   []*MatchCase
	Default Expression // optional
}

func (*MatchExpression) expressionNode()         {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MatchExpression) String() string {
	var out bytes.Buffer
	out.WriteString("match (")
	out.WriteString(me.Value.String())
	out.WriteString(") {")
	for _, c := range me.Cases {
		out.WriteString(" case ")
		for i, val := range c.Values {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(val.String())
		}
		out.WriteString(" { ")
		out.WriteString(c.Result.String())
		out.WriteString(" }")
	}
	if me.Default != nil {
		out.WriteString(" default { ")
		out.WriteString(me.Default.String())
		out.WriteString(" }")
	}
	out.WriteString(" }")
	return out.String()
}

type Identifier struct {
	Token token.Token // IDENT
	Value string
}

func (*Identifier) expressionNode()        {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type IntegerLiteral struct {
	Token token.Token // INT
	Value int64
}

func (*IntegerLiteral) expressionNode()         {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token // FLOAT
	Value float64
}

func (*FloatLiteral) expressionNode()         {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type StringLiteral struct {
	Token token.Token // STRING
	Value string
}

func (*StringLiteral) expressionNode()         {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

type TemplateLiteral struct {
	Token  token.Token // TEMPLATE
	Parts  []string
	Exprs  []Expression
	Tagged bool
	Tag    Expression
}

func (*TemplateLiteral) expressionNode()         {}
func (tl *TemplateLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TemplateLiteral) String() string {
	var out bytes.Buffer
	if tl.Tagged && tl.Tag != nil {
		out.WriteString(tl.Tag.String())
		out.WriteString(" ")
	}
	out.WriteString("t\"")
	for i, part := range tl.Parts {
		out.WriteString(escapeTemplatePart(part))
		if i < len(tl.Exprs) && tl.Exprs[i] != nil {
			out.WriteString("${")
			out.WriteString(tl.Exprs[i].String())
			out.WriteString("}")
		}
	}
	out.WriteString("\"")
	return out.String()
}

func escapeTemplatePart(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

type BooleanLiteral struct {
	Token token.Token // TRUE or FALSE
	Value bool
}

func (*BooleanLiteral) expressionNode()         {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) String() string       { return bl.Token.Literal }

type NilLiteral struct {
	Token token.Token // NIL
}

func (*NilLiteral) expressionNode()         {}
func (nl *NilLiteral) TokenLiteral() string { return nl.Token.Literal }
func (*NilLiteral) String() string          { return "nil" }

type PrefixExpression struct {
	Token    token.Token // prefix token, e.g. '-'
	Operator string
	Right    Expression
}

func (*PrefixExpression) expressionNode()         {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")
	return out.String()
}

type InfixExpression struct {
	Token    token.Token // operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (*InfixExpression) expressionNode()         {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" ")
	out.WriteString(ie.Operator)
	out.WriteString(" ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

type ConditionalExpression struct {
	Token token.Token // '?'
	Cond  Expression
	Then  Expression
	Else  Expression
}

func (*ConditionalExpression) expressionNode()         {}
func (ce *ConditionalExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *ConditionalExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ce.Cond.String())
	out.WriteString(" ? ")
	out.WriteString(ce.Then.String())
	out.WriteString(" : ")
	out.WriteString(ce.Else.String())
	out.WriteString(")")
	return out.String()
}

type CondExpr struct {
	Token token.Token // 'if'
	Then  Expression
	Cond  Expression
	Else  Expression
}

func (*CondExpr) expressionNode()         {}
func (ce *CondExpr) TokenLiteral() string { return ce.Token.Literal }
func (ce *CondExpr) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ce.Then.String())
	out.WriteString(" if ")
	out.WriteString(ce.Cond.String())
	out.WriteString(" else ")
	out.WriteString(ce.Else.String())
	out.WriteString(")")
	return out.String()
}

type AssignExpression struct {
	Token token.Token // assignment operator token
	Op    token.Type
	Left  Expression
	Value Expression
}

func (*AssignExpression) expressionNode()         {}
func (ae *AssignExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ae.Left.String())
	out.WriteString(" ")
	if ae.Token.Literal != "" {
		out.WriteString(ae.Token.Literal)
	} else {
		out.WriteString("=")
	}
	out.WriteString(" ")
	if ae.Value != nil {
		out.WriteString(ae.Value.String())
	}
	return out.String()
}

type MemberExpression struct {
	Token    token.Token // '.'
	Object   Expression
	Property *Identifier
}

func (*MemberExpression) expressionNode()         {}
func (me *MemberExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MemberExpression) String() string {
	var out bytes.Buffer
	out.WriteString(me.Object.String())
	out.WriteString(".")
	out.WriteString(me.Property.String())
	return out.String()
}

type CallExpression struct {
	Token     token.Token // '('
	Function  Expression  // identifier for now
	Arguments []Expression
}

func (*CallExpression) expressionNode()         {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Function.String())
	out.WriteString("(")
	for i, a := range ce.Arguments {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(a.String())
	}
	out.WriteString(")")
	return out.String()
}

type SpreadExpression struct {
	Token token.Token // '...'
	Value Expression
}

func (*SpreadExpression) expressionNode()         {}
func (se *SpreadExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SpreadExpression) String() string {
	var out bytes.Buffer
	out.WriteString("...")
	if se.Value != nil {
		out.WriteString(se.Value.String())
	}
	return out.String()
}

type TupleLiteral struct {
	Token    token.Token // '('
	Elements []Expression
}

func (*TupleLiteral) expressionNode()         {}
func (tl *TupleLiteral) TokenLiteral() string { return tl.Token.Literal }
func (tl *TupleLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	for i, el := range tl.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.String())
	}
	if len(tl.Elements) == 1 {
		out.WriteString(",")
	}
	out.WriteString(")")
	return out.String()
}

type ListLiteral struct {
	Token    token.Token // '['
	Elements []Expression
}

func (*ListLiteral) expressionNode()         {}
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }
func (ll *ListLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, el := range ll.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.String())
	}
	out.WriteString("]")
	return out.String()
}

type ListComprehension struct {
	Token  token.Token // '['
	Elem   Expression
	Var    *Identifier
	Seq    Expression
	Filter Expression
}

func (*ListComprehension) expressionNode()         {}
func (lc *ListComprehension) TokenLiteral() string { return lc.Token.Literal }
func (lc *ListComprehension) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	if lc.Elem != nil {
		out.WriteString(lc.Elem.String())
	}
	out.WriteString(" for ")
	if lc.Var != nil {
		out.WriteString(lc.Var.String())
	}
	out.WriteString(" in ")
	if lc.Seq != nil {
		out.WriteString(lc.Seq.String())
	}
	if lc.Filter != nil {
		out.WriteString(" if ")
		out.WriteString(lc.Filter.String())
	}
	out.WriteString("]")
	return out.String()
}

type DictLiteral struct {
	Token token.Token // '#'
	Pairs []DictPair
}

type DictPair struct {
	Key       Expression
	Value     Expression
	Shorthand *Identifier
}

func (*DictLiteral) expressionNode()         {}
func (dl *DictLiteral) TokenLiteral() string { return dl.Token.Literal }
func (dl *DictLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("#{")
	for i, p := range dl.Pairs {
		if i > 0 {
			out.WriteString(", ")
		}
		if p.Shorthand != nil {
			out.WriteString(p.Shorthand.String())
			continue
		}
		out.WriteString(p.Key.String())
		out.WriteString(": ")
		out.WriteString(p.Value.String())
	}
	out.WriteString("}")
	return out.String()
}

type IndexExpression struct {
	Token token.Token // '['
	Left  Expression
	Index Expression
}

func (*IndexExpression) expressionNode()         {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")
	return out.String()
}

type SliceExpression struct {
	Token token.Token // '['
	Left  Expression
	Low   Expression
	High  Expression
	Step  Expression
}

func (*SliceExpression) expressionNode()         {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SliceExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(se.Left.String())
	out.WriteString("[")
	if se.Low != nil {
		out.WriteString(se.Low.String())
	}
	out.WriteString(":")
	if se.High != nil {
		out.WriteString(se.High.String())
	}
	if se.Step != nil {
		out.WriteString(":")
		out.WriteString(se.Step.String())
	}
	out.WriteString("])")
	return out.String()
}
