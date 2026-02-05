package vm

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"welle/internal/formatutil"
	"welle/internal/gfx"
	"welle/internal/object"
	"welle/internal/runtimeio"
	"welle/internal/semantics"
)

var builtins = []*object.Builtin{
	{Fn: builtinPrint},          // index 0
	{Fn: builtinLen},            // 1
	{Fn: builtinStr},            // 2
	{Fn: builtinJoin},           // 3
	{Fn: builtinKeys},           // 4
	{Fn: builtinValues},         // 5
	{Fn: builtinPush},           // 6
	{Fn: builtinCount},          // 7
	{Fn: builtinRemove},         // 8
	{Fn: builtinGet},            // 9
	{Fn: builtinPop},            // 10
	{Fn: builtinError},          // 11
	{Fn: builtinRange},          // 12
	{Fn: builtinHasKey},         // 13
	{Fn: builtinSort},           // 14
	{Fn: builtinWriteFile},      // 15
	{Fn: builtinMathFloor},      // 16
	{Fn: builtinMathSqrt},       // 17
	{Fn: builtinMathSin},        // 18
	{Fn: builtinMathCos},        // 19
	{Fn: builtinGfxOpen},        // 20
	{Fn: builtinGfxClose},       // 21
	{Fn: builtinGfxShouldClose}, // 22
	{Fn: builtinGfxBeginFrame},  // 23
	{Fn: builtinGfxEndFrame},    // 24
	{Fn: builtinGfxClear},       // 25
	{Fn: builtinGfxRect},        // 26
	{Fn: builtinGfxPixel},       // 27
	{Fn: builtinGfxTime},        // 28
	{Fn: builtinGfxKeyDown},     // 29
	{Fn: builtinGfxMouseX},      // 30
	{Fn: builtinGfxMouseY},      // 31
	{Fn: builtinGfxPresent},     // 32
	{Fn: builtinImageNew},       // 33
	{Fn: builtinImageSet},       // 34
	{Fn: builtinImageFill},      // 35
	{Fn: builtinImageWidth},     // 36
	{Fn: builtinImageHeight},    // 37
	{Fn: builtinImageFillRect},  // 38
	{Fn: builtinImageFade},      // 39
	{Fn: builtinImageFadeWhite}, // 40
	{Fn: builtinMax},            // 41
	{Fn: builtinAbs},            // 42
	{Fn: builtinSum},            // 43
	{Fn: builtinReverse},        // 44
	{Fn: builtinAny},            // 45
	{Fn: builtinAll},            // 46
	{Fn: builtinMap},            // 47
	{Fn: builtinMean},           // 48
	{Fn: builtinSqrt},           // 49
	{Fn: builtinInput},          // 50
	{Fn: builtinGetPass},        // 51
	{Fn: builtinGroupDigits},    // 52
	{Fn: builtinFormatFloat},    // 53
	{Fn: builtinFormatPercent},  // 54
}

var builtinIndex = map[string]int{
	"print":            0,
	"len":              1,
	"str":              2,
	"join":             3,
	"keys":             4,
	"values":           5,
	"push":             6,
	"append":           6,
	"count":            7,
	"remove":           8,
	"get":              9,
	"pop":              10,
	"error":            11,
	"range":            12,
	"hasKey":           13,
	"sort":             14,
	"writeFile":        15,
	"math_floor":       16,
	"math_sqrt":        17,
	"math_sin":         18,
	"math_cos":         19,
	"gfx_open":         20,
	"gfx_close":        21,
	"gfx_shouldClose":  22,
	"gfx_beginFrame":   23,
	"gfx_endFrame":     24,
	"gfx_clear":        25,
	"gfx_rect":         26,
	"gfx_pixel":        27,
	"gfx_time":         28,
	"gfx_keyDown":      29,
	"gfx_mouseX":       30,
	"gfx_mouseY":       31,
	"gfx_present":      32,
	"image_new":        33,
	"image_set":        34,
	"image_fill":       35,
	"image_width":      36,
	"image_height":     37,
	"image_fill_rect":  38,
	"image_fade":       39,
	"image_fade_white": 40,
	"max":              41,
	"abs":              42,
	"sum":              43,
	"reverse":          44,
	"any":              45,
	"all":              46,
	"map":              47,
	"mean":             48,
	"sqrt":             49,
	"input":            50,
	"getpass":          51,
	"group_digits":     52,
	"format_float":     53,
	"format_percent":   54,
}

