package object

const (
	memPtrSize      int64 = 8
	memStringHead   int64 = 24
	memArrayHead    int64 = 24
	memTupleHead    int64 = 24
	memDictHead     int64 = 32
	memDictEntry    int64 = 24
	memImageHead    int64 = 24
	memErrorHead    int64 = 32
	memFunctionHead int64 = 64
	memClosureHead  int64 = 32
	memCellHead     int64 = 16
	memImagePixel   int64 = 4
)

func CostStringBytes(n int) int64 {
	if n < 0 {
		return memStringHead
	}
	return memStringHead + int64(n)
}

func CostArray(n int) int64 {
	if n < 0 {
		return memArrayHead
	}
	return memArrayHead + int64(n)*memPtrSize
}

func CostArrayElements(n int) int64 {
	if n <= 0 {
		return 0
	}
	return int64(n) * memPtrSize
}

func CostTuple(n int) int64 {
	if n < 0 {
		return memTupleHead
	}
	return memTupleHead + int64(n)*memPtrSize
}

func CostDict(n int) int64 {
	if n < 0 {
		return memDictHead
	}
	return memDictHead + int64(n)*memDictEntry
}

func CostDictEntry() int64 {
	return memDictEntry
}

func CostImage(width, height int) int64 {
	if width <= 0 || height <= 0 {
		return memImageHead
	}
	pixels := int64(width) * int64(height)
	const maxInt64 = int64(^uint64(0) >> 1)
	if pixels < 0 || pixels > maxInt64/memImagePixel {
		return maxInt64
	}
	return memImageHead + pixels*memImagePixel
}

func CostError() int64 {
	return memErrorHead
}

func CostFunction() int64 {
	return memFunctionHead
}

func CostClosure(numFree int) int64 {
	if numFree < 0 {
		return memClosureHead
	}
	return memClosureHead + int64(numFree)*memPtrSize
}

func CostCell() int64 {
	return memCellHead
}
