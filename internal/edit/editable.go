package edit

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Mode is how a value is edited (API.md W4): in a .canon source, in a JSON source, or not.
type Mode uint8

// Reason is why a value is not editable: a row of API.md §7.2.
type Reason uint8

// Op is an operation of API.md §8.3, as far as editability depends on it (§7.2 order, layer).
type Op uint8

// Editability says whether and where a value can be edited (API.md §5.2, §7).
type Editability struct {
	Mode   Mode
	Reason Reason
	File   string      // display path of the file an edit writes
	Span   source.Span // the node an edit replaces, or the literal or object a field is inserted into (W7, W9)
	Origin string      // computed: canonical path of the value's own source when it is structural, else ""
	Layer  string      // layered: the layer that sets the value (W10)
	way    string      // the refusal's way out under an edit layer, "" for none (DECISIONS 305)
}

// Editable says where op can edit r's target with edit layer editLayer (API.md §7, §7.5); ErrForeign if r is not of s.
func (s *Snapshot) Editable(r Resolved, op Op, editLayer string) (Editability, error) {
	res, err := s.reopen(r)
	switch {
	case err != nil:
		return Editability{}, err
	case res.root.enum != nil && editLayer != "":
		return Editability{Reason: ReasonLayer, way: memberWay(op, res)}, nil // W11: a member has no amendment form
	case res.root.enum != nil:
		return s.memberEditability(res), nil
	}
	j := s.judge(res, op, editLayer)
	reason := j.reason()
	if j.starErr != nil {
		return Editability{}, fmt.Errorf(fmtWrapped, ErrInternal, j.starErr)
	}
	switch reason {
	case ReasonNone:
		return j.editable(), nil
	case ReasonComputed:
		return Editability{Reason: reason, Origin: j.computedOrigin(), way: j.wayOut(reason)}, nil
	case ReasonLayered:
		return Editability{Reason: reason, Layer: j.blockedBy()}, nil
	case ReasonFormat:
		return Editability{Reason: reason, File: j.formatFile()}, nil
	default:
		return Editability{Reason: reason, way: j.wayOut(reason)}, nil
	}
}

// memberEditability is an enum member's (P7a): its declaration, which Retire edits.
func (s *Snapshot) memberEditability(res resolution) Editability {
	f := res.root.obj.File()
	d, _ := res.root.obj.Decl().(*syntax.EnumDecl)
	m, ok := res.Target.(*value.Member)
	if d == nil || f == nil || !ok || m.Index >= len(d.Members) {
		return Editability{Reason: ReasonComputed}
	}
	return Editability{Mode: ModeCanon, File: f.Src.Path, Span: f.Span(d.Members[m.Index])}
}

// state is where a path's walk stands relative to the source tree of its root (W3).
type state uint8

// cursor is the source of the value a walk has reached.
type cursor struct {
	state   state
	mode    Mode
	node    syntax.Node // canon: the literal stating the value
	file    *syntax.File
	span    source.Span
	layer   string           // layered: the amending layer; tree: the edit layer whose amendment holds the value
	files   bool             // a collection ordered by file paths: load.dir, entry declarations
	newFile bool             // a new entry goes into a file of its own (API.md N1)
	entries []item           // the root collection's entry declarations (W2)
	load    *syntax.LoadExpr // json: the load whose `at:` path reads the value (WIRE.md 6.3)
}

// ordersLiteral reports c's collection ordered by its literal for item it: no files order it,
// or it is an item of the literal that entries declared in the let's own file follow (N1, W2).
func (c cursor) ordersLiteral(it cursor) bool {
	_, declared := it.node.(*syntax.EntryDecl)
	return !c.files || !c.newFile && c.mode == ModeCanon && !declared
}

func (c cursor) to(st state) cursor {
	return cursor{state: st, mode: c.mode, file: c.file, span: c.span, layer: c.layer, load: c.load}
}

// judge is one editability question: a resolution, an op, the edit layer, and the cursor
// before each walked step (cur[0] at the root, cur[i+1] after step i).
type judge struct {
	ctx     context.Context // the caller's, when it has one: Source typing reads none
	s       *Snapshot
	res     resolution
	op      Op
	layer   string
	cur     []cursor
	rows    []func(*judge) Reason // the rows reason applies: rows, threaded into an Undo's judge
	special Reason                // the last step's own row: input, key or pseudo
	gone    bool                  // the path's lines are out, as when an Undo writes them back (undoSetReason)
	blocker string                // layered by an Undo's Set: the layer that blocks it (undoSetReason)
	at      *lineSpot             // the path's spot in the edit layer's line, once computed
	// starAt and starErr cache besideData once starDone
	starDone bool
	starAt   string
	starErr  error
}

