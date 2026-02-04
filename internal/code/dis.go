package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, w := range def.OperandWidths {
		switch w {
		case 1:
			operands[i] = int(ins[offset])
		case 2:
			operands[i] = int(binary.BigEndian.Uint16(ins[offset:]))
		default:
			panic("unsupported operand width")
		}
		offset += w
	}
	return operands, offset
}

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		op := Opcode(ins[i])
		def, ok := Lookup(op)
		if !ok {
			fmt.Fprintf(&out, "%04d UNKNOWN_OPCODE %d\n", i, op)
			i++
			continue
		}

		operands, read := ReadOperands(def, Instructions(ins[i+1:]))

		fmt.Fprintf(&out, "%04d %s", i, def.Name)
		for _, o := range operands {
			fmt.Fprintf(&out, " %d", o)
		}
		fmt.Fprintf(&out, "\n")

		i += 1 + read
	}

	return out.String()
}
