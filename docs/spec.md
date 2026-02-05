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
- `_` followed immediately by a digit is reserved (treated as an invalid numeric literal).
- Continue: identifier start chars or digits.
- Case-sensitive.

### Keywords (complete list)
`func`, `return`, `break`, `continue`, `pass`, `if`, `else`, `while`, `for`, `in`, `true`, `false`, `nil`, `null`, `and`, `or`, `not`, `is`, `import`, `from`, `as`, `try`, `catch`, `finally`, `throw`, `defer`, `export`, `switch`, `match`, `case`, `default`

### Literals
- Integers:
  - Decimal digits (`123`)
  - Base prefixes: binary `0b1010`, octal `0o755`, hex `0xFF`
  - Numeric separators: underscores between digits (`1_000`, `0xFF_FF`)
- Floats:
  - Decimal `digits '.' digits` (leading digits required, trailing digits required; `.5` and `1.` are not supported)
  - Exponent notation: `1e3`, `1.2e-3`, `10E+2`
  - Numeric separators: underscores between digits (`3.141_592`, `1_2.3_4`, `1e3`, `1.2e-3`)
- Strings:
  - Double-quoted with minimal escapes
  - Raw strings with backticks
  - Triple-quoted strings with `"""`
  - Template literals with `t"..."` and interpolation `${expr}`
- Booleans: `true`, `false`
- Nil: `nil` (alias: `null`). Both represent the Nil value; the canonical printed form is `nil` (formatters normalize to `nil`).

Numbers are either integers or floats; both are truthy (even `0`).

Numeric separator rules:
- Underscores are ignored for numeric value but must separate digits.
- Underscores are not allowed at the start or end of a digit sequence, doubled, or adjacent to the decimal point or exponent marker/sign.
- Base-prefixed integers only allow digits valid for that base.

Numeric literal errors:
- Invalid digit for the base (e.g., `0b102`).
- Malformed underscores (e.g., `1__2`, `1_.2`, `1e_3`, `0x_FF`).
- Exponent missing digits (`1e`, `1e+`, `1e-`).
- Integer or float literal overflow (outside 64-bit range).

#### Escape sequences (double-quoted strings only)
- `\"`, `\\`, `\n`, `\t`
- Unknown escapes keep the backslash literal (e.g., `\q` stays `\q`)
- Newlines are not allowed inside `"..."` (unterminated string is an error).

```welle
s1 = "line\nbreak"
s2 = `raw \n not escaped`
s3 = """multi
line"""
s4 = t"hello ${name}!"
```

#### Template literals
- Delimiter: `t"..."`.
- Interpolation: `${ expr }`.
- Template body escape rules are the same as normal double-quoted strings: `\"`, `\\`, `\n`, `\t`.
- Newlines are not allowed directly inside `t"..."` (use `\n`).
- Untagged template:
  - Evaluates interpolation expressions left-to-right exactly once.
  - Produces a string by concatenating literal parts and `str(value)` behavior (`Inspect()` text).
- Tagged template:
  - Syntax: `tag t"...${...}..."`.
  - `tag` is evaluated first and must be callable.
  - Call shape: `tag(strings, ...values)`.
  - `strings` is a tuple of literal parts (`len(strings) = len(values)+1`).
  - `values` are interpolation results without string coercion.

## 3) Syntax and semantics

### Expressions
Supported operators and precedence (low to high):

| Level | Operators                                         |
| ----- | ------------------------------------------------- |
| 1     | assignment `=`, `:=`, `+=`, `-=`, `*=`, `/=`, `%=`, `|=` |
| 2     | nullish coalescing `??`                           |
| 3     | ternary `?:`, conditional expr `a if cond else b` |
| 4     | `or`                                              |
| 5     | `and`                                             |
| 6     | bitwise OR `\|`                                   |
| 7     | bitwise XOR `^`                                   |
| 8     | bitwise AND `&`                                   |
| 9     | `==`, `!=`, `is`                                  |
| 10    | `<`, `<=`, `>`, `>=`, `in`                        |
| 11    | shifts `<<`, `>>`                                 |
| 12    | `+`, `-`                                          |
| 13    | `*`, `/`, `%`                                     |
| 14    | prefix `-`, `not`, `!`, `~`                       |
| 15    | indexing `[...]`                                  |
| 16    | calls `(...)`, member access `.`                  |

