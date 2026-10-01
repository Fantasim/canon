package edit

import (
	"reflect"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// valueDiff collects the paths where two values of a root differ, as M1 compares them (API.md
// 9.2): records by field, tables and maps by key, lists of one length by position, else whole.
type valueDiff struct {
	out     []string
	ordered func(path string) bool                     // whether the order of the table or map at path is its value's
	placed  func(path string, x, y *value.Record) bool // whether entry y lies where x did (E22)
}

// value compares x and y, the values at path; equal tables still have their entries' files compared.
func (d *valueDiff) value(path string, x, y value.Value) {
	if _, isTable := x.(*value.Table); sameText(x, y) && (!isTable || d.placed == nil) {
		return
	}
	switch a := x.(type) {
	case *value.Record:
		if b, ok := y.(*value.Record); ok && a.T.String() == b.T.String() && len(a.Fields) == len(b.Fields) {
			d.record(path, a, b)
			return
		}
	case *value.Table:
		if b, ok := y.(*value.Table); ok {
			d.table(path, a, b)
			return
		}
	case *value.Map:
		if b, ok := y.(*value.Map); ok {
			d.mapValue(path, a, b)
			return
		}
	case *value.List:
		if b, ok := y.(*value.List); ok && len(a.Elems) == len(b.Elems) {
			d.list(path, a, b)
			return
		}
	}
	d.out = append(d.out, path)
}

func (d *valueDiff) record(path string, a, b *value.Record) {
	for i, f := range fieldsOf(a.T) {
		d.value(childPath(path, Seg{Kind: SegField, Name: f.Name}), a.Fields[i], b.Fields[i])
	}
}

func (d *valueDiff) table(path string, a, b *value.Table) {
	seg := func(t *value.Table) func(int) Seg { return func(i int) Seg { return entrySeg(t.Entries[i].Ident.Key) } }
	d.keyed(path, keyedPair{
		lenX: len(a.Entries), lenY: len(b.Entries), segX: seg(a), segY: seg(b),
		find: func(i int) int {
			return slices.IndexFunc(b.Entries, func(e *value.Record) bool { return e.Ident.Key == a.Entries[i].Ident.Key })
		},
		both: func(p string, i, j int) {
			if d.placed != nil && !d.placed(p, a.Entries[i], b.Entries[j]) {
				d.out = append(d.out, p)
				return
			}
			d.value(p, a.Entries[i], b.Entries[j])
		},
	})
}

func (d *valueDiff) mapValue(path string, a, b *value.Map) {
	kt, _, _ := mapTypes(a.T)
	seg := func(m *value.Map) func(int) Seg { return func(i int) Seg { return keySeg(m.Keys[i], kt) } }
	d.keyed(path, keyedPair{
		lenX: len(a.Keys), lenY: len(b.Keys), segX: seg(a), segY: seg(b),
		find: func(i int) int {
			return slices.IndexFunc(b.Keys, func(k value.Value) bool { return sameText(k, a.Keys[i]) })
		},
		both: func(p string, i, j int) { d.value(p, a.Vals[i], b.Vals[j]) },
	})
}

// keyedPair are two keyed collections: their sizes, the segment naming each one's i-th item,
// where y holds x's i-th key (-1 for nowhere), and the comparison of two such items at their path.
type keyedPair struct {
	lenX, lenY int
	segX, segY func(int) Seg
	find       func(i int) int
	both       func(path string, i, j int)
}

// keyed compares two keyed collections at path: an item only one holds is a difference at its
// path; the collection itself differs when the keys both hold come in another order.
func (d *valueDiff) keyed(path string, k keyedPair) {
	var common []int
	for i := range k.lenX {
		j := k.find(i)
		if j < 0 {
			d.out = append(d.out, childPath(path, k.segX(i)))
			continue
		}
		common = append(common, j)
		k.both(childPath(path, k.segX(i)), i, j)
	}
	for j := range k.lenY {
		if !slices.Contains(common, j) {
			d.out = append(d.out, childPath(path, k.segY(j)))
		}
	}
	if !slices.IsSorted(common) && d.ordered(path) {
		d.out = append(d.out, path)
	}
}

func (d *valueDiff) list(path string, a, b *value.List) {
	lt, isList := baseOf(a.T).(*types.ListType)
	for i := range a.Elems {
		seg := intSeg(int64(i))
		if isList {
			seg = listSeg(lt, a.Elems[i], i)
		}
		d.value(childPath(path, seg), a.Elems[i], b.Elems[i])
	}
}

// sameText reports two values of different analyses equal: each analysis has its own types, so
// they are compared by their form and canonical text (API.md E22: values, not the files' text).
func sameText(x, y value.Value) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	return reflect.TypeOf(x) == reflect.TypeOf(y) && x.CanonText() == y.CanonText()
}
