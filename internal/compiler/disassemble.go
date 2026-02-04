package compiler

import (
	"fmt"
	"strings"

	"welle/internal/object"
)

func FormatConstants(constants []object.Object) string {
	var b strings.Builder
	b.WriteString("== constants ==\n")
	for i, c := range constants {
		switch v := c.(type) {
		case *object.Integer:
			fmt.Fprintf(&b, "%04d INTEGER %d\n", i, v.Value)
		case *object.Float:
			fmt.Fprintf(&b, "%04d FLOAT %s\n", i, v.Inspect())
		case *object.String:
			fmt.Fprintf(&b, "%04d STRING %q\n", i, v.Value)
		case *object.Boolean:
			fmt.Fprintf(&b, "%04d BOOLEAN %v\n", i, v.Value)
		case *object.CompiledFunction:
			name := v.Name
			if name == "" {
				name = "<anon>"
			}
			fmt.Fprintf(&b, "%04d COMPILED_FUNCTION %s (locals=%d params=%d ins=%dB)\n",
				i, name, v.NumLocals, v.NumParameters, len(v.Instructions))
		default:
			fmt.Fprintf(&b, "%04d %s %s\n", i, c.Type(), c.Inspect())
		}
	}
	return b.String()
}
