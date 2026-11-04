package edit_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/edit"
)

var parityOpNames = map[edit.Op]string{
	edit.OpSet: "set", edit.OpReset: "reset", edit.OpAdd: "add", edit.OpInsert: "insert",
	edit.OpAddEntry: "addEntry", edit.OpRemove: "remove", edit.OpMove: "move", edit.OpRename: "rename",
	edit.OpRetire: "retire", edit.OpSetCase: "setCase",
}

var parityReasons = map[edit.Reason]string{
	edit.ReasonNone: "editable", edit.ReasonComputed: "computed", edit.ReasonLayered: "layered",
	edit.ReasonFormat: "format", edit.ReasonInput: "input", edit.ReasonKey: "key",
	edit.ReasonPseudo: "pseudo", edit.ReasonOrder: "order", edit.ReasonLayer: "layer", edit.ReasonBroken: "broken",
}

// parityOps are an operation of each kind on path: a Set of lit, the others with a value of no
// particular type, since editability is judged before value types (API.md E21).
func parityOps(path string, lit edit.Lit) []edit.Operation {
	return []edit.Operation{
		{Kind: edit.OpSet, Path: path, Value: lit},
		{Kind: edit.OpReset, Path: path},
		{Kind: edit.OpAdd, Path: path, Value: edit.Int(1)},
		{Kind: edit.OpInsert, Path: path, Value: edit.Int(1)},
		{Kind: edit.OpAddEntry, Path: path, Key: edit.Key("zz"), Value: edit.Obj{}},
		{Kind: edit.OpRemove, Path: path},
		{Kind: edit.OpMove, Path: path},
		{Kind: edit.OpRename, Path: path, Key: edit.Key("zz")},
		{Kind: edit.OpRetire, Path: path},
		{Kind: edit.OpSetCase, Path: path, Case: "circle"},
	}
}

// verdict is the reason Editable gives an operation and the one Apply refuses it with (W5), none
// for an editable value or another cause: internal an ErrInternal, hidden an ErrBadOp or ErrBadValue
// where Editable promised it, misnamed a `layered` naming no layer, or the edit layer.
type verdict struct {
	editable, apply            edit.Reason
	internal, hidden, misnamed bool
}

func verdictOf(t *testing.T, s session, r edit.Resolved, op edit.Operation) verdict {
	t.Helper()
	e, err := s.snap.Editable(r, op.Kind, s.env.EditLayer)
	if err != nil {
		t.Fatalf("Editable(%s, %d): %v", r.Canonical, op.Kind, err)
	}
	_, err = edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	v := verdict{editable: e.Reason, internal: errors.Is(err, edit.ErrInternal)}
	if e.Mode != edit.ModeNone {
		v.editable = edit.ReasonNone
		v.hidden = errors.Is(err, edit.ErrBadOp) || errors.Is(err, edit.ErrBadValue)
	}
	v.misnamed = e.Reason == edit.ReasonLayered && (e.Layer == "" || e.Layer == s.env.EditLayer)
	if ne := (*edit.NotEditableError)(nil); errors.As(err, &ne) {
		v.apply = ne.Reason
		v.misnamed = v.misnamed || ne.Reason == edit.ReasonLayered && (ne.Layer == "" || ne.Layer == s.env.EditLayer)
	}
	return v
}

// applyText is Apply's verdict as the parity golden writes it.
func (v verdict) applyText() string {
	switch {
	case v.internal:
		return "internal"
	case v.misnamed:
		return parityReasons[v.apply] + " (layered names no blocking layer)"
	}
	return parityReasons[v.apply]
}

// checkVerdict fails t unless Editable and Apply both give op the reason want, and Apply does
// not fail inside the compiler.
func checkVerdict(t *testing.T, s session, r edit.Resolved, op edit.Operation, want edit.Reason) {
	t.Helper()
	if v := verdictOf(t, s, r, op); v.editable != want || v.apply != want || v.internal || v.misnamed {
		t.Errorf("edit layer %q: %s %s: Editable %s, Apply %s, want %s", s.env.EditLayer, parityOpNames[op.Kind], r.Canonical, parityReasons[v.editable], v.applyText(), parityReasons[want])
	}
}

// API.md W11a, V1 (DECISIONS 411; log-2026-11-01 wave G): every operation on a path below an
// amendment that loads JSON gets one reason from Editable and Apply, the edit layer active or
// not: `layer` under that layer, `layered` under another while the layer is active.
func TestW11aBelowLoadReasonParity(t *testing.T) {
	cases := []struct {
		layers    []string
		editLayer string
		want      edit.Reason
	}{
		{[]string{"dev"}, "dev", edit.ReasonLayer},
		{nil, "dev", edit.ReasonLayer},
		{nil, "lc", edit.ReasonLayer},
		{[]string{"dev"}, "", edit.ReasonLayered},
		{[]string{"dev"}, "lc", edit.ReasonLayered},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("layers=%s", strings.Join(c.layers, ",")), func(t *testing.T) {
			s := open(t, law, c.layers, c.editLayer, "a")
			f := fixture{Snapshot: s.snap, files: s.a.Files()}
			for _, path := range []string{"a:other.data.n", "a:other.data.id"} {
				for _, op := range parityOps(path, edit.Int(6)) {
					checkVerdict(t, s, resolve(t, f, path), op, c.want)
				}
			}
		})
	}
}

