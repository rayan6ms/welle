package semantics

import (
	"testing"

	"welle/internal/object"
)

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		obj  object.Object
		want bool
	}{
		{&object.Boolean{Value: true}, true},
		{&object.Boolean{Value: false}, false},
		{&object.Nil{}, false},
		{&object.Integer{Value: 0}, true},
		{&object.Integer{Value: 1}, true},
		{&object.Float{Value: 0}, true},
		{&object.String{Value: ""}, true},
	}
	for i, tt := range tests {
		if got := IsTruthy(tt.obj); got != tt.want {
			t.Fatalf("tests[%d] expected %v, got %v", i, tt.want, got)
		}
	}
}

func TestBinaryOpNumbers(t *testing.T) {
	tests := []struct {
		op    string
		left  object.Object
		right object.Object
		want  object.Object
	}{
		{"+", &object.Integer{Value: 1}, &object.Integer{Value: 2}, &object.Integer{Value: 3}},
		{"-", &object.Integer{Value: 5}, &object.Integer{Value: 3}, &object.Integer{Value: 2}},
		{"*", &object.Integer{Value: 2}, &object.Integer{Value: 4}, &object.Integer{Value: 8}},
		{"/", &object.Integer{Value: 5}, &object.Integer{Value: 2}, &object.Integer{Value: 2}},
		{"%", &object.Integer{Value: 5}, &object.Integer{Value: 2}, &object.Integer{Value: 1}},
		{"+", &object.Integer{Value: 1}, &object.Float{Value: 2.5}, &object.Float{Value: 3.5}},
		{"/", &object.Float{Value: 5.0}, &object.Integer{Value: 2}, &object.Float{Value: 2.5}},
	}
	for i, tt := range tests {
		got, err := BinaryOp(tt.op, tt.left, tt.right)
		if err != nil {
			t.Fatalf("tests[%d] unexpected error: %v", i, err)
		}
		switch want := tt.want.(type) {
		case *object.Integer:
			intObj, ok := got.(*object.Integer)
			if !ok || intObj.Value != want.Value {
				t.Fatalf("tests[%d] expected %T(%v), got %T(%v)", i, want, want.Value, got, got)
			}
		case *object.Float:
			floatObj, ok := got.(*object.Float)
			if !ok || floatObj.Value != want.Value {
				t.Fatalf("tests[%d] expected %T(%v), got %T(%v)", i, want, want.Value, got, got)
			}
		default:
			t.Fatalf("tests[%d] unsupported want type %T", i, tt.want)
		}
	}
}

func TestBinaryOpErrors(t *testing.T) {
	tests := []struct {
		op      string
		left    object.Object
		right   object.Object
		wantErr string
	}{
		{"/", &object.Integer{Value: 1}, &object.Integer{Value: 0}, "division by zero"},
		{"%", &object.Integer{Value: 1}, &object.Integer{Value: 0}, "modulo by zero"},
		{"%", &object.Float{Value: 1}, &object.Integer{Value: 2}, "modulo requires INTEGER operands"},
		{"+", &object.String{Value: "a"}, &object.Integer{Value: 1}, "type mismatch: STRING + INTEGER"},
		{"*", &object.String{Value: "a"}, &object.Float{Value: 1.5}, "repeat count must be INTEGER"},
		{"*", &object.String{Value: "a"}, &object.Integer{Value: -1}, "repeat count must be non-negative"},
		{"-", &object.String{Value: "a"}, &object.String{Value: "b"}, "unknown operator for strings: -"},
		{"+", &object.Boolean{Value: true}, &object.Boolean{Value: false}, "unknown operator for booleans: +"},
		{"+", &object.Nil{}, &object.Nil{}, "invalid operator for nil: +"},
		{"+", &object.Nil{}, &object.Integer{Value: 1}, "cannot compare nil with INTEGER using +"},
	}
	for i, tt := range tests {
		_, err := BinaryOp(tt.op, tt.left, tt.right)
		if err == nil {
			t.Fatalf("tests[%d] expected error, got nil", i)
		}
		if err.Error() != tt.wantErr {
			t.Fatalf("tests[%d] expected %q, got %q", i, tt.wantErr, err.Error())
		}
	}
}

