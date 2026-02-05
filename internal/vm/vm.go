package vm

import (
	"errors"
	"fmt"
	"strings"

	"welle/internal/code"
	"welle/internal/compiler"
	"welle/internal/limits"
	"welle/internal/object"
	"welle/internal/semantics"
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
	imports   *importTracker

	pendingErr *object.Error

	maxRecursion int
	maxSteps     int64
	stepsLeft    int64

	budget *limits.Budget
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

type importTracker struct {
	stack []string
	index map[string]int
}

func newImportTracker() *importTracker {
	return &importTracker{
		stack: []string{},
		index: map[string]int{},
	}
}

func (t *importTracker) enter(path string) error {
	if idx, ok := t.index[path]; ok {
		chain := append([]string{}, t.stack[idx:]...)
		chain = append(chain, path)
		return fmt.Errorf("WM0001 import cycle: %s", strings.Join(chain, " -> "))
	}
	t.index[path] = len(t.stack)
	t.stack = append(t.stack, path)
	return nil
}

func (t *importTracker) exit(path string) {
	delete(t.index, path)
	if len(t.stack) > 0 {
		t.stack = t.stack[:len(t.stack)-1]
	}
}

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
		imports:     newImportTracker(),
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

func cellValue(cell *object.Cell) object.Object {
	if cell.Value == nil {
		return nilObj
	}
	return cell.Value
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

func (m *VM) SetMaxRecursion(max int) {
	if max < 0 {
		max = 0
	}
	m.maxRecursion = max
}

func (m *VM) SetMaxSteps(max int64) {
	if max < 0 {
		max = 0
	}
	m.maxSteps = max
}

func (m *VM) SetMaxMemory(max int64) {
	if max < 0 {
		max = 0
	}
	m.budget = limits.NewBudget(max)
}

func (m *VM) SetBudget(b *limits.Budget) {
	m.budget = b
}

func (m *VM) Run() error {
	if m.entryPath != "" {
		if err := m.imports.enter(m.entryPath); err != nil {
			return err
		}
		defer m.imports.exit(m.entryPath)
	}
	if m.maxSteps > 0 {
		m.stepsLeft = m.maxSteps
	}
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
		if m.maxSteps > 0 {
			m.stepsLeft--
			if m.stepsLeft < 0 {
				errObj := &object.Error{Message: fmt.Sprintf("max instruction count exceeded (%d)", m.maxSteps)}
				ip := frame.ip
				if ip < 0 {
					ip = 0
				}
				catchIP, ok := activeTryCatchIP(ins, ip)
				if !ok {
					catchIP, ok = firstTryCatchIP(ins)
				}
				if ok {
					if errObj.Stack == "" {
						errObj.Stack = m.formatStackTrace(errObj.Message)
					}
					m.sp = frame.basePointer
					m.maxSteps = 0
					m.stepsLeft = 0
					if err := m.push(errObj); err != nil {
						return err
					}
					frame.ip = catchIP - 1
					continue
				}
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
		}

		switch op {
		case code.OpConstant:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			if s, ok := m.constants[idx].(*object.String); ok {
				if errObj := m.chargeMemory(object.CostStringBytes(len(s.Value))); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
			}
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
			if errObj := m.chargeMemory(object.CostArray(len(elems))); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(&object.Array{Elements: elems}); err != nil {
				return err
			}
			continue

		case code.OpArrayAppend:
			val := m.pop()
			arrObj := m.pop()
			arr, ok := arrObj.(*object.Array)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "array append expects ARRAY"}); err != nil {
					return err
				}
				continue
			}
			if errObj := m.chargeMemory(object.CostArrayElements(1)); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			arr.Elements = append(arr.Elements, val)
			if err := m.tryPush(arr); err != nil {
				return err
			}
			continue

		case code.OpTuple:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			elems := make([]object.Object, n)
			for i := n - 1; i >= 0; i-- {
				elems[i] = m.pop()
			}
			if errObj := m.chargeMemory(object.CostTuple(len(elems))); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(&object.Tuple{Elements: elems}); err != nil {
				return err
			}
			continue

		case code.OpDict:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			pairs := make(map[string]object.DictPair, n)
			raw := make([]object.DictPair, n)
			for i := 0; i < n; i++ {
				val := m.pop()
				keyObj := m.pop()
				raw[i] = object.DictPair{Key: keyObj, Value: val}
			}
			// Preserve source order so duplicate keys are last-wins.
			for i := n - 1; i >= 0; i-- {
				keyObj := raw[i].Key
				hk, ok := object.HashKeyOf(keyObj)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unusable as dict key: %s", keyObj.Type())}); err != nil {
						return err
					}
					continue
				}
				pairs[object.HashKeyString(hk)] = raw[i]
			}
			if errObj := m.chargeMemory(object.CostDict(len(pairs))); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(&object.Dict{Pairs: pairs}); err != nil {
				return err
			}
			continue

		case code.OpIterInit:
			iterable := m.pop()
			switch v := iterable.(type) {
			case *object.Array:
				if err := m.tryPush(&vmIterator{items: v.Elements}); err != nil {
					return err
				}
			case *object.Dict:
				pairs := object.SortedDictPairs(v)
				items := make([]object.Object, 0, len(pairs))
				for _, pair := range pairs {
					items = append(items, pair.Key)
				}
				if err := m.tryPush(&vmIterator{items: items}); err != nil {
					return err
				}
			case *object.String:
				rs := []rune(v.Value)
				items := make([]object.Object, 0, len(rs))
				for _, rch := range rs {
					s := &object.String{Value: string(rch)}
					if errObj := m.chargeMemory(object.CostStringBytes(len(s.Value))); errObj != nil {
						if err := m.raiseObj(errObj); err != nil {
							return err
						}
						continue
					}
					items = append(items, s)
				}
				if err := m.tryPush(&vmIterator{items: items}); err != nil {
					return err
				}
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("cannot iterate over type: %s", iterable.Type())}); err != nil {
					return err
				}
			}
			continue

		case code.OpIterInitComp:
			iterable := m.pop()
			switch v := iterable.(type) {
			case *object.Array:
				if err := m.tryPush(&vmIterator{items: v.Elements}); err != nil {
					return err
				}
			case *object.Dict:
				pairs := object.SortedDictPairs(v)
				items := make([]object.Object, 0, len(pairs))
				for _, pair := range pairs {
					items = append(items, pair.Key)
				}
				if err := m.tryPush(&vmIterator{items: items}); err != nil {
					return err
				}
			case *object.String:
				rs := []rune(v.Value)
				items := make([]object.Object, 0, len(rs))
				for _, rch := range rs {
					s := &object.String{Value: string(rch)}
					if errObj := m.chargeMemory(object.CostStringBytes(len(s.Value))); errObj != nil {
						if err := m.raiseObj(errObj); err != nil {
							return err
						}
						continue
					}
					items = append(items, s)
				}
				if err := m.tryPush(&vmIterator{items: items}); err != nil {
					return err
				}
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("cannot iterate %s in comprehension", iterable.Type())}); err != nil {
					return err
				}
			}
			continue

		case code.OpIterInitDict:
			iterable := m.pop()
			switch v := iterable.(type) {
			case *object.Dict:
				pairs := object.SortedDictPairs(v)
				items := make([]object.Object, 0, len(pairs))
				for _, pair := range pairs {
					items = append(items, pair.Key)
				}
				if err := m.tryPush(&vmIterator{items: items}); err != nil {
					return err
				}
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("for-in destructuring requires dict, got %s", iterable.Type())}); err != nil {
					return err
				}
			}
			continue

		case code.OpIterNext:
			iterObj := m.pop()
			it, ok := iterObj.(*vmIterator)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "invalid iterator"}); err != nil {
					return err
				}
				continue
			}
			val, ok := it.next()
			if err := m.tryPush(val); err != nil {
				return err
			}
			if err := m.tryPush(nativeBool(ok)); err != nil {
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

			case *object.Tuple:
				i, ok := idx.(*object.Integer)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("tuple index must be INTEGER, got %s", idx.Type())}); err != nil {
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
				out := &object.String{Value: string(rs[n])}
				if errObj := m.chargeMemory(object.CostStringBytes(len(out.Value))); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(out); err != nil {
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
			default:
				if getter, ok := left.(object.MemberGetter); ok {
					if val, ok := getter.GetMember(nameObj.Value); ok {
						if err := m.push(val); err != nil {
							if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
								return err
							}
						}
						continue
					}
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unknown member on %s: %s", left.Type(), nameObj.Value)}); err != nil {
						return err
					}
					continue
				}
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
			keyStr := object.HashKeyString(hk)
			if _, exists := d.Pairs[keyStr]; !exists {
				if errObj := m.chargeMemory(object.CostDictEntry()); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
			}
			d.Pairs[keyStr] = object.DictPair{Key: nameObj, Value: val}
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
				keyStr := object.HashKeyString(hk)
				if _, exists := l.Pairs[keyStr]; !exists {
					if errObj := m.chargeMemory(object.CostDictEntry()); errObj != nil {
						if err := m.raiseObj(errObj); err != nil {
							return err
						}
						continue
					}
				}
				l.Pairs[keyStr] = object.DictPair{Key: idx, Value: val}
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
			stepObj := m.pop()
			highObj := m.pop()
			lowObj := m.pop()
			left := m.pop()

			toOptInt := func(o object.Object, label string) (*int64, error) {
				if _, ok := o.(*object.Nil); ok {
					return nil, nil
				}
				i, ok := o.(*object.Integer)
				if !ok {
					return nil, fmt.Errorf("slice %s must be INTEGER, got: %s", label, o.Type())
				}
				v := i.Value
				return &v, nil
			}

			lowPtr, err := toOptInt(lowObj, "low")
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}
			highPtr, err := toOptInt(highObj, "high")
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}

			stepVal := int64(1)
			if _, ok := stepObj.(*object.Nil); !ok {
				i, ok := stepObj.(*object.Integer)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("slice step must be INTEGER, got: %s", stepObj.Type())}); err != nil {
						return err
					}
					continue
				}
				if i.Value == 0 {
					if err := m.raiseObj(&object.Error{Message: "slice step cannot be 0"}); err != nil {
						return err
					}
					continue
				}
				stepVal = i.Value
			}

			switch l := left.(type) {
			case *object.Array:
				out := sliceElements(l.Elements, lowPtr, highPtr, stepVal)
				if errObj := m.chargeMemory(object.CostArray(len(out))); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(&object.Array{Elements: out}); err != nil {
					return err
				}
				continue

			case *object.String:
				rs := []rune(l.Value)
				out := &object.String{Value: string(sliceRunes(rs, lowPtr, highPtr, stepVal))}
				if errObj := m.chargeMemory(object.CostStringBytes(len(out.Value))); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
				if err := m.tryPush(out); err != nil {
					return err
				}
				continue

			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("slicing not supported on %s", left.Type())}); err != nil {
					return err
				}
				continue
			}

		case code.OpUnpackTuple:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2

			val := m.pop()
			var elems []object.Object
			switch seq := val.(type) {
			case *object.Tuple:
				elems = seq.Elements
			case *object.Array:
				elems = seq.Elements
			default:
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("unpack expects tuple, got %s", val.Type())}); err != nil {
					return err
				}
				continue
			}
			if len(elems) != n {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("tuple arity mismatch: expected %d, got %d", n, len(elems))}); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(val); err != nil {
				return err
			}
			for i := 0; i < n; i++ {
				if err := m.tryPush(elems[i]); err != nil {
					return err
				}
			}
			continue

		case code.OpUnpackStar:
			n := int(code.ReadUint16(ins[frame.ip+1:]))
			starIdx := int(code.ReadUint16(ins[frame.ip+3:]))
			frame.ip += 4

			val := m.pop()
			var elems []object.Object
			switch seq := val.(type) {
			case *object.Tuple:
				elems = seq.Elements
			case *object.Array:
				elems = seq.Elements
			default:
				if err := m.raiseObj(&object.Error{Message: "cannot unpack non-sequence"}); err != nil {
					return err
				}
				continue
			}
			minLen := n - 1
			if len(elems) < minLen {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("not enough values to unpack (expected at least %d, got %d)", minLen, len(elems))}); err != nil {
					return err
				}
				continue
			}

			if err := m.tryPush(val); err != nil {
				return err
			}

			headCount := starIdx
			tailCount := n - starIdx - 1
			for i := 0; i < headCount; i++ {
				if err := m.tryPush(elems[i]); err != nil {
					return err
				}
			}

			midStart := headCount
			midEnd := len(elems) - tailCount
			mid := make([]object.Object, 0, midEnd-midStart)
			for i := midStart; i < midEnd; i++ {
				mid = append(mid, elems[i])
			}
			if errObj := m.chargeMemory(object.CostArray(len(mid))); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(&object.Array{Elements: mid}); err != nil {
				return err
			}

			for i := 0; i < tailCount; i++ {
				if err := m.tryPush(elems[len(elems)-tailCount+i]); err != nil {
					return err
				}
			}
			continue

		case code.OpSpread:
			val := m.pop()
			if err := m.tryPush(&object.Spread{Value: val}); err != nil {
				return err
			}
			continue

		case code.OpPop:
			m.pop()
			continue

		case code.OpSetGlobal:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			m.globals[idx] = m.pop()
			continue

		case code.OpDefineGlobal:
			idx := int(code.ReadUint16(ins[frame.ip+1:]))
			nameIdx := int(code.ReadUint16(ins[frame.ip+3:]))
			frame.ip += 4
			val := m.pop()
			if m.globals[idx] != nil {
				name := "<unknown>"
				if nameObj, ok := m.constants[nameIdx].(*object.String); ok {
					name = nameObj.Value
				}
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("cannot redeclare %q in this scope", name)}); err != nil {
					return err
				}
				continue
			}
			m.globals[idx] = val
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
			modVM.SetMaxRecursion(m.maxRecursion)
			modVM.SetMaxSteps(m.maxSteps)
			modVM.SetBudget(m.budget)
			modVM.modules = m.modules
			modVM.imports = m.imports
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
				modVM.SetMaxRecursion(m.maxRecursion)
				modVM.SetMaxSteps(m.maxSteps)
				modVM.SetBudget(m.budget)
				modVM.modules = m.modules
				modVM.imports = m.imports
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
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("missing export %q in module %q", nameObj.Value, pathObj.Value)}); err != nil {
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

		case code.OpDictUpdate:
			right := m.pop()
			left := m.pop()
			ld, ok := left.(*object.Dict)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "|= left operand must be dict"}); err != nil {
					return err
				}
				continue
			}
			rd, ok := right.(*object.Dict)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "|= right operand must be dict"}); err != nil {
					return err
				}
				continue
			}
			added := semantics.DictUpdateCount(ld, rd)
			if added > 0 {
				if errObj := m.chargeMemory(object.CostDictEntry() * int64(added)); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
			}
			semantics.DictUpdate(ld, rd)
			if err := m.tryPush(ld); err != nil {
				return err
			}
			continue

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod,
			code.OpBitOr, code.OpBitAnd, code.OpBitXor, code.OpShl, code.OpShr:
			if err := m.execBinaryOp(op); err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
			}
			continue

		case code.OpEqual, code.OpNotEqual, code.OpIs, code.OpGreaterThan, code.OpLessThan, code.OpLessEqual, code.OpGreaterEqual:
			if err := m.execComparison(op); err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
			}
			continue
		case code.OpIn:
			if err := m.execIn(); err != nil {
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
		case code.OpBitNot:
			right := m.pop()
			res, err := semantics.BitwiseUnary("~", right)
			if err != nil {
				if err := m.raiseObj(&object.Error{Message: err.Error()}); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(res); err != nil {
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

		case code.OpJumpIfNil:
			pos := int(code.ReadUint16(ins[frame.ip+1:]))
			frame.ip += 2
			cond := m.stack[m.sp-1]
			if cond.Type() == object.NIL_OBJ {
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
				if errObj.IsValue {
					errObj = &object.Error{
						Message: errObj.Message,
						Code:    errObj.Code,
						Stack:   errObj.Stack,
					}
				}
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
			obj := m.stack[bp+localIndex]
			if cell, ok := obj.(*object.Cell); ok {
				obj = cellValue(cell)
			} else if obj == nil {
				obj = nilObj
			}
			if err := m.tryPush(obj); err != nil {
				return err
			}
			continue

		case code.OpSetLocal:
			localIndex := int(ins[frame.ip+1])
			frame.ip += 1
			bp := frame.basePointer
			val := m.pop()
			if cell, ok := m.stack[bp+localIndex].(*object.Cell); ok {
				cell.Value = val
			} else {
				m.stack[bp+localIndex] = val
			}
			continue

		case code.OpDefineLocal:
			localIndex := int(ins[frame.ip+1])
			nameIdx := int(code.ReadUint16(ins[frame.ip+2:]))
			frame.ip += 3
			bp := frame.basePointer
			val := m.pop()
			if m.stack[bp+localIndex] != nil {
				name := "<unknown>"
				if nameObj, ok := m.constants[nameIdx].(*object.String); ok {
					name = nameObj.Value
				}
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("cannot redeclare %q in this scope", name)}); err != nil {
					return err
				}
				continue
			}
			m.stack[bp+localIndex] = val
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

			free := make([]*object.Cell, numFree)
			var memErr *object.Error
			for i := 0; i < numFree; i++ {
				obj := m.stack[m.sp-numFree+i]
				cell, ok := obj.(*object.Cell)
				if !ok {
					if obj == nil {
						obj = nilObj
					}
					if memErr == nil {
						if errObj := m.chargeMemory(object.CostCell()); errObj != nil {
							memErr = errObj
							break
						}
					}
					cell = &object.Cell{Value: obj}
				}
				free[i] = cell
			}
			if memErr != nil {
				if err := m.raiseObj(memErr); err != nil {
					return err
				}
				continue
			}
			m.sp -= numFree

			if errObj := m.chargeMemory(object.CostClosure(len(free))); errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			cl := &object.Closure{Fn: fn, Free: free}
			if err := m.tryPush(cl); err != nil {
				return err
			}
			continue

		case code.OpGetFree:
			freeIndex := int(ins[frame.ip+1])
			frame.ip += 1
			cl := m.currentFrame().cl
			if err := m.tryPush(cellValue(cl.Free[freeIndex])); err != nil {
				return err
			}
			continue

		case code.OpSetFree:
			freeIndex := int(ins[frame.ip+1])
			frame.ip += 1
			cl := m.currentFrame().cl
			cl.Free[freeIndex].Value = m.pop()
			continue

		case code.OpGetFreeCell:
			freeIndex := int(ins[frame.ip+1])
			frame.ip += 1
			cl := m.currentFrame().cl
			if err := m.tryPush(cl.Free[freeIndex]); err != nil {
				return err
			}
			continue

		case code.OpGetLocalCell:
			localIndex := int(ins[frame.ip+1])
			frame.ip += 1
			bp := frame.basePointer
			obj := m.stack[bp+localIndex]
			cell, ok := obj.(*object.Cell)
			if !ok {
				if obj == nil {
					obj = nilObj
				}
				if errObj := m.chargeMemory(object.CostCell()); errObj != nil {
					if err := m.raiseObj(errObj); err != nil {
						return err
					}
					continue
				}
				cell = &object.Cell{Value: obj}
				m.stack[bp+localIndex] = cell
			}
			if err := m.tryPush(cell); err != nil {
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

				if b == builtins[builtinIndex["map"]] {
					res, ok, err := m.runBuiltinMap(args)
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					if errObj, ok := res.(*object.Error); ok {
						if err := m.raiseObj(errObj); err != nil {
							return err
						}
						continue
					}
					if memErr := m.chargeObject(res); memErr != nil {
						if err := m.raiseObj(memErr); err != nil {
							return err
						}
						continue
					}
					if err := m.tryPush(res); err != nil {
						return err
					}
					continue
				}

				res := b.Fn(args...)
				if errObj, ok := res.(*object.Error); ok {
					if b == builtins[builtinIndex["error"]] {
						if errObj.Stack == "" {
							errObj.Stack = m.formatStackTrace(errObj.Message)
						}
						if memErr := m.chargeMemory(object.CostError()); memErr != nil {
							if err := m.raiseObj(memErr); err != nil {
								return err
							}
							continue
						}
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
				if b == builtins[builtinIndex["sort"]] {
					if arr, ok := res.(*object.Array); ok {
						extra := int64(0)
						for _, el := range arr.Elements {
							if s, ok := el.(*object.String); ok {
								extra += object.CostStringBytes(len(s.Value))
							}
						}
						if extra > 0 {
							if memErr := m.chargeMemory(extra); memErr != nil {
								if err := m.raiseObj(memErr); err != nil {
									return err
								}
								continue
							}
						}
					}
				}
				if memErr := m.chargeObject(res); memErr != nil {
					if err := m.raiseObj(memErr); err != nil {
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
			if m.maxRecursion > 0 && m.framesIndex >= m.maxRecursion+1 {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("max recursion depth exceeded (%d)", m.maxRecursion)}); err != nil {
					return err
				}
				continue
			}

			basePointer := m.sp - numArgs
			newFrame := NewFrame(cl, basePointer)
			m.pushFrame(newFrame)

			m.sp = basePointer + fn.NumLocals
			continue

		case code.OpCallSpread:
			numArgs := int(ins[frame.ip+1])
			frame.ip += 1

			rawArgs := make([]object.Object, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				rawArgs[i] = m.pop()
			}
			callee := m.pop()

			args, errObj := m.expandSpreadArgs(rawArgs)
			if errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}

			if b, ok := callee.(*object.Builtin); ok {
				if b == builtins[builtinIndex["map"]] {
					res, ok, err := m.runBuiltinMap(args)
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					if errObj, ok := res.(*object.Error); ok {
						if err := m.raiseObj(errObj); err != nil {
							return err
						}
						continue
					}
					if memErr := m.chargeObject(res); memErr != nil {
						if err := m.raiseObj(memErr); err != nil {
							return err
						}
						continue
					}
					if err := m.tryPush(res); err != nil {
						return err
					}
					continue
				}

				res := b.Fn(args...)
				if errObj, ok := res.(*object.Error); ok {
					if b == builtins[builtinIndex["error"]] {
						if errObj.Stack == "" {
							errObj.Stack = m.formatStackTrace(errObj.Message)
						}
						if memErr := m.chargeMemory(object.CostError()); memErr != nil {
							if err := m.raiseObj(memErr); err != nil {
								return err
							}
							continue
						}
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
				if b == builtins[builtinIndex["sort"]] {
					if arr, ok := res.(*object.Array); ok {
						extra := int64(0)
						for _, el := range arr.Elements {
							if s, ok := el.(*object.String); ok {
								extra += object.CostStringBytes(len(s.Value))
							}
						}
						if extra > 0 {
							if memErr := m.chargeMemory(extra); memErr != nil {
								if err := m.raiseObj(memErr); err != nil {
									return err
								}
								continue
							}
						}
					}
				}
				if memErr := m.chargeObject(res); memErr != nil {
					if err := m.raiseObj(memErr); err != nil {
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
			if len(args) != fn.NumParameters {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("wrong number of arguments: expected %d, got %d", fn.NumParameters, len(args))}); err != nil {
					return err
				}
				continue
			}
			if m.maxRecursion > 0 && m.framesIndex >= m.maxRecursion+1 {
				if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("max recursion depth exceeded (%d)", m.maxRecursion)}); err != nil {
					return err
				}
				continue
			}

			if err := m.tryPush(callee); err != nil {
				return err
			}
			for _, arg := range args {
				if err := m.tryPush(arg); err != nil {
					return err
				}
			}

			basePointer := m.sp - len(args)
			newFrame := NewFrame(cl, basePointer)
			m.pushFrame(newFrame)

			m.sp = basePointer + fn.NumLocals
			continue

		case code.OpCallMethod:
			nameIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			numArgs := int(ins[frame.ip+3])
			frame.ip += 3

			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "member name must be string constant"}); err != nil {
					return err
				}
				continue
			}

			args := make([]object.Object, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				args[i] = m.pop()
			}
			recv := m.pop()

			if d, ok := recv.(*object.Dict); ok {
				hk, ok := object.HashKeyOf(nameObj)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: "invalid member key"}); err != nil {
						return err
					}
					continue
				}
				if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
					if err := m.callWithArgs(pair.Value, args); err != nil {
						return err
					}
					continue
				}
			}

			res := applyMethod(nameObj.Value, recv, args)
			if errObj, ok := res.(*object.Error); ok {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if memErr := m.chargeObject(res); memErr != nil {
				if err := m.raiseObj(memErr); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(res); err != nil {
				return err
			}
			continue

		case code.OpCallMethodSpread:
			nameIdx := int(code.ReadUint16(ins[frame.ip+1:]))
			numArgs := int(ins[frame.ip+3])
			frame.ip += 3

			nameObj, ok := m.constants[nameIdx].(*object.String)
			if !ok {
				if err := m.raiseObj(&object.Error{Message: "member name must be string constant"}); err != nil {
					return err
				}
				continue
			}

			rawArgs := make([]object.Object, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				rawArgs[i] = m.pop()
			}
			args, errObj := m.expandSpreadArgs(rawArgs)
			if errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			recv := m.pop()

			if d, ok := recv.(*object.Dict); ok {
				hk, ok := object.HashKeyOf(nameObj)
				if !ok {
					if err := m.raiseObj(&object.Error{Message: "invalid member key"}); err != nil {
						return err
					}
					continue
				}
				if pair, exists := d.Pairs[object.HashKeyString(hk)]; exists {
					if err := m.callWithArgs(pair.Value, args); err != nil {
						return err
					}
					continue
				}
			}

			res := applyMethod(nameObj.Value, recv, args)
			if errObj, ok := res.(*object.Error); ok {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}
			if memErr := m.chargeObject(res); memErr != nil {
				if err := m.raiseObj(memErr); err != nil {
					return err
				}
				continue
			}
			if err := m.tryPush(res); err != nil {
				return err
			}
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

		case code.OpDeferSpread:
			argc := int(ins[frame.ip+1])
			frame.ip += 1

			rawArgs := make([]object.Object, argc)
			for i := argc - 1; i >= 0; i-- {
				rawArgs[i] = m.pop()
			}
			fn := m.pop()

			args, errObj := m.expandSpreadArgs(rawArgs)
			if errObj != nil {
				if err := m.raiseObj(errObj); err != nil {
					return err
				}
				continue
			}

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

func (m *VM) expandSpreadArgs(rawArgs []object.Object) ([]object.Object, *object.Error) {
	if len(rawArgs) == 0 {
		return nil, nil
	}
	out := make([]object.Object, 0, len(rawArgs))
	for _, arg := range rawArgs {
		spread, ok := arg.(*object.Spread)
		if !ok {
			out = append(out, arg)
			continue
		}

		val := spread.Value
		switch v := val.(type) {
		case *object.Tuple:
			out = append(out, v.Elements...)
		case *object.Array:
			out = append(out, v.Elements...)
		default:
			typeName := "<nil>"
			if val != nil {
				typeName = string(val.Type())
			}
			return nil, &object.Error{Message: fmt.Sprintf("cannot spread %s in call arguments", typeName)}
		}
	}
	return out, nil
}

func (m *VM) callWithArgs(callee object.Object, args []object.Object) error {
	if b, ok := callee.(*object.Builtin); ok {
		res := b.Fn(args...)
		if errObj, ok := res.(*object.Error); ok {
			if b == builtins[builtinIndex["error"]] {
				if errObj.Stack == "" {
					errObj.Stack = m.formatStackTrace(errObj.Message)
				}
				if memErr := m.chargeMemory(object.CostError()); memErr != nil {
					if err := m.raiseObj(memErr); err != nil {
						return err
					}
					return nil
				}
				if err := m.tryPush(errObj); err != nil {
					return err
				}
				return nil
			}
			if err := m.raiseObj(errObj); err != nil {
				return err
			}
			return nil
		}
		if b == builtins[builtinIndex["sort"]] {
			if arr, ok := res.(*object.Array); ok {
				extra := int64(0)
				for _, el := range arr.Elements {
					if s, ok := el.(*object.String); ok {
						extra += object.CostStringBytes(len(s.Value))
					}
				}
				if extra > 0 {
					if memErr := m.chargeMemory(extra); memErr != nil {
						if err := m.raiseObj(memErr); err != nil {
							return err
						}
						return nil
					}
				}
			}
		}
		if memErr := m.chargeObject(res); memErr != nil {
			if err := m.raiseObj(memErr); err != nil {
				return err
			}
			return nil
		}
		if err := m.tryPush(res); err != nil {
			return err
		}
		return nil
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
		return nil
	}
	fn := cl.Fn
	if len(args) != fn.NumParameters {
		if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("wrong number of arguments: expected %d, got %d", fn.NumParameters, len(args))}); err != nil {
			return err
		}
		return nil
	}
	if m.maxRecursion > 0 && m.framesIndex >= m.maxRecursion+1 {
		if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("max recursion depth exceeded (%d)", m.maxRecursion)}); err != nil {
			return err
		}
		return nil
	}

	if err := m.tryPush(callee); err != nil {
		return err
	}
	for _, arg := range args {
		if err := m.tryPush(arg); err != nil {
			return err
		}
	}

	basePointer := m.sp - len(args)
	newFrame := NewFrame(cl, basePointer)
	m.pushFrame(newFrame)
	m.sp = basePointer + fn.NumLocals
	return nil
}

