package vm

import (
	"errors"
	"fmt"

	"welle/internal/code"
	"welle/internal/compiler"
	"welle/internal/object"
)

const StackSize = 2048
const GlobalsSize = 65536
const MaxFrames = 1024

var nilObj = &object.Nil{}

type VM struct {
	constants []object.Object

	stack []object.Object
	sp    int

	globals    []object.Object
	lastPopped object.Object

	frames      []*Frame
	framesIndex int

	traps    []trap
	finallys []fin

	entryPath string
	importer  Importer
	modules   map[string]*object.Dict
	exports   *object.Dict

	pendingErr *object.Error
}

type trap struct {
	catchIP  int
	sp       int
	frameIdx int
}

type fin struct {
	finallyIP int
	afterIP   int
	sp        int
	frameIdx  int
}

type Importer func(fromPath, spec string) (*compiler.Bytecode, string, error)

func New(bc *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{
		Instructions: bc.Instructions,
		Name:         "<main>",
		File:         bc.Debug.File,
		Pos:          bc.Debug.Pos,
	}
	mainCl := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainCl, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants:   bc.Constants,
		stack:       make([]object.Object, StackSize),
		globals:     make([]object.Object, GlobalsSize),
		sp:          0,
		frames:      frames,
		framesIndex: 1,
		modules:     map[string]*object.Dict{},
		exports:     &object.Dict{Pairs: map[string]object.DictPair{}},
	}
}

func NewWithImporter(bc *compiler.Bytecode, entryPath string, imp Importer) *VM {
	m := New(bc)
	m.entryPath = entryPath
	m.importer = imp
	return m
}

func (m *VM) currentFrame() *Frame {
	return m.frames[m.framesIndex-1]
}

func (m *VM) pushFrame(f *Frame) {
	m.frames[m.framesIndex] = f
	m.framesIndex++
}

func (m *VM) popFrame() *Frame {
	m.framesIndex--
	f := m.frames[m.framesIndex]
	m.frames[m.framesIndex] = nil
	return f
}

func (m *VM) push(o object.Object) error {
	if m.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	m.stack[m.sp] = o
	m.sp++
	return nil
}

func (m *VM) tryPush(o object.Object) error {
	if err := m.push(o); err != nil {
		return m.raiseObj(&object.Error{Message: err.Error()})
	}
	return nil
}

func (m *VM) pop() object.Object {
	m.sp--
	o := m.stack[m.sp]
	m.stack[m.sp] = nil
	m.lastPopped = o
	return o
}

func (m *VM) Exports() *object.Dict {
	return m.exports
}

func (m *VM) LastPoppedStackElem() object.Object {
	return m.lastPopped
}

func (m *VM) SetGlobals(globals []object.Object) {
	if globals != nil {
		m.globals = globals
	}
}

func (m *VM) SetModuleCache(cache map[string]*object.Dict) {
	if cache != nil {
		m.modules = cache
	}
}

func (m *VM) Run() error {
	return m.run(-1)
}

