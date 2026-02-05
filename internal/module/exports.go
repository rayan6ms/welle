package module

import (
	"fmt"

	"welle/internal/ast"
	"welle/internal/token"
)

func CheckDuplicateExports(program *ast.Program, file string) error {
	if program == nil {
		return nil
	}
	seen := map[string]token.Token{}
	for _, stmt := range program.Statements {
		exp, ok := stmt.(*ast.ExportStatement)
		if !ok {
			continue
		}
		name, tok, ok := exportName(exp)
		if !ok {
			continue
		}
		if prev, exists := seen[name]; exists {
			return fmt.Errorf(
				"duplicate export %q at %s:%d:%d (previous at %s:%d:%d)",
				name,
				file, tok.Line, tok.Col,
				file, prev.Line, prev.Col,
			)
		}
		seen[name] = tok
	}
	return nil
}

func exportName(exp *ast.ExportStatement) (string, token.Token, bool) {
	switch s := exp.Stmt.(type) {
	case *ast.AssignStatement:
		if s.Name != nil {
			return s.Name.Value, s.Name.Token, true
		}
	case *ast.FuncStatement:
		if s.Name != nil {
			return s.Name.Value, s.Name.Token, true
		}
	}
	return "", token.Token{}, false
}
