package edit

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// layerUndo is the Undo under an edit layer, which writes only that layer's lines (API.md W11):
// E23's inverses when they give back its lines and every value, else each line the edit changed
// set back as it was, or Reset where there was none; never a merged value (log-2026-10-01 M4.1).
func (c *undoCheck) layerUndo() ([]Operation, error) {
	if ok, err := c.layerVerified(c.plain); ok || err != nil {
		return c.plain, err
	}
	ops, err := c.lineRestore()
	if err != nil {
		return nil, err
	}
	if ok, err := c.layerVerified(ops); ok || err != nil {
		return ops, err
	}
	return nil, fmt.Errorf(fmtWrapped, ErrInternal, errUndoUnverified)
}

// layerVerified reports ops, applied in memory, refusing nothing and giving back every value and
// every line of the edit layer.
func (c *undoCheck) layerVerified(ops []Operation) (bool, error) {
	failed, err := c.run(ops)
	if err != nil {
		return false, err
	}
	return len(failed) == 0 && len(c.misses()) == 0 && maps.Equal(c.lines(c.a.base), c.lines(c.last)), nil
}

// lineRestore gives the edit layer its lines back: added lines Reset, created entries put back in
// order (rulings c, d), changed lines written again, drivers first; a line no Source writes is
// refused as W5 refuses its Reset (computed).
func (c *undoCheck) lineRestore() ([]Operation, error) {
	before, after := c.lineTexts(c.a.base), c.lineTexts(c.after)
	readd, drop := c.entryOrder()
	if len(readd)+len(drop) > 0 && !c.layerSeen() {
		return nil, &NotEditableError{Reason: ReasonLayer} // an inactive layer's entries: no path reaches them (W5)
	}
	computed := c.computedLines()
	var out, adds, sets []Operation
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if _, kept := before[path]; !kept && !slices.Contains(drop, path) {
			out = append(out, Operation{Kind: OpReset, Path: path})
		}
	}
	for _, path := range drop {
		out = append(out, Operation{Kind: OpRemove, Path: path})
	}
	for _, path := range slices.Sorted(maps.Keys(before)) {
		a, ok := after[path]
		switch {
		case !slices.Contains(readd, path) && ok && flatText(a) == flatText(before[path]):
			continue
		case computed[path]:
			return nil, &NotEditableError{Reason: ReasonComputed}
		case slices.Contains(readd, path):
			continue
		}
		sets = append(sets, c.lineWrite(path, before[path], false))
	}
	for _, path := range readd {
		adds = append(adds, c.lineWrite(path, before[path], true))
	}
	slices.SortStableFunc(sets, func(x, y Operation) int { return boolRank(c.typedByLine(x.Path)) - boolRank(c.typedByLine(y.Path)) })
	return slices.Concat(out, adds, sets), nil
}

// entryOrder are the entry lines to add again, in their order before the edit, and those to
// remove first: both from the first entry line where the edit's result differs.
func (c *undoCheck) entryOrder() (readd, drop []string) {
	eb, ea := c.createdLines(c.a.base), c.createdLines(c.after)
	k := 0
	for k < len(eb) && k < len(ea) && eb[k] == ea[k] {
		k++
	}
	return eb[k:], ea[k:]
}

// createdLines are the edit layer's lines in s, in order, each creating an entry of a compared
// root's table literal that its own sources do not state (LAY-02).
func (c *undoCheck) createdLines(s *Snapshot) []string {
	var out []string
	for _, r := range c.roots {
		keys, ok := literalKeys(s, r)
		if !ok {
			continue
		}
		for _, path := range c.lineOrder(s) {
			p, err := Parse(path)
			if err == nil && rootPath(p) == r.String() && len(p.Segs) == 1 && !keys[segName(p.Segs[0])] {
				out = append(out, path)
			}
		}
	}
	return out
}

// literalKeys are the keys root r's own sources state: its table literal's entries and its
// entry declarations; false for a root that is no table literal.
func literalKeys(s *Snapshot, r Path) (map[string]bool, bool) {
	root, err := s.lookup(r)
	if err != nil || root.enum != nil {
		return nil, false
	}
	let, ok := root.obj.Decl().(*syntax.LetDecl)
	if !ok {
		return nil, false
	}
	lit, isLit := syntax.Unparen(let.Value).(*syntax.BraceLit)
	if _, isTable := baseOf(root.obj.Type()).(*types.TableType); !isLit || !isTable {
		return nil, false
	}
	keys := map[string]bool{}
	for _, it := range lit.Items {
		if ei, isEntry := it.(*syntax.EntryItem); isEntry && ei.Key != nil {
			keys[ei.Key.Name] = true
		}
	}
	for _, f := range root.pkg.Files {
		entryKeys(f, r.Root, keys)
	}
	return keys, true
}

