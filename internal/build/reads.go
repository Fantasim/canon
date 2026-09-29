package build

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// Read is one input a run consulted: a file, or with Dir a directory listing, by its display
// path and by the name its file system was given (API.md S3, S5).
type Read struct {
	Display string
	Abs     string
	Dir     bool
	Sources bool // with Dir: a package's directory, whose listing counts only its .canon sources
	Link    bool // the path Abs's symbolic links resolve to, not its content
}

// ReadRecorder is a file system that wants the files each load reads, with their display
// paths, for its revision (API.md S3); a run tells the file system it was opened on.
type ReadRecorder interface {
	RecordReads(reads []Read)
}

// Listed is one line of a read-set listing: a file's display path and SHA-256, or Unreadable.
type Listed struct {
	Display    string
	Sum        [sha256.Size]byte
	Unreadable bool
}

// RevisionOf is the revision of a read-set listing, "r1:" and its SHA-256 (API.md S3).
func RevisionOf(lines []Listed) string {
	ds := make([]digest, len(lines))
	for i, l := range lines {
		ds[i] = digest{path: l.Display, text: unreadMark}
		if !l.Unreadable {
			ds[i].text = hex.EncodeToString(l.Sum[:])
		}
	}
	return revisionOf(ds)
}

// Over is p reading and writing through fsys, its directory and options kept: the project as a
// workspace snapshot sees it (API.md S1).
func (p *Project) Over(fsys project.FS) *Project {
	return &Project{fs: fsys, dir: p.dir, opt: p.opt}
}

// FS is the file system p was opened on.
func (p *Project) FS() project.FS { return p.fs }

// Dir is the project directory, absolute and '/'-separated.
func (p *Project) Dir() string { return p.dir }

// Inputs is the read set every call has (API.md S3): project.canon, each source the scan lists
// and every place a canon.lock can be; the caller drops those that do not exist. A listing that
// fails ends the list with its error.
func (p *Project) Inputs() ([]Read, error) {
	out := []Read{p.fileRead(project.FileName)}
	names, err := project.Scan(p.fs, p.dir)
	if err != nil {
		return out, displayErrorIn(p.dir, err)
	}
	for _, name := range names {
		out = append(out, p.fileRead(name))
	}
	for _, name := range lockPaths(names) {
		out = append(out, p.fileRead(name))
	}
	return out, nil
}

// fileRead is the project-relative file name as a Read.
func (p *Project) fileRead(name string) Read {
	return Read{Display: name, Abs: path.Join(p.dir, name)}
}

// Abs is file's name on disk, an absolute path or a display path resolved, false for no place.
func (p *Project) Abs(file string) (string, bool) {
	if path.IsAbs(file) || filepath.IsAbs(filepath.FromSlash(file)) {
		return path.Clean(filepath.ToSlash(file)), true
	}
	if !strings.HasPrefix(file, rootMark) {
		rel := path.Clean(file)
		return path.Join(p.dir, rel), filepath.IsLocal(filepath.FromSlash(rel))
	}
	s, ok := p.roots()
	if !ok {
		return "", false
	}
	resolved, ok := s.layout.Resolve(file, "", source.Span{}, s.own)
	return resolved.Abs, ok
}

// Display is abs's display path: project-relative under the project directory, else @root/rest
// under the root that holds it most closely, else its last element.
func (p *Project) Display(abs string) string {
	if rel, ok := under(p.dir, abs); ok {
		return rel
	}
	best, bestDir := path.Base(abs), ""
	s, ok := p.roots()
	if !ok {
		return best
	}
	for _, r := range s.proj.Roots {
		root, ok := s.layout.Resolve(rootMark+r.Name, "", source.Span{}, s.own)
		rel, in := under(root.Abs, abs)
		if !ok || !in || len(root.Abs) <= len(bestDir) {
			continue
		}
		best, bestDir = rootMark+r.Name, root.Abs
		if rel != listingMark {
			best += pathSep + rel
		}
	}
	return best
}

// roots is project.canon read and its roots placed, false when it does not check.
func (p *Project) roots() (*snapshot, bool) {
	set := &source.FileSet{}
	s := &snapshot{p: p, set: set, own: diag.NewBag(set, "")}
	return s, p.readProject(s) == nil
}

// under is name relative to dir, "." for dir itself; false outside it.
func under(dir, name string) (string, bool) {
	if name == dir {
		return listingMark, true
	}
	rel, ok := strings.CutPrefix(name, strings.TrimSuffix(dir, pathSep)+pathSep)
	return rel, ok
}

// Reads is the read set of pkg (API.md S5): project.canon, the files and possible locks of pkg,
// of every package it imports and of the studio when one of them needs it, and every file or
// listing their loads and verifications consulted, in display order.
func (a *Analysis) Reads(pkg string) []Read {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.r
	i := slices.IndexFunc(r.s.units, func(u *project.Unit) bool { return u.Name == pkg })
	if i < 0 {
		return nil
	}
	units := withStudio(r.s.units, imported(r.s.units, r.s.units[i:i+1]), r.s.proj.Studio.Path)
	out := []Read{r.p.fileRead(project.FileName)}
	names := []string{""} // what no package asked for counts for every one
	for _, u := range units {
		names = append(names, u.Name)
		out = append(out, r.p.unitReads(u)...)
	}
	if l := r.host.log(); l != nil {
		out = append(out, l.reads(names, displays(r.s.set, 0))...)
	}
	slices.SortFunc(out, compareReads)
	return slices.Compact(out)
}

