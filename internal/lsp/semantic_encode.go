package lsp

import "sort"

func EncodeSemanticTokens(toks []SemTok) []uint32 {
	// Sort by (line, col)
	sort.Slice(toks, func(i, j int) bool {
		if toks[i].Line != toks[j].Line {
			return toks[i].Line < toks[j].Line
		}
		return toks[i].Col < toks[j].Col
	})

	var data []uint32
	prevLine := 1
	prevCol := 1

	for _, t := range toks {
		// LSP is 0-based internally, but we store 1-based. Convert here.
		line0 := t.Line - 1
		col0 := t.Col - 1

		prevLine0 := prevLine - 1
		prevCol0 := prevCol - 1

		deltaLine := line0 - prevLine0
		deltaStart := col0
		if deltaLine == 0 {
			deltaStart = col0 - prevCol0
		}

		if t.Length <= 0 {
			continue
		}

		data = append(data,
			uint32(deltaLine),
			uint32(deltaStart),
			uint32(t.Length),
			uint32(t.Type),
			uint32(t.Mods),
		)

		prevLine = t.Line
		prevCol = t.Col
	}

	return data
}
