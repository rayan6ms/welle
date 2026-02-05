package evaluator

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"welle/internal/ast"
	"welle/internal/object"
	"welle/internal/semantics"
	"welle/internal/token"
)

var (
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
	NIL   = &object.Nil{}

	importHook     func(string) object.Object
	importResolver func(string, string) (string, error)
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	ctx.Budget = nil
	return eval(node, env, nil, 0, 0)
}

func eval(node ast.Node, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	switch n := node.(type) {

	case *ast.Program:
		return evalProgram(n, env, r, loopDepth, switchDepth)

	case *ast.BlockStatement:
		return evalBlock(n, env, r, loopDepth, switchDepth)

	// Statements
	case *ast.ExpressionStatement:
		return eval(n.Expression, env, r, loopDepth, switchDepth)

	case *ast.AssignStatement:
		op := n.Op
		if op == token.WALRUS {
			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			if _, exists := env.GetHere(n.Name.Value); exists {
				return newErrorAt(n.OpToken, fmt.Sprintf("cannot redeclare %q in this scope", n.Name.Value))
			}
			env.Set(n.Name.Value, val)
			return val
		}
		if op == "" || op == token.ASSIGN {
			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			if _, ok := env.Assign(n.Name.Value, val); ok {
				return val
			}
			env.Set(n.Name.Value, val)
			return val
		}

		cur, ok := env.Get(n.Name.Value)
		if !ok {
			return newErrorAt(n.Token, "unknown identifier: "+n.Name.Value)
		}
		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		if isReturn(val) {
			return val
		}
		if op == token.BITOR_ASSIGN {
			tok := n.OpToken
			if tok.Type == "" {
				tok = n.Token
			}
			res := applyDictUpdate(tok, cur, val)
			if isError(res) {
				return res
			}
			if _, ok := env.Assign(n.Name.Value, res); ok {
				return res
			}
			env.Set(n.Name.Value, res)
			return res
		}
		opStr, ok := compoundAssignOp(op)
		if !ok {
			return newErrorAt(n.Token, "unknown assignment operator: "+string(op))
		}
		res, err := semantics.BinaryOp(opStr, cur, val)
		if err != nil {
			tok := n.OpToken
			if tok.Type == "" {
				tok = n.Token
			}
			return newErrorAt(tok, err.Error())
		}
		if _, ok := env.Assign(n.Name.Value, res); ok {
			return res
		}
		env.Set(n.Name.Value, res)
		return res

	case *ast.AssignExpression:
		switch left := n.Left.(type) {
		case *ast.Identifier:
			op := n.Op
			if op == token.WALRUS {
				val := eval(n.Value, env, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				if isReturn(val) {
					return val
				}
				if _, exists := env.GetHere(left.Value); exists {
					return newErrorAt(n.Token, fmt.Sprintf("cannot redeclare %q in this scope", left.Value))
				}
				env.Set(left.Value, val)
				return val
			}
			if op == "" || op == token.ASSIGN {
				val := eval(n.Value, env, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				if isReturn(val) {
					return val
				}
				if _, ok := env.Assign(left.Value, val); ok {
					return val
				}
				env.Set(left.Value, val)
				return val
			}

			cur, ok := env.Get(left.Value)
			if !ok {
				return newErrorAt(left.Token, "unknown identifier: "+left.Value)
			}
			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			if op == token.BITOR_ASSIGN {
				tok := n.Token
				if tok.Type == "" {
					tok = left.Token
				}
				res := applyDictUpdate(tok, cur, val)
				if isError(res) {
					return res
				}
				if _, ok := env.Assign(left.Value, res); ok {
					return res
				}
				env.Set(left.Value, res)
				return res
			}
			opStr, ok := compoundAssignOp(op)
			if !ok {
				return newErrorAt(left.Token, "unknown assignment operator: "+string(op))
			}
			res, err := semantics.BinaryOp(opStr, cur, val)
			if err != nil {
				tok := n.Token
				if tok.Type == "" {
					tok = left.Token
				}
				return newErrorAt(tok, err.Error())
			}
			if _, ok := env.Assign(left.Value, res); ok {
				return res
			}
			env.Set(left.Value, res)
			return res

		case *ast.IndexExpression:
			base := eval(left.Left, env, r, loopDepth, switchDepth)
			if isError(base) {
				return base
			}
			index := eval(left.Index, env, r, loopDepth, switchDepth)
			if isError(index) {
				return index
			}

			if n.Op != "" && n.Op != token.ASSIGN {
				old := evalIndexExpression(left.Token, base, index)
				if isError(old) {
					return old
				}
				val := eval(n.Value, env, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				if isReturn(val) {
					return val
				}
				if n.Op == token.BITOR_ASSIGN {
					res := applyDictUpdate(n.Token, old, val)
					if isError(res) {
						return res
					}
					return evalIndexAssign(left, base, index, res)
				}
				opStr, ok := compoundAssignOp(n.Op)
				if !ok {
					return newErrorAt(n.Token, "unknown assignment operator: "+string(n.Op))
				}
				res, err := semantics.BinaryOp(opStr, old, val)
				if err != nil {
					return newErrorAt(n.Token, err.Error())
				}
				return evalIndexAssign(left, base, index, res)
			}

			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			return evalIndexAssign(left, base, index, val)

		case *ast.MemberExpression:
			obj := eval(left.Object, env, r, loopDepth, switchDepth)
			if isError(obj) {
				return obj
			}

			d, ok := obj.(*object.Dict)
			if !ok {
				return newErrorAt(n.Token, "member assignment not supported on type: "+string(obj.Type()))
			}
			key := &object.String{Value: left.Property.Value}
			hk, ok := object.HashKeyOf(key)
			if !ok {
				return newErrorAt(n.Token, "invalid member key")
			}

			if n.Op != "" && n.Op != token.ASSIGN {
				pair, ok := d.Pairs[object.HashKeyString(hk)]
				if !ok {
					return newErrorAt(n.Token, "unknown member: "+left.Property.Value)
				}
				val := eval(n.Value, env, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				if isReturn(val) {
					return val
				}
				if n.Op == token.BITOR_ASSIGN {
					res := applyDictUpdate(n.Token, pair.Value, val)
					if isError(res) {
						return res
					}
					if d.Pairs == nil {
						d.Pairs = map[string]object.DictPair{}
					}
					d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: res}
					return res
				}
				opStr, ok := compoundAssignOp(n.Op)
				if !ok {
					return newErrorAt(n.Token, "unknown assignment operator: "+string(n.Op))
				}
				res, err := semantics.BinaryOp(opStr, pair.Value, val)
				if err != nil {
					return newErrorAt(n.Token, err.Error())
				}
				if d.Pairs == nil {
					d.Pairs = map[string]object.DictPair{}
				}
				d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: res}
				return res
			}

			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if d.Pairs == nil {
				d.Pairs = map[string]object.DictPair{}
			}
			keyStr := object.HashKeyString(hk)
			if _, exists := d.Pairs[keyStr]; !exists {
				if errObj := chargeMemoryAt(n.Token, object.CostDictEntry()); errObj != nil {
					return errObj
				}
			}
			d.Pairs[keyStr] = object.DictPair{Key: key, Value: val}
			return val

		default:
			return newErrorAt(n.Token, "invalid assignment target")
		}

	case *ast.DestructureAssignStatement:
		if n.Op != "" && n.Op != token.ASSIGN {
			return newErrorAt(n.Token, "destructuring assignment supports only '='")
		}
		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		if isReturn(val) {
			return val
		}
		starIdx := -1
		for i, t := range n.Targets {
			if t != nil && t.Star {
				starIdx = i
				break
			}
		}
		if starIdx != -1 {
			for i := starIdx + 1; i < len(n.Targets); i++ {
				if n.Targets[i] != nil && n.Targets[i].Star {
					return newErrorAt(n.Token, "destructuring assignment allows only one starred target")
				}
			}
		}

		var elems []object.Object
		switch v := val.(type) {
		case *object.Tuple:
			elems = v.Elements
		case *object.Array:
			elems = v.Elements
		default:
			if starIdx >= 0 {
				return newErrorAt(n.Token, "cannot unpack non-sequence")
			}
			return newErrorAt(n.Token, "unpack expects tuple, got "+string(val.Type()))
		}

		if starIdx == -1 {
			if len(elems) != len(n.Targets) {
				return newErrorAt(n.Token, fmt.Sprintf("tuple arity mismatch: expected %d, got %d", len(n.Targets), len(elems)))
			}
			for i, t := range n.Targets {
				if t == nil || t.Name == nil || t.Name.Value == "_" {
					continue
				}
				elem := elems[i]
				if _, ok := env.Assign(t.Name.Value, elem); ok {
					continue
				}
				env.Set(t.Name.Value, elem)
			}
			return val
		}

		minLen := len(n.Targets) - 1
		if len(elems) < minLen {
			return newErrorAt(n.Token, fmt.Sprintf("not enough values to unpack (expected at least %d, got %d)", minLen, len(elems)))
		}

		headCount := starIdx
		tailCount := len(n.Targets) - starIdx - 1

		for i := 0; i < headCount; i++ {
			t := n.Targets[i]
			if t == nil || t.Name == nil || t.Name.Value == "_" {
				continue
			}
			elem := elems[i]
			if _, ok := env.Assign(t.Name.Value, elem); ok {
				continue
			}
			env.Set(t.Name.Value, elem)
		}

		for i := 0; i < tailCount; i++ {
			t := n.Targets[len(n.Targets)-1-i]
			if t == nil || t.Name == nil || t.Name.Value == "_" {
				continue
			}
			elem := elems[len(elems)-1-i]
			if _, ok := env.Assign(t.Name.Value, elem); ok {
				continue
			}
			env.Set(t.Name.Value, elem)
		}

		midStart := headCount
		midEnd := len(elems) - tailCount
		mid := make([]object.Object, 0, midEnd-midStart)
		for i := midStart; i < midEnd; i++ {
			mid = append(mid, elems[i])
		}
		if errObj := chargeMemoryAt(n.Token, object.CostArray(len(mid))); errObj != nil {
			return errObj
		}
		midArr := &object.Array{Elements: mid}
		starTarget := n.Targets[starIdx]
		if starTarget != nil && starTarget.Name != nil && starTarget.Name.Value != "_" {
			if _, ok := env.Assign(starTarget.Name.Value, midArr); !ok {
				env.Set(starTarget.Name.Value, midArr)
			}
		}
		return val

	case *ast.IndexAssignStatement:
		idx, ok := n.Left.(*ast.IndexExpression)
		if !ok {
			return newErrorAt(n.Token, "index assignment expects index expression on left")
		}
		left := eval(idx.Left, env, r, loopDepth, switchDepth)
		if isError(left) {
			return left
		}
		index := eval(idx.Index, env, r, loopDepth, switchDepth)
		if isError(index) {
			return index
		}

		if n.Op != "" && n.Op != token.ASSIGN {
			old := evalIndexExpression(idx.Token, left, index)
			if isError(old) {
				return old
			}
			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			if n.Op == token.BITOR_ASSIGN {
				res := applyDictUpdate(n.Token, old, val)
				if isError(res) {
					return res
				}
				return evalIndexAssign(idx, left, index, res)
			}
			opStr, ok := compoundAssignOp(n.Op)
			if !ok {
				return newErrorAt(n.Token, "unknown assignment operator: "+string(n.Op))
			}
			res, err := semantics.BinaryOp(opStr, old, val)
			if err != nil {
				return newErrorAt(n.Token, err.Error())
			}
			return evalIndexAssign(idx, left, index, res)
		}

		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		return evalIndexAssign(idx, left, index, val)

	case *ast.MemberAssignStatement:
		obj := eval(n.Object, env, r, loopDepth, switchDepth)
		if isError(obj) {
			return obj
		}

		d, ok := obj.(*object.Dict)
		if !ok {
			return newErrorAt(n.Token, "member assignment not supported on type: "+string(obj.Type()))
		}
		key := &object.String{Value: n.Property.Value}
		hk, ok := object.HashKeyOf(key)
		if !ok {
			return newErrorAt(n.Token, "invalid member key")
		}

		if n.Op != "" && n.Op != token.ASSIGN {
			pair, ok := d.Pairs[object.HashKeyString(hk)]
			if !ok {
				return newErrorAt(n.Token, "unknown member: "+n.Property.Value)
			}
			val := eval(n.Value, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			if n.Op == token.BITOR_ASSIGN {
				res := applyDictUpdate(n.Token, pair.Value, val)
				if isError(res) {
					return res
				}
				if d.Pairs == nil {
					d.Pairs = map[string]object.DictPair{}
				}
				d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: res}
				return res
			}
			opStr, ok := compoundAssignOp(n.Op)
			if !ok {
				return newErrorAt(n.Token, "unknown assignment operator: "+string(n.Op))
			}
			res, err := semantics.BinaryOp(opStr, pair.Value, val)
			if err != nil {
				return newErrorAt(n.Token, err.Error())
			}
			if d.Pairs == nil {
				d.Pairs = map[string]object.DictPair{}
			}
			d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: res}
			return res
		}

		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		if d.Pairs == nil {
			d.Pairs = map[string]object.DictPair{}
		}
		d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: val}
		return val

	case *ast.ExportStatement:
		res := eval(n.Stmt, env, r, loopDepth, switchDepth)
		if isError(res) {
			return res
		}
		switch s := n.Stmt.(type) {
		case *ast.AssignStatement:
			env.MarkExport(s.Name.Value)
		case *ast.FuncStatement:
			env.MarkExport(s.Name.Value)
		default:
			return newErrorAt(n.Token, "export supports only function declarations and assignments (v0.1)")
		}
		return res

	case *ast.ReturnStatement:
		switch len(n.ReturnValues) {
		case 0:
			return &object.ReturnValue{Value: NIL}
		case 1:
			val := eval(n.ReturnValues[0], env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			if isReturn(val) {
				return val
			}
			return &object.ReturnValue{Value: val}
		default:
			elems := make([]object.Object, len(n.ReturnValues))
			for i, rv := range n.ReturnValues {
				val := eval(rv, env, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				if isReturn(val) {
					return val
				}
				elems[i] = val
			}
			return &object.ReturnValue{Value: &object.Tuple{Elements: elems}}
		}

	case *ast.DeferStatement:
		fr := currentFrame()
		if fr == nil {
			return newErrorAt(n.Token, "defer used outside of a function")
		}
		if _, ok := n.Call.(*ast.CallExpression); !ok {
			return newErrorAt(n.Token, "defer expects a call expression")
		}
		fr.defers = append(fr.defers, n.Call)
		return NIL

	case *ast.ThrowStatement:
		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		return wrapThrownValue(n.Token, val)

	case *ast.BreakStatement:
		if loopDepth == 0 && switchDepth == 0 {
			return newErrorAt(n.Token, "break used outside of a loop or switch")
		}
		return &object.Break{}

	case *ast.ContinueStatement:
		if loopDepth == 0 {
			return newErrorAt(n.Token, "continue used outside of a loop")
		}
		return &object.Continue{}

	case *ast.PassStatement:
		return NIL

	case *ast.TryStatement:
		return evalTry(n, env, r, loopDepth, switchDepth)

	case *ast.IfStatement:
		return evalIf(n, env, r, loopDepth, switchDepth)

	case *ast.WhileStatement:
		return evalWhile(n, env, r, loopDepth, switchDepth)

	case *ast.ForStatement:
		return evalForC(n, env, r, loopDepth, switchDepth)

	case *ast.ForInStatement:
		return evalForIn(n, env, r, loopDepth, switchDepth)

	case *ast.SwitchStatement:
		return evalSwitchStatement(n, env, r, loopDepth, switchDepth)

	case *ast.FuncStatement:
		fn := &object.Function{
			Name:       n.Name.Value,
			File:       ctx.File,
			Parameters: n.Parameters,
			Body:       n.Body,
			Env:        env,
		}
		if errObj := chargeMemoryAt(n.Token, object.CostFunction()); errObj != nil {
			return errObj
		}
		env.Set(n.Name.Value, fn)
		return fn

	case *ast.FunctionLiteral:
		fn := &object.Function{
			Name:       ast.AnonymousFuncName(n.Token),
			File:       ctx.File,
			Parameters: n.Parameters,
			Body:       n.Body,
			Env:        env,
		}
		if errObj := chargeMemoryAt(n.Token, object.CostFunction()); errObj != nil {
			return errObj
		}
		return fn

	case *ast.ImportStatement:
		if importHook == nil || importResolver == nil {
			return newErrorAt(n.Token, "import not available in this mode")
		}
		resolved, err := importResolver(ctx.File, n.Path.Value)
		if err != nil {
			return newErrorAt(n.Token, err.Error())
		}
		modObj := importHook(resolved)
		if isError(modObj) {
			return modObj
		}
		mod, ok := modObj.(*object.Dict)
		if !ok {
			return newErrorAt(n.Token, "import did not return a module")
		}

		name := ""
		if n.Alias != nil {
			name = n.Alias.Value
		} else {
			base := filepath.Base(resolved)
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}

		env.Set(name, mod)
		return NIL

	case *ast.FromImportStatement:
		return evalFromImport(n, env)

	// Expressions
	case *ast.MatchExpression:
		return evalMatchExpression(n, env, r, loopDepth, switchDepth)

	case *ast.Identifier:
		return evalIdentifier(n, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: n.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: n.Value}

	case *ast.StringLiteral:
		out := &object.String{Value: n.Value}
		if errObj := chargeMemoryAt(n.Token, object.CostStringBytes(len(out.Value))); errObj != nil {
			return errObj
		}
		return out

	case *ast.TemplateLiteral:
		return evalTemplateLiteral(n, env, r, loopDepth, switchDepth)

	case *ast.BooleanLiteral:
		return nativeBool(n.Value)

	case *ast.NilLiteral:
		return NIL

	case *ast.ListLiteral:
		els := evalExpressions(n.Elements, env, r, loopDepth, switchDepth)
		if len(els) == 1 && isError(els[0]) {
			return els[0]
		}
		if errObj := chargeMemoryAt(n.Token, object.CostArray(len(els))); errObj != nil {
			return errObj
		}
		return &object.Array{Elements: els}

	case *ast.ListComprehension:
		seq := eval(n.Seq, env, r, loopDepth, switchDepth)
		if isError(seq) {
			return seq
		}
		compEnv := object.NewEnclosedEnvironment(env)
		out := []object.Object{}
		appendElem := func(el object.Object) object.Object {
			out = append(out, el)
			return nil
		}

		switch s := seq.(type) {
		case *object.Array:
			for _, el := range s.Elements {
				compEnv.Set(n.Var.Value, el)
				if n.Filter != nil {
					cond := eval(n.Filter, compEnv, r, loopDepth, switchDepth)
					if isError(cond) {
						return cond
					}
					if !isTruthy(cond) {
						continue
					}
				}
				val := eval(n.Elem, compEnv, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				appendElem(val)
			}
		case *object.Dict:
			pairs := object.SortedDictPairs(s)
			for _, pair := range pairs {
				compEnv.Set(n.Var.Value, pair.Key)
				if n.Filter != nil {
					cond := eval(n.Filter, compEnv, r, loopDepth, switchDepth)
					if isError(cond) {
						return cond
					}
					if !isTruthy(cond) {
						continue
					}
				}
				val := eval(n.Elem, compEnv, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				appendElem(val)
			}
		case *object.String:
			rs := []rune(s.Value)
			for _, rch := range rs {
				strObj := &object.String{Value: string(rch)}
				if errObj := chargeMemoryAt(n.Token, object.CostStringBytes(len(strObj.Value))); errObj != nil {
					return errObj
				}
				compEnv.Set(n.Var.Value, strObj)
				if n.Filter != nil {
					cond := eval(n.Filter, compEnv, r, loopDepth, switchDepth)
					if isError(cond) {
						return cond
					}
					if !isTruthy(cond) {
						continue
					}
				}
				val := eval(n.Elem, compEnv, r, loopDepth, switchDepth)
				if isError(val) {
					return val
				}
				appendElem(val)
			}
		default:
			return newErrorAt(n.Token, "cannot iterate "+string(seq.Type())+" in comprehension")
		}

		if errObj := chargeMemoryAt(n.Token, object.CostArray(len(out))); errObj != nil {
			return errObj
		}
		return &object.Array{Elements: out}

	case *ast.TupleLiteral:
		els := evalExpressions(n.Elements, env, r, loopDepth, switchDepth)
		if len(els) == 1 && isError(els[0]) {
			return els[0]
		}
		if errObj := chargeMemoryAt(n.Token, object.CostTuple(len(els))); errObj != nil {
			return errObj
		}
		return &object.Tuple{Elements: els}

	case *ast.DictLiteral:
		return evalDictLiteral(n, env, r, loopDepth, switchDepth)

	case *ast.IndexExpression:
		left := eval(n.Left, env, r, loopDepth, switchDepth)
		if isError(left) {
			return left
		}
		idx := eval(n.Index, env, r, loopDepth, switchDepth)
		if isError(idx) {
			return idx
		}
		return evalIndexExpression(n.Token, left, idx)

	case *ast.SliceExpression:
		left := eval(n.Left, env, r, loopDepth, switchDepth)
		if isError(left) {
			return left
		}
		var lowObj object.Object
		var highObj object.Object
		var stepObj object.Object
		if n.Low != nil {
			lowObj = eval(n.Low, env, r, loopDepth, switchDepth)
			if isError(lowObj) {
				return lowObj
			}
		}
		if n.High != nil {
			highObj = eval(n.High, env, r, loopDepth, switchDepth)
			if isError(highObj) {
				return highObj
			}
		}
		if n.Step != nil {
			stepObj = eval(n.Step, env, r, loopDepth, switchDepth)
			if isError(stepObj) {
				return stepObj
			}
		}
		return evalSliceExpression(n.Token, left, lowObj, highObj, stepObj)

	case *ast.PrefixExpression:
		right := eval(n.Right, env, r, loopDepth, switchDepth)
		if isError(right) {
			return right
		}
		return evalPrefix(n.Token, n.Operator, right)

	case *ast.InfixExpression:
		if n.Operator == "??" {
			left := eval(n.Left, env, r, loopDepth, switchDepth)
			if isError(left) {
				return left
			}
			if left.Type() == object.NIL_OBJ {
				return eval(n.Right, env, r, loopDepth, switchDepth)
			}
			return left
		}
		if n.Operator == "and" || n.Operator == "or" {
			left := eval(n.Left, env, r, loopDepth, switchDepth)
			if isError(left) {
				return left
			}
			if n.Operator == "and" {
				if !isTruthy(left) {
					return FALSE
				}
				right := eval(n.Right, env, r, loopDepth, switchDepth)
				if isError(right) {
					return right
				}
				return nativeBool(isTruthy(right))
			}
			if isTruthy(left) {
				return TRUE
			}
			right := eval(n.Right, env, r, loopDepth, switchDepth)
			if isError(right) {
				return right
			}
			return nativeBool(isTruthy(right))
		}
		left := eval(n.Left, env, r, loopDepth, switchDepth)
		if isError(left) {
			return left
		}
		right := eval(n.Right, env, r, loopDepth, switchDepth)
		if isError(right) {
			return right
		}
		return evalInfix(n.Token, n.Operator, left, right)

	case *ast.ConditionalExpression:
		cond := eval(n.Cond, env, r, loopDepth, switchDepth)
		if isError(cond) {
			return cond
		}
		if isTruthy(cond) {
			return eval(n.Then, env, r, loopDepth, switchDepth)
		}
		return eval(n.Else, env, r, loopDepth, switchDepth)

	case *ast.CondExpr:
		cond := eval(n.Cond, env, r, loopDepth, switchDepth)
		if isError(cond) {
			return cond
		}
		if isTruthy(cond) {
			return eval(n.Then, env, r, loopDepth, switchDepth)
		}
		return eval(n.Else, env, r, loopDepth, switchDepth)

	case *ast.MemberExpression:
		obj := eval(n.Object, env, r, loopDepth, switchDepth)
		if isError(obj) {
			return obj
		}

		if d, ok := obj.(*object.Dict); ok {
			key := &object.String{Value: n.Property.Value}
			hk, _ := object.HashKeyOf(key)
			pair, ok := d.Pairs[object.HashKeyString(hk)]
			if !ok {
				return newErrorAt(n.Token, "unknown member: "+n.Property.Value)
			}
			return pair.Value
		}

		if getter, ok := obj.(object.MemberGetter); ok {
			if val, ok := getter.GetMember(n.Property.Value); ok {
				return val
			}
			return newErrorAt(n.Token, "unknown member on "+string(obj.Type())+": "+n.Property.Value)
		}

		return newErrorAt(n.Token, "member access not supported on type: "+string(obj.Type()))

	case *ast.CallExpression:
		if me, ok := n.Function.(*ast.MemberExpression); ok {
			recv := eval(me.Object, env, r, loopDepth, switchDepth)
			if isError(recv) {
				return recv
			}
			args := evalCallArguments(n.Arguments, env, r, loopDepth, switchDepth)
			if len(args) == 1 && isError(args[0]) {
				return args[0]
			}
			if d, ok := recv.(*object.Dict); ok {
				key := &object.String{Value: me.Property.Value}
				hk, _ := object.HashKeyOf(key)
				if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
					return applyFunction(n.Token, pair.Value, args, r)
				}
			}
			return applyMethod(n.Token, recv, me.Property.Value, args)
		}

		fn := eval(n.Function, env, r, loopDepth, switchDepth)
		if isError(fn) {
			return fn
		}
		args := evalCallArguments(n.Arguments, env, r, loopDepth, switchDepth)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(n.Token, fn, args, r)
	}

	// fallback
	return NIL
}

func evalProgram(p *ast.Program, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	var result object.Object = NIL
	for _, stmt := range p.Statements {
		result = eval(stmt, env, r, loopDepth, switchDepth)
		if rv, ok := result.(*object.ReturnValue); ok {
			return rv.Value
		}
		if isError(result) {
			return result
		}
	}
	return result
}

func evalBlock(b *ast.BlockStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	var result object.Object = NIL
	for _, stmt := range b.Statements {
		result = eval(stmt, env, r, loopDepth, switchDepth)
		if result != nil {
			switch result.Type() {
			case object.RETURN_VALUE_OBJ, object.BREAK_OBJ, object.CONTINUE_OBJ, object.ERROR_OBJ:
				return result
			}
		}
	}
	return result
}

func evalTry(n *ast.TryStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	res := eval(n.TryBlock, env, r, loopDepth, switchDepth)
	if isError(res) && n.CatchBlock != nil {
		catchEnv := object.NewEnclosedEnvironment(env)
		if errObj, ok := res.(*object.Error); ok {
			catchEnv.Set(n.CatchName.Value, &object.Error{
				Message: errObj.Message,
				Code:    errObj.Code,
				Stack:   errObj.Stack,
				IsValue: true,
			})
		} else {
			catchEnv.Set(n.CatchName.Value, res)
		}
		res = eval(n.CatchBlock, catchEnv, r, loopDepth, switchDepth)
	}

	if n.FinallyBlock != nil {
		finallyRes := eval(n.FinallyBlock, env, r, loopDepth, switchDepth)
		if isError(finallyRes) {
			return finallyRes
		}
	}
	return res
}

func evalIf(s *ast.IfStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	cond := eval(s.Condition, env, r, loopDepth, switchDepth)
	if isError(cond) {
		return cond
	}
	if isTruthy(cond) {
		return eval(s.Consequence, env, r, loopDepth, switchDepth)
	}
	if s.Alternative != nil {
		return eval(s.Alternative, env, r, loopDepth, switchDepth)
	}
	return NIL
}

func evalWhile(s *ast.WhileStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	var result object.Object = NIL
	for {
		cond := eval(s.Condition, env, r, loopDepth, switchDepth)
		if isError(cond) {
			return cond
		}
		if !isTruthy(cond) {
			break
		}
		result = eval(s.Body, env, r, loopDepth+1, switchDepth)
		if isError(result) {
			return result
		}
		if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
			return result
		}
		if isBreak(result) {
			return NIL
		}
		if isContinue(result) {
			continue
		}
	}
	return result
}

func evalForIn(s *ast.ForInStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	iterable := eval(s.Iterable, env, r, loopDepth, switchDepth)
	if isError(iterable) {
		return iterable
	}

	switch it := iterable.(type) {
	case *object.Array:
		if s.Destruct {
			return newErrorAt(s.Token, "for-in destructuring requires dict, got ARRAY")
		}
		var result object.Object = NIL
		for _, el := range it.Elements {
			env.Set(s.Var.Value, el)
			result = eval(s.Body, env, r, loopDepth+1, switchDepth)
			if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
				return result
			}
			if isError(result) {
				return result
			}
			if isBreak(result) {
				return NIL
			}
			if isContinue(result) {
				continue
			}
		}
		return result

	case *object.String:
		if s.Destruct {
			return newErrorAt(s.Token, "for-in destructuring requires dict, got STRING")
		}
		var result object.Object = NIL
		rs := []rune(it.Value)
		for _, rch := range rs {
			strObj := &object.String{Value: string(rch)}
			if errObj := chargeMemoryAt(s.Token, object.CostStringBytes(len(strObj.Value))); errObj != nil {
				return errObj
			}
			env.Set(s.Var.Value, strObj)
			result = eval(s.Body, env, r, loopDepth+1, switchDepth)
			if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
				return result
			}
			if isError(result) {
				return result
			}
			if isBreak(result) {
				return NIL
			}
			if isContinue(result) {
				continue
			}
		}
		return result

	case *object.Dict:
		var result object.Object = NIL
		pairs := object.SortedDictPairs(it)
		for _, pair := range pairs {
			if s.Destruct {
				if s.Key != nil && s.Key.Value != "_" {
					env.Set(s.Key.Value, pair.Key)
				}
				if s.Value != nil && s.Value.Value != "_" {
					env.Set(s.Value.Value, pair.Value)
				}
			} else {
				env.Set(s.Var.Value, pair.Key)
			}
			result = eval(s.Body, env, r, loopDepth+1, switchDepth)
			if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
				return result
			}
			if isError(result) {
				return result
			}
			if isBreak(result) {
				return NIL
			}
			if isContinue(result) {
				continue
			}
		}
		return result

	default:
		if s.Destruct {
			return newErrorAt(s.Token, "for-in destructuring requires dict, got "+string(iterable.Type()))
		}
		return newErrorAt(s.Token, "cannot iterate over type: "+string(iterable.Type()))
	}
}

