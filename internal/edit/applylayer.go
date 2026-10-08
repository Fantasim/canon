package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// layerSet writes a Set into the edit layer, never into a base source (API.md W10, W11, W11a):
// inside the amendment that sets the path or an ancestor, active or not, else as the path's own
// amendment line, which replaces the lines amending its descendants.
func (x *opCtx) layerSet() error {
	v, err := x.typed(x.op.Value, x.layerType())
	if err != nil {
		return err
	}
	if v, err = x.keepKey(v); err != nil {
		return err
	}
	x.nw = v
	if x.inAmendment() {
		return x.activeSet(v)
	}
	if f, line, n := x.a.snap.layerLine(x.res, x.a.env.EditLayer); line != nil {
		return x.staticEdit(staticSite{f, line, n}, len(x.res.Steps), func(value.Value) (value.Value, error) { return v, nil })
	}
	if err := x.undoNewLine(); err != nil {
		return err
	}
	text, err := x.a.canonText(v)
	if err != nil {
		return err
	}
	return x.amendLine(x.res.Canonical, text)
}

// layerType is targetType, but the declared type where the edit layer is inactive: an arm the
// analysis computes there is not the one the layer's lines select (W11a; log-2026-10-01 M4.1).
func (x *opCtx) layerType() types.Type {
	if !x.layerInactive() {
		return x.targetType()
	}
	if _, f, ok := x.field(); ok {
		return f.Type
	}
	if t, err := x.a.snap.Type(x.res.Resolved); err == nil && t != nil {
		return t
	}
	return x.targetType()
}

// undoNewLine is the inverse of a new amendment line (E23, log-2026-09-29 M4 U4b-r): Reset,
// which removes it, when the layer amended nothing at or below the path; else Set of the value
// the lines it replaces gave.
func (x *opCtx) undoNewLine() error {
	for _, f := range layerFiles(x.res.root.pkg, x.a.env.EditLayer) {
		for _, b := range f.Amends {
			if b.Target != nil && b.Target.Name == x.res.root.obj.Name() && len(x.descendantLines(b)) > 0 {
				return x.undoSet()
			}
		}
	}
	x.inverse(Operation{Kind: OpReset, Path: x.res.Canonical})
	return nil
}

// activeSet edits the path inside the active edit layer's amendment: a default leaves the field
// out of a literal of that amendment only, else it is written, since the base would show (W10).
func (x *opCtx) activeSet(v value.Value) error {
	if err := x.undoSet(); err != nil {
		return err
	}
	if x.setsDefault(v) && x.layerHolds() {
		return x.change(nil)
	}
	return x.change(v)
}

// layerHolds reports the path's field in a literal the edit layer's amendment states.
func (x *opCtx) layerHolds() bool {
	if len(x.res.Steps) == 0 {
		return false
	}
	c := x.j.cur[x.statedBelow(len(x.res.Steps)-1)]
	return c.state == stTree && c.layer == x.a.env.EditLayer
}

// layerReset removes the path's own amendment line from the edit layer, or its field from the
// amendment literal that holds it (W11, W11a); nothing when the layer does not set it (E8).
func (x *opCtx) layerReset() error {
	f, line, n := x.a.snap.layerLine(x.res, x.a.env.EditLayer)
	switch {
	case line != nil && n == len(x.res.Steps):
		return x.removeLine(staticSite{f, line, n})
	case x.inAmendment() && x.layerHolds():
		if err := x.undoSet(); err != nil {
			return err
		}
		return x.change(nil)
	case line != nil && !x.inAmendment():
		return x.staticEdit(staticSite{f, line, n}, len(x.res.Steps), func(value.Value) (value.Value, error) { return nil, nil })
	}
	return nil
}

