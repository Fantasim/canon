package gogen_test

import (
	"encoding/json"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// value builds an emitted value; a table's entries carry their identity (TYP-02).
func (w *world) value(pkg string, s valueSpec) *ir.Value {
	t, opt := w.typeRef(s.Type)
	v := &ir.Value{Name: s.Name, Doc: s.Doc, Type: t, Schema: pkg + "." + s.Name + "@00000000"}
	if opt {
		v.Type = ir.TypeRef{Kind: types.Optional, Elem: &t}
	}
	coll := &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: s.Name}
	switch {
	case t.Kind == types.Table:
		tbl := &value.Table{}
		for _, e := range s.Entries {
			raw, err := json.Marshal(e.Fields)
			if err != nil {
				w.t.Fatal(err)
			}
			id := &value.Identity{Coll: coll, Key: value.Key{S: e.ID}, Retired: e.Retired}
			tbl.Entries = append(tbl.Entries, w.record(t.Elem.Named.(*ir.Record), raw, id))
			v.IDs = append(v.IDs, e.ID)
		}
		v.V = tbl
	default:
		v.V = w.toValue(t, opt, s.Value)
		if l, ok := v.V.(*value.List); ok && t.KeyedBy != nil {
			for _, x := range l.Elems {
				x.(*value.Record).Ident.Coll = coll
			}
		}
	}
	w.built[pkg+"."+s.Name] = v
	return v
}

// fn builds an export fn: no parameter is precomputed, finite parameters make a lookup.
func (w *world) fn(s fnSpec, recv *ir.Record) *ir.ExportFn {
	fn := &ir.ExportFn{Name: s.Name, Doc: s.Doc, Kind: ir.FnPrecomputed}
	for _, p := range s.Params {
		t, _ := w.typeRef(p.Type)
		fn.Params = append(fn.Params, &ir.Param{Name: p.Name, Type: t})
		fn.Kind = ir.FnLookup
	}
	t, opt := w.typeRef(s.Result)
	fn.Result = t
	if opt {
		fn.Result = ir.TypeRef{Kind: types.Optional, Elem: &t}
	}
	switch {
	case recv != nil:
	case fn.Kind == ir.FnPrecomputed:
		fn.Value = w.toValue(t, opt, s.Value)
	case s.Cells != nil:
		fn.Table = w.table(fn, w.rawCells(fn, s.Cells))
	default:
		fn.Table = w.table(fn, w.cells(w, fn))
	}
	return fn
}

func (w *world) rawCells(fn *ir.ExportFn, raws []json.RawMessage) []value.Value {
	t, opt := unwrap(fn.Result)
	out := make([]value.Value, len(raws))
	for i, raw := range raws {
		out[i] = w.toValue(t, opt, raw)
	}
	return out
}

// table is a dense lookup table with each parameter's domain in domain order (CG-08).
func (w *world) table(fn *ir.ExportFn, cells []value.Value) *ir.LookupTable {
	t := &ir.LookupTable{Cells: cells}
	for _, p := range fn.Params {
		t.Domains = append(t.Domains, w.domain(p.Type))
	}
	return t
}

func (w *world) domain(t ir.TypeRef) []value.Value {
	var d []value.Value
	switch t.Kind {
	case types.Bool:
		d = []value.Value{&value.Bool{V: false}, &value.Bool{V: true}}
	case types.Enum:
		e := t.Named.(*ir.Enum)
		for i := range e.Members {
			d = append(d, &value.Member{Enum: w.types[e.Name].enum, Index: i})
		}
	case types.Ref:
		for _, id := range w.built[t.Ref.Pkg+"."+t.Ref.Value].IDs {
			d = append(d, &value.Ref{Key: value.Key{S: id}})
		}
	}
	return d
}

// entries is the rows of a built table value.
func (w *world) entries(qname string) []*value.Record {
	return w.built[qname].V.(*value.Table).Entries
}

// field is a record value's field by name.
func field(r *value.Record, name string) value.Value {
	for i, f := range r.T.(*types.RecordType).Fields {
		if f.Name == name {
			return r.Fields[i]
		}
	}
	return nil
}

// refKeys is the keys of a [ref T] value.
func refKeys(v value.Value) []string {
	var keys []string
	for _, x := range v.(*value.List).Elems {
		keys = append(keys, x.(*value.Ref).Key.S)
	}
	return keys
}

func refOf(key string) *value.Ref { return &value.Ref{Key: value.Key{S: key}} }
