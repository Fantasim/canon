package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalBrace is a brace literal, as the checker classified it (TYPES.md §5.2).
func evalBrace(r *run, e syntax.Expr, at *vpath) value.Value {
	lit := e.(*syntax.BraceLit)
	switch r.ev.info.Literals[lit] {
	case check.LitRecord:
		return r.recordLit(lit, lit, r.typeOf(lit), nil, at)
	case check.LitTable:
		return r.tableLit(lit, at)
	case check.LitMap:
		return r.mapLit(lit, at)
	case check.LitMapComp:
		return r.mapComp(lit)
	default:
	}
	r.bug(e)
	return nil
}

// evalTyped is `Name { … }`: one node, located at its name (EVALUATION.md §8.3).
func evalTyped(r *run, e syntax.Expr, at *vpath) value.Value {
	x := e.(*syntax.TypedLit)
	return r.recordLit(x.Lit, x, r.typeOf(x.Lit), nil, at)
}

// recordLit builds a record or case instance, an application R(args) binding its arguments (TYPES.md §11.1).
func (r *run) recordLit(lit *syntax.BraceLit, loc syntax.Node, t types.Type, ident *value.Identity, at *vpath) value.Value {
	fields := fieldsOf(t)
	r.noteType(t)
	rec := &value.Record{
		T: t.Base(), Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields)),
		Ident: ident, P: r.prov(loc, value.ProvLiteral),
	}
	outer := r.dep
	if !r.applyRecord(rec, t, outer) {
		return nil
	}
	r.dep = &depCtx{rec: rec, params: r.ev.boundParams(rec), at: loc}
	defer func() { r.dep = outer }()
	b := newBuilding(rec, lit, at)
	for _, it := range lit.Items {
		if !r.recordItem(b, it) {
			return nil
		}
	}
	if !r.defaults(rec, b.given, at) {
		return nil
	}
	r.ev.bindRefs(rec)
	return rec
}

// building is a record literal being evaluated: its written fields by index, the fields that
// have their value (written, spread or defaulted early) and the written ones evaluated.
type building struct {
	rec   *value.Record
	items []*syntax.FieldItem
	given []bool
	done  []bool
	at    *vpath
}

// newBuilding indexes the written fields only of a record some field of which reads earlier ones.
func newBuilding(rec *value.Record, lit *syntax.BraceLit, at *vpath) *building {
	n := len(rec.Fields)
	b := &building{rec: rec, given: make([]bool, n), done: make([]bool, n), at: at}
	if !slices.ContainsFunc(fieldsOf(rec.T), func(f *types.Field) bool { return len(f.DependsOn) > 0 }) {
		return b
	}
	b.items = make([]*syntax.FieldItem, n)
	for _, it := range lit.Items {
		if fi, ok := it.(*syntax.FieldItem); ok {
			if i := fieldIndex(rec.T, fi.Name.Name); i >= 0 {
				b.items[i] = fi
			}
		}
	}
	return b
}

// recordItem evaluates a spread or a field of a record literal into its record.
func (r *run) recordItem(b *building, it syntax.BraceItem) bool {
	rec := b.rec
	switch it := it.(type) {
	case *syntax.SpreadItem:
		src, ok := r.eval(it.X).(*value.Record)
		if !ok || len(src.Fields) != len(rec.Fields) {
			r.bug(it)
			return false
		}
		for i, f := range src.Fields {
			rec.Fields[i], rec.Set[i], b.given[i] = r.ev.spreadCopy(f, r.prov(it, value.ProvSpread)), src.Set[i], true
		}
	case *syntax.FieldItem:
		i := fieldIndex(rec.T, it.Name.Name)
		if i < 0 {
			r.bug(it)
			return false
		}
		return b.done[i] || r.fieldItem(b, i, it)
	}
	return true
}

// fieldItem evaluates written field i, after the earlier fields its applied record type reads.
func (r *run) fieldItem(b *building, i int, it *syntax.FieldItem) bool {
	b.done[i] = true
	rec := b.rec
	f := fieldsOf(rec.T)[i]
	if !r.pullDeps(b, f) {
		return false
	}
	fat, outer := b.at.field(f.Name), r.dep
	r.dep = outer.forField(f, it.Value)
	v := r.evalAt(it.Value, fat)
	r.dep = outer
	v = r.store(v, f.Type, r.ev.fieldSite(f, rec.T), fat)
	if v == nil {
		return false
	}
	rec.Fields[i], rec.Set[i], b.given[i] = r.ev.inField(v, rec, f), true, true
	b.at.learn(f, v)
	return true
}

