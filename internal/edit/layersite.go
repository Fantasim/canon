package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// site is where a path stands in the edit layer's line at or above it, active or not (API.md W11a;
// DECISIONS 462).
type site uint8

// loads reports a site at or below a load: its data lies outside the layer file (DECISIONS 411).
func (st site) loads() bool { return st == siteLoadNode || st == siteBelowLoad }

// lineSpot is the edit layer's line at or above a path and the first node on the path, in that
// line's right-hand side, that is no literal (a load or an expression), read from the layer's text.
type lineSpot struct {
	line  *syntax.Amendment
	n     int         // the line's path length
	node  syntax.Node // nil: no line, or its literal states the whole path
	depth int         // the steps from the root to node
}

// spot is j's spot, computed once; none for no edit layer, or with the path's lines out (gone).
func (j *judge) spot() lineSpot {
	if j.at == nil {
		at := lineSpot{}
		if j.layer != "" && !j.gone {
			at = j.s.spotOf(j.res, j.layer)
		}
		j.at = &at
	}
	return *j.at
}

// spotOf is res's spot in layer x.
func (s *Snapshot) spotOf(res resolution, x string) lineSpot {
	_, line, n := s.layerLine(res, x)
	at := lineSpot{line: line, n: n}
	if line == nil {
		return at
	}
	var node syntax.Node = syntax.Unparen(line.Value)
	for i := n; ; i++ {
		if s.nodeForm(node) != shape.FormLiteral {
			at.node, at.depth = node, i
			return at
		}
		next, ok := s.childOn(node, res, i)
		if !ok {
			return at
		}
		node = next
	}
}

// childOn is node's child for step i of res, false past the path's end or for one it lacks.
func (s *Snapshot) childOn(node syntax.Node, res resolution, i int) (syntax.Node, bool) {
	if i == len(res.Steps) {
		return nil, false
	}
	return childNode(node, res.Steps[i])
}

// site is j's site: one rule for every load or expression, line or nested (DECISIONS 462).
func (j *judge) site() site {
	at := j.spot()
	k := len(j.res.Steps)
	switch {
	case at.node == nil:
		return siteNone
	case j.s.isLoad(at.node) && at.depth == k:
		return siteLoadNode
	case j.s.isLoad(at.node):
		return siteBelowLoad
	case at.depth == k:
		return siteExprNode
	}
	return siteBelowExpr
}

// exprRow is `computed` where a session that does not walk the edit layer's line meets an
// expression of it, as one that walks it does (computedRow): below it, or a Set, Reset or Remove
// of it (DECISIONS 462).
func exprRow(j *judge) Reason {
	st := j.site()
	if j.layerWrites() && (st == siteBelowExpr || st == siteExprNode && replaces(j.op)) {
		return ReasonComputed
	}
	return ReasonNone
}

// siteRefuses reports layerRow's share of the site (DECISIONS 462): any op below a load, and at a
// load or expression node any op but a Set, Reset or Remove, which lineRow and the rows above take.
func (j *judge) siteRefuses() bool {
	st := j.site()
	return st == siteBelowLoad || (st == siteLoadNode || st == siteExprNode) && !replaces(j.op)
}

// replaces reports an op that replaces or removes its target's node: Set, Reset, Remove.
func replaces(op Op) bool { return op == OpSet || op == OpReset || op == OpRemove }

// wayOut is the way out of refusal r under an edit layer (DECISIONS 305, 462): the data way for
// every `layer` at or below a load, an edit way only where it makes the op pass.
func (j *judge) wayOut(r Reason) string {
	if j.layer == "" {
		return ""
	}
	st := j.site()
	switch {
	case r == ReasonLayer && st.loads():
		return loadWay(st == siteBelowLoad || j.op != OpRename && j.op != OpMove, j.literalPasses())
	case (r == ReasonComputed || r == ReasonLayer) && st != siteNone && j.literalPasses():
		return wayLiteral
	case r == ReasonLayer && j.noLayerPasses():
		// DECISIONS 462's one difference between the states: with the layer active the base
		// edit is `layered`, so this way prints only where the layer does not set the value.
		return wayNoLayer
	}
	return ""
}

