package views

import (
	"path"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// sources is where a value comes from (VIEWMODEL.md 12.6 `sources`): its declaring file, then
// the file a single-file load reads; the glob of a load.dir and the files it matched; the
// entries declared in other files (API.md W2).
func (b *builder) sources(o check.Object, d *syntax.LetDecl) vm.Sources {
	s := vm.Sources{Files: []string{o.File().Src.Path}}
	if n := b.entriesOf(o); n > 0 {
		s.Entries = &n
	}
	x, isLoad := shape.Unparen(d.Value).(*syntax.LoadExpr)
	if !isLoad {
		return s
	}
	p, ok := b.loaded(o.File(), x)
	switch {
	case !ok:
	case loadsDir(d):
		s.Glob = p.Display
		if n, matched := b.filesMatched(o); matched {
			s.Count = &n
		}
	default:
		s.Files = append(s.Files, p.Display)
	}
	return s
}

// loaded is the path a load reads, resolved as the loader does (WIRE.md 2); false without a
// layout or a written path (the checker reports a bad one).
func (b *builder) loaded(f *syntax.File, x *syntax.LoadExpr) (project.Path, bool) {
	if b.in.Layout == nil || len(x.Args) == 0 || x.Args[0].Name != nil {
		return project.Path{}, false
	}
	written, ok := shape.Unparen(x.Args[0].Value).(*syntax.StringLit)
	if !ok || len(written.Parts) != 1 || written.Parts[0].Interp != nil {
		return project.Path{}, false
	}
	return b.in.Layout.Resolve(written.Parts[0].Text, path.Dir(f.Src.Path), f.Span(x), diag.NewBag(nil, ""))
}

// loadsDir reports a let loaded by `load.dir` (T2, 12.6 `glob`).
func loadsDir(d *syntax.LetDecl) bool {
	x, ok := shape.Unparen(d.Value).(*syntax.LoadExpr)
	return ok && x.Method != nil && x.Method.Name == loadDirMethod
}

// filesMatched counts the files the elements of a let loaded by `load.dir` come from.
func (b *builder) filesMatched(o check.Object) (int, bool) {
	v, ok := b.colls.Let(b.pkg.Path, o.Name())
	if !ok {
		return 0, false
	}
	files := map[source.FileID]bool{}
	for _, e := range std.Elems(v) {
		if p := e.Prov(); p != nil {
			files[p.Span.File] = true
		}
	}
	return len(files), true
}

// entriesOf counts the `entry` declarations of o in files other than its own (API.md W2).
func (b *builder) entriesOf(o check.Object) int {
	n := 0
	for _, f := range b.pkg.Files {
		if f == o.File() {
			continue
		}
		for _, d := range f.Decls {
			if e, ok := d.(*syntax.EntryDecl); ok && e.Table != nil && b.in.Program.Info.NameUses[e.Table] == o {
				n++
			}
		}
	}
	return n
}

// layersOf are the active layers that amend the let o, in application order (12.6 `layers`).
func (b *builder) layersOf(o check.Object) []string {
	var out []string
	for _, name := range b.in.Layers {
		for _, blk := range b.pkg.Layers[name] {
			if blk.Target != nil && b.in.Program.Info.NameUses[blk.Target] == o {
				out = append(out, name)
				break
			}
		}
	}
	return out
}
