package parser

import (
	"testing"

	"welle/internal/ast"
	"welle/internal/lexer"
	"welle/internal/token"
)

func TestParseTour_NoErrors(t *testing.T) {
	input := `func add(a, b) {
  return a + b
}

x = add(2, 3)
print(x)

if (x > 3) {
  print("big")
} else {
  print("small")
}

i = 0
while (i < 3) {
  print(i)
  i = i + 1
}`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) == 0 {
		t.Fatal("expected statements, got 0")
	}
}

func TestParseForStatement_NoErrors(t *testing.T) {
	input := `nums = [1, 2, 3]
for (x in nums) {
  print(x)
}`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParseForStatement_NoParens_NoErrors(t *testing.T) {
	input := `nums = [1, 2, 3]
for x in nums {
  print(x)
}`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParsePassStatement_NoErrors(t *testing.T) {
	input := `pass
if (true) { pass }
while (true) { pass }
`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) < 1 {
		t.Fatalf("expected statements, got %d", len(prog.Statements))
	}
	if _, ok := prog.Statements[0].(*ast.PassStatement); !ok {
		t.Fatalf("expected first statement to be PassStatement, got %T", prog.Statements[0])
	}
}

func TestParseCForStatement_NoErrors(t *testing.T) {
	input := `sum = 0
for (i = 0; i < 3; i = i + 1) {
  sum = sum + i
}`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParseForInDestructure_NoErrors(t *testing.T) {
	input := `d = #{"a": 1, "b": 2}
for (k, v) in d {
  print(k)
  print(v)
}`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(prog.Statements))
	}

	forIn, ok := prog.Statements[1].(*ast.ForInStatement)
	if !ok {
		t.Fatalf("expected for-in statement, got %T", prog.Statements[1])
	}
	if !forIn.Destruct {
		t.Fatalf("expected destructuring for-in")
	}
	if forIn.Key == nil || forIn.Key.Value != "k" {
		t.Fatalf("expected key binding 'k', got %v", forIn.Key)
	}
	if forIn.Value == nil || forIn.Value.Value != "v" {
		t.Fatalf("expected value binding 'v', got %v", forIn.Value)
	}
}

func TestParseForInDestructure_Discard_NoErrors(t *testing.T) {
	input := `d = #{"a": 1}
for (k, _) in d { print(k) }`

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParseForInDestructure_InvalidArity(t *testing.T) {
	input := `d = #{"a": 1}
for (a, b, c) in d { print(a) }`

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected parser errors for invalid for-in destructuring arity")
	}
}

func TestParseForInDestructure_GroupedInvalid(t *testing.T) {
	input := `d = #{"a": 1}
for (a) in d { print(a) }`

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected parser errors for invalid for-in binding pattern")
	}
}

func TestParseMemberCall_NoErrors(t *testing.T) {
	input := `a = [1, 2]
a = a.append(3)
print(a.len())`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParseFromImport_NoErrors(t *testing.T) {
	input := `from "math.wll" import add, PI as pi`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}
}

func TestParseTemplateLiteral_NoErrors(t *testing.T) {
	input := "x = t\"hello ${name}!\"\n"
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	stmt, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	tpl, ok := stmt.Value.(*ast.TemplateLiteral)
	if !ok {
		t.Fatalf("expected template literal, got %T", stmt.Value)
	}
	if len(tpl.Parts) != 2 || len(tpl.Exprs) != 1 {
		t.Fatalf("unexpected template shape: parts=%d exprs=%d", len(tpl.Parts), len(tpl.Exprs))
	}
}

func TestParseTaggedTemplate_NoErrors(t *testing.T) {
	input := "x = tag t\"v=${1}\"\n"
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	stmt := prog.Statements[0].(*ast.AssignStatement)
	tpl, ok := stmt.Value.(*ast.TemplateLiteral)
	if !ok {
		t.Fatalf("expected template literal, got %T", stmt.Value)
	}
	if !tpl.Tagged {
		t.Fatalf("expected tagged template")
	}
}

