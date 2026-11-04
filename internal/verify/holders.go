package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// relatedTo relates holder, the declaration the value is stored into, else the written type t (API.md F12).
func (s *sources) relatedTo(b *diag.Builder, holder source.Span, t types.Type) *diag.Builder {
	if span, ok := s.holderOr(holder, t); ok {
		b.Related(span, diag.NoteSource(span))
	}
	return b
}

// holderOr is holder when the value has one, else where t is written; false: neither is known.
func (s *sources) holderOr(holder source.Span, t types.Type) (source.Span, bool) {
	if holder != (source.Span{}) {
		return holder, true
	}
	w, ok := s.types[t]
	return w.decl, ok
}

// depHolder is what a dependent value's conversion relates: its holder, else where its application is written.
func (s *sources) depHolder(d *depSite) source.Span {
	span, _ := s.holderOr(d.holder, d.app)
	return span
}

// rootHolder is the declaration the walked value is stored into: its let's, none for a precomputed result or a literal.
func (w *walker) rootHolder() source.Span {
	if w.result != nil {
		return source.Span{}
	}
	return w.src.lets[w.root]
}

// letHolders is the declaration `name: Type` of each top-level let with a written type.
func letHolders(prog *check.Program) map[eval.Root]source.Span {
	out := map[eval.Root]source.Span{}
	if prog == nil {
		return out
	}
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if d, ok := obj.Decl().(*syntax.LetDecl); ok && d.Type != nil && obj.File() != nil {
				out[eval.Root{Pkg: pkg.Path, Name: obj.Name()}] = declSpan(obj.File(), d.Name, d.Type)
			}
		}
	}
	return out
}

// addHolders indexes the declaration of each field of the records and variant cases f declares.
func (s *sources) addHolders(f *syntax.File, fs *fileSyntax, info *check.Info) {
	for _, d := range fs.records {
		if rt := recordOf(declaredType(info, d.Name)); rt != nil {
			s.addFields(f, rt.Fields, d.Body)
		}
	}
	for _, d := range fs.decls {
		if vd, ok := d.(*syntax.VariantDecl); ok {
			s.addCases(f, declaredType(info, vd.Name), vd)
		}
	}
}

// declaredType is the type a declaration's name declares, nil when unknown.
func declaredType(info *check.Info, name *syntax.Ident) types.Type {
	if obj := info.Defs[name]; obj != nil {
		return obj.Type()
	}
	return nil
}

// addCases indexes the fields of each case of variant vd, of type t.
func (s *sources) addCases(f *syntax.File, t types.Type, vd *syntax.VariantDecl) {
	vt, ok := t.(*types.VariantType)
	if !ok {
		return
	}
	for _, it := range vd.Items {
		c, isCase := it.(*syntax.VariantCase)
		if !isCase || c.Name == nil {
			continue
		}
		if i := slices.IndexFunc(vt.Cases, func(ct *types.CaseType) bool { return ct.Name == c.Name.Name }); i >= 0 {
			s.addFields(f, vt.Cases[i].Fields, c.Body)
		}
	}
}

// addFields indexes the declaration `name: Type` of each field body declares, as fields holds it.
func (s *sources) addFields(f *syntax.File, fields []*types.Field, body *syntax.RecordBody) {
	if body == nil {
		return
	}
	for _, it := range body.Items {
		d, ok := it.(*syntax.FieldDecl)
		if !ok || d.Name == nil || d.Type == nil {
			continue
		}
		if i := slices.IndexFunc(fields, func(tf *types.Field) bool { return tf.Name == d.Name.Name }); i >= 0 {
			s.fields[fields[i]] = declSpan(f, d.Name, d.Type)
		}
	}
}
