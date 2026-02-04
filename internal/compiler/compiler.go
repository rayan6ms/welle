package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"welle/internal/ast"
	"welle/internal/code"
	"welle/internal/object"
	"welle/internal/token"
)

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
	Debug        DebugInfo
}

type SourcePos = code.SourcePos

type DebugInfo struct {
	File string
	Pos  []SourcePos
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type compilationScope struct {
	instructions    code.Instructions
	pos             []SourcePos
	lastInstruction EmittedInstruction
	prevInstruction EmittedInstruction
}

type loopContext struct {
	continueTarget int
	breakJumps     []int
	continueJumps  []int
}

type switchContext struct {
	breakJumps []int
}

type Compiler struct {
	constants  []object.Object
	symbols    *SymbolTable
	scopes     []compilationScope
	scopeIndex int
	file       string
	curLine    int
	curCol     int
	loops      []loopContext
	switches   []switchContext
	tempIndex  int
}

var builtinIndex = map[string]int{
	"print":           0,
	"len":             1,
	"str":             2,
	"keys":            3,
	"values":          4,
	"push":            5,
	"append":          5,
	"error":           6,
	"range":           7,
	"hasKey":          8,
	"sort":            9,
	"writeFile":       10,
	"math_floor":      11,
	"math_sqrt":       12,
	"math_sin":        13,
	"math_cos":        14,
	"gfx_open":        15,
	"gfx_close":       16,
	"gfx_shouldClose": 17,
	"gfx_beginFrame":  18,
	"gfx_endFrame":    19,
	"gfx_clear":       20,
	"gfx_rect":        21,
	"gfx_pixel":       22,
	"gfx_time":        23,
	"gfx_keyDown":     24,
	"gfx_mouseX":      25,
	"gfx_mouseY":      26,
}

func New() *Compiler {
	mainScope := compilationScope{instructions: code.Instructions{}}
	return &Compiler{
		constants:  []object.Object{},
		symbols:    NewSymbolTable(),
		scopes:     []compilationScope{mainScope},
		scopeIndex: 0,
	}
}

func NewWithFile(file string) *Compiler {
	c := New()
	c.file = file
	return c
}

func NewWithFileAndSymbols(file string, symbols *SymbolTable) *Compiler {
	c := NewWithFile(file)
	if symbols != nil {
		c.symbols = symbols
	}
	return c
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
		Debug: DebugInfo{
			File: c.file,
			Pos:  c.scopes[c.scopeIndex].pos,
		},
	}
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	scope := &c.scopes[c.scopeIndex]
	ins := code.Make(op, operands...)
	pos := len(scope.instructions)
	scope.instructions = append(scope.instructions, ins...)
	if c.curLine != 0 {
		scope.pos = append(scope.pos, SourcePos{
			Offset: pos,
			Line:   c.curLine,
			Col:    c.curCol,
		})
	}

	scope.prevInstruction = scope.lastInstruction
	scope.lastInstruction = EmittedInstruction{Opcode: op, Position: pos}

	return pos
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) removeLastPop() {
	scope := &c.scopes[c.scopeIndex]
	lastPos := scope.lastInstruction.Position
	scope.instructions = scope.instructions[:lastPos]
	scope.lastInstruction = scope.prevInstruction
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) enterScope() {
	c.scopes = append(c.scopes, compilationScope{instructions: code.Instructions{}})
	c.scopeIndex++
	c.symbols = NewEnclosedSymbolTable(c.symbols)
}

func (c *Compiler) leaveScope() (code.Instructions, []SourcePos) {
	scope := c.scopes[c.scopeIndex]
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbols = c.symbols.Outer
	return scope.instructions, scope.pos
}

func (c *Compiler) pushLoop(ctx loopContext) {
	c.loops = append(c.loops, ctx)
}

func (c *Compiler) popLoop() loopContext {
	ctx := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]
	return ctx
}

func (c *Compiler) currentLoop() *loopContext {
	if len(c.loops) == 0 {
		return nil
	}
	return &c.loops[len(c.loops)-1]
}