Notes:
- `and`/`or` are short-circuiting and return booleans based on truthiness (not the original operand values).
- `??` is short-circuiting and returns the original left operand if it is not `nil`; otherwise it evaluates and returns the right operand.
- `not` and `!` are logical negation operators (`!` is an alias for `not`).
- `!` is only valid as a unary operator or as part of `!=`.
- Formatting: prefix `!` is emitted without a space (`!x`, `!(a and b)`). The AST formatter preserves `!` vs `not` based on the parsed operator.
- Assignment is right-associative and has the lowest precedence.
- Walrus `:=` is assignment-like, right-associative, and shares assignment precedence.
- Truthiness: only `false` and `nil` are falsy; everything else is truthy.
- Division or modulo by zero raises an error.
- Parentheses group expressions.
- Tuple literals use parentheses with commas: `(a, b, c)`, `()`, `(x,)`.
  - `(x)` is grouping, not a tuple.

#### Ternary conditional
Syntax: `<cond> ? <thenExpr> : <elseExpr>`

Semantics:
- Evaluate `cond` first.
- If `cond` is truthy, evaluate and return `thenExpr` (do not evaluate `elseExpr`).
- If `cond` is falsy, evaluate and return `elseExpr` (do not evaluate `thenExpr`).
- Right-associative: `a ? b : c ? d : e` parses as `a ? b : (c ? d : e)`.

#### Conditional expression (`if`/`else`)
Syntax: `<thenExpr> if <cond> else <elseExpr>`

Semantics:
- Evaluate `cond` first.
- If `cond` is truthy, evaluate and return `thenExpr` (do not evaluate `elseExpr`).
- If `cond` is falsy, evaluate and return `elseExpr` (do not evaluate `thenExpr`).
- Right-associative: `a if c1 else b if c2 else c` parses as `a if c1 else (b if c2 else c)`.
- Precedence: lower than `or`, higher than assignment (same level as `?:`).

#### Nullish coalescing
Syntax: `<left> ?? <right>`

Semantics:
- Evaluate `left` first.
- If `left` is `nil` (including `null`), evaluate and return `right`.
- Otherwise return `left` without evaluating `right`.
- Right-associative: `a ?? b ?? c` parses as `a ?? (b ?? c)`.
- Precedence: higher than assignment, lower than `or`.

```welle
name = user.name ?? "guest"
print(false ?? 1)   // false
print(false or 1)   // true (because `or` returns a boolean)
```

Operator typing rules (interpreter):
- Numbers:
  - Integers: `+ - * / %`, comparisons `== != < <= > >=`
  - Integers: bitwise `| & ^ ~ << >>` (int-only)
  - Floats: `+ - * /`, comparisons `== != < <= > >=`
  - Mixed int/float arithmetic promotes to float.
  - Integer division uses truncation (`5 / 2` -> `2`); division by zero errors.
  - `%` is integer-only; using it with floats is an error.
- String: `+` (concatenation), `*` (repeat by integer count; `"a" * 3` and `3 * "a"`), `==`, `!=`
- Boolean: `==`, `!=`
- Tuple: `==`, `!=` compare element-wise (lengths must match); other operators error.
- `nil` only compares with `==`/`!=` (with `nil` or other types)
- Mismatched types in binary operators are errors
- Identity operator `is` (deterministic; never type-errors):
  - `nil`: only `nil is nil` is true.
  - booleans: value identity (`true is true`, `false is false`).
  - numbers: type-sensitive value identity (`1 is 1` true, `1 is 1.0` false).
  - strings: value identity by exact content/code points.
  - arrays/dicts/tuples/functions/errors/images/closures/cells/builtins: reference identity (same object instance).
- Bitwise operators:
  - Operate on integers only; any non-integer operand is a runtime error.
  - `~` is bitwise NOT on int64 (two’s complement).
  - Shifts `<<` and `>>` require an integer shift count in range `0..63`.
    - Negative shift counts raise `shift count cannot be negative`.
    - Counts >= 64 raise `shift count out of range`.
  - Left shift is defined as `int64(uint64(a) << b)`; right shift is arithmetic (`a >> b`).

#### Membership operator `in`
Syntax: `x in y` (expression, left-associative; precedence with comparisons).

Semantics (returns boolean):
- If `y` is an array: true if any element equals `x` using `==` semantics. If `==` errors for any element, the error propagates.
- If `y` is a string: `x` must be a string; true if `x` is a substring of `y` (byte-based substring search; works with UTF-8 strings).
- If `y` is a dict: true if the dict has key `x` (same hashable-key rules as dict indexing).
- Any other `y` type: error `cannot use 'in' with <type>`.

