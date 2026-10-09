package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Step is one segment of a resolved path, in canonical form (API.md P8): the static type of
// the container it was read in, and the value there (nil for an input field, TYP-17).
type Step struct {
	Seg       Seg
	Container types.Type
	Value     value.Value
}

// Resolved is a path read against a snapshot: canonical form, one step per segment, target.
type Resolved struct {
	Canonical string
	Steps     []Step
	Target    value.Value
}

// Resolve reads p against s, its root then each segment (API.md §6.2-§6.5); a refusal is a *PathError.
func Resolve(s *Snapshot, p Path) (Resolved, error) {
	root, err := s.lookup(p)
	if err != nil {
		return Resolved{}, err
	}
	if root.enum != nil {
		return resolveMember(root, p)
	}
	v, err := s.force(root)
	if err != nil {
		return Resolved{}, err
	}
	w := walk{cur: v, typ: root.obj.Type(), keys: &s.keys}
	for i, seg := range p.Segs {
		if err := w.next(seg); err != nil {
			return Resolved{}, &PathError{Seg: i, Err: err}
		}
	}
	canon := Path{Package: root.pkg.Path, Root: root.obj.Name(), Segs: make([]Seg, len(w.steps))}
	for i, st := range w.steps {
		canon.Segs[i] = st.Seg
	}
	return Resolved{Canonical: canon.String(), Steps: w.steps, Target: w.cur}, nil
}

// resolveMember is `Enum.member`, the only path an enum root takes (API.md P7a).
func resolveMember(root rootRef, p Path) (Resolved, error) {
	switch {
	case len(p.Segs) == 0:
		return Resolved{}, &PathError{Seg: rootSeg, Err: ErrBadPath}
	case p.Segs[0].Kind != SegField:
		return Resolved{}, &PathError{Seg: 0, Err: ErrBadPath}
	case len(p.Segs) > 1:
		return Resolved{}, &PathError{Seg: 1, Err: ErrBadPath}
	}
	for i, m := range root.enum.Members {
		if m.Name == p.Segs[0].Name {
			v := &value.Member{Enum: root.enum, Index: i}
			canon := Path{Package: root.pkg.Path, Root: root.obj.Name(), Segs: p.Segs[:1]}
			step := Step{Seg: Seg{Kind: SegField, Name: m.Name}, Container: root.enum, Value: v}
			return Resolved{Canonical: canon.String(), Steps: []Step{step}, Target: v}, nil
		}
	}
	return Resolved{}, &PathError{Seg: 0, Err: ErrNoPath}
}

// walk is a resolution in progress: the value reached and its static type, and the key indexes
// of the tables it reads (nil: scanned).
type walk struct {
	cur   value.Value
	typ   types.Type
	steps []Step
	keys  *tableKeys
}

// read is one segment read in a container: the value there, its static type, the canonical segment.
type read struct {
	v   value.Value
	typ types.Type
	seg Seg
}

// next reads seg in the current value (API.md §6.2).
func (w *walk) next(seg Seg) error {
	ct, err := w.container()
	if err != nil {
		return err
	}
	var r read
	switch x := w.cur.(type) {
	case *value.Record:
		r, err = readRecord(x, seg)
	case *value.List:
		r, err = readList(x, ct, seg)
	case *value.Table:
		r, err = readTable(x, ct, seg, w.keys)
	case *value.Map:
		r, err = readMap(x, ct, seg)
	default:
		err = ErrBadPath // P5: a scalar, a ref or a range has no segment
	}
	if err != nil {
		return err
	}
	w.steps = append(w.steps, Step{Seg: r.seg, Container: ct, Value: r.v})
	w.cur, w.typ = r.v, r.typ
	return nil
}

// container is the static type the next segment is read in, a dependent one as verified (TYPES.md §11.6).
func (w *walk) container() (types.Type, error) {
	if _, none := w.cur.(*value.None); none || w.cur == nil {
		return nil, ErrNoPath // none, or below an input field: nothing at build time
	}
	t := w.typ
	if o, ok := t.Underlying().(*types.OptionalType); ok {
		t = o.Elem
	}
	switch t.Base().Kind() {
	case types.TypeApp, types.DepUnion, types.Any, types.Error:
		return w.cur.Type(), nil
	default:
		return t, nil
	}
}

// fieldsOf is the fields of a record, case or applied record type.
func fieldsOf(t types.Type) []*types.Field {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Fields
	case *types.CaseType:
		return d.Fields
	case *types.AppliedRecord:
		return d.Rec.Fields
	}
	return nil
}

// fieldIndex is the index of the field name in fields, -1 when there is none.
func fieldIndex(fields []*types.Field, name string) int {
	for i, f := range fields {
		if f.Name == name {
			return i
		}
	}
	return -1
}
