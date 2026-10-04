package canon_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

const buildTestProject = "project a {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"

// tierPackage is one package with a stable table, so a build appends canon.lock and writes json.
func tierPackage(pkg string) []byte {
	return []byte("/// " + pkg + ".\npackage " + pkg + "\n\n/// Tier.\nrecord Tier {\n" +
		"  /// Weight.\n  weight: Int = 1\n}\n\n/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\n" +
		"emit json { out: \"@out/" + pkg + "/\" }\n")
}

func openTierProject(t *testing.T, fsys canon.FS) *canon.Project {
	t.Helper()
	p, err := canon.Open("/law", canon.Options{FS: fsys, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// API.md B1b, DECISIONS 201: an unknown Target is refused with *ValueError, never dropped, and a list that
// is entirely unknown must not widen to every target (a build of every emit would be a lie).
func TestBuildUnknownTarget(t *testing.T) {
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": tierPackage("a")})
	p := openTierProject(t, fsys)
	_, err := p.Build(context.Background(), canon.BuildOptions{Targets: []canon.Target{"bogus"}})
	var verr *canon.ValueError
	if !errors.Is(err, canon.ErrBadValue) || !errors.As(err, &verr) || verr.Expected != "go, cpp, ts, json, view or text" || verr.Got != `"bogus"` {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := fsys.files["/law/out/a/tiers.json"]; ok {
		t.Error("an all-unknown target list wrote an output")
	}
}

// API.md B1b, S10: a writer publishes its new snapshot atomically; the revision in its result, and in
// Project.Revision after it returns, is that snapshot's (canon.lock just changed).
func TestBuildRevisionAfterWrite(t *testing.T) {
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": tierPackage("a")})
	p := openTierProject(t, fsys)
	before := p.Revision()
	res, err := p.Build(context.Background(), canon.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Check.Revision == "" || res.Check.Revision == before {
		t.Errorf("Check.Revision = %q, want a fresh one (before %q)", res.Check.Revision, before)
	}
	if got := p.Revision(); got != res.Check.Revision {
		t.Errorf("Revision() = %q, want %q", got, res.Check.Revision)
	}
	// A Check-mode Build is a read (S8): it never re-reads or changes the revision.
	again, err := p.Build(context.Background(), canon.BuildOptions{Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if again.Check.Revision != res.Check.Revision || p.Revision() != res.Check.Revision {
		t.Errorf("a Check-mode build changed the revision: %q", again.Check.Revision)
	}
}

// writeProbe widens the window of a write so two overlapping ones would be caught (S9).
type writeProbe struct {
	canon.FS
	mu     sync.Mutex
	active int
	max    int
}

func (w *writeProbe) WriteFile(name string, data []byte) error {
	w.mu.Lock()
	w.active++
	w.max = max(w.max, w.active)
	w.mu.Unlock()
	time.Sleep(time.Millisecond)
	err := w.FS.WriteFile(name, data)
	w.mu.Lock()
	w.active--
	w.mu.Unlock()
	return err
}

// API.md S7, S8, S9: a Project is safe for concurrent use; a writing Build never overlaps another one, and
// a Check (a read, S8) runs alongside a writer without error. Run with -race.
func TestBuildSerializesWrites(t *testing.T) {
	files := map[string][]byte{"/law/project.canon": []byte(buildTestProject)}
	for _, pkg := range []string{"a", "b", "c"} {
		files["/law/"+pkg+"/"+pkg+".canon"] = tierPackage(pkg)
	}
	probe := &writeProbe{FS: newMemFS(files)}
	p := openTierProject(t, probe)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Build(context.Background(), canon.BuildOptions{}); err != nil {
				errs <- err
			}
		}()
	}
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Check(context.Background()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if probe.max > 1 {
		t.Errorf("writes overlapped: %d at once, want at most 1", probe.max)
	}
}
