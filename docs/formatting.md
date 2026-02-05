# Formatting Verification

Welle ships two formatting modes:
- Default token-based formatter (`internal/format`), used by `welle fmt` with no flags.
- AST-aware formatter (`internal/format/astfmt`), enabled with `welle fmt --ast` (experimental).

### Comment and Blank-Line Preservation (AST mode)
- Line (`//`) and block (`/* */`) comments are preserved and kept in relative position.
- Inline comments on the same line as code stay on that line when possible.
- Blank lines between statement “paragraphs” are preserved (collapsed to a single blank line).
- Limitations: multi-line string literals are emitted as-is and can affect subsequent line/column tracking.

## VS Code Smoke Test
1. Build the language server:
   - `go build ./cmd/welle-lsp`
2. Open this repository in VS Code.
3. Open any `.wll` file (e.g. `examples/is_palindrome.wll`).
4. Make the file intentionally messy (remove spaces around `=` or `{`).
5. Run `Format Document` (Shift+Alt+F).
6. Expect the document to be reformatted with proper spacing and indentation.

If formatting does not trigger, confirm the server started in the Output panel
for the Welle extension and that the file extension is `.wll`.
