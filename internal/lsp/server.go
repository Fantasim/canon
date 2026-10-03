package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// Serve runs the language server on in and out until exit, the end of in, or ctx's end. It
// returns nil after shutdown then exit (or end of input), ErrNoShutdown when they came without
// shutdown, else the error that stopped it.
func Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	return serve(ctx, in, out, config{debounce: debounce})
}

// config is what tests vary: the debounce, a hook told after each pass published, one called as
// a pass starts computing, one as a project opens, one as the pass syncs a project, and one as a
// background request starts.
type config struct {
	debounce  time.Duration
	passed    func()
	computing func(ctx context.Context)
	opening   func()
	syncing   func(ctx context.Context)
	answering func()
}

// server is one session: the lifecycle on the reading goroutine, documents and projects behind
// mu, shared with the pass goroutine (schedule.go).
type server struct {
	out   *writer
	cfg   config
	phase phase
	kick  chan struct{}

	mu       sync.Mutex
	docs     map[string]*document // by absolute name
	projects map[string]*workspaceProject
	dirty    map[string]bool // project roots, and loose documents by absolute name, to recompute
	cancel   context.CancelFunc

	published map[string]map[string][]diagnostic // the pass goroutine's: by file, by owner

	running  map[string]*runningRequest // the requests running off the reading goroutine, by id; under mu
	inflight sync.WaitGroup
	gen      uint64            // the buffers' generation, bumped by every open, change and close; under mu
	versions map[string]uint64 // each open document's generation at its last open or change; under mu
}

type (
	requestHandler func(s *server, ctx context.Context, params json.RawMessage) (any, error)
	noteHandler    func(s *server, params json.RawMessage) error
)

// requests and notes dispatch by method; a method in neither is not offered (DECISIONS 274).
var (
	requests map[string]requestHandler
	notes    map[string]noteHandler
)

func init() {
	requests = map[string]requestHandler{
		methodInitialize: (*server).initialize,
		methodShutdown:   (*server).shutdown,
		methodHover:      (*server).hover,
		methodDefinition: (*server).definition,
		methodReferences: (*server).references,
		methodFormatting: (*server).formatting,
	}
	notes = map[string]noteHandler{
		methodInitialized: (*server).ignore,
		methodDidOpen:     (*server).didOpen,
		methodDidChange:   (*server).didChange,
		methodDidClose:    (*server).didClose,
		methodWatched:     (*server).didChangeWatched,
		methodCancel:      (*server).cancelRequest,
	}
}

func serve(ctx context.Context, in io.Reader, out io.Writer, cfg config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &server{
		out: &writer{out: out}, cfg: cfg, kick: make(chan struct{}, 1), docs: map[string]*document{},
		projects: map[string]*workspaceProject{}, dirty: map[string]bool{}, published: map[string]map[string][]diagnostic{},
		running: map[string]*runningRequest{}, versions: map[string]uint64{},
	}
	passes := make(chan struct{})
	safego.Go(func() error { return s.schedule(ctx) }, func(err error) { s.logged(err); close(passes) })
	defer func() { cancel(); <-passes; s.inflight.Wait(); s.close() }()
	frames := make(chan frame)
	safego.Go(func() error { return readFrames(ctx, in, frames) }, func(err error) {
		select {
		case frames <- frame{err: err}:
		case <-ctx.Done():
		}
	})
	return s.loop(ctx, frames)
}

// loop handles messages in order until one ends the session.
func (s *server) loop(ctx context.Context, frames <-chan frame) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case f := <-frames:
			if done, err := s.frame(ctx, f); done || err != nil {
				return err
			}
		}
	}
}

// frame handles one frame read; done is true when it ends the session.
func (s *server) frame(ctx context.Context, f frame) (done bool, err error) {
	switch {
	case errors.Is(f.err, errTooLarge):
		s.logged(s.respond(nullID, nil, f.err))
	case f.err != nil:
		return true, s.ended(f.err)
	default:
		if done, err := s.dispatch(ctx, f.body); done || err != nil {
			return true, err
		}
	}
	return false, s.out.failed()
}

