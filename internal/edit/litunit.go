package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// wholeUnits refuses a Duration of v, typed from lit as t, that no whole number of its wire unit
// writes in a JSON source (API.md V1; WIRE.md 5.1, E8102): unit is the scope v stands in, and
// each field of a record brings its own (WIRE.md 4.1).
func (tc *typing) wholeUnits(lit Lit, t types.Type, v value.Value, unit types.Unit) error {
	switch x := v.(type) {
	case *value.Dur:
		if x.Ms%unit.Millis() != 0 {
			return tc.refuse(t, describe(lit), detailUnit+unit.String())
		}
	case *value.Record:
		return tc.fieldUnits(lit, t, x)
	case *value.List:
		return tc.elemUnits(lit, t, x.Elems, unit)
	case *value.Map:
		return tc.elemUnits(lit, t, x.Vals, unit)
	case *value.Table:
		for i, e := range x.Entries {
			if err := tc.unitsAt(Seg{Kind: SegPos, Pos: i}, lit, t, e, unit); err != nil {
				return err
			}
		}
	}
	return nil
}

// fieldUnits is wholeUnits on each field rec writes, in the unit of that field.
func (tc *typing) fieldUnits(lit Lit, t types.Type, rec *value.Record) error {
	for i, f := range fieldsOf(rec.T) {
		if i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		if err := tc.unitsAt(Seg{Kind: SegField, Name: f.Name}, lit, t, rec.Fields[i], f.Unit); err != nil {
			return err
		}
	}
	return nil
}

// elemUnits is wholeUnits on each element of a list or value of a map, in the scope's unit.
func (tc *typing) elemUnits(lit Lit, t types.Type, vs []value.Value, unit types.Unit) error {
	for i, v := range vs {
		if err := tc.unitsAt(Seg{Kind: SegPos, Pos: i}, lit, t, v, unit); err != nil {
			return err
		}
	}
	return nil
}

// unitsAt is wholeUnits on v one segment deeper, for the refusal's detail.
func (tc *typing) unitsAt(seg Seg, lit Lit, t types.Type, v value.Value, unit types.Unit) error {
	_, err := tc.inside(seg, func() (value.Value, error) { return nil, tc.wholeUnits(lit, t, v, unit) })
	return err
}
