package edit

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/wire"
)

// keptCase is what a SetCase kept for its new type to judge (API.md E14): the variant's path,
// the new case, its fields whose refinements differ, the JSON source stating the variant, and
// the .canon node stating it or its nearest stated ancestor (file, child indices, fields down).
type keptCase struct {
	path      string
	nc        *types.CaseType
	fields    []*types.Field
	file, ptr string
	tree      string
	at        []int
	down      []string
}

// setCase applies a SetCase from the settled state, then judges its kept fields together by
// the analysis of the state it leaves (API.md E1, E14, E18): the ones refused there are left
// out, to their defaults, and Dropped, by applying the SetCase again from the settled state.
func (a *applier) setCase(op Operation) error {
	if err := a.settle(); err != nil {
		return err
	}
	cp := a.checkpoint()
	if err := a.run(op, nil); err != nil {
		return err
	}
	k := a.kept
	if len(k.fields) == 0 {
		return nil
	}
	refused, err := a.refusedKept(k)
	if err != nil || len(refused) == 0 {
		return err
	}
	a.restore(cp)
	a.omit = refused
	defer func() { a.omit = nil }()
	return a.run(op, nil)
}

// refusedKept are k's fields the analysis refuses: an error of fitCodes at the field or inside
// its value, by path, by JSON pointer, or in its .canon item, so an unreadable variant (a
// static error leaves its declaration unevaluated, TYPES.md 1) is judged too.
func (a *applier) refusedKept(k keptCase) ([]string, error) {
	if err := a.settle(); err != nil {
		return nil, err
	}
	items := a.keptItems(k)
	var out []string
	for _, f := range k.fields {
		pl, ok := a.placeOf(fieldCheck{t: touched{path: k.path, file: k.file, ptr: k.ptr}, f: f})
		pl.item = items[f.Name]
		if ok && len(a.snap.refusals(pl, true)) > 0 {
			out = append(out, f.Name)
		}
	}
	return out, nil
}

// keptItems are the items of k's case literal in the current tree, by field name; none when no
// .canon literal states the variant.
func (a *applier) keptItems(k keptCase) map[string]source.Span {
	if k.tree == "" {
		return nil
	}
	f := a.snap.tree(k.tree)
	if f == nil {
		return nil
	}
	n := childAt(f, k.at)
	for _, name := range k.down {
		fi := fieldItem(n, name)
		if fi == nil {
			return nil
		}
		n = fi.Value
	}
	lit := a.snap.caseLit(n, k.nc)
	if lit == nil {
		return nil
	}
	out := map[string]source.Span{}
	for _, it := range lit.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil {
			out[fi.Name.Name] = f.Span(fi)
		}
	}
	return out
}

// caseLit is the first brace literal under n, n included, whose type is case nc's.
func (s *Snapshot) caseLit(n syntax.Node, nc *types.CaseType) *syntax.BraceLit {
	if n == nil {
		return nil
	}
	if b, ok := n.(*syntax.BraceLit); ok {
		if c, isCase := s.info.Types[b].(*types.CaseType); isCase && c.Name == nc.Name && c.Variant.String() == nc.Variant.String() {
			return b
		}
	}
	for c := range syntax.Children(n) {
		if lit := s.caseLit(c, nc); lit != nil {
			return lit
		}
	}
	return nil
}

// nodePath is the child indices leading from root to n; false when n is not under root.
func nodePath(root, n syntax.Node) ([]int, bool) {
	if root == n {
		return nil, true
	}
	i := 0
	for c := range syntax.Children(root) {
		if c != nil {
			if at, ok := nodePath(c, n); ok {
				return append([]int{i}, at...), true
			}
		}
		i++
	}
	return nil, false
}

// childAt is the node the child indices at lead to from root; nil when the tree has no such node.
func childAt(root syntax.Node, at []int) syntax.Node {
	n := root
	for _, want := range at {
		var next syntax.Node
		i := 0
		for c := range syntax.Children(n) {
			if i == want {
				next = c
				break
			}
			i++
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// fitError reports an error of fitCodes (log-2026-09-29 M4 U4b-r3).
func fitError(f diag.Finding) bool {
	return f.Severity == diag.Error && fitCodes[f.Code]
}

// checkpoint is the applier's state before an operation, which a SetCase goes back to when it
// leaves some of its kept fields out (API.md E14).
type checkpoint struct {
	files                          map[string]fileState
	snap                           *Snapshot
	host                           wire.Host
	dirty                          bool
	undo, dropped, records, locked int
	owners, emptied                map[string]string
}

func (a *applier) checkpoint() checkpoint {
	cp := checkpoint{
		files: make(map[string]fileState, len(a.files)), snap: a.snap, host: a.host, dirty: a.dirty,
		undo: len(a.undo), dropped: len(a.dropped), records: len(a.records), locked: len(a.locked),
		owners: maps.Clone(a.owners), emptied: maps.Clone(a.emptied),
	}
	for d, s := range a.files { //canon:unordered copies a map
		cp.files[d] = *s
	}
	return cp
}

// restore goes back to cp; the writes made since are forgotten, and appending to a restored
// file's writes never reaches cp's.
func (a *applier) restore(cp checkpoint) {
	a.files = make(map[string]*fileState, len(cp.files))
	for d, s := range cp.files { //canon:unordered copies a map
		s.steps = slices.Clip(s.steps)
		a.files[d] = &s
	}
	a.snap, a.host, a.dirty = cp.snap, cp.host, cp.dirty
	a.undo, a.dropped, a.records, a.locked = a.undo[:cp.undo], a.dropped[:cp.dropped], a.records[:cp.records], a.locked[:cp.locked]
	a.owners, a.emptied, a.kept = maps.Clone(cp.owners), maps.Clone(cp.emptied), keptCase{}
}
