package lsp

import (
	"strings"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Pos struct {
	Line int
	Col  int
}

func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

func endPosByte(text string) Pos {
	line := 1
	col := 1
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return Pos{Line: line, Col: col}
}

func posLessEq(a, b Pos) bool {
	if a.Line < b.Line {
		return true
	}
	if a.Line > b.Line {
		return false
	}
	return a.Col <= b.Col
}

func posWithin(p Pos, start Pos, end Pos) bool {
	return posLessEq(start, p) && posLessEq(p, end)
}

func byteColToUTF16(lineText string, byteCol int) uint32 {
	if byteCol <= 1 {
		return 0
	}
	limit := byteCol - 1
	if limit > len(lineText) {
		limit = len(lineText)
	}
	var count uint32
	for _, r := range lineText[:limit] {
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		count += uint32(n)
	}
	return count
}

func utf16ColToByte(lineText string, utf16Col int) int {
	if utf16Col <= 0 {
		return 1
	}
	count := 0
	for idx, r := range lineText {
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		if count+n > utf16Col {
			return idx + 1
		}
		count += n
	}
	return len(lineText) + 1
}

func utf16Len(s string) int {
	count := 0
	for _, r := range s {
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		count += n
	}
	return count
}

func positionToByte(text string, pos protocol.Position) (Pos, bool) {
	lines := splitLines(text)
	lineIdx := int(pos.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return Pos{}, false
	}
	lineText := lines[lineIdx]
	byteCol := utf16ColToByte(lineText, int(pos.Character))
	return Pos{Line: lineIdx + 1, Col: byteCol}, true
}

func rangeFromPosLenUTF16(text string, line int, col int, literal string) protocol.Range {
	lines := splitLines(text)
	if line <= 0 || line > len(lines) {
		return protocol.Range{}
	}
	lineText := lines[line-1]
	start := protocol.Position{Line: uint32(line - 1), Character: byteColToUTF16(lineText, col)}
	length := utf16Len(literal)
	end := protocol.Position{Line: start.Line, Character: start.Character + uint32(max(1, length))}
	return protocol.Range{Start: start, End: end}
}
