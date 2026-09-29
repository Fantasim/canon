package canon_test

import (
	"context"
	"errors"
	"maps"
	"path"
	"runtime"
	"slices"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// watchLaw is three packages: a loads a file beside it, b imports a, c stands alone.
var watchLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"a.json\")\n",
	"a/a.json":      "[1, 2]\n",
	"b/b.canon":     "/// B.\npackage b\n\nimport a\n\n/// N.\nconst N = 1\n",
	"c/c.canon":     "/// C.\npackage c\n\n/// M.\nconst M = 1\n",
}

// eventWait bounds every wait for an event, generous for -race on a loaded machine; eventNone is
// how long no event must come, several coalescing windows (API.md W14).
const (
	eventWait = 20 * time.Second
	eventNone = 500 * time.Millisecond
)

// watch starts a watch of p, its events sent to the channel returned, stopped when the test ends.
func watch(t *testing.T, p *canon.Project, fn func(canon.Event)) (<-chan canon.Event, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	events := make(chan canon.Event, 64)
	if err := p.Watch(ctx, func(ev canon.Event) {
		if fn != nil {
			fn(ev)
		}
		events <- ev
	}); err != nil {
		t.Fatal(err)
	}
	return events, cancel
}

func nextEvent(t *testing.T, events <-chan canon.Event) canon.Event {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(eventWait):
		t.Fatal("no event")
	}
	return canon.Event{}
}

func noEvent(t *testing.T, events <-chan canon.Event, why string) {
	t.Helper()
	select {
	case ev := <-events:
		t.Errorf("%s: an event %s %v", why, ev.Cause, ev.Files)
	case <-time.After(eventNone):
	}
}

// openChecked is watchLaw opened and checked, every package loaded, a's load read (W12).
func openChecked(t *testing.T) (*canon.Project, canon.Options) {
	t.Helper()
	p, opts := openLaw(t, watchLaw)
	if _, err := p.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, opts
}

func writeLaw(t *testing.T, fsys canon.FS, name, text string) {
	t.Helper()
	if err := fsys.WriteFile("/law/"+name, []byte(text)); err != nil {
		t.Fatal(err)
	}
}

// API.md W12, W14, S2: a change is re-checked in the packages that read it and in those
// importing them, their reads learnt when the watch started; Findings replace what the packages
// held; the revision is the snapshot's.
func TestWatchRecheck(t *testing.T) {
	p, opts := openLaw(t, watchLaw)
	events, _ := watch(t, p, nil)
	writeLaw(t, opts.FS, "a/a.json", "[3]\n")
	ev := nextEvent(t, events)
	if ev.Cause != canon.CauseExternal || !slices.Equal(ev.Files, []string{"a/a.json"}) ||
		!slices.Equal(ev.Packages, []string{"a", "b"}) || ev.Err != nil || ev.Revision != p.Revision() {
		t.Fatalf("first event: %+v", ev)
	}
	writeLaw(t, opts.FS, "c/c.canon", "/// C.\npackage c\n\n/// M.\nconst M: Int = \"x\"\n")
	ev = nextEvent(t, events)
	if !slices.Equal(ev.Packages, []string{"c"}) || ev.Summary.Errors == 0 || len(ev.Findings) == 0 || ev.Findings[0].Package != "c" {
		t.Fatalf("an error in c: %+v", ev)
	}
	writeLaw(t, opts.FS, "c/c.canon", watchLaw["c/c.canon"])
	if ev = nextEvent(t, events); !slices.Equal(ev.Packages, []string{"c"}) || len(ev.Findings) != 0 || ev.Summary.Errors != 0 {
		t.Fatalf("c fixed: %+v", ev)
	}
	writeLaw(t, opts.FS, "a/a.json", "[4]\n")
	if ev = nextEvent(t, events); !slices.Equal(ev.Packages, []string{"a", "b"}) {
		t.Errorf("a's load changed: packages %v, want a and its importer b", ev.Packages)
	}
}

