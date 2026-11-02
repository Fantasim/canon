package build

import (
	"context"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// placedEmit is one emit of the run, every copy and target, with the directory its files go
// below, as placed and by real path, and its files, made on first use (DECISIONS 341).
type placedEmit struct {
	p      *ir.Package
	e      *ir.Emit
	dir    string
	real   string
	files  []plannedFile
	byReal map[string]bool
	listed bool
}

// plannedFile is a file an emit will write: its path as placed and by real path.
type plannedFile struct {
	abs  string
	real string
}

// globLoad is a literal glob: its prefix's directory and the glob after it (WIRE.md §6.5).
type globLoad struct {
	base project.Path
	rest string
}

// foundAt is a finding's key: one per load and physical file, by real path.
type foundAt struct {
	span source.Span
	real string
}

// feedback is one search for files a load reads and an emit of the same build writes: the
// run's emits, the real paths both are compared by, and the findings, reported at the end.
type feedback struct {
	r      *run
	real   *realPaths
	emits  []*placedEmit
	failed bool // the run had an error after stage E: a build writes no code or data
	found  []*diag.Builder
	pkgs   []string // the package of each finding
	seen   map[foundAt]bool
}

// feedback is E8026 at each load of the run that reads a file an emit of the run writes:
// compared by real path where the disk has it, lexically otherwise (DECISIONS 341); free is
// whether the run had no error after stage E.
func (r *run) feedback(ctx context.Context, free bool) error {
	fb := &feedback{r: r, real: newRealPaths(r.p.fs), failed: !free, seen: map[foundAt]bool{}}
	fb.emits = r.placedEmits()
	for _, pe := range fb.emits {
		pe.real = fb.real.dir(pe.dir)
	}
	if len(fb.emits) == 0 || !fb.overlaps() {
		return nil
	}
	for _, cp := range r.prog.Packages {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, f := range cp.Files {
			if err := fb.file(cp.Path, f); err != nil {
				return err
			}
		}
	}
	for i, b := range fb.found {
		b.Report(r.s.bag(fb.pkgs[i]))
	}
	return nil
}

// file judges every load of f, a source of package pkg: the files it names, and for a load.dir
// the paths the run's emits will write (DECISIONS 451).
func (fb *feedback) file(pkg string, f *syntax.File) error {
	from := path.Dir(f.Src.Path)
	for _, at := range fb.r.s.gen.loadsOf(f) {
		named, _ := load.Names(fb.r.p.fs, fb.r.s.layout, from, at.e) // a path that is no literal names nothing
		if err := fb.judge(pkg, at.span, named.Files); err != nil {
			return err
		}
		g, ok := fb.globOf(from, at.e)
		if !ok {
			continue
		}
		if err := fb.planned(pkg, at.span, g); err != nil {
			return err
		}
	}
	return nil
}

// overlaps reports a file the run's loads read, by real path, at or below an emit's directory,
// or a load.dir whose base meets one: else no load reads an output, and none is walked again.
func (fb *feedback) overlaps() bool {
	for abs := range fb.r.inputs().loaded { //canon:unordered an existence test
		real := fb.real.file(abs)
		for _, pe := range fb.emits {
			if _, in := under(pe.real, real); in {
				return true
			}
		}
	}
	return fb.globsMeet()
}

// globsMeet reports a load.dir of the run whose base is at, above or below an emit's directory.
func (fb *feedback) globsMeet() bool {
	for _, cp := range fb.r.prog.Packages {
		for _, f := range cp.Files {
			if fb.fileMeets(f) {
				return true
			}
		}
	}
	return false
}

// fileMeets is globsMeet for the loads of f.
func (fb *feedback) fileMeets(f *syntax.File) bool {
	from := path.Dir(f.Src.Path)
	for _, at := range fb.r.s.gen.loadsOf(f) {
		if g, ok := fb.globOf(from, at.e); ok && len(fb.related(g)) > 0 {
			return true
		}
	}
	return false
}

// globOf is e as a globLoad, written in a file of the directory from: false but for a load.dir of
// a literal pattern with a glob (a missing file stays E7004).
func (fb *feedback) globOf(from string, e *syntax.LoadExpr) (globLoad, bool) {
	prefix, rest, ok := load.DirGlob(e)
	if !ok {
		return globLoad{}, false
	}
	base, ok := fb.r.s.layout.Resolve(prefix, from, source.Span{}, diag.NewBag(nil, ""))
	return globLoad{base: base, rest: rest}, ok
}

// related is the emits whose directory is at, above or below g's base, by path or by real path.
func (fb *feedback) related(g globLoad) []*placedEmit {
	baseReal := fb.real.dir(g.base.Abs)
	var out []*placedEmit
	for _, pe := range fb.emits {
		if meets(pe.dir, g.base.Abs) || meets(pe.real, baseReal) {
			out = append(out, pe)
		}
	}
	return out
}

// meets reports one of the directories a and b at or below the other.
func meets(a, b string) bool {
	if _, in := under(a, b); in {
		return true
	}
	_, in := under(b, a)
	return in
}

// planned records E8026 for each path an emit will write that g selects (CODEGEN.md §2.4, DECISIONS 451).
func (fb *feedback) planned(pkg string, span source.Span, g globLoad) error {
	shown := strings.TrimSuffix(g.base.Display, pathSep) + pathSep
	baseReal := fb.real.dir(g.base.Abs)
	for _, pe := range fb.related(g) {
		files, err := fb.plannedOf(pe)
		if err != nil {
			return err
		}
		for _, f := range files {
			rel, ok := below(g.base.Abs, f.abs)
			if !ok {
				rel, ok = below(baseReal, f.real)
			}
			if ok && load.MatchesPlanned(shown+g.rest, shown+rel) {
				fb.record(pkg, span, path.Join(g.base.Display, rel), f.real, pe)
			}
		}
	}
	return nil
}

// below is name's path below dir, false for dir itself or a name outside it.
func below(dir, name string) (string, bool) {
	rel, in := under(dir, name)
	return rel, in && rel != listingMark
}

// record is E8026 at span of pkg for the file shown as display, at real, once, naming pe.
func (fb *feedback) record(pkg string, span source.Span, display, real string, pe *placedEmit) {
	key := foundAt{span: span, real: real}
	if fb.seen[key] {
		return
	}
	fb.seen[key] = true
	fb.found = append(fb.found, diag.E8026.At(span, display, targetWords[pe.e.Target], pe.p.Name))
	fb.pkgs = append(fb.pkgs, pkg)
}

// placedEmits is every emit of the run stage E placed, with its directory.
func (r *run) placedEmits() []*placedEmit {
	var out []*placedEmit
	for _, p := range r.ir {
		for _, e := range p.Emits {
			if e.Dir == "" {
				continue
			}
			at, ok := r.outPath(e)
			if !ok {
				continue
			}
			dir := at.Abs
			if e.FileName != "" {
				dir = project.DirOf(dir)
			}
			out = append(out, &placedEmit{p: p, e: e, dir: dir})
		}
	}
	return out
}

// judge records E8026 at the load at span of pkg for each of files an emit writes: one finding
// per file, naming the first such emit.
func (fb *feedback) judge(pkg string, span source.Span, files []load.File) error {
	for _, f := range files {
		real := fb.real.file(f.Abs)
		for _, pe := range fb.emits {
			if _, in := under(pe.real, real); !in {
				continue
			}
			writes, err := fb.writes(pe, real)
			if err != nil {
				return err
			}
			if writes {
				fb.record(pkg, span, f.Display, real, pe)
				break
			}
		}
	}
	return nil
}

// writes reports real, a real path, among pe's files.
func (fb *feedback) writes(pe *placedEmit, real string) (bool, error) {
	if _, err := fb.plannedOf(pe); err != nil {
		return false, err
	}
	return pe.byReal[real], nil
}

// plannedOf is the files pe writes, made on first use.
func (fb *feedback) plannedOf(pe *placedEmit) ([]plannedFile, error) {
	if pe.listed {
		return pe.files, nil
	}
	files, err := fb.filesOf(pe)
	if err != nil {
		return nil, err
	}
	pe.byReal, pe.listed = map[string]bool{}, true
	for _, f := range files {
		abs := project.Join(pe.dir, f.Path)
		pf := plannedFile{abs: abs, real: fb.real.file(abs)}
		pe.files = append(pe.files, pf)
		pe.byReal[pf.real] = true
	}
	return pe.files, nil
}

// filesOf is the files pe writes, by name (fileNames): a view's one file, which a build writes
// even after an error; any other emit's none when the run has an error, after which a build
// writes no such file (API.md B1).
func (fb *feedback) filesOf(pe *placedEmit) ([]ir.File, error) {
	if fb.failed && pe.e.Target != ir.TargetView {
		return nil, nil
	}
	return fb.r.fileNames(ir.CopyOf(fb.r.s.proj, pe.p, pe.e), pe.e)
}

// errorFree reports no error yet in the project or a loaded package: a build then emits every target.
func (r *run) errorFree() bool {
	if r.s.own.ErrorCount() > 0 {
		return false
	}
	for _, u := range r.loaded {
		if r.s.bag(u.Name).ErrorCount() > 0 {
			return false
		}
	}
	return true
}
