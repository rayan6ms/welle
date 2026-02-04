package diag

import "fmt"

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

type Range struct {
	Line   int // 1-based
	Col    int // 1-based
	Length int // best-effort; can be 1 if unknown
}

type Diagnostic struct {
	Code     string
	Message  string
	Severity Severity
	Range    Range
}

func (d Diagnostic) Format(path string) string {
	if d.Code != "" {
		return fmt.Sprintf("%s:%d:%d: %s %s: %s", path, d.Range.Line, d.Range.Col, d.Severity.String(), d.Code, d.Message)
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", path, d.Range.Line, d.Range.Col, d.Severity.String(), d.Message)
}
