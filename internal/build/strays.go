package build

import (
	"bytes"
	"cmp"
	"context"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// strayDir is a directory an emit of the selected packages writes into, with the emit whose
// out entry a finding there points at.
type strayDir struct {
	abs, display string
	at           source.Span
	pkg          string
}

// marker is what a generated file's marker names: a package, or a runtime (CODEGEN.md §2.4).
type marker struct {
	name    string
	runtime bool
}

// stray is a marked file no emit of the build writes, with its directory and its marker.
type stray struct {
	d          strayDir
	abs, shown string
	m          marker
}

// strays is E8029 under --check, else W8030, for each marked file of the selected packages' emit directories that no emit of the project writes, never deleted (CODEGEN.md §2.4 "Stray generated files", DECISIONS 356).
func (r *run) strays(ctx context.Context, opt BuildOptions, outputs []*output) error {
	written := r.unrun(opt.Targets)
	for _, o := range outputs {
		if !o.remove {
			written[strings.ToLower(o.Abs)] = true
		}
	}
	into := map[string]bool{}
	for key := range written { //canon:unordered a set
		into[project.DirOf(key)] = true
	}
	var found []stray
	for _, d := range r.strayDirs(opt) {
		got, err := r.straysIn(d, written, into[strings.ToLower(d.abs)])
		if err != nil {
			return err
		}
		found = append(found, got...)
	}
	found, err := r.unclaimed(ctx, found)
	for _, s := range found {
		b := diag.W8030.At(s.d.at, s.shown, s.m.name)
		if opt.Check {
			b = diag.E8029.At(s.d.at, s.shown, s.m.name)
		}
		b.Report(r.bags[s.d.pkg])
	}
	return err
}

// strayDirs are the directories the emits of the selected packages write into, each once, in emit order.
func (r *run) strayDirs(opt BuildOptions) []strayDir {
	var all []strayDir
	for _, p := range r.ir {
		for _, e := range p.Emits {
			all = append(all, r.emitStrayDirs(p, e, opt.OnlyRoot)...)
		}
	}
	seen := map[string]bool{}
	return slices.DeleteFunc(all, func(d strayDir) bool {
		key := strings.ToLower(d.abs)
		again := seen[key]
		seen[key] = true
		return again
	})
}

// emitStrayDirs are the directories e writes into: its out, a file-mode out's directory, Go's
// <out>/rt; none under an absent root, and under a consumer root only for --only-root of it.
func (r *run) emitStrayDirs(p *ir.Package, e *ir.Emit, only string) []strayDir {
	at, ok := r.outPath(e)
	if e.Dir == "" || !ok {
		return nil
	}
	display, abs := strings.TrimSuffix(at.Display, pathSep), at.Abs
	if e.FileName != "" {
		display, abs = path.Dir(display), project.DirOf(abs)
	}
	if r.absentAt(abs) != "" || r.consumerOf(display) != only {
		return nil
	}
	dirs := []strayDir{{abs: abs, display: display, at: r.emitSpan(p, e), pkg: p.Name}}
	if e.Target == ir.TargetGo {
		dirs = append(dirs, strayDir{abs: project.Join(abs, goRuntimeDir), display: path.Join(display, goRuntimeDir), at: dirs[0].at, pkg: p.Name})
	}
	return dirs
}

// straysIn are the marked files of d no emit of the build writes, in name order; written holds
// the files the build's emits write, filled tells whether any of them lies in d.
func (r *run) straysIn(d strayDir, written map[string]bool, filled bool) ([]stray, error) {
	entries, err := r.p.fs.ReadDir(d.abs)
	switch {
	case cannotExist(err):
		return nil, nil
	case err != nil:
		return nil, displayError(d.display, err)
	}
	var out []stray
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	for _, e := range entries {
		abs := project.Join(d.abs, e.Name())
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), tempPrefix) || written[strings.ToLower(abs)] { // a hidden name, like a staged file's
			continue
		}
		m, ok, err := r.markerOf(abs)
		shown := path.Join(d.display, e.Name())
		switch {
		case err != nil:
			return nil, displayError(shown, err)
		case ok && r.flagged(m, filled):
			out = append(out, stray{d: d, abs: abs, shown: shown, m: m})
		}
	}
	return out, nil
}

// flagged reports a marker a stray file is reported for: a selected package's, or one naming no
// package of the project, a runtime's only in a directory no file of the build lands in.
func (r *run) flagged(m marker, filled bool) bool {
	if m.runtime {
		return !filled
	}
	return r.selects(m.name) || !r.inProject(m.name)
}

// inProject reports a package of the project, selected or not.
func (r *run) inProject(pkg string) bool {
	return slices.ContainsFunc(r.s.units, func(u *project.Unit) bool { return u.Name == pkg })
}

// markerOf is what the generated-file marker of the file abs names: the package of a code
// marker's directory or of a $schema's qualified type, or a runtime; false for a file with none.
func (r *run) markerOf(abs string) (marker, bool, error) {
	if path.Ext(abs) == ir.JSONExt {
		data, err := r.p.fs.ReadFile(abs)
		if err != nil {
			return marker{}, false, ignoreGone(err)
		}
		schema, ok := jsonSchema(data, schemaMarker)
		typ, _, _ := strings.Cut(schema, schemaTypeSep)
		return marker{name: typ[:max(strings.LastIndex(typ, qnameSep), 0)]}, ok, nil
	}
	head, err := r.readHead(abs)
	if err != nil {
		return marker{}, false, ignoreGone(err)
	}
	line, _, _ := bytes.Cut(head, []byte(lineEnd))
	line = bytes.TrimSuffix(line, []byte(carriageReturn))
	if m := markerDir.FindSubmatch(line); m != nil {
		return marker{name: dotted(string(m[1]))}, true, nil
	}
	if m := markerRuntime.FindSubmatch(line); m != nil {
		return marker{name: string(m[1]), runtime: true}, true, nil
	}
	return marker{}, false, nil
}

// ignoreGone is err, nil when the file is gone meanwhile: it is no stray then.
func ignoreGone(err error) error {
	if cannotExist(err) {
		return nil
	}
	return err
}
