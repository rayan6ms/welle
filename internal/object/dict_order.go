package object

import "sort"

type dictSortEntry struct {
	pair     DictPair
	rank     int
	boolVal  bool
	intVal   int64
	strVal   string
	typeName string
	inspect  string
}

const (
	dictRankBool = iota
	dictRankInt
	dictRankString
	dictRankOther
)

// SortedDictPairs returns dict pairs in deterministic order.
// Order: bool < int < string; within type: false < true, numeric asc, lexicographic asc.
func SortedDictPairs(d *Dict) []DictPair {
	if d == nil || len(d.Pairs) == 0 {
		return nil
	}
	entries := make([]dictSortEntry, 0, len(d.Pairs))
	for _, pair := range d.Pairs {
		e := dictSortEntry{pair: pair}
		switch k := pair.Key.(type) {
		case *Boolean:
			e.rank = dictRankBool
			e.boolVal = k.Value
		case *Integer:
			e.rank = dictRankInt
			e.intVal = k.Value
		case *String:
			e.rank = dictRankString
			e.strVal = k.Value
		default:
			e.rank = dictRankOther
			if pair.Key != nil {
				e.typeName = string(pair.Key.Type())
				e.inspect = pair.Key.Inspect()
			}
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		a := entries[i]
		b := entries[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		switch a.rank {
		case dictRankBool:
			if a.boolVal == b.boolVal {
				return false
			}
			return !a.boolVal && b.boolVal
		case dictRankInt:
			return a.intVal < b.intVal
		case dictRankString:
			return a.strVal < b.strVal
		default:
			if a.typeName != b.typeName {
				return a.typeName < b.typeName
			}
			return a.inspect < b.inspect
		}
	})

	out := make([]DictPair, len(entries))
	for i, e := range entries {
		out[i] = e.pair
	}
	return out
}