// judge is the editability question of op on res under editLayer, judged by rows.
func (s *Snapshot) judge(res resolution, op Op, editLayer string) *judge {
	j := s.walk(res, op, editLayer)
	j.rows = rows[:]
	return j
}

// walk is the judge of res before any row: its cursors.
func (s *Snapshot) walk(res resolution, op Op, editLayer string) *judge {
	j := &judge{s: s, res: res, op: op, layer: editLayer, special: specialRow(res)}
	c := s.source(s.rootCursor(res))
	j.cur = append(j.cur, c)
	walked := len(res.Steps)
	if j.special != ReasonNone {
		walked-- // a pseudo-field, a key or an input is judged by its own row, after its container
	}
	for i := range walked {
		if p := provOf(res.Steps[i].Value); p != nil && p.Kind == value.ProvLayer {
			c = j.layerStep(i) // W10: an active layer sets the final value, whatever its base
		} else {
			c = steppers[c.state](j, c, i)
		}
		c = s.source(c)
		j.cur = append(j.cur, c)
	}
	return j
}

// source is c, or the format row for a source no edit writes, which a crashed edit's journal
// could not name: a file on a hidden path, a JSON source not named `.json` (API.md W4;
// log-2026-09-29 U4c final, M4 U5b-r, U5b-r3).
func (s *Snapshot) source(c cursor) cursor {
	if c.state != stTree {
		return c
	}
	d := s.sourcePath(c)
	switch {
	case d == "":
		return c
	case !visible(d):
		return c.to(stFormat)
	case c.mode == ModeJSON && s.files[c.span.File] == nil && !jsonFileName(d):
		return c.to(stFormat)
	}
	return c
}

// sourcePath is the display path of the file stating c's value, "" when none is known.
func (s *Snapshot) sourcePath(c cursor) string {
	switch {
	case c.mode == ModeCanon && c.file != nil:
		return c.file.Src.Path
	case c.span.End == 0:
		return ""
	}
	return s.display(c.span.File)
}

// hiddenFile is the hidden path that makes j's value not editable, "" for none (log-2026-09-29
// M4 U5b-r3).
func (j *judge) hiddenFile() string {
	if d := j.s.sourcePath(j.last()); d != "" && !visible(d) {
		return d
	}
	return ""
}

// jsonFileName reports a file named `.json`, its extension in lower case.
func jsonFileName(name string) bool {
	ext := path.Ext(name)
	return types.FormatOfPath(name) == types.FormatJSON && ext == strings.ToLower(ext)
}

func (j *judge) last() cursor { return j.cur[len(j.cur)-1] }

// reason is the first row of API.md §7.2 that applies, ReasonNone for an editable value.
func (j *judge) reason() Reason {
	for _, row := range j.rows {
		if r := row(j); r != ReasonNone {
			return r
		}
	}
	return ReasonNone
}

// computedOrigin is a computed value's Origin: its own source's path when the walk left the
// source tree, "" when an edit-layer line refuses it (lineRow), which no structure states.
func (j *judge) computedOrigin() string {
	if j.last().state != stComputed {
		return ""
	}
	return j.s.origin(j.res.Target)
}

// editable is where the edit goes: the source node, or with an edit layer that layer's file
// (W11), inside the amendment that already holds the path or an ancestor (W11a).
func (j *judge) editable() Editability {
	c := j.last()
	switch {
	case j.layer == "":
		return Editability{Mode: c.mode, File: j.s.display(c.span.File), Span: c.span}
	case c.layer == j.layer: // never inside a JSON file the amendment loads (layerRow)
		return Editability{Mode: ModeCanon, File: j.s.display(c.span.File), Span: c.span}
	}
	if e, ok := j.s.staticAmendment(j.res, j.layer); ok {
		e = j.staticVerdict(e) // a computed value there comes from the amendment's expression: no structural origin
		if e.Mode == ModeNone {
			e.way = j.wayOut(e.Reason)
		}
		return e
	}
	return Editability{Mode: ModeCanon, File: layerFile(j.res.root.pkg, j.layer)}
}

// staticVerdict is e, as Apply reads the edit layer's line the session does not show: `computed`
// when no Source states the line's part the edit rewrites (W11a; DECISIONS 273, 462), `layer` for
// a Remove of a list element inside it, which a line names no entry of (API.md W11).
func (j *judge) staticVerdict(e Editability) Editability {
	if e.Mode == ModeNone {
		return e
	}
	f, line, n := j.s.layerLine(j.res, j.layer)
	k := len(j.res.Steps)
	switch {
	case line == nil:
		return e
	case !j.s.partSource(j.ctx, staticSite{f, line, n}, j.res, j.editDepth()):
		return Editability{Reason: ReasonComputed}
	case j.op != OpRemove || n == k:
		return e
	}
	switch j.res.parent(k - 1).(type) {
	case *value.Table, *value.Map:
		return e
	}
	return Editability{Reason: ReasonLayer}
}