func TestParseNilLiteral(t *testing.T) {
	input := `a = nil
[nil]
#{"x": nil}
if (nil) { a = 1 } else { a = 2 }`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	assignStmt, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.AssignStatement, got %T", prog.Statements[0])
	}
	if _, ok := assignStmt.Value.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[0] - expected nil literal value, got %T", assignStmt.Value)
	}

	listStmt, ok := prog.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.ExpressionStatement, got %T", prog.Statements[1])
	}
	listLit, ok := listStmt.Expression.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.ListLiteral, got %T", listStmt.Expression)
	}
	if len(listLit.Elements) != 1 {
		t.Fatalf("stmt[1] - expected 1 list element, got %d", len(listLit.Elements))
	}
	if _, ok := listLit.Elements[0].(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[1] - expected nil list element, got %T", listLit.Elements[0])
	}

	dictStmt, ok := prog.Statements[2].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.ExpressionStatement, got %T", prog.Statements[2])
	}
	dictLit, ok := dictStmt.Expression.(*ast.DictLiteral)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.DictLiteral, got %T", dictStmt.Expression)
	}
	if len(dictLit.Pairs) != 1 {
		t.Fatalf("stmt[2] - expected 1 dict pair, got %d", len(dictLit.Pairs))
	}
	if _, ok := dictLit.Pairs[0].Value.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[2] - expected nil dict value, got %T", dictLit.Pairs[0].Value)
	}

	ifStmt, ok := prog.Statements[3].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[3] - expected *ast.IfStatement, got %T", prog.Statements[3])
	}
	if _, ok := ifStmt.Condition.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[3] - expected nil condition, got %T", ifStmt.Condition)
	}
	if ifStmt.Alternative == nil {
		t.Fatal("stmt[3] - expected else block")
	}
}

func TestParseNullLiteral(t *testing.T) {
	input := `x = null
[null]
#{"x": null}
if (null) { x = 1 } else { x = 2 }
a = (null == nil)`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 5 {
		t.Fatalf("expected 5 statements, got %d", len(prog.Statements))
	}

	assignStmt, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.AssignStatement, got %T", prog.Statements[0])
	}
	if _, ok := assignStmt.Value.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[0] - expected null to parse as nil literal, got %T", assignStmt.Value)
	}

	listStmt, ok := prog.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.ExpressionStatement, got %T", prog.Statements[1])
	}
	listLit, ok := listStmt.Expression.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.ListLiteral, got %T", listStmt.Expression)
	}
	if len(listLit.Elements) != 1 {
		t.Fatalf("stmt[1] - expected 1 list element, got %d", len(listLit.Elements))
	}
	if _, ok := listLit.Elements[0].(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[1] - expected null list element to parse as nil, got %T", listLit.Elements[0])
	}

	dictStmt, ok := prog.Statements[2].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.ExpressionStatement, got %T", prog.Statements[2])
	}
	dictLit, ok := dictStmt.Expression.(*ast.DictLiteral)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.DictLiteral, got %T", dictStmt.Expression)
	}
	if len(dictLit.Pairs) != 1 {
		t.Fatalf("stmt[2] - expected 1 dict pair, got %d", len(dictLit.Pairs))
	}
	if _, ok := dictLit.Pairs[0].Value.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[2] - expected null dict value to parse as nil, got %T", dictLit.Pairs[0].Value)
	}

	ifStmt, ok := prog.Statements[3].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[3] - expected *ast.IfStatement, got %T", prog.Statements[3])
	}
	if _, ok := ifStmt.Condition.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[3] - expected null condition to parse as nil, got %T", ifStmt.Condition)
	}
	if ifStmt.Alternative == nil {
		t.Fatal("stmt[3] - expected else block")
	}

	exprStmt, ok := prog.Statements[4].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[4] - expected *ast.AssignStatement, got %T", prog.Statements[4])
	}
	infix, ok := exprStmt.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("stmt[4] - expected *ast.InfixExpression, got %T", exprStmt.Value)
	}
	if _, ok := infix.Left.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[4] - expected null to parse as nil on left, got %T", infix.Left)
	}
	if _, ok := infix.Right.(*ast.NilLiteral); !ok {
		t.Fatalf("stmt[4] - expected nil literal on right, got %T", infix.Right)
	}
}