func (m *VM) run(stopFrames int) error {
	for {
		if stopFrames >= 0 && m.framesIndex <= stopFrames {
			return nil
		}
		frame := m.currentFrame()
		if frame == nil {
			return nil
		}
		ins := frame.Instructions()
		if frame.ip+1 >= len(ins) {
			if stopFrames >= 0 {
				return errors.New(m.formatStackTrace("unexpected end of instructions"))
			}
			return nil
		}
		frame.ip++
		op := code.Opcode(ins[frame.ip])

		switch op {
		case code.OpConstant:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			if err := m.tryPush(m.constants[idx]); err != nil {
				return err
			}
			continue

		case code.OpTrue:
			if err := m.tryPush(&object.Boolean{Value: true}); err != nil {
				return err
			}
			continue

		case code.OpFalse:
			if err := m.tryPush(&object.Boolean{Value: false}); err != nil {
				return err
			}
			continue

		case code.OpNull:
			if err := m.tryPush(nilObj); err != nil {
				return err
			}
			continue

		case code.OpArray:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			elems := make([]object.Object, n)
			for i := n - 1; i >= 0; i-- {
				elems[i] = m.pop()
			}
			if err := m.tryPush(&object.Array{Elements: elems}); err != nil {
				return err
			}
			continue

		case code.OpDict:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			pairs := make(map[string]object.DictPair, n)
			for i := 0; i < n; i++ {
				val := m.pop()
				keyObj := m.pop()

				hk, ok := object.HashKeyOf(keyObj)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unusable as dict key: %s", keyObj.Type())}); err != nil {
						return err
					}
					continue
				}
				pairs[object.HashKeyString(hk)] = object.DictPair{Key: keyObj, Value: val}
			}
			if err := m.tryPush(&object.Dict{Pairs: pairs}); err != nil {
				return err
			}
			continue

		case code.OpIndex:
			idx := m.pop()
			left := m.pop()

			switch l := left.(type) {
			case *object.Array:
				i, ok := idx.(*object.Integer)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("array index must be INTEGER, got %s", idx.Type())}); err != nil {
						return err
					}
					continue
				}
				n := int(i.Value)
				L := len(l.Elements)
				if n < 0 {
					n = L + n
				}
				if n < 0 || n >= L {
					if err := m.raiseObj(&object.Error{Message: "index out of range"}); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(l.Elements[n]); err != nil {
					return err
				}
				continue

			case *object.String:
				i, ok := idx.(*object.Integer)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("string index must be INTEGER, got %s", idx.Type())}); err != nil {
						return err
					}
					continue
				}
				rs := []rune(l.Value)
				n := int(i.Value)
				L := len(rs)
				if n < 0 {
					n = L + n
				}
				if n < 0 || n >= L {
					if err := m.raiseObj(&object.Error{Message: "index out of range"}); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(&object.String{Value: string(rs[n])}); err != nil {
					return err
				}
				continue

			case *object.Dict:
				hk, ok := object.HashKeyOf(idx)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unusable as dict key: %s", idx.Type())}); err != nil {
						return err
					}
					continue
				}
				pair, ok := l.Pairs[object.HashKeyString(hk)]
				if !ok {
					if err := m.tryPush(nilObj); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(pair.Value); err != nil {
					return err
				}
				continue

			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("indexing not supported on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

		case code.OpGetMember:
			nameIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "member name must be string constant"}); err != nil {
					return err
				}
				continue
			}

			left := m.pop()
			switch l := left.(type) {
			case *object.Dict:
				hk, ok := object.HashKeyOf(nameObj)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: "invalid member key"}); err != nil {
						return err
					}
					continue
				}
				pair, ok := l.Pairs[object.HashKeyString(hk)]
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unknown member: %s", nameObj.Value)}); err != nil {
						return err
					}
					continue
				}
				if err := m.push(pair.Value); err != nil {
					if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
						return err
					}
					continue
				}
				continue
			case *object.Error:
				switch nameObj.Value {
				case "message":
					if err := m.push(&object.String{Value: l.Message}); err != nil {
						if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
							return err
						}
					}
					continue
				case "code":
					if err := m.push(&object.Integer{Value: l.Code}); err != nil {
						if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
							return err
						}
					}
					continue
				case "stack":
					if err := m.push(&object.String{Value: l.Stack}); err != nil {
						if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
							return err
						}
					}
					continue
				}
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unknown member on ERROR: %s", nameObj.Value)}); err != nil {
					return err
				}
				continue
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("no member access on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

		case code.OpSetMember:
			nameIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "member name must be string constant"}); err != nil {
					return err
				}
				continue
			}

			val := m.pop()
			left := m.pop()

			d, ok := left.(*object.Dict)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("member assignment not supported on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

			hk, ok := object.HashKeyOf(nameObj)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "invalid member key"}); err != nil {
					return err
				}
				continue
			}
			if d.Pairs == nil {
				d.Pairs = map[string]object.DictPair{}
			}
			d.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: nameObj, Value: val}
			if err := m.tryPush(val); err != nil {
				return err
			}
			continue

		case code.OpSetIndex:
			val := m.pop()
			idx := m.pop()
			left := m.pop()

			switch l := left.(type) {
			case *object.Array:
				i, ok := idx.(*object.Integer)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("array index must be INTEGER, got %s", idx.Type())}); err != nil {
						return err
					}
					continue
				}
				n := int(i.Value)
				L := len(l.Elements)
				if n < 0 {
					n = L + n
				}
				if n < 0 || n >= L {
					if err := m.raiseObj(&object.Error{Message: "index out of range"}); err != nil {
						return err
					}
					continue
				}
				l.Elements[n] = val
				if err := m.tryPush(val); err != nil {
					return err
				}
				continue

			case *object.Dict:
				hk, ok := object.HashKeyOf(idx)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unusable as dict key: %s", idx.Type())}); err != nil {
						return err
					}
					continue
				}
				if l.Pairs == nil {
					l.Pairs = map[string]object.DictPair{}
				}
				l.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: idx, Value: val}
				if err := m.tryPush(val); err != nil {
					return err
				}
				continue

			case *object.String:
				if err := m.raiseObj(&object.Error{Message: "cannot assign into STRING (immutable)"}); err != nil {
					return err
				}
				continue

			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("index assignment not supported on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

		case code.OpSlice:
			highObj := m.pop()
			lowObj := m.pop()
			left := m.pop()

			toOptInt := func(o object.Object) (*int64, error) {
				if _, ok := o.(*object.Nil); ok {
					return nil, nil
				}
				i, ok := o.(*object.Integer)
				if !ok {
					return nil, fmt.Errorf("slice bound must be INTEGER or nil, got %s", o.Type())
				}
				v := i.Value
				return &v, nil
			}

			lowPtr, err := toOptInt(lowObj)
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}
			highPtr, err := toOptInt(highObj)
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}

			norm := func(x, length int64) int64 {
				if x < 0 {
					return length + x
				}
				return x
			}
			clamp := func(x, lo, hi int64) int64 {
				if x < lo {
					return lo
				}
				if x > hi {
					return hi
				}
				return x
			}
			bounds := func(length int64) (int64, int64) {
				lo := int64(0)
				hi := length
				if lowPtr != nil {
					lo = norm(*lowPtr, length)
				}
				if highPtr != nil {
					hi = norm(*highPtr, length)
				}
				lo = clamp(lo, 0, length)
				hi = clamp(hi, 0, length)
				if lo > hi {
					lo = hi
				}
				return lo, hi
			}

			switch l := left.(type) {
			case *object.Array:
				length := int64(len(l.Elements))
				lo, hi := bounds(length)
				out := make([]object.Object, 0, int(hi-lo))
				for i := int(lo); i < int(hi); i++ {
					out = append(out, l.Elements[i])
				}
				if err := m.tryPush(&object.Array{Elements: out}); err != nil {
					return err
				}
				continue

			case *object.String:
				rs := []rune(l.Value)
				length := int64(len(rs))
				lo, hi := bounds(length)
				if err := m.tryPush(&object.String{Value: string(rs[int(lo):int(hi)])}); err != nil {
					return err
				}
				continue

			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("slicing not supported on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

		case code.OpPop:
			m.pop()
			continue

		case code.OpSetGlobal:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			m.globals[idx] = m.pop()
			continue

		case code.OpGetGlobal:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			val := m.globals[idx]
			if val == nil {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("uninitialized global at %d", idx)}); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(val); err != nil {
				return err
			}
			continue

		case code.OpImportModule:
			pathIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			if m.importer == nil {
				if err := m.raiseObj(&object.Error{Message: "module importer not configured"}); err != nil {
					return err
				}
				continue
			}

			pathObj, ok := m.constants[pathIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "import path must be string constant"}); err != nil {
					return err
				}
				continue
			}

			fromFile := m.entryPath
			if frame != nil && frame.cl != nil && frame.cl.Fn != nil && frame.cl.Fn.File != "" {
				fromFile = frame.cl.Fn.File
			}
			bc, absPath, err := m.importer(fromFile, pathObj.Value)
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}

			if mod, ok := m.modules[absPath]; ok {
				if err := m.tryPush(mod); err != nil {
					return err
				}
				continue
			}

			modVM := NewWithImporter(bc, absPath, m.importer)
			modVM.modules = m.modules
			if err := modVM.Run(); err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}
			mod := modVM.Exports()
			m.modules[absPath] = mod
			if err := m.tryPush(mod); err != nil {
				return err
			}
			continue

		case code.OpImportFrom:
			pathIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			nameIdx := int(code.ReadUint16(ins[frame.ip+3:]))
			frame.ip += 4

			if m.importer == nil {
				if err := m.raiseObj(&object.Error{Message: "module importer not configured"}); err != nil {
					return err
				}
				continue
			}

			pathObj, ok := m.constants[pathIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "import path must be string constant"}); err != nil {
					return err
				}
				continue
			}
			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "import name must be string constant"}); err != nil {
					return err
				}
				continue
			}

			fromFile := m.entryPath
			if frame != nil && frame.cl != nil && frame.cl.Fn != nil && frame.cl.Fn.File != "" {
				fromFile = frame.cl.Fn.File
			}
			bc, absPath, err := m.importer(fromFile, pathObj.Value)
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}

			mod, ok := m.modules[absPath]
			if !ok {
				modVM := NewWithImporter(bc, absPath, m.importer)
				modVM.modules = m.modules
				if err := modVM.Run(); err != nil {
					if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
						return err
					}
					continue
				}
				mod = modVM.Exports()
				m.modules[absPath] = mod
			}

			hk, ok := object.HashKeyOf(nameObj)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "invalid from-import name"}); err != nil {
					return err
				}
				continue
			}
			pair, ok := mod.Pairs[object.HashKeyString(hk)]
			if !ok {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("module has no exported member: %s", nameObj.Value)}); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(pair.Value); err != nil {
				return err
			}
			continue

		case code.OpExport:
			nameIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "export name must be string constant"}); err != nil {
					return err
				}
				continue
			}
			val := m.pop()

			hk, ok := object.HashKeyOf(nameObj)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "invalid export name"}); err != nil {
					return err
				}
				continue
			}
			m.exports.Pairs[object.HashKeyString(hk)] = object.DictPair{Key: nameObj, Value: val}
			continue

		case code.OpGetBuiltin:
			idx := int(ins[frame.ip+1])
			frame.ip += 1
			if err := m.tryPush(builtins[idx]); err != nil {
				return err
			}
			continue

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			if err := m.execBinaryNumericOp(op); err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
			}
			continue

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			if err := m.execComparison(op); err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
			}
			continue

		case code.OpMinus:
			right := m.pop()
			switch v := right.(type) {
			case *object.Integer:
				if err := m.tryPush(&object.Integer{Value: -v.Value}); err != nil {
					return err
				}
			case *object.Float:
				if err := m.tryPush(&object.Float{Value: -v.Value}); err != nil {
					return err
				}
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unsupported operand for unary -: %s", right.Type())}); err != nil {
					return err
				}
			}
			continue

		case code.OpBang:
			right := m.pop()
			if err := m.tryPush(nativeBool(!isTruthy(right))); err != nil {
				return err
			}
			continue

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			cond := m.pop()
			if !isTruthy(cond) {
				frame.ip = pos - 1
			}
			continue

		case code.OpJump:
			pos := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip = pos - 1
			continue

		case code.OpTry:
			catch := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			m.traps = append(m.traps, trap{
				catchIP:  catch,
				sp:       m.sp,
				frameIdx: m.framesIndex,
			})
			continue

		case code.OpTryFinally:
			finallyIP := int(code.ReadUint16(ins[frame.ip+1:]))
			afterIP := int(code.ReadUint16(ins[frame.ip+3:]))
			frame.ip += 4
			m.finallys = append(m.finallys, fin{
				finallyIP: finallyIP,
				afterIP:   afterIP,
				sp:        m.sp,
				frameIdx:  m.framesIndex,
			})
			continue

		case code.OpEndTry:
			if len(m.traps) == 0 {
				return errors.New(m.formatStackTrace("EndTry with no active trap"))
			}
			m.traps = m.traps[:len(m.traps)-1]
			continue

		case code.OpEndFinally:
			if len(m.finallys) == 0 {
				return errors.New(m.formatStackTrace("EndFinally with no active finally"))
			}
			m.finallys = m.finallys[:len(m.finallys)-1]
			continue

		case code.OpRethrowPending:
			if m.pendingErr != nil {
				errObj := m.pendingErr
				m.pendingErr = nil
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			continue

		case code.OpThrow:
			val := m.pop()
			var errObj *object.Error
			switch obj := val.(type) {
			case *object.Error:
				errObj = obj
			case *object.String:
				errObj = &object.Error{Message: obj.Value}
			default:
				errObj = &object.Error{Message: obj.Inspect()}
			}
			if err := m.raiseObj(errObj); err != nil {
				return err
			}
			continue

		case code.OpPrint:
			val := m.pop()
			fmt.Println(val.Inspect())
			continue

		case code.OpGetLocal:
			localIndex := int(ins[frame.ip+1])
			frame.ip += 1
			bp := frame.basePointer
			if err := m.tryPush(m.stack[bp+localIndex]); err != nil {
				return err
			}
			continue

		case code.OpSetLocal:
			localIndex := int(ins[frame.ip+1])
			frame.ip += 1
			bp := frame.basePointer
			m.stack[bp+localIndex] = m.pop()
			continue

		case code.OpClosure:
			constIndex := int(code.ReadUint16(ins[frame.ip+1:]))
			numFree := int(ins[frame.ip+3])
			frame.ip += 3

			fnObj := m.constants[constIndex]
			fn, ok := fnObj.(*object.CompiledFunction)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "constant is not CompiledFunction"}); err != nil {
					return err
				}
				continue
			}

			free := make([]object.Object, numFree)
			for i := 0; i < numFree; i++ {
				free[i] = m.stack[m.sp-numFree+i]
			}
			m.sp -= numFree

			cl := &object.Closure{Fn: fn, Free: free}
			if err := m.tryPush(cl); err != nil {
				return err
			}
			continue

		case code.OpGetFree:
			freeIndex := int(ins[frame.ip+1])
			frame.ip += 1
			cl := m.currentFrame().cl
			if err := m.tryPush(cl.Free[freeIndex]); err != nil {
				return err
			}
			continue

		case code.OpCurrentClosure:
			if err := m.tryPush(m.currentFrame().cl); err != nil {
				return err
			}
			continue

		case code.OpCall:
			numArgs := int(ins[frame.ip+1])
			frame.ip += 1

			callee := m.stack[m.sp-1-numArgs]
			if b, ok := callee.(*object.Builtin); ok {
				args := make([]object.Object, numArgs)
				for i := numArgs - 1; i >= 0; i-- {
					args[i] = m.pop()
				}
				m.pop() // callee

				res := b.Fn(args...)
				if errObj, ok := res.(*object.Error); ok {
					if b == builtins[builtinIndex["error"]] {
						if err := m.tryPush(errObj); err != nil {
							return err
						}
						continue
					}
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(res); err != nil {
					return err
				}
				continue
			}

			cl, ok := callee.(*object.Closure)
			if !ok {
				typeName := "<nil>"
				if callee != nil {
					typeName = string(callee.Type())
				}
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("attempted to call non-function: %s", typeName)}); err != nil {
					return err
				}
				continue
			}
			fn := cl.Fn
			if numArgs != fn.NumParameters {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("wrong number of arguments: expected %d, got %d", fn.NumParameters, numArgs)}); err != nil {
					return err
				}
				continue
			}

			basePointer := m.sp - numArgs
			newFrame := NewFrame(cl, basePointer)
			m.pushFrame(newFrame)

			m.sp = basePointer + fn.NumLocals
			continue

		case code.OpDefer:
			argc := int(ins[frame.ip+1])
			frame.ip += 1

			args := make([]object.Object, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = m.pop()
			}
			fn := m.pop()

			frame.defers = append(frame.defers, deferredCall{fn: fn, args: args})
			continue

		case code.OpReturnValue:
			ret := m.pop()
			oldFrame := m.currentFrame()
			if err := m.runDefers(oldFrame); err != nil {
				return err
			}
			if m.currentFrame() != oldFrame {
				continue
			}
			oldFrame = m.popFrame()
			m.sp = oldFrame.basePointer - 1
			if err := m.tryPush(ret); err != nil {
				return err
			}
			continue

		case code.OpReturn:
			oldFrame := m.currentFrame()
			if err := m.runDefers(oldFrame); err != nil {
				return err
			}
			if m.currentFrame() != oldFrame {
				continue
			}
			oldFrame = m.popFrame()
			m.sp = oldFrame.basePointer - 1
			if err := m.tryPush(nilObj); err != nil {
				return err
			}
			continue

		default:
			return errors.New(m.formatStackTrace(fmt.Sprintf("unknown opcode: %d", op)))
		}
	}
	return nil
}

