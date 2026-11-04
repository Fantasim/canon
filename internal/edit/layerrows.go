package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// removable reports an item the edit layer states, through its active amendment or in one of its
// lines, which a Remove takes out of its text; not the path's own line at a key the value has
// without the layer (API.md W11; log-2026-11-01 EDIT-OWED re-review 1).
func (j *judge) removable() bool {
	_, line, n := j.s.layerLine(j.res, j.layer)
	if line != nil && n == len(j.res.Steps) {
		_, kept := j.s.withoutLayer(j.res, j.layer)
		return !kept
	}
	c := j.last()
	return line != nil || c.state == stTree && c.layer == j.layer
}

// layerWrites reports an edit of the edit layer's text that does not walk through its active
// amendment: a new line, or a line the session does not show, which the base's rows do not judge.
func (j *judge) layerWrites() bool {
	return j.layer != "" && !slices.ContainsFunc(j.cur, func(c cursor) bool { return c.layer == j.layer })
}

// layerRow refuses, under an edit layer, all but Set, Reset and AddEntry below a let (EVALUATION.md
// 9.2), any op below a load of the edit layer's line, its data outside the layer file (API.md W11a;
// DECISIONS 411), and at a load or expression node any op but a Set, Reset or Remove (DECISIONS 462).
func layerRow(j *judge) Reason {
	if j.layer == "" {
		return ReasonNone
	}
	amendable := j.op == OpSet || j.op == OpReset || j.op == OpAddEntry || j.op == OpRemove && j.removable()
	rootValue := len(j.res.Steps) == 0 && j.op != OpAddEntry // an entry added to a root table is an amendment (EVALUATION.md §9.3)
	if !amendable || rootValue || j.res.root.obj.Kind() != check.ObjLet || j.siteRefuses() || j.noAmendForm() || j.unseenEntry() || j.unseenLine() {
		return ReasonLayer
	}
	return ReasonNone
}

// memberWay is the way out of an enum member's Retire under an edit layer: the Retire without it.
func memberWay(op Op, res resolution) string {
	if _, err := retireMember(res); op != OpRetire || err != nil {
		return ""
	}
	return wayNoLayer
}

// writesLine reports an op that could write back an edit-layer line: a Set, Reset, AddEntry or Remove.
func (j *judge) writesLine() bool {
	return j.layer != "" && !j.gone && (j.op == OpSet || j.op == OpReset || j.op == OpAddEntry || j.op == OpRemove)
}

// noLiteral reports the path's own line, whose op is a Set, Reset or Remove, one that has no
// literal form: its Undo would write back a line no Source states (DECISIONS 273).
func (j *judge) noLiteral(f *syntax.File, line *syntax.Amendment) bool {
	return j.op != OpAddEntry && !j.s.lineSource(j.ctx, f, j.res.root.at(), line)
}

// lineRow is computed for an edit whose Undo would write back an edit-layer line no Source states
// (DECISIONS 273): a Set, Reset or Remove of the path's own line or of a load nested in a line's
// literal (DECISIONS 462), a Set replacing one below it, an edit beside an active line's spread.
func lineRow(j *judge) Reason {
	if !j.writesLine() {
		return ReasonNone
	}
	f, line, n := j.s.layerLine(j.res, j.layer)
	switch {
	case line == nil && j.op == OpSet && j.s.computedBelow(j.ctx, j.res, j.layer):
		return ReasonComputed
	case line == nil:
		return ReasonNone
	case n == len(j.res.Steps) && j.noLiteral(f, line):
		return ReasonComputed
	case n < len(j.res.Steps) && j.op != OpAddEntry && j.site() == siteLoadNode:
		return ReasonComputed
	case n < len(j.res.Steps) && j.last().state == stSpread && j.inEditLayer():
		return ReasonComputed
	}
	return ReasonNone
}

// undoRow is, under an edit layer, the refusal a Set its Undo writes would get (log-2026-11-01
// EDIT-OWED review): a Reset's, of the path's own line or inside a line (resetReason); a Set's,
// replacing lines below the path, each written back once its value is the one without the layer.
func undoRow(j *judge) Reason {
	switch {
	case j.layer == "" || j.gone:
		return ReasonNone
	case j.op == OpReset:
		return resetReason(j)
	case j.op == OpSet && j.newLine():
		return j.belowReason()
	}
	return ReasonNone
}

