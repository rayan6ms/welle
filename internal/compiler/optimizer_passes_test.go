package compiler

import (
	"testing"

	"welle/internal/code"
	"welle/internal/object"
)

func TestFoldConstantsBinaryInt(t *testing.T) {
	constants := []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}
	ins := append(code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1)...)
	ins = append(ins, code.Make(code.OpAdd)...)

	out, _, err := foldConstants(ins, nil, &constants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(constants) < 3 {
		t.Fatalf("expected folded constant appended, got %d", len(constants))
	}
	last, ok := constants[len(constants)-1].(*object.Integer)
	if !ok || last.Value != 3 {
		t.Fatalf("expected folded constant 3, got %T (%v)", constants[len(constants)-1], constants[len(constants)-1])
	}
	if len(out) != len(code.Make(code.OpConstant, len(constants)-1)) {
		t.Fatalf("expected single constant instruction, got %d bytes", len(out))
	}
	if code.Opcode(out[0]) != code.OpConstant {
		t.Fatalf("expected OpConstant, got %v", out[0])
	}
}

func TestFoldConstantsUnary(t *testing.T) {
	constants := []object.Object{}
	ins := append(code.Make(code.OpTrue), code.Make(code.OpBang)...)

	out, _, err := foldConstants(ins, nil, &constants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected single instruction, got %d bytes", len(out))
	}
	if code.Opcode(out[0]) != code.OpFalse {
		t.Fatalf("expected OpFalse, got %v", out[0])
	}
}

func TestFoldConstantsDivisionByZeroNotFolded(t *testing.T) {
	constants := []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 0}}
	ins := append(code.Make(code.OpConstant, 0), code.Make(code.OpConstant, 1)...)
	ins = append(ins, code.Make(code.OpDiv)...)

	out, _, err := foldConstants(ins, nil, &constants)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(ins) {
		t.Fatalf("expected no folding, got %d bytes", len(out))
	}
}

func TestPeepholeRemovesNullPop(t *testing.T) {
	ins := append(code.Make(code.OpNull), code.Make(code.OpPop)...)
	out, _ := peephole(ins, nil)
	if len(out) != 0 {
		t.Fatalf("expected instructions to be removed, got %d bytes", len(out))
	}
}

func TestRebuildRemapsJumpTargets(t *testing.T) {
	ins := append(code.Make(code.OpJump, 4), code.Make(code.OpNull)...)
	ins = append(ins, code.Make(code.OpPop)...)

	rewrite := func(at int, op code.Opcode, ins code.Instructions) (code.Instructions, int, bool, error) {
		if op == code.OpNull {
			return nil, instrSize(ins, at), true, nil
		}
		return nil, 0, false, nil
	}

	out, _, changed, err := rebuild(ins, nil, rewrite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected rebuild to report changes")
	}
	if len(out) < 3 {
		t.Fatalf("expected jump instruction, got %d bytes", len(out))
	}
	if code.Opcode(out[0]) != code.OpJump {
		t.Fatalf("expected OpJump, got %v", out[0])
	}
	operand := int(code.ReadUint16(out[1:]))
	if operand != 3 {
		t.Fatalf("expected remapped jump target 3, got %d", operand)
	}
}