func (m *VM) runBuiltinMap(args []object.Object) (object.Object, bool, error) {
	if len(args) != 2 {
		return &object.Error{Message: fmt.Sprintf("wrong number of arguments: expected 2, got %d", len(args))}, true, nil
	}
	fn := args[0]
	arr, ok := args[1].(*object.Array)
	if !ok {
		return &object.Error{Message: "map() second argument must be ARRAY"}, true, nil
	}
	switch fn.(type) {
	case *object.Builtin, *object.Closure:
	default:
		return &object.Error{Message: "map() first argument must be FUNCTION"}, true, nil
	}

	out := make([]object.Object, len(arr.Elements))
	for i, el := range arr.Elements {
		res, err := m.applyFunction(fn, []object.Object{el})
		if err != nil {
			return nil, false, err
		}
		if res == nil {
			return nil, false, nil
		}
		if errObj, ok := res.(*object.Error); ok && !errObj.IsValue {
			if err := m.raiseObj(errObj); err != nil {
				return nil, false, err
			}
			return nil, false, nil
		}
		out[i] = res
	}
	return &object.Array{Elements: out}, true, nil
}

func (m *VM) applyFunction(fn object.Object, args []object.Object) (object.Object, error) {
	if b, ok := fn.(*object.Builtin); ok {
		if b == builtins[builtinIndex["map"]] {
			res, ok, err := m.runBuiltinMap(args)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, nil
			}
			return res, nil
		}
		res := b.Fn(args...)
		if errObj, ok := res.(*object.Error); ok {
			if b == builtins[builtinIndex["error"]] {
				if errObj.Stack == "" {
					errObj.Stack = m.formatStackTrace(errObj.Message)
				}
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
	if m.maxRecursion > 0 && m.framesIndex >= m.maxRecursion+1 {
		if err := m.raiseObj(&object.Error{Message: fmt.Sprintf("max recursion depth exceeded (%d)", m.maxRecursion)}); err != nil {
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
	if errObj.Code != limits.MemoryErrorCode {
		if memErr := m.chargeMemory(object.CostError()); memErr != nil {
			errObj = memErr
		}
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

func (m *VM) execBinaryOp(op code.Opcode) error {
	right := m.pop()
	left := m.pop()

	res, err := semantics.BinaryOp(opString(op), left, right)
	if err != nil {
		return err
	}
	if s, ok := res.(*object.String); ok {
		if errObj := m.chargeMemory(object.CostStringBytes(len(s.Value))); errObj != nil {
			if err := m.raiseObj(errObj); err != nil {
				return err
			}
			return nil
		}
	}
	return m.push(res)
}

func (m *VM) execComparison(op code.Opcode) error {
	right := m.pop()
	left := m.pop()

	b, err := semantics.Compare(opString(op), left, right)
	if err != nil {
		return err
	}
	return m.push(nativeBool(b))
}

func (m *VM) execIn() error {
	right := m.pop()
	left := m.pop()
	b, err := semantics.InOp(left, right)
	if err != nil {
		return err
	}
	return m.push(nativeBool(b))
}

func isTruthy(o object.Object) bool {
	return semantics.IsTruthy(o)
}

func nativeBool(b bool) object.Object {
	if b {
		return &object.Boolean{Value: true}
	}
	return &object.Boolean{Value: false}
}

func activeTryCatchIP(ins code.Instructions, ip int) (int, bool) {
	stack := []int{}
	for i := 0; i < len(ins) && i <= ip; {
		op := code.Opcode(ins[i])
		def, ok := code.Lookup(op)
		if !ok {
			i++
			continue
		}
		operands, read := code.ReadOperands(def, ins[i+1:])
		switch op {
		case code.OpTry:
			if len(operands) > 0 {
				stack = append(stack, operands[0])
			}
		case code.OpEndTry:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
		i += 1 + read
	}
	if len(stack) == 0 {
		return 0, false
	}
	return stack[len(stack)-1], true
}

func firstTryCatchIP(ins code.Instructions) (int, bool) {
	for i := 0; i < len(ins); {
		op := code.Opcode(ins[i])
		def, ok := code.Lookup(op)
		if !ok {
			i++
			continue
		}
		operands, read := code.ReadOperands(def, ins[i+1:])
		if op == code.OpTry && len(operands) > 0 {
			return operands[0], true
		}
		i += 1 + read
	}
	return 0, false
}

func opString(op code.Opcode) string {
	switch op {
	case code.OpAdd:
		return "+"
	case code.OpSub:
		return "-"
	case code.OpMul:
		return "*"
	case code.OpDiv:
		return "/"
	case code.OpMod:
		return "%"
	case code.OpBitOr:
		return "|"
	case code.OpBitAnd:
		return "&"
	case code.OpBitXor:
		return "^"
	case code.OpShl:
		return "<<"
	case code.OpShr:
		return ">>"
	case code.OpEqual:
		return "=="
	case code.OpNotEqual:
		return "!="
	case code.OpIs:
		return "is"
	case code.OpGreaterThan:
		return ">"
	case code.OpLessThan:
		return "<"
	case code.OpLessEqual:
		return "<="
	case code.OpGreaterEqual:
		return ">="
	case code.OpIn:
		return "in"
	default:
		return fmt.Sprintf("op(%d)", op)
	}
}
