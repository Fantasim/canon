package lsp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

// passLaw is a project whose one file has a type error, and a loose file with a syntax error.
const passLaw = `-- proj/project.canon --
project acme {
  canon: "0.1"
}
-- proj/a/a.canon --
/// A.
package a

/// N.
let n: Int = "x"
-- loose/x.canon --
let = 1
-- edits/bool.canon --
/// A.
package a

/// N.
let n: Bool = 2
`

// quiet is how long no further pass may come: several debounces.
const quiet = 4 * debounce

// IMPLEMENTATION-PLAN §8.4 Sync: a change cancels the pass; one fresh publication follows.
func TestCancelDuringPass(t *testing.T) {
	entered := make(chan struct{})
	first := true
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, computing: func(ctx context.Context) {
		if first {
			first = false
			close(entered)
			<-ctx.Done()
		}
	}})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	waitClosed(t, entered)
	must(t, s.change([]string{"proj/a/a.canon", "edits/bool.canon"}))
	must(t, s.wait(nil))
	expectQuiet(t, s)
	published := s.publications()
	if len(published) != 1 || !strings.Contains(published[0], "expected Bool") {
		t.Fatalf("publications %q, want one, of the changed text", published)
	}
	must(t, s.in.Close())
	<-done
}

// log-2026-10-02 L1: a pass that panics is logged and its dirty keys wait for the next pass.
func TestPanickedPassRequeues(t *testing.T) {
	panicked := make(chan struct{})
	first := true
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, computing: func(context.Context) {
		if first {
			first = false
			close(panicked)
			panic("seam")
		}
	}})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	waitClosed(t, panicked)
	must(t, s.open([]string{"loose/x.canon"}))
	must(t, s.wait(nil))
	published := strings.Join(s.publications(), "\n")
	for _, want := range []string{"/proj/a/a.canon", "/loose/x.canon"} {
		if !strings.Contains(published, want) {
			t.Errorf("no publication for %s after the panic: %s", want, published)
		}
	}
	must(t, s.in.Close())
	<-done
	if !strings.Contains(string(joined(s.rec.since(0))), "panic on a background goroutine: seam") {
		t.Error("the panic was not logged")
	}
}

// IMPLEMENTATION-PLAN §8.4 Sync: a finding with no file is published on project.canon at 0:0.
func TestNoFileOnProjectCanon(t *testing.T) {
	set := &source.FileSet{}
	bag := diag.NewBag(set, "")
	diag.E1003.At(source.Span{}, "/r").Report(bag)
	got, err := diagnosticsOf(build.Findings{Files: set, List: bag.Findings()}, "/r")
	must(t, err)
	list := got[project.Join("/r", project.FileName)]
	if len(got) != 1 || len(list) != 1 || list[0].Range != (textRange{}) || list[0].Code != string(diag.E1003.Def().Code) {
		t.Fatalf("got %+v", got)
	}
}

func waitClosed(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(patience):
		t.Fatal("the seam was not reached")
	}
}

// expectQuiet fails when another pass publishes within quiet.
func expectQuiet(t *testing.T, s *session) {
	t.Helper()
	select {
	case <-s.passes:
		t.Fatal("a second pass published")
	case <-time.After(quiet):
	}
}

// publications are the publishDiagnostics messages received so far.
func (s *session) publications() []string {
	var out []string
	for _, m := range s.rec.since(0) {
		if strings.Contains(string(m), methodPublish) {
			out = append(out, string(m))
		}
	}
	return out
}

func joined(msgs [][]byte) []byte {
	var out []byte
	for _, m := range msgs {
		out = append(out, m...)
	}
	return out
}
