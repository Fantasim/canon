package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// keyedMap reports a map field whose keys have a type computed from a value (TYPES.md 11.5):
// E15 judges it by its keys, and drops it to its default or none when they fit no longer
// (log-2026-09-29 M4 B7-r4).
func keyedMap(f *types.Field) bool {
	m, ok := present(f.Type).Base().(*types.MapType)
	return ok && dependent(m.Key) && hasDefault(f)
}

// refused reports the final state refusing c's field: an error of fitCodes at its exact place,
// or for a keyed map at one of its entries, whose key no longer fits (API.md E15).
func (a *applier) refused(c fieldCheck) bool {
	pl, ok := a.placeOf(c)
	if !ok {
		return false
	}
	if !c.keyed {
		return len(a.snap.refusals(pl, false)) > 0
	}
	for _, f := range a.snap.refusals(pl, true) {
		if entryPlace(f.Path, pl.rel) || pl.ptr != "" && a.snap.display(f.Span.File) == pl.file && entryPointer(f.Pointer, pl.ptr) {
			return true
		}
	}
	return false
}

// entryPlace reports path p at place, or at an entry of the map at place: one step deeper.
func entryPlace(p, place string) bool {
	if p == place || !within(p, place, pathMarks, true) {
		return p == place
	}
	pp, err := Parse(p)
	if err != nil {
		return false
	}
	at, err := Parse(place)
	return err == nil && len(pp.Segs) == len(at.Segs)+1
}

// entryPointer reports pointer p at place, or at a member of the object at place.
func entryPointer(p, place string) bool {
	rest, ok := strings.CutPrefix(p, place)
	return ok && (rest == "" || strings.HasPrefix(rest, pointerSep) && !strings.Contains(rest[1:], pointerSep))
}

// dropOp is the operation E15 drops c's field with: none for an optional field (E7 rules), its
// default for a keyed map that is not optional (log-2026-09-29 M4 B7-r4).
func dropOp(c fieldCheck) (Operation, func(*opCtx) error) {
	if isOptional(c.f.Type) {
		return Operation{Kind: OpSet, Path: c.path(), Value: None{}}, setOp
	}
	return Operation{Kind: OpReset, Path: c.path()}, resetOp
}
