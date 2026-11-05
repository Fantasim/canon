package edit

import (
	"cmp"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

// keyedName is a map item of file f whose word key a rename writes as key, a string.
type keyedName struct {
	f   *syntax.File
	key string
}

// keyedItems writes each map item whose word key the rename quotes (DECISIONS 318) as one Replace of
// the item: the new key, then every byte of the item after its key, comments kept, with the rename's
// changes inside it spliced in and dropped; innermost first, so that no two changes overlap.
func (r *renaming) keyedItems() {
	items := slices.Collect(maps.Keys(r.items))
	slices.SortFunc(items, func(a, b *syntax.FieldItem) int {
		fa, fb := r.items[a].f, r.items[b].f
		sa, sb := fa.Span(a), fb.Span(b)
		return cmp.Or(cmp.Compare(sa.End-sa.Start, sb.End-sb.Start), cmp.Compare(fa.Src.Path, fb.Src.Path), cmp.Compare(sa.Start, sb.Start))
	})
	for _, fi := range items {
		k := r.items[fi]
		path := k.f.Src.Path
		inner, kept := splitInside(k.f, fi, r.text[path])
		r.text[path] = append(kept, format.Change{Kind: format.Replace, Node: fi, Text: k.key + spliced(k.f, fi, inner)})
	}
}

// splitInside parts changes of f into those inside item fi and the others.
func splitInside(f *syntax.File, fi *syntax.FieldItem, changes []format.Change) (inner, kept []format.Change) {
	sp := f.Span(fi)
	for _, c := range changes {
		if contains(sp, f.Span(c.Node)) {
			inner = append(inner, c)
		} else {
			kept = append(kept, c)
		}
	}
	return inner, kept
}

// spliced is f's bytes of item fi after its key, with each change of inner written over its node.
func spliced(f *syntax.File, fi *syntax.FieldItem, inner []format.Change) string {
	from, to := f.Span(fi.Name).End, f.Span(fi).End
	slices.SortFunc(inner, func(a, b format.Change) int { return cmp.Compare(f.Span(b.Node).Start, f.Span(a.Node).Start) })
	out := slices.Clone(f.Src.Content[from:to])
	for _, c := range inner {
		sp := f.Span(c.Node)
		out = slices.Concat(out[:sp.Start-from], []byte(c.Text), out[sp.End-from:])
	}
	return string(out)
}