Errors:
- String RHS but non-string LHS: `left operand of 'in' must be string when right operand is string`.
- Dict RHS with unhashable key: `unusable as dict key: <type>` (same as dict indexing).

```welle
ok = not (1 < 2 and 0)
alt = !(1 < 2 and 0)
bits = 5 | 2
mask = 5 & 2
flip = 5 ^ 2
not0 = ~0
shifted = 1 << 3
```

VM note: the bytecode compiler/runtime aims to match interpreter semantics for operators and control flow listed below.
Optimizer note: the VM optimizer applies constant folding and peephole passes and is verified by tests that compare optimized vs unoptimized execution plus unit tests for each pass. Division/modulo by zero is never folded away.

### Statements and blocks
- Programs are sequences of statements separated by NEWLINE or `;`.
- Blocks are `{ ... }` and can contain zero or more statements.
- Expression statements evaluate and discard the result.
- `pass` is a no-op statement placeholder.

```welle
x = 1
y = 2; z = 3
```

### Variables and assignment
- Assignment: `name = expression`
  - Updates the nearest existing variable; otherwise defines in the current scope.
  - Assignment expressions evaluate to the assigned value and can be used inside larger expressions.
- Walrus: `name := expression`
  - Always defines in the current scope and returns the assigned value.
  - If `name` already exists in the current scope, it errors: `cannot redeclare "<name>" in this scope`.
  - `:=` only accepts identifier targets.
- Compound assignment: `+=`, `-=`, `*=`, `/=`, `%=`, `|=` for variables, index, and member assignments.
  - `+=`, `-=`, `*=`, `/=`, `%=` are equivalent to `a = a <op> b` (same errors and numeric behavior).
  - `|=` updates dicts in-place by copying entries from the RHS dict; overlapping keys are overwritten.
  - Errors: `|= left operand must be dict`, `|= right operand must be dict`.
  - Evaluation order for index/member compound assignment:
    1) evaluate base and index/member key once
    2) read the old value once
    3) evaluate RHS once
    4) compute operation
    5) write back
- Index assignment: `arr[i] = v`, `dict[key] = v`
- Member assignment: `dict.field = v` (property is a string key)
- Destructuring assignment: `(a, b) = expr`
  - `expr` must evaluate to a tuple or array.
  - Targets must be identifiers or `_` (discard).
  - Star-unpacking is allowed exactly once: `(a, *mid, b) = expr`
    - Left side has `N` targets with one star target.
    - Right side must have length `>= N-1`.
    - Star target receives an array of the “middle” items (possibly empty).
    - `_` still discards; `*_` discards the middle.
  - Errors:
    - more than one starred target -> parse error (`WP0001`)
    - non-sequence RHS with star -> `cannot unpack non-sequence`
    - too-short RHS with star -> `not enough values to unpack (expected at least X, got Y)`

```welle
a = [1, 2, 3]
a[1] = 99

d = #{"a": 1}
d["b"] = 2
d.count = 3

x = 1
x += 2
a[0] *= 3
d.count -= 1

print(x = 3)
a = b = 7
x = (y = 2) + 1

(left, right) = (1, 2)
(x, _) = (3, 4)
```

### Control flow
- `if (cond) { ... } else { ... }` (block form; parentheses required)
- `if (cond) stmt` or `if (cond) stmt else stmt` (single-statement form; `stmt` is exactly one statement, blocks require `{ ... }`)
- `while (cond) { ... }`
- `for` (two forms):
  - C-style: `for (init; cond; post) { ... }`
    - `init` and `post` are assignments (`name = expr` or compound) or omitted.
    - `cond` is any expression or omitted (treated as `true`).
  - For-in: `for x in expr { ... }` or `for (x in expr) { ... }`
    - `expr` may be an array, string, or dict (dict iteration order is deterministic; see Dict ordering below).
    - When iterating an array, `x` is bound to each element.
    - When iterating a string, `x` is bound to each Unicode code point as a 1-length string.
    - When iterating a dict, `x` is bound to each key.
  - For-in destructuring (dict-only): `for (k, v) in dictExpr { ... }`
    - Iterates dict keys in deterministic order.
    - `k` is bound to the key, `v` is bound to the value for that key.
    - `_` may be used as a discard target and does not create a binding.
    - Only `(k, v)` (two targets) is valid. `(a)` is not a binding pattern, and `(a, b, c)` is invalid.
    - Runtime error if the right-hand side is not a dict.
