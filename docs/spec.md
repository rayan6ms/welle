# Welle Language Spec (current)

## 1) Overview
Welle is a small, dynamically typed language with curly-brace blocks, newline/semicolon statement separators, a tree-walk interpreter, and an optional bytecode VM; the syntax and behaviors below reflect what is implemented today in the lexer/parser, evaluator, compiler, and VM.

## 2) Lexical structure

### Comments
- Line comments: `//` to end of line.
- Block comments: `/* ... */` (not nested).

```welle
// line comment
/* block
   comment */
```

### Whitespace and newlines
- Spaces, tabs, and `\r` are skipped.
- Newlines are significant tokens and separate statements (like `;`).
- `;` is an explicit statement separator (also required inside `for (...)` headers).

### Identifiers
- Start: ASCII letter, `_`, or a Unicode letter (byte >= 128 and `unicode.IsLetter`).
- Continue: identifier start chars or digits.
- Case-sensitive.

### Keywords (complete list)
`func`, `return`, `break`, `continue`, `if`, `else`, `while`, `for`, `in`, `true`, `false`, `nil`, `and`, `or`, `not`, `import`, `from`, `as`, `try`, `catch`, `finally`, `throw`, `defer`, `export`, `switch`, `match`, `case`, `default`

### Literals
- Integers: base-10 (`123`)
- Floats: decimal `digits '.' digits` (leading digits required, trailing digits required; `.5` and `1.` are not supported)
- Strings:
  - Double-quoted with minimal escapes
  - Raw strings with backticks
  - Triple-quoted strings with `"""`
- Booleans: `true`, `false`
- Nil: `nil`

Numbers are either integers or floats; both are truthy (even `0`).

#### Escape sequences (double-quoted strings only)
- `\"`, `\\`, `\n`, `\t`
- Unknown escapes keep the backslash literal (e.g., `\q` stays `\q`)
- Newlines are not allowed inside `"..."` (unterminated string is an error).

```welle
s1 = "line\nbreak"
s2 = `raw \n not escaped`
s3 = """multi
line"""
```

## 3) Syntax and semantics

### Expressions
Supported operators and precedence (low to high):

| Level | Operators |
| --- | --- |
| 1 | `or` |
| 2 | `and` |
| 3 | `==`, `!=` |
| 4 | `<`, `<=`, `>`, `>=` |
| 5 | `+`, `-` |
| 6 | `*`, `/`, `%` |
| 7 | prefix `-`, `not` |
| 8 | indexing `[...]` |
| 9 | calls `(...)`, member access `.` |

Notes:
- `and`/`or` are short-circuiting and return booleans based on truthiness (not the original operand values).
- `not` is the only logical negation operator; `!` is illegal unless part of `!=`.
- Truthiness: only `false` and `nil` are falsy; everything else is truthy.
- Division or modulo by zero raises an error.
- Parentheses group expressions.

Operator typing rules (interpreter):
- Numbers:
  - Integers: `+ - * / %`, comparisons `== != < <= > >=`
  - Floats: `+ - * /`, comparisons `== != < <= > >=`
  - Mixed int/float arithmetic promotes to float.
  - Integer division uses truncation (`5 / 2` -> `2`); division by zero errors.
  - `%` is integer-only; using it with floats is an error.
- String: `+` (concatenation), `==`, `!=`
- Boolean: `==`, `!=`
- `nil` only compares with `==`/`!=` (with `nil` or other types)
- Mismatched types in binary operators are errors

```welle
ok = not (1 < 2 and 0)
```

VM note: the bytecode compiler does not support `and`, `or`, or string operators; those work only in the interpreter.

### Statements and blocks
- Programs are sequences of statements separated by NEWLINE or `;`.
- Blocks are `{ ... }` and can contain zero or more statements.
- Expression statements evaluate and discard the result.

```welle
x = 1
y = 2; z = 3
```