func builtinPrint(args ...object.Object) object.Object {
	for i, a := range args {
		if i > 0 {
			_, _ = fmt.Fprint(os.Stdout, " ")
		}
		_, _ = fmt.Fprint(os.Stdout, a.Inspect())
	}
	_, _ = fmt.Fprintln(os.Stdout)
	return nilObj
}

func builtinLen(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "len expects 1 argument"}
	}
	switch v := args[0].(type) {
	case *object.String:
		return &object.Integer{Value: int64(utf8.RuneCountInString(v.Value))}
	case *object.Array:
		return &object.Integer{Value: int64(len(v.Elements))}
	case *object.Dict:
		return &object.Integer{Value: int64(len(v.Pairs))}
	default:
		return &object.Error{Message: "len unsupported for type: " + string(args[0].Type())}
	}
}

func builtinStr(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "str expects 1 argument"}
	}
	return &object.String{Value: args[0].Inspect()}
}

func builtinGroupDigits(args ...object.Object) object.Object {
	if len(args) < 1 || len(args) > 3 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1 to 3, got %d", len(args))}
	}
	sep := ","
	group := int64(3)
	if len(args) >= 2 {
		s, ok := args[1].(*object.String)
		if !ok {
			return &object.Error{Message: "group_digits() sep must be STRING"}
		}
		sep = s.Value
	}
	if len(args) == 3 {
		g, ok := args[2].(*object.Integer)
		if !ok {
			return &object.Error{Message: "group_digits() group must be INTEGER"}
		}
		group = g.Value
	}

	var out string
	var err error
	switch x := args[0].(type) {
	case *object.Integer:
		out, err = formatutil.GroupDigitsFromInt(x.Value, sep, int(group))
	case *object.String:
		out, err = formatutil.GroupDigitsFromString(x.Value, sep, int(group))
	default:
		return &object.Error{Message: "group_digits() x must be INTEGER or STRING"}
	}
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.String{Value: out}
}

func builtinFormatFloat(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 2, got %d", len(args))}
	}
	dec, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "format_float() decimals must be INTEGER"}
	}
	var x float64
	switch v := args[0].(type) {
	case *object.Integer:
		x = float64(v.Value)
	case *object.Float:
		x = v.Value
	default:
		return &object.Error{Message: "format_float() x must be NUMBER"}
	}
	out, err := formatutil.FormatFloat(x, int(dec.Value))
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.String{Value: out}
}

func builtinFormatPercent(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 2, got %d", len(args))}
	}
	dec, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "format_percent() decimals must be INTEGER"}
	}
	var x float64
	switch v := args[0].(type) {
	case *object.Integer:
		x = float64(v.Value)
	case *object.Float:
		x = v.Value
	default:
		return &object.Error{Message: "format_percent() x must be NUMBER"}
	}
	out, err := formatutil.FormatPercent(x, int(dec.Value))
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.String{Value: out}
}

func builtinJoin(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 2, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "join() first argument must be ARRAY"}
	}
	sep, ok := args[1].(*object.String)
	if !ok {
		return &object.Error{Message: "join() separator must be STRING"}
	}
	parts := make([]string, len(arr.Elements))
	for i, el := range arr.Elements {
		s, ok := el.(*object.String)
		if !ok {
			return &object.Error{Message: "join() array elements must be STRING"}
		}
		parts[i] = s.Value
	}
	return &object.String{Value: strings.Join(parts, sep.Value)}
}

