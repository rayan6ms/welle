package astfmt

import "strings"

type Comment struct {
	Raw        string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	IsBlock    bool
	HasNewline bool
}

func scanComments(src string) []Comment {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	var comments []Comment
	line, col := 1, 0
	advance := func(ch byte) {
		if ch == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}

	i := 0
	for i < len(src) {
		ch := src[i]
		if ch == '"' {
			// triple quote
			if i+2 < len(src) && src[i+1] == '"' && src[i+2] == '"' {
				advance('"')
				advance('"')
				advance('"')
				i += 3
				for i < len(src) {
					if src[i] == '"' && i+2 < len(src) && src[i+1] == '"' && src[i+2] == '"' {
						advance('"')
						advance('"')
						advance('"')
						i += 3
						break
					}
					advance(src[i])
					i++
				}
				continue
			}
			// normal string
			advance('"')
			i++
			for i < len(src) {
				if src[i] == '\\' {
					advance(src[i])
					i++
					if i < len(src) {
						advance(src[i])
						i++
					}
					continue
				}
				if src[i] == '"' {
					advance(src[i])
					i++
					break
				}
				if src[i] == '\n' {
					advance(src[i])
					i++
					break
				}
				advance(src[i])
				i++
			}
			continue
		}
		if ch == '`' {
			advance('`')
			i++
			for i < len(src) {
				if src[i] == '`' {
					advance('`')
					i++
					break
				}
				advance(src[i])
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(src) {
			next := src[i+1]
			if next == '/' {
				startLine, startCol := line, col+1
				start := i
				advance('/')
				advance('/')
				i += 2
				for i < len(src) && src[i] != '\n' {
					advance(src[i])
					i++
				}
				raw := src[start:i]
				comments = append(comments, Comment{
					Raw:        raw,
					StartLine:  startLine,
					StartCol:   startCol,
					EndLine:    line,
					EndCol:     col,
					IsBlock:    false,
					HasNewline: false,
				})
				continue
			}
			if next == '*' {
				startLine, startCol := line, col+1
				start := i
				advance('/')
				advance('*')
				i += 2
				hasNewline := false
				for i < len(src) {
					if src[i] == '*' && i+1 < len(src) && src[i+1] == '/' {
						advance('*')
						advance('/')
						i += 2
						break
					}
					if src[i] == '\n' {
						hasNewline = true
					}
					advance(src[i])
					i++
				}
				raw := src[start:i]
				comments = append(comments, Comment{
					Raw:        raw,
					StartLine:  startLine,
					StartCol:   startCol,
					EndLine:    line,
					EndCol:     col,
					IsBlock:    true,
					HasNewline: hasNewline,
				})
				continue
			}
		}

		advance(ch)
		i++
	}

	return comments
}
