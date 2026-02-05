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
	"welle/internal/format/astfmt"
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
	if len(os.Args) > 1 && os.Args[1] == "test" {
		runTest(os.Args[2:])
		return
	}

	tokensMode := flag.Bool("tokens", false, "print tokens instead of running")
	astMode := flag.Bool("ast", false, "print AST instead of running")
	vmMode := flag.Bool("vm", false, "run using bytecode VM")
	disMode := flag.Bool("dis", false, "dump bytecode instructions and constants")
	optMode := flag.Bool("O", false, "enable bytecode optimizer")
	maxRecursion := flag.Int("max-recursion", -1, "max recursion depth (0 = unlimited)")
	maxSteps := flag.Int64("max-steps", -1, "max VM instruction count (0 = unlimited)")
	maxMem := flag.Int64("max-mem", -1, "max memory allocation in bytes (0 = unlimited)")
	maxMemory := flag.Int64("max-memory", -1, "max memory allocation in bytes (0 = unlimited)")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	defaultStdRoot := filepath.Join(cwd, "std")
	if abs, err := filepath.Abs(defaultStdRoot); err == nil {
		defaultStdRoot = abs
	}

	args := flag.Args()
	if len(args) == 0 {
		recLimit, stepLimit, memLimit, err := resolveLimits(*maxRecursion, *maxSteps, *maxMem, *maxMemory, nil)
		if err != nil {
			fmt.Println("repl error:", err)
			os.Exit(1)
		}
		repl.Start(os.Stdin, os.Stdout, defaultStdRoot, repl.Limits{
			MaxRecursion: recLimit,
			MaxSteps:     stepLimit,
			MaxMemory:    memLimit,
		})
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
	var manifest *config.Manifest
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
		recLimit, stepLimit, memLimit, err := resolveLimits(*maxRecursion, *maxSteps, *maxMem, *maxMemory, nil)
		if err != nil {
			fmt.Println("repl error:", err)
			os.Exit(1)
		}
		repl.Start(os.Stdin, os.Stdout, defaultStdRoot, repl.Limits{
			MaxRecursion: recLimit,
			MaxSteps:     stepLimit,
			MaxMemory:    memLimit,
		})
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
		entrySpec, projectRoot, manifest, err = resolveRunTarget(target)
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
		entrySpec, projectRoot, manifest, err = resolveRunTarget(target)
		if err != nil {
			fmt.Println("gfx error:", err)
			os.Exit(1)
		}
	default:
		fmt.Println("unknown command:", cmd)
		os.Exit(1)
	}

	resolver, err := buildResolver(cwd, projectRoot, manifest)
	if err != nil {
		fmt.Println("resolver error:", err)
		os.Exit(1)
	}
	loader := module.NewLoader(resolver)
	recLimit, stepLimit, memLimit, err := resolveLimits(*maxRecursion, *maxSteps, *maxMem, *maxMemory, manifest)
	if err != nil {
		fmt.Println("run error:", err)
		os.Exit(1)
	}

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
		m.SetMaxRecursion(recLimit)
		m.SetMaxSteps(stepLimit)
		m.SetMaxMemory(memLimit)
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
		runner.SetMaxRecursion(recLimit)
		runner.SetMaxMemory(memLimit)
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
	runner.SetMaxRecursion(recLimit)
	runner.SetMaxMemory(memLimit)
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

