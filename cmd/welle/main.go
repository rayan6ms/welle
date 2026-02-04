package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"welle/internal/compiler"
	"welle/internal/config"
	"welle/internal/diag"
	"welle/internal/evaluator"
	"welle/internal/format"
	"welle/internal/gfx"
	"welle/internal/lexer"
	"welle/internal/lint"
	"welle/internal/module"
	"welle/internal/object"
	"welle/internal/parser"
	"welle/internal/repl"
	"welle/internal/token"
	"welle/internal/tools"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "fmt" {
		runFmt(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "lint" {
		runLint(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "tools" {
		runTools(os.Args[2:])
		return
	}

	tokensMode := flag.Bool("tokens", false, "print tokens instead of running")
	astMode := flag.Bool("ast", false, "print AST instead of running")
	vmMode := flag.Bool("vm", false, "run using bytecode VM")
	disMode := flag.Bool("dis", false, "dump bytecode instructions and constants")
	optMode := flag.Bool("O", false, "enable bytecode optimizer")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	stdRoot := filepath.Join(cwd, "std")
	if abs, err := filepath.Abs(stdRoot); err == nil {
		stdRoot = abs
	}

	args := flag.Args()
	if len(args) == 0 {
		repl.Start(os.Stdin, os.Stdout, stdRoot)
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]
	if cmd != "run" && cmd != "repl" && cmd != "gfx" {
		cmd = "run"
		cmdArgs = args
	}

	var entrySpec string
	var projectRoot string
	switch cmd {
	case "repl":
		if *tokensMode || *astMode || *disMode {
			fmt.Println("repl does not support -tokens, -ast, or -dis")
			os.Exit(1)
		}
		if len(cmdArgs) != 0 {
			fmt.Println("usage: welle repl")
			os.Exit(1)
		}
		repl.Start(os.Stdin, os.Stdout, stdRoot)
		return
	case "run":
		if len(cmdArgs) > 1 {
			fmt.Println("usage: welle run [pathOrSpec]")
			os.Exit(1)
		}
		target := "."
		if len(cmdArgs) == 1 {
			target = cmdArgs[0]
		}
		var err error
		entrySpec, projectRoot, err = resolveRunTarget(target)
		if err != nil {
			fmt.Println("run error:", err)
			os.Exit(1)
		}
	case "gfx":
		if *tokensMode || *astMode || *disMode || *vmMode {
			fmt.Println("gfx does not support -tokens, -ast, -dis, or -vm")
			os.Exit(1)
		}
		if len(cmdArgs) > 1 {
			fmt.Println("usage: welle gfx [pathOrSpec]")
			os.Exit(1)
		}
		target := "."
		if len(cmdArgs) == 1 {
			target = cmdArgs[0]
		}
		var err error
		entrySpec, projectRoot, err = resolveRunTarget(target)
		if err != nil {
			fmt.Println("gfx error:", err)
			os.Exit(1)
		}
	default:
		fmt.Println("unknown command:", cmd)
		os.Exit(1)
	}

	extraPaths := []string{cwd}
	if projectRoot != "" && projectRoot != cwd {
		extraPaths = append(extraPaths, projectRoot)
	}
	resolver := module.NewResolver(stdRoot, extraPaths)
	loader := module.NewLoader(resolver)

	entryFrom := filepath.Join(cwd, "__entry.wll")

	if *tokensMode || *astMode {
		entryPath, err := resolver.Resolve(entryFrom, entrySpec)
		if err != nil {
			fmt.Println("resolve error:", err)
			os.Exit(1)
		}
		b, err := os.ReadFile(entryPath)
		if err != nil {
			fmt.Println("read error:", err)
			os.Exit(1)
		}
		src := string(b)

		if *tokensMode {
			l := lexer.New(src)
			for {
				tok := l.NextToken()
				fmt.Printf("%4d:%-3d  %-10s  %q\n", tok.Line, tok.Col, tok.Type, tok.Literal)
				if tok.Type == token.EOF {
					break
				}
			}
			return
		}

		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) > 0 {
			for _, e := range p.Errors() {
				fmt.Println("parse error:", e)
			}
			os.Exit(1)
		}

		fmt.Println(program.String())
		return
	}

	if *disMode {
		*vmMode = true
	}

	if *vmMode {
		bc, entryPath, err := loader.LoadBytecode(entryFrom, entrySpec, *optMode)
		if err != nil {
			fmt.Println("load error:", err)
			os.Exit(1)
		}
		if *disMode {
			fmt.Print(compiler.FormatConstants(bc.Constants))
			fmt.Println()
			fmt.Print("== instructions ==\n")
			fmt.Print(bc.Instructions.String())
			fmt.Println()
		}
		m := loader.NewVM(bc, entryPath)
		if err := m.Run(); err != nil {
			fmt.Println("vm error:", err)
			os.Exit(1)
		}
		return
	}

	entryPath, err := resolver.Resolve(entryFrom, entrySpec)
	if err != nil {
		fmt.Println("resolve error:", err)
		os.Exit(1)
	}

	if cmd == "gfx" {
		runner := evaluator.NewRunner()
		runner.SetResolver(resolver)
		runner.EnableImports()
		var env *object.Environment
		var setupFn object.Object
		var updateFn object.Object
		var drawFn object.Object
		getFn := func(name string) object.Object {
			if env == nil {
				return nil
			}
			if v, ok := env.Get(name); ok {
				return v
			}
			return nil
		}

		callFn := func(fn object.Object, args ...object.Object) error {
			if fn == nil {
				return nil
			}
			res := runner.Call(fn, args...)
			if res != nil && res.Type() == object.ERROR_OBJ {
				return errors.New(res.Inspect())
			}
			return nil
		}

		err := gfx.Run(gfx.LoopFuncs{
			Setup: func() error {
				// Evaluate after gfx backend is active so top-level gfx calls work.
				var res object.Object
				env, res = runner.RunFileEnv(entryPath)
				if res != nil && res.Type() == object.ERROR_OBJ {
					return errors.New(res.Inspect())
				}
				setupFn = getFn("setup")
				updateFn = getFn("update")
				drawFn = getFn("draw")
				return callFn(setupFn)
			},
			Update: func(dt float64) error {
				return callFn(updateFn, &object.Float{Value: dt})
			},
			Draw: func() error {
				return callFn(drawFn)
			},
		})
		if err != nil {
			fmt.Println("gfx error:", err)
			os.Exit(1)
		}
		return
	}

	runner := evaluator.NewRunner()
	runner.SetResolver(resolver)
	runner.EnableImports()
	res := runner.RunFile(entryPath)
	if res != nil && res.Type() == object.ERROR_OBJ {
		fmt.Println(res.Inspect())
		os.Exit(1)
	}
}

func isPathSpec(spec string) bool {
	if strings.HasPrefix(spec, "std:") {
		return false
	}
	if spec == "." || spec == ".." {
		return true
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || filepath.IsAbs(spec) {
		return true
	}
	return strings.Contains(spec, string(os.PathSeparator))
}

func resolveRunTarget(target string) (string, string, error) {
	if !isPathSpec(target) {
		return target, "", nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("path not found: %s", target)
		}
		return "", "", err
	}
	if !info.IsDir() {
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return "", "", err
		}
		return absTarget, filepath.Dir(absTarget), nil
	}
	projectRoot, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	manifestPath := filepath.Join(projectRoot, "welle.toml")
	man, err := config.LoadManifest(manifestPath)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(man.Entry) == "" {
		return "", "", fmt.Errorf("%s: missing entry", manifestPath)
	}
	entryPath := filepath.Join(projectRoot, man.Entry)
	entryPath, err = filepath.Abs(entryPath)
	if err != nil {
		return "", "", err
	}
	return entryPath, projectRoot, nil
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "project name")
	entry := fs.String("entry", "main.wll", "entry file")
	force := fs.Bool("force", false, "overwrite existing files")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Println("usage: welle init [--name <name>] [--entry <file>] [--force]")
		os.Exit(1)
	}
	if strings.TrimSpace(*entry) == "" {
		fmt.Println("init error: entry cannot be empty")
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	manifestPath := filepath.Join(cwd, "welle.toml")
	manifestExists, err := pathExists(manifestPath)
	if err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}
	if manifestExists && !*force {
		fmt.Println("init error: welle.toml already exists (use --force to overwrite)")
		os.Exit(1)
	}

	manifest := buildManifest(*name, *entry)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}

	entryPath := filepath.Join(cwd, *entry)
	if err := ensureDir(entryPath); err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}

	entryExists, err := pathExists(entryPath)
	if err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}
	if !entryExists || *force {
		if err := os.WriteFile(entryPath, []byte(starterProgram()), 0o644); err != nil {
			fmt.Println("init error:", err)
			os.Exit(1)
		}
	}
}

