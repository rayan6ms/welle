package object

import "fmt"

type HashKey struct {
	Type  Type
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

func (s *String) HashKey() HashKey {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(s.Value); i++ {
		h ^= uint64(s.Value[i])
		h *= prime64
	}
	return HashKey{Type: STRING_OBJ, Value: h}
}

func (i *Integer) HashKey() HashKey {
	return HashKey{Type: INTEGER_OBJ, Value: uint64(i.Value)}
}

func (b *Boolean) HashKey() HashKey {
	if b.Value {
		return HashKey{Type: BOOLEAN_OBJ, Value: 1}
	}
	return HashKey{Type: BOOLEAN_OBJ, Value: 0}
}

func HashKeyOf(o Object) (HashKey, bool) {
	h, ok := o.(Hashable)
	if !ok {
		return HashKey{}, false
	}
	return h.HashKey(), true
}

func HashKeyString(k HashKey) string {
	return fmt.Sprintf("%s:%d", k.Type, k.Value)
}
