package token

type Type string

type Token struct {
	Type    Type
	Literal string
	// Raw preserves the original lexeme when Literal is normalized (e.g., strings).
	Raw  string
	Line int
	Col  int
}

const (
	// Special
	ILLEGAL Type = "ILLEGAL"
	EOF     Type = "EOF"

	// Separators
	NEWLINE   Type = "NEWLINE"
	SEMICOLON Type = ";"

	// Identifiers + literals
	IDENT    Type = "IDENT"
	INT      Type = "INT"
	FLOAT    Type = "FLOAT"
	STRING   Type = "STRING"
	TEMPLATE Type = "TEMPLATE"

	// Keywords
	FUNC     Type = "FUNC"
	RETURN   Type = "RETURN"
	BREAK    Type = "BREAK"
	CONTINUE Type = "CONTINUE"
	IF       Type = "IF"
	ELSE     Type = "ELSE"
	WHILE    Type = "WHILE"
	FOR      Type = "FOR"
	IN       Type = "IN"
	TRUE     Type = "TRUE"
	FALSE    Type = "FALSE"
	NIL      Type = "NIL"
	AND      Type = "AND"
	OR       Type = "OR"
	NOT      Type = "NOT"
	IS       Type = "IS"
	IMPORT   Type = "IMPORT"
	FROM     Type = "FROM"
	AS       Type = "AS"
	TRY      Type = "TRY"
	CATCH    Type = "CATCH"
	FINALLY  Type = "FINALLY"
	THROW    Type = "THROW"
	DEFER    Type = "DEFER"
	EXPORT   Type = "EXPORT"
	SWITCH   Type = "SWITCH"
	MATCH    Type = "MATCH"
	CASE     Type = "CASE"
	DEFAULT  Type = "DEFAULT"
	PASS     Type = "PASS"

	// Operators
	ASSIGN   Type = "="
	WALRUS   Type = ":="
	PLUS     Type = "+"
	MINUS    Type = "-"
	STAR     Type = "*"
	SLASH    Type = "/"
	PERCENT  Type = "%"
	BANG     Type = "!"
	BITOR    Type = "|"
	BITAND   Type = "&"
	BITXOR   Type = "^"
	BITNOT   Type = "~"
	SHL      Type = "<<"
	SHR      Type = ">>"
	QUESTION Type = "?"
	NULLISH  Type = "??"

	PLUS_ASSIGN    Type = "+="
	MINUS_ASSIGN   Type = "-="
	STAR_ASSIGN    Type = "*="
	SLASH_ASSIGN   Type = "/="
	PERCENT_ASSIGN Type = "%="
	BITOR_ASSIGN   Type = "|="

	EQ Type = "=="
	NE Type = "!="
	LT Type = "<"
	LE Type = "<="
	GT Type = ">"
	GE Type = ">="

	// Delimiters
	HASH     Type = "#"
	COMMA    Type = ","
	COLON    Type = ":"
	DOT      Type = "."
	ELLIPSIS Type = "..."
	LPAREN   Type = "("
	RPAREN   Type = ")"
	LBRACKET Type = "["
	RBRACKET Type = "]"
	LBRACE   Type = "{"
	RBRACE   Type = "}"
)

var keywords = map[string]Type{
	"func":     FUNC,
	"return":   RETURN,
	"break":    BREAK,
	"continue": CONTINUE,
	"if":       IF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"true":     TRUE,
	"false":    FALSE,
	"nil":      NIL,
	"null":     NIL,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"is":       IS,
	"import":   IMPORT,
	"from":     FROM,
	"as":       AS,
	"try":      TRY,
	"catch":    CATCH,
	"finally":  FINALLY,
	"throw":    THROW,
	"defer":    DEFER,
	"export":   EXPORT,
	"switch":   SWITCH,
	"match":    MATCH,
	"case":     CASE,
	"default":  DEFAULT,
	"pass":     PASS,
}

func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
