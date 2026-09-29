package views

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// search is the `search` section (VIEWMODEL.md 12.8): one index per collection S1 names.
func (b *builder) search() {
	for _, o := range b.pkg.Decls {
		if b.ctx.Err() != nil {
			return // Build reports the cancellation
		}
		if o.Kind() == check.ObjLet && !b.broken(o) && b.indexed(o) {
			b.m.Search[valueID(b.pkg.Path, o.Name())] = b.searchIndex(o)
		}
	}
}

// indexed reports a let S1 indexes: a public table, stable table, keyed list or define table,
// or such a collection of any visibility that a `ref` type of the package targets.
func (b *builder) indexed(o check.Object) bool {
	if collectionOf(o) == nil {
		return false
	}
	if d, ok := o.Decl().(*syntax.LetDecl); ok && !shape.Local(d) {
		return true
	}
	if b.targets == nil {
		b.targets = refTargets(b.in.Program.Info, b.pkg)
	}
	return b.targets[o.Name()]
}

// collectionOf is the collection the let o is, as a ref names it (J12); nil when it is not a
// table, a keyed list or a define table.
func collectionOf(o check.Object) *types.Collection {
	c := &types.Collection{Kind: types.CollLet, Pkg: o.Pkg(), Name: o.Name()}
	switch t := o.Type().Base().(type) {
	case *types.TableType:
		c.Elem = t.Elem
		if t.Elem.Kind() == types.Define {
			c.Kind = types.CollDefines
		}
	case *types.ListType:
		if t.KeyedBy == nil {
			return nil
		}
		c.Elem, c.KeyedBy = t.Elem, t.KeyedBy
	default:
		return nil
	}
	return c
}

// refTargets are the lets of p that a ref type of p targets (S1): in every type expression its
// files write (fields, parameters, signatures, arguments) and every let's type.
func refTargets(info *check.Info, p *check.Package) map[string]bool {
	out := map[string]bool{}
	mark := func(c *types.Collection) {
		if c.Kind != types.CollField && c.Pkg == p.Path && len(c.FieldPath) == 0 {
			out[c.Name] = true
		}
	}
	fns := map[*types.TypeFunc]bool{}
	var walk func(types.Type)
	walk = func(t types.Type) {
		encode.Walk(t, func(x types.Type) bool {
			switch y := x.Base().(type) {
			case *types.RefType:
				mark(y.Target)
			case *types.DepMapType:
				mark(y.Coll)
			case *types.TypeAppType:
				walkFunc(y.Fn, fns, walk)
			case *types.DepUnionType:
				walkFunc(y.Fn, fns, walk)
			}
			return true
		})
	}
	for _, t := range writtenTypes(info, p) {
		walk(t)
	}
	return out
}

// writtenTypes are the types of every type expression of p's files, then of its lets.
func writtenTypes(info *check.Info, p *check.Package) []types.Type {
	var out []types.Type
	for _, f := range p.Files {
		syntax.Inspect(f, func(n syntax.Node) bool {
			if t, isType := n.(syntax.Type); isType && info.TypeExprs[t] != nil {
				out = append(out, info.TypeExprs[t])
			}
			return true
		})
	}
	for _, o := range p.Decls {
		if o.Kind() == check.ObjLet && o.Type() != nil {
			out = append(out, o.Type())
		}
	}
	return out
}

// walkFunc walks the branches and the body of a type function once (TYPES.md 11).
func walkFunc(fn *types.TypeFunc, seen map[*types.TypeFunc]bool, walk func(types.Type)) {
	if fn == nil || seen[fn] {
		return
	}
	seen[fn] = true
	for _, a := range fn.Arms {
		walk(a.Result)
	}
	if fn.Body != nil {
		walk(fn.Body)
	}
}