func builtinKeys(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "keys expects 1 argument"}
	}
	d, ok := args[0].(*object.Dict)
	if !ok {
		return &object.Error{Message: "keys expects DICT"}
	}
	pairs := object.SortedDictPairs(d)
	out := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.Key)
	}
	return &object.Array{Elements: out}
}

func builtinValues(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "values expects 1 argument"}
	}
	d, ok := args[0].(*object.Dict)
	if !ok {
		return &object.Error{Message: "values expects DICT"}
	}
	pairs := object.SortedDictPairs(d)
	out := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, pair.Value)
	}
	return &object.Array{Elements: out}
}

func builtinPush(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "push expects 2 arguments: (array, value)"}
	}
	a, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "push expects ARRAY as first argument"}
	}
	newElems := make([]object.Object, 0, len(a.Elements)+1)
	newElems = append(newElems, a.Elements...)
	newElems = append(newElems, args[1])
	return &object.Array{Elements: newElems}
}

func builtinCount(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "count() expects 2 arguments"}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "count() expects ARRAY as first argument"}
	}
	target := args[1]
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

func builtinRemove(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "remove() expects 2 arguments"}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "remove() expects ARRAY as first argument"}
	}
	target := args[1]
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

func builtinGet(args ...object.Object) object.Object {
	if len(args) != 2 && len(args) != 3 {
		return &object.Error{Message: "get() expects 2 or 3 arguments"}
	}
	d, ok := args[0].(*object.Dict)
	if !ok {
		return &object.Error{Message: "get() expects DICT as first argument"}
	}
	hk, ok := object.HashKeyOf(args[1])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[1].Type())}
	}
	if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
		return pair.Value
	}
	if len(args) == 3 {
		return args[2]
	}
	return nilObj
}

func builtinPop(args ...object.Object) object.Object {
	switch len(args) {
	case 1:
		arr, ok := args[0].(*object.Array)
		if !ok {
			return &object.Error{Message: "pop() expects ARRAY as first argument when called with 1 argument"}
		}
		if len(arr.Elements) == 0 {
			return &object.Error{Message: "pop from empty array"}
		}
		last := arr.Elements[len(arr.Elements)-1]
		arr.Elements = arr.Elements[:len(arr.Elements)-1]
		return last
	case 2, 3:
		d, ok := args[0].(*object.Dict)
		if !ok {
			return &object.Error{Message: "pop() expects DICT as first argument when called with 2 or 3 arguments"}
		}
		hk, ok := object.HashKeyOf(args[1])
		if !ok {
			return &object.Error{Message: "unusable as dict key: " + string(args[1].Type())}
		}
		key := object.HashKeyString(hk)
		if pair, exists := d.Pairs[key]; exists {
			delete(d.Pairs, key)
			return pair.Value
		}
		if len(args) == 3 {
			return args[2]
		}
		return &object.Error{Message: "key not found"}
	default:
		return &object.Error{Message: "pop() expects 1 argument (array) or 2/3 arguments (dict)"}
	}
}

func builtinError(args ...object.Object) object.Object {
	if len(args) < 1 || len(args) > 2 {
		return &object.Error{Message: "error expects 1 or 2 arguments: (message, code?)"}
	}

	var msg string
	switch v := args[0].(type) {
	case *object.String:
		msg = v.Value
	default:
		msg = v.Inspect()
	}

	errObj := &object.Error{Message: msg, IsValue: true}
	if len(args) == 2 {
		codeObj, ok := args[1].(*object.Integer)
		if !ok {
			return &object.Error{Message: "error code must be integer"}
		}
		errObj.Code = codeObj.Value
	}

	return errObj
}

