package evaluator

import (
	"welle/internal/ast"
	"welle/internal/object"
)

type callFrame struct {
	defers []ast.Expression
}

var callStack []callFrame

func pushFrame() {
	callStack = append(callStack, callFrame{})
}

func popFrame() callFrame {
	top := callStack[len(callStack)-1]
	callStack = callStack[:len(callStack)-1]
	return top
}

func currentFrame() *callFrame {
	if len(callStack) == 0 {
		return nil
	}
	return &callStack[len(callStack)-1]
}

func runDefers(frame callFrame, env *object.Environment) object.Object {
	for i := len(frame.defers) - 1; i >= 0; i-- {
		res := Eval(frame.defers[i], env)
		if isError(res) {
			return res
		}
	}
	return nil
}
