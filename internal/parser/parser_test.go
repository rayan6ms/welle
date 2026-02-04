package parser

import (
	"testing"

	"welle/internal/ast"
	"welle/internal/lexer"
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