func builtinRange(args ...object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 && len(args) != 3 {
		return &object.Error{Message: "range expects 1, 2, or 3 arguments"}
	}

	toInt := func(o object.Object) (*object.Integer, bool) {
		i, ok := o.(*object.Integer)
		return i, ok
	}

	var start, end, step int64
	step = 1

	if len(args) == 1 {
		n, ok := toInt(args[0])
		if !ok {
			return &object.Error{Message: "range expects INTEGER arguments"}
		}
		start = 0
		end = n.Value
	} else if len(args) == 2 {
		a, ok1 := toInt(args[0])
		b, ok2 := toInt(args[1])
		if !ok1 || !ok2 {
			return &object.Error{Message: "range expects INTEGER arguments"}
		}
		start = a.Value
		end = b.Value
	} else {
		a, ok1 := toInt(args[0])
		b, ok2 := toInt(args[1])
		c, ok3 := toInt(args[2])
		if !ok1 || !ok2 || !ok3 {
			return &object.Error{Message: "range expects INTEGER arguments"}
		}
		start = a.Value
		end = b.Value
		step = c.Value
		if step == 0 {
			return &object.Error{Message: "range step cannot be 0"}
		}
	}

	els := []object.Object{}
	if step > 0 {
		for i := start; i < end; i += step {
			els = append(els, &object.Integer{Value: i})
		}
	} else {
		for i := start; i > end; i += step {
			els = append(els, &object.Integer{Value: i})
		}
	}

	return &object.Array{Elements: els}
}

func builtinHasKey(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "hasKey expects 2 arguments"}
	}
	d, ok := args[0].(*object.Dict)
	if !ok {
		return &object.Error{Message: "hasKey expects DICT as first argument"}
	}
	hk, ok := object.HashKeyOf(args[1])
	if !ok {
		return &object.Error{Message: "unusable as dict key: " + string(args[1].Type())}
	}
	_, exists := d.Pairs[object.HashKeyString(hk)]
	if exists {
		return &object.Boolean{Value: true}
	}
	return &object.Boolean{Value: false}
}

func builtinSort(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "sort expects 1 argument"}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "sort expects ARRAY"}
	}
	els := make([]object.Object, len(arr.Elements))
	copy(els, arr.Elements)
	if len(els) < 2 {
		return &object.Array{Elements: els}
	}

	switch els[0].Type() {
	case object.INTEGER_OBJ:
		for _, e := range els {
			if e.Type() != object.INTEGER_OBJ {
				return &object.Error{Message: "sort requires all elements to be INTEGER"}
			}
		}
		ints := make([]int64, len(els))
		for i, e := range els {
			ints[i] = e.(*object.Integer).Value
		}
		sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })
		out := make([]object.Object, len(ints))
		for i, v := range ints {
			out[i] = &object.Integer{Value: v}
		}
		return &object.Array{Elements: out}

	case object.STRING_OBJ:
		for _, e := range els {
			if e.Type() != object.STRING_OBJ {
				return &object.Error{Message: "sort requires all elements to be STRING"}
			}
		}
		ss := make([]string, len(els))
		for i, e := range els {
			ss[i] = e.(*object.String).Value
		}
		sort.Strings(ss)
		out := make([]object.Object, len(ss))
		for i, v := range ss {
			out[i] = &object.String{Value: v}
		}
		return &object.Array{Elements: out}
	default:
		return &object.Error{Message: "sort supports only INTEGER or STRING lists (v0.1)"}
	}
}

