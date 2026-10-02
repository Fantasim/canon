package edit

import (
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// heldValue is the value a Set inverse writes back to a whole stable table or one of its entries,
// until the edit's lock facts are known (API.md E23): old the value before the operation, lit its
// literal.
type heldValue struct {
	old   value.Value
	scope *types.Field
	lit   Lit
}

func (heldValue) isLit() {}

// lockFacts are the entries of root stable tables the edit locked (E20), by path, as its result
// holds them (nil when it was not read), and the ones the before state lacked: an Undo never
// undoes a lock fact (API.md E23).
type lockFacts struct {
	ids    map[string]*value.Record
	fresh  map[string]bool
	stable map[string][]string    // each id's @stable fields
	contra map[string]value.Value // the before values a lock fact forbids, as the result holds them
	gone   map[string]bool        // the before entries a lock fact forbids, which the result lacks, by both names
}

// keepLocked is undo keeping the edit's lock facts (API.md E23): no inverse the Undo leaves to
// the result (kept), each write back of a stable table or entry in its form, and a Retire of
// each fresh id, last; an entry the result no longer holds was never locked and takes none.
func (a *applier) keepLocked(undo []Operation, locked []Locked) ([]Operation, error) {
	all := a.lockedEntries(locked)
	if len(all) == 0 {
		return plainHeld(undo), nil
	}
	if a.multi || a.allowErrors {
		if err := a.settle(); err != nil {
			return nil, err
		}
	}
	skipped, err := a.skipped(locked)
	if err != nil {
		return nil, err
	}
	gone := a.lockedEntries(skipped)
	a.facts = a.readFacts(slices.DeleteFunc(all, func(p string) bool { return slices.Contains(gone, p) }))
	a.facts.renamedGone(undo)
	out, err := a.keptOps(undo, gone)
	if err != nil {
		return nil, err
	}
	return append(out, a.facts.retires()...), nil
}

// keptOps are the operations of undo the lock facts leave, their held write backs in form; the
// Retire of an id E20 skipped is a Remove (API.md E23).
func (a *applier) keptOps(undo []Operation, skipped []string) ([]Operation, error) {
	var out []Operation
	for _, op := range undo {
		switch {
		case op.Kind == OpRetire && slices.Contains(skipped, op.Path) && a.holds(op.Path):
			op.Kind = OpRemove
		case op.Kind == OpRetire || a.facts.kept(op.Path) || a.facts.kept(targetOf(op)):
			continue
		}
		if h, ok := op.Value.(heldValue); ok {
			lit, err := a.heldLit(h, op.Path)
			if err != nil {
				return nil, err
			}
			op.Value = lit
		}
		out = append(out, op)
	}
	return out, nil
}

// retires are the Retires of the fresh ids the result holds live, in path order.
func (f *lockFacts) retires() []Operation {
	var out []Operation
	for _, p := range slices.Sorted(maps.Keys(f.fresh)) {
		if rec := f.ids[p]; f.fresh[p] && (rec == nil || !isRetired(rec)) {
			out = append(out, Operation{Kind: OpRetire, Path: p})
		}
	}
	return out
}

// lockedEntries are the paths of the entries of root stable tables among locked, an enum
// member's left out.
func (a *applier) lockedEntries(locked []Locked) []string {
	var out []string
	for _, l := range locked {
		dot := strings.LastIndex(l.Name, dotSeg)
		if dot < 0 {
			continue
		}
		root := Path{Package: l.Name[:dot], Root: l.Name[dot+len(dotSeg):]}
		res, err := a.base.open(root)
		if err != nil {
			res, err = a.snap.open(root)
		}
		if err != nil || !stableTable(res.Target.Type()) {
			continue
		}
		out = append(out, childPath(root.String(), entrySeg(value.Key{S: l.Key})))
	}
	return out
}

// readFacts reads each locked entry at paths in the edit's result and the before state; one a
// request of several operations no longer holds was never locked (E20).
func (a *applier) readFacts(paths []string) *lockFacts {
	f := &lockFacts{ids: map[string]*value.Record{}, fresh: map[string]bool{}, stable: map[string][]string{}, gone: map[string]bool{}}
	for _, p := range paths {
		rec, inResult := entryAt(a.snap, p)
		switch {
		case !a.multi:
			rec = nil // a lone operation's result is not read: nothing of it is written back
		case !inResult:
			continue
		}
		before, held := entryAt(a.base, p)
		f.ids[p], f.fresh[p] = rec, !held
		if held {
			f.stable[p] = stableNames(before)
		}
	}
	if a.multi {
		f.contra = a.contradicted(f)
	}
	return f
}

// stableNames are the @stable fields of rec.
func stableNames(rec *value.Record) []string {
	var out []string
	for _, fd := range fieldsOf(rec.T) {
		if fd.Stable {
			out = append(out, fd.Name)
		}
	}
	return out
}

// kept reports path at or inside a value the Undo leaves as the edit's result holds it: an entry
// the before state lacked, a @stable field of a locked entry, or a contradicted value.
func (f *lockFacts) kept(path string) bool {
	if f == nil {
		return false
	}
	if underAny(path, maps.Keys(f.contra)) || underAny(path, maps.Keys(f.gone)) {
		return true
	}
	for p := range f.ids { //canon:unordered any match decides
		if f.fresh[p] && within(path, p, pathMarks, true) {
			return true
		}
		for _, name := range f.stable[p] {
			if within(path, childPath(p, Seg{Kind: SegField, Name: name}), pathMarks, true) {
				return true
			}
		}
	}
	return false
}

// underAny reports path at or inside one of places, in any order.
func underAny(path string, places iter.Seq[string]) bool {
	for place := range places {
		if within(path, place, pathMarks, true) {
			return true
		}
	}
	return false
}

// plainHeld is undo with each held write back given its old literal.
func plainHeld(undo []Operation) []Operation {
	for i, op := range undo {
		if h, ok := op.Value.(heldValue); ok {
			undo[i].Value = h.lit
		}
	}
	return undo
}

// heldLit is the literal of h, written back at path: its old value with the lock facts kept.
func (a *applier) heldLit(h heldValue, path string) (Lit, error) {
	v := a.facts.form(a.base, h.old, path)
	if v == h.old {
		return h.lit, nil
	}
	return a.sourceLit(v, h.scope)
}

// form is v, written back at path, with the lock facts kept: a stable table its locked entries as
// the result holds them, without the entries neither the before state holds nor the edit locked;
// a locked entry its id and @stable fields as the result holds them. v itself when none applies.
func (f *lockFacts) form(base *Snapshot, v value.Value, path string) value.Value {
	if f == nil {
		return v
	}
	switch x := v.(type) {
	case *value.Table:
		if stableTable(x.T) {
			return f.table(base, x, path)
		}
	case *value.Record:
		if rec, ok := f.ids[path]; ok && rec != nil && !f.fresh[path] {
			return f.entry(x, rec)
		}
		return f.contraEntry(x, path)
	}
	return v
}

// table is old, the stable table at path, with the lock facts kept (form).
func (f *lockFacts) table(base *Snapshot, old *value.Table, path string) *value.Table {
	before := baseKeys(base, path)
	var entries []*value.Record
	for _, e := range old.Entries {
		p := childPath(path, entrySeg(e.Ident.Key))
		rec, isID := f.ids[p]
		switch {
		case isID && rec == nil:
			entries = append(entries, e)
		case isID && f.fresh[p]:
			entries = append(entries, rec)
		case isID:
			entries = append(entries, f.entry(e, rec))
		case before[e.Ident.Key.Text()] && !f.gone[p]:
			entries = append(entries, f.contraEntry(e, p))
		}
	}
	for _, p := range slices.Sorted(maps.Keys(f.ids)) {
		rec := f.ids[p]
		if parent, _ := enclosing(p); parent == path && rec != nil && entryIndex(entries, rec.Ident.Key.Text()) < 0 {
			entries = append(entries, rec)
		}
	}
	return &value.Table{T: old.T, Entries: entries, P: old.P}
}

// entry is old, a locked entry the before state held, with its retirement and @stable fields as
// rec, the result's, holds them.
func (f *lockFacts) entry(old, rec *value.Record) *value.Record {
	out, id := *old, *old.Ident
	id.Retired = isRetired(rec)
	out.Ident = &id
	out.Fields, out.Set = slices.Clone(old.Fields), slices.Clone(old.Set)
	for i, fd := range fieldsOf(old.T) {
		if fd.Stable && i < len(rec.Fields) && i < len(rec.Set) {
			out.Fields[i], out.Set[i] = rec.Fields[i], rec.Set[i]
		}
	}
	return &out
}

// baseKeys are the keys of the table at path in base.
func baseKeys(base *Snapshot, path string) map[string]bool {
	out := map[string]bool{}
	p, err := Parse(path)
	if err != nil {
		return out
	}
	res, err := base.open(p)
	if err != nil {
		return out
	}
	if t, ok := res.Target.(*value.Table); ok {
		for _, e := range t.Entries {
			out[e.Ident.Key.Text()] = true
		}
	}
	return out
}

// entryAt is the table entry at path in s, false when s has none there.
func entryAt(s *Snapshot, path string) (*value.Record, bool) {
	p, err := Parse(path)
	if err != nil {
		return nil, false
	}
	res, err := s.open(p)
	if err != nil {
		return nil, false
	}
	rec, ok := res.Target.(*value.Record)
	return rec, ok && rec.Ident != nil
}
