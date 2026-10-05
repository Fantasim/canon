package edit

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// contradicted are the before state's @stable values, by path, a lock fact of the edit gives to
// another id, each as the edit's result holds it: their restore would contradict the lock, so
// the Undo keeps the result's (API.md E23) and verification excludes them (E22).
func (a *applier) contradicted(f *lockFacts) map[string]value.Value {
	out := map[string]value.Value{}
	for _, table := range f.tables() {
		before, ok := tableAt(a.base, table)
		if !ok {
			continue
		}
		for _, e := range before.Entries {
			p := childPath(table, entrySeg(e.Ident.Key))
			if _, isID := f.ids[p]; !isID {
				a.contradictedIn(f, p, e, out)
			}
		}
	}
	return out
}

// contradictedIn adds to out each @stable field of e, the before state's entry at path p, whose
// value an id of its table now holds and whose value in the edit's result differs.
func (a *applier) contradictedIn(f *lockFacts, p string, e *value.Record, out map[string]value.Value) {
	now, inResult := entryAt(a.snap, p)
	if !inResult {
		a.goneIn(f, p, e)
		return
	}
	for i, fd := range fieldsOf(e.T) {
		if !fd.Stable || i >= len(e.Fields) || !inResult || i >= len(now.Fields) || sameText(e.Fields[i], now.Fields[i]) {
			continue
		}
		if f.lockedElsewhere(p, i, e.Fields[i]) {
			out[childPath(p, Seg{Kind: SegField, Name: fd.Name})] = now.Fields[i]
		}
	}
}

// lockedElsewhere reports v the value of field i of an id of the table holding p, other than p,
// as the edit's result holds it.
func (f *lockFacts) lockedElsewhere(p string, i int, v value.Value) bool {
	table, _ := enclosing(p)
	for id, rec := range f.ids { //canon:unordered any match decides
		if parent, _ := enclosing(id); parent == table && id != p && rec != nil && i < len(rec.Fields) && sameText(rec.Fields[i], v) {
			return true
		}
	}
	return false
}

// tables are the paths of the tables holding the ids, in order.
func (f *lockFacts) tables() []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(f.ids)) {
		if parent, ok := enclosing(id); ok && !slices.Contains(out, parent) {
			out = append(out, parent)
		}
	}
	return out
}

// contraEntry is e, the entry at path p, with its contradicted values as the result holds them.
func (f *lockFacts) contraEntry(e *value.Record, p string) *value.Record {
	out := e
	for i, fd := range fieldsOf(e.T) {
		v, ok := f.contra[childPath(p, Seg{Kind: SegField, Name: fd.Name})]
		if !ok || i >= len(e.Fields) || i >= len(e.Set) {
			continue
		}
		if out == e {
			c := *e
			c.Fields, c.Set = slices.Clone(e.Fields), slices.Clone(e.Set)
			out = &c
		}
		out.Fields[i], out.Set[i] = v, true
	}
	return out
}

// tableAt is the table at path in s.
func tableAt(s *Snapshot, path string) (*value.Table, bool) {
	p, err := Parse(path)
	if err != nil {
		return nil, false
	}
	res, err := s.open(p)
	if err != nil {
		return nil, false
	}
	t, ok := res.Target.(*value.Table)
	return t, ok
}

// goneIn marks e, the before state's entry at path p that the result lacks (removed or renamed),
// gone when an id of its table now holds one of its @stable values: its restore would contradict
// the lock, so it stays as the result holds it (API.md E23).
func (a *applier) goneIn(f *lockFacts, p string, e *value.Record) {
	for i, fd := range fieldsOf(e.T) {
		if fd.Stable && i < len(e.Fields) && f.lockedElsewhere(p, i, e.Fields[i]) {
			f.gone[p] = true
			return
		}
	}
}

// renamedGone marks gone, too, the name the result gives each gone entry: the path of the Rename
// of undo that gives it back its before name.
func (f *lockFacts) renamedGone(undo []Operation) {
	for _, op := range undo {
		if to, ok := renamedTo(op); ok && f.gone[to] {
			f.gone[op.Path] = true
		}
	}
}

// targetOf is the path an operation writes: for an AddEntry or Insert, the item it adds.
func targetOf(op Operation) string {
	switch k := op.Key.(type) {
	case PathKey:
		if op.Kind == OpAddEntry {
			return childPath(op.Path, entrySeg(value.Key{S: string(k)}))
		}
	case IntKey:
		if op.Kind == OpAddEntry {
			return childPath(op.Path, entrySeg(value.Key{I: int64(k), IsInt: true}))
		}
	}
	return op.Path
}

// holds reports the edit's result holding the entry at path; when the result's table has no
// value (AllowErrors, EVALUATION.md 7.2), as the request's operations left it (API.md E23).
func (a *applier) holds(path string) bool {
	if _, ok := entryAt(a.snap, path); ok {
		return true
	}
	parent, _ := enclosing(path)
	_, readable := tableAt(a.snap, parent)
	return !readable && a.held[path]
}

// heldBy records, for a Set of a whole root table to v, which entries an earlier AddEntry put
// there it keeps (holds).
func (x *opCtx) heldBy(v value.Value) {
	t, ok := v.(*value.Table)
	if !ok || len(x.res.Steps) != 0 {
		return
	}
	for p := range x.a.held { //canon:unordered each entry is judged alone
		if parent, _ := enclosing(p); parent == x.res.Canonical {
			x.a.held[p] = slices.ContainsFunc(t.Entries, func(e *value.Record) bool {
				return childPath(parent, entrySeg(e.Ident.Key)) == p
			})
		}
	}
}