func builtinMax(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "max() expects ARRAY"}
	}
	if len(arr.Elements) == 0 {
		return &object.Error{Message: "max() arg is an empty sequence"}
	}

	switch first := arr.Elements[0].(type) {
	case *object.Integer, *object.Float:
		maxFloat := 0.0
		maxInt := int64(0)
		hasFloat := false
		if v, ok := first.(*object.Float); ok {
			maxFloat = v.Value
			hasFloat = true
		} else {
			maxInt = first.(*object.Integer).Value
		}
		for i := 1; i < len(arr.Elements); i++ {
			switch v := arr.Elements[i].(type) {
			case *object.Integer:
				if hasFloat {
					val := float64(v.Value)
					if val > maxFloat {
						maxFloat = val
					}
				} else if v.Value > maxInt {
					maxInt = v.Value
				}
			case *object.Float:
				if !hasFloat {
					maxFloat = float64(maxInt)
					hasFloat = true
				}
				if v.Value > maxFloat {
					maxFloat = v.Value
				}
			default:
				return &object.Error{Message: "max() requires all elements to be NUMBER"}
			}
		}
		if hasFloat {
			return &object.Float{Value: maxFloat}
		}
		return &object.Integer{Value: maxInt}

	case *object.String:
		maxStr := first.Value
		for i := 1; i < len(arr.Elements); i++ {
			v, ok := arr.Elements[i].(*object.String)
			if !ok {
				return &object.Error{Message: "max() requires all elements to be STRING"}
			}
			if v.Value > maxStr {
				maxStr = v.Value
			}
		}
		return &object.String{Value: maxStr}

	default:
		return &object.Error{Message: "max() requires NUMBER or STRING elements"}
	}
}

func builtinAbs(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	switch v := args[0].(type) {
	case *object.Integer:
		if v.Value < 0 {
			return &object.Integer{Value: -v.Value}
		}
		return &object.Integer{Value: v.Value}
	case *object.Float:
		if v.Value < 0 {
			return &object.Float{Value: -v.Value}
		}
		return &object.Float{Value: v.Value}
	default:
		return &object.Error{Message: "abs() expects NUMBER"}
	}
}

func builtinSum(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "sum() expects ARRAY"}
	}
	var totalInt int64
	var totalFloat float64
	hasFloat := false
	for _, el := range arr.Elements {
		switch v := el.(type) {
		case *object.Integer:
			if hasFloat {
				totalFloat += float64(v.Value)
			} else {
				totalInt += v.Value
			}
		case *object.Float:
			if !hasFloat {
				totalFloat = float64(totalInt)
				hasFloat = true
			}
			totalFloat += v.Value
		default:
			return &object.Error{Message: "sum() requires all elements to be NUMBER"}
		}
	}
	if hasFloat {
		return &object.Float{Value: totalFloat}
	}
	return &object.Integer{Value: totalInt}
}

func builtinReverse(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	switch v := args[0].(type) {
	case *object.Array:
		out := make([]object.Object, len(v.Elements))
		for i := range v.Elements {
			out[len(v.Elements)-1-i] = v.Elements[i]
		}
		return &object.Array{Elements: out}
	case *object.String:
		runes := []rune(v.Value)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return &object.String{Value: string(runes)}
	default:
		return &object.Error{Message: "reverse() expects ARRAY or STRING"}
	}
}

func builtinAny(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "any() expects ARRAY"}
	}
	for _, el := range arr.Elements {
		if isTruthy(el) {
			return nativeBool(true)
		}
	}
	return nativeBool(false)
}

func builtinAll(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "all() expects ARRAY"}
	}
	for _, el := range arr.Elements {
		if !isTruthy(el) {
			return nativeBool(false)
		}
	}
	return nativeBool(true)
}

func builtinMap(args ...object.Object) object.Object {
	return &object.Error{Message: "map() is not directly callable"}
}

func builtinMean(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 1, got %d", len(args))}
	}
	arr, ok := args[0].(*object.Array)
	if !ok {
		return &object.Error{Message: "mean() expects ARRAY"}
	}
	if len(arr.Elements) == 0 {
		return &object.Error{Message: "mean() arg is an empty sequence"}
	}

	totalInt := int64(0)
	totalFloat := float64(0)
	hasFloat := false
	for _, el := range arr.Elements {
		switch v := el.(type) {
		case *object.Integer:
			if hasFloat {
				totalFloat += float64(v.Value)
			} else {
				totalInt += v.Value
			}
		case *object.Float:
			if !hasFloat {
				totalFloat = float64(totalInt)
				hasFloat = true
			}
			totalFloat += v.Value
		default:
			return &object.Error{Message: "mean() requires all elements to be NUMBER"}
		}
	}

	count := int64(len(arr.Elements))
	if hasFloat {
		return &object.Float{Value: totalFloat / float64(count)}
	}
	if totalInt%count == 0 {
		return &object.Integer{Value: totalInt / count}
	}
	return &object.Float{Value: float64(totalInt) / float64(count)}
}