// API.md 6.4, W11 (GRAMMAR.md 5.7, EVALUATION.md 9.2; DECISIONS 450): a new line starting with neither
// a name nor a String-keyed list's word key, and a Remove of a list element of a line the session
// does not show, are `layer`; a Remove of an item of the layer's literal takes it out.
func TestW11NoAmendFormParity(t *testing.T) {
	cases := []struct {
		layers    []string
		editLayer string
		op        edit.Operation
		want      edit.Reason
	}{
		{nil, "lc", edit.Operation{Kind: edit.OpSet, Path: "a:byColor[red]", Value: edit.Int(5)}, edit.ReasonLayer},
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpSet, Path: "a:items[a].n", Value: edit.Int(5)}, edit.ReasonNone}, // DECISIONS 450
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpSet, Path: "a:mixed[1]", Value: edit.Int(5)}, edit.ReasonLayer},
		{nil, "lc", edit.Operation{Kind: edit.OpAddEntry, Path: "a:named", Key: edit.Key("zz"), Value: edit.Int(3)}, edit.ReasonLayer},
		{nil, "le", edit.Operation{Kind: edit.OpRemove, Path: "a:bag.xs[0]"}, edit.ReasonLayer},
		{nil, "le", edit.Operation{Kind: edit.OpRemove, Path: "a:bag.ts[red]"}, edit.ReasonLayer},
		{nil, "stage", edit.Operation{Kind: edit.OpSet, Path: "a:items[a].n", Value: edit.Int(5)}, edit.ReasonNone},
		{nil, "le", edit.Operation{Kind: edit.OpRemove, Path: "a:bag.byC[red]"}, edit.ReasonNone},
		{[]string{"le"}, "le", edit.Operation{Kind: edit.OpRemove, Path: "a:bag.xs[0]"}, edit.ReasonNone},
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpSet, Path: "a:bag.xs[0]", Value: edit.Int(5)}, edit.ReasonNone},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("layers=%s", strings.Join(c.layers, ",")), func(t *testing.T) {
			s := open(t, law, c.layers, c.editLayer, "a")
			f := fixture{Snapshot: s.snap, files: s.a.Files()}
			checkVerdict(t, s, resolve(t, f, c.op.Path), c.op, c.want)
		})
	}
}

// API.md W5, W11a (DECISIONS 273, 462; log-2026-11-01 EDIT-OWED K, S, O): Editable refuses what
// Apply refuses: a Set or Reset of a line no Source states, active or not; inside a spread line, an
// edit of a field the spread supplies, a literal beside it judged as written; a base Set its files order.
func TestDecisions273LineParity(t *testing.T) {
	cases := []struct {
		layers    []string
		editLayer string
		op        edit.Operation
		want      edit.Reason
	}{
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpSet, Path: "a:other.data", Value: edit.Obj{"id": edit.Str("x")}}, edit.ReasonComputed},
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpReset, Path: "a:base.sub"}, edit.ReasonComputed},
		{nil, "lc", edit.Operation{Kind: edit.OpSet, Path: "a:other.data", Value: edit.Obj{"id": edit.Str("x")}}, edit.ReasonComputed},
		{nil, "lc", edit.Operation{Kind: edit.OpReset, Path: "a:holder.data"}, edit.ReasonComputed},
		{nil, "lf", edit.Operation{Kind: edit.OpSet, Path: "a:bag.b.y", Value: edit.Int(4)}, edit.ReasonNone}, // DECISIONS 462
		{nil, "lf", edit.Operation{Kind: edit.OpSet, Path: "a:bag.b.x", Value: edit.Int(4)}, edit.ReasonComputed},
		{[]string{"lf"}, "lf", edit.Operation{Kind: edit.OpSet, Path: "a:bag.b.y", Value: edit.Int(4)}, edit.ReasonNone},
		{[]string{"lf"}, "lf", edit.Operation{Kind: edit.OpReset, Path: "a:bag.b.x"}, edit.ReasonComputed},
		{[]string{"lf"}, "lf", edit.Operation{Kind: edit.OpSet, Path: "a:bag.b.sub", Value: edit.Obj{"id": edit.Str("x")}}, edit.ReasonComputed},
		{nil, "", edit.Operation{Kind: edit.OpSet, Path: "a:entries", Value: edit.Source("{ one { v: 1 } }")}, edit.ReasonOrder},
		{[]string{"dev"}, "dev", edit.Operation{Kind: edit.OpSet, Path: "a:holder.data", Value: edit.Obj{"id": edit.Str("x")}}, edit.ReasonNone},
		{[]string{"lf"}, "lf", edit.Operation{Kind: edit.OpSet, Path: "a:bag.xs[0]", Value: edit.Int(4)}, edit.ReasonNone},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("layers=%s", strings.Join(c.layers, ",")), func(t *testing.T) {
			s := open(t, law, c.layers, c.editLayer, "a")
			f := fixture{Snapshot: s.snap, files: s.a.Files()}
			checkVerdict(t, s, resolve(t, f, c.op.Path), c.op, c.want)
		})
	}
}