func TestParseListComprehensionBasic(t *testing.T) {
	input := "[x for i in arr]"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	comp, ok := stmt.Expression.(*ast.ListComprehension)
	if !ok {
		t.Fatalf("expected list comprehension, got %T", stmt.Expression)
	}
	if comp.Var == nil || comp.Var.Value != "i" {
		t.Fatalf("expected var i, got %v", comp.Var)
	}
	if _, ok := comp.Seq.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier seq, got %T", comp.Seq)
	}
	if _, ok := comp.Elem.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier elem, got %T", comp.Elem)
	}
}

func TestParseListComprehensionFilter(t *testing.T) {
	input := "[x for i in arr if i]"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt := prog.Statements[0].(*ast.ExpressionStatement)
	comp, ok := stmt.Expression.(*ast.ListComprehension)
	if !ok {
		t.Fatalf("expected list comprehension, got %T", stmt.Expression)
	}
	if comp.Filter == nil {
		t.Fatalf("expected filter expression")
	}
}

func TestParseListComprehensionCondExpr(t *testing.T) {
	input := "[(a if cond else b) for i in arr]"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt := prog.Statements[0].(*ast.ExpressionStatement)
	comp, ok := stmt.Expression.(*ast.ListComprehension)
	if !ok {
		t.Fatalf("expected list comprehension, got %T", stmt.Expression)
	}
	if _, ok := comp.Elem.(*ast.CondExpr); !ok {
		t.Fatalf("expected cond expr elem, got %T", comp.Elem)
	}
}

func TestParseSliceWithStep(t *testing.T) {
	input := "a[1:5:2]"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	slice, ok := stmt.Expression.(*ast.SliceExpression)
	if !ok {
		t.Fatalf("expected slice expression, got %T", stmt.Expression)
	}
	if slice.Step == nil {
		t.Fatalf("expected slice step")
	}
}

func TestParseDestructureAssignmentStar(t *testing.T) {
	input := "(a, *mid, b) = t"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	ds, ok := prog.Statements[0].(*ast.DestructureAssignStatement)
	if !ok {
		t.Fatalf("expected destructure assignment, got %T", prog.Statements[0])
	}
	if len(ds.Targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(ds.Targets))
	}
	if !ds.Targets[1].Star || ds.Targets[1].Name.Value != "mid" {
		t.Fatalf("expected starred mid target, got %+v", ds.Targets[1])
	}
}

func TestParseDestructureAssignmentMultiStarError(t *testing.T) {
	input := "(*a, *b) = t"

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors for multiple starred targets")
	}
}

func TestParseCompoundAssignments(t *testing.T) {
	input := "x += 1\na[0] -= 2\nd.x *= 3\nd |= other\n"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	assignStmt, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.AssignStatement, got %T", prog.Statements[0])
	}
	if assignStmt.Op != token.PLUS_ASSIGN {
		t.Fatalf("stmt[0] - expected Op PLUS_ASSIGN, got %q", assignStmt.Op)
	}

	indexStmt, ok := prog.Statements[1].(*ast.IndexAssignStatement)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.IndexAssignStatement, got %T", prog.Statements[1])
	}
	if indexStmt.Op != token.MINUS_ASSIGN {
		t.Fatalf("stmt[1] - expected Op MINUS_ASSIGN, got %q", indexStmt.Op)
	}

	memberStmt, ok := prog.Statements[2].(*ast.MemberAssignStatement)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.MemberAssignStatement, got %T", prog.Statements[2])
	}
	if memberStmt.Op != token.STAR_ASSIGN {
		t.Fatalf("stmt[2] - expected Op STAR_ASSIGN, got %q", memberStmt.Op)
	}

	assignStmt2, ok := prog.Statements[3].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[3] - expected *ast.AssignStatement, got %T", prog.Statements[3])
	}
	if assignStmt2.Op != token.BITOR_ASSIGN {
		t.Fatalf("stmt[3] - expected Op BITOR_ASSIGN, got %q", assignStmt2.Op)
	}
}