// API.md W15: overlays are overlay events; a writing Build's own writes are one edit event,
// never reported again as external.
func TestWatchOwnChanges(t *testing.T) {
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": tierPackage("a")})
	p := openTierProject(t, fsys)
	ctx := context.Background()
	if _, err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	events, _ := watch(t, p, nil)
	if err := p.SetOverlay("a/a.canon", append(tierPackage("a"), '\n')); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); ev.Cause != canon.CauseOverlay || !slices.Equal(ev.Files, []string{"a/a.canon"}) || !slices.Equal(ev.Packages, []string{"a"}) {
		t.Errorf("overlay event: %+v", ev)
	}
	if err := p.ClearOverlay("a/a.canon"); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); ev.Cause != canon.CauseOverlay {
		t.Errorf("cleared overlay event: %s", ev.Cause)
	}
	res, err := p.Build(ctx, canon.BuildOptions{})
	if err != nil || res.Check.HasErrors() {
		t.Fatalf("Build: %v", err)
	}
	ev := nextEvent(t, events)
	if ev.Cause != canon.CauseEdit || ev.Revision != res.Check.Revision || !slices.Contains(ev.Files, "a/canon.lock") {
		t.Errorf("the build's event: %s %s %v, want edit at %s", ev.Cause, ev.Revision, ev.Files, res.Check.Revision)
	}
	noEvent(t, events, "after the build's own event")
}

// API.md W16: every watch receives every event.
func TestWatchTwice(t *testing.T) {
	p, opts := openLaw(t, watchLaw)
	one, _ := watch(t, p, nil)
	two, _ := watch(t, p, nil)
	writeLaw(t, opts.FS, "b/b.canon", watchLaw["b/b.canon"]+"\n")
	if a, b := nextEvent(t, one), nextEvent(t, two); a.Revision != b.Revision || !slices.Equal(a.Files, b.Files) {
		t.Errorf("two watches: %s %v and %s %v", a.Revision, a.Files, b.Revision, b.Files)
	}
}

// API.md W12, O6: a watch ends with its ctx; Close ends every watch, and Watch after it is
// ErrClosed; nothing is left running.
func TestWatchEnds(t *testing.T) {
	p, opts := openChecked(t)
	base := runtime.NumGoroutine()
	cancelled, cancelOne := watch(t, p, nil)
	closed, _ := watch(t, p, nil)
	cancelOne()
	writeLaw(t, opts.FS, "a/a.json", "[5]\n")
	nextEvent(t, closed)
	noEvent(t, cancelled, "a watch whose ctx is done")
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	writeLaw(t, opts.FS, "a/a.json", "[6]\n")
	noEvent(t, closed, "a watch of a closed project")
	if err := p.Watch(context.Background(), func(canon.Event) {}); !errors.Is(err, canon.ErrClosed) {
		t.Errorf("Watch after Close: %v", err)
	}
	deadline := time.Now().Add(eventWait)
	for runtime.NumGoroutine() > base && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > base {
		t.Errorf("%d goroutines left, %d before", n, base)
	}
}

// API.md X2: a panic in fn does not end the watch; it is an *InternalError on the next event.
func TestWatchFnPanics(t *testing.T) {
	p, opts := openChecked(t)
	calls, first := 0, make(chan struct{})
	events, _ := watch(t, p, func(canon.Event) {
		calls++ // on the watch's one goroutine (W13)
		if calls == 1 {
			close(first)
			panic("client bug")
		}
	})
	writeLaw(t, opts.FS, "a/a.json", "[7]\n")
	select {
	case <-first:
	case <-time.After(eventWait):
		t.Fatal("no event")
	}
	writeLaw(t, opts.FS, "a/a.json", "[8]\n")
	ev := nextEvent(t, events)
	var ierr *canon.InternalError
	if !errors.As(ev.Err, &ierr) || ierr.Msg != "client bug" || ierr.Stack == "" {
		t.Errorf("the event after a panic: %v", ev.Err)
	}
	writeLaw(t, opts.FS, "a/a.json", "[9]\n")
	if ev := nextEvent(t, events); ev.Err != nil {
		t.Errorf("the event after that: %v", ev.Err)
	}
}

