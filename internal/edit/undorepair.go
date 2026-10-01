package edit

import (
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/value"
)

// build is the Undo to verify: the plain inverses but the refused and the value ones in a region,
// each region restored where its last value inverse was, under its name there, so a Rename around
// it runs with the values it ran with (E11), else at the end; c.origin records each one's source.
func (c *undoCheck) build() []Operation {
	last := make([]int, len(c.regions))
	for r := range last {
		last[r] = -1
	}
	var kept []Operation
	var from []int
	for i, op := range c.plain {
		r := c.regionHolding(i, op.Path)
		switch {
		case c.dropped[i]:
		case valueOps[op.Kind] && r >= 0:
			last[r] = i
		default:
			kept, from = append(kept, op), append(from, i)
		}
	}
	return c.placeRestores(kept, from, last)
}

// regionHolding is the index of the region path lies in, at plain inverse i: under the name the
// region's item has there, the Renames after it not yet undone; -1 for none.
func (c *undoCheck) regionHolding(i int, path string) int {
	var later []Operation
	for j := i + 1; j < len(c.plain); j++ {
		if !c.dropped[j] {
			later = append(later, c.plain[j])
		}
	}
	return slices.IndexFunc(c.regions, func(r string) bool { return within(path, renamedPath(later, r), pathMarks, true) })
}

// placeRestores lists kept, from their plain indexes, with each region's restore before the first
// kept operation past last[r], or at the end, its path the name the item has there.
func (c *undoCheck) placeRestores(kept []Operation, from, last []int) []Operation {
	at := make([][]Operation, len(kept)+1)
	who := make([][]int, len(kept)+1)
	for r, path := range c.regions {
		k := c.placeOf(path, placings(kept, from, last[r], path))
		alias := renamedPath(kept[k:], path)
		for _, op := range c.restore(path) {
			op.Path = alias + strings.TrimPrefix(op.Path, path)
			at[k], who[k] = append(at[k], op), append(who[k], -1-r)
		}
	}
	var out []Operation
	c.origin = c.origin[:0]
	for k := range at {
		out, c.origin = append(out, at[k]...), append(c.origin, who[k]...)
		if k < len(kept) {
			out, c.origin = append(out, kept[k]), append(c.origin, from[k])
		}
	}
	return out
}

