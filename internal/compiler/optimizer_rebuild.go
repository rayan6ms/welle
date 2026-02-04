package compiler

import "welle/internal/code"

type rewriteFunc func(at int, op code.Opcode, ins code.Instructions) (code.Instructions, int, bool, error)

func rebuild(ins code.Instructions, pos []SourcePos, rewrite rewriteFunc) (code.Instructions, []SourcePos, bool, error) {
	oldToNew := make(map[int]int, len(ins))
	newIns := make([]byte, 0, len(ins))
	changed := false

	i := 0
	for i < len(ins) {
		op := code.Opcode(ins[i])
		if rewrite != nil {
			repl, oldSize, ok, err := rewrite(i, op, ins)
			if err != nil {
				return nil, nil, false, err
			}
			if ok {
				changed = true
				mapOffsets(oldToNew, ins, i, oldSize, len(newIns))
				if len(repl) > 0 {
					newIns = append(newIns, repl...)
				}
				i += oldSize
				continue
			}
		}

		size := instrSize(ins, i)
		oldToNew[i] = len(newIns)
		newIns = append(newIns, ins[i:i+size]...)
		i += size
	}

	remapJumps(newIns, oldToNew)
	newPos := remapPositions(pos, oldToNew)
	return newIns, newPos, changed, nil
}

func instrSize(ins code.Instructions, at int) int {
	op := code.Opcode(ins[at])
	def, ok := code.Lookup(op)
	if !ok {
		return 1
	}
	_, read := code.ReadOperands(def, ins[at+1:])
	return 1 + read
}

func mapOffsets(oldToNew map[int]int, ins code.Instructions, start, size, newOffset int) {
	end := start + size
	i := start
	for i < end && i < len(ins) {
		oldToNew[i] = newOffset
		i += instrSize(ins, i)
	}
}

func remapJumps(ins code.Instructions, oldToNew map[int]int) {
	i := 0
	for i < len(ins) {
		op := code.Opcode(ins[i])
		def, ok := code.Lookup(op)
		if !ok {
			i++
			continue
		}

		operands, read := code.ReadOperands(def, ins[i+1:])
		size := 1 + read

		switch op {
		case code.OpJump, code.OpJumpNotTruthy:
			oldTarget := operands[0]
			if newTarget, ok := oldToNew[oldTarget]; ok {
				fixed := code.Make(op, newTarget)
				copy(ins[i:i+len(fixed)], fixed)
			}
		}

		i += size
	}
}

func remapPositions(pos []SourcePos, oldToNew map[int]int) []SourcePos {
	if len(pos) == 0 {
		return pos
	}
	out := make([]SourcePos, 0, len(pos))
	for _, p := range pos {
		newOffset, ok := oldToNew[p.Offset]
		if !ok {
			continue
		}
		out = append(out, SourcePos{
			Offset: newOffset,
			Line:   p.Line,
			Col:    p.Col,
		})
	}
	return out
}
