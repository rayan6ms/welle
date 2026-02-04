# Formatting Verification

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