// removeLine removes the path's own amendment line; its inverse sets the value it gave again.
func (x *opCtx) removeLine(at staticSite) error {
	old := x.res.Target
	if !x.inAmendment() {
		v, _, err := x.lineValue(at, len(x.res.Steps))
		if err != nil {
			return err
		}
		old = v
	}
	lit, err := x.a.sourceLit(old, x.scopeAt(len(x.res.Steps), true), x.statedAt(len(x.res.Steps)))
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.res.Canonical, Value: lit})
	x.w.addCanon(at.f.Src.Path, x.res.root.pkg.Path, []format.Change{lineRemoval(at.f, at.line)})
	return nil
}

// layerAddEntry adds an entry through the edit layer (W11, LAY-02): into the amendment that
// states the collection, active or not (W11a), else as an amendment line creating the key.
func (x *opCtx) layerAddEntry() error {
	t, isTable := x.res.Target.(*value.Table)
	m, isMap := x.res.Target.(*value.Map)
	if !isTable && !isMap {
		return ErrBadOp
	}
	if x.inAmendment() {
		if isTable {
			return x.addTableEntry(t)
		}
		return x.addMapEntry(m)
	}
	if f, line, n := x.a.snap.layerLine(x.res, x.a.env.EditLayer); line != nil {
		return x.staticEdit(staticSite{f, line, n}, len(x.res.Steps), x.grow)
	}
	if isTable {
		return x.amendEntry(x.newTableEntry(t))
	}
	return x.amendEntry(x.newMapEntry(m))
}

// grow is the table or map the edit layer's amendment states with AddEntry's entry added.
func (x *opCtx) grow(cur value.Value) (value.Value, error) {
	var out value.Value
	var e entryParts
	var err error
	switch c := cur.(type) {
	case *value.Table:
		if e, err = x.newTableEntry(c); err == nil {
			rec, _ := e.v.(*value.Record)
			out = &value.Table{T: c.T, Entries: slices.Concat(c.Entries, []*value.Record{rec}), P: c.P}
		}
	case *value.Map:
		if e, err = x.newMapEntry(c); err == nil {
			out = &value.Map{T: c.T, Keys: slices.Concat(c.Keys, []value.Value{e.key}), Vals: slices.Concat(c.Vals, []value.Value{e.v}), P: c.P}
		}
	default:
		return nil, ErrBadOp
	}
	return out, err
}

// amendEntry writes the amendment line of a new entry e.
func (x *opCtx) amendEntry(e entryParts, err error) error {
	if err != nil {
		return err
	}
	text, err := x.a.canonText(e.v)
	if err != nil {
		return err
	}
	path := childPath(x.res.Canonical, e.seg)
	x.inverse(Operation{Kind: OpRemove, Path: path})
	return x.amendLine(path, text)
}

// layerRemove removes an entry the edit layer itself states (log-2026-09-29 M4 U4b-r): its own
// amendment line, or its item in the amendment's literal, active or not.
func (x *opCtx) layerRemove(parent value.Value, pos int) error {
	f, line, n := x.a.snap.layerLine(x.res, x.a.env.EditLayer)
	switch {
	case line != nil && n == len(x.res.Steps):
		if err := x.undoRemove(parent, pos); err != nil {
			return err
		}
		x.w.addCanon(f.Src.Path, x.res.root.pkg.Path, []format.Change{lineRemoval(f, line)})
		return nil
	case x.inAmendment() && x.layerHolds():
		if err := x.undoRemove(parent, pos); err != nil {
			return err
		}
		return x.removeCanon(x.j.last(), x.j.holder())
	case line != nil && !x.inAmendment():
		seg := x.res.Steps[len(x.res.Steps)-1]
		return x.staticEdit(staticSite{f, line, n}, len(x.res.Steps)-1, func(cur value.Value) (value.Value, error) {
			return x.withoutSeg(cur, seg)
		})
	}
	return &NotEditableError{Reason: ReasonLayer}
}

// inAmendment reports a path whose value the active edit layer's amendment states (W11a).
func (x *opCtx) inAmendment() bool {
	k := x.statedIndex()
	return x.j.cur[k].state == stTree && x.j.cur[k].layer == x.a.env.EditLayer
}
