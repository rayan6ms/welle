package evaluator

import "welle/internal/limits"

type RuntimeContext struct {
	File   string
	Stack  []stackFrame
	Budget *limits.Budget
}

var ctx = &RuntimeContext{}

type stackFrame struct {
	Func string
	File string
	Line int
	Col  int
}
