package object

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"welle/internal/ast"
	"welle/internal/code"
)

type Type string

const (
	INTEGER_OBJ           Type = "INTEGER"
	FLOAT_OBJ             Type = "FLOAT"
	STRING_OBJ            Type = "STRING"
	BOOLEAN_OBJ           Type = "BOOLEAN"
	NIL_OBJ               Type = "NIL"
	RETURN_VALUE_OBJ      Type = "RETURN_VALUE"
	BREAK_OBJ             Type = "BREAK"
	CONTINUE_OBJ          Type = "CONTINUE"
	FUNCTION_OBJ          Type = "FUNCTION"
	COMPILED_FUNCTION_OBJ Type = "COMPILED_FUNCTION"
	CLOSURE_OBJ           Type = "CLOSURE"
	ARRAY_OBJ             Type = "ARRAY"
	DICT_OBJ              Type = "DICT"
	BUILTIN_OBJ           Type = "BUILTIN"
	ERROR_OBJ             Type = "ERROR"
	IMAGE_OBJ             Type = "IMAGE"
)

type Object interface {
	Type() Type
	Inspect() string
}

type Integer struct{ Value int64 }

func (*Integer) Type() Type        { return INTEGER_OBJ }
func (i *Integer) Inspect() string { return itoa(i.Value) }

type Float struct{ Value float64 }

func (*Float) Type() Type { return FLOAT_OBJ }
func (f *Float) Inspect() string {
	return strconv.FormatFloat(f.Value, 'g', -1, 64)
}

type String struct{ Value string }

func (*String) Type() Type        { return STRING_OBJ }
func (s *String) Inspect() string { return s.Value }

type Boolean struct{ Value bool }

func (*Boolean) Type() Type { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string {
	if b.Value {
		return "true"
	}
	return "false"
}

type Nil struct{}

func (*Nil) Type() Type      { return NIL_OBJ }
func (*Nil) Inspect() string { return "nil" }

type ReturnValue struct{ Value Object }

func (*ReturnValue) Type() Type         { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

type Break struct{}

func (*Break) Type() Type      { return BREAK_OBJ }
func (*Break) Inspect() string { return "break" }

type Continue struct{}

func (*Continue) Type() Type      { return CONTINUE_OBJ }
func (*Continue) Inspect() string { return "continue" }

type Function struct {
	Name       string
	File       string
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (*Function) Type() Type { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}
	out.WriteString("func(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(f.Body.String())
	return out.String()
}

type CompiledFunction struct {
	Instructions  code.Instructions
	NumLocals     int
	NumParameters int
	Name          string
	File          string
	Pos           []code.SourcePos
}

func (*CompiledFunction) Type() Type { return COMPILED_FUNCTION_OBJ }
func (*CompiledFunction) Inspect() string {
	return "compiled-func"
}

type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (*Closure) Type() Type { return CLOSURE_OBJ }
func (*Closure) Inspect() string {
	return "closure"
}

type BuiltinFunction func(args ...Object) Object

type Builtin struct{ Fn BuiltinFunction }

func (*Builtin) Type() Type      { return BUILTIN_OBJ }
func (*Builtin) Inspect() string { return "<builtin>" }

type Array struct {
	Elements []Object
}

func (*Array) Type() Type { return ARRAY_OBJ }
func (a *Array) Inspect() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, el := range a.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(el.Inspect())
	}
	out.WriteString("]")
	return out.String()
}

type DictPair struct {
	Key   Object
	Value Object
}

type Dict struct {
	Pairs map[string]DictPair
}

func (*Dict) Type() Type { return DICT_OBJ }
func (d *Dict) Inspect() string {
	var out bytes.Buffer
	out.WriteString("#{")
	keys := make([]string, 0, len(d.Pairs))
	for k := range d.Pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			out.WriteString(", ")
		}
		pair := d.Pairs[k]
		if ks, ok := pair.Key.(*String); ok {
			out.WriteString(`"` + ks.Value + `": `)
		} else {
			out.WriteString(pair.Key.Inspect())
			out.WriteString(": ")
		}
		out.WriteString(pair.Value.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

// tiny helper to avoid fmt import
func itoa(n int64) string {
	// simple base10
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [32]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

type Error struct {
	Message string
	Code    int64
	Stack   string
	IsValue bool
}

func (*Error) Type() Type { return ERROR_OBJ }

func (e *Error) Inspect() string {
	if e.Code != 0 {
		return fmt.Sprintf("error(%d): %s", e.Code, e.Message)
	}
	return "error: " + e.Message
}