// ended is the result of the input ending with err.
func (s *server) ended(err error) error {
	switch {
	case !errors.Is(err, io.EOF):
		return err
	case s.phase == phaseShutdown:
		return nil
	}
	return ErrNoShutdown
}

// dispatch handles one message; done is true on exit.
func (s *server) dispatch(ctx context.Context, body []byte) (done bool, err error) {
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return false, s.respond(nullID, nil, fmt.Errorf(fmtWrap, errParse, err))
	}
	switch {
	case m.Method == methodExit:
		return true, s.ended(io.EOF)
	case m.Method == "" && m.ID == nil:
		return false, s.respond(nullID, nil, errInvalidRequest) // neither a request nor a response (JSON-RPC 2.0 §5)
	case m.Method == "":
		return false, nil // a response: the server sends no request
	case m.ID == nil:
		s.note(m)
		return false, nil
	case m.JSONRPC != jsonrpcV2:
		return false, s.respond(m.ID, nil, errInvalidRequest)
	}
	h, err := s.handler(m)
	switch {
	case err != nil:
		return false, s.respond(m.ID, nil, err)
	case readers[m.Method]:
		s.background(ctx, m, h)
		return false, nil
	}
	result, err := h(s, ctx, m.Params)
	return false, s.respond(m.ID, result, err)
}

// handler is a request's handler, once the lifecycle allows it (LSP 3.17 lifecycle).
func (s *server) handler(m message) (requestHandler, error) {
	switch {
	case s.phase == phaseNew && m.Method != methodInitialize:
		return nil, errNotInitialized
	case s.phase != phaseNew && m.Method == methodInitialize, s.phase == phaseShutdown:
		return nil, errInvalidRequest
	}
	h, ok := requests[m.Method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errMethodNotFound, m.Method)
	}
	return h, nil
}

// note runs a notification's handler while the server runs; one that fails is logged, an
// unknown one ignored ($/ ones included, as the protocol allows).
func (s *server) note(m message) {
	h, ok := notes[m.Method]
	if !ok || s.phase != phaseRunning || strings.HasPrefix(m.Method, reservedNote) && m.Method != methodCancel {
		return
	}
	s.logged(h(s, m.Params))
}

// logged sends err to the client's log, if any (window/logMessage).
func (s *server) logged(err error) {
	if err == nil {
		return
	}
	_ = s.notify(methodLog, logMessage{Type: messageError, Message: err.Error()}) // a failed write sticks: loop returns it
}

type logMessage struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

type initializeResult struct {
	Capabilities capabilities `json:"capabilities"`
	ServerInfo   serverInfo   `json:"serverInfo"`
}

type capabilities struct {
	PositionEncoding           string      `json:"positionEncoding"`
	TextDocumentSync           syncOptions `json:"textDocumentSync"`
	HoverProvider              bool        `json:"hoverProvider"`
	DefinitionProvider         bool        `json:"definitionProvider"`
	ReferencesProvider         bool        `json:"referencesProvider"`
	DocumentFormattingProvider bool        `json:"documentFormattingProvider"`
}

type syncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
}

type serverInfo struct {
	Name string `json:"name"`
}

// initialize offers UTF-16 positions, full-document sync, hover, definition, references and
// formatting; completion, code actions and rename are not offered (DECISIONS 274).
func (s *server) initialize(context.Context, json.RawMessage) (any, error) {
	s.phase = phaseRunning
	return initializeResult{
		Capabilities: capabilities{
			PositionEncoding: encodingUTF16, TextDocumentSync: syncOptions{OpenClose: true, Change: syncFull},
			HoverProvider: true, DefinitionProvider: true, ReferencesProvider: true, DocumentFormattingProvider: true,
		},
		ServerInfo: serverInfo{Name: canonName},
	}, nil
}

func (s *server) shutdown(context.Context, json.RawMessage) (any, error) {
	s.phase = phaseShutdown
	return nil, nil
}

func (s *server) ignore(json.RawMessage) error { return nil }