func evalForC(s *ast.ForStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	var result object.Object = NIL
	if s.Init != nil {
		result = eval(s.Init, env, r, loopDepth, switchDepth)
		if isError(result) {
			return result
		}
	}

	for {
		if s.Cond != nil {
			cond := eval(s.Cond, env, r, loopDepth, switchDepth)
			if isError(cond) {
				return cond
			}
			if !isTruthy(cond) {
				break
			}
		}

		result = eval(s.Body, env, r, loopDepth+1, switchDepth)
		if isError(result) {
			return result
		}
		if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
			return result
		}
		if isBreak(result) {
			return NIL
		}
		if isContinue(result) {
			if s.Post != nil {
				postResult := eval(s.Post, env, r, loopDepth, switchDepth)
				if isError(postResult) {
					return postResult
				}
			}
			continue
		}

		if s.Post != nil {
			postResult := eval(s.Post, env, r, loopDepth, switchDepth)
			if isError(postResult) {
				return postResult
			}
		}
	}

	return result
}

func evalSwitchStatement(n *ast.SwitchStatement, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	val := eval(n.Value, env, r, loopDepth, switchDepth)
	if isError(val) {
		return val
	}

	for _, c := range n.Cases {
		for _, cond := range c.Values {
			cv := eval(cond, env, r, loopDepth, switchDepth)
			if isError(cv) {
				return cv
			}

			eq := evalInfix(c.Token, "==", val, cv)
			if isError(eq) {
				return eq
			}

			if isTruthy(eq) {
				result := eval(c.Body, env, r, loopDepth, switchDepth+1)
				if isError(result) {
					return result
				}
				if isReturn(result) {
					return result
				}
				if isBreak(result) {
					return NIL
				}
				return result
			}
		}
	}

	if n.Default != nil {
		result := eval(n.Default, env, r, loopDepth, switchDepth+1)
		if isError(result) {
			return result
		}
		if isReturn(result) {
			return result
		}
		if isBreak(result) {
			return NIL
		}
		return result
	}

	return NIL
}