func (c *Compiler) pushSwitch() {
	c.switches = append(c.switches, switchContext{})
}

func (c *Compiler) popSwitch() switchContext {
	ctx := c.switches[len(c.switches)-1]
	c.switches = c.switches[:len(c.switches)-1]
	return ctx
}

func (c *Compiler) currentSwitch() *switchContext {
	if len(c.switches) == 0 {
		return nil
	}
	return &c.switches[len(c.switches)-1]
}

func (c *Compiler) newTempSymbol(prefix string) Symbol {
	name := fmt.Sprintf("__welle_%s_%d", prefix, c.tempIndex)
	c.tempIndex++
	return c.symbols.DefineTemp(name)
}

func (c *Compiler) setPosFromToken(tok token.Token) {
	c.curLine = tok.Line
	c.curCol = tok.Col
}

func (c *Compiler) Compile(node ast.Node) error {
	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Expression); err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.AssignStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Value); err != nil {
			return err
		}

		sym, ok := c.symbols.Resolve(n.Name.Value)
		if !ok {
			sym = c.symbols.Define(n.Name.Value)
		}

		switch sym.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, sym.Index)
			c.emit(code.OpGetGlobal, sym.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, sym.Index)
			c.emit(code.OpGetLocal, sym.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
		}

	case *ast.ImportStatement:
		c.setPosFromToken(n.Token)
		pathIdx := c.addConstant(&object.String{Value: n.Path.Value})
		c.emit(code.OpImportModule, pathIdx)

		name := ""
		if n.Alias != nil {
			name = n.Alias.Value
		} else {
			base := filepath.Base(n.Path.Value)
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}

		sym, ok := c.symbols.Resolve(name)
		if !ok {
			sym = c.symbols.Define(name)
		}

		switch sym.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, sym.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, sym.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
		}

	case *ast.FromImportStatement:
		c.setPosFromToken(n.Token)
		pathIdx := c.addConstant(&object.String{Value: n.Path.Value})
		for _, it := range n.Items {
			nameIdx := c.addConstant(&object.String{Value: it.Name.Value})
			c.emit(code.OpImportFrom, pathIdx, nameIdx)

			bind := it.Name.Value
			if it.Alias != nil {
				bind = it.Alias.Value
			}

			sym, ok := c.symbols.Resolve(bind)
			if !ok {
				sym = c.symbols.Define(bind)
			}

			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
		}

	case *ast.ExportStatement:
		c.setPosFromToken(n.Token)
		switch s := n.Stmt.(type) {
		case *ast.AssignStatement:
			if err := c.Compile(s.Value); err != nil {
				return err
			}

			sym, ok := c.symbols.Resolve(s.Name.Value)
			if !ok {
				sym = c.symbols.Define(s.Name.Value)
			}

			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}

			nameIdx := c.addConstant(&object.String{Value: s.Name.Value})
			c.emit(code.OpExport, nameIdx)

		case *ast.FuncStatement:
			if err := c.Compile(s); err != nil {
				return err
			}

			sym, ok := c.symbols.Resolve(s.Name.Value)
			if !ok {
				return fmt.Errorf("exported function not defined: %s", s.Name.Value)
			}

			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}

			nameIdx := c.addConstant(&object.String{Value: s.Name.Value})
			c.emit(code.OpExport, nameIdx)

		default:
			return fmt.Errorf("export supports only assignments and function declarations")
		}

	case *ast.IndexAssignStatement:
		c.setPosFromToken(n.Token)
		idx, ok := n.Left.(*ast.IndexExpression)
		if !ok {
			return fmt.Errorf("index assignment expects index expression on left")
		}
		if err := c.Compile(idx.Left); err != nil {
			return err
		}
		if err := c.Compile(idx.Index); err != nil {
			return err
		}
		if err := c.Compile(n.Value); err != nil {
			return err
		}
		c.emit(code.OpSetIndex)

	case *ast.MemberAssignStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Object); err != nil {
			return err
		}
		if err := c.Compile(n.Value); err != nil {
			return err
		}
		nameIdx := c.addConstant(&object.String{Value: n.Property.Value})
		c.emit(code.OpSetMember, nameIdx)

	case *ast.ReturnStatement:
		c.setPosFromToken(n.Token)
		if n.ReturnValue == nil {
			c.emit(code.OpReturn)
			return nil
		}
		if err := c.Compile(n.ReturnValue); err != nil {
			return err
		}
		c.emit(code.OpReturnValue)

	case *ast.DeferStatement:
		c.setPosFromToken(n.Token)
		ce, ok := n.Call.(*ast.CallExpression)
		if !ok {
			return fmt.Errorf("defer expects call expression")
		}
		if err := c.Compile(ce.Function); err != nil {
			return err
		}
		for _, arg := range ce.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}
		c.emit(code.OpDefer, len(ce.Arguments))

	case *ast.ThrowStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Value); err != nil {
			return err
		}
		c.emit(code.OpThrow)

	case *ast.IntegerLiteral:
		c.setPosFromToken(n.Token)
		idx := c.addConstant(&object.Integer{Value: n.Value})
		c.emit(code.OpConstant, idx)
	case *ast.FloatLiteral:
		c.setPosFromToken(n.Token)
		idx := c.addConstant(&object.Float{Value: n.Value})
		c.emit(code.OpConstant, idx)

	case *ast.BooleanLiteral:
		c.setPosFromToken(n.Token)
		if n.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.NilLiteral:
		c.setPosFromToken(n.Token)
		c.emit(code.OpNull)

	case *ast.StringLiteral:
		c.setPosFromToken(n.Token)
		idx := c.addConstant(&object.String{Value: n.Value})
		c.emit(code.OpConstant, idx)

	case *ast.ListLiteral:
		c.setPosFromToken(n.Token)
		for _, el := range n.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(n.Elements))

	case *ast.DictLiteral:
		c.setPosFromToken(n.Token)
		for _, pair := range n.Pairs {
			if err := c.Compile(pair.Key); err != nil {
				return err
			}
			if err := c.Compile(pair.Value); err != nil {
				return err
			}
		}
		c.emit(code.OpDict, len(n.Pairs))

	case *ast.PrefixExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Right); err != nil {
			return err
		}
		switch n.Operator {
		case "-":
			c.emit(code.OpMinus)
		case "!", "not":
			c.emit(code.OpBang)
		default:
			return fmt.Errorf("unknown prefix operator: %s", n.Operator)
		}

	case *ast.InfixExpression:
		c.setPosFromToken(n.Token)
		if n.Operator == "<" {
			if err := c.Compile(n.Right); err != nil {
				return err
			}
			if err := c.Compile(n.Left); err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			return nil
		}
		if n.Operator == "<=" {
			if err := c.Compile(n.Left); err != nil {
				return err
			}
			if err := c.Compile(n.Right); err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			c.emit(code.OpBang)
			return nil
		}
		if n.Operator == ">=" {
			if err := c.Compile(n.Right); err != nil {
				return err
			}
			if err := c.Compile(n.Left); err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			c.emit(code.OpBang)
			return nil
		}

		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if err := c.Compile(n.Right); err != nil {
			return err
		}

		switch n.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "%":
			c.emit(code.OpMod)
		case ">":
			c.emit(code.OpGreaterThan)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		default:
			return fmt.Errorf("unknown infix operator: %s", n.Operator)
		}

	case *ast.IndexExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if err := c.Compile(n.Index); err != nil {
			return err
		}
		c.emit(code.OpIndex)

	case *ast.MemberExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Object); err != nil {
			return err
		}
		nameIdx := c.addConstant(&object.String{Value: n.Property.Value})
		c.emit(code.OpGetMember, nameIdx)

	case *ast.SliceExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Left); err != nil {
			return err
		}
		if n.Low != nil {
			if err := c.Compile(n.Low); err != nil {
				return err
			}
		} else {
			c.emit(code.OpNull)
		}
		if n.High != nil {
			if err := c.Compile(n.High); err != nil {
				return err
			}
		} else {
			c.emit(code.OpNull)
		}
		c.emit(code.OpSlice)

	case *ast.IfStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Condition); err != nil {
			return err
		}

		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		if err := c.Compile(n.Consequence); err != nil {
			return err
		}
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}

		jmpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.replaceOperand(jntPos, afterConsequencePos)

		if n.Alternative != nil {
			if err := c.Compile(n.Alternative); err != nil {
				return err
			}
			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}
		} else {
			c.emit(code.OpNull)
		}

		afterAltPos := len(c.currentInstructions())
		c.replaceOperand(jmpPos, afterAltPos)

	case *ast.WhileStatement:
		c.setPosFromToken(n.Token)
		loopStart := len(c.currentInstructions())

		if err := c.Compile(n.Condition); err != nil {
			return err
		}
		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		c.pushLoop(loopContext{continueTarget: loopStart})
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		c.emit(code.OpJump, loopStart)

		afterLoopPos := len(c.currentInstructions())
		c.replaceOperand(jntPos, afterLoopPos)

		ctx := c.popLoop()
		for _, bp := range ctx.breakJumps {
			c.replaceOperand(bp, afterLoopPos)
		}
		for _, cp := range ctx.continueJumps {
			c.replaceOperand(cp, ctx.continueTarget)
		}

	case *ast.ForStatement:
		c.setPosFromToken(n.Token)
		if n.Init != nil {
			if err := c.Compile(n.Init); err != nil {
				return err
			}
			c.emit(code.OpPop)
		}

		loopStart := len(c.currentInstructions())

		if n.Cond != nil {
			if err := c.Compile(n.Cond); err != nil {
				return err
			}
		} else {
			c.emit(code.OpTrue)
		}

		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		c.pushLoop(loopContext{continueTarget: -1})
		if err := c.Compile(n.Body); err != nil {
			return err
		}

		postStart := len(c.currentInstructions())
		if n.Post != nil {
			if err := c.Compile(n.Post); err != nil {
				return err
			}
			c.emit(code.OpPop)
		}

		if loop := c.currentLoop(); loop != nil {
			loop.continueTarget = postStart
		}

		c.emit(code.OpJump, loopStart)

		afterLoopPos := len(c.currentInstructions())
		c.replaceOperand(jntPos, afterLoopPos)

		ctx := c.popLoop()
		for _, bp := range ctx.breakJumps {
			c.replaceOperand(bp, afterLoopPos)
		}
		for _, cp := range ctx.continueJumps {
			target := ctx.continueTarget
			if target < 0 {
				target = loopStart
			}
			c.replaceOperand(cp, target)
		}

	case *ast.SwitchStatement:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Value); err != nil {
			return err
		}

		tmp := c.newTempSymbol("switch")
		switch tmp.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, tmp.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, tmp.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", tmp.Scope)
		}

		emitGetTmp := func() {
			switch tmp.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, tmp.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, tmp.Index)
			}
		}

		c.pushSwitch()
		endJumps := []int{}

		for _, cs := range n.Cases {
			matchJumps := []int{}

			for _, v := range cs.Values {
				emitGetTmp()
				if err := c.Compile(v); err != nil {
					return err
				}
				c.emit(code.OpEqual)

				jntPos := c.emit(code.OpJumpNotTruthy, 9999)
				matchJumps = append(matchJumps, c.emit(code.OpJump, 9999))
				nextCheckPos := len(c.currentInstructions())
				c.replaceOperand(jntPos, nextCheckPos)
			}

			jumpNextCase := c.emit(code.OpJump, 9999)

			bodyPos := len(c.currentInstructions())
			for _, j := range matchJumps {
				c.replaceOperand(j, bodyPos)
			}

			if err := c.Compile(cs.Body); err != nil {
				return err
			}
			endJumps = append(endJumps, c.emit(code.OpJump, 9999))

			nextCasePos := len(c.currentInstructions())
			c.replaceOperand(jumpNextCase, nextCasePos)
		}

		if n.Default != nil {
			if err := c.Compile(n.Default); err != nil {
				return err
			}
		}

		endPos := len(c.currentInstructions())
		for _, j := range endJumps {
			c.replaceOperand(j, endPos)
		}
		ctx := c.popSwitch()
		for _, bj := range ctx.breakJumps {
			c.replaceOperand(bj, endPos)
		}

	case *ast.MatchExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Value); err != nil {
			return err
		}

		tmp := c.newTempSymbol("match")
		switch tmp.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, tmp.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, tmp.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", tmp.Scope)
		}

		emitGetTmp := func() {
			switch tmp.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, tmp.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, tmp.Index)
			}
		}

		endJumps := []int{}

		for _, cs := range n.Cases {
			for _, v := range cs.Values {
				emitGetTmp()
				if err := c.Compile(v); err != nil {
					return err
				}
				c.emit(code.OpEqual)

				jntPos := c.emit(code.OpJumpNotTruthy, 9999)
				if err := c.Compile(cs.Result); err != nil {
					return err
				}
				endJumps = append(endJumps, c.emit(code.OpJump, 9999))

				nextCheckPos := len(c.currentInstructions())
				c.replaceOperand(jntPos, nextCheckPos)
			}
		}

		if n.Default != nil {
			if err := c.Compile(n.Default); err != nil {
				return err
			}
		} else {
			c.emit(code.OpNull)
		}

		endPos := len(c.currentInstructions())
		for _, j := range endJumps {
			c.replaceOperand(j, endPos)
		}

	case *ast.BlockStatement:
		c.setPosFromToken(n.Token)
		for _, s := range n.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *ast.ForInStatement:
		return fmt.Errorf("compile not supported for node: %T", node)

	case *ast.TryStatement:
		c.setPosFromToken(n.Token)
		const noCatch = 0xFFFF

		tryPos := c.emit(code.OpTry, noCatch)
		finallyPos := -1
		if n.FinallyBlock != nil {
			finallyPos = c.emit(code.OpTryFinally, 9999, 9999)
		}

		if err := c.Compile(n.TryBlock); err != nil {
			return err
		}
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}
		c.emit(code.OpEndTry)

		jumpAfterTry := -1
		if n.CatchBlock != nil || n.FinallyBlock != nil {
			jumpAfterTry = c.emit(code.OpJump, 9999)
		}

		jumpAfterCatch := -1
		if n.CatchBlock != nil {
			catchPos := len(c.currentInstructions())
			c.replaceOperands(tryPos, catchPos)

			sym, ok := c.symbols.Resolve(n.CatchName.Value)
			if !ok {
				sym = c.symbols.Define(n.CatchName.Value)
			}
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}

			if err := c.Compile(n.CatchBlock); err != nil {
				return err
			}
			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			}

			if n.FinallyBlock != nil {
				jumpAfterCatch = c.emit(code.OpJump, 9999)
			} else {
				afterCatchPos := len(c.currentInstructions())
				c.replaceOperands(jumpAfterTry, afterCatchPos)
				return nil
			}
		} else {
			c.replaceOperands(tryPos, noCatch)
		}

		if n.FinallyBlock == nil {
			return nil
		}

		finallyStart := len(c.currentInstructions())
		c.replaceOperands(finallyPos, finallyStart, 9999)
		if jumpAfterTry != -1 {
			c.replaceOperands(jumpAfterTry, finallyStart)
		}
		if jumpAfterCatch != -1 {
			c.replaceOperands(jumpAfterCatch, finallyStart)
		}

		c.emit(code.OpEndFinally)
		if err := c.Compile(n.FinallyBlock); err != nil {
			return err
		}
		c.emit(code.OpRethrowPending)

		afterFinally := len(c.currentInstructions())
		c.replaceOperands(finallyPos, finallyStart, afterFinally)

	case *ast.FuncStatement:
		c.setPosFromToken(n.Token)
		compiled, freeSymbols, err := c.compileFunction(n.Name.Value, n.Parameters, n.Body)
		if err != nil {
			return err
		}

		idx := c.addConstant(compiled)
		for _, fs := range freeSymbols {
			switch fs.Scope {
			case LocalScope:
				c.emit(code.OpGetLocal, fs.Index)
			case FreeScope:
				c.emit(code.OpGetFree, fs.Index)
			case GlobalScope:
				c.emit(code.OpGetGlobal, fs.Index)
			default:
				return fmt.Errorf("unsupported free symbol scope: %s", fs.Scope)
			}
		}
		c.emit(code.OpClosure, idx, len(freeSymbols))

		sym, ok := c.symbols.Resolve(n.Name.Value)
		if !ok {
			sym = c.symbols.Define(n.Name.Value)
		}

		switch sym.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, sym.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, sym.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
		}

	case *ast.CallExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Function); err != nil {
			return err
		}
		for _, a := range n.Arguments {
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		c.emit(code.OpCall, len(n.Arguments))

	case *ast.Identifier:
		c.setPosFromToken(n.Token)
		if idx, ok := builtinIndex[n.Value]; ok {
			c.emit(code.OpGetBuiltin, idx)
			return nil
		}

		sym, ok := c.symbols.Resolve(n.Value)
		if !ok {
			return fmt.Errorf("undefined variable: %s", n.Value)
		}
		switch sym.Scope {
		case GlobalScope:
			c.emit(code.OpGetGlobal, sym.Index)
		case LocalScope:
			c.emit(code.OpGetLocal, sym.Index)
		case FreeScope:
			c.emit(code.OpGetFree, sym.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
		}

	case *ast.BreakStatement:
		c.setPosFromToken(n.Token)
		if loop := c.currentLoop(); loop != nil {
			pos := c.emit(code.OpJump, 9999)
			loop.breakJumps = append(loop.breakJumps, pos)
			return nil
		}
		if sw := c.currentSwitch(); sw != nil {
			pos := c.emit(code.OpJump, 9999)
			sw.breakJumps = append(sw.breakJumps, pos)
			return nil
		}
		return fmt.Errorf("break used outside of loop/switch")

	case *ast.ContinueStatement:
		c.setPosFromToken(n.Token)
		loop := c.currentLoop()
		if loop == nil {
			return fmt.Errorf("continue used outside of loop")
		}
		pos := c.emit(code.OpJump, 9999)
		loop.continueJumps = append(loop.continueJumps, pos)

	default:
		return fmt.Errorf("compile not supported for node: %T", node)
	}

	return nil
}

