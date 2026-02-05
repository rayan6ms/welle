package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/parser"
	"welle/internal/vm"
)

const (
	prompt1 = "welle> "
	prompt2 = "....> "
)

type Limits struct {
	MaxRecursion int
	MaxSteps     int64
	MaxMemory    int64
}

func Start(in io.Reader, out io.Writer, stdRoot string, limits Limits) {
	scanner := bufio.NewScanner(in)
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	stdPath := stdRoot
	if stdPath == "" {
		stdPath = filepath.Join(cwd, "std")
	}
	resolver := module.NewResolver(stdPath, []string{cwd})
	loader := module.NewLoader(resolver)
	symbols := compiler.NewSymbolTable()
	globals := make([]object.Object, vm.GlobalsSize)
	moduleCache := map[string]*object.Dict{}
	entryPath := "<repl>"

	fmt.Fprint(out, "Welle REPL (Ctrl+D to exit)\n")

	var buf strings.Builder
	depthBraces := 0
	depthParens := 0
	inString := false
	escaped := false
	inBlockComment := false

	for {
		// choose prompt
		if buf.Len() == 0 {
			fmt.Fprint(out, prompt1)
		} else {
			fmt.Fprint(out, prompt2)
		}

		if !scanner.Scan() {
			fmt.Fprint(out, "\n")
			return
		}

		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// allow quick exit
		if buf.Len() == 0 && (trim == "exit" || trim == "quit") {
			return
		}

		// accumulate
		buf.WriteString(line)
		buf.WriteString("\n")

		// update balance state based on the new line
		depthBraces, depthParens, inString, escaped, inBlockComment = updateBalance(line, depthBraces, depthParens, inString, escaped, inBlockComment)

		// if not complete, continue reading
		if depthBraces > 0 || depthParens > 0 || inString {
			continue
		}

		// parse + eval the accumulated buffer
		src := buf.String()
		buf.Reset()

		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		c := compiler.NewWithFileAndSymbols(entryPath, symbols)
		if err := c.Compile(program); err != nil {
			fmt.Fprintf(out, "compile error: %s\n", err)
			continue
		}
		bc := c.Bytecode()
		m := loader.NewVM(bc, entryPath)
		m.SetMaxRecursion(limits.MaxRecursion)
		m.SetMaxSteps(limits.MaxSteps)
		m.SetMaxMemory(limits.MaxMemory)
		m.SetGlobals(globals)
		m.SetModuleCache(moduleCache)
		if err := m.Run(); err != nil {
			fmt.Fprintln(out, err)
			continue
		}
		result := m.LastPoppedStackElem()
		if result != nil && result.Type() != object.NIL_OBJ {
			fmt.Fprintln(out, result.Inspect())
		}
	}
}

func updateBalance(line string, braces, parens int, inString, escaped, inBlockComment bool) (int, int, bool, bool, bool) {
	for i := 0; i < len(line); i++ {
		ch := line[i]

		if inBlockComment {
			if ch == '*' && i+1 < len(line) && line[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		// not in string: handle comment start // or /*
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		if ch == '/' && i+1 < len(line) && line[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '(':
			parens++
		case ')':
			if parens > 0 {
				parens--
			}
		}
	}
	return braces, parens, inString, escaped, inBlockComment
}

func printParserErrors(out io.Writer, errs []string) {
	for _, e := range errs {
		fmt.Fprintf(out, "parse error: %s\n", e)
	}
}