// belowReason is the first refusal a Set writing back a line of the edit layer below the path gets.
func (j *judge) belowReason() Reason {
	r, reason := j.res.root.at(), ReasonNone
	j.s.lineBelow(j.res, j.layer, func(f *syntax.File, it *syntax.Amendment) bool {
		p, err := Parse(j.s.linePathOf(f, r, it.Path))
		if err != nil {
			return false
		}
		res, err := j.s.open(p)
		if err != nil {
			return false
		}
		if base, kept := j.s.withoutLayer(res, j.layer); kept {
			reason = j.undoSetReason(res, base)
		}
		return reason != ReasonNone
	})
	return reason
}

// resetReason is a Reset's refusal: of the path's own line, what the Set its Undo writes gets
// without that line; inside a line, `computed` for a base value no literal states, `layer` for an
// item the value lacks without the layer that is no map or table entry, nor a defaulted field.
func resetReason(j *judge) Reason {
	_, line, n := j.s.layerLine(j.res, j.layer)
	if line == nil {
		return ReasonNone
	}
	k := len(j.res.Steps)
	base, kept := j.s.withoutLayer(j.res, j.layer)
	switch {
	case n == k && kept:
		return j.undoSetReason(j.res, base)
	case kept && !printable(base):
		return ReasonComputed // no literal states the base value (W11)
	case n == k || kept || dropsOut(j.res):
		return ReasonNone
	}
	return ReasonLayer
}

// dropsOut reports the item at res's path one a literal can leave out: an entry of a table or
// map, or a field with a default, which leaving it out gives.
func dropsOut(res resolution) bool {
	k := len(res.Steps)
	switch p := res.parent(k - 1).(type) {
	case *value.Table, *value.Map:
		return true
	case *value.Record:
		fields := fieldsOf(p.T)
		i := fieldIndex(fields, res.Steps[k-1].Seg.Name)
		return i >= 0 && hasDefault(fields[i])
	}
	return false
}

// undoSetReason is the reason the Undo's Set writing the path's line back gets once its value is
// base, judged by j's rows: a new line no amend path names when its first step starts none (API.md
// E22, 6.4; startsLine), `layered` where another layer amends it (log-2026-11-01 EDIT-OWED round 7).
func (j *judge) undoSetReason(res resolution, base value.Value) Reason {
	if !startsLine(res.Steps[0]) {
		return ReasonLayer
	}
	res.Steps = slices.Clone(res.Steps)
	res.Steps[len(res.Steps)-1].Value = base
	res.Target = base
	u := j.s.walk(res, OpSet, j.layer)
	u.ctx, u.rows, u.gone = j.ctx, j.rows, true
	r := u.reason()
	if r == ReasonLayered {
		j.blocker = u.blockedBy()
	}
	return r
}

// unseenLine reports a Set under an edit layer the session does not apply that replaces a line
// below the path naming an entry the session does not show: its Undo needs an AddEntry no path
// reaches (API.md W11a; log-2026-11-01 EDIT-OWED review).
func (j *judge) unseenLine() bool {
	return j.op == OpSet && !j.gone && !j.s.layerActive(j.layer) && j.newLine() && j.s.unseenBelow(j.res, j.layer)
}

// newLine reports an edit the edit layer holds no amendment for: it writes a line of its own.
func (j *judge) newLine() bool {
	if j.inEditLayer() {
		return false
	}
	if j.gone {
		return true
	}
	_, line, _ := j.s.layerLine(j.res, j.layer)
	return line == nil
}

// unseenEntry reports an AddEntry writing a new line of an edit layer the session does not
// apply: no path reaches the entry to undo it (API.md W11a; DECISIONS 273).
func (j *judge) unseenEntry() bool {
	return j.op == OpAddEntry && !j.s.layerActive(j.layer) && j.newLine()
}

// noAmendForm reports a Set or AddEntry that writes a new amendment line whose path cannot start
// with a name (startsLine): below a root map or list (except a String-keyed list's word key,
// DECISIONS 450), or an entry of a root map (API.md 6.4, GRAMMAR.md 5.7).
func (j *judge) noAmendForm() bool {
	if (j.op != OpSet && j.op != OpAddEntry) || !j.newLine() {
		return false
	}
	if len(j.res.Steps) > 0 {
		return !startsLine(j.res.Steps[0])
	}
	_, isMap := j.res.Target.(*value.Map)
	return isMap
}