func TestBinaryOpStringRepeat(t *testing.T) {
	tests := []struct {
		left  object.Object
		right object.Object
		want  string
	}{
		{&object.String{Value: "a"}, &object.Integer{Value: 3}, "aaa"},
		{&object.Integer{Value: 2}, &object.String{Value: "ab"}, "abab"},
		{&object.String{Value: ""}, &object.Integer{Value: 3}, ""},
		{&object.String{Value: "🙂"}, &object.Integer{Value: 2}, "🙂🙂"},
	}
	for i, tt := range tests {
		got, err := BinaryOp("*", tt.left, tt.right)
		if err != nil {
			t.Fatalf("tests[%d] unexpected error: %v", i, err)
		}
		strObj, ok := got.(*object.String)
		if !ok || strObj.Value != tt.want {
			t.Fatalf("tests[%d] expected %q, got %T(%v)", i, tt.want, got, got)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		op    string
		left  object.Object
		right object.Object
		want  bool
	}{
		{"==", &object.Integer{Value: 1}, &object.Float{Value: 1.0}, true},
		{"!=", &object.Integer{Value: 1}, &object.Float{Value: 1.5}, true},
		{">", &object.Float{Value: 1.5}, &object.Integer{Value: 1}, true},
		{"<", &object.Float{Value: 1.5}, &object.Integer{Value: 2}, true},
		{">=", &object.Float{Value: 2.0}, &object.Integer{Value: 2}, true},
		{"<=", &object.Float{Value: 2.0}, &object.Integer{Value: 1}, false},
		{"==", &object.String{Value: "a"}, &object.String{Value: "a"}, true},
		{"!=", &object.String{Value: "a"}, &object.String{Value: "b"}, true},
		{"==", &object.Boolean{Value: true}, &object.Boolean{Value: true}, true},
		{"!=", &object.Boolean{Value: true}, &object.Boolean{Value: false}, true},
		{"==", &object.Nil{}, &object.Nil{}, true},
		{"!=", &object.Nil{}, &object.Integer{Value: 1}, true},
		{"==", &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, true},
		{"!=", &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 3}}}, true},
		{"!=", &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}}}, &object.Tuple{Elements: []object.Object{&object.Integer{Value: 1}, &object.Integer{Value: 2}}}, true},
	}
	for i, tt := range tests {
		got, err := Compare(tt.op, tt.left, tt.right)
		if err != nil {
			t.Fatalf("tests[%d] unexpected error: %v", i, err)
		}
		if got != tt.want {
			t.Fatalf("tests[%d] expected %v, got %v", i, tt.want, got)
		}
	}
}

func TestCompareErrors(t *testing.T) {
	tests := []struct {
		op      string
		left    object.Object
		right   object.Object
		wantErr string
	}{
		{"<", &object.String{Value: "a"}, &object.String{Value: "b"}, "unknown operator for strings: <"},
		{">", &object.Boolean{Value: true}, &object.Boolean{Value: false}, "unknown operator for booleans: >"},
		{"==", &object.Integer{Value: 1}, &object.String{Value: "1"}, "type mismatch: INTEGER == STRING"},
		{">", &object.Nil{}, &object.Integer{Value: 1}, "cannot compare nil with INTEGER using >"},
		{">", &object.Nil{}, &object.Nil{}, "invalid operator for nil: >"},
		{"<", &object.Tuple{Elements: []object.Object{}}, &object.Tuple{Elements: []object.Object{}}, "unknown operator for tuples: <"},
	}
	for i, tt := range tests {
		_, err := Compare(tt.op, tt.left, tt.right)
		if err == nil {
			t.Fatalf("tests[%d] expected error, got nil", i)
		}
		if err.Error() != tt.wantErr {
			t.Fatalf("tests[%d] expected %q, got %q", i, tt.wantErr, err.Error())
		}
	}
}

