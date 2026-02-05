package semantics

import (
	"fmt"
	"strings"

	"welle/internal/object"
)

func IsTruthy(obj object.Object) bool {
	switch v := obj.(type) {
	case *object.Boolean:
		return v.Value
	case *object.Nil:
		return false
	default:
		return true
	}
}

func BinaryOp(op string, left, right object.Object) (object.Object, error) {
	if isBitwiseOp(op) {
		return BitwiseBinary(op, left, right)
	}
	if ls, ok := left.(*object.String); ok {
		if op == "*" {
			if ri, ok := right.(*object.Integer); ok {
				out, err := repeatString(ls.Value, ri.Value)
				if err != nil {
					return nil, err
				}
				return &object.String{Value: out}, nil
			}
			return nil, fmt.Errorf("repeat count must be INTEGER")
		}
		if rs, ok := right.(*object.String); ok {
			if op == "+" {
				return &object.String{Value: ls.Value + rs.Value}, nil
			}
			return nil, fmt.Errorf("unknown operator for strings: %s", op)
		}
	}
	if rs, ok := right.(*object.String); ok {
		if op == "*" {
			if li, ok := left.(*object.Integer); ok {
				out, err := repeatString(rs.Value, li.Value)
				if err != nil {
					return nil, err
				}
				return &object.String{Value: out}, nil
			}
			return nil, fmt.Errorf("repeat count must be INTEGER")
		}
	}

	if _, ok := left.(*object.Boolean); ok {
		if _, ok := right.(*object.Boolean); ok {
			return nil, fmt.Errorf("unknown operator for booleans: %s", op)
		}
	}

	if left.Type() == object.NIL_OBJ && right.Type() == object.NIL_OBJ {
		return nil, fmt.Errorf("invalid operator for nil: %s", op)
	}
	if left.Type() == object.NIL_OBJ || right.Type() == object.NIL_OBJ {
		nonNil := left
		if nonNil.Type() == object.NIL_OBJ {
			nonNil = right
		}
		return nil, fmt.Errorf("cannot compare nil with %s using %s", nonNil.Type(), op)
	}

	if li, lok := left.(*object.Integer); lok {
		if ri, rok := right.(*object.Integer); rok {
			switch op {
			case "+":
				return &object.Integer{Value: li.Value + ri.Value}, nil
			case "-":
				return &object.Integer{Value: li.Value - ri.Value}, nil
			case "*":
				return &object.Integer{Value: li.Value * ri.Value}, nil
			case "/":
				if ri.Value == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return &object.Integer{Value: li.Value / ri.Value}, nil
			case "%":
				if ri.Value == 0 {
					return nil, fmt.Errorf("modulo by zero")
				}
				return &object.Integer{Value: li.Value % ri.Value}, nil
			default:
				return nil, fmt.Errorf("unknown operator for integers: %s", op)
			}
		}
	}

	if isNumeric(left) && isNumeric(right) {
		lf := toFloat(left)
		rf := toFloat(right)
		switch op {
		case "+":
			return &object.Float{Value: lf + rf}, nil
		case "-":
			return &object.Float{Value: lf - rf}, nil
		case "*":
			return &object.Float{Value: lf * rf}, nil
		case "/":
			if rf == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return &object.Float{Value: lf / rf}, nil
		case "%":
			return nil, fmt.Errorf("modulo requires INTEGER operands")
		default:
			return nil, fmt.Errorf("unknown operator for numbers: %s", op)
		}
	}

	if left.Type() != right.Type() {
		return nil, fmt.Errorf("type mismatch: %s %s %s", left.Type(), op, right.Type())
	}

	return nil, fmt.Errorf("unknown operator: %s %s %s", left.Type(), op, right.Type())
}

func repeatString(s string, count int64) (string, error) {
	if count < 0 {
		return "", fmt.Errorf("repeat count must be non-negative")
	}
	if count == 0 || s == "" {
		return "", nil
	}
	maxInt := int64(int(^uint(0) >> 1))
	if count > maxInt {
		return "", fmt.Errorf("repeat count too large")
	}
	size := int64(len(s))
	if size > 0 && size > maxInt/count {
		return "", fmt.Errorf("repeat count too large")
	}
	return strings.Repeat(s, int(count)), nil
}

