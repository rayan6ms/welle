package compiler_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"welle/internal/compiler"
	"welle/internal/lexer"
	"welle/internal/parser"
	"welle/internal/vm"
)

type vmResult struct {
	stdout string
	errMsg string
	last   string
}

func TestOptimizerCorrectnessPrograms(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "fold_literals",
			src: `print(0b1010_0011 + 0xF_F)
print(1_000 + 2_000)
print(1.5 + 2.25)`,
		},
		{
			name: "div_mod_zero",
			src: `try { x = 1 / 0 } catch (e) { print(e.message) }
try { y = 1 % 0 } catch (e) { print(e.message) }`,
		},
		{
			name: "short_circuit",
			src: `x = 0
func bump() { x = x + 1; return true }
false and bump()
true or bump()
true and bump()
print(x)`,
		},
		{
			name: "control_flow",
			src: `sum = 0
for (i = 0; i < 6; i = i + 1) {
  if (i == 2) { continue }
  if (i == 5) { break }
  sum = sum + i
}
print(sum)`,
		},
		{
			name: "assignment_expressions",
			src: `x = 1
print(x += 2)
a = 0
b = 0
a = b = 7
print(a)
print(b)`,
		},
		{
			name: "try_catch_finally_defer",
			src: `func test() {
  defer print("defer")
  try { throw "boom" } catch (e) { print("catch") } finally { print("finally") }
}
test()`,
		},
		{
			name: "tuple_destructuring",
			src: `t = (1, 2, 3)
(a, b, c) = t
print(a)
print(b)
print(c)`,
		},
		{
			name: "closures",
			src: `func make(x) { return func(y) { return x + y } }
f = make(2)
print(f(3))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runVMSource(t, tt.src, false)
			opt := runVMSource(t, tt.src, true)

			if got.stdout != opt.stdout {
				t.Fatalf("stdout mismatch\nplain: %q\nopt:   %q", got.stdout, opt.stdout)
			}
			if got.errMsg != opt.errMsg {
				t.Fatalf("error mismatch\nplain: %q\nopt:   %q", got.errMsg, opt.errMsg)
			}
			if got.errMsg == "" && got.last != opt.last {
				t.Fatalf("last value mismatch\nplain: %q\nopt:   %q", got.last, opt.last)
			}
		})
	}
}

func runVMSource(t *testing.T, src string, optimize bool) vmResult {
	t.Helper()

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	c := compiler.NewWithFile("test.wll")
	if err := c.Compile(program); err != nil {
		t.Fatalf("compile error: %v", err)
	}
	bc := c.Bytecode()

	if optimize {
		opt := &compiler.Optimizer{}
		var err error
		bc, err = opt.Optimize(bc)
		if err != nil {
			return vmResult{errMsg: err.Error()}
		}
	}

	m := vm.New(bc)
	stdout, err := captureStdout(t, func() error {
		return m.Run()
	})

	res := vmResult{stdout: stdout}
	if err != nil {
		res.errMsg = normalizeVMError(err.Error())
		return res
	}
	if last := m.LastPoppedStackElem(); last != nil {
		res.last = last.Inspect()
	}
	return res
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	err = fn()
	_ = w.Close()
	os.Stdout = oldStdout
	stdout := <-outCh
	_ = r.Close()

	return stdout, err
}

func normalizeVMError(msg string) string {
	if strings.HasPrefix(msg, "error: ") {
		line := strings.SplitN(msg, "\n", 2)[0]
		return strings.TrimPrefix(line, "error: ")
	}
	return msg
}