func builtinWriteFile(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "writeFile expects 2 arguments: (path, content)"}
	}
	pathObj, ok := args[0].(*object.String)
	if !ok {
		return &object.Error{Message: "writeFile expects STRING path"}
	}
	contentObj, ok := args[1].(*object.String)
	if !ok {
		return &object.Error{Message: "writeFile expects STRING content"}
	}
	if err := os.WriteFile(pathObj.Value, []byte(contentObj.Value), 0644); err != nil {
		return &object.Error{Message: "writeFile failed: " + err.Error()}
	}
	return nilObj
}

func builtinMathFloor(args ...object.Object) object.Object {
	v, err := builtinFloatArg("math_floor", args...)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Integer{Value: int64(math.Floor(v))}
}

func builtinMathSqrt(args ...object.Object) object.Object {
	v, err := builtinFloatArg("math_sqrt", args...)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Float{Value: math.Sqrt(v)}
}

func builtinMathSin(args ...object.Object) object.Object {
	v, err := builtinFloatArg("math_sin", args...)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Float{Value: math.Sin(v)}
}

func builtinMathCos(args ...object.Object) object.Object {
	v, err := builtinFloatArg("math_cos", args...)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Float{Value: math.Cos(v)}
}

func builtinSqrt(args ...object.Object) object.Object {
	return builtinMathSqrt(args...)
}

func builtinInput(args ...object.Object) object.Object {
	if len(args) > 1 {
		return &object.Error{Message: "input() expects 0 or 1 arguments"}
	}
	prompt := ""
	if len(args) == 1 {
		str, ok := args[0].(*object.String)
		if !ok {
			return &object.Error{Message: "input() expects STRING prompt"}
		}
		prompt = str.Value
	}
	line, err := runtimeio.Input(prompt)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.String{Value: line}
}

func builtinGetPass(args ...object.Object) object.Object {
	if len(args) > 1 {
		return &object.Error{Message: "getpass() expects 0 or 1 arguments"}
	}
	prompt := ""
	if len(args) == 1 {
		str, ok := args[0].(*object.String)
		if !ok {
			return &object.Error{Message: "getpass() expects STRING prompt"}
		}
		prompt = str.Value
	}
	line, err := runtimeio.GetPass(prompt)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.String{Value: line}
}

func builtinFloatArg(name string, args ...object.Object) (float64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("%s expects 1 argument", name)
	}
	switch v := args[0].(type) {
	case *object.Integer:
		return float64(v.Value), nil
	case *object.Float:
		return v.Value, nil
	default:
		return 0, fmt.Errorf("%s expects NUMBER", name)
	}
}

func builtinGfxOpen(args ...object.Object) object.Object {
	if len(args) != 3 {
		return &object.Error{Message: "gfx_open expects 3 arguments: (width, height, title)"}
	}
	w, ok := args[0].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_open expects INTEGER width"}
	}
	h, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_open expects INTEGER height"}
	}
	title, ok := args[2].(*object.String)
	if !ok {
		return &object.Error{Message: "gfx_open expects STRING title"}
	}
	if err := gfx.Open(int(w.Value), int(h.Value), title.Value); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxClose(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_close expects no arguments"}
	}
	if err := gfx.Close(); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxShouldClose(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_shouldClose expects no arguments"}
	}
	return nativeBool(gfx.ShouldClose())
}