// pullDeps evaluates first each earlier field f's type reads, written or defaulted (TYPES.md §11.1).
func (r *run) pullDeps(b *building, f *types.Field) bool {
	for _, j := range f.DependsOn {
		if !r.settleField(b, j) {
			return false
		}
	}
	return true
}

// settleField gives field j its value now: its written item, else its default once every earlier
// field has one, since a default reads earlier fields.
func (r *run) settleField(b *building, j int) bool {
	switch {
	case b.items[j] != nil:
		return b.done[j] || r.fieldItem(b, j, b.items[j])
	case b.given[j]:
		return true
	}
	for k := range j {
		if !r.settleField(b, k) {
			return false
		}
	}
	b.given[j] = true
	return r.fill(b.rec, j, b.at)
}

// spreadCopy is a field a spread copies (EVALUATION.md §4.2, §13).
func (e *Evaluator) spreadCopy(v value.Value, p *value.Prov) value.Value {
	if v == nil {
		return nil
	}
	p.Via = v.Prov()
	return e.reprov(v, p, false)
}

// defaults fills the fields no item gave (TYPES.md §15).
func (r *run) defaults(rec *value.Record, given []bool, at *vpath) bool {
	for i := range rec.Fields {
		if !given[i] && !r.fill(rec, i, at) {
			return false
		}
	}
	return true
}

// fill gives field i of rec its default, none for an optional without one; an input field stays nil.
func (r *run) fill(rec *value.Record, i int, at *vpath) bool {
	f := fieldsOf(rec.T)[i]
	switch {
	case f.Input != nil:
		return true
	case f.Default == nil:
		rec.Fields[i] = &value.None{T: f.Type, P: r.ev.leftOut(f, rec.T, rec.P)}
		return true
	}
	v := r.defaultValue(rec, f, at.field(f.Name), rec.P)
	if v == nil {
		return false
	}
	rec.Fields[i] = r.ev.inField(v, rec, f)
	at.learn(f, v)
	return true
}

// leftOut is an omitted optional without default: at its declaration, via what omitted it (EVALUATION.md §13).
func (e *Evaluator) leftOut(f *types.Field, t types.Type, via *value.Prov) *value.Prov {
	return &value.Prov{Kind: value.ProvDefault, Span: e.fieldSite(f, t).decl, Via: via}
}

// defaultValue evaluates f's default for rec in its implicit frame, `default` via via (EVALUATION.md §13).
func (r *run) defaultValue(rec *value.Record, f *types.Field, at *vpath, via *value.Prov) value.Value {
	saved, dep := r.fr, r.dep
	r.fr = (&frame{vars: map[check.Object]value.Value{}, self: rec, file: r.ev.declFile(rec.T), decl: true}).under(saved)
	r.fr.pkg = r.ev.index.pkg[r.fr.file]
	r.noteCode(r.fr.file)
	r.dep = &depCtx{rec: rec, params: r.ev.boundParams(rec), field: f.Type, scope: newFieldScope(f, f.Default), at: f.Default}
	defer func() { r.fr, r.dep = saved, dep }()
	if !r.nest(r.span(f.Default)) {
		return nil
	}
	v := r.evalAt(f.Default, at)
	r.unnest()
	if v != nil {
		p := &value.Prov{Kind: value.ProvDefault, Span: r.span(f.Default), Via: via}
		v = r.store(r.ev.reprov(v, p, true), f.Type, r.ev.fieldSite(f, rec.T), at)
	}
	return v
}

// bareCase is a case written without fields: its defaults (TYPES.md §8.2).
func (r *run) bareCase(ct *types.CaseType, at syntax.Expr) value.Value {
	return r.recordLit(&syntax.BraceLit{}, at, ct, nil, nil)
}

// declFile is the file declaring a record or a case's variant.
func (e *Evaluator) declFile(t types.Type) *syntax.File {
	n, name := typeDecl(t)
	if name == nil {
		return nil
	}
	if f := e.fileOf(n); f != nil || e.info == nil {
		return f
	}
	if obj := e.info.Defs[name]; obj != nil { // a fold's evaluator indexes only the files it meets
		e.index.add(obj.File(), obj.Pkg())
	}
	return e.fileOf(n)
}

// typeDecl is the declaration of a record or a case's variant, and its name; none for another type.
func typeDecl(t types.Type) (syntax.Node, *syntax.Ident) {
	switch x := t.Base().(type) {
	case *types.RecordType:
		if x.Decl != nil {
			return x.Decl, x.Decl.Name
		}
	case *types.AppliedRecord:
		return typeDecl(x.Rec)
	case *types.CaseType:
		if x.Variant != nil && x.Variant.Decl != nil {
			return x.Variant.Decl, x.Variant.Decl.Name
		}
	}
	return nil, nil
}
