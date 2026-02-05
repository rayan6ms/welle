package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"welle/internal/diag"
	"welle/internal/lexer"
	"welle/internal/lint"
	"welle/internal/lsp"
	"welle/internal/parser"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const (
	lsName  = "welle-lsp"
	version = "0.1"
)

var store = lsp.NewStore()
var handler protocol.Handler
var ws *lsp.Workspace

func main() {
	handler = protocol.Handler{
		Initialize:                     initialize,
		Initialized:                    initialized,
		TextDocumentDidOpen:            textDocumentDidOpen,
		TextDocumentDidChange:          textDocumentDidChange,
		TextDocumentDidSave:            textDocumentDidSave,
		TextDocumentDidClose:           textDocumentDidClose,
		TextDocumentCodeAction:         textDocumentCodeAction,
		TextDocumentFormatting:         textDocumentFormatting,
		TextDocumentSemanticTokensFull: textDocumentSemanticTokensFull,
		TextDocumentDefinition:         textDocumentDefinition,
		TextDocumentDocumentSymbol:     textDocumentDocumentSymbol,
		TextDocumentCompletion:         textDocumentCompletion,
		TextDocumentHover:              textDocumentHover,
		TextDocumentRename:             textDocumentRename,
		TextDocumentReferences:         textDocumentReferences,
		TextDocumentSignatureHelp:      textDocumentSignatureHelp,
	}

	server := server.NewServer(&handler, lsName, false)
	server.RunStdio()
}

func initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	root := "."
	if params.RootURI != nil {
		root = lsp.UriToPath(*params.RootURI)
	} else if params.RootPath != nil {
		root = *params.RootPath
	}
	if root == "" {
		root = "."
	}
	ws = lsp.NewWorkspace(root)

	full := protocol.TextDocumentSyncKindFull
	legend := protocol.SemanticTokensLegend{
		TokenTypes: []string{
			string(protocol.SemanticTokenTypeKeyword),
			string(protocol.SemanticTokenTypeString),
			string(protocol.SemanticTokenTypeNumber),
			string(protocol.SemanticTokenTypeOperator),
			string(protocol.SemanticTokenTypeFunction),
			string(protocol.SemanticTokenTypeVariable),
			string(protocol.SemanticTokenTypeParameter),
			string(protocol.SemanticTokenTypeNamespace),
			string(protocol.SemanticTokenTypeType),
			string(protocol.SemanticTokenTypeComment),
		},
		TokenModifiers: []string{
			string(protocol.SemanticTokenModifierDeclaration),
			string(protocol.SemanticTokenModifierReadonly),
		},
	}
	caps := protocol.ServerCapabilities{
		TextDocumentSync: &protocol.TextDocumentSyncOptions{
			OpenClose: &protocol.True,
			Change:    &full,
			Save:      protocol.SaveOptions{IncludeText: &protocol.False},
		},
		CodeActionProvider: protocol.CodeActionOptions{
			CodeActionKinds: []protocol.CodeActionKind{protocol.CodeActionKindQuickFix},
		},
		SemanticTokensProvider: &protocol.SemanticTokensOptions{
			Legend: legend,
			Full:   true,
			Range:  false,
		},
		DocumentFormattingProvider: true,
		DefinitionProvider:         true,
		DocumentSymbolProvider:     true,
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{".", "\""},
		},
		HoverProvider:      true,
		RenameProvider:     true,
		ReferencesProvider: true,
		SignatureHelpProvider: &protocol.SignatureHelpOptions{
			TriggerCharacters:   []string{"(", ","},
			RetriggerCharacters: []string{")"},
		},
	}

	b, _ := json.Marshal(caps)
	fmt.Fprintln(os.Stderr, "INIT CAPS:", string(b))

	return protocol.InitializeResult{
		Capabilities: caps,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: ptrString(version),
		},
	}, nil
}

func initialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func textDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	store.Set(uri, params.TextDocument.Text)
	updateIndex(uri, params.TextDocument.Text)
	return publishDiagnostics(ctx, uri, params.TextDocument.Text)
}

func textDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	if len(params.ContentChanges) == 0 {
		return nil
	}

	text, ok := extractFullText(params.ContentChanges[len(params.ContentChanges)-1])
	if !ok {
		return nil
	}

	store.Set(uri, text)
	updateIndex(uri, text)
	return publishDiagnostics(ctx, uri, text)
}

func textDocumentDidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	if text, ok := store.Get(uri); ok {
		return publishDiagnostics(ctx, uri, text)
	}
	return nil
}

func textDocumentDidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	store.Delete(uri)
	if ws != nil {
		ws.DropURI(uri)
	}
	return publishDiagnostics(ctx, uri, "")
}

