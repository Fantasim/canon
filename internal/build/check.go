package build

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Result is one check: the selected packages and their findings, the project's own too (API.md R2).
type Result struct {
	Packages []string
	Findings
	Revision string
}

// Units is the scan of a snapshot: every package of the project, in name order (API.md §5.5).
type Units struct {
	Units    []*project.Unit
	Revision string
}

// snapshot is what one call reads: every file under a new file set, and a bag per package.
type snapshot struct {
	p      *Project
	set    *source.FileSet
	own    *diag.Bag
	bags   map[string]*diag.Bag
	proj   *project.Project
	layout *project.Layout
	names  []string
	units  []*project.Unit
	sums   []project.FileSum
	locks  map[string][]byte // every package directory's canon.lock read, by project-relative path
	canon  [sha256.Size]byte // project.canon's SHA-256
	gen    *cacheGen         // the cache generation whose set this is; nil without a cache
	base   source.FileID     // the set's last file when the snapshot began
}

// add is a file of the snapshot's set: the cache's when it holds that content, else added.
func (s *snapshot) add(display, abs string, content []byte) (*source.File, error) {
	if s.gen == nil {
		return s.set.Add(display, abs, content)
	}
	return s.gen.file(display, abs, content)
}

// Packages scans and parses the project and lists its packages (API.md §5.5, O4).
func (p *Project) Packages(ctx context.Context) (*Units, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.checkLayers(s, s.units); err != nil {
		return nil, err
	}
	return &Units{Units: s.units, Revision: s.revision()}, nil
}

// Info is project.canon read and checked, as every call reads it (API.md §5.5): the failures Packages reports for it.
func (p *Project) Info() (*project.Project, error) {
	s := &snapshot{p: p, set: &source.FileSet{}, bags: map[string]*diag.Bag{}}
	s.own = diag.NewBag(s.set, "")
	if err := p.readOwn(s); err != nil {
		return nil, err
	}
	return s.proj, nil
}

// Revision is the revision of what is on disk now (API.md S1, S3), whether project.canon
// checks or not; a file, or the listing of the file set, that cannot be read is marked so. The
// canon.lock of every package directory counts, whatever a call selects.
func (p *Project) Revision(ctx context.Context) (string, error) {
	lines := []digest{p.digestOf(project.FileName)}
	names, err := project.Scan(p.fs, p.dir)
	if err != nil {
		lines = append(lines, digest{path: listingMark, text: unreadMark})
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		lines = append(lines, p.digestOf(name))
	}
	for _, name := range lockPaths(names) {
		data, err := p.fs.ReadFile(project.Join(p.dir, name))
		if !errors.Is(err, fs.ErrNotExist) {
			lines = append(lines, digestData(name, data, err))
		}
	}
	return revisionOf(lines), nil
}

