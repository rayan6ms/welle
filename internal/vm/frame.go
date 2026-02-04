package vm

import "welle/internal/object"

type deferredCall struct {
	fn   object.Object
	args []object.Object
}

type Frame struct {
	cl          *object.Closure
	ip          int
	basePointer int
	defers      []deferredCall
}

func NewFrame(cl *object.Closure, basePointer int) *Frame {
	return &Frame{cl: cl, ip: -1, basePointer: basePointer}
}

func (f *Frame) Instructions() []byte { return f.cl.Fn.Instructions }
