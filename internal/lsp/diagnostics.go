package lsp

import (
	"welle/internal/diag"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// LSP positions are 0-based.
func toLspPosition(line1, col1 int) protocol.Position {
	line := uint32(0)
	char := uint32(0)
	if line1 > 0 {
		line = uint32(line1 - 1)
	}
	if col1 > 0 {
		char = uint32(col1 - 1)
	}
	return protocol.Position{Line: line, Character: char}
}

func ToLspDiagnostics(ds []diag.Diagnostic) []protocol.Diagnostic {
	out := make([]protocol.Diagnostic, 0, len(ds))
	for _, d := range ds {
		start := toLspPosition(d.Range.Line, d.Range.Col)
		end := start
		if d.Range.Length > 0 {
			end.Character = start.Character + uint32(d.Range.Length)
		} else {
			end.Character = start.Character + 1
		}

		severity := protocol.DiagnosticSeverityError
		switch d.Severity {
		case diag.SeverityWarning:
			severity = protocol.DiagnosticSeverityWarning
		case diag.SeverityInfo:
			severity = protocol.DiagnosticSeverityInformation
		}

		pd := protocol.Diagnostic{
			Range:    protocol.Range{Start: start, End: end},
			Severity: &severity,
			Source:   ptrString("welle"),
			Message:  d.Message,
		}
		if d.Code != "" {
			code := protocol.IntegerOrString{Value: d.Code}
			pd.Code = &code
		}
		out = append(out, pd)
	}
	return out
}

func ptrString(s string) *string { return &s }
