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

// Listed is one line of a read-set listing: a file's display path and SHA-256, or Unreadable.
type Listed struct {
	Display    string
	Sum        [sha256.Size]byte
	Unreadable bool
}

// RevisionOf is the revision of a read-set listing, "r1:" and its SHA-256 (API.md S3). Lines
// already in strictly ascending display order are taken as they are: no other order sorts so.
func RevisionOf(lines []Listed) string {
	byDisplay := func(a, b Listed) int { return cmp.Compare(a.Display, b.Display) }
	if !strictlySorted(lines) {
		lines = slices.SortedFunc(slices.Values(lines), byDisplay) // as revisionOf sorts, ties included
	}
	return revisionOfSorted(lines, func(l Listed) string { return l.Display }, appendSum)
}

// appendSum is buf and l's text in a listing: its SHA-256 in hex, or unreadMark (API.md S3).
func appendSum(buf []byte, l Listed) []byte {
	if l.Unreadable {
		return append(buf, unreadMark...)
	}
	return hex.AppendEncode(buf, l.Sum[:])
}

// strictlySorted reports each line's display greater than the one before.
func strictlySorted(lines []Listed) bool {
	for i := 1; i < len(lines); i++ {
		if lines[i-1].Display >= lines[i].Display {
			return false
		}
	}
	return true
}

// Over is p reading and writing through fsys, its directory and options kept: the project as a
// workspace snapshot sees it (API.md S1).
func (p *Project) Over(fsys project.FS) *Project {
	return &Project{fs: fsys, dir: p.dir, opt: p.opt, cache: p.cache}
}

// FS is the file system p was opened on.
func (p *Project) FS() project.FS { return p.fs }

// Dir is the project directory, absolute and '/'-separated.
func (p *Project) Dir() string { return p.dir }

// Layered reports an active layer (Options.Layers).
func (p *Project) Layered() bool { return len(p.opt.Layers) > 0 }