func resolveRunTarget(target string) (string, string, *config.Manifest, error) {
	if !isPathSpec(target) {
		projectRoot, man, err := findManifest(".")
		if err != nil {
			return "", "", nil, err
		}
		return target, projectRoot, man, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil, fmt.Errorf("path not found: %s", target)
		}
		return "", "", nil, err
	}
	if !info.IsDir() {
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return "", "", nil, err
		}
		projectRoot, man, err := findManifest(filepath.Dir(absTarget))
		if err != nil {
			return "", "", nil, err
		}
		return absTarget, projectRoot, man, nil
	}
	startDir, err := filepath.Abs(target)
	if err != nil {
		return "", "", nil, err
	}
	projectRoot, man, err := findManifest(startDir)
	if err != nil {
		return "", "", nil, err
	}
	if man == nil {
		return "", "", nil, fmt.Errorf("welle.toml not found in or above %s", startDir)
	}
	if strings.TrimSpace(man.Entry) == "" {
		return "", "", nil, fmt.Errorf("%s: missing entry", filepath.Join(projectRoot, "welle.toml"))
	}
	entryPath := filepath.Join(projectRoot, man.Entry)
	entryPath, err = filepath.Abs(entryPath)
	if err != nil {
		return "", "", nil, err
	}
	return entryPath, projectRoot, man, nil
}

func findManifest(start string) (string, *config.Manifest, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", nil, err
	}
	for {
		manifestPath := filepath.Join(dir, "welle.toml")
		info, err := os.Stat(manifestPath)
		if err == nil && !info.IsDir() {
			man, err := config.LoadManifest(manifestPath)
			if err != nil {
				return "", nil, err
			}
			return dir, man, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil, nil
}

func buildResolver(cwd, projectRoot string, man *config.Manifest) (*module.Resolver, error) {
	baseRoot := cwd
	if projectRoot != "" {
		baseRoot = projectRoot
	}
	defaultStdRoot := filepath.Join(baseRoot, "std")
	if abs, err := filepath.Abs(defaultStdRoot); err == nil {
		defaultStdRoot = abs
	}

	stdRoot := defaultStdRoot
	modulePaths := []string{}
	if man != nil && projectRoot != "" {
		var err error
		stdRoot, modulePaths, err = man.ResolvePaths(projectRoot, defaultStdRoot)
		if err != nil {
			return nil, err
		}
	}

	extraPaths := append([]string{}, modulePaths...)
	extraPaths = append(extraPaths, cwd)
	if projectRoot != "" && projectRoot != cwd {
		extraPaths = append(extraPaths, projectRoot)
	}

	return module.NewResolver(stdRoot, extraPaths), nil
}

func resolveLimits(cliRec int, cliSteps int64, cliMem int64, cliMemAlt int64, man *config.Manifest) (int, int64, int64, error) {
	if cliRec < -1 {
		return 0, 0, 0, fmt.Errorf("max-recursion must be >= 0")
	}
	if cliSteps < -1 {
		return 0, 0, 0, fmt.Errorf("max-steps must be >= 0")
	}
	if cliMem < -1 || cliMemAlt < -1 {
		return 0, 0, 0, fmt.Errorf("max-mem must be >= 0")
	}
	if cliMem >= 0 && cliMemAlt >= 0 && cliMem != cliMemAlt {
		return 0, 0, 0, fmt.Errorf("max-mem and max-memory differ; use only one")
	}

	rec := 0
	if cliRec >= 0 {
		rec = cliRec
	} else if man != nil && man.MaxRecursion > 0 {
		rec = man.MaxRecursion
	}

	steps := int64(0)
	if cliSteps >= 0 {
		steps = cliSteps
	} else if man != nil && man.MaxSteps > 0 {
		steps = man.MaxSteps
	}

	mem := int64(0)
	cliVal := cliMem
	if cliVal < 0 {
		cliVal = cliMemAlt
	}
	if cliVal >= 0 {
		mem = cliVal
	} else if man != nil && man.MaxMem > 0 {
		mem = man.MaxMem
	}

	return rec, steps, mem, nil
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
	useAST := fs.Bool("ast", false, "use AST-aware formatter (experimental)")
	if err := fs.Parse(args); err != nil {
		fmt.Println("usage: welle fmt [-w] [-i <indent>] [--ast] <path>")
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
		formatted, err := formatWithMode(b, *indent, *useAST)
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

func formatWithMode(src []byte, indent string, useAST bool) (string, error) {
	if useAST {
		out, err := astfmt.FormatASTWithIndent(src, indent)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	return format.Format(string(src), format.Options{Indent: indent})
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