- `break` exits loops and `switch`; `continue` advances loops only.

```welle
if (x > 3) { print("big") } else { print("small") }
if (x > 3) print("big") else print("small")

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

// for-in
sum = 0
for x in range(5) { sum = sum + x }

d = #{"b": 2, "a": 1}
for (k, v) in d {
  print(k)
  print(v)
}
for (k, _) in d { print(k) }

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
- Functions are first-class values and can be returned.
- Closures capture outer bindings for reads and writes; assigning to a captured variable updates the shared binding in both the interpreter and VM.
- Function literals (anonymous functions) are expressions:
  - Syntax: `func(params) { ... }`
  - Example: `f = func(x) { return x + 1 }`
- Return statements:
  - `return` yields `nil`.
  - `return expr` yields that value.
  - `return a, b` yields a tuple value `(a, b)`; multiple return values are not implicitly unpacked at call sites (use `...` to spread a tuple into call arguments).

#### Call argument spread (tuples/arrays)
- Syntax: `f(...tupleExpr)` or `f(1, ...t, 9)`.
- Spread is only valid inside call argument lists.
- Supported types: tuples and arrays (arrays expand in order).
- Evaluation is left-to-right; each spread expression is evaluated exactly once.
- If the spread value is not a tuple/array, it raises a runtime error: `cannot spread <type> in call arguments`.

```welle
func makeAdder(x) {
  func add(y) { return x + y }
  return add
}

inc = func(x) { return x + 1 }
print(inc(2))

print((func(x) { return x * 2 })(21))
```

### Data structures
- Tuples: `(a, b, c)` (immutable, fixed-size, ordered)
  - Created via tuple literals or multi-value `return`.
  - Not indexable or mutable in v0.1.
- Lists: `[a, b, c]`
  - List comprehensions: `[expr for i in sequence]`
    - Optional filter: `[expr for i in sequence if cond]`
    - `expr` may be a conditional expression: `[(a if cond else b) for i in sequence]`
    - `sequence` must be an array, string, or dict:
      - array: iterates elements
      - string: iterates Unicode code points as 1-length strings
      - dict: iterates keys in deterministic order
    - Evaluation order:
      1) evaluate `sequence` once
      2) iterate in order
      3) per item: bind `i`, evaluate filter (if present), then evaluate `expr`
      4) append to a new list
    - The loop variable `i` is scoped to the comprehension and does not leak.
    - Errors: wrong `sequence` type -> `cannot iterate <type> in comprehension`
- Dicts: `#{ key: value, ... }` (keys must be string, integer, or boolean)
  - Shorthand entries are allowed for bare identifiers: `#{ name, age }` is equivalent to `#{ "name": name, "age": age }`.
  - Shorthand and explicit entries can be mixed.
  - Duplicate keys are last-wins in source order.
  - Deterministic iteration order for dict keys:
    - Type order: `bool` < `int` < `string`.
    - Within type: `false < true`, integers ascending, strings lexicographic by Unicode code point.
  - `for (k in dict)`, `keys(dict)`, and `values(dict)` all use this order.
- Images: `Image` objects are created via `image_new(width, height)` and store an RGBA byte buffer.
- Indexing:
  - Arrays/strings use integer indices (negative indices count from the end).
  - Dicts return `nil` for missing keys.
  - Array/string indices out of range raise an error.
- Member access: `dict.field` uses the string key `"field"` and errors if missing.
- Slicing: `a[low:high]`, `a[:high]`, `a[low:]`, `a[low:high:step]`, `a[::step]`
  - Supported on arrays and strings.
  - Strings index/slice by Unicode code points.
  - `step` must be a non-zero integer.
  - Defaults:
    - `step > 0`: `low = 0`, `high = len`
    - `step < 0`: `low = len-1`, `high = -1`
  - Bounds are clamped (Python-like); with `step > 0`, if `low > high` the result is empty.
  - Examples: `a[::2]`, `a[1:5:2]`, `a[::-1]`
  - Errors:
    - `slice step cannot be 0`
    - non-integer indices or step are errors (consistent with index/slice typing rules)

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
  - If `expr` is an Error, it is thrown as-is.
  - If `expr` is a string, the message is the string value.
  - Otherwise the message uses `Inspect()`.
