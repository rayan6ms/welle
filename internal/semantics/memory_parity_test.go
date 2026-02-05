package semantics_test

import (
	"fmt"
	"strings"
	"testing"

	"welle/internal/compiler"
	"welle/internal/evaluator"
	"welle/internal/lexer"
	"welle/internal/object"
	"welle/internal/parser"
	"welle/internal/vm"
)

func runInterpreterWithMaxMemory(input string, maxMem int64) runResult {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return runResult{errMsg: fmt.Sprintf("parse errors: %s", strings.Join(p.Errors(), "; "))}
	}

	runner := evaluator.NewRunner()
	runner.SetMaxMemory(maxMem)
	res := runner.Eval(program)
	if errObj, ok := res.(*object.Error); ok {
		return runResult{errMsg: errObj.Message}
	}
	return runResult{exports: snapshotExports(runner.Env)}
}

func runVMWithMaxMemory(input string, maxMem int64) runResult {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return runResult{errMsg: fmt.Sprintf("parse errors: %s", strings.Join(p.Errors(), "; "))}
	}

	c := compiler.NewWithFile("parity.wll")
	if err := c.Compile(program); err != nil {
		return runResult{errMsg: err.Error()}
	}
	bc := c.Bytecode()

	m := vm.New(bc)
	m.SetMaxMemory(maxMem)
	if err := m.Run(); err != nil {
		return runResult{exports: m.Exports(), errMsg: vmErrorMessage(err)}
	}
	return runResult{exports: m.Exports()}
}

func TestMemoryLimitParity(t *testing.T) {
	input := `s = "hello"`
	maxMem := int64(10)

	intRes := runInterpreterWithMaxMemory(input, maxMem)
	vmRes := runVMWithMaxMemory(input, maxMem)

	if intRes.errMsg == "" || vmRes.errMsg == "" {
		t.Fatalf("expected memory errors, got interp=%q vm=%q", intRes.errMsg, vmRes.errMsg)
	}
	if intRes.errMsg != vmRes.errMsg {
		t.Fatalf("memory error mismatch: interp=%q vm=%q", intRes.errMsg, vmRes.errMsg)
	}
}
