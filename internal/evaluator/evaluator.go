package evaluator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"welle/internal/ast"
	"welle/internal/object"
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
		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}

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
			l.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: index, Value: val}
			return val

		case *object.String:
			return newErrorAt(idx.Token, "cannot assign into STRING (immutable)")

		default:
			return newErrorAt(idx.Token, "index assignment not supported on type: "+string(left.Type()))
		}

	case *ast.MemberAssignStatement:
		obj := eval(n.Object, env, r, loopDepth, switchDepth)
		if isError(obj) {
			return obj
		}
		val := eval(n.Value, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
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
		val := eval(n.ReturnValue, env, r, loopDepth, switchDepth)
		if isError(val) {
			return val
		}
		if isReturn(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

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
		return newErrorAt(n.Token, val.Inspect())

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
		env.Set(n.Name.Value, fn)
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
		return &object.String{Value: n.Value}

	case *ast.BooleanLiteral:
		return nativeBool(n.Value)

	case *ast.NilLiteral:
		return NIL

	case *ast.ListLiteral:
		els := evalExpressions(n.Elements, env, r, loopDepth, switchDepth)
		if len(els) == 1 && isError(els[0]) {
			return els[0]
		}
		return &object.Array{Elements: els}

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
		return evalSliceExpression(n.Token, left, lowObj, highObj)

	case *ast.PrefixExpression:
		right := eval(n.Right, env, r, loopDepth, switchDepth)
		if isError(right) {
			return right
		}
		return evalPrefix(n.Token, n.Operator, right)

	case *ast.InfixExpression:
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

		return newErrorAt(n.Token, "member access not supported on type: "+string(obj.Type()))

	case *ast.CallExpression:
		if me, ok := n.Function.(*ast.MemberExpression); ok {
			recv := eval(me.Object, env, r, loopDepth, switchDepth)
			if isError(recv) {
				return recv
			}
			args := evalExpressions(n.Arguments, env, r, loopDepth, switchDepth)
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
		args := evalExpressions(n.Arguments, env, r, loopDepth, switchDepth)
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
		catchEnv.Set(n.CatchName.Value, res)
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

	case *object.Dict:
		var result object.Object = NIL
		ks := make([]string, 0, len(it.Pairs))
		for k := range it.Pairs {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			pair := it.Pairs[k]
			env.Set(s.Var.Value, pair.Key)
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
			return newErrorAt(n.Token, "module has no exported member: "+name)
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
	case "not":
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

	// int-int
	if li, lok := left.(*object.Integer); lok {
		if ri, rok := right.(*object.Integer); rok {
			switch op {
			case "+":
				return &object.Integer{Value: li.Value + ri.Value}
			case "-":
				return &object.Integer{Value: li.Value - ri.Value}
			case "*":
				return &object.Integer{Value: li.Value * ri.Value}
			case "/":
				if ri.Value == 0 {
					return newErrorAt(tok, "division by zero")
				}
				return &object.Integer{Value: li.Value / ri.Value}
			case "%":
				if ri.Value == 0 {
					return newErrorAt(tok, "modulo by zero")
				}
				return &object.Integer{Value: li.Value % ri.Value}
			case "==":
				return nativeBool(li.Value == ri.Value)
			case "!=":
				return nativeBool(li.Value != ri.Value)
			case "<":
				return nativeBool(li.Value < ri.Value)
			case "<=":
				return nativeBool(li.Value <= ri.Value)
			case ">":
				return nativeBool(li.Value > ri.Value)
			case ">=":
				return nativeBool(li.Value >= ri.Value)
			default:
				return newErrorAt(tok, "unknown operator for integers: "+op)
			}
		}
	}

	// float or mixed numeric
	if isNumeric(left) && isNumeric(right) {
		lf := toFloat(left)
		rf := toFloat(right)
		switch op {
		case "+":
			return &object.Float{Value: lf + rf}
		case "-":
			return &object.Float{Value: lf - rf}
		case "*":
			return &object.Float{Value: lf * rf}
		case "/":
			if rf == 0 {
				return newErrorAt(tok, "division by zero")
			}
			return &object.Float{Value: lf / rf}
		case "%":
			return newErrorAt(tok, "modulo requires INTEGER operands")
		case "==":
			return nativeBool(lf == rf)
		case "!=":
			return nativeBool(lf != rf)
		case "<":
			return nativeBool(lf < rf)
		case "<=":
			return nativeBool(lf <= rf)
		case ">":
			return nativeBool(lf > rf)
		case ">=":
			return nativeBool(lf >= rf)
		default:
			return newErrorAt(tok, "unknown operator for numbers: "+op)
		}
	}

	// string-string
	if ls, lok := left.(*object.String); lok {
		if rs, rok := right.(*object.String); rok {
			switch op {
			case "+":
				return &object.String{Value: ls.Value + rs.Value}
			case "==":
				return nativeBool(ls.Value == rs.Value)
			case "!=":
				return nativeBool(ls.Value != rs.Value)
			default:
				return newErrorAt(tok, "unknown operator for strings: "+op)
			}
		}
	}

	// bool-bool equality
	if lb, lok := left.(*object.Boolean); lok {
		if rb, rok := right.(*object.Boolean); rok {
			switch op {
			case "==":
				return nativeBool(lb.Value == rb.Value)
			case "!=":
				return nativeBool(lb.Value != rb.Value)
			default:
				return newErrorAt(tok, "unknown operator for booleans: "+op)
			}
		}
	}

	// nil comparisons
	if left.Type() == object.NIL_OBJ && right.Type() == object.NIL_OBJ {
		switch op {
		case "==":
			return TRUE
		case "!=":
			return FALSE
		default:
			return newErrorAt(tok, "invalid operator for nil: "+op)
		}
	}
	if left.Type() == object.NIL_OBJ || right.Type() == object.NIL_OBJ {
		switch op {
		case "==":
			return FALSE
		case "!=":
			return TRUE
		default:
			return newErrorAt(tok, "cannot compare nil with "+string(nonNil(left, right).Type())+" using "+op)
		}
	}

	// mismatched types
	if left.Type() != right.Type() {
		return newErrorAt(tok, "type mismatch: "+string(left.Type())+" "+op+" "+string(right.Type()))
	}

	return newErrorAt(tok, "unknown operator: "+string(left.Type())+" "+op+" "+string(right.Type()))
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

func nonNil(a, b object.Object) object.Object {
	if a.Type() != object.NIL_OBJ {
		return a
	}
	return b
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

func evalDictLiteral(n *ast.DictLiteral, env *object.Environment, r *Runner, loopDepth int, switchDepth int) object.Object {
	pairs := make(map[string]object.DictPair, len(n.Pairs))
	for _, pair := range n.Pairs {
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
		return &object.String{Value: string(r[n])}
	}

	return newErrorAt(tok, "indexing not supported on type: "+string(left.Type()))
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

func normSliceBounds(low *int64, high *int64, length int64) (int64, int64) {
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

func evalSliceExpression(tok token.Token, left object.Object, low object.Object, high object.Object) object.Object {
	var lowPtr *int64
	var highPtr *int64

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

	switch v := left.(type) {
	case *object.Array:
		n := int64(len(v.Elements))
		lo, hi := normSliceBounds(lowPtr, highPtr, n)
		out := make([]object.Object, 0, int(hi-lo))
		for i := int(lo); i < int(hi); i++ {
			out = append(out, v.Elements[i])
		}
		return &object.Array{Elements: out}
	case *object.String:
		rs := []rune(v.Value)
		n := int64(len(rs))
		lo, hi := normSliceBounds(lowPtr, highPtr, n)
		return &object.String{Value: string(rs[int(lo):int(hi)])}
	default:
		return newErrorAt(tok, "slicing not supported on type: "+string(left.Type()))
	}
}

func applyMethod(tok token.Token, recv object.Object, name string, args []object.Object) object.Object {
	switch recv.Type() {
	case object.ARRAY_OBJ:
		switch name {
		case "append":
			return builtinAppend(tok, recv, args...)
		case "len":
			return builtinLen(tok, recv, args...)
		default:
			return newErrorAt(tok, "unknown method for ARRAY: "+name)
		}
	case object.DICT_OBJ:
		switch name {
		case "keys":
			return builtinKeys(tok, recv, args...)
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
		default:
			return newErrorAt(tok, "unknown method for STRING: "+name)
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
		return f.Fn(args...)
	}

	return newErrorAt(tok, "attempted to call non-function: "+string(fn.Type()))
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
	return &object.Array{Elements: els}
}

func builtinKeys(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("keys() takes 0 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "keys() receiver must be DICT")
	}
	ks := make([]string, 0, len(d.Pairs))
	for k := range d.Pairs {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	els := make([]object.Object, 0, len(ks))
	for _, k := range ks {
		els = append(els, d.Pairs[k].Key)
	}
	return &object.Array{Elements: els}
}

func builtinValues(tok token.Token, recv object.Object, args ...object.Object) object.Object {
	if len(args) != 0 {
		return newErrorAt(tok, fmt.Sprintf("values() takes 0 arguments, got %d", len(args)))
	}
	d, ok := recv.(*object.Dict)
	if !ok {
		return newErrorAt(tok, "values() receiver must be DICT")
	}
	ks := make([]string, 0, len(d.Pairs))
	for k := range d.Pairs {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	els := make([]object.Object, 0, len(ks))
	for _, k := range ks {
		els = append(els, d.Pairs[k].Value)
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

func nativeBool(b bool) object.Object {
	if b {
		return TRUE
	}
	return FALSE
}

func isTruthy(obj object.Object) bool {
	switch o := obj.(type) {
	case *object.Boolean:
		return o.Value
	case *object.Nil:
		return false
	default:
		return true
	}
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
	e := &object.Error{
		Message: msg,
	}
	e.Stack = formatStackTrace(msg, ctx.Stack)
	return e
}

func newErrorAt(tok token.Token, msg string) object.Object {
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