// API.md S3 (log-2026-09-29 M4 U5-r 3): an event's revision is taken after its re-check read what
// it needed, a load file read for the first time included: a Check of that snapshot agrees.
func TestWatchRevisionAfterRecheck(t *testing.T) {
	p, opts := openLaw(t, watchLaw)
	events, _ := watch(t, p, nil)
	writeLaw(t, opts.FS, "c/n.json", "[5]\n")
	writeLaw(t, opts.FS, "c/c.canon", watchLaw["c/c.canon"]+"\n/// Ns.\nlet ns: [Int] = load(\"n.json\")\n")
	ev := nextEvent(t, events)
	res, err := p.Check(context.Background(), "c")
	if err != nil || ev.Revision != res.Revision || !slices.Equal(ev.Packages, []string{"c"}) {
		t.Errorf("event at %s, Check at %s (%v), packages %v", ev.Revision, res.Revision, err, ev.Packages)
	}
}

// API.md S2 (log-2026-09-29 M4 U5-r 4): a package removed is listed with no findings, even before
// the first event and while project.canon was broken, which is Err with its findings.
func TestWatchRemovedPackages(t *testing.T) {
	p, opts := openLaw(t, watchLaw)
	events, _ := watch(t, p, nil)
	if err := opts.FS.Remove("/law/c/c.canon"); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); !slices.Equal(ev.Packages, []string{"c"}) || len(ev.Findings) != 0 || !slices.Equal(ev.Files, []string{"c/c.canon"}) {
		t.Fatalf("c removed: %+v", ev)
	}
	writeLaw(t, opts.FS, "project.canon", "project acme {\n  canon: \n}\n")
	ev := nextEvent(t, events)
	var perr *canon.ProjectError
	if !errors.As(ev.Err, &perr) || len(ev.Findings) == 0 || len(ev.Packages) != 0 {
		t.Fatalf("project.canon broken: %+v", ev)
	}
	if err := opts.FS.Remove("/law/b/b.canon"); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); !errors.Is(ev.Err, canon.ErrProject) {
		t.Fatalf("b removed while broken: %+v", ev)
	}
	writeLaw(t, opts.FS, "project.canon", watchLaw["project.canon"])
	if ev := nextEvent(t, events); ev.Err != nil || !slices.Equal(ev.Packages, []string{"a", "b"}) {
		t.Errorf("project.canon fixed: %v, packages %v, want a re-checked and b removed", ev.Err, ev.Packages)
	}
}

// API.md W12 (log-2026-09-29 M4 U5-r 7): editor files and .canon/ are nothing a package reads: no
// event, and never among an event's files.
func TestWatchIgnoresOtherFiles(t *testing.T) {
	p, opts := openLaw(t, watchLaw)
	events, _ := watch(t, p, nil)
	writeLaw(t, opts.FS, "a/.a.canon.swp", "swap")
	writeLaw(t, opts.FS, ".canon/cache/entry", "cache")
	noEvent(t, events, "a swap file and a .canon/ write")
	writeLaw(t, opts.FS, "b/.b.canon.swp", "swap")
	writeLaw(t, opts.FS, "b/b.canon", watchLaw["b/b.canon"]+"\n")
	if ev := nextEvent(t, events); !slices.Equal(ev.Files, []string{"b/b.canon"}) {
		t.Errorf("files %v, want b/b.canon alone", ev.Files)
	}
}