func TestParseFunctionLiteral(t *testing.T) {
	input := `f = func(x, y) { return x + y }
print(f(1, 2))`

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) < 1 {
		t.Fatalf("expected at least 1 statement, got %d", len(prog.Statements))
	}

	assignStmt, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.AssignStatement, got %T", prog.Statements[0])
	}
	lit, ok := assignStmt.Value.(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.FunctionLiteral, got %T", assignStmt.Value)
	}
	if len(lit.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(lit.Parameters))
	}
}

func TestParseTupleLiteral(t *testing.T) {
	input := "(1, 2)\n(1)\n(1,)\n()"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	es0, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	if _, ok := es0.Expression.(*ast.TupleLiteral); !ok {
		t.Fatalf("expected tuple literal for first statement, got %T", es0.Expression)
	}

	es1, ok := prog.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[1])
	}
	if _, ok := es1.Expression.(*ast.TupleLiteral); ok {
		t.Fatalf("expected grouped expression for (1), got tuple literal")
	}

	es2, ok := prog.Statements[2].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[2])
	}
	if tl, ok := es2.Expression.(*ast.TupleLiteral); !ok || len(tl.Elements) != 1 {
		t.Fatalf("expected singleton tuple literal for (1,), got %T", es2.Expression)
	}

	es3, ok := prog.Statements[3].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[3])
	}
	if tl, ok := es3.Expression.(*ast.TupleLiteral); !ok || len(tl.Elements) != 0 {
		t.Fatalf("expected empty tuple literal for (), got %T", es3.Expression)
	}
}

func TestParseReturnList(t *testing.T) {
	input := "return 1, 2\nreturn\n"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}

	rs0, ok := prog.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("expected return statement, got %T", prog.Statements[0])
	}
	if len(rs0.ReturnValues) != 2 {
		t.Fatalf("expected 2 return values, got %d", len(rs0.ReturnValues))
	}

	rs1, ok := prog.Statements[1].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("expected return statement, got %T", prog.Statements[1])
	}
	if len(rs1.ReturnValues) != 0 {
		t.Fatalf("expected no return values, got %d", len(rs1.ReturnValues))
	}
}

func TestParseDestructureAssignment(t *testing.T) {
	input := "(a, b) = (1, 2)"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	ds, ok := prog.Statements[0].(*ast.DestructureAssignStatement)
	if !ok {
		t.Fatalf("expected destructure assignment, got %T", prog.Statements[0])
	}
	if len(ds.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(ds.Targets))
	}
}

func TestParseAssignmentExpressionPrecedence(t *testing.T) {
	input := "x = y + 1 * 2"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	infix, ok := assign.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected infix value, got %T", assign.Value)
	}
	if infix.Operator != "+" {
		t.Fatalf("expected '+' operator, got %q", infix.Operator)
	}
	if _, ok := infix.Left.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier left, got %T", infix.Left)
	}
	right, ok := infix.Right.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected infix right, got %T", infix.Right)
	}
	if right.Operator != "*" {
		t.Fatalf("expected '*' operator, got %q", right.Operator)
	}
}

