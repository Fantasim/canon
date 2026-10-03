package lsp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/fantasim/canonlang/internal/workspace/safego"
	"golang.org/x/tools/txtar"
)

// A request into a project being opened is answered null, and the reader keeps reading.
func TestRequestIntoOpening(t *testing.T) {
	// DECISIONS 285; IMPLEMENTATION-PLAN §8.4 Sync
	entered, release := make(chan struct{}), make(chan struct{})
	var once, freed sync.Once
	free := func() { freed.Do(func() { close(release) }) }
	t.Cleanup(free) // a failed wait still lets the open, then the server, end
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, opening: func() {
		once.Do(func() { close(entered) })
		<-release
	}})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	waitClosed(t, entered)
	must(t, s.sendLine([]string{requestAt(1, methodDefinition, "proj/a/a.canon")}))
	must(t, s.open([]string{"loose/x.canon"}))
	must(t, s.sendLine([]string{requestAt(2, methodHover, "loose/x.canon")}))
	answers := string(joined(s.rec.since(0)))
	for _, want := range []string{`"id":1,"result":null`, `"id":2,"result":null`} {
		if !strings.Contains(answers, want) {
			t.Errorf("no %s while the open is held: %s", want, answers)
		}
	}
	free()
	for !strings.Contains(strings.Join(s.publications(), "\n"), "/proj/a/a.canon") {
		must(t, s.wait(nil))
	}
	must(t, s.in.Close())
	<-done
}

// A request waiting for its project's sync is cancelled by a $/cancelRequest read meanwhile.
func TestCancelWhileWaiting(t *testing.T) {
	// DECISIONS 285; LSP 3.17 $/cancelRequest
	entered, release := make(chan struct{}), make(chan struct{})
	var syncs atomic.Int32
	var freed sync.Once
	free := func() { freed.Do(func() { close(release) }) }
	t.Cleanup(free)
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, syncing: func(context.Context) {
		if syncs.Add(1) == 2 {
			close(entered)
			<-release
		}
	}})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	must(t, s.wait(nil))
	must(t, s.change([]string{"proj/a/a.canon", "edits/bool.canon"}))
	waitClosed(t, entered)
	must(t, s.send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": methodHover, "params": positionParamsOf(s, "proj/a/a.canon")}))
	must(t, s.note(methodCancel, map[string]any{"id": 3}))
	must(t, s.response([]byte("3")))
	if answer := string(joined(s.rec.since(0))); !strings.Contains(answer, fmt.Sprintf(`"id":3,"error":{"code":%d`, codeCancelled)) {
		t.Errorf("the waiting request was not cancelled: %s", answer)
	}
	free()
	must(t, s.wait(nil))
	must(t, s.in.Close())
	<-done
}

// requestAt is a request at the start of a file, as a script line.
func requestAt(id int, method, file string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"%s","params":{"textDocument":{"uri":"$URI/%s"},"position":{"line":0,"character":0}}}`, id, method, file)
}

func positionParamsOf(s *session, file string) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": s.uri + "/" + file}, "position": map[string]any{"line": 4, "character": 4}}
}

// A request whose document changes while it is answered answers ContentModified.
func TestModifiedWhileAnswering(t *testing.T) {
	// DECISIONS 285; LSP 3.17 ContentModified
	entered, release := make(chan struct{}), make(chan struct{})
	var once, freed sync.Once
	free := func() { freed.Do(func() { close(release) }) }
	t.Cleanup(free)
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, answering: func() {
		once.Do(func() {
			close(entered)
			<-release
		})
	}})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	must(t, s.wait(nil))
	must(t, s.send(map[string]any{"jsonrpc": "2.0", "id": 4, "method": methodHover, "params": positionParamsOf(s, "proj/a/a.canon")}))
	waitClosed(t, entered)
	must(t, s.change([]string{"proj/a/a.canon", "edits/bool.canon"}))
	must(t, s.sendLine([]string{fmt.Sprintf(`{"jsonrpc":"2.0","id":5,"method":"%s","params":{"textDocument":{"uri":"$URI/proj/a/a.canon"}}}`, methodFormatting)}))
	free()
	must(t, s.response([]byte("4")))
	if answer := string(joined(s.rec.since(0))); !strings.Contains(answer, fmt.Sprintf(`"id":4,"error":{"code":%d`, codeModified)) {
		t.Errorf("the request answered for text that changed: %s", answer)
	}
	must(t, s.in.Close())
	<-done
}

// A sync never applies buffers older than those applied, and a request reads them under the turn.
func TestBuffersNeverGoBack(t *testing.T) {
	// DECISIONS 285
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	abs := s.dir + "/proj/a/a.canon"
	var mu sync.Mutex
	now := buffers{docs: map[string][]byte{abs: s.files["proj/a/a.canon"]}, gen: 1}
	read := func() buffers { mu.Lock(); defer mu.Unlock(); return now }
	p := newProject(s.dir+"/proj", projectHooks{}, read)
	defer p.close()
	ctx := context.Background()
	must(t, p.sync(ctx))
	newer := buffers{docs: map[string][]byte{abs: s.files["edits/bool.canon"]}, gen: 3}
	must(t, p.take(ctx))
	synced := make(chan error, 1)
	safego.Go(func() error { _, err := p.opened(ctx); return err }, func(err error) { synced <- err })
	mu.Lock()
	now = newer // the request reads the buffers once it holds the turn: these
	mu.Unlock()
	p.give()
	must(t, <-synced)
	must(t, p.take(ctx))
	must(t, p.overlay(ctx, buffers{docs: map[string][]byte{abs: []byte("stale")}, gen: 2}))
	p.give()
	if string(p.applied[abs]) != string(newer.docs[abs]) || p.gen != newer.gen {
		t.Errorf("applied %q at generation %d, want the newest buffers, generation %d", p.applied[abs], p.gen, newer.gen)
	}
}
