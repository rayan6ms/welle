# Welle

Welle is a small, dynamically typed programming language with curly-brace blocks, newline/semicolon statement separators, a tree-walk interpreter, and an optional bytecode VM. It also ships with a formatter, linter, and a VS Code extension (syntax highlighting + LSP features).

> Status: early but usable. The language is evolving; see the spec and “VM limitations” below.

- Full language spec: **docs/spec.md**
- Formatting verification notes: **docs/formatting.md**
- VS Code extension source: **vscode-welle/**

---

## Features (today)

### Language
- Curly-brace blocks: `{ ... }`
- Newlines are statement separators (like `;`); semicolons are supported
- Variables, assignments, and expressions
- Control flow: `if/else`, `while`, `for (...)`, `break`, `continue`
- `switch` statement and `match` expression
- Named functions (`func name(...) { ... }`) + closures (captures for reads)
- Arrays (`[...]`), dicts (`#{...}`), indexing, slicing (strings slice by Unicode code points)
- Exceptions: `throw`, `try/catch/finally`, and `defer` (LIFO)

### Tooling
- CLI runner + REPL
- Formatter: `welle fmt`
- Linter: `welle lint`
- Language Server (LSP): diagnostics, semantic tokens, go-to-definition, document symbols, quick fixes, formatting

---

## Install

### Option A: Build from source (recommended right now)
You need Go installed.

```bash
# From the repo root
go test ./...
go build -o bin/welle ./cmd/welle
go build -o bin/welle-lsp ./cmd/welle-lsp

# Run
./bin/welle run examples/is_palindrome.wll
./bin/welle repl
```

### Option B: Use GitHub Releases

If I ever finish the repo releases with prebuilt binaries, download the latest one and put it on your `PATH`.

---

## Quick start

Create a file `hello.wll`:

```welle
print("Hello from Welle!")
```

Run:

```bash
welle run hello.wll
# or (if you built locally)
./bin/welle run hello.wll
```

---

## CLI usage

Run a file (or a “spec” string if you pass code directly):

```bash
welle [run] <pathOrSpec>
```

Start REPL:

```bash
welle repl
# or just:
welle
```

Useful flags:

* `-tokens` print lexer tokens
* `-ast` print AST
* `-vm` run using the bytecode VM
* `-dis` dump VM bytecode (implies `-vm`)
* `-O` enable bytecode optimizer (VM only)

Subcommands:

* `welle repl`
* `welle gfx [pathOrSpec]`
* `welle init [--name <name>] [--entry <file>] [--force]`
* `welle fmt [-w] [-i <indent>] <path|dir>`
* `welle lint <file|dir>...`
* `welle tools install [--bin <dir>]`

---

## Formatter (`welle fmt`)

Format a file to stdout:

```bash
welle fmt examples/is_palindrome.wll
```

Write changes in place:

```bash
welle fmt -w examples/is_palindrome.wll
```

Format a directory:

```bash
welle fmt -w examples
```

Notes:

* Token-based formatter (not AST-based)
* Normalizes spacing around operators/punctuation
* Puts `{` and `}` on their own lines and indents blocks
* Converts `;` into line breaks
* Collapses multiple blank lines to at most one
* Ensures a trailing newline

---

## Linter (`welle lint`)

```bash
welle lint examples
```

Warnings:

* `WL0001` unused variable
* `WL0002` unused parameter
* `WL0003` unreachable code (after `return` or `throw` in a block)
* `WL0004` variable shadows outer variable (enabled by default)

Parser errors use code `WP0001`.

---

## Interpreter vs VM (important)

Welle has two execution engines:

* **Interpreter** (tree-walk) — the most complete feature set today.
* **VM** (bytecode) — faster and powers the REPL/LSP, but has some gaps.

Known VM limitations (current):

* The bytecode compiler does **not** support:

  * logical operators `and` / `or`
  * string operators (e.g. string concatenation with `+`)
* `for-in` is **interpreter-only** (`for x in expr { ... }`)
* Assigning to a captured free variable is unsupported (compile error)

If you hit a limitation, try running without `-vm`:

```bash
welle run myfile.wll
# instead of:
welle -vm run myfile.wll
```

---

## GFX mode

Welle includes a small graphics API exposed via builtins and stdlib (`std:gfx`, `std:image`, `std:noise`, etc.).

Run GFX scripts using:

```bash
welle gfx examples/gfx_demo.wll
```

A typical pattern is:

* `setup()` called once (open window, allocate buffers)
* `draw()` called every frame (begin_frame → draw → end_frame)

Check `examples/gfx_*.wll` for working demos.

---

## Standard library

The `std/` folder contains Welle modules you can import, e.g.:

```welle
import "std:math" as math
import "std:rand" as rand
import "std:color" as color
import "std:noise" as noise
import "std:gfx" as gfx
import "std:image" as image
```

See **docs/spec.md** for the full list of builtins and stdlib functions.

---

## VS Code extension

The extension lives in `vscode-welle/` and provides:

* Syntax highlighting for `.wll`
* Language configuration (brackets, comments, etc.)
* LSP features (diagnostics, semantic tokens, go-to-definition, symbols, quick fixes, formatting)

### Build & package (VSIX)

From `vscode-welle/`:

```bash
npm install
npx vsce package
```

The extension’s prepublish script builds the bundled LSP server automatically:

* `npm run build:lsp` → builds `cmd/welle-lsp` into `vscode-welle/server/welle-lsp`
* `vscode:prepublish` also runs `chmod +x server/welle-lsp`

Then install the produced `.vsix` in VS Code:

* Extensions view → `...` → **Install from VSIX...**

### Formatting in VS Code

Open a `.wll` file and run:

* **Format Document** (Shift+Alt+F)

If formatting does not trigger:

* Ensure the file extension is `.wll`
* Check the Output panel for “Welle” logs
* Make sure your extension is activated and the LSP started

See **docs/formatting.md** for the smoke test.

---

## Optional: Welle file icon in VS Code

This repo includes the Welle icon at:

* `assets/welle.svg`

If you use the “VSCode Icons” extension (or a similar icon theming setup), you can associate `.wll` files to use the Welle icon.

Example snippet to add to your VS Code user settings (typically `~/.config/Code/User/settings.json` on Linux):

```json
"vsicons.associations.files": [
  {
    "icon": "welle",
    "extensions": ["wll"],
    "format": "svg"
  }
]
```

Then ensure your icon theme knows where to find the SVG. A simple approach is to copy `assets/welle.svg` into the icon theme’s custom icons directory (varies by theme), and register `"welle"` to that SVG.

> Icon theme configuration differs across themes; the snippet above is the file-association part.

---

## Project layout

* `cmd/welle/` — CLI entrypoint
* `cmd/welle-lsp/` — language server (LSP)
* `internal/` — lexer/parser/evaluator/compiler/vm/formatter/linter/LSP internals
* `std/` — Welle stdlib modules
* `examples/` — runnable Welle programs
* `docs/` — spec and development notes
* `vscode-welle/` — VS Code extension

---

## Contributing

Issues and PRs are welcome.

* Please include repro steps and expected/actual behavior.
* For language changes, update **docs/spec.md** when behavior changes.
* Add/adjust tests under `internal/**` where appropriate.

---

## License

GPLv3 (see [LICENSE](LICENSE))