func TestParseAssignmentExpressionRightAssociative(t *testing.T) {
	input := "a = b = 3"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	inner, ok := assign.Value.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected assignment expression, got %T", assign.Value)
	}
	if _, ok := inner.Left.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier left, got %T", inner.Left)
	}
	if _, ok := inner.Value.(*ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer value, got %T", inner.Value)
	}
}

func TestParseAssignmentExpressionNesting(t *testing.T) {
	input := "x = (y = 2) + 1"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	infix, ok := assign.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected infix value, got %T", assign.Value)
	}
	if _, ok := infix.Left.(*ast.AssignExpression); !ok {
		t.Fatalf("expected assignment expression left, got %T", infix.Left)
	}
}

func TestParseAssignmentExpressionInvalidTarget(t *testing.T) {
	input := "a + b = 3"

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected parser errors for invalid assignment target")
	}
}

func TestParseAssignmentExpressionInvalidDestructure(t *testing.T) {
	input := "print((a, b) = (1, 2))"

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected parser errors for destructuring assignment in expression")
	}
}

func TestParseWalrusExpressionRightAssociative(t *testing.T) {
	input := "a := b := 3"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	if assign.Op != token.WALRUS {
		t.Fatalf("expected walrus op, got %q", assign.Op)
	}
	inner, ok := assign.Value.(*ast.AssignExpression)
	if !ok {
		t.Fatalf("expected nested assignment expression, got %T", assign.Value)
	}
	if inner.Op != token.WALRUS {
		t.Fatalf("expected nested walrus op, got %q", inner.Op)
	}
}

func TestParseWalrusInvalidTarget(t *testing.T) {
	input := "arr[0] := 1"

	l := lexer.New(input)
	p := New(l)
	_ = p.ParseProgram()

	if len(p.Errors()) == 0 {
		t.Fatal("expected parser errors for invalid walrus target")
	}
}

func TestParseDictLiteralShorthand(t *testing.T) {
	input := "person = #{name, age, \"role\": role}"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	dict, ok := assign.Value.(*ast.DictLiteral)
	if !ok {
		t.Fatalf("expected dict literal, got %T", assign.Value)
	}
	if len(dict.Pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(dict.Pairs))
	}
	if dict.Pairs[0].Shorthand == nil || dict.Pairs[0].Shorthand.Value != "name" {
		t.Fatalf("expected first shorthand pair to be name, got %#v", dict.Pairs[0])
	}
	if dict.Pairs[1].Shorthand == nil || dict.Pairs[1].Shorthand.Value != "age" {
		t.Fatalf("expected second shorthand pair to be age, got %#v", dict.Pairs[1])
	}
	if dict.Pairs[2].Shorthand != nil {
		t.Fatalf("expected third pair explicit, got shorthand %#v", dict.Pairs[2].Shorthand)
	}
}

func TestParsePrefixBang(t *testing.T) {
	input := "x = !a"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("expected assign statement, got %T", prog.Statements[0])
	}
	prefix, ok := assign.Value.(*ast.PrefixExpression)
	if !ok {
		t.Fatalf("expected prefix expression, got %T", assign.Value)
	}
	if prefix.Operator != "!" {
		t.Fatalf("expected '!' operator, got %q", prefix.Operator)
	}
	if _, ok := prefix.Right.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier right, got %T", prefix.Right)
	}
}

func TestParseConditionalExpression(t *testing.T) {
	input := "a ? b : c"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	cond, ok := stmt.Expression.(*ast.ConditionalExpression)
	if !ok {
		t.Fatalf("expected conditional expression, got %T", stmt.Expression)
	}
	if _, ok := cond.Cond.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier condition, got %T", cond.Cond)
	}
	if _, ok := cond.Then.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier then, got %T", cond.Then)
	}
	if _, ok := cond.Else.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier else, got %T", cond.Else)
	}
}

