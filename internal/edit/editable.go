package edit

import (
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
}

// Editable says where op can edit r's target with edit layer editLayer (API.md §7, §7.5); ErrForeign if r is not of s.
func (s *Snapshot) Editable(r Resolved, op Op, editLayer string) (Editability, error) {
	res, err := s.reopen(r)
	switch {
	case err != nil:
		return Editability{}, err
	case res.root.enum != nil && editLayer != "":
		return Editability{Reason: ReasonLayer}, nil // W11: a member has no amendment form
	case res.root.enum != nil:
		return s.memberEditability(res), nil
	}
	j := s.judge(res, op, editLayer)
	switch reason := j.reason(); reason {
	case ReasonNone:
		return j.editable(), nil
	case ReasonComputed:
		return Editability{Reason: reason, Origin: s.origin(r.Target)}, nil
	case ReasonLayered:
		return Editability{Reason: reason, Layer: j.last().layer}, nil
	default:
		return Editability{Reason: reason}, nil
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
	layer   string // layered: the amending layer; tree: the edit layer whose amendment holds the value
	files   bool   // a collection ordered by file paths: load.dir, entry declarations
	entries []item // the root collection's entry declarations (W2)
}

func (c cursor) to(st state) cursor {
	return cursor{state: st, mode: c.mode, file: c.file, span: c.span, layer: c.layer}
}

// judge is one editability question: a resolution, an op, the edit layer, and the cursor
// before each walked step (cur[0] at the root, cur[i+1] after step i).
type judge struct {
	s       *Snapshot
	res     resolution
	op      Op
	layer   string
	cur     []cursor
	special Reason // the last step's own row: input, key or pseudo
}

func (s *Snapshot) judge(res resolution, op Op, editLayer string) *judge {
	j := &judge{s: s, res: res, op: op, layer: editLayer, special: specialRow(res)}
	c := s.rootCursor(res)
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
		j.cur = append(j.cur, c)
	}
	return j
}

func (j *judge) last() cursor { return j.cur[len(j.cur)-1] }

// reason is the first row of API.md §7.2 that applies, ReasonNone for an editable value.
func (j *judge) reason() Reason {
	for _, row := range rows {
		if r := row(j); r != ReasonNone {
			return r
		}
	}
	return ReasonNone
}

// editable is where the edit goes: the source node, or with an edit layer that layer's file
// (W11), inside the amendment that already holds the path or an ancestor (W11a).
func (j *judge) editable() Editability {
	c := j.last()
	switch {
	case j.layer == "":
		return Editability{Mode: c.mode, File: j.s.display(c.span.File), Span: c.span}
	case c.layer == j.layer:
		mode := ModeCanon
		if c.mode == ModeJSON && j.s.files[c.span.File] == nil {
			mode = ModeJSON // inside a JSON file the amendment loads
		}
		return Editability{Mode: mode, File: j.s.display(c.span.File), Span: c.span}
	}
	if e, ok := j.s.staticAmendment(j.res, j.layer); ok {
		return e // a computed value there comes from the amendment's expression: no structural origin
	}
	return Editability{Mode: ModeCanon, File: layerFile(j.res.root.pkg, j.layer)}
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

func computedRow(j *judge) Reason { return onState(j, stComputed, ReasonComputed) }

func layeredRow(j *judge) Reason { return onState(j, stLayered, ReasonLayered) }

func formatRow(j *judge) Reason { return onState(j, stFormat, ReasonFormat) }

func onState(j *judge, st state, r Reason) Reason {
	if j.last().state == st {
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

// orderRow is Insert into, or Move inside, a collection whose order comes from file paths.
func orderRow(j *judge) Reason {
	n := len(j.cur) - 1
	switch {
	case j.op == OpInsert && j.cur[n].files, j.op == OpMove && n > 0 && j.cur[n-1].files:
		return ReasonOrder
	}
	return ReasonNone
}

// layerRow refuses, under an edit layer, all but Set, Reset and AddEntry below a let (EVALUATION.md §9.2).
func layerRow(j *judge) Reason {
	if j.layer == "" {
		return ReasonNone
	}
	amendable := j.op == OpSet || j.op == OpReset || j.op == OpAddEntry
	rootValue := len(j.res.Steps) == 0 && j.op != OpAddEntry // an entry added to a root table or map is an amendment (EVALUATION.md §9.3)
	if !amendable || rootValue || j.res.root.obj.Kind() != check.ObjLet {
		return ReasonLayer
	}
	return ReasonNone
}
