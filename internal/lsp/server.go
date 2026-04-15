package lsp

import (
	"context"
	"io"
	"os"
	"sync"

	jrpc "github.com/sourcegraph/jsonrpc2"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/decade-eng/k8s-crd-lsp/internal/kubectl"
	"github.com/decade-eng/k8s-crd-lsp/internal/schema"
	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

type Server struct {
	kubectl  *kubectl.Executor
	registry *schema.Registry
	store    *myyaml.Store

	mu            sync.Mutex
	schemasReady  bool
	schemaLoadErr error
	notifyFn      func(method string, params any)

	handler *protocol.Handler
}

func NewServer(kubectlPath string) *Server {
	s := &Server{
		kubectl:  kubectl.New(kubectlPath),
		registry: schema.NewRegistry(),
		store:    myyaml.NewStore(),
	}
	s.handler = &protocol.Handler{
		Initialize:             s.initialize,
		Initialized:            func(_ *glsp.Context, _ *protocol.InitializedParams) error { return nil },
		Shutdown:               s.shutdown,
		TextDocumentDidOpen:    s.textDocumentDidOpen,
		TextDocumentDidChange:  s.textDocumentDidChange,
		TextDocumentDidClose:   s.textDocumentDidClose,
		TextDocumentCompletion: s.textDocumentCompletion,
	}
	return s
}

func (s *Server) Start() error {
	return s.startWithStream(stdrwc{})
}

func (s *Server) StartWithPipes(in io.Reader, out io.Writer) error {
	return s.startWithStream(&pipeRWC{r: in, w: out})
}

func (s *Server) startWithStream(stream io.ReadWriteCloser) error {
	h := jrpc.HandlerWithError(func(ctx context.Context, conn *jrpc.Conn, req *jrpc.Request) (any, error) {
		glspCtx := &glsp.Context{
			Method: req.Method,
			Notify: func(method string, params any) {
				conn.Notify(ctx, method, params) //nolint
			},
			Call: func(method string, params any, result any) {
				conn.Call(ctx, method, params, result) //nolint
			},
		}
		if req.Params != nil {
			glspCtx.Params = *req.Params
		}

		if req.Method == "exit" {
			s.handler.Handle(glspCtx)
			conn.Close()
			return nil, nil
		}

		r, validMethod, validParams, err := s.handler.Handle(glspCtx)
		if !validMethod {
			return nil, &jrpc.Error{Code: jrpc.CodeMethodNotFound, Message: "method not supported: " + req.Method}
		}
		if !validParams {
			if err != nil {
				return nil, &jrpc.Error{Code: jrpc.CodeInvalidParams, Message: err.Error()}
			}
			return nil, &jrpc.Error{Code: jrpc.CodeInvalidParams}
		}
		if err != nil {
			return nil, &jrpc.Error{Code: jrpc.CodeInvalidRequest, Message: err.Error()}
		}
		return r, nil
	})

	conn := jrpc.NewConn(
		context.Background(),
		jrpc.NewBufferedStream(stream, jrpc.VSCodeObjectCodec{}),
		h,
	)
	<-conn.DisconnectNotify()
	return nil
}

func (s *Server) initialize(ctx *glsp.Context, _ *protocol.InitializeParams) (any, error) {
	s.handler.SetInitialized(true)

	s.mu.Lock()
	s.notifyFn = ctx.Notify
	s.mu.Unlock()

	go s.loadSchemas()

	trueVal := true
	syncKind := protocol.TextDocumentSyncKindFull
	return protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: &trueVal,
				Change:    &syncKind,
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", " "},
			},
		},
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name: "k8s-crd-lsp",
		},
	}, nil
}

func (s *Server) shutdown(_ *glsp.Context) error {
	return nil
}

func (s *Server) textDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	s.store.Update(uri, params.TextDocument.Text)
	s.publishDiagnosticsForURI(ctx.Notify, uri)
	return nil
}

func (s *Server) textDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	for _, change := range params.ContentChanges {
		if whole, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			s.store.Update(uri, whole.Text)
		}
	}
	s.publishDiagnosticsForURI(ctx.Notify, uri)
	return nil
}

func (s *Server) textDocumentDidClose(_ *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	s.store.Remove(uri)
	return nil
}

func (s *Server) textDocumentCompletion(_ *glsp.Context, params *protocol.CompletionParams) (any, error) {
	uri := string(params.TextDocument.URI)
	line := int(params.Position.Line)
	col := int(params.Position.Character)

	docs := s.store.Get(uri)
	doc := myyaml.DocumentAtPosition(docs, line)

	s.mu.Lock()
	reg := s.registry
	s.mu.Unlock()

	items := Provide(doc, line, col, reg)
	result := make([]protocol.CompletionItem, len(items))
	for i, item := range items {
		kind := protocol.CompletionItemKind(item.Kind)
		detail := item.Detail
		insertText := item.InsertText
		sortText := item.SortText
		result[i] = protocol.CompletionItem{
			Label:      item.Label,
			Kind:       &kind,
			InsertText: &insertText,
			SortText:   &sortText,
		}
		if detail != "" {
			result[i].Detail = &detail
		}
	}
	return result, nil
}

func (s *Server) publishDiagnosticsForURI(notify func(string, any), uri string) {
	docs := s.store.Get(uri)

	s.mu.Lock()
	reg := s.registry
	schemaLoadErr := s.schemaLoadErr
	s.mu.Unlock()

	var lspDiags []protocol.Diagnostic

	if schemaLoadErr != nil {
		msg := "k8s-crd-lsp: unable to load schemas: " + schemaLoadErr.Error()
		sev := protocol.DiagnosticSeverity(SeverityWarning)
		lspDiags = append(lspDiags, protocol.Diagnostic{
			Range:    zeroRange(),
			Severity: &sev,
			Message:  msg,
			Source:   strPtr("k8s-crd-lsp"),
		})
	} else {
		diags := ValidateAll(docs, reg)
		lspDiags = convertDiagnostics(diags)
	}

	notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentUri(uri),
		Diagnostics: lspDiags,
	})
}

func (s *Server) loadSchemas() {
	schemas, err := schema.FetchAllSchemas(s.kubectl)

	s.mu.Lock()
	if err != nil {
		s.schemaLoadErr = err
		s.mu.Unlock()
		return
	}

	s.registry.Load(schemas) //nolint
	s.schemasReady = true
	notifyFn := s.notifyFn
	store := s.store
	s.mu.Unlock()

	// Revalidate all open documents
	if notifyFn == nil {
		return
	}
	for _, uri := range store.URIs() {
		s.publishDiagnosticsForURI(notifyFn, uri)
	}
}

func convertDiagnostics(diags []Diagnostic) []protocol.Diagnostic {
	result := make([]protocol.Diagnostic, len(diags))
	for i, d := range diags {
		sev := protocol.DiagnosticSeverity(d.Severity)
		result[i] = protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(d.StartLine), Character: uint32(d.StartCol)},
				End:   protocol.Position{Line: uint32(d.EndLine), Character: uint32(d.EndCol)},
			},
			Severity: &sev,
			Message:  d.Message,
			Source:   strPtr(d.Source),
		}
	}
	return result
}

func zeroRange() protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}
}

func strPtr(s string) *string {
	return &s
}

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdrwc) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}

type pipeRWC struct {
	r io.Reader
	w io.Writer
}

func (p *pipeRWC) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeRWC) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p *pipeRWC) Close() error                { return nil }