func evalMatchExpression(n *ast.MatchExpression, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	val := eval(n.Value, env, r, loopDepth, switchDepth)
	if isError(val) {
		return val
	}

	for _, c := range n.Cases {
		for _, cond := range c.Values {
			cv := eval(cond, env, r, loopDepth, switchDepth)
			if isError(cv) {
				return cv
			}

			eq := evalInfix(c.Token, "==", val, cv)
			if isError(eq) {
				return eq
			}

			if isTruthy(eq) {
				result := eval(c.Result, env, r, loopDepth, switchDepth)
				if isError(result) {
					return result
				}
				return result
			}
		}
	}

	if n.Default != nil {
		result := eval(n.Default, env, r, loopDepth, switchDepth)
		if isError(result) {
			return result
		}
		return result
	}

	return NIL
}

func evalFromImport(n *ast.FromImportStatement, env *object.Environment) object.Object {
	if importHook == nil || importResolver == nil {
		return newErrorAt(n.Token, "import not available in this mode")
	}

	resolved, err := importResolver(ctx.File, n.Path.Value)
	if err != nil {
		return newErrorAt(n.Token, err.Error())
	}
	modObj := importHook(resolved)
	if isError(modObj) {
		return modObj
	}
	mod, ok := modObj.(*object.Dict)
	if !ok {
		return newErrorAt(n.Token, "from-import did not return a module")
	}

	for _, it := range n.Items {
		name := it.Name.Value
		key := &object.String{Value: name}
		hk, _ := object.HashKeyOf(key)
		pair, ok := mod.Pairs[object.HashKeyString(hk)]
		if !ok {
			return newErrorAt(n.Token, fmt.Sprintf("missing export %q in module %q", name, n.Path.Value))
		}
		bind := name
		if it.Alias != nil {
			bind = it.Alias.Value
		}
		env.Set(bind, pair.Value)
	}

	return NIL
}