func (m *VM) runDefers(frame *Frame) error {
	if len(frame.defers) == 0 {
		return nil
	}
	defers := frame.defers
	frame.defers = nil
	for i := len(defers) - 1; i >= 0; i-- {
		d := defers[i]
		if _, err := m.applyFunction(d.fn, d.args); err != nil {
			return err
		}
		if m.currentFrame() != frame {
			return nil
		}
	}
	return nil
}

func (m *VM) applyFunction(fn object.Object, args []object.Object) (object.Object, error) {
	if b, ok := fn.(*object.Builtin); ok {
		res := b.Fn(args...)
		if errObj, ok := res.(*object.Error); ok {
			if b == builtins[builtinIndex["error"]] {
				return errObj, nil
			}
			if err := m.raiseObj(errObj); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return res, nil
	}

	cl, ok := fn.(*object.Closure)
	if !ok {
		typeName := "<nil>"
		if fn != nil {
			typeName = string(fn.Type())
		}
		if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("attempted to call non-function: %s", typeName)}); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if len(args) != cl.Fn.NumParameters {
		if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("wrong number of arguments: expected %d, got %d", cl.Fn.NumParameters, len(args))}); err != nil {
			return nil, err
		}
		return nil, nil
	}

	startSP := m.sp
	if err := m.tryPush(cl); err != nil {
		return nil, err
	}
	for _, arg := range args {
		if err := m.tryPush(arg); err != nil {
			return nil, err
		}
	}

	basePointer := m.sp - len(args)
	newFrame := NewFrame(cl, basePointer)
	stopFrames := m.framesIndex
	m.pushFrame(newFrame)
	m.sp = basePointer + cl.Fn.NumLocals

	if err := m.run(stopFrames); err != nil {
		return nil, err
	}

	if m.sp == startSP+1 {
		return m.pop(), nil
	}
	return nil, nil
}

