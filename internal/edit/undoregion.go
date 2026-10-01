package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// regionOf is the smallest item enclosing res's value and the fields its type is computed from
// (log-2026-09-29 U-E22-r): the record holding the shallowest field on the way whose type a
// field beside it computes, or that computes one; res's value itself when there is none.
func regionOf(res resolution) string {
	for i, st := range res.Steps {
		rec, ok := res.parent(i).(*value.Record)
		if !ok || st.Seg.Kind != SegField {
			continue
		}
		fields := fieldsOf(rec.T)
		if k := fieldIndex(fields, st.Seg.Name); k >= 0 && (len(fields[k].DependsOn) > 0 || drives(fields, k)) {
			return (&opCtx{res: res}).pathAt(i)
		}
	}
	return res.Canonical
}

// drives reports field k of fields read by the type of another (TYPES.md §11).
func drives(fields []*types.Field, k int) bool {
	return slices.ContainsFunc(fields, func(f *types.Field) bool { return slices.Contains(f.DependsOn, k) })
}

// enclosing is the canonical path of the item holding the value at path, false for a root.
func enclosing(path string) (string, bool) {
	p, err := Parse(path)
	if err != nil || len(p.Segs) == 0 {
		return "", false
	}
	p.Segs = p.Segs[:len(p.Segs)-1]
	return p.String(), true
}

// touchesDependent reports x's path going through a field whose type a value computes, or that
// computes one, or the value it writes holding one (API.md E15, E22): what a later operation's
// inverse may need typed after its driver.
func (x *opCtx) touchesDependent() bool {
	for i, st := range x.res.Steps {
		rec, ok := x.res.parent(i).(*value.Record)
		if !ok || st.Seg.Kind != SegField {
			continue
		}
		fields := fieldsOf(rec.T)
		if k := fieldIndex(fields, st.Seg.Name); k >= 0 && (computed(fields[k].Type) || drives(fields, k)) {
			return true
		}
	}
	t, err := x.a.snap.Type(x.res.Resolved)
	if err != nil {
		return true
	}
	return (&depScan{seen: map[types.Type]bool{}}).visit(t)
}

// computed reports a type holding an application, a dependent union or map, or an applied
// record (SPEC 5.11), not crossing records.
func computed(t types.Type) bool {
	switch x := baseOf(t).(type) {
	case *types.TypeAppType, *types.DepUnionType, *types.DepMapType, *types.AppliedRecord:
		return true
	case *types.OptionalType:
		return computed(x.Elem)
	case *types.ListType:
		return computed(x.Elem)
	case *types.MapType:
		return computed(x.Key) || computed(x.Value)
	case *types.LitUnionType:
		return computed(x.Of)
	}
	return false
}

// depScan walks the values of a type, not crossing refs, each type once, for a field computed().
type depScan struct {
	seen map[types.Type]bool
}

func (s *depScan) visit(t types.Type) bool {
	if t == nil || s.seen[t] {
		return false
	}
	s.seen[t] = true
	if computed(t) {
		return true
	}
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return s.visit(x.Elem)
	case *types.ListType:
		return s.visit(x.Elem)
	case *types.TableType:
		return s.visit(x.Elem)
	case *types.MapType:
		return s.visit(x.Key) || s.visit(x.Value)
	case *types.VariantType:
		return slices.ContainsFunc(x.Cases, func(c *types.CaseType) bool { return s.visit(c) })
	case *types.RecordType, *types.CaseType:
		return slices.ContainsFunc(fieldsOf(x), func(f *types.Field) bool { return s.visit(f.Type) })
	}
	return false
}