// unitReads is u's files, the sources listed in each directory holding one, so that a source
// added there after a base is a change (API.md S5), and every place its canon.lock can be.
func (p *Project) unitReads(u *project.Unit) []Read {
	var out []Read
	var files []string
	for _, f := range u.Files {
		dir := path.Dir(f.Src.Path)
		out = append(out, Read{Display: f.Src.Path, Abs: f.Src.Abs}, Read{Display: dir, Abs: path.Join(p.dir, dir), Dir: true, Sources: true})
		files = append(files, f.Src.Path)
	}
	for _, name := range lockPaths(files) {
		out = append(out, p.fileRead(name))
	}
	return out
}

func compareReads(a, b Read) int {
	return cmp.Or(cmp.Compare(a.Display, b.Display), cmp.Compare(a.Abs, b.Abs), boolOrder(a.Dir, b.Dir),
		boolOrder(a.Sources, b.Sources), boolOrder(a.Link, b.Link))
}

func boolOrder(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	}
	return 1
}

// displays is the display path of every file set's file added after id from, by absolute
// name, the first one kept: a file keeps its first reference's form.
func displays(set *source.FileSet, from source.FileID) map[string]string {
	out := map[string]string{}
	for id := from + 1; set.File(id) != nil; id++ {
		f := set.File(id)
		if _, seen := out[f.Abs]; !seen {
			out[f.Abs] = f.Path
		}
	}
	return out
}

// touch is one name a run's loads or asset checks consulted: a file read or stat'd, a
// directory listed, or a path whose links were resolved.
type touch struct {
	abs  string
	dir  bool
	link bool
}

// readLog is a run's file system as its loads and asset checks see it: every name they read,
// stat or list is kept under the package whose load or verification asked (API.md S5).
type readLog struct {
	fs      project.FS
	mu      sync.Mutex
	pkg     string
	by      map[string]map[touch]bool
	pending []string // files read since they were last told to the file system (API.md S3)
	seen    source.FileID
	dir     string // the project directory, which displays a file the file set lacks
}

func (l *readLog) ReadFile(name string) ([]byte, error) {
	l.note(touch{abs: name}, true)
	return l.fs.ReadFile(name)
}

func (l *readLog) Stat(name string) (fs.FileInfo, error) {
	l.note(touch{abs: name}, false)
	return l.fs.Stat(name)
}

func (l *readLog) ReadDir(name string) ([]fs.DirEntry, error) {
	l.note(touch{abs: name, dir: true}, false)
	return l.fs.ReadDir(name)
}

// EvalSymlinks resolves links as the file system under the log does, charged like a read, so a
// retargeted link is a change (API.md S5).
func (l *readLog) EvalSymlinks(name string) (string, error) {
	l.note(touch{abs: name, link: true}, false)
	return project.EvalSymlinks(l.fs, name)
}

func (l *readLog) note(t touch, read bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.by[l.pkg] == nil {
		l.by[l.pkg] = map[touch]bool{}
	}
	l.by[l.pkg][t] = true
	if read {
		l.pending = append(l.pending, t.abs)
	}
}

// enter charges what is read next to pkg and returns whom it charged before.
func (l *readLog) enter(pkg string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	prev := l.pkg
	l.pkg = pkg
	return prev
}

// flush tells the file system under the log, if it records reads, each file read since the
// last flush, by the display path the file set gives it or else by display (API.md S3).
func (l *readLog) flush(set *source.FileSet) {
	l.mu.Lock()
	pending, from := l.pending, l.seen
	l.pending = nil
	for set.File(l.seen+1) != nil {
		l.seen++
	}
	l.mu.Unlock()
	rec, ok := l.fs.(ReadRecorder)
	if !ok || len(pending) == 0 {
		return
	}
	known := displays(set, from)
	out := make([]Read, 0, len(pending))
	for _, abs := range pending {
		out = append(out, Read{Display: l.displayOf(known, abs), Abs: abs})
	}
	rec.RecordReads(out)
}

// reads is every name the packages names consulted.
func (l *readLog) reads(names []string, known map[string]string) []Read {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Read
	for _, name := range names {
		//canon:unordered the caller sorts the reads
		for t := range l.by[name] {
			out = append(out, Read{Display: l.displayOf(known, t.abs), Abs: t.abs, Dir: t.dir, Link: t.link})
		}
	}
	return out
}

// displayOf is abs as the file set displays it, else relative to the project directory.
func (l *readLog) displayOf(known map[string]string, abs string) string {
	if d, ok := known[abs]; ok {
		return d
	}
	return relativeTo(l.dir, abs, filepath.Separator)
}

// track points the run's loads and asset checks at its read log, made on first use, and
// charges what they consult to pkg until the returned function runs (API.md S3, S5).
func (h *evalHost) track(pkg string) func() {
	if h.loader == nil {
		return func() {}
	}
	l := h.log()
	if l == nil {
		l = &readLog{fs: h.loader.FS, by: map[string]map[touch]bool{}}
		if h.loader.Layout != nil {
			l.dir = h.loader.Layout.Dir
		}
		h.loader.FS = l
		h.loader.Reused = func(abs string) { l.note(touch{abs: abs}, false) } // a cached header counts too (S5)
		if h.assets != nil {
			h.assets.fs = l
		}
	}
	prev := l.enter(pkg)
	return func() {
		l.enter(prev)
		l.flush(h.loader.Set)
	}
}

// log is the read log over the loader's file system, nil before the first load or verification.
func (h *evalHost) log() *readLog {
	if h == nil || h.loader == nil {
		return nil
	}
	l, _ := h.loader.FS.(*readLog)
	return l
}