// lockPaths is the canon.lock path of every directory holding a source file or above one (LOCK.md §2.1).
func lockPaths(names []string) []string {
	var out []string
	for _, name := range names {
		for dir := path.Dir(name); dir != listingMark; dir = path.Dir(dir) {
			out = append(out, path.Join(dir, lockName))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// readLocks reads the canon.lock of every package directory into the snapshot and its read set.
func (s *snapshot) readLocks(fsys project.FS, dir string) error {
	s.locks = map[string][]byte{}
	for _, name := range lockPaths(s.names) {
		data, err := fsys.ReadFile(project.Join(dir, name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return displayError(name, err)
		}
		s.locks[name] = data
		s.sums = append(s.sums, project.FileSum{Path: name, Sum: sha256.Sum256(data)})
	}
	return nil
}

// digest is one line of a read-set listing: a file's SHA-256 in hex, or unreadMark.
type digest struct {
	path, text string
}

func (p *Project) digestOf(name string) digest {
	data, err := p.fs.ReadFile(project.Join(p.dir, name))
	return digestData(name, data, err)
}

// digestData is the listing line of a file read, unreadMark when reading it failed.
func digestData(name string, data []byte, err error) digest {
	if err != nil {
		return digest{path: name, text: unreadMark}
	}
	sum := sha256.Sum256(data)
	return digest{path: name, text: hex.EncodeToString(sum[:])}
}

// Check runs phases 1 to 7 on the selected packages and their imports (CLI.md §3.3, API.md R2).
func (p *Project) Check(ctx context.Context, selectors []string) (*Result, error) {
	a, err := p.Analyze(ctx, selectors)
	if err != nil {
		return nil, err
	}
	return a.Result(), nil
}

// load reads a new snapshot: project.canon, then every source file, parsed.
func (p *Project) load(ctx context.Context) (*snapshot, error) {
	s, err := p.open()
	if err != nil {
		return nil, err
	}
	r := &project.Reader{FS: p.fs, Dir: p.dir, Set: s.set, BagOf: s.bag}
	if s.gen != nil {
		r.Reuse = s.gen.reuse
	}
	if s.units, err = r.Parse(ctx, s.names); err != nil {
		return nil, displayErrorIn(p.dir, err)
	}
	s.sums = append(s.sums, r.Sums...)
	if err := s.readLocks(p.fs, p.dir); err != nil {
		return nil, err
	}
	project.CheckStudio(s.proj, s.units, s.own)
	s.noteParsed()
	return s, nil
}

// noteParsed tells the cache how many bytes of its set this snapshot uses, which decides when
// it is compacted.
func (s *snapshot) noteParsed() {
	if s.gen == nil {
		return
	}
	n := 0
	for _, u := range s.units {
		for _, f := range u.Files {
			n += len(f.Src.Content)
		}
	}
	for _, data := range s.locks { //canon:unordered a sum
		n += len(data)
	}
	s.gen.parsed(s.base, n)
}

// bag is the bag of package pkg; "" is the project's own.
func (s *snapshot) bag(pkg string) *diag.Bag {
	if pkg == "" {
		return s.own
	}
	if s.bags[pkg] == nil {
		s.bags[pkg] = s.p.newBag(s.set, pkg)
	}
	return s.bags[pkg]
}

func (s *snapshot) bagsOf(units []*project.Unit) map[string]*diag.Bag {
	out := map[string]*diag.Bag{}
	for _, u := range units {
		out[u.Name] = s.bag(u.Name)
	}
	return out
}

func (s *snapshot) revision() string {
	lines := make([]digest, len(s.sums))
	for i, f := range s.sums {
		lines[i] = digest{path: f.Path, text: hex.EncodeToString(f.Sum[:])}
	}
	return revisionOf(lines)
}

// revisionOf is "r1:" and the SHA-256 of the listing of the files read (API.md S3).
func revisionOf(lines []digest) string {
	sorted := slices.SortedFunc(slices.Values(lines), func(a, b digest) int { return cmp.Compare(a.path, b.path) })
	return revisionOfSorted(sorted, func(l digest) string { return l.path }, func(buf []byte, l digest) []byte {
		return append(buf, l.text...)
	})
}

// revisionOfSorted is "r1:" and the SHA-256 of the listing of lines in their order: per line its
// display, NUL, the text text appends (a SHA-256 in hex or unreadMark) and LF (API.md S3).
func revisionOfSorted[L any](lines []L, display func(L) string, text func([]byte, L) []byte) string {
	size := 0
	for _, l := range lines {
		size += len(display(l)) + len(listingSep) + hex.EncodedLen(sha256.Size) + len(listingEnd)
	}
	listing := make([]byte, 0, size)
	for _, l := range lines {
		listing = append(text(append(append(listing, display(l)...), listingSep...), l), listingEnd...)
	}
	sum := sha256.Sum256(listing)
	return revisionPrefix + hex.EncodeToString(sum[:])
}

// imported is selected and every package they import, directly or not, in name order; an
// import of no package is check's E2003.
func imported(all, selected []*project.Unit) []*project.Unit {
	out := slices.Clone(selected)
	for i := 0; i < len(out); i++ {
		for _, name := range out[i].Imports {
			j := slices.IndexFunc(all, func(u *project.Unit) bool { return u.Name == name })
			if j >= 0 && !slices.Contains(out, all[j]) {
				out = append(out, all[j])
			}
		}
	}
	slices.SortFunc(out, func(a, b *project.Unit) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// withStudio is loaded plus project.studio's package and its own imports, added only when a
// loaded package needs it: a view, a `@menu` or an `emit view` (DECISIONS 227).
func withStudio(all, loaded []*project.Unit, studio string) []*project.Unit {
	if studio == "" || !slices.ContainsFunc(loaded, usesStudio) {
		return loaded
	}
	i := slices.IndexFunc(all, func(u *project.Unit) bool { return u.Name == studio })
	if i < 0 || slices.Contains(loaded, all[i]) {
		return loaded
	}
	return imported(all, append(slices.Clone(loaded), all[i]))
}

// usesStudio reports a unit with a view, a `@menu` or an `emit view` (DECISIONS 227).
func usesStudio(u *project.Unit) bool {
	for _, f := range u.Files {
		if f.FileKind != syntax.FileSource {
			continue
		}
		if slices.ContainsFunc(f.Decls, declUsesStudio) {
			return true
		}
	}
	return false
}

// declUsesStudio reports a view, an `emit view`, or a `let` with `@menu` (VIEWMODEL.md G16, G23).
func declUsesStudio(d syntax.Decl) bool {
	switch d := d.(type) {
	case *syntax.ViewDecl:
		return true
	case *syntax.EmitDecl:
		return isEmitView(d)
	case *syntax.LetDecl:
		return slices.ContainsFunc(d.Annotations, func(a *syntax.Annotation) bool {
			return a.Name != nil && a.Name.Name == syntax.AnnMenu
		})
	default:
		return false
	}
}

// hasEmitView reports files with an `emit view` (I18N.md W1, VIEWMODEL.md N4's scope).
func hasEmitView(files []*syntax.File) bool {
	for _, f := range files {
		for _, d := range f.Decls {
			if e, ok := d.(*syntax.EmitDecl); ok && isEmitView(e) {
				return true
			}
		}
	}
	return false
}

// isEmitView reports d as `emit view` (DECISIONS 227, I18N.md W1).
func isEmitView(d *syntax.EmitDecl) bool {
	return d.Target != nil && d.Target.Name == check.TargetView
}

func filesOf(units []*project.Unit) []*syntax.File {
	var out []*syntax.File
	for _, u := range units {
		out = append(out, u.Files...)
	}
	return out
}

// checkLayers refuses each layer no loaded package has a file for, E1901, as an *OpenError (EVALUATION.md §9.1).
func (p *Project) checkLayers(s *snapshot, loaded []*project.Unit) error {
	bag := diag.NewBag(s.set, "")
	for _, name := range p.opt.Layers {
		if !slices.ContainsFunc(loaded, func(u *project.Unit) bool { return slices.Contains(u.Layers, name) }) {
			diag.E1901.At(source.Span{}, name).Report(bag)
		}
	}
	if bag.Summary().Errors == 0 {
		return nil
	}
	return &OpenError{Err: ErrUnknownLayer, Findings: collect(s.set, bag)}
}
