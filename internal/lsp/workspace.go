package lsp

import (
	"os"
	"path/filepath"
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
}

func NewWorkspace(rootPath string) *Workspace {
	rootAbs, _ := filepath.Abs(rootPath)
	stdRoot := filepath.Join(rootAbs, "std")

	return &Workspace{
		rootPath: rootAbs,
		stdRoot:  stdRoot,
		resolver: module.NewResolver(stdRoot, []string{rootAbs}),
		byPath:   map[string]*DocIndex{},
		byURI:    map[string]*DocIndex{},
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

	if pth := UriToPath(uri); pth != "" {
		pthAbs, _ := filepath.Abs(pth)
		w.byPath[pthAbs] = ix
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
	if pth := UriToPath(uri); pth != "" {
		pthAbs, _ := filepath.Abs(pth)
		delete(w.byPath, pthAbs)
	}
}
