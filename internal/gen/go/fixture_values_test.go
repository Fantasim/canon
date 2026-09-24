package gogen_test

import (
	"encoding/json"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var basics = map[string]ir.TypeRef{
	"Bool": {Kind: types.Bool}, "String": {Kind: types.String}, "Duration": {Kind: types.Duration},
	"Int": {Kind: types.Int, Bits: 64, Signed: true}, "Int8": {Kind: types.Int, Bits: 8, Signed: true},
	"Int16": {Kind: types.Int, Bits: 16, Signed: true}, "UInt8": {Kind: types.Int, Bits: 8},
	"UInt16": {Kind: types.Int, Bits: 16}, "UInt64": {Kind: types.Int, Bits: 64},
	"Float": {Kind: types.Float, Bits: 64}, "Float32": {Kind: types.Float, Bits: 32},
	"Never": {Kind: types.Never},
}

// typeRef reads the fixture type syntax: T?, [T], [T keyed by f], {K: V}, table T, ref v,
// a basic type or a declared name.
func (w *world) typeRef(s string) (ir.TypeRef, bool) {
	if strings.HasSuffix(s, "?") {
		t, _ := w.typeRef(strings.TrimSuffix(s, "?"))
		return t, true
	}
	switch {
	case strings.HasPrefix(s, "[") && strings.Contains(s, " keyed by "):
		elem, key, _ := strings.Cut(strings.Trim(s, "[]"), " keyed by ")
		e, _ := w.typeRef(elem)
		return ir.TypeRef{Kind: types.List, Elem: &e, KeyedBy: &ir.KeyField{Name: key, WirePath: []string{key}}}, false
	case strings.HasPrefix(s, "["):
		e, _ := w.typeRef(s[1 : len(s)-1])
		return ir.TypeRef{Kind: types.List, Elem: &e}, false
	case strings.HasPrefix(s, "{"):
		k, v, _ := strings.Cut(s[1:len(s)-1], ": ")
		kt, _ := w.typeRef(k)
		vt, _ := w.typeRef(v)
		return ir.TypeRef{Kind: types.Map, Key: &kt, Elem: &vt}, false
	case strings.HasPrefix(s, "table "):
		e, _ := w.typeRef(strings.TrimPrefix(s, "table "))
		return ir.TypeRef{Kind: types.Table, Elem: &e}, false
	case strings.HasPrefix(s, "ref "):
		return w.refType(strings.TrimPrefix(s, "ref ")), false
	}
	if b, ok := basics[s]; ok {
		return b, false
	}
	n := w.types[s]
	if n == nil {
		w.t.Fatalf("fixture: unknown type %q", s)
	}
	kind := map[bool]types.Kind{true: types.Enum, false: types.Record}[n.enum != nil]
	if n.variant != nil {
		kind = types.Variant
	}
	return ir.TypeRef{Kind: kind, Named: n.ir}, false
}

// refType is a ref into a table or keyed list value of the current package.
func (w *world) refType(target string) ir.TypeRef {
	vs := w.values[w.cur.Name+"."+target]
	if vs == nil {
		w.t.Fatalf("fixture: ref into unknown value %q", target)
	}
	coll, _ := w.typeRef(vs.Type)
	r := &ir.RefTarget{Coll: types.CollLet, Pkg: w.cur.Name, Value: target, Elem: coll.Elem.Named}
	key := basics["String"]
	if coll.KeyedBy != nil {
		r.Keyed = true
		for _, f := range coll.Elem.Named.(*ir.Record).Fields {
			if f.Name == coll.KeyedBy.Name {
				key = f.Type
			}
		}
	}
	return ir.TypeRef{Kind: types.Ref, Ref: r, Key: &key}
}

// toValue converts fixture JSON to a value of t; absent is none for an optional.
func (w *world) toValue(t ir.TypeRef, opt bool, raw json.RawMessage) value.Value {
	if len(raw) == 0 || string(raw) == "null" {
		if !opt {
			w.t.Fatalf("fixture: missing value of kind %d", t.Kind)
		}
		return &value.None{}
	}
	switch t.Kind {
	case types.Bool:
		var b bool
		w.decode(raw, &b)
		return &value.Bool{V: b}
	case types.Int:
		var i int64
		w.decode(raw, &i)
		return &value.Int{V: i, T: types.IntType}
	case types.Float:
		var f float64
		w.decode(raw, &f)
		return &value.Float{V: f, T: types.FloatType}
	case types.Duration:
		var ms int64
		w.decode(raw, &ms)
		return &value.Dur{Ms: ms}
	case types.String:
		var s string
		w.decode(raw, &s)
		return &value.Str{V: s, T: types.StringType}
	case types.Enum:
		return w.member(t, raw)
	case types.Ref:
		var key string
		w.decode(raw, &key)
		return &value.Ref{Key: value.Key{S: key}}
	case types.List:
		return w.list(t, raw)
	case types.Map:
		return w.mapValue(t, raw)
	case types.Record:
		return w.record(t.Named.(*ir.Record), raw, nil)
	case types.Variant:
		return w.variantValue(t.Named.(*ir.Variant), raw)
	}
	w.t.Fatalf("fixture: no value of kind %d", t.Kind)
	return nil
}

func (w *world) decode(raw json.RawMessage, into any) {
	if err := json.Unmarshal(raw, into); err != nil {
		w.t.Fatalf("fixture: %s: %v", raw, err)
	}
}

func (w *world) member(t ir.TypeRef, raw json.RawMessage) value.Value {
	var name string
	w.decode(raw, &name)
	e := t.Named.(*ir.Enum)
	for i, m := range e.Members {
		if m.Name == name {
			return &value.Member{Enum: w.types[e.Name].enum, Index: i}
		}
	}
	w.t.Fatalf("fixture: enum %s has no %s", e.Name, name)
	return nil
}

func (w *world) list(t ir.TypeRef, raw json.RawMessage) value.Value {
	var items []json.RawMessage
	w.decode(raw, &items)
	l := &value.List{}
	for _, it := range items {
		x := w.toValue(*t.Elem, false, it)
		if t.KeyedBy != nil {
			r := x.(*value.Record)
			r.Ident = &value.Identity{Key: w.key(r, t.KeyedBy.Name)}
		}
		l.Elems = append(l.Elems, x)
	}
	return l
}

func (w *world) key(r *value.Record, field string) value.Key {
	for i, f := range r.T.(*types.RecordType).Fields {
		if f.Name != field {
			continue
		}
		switch k := r.Fields[i].(type) {
		case *value.Int:
			return value.Key{I: k.V, IsInt: true}
		case *value.Str:
			return value.Key{S: k.V}
		}
	}
	w.t.Fatalf("fixture: no key %s", field)
	return value.Key{}
}

func (w *world) mapValue(t ir.TypeRef, raw json.RawMessage) value.Value {
	var pairs [][2]json.RawMessage
	w.decode(raw, &pairs)
	m := &value.Map{}
	for _, p := range pairs {
		m.Keys = append(m.Keys, w.toValue(*t.Key, false, p[0]))
		m.Vals = append(m.Vals, w.toValue(*t.Elem, false, p[1]))
	}
	return m
}

// record converts a JSON object; "$fns" holds its precomputed export fn results.
func (w *world) record(rec *ir.Record, raw json.RawMessage, ident *value.Identity) *value.Record {
	var obj map[string]json.RawMessage
	w.decode(raw, &obj)
	r := &value.Record{T: w.types[rec.Name].rec, Ident: ident}
	r.Fields = w.fieldValues(rec.Fields, obj)
	var fns map[string]json.RawMessage
	if raw, ok := obj["$fns"]; ok {
		w.decode(raw, &fns)
	}
	for _, fn := range rec.Methods {
		in := &ir.Instance{Recv: r}
		if fn.Kind == ir.FnLookup {
			var cells []json.RawMessage
			w.decode(fns[fn.Name], &cells)
			in.Table = w.table(fn, w.rawCells(fn, cells))
		} else {
			t, opt := unwrap(fn.Result)
			in.Result = w.toValue(t, opt, fns[fn.Name])
		}
		fn.Instances = append(fn.Instances, in)
	}
	return r
}

func (w *world) fieldValues(fields []*ir.Field, obj map[string]json.RawMessage) []value.Value {
	var out []value.Value
	for _, f := range fields {
		raw, ok := obj[f.Name]
		if !ok {
			raw = w.fieldDefault(f)
		}
		out = append(out, w.toValue(f.Type, f.Optional, raw))
	}
	return out
}

// fieldDefault is the default a fixture declares for the field, if any.
func (w *world) fieldDefault(f *ir.Field) json.RawMessage {
	var specs []fieldSpec
	for _, r := range w.cur.Records {
		specs = append(specs, r.Fields...)
	}
	for _, v := range w.cur.Variants {
		for _, c := range v.Cases {
			specs = append(specs, c.Fields...)
		}
	}
	for _, s := range specs {
		if s.Name == f.Name && s.Doc == f.Doc {
			return s.Default
		}
	}
	return nil
}

func (w *world) variantValue(v *ir.Variant, raw json.RawMessage) value.Value {
	var obj struct {
		Case   string
		Fields map[string]json.RawMessage
	}
	w.decode(raw, &obj)
	n := w.types[v.Name]
	for i, c := range v.Cases {
		if c.Name == obj.Case {
			return &value.Record{T: n.variant.Cases[i], Fields: w.fieldValues(c.Fields, obj.Fields)}
		}
	}
	w.t.Fatalf("fixture: variant %s has no %s", v.Name, obj.Case)
	return nil
}

func unwrap(t ir.TypeRef) (ir.TypeRef, bool) {
	if t.Kind == types.Optional {
		return *t.Elem, true
	}
	return t, false
}
