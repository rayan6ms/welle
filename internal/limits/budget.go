package limits

import "fmt"

const MemoryErrorCode int64 = 8001

type Budget struct {
	limit int64
	used  int64
}

func NewBudget(limit int64) *Budget {
	if limit < 0 {
		limit = 0
	}
	return &Budget{limit: limit}
}

func (b *Budget) Limit() int64 {
	if b == nil {
		return 0
	}
	return b.limit
}

func (b *Budget) Used() int64 {
	if b == nil {
		return 0
	}
	return b.used
}

func MaxMemoryMessage(limit int64) string {
	return fmt.Sprintf("max memory exceeded (%d bytes)", limit)
}

type MaxMemoryError struct {
	Limit int64
}

func (e MaxMemoryError) Error() string {
	return MaxMemoryMessage(e.Limit)
}

func (b *Budget) Charge(n int64) error {
	if b == nil || b.limit == 0 {
		return nil
	}
	if n <= 0 {
		return nil
	}
	if b.used+n > b.limit {
		return MaxMemoryError{Limit: b.limit}
	}
	b.used += n
	return nil
}