func evalIdentifier(i *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(i.Value); ok {
		return val
	}
	if b, ok := builtins[i.Value]; ok {
		return b
	}
	return newErrorAt(i.Token, "unknown identifier: "+i.Value)
}

func evalPrefix(tok token.Token, op string, right object.Object) object.Object {
	if isError(right) {
		return right
	}

	switch op {
	case "-":
		switch r := right.(type) {
		case *object.Integer:
			return &object.Integer{Value: -r.Value}
		case *object.Float:
			return &object.Float{Value: -r.Value}
		default:
			return newErrorAt(tok, "invalid operand for unary '-': "+string(right.Type()))
		}
	case "~":
		res, err := semantics.BitwiseUnary(op, right)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		return res
	case "not", "!":
		return nativeBool(!isTruthy(right))
	default:
		return newErrorAt(tok, "unknown prefix operator: "+op)
	}
}

func evalInfix(tok token.Token, op string, left, right object.Object) object.Object {
	if isError(left) {
		return left
	}
	if isError(right) {
		return right
	}

	switch op {
	case "+", "-", "*", "/", "%", "|", "&", "^", "<<", ">>":
		res, err := semantics.BinaryOp(op, left, right)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		if s, ok := res.(*object.String); ok {
			if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(s.Value))); errObj != nil {
				return errObj
			}
		}
		return res
	case "==", "!=", "is", "<", "<=", ">", ">=":
		b, err := semantics.Compare(op, left, right)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		return nativeBool(b)
	case "in":
		b, err := semantics.InOp(left, right)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		return nativeBool(b)
	default:
		return newErrorAt(tok, "unknown operator: "+string(left.Type())+" "+op+" "+string(right.Type()))
	}
}

