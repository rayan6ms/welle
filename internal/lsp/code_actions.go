package lsp

import (
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Convert 0-based LSP position -> absolute index in string.
// NOTE: This assumes ASCII; for UTF-16 correctness we will upgrade later.
func indexFromPos(text string, pos protocol.Position) int {
	line := int(pos.Line)
	ch := int(pos.Character)

	i := 0
	curLine := 0
	for curLine < line && i < len(text) {
		if text[i] == '\n' {
			curLine++
		}
		i++
	}
	return min(i+ch, len(text))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func lineLengths(text string) []int {
	lines := strings.Split(text, "\n")
	lengths := make([]int, 0, len(lines))
	for _, line := range lines {
		lengths = append(lengths, len(line))
	}
	return lengths
}

func MakeRemoveLineAction(uri string, text string, r protocol.Range, title string) (protocol.CodeAction, bool) {
	startLine := int(r.Start.Line)
	lengths := lineLengths(text)
	if startLine < 0 || startLine >= len(lengths) {
		return protocol.CodeAction{}, false
	}

	endLine := startLine
	endChar := uint32(lengths[startLine])
	if startLine+1 < len(lengths) {
		endLine = startLine + 1
		endChar = 0
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri(uri): {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(startLine), Character: 0},
						End:   protocol.Position{Line: uint32(endLine), Character: endChar},
					},
					NewText: "",
				},
			},
		},
	}

	kind := protocol.CodeActionKindQuickFix
	return protocol.CodeAction{
		Title: title,
		Kind:  &kind,
		Edit:  &edit,
	}, true
}

func MakePrefixUnderscoreAction(uri string, text string, r protocol.Range) (protocol.CodeAction, bool) {
	start := indexFromPos(text, r.Start)
	end := indexFromPos(text, r.End)
	if end <= start {
		end = start + 1
		if end > len(text) {
			end = len(text)
		}
	}

	ident := strings.TrimSpace(text[start:end])
	if ident == "" || strings.HasPrefix(ident, "_") {
		return protocol.CodeAction{}, false
	}

	newIdent := "_" + ident
	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.DocumentUri(uri): {
				{
					Range:   r,
					NewText: newIdent,
				},
			},
		},
	}

	kind := protocol.CodeActionKindQuickFix
	return protocol.CodeAction{
		Title: "Prefix with '_' to mark unused",
		Kind:  &kind,
		Edit:  &edit,
	}, true
}
