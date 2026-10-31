package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// reportedWait bounds how long a change reported is waited for: many resyncs, which alone never
// see it, since its stamp is kept.
const reportedWait = 10 * time.Second

// API.md S1, W12, DECISIONS 421: a file an OS notification names is read again whatever its
// stamp, so new bytes keeping their size and modification time arrive as a Change.
func TestWatchEventRereadsKeptStamp(t *testing.T) {
	fsys := newKeptFS()
	p := keptProject(t, fsys)
	fake := &fakeNotifier{ev: make(chan fsnotify.Event), er: make(chan error)}
	changes := make(chan Change, 16)
	ctx, cancel := context.WithCancel(context.Background())
	w, err := p.Watch(ctx, func(_ context.Context, c Change) { changes <- c }, WatchOptions{OS: true, notify: func() (notifier, error) { return fake, nil }})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { cancel(); <-w.Done() }()
	fsys.replace(t)
	deadline := time.After(reportedWait)
	select {
	case fake.ev <- fsnotify.Event{Name: filepath.FromSlash(keptFile), Op: fsnotify.Write}:
	case <-deadline:
		t.Fatal("the watcher took no notification")
	}
	for {
		select {
		case c := <-changes:
			if slices.ContainsFunc(c.Files, func(f Path) bool { return f.Display == keptDisplay }) {
				return
			}
		case <-deadline:
			t.Fatalf("API.md S1: no Change names %s", keptDisplay)
		}
	}
}

// API.md S1, N9, DECISIONS 352, 421: each file a *StaleError of the writer names is read again
// at the refresh after it, whatever its stamp, whichever package's StaleError it is.
func TestWriteStaleSuspects(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
	}{
		{"edit", &edit.StaleError{Files: []string{keptDisplay}}},
		{"workspace", &StaleError{Files: []string{keptDisplay}}},
		{"build", &build.StaleError{Files: []string{keptDisplay}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			fsys := newKeptFS()
			p := keptProject(t, fsys)
			fsys.replace(t)
			after := func() wrote { return wrote{cause: CauseExternal} }
			next, err := p.writeAs(context.Background(), after, func(context.Context, *Snapshot) error { return c.err })
			if !errors.Is(err, c.err) || next == nil {
				t.Fatalf("writeAs: %v, %v", next, err)
			}
			data, _ := next.fs.ReadFile(keptFile)
			if got, _ := readKept(t, p); string(data) != keptChanged || got != keptChanged {
				t.Errorf("API.md S1: after a stale writer the snapshot reads %q, then %q", data, got)
			}
		})
	}
}

// API.md N9, S1, DECISIONS 421: an edit of a file replaced keeping its stamp is stale once, then
// its retry reads the file again and applies.
func TestEditStaleRereadsKeptStamp(t *testing.T) {
	fsys := newKeptFS()
	p := keptProject(t, fsys)
	fsys.replace(t)
	set := EditRequest{Changes: Changes{Host: build.EditHost, Ops: []edit.Operation{
		{Kind: edit.OpSet, Path: "x:n", Value: edit.Int(2)},
	}}}
	_, err := p.Edit(context.Background(), set)
	var stale *edit.StaleError
	if !errors.As(err, &stale) || !slices.Equal(stale.Files, []string{keptDisplay}) {
		t.Fatalf("API.md N9: the first edit gave %v", err)
	}
	out, err := p.Edit(context.Background(), set)
	if err != nil || !out.Applied {
		t.Fatalf("API.md S1: the retried edit gave %+v, %v", out, err)
	}
	data, _ := fsys.ReadFile(keptFile)
	if want := strings.Replace(memoPackage, "= 1", "= 2", 1); string(data) != want {
		t.Errorf("the disk holds %q, want %q", data, want)
	}
}
