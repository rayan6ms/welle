package object

type Environment struct {
	store map[string]Object
	outer *Environment
}

const ExportSetName = "__welle_exports__"

func NewEnvironment() *Environment {
	return &Environment{store: map[string]Object{}}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) GetHere(name string) (Object, bool) {
	obj, ok := e.store[name]
	return obj, ok
}

func (e *Environment) Assign(name string, val Object) (Object, bool) {
	if _, ok := e.store[name]; ok {
		e.store[name] = val
		return val, true
	}

	if e.outer != nil {
		return e.outer.Assign(name, val)
	}

	return nil, false
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) Snapshot() map[string]Object {
	out := make(map[string]Object, len(e.store))
	for k, v := range e.store {
		out[k] = v
	}
	return out
}

func (e *Environment) MarkExport(name string) {
	set, ok := e.store[ExportSetName].(*Dict)
	if !ok {
		set = &Dict{Pairs: map[string]DictPair{}}
		e.store[ExportSetName] = set
	}
	key := &String{Value: name}
	hk, _ := HashKeyOf(key)
	set.Pairs[HashKeyString(hk)] = DictPair{Key: key, Value: &Boolean{Value: true}}
}

func (e *Environment) ExportedNames() map[string]bool {
	out := map[string]bool{}
	set, ok := e.store[ExportSetName].(*Dict)
	if !ok {
		return out
	}
	for _, pair := range set.Pairs {
		if ks, ok := pair.Key.(*String); ok {
			out[ks.Value] = true
		}
	}
	return out
}
