package edit

import (
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// retireOp retires an entry of a stable table: `retired ` before its key (N7), or
// `"$retired": true` first in its JSON object (E10). It has no inverse (E4, E23); an entry
// already retired stays as it is.
func retireOp(x *opCtx) error {
	rec, ok := x.res.Target.(*value.Record)
	switch {
	case !ok || !x.stableTable():
		return ErrBadOp
	case isRetired(rec):
		return nil
	}
	c := x.j.last()
	if c.mode == ModeJSON {
		n, display, err := x.jsonAt(rec)
		if err != nil {
			return err
		}
		m, err := retiredMember()
		if err != nil {
			return err
		}
		d := &jsonDiff{a: x.a}
		d.insert(n, 0, m.Key, m.Value)
		x.w.addJSON(display, x.res.root.pkg.Path, d.out)
		return nil
	}
	switch c.node.(type) {
	case *syntax.EntryItem, *syntax.EntryDecl:
		x.w.addCanon(c.file.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Retire, Node: c.node}})
		return nil
	}
	return &NotEditableError{Reason: ReasonComputed}
}

// unretireOp is always refused: retirement is one-way (API.md E4).
func unretireOp(*opCtx) error {
	return ErrStableKey
}

// retireMember retires a member of an @codes enum: `retired ` before its name (API.md N7).
func (a *applier) retireMember(res resolution) (*work, error) {
	m, ok := res.Target.(*value.Member)
	d, isEnum := res.root.obj.Decl().(*syntax.EnumDecl)
	switch {
	case !ok || !isEnum || res.root.enum.Codes == nil || m.Index >= len(d.Members):
		return nil, ErrBadOp
	case res.root.enum.Members[m.Index].Retired:
		return newWork(), nil
	}
	w := newWork()
	f := res.root.obj.File()
	w.addCanon(f.Src.Path, res.root.pkg.Path, []format.Change{{Kind: format.Retire, Node: d.Members[m.Index]}})
	return w, nil
}
