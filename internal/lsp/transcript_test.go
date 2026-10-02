package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/workspace/safego"
	"golang.org/x/tools/txtar"
)

// IMPLEMENTATION-PLAN §6 M5, §8.4: scripted JSON-RPC transcripts, testdata/*.txtar.
func TestTranscripts(t *testing.T) {
	// Every file of a case but "script" and "transcript" is written under a temporary directory,
	// $URI in the transcript; "script" drives the server a directive a line; "transcript" is
	// the golden (-update).
	golden.Run(t, "testdata/*.txtar", runTranscript, golden.Expected("transcript"))
}

// patience bounds every wait of a transcript, generous for -race on a loaded machine.
const patience = 30 * time.Second

// session is one scripted client: what it sent and received, as the transcript prints it.
type session struct {
	t      *testing.T
	dir    string
	uri    string
	files  map[string][]byte
	in     *io.PipeWriter
	rec    *recorder
	passes chan struct{}
	seen   int
	out    strings.Builder
}

var directives = map[string]func(*session, []string) error{
	"initialize": (*session).initialize,
	"send":       (*session).sendLine,
	"raw":        (*session).raw,
	"open":       (*session).open,
	"change":     (*session).change,
	"close":      (*session).closeDoc,
	"write":      (*session).write,
	"wait":       (*session).wait,
}

func runTranscript(t *testing.T, c golden.Case) []byte {
	s := newSession(t, c.Archive)
	done := s.start()
	for _, line := range strings.Split(strings.TrimSpace(string(s.files["script"])), "\n") {
		fields := strings.Fields(line)
		fmt.Fprintf(&s.out, "> %s\n", line)
		run, ok := directives[fields[0]]
		if !ok {
			t.Fatalf("unknown directive %q", line)
		}
		if err := run(s, fields[1:]); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		s.drain()
	}
	if err := s.in.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		s.drain()
		fmt.Fprintf(&s.out, "= %v\n", err)
	case <-time.After(patience):
		t.Fatal("Serve did not return")
	}
	return []byte(strings.ReplaceAll(strings.ReplaceAll(s.out.String(), s.uri, "$URI"), s.dir, "$ROOT"))
}

// newSession is a client of a project written from a under a temporary directory.
func newSession(t *testing.T, a *txtar.Archive) *session {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := sessionIn(t, dir)
	for _, f := range a.Files {
		s.files[f.Name] = f.Data
		if f.Name == "script" || f.Name == "transcript" {
			continue
		}
		name := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// sessionIn is a client of the files under dir.
func sessionIn(t *testing.T, dir string) *session {
	dir = filepath.ToSlash(dir)
	s := &session{t: t, dir: dir, uri: uriOf(dir), files: map[string][]byte{}, rec: &recorder{}, passes: make(chan struct{}, 16)}
	s.rec.cond = sync.NewCond(&s.rec.mu)
	return s
}

// start runs the server on the session's pipes; the channel receives what Serve returns.
func (s *session) start() <-chan error {
	return s.startWith(config{debounce: debounce})
}

// startWith is start with cfg, its passed hook the session's.
func (s *session) startWith(cfg config) <-chan error {
	inR, inW := io.Pipe()
	s.in = inW
	done := make(chan error, 1)
	cfg.passed = func() { s.passes <- struct{}{} }
	safego.Go(func() error { return serve(context.Background(), inR, s.rec, cfg) }, func(err error) { done <- err })
	return done
}

// drain prints every message received since the last drain.
func (s *session) drain() {
	for _, m := range s.rec.since(s.seen) {
		fmt.Fprintf(&s.out, "< %s\n", m)
		s.seen++
	}
}

func (s *session) send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.in, headerFormat+"%s", len(body), body)
	return err
}

func (s *session) initialize([]string) error {
	return s.request(map[string]any{"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": map[string]any{"processId": nil, "rootUri": nil, "capabilities": map[string]any{}}})
}

// sendLine sends the rest of the line, $URI replaced; a request (id and method) waits for its response.
func (s *session) sendLine(args []string) error {
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(strings.Join(args, " "), "$URI", s.uri)), &m); err != nil {
		return err
	}
	_, id := m["id"]
	if _, method := m["method"]; id && method {
		return s.request(m)
	}
	return s.send(m)
}

// raw sends the rest of the line as a message body, unparsed, and waits for the error that
// answers it, with a null id.
func (s *session) raw(args []string) error {
	body := strings.Join(args, " ")
	if _, err := fmt.Fprintf(s.in, headerFormat+"%s", len(body), body); err != nil {
		return err
	}
	return s.response(nullID)
}

func (s *session) request(m map[string]any) error {
	if err := s.send(m); err != nil {
		return err
	}
	id, err := json.Marshal(m["id"])
	if err != nil {
		return err
	}
	return s.response(id)
}

// response blocks until the response with id is received.
func (s *session) response(id []byte) error {
	return s.rec.await(func(msg []byte) bool {
		var r struct{ ID json.RawMessage }
		return json.Unmarshal(msg, &r) == nil && bytes.Equal(r.ID, id)
	})
}

func (s *session) note(method string, params any) error {
	return s.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// open sends a file's disk content as an opened buffer.
func (s *session) open(args []string) error {
	data, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(args[0])))
	if err != nil {
		return err
	}
	item := map[string]any{"uri": s.uri + "/" + args[0], "languageId": strings.TrimPrefix(path.Ext(args[0]), "."), "version": 1, "text": string(data)}
	return s.note(methodDidOpen, map[string]any{"textDocument": item})
}

// change sends the archive file args[1] as the full text of the buffer args[0].
func (s *session) change(args []string) error {
	doc := map[string]any{"uri": s.uri + "/" + args[0], "version": 2}
	return s.note(methodDidChange, map[string]any{"textDocument": doc, "contentChanges": []any{map[string]any{"text": string(s.files[args[1]])}}})
}

func (s *session) closeDoc(args []string) error {
	return s.note(methodDidClose, map[string]any{"textDocument": map[string]any{"uri": s.uri + "/" + args[0]}})
}

// write replaces a file on the disk by the archive file args[1].
func (s *session) write(args []string) error {
	return os.WriteFile(filepath.Join(s.dir, filepath.FromSlash(args[0])), s.files[args[1]], 0o644)
}

// wait blocks until a pass published.
func (s *session) wait([]string) error {
	select {
	case <-s.passes:
		return nil
	case <-time.After(patience):
		return fmt.Errorf("no pass in %v", patience)
	}
}

// recorder is the server's output, split into messages as they are written.
type recorder struct {
	mu   sync.Mutex
	cond *sync.Cond
	msgs [][]byte
}

func (r *recorder) Write(p []byte) (int, error) {
	body, err := readFrame(bufio.NewReader(bytes.NewReader(p)))
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, body)
	r.cond.Broadcast()
	return len(p), nil
}

func (r *recorder) since(n int) [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]byte(nil), r.msgs[n:]...)
}

// await blocks until a message matches.
func (r *recorder) await(match func([]byte) bool) error {
	deadline := time.AfterFunc(patience, func() { r.mu.Lock(); r.cond.Broadcast(); r.mu.Unlock() })
	defer deadline.Stop()
	start := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; ; {
		for ; i < len(r.msgs); i++ {
			if match(r.msgs[i]) {
				return nil
			}
		}
		if time.Since(start) > patience {
			return fmt.Errorf("no response in %v", patience)
		}
		r.cond.Wait()
	}
}
