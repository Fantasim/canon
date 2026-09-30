package workspace

import (
	"cmp"
	"crypto/sha256"
	"errors"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// kind is what an entry of a snapshot holds.
type kind uint8

// class is what reading an entry gave.
type class uint8

// name keys an entry: its kind and the absolute name its file system was given.
type name struct {
	kind kind
	abs  string
}

// sum is an entry's content key: what it held, and the SHA-256 of it (API.md S3, S5).
type sum struct {
	class class
	hash  [sha256.Size]byte
}

// stamp is what a stat says of an entry's name, compared at each refresh (API.md S1).
type stamp struct {
	class class
	size  int64
	mtime int64
	zero  bool // no modification time: the entry is read again at every refresh
}

// entry is one name a snapshot has read, immutable once stored: a file's bytes, a directory's
// sorted listing or a stat, what reading it returned, and when (API.md S1).
type entry struct {
	sum   sum
	src   sum // a listing's key over its sources alone (API.md S5)
	data  []byte
	list  []fs.DirEntry
	info  fs.FileInfo
	err   error
	stamp stamp
	at    time.Time
	over  bool // an overlay's: never compared with the disk
}

// snapFS is one snapshot's file system: each name read from the one under it once, on first
// use, overlays in place of files; writes go through (API.md S1).
type snapFS struct {
	base   project.FS
	over   map[string][]byte // overlays by absolute name, never changed once the snapFS is read by a call
	mine   map[string]bool   // the overlays an edit planned in memory, not a user's (settle)
	now    func() time.Time
	root   string // the project directory, which names an entry as the scan does
	mu     sync.Mutex
	ents   map[name]*entry
	inputs map[string]string // every file a load read, by absolute name, to its display path (S3)
	gen    int               // bumped at each new entry, so an unchanged snapshot is not recorded twice
	ingen  int               // bumped at each change to inputs
	rev    string            // the revision last computed, while ingen is revAt
	revAt  int
	older  []weak.Pointer[snapFS] // the snapshots this one descends from, while a caller holds them

	plansMu sync.Mutex             // plans grows after the snapFS exists; mu may be held around it
	plans   []weak.Pointer[snapFS] // this snapshot with an edit applied in memory, while held
}

func newSnapFS(base project.FS, over map[string][]byte, now func() time.Time) *snapFS {
	return &snapFS{base: base, over: over, now: now, ents: map[name]*entry{}, inputs: map[string]string{}}
}

// fork is a new snapshot's file system: s's entries and inputs kept, except those replaced.
func (s *snapFS) fork(over map[string][]byte, replaced map[name]*entry) *snapFS {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := &snapFS{base: s.base, over: over, now: s.now, root: s.root, ents: maps.Clone(s.ents), inputs: maps.Clone(s.inputs)}
	for _, o := range s.lineage() {
		next.older = append(next.older, weak.Make(o))
	}
	//canon:unordered each replaced entry is stored under its own name
	for n, e := range replaced {
		if e == nil {
			delete(next.ents, n)
		} else {
			next.ents[n] = e
		}
	}
	return next
}

// get is n's entry, read by load on first use; two first uses keep the first stored.
func (s *snapFS) get(n name, load func(string) *entry) *entry {
	s.mu.Lock()
	e, ok := s.ents[n]
	s.mu.Unlock()
	if ok {
		return e
	}
	fresh := load(n.abs)
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.ents[n]; ok {
		return e
	}
	s.ents[n] = fresh
	s.gen++
	return fresh
}

func (s *snapFS) ReadFile(abs string) ([]byte, error) {
	e := s.get(name{kind: kindFile, abs: abs}, s.readFile)
	return slices.Clone(e.data), e.err
}

func (s *snapFS) Stat(abs string) (fs.FileInfo, error) {
	e := s.get(name{kind: kindStat, abs: abs}, s.stat)
	return e.info, e.err
}

// ReadDir is abs's listing with the overlays directly in it, sorted by name (API.md §2.2).
func (s *snapFS) ReadDir(abs string) ([]fs.DirEntry, error) {
	e := s.get(name{kind: kindDir, abs: abs}, s.readDir)
	return slices.Clone(e.list), e.err
}

// EvalSymlinks resolves links as the file system under the snapshot does, once (WIRE.md §6.5).
func (s *snapFS) EvalSymlinks(abs string) (string, error) {
	e := s.get(name{kind: kindLink, abs: abs}, s.resolve)
	return string(e.data), e.err
}

// resolve is abs's real path now; a refresh resolves it again, so a retargeted link is a change.
func (s *snapFS) resolve(abs string) *entry {
	real, err := project.EvalSymlinks(s.base, abs)
	e := &entry{data: []byte(real), err: err, sum: sum{class: classOf(err)}}
	if err == nil {
		e.sum.hash = sha256.Sum256(e.data)
	}
	return e
}

// lineage is s, then every snapshot it descends from and every edit planned in memory on one of
// them (API.md E18, V13) that is still in use, which a writer pins, each once.
func (s *snapFS) lineage() []*snapFS {
	out := []*snapFS{s}
	for _, w := range s.older {
		if o := w.Value(); o != nil {
			out = append(out, o)
		}
	}
	seen := map[*snapFS]bool{}
	for _, o := range out {
		seen[o] = true
	}
	for _, o := range slices.Clone(out) {
		for _, p := range o.planned() {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// planned is every edit planned in memory on s still in use.
func (s *snapFS) planned() []*snapFS {
	s.plansMu.Lock()
	defer s.plansMu.Unlock()
	var out []*snapFS
	for _, w := range s.plans {
		if p := w.Value(); p != nil {
			out = append(out, p)
		}
	}
	return out
}

// plan records next, s with an edit applied in memory, as read while a writer writes (S9); the
// plans no longer in use are forgotten.
func (s *snapFS) plan(next *snapFS) {
	s.plansMu.Lock()
	defer s.plansMu.Unlock()
	live := slices.DeleteFunc(s.plans, func(w weak.Pointer[snapFS]) bool { return w.Value() == nil })
	s.plans = append(live, weak.Make(next))
}

// readFile reads abs now: stat first, so a change during the read shows at the next refresh.
func (s *snapFS) readFile(abs string) *entry {
	if data, ok := s.over[abs]; ok {
		if data == nil {
			return gone(abs)
		}
		return &entry{sum: sum{class: classOK, hash: sha256.Sum256(data)}, data: data, over: true}
	}
	at := s.now()
	st := stampOf(s.base.Stat(abs))
	data, err := s.base.ReadFile(abs)
	e := &entry{data: data, err: err, stamp: st, at: at, sum: sum{class: classOf(err)}}
	if err == nil {
		e.sum.hash = sha256.Sum256(data)
	}
	return e
}

func (s *snapFS) stat(abs string) *entry {
	if data, ok := s.over[abs]; ok && data == nil {
		return gone(abs)
	}
	if info, ok := s.overlayStat(abs); ok {
		return &entry{info: info, sum: statSum(info, nil), over: true}
	}
	info, err := s.base.Stat(abs)
	return &entry{info: info, err: err, sum: statSum(info, err)}
}

// readDir lists abs now, the overlays directly in it added, sorted by name.
func (s *snapFS) readDir(abs string) *entry {
	at := s.now()
	st := stampOf(s.base.Stat(abs))
	list, err := s.base.ReadDir(abs)
	if errors.Is(err, fs.ErrNotExist) && s.holdsOverlay(abs) {
		list, err = nil, nil
	}
	if err == nil {
		list = s.withOverlays(abs, list)
	}
	slices.SortFunc(list, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	return &entry{list: list, err: err, stamp: st, at: at, sum: listSum(list, err, nil), src: listSum(list, err, s.isSource(abs))}
}

// RecordReads keeps the files a load read, each by its least display path, whatever the order
// of the runs that read it (build.ReadRecorder; log-2026-09-29 M4 U8-r).
func (s *snapFS) RecordReads(reads []build.Read) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range reads {
		if d, ok := s.inputs[r.Abs]; !ok || r.Display < d {
			s.inputs[r.Abs] = r.Display
			s.ingen++
		}
	}
}

// recorded is every file a load read, by absolute name.
func (s *snapFS) recorded() []build.Read {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]build.Read, 0, len(s.inputs))
	for _, abs := range slices.Sorted(maps.Keys(s.inputs)) {
		out = append(out, build.Read{Display: s.inputs[abs], Abs: abs})
	}
	return out
}

// sums is the content key of every entry, for the history (API.md S4).
func (s *snapFS) sums() (map[name]sum, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[name]sum, len(s.ents))
	//canon:unordered each key is copied under its own name
	for n, e := range s.ents {
		out[n] = e.sum
		if n.kind == kindDir {
			out[name{kind: kindSources, abs: n.abs}] = e.src
		}
	}
	return out, s.gen
}

func classOf(err error) class {
	switch {
	case err == nil:
		return classOK
	case errors.Is(err, fs.ErrNotExist):
		return classMissing
	}
	return classUnreadable
}

// modTime is the modification time the stamp holds.
func (st stamp) modTime() time.Time { return time.Unix(0, st.mtime) }

func stampOf(info fs.FileInfo, err error) stamp {
	if err != nil {
		return stamp{class: classOf(err)}
	}
	return stamp{size: info.Size(), mtime: info.ModTime().UnixNano(), zero: info.ModTime().IsZero()}
}

// statSum is a stat's content key: whether the name exists, and as a directory or a regular file.
func statSum(info fs.FileInfo, err error) sum {
	out := sum{class: classOf(err)}
	if err != nil {
		return out
	}
	if info.IsDir() {
		out.hash[0] |= statDir
	}
	if info.Mode().IsRegular() {
		out.hash[0] |= statRegular
	}
	return out
}

// listSum is a listing's content key: the SHA-256 of the names and kinds keep keeps, all when
// keep is nil, in name order.
func listSum(list []fs.DirEntry, err error, keep func(fs.DirEntry) bool) sum {
	out := sum{class: classOf(err)}
	if err != nil {
		return out
	}
	var buf []byte
	for _, e := range list {
		if keep != nil && !keep(e) {
			continue
		}
		flags := byte(0)
		if e.IsDir() {
			flags |= statDir
		}
		if e.Type().IsRegular() {
			flags |= statRegular
		}
		buf = append(append(buf, e.Name()...), flags, 0)
	}
	out.hash = sha256.Sum256(buf)
	return out
}

// isSource reports an entry of directory dir that the project scan reads as a source, by the
// scan's own predicate, so a package's sources key and the scan never disagree (API.md O2, S5).
func (s *snapFS) isSource(dir string) func(fs.DirEntry) bool {
	rel := ""
	if dir != s.root {
		rel = strings.TrimPrefix(dir, strings.TrimSuffix(s.root, pathSep)+pathSep)
	}
	return func(e fs.DirEntry) bool { return project.IsSource(path.Join(rel, e.Name()), e) }
}

// sources is the absolute name of every source a listing of dir holds.
func (s *snapFS) sources(dir string, list []fs.DirEntry) []string {
	keep := s.isSource(dir)
	var out []string
	for _, e := range list {
		if keep(e) {
			out = append(out, project.Join(dir, e.Name()))
		}
	}
	return out
}