func textDocumentCodeAction(ctx *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}

	actions := make([]protocol.CodeAction, 0)
	for _, d := range params.Context.Diagnostics {
		code := diagnosticCode(d)
		switch code {
		case "WL0003":
			if action, ok := lsp.MakeRemoveLineAction(uri, text, d.Range, "Remove unreachable code"); ok {
				actions = append(actions, action)
			}
		case "WL0001":
			if action, ok := lsp.MakePrefixUnderscoreAction(uri, text, d.Range); ok {
				actions = append(actions, action)
			}
			if action, ok := lsp.MakeRemoveLineAction(uri, text, d.Range, "Remove unused assignment"); ok {
				actions = append(actions, action)
			}
		case "WL0002":
			if action, ok := lsp.MakePrefixUnderscoreAction(uri, text, d.Range); ok {
				actions = append(actions, action)
			}
		}
	}

	if len(actions) == 0 {
		return nil, nil
	}
	return actions, nil
}

func diagnosticCode(d protocol.Diagnostic) string {
	if d.Code == nil {
		return ""
	}
	switch v := d.Code.Value.(type) {
	case string:
		return v
	case protocol.Integer:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

func textDocumentSemanticTokensFull(ctx *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}

	sem := lsp.SemanticTokensForText(text)
	data := lsp.EncodeSemanticTokens(sem)
	return &protocol.SemanticTokens{Data: data}, nil
}

func textDocumentDefinition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}

	ref := lsp.MemberAt(text, params.Position)
	if !ref.Ok {
		return nil, nil
	}

	if ws == nil {
		return nil, nil
	}

	ix := ws.GetIndexForURI(uri)
	if ix == nil {
		return nil, nil
	}

	if ref.Alias != "" {
		spec, ok := ix.Imports[ref.Alias]
		if !ok {
			return nil, nil
		}

		fromPath := lsp.UriToPath(uri)
		if fromPath == "" {
			return nil, nil
		}
		resolvedPath, err := ws.ResolveImport(fromPath, spec)
		if err != nil {
			return nil, nil
		}

		modIx, err := ws.IndexPath(resolvedPath)
		if err != nil {
			return nil, nil
		}

		if loc, ok := modIx.Exports[ref.Member]; ok {
			return []protocol.Location{loc}, nil
		}
		if loc, ok := modIx.Defs[ref.Member]; ok {
			return []protocol.Location{loc}, nil
		}
		return nil, nil
	}

	if loc, ok := ix.Defs[ref.Member]; ok {
		return []protocol.Location{loc}, nil
	}

	return nil, nil
}

func textDocumentDocumentSymbol(ctx *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	uri := string(params.TextDocument.URI)
	if ws == nil {
		return []protocol.DocumentSymbol{}, nil
	}
	ix := ws.GetIndexForURI(uri)
	if ix == nil {
		return []protocol.DocumentSymbol{}, nil
	}
	return ix.Symbols, nil
}

func textDocumentCompletion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}
	items := lsp.CompletionItems(ws, uri, text, params.Position)
	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

func textDocumentHover(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}
	return lsp.HoverAt(ws, uri, text, params.Position)
}

func textDocumentRename(ctx *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}
	return lsp.RenameAt(ws, uri, text, params.Position, params.NewName)
}

func textDocumentReferences(ctx *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}
	return lsp.ReferencesAt(ws, uri, text, params.Position, params.Context.IncludeDeclaration)
}

func textDocumentSignatureHelp(ctx *glsp.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	uri := string(params.TextDocument.URI)
	text, ok := store.Get(uri)
	if !ok {
		return nil, nil
	}
	return lsp.SignatureHelpAt(ws, uri, text, params.Position)
}

func updateIndex(uri string, text string) {
	if !strings.HasSuffix(strings.ToLower(uri), ".wll") {
		if ws != nil {
			ws.DropURI(uri)
		}
		return
	}
	if ws != nil {
		_, _ = ws.UpdateOpenDoc(uri, text)
	}
}

func publishDiagnostics(ctx *glsp.Context, uri string, text string) error {
	if !strings.HasSuffix(strings.ToLower(uri), ".wll") {
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
			URI:         protocol.DocumentUri(uri),
			Diagnostics: []protocol.Diagnostic{},
		})
		return nil
	}

	lx := lexer.New(text)
	p := parser.New(lx)
	prog := p.ParseProgram()

	diags := append([]diag.Diagnostic{}, p.Diagnostics()...)
	if prog != nil {
		diags = append(diags, lint.Run(prog)...)
	}
	lspDiags := lsp.ToLspDiagnostics(diags)

	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentUri(uri),
		Diagnostics: lspDiags,
	})
	return nil
}

func extractFullText(change any) (string, bool) {
	switch typed := change.(type) {
	case protocol.TextDocumentContentChangeEventWhole:
		return typed.Text, true
	case protocol.TextDocumentContentChangeEvent:
		return typed.Text, true
	default:
		return "", false
	}
}

type posKey struct {
	Line int
	Col  int
}

func ptrString(s string) *string { return &s }
