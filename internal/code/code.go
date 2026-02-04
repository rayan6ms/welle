package code

import "encoding/binary"

type Opcode byte

const (
	OpConstant Opcode = iota // push constants[operand]
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPop

	OpTrue
	OpFalse

	OpEqual
	OpNotEqual
	OpGreaterThan

	OpMinus
	OpBang

	OpJumpNotTruthy // operand: jump address
	OpJump          // operand: jump address

	OpNull

	OpSetGlobal
	OpGetGlobal

	OpGetBuiltin // operand: builtin index (1 byte)

	OpPrint // pop arg(s) and print? (v0.1: print 1 arg)

	OpCall
	OpDefer
	OpReturnValue
	OpReturn

	OpSetLocal
	OpGetLocal

	OpClosure
	OpGetFree
	OpCurrentClosure

	OpArray     // operand: elementCount (2 bytes)
	OpDict      // operand: pairCount (2 bytes)
	OpIndex     // no operands
	OpGetMember // operand: nameConst (2 bytes)
	OpSetMember // operand: nameConst (2 bytes)
	OpSetIndex
	OpSlice // no operands (expects: left, lowOrNull, highOrNull)

	OpImportModule // operand: constIndex (2 bytes) for path string literal
	OpImportFrom   // operands: modulePathConst(2), nameConst(2)
	OpExport       // operand: nameConst (2 bytes)
	OpTry          // operand: catch address (2 bytes)
	OpEndTry       // no operands
	OpTryFinally   // operands: finally address (2 bytes), afterFinally (2 bytes)
	OpEndFinally   // no operands
	OpRethrowPending
	OpThrow // no operands
)

type Instructions []byte

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:       {"OpConstant", []int{2}},
	OpAdd:            {"OpAdd", nil},
	OpSub:            {"OpSub", nil},
	OpMul:            {"OpMul", nil},
	OpDiv:            {"OpDiv", nil},
	OpMod:            {"OpMod", nil},
	OpPop:            {"OpPop", nil},
	OpTrue:           {"OpTrue", nil},
	OpFalse:          {"OpFalse", nil},
	OpEqual:          {"OpEqual", nil},
	OpNotEqual:       {"OpNotEqual", nil},
	OpGreaterThan:    {"OpGreaterThan", nil},
	OpMinus:          {"OpMinus", nil},
	OpBang:           {"OpBang", nil},
	OpJumpNotTruthy:  {"OpJumpNotTruthy", []int{2}},
	OpJump:           {"OpJump", []int{2}},
	OpNull:           {"OpNull", nil},
	OpSetGlobal:      {"OpSetGlobal", []int{2}},
	OpGetGlobal:      {"OpGetGlobal", []int{2}},
	OpGetBuiltin:     {"OpGetBuiltin", []int{1}},
	OpPrint:          {"OpPrint", nil},
	OpCall:           {"OpCall", []int{1}},
	OpDefer:          {"OpDefer", []int{1}},
	OpReturnValue:    {"OpReturnValue", nil},
	OpReturn:         {"OpReturn", nil},
	OpSetLocal:       {"OpSetLocal", []int{1}},
	OpGetLocal:       {"OpGetLocal", []int{1}},
	OpClosure:        {"OpClosure", []int{2, 1}},
	OpGetFree:        {"OpGetFree", []int{1}},
	OpCurrentClosure: {"OpCurrentClosure", nil},
	OpArray:          {"OpArray", []int{2}},
	OpDict:           {"OpDict", []int{2}},
	OpIndex:          {"OpIndex", nil},
	OpGetMember:      {"OpGetMember", []int{2}},
	OpSetMember:      {"OpSetMember", []int{2}},
	OpSetIndex:       {"OpSetIndex", nil},
	OpSlice:          {"OpSlice", nil},
	OpImportModule:   {"OpImportModule", []int{2}},
	OpImportFrom:     {"OpImportFrom", []int{2, 2}},
	OpExport:         {"OpExport", []int{2}},
	OpTry:            {"OpTry", []int{2}},
	OpEndTry:         {"OpEndTry", nil},
	OpTryFinally:     {"OpTryFinally", []int{2, 2}},
	OpEndFinally:     {"OpEndFinally", nil},
	OpRethrowPending: {"OpRethrowPending", nil},
	OpThrow:          {"OpThrow", nil},
}

func Lookup(op Opcode) (*Definition, bool) {
	def, ok := definitions[op]
	return def, ok
}

func Make(op Opcode, operands ...int) Instructions {
	def := definitions[op]
	insLen := 1
	for _, w := range def.OperandWidths {
		insLen += w
	}

	ins := make([]byte, insLen)
	ins[0] = byte(op)

	offset := 1
	for i, o := range operands {
		w := def.OperandWidths[i]
		switch w {
		case 1:
			ins[offset] = byte(o)
		case 2:
			binary.BigEndian.PutUint16(ins[offset:], uint16(o))
		}
		offset += w
	}
	return ins
}

func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}
