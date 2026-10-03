package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ValuesAt are the values and map keys of the lets that a source states at byte offset off of
// file, the values amendments replaced included: those of the narrowest provenance span holding
// off, each once, in walk order. A ref stated once and evaluated per instance is one per instance.
func (s *Snapshot) ValuesAt(ctx context.Context, file source.FileID, off int) ([]value.Value, error) {
	var out []value.Value
	var narrowest source.Span
	in := map[value.Value]bool{}
	w := s.newWalk(func(_ rootRef, v, _ value.Value, _ []Seg) bool {
		p := provOf(v)
		switch {
		case p == nil || p.Span.File != file || !holds(p.Span, off) || in[v]:
		case len(out) == 0 || p.Span.Len() < narrowest.Len():
			out, narrowest, in = []value.Value{v}, p.Span, map[value.Value]bool{v: true}
		case p.Span == narrowest:
			out, in[v] = append(out, v), true
		}
		return false
	}, true)
	if err := w.run(ctx, s); err != nil {
		return nil, err
	}
	return out, nil
}

// holds reports the byte offset off within sp.
func holds(sp source.Span, off int) bool {
	return int(sp.Start) <= off && off < int(sp.End)
}

// FileOf is the file the lets' values were read from whose absolute name is abs, NoFile for
// none: the version this analysis read, where the file set holds several.
func (s *Snapshot) FileOf(ctx context.Context, abs string) (source.FileID, error) {
	set, ok := s.a.Files().(*source.FileSet)
	if !ok {
		return source.NoFile, nil
	}
	found := source.NoFile
	w := s.newWalk(func(_ rootRef, v, _ value.Value, _ []Seg) bool {
		p := provOf(v)
		if p != nil {
			if f := set.File(p.Span.File); f != nil && f.Abs == abs {
				found = f.ID
			}
		}
		return found != source.NoFile
	}, true)
	err := w.run(ctx, s)
	return found, err
}

// identKey is an entry's identity as a ref names it: its collection, owner and key.
type identKey struct {
	coll  *types.Collection
	owner *value.Record
	key   value.Key
}

// letName is a let by package and name.
type letName struct{ pkg, name string }

// EntriesOf are the table entries and keyed-list elements refs name, where the lets' values hold
// them, each once, in walk order: a ref into a record field's collection names its own instance's
// (API.md R7). Only a record in a table or a keyed list is an entry.
func (s *Snapshot) EntriesOf(ctx context.Context, refs []*value.Ref) ([]Resolved, error) {
	want, lets := map[identKey]bool{}, map[letName]bool{}
	field := false
	for _, ref := range refs {
		if rt, ok := baseOf(ref.T).(*types.RefType); ok && rt.Target != nil {
			want[identKey{rt.Target, ref.Owner, ref.Key}] = true
			lets[letName{rt.Target.Pkg, rt.Target.Name}] = true
			field = field || rt.Target.Kind != types.CollLet
		}
	}
	var found []*value.Record
	var paths []Path
	w := s.newWalk(func(r rootRef, v, parent value.Value, segs []Seg) bool {
		rec, ok := v.(*value.Record)
		if ok && rec.Ident != nil && parent != nil && keyedContainer(parent.Type()) &&
			want[identKey{rec.Ident.Coll, rec.Ident.Owner, rec.Ident.Key}] && !slices.Contains(found, rec) {
			found, paths = append(found, rec), append(paths, Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs})
		}
		return false
	}, false)
	if !field {
		w.roots = func(r rootRef) bool { return lets[letName{r.pkg.Path, r.obj.Name()}] }
	}
	if err := w.run(ctx, s); err != nil {
		return nil, err
	}
	return s.resolveFound(found, paths)
}

// resolveFound resolves each path, keeping those that lead back to the record found there.
func (s *Snapshot) resolveFound(found []*value.Record, paths []Path) ([]Resolved, error) {
	var out []Resolved
	for i, p := range paths {
		res, err := Resolve(s, p)
		if err != nil {
			return nil, err
		}
		if res.Target == found[i] {
			out = append(out, res)
		}
	}
	return out, nil
}
