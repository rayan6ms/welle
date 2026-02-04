package evaluator

type RuntimeContext struct {
	File  string
	Stack []stackFrame
}

var ctx = &RuntimeContext{}

type stackFrame struct {
	Func string
	File string
	Line int
	Col  int
}