func evalTemplateLiteral(n *ast.TemplateLiteral, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	if n.Tagged {
		tag := eval(n.Tag, env, r, loopDepth, switchDepth)
		if isError(tag) {
			return tag
		}

		parts := make([]object.Object, len(n.Parts))
		for i, part := range n.Parts {
			if errObj := chargeMemoryAt(n.Token, object.CostStringBytes(len(part))); errObj != nil {
				return errObj
			}
			parts[i] = &object.String{Value: part}
		}
		if errObj := chargeMemoryAt(n.Token, object.CostTuple(len(parts))); errObj != nil {
			return errObj
		}

		args := make([]object.Object, 0, len(n.Exprs)+1)
		args = append(args, &object.Tuple{Elements: parts})
		for _, ex := range n.Exprs {
			val := eval(ex, env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			args = append(args, val)
		}
		return applyFunction(n.Token, tag, args, r)
	}

	var b strings.Builder
	for i, part := range n.Parts {
		b.WriteString(part)
		if i < len(n.Exprs) {
			val := eval(n.Exprs[i], env, r, loopDepth, switchDepth)
			if isError(val) {
				return val
			}
			b.WriteString(val.Inspect())
		}
	}
	out := b.String()
	if errObj := chargeMemoryAt(n.Token, object.CostStringBytes(len(out))); errObj != nil {
		return errObj
	}
	return &object.String{Value: out}
}

func evalExpressions(exps []ast.Expression, env *object.Environment, r *Runner, loopDepth int, switchDepth int) []object.Object {
	out := make([]object.Object, 0, len(exps))
	for _, e := range exps {
		evaluated := eval(e, env, r, loopDepth, switchDepth)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		out = append(out, evaluated)
	}
	return out
}

func evalCallArguments(exps []ast.Expression, env *object.Environment, r *Runner, loopDepth int, switchDepth int) []object.Object {
	out := make([]object.Object, 0, len(exps))
	for _, e := range exps {
		if spread, ok := e.(*ast.SpreadExpression); ok {
			value := eval(spread.Value, env, r, loopDepth, switchDepth)
			if isError(value) {
				return []object.Object{value}
			}
			switch v := value.(type) {
			case *object.Tuple:
				out = append(out, v.Elements...)
			case *object.Array:
				out = append(out, v.Elements...)
			default:
				return []object.Object{newErrorAt(spread.Token, "cannot spread "+string(value.Type())+" in call arguments")}
			}
			continue
		}

		evaluated := eval(e, env, r, loopDepth, switchDepth)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		out = append(out, evaluated)
	}
	return out
}

func evalDictLiteral(n *ast.DictLiteral, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	pairs := make(map[string]object.DictPair, len(n.Pairs))
	for _, pair := range n.Pairs {
		if pair.Shorthand != nil {
			key := &object.String{Value: pair.Shorthand.Value}
			hk, _ := object.HashKeyOf(key)

			v := eval(pair.Shorthand, env, r, loopDepth, switchDepth)
			if isError(v) {
				return v
			}

			pairs[object.HashKeyString(hk)] = object.DictPair{Key: key, Value: v}
			continue
		}

		k := eval(pair.Key, env, r, loopDepth, switchDepth)
		if isError(k) {
			return k
		}
		hk, ok := object.HashKeyOf(k)
		if !ok {
			return newErrorAt(n.Token, "unusable as dict key: "+string(k.Type()))
		}

		v := eval(pair.Value, env, r, loopDepth, switchDepth)
		if isError(v) {
			return v
		}

		pairs[object.HashKeyString(hk)] = object.DictPair{Key: k, Value: v}
	}
	if errObj := chargeMemoryAt(n.Token, object.CostDict(len(pairs))); errObj != nil {
		return errObj
	}
	return &object.Dict{Pairs: pairs}
}

func evalIndexExpression(tok token.Token, left, index object.Object) object.Object {
	if arr, ok := left.(*object.Array); ok {
		i, ok := index.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "list index must be INTEGER, got: "+string(index.Type()))
		}
		n := int(i.Value)
		l := len(arr.Elements)
		if n < 0 {
			n = l + n
		}
		if n < 0 || n >= l {
			return newErrorAt(tok, "index out of range")
		}
		return arr.Elements[n]
	}

	if tup, ok := left.(*object.Tuple); ok {
		i, ok := index.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "tuple index must be INTEGER, got: "+string(index.Type()))
		}
		n := int(i.Value)
		l := len(tup.Elements)
		if n < 0 {
			n = l + n
		}
		if n < 0 || n >= l {
			return newErrorAt(tok, "index out of range")
		}
		return tup.Elements[n]
	}

	if d, ok := left.(*object.Dict); ok {
		hk, ok := object.HashKeyOf(index)
		if !ok {
			return newErrorAt(tok, "unusable as dict key: "+string(index.Type()))
		}
		pair, ok := d.Pairs[object.HashKeyString(hk)]
		if !ok {
			return NIL
		}
		return pair.Value
	}

	if s, ok := left.(*object.String); ok {
		i, ok := index.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "string index must be INTEGER, got: "+string(index.Type()))
		}

		r := []rune(s.Value)
		n := int(i.Value)
		l := len(r)
		if n < 0 {
			n = l + n
		}
		if n < 0 || n >= l {
			return newErrorAt(tok, "index out of range")
		}
		out := &object.String{Value: string(r[n])}
		if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
			return errObj
		}
		return out
	}

	return newErrorAt(tok, "indexing not supported on type: "+string(left.Type()))
}

