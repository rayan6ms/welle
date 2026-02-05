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
	"print":  0,
	"len":    1,
	"str":    2,
	"join":   3,
	"keys":   4,
	"values": 5,
	"push":   6,
	"append": 6,

	"count":  7,
	"remove": 8,
	"get":    9,
	"pop":    10,

	"error":     11,
	"range":     12,
	"hasKey":    13,
	"sort":      14,
	"writeFile": 15,

	"math_floor": 16,
	"math_sqrt":  17,
	"math_sin":   18,
	"math_cos":   19,

	"gfx_open":        20,
	"gfx_close":       21,
	"gfx_shouldClose": 22,
	"gfx_beginFrame":  23,
	"gfx_endFrame":    24,
	"gfx_clear":       25,
	"gfx_rect":        26,
	"gfx_pixel":       27,
	"gfx_time":        28,
	"gfx_keyDown":     29,
	"gfx_mouseX":      30,
	"gfx_mouseY":      31,
	"gfx_present":     32,

	"image_new":        33,
	"image_set":        34,
	"image_fill":       35,
	"image_width":      36,
	"image_height":     37,
	"image_fill_rect":  38,
	"image_fade":       39,
	"image_fade_white": 40,

	"max":            41,
	"abs":            42,
	"sum":            43,
	"reverse":        44,
	"any":            45,
	"all":            46,
	"map":            47,
	"mean":           48,
	"sqrt":           49,
	"input":          50,
	"getpass":        51,
	"group_digits":   52,
	"format_float":   53,
	"format_percent": 54,
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
		posTok := n.Token
		if n.OpToken.Type != "" {
			posTok = n.OpToken
		}
		c.setPosFromToken(posTok)

		op := n.Op
		if op == token.WALRUS {
			if err := c.Compile(n.Value); err != nil {
				return err
			}

			sym, ok := c.symbols.ResolveCurrent(n.Name.Value)
			if !ok {
				sym = c.symbols.Define(n.Name.Value)
			}
			nameIdx := c.addConstant(&object.String{Value: n.Name.Value})

			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpDefineGlobal, sym.Index, nameIdx)
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpDefineLocal, sym.Index, nameIdx)
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope for walrus: %s", sym.Scope)
			}
			return nil
		}
		if op == "" || op == token.ASSIGN {
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
			case FreeScope:
				c.emit(code.OpSetFree, sym.Index)
				c.emit(code.OpGetFree, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}

		opcode, ok := compoundAssignOpcode(op)
		if !ok {
			return fmt.Errorf("unsupported assignment operator: %s", op)
		}

		sym, ok := c.symbols.Resolve(n.Name.Value)
		if !ok {
			return fmt.Errorf("unknown identifier: %s", n.Name.Value)
		}

		emitGet := func() error {
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
			return nil
		}
		emitSet := func() error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			case FreeScope:
				c.emit(code.OpSetFree, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}

		if err := emitGet(); err != nil {
			return err
		}
		if err := c.Compile(n.Value); err != nil {
			return err
		}
		c.emit(opcode)
		if err := emitSet(); err != nil {
			return err
		}
		if err := emitGet(); err != nil {
			return err
		}

	case *ast.AssignExpression:
		switch left := n.Left.(type) {
		case *ast.Identifier:
			stmt := &ast.AssignStatement{
				Token:   left.Token,
				OpToken: n.Token,
				Op:      n.Op,
				Name:    left,
				Value:   n.Value,
			}
			return c.Compile(stmt)
		case *ast.IndexExpression:
			stmt := &ast.IndexAssignStatement{
				Token: n.Token,
				Op:    n.Op,
				Left:  left,
				Value: n.Value,
			}
			return c.Compile(stmt)
		case *ast.MemberExpression:
			stmt := &ast.MemberAssignStatement{
				Token:    n.Token,
				Op:       n.Op,
				Object:   left.Object,
				Property: left.Property,
				Value:    n.Value,
			}
			return c.Compile(stmt)
		default:
			return fmt.Errorf("invalid assignment target")
		}

	case *ast.DestructureAssignStatement:
		posTok := n.Token
		if n.OpToken.Type != "" {
			posTok = n.OpToken
		}
		c.setPosFromToken(posTok)
		if n.Op != "" && n.Op != token.ASSIGN {
			return fmt.Errorf("destructuring assignment supports only '='")
		}
		if err := c.Compile(n.Value); err != nil {
			return err
		}
		starIdx := -1
		for i, t := range n.Targets {
			if t != nil && t.Star {
				starIdx = i
				break
			}
		}
		if starIdx >= 0 {
			c.emit(code.OpUnpackStar, len(n.Targets), starIdx)
		} else {
			c.emit(code.OpUnpackTuple, len(n.Targets))
		}
		for i := len(n.Targets) - 1; i >= 0; i-- {
			t := n.Targets[i]
			if t == nil || t.Name == nil || t.Name.Value == "_" {
				c.emit(code.OpPop)
				continue
			}
			sym, ok := c.symbols.Resolve(t.Name.Value)
			if !ok {
				sym = c.symbols.Define(t.Name.Value)
			}
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			case FreeScope:
				c.emit(code.OpSetFree, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
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
			if err := c.Compile(s); err != nil {
				return err
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

		if n.Op == "" || n.Op == token.ASSIGN {
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
			return nil
		}

		opcode, ok := compoundAssignOpcode(n.Op)
		if !ok {
			return fmt.Errorf("unsupported assignment operator: %s", n.Op)
		}

		baseTmp := c.newTempSymbol("idx_base")
		indexTmp := c.newTempSymbol("idx_key")
		valueTmp := c.newTempSymbol("idx_val")

		emitSetTmp := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}
		emitGetTmp := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}

		if err := c.Compile(idx.Left); err != nil {
			return err
		}
		if err := emitSetTmp(baseTmp); err != nil {
			return err
		}

		if err := c.Compile(idx.Index); err != nil {
			return err
		}
		if err := emitSetTmp(indexTmp); err != nil {
			return err
		}

		if err := emitGetTmp(baseTmp); err != nil {
			return err
		}
		if err := emitGetTmp(indexTmp); err != nil {
			return err
		}
		c.emit(code.OpIndex)

		if err := c.Compile(n.Value); err != nil {
			return err
		}
		c.emit(opcode)

		if err := emitSetTmp(valueTmp); err != nil {
			return err
		}
		if err := emitGetTmp(baseTmp); err != nil {
			return err
		}
		if err := emitGetTmp(indexTmp); err != nil {
			return err
		}
		if err := emitGetTmp(valueTmp); err != nil {
			return err
		}
		c.emit(code.OpSetIndex)

	case *ast.MemberAssignStatement:
		c.setPosFromToken(n.Token)

		if n.Op == "" || n.Op == token.ASSIGN {
			if err := c.Compile(n.Object); err != nil {
				return err
			}
			if err := c.Compile(n.Value); err != nil {
				return err
			}
			nameIdx := c.addConstant(&object.String{Value: n.Property.Value})
			c.emit(code.OpSetMember, nameIdx)
			return nil
		}

		opcode, ok := compoundAssignOpcode(n.Op)
		if !ok {
			return fmt.Errorf("unsupported assignment operator: %s", n.Op)
		}

		objTmp := c.newTempSymbol("member_obj")
		valTmp := c.newTempSymbol("member_val")

		emitSetTmp := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}
		emitGetTmp := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}

		if err := c.Compile(n.Object); err != nil {
			return err
		}
		if err := emitSetTmp(objTmp); err != nil {
			return err
		}

		nameIdx := c.addConstant(&object.String{Value: n.Property.Value})
		if err := emitGetTmp(objTmp); err != nil {
			return err
		}
		c.emit(code.OpGetMember, nameIdx)

		if err := c.Compile(n.Value); err != nil {
			return err
		}
		c.emit(opcode)

		if err := emitSetTmp(valTmp); err != nil {
			return err
		}
		if err := emitGetTmp(objTmp); err != nil {
			return err
		}
		if err := emitGetTmp(valTmp); err != nil {
			return err
		}
		c.emit(code.OpSetMember, nameIdx)

	case *ast.ReturnStatement:
		c.setPosFromToken(n.Token)
		if len(n.ReturnValues) == 0 {
			c.emit(code.OpReturn)
			return nil
		}
		if len(n.ReturnValues) == 1 {
			if err := c.Compile(n.ReturnValues[0]); err != nil {
				return err
			}
			c.emit(code.OpReturnValue)
			return nil
		}
		for _, rv := range n.ReturnValues {
			if err := c.Compile(rv); err != nil {
				return err
			}
		}
		c.emit(code.OpTuple, len(n.ReturnValues))
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
		hasSpread := false
		for _, arg := range ce.Arguments {
			if _, ok := arg.(*ast.SpreadExpression); ok {
				hasSpread = true
				break
			}
		}
		for _, arg := range ce.Arguments {
			if spread, ok := arg.(*ast.SpreadExpression); ok {
				if err := c.Compile(spread.Value); err != nil {
					return err
				}
				c.emit(code.OpSpread)
				continue
			}
			if err := c.Compile(arg); err != nil {
				return err
			}
		}
		if hasSpread {
			c.emit(code.OpDeferSpread, len(ce.Arguments))
		} else {
			c.emit(code.OpDefer, len(ce.Arguments))
		}

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

	case *ast.TemplateLiteral:
		c.setPosFromToken(n.Token)
		if n.Tagged {
			if err := c.Compile(n.Tag); err != nil {
				return err
			}
			for _, part := range n.Parts {
				idx := c.addConstant(&object.String{Value: part})
				c.emit(code.OpConstant, idx)
			}
			c.emit(code.OpTuple, len(n.Parts))
			for _, ex := range n.Exprs {
				if err := c.Compile(ex); err != nil {
					return err
				}
			}
			c.emit(code.OpCall, len(n.Exprs)+1)
			return nil
		}

		if len(n.Parts) == 0 {
			idx := c.addConstant(&object.String{Value: ""})
			c.emit(code.OpConstant, idx)
			return nil
		}

		part0 := c.addConstant(&object.String{Value: n.Parts[0]})
		c.emit(code.OpConstant, part0)
		strIdx, ok := builtinIndex["str"]
		if !ok {
			return fmt.Errorf("missing builtin: str")
		}
		for i, ex := range n.Exprs {
			c.emit(code.OpGetBuiltin, strIdx)
			if err := c.Compile(ex); err != nil {
				return err
			}
			c.emit(code.OpCall, 1)
			c.emit(code.OpAdd)

			nextPart := ""
			if i+1 < len(n.Parts) {
				nextPart = n.Parts[i+1]
			}
			partIdx := c.addConstant(&object.String{Value: nextPart})
			c.emit(code.OpConstant, partIdx)
			c.emit(code.OpAdd)
		}

	case *ast.ListComprehension:
		c.setPosFromToken(n.Token)
		emitSet := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}
		emitGet := func(sym Symbol) error {
			switch sym.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, sym.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, sym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", sym.Scope)
			}
			return nil
		}

		if err := c.Compile(n.Seq); err != nil {
			return err
		}
		seqSym := c.newTempSymbol("comp_seq")
		if err := emitSet(seqSym); err != nil {
			return err
		}
		if err := emitGet(seqSym); err != nil {
			return err
		}
		c.emit(code.OpIterInitComp)

		iterSym := c.newTempSymbol("comp_iter")
		if err := emitSet(iterSym); err != nil {
			return err
		}

		c.emit(code.OpArray, 0)
		outSym := c.newTempSymbol("comp_out")
		if err := emitSet(outSym); err != nil {
			return err
		}

		loopVarSym := c.newTempSymbol("comp_var")
		prevSym, hadPrev := c.symbols.store[n.Var.Value]
		c.symbols.store[n.Var.Value] = loopVarSym
		restore := func() {
			if hadPrev {
				c.symbols.store[n.Var.Value] = prevSym
			} else {
				delete(c.symbols.store, n.Var.Value)
			}
		}

		err := func() error {
			loopStart := len(c.currentInstructions())
			if err := emitGet(iterSym); err != nil {
				return err
			}
			c.emit(code.OpIterNext)
			jntPos := c.emit(code.OpJumpNotTruthy, 9999)

			if err := emitSet(loopVarSym); err != nil {
				return err
			}

			loopContinue := -1
			if n.Filter != nil {
				if err := c.Compile(n.Filter); err != nil {
					return err
				}
				loopContinue = c.emit(code.OpJumpNotTruthy, 9999)
			}

			if err := emitGet(outSym); err != nil {
				return err
			}
			if err := c.Compile(n.Elem); err != nil {
				return err
			}
			c.emit(code.OpArrayAppend)
			c.emit(code.OpPop)

			if loopContinue != -1 {
				contPos := len(c.currentInstructions())
				c.replaceOperand(loopContinue, contPos)
			}

			c.emit(code.OpJump, loopStart)

			endPos := len(c.currentInstructions())
			c.replaceOperand(jntPos, endPos)
			c.emit(code.OpPop)

			if err := emitGet(outSym); err != nil {
				return err
			}
			return nil
		}()
		restore()
		if err != nil {
			return err
		}

	case *ast.ListLiteral:
		c.setPosFromToken(n.Token)
		for _, el := range n.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(n.Elements))

	case *ast.TupleLiteral:
		c.setPosFromToken(n.Token)
		for _, el := range n.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(code.OpTuple, len(n.Elements))

	case *ast.DictLiteral:
		c.setPosFromToken(n.Token)
		for _, pair := range n.Pairs {
			if pair.Shorthand != nil {
				keyIdx := c.addConstant(&object.String{Value: pair.Shorthand.Value})
				c.emit(code.OpConstant, keyIdx)
				if err := c.Compile(pair.Shorthand); err != nil {
					return err
				}
				continue
			}
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
		case "~":
			c.emit(code.OpBitNot)
		default:
			return fmt.Errorf("unknown prefix operator: %s", n.Operator)
		}

	case *ast.InfixExpression:
		c.setPosFromToken(n.Token)
		if n.Operator == "and" || n.Operator == "or" {
			return c.compileLogical(n.Operator, n.Left, n.Right)
		}
		if n.Operator == "??" {
			return c.compileNullish(n.Left, n.Right)
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
		case "|":
			c.emit(code.OpBitOr)
		case "&":
			c.emit(code.OpBitAnd)
		case "^":
			c.emit(code.OpBitXor)
		case "<<":
			c.emit(code.OpShl)
		case ">>":
			c.emit(code.OpShr)
		case ">":
			c.emit(code.OpGreaterThan)
		case "<":
			c.emit(code.OpLessThan)
		case "<=":
			c.emit(code.OpLessEqual)
		case ">=":
			c.emit(code.OpGreaterEqual)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		case "is":
			c.emit(code.OpIs)
		case "in":
			c.emit(code.OpIn)
		default:
			return fmt.Errorf("unknown infix operator: %s", n.Operator)
		}

	case *ast.ConditionalExpression:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Cond); err != nil {
			return err
		}
		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		if err := c.Compile(n.Then); err != nil {
			return err
		}
		jmpPos := c.emit(code.OpJump, 9999)

		elsePos := len(c.currentInstructions())
		c.replaceOperand(jntPos, elsePos)

		if err := c.Compile(n.Else); err != nil {
			return err
		}

		endPos := len(c.currentInstructions())
		c.replaceOperand(jmpPos, endPos)

	case *ast.CondExpr:
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Cond); err != nil {
			return err
		}
		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		if err := c.Compile(n.Then); err != nil {
			return err
		}
		jmpPos := c.emit(code.OpJump, 9999)

		elsePos := len(c.currentInstructions())
		c.replaceOperand(jntPos, elsePos)

		if err := c.Compile(n.Else); err != nil {
			return err
		}

		endPos := len(c.currentInstructions())
		c.replaceOperand(jmpPos, endPos)

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
		if n.Step != nil {
			if err := c.Compile(n.Step); err != nil {
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
			if _, ok := n.Init.(*ast.ExpressionStatement); !ok {
				c.emit(code.OpPop)
			}
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
			if _, ok := n.Post.(*ast.ExpressionStatement); !ok {
				c.emit(code.OpPop)
			}
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
		c.setPosFromToken(n.Token)
		if err := c.Compile(n.Iterable); err != nil {
			return err
		}
		var dictSym Symbol
		if n.Destruct {
			dictSym = c.newTempSymbol("iter_dict")
			switch dictSym.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, dictSym.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, dictSym.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", dictSym.Scope)
			}
			switch dictSym.Scope {
			case GlobalScope:
				c.emit(code.OpGetGlobal, dictSym.Index)
			case LocalScope:
				c.emit(code.OpGetLocal, dictSym.Index)
			}
			c.emit(code.OpIterInitDict)
		} else {
			c.emit(code.OpIterInit)
		}

		iterSym := c.newTempSymbol("iter")
		switch iterSym.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, iterSym.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, iterSym.Index)
		default:
			return fmt.Errorf("unsupported symbol scope: %s", iterSym.Scope)
		}

		loopStart := len(c.currentInstructions())
		switch iterSym.Scope {
		case GlobalScope:
			c.emit(code.OpGetGlobal, iterSym.Index)
		case LocalScope:
			c.emit(code.OpGetLocal, iterSym.Index)
		}
		c.emit(code.OpIterNext)
		jntPos := c.emit(code.OpJumpNotTruthy, 9999)

		if n.Destruct {
			keyTemp := c.newTempSymbol("iter_key")
			switch keyTemp.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, keyTemp.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, keyTemp.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", keyTemp.Scope)
			}

			if n.Key != nil && n.Key.Value != "_" {
				keyVar, ok := c.symbols.Resolve(n.Key.Value)
				if !ok {
					keyVar = c.symbols.Define(n.Key.Value)
				}
				switch keyTemp.Scope {
				case GlobalScope:
					c.emit(code.OpGetGlobal, keyTemp.Index)
				case LocalScope:
					c.emit(code.OpGetLocal, keyTemp.Index)
				}
				switch keyVar.Scope {
				case GlobalScope:
					c.emit(code.OpSetGlobal, keyVar.Index)
				case LocalScope:
					c.emit(code.OpSetLocal, keyVar.Index)
				case FreeScope:
					c.emit(code.OpSetFree, keyVar.Index)
				default:
					return fmt.Errorf("unsupported symbol scope: %s", keyVar.Scope)
				}
			}

			if n.Value != nil && n.Value.Value != "_" {
				switch dictSym.Scope {
				case GlobalScope:
					c.emit(code.OpGetGlobal, dictSym.Index)
				case LocalScope:
					c.emit(code.OpGetLocal, dictSym.Index)
				}
				switch keyTemp.Scope {
				case GlobalScope:
					c.emit(code.OpGetGlobal, keyTemp.Index)
				case LocalScope:
					c.emit(code.OpGetLocal, keyTemp.Index)
				}
				c.emit(code.OpIndex)

				valVar, ok := c.symbols.Resolve(n.Value.Value)
				if !ok {
					valVar = c.symbols.Define(n.Value.Value)
				}
				switch valVar.Scope {
				case GlobalScope:
					c.emit(code.OpSetGlobal, valVar.Index)
				case LocalScope:
					c.emit(code.OpSetLocal, valVar.Index)
				case FreeScope:
					c.emit(code.OpSetFree, valVar.Index)
				default:
					return fmt.Errorf("unsupported symbol scope: %s", valVar.Scope)
				}
			}
		} else {
			loopVar, ok := c.symbols.Resolve(n.Var.Value)
			if !ok {
				loopVar = c.symbols.Define(n.Var.Value)
			}
			switch loopVar.Scope {
			case GlobalScope:
				c.emit(code.OpSetGlobal, loopVar.Index)
			case LocalScope:
				c.emit(code.OpSetLocal, loopVar.Index)
			default:
				return fmt.Errorf("unsupported symbol scope: %s", loopVar.Scope)
			}
		}

		c.pushLoop(loopContext{continueTarget: loopStart})
		if err := c.Compile(n.Body); err != nil {
			return err
		}
		c.emit(code.OpJump, loopStart)

		cleanupPos := len(c.currentInstructions())
		c.replaceOperand(jntPos, cleanupPos)
		c.emit(code.OpPop)

		afterLoopPos := len(c.currentInstructions())
		ctx := c.popLoop()
		for _, bp := range ctx.breakJumps {
			c.replaceOperand(bp, afterLoopPos)
		}
		for _, cp := range ctx.continueJumps {
			c.replaceOperand(cp, ctx.continueTarget)
		}

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
				c.emit(code.OpGetLocalCell, fs.Index)
			case FreeScope:
				c.emit(code.OpGetFreeCell, fs.Index)
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

	case *ast.FunctionLiteral:
		c.setPosFromToken(n.Token)
		compiled, freeSymbols, err := c.compileFunction(ast.AnonymousFuncName(n.Token), n.Parameters, n.Body)
		if err != nil {
			return err
		}

		idx := c.addConstant(compiled)
		for _, fs := range freeSymbols {
			switch fs.Scope {
			case LocalScope:
				c.emit(code.OpGetLocalCell, fs.Index)
			case FreeScope:
				c.emit(code.OpGetFreeCell, fs.Index)
			case GlobalScope:
				c.emit(code.OpGetGlobal, fs.Index)
			default:
				return fmt.Errorf("unsupported free symbol scope: %s", fs.Scope)
			}
		}
		c.emit(code.OpClosure, idx, len(freeSymbols))

	case *ast.CallExpression:
		c.setPosFromToken(n.Token)
		if me, ok := n.Function.(*ast.MemberExpression); ok {
			if err := c.Compile(me.Object); err != nil {
				return err
			}
			hasSpread := false
			for _, a := range n.Arguments {
				if _, ok := a.(*ast.SpreadExpression); ok {
					hasSpread = true
					break
				}
			}
			for _, a := range n.Arguments {
				if spread, ok := a.(*ast.SpreadExpression); ok {
					if err := c.Compile(spread.Value); err != nil {
						return err
					}
					c.emit(code.OpSpread)
					continue
				}
				if err := c.Compile(a); err != nil {
					return err
				}
			}
			nameIdx := c.addConstant(&object.String{Value: me.Property.Value})
			if hasSpread {
				c.emit(code.OpCallMethodSpread, nameIdx, len(n.Arguments))
			} else {
				c.emit(code.OpCallMethod, nameIdx, len(n.Arguments))
			}
			return nil
		}

		if err := c.Compile(n.Function); err != nil {
			return err
		}
		hasSpread := false
		for _, a := range n.Arguments {
			if _, ok := a.(*ast.SpreadExpression); ok {
				hasSpread = true
				break
			}
		}
		for _, a := range n.Arguments {
			if spread, ok := a.(*ast.SpreadExpression); ok {
				if err := c.Compile(spread.Value); err != nil {
					return err
				}
				c.emit(code.OpSpread)
				continue
			}
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		if hasSpread {
			c.emit(code.OpCallSpread, len(n.Arguments))
		} else {
			c.emit(code.OpCall, len(n.Arguments))
		}

	case *ast.Identifier:
		c.setPosFromToken(n.Token)
		sym, ok := c.symbols.Resolve(n.Value)
		if ok {
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
			return nil
		}

		if idx, ok := builtinIndex[n.Value]; ok {
			c.emit(code.OpGetBuiltin, idx)
			return nil
		}

		return fmt.Errorf("unknown identifier: %s", n.Value)

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

	case *ast.PassStatement:
		c.setPosFromToken(n.Token)
		return nil

	default:
		return fmt.Errorf("compile not supported for node: %T", node)
	}

	return nil
}