func lookupPos(pos []compiler.SourcePos, ip int) (line, col int) {
	l, r := 0, len(pos)-1
	best := -1
	for l <= r {
		m := (l + r) / 2
		if pos[m].Offset <= ip {
			best = m
			l = m + 1
		} else {
			r = m - 1
		}
	}
	if best == -1 {
		return 0, 0
	}
	return pos[best].Line, pos[best].Col
}

func (m *VM) formatStackTrace(message string) string {
	out := "error: " + message + "\nstack trace:\n"
	for i := m.framesIndex - 1; i >= 0; i-- {
		f := m.frames[i]
		if f == nil || f.cl == nil || f.cl.Fn == nil {
			continue
		}
		fn := f.cl.Fn
		line, col := lookupPos(fn.Pos, f.ip)
		name := fn.Name
		if name == "" {
			name = "<anon>"
		}
		file := fn.File
		if file == "" {
			file = "<unknown>"
		}
		out += fmt.Sprintf("  at %s (%s:%d:%d)\n", name, file, line, col)
	}
	return out
}

func (m *VM) raiseObj(errObj *object.Error) error {
	if errObj == nil {
		return nil
	}
	if errObj.Stack == "" {
		errObj.Stack = m.formatStackTrace(errObj.Message)
	}
	const noCatch = 0xFFFF

	if len(m.traps) > 0 {
		t := m.traps[len(m.traps)-1]
		if t.catchIP != noCatch {
			m.traps = m.traps[:len(m.traps)-1]
			for m.framesIndex > t.frameIdx {
				f := m.frames[m.framesIndex-1]
				if f != nil {
					if err := m.runDefers(f); err != nil {
						return err
					}
					if m.currentFrame() != f {
						return nil
					}
				}
				m.frames[m.framesIndex-1] = nil
				m.framesIndex--
			}
			m.sp = t.sp

			cf := m.currentFrame()
			cf.ip = t.catchIP - 1

			if err := m.push(errObj); err != nil {
				return errors.New(errObj.Stack)
			}
			return nil
		}
		m.traps = m.traps[:len(m.traps)-1]
	}

	if len(m.finallys) > 0 {
		f := m.finallys[len(m.finallys)-1]
		m.finallys = m.finallys[:len(m.finallys)-1]
		m.pendingErr = errObj

		for m.framesIndex > f.frameIdx {
			frame := m.frames[m.framesIndex-1]
			if frame != nil {
				if err := m.runDefers(frame); err != nil {
					return err
				}
				if m.currentFrame() != frame {
					return nil
				}
			}
			m.frames[m.framesIndex-1] = nil
			m.framesIndex--
		}
		m.sp = f.sp

		cf := m.currentFrame()
		cf.ip = f.finallyIP - 1
		return nil
	}

	for m.framesIndex > 0 {
		f := m.frames[m.framesIndex-1]
		if f != nil {
			if err := m.runDefers(f); err != nil {
				return err
			}
			if m.currentFrame() != f {
				return nil
			}
		}
		m.frames[m.framesIndex-1] = nil
		m.framesIndex--
	}

	return errors.New(errObj.Stack)
}