// statedBelow is the last cursor from k down still in the source tree.
func (j *judge) statedBelow(k int) int {
	k = min(k, len(j.cur)-1)
	for k > 0 && j.cur[k].state != stTree {
		k--
	}
	return k
}

// inEditLayer reports a path whose value the active edit layer's amendment states (W11a).
func (j *judge) inEditLayer() bool {
	c := j.cur[j.statedBelow(len(j.cur)-1)]
	return c.state == stTree && c.layer == j.layer
}

// layerFile is the package's file of layer x, or the one W11 creates in the package's directory,
// its name with dots as slashes (project.Unit.Dir).
func layerFile(pkg *check.Package, x string) string {
	if files := layerFiles(pkg, x); len(files) > 0 {
		return files[0].Src.Path
	}
	return strings.ReplaceAll(pkg.Path, string(fieldMark), pathSep) + pathSep + x + layerFileSuffix
}

// specialRow is the row the last segment itself takes: a pseudo-field (P3), an input field,
// or the key field of a keyed-list element.
func specialRow(res resolution) Reason {
	n := len(res.Steps)
	if n == 0 {
		return ReasonNone
	}
	rec, ok := res.parent(n - 1).(*value.Record)
	if !ok {
		return ReasonNone
	}
	fields := fieldsOf(rec.T)
	i := fieldIndex(fields, res.Steps[n-1].Seg.Name)
	switch {
	case i < 0:
		return ReasonPseudo
	case fields[i].Input != nil:
		return ReasonInput
	case n >= keyDepth && isKeyOf(res.parent(n-keyDepth), fields[i]):
		return ReasonKey
	}
	return ReasonNone
}

// isKeyOf reports f is the key field of the keyed list coll.
func isKeyOf(coll value.Value, f *types.Field) bool {
	l, ok := coll.(*value.List)
	if !ok {
		return false
	}
	lt, ok := l.T.Base().(*types.ListType)
	return ok && lt.KeyedBy != nil && lt.KeyedBy.Name == f.Name
}

// computedRow is a walk that left the source tree; at an expression node of the edit layer's line,
// an op that does not replace or remove it is layerRow's (DECISIONS 462).
func computedRow(j *judge) Reason {
	if j.site() == siteExprNode && !replaces(j.op) {
		return ReasonNone
	}
	return onState(j, stComputed, ReasonComputed)
}

// layeredRow is another active layer amending the path or an ancestor, whatever the stack
// (log-2026-11-01 EDIT-OWED round-5 review).
func layeredRow(j *judge) Reason {
	if j.last().state == stLayered {
		return ReasonLayered
	}
	return ReasonNone
}

// blockedBy is the layer a `layered` refusal names: the one setting the value, or the one
// blocking the Undo's Set, never the edit layer (log-2026-11-01 EDIT-OWED re-review 3).
func (j *judge) blockedBy() string {
	if j.blocker != "" {
		return j.blocker
	}
	return j.last().layer
}

func formatRow(j *judge) Reason { return onState(j, stFormat, ReasonFormat) }

// onState is r when the walk ends in st, for an edit of the base or inside the active edit
// layer's amendment: a new line, or a line the session does not show, is judged by what it writes,
// never by how the base gives the value (log-2026-11-01 EDIT-OWED re-review 4).
func onState(j *judge, st state, r Reason) Reason {
	if j.last().state == st && !j.layerWrites() {
		return r
	}
	return ReasonNone
}

func inputRow(j *judge) Reason { return onSpecial(j, ReasonInput) }

func pseudoRow(j *judge) Reason { return onSpecial(j, ReasonPseudo) }

func onSpecial(j *judge, r Reason) Reason {
	if j.special == r {
		return r
	}
	return ReasonNone
}

// keyRow is a keyed-list element's key field, and a Rename in a dependent map (E13).
func keyRow(j *judge) Reason {
	n := len(j.res.Steps)
	if j.special == ReasonKey {
		return ReasonKey
	}
	if j.op == OpRename && n > 0 {
		if _, dep := j.res.Steps[n-1].Container.Base().(*types.DepMapType); dep {
			return ReasonKey
		}
	}
	return ReasonNone
}

// orderRow is Insert into, or Move inside, a collection whose order comes from file paths, and a
// base Set of such a collection whole, which no one node states (log-2026-11-01 EDIT-OWED O).
func orderRow(j *judge) Reason {
	n := len(j.cur) - 1
	switch {
	case j.op == OpInsert && !j.cur[n].ordersLiteral(cursor{}), j.op == OpMove && n > 0 && !j.cur[n-1].ordersLiteral(j.cur[n]):
		return ReasonOrder
	case j.op == OpSet && j.layer == "" && j.cur[n].files:
		return ReasonOrder
	}
	return ReasonNone
}