- Error objects expose members: `message` (string), `code` (int, default `0`), and `stack` (string); member access works in both interpreter and VM.
- Stack traces include anonymous function names as `<anon@line:col>`.
- `try { ... } catch (e) { ... } finally { ... }`
  - `catch` is optional, `finally` is optional, but at least one must be present.
  - `catch` binds the error object to the identifier.
  - `finally` always runs; if it errors, it overrides the prior result.
- `defer` registers a call to run when the current function returns.
  - LIFO order.
  - Runs on normal return and on thrown errors.
  - `defer` must wrap a call expression. The interpreter requires `defer` to appear inside a function; the VM allows it at top level (it runs when the entry frame returns).

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
- Bare names try `<stdRoot>/<name>.wll` first, then module search paths.

Module search paths:
- `module_paths` from `welle.toml` (if present; prepended in order).
- The current working directory.
- The project root (directory containing `welle.toml`), if different from the current working directory.

`welle.toml` (project file) keys:
- `entry = "main.wll"` (required for `welle run <dir>` and `welle gfx <dir>`)
- `std_root = "path/to/std"` (optional, overrides default `<cwd>/std`)
- `module_paths = ["path/one", "path/two"]` (optional, searched before cwd/project root)
- `max_recursion = 1000` (optional, max function call depth; `0` = unlimited)
- `max_steps = 1_000_000` (optional, max VM instruction count; `0` = unlimited)
- `max_mem = 100_000_000` (optional, max allocation budget in bytes; `0` = unlimited)

Config precedence:
- CLI flags (if any) override `welle.toml`.
- `welle.toml` overrides defaults.

### Export behavior
Modules export only names marked with `export`. If a module contains no exports, importing it yields an empty module dict. `from`-imports must match an exported member, or they error.

### Module caching and cycles
- Each module is loaded at most once per run; subsequent imports reuse the cached module exports.
- Import cycles are detected and reported with error code `WM0001` and a chain like `A -> B -> A`.

### Import errors
- Missing module: includes the module spec and attempted resolved paths.
- Missing export: `missing export "<name>" in module "<spec>"`.
- Duplicate export: errors on multiple `export` of the same name, reporting both source locations when available.

## 5) Builtins and stdlib

### Builtins (interpreter + VM)
Functions in `internal/evaluator/builtins.go` and `internal/vm/builtins.go`:
- `print(...args) -> nil`  
  Prints `Inspect()` of each argument. In the interpreter, if any argument is an Error object, it propagates that error instead of printing; the VM always prints and returns `nil`.
- `len(x) -> int`  
  Supports string, array, and dict; wrong type or arg count is an error.
- `str(x) -> string`  
  Returns `Inspect()` as a string.
- `group_digits(x, sep=",", group=3) -> string`
  - `x` is INTEGER or digit STRING (underscores allowed in string input and ignored).
  - Groups digits from the right; negative ints keep `-`.
  - Errors on invalid digit strings or `group <= 0`.
- `format_float(x, decimals) -> string`
  - `x` is int/float, `decimals` is integer `>= 0`.
  - Deterministic fixed-point formatting with `.` decimal separator.
- `format_percent(x, decimals) -> string`
  - Equivalent to `format_float(x*100, decimals) + "%"`.
- `join(array, sep) -> string`  
  Joins an array of strings with a separator.
- `keys(dict) -> [key]`  
  Returns keys in deterministic dict order (bool < int < string; false < true; ints asc; strings lexicographic).
- `values(dict) -> [value]`  
  Returns values in the same order as `keys`.
- `range(n)`, `range(start, end)`, `range(start, end, step) -> [int]`  
  Integers only; `step` cannot be 0; end is exclusive. Negative `step` is allowed (iterates while `i > end`).
- `append(array, value) -> [any]`  
  Returns a new array; errors if first arg is not array.
- `push(array, value) -> [any]`  
  Alias of `append`.
- `count(array, value) -> int`  
  Counts occurrences using `==`; errors if equality comparison errors.
- `remove(array, value) -> bool`  
  Removes the first matching element (mutates array); returns true if removed, false if not found.
- `get(dict, key, default?) -> any`  
  Returns value if present; otherwise returns `default` or `nil`. Errors if key is not hashable.
- `pop(array) -> any`  
  Removes and returns the last element (error on empty).
- `pop(dict, key, default?) -> any`  
  Removes and returns the value if present; if missing returns `default` or errors.
- `hasKey(dict, key) -> bool`  
  Errors if key is not hashable.
- `sort(array) -> [any]`  
  Returns a new array sorted; supports all-int or all-string arrays only.