func (m *VM) execBinaryNumericOp(op code.Opcode) error {
	right := m.pop()
	left := m.pop()

	li, lok := left.(*object.Integer)
	ri, rok := right.(*object.Integer)
	if lok && rok {
		var res int64
		switch op {
		case code.OpAdd:
			res = li.Value + ri.Value
		case code.OpSub:
			res = li.Value - ri.Value
		case code.OpMul:
			res = li.Value * ri.Value
		case code.OpDiv:
			if ri.Value == 0 {
				return fmt.Errorf("division by zero")
			}
			res = li.Value / ri.Value
		case code.OpMod:
			if ri.Value == 0 {
				return fmt.Errorf("modulo by zero")
			}
			res = li.Value % ri.Value
		default:
			return fmt.Errorf("unknown integer op: %d", op)
		}
		return m.push(&object.Integer{Value: res})
	}

	lf, lok := left.(*object.Float)
	rf, rok := right.(*object.Float)
	if !lok {
		if li, ok := left.(*object.Integer); ok {
			lf = &object.Float{Value: float64(li.Value)}
			lok = true
		}
	}
	if !rok {
		if ri, ok := right.(*object.Integer); ok {
			rf = &object.Float{Value: float64(ri.Value)}
			rok = true
		}
	}
	if !lok || !rok {
		return fmt.Errorf("numeric op on non-numbers: %s %s", left.Type(), right.Type())
	}

	switch op {
	case code.OpAdd:
		return m.push(&object.Float{Value: lf.Value + rf.Value})
	case code.OpSub:
		return m.push(&object.Float{Value: lf.Value - rf.Value})
	case code.OpMul:
		return m.push(&object.Float{Value: lf.Value * rf.Value})
	case code.OpDiv:
		if rf.Value == 0 {
			return fmt.Errorf("division by zero")
		}
		return m.push(&object.Float{Value: lf.Value / rf.Value})
	case code.OpMod:
		return fmt.Errorf("modulo requires INTEGER operands")
	default:
		return fmt.Errorf("unknown numeric op: %d", op)
	}
}