### Variables and assignment
- Assignment: `name = expression`
  - Interpreter: updates nearest existing variable; otherwise defines in current scope.
  - VM: assignments are local/global only; assigning to a captured free variable is unsupported (compile error).
- Index assignment: `arr[i] = v`, `dict[key] = v`
- Member assignment: `dict.field = v` (property is a string key)

```welle
a = [1, 2, 3]
a[1] = 99

d = #{"a": 1}
d["b"] = 2
d.count = 3
```

### Control flow
- `if (cond) { ... } else { ... }` (parentheses required)
- `while (cond) { ... }`
- `for` (two forms):
  - C-style: `for (init; cond; post) { ... }`
    - `init` and `post` are simple assignments (`name = expr`) or omitted.
    - `cond` is any expression or omitted (treated as `true`).
  - For-in: `for x in expr { ... }` or `for (x in expr) { ... }`
    - Interpreter supports arrays and dicts (dict iterates keys sorted by internal hash-key string).
    - VM does not support `for-in`.
- `break` exits loops and `switch`; `continue` advances loops only.

```welle
if (x > 3) { print("big") } else { print("small") }

i = 0
while (i < 3) {
  print(i)
  i = i + 1
}

// C-style for
sum = 0
for (i = 0; i < 10; i = i + 1) {
  sum = sum + i
}

// for-in (interpreter only)
sum = 0
for x in range(5) { sum = sum + x }

for (i = 0; i < 6; i = i + 1) {
  if (i == 2) { continue }
  if (i == 4) { break }
  print(i)
}
```

#### switch statement
```
switch (expr) {
  case v1, v2 { ... }
  default { ... }
}
```
- No fallthrough.
- `break` exits the switch.
- Case comparisons use `==` and will error on type mismatches.

```welle
switch (x) {
  case 1, 3 { print("odd") }
  default { print("other") }
}
```

#### match expression
```
match (expr) {
  case v1, v2 { resultExpr }
  default { resultExpr }
}
```
- Returns the first matching case result.
- If no case matches and there is no `default`, the result is `nil`.
- Case bodies are single expressions (not statement blocks).
- Matching uses `==` and errors on type mismatches.

```welle
grade = match (score) {
  case 9 { "B" }
  default { "A" }
}
```

### Functions
- Declaration: `func name(params) { ... }`
- Only named function declarations exist; there is no function literal syntax.
- Functions are first-class values and can be returned.
- Closures capture outer bindings for reads.

```welle
func makeAdder(x) {
  func add(y) { return x + y }
  return add
}
```

### Data structures
- Lists: `[a, b, c]`
- Dicts: `#{ key: value, ... }` (keys must be string, integer, or boolean)
- Images: `Image` objects are created via `image_new(width, height)` and store an RGBA byte buffer.
- Indexing:
  - Arrays/strings use integer indices (negative indices count from the end).
  - Dicts return `nil` for missing keys.
  - Array/string indices out of range raise an error.
- Member access: `dict.field` uses the string key `"field"` and errors if missing.
- Slicing: `a[low:high]`, `a[:high]`, `a[low:]`
  - Bounds are clamped; if `low > high`, the result is empty.
  - Supported on arrays and strings.
  - Strings index/slice by Unicode code points.

```welle
a = [10, 20, 30]
print(a[-1])
print(a[1:])

d = #{"a": 1, "b": 2}
print(d["b"])

s = "café"
print(s[-1])
print(s[1:3])
```

### Errors, throw/try/catch/finally, and defer
- `throw expr` raises an error.
  - Interpreter: wraps `expr.Inspect()` as the error message.
  - VM: if `expr` is an Error, it is thrown as-is; strings become message; others use `Inspect()`.
- `try { ... } catch (e) { ... } finally { ... }`
  - `catch` is optional, `finally` is optional, but at least one must be present.
  - `catch` binds the error object to the identifier.
  - `finally` always runs; if it errors, it overrides the prior result.
- `defer` registers a call to run when the current function returns.
  - LIFO order.
  - Runs on normal return and on thrown errors.
  - `defer` must wrap a call expression and is only valid inside a function.

