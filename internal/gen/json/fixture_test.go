package jsongen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ftype is one type as the values carry it (types) and as the fingerprint reads it (ir).
type ftype struct {
	t  types.Type
	ir ir.TypeRef
}

var (
	tString   = ftype{types.StringType, ir.TypeRef{Kind: types.String}}
	tInt      = ftype{types.IntType, ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}}
	tBool     = ftype{types.BoolType, ir.TypeRef{Kind: types.Bool}}
	tDuration = ftype{types.DurationType, ir.TypeRef{Kind: types.Duration}}
)

func listOf(e ftype) ftype {
	return ftype{&types.ListType{Elem: e.t}, ir.TypeRef{Kind: types.List, Elem: &e.ir}}
}

// refTo is `ref` to a table or keyed list of String keys.
func refTo(coll *types.Collection) ftype {
	key := tString.ir
	return ftype{&types.RefType{Target: coll}, ir.TypeRef{Kind: types.Ref, Key: &key}}
}

// enumDef is an enum; members are "name" or "name=wire".
type enumDef struct {
	t  *types.EnumType
	ir *ir.Enum
}

func newEnum(pkg, name string, members ...string) enumDef {
	d := enumDef{t: &types.EnumType{Pkg: pkg, Name: name}, ir: &ir.Enum{Pkg: pkg, Name: name}}
	for i, m := range members {
		n, w, ok := strings.Cut(m, "=")
		if !ok {
			w = n
		}
		d.t.Members = append(d.t.Members, &types.Member{Name: n, Wire: w, Index: i})
		d.ir.Members = append(d.ir.Members, &ir.EnumMember{Name: n, Wire: w, Index: i})
	}
	return d
}

func (d enumDef) typ() ftype { return ftype{d.t, ir.TypeRef{Kind: types.Enum, Named: d.ir}} }

// m is the member called name; an unknown name has index -1, which wire refuses.
func (d enumDef) m(name string) *value.Member {
	for i, m := range d.t.Members {
		if m.Name == name {
			return &value.Member{Enum: d.t, Index: i}
		}
	}
	return &value.Member{Enum: d.t, Index: -1}
}

// all is every member in declaration order: an enum parameter's domain (WIRE.md §5.11).
func (d enumDef) all() []value.Value {
	out := make([]value.Value, len(d.t.Members))
	for i := range d.t.Members {
		out[i] = &value.Member{Enum: d.t, Index: i}
	}
	return out
}

// fdef is a field; wire is its key (its name when empty).
type fdef struct {
	name, wire string
	typ        ftype
	optional   bool
}

func fld(name string, t ftype) fdef { return fdef{name: name, typ: t} }

func (f fdef) at(wire string) fdef { f.wire = wire; return f }

func (f fdef) opt() fdef { f.optional = true; return f }

// recDef is a record type.
type recDef struct {
	t  *types.RecordType
	ir *ir.Record
}

func newRecord(pkg, name string, fields ...fdef) recDef {
	d := recDef{t: &types.RecordType{Pkg: pkg, Name: name}, ir: &ir.Record{Pkg: pkg, Name: name}}
	for i, f := range fields {
		if f.wire == "" {
			f.wire = f.name
		}
		t := f.typ.t
		if f.optional {
			t = &types.OptionalType{Elem: t}
		}
		path := []string{f.wire}
		d.t.Fields = append(d.t.Fields, &types.Field{Name: f.name, Index: i, Type: t, Wire: f.wire, WirePath: path})
		d.ir.Fields = append(d.ir.Fields, &ir.Field{Name: f.name, WirePath: path, Type: f.typ.ir, Optional: f.optional})
	}
	return d
}

func (d recDef) typ() ftype { return ftype{d.t, ir.TypeRef{Kind: types.Record, Named: d.ir}} }

func (d recDef) rec(fields ...value.Value) *value.Record {
	return &value.Record{T: d.t, Fields: fields, Set: make([]bool, len(fields))}
}

// method adds a precomputed method to the record, with one result per receiver.
func (d recDef) method(name string, result ftype, recvs []*value.Record, fn func(r *value.Record) value.Value) *ir.ExportFn {
	m := &ir.ExportFn{Name: name, Kind: ir.FnPrecomputed, Result: result.ir}
	for _, r := range recvs {
		m.Instances = append(m.Instances, &ir.Instance{Recv: r, Result: fn(r)})
	}
	d.ir.Methods = append(d.ir.Methods, m)
	return m
}

// tableDef is a `stable table` let whose record type is built after it, for self refs.
type tableDef struct {
	coll *types.Collection
	rec  recDef
}

func newTable(pkg, name string) *tableDef {
	return &tableDef{coll: &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: name}}
}

func (t *tableDef) of(r recDef) *tableDef {
	t.rec, t.coll.Elem = r, r.t
	return t
}

func (t *tableDef) entry(key string, fields ...value.Value) *value.Record {
	r := t.rec.rec(fields...)
	r.Ident = &value.Identity{Coll: t.coll, Key: value.Key{S: key}}
	return r
}

func (t *tableDef) ref(key string) *value.Ref {
	return &value.Ref{T: &types.RefType{Target: t.coll}, Key: value.Key{S: key}}
}

func (t *tableDef) refs(keys ...string) *value.List {
	l := &value.List{T: &types.ListType{Elem: &types.RefType{Target: t.coll}}}
	for _, k := range keys {
		l.Elems = append(l.Elems, t.ref(k))
	}
	return l
}

// value is the table's IR value; its domain as a ref parameter is every entry, in order.
func (t *tableDef) value(entries ...*value.Record) *ir.Value {
	elem := t.rec.typ().ir
	tbl := &value.Table{T: &types.TableType{Elem: t.rec.t, Stable: true}, Entries: entries}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.Ident.Key.S
	}
	return &ir.Value{Name: t.coll.Name, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: tbl, IDs: ids}
}

func str(s string) *value.Str { return &value.Str{V: s, T: types.StringType} }

func num(i int64) *value.Int { return &value.Int{V: i, T: types.IntType} }

func boolean(b bool) *value.Bool { return &value.Bool{V: b} }

func dur(ms int64) *value.Dur { return &value.Dur{Ms: ms} }

func none(t ftype) *value.None { return &value.None{T: &types.OptionalType{Elem: t.t}} }

func list(t ftype, elems ...value.Value) *value.List {
	return &value.List{T: &types.ListType{Elem: t.t}, Elems: elems}
}

// jsonEmit is a directory-mode json emit.
func jsonEmit(dir string, values ...string) *ir.Emit {
	return &ir.Emit{Target: ir.TargetJSON, Out: dir, Dir: dir, Values: values}
}
