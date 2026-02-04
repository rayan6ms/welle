# Welle (VS Code Extension)

Language support for **Welle** (`.wll`).

This extension provides:
- Syntax highlighting
- Bracket/indentation rules
- Semantic tokens
- Diagnostics (parser + linter)
- Go to Definition (identifiers and `alias.member` imports)
- Document Symbols
- Quick Fixes for common lints
- **Format Document** (Shift+Alt+F) via the bundled `welle-lsp`

---

## Requirements

- VS Code **1.80+**
- Linux/macOS/Windows: the extension bundles a `welle-lsp` binary for your platform (if provided in the package).  
  If the bundled server isn’t available for your platform, install/build `welle-lsp` and point the extension to it (see below).

---

## Getting started

1. Install the extension.
2. Open any `.wll` file.
3. You should see diagnostics and semantic highlighting automatically.
4. Use **Format Document** (Shift+Alt+F) to format the current file.

---

## Formatting

The language server provides formatting. You can trigger it via:
- **Shift+Alt+F** (Format Document)
- Format on Save (optional)

To enable format on save, add to your VS Code settings:

```jsonc
"[welle]": {
  "editor.formatOnSave": true
}
```

---

## Configure the language server path (optional)

By default, the extension tries:

1. your setting `welle.lspPath`
2. `<workspace>/bin/welle-lsp`
3. `welle-lsp` on `PATH`

To override the server path:

```jsonc
{
  "welle.lspPath": "/absolute/path/to/welle-lsp"
}
```

---

## Using the official Welle icon with VSCode Icons (optional)

If you use the **VSCode Icons** extension, you can associate `.wll` with the Welle icon.

1. Copy `welle.svg` from this extension (or from the repo) into a local folder, e.g.:
   `~/.config/Code/User/icons/welle.svg`

2. Add this to your VS Code `settings.json`:

```jsonc
"vsicons.associations.files": [
  {
    "icon": "welle",
    "extensions": ["wll"],
    "format": "svg"
  }
]
```

(Depending on your icon theme setup, you may also need to register the icon asset in the theme’s custom icon configuration.)

---

## Known limitations

* Some language features may behave differently depending on whether you run Welle in interpreter mode or VM mode (see the language spec).
* Formatting is provided by the LSP and follows the formatter rules in the Welle project.

---

## Links

* Welle repository: [https://github.com/rayan6ms/welle](https://github.com/rayan6ms/welle)
* Issues: [https://github.com/rayan6ms/welle/issues](https://github.com/rayan6ms/welle/issues)

---

## License

GPLv3 (see [LICENSE](LICENSE))
