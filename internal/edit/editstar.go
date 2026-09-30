package edit

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// starRow refuses an edit of a collection an `at:` path reads past a `*` whose items hold data
// beside what the path selects: taking out, moving or adding an item would touch it, or another
// load's view of the file (log-2026-09-29 M4 B10).
func starRow(j *judge) Reason {
	if j.besideData() != "" {
		return ReasonFormat
	}
	return ReasonNone
}

// formatFile is what makes j's value format: its hidden path, else the data beside the selection.
func (j *judge) formatFile() string {
	if d := j.hiddenFile(); d != "" {
		return d
	}
	return j.besideData()
}

// besideData names, as its file and pointer, the first datum beside the selection the operation
// would touch, or the container whose items no item can state; "" for none. A source that does
// not parse is j.starErr, an internal failure.
func (j *judge) besideData() string {
	if !j.starDone {
		j.starDone = true
		j.starAt, j.starErr = j.findBeside()
	}
	return j.starAt
}

// findBeside is besideData: under the target, or for an operation on an item under its
// collection, each container a `*` reads must hold, in each item, only the steps it selects,
// none an index past 0, which no new item can be built with (log-2026-09-29 M4 B10-r2).
func (j *judge) findBeside() (string, error) {
	if o, err := j.fileBeside(); o != "" || err != nil {
		return o, err
	}
	c, v := j.last(), j.res.Target
	if n := len(j.res.Steps); itemOps[j.op] && n > 0 && n < len(j.cur) {
		c, v = j.cur[n-1], j.res.parent(n-1)
	}
	steps, p := starSteps(c.load), provOf(v)
	if steps == nil || c.state != stTree || c.mode != ModeJSON || p == nil || p.Kind != value.ProvJSON {
		return "", nil
	}
	root, err := j.jsonDoc(p)
	if err != nil {
		return "", err
	}
	stars := map[string][]atStep{}
	markStars(stars, root, steps)
	for _, ptr := range slices.Sorted(maps.Keys(stars)) {
		if ptr != p.Pointer && !strings.HasPrefix(ptr, p.Pointer+pointerSep) {
			continue
		}
		if o := unbuildable(root.Find(ptr), restOf(stars[ptr])); o != nil {
			return j.s.display(p.Span.File) + pointerFragment + o.Pointer(), nil
		}
	}
	return "", nil
}

// unbuildable is the first datum an item of container c holds beside what rest selects, or c
// itself when rest takes an index past 0; nil for none.
func unbuildable(c *jsonsrc.Node, rest []atStep) *jsonsrc.Node {
	if slices.ContainsFunc(rest, func(st atStep) bool { return st.Kind == wire.AtIndex && st.Index > 0 }) {
		return c
	}
	return besideSelection(c, rest)
}

// jsonDoc is the document of the JSON source p names, as the snapshot read it.
func (j *judge) jsonDoc(p *value.Prov) (*jsonsrc.Node, error) {
	_, root, err := parseJSONText(j.s.a.Files().Content(p.Span.File))
	if err != nil {
		return nil, fmt.Errorf(fmtWrapped, errNoTree, err)
	}
	return root, nil
}

// fileBeside is, for a Remove of a load.dir element, which deletes its whole file (API.md N6), the
// first datum of the file outside what the load's `at:` path selects; "" for none (log-2026-09-29
// M4 B10-r).
func (j *judge) fileBeside() (string, error) {
	n := len(j.res.Steps)
	if j.op != OpRemove || n == 0 || n >= len(j.cur) || !j.cur[n-1].files || j.cur[n-1].mode != ModeJSON {
		return "", nil
	}
	steps, p := atSteps(j.cur[n-1].load), provOf(j.res.Target)
	if steps == nil || p == nil || p.Kind != value.ProvJSON {
		return "", nil
	}
	root, err := j.jsonDoc(p)
	if err != nil {
		return "", err
	}
	if o := besidePath(root, steps); o != nil {
		return j.s.display(p.Span.File) + pointerFragment + o.Pointer(), nil
	}
	return "", nil
}

// besideSelection is the first datum an item of container c holds beside what rest selects,
// nil when each item holds nothing else.
func besideSelection(c *jsonsrc.Node, rest []atStep) *jsonsrc.Node {
	if c == nil {
		return nil
	}
	for _, it := range itemNodes(c) {
		if o := besidePath(it, rest); o != nil {
			return o
		}
	}
	return nil
}

// besidePath is the first member or element on the way steps lead from n that they do not take,
// each item of a `*` followed in turn; nil for none.
func besidePath(n *jsonsrc.Node, steps []atStep) *jsonsrc.Node {
	for i, st := range steps {
		if isStar(st) {
			return besideSelection(n, steps[i+1:])
		}
		next := atChild(n, st)
		if i := slices.IndexFunc(itemNodes(n), func(c *jsonsrc.Node) bool { return c != next }); i >= 0 {
			return itemNodes(n)[i]
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return nil
}
