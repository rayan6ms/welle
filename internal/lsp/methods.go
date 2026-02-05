package lsp

type MethodInfo struct {
	Name      string
	Signature string
	Doc       string
	Params    []string
}

var methodDocs = map[string]MethodInfo{
	"strip": {
		Name:      "strip",
		Signature: "strip() -> string",
		Doc:       "Removes leading and trailing whitespace.",
		Params:    []string{},
	},
	"capitalize": {
		Name:      "capitalize",
		Signature: "capitalize() -> string",
		Doc:       "Uppercases the first Unicode code point and lowercases the rest.",
		Params:    []string{},
	},
	"uppercase": {
		Name:      "uppercase",
		Signature: "uppercase() -> string",
		Doc:       "Uppercases the string using Unicode-aware case mapping.",
		Params:    []string{},
	},
	"lowercase": {
		Name:      "lowercase",
		Signature: "lowercase() -> string",
		Doc:       "Lowercases the string using Unicode-aware case mapping.",
		Params:    []string{},
	},
	"startswith": {
		Name:      "startswith",
		Signature: "startswith(prefix) -> bool",
		Doc:       "True if the string begins with prefix.",
		Params:    []string{"prefix"},
	},
	"endswith": {
		Name:      "endswith",
		Signature: "endswith(suffix) -> bool",
		Doc:       "True if the string ends with suffix.",
		Params:    []string{"suffix"},
	},
	"slice": {
		Name:      "slice",
		Signature: "slice(low?, high?) -> string",
		Doc:       "Returns a substring using the same rules as s[low:high].",
		Params:    []string{"low?", "high?"},
	},
}

func methodInfo(name string) *MethodInfo {
	if info, ok := methodDocs[name]; ok {
		return &info
	}
	return nil
}
