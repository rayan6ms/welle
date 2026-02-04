package compiler

import "welle/internal/code"

func peephole(ins code.Instructions, pos []SourcePos) (code.Instructions, []SourcePos) {
	rewrite := func(at int, op code.Opcode, ins code.Instructions) (code.Instructions, int, bool, error) {
		switch op {
		case code.OpNull:
			next := at + instrSize(ins, at)
			if next < len(ins) && code.Opcode(ins[next]) == code.OpPop {
				return nil, instrSize(ins, at)+instrSize(ins, next), true, nil
			}
		case code.OpPop:
			next := at + instrSize(ins, at)
			if next < len(ins) && code.Opcode(ins[next]) == code.OpReturnValue {
				return nil, instrSize(ins, at), true, nil
			}
		case code.OpJump:
			if at+3 <= len(ins) {
				target := int(code.ReadUint16(ins[at+1:]))
				if target == at+instrSize(ins, at) {
					return nil, instrSize(ins, at), true, nil
				}
			}
		}
		return nil, 0, false, nil
	}

	for {
		var changed bool
		var err error
		ins, pos, changed, err = rebuild(ins, pos, rewrite)
		if err != nil || !changed {
			break
		}
	}
	return ins, pos
}