// API.md W12 (log-2026-09-29 M4 U5-r3): a new file in a directory a load.dir glob lists is an
// event, re-checked; a file the glob does not select is none, nor a re-check.
func TestWatchLoadDirectory(t *testing.T) {
	files := maps.Clone(watchLaw)
	files["d/d.canon"] = "/// D.\npackage d\n\n/// Ns.\nlet ns: [Int] = load.dir(\"rows/*.json\")\n"
	files["d/rows/1.json"] = "1\n"
	p, opts := openLaw(t, files)
	events, _ := watch(t, p, nil)
	writeLaw(t, opts.FS, "d/rows/2.json", "2\n")
	if ev := nextEvent(t, events); !slices.Equal(ev.Packages, []string{"d"}) || !slices.Equal(ev.Files, []string{"d/rows/2.json"}) || ev.Err != nil {
		t.Errorf("a new row: %+v", ev)
	}
	writeLaw(t, opts.FS, "d/rows/notes.txt", "not a row")
	noEvent(t, events, "a file the glob does not select")
	writeLaw(t, opts.FS, "d/rows/other/readme.txt", "not a row either")
	noEvent(t, events, "a new directory holding no file the glob selects")
	if err := opts.FS.Remove("/law/d/rows/1.json"); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, events); !slices.Equal(ev.Packages, []string{"d"}) || !slices.Equal(ev.Files, []string{"d/rows/1.json"}) {
		t.Errorf("a row deleted: %+v", ev)
	}
}

// API.md W12, S2 (log-2026-09-29 M4 U5-r2): a file read in a directory that did not exist, and
// one below new directories a recursive glob lists, are events that re-check their package and
// name the file, though no listing the snapshot held names it.
func TestWatchNewDirectories(t *testing.T) {
	cases := []struct{ name, src, file, text string }{
		{"a load in a missing directory", "let xs: [Int] = load(\"gen/x.json\")", "e/gen/x.json", "[1]\n"},
		{"a recursive glob", "let xs: [Int] = load.dir(\"rows/**/*.json\")", "e/rows/sub/deep/2.json", "2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(watchLaw)
			files["e/e.canon"] = "/// E.\npackage e\n\n/// Xs.\n" + tc.src + "\n"
			files["e/rows/1.json"] = "1\n"
			p, opts := openLaw(t, files)
			events, _ := watch(t, p, nil)
			writeLaw(t, opts.FS, tc.file, tc.text)
			ev := nextEvent(t, events)
			if !slices.Equal(ev.Packages, []string{"e"}) || !slices.Contains(ev.Files, tc.file) {
				t.Errorf("event: packages %v, files %v, err %v", ev.Packages, ev.Files, ev.Err)
			}
			res, err := p.Check(context.Background())
			if err != nil || res.Revision != ev.Revision {
				t.Errorf("event at %s, Check at %s (%v)", ev.Revision, res.Revision, err)
			}
			writeLaw(t, opts.FS, "e/rows/other/deep/readme.txt", "no row")
			noEvent(t, events, "new directories holding no file a glob selects")
		})
	}
}

// API.md W12, S1 (log-2026-09-29 M4 U5-r4): a load.dir of CSV or text files is watched by its
// own glob too: a new matching file is one event naming it, any other file none.
func TestWatchLoadDirectoryFormats(t *testing.T) {
	cases := []struct{ name, glob, file, text, other string }{
		{"csv", "rows/*.csv", "f/rows/2.csv", "a\n2\n", "f/rows/notes.txt"},
		{"text", "notes/*.txt", "f/notes/2.txt", "two\n", "f/notes/2.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(watchLaw)
			files["f/f.canon"] = "/// F.\npackage f\n\n/// Xs.\nlet xs: [Int] = load.dir(\"" + tc.glob + "\")\n"
			files[path.Dir(tc.file)+"/1"+path.Ext(tc.file)] = tc.text
			p, opts := openLaw(t, files)
			events, _ := watch(t, p, nil)
			writeLaw(t, opts.FS, tc.other, "{}\n")
			noEvent(t, events, "a file the glob does not select")
			writeLaw(t, opts.FS, tc.file, tc.text)
			if ev := nextEvent(t, events); !slices.Equal(ev.Packages, []string{"f"}) || !slices.Contains(ev.Files, tc.file) {
				t.Errorf("a new matching file: packages %v, files %v, err %v", ev.Packages, ev.Files, ev.Err)
			}
			noEvent(t, events, "after the one event")
		})
	}
}