func BitwiseUnary(op string, right object.Object) (object.Object, error) {
	ri, ok := right.(*object.Integer)
	if !ok {
		return nil, fmt.Errorf("unsupported operand type for %s: %s", op, right.Type())
	}
	switch op {
	case "~":
		return &object.Integer{Value: ^ri.Value}, nil
	default:
		return nil, fmt.Errorf("unknown unary operator: %s", op)
	}
}

func BitwiseBinary(op string, left, right object.Object) (object.Object, error) {
	li, lok := left.(*object.Integer)
	ri, rok := right.(*object.Integer)
	if !lok || !rok {
		return nil, fmt.Errorf("unsupported operand types for %s: %s, %s", op, left.Type(), right.Type())
	}
	switch op {
	case "|":
		return &object.Integer{Value: li.Value | ri.Value}, nil
	case "&":
		return &object.Integer{Value: li.Value & ri.Value}, nil
	case "^":
		return &object.Integer{Value: li.Value ^ ri.Value}, nil
	case "<<":
		if ri.Value < 0 {
			return nil, fmt.Errorf("shift count cannot be negative")
		}
		if ri.Value >= 64 {
			return nil, fmt.Errorf("shift count out of range")
		}
		return &object.Integer{Value: int64(uint64(li.Value) << uint64(ri.Value))}, nil
	case ">>":
		if ri.Value < 0 {
			return nil, fmt.Errorf("shift count cannot be negative")
		}
		if ri.Value >= 64 {
			return nil, fmt.Errorf("shift count out of range")
		}
		return &object.Integer{Value: li.Value >> uint64(ri.Value)}, nil
	default:
		return nil, fmt.Errorf("unknown bitwise operator: %s", op)
	}
}

func Compare(op string, left, right object.Object) (bool, error) {
	if op == "is" {
		return Identity(left, right), nil
	}

	if left.Type() == object.NIL_OBJ && right.Type() == object.NIL_OBJ {
		switch op {
		case "==":
			return true, nil
		case "!=":
			return false, nil
		default:
			return false, fmt.Errorf("invalid operator for nil: %s", op)
		}
	}
	if left.Type() == object.NIL_OBJ || right.Type() == object.NIL_OBJ {
		switch op {
		case "==":
			return false, nil
		case "!=":
			return true, nil
		default:
			nonNil := left
			if nonNil.Type() == object.NIL_OBJ {
				nonNil = right
			}
			return false, fmt.Errorf("cannot compare nil with %s using %s", nonNil.Type(), op)
		}
	}

	if li, lok := left.(*object.Integer); lok {
		if ri, rok := right.(*object.Integer); rok {
			switch op {
			case "==":
				return li.Value == ri.Value, nil
			case "!=":
				return li.Value != ri.Value, nil
			case "<":
				return li.Value < ri.Value, nil
			case "<=":
				return li.Value <= ri.Value, nil
			case ">":
				return li.Value > ri.Value, nil
			case ">=":
				return li.Value >= ri.Value, nil
			default:
				return false, fmt.Errorf("unknown operator for integers: %s", op)
			}
		}
	}

	if isNumeric(left) && isNumeric(right) {
		lf := toFloat(left)
		rf := toFloat(right)
		switch op {
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		case "<":
			return lf < rf, nil
		case "<=":
			return lf <= rf, nil
		case ">":
			return lf > rf, nil
		case ">=":
			return lf >= rf, nil
		default:
			return false, fmt.Errorf("unknown operator for numbers: %s", op)
		}
	}

	if ls, ok := left.(*object.String); ok {
		if rs, ok := right.(*object.String); ok {
			switch op {
			case "==":
				return ls.Value == rs.Value, nil
			case "!=":
				return ls.Value != rs.Value, nil
			default:
				return false, fmt.Errorf("unknown operator for strings: %s", op)
			}
		}
	}

	if lb, ok := left.(*object.Boolean); ok {
		if rb, ok := right.(*object.Boolean); ok {
			switch op {
			case "==":
				return lb.Value == rb.Value, nil
			case "!=":
				return lb.Value != rb.Value, nil
			default:
				return false, fmt.Errorf("unknown operator for booleans: %s", op)
			}
		}
	}

	if lt, ok := left.(*object.Tuple); ok {
		if rt, ok := right.(*object.Tuple); ok {
			switch op {
			case "==", "!=":
				if len(lt.Elements) != len(rt.Elements) {
					return op == "!=", nil
				}
				for i := range lt.Elements {
					eq, err := Compare("==", lt.Elements[i], rt.Elements[i])
					if err != nil {
						return false, err
					}
					if !eq {
						return op == "!=", nil
					}
				}
				return op == "==", nil
			default:
				return false, fmt.Errorf("unknown operator for tuples: %s", op)
			}
		}
	}

	if left.Type() != right.Type() {
		return false, fmt.Errorf("type mismatch: %s %s %s", left.Type(), op, right.Type())
	}

	return false, fmt.Errorf("unknown operator: %s %s %s", left.Type(), op, right.Type())
}

