package vm

import "welle/internal/object"

func sliceBounds(lowPtr *int64, highPtr *int64, stepVal int64, length int64) (int64, int64) {
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
	if stepVal > 0 {
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
	lo := length - 1
	hi := int64(-1)
	if lowPtr != nil {
		lo = norm(*lowPtr, length)
	}
	if highPtr != nil {
		hi = norm(*highPtr, length)
	}
	lo = clamp(lo, -1, length-1)
	hi = clamp(hi, -1, length-1)
	if lo < hi {
		lo = hi
	}
	return lo, hi
}

func sliceElements(elements []object.Object, lowPtr *int64, highPtr *int64, stepVal int64) []object.Object {
	length := int64(len(elements))
	lo, hi := sliceBounds(lowPtr, highPtr, stepVal, length)
	out := make([]object.Object, 0)
	if stepVal > 0 {
		for i := lo; i < hi; i += stepVal {
			out = append(out, elements[int(i)])
		}
	} else {
		for i := lo; i > hi; i += stepVal {
			out = append(out, elements[int(i)])
		}
	}
	return out
}

func sliceRunes(rs []rune, lowPtr *int64, highPtr *int64, stepVal int64) []rune {
	length := int64(len(rs))
	lo, hi := sliceBounds(lowPtr, highPtr, stepVal, length)
	out := make([]rune, 0)
	if stepVal > 0 {
		for i := lo; i < hi; i += stepVal {
			out = append(out, rs[int(i)])
		}
	} else {
		for i := lo; i > hi; i += stepVal {
			out = append(out, rs[int(i)])
		}
	}
	return out
}
