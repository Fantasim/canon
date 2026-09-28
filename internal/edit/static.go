package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// resolution is what an analysis of a Resolved reads again: its root and the root's value.
type resolution struct {
	Resolved
	root rootRef
	val  value.Value
}

// open resolves p with its root and the root's value.
func (s *Snapshot) open(p Path) (resolution, error) {
	r, err := Resolve(s, p)
	if err != nil {
		return resolution{}, err
	}
	root, err := s.lookup(p)
	if err != nil {
		return resolution{}, err
	}
	res := resolution{Resolved: r, root: root}
	if root.enum == nil {
		res.val, err = s.force(root)
	}
	return res, err
}

// reopen checks that r is this snapshot's: its canonical path resolves here to the same value
// at every step, else ErrForeign.
func (s *Snapshot) reopen(r Resolved) (resolution, error) {
	p, err := Parse(r.Canonical)
	if err != nil {
		return resolution{}, ErrForeign
	}
	res, err := s.open(p)
	if err != nil || len(res.Steps) != len(r.Steps) || !same(res.Target, r.Target) {
		return resolution{}, ErrForeign
	}
	for i, st := range r.Steps {
		if !same(res.Steps[i].Value, st.Value) {
			return resolution{}, ErrForeign
		}
	}
	res.Resolved = r
	return res, nil
}

// same reports one value: the instance, or a pseudo-field or member Resolve builds afresh from it.
func same(a, b value.Value) bool {
	if a == b {
		return true
	}
	if ma, ok := a.(*value.Member); ok {
		mb, isMember := b.(*value.Member)
		return isMember && ma.Enum == mb.Enum && ma.Index == mb.Index
	}
	return a != nil && b != nil && a.Prov() != nil && a.Prov() == b.Prov() && a.CanonText() == b.CanonText()
}

// parent is the value step i was read in: the previous step's, or the root's.
func (res resolution) parent(i int) value.Value {
	if i == 0 {
		return res.val
	}
	return res.Steps[i-1].Value
}

// Type is the declared type of r's target (API.md §5.2 TypeInfo); ErrForeign if r is not of s.
func (s *Snapshot) Type(r Resolved) (types.Type, error) {
	res, err := s.reopen(r)
	switch {
	case err != nil:
		return nil, err
	case res.root.enum != nil:
		return res.root.enum, nil
	case len(r.Steps) == 0:
		return res.root.obj.Type(), nil
	}
	last := len(r.Steps) - 1
	st := r.Steps[last]
	if rec, isRec := res.parent(last).(*value.Record); isRec {
		fields := fieldsOf(rec.T)
		if i := fieldIndex(fields, st.Seg.Name); i >= 0 {
			return fields[i].Type, nil
		}
		_, t, _ := pseudo(rec, st.Seg.Name)
		return t, nil
	}
	_, vt, isMap := mapTypes(st.Container)
	if isMap {
		return vt, nil
	}
	return elemType(st.Container), nil
}

// elemType is the static type of an element or entry of a list or table type.
func elemType(ct types.Type) types.Type {
	switch b := ct.Base().(type) {
	case *types.ListType:
		return b.Elem
	case *types.TableType:
		return b.Elem
	}
	return nil
}
