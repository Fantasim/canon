package std

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// positionCarriers test the operands of a built-in whose result carries a position mark (EVALUATION.md §13.1).
var positionCarriers = map[string]func(c *Call) bool{
	bInt: firstMarked, bFloat: firstMarked, bString: firstMarked, bJoin: elementMarked,
}

// CarriesPosition reports an Int, Float, String or join call over a marked operand: the caller marks c.Prov.
func CarriesPosition(c *Call) bool {
	test := positionCarriers[c.Name]
	return test != nil && test(c)
}

// firstMarked is a conversion of a marked value.
func firstMarked(c *Call) bool {
	return len(c.Args) > 0 && value.IsPosition(c.Args[0])
}

// elementMarked is a join over a marked element; the separator is no element.
func elementMarked(c *Call) bool {
	for _, x := range Elems(c.Recv) {
		if value.IsPosition(x) {
			return true
		}
	}
	return false
}

// entryInts make the positions of a receiver's elements, those of entries marked (EVALUATION.md §13.1).
type entryInts struct {
	h      Host
	c      *Call
	marked *value.Prov
}

// at is position i of element x; the marked provenance is made once per call.
func (e *entryInts) at(i int, x value.Value) value.Value {
	p := e.c.Prov
	if value.IsEntry(x) {
		if e.marked == nil {
			e.marked = e.h.Position(p)
		}
		p = e.marked
	}
	return &value.Int{V: int64(i), T: types.IntType, P: p}
}

// unmarked is v, rebuilt at c's provenance when it is an entry position: only §13.1's carriers keep the mark.
func unmarked(c *Call, v value.Value) value.Value {
	switch x := v.(type) {
	case *value.Int:
		if x.P != nil && x.P.Position {
			return &value.Int{V: x.V, T: x.T, P: c.Prov}
		}
	case *value.Float:
		if x.P != nil && x.P.Position {
			return &value.Float{V: x.V, T: x.T, P: c.Prov}
		}
	case *value.Dur:
		if x.P != nil && x.P.Position {
			return &value.Dur{Ms: x.Ms, P: c.Prov}
		}
	}
	return v
}

// elemAt is element i of a list or table, without copying a table's entries.
func elemAt(v value.Value, i int) value.Value {
	if t, ok := v.(*value.Table); ok {
		return t.Entries[i]
	}
	return Elems(v)[i]
}