// Inputs is the read set every call has (API.md S3): project.canon, the place of
// project.local.canon, each source the scan lists and every place a canon.lock can be; the
// caller drops those that do not exist. A listing that fails ends the list with its error.
func (p *Project) Inputs() ([]Read, error) {
	out := p.ownReads()
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

// ownReads is project.canon and the place of project.local.canon, which may not exist (API.md S3, S5).
func (p *Project) ownReads() []Read {
	return []Read{p.fileRead(project.FileName), p.fileRead(project.LocalFileName)}
}

// fileRead is the project-relative file name as a Read.
func (p *Project) fileRead(name string) Read {
	return Read{Display: name, Abs: project.Join(p.dir, name)}
}

// Abs is file's name on disk, an absolute path or a display path resolved, false for no place.
func (p *Project) Abs(file string) (string, bool) {
	return p.absIn(project.HostPaths(), file)
}

// absIn is Abs on a system that writes names as sys does (API.md §2.2, log M4 B12-r).
func (p *Project) absIn(sys project.Paths, file string) (string, bool) {
	file = sys.FromAPI(file, p.dir)
	if path.IsAbs(file[len(sys.Volume(file)):]) {
		return file, true
	}
	if !strings.HasPrefix(file, rootMark) {
		rel := path.Clean(file)
		return sys.Join(p.dir, rel), filepath.IsLocal(filepath.FromSlash(rel))
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

// Layout is the roots as this machine places them now: project.local.canon and the overrides
// applied, presence judged; false when the project does not open (API.md O2, W12).
func (p *Project) Layout() (*project.Layout, bool) {
	s, ok := p.roots()
	return s.layout, ok
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

// Reads is the read set of pkg (API.md S5): the project files, the static read sets of pkg, of every
// package it imports and of the studio when one of them needs it (DECISIONS 330), and every file
// or listing their loads and asset checks consulted in this analysis, in display order.
func (a *Analysis) Reads(pkg string) []Read {
	names := a.closure(pkg)
	if names == nil {
		return nil
	}
	st := a.static()
	out := a.r.p.ownReads()
	for _, name := range names {
		out = append(out, st.Of(name)...)
	}
	out = append(out, a.Consulted(pkg)...)
	slices.SortFunc(out, compareReads)
	return slices.Compact(out)
}

// Consulted is every file or listing the loads and asset checks of pkg, of every package it
// imports and of the studio when one of them needs it consulted in this analysis (API.md S5): a
// listing an asset check reads is known only once it ran. Unsorted.
func (a *Analysis) Consulted(pkg string) []Read {
	names := a.closure(pkg)
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.r.host.log()
	if names == nil || l == nil {
		return nil
	}
	return l.reads(append(names, "")) // what no package asked for counts for every one
}

// closure is pkg, every package it imports and the studio when one of them needs it, nil for a
// package the snapshot lacks.
func (a *Analysis) closure(pkg string) []string {
	units := a.r.s.units
	i := slices.IndexFunc(units, func(u *project.Unit) bool { return u.Name == pkg })
	if i < 0 {
		return nil
	}
	var out []string
	for _, u := range withStudio(units, imported(units, units[i:i+1]), a.r.s.proj.Studio.Path) {
		out = append(out, u.Name)
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
	fs    project.FS
	mu    sync.Mutex
	pkg   string
	by    map[string]map[touch]bool
	known map[string]string     // each file this run's loads added to the set, by its least display (added)
	dir   string                // the project directory, which displays a file the file set lacks
	kept  map[string]loadedFile // each file the loads handed the file set, by name (keep)
	site  siteKey               // the load reading now, the zero key for none
	globs []globMatch           // each load.dir's matches as used, in order (globbed)
}

func (l *readLog) ReadFile(name string) ([]byte, error) {
	l.note(touch{abs: name})
	return l.fs.ReadFile(name)
}

func (l *readLog) Stat(name string) (fs.FileInfo, error) {
	l.note(touch{abs: name})
	return l.fs.Stat(name)
}

func (l *readLog) ReadDir(name string) ([]fs.DirEntry, error) {
	l.note(touch{abs: name, dir: true})
	return l.fs.ReadDir(name)
}

// EvalSymlinks resolves links as the file system under the log does, charged like a read, so a
// retargeted link is a change (API.md S5).
func (l *readLog) EvalSymlinks(name string) (string, error) {
	l.note(touch{abs: name, link: true})
	return project.EvalSymlinks(l.fs, name)
}

// SumFile is the SHA-256 the file system under the log knows of name, charged as ReadFile charges
// it, so a load that takes the content unread reads as a cold one (project.SumFile, API.md S3, S5).
func (l *readLog) SumFile(name string) (sha256Sum, bool, error) {
	sum, ok, err := project.SumFile(l.fs, name)
	if ok {
		l.note(touch{abs: name})
	}
	return sum, ok, err
}

func (l *readLog) note(t touch) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.by[l.pkg] == nil {
		l.by[l.pkg] = map[touch]bool{}
	}
	l.by[l.pkg][t] = true
}

// added notes the display a load of this run gave a file it read, kept when least (S3).
func (l *readLog) added(src *source.File) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d, ok := l.known[src.Abs]; !ok || src.Path < d {
		l.known[src.Abs] = src.Path
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

// reads is every name the packages names consulted.
func (l *readLog) reads(names []string) []Read {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Read
	for _, name := range names {
		//canon:unordered the caller sorts the reads
		for t := range l.by[name] {
			out = append(out, Read{Display: l.displayOf(t.abs), Abs: t.abs, Dir: t.dir, Link: t.link})
		}
	}
	return out
}

// displayOf is abs as this run's loads displayed it, else relative to the project directory.
func (l *readLog) displayOf(abs string) string {
	if d, ok := l.known[abs]; ok {
		return d
	}
	return relativeTo(l.dir, abs, filepath.Separator)
}

// track points the run's loads and asset checks at its read log, made on first use, and
// charges what they consult to pkg until the returned function runs (API.md S5).
func (h *evalHost) track(pkg string) func() {
	if h.loader == nil {
		return func() {}
	}
	l := h.log()
	if l == nil {
		l = &readLog{
			fs: h.loader.FS, by: map[string]map[touch]bool{}, known: map[string]string{},
			kept: map[string]loadedFile{},
		}
		if h.loader.Layout != nil {
			l.dir = h.loader.Layout.Dir
		}
		h.loader.FS = l
		h.loader.Reused = func(abs string) { l.note(touch{abs: abs}) } // a cached header counts too (S5)
		h.loader.Add, h.loader.Kept = h.adder(l), h.keeper(l)
		h.loader.Globbed = l.globbed
		if h.assets != nil {
			h.assets.fs = l
		}
	}
	prev := l.enter(pkg)
	return func() { l.enter(prev) }
}

// log is the read log over the loader's file system, nil before the first load or verification.
func (h *evalHost) log() *readLog {
	if h == nil || h.loader == nil {
		return nil
	}
	l, _ := h.loader.FS.(*readLog)
	return l
}
