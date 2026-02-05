package lsp

type BuiltinInfo struct {
	Name      string
	Signature string
	Doc       string
	Params    []string
}

var builtinDocs = map[string]BuiltinInfo{
	"print": {
		Name:      "print",
		Signature: "print(...args) -> nil",
		Doc:       "Prints Inspect() of each argument.",
		Params:    []string{"...args"},
	},
	"len": {
		Name:      "len",
		Signature: "len(x) -> int",
		Doc:       "Supports string, array, and dict; wrong type or arg count is an error.",
		Params:    []string{"x"},
	},
	"str": {
		Name:      "str",
		Signature: "str(x) -> string",
		Doc:       "Converts a value to string.",
		Params:    []string{"x"},
	},
	"join": {
		Name:      "join",
		Signature: "join(array, sep) -> string",
		Doc:       "Joins an array of strings with a separator.",
		Params:    []string{"array", "sep"},
	},
	"keys": {
		Name:      "keys",
		Signature: "keys(dict) -> [key]",
		Doc:       "Returns keys sorted by internal hash-key string.",
		Params:    []string{"dict"},
	},
	"values": {
		Name:      "values",
		Signature: "values(dict) -> [value]",
		Doc:       "Returns values sorted by the same order as keys().",
		Params:    []string{"dict"},
	},
	"range": {
		Name:      "range",
		Signature: "range(n) | range(start, end) | range(start, end, step) -> [int]",
		Doc:       "Creates a list of ints from start to end (exclusive).",
		Params:    []string{"n|start", "end?", "step?"},
	},
	"append": {
		Name:      "append",
		Signature: "append(array, value) -> [any]",
		Doc:       "Returns a new array; errors if first arg is not array.",
		Params:    []string{"array", "value"},
	},
	"push": {
		Name:      "push",
		Signature: "push(array, value) -> [any]",
		Doc:       "Alias of append.",
		Params:    []string{"array", "value"},
	},
	"count": {
		Name:      "count",
		Signature: "count(array, value) -> int",
		Doc:       "Counts occurrences using ==; errors if equality comparison errors.",
		Params:    []string{"array", "value"},
	},
	"remove": {
		Name:      "remove",
		Signature: "remove(array, value) -> bool",
		Doc:       "Removes first matching element and returns true/false.",
		Params:    []string{"array", "value"},
	},
	"get": {
		Name:      "get",
		Signature: "get(dict, key, default?) -> any",
		Doc:       "Returns value if present; otherwise default or nil.",
		Params:    []string{"dict", "key", "default?"},
	},
	"pop": {
		Name:      "pop",
		Signature: "pop(array) -> any | pop(dict, key, default?) -> any",
		Doc:       "Array pop removes last element; dict pop removes by key.",
		Params:    []string{"array|dict", "key?", "default?"},
	},
	"hasKey": {
		Name:      "hasKey",
		Signature: "hasKey(dict, key) -> bool",
		Doc:       "Returns true if dict has key.",
		Params:    []string{"dict", "key"},
	},
	"sort": {
		Name:      "sort",
		Signature: "sort(array) -> [any]",
		Doc:       "Returns a new array sorted; supports all-int or all-string arrays only.",
		Params:    []string{"array"},
	},
	"max": {
		Name:      "max",
		Signature: "max(array) -> number|string",
		Doc:       "Returns max element; supports all-number (int/float) or all-string arrays.",
		Params:    []string{"array"},
	},
	"abs": {
		Name:      "abs",
		Signature: "abs(x) -> number",
		Doc:       "Absolute value of int or float.",
		Params:    []string{"x"},
	},
	"sum": {
		Name:      "sum",
		Signature: "sum(array) -> number",
		Doc:       "Sums numeric elements; empty array returns 0.",
		Params:    []string{"array"},
	},
	"reverse": {
		Name:      "reverse",
		Signature: "reverse(array|string) -> array|string",
		Doc:       "Returns a new reversed array or string.",
		Params:    []string{"array|string"},
	},
	"any": {
		Name:      "any",
		Signature: "any(array) -> bool",
		Doc:       "True if any element is truthy (only false/nil are falsy).",
		Params:    []string{"array"},
	},
	"all": {
		Name:      "all",
		Signature: "all(array) -> bool",
		Doc:       "True if all elements are truthy; empty array returns true.",
		Params:    []string{"array"},
	},
	"error": {
		Name:      "error",
		Signature: "error(message, code?) -> Error",
		Doc:       "Constructs an error object without throwing.",
		Params:    []string{"message", "code?"},
	},
	"writeFile": {
		Name:      "writeFile",
		Signature: "writeFile(path, content) -> nil",
		Doc:       "Writes a string to disk; errors if path/content are not strings or write fails.",
		Params:    []string{"path", "content"},
	},
	"sqrt": {
		Name:      "sqrt",
		Signature: "sqrt(x) -> float",
		Doc:       "Square root; same behavior as math_sqrt.",
		Params:    []string{"x"},
	},
	"input": {
		Name:      "input",
		Signature: "input(prompt?) -> string",
		Doc:       "Reads a line from stdin; errors in non-interactive mode.",
		Params:    []string{"prompt?"},
	},
	"getpass": {
		Name:      "getpass",
		Signature: "getpass(prompt?) -> string",
		Doc:       "Reads a line from stdin without echo when possible; errors in non-interactive mode.",
		Params:    []string{"prompt?"},
	},
	"group_digits": {
		Name:      "group_digits",
		Signature: "group_digits(x, sep=\",\", group=3) -> string",
		Doc:       "Groups integer digits from the right. x may be int or digit string with optional underscores.",
		Params:    []string{"x", "sep?", "group?"},
	},
	"format_float": {
		Name:      "format_float",
		Signature: "format_float(x, decimals) -> string",
		Doc:       "Formats a number with fixed decimals and deterministic rounding.",
		Params:    []string{"x", "decimals"},
	},
	"format_percent": {
		Name:      "format_percent",
		Signature: "format_percent(x, decimals) -> string",
		Doc:       "Formats x*100 with decimals and appends '%'.",
		Params:    []string{"x", "decimals"},
	},
}

func builtinInfo(name string) *BuiltinInfo {
	if info, ok := builtinDocs[name]; ok {
		return &info
	}
	return nil
}