func Identity(left, right object.Object) bool {
	if left == nil || right == nil {
		return left == right
	}

	switch l := left.(type) {
	case *object.Nil:
		_, ok := right.(*object.Nil)
		return ok
	case *object.Boolean:
		r, ok := right.(*object.Boolean)
		return ok && l.Value == r.Value
	case *object.Integer:
		r, ok := right.(*object.Integer)
		return ok && l.Value == r.Value
	case *object.Float:
		r, ok := right.(*object.Float)
		return ok && l.Value == r.Value
	case *object.String:
		r, ok := right.(*object.String)
		return ok && l.Value == r.Value
	case *object.Array:
		r, ok := right.(*object.Array)
		return ok && l == r
	case *object.Dict:
		r, ok := right.(*object.Dict)
		return ok && l == r
	case *object.Tuple:
		r, ok := right.(*object.Tuple)
		return ok && l == r
	case *object.Function:
		r, ok := right.(*object.Function)
		return ok && l == r
	case *object.CompiledFunction:
		r, ok := right.(*object.CompiledFunction)
		return ok && l == r
	case *object.Closure:
		r, ok := right.(*object.Closure)
		return ok && l == r
	case *object.Cell:
		r, ok := right.(*object.Cell)
		return ok && l == r
	case *object.Error:
		r, ok := right.(*object.Error)
		return ok && l == r
	case *object.Image:
		r, ok := right.(*object.Image)
		return ok && l == r
	case *object.Builtin:
		r, ok := right.(*object.Builtin)
		return ok && l == r
	default:
		return left == right
	}
}

func InOp(left, right object.Object) (bool, error) {
	switch r := right.(type) {
	case *object.Array:
		for _, el := range r.Elements {
			eq, err := Compare("==", left, el)
			if err != nil {
				return false, err
			}
			if eq {
				return true, nil
			}
		}
		return false, nil
	case *object.String:
		ls, ok := left.(*object.String)
		if !ok {
			return false, fmt.Errorf("left operand of 'in' must be string when right operand is string")
		}
		return strings.Contains(r.Value, ls.Value), nil
	case *object.Dict:
		hk, ok := object.HashKeyOf(left)
		if !ok {
			return false, fmt.Errorf("unusable as dict key: %s", left.Type())
		}
		_, exists := r.Pairs[object.HashKeyString(hk)]
		return exists, nil
	default:
		return false, fmt.Errorf("cannot use 'in' with %s", right.Type())
	}
}

// DictUpdateCount returns how many new entries would be added by merging src into dst.
func DictUpdateCount(dst, src *object.Dict) int {
	if dst == nil || src == nil || len(src.Pairs) == 0 {
		return 0
	}
	if dst.Pairs == nil {
		return len(src.Pairs)
	}
	count := 0
	for k := range src.Pairs {
		if _, exists := dst.Pairs[k]; !exists {
			count++
		}
	}
	return count
}

// DictUpdate merges src into dst in-place and returns the number of new entries added.
func DictUpdate(dst, src *object.Dict) int {
	if dst == nil || src == nil || len(src.Pairs) == 0 {
		return 0
	}
	if dst.Pairs == nil {
		dst.Pairs = make(map[string]object.DictPair, len(src.Pairs))
	}
	added := 0
	for k, pair := range src.Pairs {
		if _, exists := dst.Pairs[k]; !exists {
			added++
		}
		dst.Pairs[k] = pair
	}
	return added
}

func isNumeric(o object.Object) bool {
	switch o.(type) {
	case *object.Integer, *object.Float:
		return true
	default:
		return false
	}
}

func toFloat(o object.Object) float64 {
	switch v := o.(type) {
	case *object.Float:
		return v.Value
	case *object.Integer:
		return float64(v.Value)
	default:
		return 0
	}
}

func isBitwiseOp(op string) bool {
	switch op {
	case "|", "&", "^", "<<", ">>":
		return true
	default:
		return false
	}
}
