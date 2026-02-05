package compiler

type SymbolScope string

const GlobalScope SymbolScope = "GLOBAL"
const LocalScope SymbolScope = "LOCAL"
const FreeScope SymbolScope = "FREE"

type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

type SymbolTable struct {
	Outer          *SymbolTable
	store          map[string]Symbol
	numDefinitions int
	FreeSymbols    []Symbol
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{store: map[string]Symbol{}}
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	st := NewSymbolTable()
	st.Outer = outer
	return st
}

func (st *SymbolTable) Define(name string) Symbol {
	scope := GlobalScope
	if st.Outer != nil {
		scope = LocalScope
	}
	sym := Symbol{Name: name, Scope: scope, Index: st.numDefinitions}
	st.store[name] = sym
	st.numDefinitions++
	return sym
}

func (st *SymbolTable) DefineTemp(name string) Symbol {
	scope := GlobalScope
	if st.Outer != nil {
		scope = LocalScope
	}
	sym := Symbol{Name: name, Scope: scope, Index: st.numDefinitions}
	st.numDefinitions++
	return sym
}

func (st *SymbolTable) defineFree(original Symbol) Symbol {
	st.FreeSymbols = append(st.FreeSymbols, original)
	sym := Symbol{Name: original.Name, Index: len(st.FreeSymbols) - 1, Scope: FreeScope}
	st.store[original.Name] = sym
	return sym
}

func (st *SymbolTable) Resolve(name string) (Symbol, bool) {
	if sym, ok := st.store[name]; ok {
		return sym, true
	}
	if st.Outer == nil {
		return Symbol{}, false
	}

	outerSym, ok := st.Outer.Resolve(name)
	if !ok {
		return Symbol{}, false
	}

	if outerSym.Scope == GlobalScope {
		return outerSym, true
	}

	free := st.defineFree(outerSym)
	return free, true
}

func (st *SymbolTable) ResolveCurrent(name string) (Symbol, bool) {
	sym, ok := st.store[name]
	if !ok {
		return Symbol{}, false
	}
	if st.Outer == nil {
		if sym.Scope == GlobalScope {
			return sym, true
		}
		return Symbol{}, false
	}
	if sym.Scope == LocalScope {
		return sym, true
	}
	return Symbol{}, false
}