- `max(array) -> number|string`  
  Returns the maximum element. Arrays must be all-number (int/float) or all-string. Empty array is an error.
- `abs(x) -> number`  
  Absolute value for int/float.
- `sum(array) -> number`  
  Sums numeric elements (int/float mix allowed). Empty array returns `0`.
- `reverse(array|string) -> array|string`  
  Returns a new reversed array or string (string reversal is by Unicode code points).
- `any(array) -> bool`  
  True if any element is truthy; empty array returns false.
- `all(array) -> bool`  
  True if all elements are truthy; empty array returns true.
- `map(fn, array) -> [any]`  
  Applies `fn` to each element and returns a new array. `fn` must be callable; evaluation order is left-to-right.
- `mean(array) -> number`  
  Arithmetic mean of numeric elements. Accepts int/float (mixed allowed). Returns int if the mean is an integer and inputs are all int; otherwise returns float. Empty arrays are an error.
- `error(message, code?) -> Error`  
  Constructs an error object without throwing.
- `writeFile(path, content) -> nil`  
  Writes a string to disk; errors if path/content are not strings or write fails.
- `input(prompt?) -> string`  
  Reads a line from stdin. If stdin is not interactive/available (e.g., tests), it raises `input is not available in non-interactive mode`. Prompt is printed without a trailing newline.
- `getpass(prompt?) -> string`  
  Reads a line from stdin without echo when possible; if not possible, falls back to normal read. If stdin is not interactive/available (e.g., tests), it raises `getpass is not available in non-interactive mode`. Prompt is printed without a trailing newline.
- `sqrt(x) -> float`  
  Alias of `math_sqrt` (same type/arity/negative-input behavior).
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
- `image_fade_white(image:Image, amount:number) -> nil`  
  Blends each channel toward 255 (amount in 0..1).
- `image_width(image:Image) -> int`, `image_height(image:Image) -> int`  
  Returns image dimensions.

Methods (interpreter + VM) via `obj.method(...)`:
- Array: `append(value)`, `len()`, `count(value)`, `pop()`, `remove(value)`
- Dict: `keys()`, `values()`, `hasKey(key)`, `count()`, `get(key, default?)`, `pop(key, default?)`, `remove(key)`
- String: `len()`, `strip()`, `uppercase()`, `lowercase()`, `capitalize()`, `startswith(prefix)`, `endswith(suffix)`, `slice(low?, high?)`
- Number (int/float): `format(decimals)`

Array/Dict method semantics:
- `array.count(value)` returns the number of elements equal to `value`.
- `array.pop()` removes and returns the last element (error on empty).
- `array.remove(value)` removes the first matching element and returns `true` (or `false` if not found).
- `array.count`/`array.remove` use `==` for comparisons; if `==` errors, the method errors.
- `dict.count()` returns the number of entries.
- `dict.get(key, default?)` returns the value if present; otherwise returns `default` or `nil`.
- `dict.pop(key, default?)` removes and returns the value if present; if missing returns `default` or errors.
- `dict.remove(key)` removes the entry and returns `nil` (error if missing).
- Calling dict-only methods on non-dicts raises `<method>() receiver must be DICT` (e.g., `get()`).

String method semantics:
- `strip()` removes leading/trailing Unicode whitespace (same definition as Go `strings.TrimSpace`).
- `uppercase()`/`lowercase()` use Unicode-aware case mapping.
- `capitalize()` returns empty string for empty input; otherwise uppercases the first Unicode code point and lowercases the rest (no trimming).
- `startswith(prefix)`/`endswith(suffix)` require string args; empty prefix/suffix returns true.
- `slice(low?, high?)` mirrors `s[low:high]` (Unicode code-point indices, negative indices from end, clamped bounds, empty if `low > high`).

Number formatting:
- `n.format(decimals:int) -> string`
  - `decimals` must be an integer >= 0.
  - Rounds half away from zero.
  - When `decimals > 0`, always prints exactly that many digits after `.` (padding with zeros).
  - When `decimals == 0`, no decimal point is printed.
  - Examples: `(1.234).format(2) == "1.23"`, `(1.2).format(4) == "1.2000"`, `(-1.235).format(2) == "-1.24"`.

Note:
- Welle does not implement Python-style f-strings or full format mini-language.
- Minimal formatting utilities are provided via builtins: `group_digits`, `format_float`, `format_percent`.

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
  - `fade_white(img, amount)`

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
- `-max-recursion` max function call depth (`0` = unlimited)
- `-max-steps` max VM instruction count (`0` = unlimited)
- `-max-mem` / `-max-memory` max allocation budget in bytes (`0` = unlimited)