```welle
func f() {
  defer cleanup()
  throw "boom"
}
try { f() } catch (e) { print("caught") }
```

```welle
out = ""
try { out = out + "try" } catch (e) { out = out + "catch" } finally { out = out + "finally" }
```

VM note: in VM mode, error objects expose `message`, `code`, and `stack` members (e.g., `e.message`); the interpreter does not support member access on errors.

## 4) Modules

### Import syntax
- `import "path" [as alias]`
  - If `as` is omitted, the binding name is the file stem.
- `from "path" import name[, name as alias]`

```welle
import "std:math" as math
from "math.wll" import add, PI as pi
```

### Export syntax
- `export name = expr`
- `export func name(...) { ... }`
- Only assignments and function declarations are supported after `export`.

```welle
export PI = 3
export func add(a, b) { return a + b }
```

### Resolution rules
Imports are resolved by `internal/module`:
- `std:<name>` maps to `<stdRoot>/<name>.wll`.
- `./`, `../`, or absolute paths resolve relative to the importing file (adds `.wll` if missing).
- Bare names try `<stdRoot>/<name>.wll` first, then extra search paths.

### Export behavior
Modules export only names marked with `export`. If a module contains no exports, importing it yields an empty module dict. `from`-imports must match an exported member, or they error.

## 5) Builtins and stdlib

### Builtins (interpreter + VM)
Functions in `internal/evaluator/builtins.go` and `internal/vm/builtins.go`:
- `print(...args) -> nil`  
  Prints `Inspect()` of each argument; returns error if any argument is an error.
- `len(x) -> int`  
  Supports string, array, and dict; wrong type or arg count is an error.
- `str(x) -> string`  
  Returns `Inspect()` as a string.
- `keys(dict) -> [key]`  
  Returns keys sorted by internal hash-key string.
- `values(dict) -> [value]`  
  Returns values sorted by the same order as `keys`.
- `range(n)`, `range(start, end)`, `range(start, end, step) -> [int]`  
  Integers only; `step` cannot be 0; end is exclusive.
- `append(array, value) -> [any]`  
  Returns a new array; errors if first arg is not array.
- `push(array, value) -> [any]`  
  Alias of `append`.
- `hasKey(dict, key) -> bool`  
  Errors if key is not hashable.
- `sort(array) -> [any]`  
  Returns a new array sorted; supports all-int or all-string arrays only.
- `error(message, code?) -> Error`  
  Constructs an error object without throwing.
- `writeFile(path, content) -> nil`  
  Writes a string to disk; errors if path/content are not strings or write fails.
- `math_floor(x) -> int`  
  Returns the floor of a number.
- `math_sqrt(x) -> float`  
  Returns the square root.
- `math_sin(x) -> float`  
  Returns the sine (radians).
- `math_cos(x) -> float`  
  Returns the cosine (radians).
- `gfx_open(width:int, height:int, title:string) -> nil`  
  Creates/sets a window for gfx mode; errors if gfx backend is not running.
- `gfx_close() -> nil`  
  Requests the gfx loop to exit.
- `gfx_shouldClose() -> bool`  
  Returns true if the window is closing (or gfx is not running).
- `gfx_beginFrame() -> nil`  
  Clears the command list and resets clear color to black.
- `gfx_endFrame() -> nil`  
  Ends the current frame (no-op placeholder).
- `gfx_clear(r:number, g:number, b:number, a:number) -> nil`  
  Sets the frame clear color; channels are 0..255.
- `gfx_rect(x:number, y:number, w:number, h:number, r:number, g:number, b:number, a:number) -> nil`  
  Draws a filled rectangle in the current frame.
- `gfx_pixel(x:int, y:int, r:int, g:int, b:int, a:int) -> nil`  
  Draws a single pixel.
- `gfx_present(image:Image) -> nil`  
  Uploads the Image RGBA buffer and draws it fullscreen in the current frame.
