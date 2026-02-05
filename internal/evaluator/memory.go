package evaluator

import (
	"welle/internal/limits"
	"welle/internal/object"
	"welle/internal/token"
)

func memoryError(limit int64) *object.Error {
	return &object.Error{
		Message: limits.MaxMemoryMessage(limit),
		Code:    limits.MemoryErrorCode,
	}
}

func memoryErrorAt(tok token.Token, limit int64) object.Object {
	errObj := memoryError(limit)
	frames := make([]stackFrame, 0, len(ctx.Stack)+1)
	frames = append(frames, ctx.Stack...)
	frames = append(frames, stackFrame{
		Func: "<main>",
		File: ctx.File,
		Line: tok.Line,
		Col:  tok.Col,
	})
	errObj.Stack = formatStackTrace(errObj.Message, frames)
	return errObj
}

func chargeMemoryAt(tok token.Token, n int64) object.Object {
	if ctx.Budget == nil {
		return nil
	}
	if err := ctx.Budget.Charge(n); err != nil {
		if memErr, ok := err.(limits.MaxMemoryError); ok {
			return memoryErrorAt(tok, memErr.Limit)
		}
		return newErrorAt(tok, err.Error())
	}
	return nil
}

func chargeMemory(n int64) object.Object {
	if ctx.Budget == nil {
		return nil
	}
	if err := ctx.Budget.Charge(n); err != nil {
		if memErr, ok := err.(limits.MaxMemoryError); ok {
			return memoryError(memErr.Limit)
		}
		return newError(err.Error())
	}
	return nil
}
