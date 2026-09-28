package encode

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Force is the settled value of a top-level let of a package, false when it has none (an
// unselected package, a failed evaluation).
type Force func(pkg, name string) (value.Value, bool)

// Colls reads the entries of the program's collections as evaluated in this build (VIEWMODEL.md
// C3, J12), each collection once.
type Colls struct {
	force Force
	seen  map[*types.Collection][]*value.Record
}

// NewColls reads collections through force; a nil force reads every collection as empty.
func NewColls(force Force) *Colls {
	return &Colls{force: force, seen: map[*types.Collection][]*value.Record{}}
}

// Counts are a collection's entries and active entries (J12); 0 for a field of an enclosing
// record, whose entries are per instance.
func (c *Colls) Counts(coll *types.Collection) (count, active int) {
	for _, e := range c.Entries(coll) {
		count++
		if e.Ident == nil || !e.Ident.Retired {
			active++
		}
	}
	return count, active
}

// Entries are a let's table entries or keyed-list elements in collection order, retired ones
// included; nil for a field of an enclosing record or a value this build did not settle.
func (c *Colls) Entries(coll *types.Collection) []*value.Record {
	if es, ok := c.seen[coll]; ok {
		return es
	}
	var es []*value.Record
	if coll.Kind != types.CollField && c.force != nil {
		if v, ok := c.force(coll.Pkg, coll.Name); ok {
			es = entries(fieldPath(v, coll.FieldPath))
		}
	}
	c.seen[coll] = es
	return es
}

// fieldPath is v's field named by path, record by record; nil when one is missing.
func fieldPath(v value.Value, path []string) value.Value {
	for _, name := range path {
		r, ok := v.(*value.Record)
		if !ok {
			return nil
		}
		v = nil
		for i, f := range FieldsOf(r.T) {
			if f.Name == name && i < len(r.Fields) {
				v = r.Fields[i]
			}
		}
	}
	return v
}

// entries are a table's entries or a list's record elements.
func entries(v value.Value) []*value.Record {
	switch x := v.(type) {
	case *value.Table:
		return x.Entries
	case *value.List:
		out := make([]*value.Record, 0, len(x.Elems))
		for _, e := range x.Elems {
			if r, ok := e.(*value.Record); ok {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}