- `gfx_time() -> number`  
  Seconds since gfx start.
- `gfx_keyDown(key:string) -> bool`  
  Returns true if a key is pressed; supported keys include letters `a`-`z`, digits `0`-`9`, and `space`, `enter`, `escape`, `left`, `right`, `up`, `down`, `shift`, `ctrl`, `alt`.
- `gfx_mouseX() -> int`, `gfx_mouseY() -> int`  
  Current mouse position in window coordinates.
  Gfx builtins require running via `welle gfx`; otherwise they return an Error (or `gfx_shouldClose()` returns true).
- Render loop pattern: call `gfx_beginFrame()` at the start of each `draw`, issue draw/present commands, then call `gfx_endFrame()`; `gfx_present()` should be called between begin/end.
- `image_new(width:int, height:int) -> Image`  
  Creates an RGBA pixel buffer (width/height must be positive).
- `image_set(image:Image, x:int, y:int, r:int, g:int, b:int, a:int) -> nil`  
  Sets a pixel (bounds-checked, channels 0..255).
- `image_fill(image:Image, r:int, g:int, b:int, a:int) -> nil`  
  Fills the entire buffer.
- `image_fill_rect(image:Image, x:int, y:int, w:int, h:int, r:int, g:int, b:int, a:int) -> nil`  
  Fills a rectangle (bounds-checked, `w`/`h` must be positive).
- `image_fade(image:Image, amount:number) -> nil`  
  Multiplies each channel by `1 - amount` (amount in 0..1).
- `image_width(image:Image) -> int`, `image_height(image:Image) -> int`  
  Returns image dimensions.

Interpreter-only methods (via `obj.method(...)` in `internal/evaluator/evaluator.go`):
- Array: `append(value)`, `len()`
- Dict: `keys()`, `values()`, `hasKey(key)`
- String: `len()`

```welle
a = [1, 2]
a = a.append(3)
print(a.len())
```

```welle
e = error("bad", 123)
print(e)
```

### stdlib modules
Files in `std/` are normal Welle modules:
- `std:math`
  - `add(a, b)`, `sub(a, b)`, `floor(x)`, `sqrt(x)`, `sin(x)`, `cos(x)`
- `std:strings`
  - `length(s)`, `is_empty(s)`
- `std:rand`
  - `seed(n)`, `int(max)`, `range(min, max)`
- `std:color`
  - `rgb(r, g, b)`, `lerp(c1, c2, t)`, `distance(c1, c2)`
- `std:noise`
  - `noise2(x, y, scale, time) -> int (0..1000)`
- `std:gfx`
  - `open(w, h, title)`, `close()`, `should_close()`, `begin_frame()`, `end_frame()`
  - `clear(r, g, b, a)`, `rect(x, y, w, h, r, g, b, a)`, `pixel(x, y, r, g, b, a)`
  - `present(image)`, `time()`, `key_down(k)`, `mouse_x()`, `mouse_y()`
- `std:image`
  - `new(w, h)`, `set(img, x, y, r, g, b, a)`, `fill(img, r, g, b, a)`
  - `fill_rect(img, x, y, w, h, r, g, b, a)`, `fade(img, amount)`
  - `width(img)`, `height(img)`

Example (animated noise grid):

```welle
import "std:gfx" as gfx
import "std:noise" as noise
import "std:color" as color
import "std:math" as math

func setup() {
  gfx.open(320, 320, "Welle GFX Noise")
}

func draw() {
  gfx.begin_frame()
  gfx.clear(12, 16, 24, 255)

  t = math.floor(gfx.time() * 8)
  cell = 8
  scale = 32
  c1 = color.rgb(24, 40, 80)
  c2 = color.rgb(230, 240, 255)

  for (y in range(0, 320, cell)) {
    for (x in range(0, 320, cell)) {
      v = noise.noise2(x, y, scale, t)
      c = color.lerp(c1, c2, v)
      gfx.rect(x, y, cell, cell, c["r"], c["g"], c["b"], 255)
    }
  }

  gfx.end_frame()
}
```