func builtinGfxBeginFrame(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_beginFrame expects no arguments"}
	}
	if err := gfx.BeginFrame(); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxEndFrame(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_endFrame expects no arguments"}
	}
	if err := gfx.EndFrame(); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxClear(args ...object.Object) object.Object {
	if len(args) != 4 {
		return &object.Error{Message: "gfx_clear expects 4 arguments: (r, g, b, a)"}
	}
	r, ok := gfxNumber(args[0])
	if !ok {
		return &object.Error{Message: "gfx_clear expects NUMBER channels"}
	}
	g, ok := gfxNumber(args[1])
	if !ok {
		return &object.Error{Message: "gfx_clear expects NUMBER channels"}
	}
	b, ok := gfxNumber(args[2])
	if !ok {
		return &object.Error{Message: "gfx_clear expects NUMBER channels"}
	}
	a, ok := gfxNumber(args[3])
	if !ok {
		return &object.Error{Message: "gfx_clear expects NUMBER channels"}
	}
	if err := gfx.Clear(r, g, b, a); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxRect(args ...object.Object) object.Object {
	if len(args) != 8 {
		return &object.Error{Message: "gfx_rect expects 8 arguments: (x, y, w, h, r, g, b, a)"}
	}
	x, ok := gfxNumber(args[0])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER position/size"}
	}
	y, ok := gfxNumber(args[1])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER position/size"}
	}
	w, ok := gfxNumber(args[2])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER position/size"}
	}
	h, ok := gfxNumber(args[3])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER position/size"}
	}
	r, ok := gfxNumber(args[4])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER channels"}
	}
	g, ok := gfxNumber(args[5])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER channels"}
	}
	b, ok := gfxNumber(args[6])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER channels"}
	}
	a, ok := gfxNumber(args[7])
	if !ok {
		return &object.Error{Message: "gfx_rect expects NUMBER channels"}
	}
	if err := gfx.Rect(x, y, w, h, r, g, b, a); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxPixel(args ...object.Object) object.Object {
	if len(args) != 6 {
		return &object.Error{Message: "gfx_pixel expects 6 arguments: (x, y, r, g, b, a)"}
	}
	x, ok := args[0].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER x/y"}
	}
	y, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER x/y"}
	}
	r, ok := args[2].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER channels"}
	}
	g, ok := args[3].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER channels"}
	}
	b, ok := args[4].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER channels"}
	}
	a, ok := args[5].(*object.Integer)
	if !ok {
		return &object.Error{Message: "gfx_pixel expects INTEGER channels"}
	}
	if err := gfx.Pixel(int(x.Value), int(y.Value), int(r.Value), int(g.Value), int(b.Value), int(a.Value)); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinGfxTime(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_time expects no arguments"}
	}
	v, err := gfx.TimeSeconds()
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Float{Value: v}
}

func builtinGfxKeyDown(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "gfx_keyDown expects 1 argument: (key)"}
	}
	key, ok := args[0].(*object.String)
	if !ok {
		return &object.Error{Message: "gfx_keyDown expects STRING key"}
	}
	v, err := gfx.KeyDown(key.Value)
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nativeBool(v)
}

func builtinGfxMouseX(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_mouseX expects no arguments"}
	}
	v, err := gfx.MouseX()
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Integer{Value: int64(v)}
}

func builtinGfxMouseY(args ...object.Object) object.Object {
	if len(args) != 0 {
		return &object.Error{Message: "gfx_mouseY expects no arguments"}
	}
	v, err := gfx.MouseY()
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return &object.Integer{Value: int64(v)}
}

func builtinGfxPresent(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "gfx_present expects 1 argument: (image)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "gfx_present expects IMAGE"}
	}
	if err := gfx.PresentRGBA(img.Width, img.Height, img.Data); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageNew(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "image_new expects 2 arguments: (width, height)"}
	}
	w, ok := args[0].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_new expects INTEGER width"}
	}
	h, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_new expects INTEGER height"}
	}
	img, err := object.NewImage(int(w.Value), int(h.Value))
	if err != nil {
		return &object.Error{Message: err.Error()}
	}
	return img
}

