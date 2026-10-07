package build

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Static is the static read sets of a parsed snapshot, each package's and every file a load names,
// found from the parse alone: no package is checked or evaluated for it (API.md S3, S5, E17).
type Static struct {
	Units []*project.Unit // every package of the snapshot, in name order
	units map[string]*unitStatic
	loads []Read // every file a load names, once by real path, by its least display, in display order
	fs    project.FS
}

// unitStatic is one package's static read set and the asset roots its sources declare.
type unitStatic struct {
	reads  []Read
	assets []string // each asset root's directory, by the name its file system is given
}

// Static is the static read sets of the project's files as they are now (API.md S3, S5): the
// sources scanned and parsed, the loads' paths resolved and their globs walked.
func (p *Project) Static(ctx context.Context) (*Static, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	return staticOf(s), nil
}

// static is the static read sets of the snapshot a analyzed, made on first use (API.md S5).
func (a *Analysis) static() *Static {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.statics == nil {
		a.statics = staticOf(a.r.s)
	}
	return a.statics
}

// staticOf is s's static read sets; every package's, in name order.
func staticOf(s *snapshot) *Static {
	st := &Static{Units: s.units, units: make(map[string]*unitStatic, len(s.units)), fs: s.p.fs}
	real := newRealPaths(s.p.fs)
	least := map[string]Read{} // each loaded file by its real path, under its least display
	var files []*syntax.File
	for _, u := range s.units {
		us, loaded := s.unitStatic(u)
		st.units[u.Name] = us
		for _, f := range loaded {
			key := real.file(f.Abs)
			if was, ok := least[key]; !ok || f.Display < was.Display {
				least[key] = Read{Display: f.Display, Abs: f.Abs}
			}
		}
		files = append(files, u.Files...)
	}
	s.gen.keepLoads(files)
	//canon:unordered the loads are sorted next
	for _, rd := range least {
		st.loads = append(st.loads, rd)
	}
	slices.SortFunc(st.loads, compareReads)
	return st
}

// unitStatic is u's static read set (API.md S5): its files and its directories' listings of
// sources, its own canon.lock, what the loads of its source and layer files name; with its asset
// roots, and every file its loads name.
func (s *snapshot) unitStatic(u *project.Unit) (*unitStatic, []load.File) {
	us := &unitStatic{}
	var loaded []load.File
	for _, f := range u.Files {
		dir := path.Dir(f.Src.Path)
		us.reads = append(us.reads, Read{Display: f.Src.Path, Abs: f.Src.Abs}, Read{Display: dir, Abs: project.Join(s.p.dir, dir), Dir: true, Sources: true})
		for _, at := range s.gen.loadsOf(f) {
			named, _ := load.Names(s.p.fs, s.layout, dir, at.e) // a path that is no literal is E7008, naming nothing
			loaded = append(loaded, named.Files...)
			us.reads = append(us.reads, s.namedReads(named)...)
		}
		us.assets = append(us.assets, s.assetRoots(f)...)
	}
	us.reads = append(us.reads, s.p.fileRead(lockOf(u.Name)))
	slices.SortFunc(us.reads, compareReads)
	us.reads = slices.Compact(us.reads)
	return us, loaded
}

// namedReads is what a load names as reads: its files, the directories its walk lists, the names
// whose links it resolves, each displayed relative to the project directory but a file's.
func (s *snapshot) namedReads(n load.Named) []Read {
	out := make([]Read, 0, len(n.Files)+len(n.Dirs)+len(n.Links))
	for _, f := range n.Files {
		out = append(out, Read{Display: f.Display, Abs: f.Abs})
	}
	for _, d := range n.Dirs {
		out = append(out, Read{Display: s.shown(d), Abs: d, Dir: true})
	}
	for _, l := range n.Links {
		out = append(out, Read{Display: s.shown(l), Abs: l, Link: true})
	}
	return out
}

// shown is abs relative to the project directory, else its last element: never an absolute path.
func (s *snapshot) shown(abs string) string {
	if rel, ok := under(s.p.dir, abs); ok {
		return rel
	}
	return path.Base(abs)
}

// assetRoots is the directory of each asset root f's types write, placed as an asset check
// places it; a root that does not resolve is none (TYPES.md 13.4).
func (s *snapshot) assetRoots(f *syntax.File) []string {
	var out []string
	syntax.Inspect(f, func(n syntax.Node) bool {
		a, ok := n.(*syntax.AssetType)
		if !ok {
			return true
		}
		if root, ok := check.LiteralText(a.Dir); ok {
			if p, ok := s.layout.Resolve(root, path.Dir(f.Src.Path), source.Span{}, diag.NewBag(nil, "")); ok {
				out = append(out, p.Abs)
			}
		}
		return true
	})
	return out
}

// lockOf is the project-relative path of package pkg's canon.lock (LOCK.md §2.1).
func lockOf(pkg string) string {
	return path.Join(packageRel(pkg), lockName)
}

// Of is pkg's static read set (API.md S5), none for a package the snapshot lacks.
func (st *Static) Of(pkg string) []Read {
	if us, ok := st.units[pkg]; ok {
		return slices.Clone(us.reads)
	}
	return nil
}

// Loads is every file a load of any package names, each once by real path under the least
// display the loads give it, in display order (API.md S3).
func (st *Static) Loads() []Read {
	return slices.Clone(st.loads)
}

// Readers are the packages whose static read set holds one of files or lists one of dirs, or that
// declare an asset root holding one of names, a name created or removed, all by real paths, sorted;
// with the other names files are read by, each of names by its real path (API.md E17, DECISIONS 330).
func (st *Static) Readers(files, dirs, names []string) (pkgs []string, aliases map[string]string) {
	real := newRealPaths(st.fs)
	wantFiles, wantDirs := map[string]string{}, map[string]bool{}
	for _, f := range files {
		wantFiles[real.file(f)] = f
	}
	for _, d := range dirs {
		wantDirs[real.dir(d)] = true
	}
	aliases = map[string]string{}
	hit := func(rd Read) bool {
		if rd.Dir {
			return wantDirs[real.dir(rd.Abs)]
		}
		f, ok := wantFiles[real.file(rd.Abs)]
		if ok && f != rd.Abs && !rd.Link {
			aliases[rd.Abs] = f
		}
		return ok
	}
	held := realNames(real, names, aliases)
	for _, u := range st.Units {
		us := st.units[u.Name]
		reads := false
		for _, rd := range us.reads { // every read is asked, for its aliases
			reads = hit(rd) || reads
		}
		if reads || holdsAny(real, us.assets, held) {
			pkgs = append(pkgs, u.Name)
		}
	}
	slices.Sort(pkgs)
	return pkgs, aliases
}

// realName is a name an edit creates or removes, and its real path.
type realName struct {
	name, real string
}

// realNames is each of names with its real path, noted in aliases when it differs: a walk or an
// asset check lists real paths, and reads the name by its own.
func realNames(real *realPaths, names []string, aliases map[string]string) []realName {
	out := make([]realName, len(names))
	for i, n := range names {
		out[i] = realName{name: n, real: real.file(n)}
		if out[i].real != n {
			aliases[out[i].real] = n
		}
	}
	return out
}

// holdsAny reports one of names below one of roots, both by real path.
func holdsAny(real *realPaths, roots []string, names []realName) bool {
	for _, root := range roots {
		dir := strings.TrimSuffix(real.dir(root), pathSep) + pathSep
		if slices.ContainsFunc(names, func(n realName) bool { return strings.HasPrefix(n.real, dir) }) {
			return true
		}
	}
	return false
}
