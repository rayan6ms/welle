package lint

import (
	"welle/internal/ast"
	"welle/internal/diag"
)

type Options struct {
	CheckShadowing bool
}

func DefaultOptions() Options {
	return Options{CheckShadowing: true}
}

type Linter struct {
	opts Options
}

func New() *Linter {
	return &Linter{opts: DefaultOptions()}
}

func NewWithOptions(opts Options) *Linter {
	return &Linter{opts: opts}
}

func Run(program *ast.Program) []diag.Diagnostic {
	return New().Run(program)
}

func RunWithOptions(program *ast.Program, opts Options) []diag.Diagnostic {
	return NewWithOptions(opts).Run(program)
}

func (l *Linter) Run(program *ast.Program) []diag.Diagnostic {
	if program == nil {
		return nil
	}
	r := &Runner{sc: newScope(nil), opts: l.opts}
	r.walkProgram(program)
	return r.diags
}