func builtinImageSet(args ...object.Object) object.Object {
	if len(args) != 7 {
		return &object.Error{Message: "image_set expects 7 arguments: (image, x, y, r, g, b, a)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_set expects IMAGE"}
	}
	x, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER x/y"}
	}
	y, ok := args[2].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER x/y"}
	}
	r, ok := args[3].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER channels"}
	}
	g, ok := args[4].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER channels"}
	}
	b, ok := args[5].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER channels"}
	}
	a, ok := args[6].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_set expects INTEGER channels"}
	}
	if err := img.SetPixel(int(x.Value), int(y.Value), int(r.Value), int(g.Value), int(b.Value), int(a.Value)); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageFill(args ...object.Object) object.Object {
	if len(args) != 5 {
		return &object.Error{Message: "image_fill expects 5 arguments: (image, r, g, b, a)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_fill expects IMAGE"}
	}
	r, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill expects INTEGER channels"}
	}
	g, ok := args[2].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill expects INTEGER channels"}
	}
	b, ok := args[3].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill expects INTEGER channels"}
	}
	a, ok := args[4].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill expects INTEGER channels"}
	}
	if err := img.Fill(int(r.Value), int(g.Value), int(b.Value), int(a.Value)); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageFillRect(args ...object.Object) object.Object {
	if len(args) != 9 {
		return &object.Error{Message: "image_fill_rect expects 9 arguments: (image, x, y, w, h, r, g, b, a)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects IMAGE"}
	}
	x, ok := args[1].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER x/y/w/h"}
	}
	y, ok := args[2].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER x/y/w/h"}
	}
	w, ok := args[3].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER x/y/w/h"}
	}
	h, ok := args[4].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER x/y/w/h"}
	}
	r, ok := args[5].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER channels"}
	}
	g, ok := args[6].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER channels"}
	}
	b, ok := args[7].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER channels"}
	}
	a, ok := args[8].(*object.Integer)
	if !ok {
		return &object.Error{Message: "image_fill_rect expects INTEGER channels"}
	}
	if err := img.FillRect(int(x.Value), int(y.Value), int(w.Value), int(h.Value), int(r.Value), int(g.Value), int(b.Value), int(a.Value)); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageFade(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "image_fade expects 2 arguments: (image, amount)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_fade expects IMAGE"}
	}
	amount, ok := gfxNumber(args[1])
	if !ok {
		return &object.Error{Message: "image_fade expects NUMBER amount"}
	}
	if err := img.Fade(amount); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageFadeWhite(args ...object.Object) object.Object {
	if len(args) != 2 {
		return &object.Error{Message: "image_fade_white expects 2 arguments: (image, amount)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_fade_white expects IMAGE"}
	}
	amount, ok := gfxNumber(args[1])
	if !ok {
		return &object.Error{Message: "image_fade_white expects NUMBER amount"}
	}
	if err := img.FadeToWhite(amount); err != nil {
		return &object.Error{Message: err.Error()}
	}
	return nilObj
}

func builtinImageWidth(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "image_width expects 1 argument: (image)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_width expects IMAGE"}
	}
	return &object.Integer{Value: int64(img.Width)}
}

func builtinImageHeight(args ...object.Object) object.Object {
	if len(args) != 1 {
		return &object.Error{Message: "image_height expects 1 argument: (image)"}
	}
	img, ok := args[0].(*object.Image)
	if !ok {
		return &object.Error{Message: "image_height expects IMAGE"}
	}
	return &object.Integer{Value: int64(img.Height)}
}

func gfxNumber(o object.Object) (float64, bool) {
	switch v := o.(type) {
	case *object.Integer:
		return float64(v.Value), true
	case *object.Float:
		return v.Value, true
	default:
		return 0, false
	}
}