// entryKeys adds to keys those of f's entry declarations of the table root.
func entryKeys(f *syntax.File, root string, keys map[string]bool) {
	for _, d := range f.Decls {
		if e, isEntry := d.(*syntax.EntryDecl); isEntry && e.Table != nil && e.Table.Name == root && e.Key != nil {
			sp := f.Span(e.Key)
			keys[string(f.Src.Content[sp.Start:sp.End])] = true
		}
	}
}

// segName is a segment's name or key text.
func segName(s Seg) string {
	if s.Kind == SegKey {
		return s.Key.Text
	}
	return s.Name
}

// rootPath is p's root as a path.
func rootPath(p Path) string {
	return Path{Package: p.Package, Root: p.Root}.String()
}

// computedLines are the edit layer's lines in the base, amending the compared roots, whose text
// does not type as a Source at its path: no operation writes them (W5 computed).
func (c *undoCheck) computedLines() map[string]bool {
	out := map[string]bool{}
	for path, text := range c.lineTexts(c.a.base) { //canon:unordered fills a set
		out[path] = !c.typesAsSource(path, text)
	}
	return out
}

// typesAsSource reports text a value of the declared type at path, read in the base, an entry's
// line by its collection's item type.
func (c *undoCheck) typesAsSource(path, text string) bool {
	_, ok := c.lineTyped(path, text)
	return ok
}

// lineTyped is text typed as a Source at path, as typesAsSource reads it.
func (c *undoCheck) lineTyped(path, text string) (value.Value, bool) {
	t, pkg, ok := c.lineType(path)
	if !ok {
		return nil, false
	}
	ty := Typer{Host: c.a.baseHost, Pkg: pkg, marks: c.a.marks}
	v, err := ty.Value(c.a.ctx, Source(flatText(text)), t)
	return v, err == nil
}

// lineValue is the line at path as verification compares it (API.md E22, W11): the base kind and
// canonical text of the value it types to, so a spelling the formatter keeps (parentheses, a
// number's digits, a multiline string) compares equal; a line no Source writes by its text.
func (c *undoCheck) lineValue(path, text string) string {
	if v, ok := c.lineTyped(path, text); ok {
		if canon, err := c.a.canonText(v); err == nil {
			return strconv.Itoa(int(v.Type().Base().Kind())) + newline + canon
		}
	}
	return flatText(text)
}

// lineType is the declared type of the value at path in the base, or of an item of the
// collection holding it when the base has no such item, and its package.
func (c *undoCheck) lineType(path string) (types.Type, string, bool) {
	p, err := Parse(path)
	if err != nil {
		return nil, "", false
	}
	if res, err := c.a.base.open(p); err == nil {
		t, err := c.a.base.Type(res.Resolved)
		return t, res.root.pkg.Path, err == nil && t != nil
	}
	parent, ok := enclosing(path)
	if !ok {
		return nil, "", false
	}
	pp, _ := Parse(parent)
	res, err := c.a.base.open(pp)
	if err != nil {
		return nil, "", false
	}
	t, err := c.a.base.Type(res.Resolved)
	if err != nil || t == nil {
		return nil, "", false
	}
	if _, vt, isMap := mapTypes(present(t)); isMap {
		return vt, res.root.pkg.Path, true
	}
	et := elemType(present(t))
	return et, res.root.pkg.Path, et != nil
}

// lineWrite writes the line at path with text again: a Set, or with add, or where the edit's
// result has no such entry, an AddEntry of the entry the line creates.
func (c *undoCheck) lineWrite(path, text string, add bool) Operation {
	lit := Source(flatText(text))
	p, err := Parse(path)
	if err != nil || len(p.Segs) == 0 {
		return Operation{Kind: OpSet, Path: path, Value: lit}
	}
	if _, err := c.after.open(p); err == nil && !add {
		return Operation{Kind: OpSet, Path: path, Value: lit}
	}
	last := p.Segs[len(p.Segs)-1]
	key := last.Name
	if last.Kind == SegKey {
		key = last.Key.Text
	}
	parent, _ := enclosing(path)
	return Operation{Kind: OpAddEntry, Path: parent, Key: PathKey(key), Value: lit}
}

