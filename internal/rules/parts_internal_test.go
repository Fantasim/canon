package rules

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// EVALUATION.md §8.1: eachPart meets the parts namedParts names, in its order, and stops when fn says so.
func TestEachPartMatchesNamedParts(t *testing.T) {
	row := &types.RecordType{Name: "Row", Fields: []*types.Field{{Name: "a", Type: types.IntType}, {Name: "b", Type: types.IntType, Index: 1}}}
	coll := &types.Collection{Kind: types.CollLet, Name: "rows", Elem: row}
	num := func(n int64) value.Value { return &value.Int{V: n, T: types.IntType} }
	rec := func(a, b int64, key string) *value.Record {
		r := &value.Record{T: row, Fields: []value.Value{num(a), num(b)}}
		if key != "" {
			r.Ident = &value.Identity{Coll: coll, Key: value.Key{S: key}}
		}
		return r
	}
	short := &value.Record{T: row, Fields: []value.Value{num(9)}}
	tbl := &value.Table{T: &types.TableType{Elem: row}, Entries: []*value.Record{rec(1, 2, "x"), nil, rec(3, 4, ""), rec(5, 6, "y")}}
	mp := &value.Map{T: &types.MapType{Key: types.StringType, Value: row}, Keys: []value.Value{&value.Str{V: "k"}, &value.Str{V: "l"}}, Vals: []value.Value{rec(7, 8, "")}}
	values := []value.Value{
		rec(1, 2, ""), short, tbl, mp,
		&value.List{T: &types.ListType{Elem: row}, Elems: []value.Value{rec(1, 1, ""), short}},
		&value.Pair{A: num(1), B: num(2)}, num(3), nil,
	}
	for i, v := range values {
		var named, plain []value.Value
		namedParts(v, nil, nil, func(p part) bool { named = append(named, p.v); return true })
		eachPart(v, func(p value.Value) bool { plain = append(plain, p); return true })
		if !slices.Equal(named, plain) {
			t.Errorf("value %d: eachPart %v, namedParts %v", i, plain, named)
		}
		n := 0
		eachPart(v, func(value.Value) bool { n++; return false })
		if n > 1 {
			t.Errorf("value %d: eachPart went on after fn returned false (%d calls)", i, n)
		}
	}
}