func TestIdentity(t *testing.T) {
	a := &object.Array{Elements: []object.Object{&object.Integer{Value: 1}}}
	b := &object.Array{Elements: []object.Object{&object.Integer{Value: 1}}}
	fn := &object.Function{}
	fnAlias := fn

	tests := []struct {
		name  string
		left  object.Object
		right object.Object
		want  bool
	}{
		{"nil_true", &object.Nil{}, &object.Nil{}, true},
		{"nil_false", &object.Nil{}, &object.Integer{Value: 0}, false},
		{"bool_true", &object.Boolean{Value: true}, &object.Boolean{Value: true}, true},
		{"int_true", &object.Integer{Value: 257}, &object.Integer{Value: 257}, true},
		{"int_float_false", &object.Integer{Value: 1}, &object.Float{Value: 1.0}, false},
		{"string_true", &object.String{Value: "abc"}, &object.String{Value: "abc"}, true},
		{"array_ref_true", a, a, true},
		{"array_ref_false", a, b, false},
		{"function_ref_true", fn, fnAlias, true},
		{"function_ref_false", &object.Function{}, &object.Function{}, false},
	}
	for _, tt := range tests {
		if got := Identity(tt.left, tt.right); got != tt.want {
			t.Fatalf("%s expected %v, got %v", tt.name, tt.want, got)
		}
	}

	is, err := Compare("is", &object.Integer{Value: 1}, &object.Float{Value: 1.0})
	if err != nil {
		t.Fatalf("unexpected compare error: %v", err)
	}
	if is {
		t.Fatalf("expected numeric type-sensitive identity to be false")
	}
}

func TestBitwiseOps(t *testing.T) {
	tests := []struct {
		op    string
		left  object.Object
		right object.Object
		want  int64
	}{
		{"|", &object.Integer{Value: 5}, &object.Integer{Value: 2}, 7},
		{"&", &object.Integer{Value: 5}, &object.Integer{Value: 2}, 0},
		{"^", &object.Integer{Value: 5}, &object.Integer{Value: 2}, 7},
		{"<<", &object.Integer{Value: 5}, &object.Integer{Value: 1}, 10},
		{">>", &object.Integer{Value: 5}, &object.Integer{Value: 1}, 2},
	}
	for i, tt := range tests {
		got, err := BitwiseBinary(tt.op, tt.left, tt.right)
		if err != nil {
			t.Fatalf("tests[%d] unexpected error: %v", i, err)
		}
		intObj, ok := got.(*object.Integer)
		if !ok || intObj.Value != tt.want {
			t.Fatalf("tests[%d] expected %d, got %T(%v)", i, tt.want, got, got)
		}
	}

	unary, err := BitwiseUnary("~", &object.Integer{Value: 0})
	if err != nil {
		t.Fatalf("unexpected unary error: %v", err)
	}
	if intObj, ok := unary.(*object.Integer); !ok || intObj.Value != -1 {
		t.Fatalf("expected unary ~0 to be -1, got %T(%v)", unary, unary)
	}
}

func TestBitwiseErrors(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		left    object.Object
		right   object.Object
		wantErr string
	}{
		{"type_mismatch", "|", &object.Integer{Value: 1}, &object.Float{Value: 1.0}, "unsupported operand types for |: INTEGER, FLOAT"},
		{"shift_too_large", "<<", &object.Integer{Value: 1}, &object.Integer{Value: 64}, "shift count out of range"},
		{"shift_negative", ">>", &object.Integer{Value: 1}, &object.Integer{Value: -1}, "shift count cannot be negative"},
	}
	for _, tt := range tests {
		_, err := BitwiseBinary(tt.op, tt.left, tt.right)
		if err == nil {
			t.Fatalf("%s expected error, got nil", tt.name)
		}
		if err.Error() != tt.wantErr {
			t.Fatalf("%s expected %q, got %q", tt.name, tt.wantErr, err.Error())
		}
	}

	_, err := BitwiseUnary("~", &object.Float{Value: 1.0})
	if err == nil {
		t.Fatalf("expected unary error, got nil")
	}
	if err.Error() != "unsupported operand type for ~: FLOAT" {
		t.Fatalf("expected unary error %q, got %q", "unsupported operand type for ~: FLOAT", err.Error())
	}
}
