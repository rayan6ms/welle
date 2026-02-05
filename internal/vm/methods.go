package vm

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"welle/internal/object"
	"welle/internal/semantics"
)

func applyMethod(name string, recv object.Object, args []object.Object) object.Object {
	if name == "get" && recv.Type() != object.DICT_OBJ {
		return &object.Error{Message: "get() receiver must be DICT"}
	}
	switch recv.Type() {
	case object.ARRAY_OBJ:
		switch name {
		case "append":
			return methodAppend(recv, args...)
		case "count":
			return methodArrayCount(recv, args...)
		case "len":
			return methodLen(recv, args...)
		case "pop":
			return methodArrayPop(recv, args...)
		case "remove":
			return methodArrayRemove(recv, args...)
		default:
			return &object.Error{Message: "unknown method for ARRAY: " + name}
		}
	case object.DICT_OBJ:
		switch name {
		case "count":
			return methodDictCount(recv, args...)
		case "get":
			return methodDictGet(recv, args...)
		case "keys":
			return methodKeys(recv, args...)
		case "pop":
			return methodDictPop(recv, args...)
		case "remove":
			return methodDictRemove(recv, args...)
		case "values":
			return methodValues(recv, args...)
		case "hasKey":
			return methodHasKey(recv, args...)
		default:
			return &object.Error{Message: "unknown method for DICT: " + name}
		}
	case object.STRING_OBJ:
		switch name {
		case "len":
			return methodLen(recv, args...)
		case "strip":
			return methodStrip(recv, args...)
		case "uppercase":
			return methodUppercase(recv, args...)
		case "lowercase":
			return methodLowercase(recv, args...)
		case "capitalize":
			return methodCapitalize(recv, args...)
		case "startswith":
			return methodStartsWith(recv, args...)
		case "endswith":
			return methodEndsWith(recv, args...)
		case "slice":
			return methodSlice(recv, args...)
		default:
			return &object.Error{Message: "unknown method for STRING: " + name}
		}
	case object.INTEGER_OBJ, object.FLOAT_OBJ:
		switch name {
		case "format":
			return methodFormatNumber(recv, args...)
		default:
			return &object.Error{Message: "unknown method for " + string(recv.Type()) + ": " + name}
		}
	}

	return &object.Error{Message: "type has no methods: " + string(recv.Type())}
}

func methodLen(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("len() takes 0 arguments, got %d", len(args))}
	}
	switch v := recv.(type) {
	case *object.String:
		return &object.Integer{Value: int64(utf8.RuneCountInString(v.Value))}
	case *object.Array:
		return &object.Integer{Value: int64(len(v.Elements))}
	case *object.Dict:
		return &object.Integer{Value: int64(len(v.Pairs))}
	default:
		return &object.Error{Message: "len() not supported for type: " + string(recv.Type())}
	}
}

func methodAppend(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("append() takes 1 argument, got %d", len(args))}
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return &object.Error{Message: "append() receiver must be ARRAY"}
	}
	els := make([]object.Object, 0, len(arr.Elements)+1)
	els = append(els, arr.Elements...)
	els = append(els, args[0])
	return &object.Array{Elements: els}
}

func methodArrayCount(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("count() takes 1 argument, got %d", len(args))}
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return &object.Error{Message: "count() receiver must be ARRAY"}
	}
	target := args[0]
	var count int64
	for _, el := range arr.Elements {
		eq, err := semantics.Compare("==", el, target)
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		if eq {
			count++
		}
	}
	return &object.Integer{Value: count}
}

func methodArrayPop(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("pop() takes 0 arguments, got %d", len(args))}
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return &object.Error{Message: "pop() receiver must be ARRAY"}
	}
	if len(arr.Elements) == 0 {
		return &object.Error{Message: "pop from empty array"}
	}
	last := arr.Elements[len(arr.Elements)-1]
	arr.Elements = arr.Elements[:len(arr.Elements)-1]
	return last
}

func methodArrayRemove(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("remove() takes 1 argument, got %d", len(args))}
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return &object.Error{Message: "remove() receiver must be ARRAY"}
	}
	target := args[0]
	for i, el := range arr.Elements {
		eq, err := semantics.Compare("==", el, target)
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		if eq {
			arr.Elements = append(arr.Elements[:i], arr.Elements[i+1:]...)
			return nativeBool(true)
		}
	}
	return nativeBool(false)
}

func methodKeys(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("keys() takes 0 arguments, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "keys() receiver must be DICT"}
	}
	pairs := object.SortedDictPairs(d)
	els := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		els = append(els, pair.Key)
	}
	return &object.Array{Elements: els}
}

func methodDictCount(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("count() takes 0 arguments, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "count() receiver must be DICT"}
	}
	return &object.Integer{Value: int64(len(d.Pairs))}
}

func methodDictGet(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("get() takes 1 or 2 arguments, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "get() receiver must be DICT"}
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[0].Type())}
	}
	if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
		return pair.Value
	}
	if len(args) == 2 {
		return args[1]
	}
	return nilObj
}