func (m *VM) execComparison(op code.Opcode) error {
	right := m.pop()
	left := m.pop()

	if li, ok := left.(*object.Integer); ok {
		if ri, ok := right.(*object.Integer); ok {
			var b bool
			switch op {
			case code.OpEqual:
				b = li.Value == ri.Value
			case code.OpNotEqual:
				b = li.Value != ri.Value
			case code.OpGreaterThan:
				b = li.Value > ri.Value
			}
			return m.push(nativeBool(b))
		}
	}

	if isNumber(left) && isNumber(right) {
		lf := toFloat(left)
		rf := toFloat(right)
		var b bool
		switch op {
		case code.OpEqual:
			b = lf == rf
		case code.OpNotEqual:
			b = lf != rf
		case code.OpGreaterThan:
			b = lf > rf
		default:
			return fmt.Errorf("unknown numeric comparison op")
		}
		return m.push(nativeBool(b))
	}

	if lb, ok := left.(*object.Boolean); ok {
		rb, ok := right.(*object.Boolean)
		if !ok {
			return fmt.Errorf("comparison type mismatch: %s vs %s", left.Type(), right.Type())
		}
		var b bool
		switch op {
		case code.OpEqual:
			b = lb.Value == rb.Value
		case code.OpNotEqual:
			b = lb.Value != rb.Value
		default:
			return fmt.Errorf("unsupported boolean comparison op")
		}
		return m.push(nativeBool(b))
	}

	return fmt.Errorf("unsupported comparison types: %s and %s", left.Type(), right.Type())
}

func isNumber(o object.Object) bool {
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

func isTruthy(o object.Object) bool {
	switch v := o.(type) {
	case *object.Boolean:
		return v.Value
	case *object.Nil:
		return false
	default:
		return true
	}
}

func nativeBool(b bool) object.Object {
	if b {
		return &object.Boolean{Value: true}
	}
	return &object.Boolean{Value: false}
}