func evalIndexAssign(idx *ast.IndexExpression, left, index, val object.Object) object.Object {
	switch l := left.(type) {
	case *object.Array:
		i, ok := index.(*object.Integer)
		if !ok {
			return newErrorAt(idx.Token, "array index must be INTEGER, got: "+string(index.Type()))
		}
		length := int64(len(l.Elements))
		pos := i.Value
		if pos < 0 {
			pos = length + pos
		}
		if pos < 0 || pos >= length {
			return newErrorAt(idx.Token, "index out of range")
		}
		l.Elements[int(pos)] = val
		return val

	case *object.Dict:
		hk, ok := object.HashKeyOf(index)
		if !ok {
			return newErrorAt(idx.Token, "unusable as dict key: "+string(index.Type()))
		}
		if l.Pairs == nil {
			l.Pairs = map[string]object.DictPair{}
		}
		keyStr := object.HashKeyString(hk)
		if _, exists := l.Pairs[keyStr]; !exists {
			if errObj := chargeMemoryAt(idx.Token, object.CostDictEntry()); errObj != nil {
				return errObj
			}
		}
		l.Pairs[keyStr] = object.DictPair{Key: index, Value: val}
		return val

	case *object.String:
		return newErrorAt(idx.Token, "cannot assign into STRING (immutable)")

	default:
		return newErrorAt(idx.Token, "index assignment not supported on type: "+string(left.Type()))
	}
}

func compoundAssignOp(op token.Type) (string, bool) {
	switch op {
	case token.PLUS_ASSIGN:
		return "+", true
	case token.MINUS_ASSIGN:
		return "-", true
	case token.STAR_ASSIGN:
		return "*", true
	case token.SLASH_ASSIGN:
		return "/", true
	case token.PERCENT_ASSIGN:
		return "%", true
	default:
		return "", false
	}
}

func applyDictUpdate(tok token.Token, left, right object.Object) object.Object {
	ld, ok := left.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "|= left operand must be dict")
	}
	rd, ok := right.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "|= right operand must be dict")
	}
	added := semantics.DictUpdateCount(ld, rd)
	if added > 0 {
		if errObj := chargeMemoryAt(tok, object.CostDictEntry()*int64(added)); errObj != nil {
			return errObj
		}
	}
	semantics.DictUpdate(ld, rd)
	return ld
}

func clamp(x, lo, hi int64) int64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func normIndex(idx, length int64) int64 {
	if idx < 0 {
		return length + idx
	}
	return idx
}

func normSliceBounds(low *int64, high *int64, step int64, length int64) (int64, int64) {
	if step > 0 {
		lo := int64(0)
		hi := length
		if low != nil {
			lo = normIndex(*low, length)
		}
		if high != nil {
			hi = normIndex(*high, length)
		}
		lo = clamp(lo, 0, length)
		hi = clamp(hi, 0, length)
		if lo > hi {
			lo = hi
		}
		return lo, hi
	}

	lo := length - 1
	hi := int64(-1)
	if low != nil {
		lo = normIndex(*low, length)
	}
	if high != nil {
		hi = normIndex(*high, length)
	}
	lo = clamp(lo, -1, length-1)
	hi = clamp(hi, -1, length-1)
	if lo < hi {
		lo = hi
	}
	return lo, hi
}

func evalSliceExpression(tok token.Token, left object.Object, low object.Object, high object.Object, step object.Object) object.Object {
	var lowPtr *int64
	var highPtr *int64
	stepVal := int64(1)

	if low != nil {
		i, ok := low.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "slice low must be INTEGER, got: "+string(low.Type()))
		}
		v := i.Value
		lowPtr = &v
	}

	if high != nil {
		i, ok := high.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "slice high must be INTEGER, got: "+string(high.Type()))
		}
		v := i.Value
		highPtr = &v
	}

	if step != nil {
		i, ok := step.(*object.Integer)
		if !ok {
			return newErrorAt(tok, "slice step must be INTEGER, got: "+string(step.Type()))
		}
		if i.Value == 0 {
			return newErrorAt(tok, "slice step cannot be 0")
		}
		stepVal = i.Value
	}

	switch v := left.(type) {
	case *object.Array:
		n := int64(len(v.Elements))
		lo, hi := normSliceBounds(lowPtr, highPtr, stepVal, n)
		out := make([]object.Object, 0)
		if stepVal > 0 {
			for i := lo; i < hi; i += stepVal {
				out = append(out, v.Elements[int(i)])
			}
		} else {
			for i := lo; i > hi; i += stepVal {
				out = append(out, v.Elements[int(i)])
			}
		}
		if errObj := chargeMemoryAt(tok, object.CostArray(len(out))); errObj != nil {
			return errObj
		}
		return &object.Array{Elements: out}
	case *object.String:
		rs := []rune(v.Value)
		n := int64(len(rs))
		lo, hi := normSliceBounds(lowPtr, highPtr, stepVal, n)
		buf := make([]rune, 0)
		if stepVal > 0 {
			for i := lo; i < hi; i += stepVal {
				buf = append(buf, rs[int(i)])
			}
		} else {
			for i := lo; i > hi; i += stepVal {
				buf = append(buf, rs[int(i)])
			}
		}
		out := &object.String{Value: string(buf)}
		if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
			return errObj
		}
		return out
	default:
		return newErrorAt(tok, "slicing not supported on type: "+string(left.Type()))
	}
}