func compoundAssignOpcode(op token.Type) (code.Opcode, bool) {
	switch op {
	case token.PLUS_ASSIGN:
		return code.OpAdd, true
	case token.MINUS_ASSIGN:
		return code.OpSub, true
	case token.STAR_ASSIGN:
		return code.OpMul, true
	case token.SLASH_ASSIGN:
		return code.OpDiv, true
	case token.PERCENT_ASSIGN:
		return code.OpMod, true
	case token.BITOR_ASSIGN:
		return code.OpDictUpdate, true
	default:
		return 0, false
	}
}

func (c *Compiler) compileLogical(op string, left, right ast.Expression) error {
	if err := c.Compile(left); err != nil {
		return err
	}

	switch op {
	case "and":
		jntLeft := c.emit(code.OpJumpNotTruthy, 9999)
		if err := c.Compile(right); err != nil {
			return err
		}
		jntRight := c.emit(code.OpJumpNotTruthy, 9999)
		c.emit(code.OpTrue)
		endJump := c.emit(code.OpJump, 9999)

		falsePos := len(c.currentInstructions())
		c.replaceOperand(jntLeft, falsePos)
		c.replaceOperand(jntRight, falsePos)
		c.emit(code.OpFalse)

		endPos := len(c.currentInstructions())
		c.replaceOperand(endJump, endPos)
		return nil

	case "or":
		jntLeft := c.emit(code.OpJumpNotTruthy, 9999)
		c.emit(code.OpTrue)
		endJumpLeft := c.emit(code.OpJump, 9999)

		rightPos := len(c.currentInstructions())
		c.replaceOperand(jntLeft, rightPos)
		if err := c.Compile(right); err != nil {
			return err
		}
		jntRight := c.emit(code.OpJumpNotTruthy, 9999)
		c.emit(code.OpTrue)
		endJumpRight := c.emit(code.OpJump, 9999)

		falsePos := len(c.currentInstructions())
		c.replaceOperand(jntRight, falsePos)
		c.emit(code.OpFalse)

		endPos := len(c.currentInstructions())
		c.replaceOperand(endJumpLeft, endPos)
		c.replaceOperand(endJumpRight, endPos)
		return nil
	}

	return fmt.Errorf("unknown logical operator: %s", op)
}

func (c *Compiler) compileNullish(left, right ast.Expression) error {
	if err := c.Compile(left); err != nil {
		return err
	}

	jumpIfNil := c.emit(code.OpJumpIfNil, 9999)
	endJump := c.emit(code.OpJump, 9999)

	rightPos := len(c.currentInstructions())
	c.replaceOperand(jumpIfNil, rightPos)
	c.emit(code.OpPop)

	if err := c.Compile(right); err != nil {
		return err
	}

	endPos := len(c.currentInstructions())
	c.replaceOperand(endJump, endPos)
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
