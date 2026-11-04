package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fits reports op valid for res's target without an edit layer: no ErrBadOp nor ErrStableKey
// from its handler, whatever its value (fitters; DECISIONS 462).
func (s *Snapshot) fits(op Op, res resolution) bool {
	return int(op) < len(fitters) && fitters[op](s, res)
}

func fitsAny(*Snapshot, resolution) bool { return true }

func fitsNone(*Snapshot, resolution) bool { return false }

// fitsList is Add's and Insert's target: a list.
func fitsList(_ *Snapshot, res resolution) bool {
	_, ok := res.Target.(*value.List)
	return ok
}

// fitsEntries is AddEntry's target: a table or a map.
func fitsEntries(_ *Snapshot, res resolution) bool {
	switch res.Target.(type) {
	case *value.Table, *value.Map:
		return true
	}
	return false
}

// fitsReset is a base Reset's target: a record field with a default (resetOp).
func fitsReset(_ *Snapshot, res resolution) bool {
	_, f, ok := fieldAt(res)
	return ok && hasDefault(f)
}

// fitsItem is Move's target: an item of a list, a map or a table.
func fitsItem(_ *Snapshot, res resolution) bool {
	_, _, ok := siblingOf(res)
	return ok
}

// fitsRemove is Remove's target: an item whose key no lock holds (removeOp).
func fitsRemove(s *Snapshot, res resolution) bool {
	return fitsItem(s, res) && !s.lockedIn(res, "", nil)
}

// fitsRename is Rename's target: an item of a keyed collection whose key no lock holds (renameOp).
func fitsRename(s *Snapshot, res resolution) bool {
	parent, _, ok := siblingOf(res)
	if !ok || s.lockedIn(res, "", nil) {
		return false
	}
	_, ok = keyType(parent)
	return ok
}

// fitsRetire is Retire's target: an entry of a stable table (retireOp).
func fitsRetire(_ *Snapshot, res resolution) bool {
	_, ok := res.Target.(*value.Record)
	return ok && stableIn(res)
}

// fitsCase is SetCase's target: a value of a variant case (setCaseOp).
func fitsCase(_ *Snapshot, res resolution) bool {
	rec, ok := res.Target.(*value.Record)
	if !ok {
		return false
	}
	_, ok = rec.T.Base().(*types.CaseType)
	return ok
}

// inEntries reports res's target an entry of a table or a map: a Remove a literal takes it out of.
func inEntries(res resolution) bool {
	n := len(res.Steps)
	if n == 0 {
		return false
	}
	switch res.parent(n - 1).(type) {
	case *value.Table, *value.Map:
		return true
	}
	return false
}
