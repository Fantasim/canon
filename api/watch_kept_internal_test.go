package canon

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/workspace"
)

// keptLaw is a package d with a table ps and a layer lf amending its row one.
var keptLaw = map[string][]byte{
	"/law/project.canon":    []byte("project acme {\n  canon: \"0.1\"\n}\n"),
	"/law/d/d.canon":        []byte("/// D.\npackage d\n\n/// A row.\nrecord P {\n  /// V.\n  v: Int = 0\n}\n\n/// Rows.\nlet ps: table P = {\n  one { v: 1 }\n  two { v: 2 }\n}\n"),
	"/law/d/lf.layer.canon": []byte("package d\nlayer lf\n\namend ps {\n  one.v: 2\n}\n"),
}

// keptWait bounds the wait for an edit's event.
const keptWait = 20 * time.Second

// DECISIONS 463, 469, API.md W13, W15: an edit event after the first reads the merged findings its
// re-check kept on the snapshot it published: Checked of that analysis gives the kept result
// itself, the same value at every call, which a fresh merge never is once a layer added one.
func TestWatchEditEventKept(t *testing.T) {
	ctx := context.Background()
	p, err := Open("/law", Options{FS: newWriteFS(keptLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if _, err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	events := make(chan Event, 16)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := p.Watch(wctx, func(ev Event) { events <- ev }); err != nil {
		t.Fatal(err)
	}
	var ev Event
	for _, op := range []Op{Set("d:ps[two].v", Int(7)), Remove("d:ps[one]")} { // a warm-up, then the break
		if res, err := p.Edit(ctx, Edit{Ops: []Op{op}, AllowErrors: true}); err != nil || !res.Applied {
			t.Fatalf("%v, want the draft written", err)
		}
		select {
		case ev = <-events:
		case <-time.After(keptWait):
			t.Fatal("no edit event")
		}
	}
	s, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a := workspace.KeptSelection(s, ev.Packages)
	if ev.Cause != CauseEdit || a == nil {
		t.Fatalf("event %s %v: want an edit event whose snapshot keeps its re-check", ev.Cause, ev.Packages)
	}
	first, err := workspace.Checked(ctx, s, a)
	if err != nil {
		t.Fatal(err)
	}
	again, err := workspace.Checked(ctx, s, a)
	if err != nil || first != again || first == a.Result() {
		t.Errorf("Checked twice: %p, %p (own %p): want the one kept merge, layers' findings added", first, again, a.Result())
	}
	if got := fromDiag(first.Files, first.List); !slices.EqualFunc(got, ev.Findings, func(x, y Finding) bool {
		return x.Code == y.Code && x.File == y.File && x.Line == y.Line && x.Layer == y.Layer
	}) {
		t.Errorf("event %v, kept %v: want the event to report the kept merge", ev.Findings, got)
	}
}