// API.md W11, E22 (DECISIONS 273; log-2026-11-01 EDIT-OWED Q and review): a layer line whose key is
// quoted or a qualified member is a Source line like any other: a Set or Reset of it applies,
// Editable agrees, and its Undo restores every value; a Remove of it re-values a base key: `layer`.
func TestW11QuotedKeyLineUndo(t *testing.T) {
	ar, err := txtar.ParseFile(filepath.Join("testdata", "edits", "layer_keys_again.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	keys := readCase(t, ar)
	ld := goldenCase{fsys: law, pkgs: []string{"a"}, editLayer: "ld"}
	cases := []struct {
		c    goldenCase
		op   edit.Operation
		want edit.Reason
	}{
		{ld, edit.Operation{Kind: edit.OpSet, Path: "a:bag.byC", Value: edit.Source("{ red: 1, green: 6 }")}, edit.ReasonNone},
		{ld, edit.Operation{Kind: edit.OpReset, Path: "a:bag.byC[green]"}, edit.ReasonNone},
		{ld, edit.Operation{Kind: edit.OpRemove, Path: "a:bag.byC[green]"}, edit.ReasonLayer},
		{keys, edit.Operation{Kind: edit.OpSet, Path: "p:conf.byWord", Value: edit.Source("{ if: 4 }")}, edit.ReasonNone},
		{keys, edit.Operation{Kind: edit.OpRemove, Path: "p:conf.byWord[if]"}, edit.ReasonLayer},
	}
	for _, c := range cases {
		s := open(t, c.c.fsys, c.c.layers, c.c.editLayer, c.c.pkgs...)
		checkVerdict(t, s, resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, c.op.Path), c.op, c.want)
		if c.want == edit.ReasonNone {
			applyAndUndo(t, c.c, c.op)
		}
	}
}

// applyAndUndo applies op alone in c, then its Undo, which must restore every value (API.md E22).
func applyAndUndo(t *testing.T, c goldenCase, op edit.Operation) {
	t.Helper()
	s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil || len(plan.Undo) == 0 {
		t.Errorf("%s %s: plan %v, %v; want it applied with its Undo", parityOpNames[op.Kind], op.Path, plan, err)
		return
	}
	checkUndo(t, c, []edit.Operation{op}, plan)
}

// API.md E22, W11a (DECISIONS 273; log-2026-11-01 EDIT-OWED): a Set replacing a map whose keys the
// layer amends by lines, with a value that drops such a key, applies; its Undo writes each line
// back, a Set where the map's own literal states the key, and restores every value.
func TestE22MapLineUndo(t *testing.T) {
	ar, err := txtar.ParseFile(filepath.Join("testdata", "edits", "layer_keys_again.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	c := readCase(t, ar)
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "p:conf.weights", Value: edit.Source(`{ "b": 2 }`)},
		{Kind: edit.OpSet, Path: "p:conf.byWord", Value: edit.Source("{ plain: 2 }")},
	} {
		s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
		if err != nil {
			t.Errorf("set %s: %v, want it applied", op.Path, err)
			continue
		}
		checkUndo(t, c, []edit.Operation{op}, plan)
	}
}

// keptLine is a project whose layer adds one table entry by a call, which no Source states, and
// one by a literal.
var keptLine = mapFS{
	"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/p/p.canon": file("/// P.\npackage p\n\n/// An entry.\nrecord E {\n  /// A value.\n  v: Int = 0\n}\n\n" +
		"/// Entries.\nlet t: table E = {\n  one { v: 1 }\n}\n\n/// Makes one.\nlocal fn mkE() -> E { return { v: 2 } }\n"),
	"law/p/dev.layer.canon": file("package p\nlayer dev\n\namend t {\n  two: mkE()\n  three: { v: 3 }\n}\n"),
}

// API.md W11, E22 (DECISIONS 273): a Remove of an entry whose own layer line no Source states is
// computed for Editable and Apply, its Undo could not write that line back; a literal line's applies.
func TestDecisions273RemoveComputedLine(t *testing.T) {
	s := open(t, keptLine, []string{"dev"}, "dev", "p")
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	for path, want := range map[string]edit.Reason{"p:t.two": edit.ReasonComputed, "p:t.three": edit.ReasonNone} { //canon:unordered each case is judged alone
		op := edit.Operation{Kind: edit.OpRemove, Path: path}
		checkVerdict(t, s, resolve(t, f, path), op, want)
	}
}