func runFmt(args []string) {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	writeBack := fs.Bool("w", false, "write result to (source) file")
	indent := fs.String("i", "  ", "indent string")
	if err := fs.Parse(args); err != nil {
		fmt.Println("usage: welle fmt [-w] [-i <indent>] <path>")
		os.Exit(1)
	}

	targets := fs.Args()
	if len(targets) == 0 {
		targets = []string{"."}
	}

	files, err := collectWelleFiles(targets)
	if err != nil {
		fmt.Println("fmt error:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		return
	}
	sort.Strings(files)

	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("fmt error:", err)
			os.Exit(1)
		}
		formatted, err := format.Format(string(b), format.Options{Indent: *indent})
		if err != nil {
			fmt.Println("fmt error:", err)
			os.Exit(1)
		}

		if *writeBack && string(b) != formatted {
			if err := writeFileAtomic(path, []byte(formatted)); err != nil {
				fmt.Println("fmt error:", err)
				os.Exit(1)
			}
		}
		fmt.Printf("formatted %s\n", path)
	}
}

func runLint(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: welle lint <file|dir> [more...]")
		os.Exit(2)
	}

	files, err := collectWelleFiles(args)
	if err != nil {
		fmt.Println("lint error:", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		return
	}
	sort.Strings(files)

	hadErrors := false
	for _, path := range files {
		diags, err := lintFile(path)
		if err != nil {
			fmt.Println("lint error:", err)
			hadErrors = true
			continue
		}
		for _, d := range diags {
			fmt.Println(d.Format(path))
			if d.Severity == diag.SeverityError {
				hadErrors = true
			}
		}
	}

	if hadErrors {
		os.Exit(1)
	}
}