func applyMethod(tok token.Token, recv object.Object, name string, args []object.Object) object.Object {
	if name == "get" && recv.Type() != object.DICT_OBJ {
		return newErrorAt(tok, "get() receiver must be DICT")
	}
	switch recv.Type() {
	case object.ARRAY_OBJ:
		switch name {
		case "append":
			return builtinAppend(tok, recv, args...)
		case "count":
			return builtinArrayCount(tok, recv, args...)
		case "len":
			return builtinLen(tok, recv, args...)
		case "pop":
			return builtinArrayPop(tok, recv, args...)
		case "remove":
			return builtinArrayRemove(tok, recv, args...)
		default:
			return newErrorAt(tok, "unknown method for ARRAY: "+name)
		}
	case object.DICT_OBJ:
		switch name {
		case "count":
			return builtinDictCount(tok, recv, args...)
		case "get":
			return builtinDictGet(tok, recv, args...)
		case "keys":
			return builtinKeys(tok, recv, args...)
		case "pop":
			return builtinDictPop(tok, recv, args...)
		case "remove":
			return builtinDictRemove(tok, recv, args...)
		case "values":
			return builtinValues(tok, recv, args...)
		case "hasKey":
			return builtinHasKey(tok, recv, args...)
		default:
			return newErrorAt(tok, "unknown method for DICT: "+name)
		}
	case object.STRING_OBJ:
		switch name {
		case "len":
			return builtinLen(tok, recv, args...)
		case "strip":
			return builtinStrip(tok, recv, args...)
		case "uppercase":
			return builtinUppercase(tok, recv, args...)
		case "lowercase":
			return builtinLowercase(tok, recv, args...)
		case "capitalize":
			return builtinCapitalize(tok, recv, args...)
		case "startswith":
			return builtinStartsWith(tok, recv, args...)
		case "endswith":
			return builtinEndsWith(tok, recv, args...)
		case "slice":
			return builtinSlice(tok, recv, args...)
		default:
			return newErrorAt(tok, "unknown method for STRING: "+name)
		}
	case object.INTEGER_OBJ, object.FLOAT_OBJ:
		switch name {
		case "format":
			return builtinFormatNumber(tok, recv, args...)
		default:
			return newErrorAt(tok, "unknown method for "+string(recv.Type())+": "+name)
		}
	}

	return newErrorAt(tok, "type has no methods: "+string(recv.Type()))
}

func applyFunction(tok token.Token, fn object.Object, args []object.Object, r *Runner) object.Object {
	if isError(fn) {
		return fn
	}

	switch f := fn.(type) {
	case *object.Function:
		if r != nil && r.maxRecursion > 0 {
			if r.recursion+1 > r.maxRecursion {
				return newErrorAt(tok, fmt.Sprintf("max recursion depth exceeded (%d)", r.maxRecursion))
			}
			r.recursion++
			defer func() { r.recursion-- }()
		}
		fnName := f.Name
		if fnName == "" {
			fnName = "<anon>"
		}
		ctx.Stack = append(ctx.Stack, stackFrame{
			Func: fnName,
			File: ctx.File,
			Line: tok.Line,
			Col:  tok.Col,
		})
		defer func() { ctx.Stack = ctx.Stack[:len(ctx.Stack)-1] }()

		prevFile := ctx.File
		if f.File != "" {
			ctx.File = f.File
		}
		defer func() { ctx.File = prevFile }()

		extended := object.NewEnclosedEnvironment(f.Env)

		if len(args) != len(f.Parameters) {
			return newErrorAt(tok, fmt.Sprintf(
				"wrong number of arguments: expected %d, got %d",
				len(f.Parameters), len(args),
			))
		}

		pushFrame()
		deferFramePopped := false
		defer func() {
			if !deferFramePopped {
				popFrame()
			}
		}()

		for i, p := range f.Parameters {
			extended.Set(p.Value, args[i])
		}

		evaluated := eval(f.Body, extended, r, 0, 0)
		frame := popFrame()
		deferFramePopped = true
		if dres := runDefers(frame, extended); dres != nil {
			return dres
		}
		return unwrapReturnValue(evaluated)

	case *object.Builtin:
		if f == builtinMap {
			return applyBuiltinMap(tok, args, r)
		}
		res := f.Fn(args...)
		if errObj, ok := res.(*object.Error); ok && errObj.Stack == "" {
			if !errObj.IsValue {
				if memErr := chargeMemoryAt(tok, object.CostError()); memErr != nil {
					return memErr
				}
			}
			frames := make([]stackFrame, 0, len(ctx.Stack)+1)
			frames = append(frames, ctx.Stack...)
			frames = append(frames, stackFrame{
				Func: "<main>",
				File: ctx.File,
				Line: tok.Line,
				Col:  tok.Col,
			})
			errObj.Stack = formatStackTrace(errObj.Message, frames)
		}
		return res
	}

	return newErrorAt(tok, "attempted to call non-function: "+string(fn.Type()))
}

func applyBuiltinMap(tok token.Token, args []object.Object, r *Runner) object.Object {
	if len(args) != 2 {
		return newErrorAt(tok, fmt.Sprintf("wrong number of arguments: expected 2, got %d", len(args)))
	}
	fn := args[0]
	arr, ok := args[1].(*object.Array)
	if !ok {
		return newErrorAt(tok, "map() second argument must be ARRAY")
	}
	switch fn.(type) {
	case *object.Function, *object.Builtin:
	default:
		return newErrorAt(tok, "map() first argument must be FUNCTION")
	}

	out := make([]object.Object, len(arr.Elements))
	for i, el := range arr.Elements {
		res := applyFunction(tok, fn, []object.Object{el}, r)
		if isError(res) {
			return res
		}
		out[i] = res
	}
	if errObj := chargeMemoryAt(tok, object.CostArray(len(out))); errObj != nil {
		return errObj
	}
	return &object.Array{Elements: out}
}

func unwrapReturnValue(obj object.Object) object.Object {
	if rv, ok := obj.(*object.ReturnValue); ok {
		return rv.Value
	}
	return obj
}

func builtinLen(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("len() takes 0 arguments, got %d", len(args)))
	}
	switch v := recv.(type) {
	case *object.String:
		return &object.Integer{Value: int64(utf8.RuneCountInString(v.Value))}
	case *object.Array:
		return &object.Integer{Value: int64(len(v.Elements))}
	case *object.Dict:
		return &object.Integer{Value: int64(len(v.Pairs))}
	default:
		return newErrorAt(tok, "len() not supported for type: "+string(recv.Type()))
	}
}

func builtinAppend(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("append() takes 1 argument, got %d", len(args)))
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return newErrorAt(tok, "append() receiver must be ARRAY")
	}
	els := make([]object.Object, 0, len(arr.Elements)+1)
	els = append(els, arr.Elements...)
	els = append(els, args[0])
	if errObj := chargeMemoryAt(tok, object.CostArray(len(els))); errObj != nil {
		return errObj
	}
	return &object.Array{Elements: els}
}

func builtinArrayCount(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("count() takes 1 argument, got %d", len(args)))
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return newErrorAt(tok, "count() receiver must be ARRAY")
	}
	target := args[0]
	var count int64
	for _, el := range arr.Elements {
		eq, err := semantics.Compare("==", el, target)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		if eq {
			count++
		}
	}
	return &object.Integer{Value: count}
}

func builtinArrayPop(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("pop() takes 0 arguments, got %d", len(args)))
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return newErrorAt(tok, "pop() receiver must be ARRAY")
	}
	if len(arr.Elements) == 0 {
		return newErrorAt(tok, "pop from empty array")
	}
	last := arr.Elements[len(arr.Elements)-1]
	arr.Elements = arr.Elements[:len(arr.Elements)-1]
	return last
}

func builtinArrayRemove(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("remove() takes 1 argument, got %d", len(args)))
	}
	arr, ok := recv.(*object.Array)
	if !ok {
		return newErrorAt(tok, "remove() receiver must be ARRAY")
	}
	target := args[0]
	for i, el := range arr.Elements {
		eq, err := semantics.Compare("==", el, target)
		if err != nil {
			return newErrorAt(tok, err.Error())
		}
		if eq {
			arr.Elements = append(arr.Elements[:i], arr.Elements[i+1:]...)
			return TRUE
		}
	}
	return FALSE
}

func builtinKeys(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("keys() takes 0 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "keys() receiver must be DICT")
	}
	pairs := object.SortedDictPairs(d)
	els := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		els = append(els, pair.Key)
	}
	if errObj := chargeMemoryAt(tok, object.CostArray(len(els))); errObj != nil {
		return errObj
	}
	return &object.Array{Elements: els}
}

func builtinDictCount(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("count() takes 0 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "count() receiver must be DICT")
	}
	return &object.Integer{Value: int64(len(d.Pairs))}
}