Subcommands:
- `welle repl`
- `welle gfx [pathOrSpec]`
- `welle init [--name <name>] [--entry <file>] [--force]`
- `welle fmt [-w] [-i <indent>] [--ast] <path|dir> [more...]` (defaults to `.` if no path is provided)
- `welle lint <file|dir> [more...]`
- `welle test [path|dir]...`
- `welle tools install [--bin <dir>]`

`welle run`/`welle gfx` accept:
- a file path
- a directory within a project (searches up for `welle.toml`, uses its `entry`)
- a module spec (e.g., `std:math`)

### Tests (`welle test`)
Runs `.wll` tests in the provided files or directories.

Test discovery:
- Files ending in `.test.wll`
- Any `.wll` file under a `tests/` directory

Expectations via top-of-file directives:
- `// expect: ok`
- `// expect: error`
- `// expect: error contains "substring"`
- `// expect: stdout "exact string"`
- `// expect: stdout contains "substring"`
- `// expect: stdout file "relative/path/to.golden"`

Notes:
- Multiple `expect` lines are allowed (for example, combine `expect: error` with `expect: stdout ...`).
- Stdout comparisons normalize Windows newlines (`\r\n` becomes `\n`), but otherwise compare output exactly.
- `stdout file` paths are resolved relative to the test file's directory.

Example:
```welle
// expect: ok
// expect: stdout file "golden/output.txt"

print("hello")
print("world")
```

Defaults to interpreter mode; pass `--vm` to run tests on the bytecode VM.

### REPL
- Uses the VM compiler/runtime (same limitations as `-vm`).
- Multiline input continues while braces/parentheses are unbalanced or inside double-quoted strings.
- `exit` and `quit` leave the REPL.
- Prints the last non-`nil` expression result.

### Runtime limits
Limits are opt-in; defaults are unlimited unless configured.
- `max_recursion` / `-max-recursion` limits function call depth in both interpreter and VM.
- `max_steps` / `-max-steps` limits VM instruction count per run (including REPL inputs and module loads).
- `max_mem` / `-max-mem` / `-max-memory` limits the allocation budget (bytes) in both interpreter and VM.

Memory limit accounting (allocation budget, monotonic; no GC):
- Strings: `24 + len(utf8 bytes)` bytes.
- Arrays: `24 + 8*len(elements)` bytes (shallow; elements counted when created).
- Tuples: `24 + 8*len(elements)` bytes (shallow).
- Dicts: `32 + 24*len(entries)` bytes (shallow; keys/values counted when created). New dict entries from assignment add one entry cost.
- Images: `24 + width*height*4` bytes (full RGBA buffer).
- Errors: `32` bytes.
- Functions: `64` bytes.
- Closures: `32 + 8*len(free)` bytes; cells: `16` bytes.

Limit violations raise catchable errors:
- `max recursion depth exceeded (<limit>)`
- `max instruction count exceeded (<limit>)`
- `max memory exceeded (<limit> bytes)` (error code `8001`)

### Formatter (`welle fmt`)
Token-based formatter (`internal/format`):
- Normalizes spacing around operators and punctuation.
- Indents lines based on `{`/`}` nesting.
- Converts `;` into line breaks.
- Collapses multiple blank lines to at most one.
- Ensures a trailing newline.

AST-aware formatter (`internal/format/astfmt`, experimental, enable with `--ast`):
- Parses the program and prints from the AST with deterministic output.
- Preserves comments and blank lines in relative position when possible.
- Keeps string and numeric literal textual forms (raw/backticks/triple quotes, underscores, bases).
- Known limitations: inline comments inside nested expressions are still associated at the statement level.

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
- Document formatting
- Code actions for `WL0001`/`WL0002`/`WL0003` (prefix `_` or remove line)
- Completion (locals/params, top-levels, imports, builtins, stdlib modules, module members)
- Hover (kind + signature; builtin docs; module members when available)
- Rename (workspace-wide for module exports/imports and `alias.member` references; locals/params stay file-scoped)
- Find references (workspace-wide for module exports/imports and `alias.member` references; locals/params stay file-scoped)
- Signature help (user-defined functions, builtins, stdlib module functions)