Example (animated perlin image buffer):

```welle
import "std:gfx" as gfx
import "std:image" as image
import "std:noise" as noise
import "std:color" as color
import "std:math" as math

width = 320
height = 240
img = nil
palette = [
  color.rgb(12, 20, 32),
  color.rgb(24, 60, 96),
  color.rgb(96, 180, 200),
  color.rgb(240, 250, 255),
]

func setup() {
  gfx.open(width, height, "Welle Perlin Image")
  img = image.new(width, height)
}

func draw() {
  gfx.begin_frame()
  t = math.floor(gfx.time() * 6)
  scale = 48
  seg_size = 334

  for (y = 0; y < height; y = y + 1) {
    for (x = 0; x < width; x = x + 1) {
      v = noise.noise2(x + (t * 2), y + t, scale, t)
      seg = v / seg_size
      if (seg > 2) { seg = 2 }
      blend = (v - (seg * seg_size)) * 1000 / seg_size
      c1 = palette[seg]
      c2 = palette[seg + 1]
      c = color.lerp(c1, c2, blend)
      image.set(img, x, y, c["r"], c["g"], c["b"], 255)
    }
  }

  gfx.present(img)
  gfx.end_frame()
}
```

## 6) Tooling

### CLI usage
`welle [run] [pathOrSpec]` runs a file or spec; no args starts the REPL.

Flags:
- `-tokens` print tokens
- `-ast` print AST
- `-vm` run using bytecode VM
- `-dis` dump VM bytecode (implies `-vm`)
- `-O` enable bytecode optimizer (VM only)

Subcommands:
- `welle repl`
- `welle gfx [pathOrSpec]`
- `welle init [--name <name>] [--entry <file>] [--force]`
- `welle fmt [-w] [-i <indent>] <path|dir>`
- `welle lint <file|dir>...`
- `welle tools install [--bin <dir>]`

### REPL
- Uses the VM compiler/runtime (same limitations as `-vm`).
- Multiline input continues while braces/parentheses are unbalanced or inside strings.
- `exit` and `quit` leave the REPL.
- Prints the last non-`nil` expression result.

### Formatter (`welle fmt`)
Token-based formatter (`internal/format`):
- Normalizes spacing around operators and punctuation.
- Puts `{` and `}` on their own lines and indents blocks.
- Converts `;` into line breaks.
- Collapses multiple blank lines to at most one.
- Ensures a trailing newline.

### Linter
Diagnostics from `internal/lint` (warnings):
- `WL0001` unused variable
- `WL0002` unused parameter
- `WL0003` unreachable code (after `return` or `throw` in a block)
- `WL0004` variable shadows outer variable (enabled by default)

Parser errors use code `WP0001`.

### LSP (`welle-lsp`)
Implemented features:
- Diagnostics (parser + linter)
- Semantic tokens
- Go-to-definition for identifiers and `alias.member` imports
- Document symbols
- Code actions for `WL0001`/`WL0002`/`WL0003` (prefix `_` or remove line)

## 7) Not implemented yet
- Floating-point or non-decimal numeric literals (only base-10 integers exist).
- Unary `!` operator (only `not` is supported).
- Function literals/anonymous `func` expressions (only named `func` statements).

## 8) Appendix: Complete keyword/operator/token list

### Keywords
`func`, `return`, `break`, `continue`, `if`, `else`, `while`, `for`, `in`, `true`, `false`, `nil`, `and`, `or`, `not`, `import`, `from`, `as`, `try`, `catch`, `finally`, `throw`, `defer`, `export`, `switch`, `match`, `case`, `default`

### Operators
`=`, `+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `and`, `or`, `not`, `.`

### Delimiters and separators
Separators: `NEWLINE`, `;`  
Delimiters: `#`, `,`, `:`, `(`, `)`, `[`, `]`, `{`, `}`
