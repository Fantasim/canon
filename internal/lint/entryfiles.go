package lint

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// offEntry is an entry declared off the path its let's @files template gives it: the file it sits in and the path the template gives.
type offEntry struct {
	file, want string
	at         source.Pos
}

// entryFiles reports W0006 (API.md L7): once per sound let with @files, at the annotation, the files other than the
// let's that hold an entry at a path other than the one N2 and N3 give it, the first file's first entry naming the path.
func entryFiles(p *pass, pkg *check.Package, bag *diag.Bag) {
	for _, f := range sources(pkg) {
		for _, d := range f.Decls {
			let, ok := d.(*syntax.LetDecl)
			if !ok || let.Name == nil || p.declared(let.Name) == nil {
				continue
			}
			ann := filesAnnotation(let)
			if ann == nil {
				continue
			}
			off := p.offPath(pkg, f, let)
			slices.SortFunc(off, compareOff)
			off = slices.CompactFunc(off, func(a, b offEntry) bool { return a.file == b.file })
			if len(off) > 0 {
				diag.W0006.At(f.Span(ann), let.Name.Name, int64(len(off)), off[0].file, off[0].want).Report(bag)
			}
		}
	}
}

// filesAnnotation is the `@files` annotation of let, nil when it has none.
func filesAnnotation(let *syntax.LetDecl) *syntax.Annotation {
	for _, a := range let.Annotations {
		if a.Name != nil && a.Name.Name == syntax.AnnFiles {
			return a
		}
	}
	return nil
}

// compareOff orders entries by file path in bytes, then by position.
func compareOff(a, b offEntry) int {
	return cmp.Or(strings.Compare(a.file, b.file), cmp.Compare(a.at, b.at))
}

// offPath are the entries of let, a let of pkg written in file home, declared in another file of pkg at a path other than the
// one its template gives their evaluated value; an entry its template cannot be filled for is not judged.
func (p *pass) offPath(pkg *check.Package, home *syntax.File, let *syntax.LetDecl) []offEntry {
	entries := p.entriesOf(pkg, let)
	if entries == nil {
		return nil
	}
	var out []offEntry
	for _, f := range sources(pkg) {
		if f != home {
			out = append(out, p.offIn(pkg, f, let, entries)...)
		}
	}
	return out
}

// offIn are the entries of let declared in f at a path other than the one the template of let gives them.
func (p *pass) offIn(pkg *check.Package, f *syntax.File, let *syntax.LetDecl, entries map[value.Key]*value.Record) []offEntry {
	var out []offEntry
	for _, d := range f.Decls {
		e, ok := d.(*syntax.EntryDecl)
		if !ok || e.Table == nil || e.Table.Name != let.Name.Name || p.broken.nodes[e] {
			continue
		}
		rec := entries[entryKey(e.Key)]
		if rec == nil {
			continue
		}
		if want, err := p.placer().Path(pkg, let, rec); err == nil && want != f.Src.Path {
			out = append(out, offEntry{file: f.Src.Path, want: want, at: f.Span(e).Start})
		}
	}
	return out
}

// entriesOf are the entries of let's settled value by key, nil when the analysis did not settle it.
func (p *pass) entriesOf(pkg *check.Package, let *syntax.LetDecl) map[value.Key]*value.Record {
	v, ok := p.a.Force(eval.Root{Pkg: pkg.Path, Name: let.Name.Name})
	if !ok {
		return nil
	}
	var recs []*value.Record
	switch v := v.(type) {
	case *value.Table:
		recs = v.Entries
	case *value.List:
		for _, el := range v.Elems {
			if rec, isRec := el.(*value.Record); isRec {
				recs = append(recs, rec)
			}
		}
	}
	out := map[value.Key]*value.Record{}
	for _, rec := range recs {
		if rec.Ident != nil {
			out[rec.Ident.Key] = rec
		}
	}
	return out
}

// entryKey is the key an entry declaration writes: a word or an integer.
func entryKey(k syntax.EntryKey) value.Key {
	switch x := k.(type) {
	case *syntax.Ident:
		return value.Key{S: x.Name}
	case *syntax.IntLit:
		return value.Key{I: x.Value.Int64(), IsInt: true}
	}
	return value.Key{}
}

// placer is the Placer of this pass, made on first use: its host evaluates a default a template reads.
func (p *pass) placer() *edit.Placer {
	if p.places == nil {
		p.places = p.snap.NewPlacer(p.ctx, build.EditHost(p.a))
	}
	return p.places
}