// placings are the places a region's restore is tried at among kept, in turn: where its last
// value inverse was (last, a plain index), before the last Rename giving it its name, first,
// last (E11: a Rename moves its file only when the template matches the values then).
func placings(kept []Operation, from []int, last int, path string) []int {
	k := slices.IndexFunc(from, func(f int) bool { return f > last })
	if k < 0 || last < 0 {
		k = len(kept)
	}
	var out []int
	for _, p := range []int{k, renameBack(kept, path), 0, len(kept)} {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// placeOf is the place the restore of the region at path is tried at now, among places.
func (c *undoCheck) placeOf(path string, places []int) int {
	c.places[path] = len(places)
	return places[min(c.tried[path], len(places)-1)]
}

// laterName is the name the item at path has after ops: each Rename of it, or of an item
// holding it, gives it the name the Rename gives back (the base's, at the end of an Undo).
func laterName(ops []Operation, path string) string {
	for _, op := range ops {
		if to, ok := renamedTo(op); ok && within(path, op.Path, pathMarks, true) {
			path = to + strings.TrimPrefix(path, op.Path)
		}
	}
	return path
}

// renameBack is the index among ops of the last Rename giving the item at path its name, so its
// file follows its template with the values restored (E11, N8); len(ops) for none.
func renameBack(ops []Operation, path string) int {
	for k, op := range slices.Backward(ops) {
		if to, ok := renamedTo(op); ok && within(path, to, pathMarks, true) {
			return k
		}
	}
	return len(ops)
}

// renamedPath is the name the item at path, after ops, has before them: each Rename that gives
// it, or an item holding it, its later name, undone from the last (E11).
func renamedPath(ops []Operation, path string) string {
	for _, op := range slices.Backward(ops) {
		if to, ok := renamedTo(op); ok && within(path, to, pathMarks, true) {
			path = op.Path + strings.TrimPrefix(path, to)
		}
	}
	return path
}

// renamedTo is the path a Rename gives its item, false for another operation.
func renamedTo(op Operation) (string, bool) {
	parent, ok := enclosing(op.Path)
	if op.Kind != OpRename || !ok {
		return "", false
	}
	p, err := Parse(parent)
	now, nerr := Parse(op.Path)
	if err != nil || nerr != nil || len(now.Segs) == 0 {
		return "", false
	}
	var seg Seg
	switch k := op.Key.(type) {
	case IntKey:
		seg = intSeg(int64(k))
	case PathKey:
		seg = Seg{Kind: SegKey, Key: textKey(string(k))}
	case Key:
		seg = Seg{Kind: SegKey, Key: textKey(string(k))}
	default:
		return "", false
	}
	if now.Segs[len(now.Segs)-1].Kind == SegField && seg.Key.Kind == KeyWord {
		seg = Seg{Kind: SegField, Name: seg.Key.Text} // a table entry named by a word (P8)
	}
	p.Segs = append(p.Segs, seg)
	return p.String(), true
}

// restore gives the item at path its value before the edit, carried as E23 carries it: one Set,
// or for a record restored field by field each field that differs, drivers first (TYPES.md 11);
// none for an item the base lacks, which its holder restores.
func (c *undoCheck) restore(path string) []Operation {
	p, err := Parse(path)
	if err != nil {
		return nil
	}
	res, err := c.a.base.open(p)
	if err != nil || res.Target == nil {
		return nil
	}
	if rec, ok := res.Target.(*value.Record); ok && c.fieldwise[path] {
		return c.restoreFields(res, p, rec)
	}
	x := &opCtx{a: c.a, res: res}
	lit, err := c.a.baseLit(res.Target, x.scopeAt(len(res.Steps), true))
	if err != nil {
		return nil
	}
	return []Operation{{Kind: OpSet, Path: res.Canonical, Value: lit}}
}

// restoreFields are the Sets and Resets giving the record rec at res, read at p, its fields
// before the edit, each one the last run left otherwise.
func (c *undoCheck) restoreFields(res resolution, p Path, rec *value.Record) []Operation {
	now, _ := c.last.open(p)
	cur, _ := now.Target.(*value.Record)
	var out []Operation
	for i, f := range fieldsOf(rec.T) {
		if f.Input != nil || cur != nil && i < len(cur.Fields) && sameText(rec.Fields[i], cur.Fields[i]) {
			continue
		}
		fp := childPath(res.Canonical, Seg{Kind: SegField, Name: f.Name})
		if !rec.Set[i] && hasDefault(f) {
			out = append(out, Operation{Kind: OpReset, Path: fp})
			continue
		}
		lit, err := c.a.baseLit(rec.Fields[i], fieldRules(f))
		if err != nil {
			return nil
		}
		out = append(out, Operation{Kind: OpSet, Path: fp, Value: lit})
	}
	return out
}

// repair leaves out the refused plain inverses, restoring their regions and those of the values
// not given back, but values the base holds in error; a region that still misses goes field by
// field, then grows to its holder. False when nothing changed.
func (c *undoCheck) repair(cur []Operation, failed map[int]error, misses []string) bool {
	grow := map[string]bool{}
	var found []string
	for _, m := range misses {
		if r := c.inRegion(m); r >= 0 {
			grow[c.regions[r]] = true
			continue
		}
		found = append(found, c.regionAt(m))
	}
	changed := false
	for _, i := range slices.Sorted(maps.Keys(failed)) {
		o := c.origin[i]
		switch {
		case o < 0:
			grow[c.regions[-1-o]] = true
		case c.excluded(cur[i].Path):
			c.dropped[o], changed = true, true
		default:
			c.dropped[o], changed = true, true
			found = append(found, c.regionAt(laterName(cur[i+1:], cur[i].Path)))
		}
	}
	for _, r := range slices.Sorted(maps.Keys(grow)) {
		changed = c.grow(r, &found) || changed
	}
	for _, f := range found {
		changed = c.add(f) || changed
	}
	return changed
}

// grow tries region r's restore at its next place, then field by field when it holds a record,
// else adds to found the region of the item holding it; false when nothing is left to try.
func (c *undoCheck) grow(r string, found *[]string) bool {
	if c.tried[r]+1 < c.places[r] {
		c.tried[r]++
		return true
	}
	if p, err := Parse(r); err == nil && !c.fieldwise[r] {
		if res, err := c.a.base.open(p); err == nil {
			if _, isRec := res.Target.(*value.Record); isRec {
				c.fieldwise[r] = true
				return true
			}
		}
	}
	if parent, ok := enclosing(r); ok {
		*found = append(*found, c.regionAt(parent))
	}
	return false
}

// regionAt is the region of the value at path (regionOf), read in the base, else in the edit's
// result, else in the last run's state; "" when none reaches it.
func (c *undoCheck) regionAt(path string) string {
	p, err := Parse(path)
	if err != nil {
		return ""
	}
	for _, s := range []*Snapshot{c.a.base, c.after, c.last} {
		if res, err := s.open(p); err == nil {
			return regionOf(res)
		}
	}
	return ""
}

// inRegion is the index of the region path is at or inside, -1 for none.
func (c *undoCheck) inRegion(path string) int {
	return slices.IndexFunc(c.regions, func(r string) bool { return within(path, r, pathMarks, true) })
}

// add makes path a region, the regions inside it gone; false when it is "" or already inside one.
func (c *undoCheck) add(path string) bool {
	if path == "" || c.inRegion(path) >= 0 {
		return false
	}
	c.regions = slices.DeleteFunc(c.regions, func(r string) bool { return within(r, path, pathMarks, true) })
	c.regions = append(c.regions, path)
	slices.Sort(c.regions)
	return true
}