Limitations:
- Workspace-wide rename/references only scan `.wll` files under the workspace root (stdlib folder is excluded).
- Rename/references are conservative when an import cannot be resolved to a concrete module path.
- Hover docs for stdlib functions are minimal (signatures only if parsed from std modules).
- Import-string completions only suggest stdlib module names.

## 7) Not implemented yet

* [x] **JS-style single-line `if` without braces** (`if (isAdmin) return true;`).
* [x] **Ternary operator** (`cond ? a : b`) — now supported.
* [x] **`null` accepted as alias of `nil`** — `null` is an alias; canonical form remains `nil`.
* [x] **Bitwise operators** (`|`, `&`, `^`, `~`, `<<`, `>>`).
* [x] **More stdlib/builtins like `abs`, `reverse`, `sum`**
  * `abs`, `reverse`, `sum` are builtins now.
* [x] **Python-like list comprehensions** (`[expr for i in seq]`, conditional forms).
* [x] **Python slicing with step** (`[::-1]`).
* [x] **Nullish coalescing `??`** — defaults only when value is `nil`.
* [x] **String repetition** (`"a" * 10`) — now supported for string/int.
* [x] **Star-unpacking in destructuring** (`a, *_, b = (...)`).
* [x] **`any()` / `all()`** — now listed as builtins.
* [x] **`...` or `pass`** no-op statement — `pass` is supported.
* [x] **Collection APIs + small utilities** (`count`, `get`, `pop`, `remove`, `map`, `mean`) — now documented and implemented as methods/builtins.
* [x] **Updating dicts with `|=`** — now supported.
* [x] **String casing helpers** (`strip`, `capitalize`, `uppercase`, `lowercase`, `startswith`, `endswith`).
* [x] **String slicing method** (`slice(low?, high?)`).
* [x] **Number formatting method** (`n.format(decimals)`).
* [x] **`max`** — now present as a builtin.
* [x] **Tagged Template Literals** — `tag t"...${...}..."` is implemented.
* [x] **“membership” operator `in` like JavaScript** (`item in list`) — now supported as an expression.
* [x] **Generic `sqrt` builtin**
  * `sqrt` is now a builtin alias of `math_sqrt`.
* [x] **Object property shorthand** (`{ name, age }`) — dict literals support identifier shorthand via `#{ name, age }`.
* [x] **Walrus operator `:=`** — implemented as define-in-current-scope assignment expression.
* [x] **String digit-group formatting** (`'14_310_023' -> '14,310,023'`) — via `group_digits(...)`.
* [x] **Float/percent formatting utilities** — via `format_float(...)` and `format_percent(...)` (not full f-strings).
* [] **Float format specifiers / f-strings** (`f"{pct:.1%}"`) — still not present.
* [x] **Identity operator `is` + deterministic semantics** — implemented without runtime caching quirks.
* [] **Additional utilities** (`add`, `acc`, etc.) — still not specified.
* [x] **`getpass()` / `input()`** — now present.

## 8) Appendix: Complete keyword/operator/token list

### Keywords
`func`, `return`, `break`, `continue`, `pass`, `if`, `else`, `while`, `for`, `in`, `true`, `false`, `nil`, `null`, `and`, `or`, `not`, `is`, `import`, `from`, `as`, `try`, `catch`, `finally`, `throw`, `defer`, `export`, `switch`, `match`, `case`, `default`

### Operators
`=`, `:=`, `+=`, `-=`, `*=`, `/=`, `%=`, `|=`, `+`, `-`, `*`, `/`, `%`, `|`, `&`, `^`, `~`, `<<`, `>>`, `==`, `!=`, `is`, `<`, `<=`, `>`, `>=`, `in`, `and`, `or`, `not`, `!`, `?`, `??`, `.`

### Delimiters and separators
Separators: `NEWLINE`, `;`  
Delimiters: `#`, `,`, `:`, `(`, `)`, `[`, `]`, `{`, `}`

## Verification
- Commands run:
  - `gofmt -w` on touched Go files
  - `go test ./...` (pass)
  - `go test ./internal/spec -run TestSpec` (pass)
  - `go run ./cmd/welle test ./tests` (pass: `passed 7, failed 0`)
  - `go run ./cmd/welle run /tmp/re26_demo.wll` (pass)
  - `go run ./cmd/welle -vm run /tmp/re26_demo.wll` (pass)
- Validated: lexer/token rules, parser precedence and forms, evaluator/VM semantics (including errors/throw/try/defer), builtins, stdlib exports, CLI flags/subcommands, formatter behavior, linter/LSP capabilities.
- Date: 2026-02-05
