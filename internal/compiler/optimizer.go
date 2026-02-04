package compiler

import (
	"welle/internal/code"
	"welle/internal/object"
)

type Optimizer struct{}

func (o *Optimizer) Optimize(bc *Bytecode) (*Bytecode, error) {
	if err := optimizeInstructions(&bc.Instructions, &bc.Debug.Pos, &bc.Constants); err != nil {
		return nil, err
	}
	for i := 0; i < len(bc.Constants); i++ {
		if fn, ok := bc.Constants[i].(*object.CompiledFunction); ok {
			if err := optimizeInstructions(&fn.Instructions, &fn.Pos, &bc.Constants); err != nil {
				return nil, err
			}
		}
	}
	return bc, nil
}

func optimizeInstructions(ins *code.Instructions, pos *[]SourcePos, constants *[]object.Object) error {
	var err error
	*ins, *pos, err = foldConstants(*ins, *pos, constants)
	if err != nil {
		return err
	}
	*ins, *pos = peephole(*ins, *pos)
	return nil
}