func methodDictPop(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("pop() takes 1 or 2 arguments, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "pop() receiver must be DICT"}
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[0].Type())}
	}
	key := object.HashKeyString(hk)
	if pair, exists := d.Pairs[key]; exists {
		delete(d.Pairs, key)
		return pair.Value
	}
	if len(args) == 2 {
		return args[1]
	}
	return &object.Error{Message: "key not found"}
}

func methodDictRemove(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("remove() takes 1 argument, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "remove() receiver must be DICT"}
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[0].Type())}
	}
	key := object.HashKeyString(hk)
	if _, exists := d.Pairs[key]; !exists {
		return &object.Error{Message: "key not found"}
	}
	delete(d.Pairs, key)
	return nilObj
}

func methodValues(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("values() takes 0 arguments, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "values() receiver must be DICT"}
	}
	pairs := object.SortedDictPairs(d)
	els := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		els = append(els, pair.Value)
	}
	return &object.Array{Elements: els}
}

func methodHasKey(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("hasKey() takes 1 argument, got %d", len(args))}
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return &object.Error{Message: "hasKey() receiver must be DICT"}
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[0].Type())}
	}
	_, exists := d.Pairs[object.HashKeyString(hk)]
	return nativeBool(exists)
}

func methodStrip(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("strip() takes 0 arguments, got %d", len(args))}
	}
	s := recv.(*object.String)
	return &object.String{Value: strings.TrimSpace(s.Value)}
}

func methodUppercase(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("uppercase() takes 0 arguments, got %d", len(args))}
	}
	s := recv.(*object.String)
	return &object.String{Value: strings.ToUpper(s.Value)}
}

func methodLowercase(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("lowercase() takes 0 arguments, got %d", len(args))}
	}
	s := recv.(*object.String)
	return &object.String{Value: strings.ToLower(s.Value)}
}

func methodCapitalize(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: fmt.Sprintf("capitalize() takes 0 arguments, got %d", len(args))}
	}
	s := recv.(*object.String)
	if s.Value == "" {
		return s
	}
	rs := []rune(s.Value)
	first := strings.ToUpper(string(rs[0]))
	rest := ""
	if len(rs) > 1 {
		rest = strings.ToLower(string(rs[1:]))
	}
	return &object.String{Value: first + rest}
}

func methodStartsWith(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("startswith() takes 1 argument, got %d", len(args))}
	}
	prefix, ok := args[0].(*object.String)
	if !ok {
		return &object.Error{Message: "startswith() prefix must be STRING"}
	}
	s := recv.(*object.String)
	return nativeBool(strings.HasPrefix(s.Value, prefix.Value))
}

func methodEndsWith(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("endswith() takes 1 argument, got %d", len(args))}
	}
	suffix, ok := args[0].(*object.String)
	if !ok {
		return &object.Error{Message: "endswith() suffix must be STRING"}
	}
	s := recv.(*object.String)
	return nativeBool(strings.HasSuffix(s.Value, suffix.Value))
}

func methodSlice(recv object.Object, args ...object.Object) object.Object {
	if len(args) > 2 {
		return &object.Error{Message: fmt.Sprintf("slice() takes 0, 1, or 2 arguments, got %d", len(args))}
	}
	var lowPtr *int64
	var highPtr *int64
	if len(args) >= 1 {
		i, ok := args[0].(*object.Integer)
		if !ok {
			return &object.Error{Message: fmt.Sprintf("slice low must be INTEGER, got: %s", args[0].Type())}
		}
		v := i.Value
		lowPtr = &v
	}
	if len(args) == 2 {
		i, ok := args[1].(*object.Integer)
		if !ok {
			return &object.Error{Message: fmt.Sprintf("slice high must be INTEGER, got: %s", args[1].Type())}
		}
		v := i.Value
		highPtr = &v
	}
	s := recv.(*object.String)
	rs := []rune(s.Value)
	out := &object.String{Value: string(sliceRunes(rs, lowPtr, highPtr, 1))}
	return out
}

func methodFormatNumber(recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("format() takes 1 argument, got %d", len(args))}
	}
	decObj, ok := args[0].(*object.Integer)
	if !ok {
		return &object.Error{Message: "format() decimals must be INTEGER"}
	}
	if decObj.Value < 0 {
		return &object.Error{Message: "format() decimals must be >= 0"}
	}
	decimals := int(decObj.Value)

	switch v := recv.(type) {
	case *object.Integer:
		return &object.String{Value: formatIntFixed(v.Value, decimals)}
	case *object.Float:
		return &object.String{Value: formatFloatFixed(v.Value, decimals)}
	default:
		return &object.Error{Message: "format() receiver must be NUMBER"}
	}
}

func formatIntFixed(value int64, decimals int) string {
	if decimals == 0 {
		return strconv.FormatInt(value, 10)
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return sign + strconv.FormatInt(value, 10) + "." + strings.Repeat("0", decimals)
}

func formatFloatFixed(value float64, decimals int) string {
	if decimals == 0 {
		return strconv.FormatFloat(math.Round(value), 'f', 0, 64)
	}
	scale := math.Pow10(decimals)
	rounded := math.Round(value*scale) / scale
	return strconv.FormatFloat(rounded, 'f', decimals, 64)
}