// loadWay is the way out of `layer` at or below a load: its data file, where the op acts on the
// data (a Rename or Move of the load's own node acts on the layer file), and the literal in place.
func loadWay(data, literal bool) string {
	switch {
	case !data:
		return ""
	case literal:
		return wayLoad
	}
	return wayData
}

// literalPasses reports DECISIONS 462's allowlist holding (literalOps): with a literal of its value
// in place of the load or expression, the op passes: a Set, a Reset the line allows (resetReason),
// an AddEntry into a table or map, a Remove of an entry the layer states (W11) of no stable table.
func (j *judge) literalPasses() bool {
	at := j.spot()
	if at.node == nil || j.special != ReasonNone || j.blind() || !j.literalAfter(at) {
		return false
	}
	return int(j.op) < len(literalOps) && literalOps[j.op] != nil && literalOps[j.op](j)
}

// literalAfter reports a part of the line for the edit to rewrite (partSource) once at's node is a
// literal: a node on the path, at most the edit's depth down, whose one load or expression it is.
func (j *judge) literalAfter(at lineSpot) bool {
	for d := at.n; d <= min(max(j.editDepth(), at.n), at.depth); d++ {
		if node, ok := j.s.nodeAt(at.line, at.n, j.res, d); ok && j.s.opaqueIn(node) == 1 {
			return true
		}
	}
	return false
}

// blindRow is `computed`, no way out, for an edit an edit layer the session does not apply writes in
// a root whose declared type holds a dependent type: no driver of that layer is seen (DECISIONS 273, 462).
func blindRow(j *judge) Reason {
	if j.blind() {
		return ReasonComputed
	}
	return ReasonNone
}

// blind reports blindRow's edit: a Set, Reset, AddEntry or Remove through an inactive edit layer.
func (j *judge) blind() bool {
	return j.writesLine() && !j.s.layerActive(j.layer) && holdsDependent(j.res.root.obj.Type())
}

// noLayerPasses reports the same op passing without an edit layer: the rows, and its target
// taking it (fits), as Apply would make it (DECISIONS 462).
func (j *judge) noLayerPasses() bool {
	b := j.s.judge(j.res, j.op, "")
	b.ctx = j.ctx
	return b.reason() == ReasonNone && b.starErr == nil && j.s.fits(j.op, j.res)
}

// opaqueIn counts the nodes of n no Source states, none below one: loads, expressions, spreads.
func (s *Snapshot) opaqueIn(n syntax.Node) int {
	if s.nodeForm(n) != shape.FormLiteral {
		return 1
	}
	k := 0
	if hasSpread(n) {
		k++
	}
	for _, it := range items(n, nil) {
		k += s.opaqueIn(it.node)
	}
	return k
}

// isLoad reports n a load of any format, whose data lies outside the file stating it.
func (s *Snapshot) isLoad(n syntax.Node) bool {
	f := s.nodeForm(n)
	return f == shape.FormJSON || f == shape.FormFormat
}

func literalSet(*judge) bool { return true }

func literalReset(j *judge) bool { return resetReason(j) == ReasonNone }

func literalAddEntry(j *judge) bool { return fitsEntries(j.s, j.res) }

func literalRemove(j *judge) bool { return inEntries(j.res) && j.removable() && !stableIn(j.res) }

// editDepth is how far down the path lies the value an edit rewrites inside a line: the collection
// for a Remove, and for a Reset of an item the value lacks without the layer (dropEntry), else the target.
func (j *judge) editDepth() int {
	k := len(j.res.Steps)
	if j.op == OpRemove {
		return k - 1
	}
	if _, kept := j.s.withoutLayer(j.res, j.layer); j.op == OpReset && !kept {
		return k - 1
	}
	return k
}