// typedByLine reports the line at path setting a field whose type a value computes.
func (c *undoCheck) typedByLine(path string) bool {
	p, err := Parse(path)
	if err != nil {
		return false
	}
	res, err := c.after.open(p)
	if err != nil {
		res, err = c.a.base.open(p)
	}
	if err != nil {
		return false
	}
	f := (&opCtx{res: res}).fieldAt(len(res.Steps))
	return f != nil && computed(f.Type)
}

// lines are the edit layer's lines in s amending the compared roots, path to canonical text:
// spacing aside, string literals kept; a line also names the earlier lines on paths overlapping its
// own, whose order is part of the layer's view (log-2026-10-01 M4.1 rulings).
func (c *undoCheck) lines(s *Snapshot) map[string]string {
	texts, order := c.lineTexts(s), c.lineOrder(s)
	out := map[string]string{}
	if !c.layerSeen() {
		out[""] = strings.Join(c.createdLines(s), newline) // the entries' order, which no value shows (ruling c)
	}
	for path, text := range texts { //canon:unordered fills a map by path
		var before []string
		for _, q := range order {
			if q == path {
				break
			}
			if within(q, path, pathMarks, true) || within(path, q, pathMarks, true) {
				before = append(before, q)
			}
		}
		out[path] = c.lineValue(path, text) + newline + strings.Join(before, newline)
	}
	return out
}

// layerSeen reports the base's values showing the edit layer: a value at one of its lines' paths
// comes from it, so the order of the entries it creates is compared there (E22).
func (c *undoCheck) layerSeen() bool {
	return slices.ContainsFunc(c.lineOrder(c.a.base), func(path string) bool {
		p, err := Parse(path)
		if err != nil {
			return false
		}
		res, err := c.a.base.open(p)
		return err == nil && res.Target != nil && res.Target.Prov() != nil && res.Target.Prov().Layer == c.a.env.EditLayer
	})
}

// lineOrder are the paths of the edit layer's lines in s amending the compared roots, in order.
func (c *undoCheck) lineOrder(s *Snapshot) []string {
	var out []string
	c.eachLayerLine(s, func(path string, _ *syntax.File, _ *syntax.Amendment) { out = append(out, path) })
	return out
}

// lineTexts are the edit layer's lines in s amending the compared roots: path to text.
func (c *undoCheck) lineTexts(s *Snapshot) map[string]string {
	out := map[string]string{}
	c.eachLayerLine(s, func(path string, f *syntax.File, it *syntax.Amendment) {
		sp := f.Span(it.Value)
		out[path] = string(f.Src.Content[sp.Start:sp.End])
	})
	return out
}

// eachLayerLine calls do with each line of the edit layer in s amending a compared root.
func (c *undoCheck) eachLayerLine(s *Snapshot, do func(string, *syntax.File, *syntax.Amendment)) {
	for _, r := range c.roots {
		pkg := s.byPath[r.Package]
		if pkg == nil {
			continue
		}
		for _, f := range layerFiles(pkg, c.a.env.EditLayer) {
			eachLine(f, r, func(path string, it *syntax.Amendment) { do(path, f, it) })
		}
	}
}

// eachLine calls do with each line of f amending root r, and the path it amends as an operation
// names it.
func eachLine(f *syntax.File, r Path, do func(string, *syntax.Amendment)) {
	for _, b := range f.Amends {
		if b.Target == nil || b.Target.Name != r.Root {
			continue
		}
		for _, it := range b.Items {
			if len(it.Path) == 0 {
				continue
			}
			from, to := f.Span(it.Path[0]), f.Span(it.Path[len(it.Path)-1])
			rel := string(f.Src.Content[from.Start:to.End])
			if !strings.HasPrefix(rel, string(bracketOpen)) {
				rel = string(fieldMark) + rel
			}
			do(r.String()+rel, it)
		}
	}
}

// layerInactive reports the edit layer amending x's root without setting the value at any of its
// lines' paths: the analysis does not see it, so an arm computed there is not the layer's (W11a).
func (x *opCtx) layerInactive() bool {
	root := Path{Package: x.res.root.pkg.Path, Root: x.res.root.obj.Name()}
	amends, seen := false, false
	for _, f := range layerFiles(x.res.root.pkg, x.a.env.EditLayer) {
		eachLine(f, root, func(path string, _ *syntax.Amendment) {
			amends = true
			p, err := Parse(path)
			if err != nil {
				return
			}
			if res, err := x.a.snap.open(p); err == nil && res.Target != nil && res.Target.Prov() != nil {
				seen = seen || res.Target.Prov().Layer == x.a.env.EditLayer
			}
		})
	}
	return amends && !seen
}