func TestParseConditionalExpressionRightAssociative(t *testing.T) {
	input := "a ? b : c ? d : e"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	cond, ok := stmt.Expression.(*ast.ConditionalExpression)
	if !ok {
		t.Fatalf("expected conditional expression, got %T", stmt.Expression)
	}
	if _, ok := cond.Else.(*ast.ConditionalExpression); !ok {
		t.Fatalf("expected right-associative else to be conditional, got %T", cond.Else)
	}
}

func TestParseConditionalExpressionPrecedence(t *testing.T) {
	input := "" +
		"x = a ? 1 : 2\n" +
		"a ? b : c or d\n" +
		"d = #{\"x\": a ? 1 : 2}\n"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	assign0, ok := prog.Statements[0].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected assign statement, got %T", prog.Statements[0])
	}
	if _, ok := assign0.Value.(*ast.ConditionalExpression); !ok {
		t.Fatalf("stmt[0] - expected conditional expression value, got %T", assign0.Value)
	}

	stmt1, ok := prog.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt[1] - expected expression statement, got %T", prog.Statements[1])
	}
	cond1, ok := stmt1.Expression.(*ast.ConditionalExpression)
	if !ok {
		t.Fatalf("stmt[1] - expected conditional expression, got %T", stmt1.Expression)
	}
	if infix, ok := cond1.Else.(*ast.InfixExpression); !ok || infix.Operator != "or" {
		t.Fatalf("stmt[1] - expected else to be 'or' infix, got %T", cond1.Else)
	}

	assign2, ok := prog.Statements[2].(*ast.AssignStatement)
	if !ok {
		t.Fatalf("stmt[2] - expected assign statement, got %T", prog.Statements[2])
	}
	dict, ok := assign2.Value.(*ast.DictLiteral)
	if !ok {
		t.Fatalf("stmt[2] - expected dict literal, got %T", assign2.Value)
	}
	if len(dict.Pairs) != 1 {
		t.Fatalf("stmt[2] - expected 1 dict pair, got %d", len(dict.Pairs))
	}
	if _, ok := dict.Pairs[0].Value.(*ast.ConditionalExpression); !ok {
		t.Fatalf("stmt[2] - expected conditional in dict value, got %T", dict.Pairs[0].Value)
	}
}

func TestParseCondExpr(t *testing.T) {
	input := "a if b else c"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	cond, ok := stmt.Expression.(*ast.CondExpr)
	if !ok {
		t.Fatalf("expected cond expr, got %T", stmt.Expression)
	}
	if _, ok := cond.Then.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier then, got %T", cond.Then)
	}
	if _, ok := cond.Cond.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier cond, got %T", cond.Cond)
	}
	if _, ok := cond.Else.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier else, got %T", cond.Else)
	}
}

func TestParseCondExprRightAssociative(t *testing.T) {
	input := "a if b else c if d else e"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	stmt, ok := prog.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", prog.Statements[0])
	}
	cond, ok := stmt.Expression.(*ast.CondExpr)
	if !ok {
		t.Fatalf("expected cond expr, got %T", stmt.Expression)
	}
	if _, ok := cond.Else.(*ast.CondExpr); !ok {
		t.Fatalf("expected right-associative else to be cond expr, got %T", cond.Else)
	}
}

func TestParseBitwisePrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1 | 2 ^ 3 & 4", "(1 | (2 ^ (3 & 4)))\n"},
		{"1 & 2 | 3", "((1 & 2) | 3)\n"},
		{"1 ^ 2 | 3", "((1 ^ 2) | 3)\n"},
		{"1 + 2 << 3", "((1 + 2) << 3)\n"},
		{"1 << 2 < 3", "((1 << 2) < 3)\n"},
		{"~1 + 2", "((~1) + 2)\n"},
		{"~1 << 2", "((~1) << 2)\n"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		prog := p.ParseProgram()

		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				t.Error(e)
			}
			t.Fatalf("parser had %d errors", len(p.Errors()))
		}

		if got := prog.String(); got != tt.expected {
			t.Fatalf("expected %q, got %q", tt.expected, got)
		}
	}
}

