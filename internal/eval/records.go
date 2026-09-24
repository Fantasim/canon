package eval

import (
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

// recordLit builds a record or case instance (EVALUATION.md §2.2, §4.2).
func (r *run) recordLit(lit *syntax.BraceLit, loc syntax.Node, t types.Type, ident *value.Identity, at *vpath) value.Value {
	fields := fieldsOf(t)
	rec := &value.Record{
		T: t.Base(), Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields)),
		Ident: ident, P: r.prov(loc, value.ProvLiteral),
	}
	given := make([]bool, len(fields))
	for _, it := range lit.Items {
		if !r.recordItem(rec, it, given, at) {
			return nil
		}
	}
	if !r.defaults(rec, given, at) {
		return nil
	}
	r.ev.bindRefs(rec)
	return rec
}

// recordItem evaluates a spread or a field of a record literal into rec.
func (r *run) recordItem(rec *value.Record, it syntax.BraceItem, given []bool, at *vpath) bool {
	switch it := it.(type) {
	case *syntax.SpreadItem:
		src, ok := r.eval(it.X).(*value.Record)
		if !ok || len(src.Fields) != len(rec.Fields) {
			r.bug(it)
			return false
		}
		for i, f := range src.Fields {
			rec.Fields[i], rec.Set[i], given[i] = r.ev.spreadCopy(f, r.prov(it, value.ProvSpread)), src.Set[i], true
		}
	case *syntax.FieldItem:
		i := fieldIndex(rec.T, it.Name.Name)
		if i < 0 {
			r.bug(it)
			return false
		}
		f := fieldsOf(rec.T)[i]
		fat := at.field(f.Name)
		v := r.store(r.evalAt(it.Value, fat), f.Type, r.ev.fieldSite(f, rec.T), fat)
		if v == nil {
			return false
		}
		rec.Fields[i], rec.Set[i], given[i] = r.ev.inField(v, rec, f), true, true
	}
	return true
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
	for i, f := range fieldsOf(rec.T) {
		switch {
		case given[i] || f.Input != nil:
			continue
		case f.Default == nil:
			rec.Fields[i] = &value.None{T: f.Type, P: rec.P}
			continue
		}
		v := r.defaultValue(rec, f, at.field(f.Name))
		if v == nil {
			return false
		}
		rec.Fields[i] = r.ev.inField(v, rec, f)
	}
	return true
}

// defaultValue evaluates a field default for rec in its implicit frame (EVALUATION.md §13, DECISIONS 210).
func (r *run) defaultValue(rec *value.Record, f *types.Field, at *vpath) value.Value {
	saved := r.fr
	r.fr = (&frame{vars: map[check.Object]value.Value{}, self: rec, file: r.ev.declFile(rec.T)}).under(saved)
	r.fr.pkg = r.ev.index.pkg[r.fr.file]
	if !r.nest(r.span(f.Default)) {
		r.fr = saved
		return nil
	}
	v := r.eval(f.Default)
	r.unnest()
	var p *value.Prov
	if v != nil {
		p = &value.Prov{Kind: value.ProvDefault, Span: r.span(f.Default), Via: rec.P}
		v = r.store(r.ev.reprov(v, p, true), f.Type, r.ev.fieldSite(f, rec.T), at)
	}
	r.fr = saved
	return v
}

// bareCase is a case written without fields: its defaults (TYPES.md §8.2).
func (r *run) bareCase(ct *types.CaseType, at syntax.Expr) value.Value {
	return r.recordLit(&syntax.BraceLit{}, at, ct, nil, nil)
}

// declFile is the file declaring a record or a case's variant.
func (e *Evaluator) declFile(t types.Type) *syntax.File {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return e.index.file[x.Decl]
	case *types.AppliedRecord:
		return e.index.file[x.Rec.Decl]
	case *types.CaseType:
		return e.index.file[x.Variant.Decl]
	}
	return nil
}
