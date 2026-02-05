package compiler

import (
	"welle/internal/code"
	"welle/internal/object"
)

func foldConstants(ins code.Instructions, pos []SourcePos, constants *[]object.Object) (code.Instructions, []SourcePos, error) {
	rewrite := func(at int, op code.Opcode, ins code.Instructions) (code.Instructions, int, bool, error) {
		if left, leftSize, ok := readConstAt(ins, at, *constants); ok {
			if right, rightSize, ok := readConstAt(ins, at+leftSize, *constants); ok {
				opOffset := at + leftSize + rightSize
				if opOffset < len(ins) {
					binOp := code.Opcode(ins[opOffset])
					if isFoldableBinOp(binOp) {
						res, ok, err := foldBin(binOp, left, right)
						if err != nil {
							return nil, 0, false, err
						}
						if ok {
							size := leftSize + rightSize + instrSize(ins, opOffset)
							return constToInstruction(res, constants), size, true, nil
						}
					}
				}
			}
		}

		if val, valSize, ok := readConstAt(ins, at, *constants); ok {
			next := at + valSize
			if next < len(ins) {
				unOp := code.Opcode(ins[next])
				if unOp == code.OpMinus || unOp == code.OpBang || unOp == code.OpBitNot {
					res, ok := foldUnary(unOp, val)
					if ok {
						size := valSize + instrSize(ins, next)
						return constToInstruction(res, constants), size, true, nil
					}
				}
			}
		}

		return nil, 0, false, nil
	}

	changed := true
	for changed {
		var err error
		ins, pos, changed, err = rebuild(ins, pos, rewrite)
		if err != nil {
			return nil, nil, err
		}
	}

	return ins, pos, nil
}

func readConstAt(ins code.Instructions, at int, constants []object.Object) (object.Object, int, bool) {
	if at >= len(ins) {
		return nil, 0, false
	}
	switch code.Opcode(ins[at]) {
	case code.OpConstant:
		if at+3 > len(ins) {
			return nil, 0, false
		}
		idx := int(code.ReadUint16(ins[at+1:]))
		if idx < 0 || idx >= len(constants) {
			return nil, 0, false
		}
		return constants[idx], 3, true
	case code.OpTrue:
		return &object.Boolean{Value: true}, 1, true
	case code.OpFalse:
		return &object.Boolean{Value: false}, 1, true
	case code.OpNull:
		return &object.Nil{}, 1, true
	default:
		return nil, 0, false
	}
}

func constToInstruction(obj object.Object, constants *[]object.Object) code.Instructions {
	switch v := obj.(type) {
	case *object.Boolean:
		if v.Value {
			return code.Make(code.OpTrue)
		}
		return code.Make(code.OpFalse)
	default:
		idx := len(*constants)
		*constants = append(*constants, obj)
		return code.Make(code.OpConstant, idx)
	}
}

func isFoldableBinOp(op code.Opcode) bool {
	switch op {
	case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod,
		code.OpBitOr, code.OpBitAnd, code.OpBitXor, code.OpShl, code.OpShr,
		code.OpEqual, code.OpNotEqual, code.OpIs, code.OpGreaterThan, code.OpLessThan, code.OpLessEqual, code.OpGreaterEqual:
		return true
	default:
		return false
	}
}

func foldBin(op code.Opcode, a, b object.Object) (object.Object, bool, error) {
	ai, aok := a.(*object.Integer)
	bi, bok := b.(*object.Integer)
	if aok && bok {
		switch op {
		case code.OpAdd:
			return &object.Integer{Value: ai.Value + bi.Value}, true, nil
		case code.OpSub:
			return &object.Integer{Value: ai.Value - bi.Value}, true, nil
		case code.OpMul:
			return &object.Integer{Value: ai.Value * bi.Value}, true, nil
		case code.OpDiv:
			if bi.Value == 0 {
				return nil, false, nil
			}
			return &object.Integer{Value: ai.Value / bi.Value}, true, nil
		case code.OpMod:
			if bi.Value == 0 {
				return nil, false, nil
			}
			return &object.Integer{Value: ai.Value % bi.Value}, true, nil
		case code.OpBitOr:
			return &object.Integer{Value: ai.Value | bi.Value}, true, nil
		case code.OpBitAnd:
			return &object.Integer{Value: ai.Value & bi.Value}, true, nil
		case code.OpBitXor:
			return &object.Integer{Value: ai.Value ^ bi.Value}, true, nil
		case code.OpShl:
			if bi.Value < 0 || bi.Value >= 64 {
				return nil, false, nil
			}
			return &object.Integer{Value: int64(uint64(ai.Value) << uint64(bi.Value))}, true, nil
		case code.OpShr:
			if bi.Value < 0 || bi.Value >= 64 {
				return nil, false, nil
			}
			return &object.Integer{Value: ai.Value >> uint64(bi.Value)}, true, nil
		case code.OpEqual:
			return &object.Boolean{Value: ai.Value == bi.Value}, true, nil
		case code.OpNotEqual:
			return &object.Boolean{Value: ai.Value != bi.Value}, true, nil
		case code.OpIs:
			return &object.Boolean{Value: ai.Value == bi.Value}, true, nil
		case code.OpGreaterThan:
			return &object.Boolean{Value: ai.Value > bi.Value}, true, nil
		case code.OpLessThan:
			return &object.Boolean{Value: ai.Value < bi.Value}, true, nil
		case code.OpLessEqual:
			return &object.Boolean{Value: ai.Value <= bi.Value}, true, nil
		case code.OpGreaterEqual:
			return &object.Boolean{Value: ai.Value >= bi.Value}, true, nil
		}
	}

	ab, aok := a.(*object.Boolean)
	bb, bok := b.(*object.Boolean)
	if aok && bok {
		switch op {
		case code.OpEqual:
			return &object.Boolean{Value: ab.Value == bb.Value}, true, nil
		case code.OpNotEqual:
			return &object.Boolean{Value: ab.Value != bb.Value}, true, nil
		case code.OpIs:
			return &object.Boolean{Value: ab.Value == bb.Value}, true, nil
		}
	}

	as, aok := a.(*object.String)
	bs, bok := b.(*object.String)
	if aok && bok {
		switch op {
		case code.OpEqual:
			return &object.Boolean{Value: as.Value == bs.Value}, true, nil
		case code.OpNotEqual:
			return &object.Boolean{Value: as.Value != bs.Value}, true, nil
		case code.OpIs:
			return &object.Boolean{Value: as.Value == bs.Value}, true, nil
		}
	}

	_, an := a.(*object.Nil)
	_, bn := b.(*object.Nil)
	if an && bn {
		switch op {
		case code.OpEqual, code.OpIs:
			return &object.Boolean{Value: true}, true, nil
		case code.OpNotEqual:
			return &object.Boolean{Value: false}, true, nil
		}
	}

	return nil, false, nil
}

func foldUnary(op code.Opcode, a object.Object) (object.Object, bool) {
	switch op {
	case code.OpMinus:
		if ai, ok := a.(*object.Integer); ok {
			return &object.Integer{Value: -ai.Value}, true
		}
	case code.OpBitNot:
		if ai, ok := a.(*object.Integer); ok {
			return &object.Integer{Value: ^ai.Value}, true
		}
	case code.OpBang:
		if ab, ok := a.(*object.Boolean); ok {
			return &object.Boolean{Value: !ab.Value}, true
		}
		if _, ok := a.(*object.Nil); ok {
			return &object.Boolean{Value: true}, true
		}
	}
	return nil, false
}