func TestParseIfSingleStatement(t *testing.T) {
	input := "" +
		"if (x) y = 1\n" +
		"if (x) y = 1 else y = 2\n" +
		"if (x) { y = 1 } else y = 2\n" +
		"if (a) if (b) x = 1 else x = 2\n"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	ifStmt0, ok := prog.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[0] - expected *ast.IfStatement, got %T", prog.Statements[0])
	}
	if _, ok := ifStmt0.Consequence.(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[0] - expected assign consequence, got %T", ifStmt0.Consequence)
	}
	if ifStmt0.Alternative != nil {
		t.Fatalf("stmt[0] - expected nil alternative, got %T", ifStmt0.Alternative)
	}

	ifStmt1, ok := prog.Statements[1].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[1] - expected *ast.IfStatement, got %T", prog.Statements[1])
	}
	if _, ok := ifStmt1.Consequence.(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[1] - expected assign consequence, got %T", ifStmt1.Consequence)
	}
	if _, ok := ifStmt1.Alternative.(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[1] - expected assign alternative, got %T", ifStmt1.Alternative)
	}

	ifStmt2, ok := prog.Statements[2].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[2] - expected *ast.IfStatement, got %T", prog.Statements[2])
	}
	if _, ok := ifStmt2.Consequence.(*ast.BlockStatement); !ok {
		t.Fatalf("stmt[2] - expected block consequence, got %T", ifStmt2.Consequence)
	}
	if _, ok := ifStmt2.Alternative.(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[2] - expected assign alternative, got %T", ifStmt2.Alternative)
	}

	ifStmt3, ok := prog.Statements[3].(*ast.IfStatement)
	if !ok {
		t.Fatalf("stmt[3] - expected *ast.IfStatement, got %T", prog.Statements[3])
	}
	if _, ok := ifStmt3.Consequence.(*ast.IfStatement); !ok {
		t.Fatalf("stmt[3] - expected inner if consequence, got %T", ifStmt3.Consequence)
	}
	if ifStmt3.Alternative != nil {
		t.Fatalf("stmt[3] - expected nil alternative, got %T", ifStmt3.Alternative)
	}
}

func TestParseIfSingleStatementBoundaries(t *testing.T) {
	input := "" +
		"if (x) y = 1\n" +
		"z = 2\n" +
		"if (x) y = 1; z = 2\n"

	l := lexer.New(input)
	p := New(l)
	prog := p.ParseProgram()

	if prog == nil {
		t.Fatal("program is nil")
	}
	if len(p.Errors()) > 0 {
		for _, e := range p.Errors() {
			t.Error(e)
		}
		t.Fatalf("parser had %d errors", len(p.Errors()))
	}

	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	if _, ok := prog.Statements[0].(*ast.IfStatement); !ok {
		t.Fatalf("stmt[0] - expected *ast.IfStatement, got %T", prog.Statements[0])
	}
	if _, ok := prog.Statements[1].(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[1] - expected *ast.AssignStatement, got %T", prog.Statements[1])
	}
	if _, ok := prog.Statements[2].(*ast.IfStatement); !ok {
		t.Fatalf("stmt[2] - expected *ast.IfStatement, got %T", prog.Statements[2])
	}
	if _, ok := prog.Statements[3].(*ast.AssignStatement); !ok {
		t.Fatalf("stmt[3] - expected *ast.AssignStatement, got %T", prog.Statements[3])
	}
}

func TestParseIfSingleStatementErrors(t *testing.T) {
	tests := []string{
		"if (x) else y = 1\n",
		"if (x) {\n",
	}

	for _, input := range tests {
		l := lexer.New(input)
		p := New(l)
		_ = p.ParseProgram()

		if len(p.Errors()) == 0 {
			t.Fatalf("expected parser errors for input: %q", input)
		}
	}
}
