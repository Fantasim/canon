package build

import (
	"context"
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// placedEmit is one emit of the run, every copy and target, with the directory its files go
// below, as placed and by real path, and its files by real path, made on first use (DECISIONS 341).
type placedEmit struct {
	p      *ir.Package
	e      *ir.Emit
	dir    string
	real   string
	files  map[string]bool
	listed bool
}

// feedback is one search for files a load reads and an emit of the same build writes: the
// run's emits, the real paths both are compared by, and the findings, reported at the end.
type feedback struct {
	r      *run
	real   *realPaths
	emits  []*placedEmit
	asked  bool // failed is known
	failed bool
	found  []*diag.Builder
	pkgs   []string // the package of each finding
}

// feedback is E8026 at each load of the run that reads a file an emit of the run writes:
// compared by real path where the disk has it, lexically otherwise (DECISIONS 341).
func (r *run) feedback(ctx context.Context) error {
	fb := &feedback{r: r, real: newRealPaths(r.p.fs)}
	fb.emits = r.placedEmits(fb.real)
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

// file judges every load of f, a source of package pkg.
func (fb *feedback) file(pkg string, f *syntax.File) error {
	from := path.Dir(f.Src.Path)
	for _, at := range fb.r.s.gen.loadsOf(f) {
		named, _ := load.Names(fb.r.p.fs, fb.r.s.layout, from, at.e) // a path that is no literal names nothing
		if err := fb.judge(pkg, at.span, named.Files); err != nil {
			return err
		}
	}
	return nil
}

// overlaps reports a file the run's loads read, by real path, at or below an emit's directory:
// else no load reads an output, and none is walked again.
func (fb *feedback) overlaps() bool {
	for abs := range fb.r.inputs().loaded { //canon:unordered an existence test
		real := fb.real.file(abs)
		for _, pe := range fb.emits {
			if _, in := under(pe.real, real); in {
				return true
			}
		}
	}
	return false
}

// placedEmits is every emit of the run stage E placed, with its directory.
func (r *run) placedEmits(real *realPaths) []*placedEmit {
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
			out = append(out, &placedEmit{p: p, e: e, dir: dir, real: real.dir(dir)})
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
				fb.found = append(fb.found, diag.E8026.At(span, f.Display, targetWords[pe.e.Target], pe.p.Name))
				fb.pkgs = append(fb.pkgs, pkg)
				break
			}
		}
	}
	return nil
}

// writes reports real, a real path, among pe's files.
func (fb *feedback) writes(pe *placedEmit, real string) (bool, error) {
	if !pe.listed {
		files, err := fb.filesOf(pe)
		if err != nil {
			return false, err
		}
		pe.files, pe.listed = map[string]bool{}, true
		for _, f := range files {
			pe.files[fb.real.file(project.Join(pe.dir, f.Path))] = true
		}
	}
	return pe.files[real], nil
}

// filesOf is the files pe writes: a view's one file, which a build writes even after an error;
// any other emit's as its generator makes them, none when the run has an error, after which a
// build writes no such file (API.md B1).
func (fb *feedback) filesOf(pe *placedEmit) ([]ir.File, error) {
	if pe.e.Target == ir.TargetView {
		return []ir.File{{Path: pe.e.FileName}}, nil
	}
	if !fb.asked {
		fb.asked, fb.failed = true, !fb.r.errorFree()
	}
	if fb.failed {
		return nil, nil
	}
	cp := ir.CopyOf(fb.r.s.proj, pe.p, pe.e)
	if err := complete(cp); err != nil { // a run without error leaves every value set
		return nil, err
	}
	return fb.r.generate(cp, pe.e)
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