func (c *Compiler) replaceOperand(opPos int, operand int) {
	c.replaceOperands(opPos, operand)
}

func (c *Compiler) replaceOperands(opPos int, operands ...int) {
	scope := &c.scopes[c.scopeIndex]
	op := code.Opcode(scope.instructions[opPos])
	def := code.Make(op, operands...)
	for i := 0; i < len(def); i++ {
		scope.instructions[opPos+i] = def[i]
	}
}

func (c *Compiler) compileFunction(name string, params []*ast.Identifier, body *ast.BlockStatement) (*object.CompiledFunction, []Symbol, error) {
	c.enterScope()

	for _, p := range params {
		c.symbols.Define(p.Value)
	}

	if err := c.Compile(body); err != nil {
		return nil, nil, err
	}

	if !c.lastInstructionIs(code.OpReturnValue) && !c.lastInstructionIs(code.OpReturn) {
		c.emit(code.OpReturn)
	}

	numLocals := c.symbols.numDefinitions
	freeSymbols := c.symbols.FreeSymbols
	instructions, pos := c.leaveScope()
	if name == "" {
		name = "<anon>"
	}

	return &object.CompiledFunction{
		Instructions:  instructions,
		NumLocals:     numLocals,
		NumParameters: len(params),
		Name:          name,
		File:          c.file,
		Pos:           pos,
	}, freeSymbols, nil
}
