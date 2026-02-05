package vm

import (
	"welle/internal/limits"
	"welle/internal/object"
)

func (m *VM) memoryError(limit int64) *object.Error {
	msg := limits.MaxMemoryMessage(limit)
	return &object.Error{
		Message: msg,
		Code:    limits.MemoryErrorCode,
		Stack:   m.formatStackTrace(msg),
	}
}

func (m *VM) chargeMemory(n int64) *object.Error {
	if m.budget == nil {
		return nil
	}
	if err := m.budget.Charge(n); err != nil {
		if memErr, ok := err.(limits.MaxMemoryError); ok {
			return m.memoryError(memErr.Limit)
		}
		return &object.Error{Message: err.Error()}
	}
	return nil
}

func (m *VM) costOfObject(obj object.Object) int64 {
	switch v := obj.(type) {
	case *object.String:
		return object.CostStringBytes(len(v.Value))
	case *object.Array:
		return object.CostArray(len(v.Elements))
	case *object.Tuple:
		return object.CostTuple(len(v.Elements))
	case *object.Dict:
		return object.CostDict(len(v.Pairs))
	case *object.Image:
		return object.CostImage(v.Width, v.Height)
	case *object.Error:
		return object.CostError()
	case *object.Closure:
		return object.CostClosure(len(v.Free))
	case *object.Cell:
		return object.CostCell()
	default:
		return 0
	}
}

func (m *VM) chargeObject(obj object.Object) *object.Error {
	if obj == nil {
		return nil
	}
	cost := m.costOfObject(obj)
	if cost == 0 {
		return nil
	}
	return m.chargeMemory(cost)
}