func builtinDictGet(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return newErrorAt(tok, fmt.Sprintf("get() takes 1 or 2 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "get() receiver must be DICT")
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return newErrorAt(tok, "unusable as dict key: "+string(args[0].Type()))
	}
	if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
		return pair.Value
	}
	if len(args) == 2 {
		return args[1]
	}
	return NIL
}

func builtinDictPop(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 && len(args) != 2 {
		return newErrorAt(tok, fmt.Sprintf("pop() takes 1 or 2 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "pop() receiver must be DICT")
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return newErrorAt(tok, "unusable as dict key: "+string(args[0].Type()))
	}
	key := object.HashKeyString(hk)
	if pair, exists := d.Pairs[key]; exists {
		delete(d.Pairs, key)
		return pair.Value
	}
	if len(args) == 2 {
		return args[1]
	}
	return newErrorAt(tok, "key not found")
}

func builtinDictRemove(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("remove() takes 1 argument, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "remove() receiver must be DICT")
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return newErrorAt(tok, "unusable as dict key: "+string(args[0].Type()))
	}
	key := object.HashKeyString(hk)
	if _, exists := d.Pairs[key]; !exists {
		return newErrorAt(tok, "key not found")
	}
	delete(d.Pairs, key)
	return NIL
}

func builtinValues(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("values() takes 0 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "values() receiver must be DICT")
	}
	pairs := object.SortedDictPairs(d)
	els := make([]object.Object, 0, len(pairs))
	for _, pair := range pairs {
		els = append(els, pair.Value)
	}
	if errObj := chargeMemoryAt(tok, object.CostArray(len(els))); errObj != nil {
		return errObj
	}
	return &object.Array{Elements: els}
}

func builtinHasKey(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("hasKey() takes 1 argument, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "hasKey() receiver must be DICT")
	}
	hk, ok := object.HashKeyOf(args[0])
	if !ok {
		return newErrorAt(tok, "unusable as dict key: "+string(args[0].Type()))
	}
	_, exists := d.Pairs[object.HashKeyString(hk)]
	return nativeBool(exists)
}

func builtinStrip(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("strip() takes 0 arguments, got %d", len(args)))
	}
	s := recv.(*object.String)
	out := &object.String{Value: strings.TrimSpace(s.Value)}
	if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
		return errObj
	}
	return out
}

func builtinUppercase(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("uppercase() takes 0 arguments, got %d", len(args)))
	}
	s := recv.(*object.String)
	out := &object.String{Value: strings.ToUpper(s.Value)}
	if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
		return errObj
	}
	return out
}

func builtinLowercase(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("lowercase() takes 0 arguments, got %d", len(args)))
	}
	s := recv.(*object.String)
	out := &object.String{Value: strings.ToLower(s.Value)}
	if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
		return errObj
	}
	return out
}

func builtinCapitalize(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("capitalize() takes 0 arguments, got %d", len(args)))
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
	out := &object.String{Value: first + rest}
	if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
		return errObj
	}
	return out
}

func builtinStartsWith(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("startswith() takes 1 argument, got %d", len(args)))
	}
	prefix, ok := args[0].(*object.String)
	if !ok {
		return newErrorAt(tok, "startswith() prefix must be STRING")
	}
	s := recv.(*object.String)
	return nativeBool(strings.HasPrefix(s.Value, prefix.Value))
}

func builtinEndsWith(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("endswith() takes 1 argument, got %d", len(args)))
	}
	suffix, ok := args[0].(*object.String)
	if !ok {
		return newErrorAt(tok, "endswith() suffix must be STRING")
	}
	s := recv.(*object.String)
	return nativeBool(strings.HasSuffix(s.Value, suffix.Value))
}

func builtinSlice(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) > 2 {
		return newErrorAt(tok, fmt.Sprintf("slice() takes 0, 1, or 2 arguments, got %d", len(args)))
	}
	var low object.Object
	var high object.Object
	if len(args) >= 1 {
		low = args[0]
	}
	if len(args) == 2 {
		high = args[1]
	}
	return evalSliceExpression(tok, recv, low, high, nil)
}

func builtinFormatNumber(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 1 {
		return newErrorAt(tok, fmt.Sprintf("format() takes 1 argument, got %d", len(args)))
	}
	decObj, ok := args[0].(*object.Integer)
	if !ok {
		return newErrorAt(tok, "format() decimals must be INTEGER")
	}
	if decObj.Value < 0 {
		return newErrorAt(tok, "format() decimals must be >= 0")
	}
	decimals := int(decObj.Value)

	switch v := recv.(type) {
	case *object.Integer:
		out := &object.String{Value: formatIntFixed(v.Value, decimals)}
		if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
			return errObj
		}
		return out
	case *object.Float:
		out := &object.String{Value: formatFloatFixed(v.Value, decimals)}
		if errObj := chargeMemoryAt(tok, object.CostStringBytes(len(out.Value))); errObj != nil {
			return errObj
		}
		return out
	default:
		return newErrorAt(tok, "format() receiver must be NUMBER")
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

func nativeBool(b bool) object.Object {
	if b {
		return TRUE
	}
	return FALSE
}

func isTruthy(obj object.Object) bool {
	return semantics.IsTruthy(obj)
}

func isReturn(obj object.Object) bool {
	return obj != nil && obj.Type() == object.RETURN_VALUE_OBJ
}

func isBreak(obj object.Object) bool {
	return obj != nil && obj.Type() == object.BREAK_OBJ
}

func isContinue(obj object.Object) bool {
	return obj != nil && obj.Type() == object.CONTINUE_OBJ
}

func newError(msg string) object.Object {
	if errObj := chargeMemory(object.CostError()); errObj != nil {
		return errObj
	}
	e := &object.Error{
		Message: msg,
	}
	e.Stack = formatStackTrace(msg, ctx.Stack)
	return e
}

func newErrorAt(tok token.Token, msg string) object.Object {
	if errObj := chargeMemoryAt(tok, object.CostError()); errObj != nil {
		return errObj
	}
	e := &object.Error{
		Message: msg,
	}
	frames := make([]stackFrame, 0, len(ctx.Stack)+1)
	frames = append(frames, ctx.Stack...)
	frames = append(frames, stackFrame{
		Func: "<main>",
		File: ctx.File,
		Line: tok.Line,
		Col:  tok.Col,
	})
	e.Stack = formatStackTrace(msg, frames)
	return e
}

func wrapThrownValue(tok token.Token, val object.Object) object.Object {
	if errObj, ok := val.(*object.Error); ok {
		out := errObj
		if errObj.IsValue {
			if memErr := chargeMemoryAt(tok, object.CostError()); memErr != nil {
				return memErr
			}
			out = &object.Error{
				Message: errObj.Message,
				Code:    errObj.Code,
				Stack:   errObj.Stack,
			}
		}
		if out.Stack == "" {
			frames := make([]stackFrame, 0, len(ctx.Stack)+1)
			frames = append(frames, ctx.Stack...)
			frames = append(frames, stackFrame{
				Func: "<main>",
				File: ctx.File,
				Line: tok.Line,
				Col:  tok.Col,
			})
			out.Stack = formatStackTrace(out.Message, frames)
		}
		return out
	}

	switch v := val.(type) {
	case *object.String:
		return newErrorAt(tok, v.Value)
	default:
		return newErrorAt(tok, val.Inspect())
	}
}

func isError(obj object.Object) bool {
	if obj == nil || obj.Type() != object.ERROR_OBJ {
		return false
	}
	if errObj, ok := obj.(*object.Error); ok && errObj.IsValue {
		return false
	}
	return true
}

func formatStackTrace(message string, frames []stackFrame) string {
	out := "error: " + message + "\nstack trace:\n"
	for i := len(frames) - 1; i >= 0; i-- {
		f := frames[i]
		name := f.Func
		if name == "" {
			name = "<anon>"
		}
		file := f.File
		if file == "" {
			file = "<unknown>"
		}
		out += fmt.Sprintf("  at %s (%s:%d:%d)\n", name, file, f.Line, f.Col)
	}
	return out
}
