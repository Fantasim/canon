package wire_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

func field(name string, t types.Type, path ...string) *types.Field {
	if len(path) == 0 {
		path = []string{name}
	}
	return &types.Field{Name: name, Type: t, Wire: path[len(path)-1], WirePath: path}
}

func record(pkg, name string, fields ...*types.Field) *types.RecordType {
	for i, f := range fields {
		f.Index = i
	}
	return &types.RecordType{Pkg: pkg, Name: name, Fields: fields}
}

func enum(pkg, name string, wires ...string) *types.EnumType {
	e := &types.EnumType{Pkg: pkg, Name: name}
	for i, w := range wires {
		e.Members = append(e.Members, &types.Member{Name: w, Wire: w, Index: i})
	}
	return e
}

func codes(e *types.EnumType, codes ...int64) *types.EnumType {
	e.Codes = &types.UInt8Type
	for i, c := range codes {
		e.Members[i].Code, e.Members[i].HasCode = c, true
	}
	return e
}

func str(s string) *value.Str { return &value.Str{V: s, T: types.StringType} }

func num(i int64) *value.Int { return &value.Int{V: i, T: types.IntType} }

func flt(f float64) *value.Float { return &value.Float{V: f, T: types.FloatType} }

func boolean(b bool) *value.Bool { return &value.Bool{V: b} }

func dur(ms int64) *value.Dur { return &value.Dur{Ms: ms} }

func member(e *types.EnumType, i int) *value.Member { return &value.Member{Enum: e, Index: i} }

func none(t types.Type) *value.None { return &value.None{T: t} }

func rec(t types.Type, fields ...value.Value) *value.Record {
	return &value.Record{T: t, Fields: fields, Set: make([]bool, len(fields))}
}

func entry(coll *types.Collection, key string, retired bool, r *value.Record) *value.Record {
	r.Ident = &value.Identity{Coll: coll, Key: value.Key{S: key}, Retired: retired}
	return r
}

func list(t types.Type, elems ...value.Value) *value.List {
	return &value.List{T: &types.ListType{Elem: t}, Elems: elems}
}

func ref(coll *types.Collection, key string) *value.Ref {
	return &value.Ref{T: &types.RefType{Target: coll}, Key: value.Key{S: key}}
}

// encode encodes a document, failing the test on an error.
func encode(t *testing.T, d *wire.Document) string {
	t.Helper()
	b, err := d.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return string(b)
}

// row is the compact row n (from 0) of a rows document (WIRE.md §8.2: one row per line).
func row(t *testing.T, doc string, n int) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	const firstRow = 3
	if firstRow+n >= len(lines) {
		t.Fatalf("no row %d in\n%s", n, doc)
	}
	return strings.TrimSuffix(strings.TrimPrefix(lines[firstRow+n], "    "), ",")
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// methods is a Methods giving the records of type t one `$` key, computed by fn.
func methods(t types.Type, name string, fn func(r *value.Record) value.Value) wire.Methods {
	return func(r *value.Record) []wire.Fn {
		if r.T != t {
			return nil
		}
		return []wire.Fn{{Name: name, Result: fn(r)}}
	}
}