// searchIndex is the index of the let o (S1, S2): its element, key type and entry counts, the
// asset its rows preview, and a row per entry in collection order, retired ones flagged.
func (b *builder) searchIndex(o check.Object) vm.SearchIndex {
	coll := collectionOf(o)
	count, active := b.colls.Counts(coll)
	idx := vm.SearchIndex{Type: encode.Element(coll), KeyType: encode.KeyType(coll), Count: count, Active: active, Rows: []vm.Row{}}
	idx.Preview = b.previewAsset(o, coll)
	var entries []*value.Record
	for i, e := range b.colls.Entries(coll) {
		if b.ctx.Err() != nil {
			break
		}
		if e.Ident != nil {
			entries = append(entries, e)
			// VIEWMODEL.md 3.4: a keyed list's element with its `index`, as Evaluate gives it (API.md 11)
			idx.Rows = append(idx.Rows, b.row(e, b.render.At(e, render.Position(coll, i))))
		}
	}
	disambiguate(idx.Rows, entries)
	return idx
}

// row is an entry's row (S2), rendered by r: its key, rendered title, subtitle and terms, preview
// file, retired flag, and the title and subtitle in each other language where they render
// differently.
func (b *builder) row(e *value.Record, r *render.Renderer) vm.Row {
	out := vm.Row{Key: keyScalar(e.Ident.Key), Retired: e.Ident.Retired, Terms: r.Terms(e)}
	out.Title = rendered(r.Title(e, ""))
	out.Subtitle = rendered(r.Subtitle(e, ""))
	out.Preview, _ = r.Preview(e)
	for _, lang := range b.otherLanguages() {
		t := vm.RowText{Title: differs(out.Title, rendered(r.Title(e, lang))), Subtitle: differs(out.Subtitle, rendered(r.Subtitle(e, lang)))}
		if t.Title == nil && t.Subtitle == nil {
			continue
		}
		if out.Tr == nil {
			out.Tr = map[string]vm.RowText{}
		}
		out.Tr[lang] = t
	}
	return out
}

// rendered is a rendering's text, nil when it failed or there is none.
func rendered(s string, ok bool) *string {
	if !ok {
		return nil
	}
	return &s
}

// differs is t when it is not src's text, nil otherwise.
func differs(src, t *string) *string {
	if t == nil || src != nil && *src == *t {
		return nil
	}
	return t
}

// keyScalar is an entry's key as the model writes it: a string, or an integer (J10).
func keyScalar(k value.Key) vm.Scalar {
	if k.IsInt {
		return vm.Scalar(vm.Int(k.I))
	}
	return vm.Scalar{Text: k.S, Quoted: true}
}

// disambiguate writes each title two entries share as `<title> (<key>)`, in every language,
// the source-language titles compared (S9).
func disambiguate(rows []vm.Row, entries []*value.Record) {
	seen := map[string]int{}
	for _, r := range rows {
		if r.Title != nil {
			seen[*r.Title]++
		}
	}
	for i := range rows {
		if rows[i].Title == nil || seen[*rows[i].Title] < render.SharedTitle {
			continue
		}
		key := entries[i].Ident.Key.Text()
		t := render.Disambiguated(*rows[i].Title, key)
		rows[i].Title = &t
		//canon:unordered each language's title suffixed alone
		for lang, tr := range rows[i].Tr {
			if tr.Title != nil {
				s := render.Disambiguated(*tr.Title, key)
				tr.Title = &s
				rows[i].Tr[lang] = tr
			}
		}
	}
}

// previewAsset is the asset root and extensions the rows of o preview (12.8 `preview`).
func (b *builder) previewAsset(o check.Object, coll *types.Collection) *vm.SearchPreview {
	v, ok := b.index.ViewOf(coll.Elem.Base())
	if coll.Kind == types.CollDefines {
		v, ok = b.index.LetView(o)
	}
	if !ok {
		return nil
	}
	if p, isPreview := v.Item(syntax.KindViewPreview).(*syntax.ViewPreview); isPreview {
		return b.assetOf(b.in.Program.Info.Types[p.X])
	}
	return nil
}

// assetOf is the root and extensions of an asset type or its optional, nil for another type.
func (b *builder) assetOf(t types.Type) *vm.SearchPreview {
	if t == nil {
		return nil
	}
	if a := shape.LayersOf(shape.StripOptional(t)).Asset; a != nil {
		return &vm.SearchPreview{Root: b.roots.Root(a), Ext: a.Exts}
	}
	return nil
}

// otherLanguages are the project's languages but the source one.
func (b *builder) otherLanguages() []string {
	if len(b.in.Languages) == 0 {
		return nil
	}
	return b.in.Languages[1:]
}
