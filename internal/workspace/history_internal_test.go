package workspace

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

// clockFS is one file whose content and modification time a test sets.
type clockFS struct {
	fstest.MapFS
}

func (c *clockFS) ReadFile(name string) ([]byte, error)       { return c.MapFS.ReadFile(name[1:]) }
func (c *clockFS) Stat(name string) (fs.FileInfo, error)      { return c.MapFS.Stat(name[1:]) }
func (c *clockFS) ReadDir(name string) ([]fs.DirEntry, error) { return c.MapFS.ReadDir(name[1:]) }

// API.md S1: a file hashed within 2 s of its modification time is hashed again at each refresh
// ("racily clean"), so a change within the same timestamp and size is seen; one hashed later is
// trusted to its stat.
func TestRefreshRacilyClean(t *testing.T) {
	mtime := time.Unix(1000, 0)
	for _, c := range []struct {
		hashedAfter time.Duration
		seen        bool
	}{{time.Second, true}, {3 * time.Second, false}} {
		fsys := &clockFS{MapFS: fstest.MapFS{"f": {Data: []byte("a"), ModTime: mtime}}}
		s := newSnapFS(fsys, nil, func() time.Time { return mtime.Add(c.hashedAfter) })
		if _, err := s.ReadFile("/f"); err != nil {
			t.Fatal(err)
		}
		fsys.MapFS["f"] = &fstest.MapFile{Data: []byte("b"), ModTime: mtime}
		next, changed, err := s.refresh(context.Background())
		if err != nil || (next != nil) != c.seen || (len(changed) == 1) != c.seen {
			t.Errorf("hashed %v after: changed %v, want seen %v", c.hashedAfter, changed, c.seen)
		}
	}
}

// linkFS resolves the links a test sets.
type linkFS struct {
	clockFS
	links map[string]string
	calls int
}

func (l *linkFS) EvalSymlinks(name string) (string, error) {
	l.calls++
	if real, ok := l.links[name]; ok {
		return real, nil
	}
	return name, nil
}

// API.md S1, WIRE.md §6.5: a link is resolved once per snapshot; a retargeted one is a change.
func TestRefreshLinks(t *testing.T) {
	fsys := &linkFS{clockFS: clockFS{MapFS: fstest.MapFS{}}, links: map[string]string{"/l": "/a"}}
	s := newSnapFS(fsys, nil, time.Now)
	for range 2 {
		if real, err := s.EvalSymlinks("/l"); err != nil || real != "/a" || fsys.calls != 1 {
			t.Fatalf("EvalSymlinks = %s, %v after %d resolutions", real, err, fsys.calls)
		}
	}
	if next, _, err := s.refresh(context.Background()); next != nil || err != nil {
		t.Errorf("an unchanged link is a change: %v", err)
	}
	fsys.links["/l"] = "/b"
	next, changed, err := s.refresh(context.Background())
	if err != nil || next == nil || len(changed) != 1 {
		t.Fatalf("a retargeted link: %v, %v", changed, err)
	}
	if real, _ := next.EvalSymlinks("/l"); real != "/b" {
		t.Errorf("the next snapshot resolves /l to %s", real)
	}
	if real, _ := s.EvalSymlinks("/l"); real != "/a" {
		t.Errorf("the old snapshot resolves /l to %s", real)
	}
}

// API.md S4, S5: a name a snapshot dropped is not carried into its record, a name changed then
// reverted is judged by the newest record of the base, and neither depends on where the last
// full copy sits.
func TestHistoryRemovalAndRevert(t *testing.T) {
	n := name{kind: kindFile, abs: "/f"}
	key := func(text string) sum { return sum{hash: sha256.Sum256([]byte(text))} }
	for pad := range fullEvery {
		var h history
		s := newSnapFS(nil, nil, time.Now)
		for i := range pad { // moves the full copies against the records below
			put(s, name{kind: kindFile, abs: fmt.Sprint("/pad", i)}, &entry{})
			h.add(fmt.Sprint("pad", i), s)
		}
		set := func(rev string, e *entry) {
			put(s, n, e)
			h.add(rev, s)
		}
		set("A", &entry{sum: key("a")})
		set("dropped", nil)
		set("B", &entry{sum: key("b")})
		set("A", &entry{sum: key("a")})
		dropped, _ := h.find("dropped")
		if v, ok := h.at(dropped, n); !ok || v != key("b") {
			t.Errorf("pad %d: at the drop /f is %v, %v, want what B first read", pad, v, ok)
		}
		if _, ok := h.held(dropped, n); ok {
			t.Errorf("pad %d: the record of the drop holds /f", pad)
		}
		a, _ := h.find("A")
		if _, stale := h.changed(a, []probe{{n: n, now: key("a")}}); stale {
			t.Errorf("pad %d: a revert to A is stale against A", pad)
		}
		if whole := h.whole(len(h.recs) - 2); whole[n] != key("b") {
			t.Errorf("pad %d: whole(B) = %v", pad, whole[n])
		}
		if whole := h.whole(dropped); len(whole) != pad {
			t.Errorf("pad %d: whole(dropped) holds %d names", pad, len(whole))
		}
	}
}

// API.md S4: records are deltas with a full copy every 16; past 64 the oldest go, the next made
// whole, and every remembered record still gives each name the key it had.
func TestHistoryDeltas(t *testing.T) {
	var h history
	s := newSnapFS(nil, nil, time.Now)
	for i := range 100 {
		put(s, name{kind: kindFile, abs: fmt.Sprint("/f", i%7)}, &entry{sum: sum{hash: sha256.Sum256([]byte(fmt.Sprint(i)))}})
		h.add(fmt.Sprint("r", i), s)
	}
	if len(h.recs) != historyLen || !h.recs[0].full {
		t.Fatalf("%d records, oldest full %v", len(h.recs), h.recs[0].full)
	}
	for i, r := range h.recs {
		if r.full != (r.seq%fullEvery == 0 || i == 0) {
			t.Errorf("record %d (seq %d) full %v", i, r.seq, r.full)
		}
		n := r.seq - 1 // the revision's own write, "r<n>", set /f<n mod 7> to n
		if v, ok := h.at(i, name{kind: kindFile, abs: fmt.Sprint("/f", n%7)}); !ok || v.hash != sha256.Sum256([]byte(fmt.Sprint(n))) {
			t.Errorf("record %d: /f%d = %v, %v", i, n%7, v, ok)
		}
	}
	if _, ok := h.find("r35"); ok {
		t.Error("revision r35 of 100 is remembered")
	}
	if i, ok := h.find("r36"); !ok || i != 0 {
		t.Errorf("r36 at %d, %v", i, ok)
	}
}

// put stores e under n in s, or drops n for nil, as a snapshot's reads, forks and settle do: in
// its log, which the history reads (log-2026-09-29 M4 P18).
func put(s *snapFS, n name, e *entry) {
	if e == nil {
		delete(s.ents, n)
	} else {
		s.ents[n] = e
	}
	s.log = append(s.log, n)
	s.gen++
}