func runTools(args []string) {
	if len(args) == 0 || args[0] != "install" {
		fmt.Println("usage: welle tools install [--bin <dir>]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("tools install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	binDir := fs.String("bin", "bin", "output directory for tools")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		fmt.Println("usage: welle tools install [--bin <dir>]")
		os.Exit(2)
	}

	if err := tools.Install(tools.InstallOptions{BinDir: *binDir}); err != nil {
		fmt.Println("install error:", err)
		os.Exit(1)
	}
	fmt.Printf("installed: %s, %s\n", filepath.Join(*binDir, "welle"), filepath.Join(*binDir, "welle-lsp"))
}

func lintFile(path string) ([]diag.Diagnostic, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	l := lexer.New(string(b))
	p := parser.New(l)
	prog := p.ParseProgram()
	diags := append([]diag.Diagnostic{}, p.Diagnostics()...)
	if prog != nil {
		diags = append(diags, lint.Run(prog)...)
	}
	return diags, nil
}

func collectWelleFiles(targets []string) ([]string, error) {
	var files []string
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if strings.HasSuffix(target, ".wll") {
				files = append(files, target)
			}
			continue
		}

		err = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".wll") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func writeFileAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wellefmt-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

func buildManifest(name, entry string) string {
	var b strings.Builder
	if strings.TrimSpace(name) != "" {
		fmt.Fprintf(&b, "name = %q\n", name)
	}
	fmt.Fprintf(&b, "entry = %q\n", entry)
	return b.String()
}

func starterProgram() string {
	return "import \"std:math\" as math\n\nprint(math.add(2, 3))\n"
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
