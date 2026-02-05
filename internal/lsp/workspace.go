package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"welle/internal/lexer"
	"welle/internal/module"
	"welle/internal/parser"
)

type Workspace struct {
	mu sync.RWMutex

	rootPath string
	stdRoot  string

	resolver *module.Resolver

	byPath map[string]*DocIndex
	byURI  map[string]*DocIndex

	docTextByURI  map[string]string
	docTextByPath map[string]string
}

func NewWorkspace(rootPath string) *Workspace {
	rootAbs, _ := filepath.Abs(rootPath)
	stdRoot := filepath.Join(rootAbs, "std")

	return &Workspace{
		rootPath:      rootAbs,
		stdRoot:       stdRoot,
		resolver:      module.NewResolver(stdRoot, []string{rootAbs}),
		byPath:        map[string]*DocIndex{},
		byURI:         map[string]*DocIndex{},
		docTextByURI:  map[string]string{},
		docTextByPath: map[string]string{},
	}
}

func (w *Workspace) UpdateOpenDoc(uri string, text string) (*DocIndex, error) {
	lx := lexer.New(text)
	p := parser.New(lx)
	prog := p.ParseProgram()

	ix := BuildIndex(uri, prog)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.byURI[uri] = ix
	w.docTextByURI[uri] = text

	if pth := UriToPath(uri); pth != "" {
		pthAbs, _ := filepath.Abs(pth)
		w.byPath[pthAbs] = ix
		w.docTextByPath[pthAbs] = text
	}
	return ix, nil
}

func (w *Workspace) IndexPath(absPath string) (*DocIndex, error) {
	absPath, _ = filepath.Abs(absPath)

	w.mu.RLock()
	if ix, ok := w.byPath[absPath]; ok {
		w.mu.RUnlock()
		return ix, nil
	}
	if text, ok := w.docTextByPath[absPath]; ok {
		w.mu.RUnlock()
		uri := PathToURI(absPath)
		lx := lexer.New(text)
		p := parser.New(lx)
		prog := p.ParseProgram()
		ix := BuildIndex(uri, prog)
		w.mu.Lock()
		w.byPath[absPath] = ix
		w.mu.Unlock()
		return ix, nil
	}
	w.mu.RUnlock()

	b, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	uri := PathToURI(absPath)

	lx := lexer.New(string(b))
	p := parser.New(lx)
	prog := p.ParseProgram()
	ix := BuildIndex(uri, prog)

	w.mu.Lock()
	w.byPath[absPath] = ix
	w.mu.Unlock()
	return ix, nil
}

func (w *Workspace) ResolveImport(fromFilePath string, spec string) (string, error) {
	return w.resolver.Resolve(fromFilePath, spec)
}

func (w *Workspace) GetIndexForURI(uri string) *DocIndex {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.byURI[uri]
}

func (w *Workspace) DropURI(uri string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.byURI, uri)
	delete(w.docTextByURI, uri)
	if pth := UriToPath(uri); pth != "" {
		pthAbs, _ := filepath.Abs(pth)
		delete(w.byPath, pthAbs)
		delete(w.docTextByPath, pthAbs)
	}
}

func (w *Workspace) StdModules() []string {
	if w == nil {
		return nil
	}
	entries, err := os.ReadDir(w.stdRoot)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".wll" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (w *Workspace) TextForURI(uri string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	text, ok := w.docTextByURI[uri]
	return text, ok
}

func (w *Workspace) TextForPath(absPath string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	text, ok := w.docTextByPath[absPath]
	return text, ok
}

func (w *Workspace) OpenDocPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, 0, len(w.docTextByPath))
	for pth := range w.docTextByPath {
		out = append(out, pth)
	}
	return out
}

func (w *Workspace) WorkspaceFiles() ([]string, error) {
	if w == nil {
		return nil, nil
	}
	root := w.rootPath
	if root == "" {
		return nil, nil
	}
	stdRoot := w.stdRoot
	files := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" {
				return filepath.SkipDir
			}
			if stdRoot != "" {
				abs, _ := filepath.Abs(path)
				if abs == stdRoot || strings.HasPrefix(abs, stdRoot+string(os.PathSeparator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if filepath.Ext(d.Name()) == ".wll" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
