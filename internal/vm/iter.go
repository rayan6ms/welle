package vm

import "welle/internal/object"

type vmIterator struct {
	items []object.Object
	idx   int
}

func (*vmIterator) Type() object.Type { return object.Type("ITER") }
func (*vmIterator) Inspect() string   { return "<iter>" }

func (it *vmIterator) next() (object.Object, bool) {
	if it.idx >= len(it.items) {
		return nilObj, false
	}
	val := it.items[it.idx]
	it.idx++
	return val, true
}
