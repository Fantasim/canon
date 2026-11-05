package edit_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// DECISIONS 466, 467, API.md W9, E22, E23: a grow op or SetCase on a spread-supplied field or case no
// active layer amends (no layer, dev, hot for extra and over's stated nodes) writes an override; its
// Undo and the Undo's Undo give every value back in the session's layers and with none.
func TestSpreadGrowAccepted(t *testing.T) {
	for _, layers := range [][]string{nil, {"dev"}} {
		for _, root := range spreadGrowRoots {
			for _, op := range append(spreadGrowOps(root), spreadGrowCases(root+".sh")...) {
				spreadGrowUndo(t, undoView{layers, ""}, op)
			}
		}
		for _, op := range spreadCaseOps() {
			spreadGrowUndo(t, undoView{layers, ""}, op)
		}
	}
	for _, op := range []edit.Operation{
		{Kind: edit.OpAdd, Path: "d:derived.extra", Value: edit.Str("n")},
		{Kind: edit.OpInsert, Path: "d:chain.extra", Index: 0, Value: edit.Str("n")},
		{Kind: edit.OpAdd, Path: "d:deep.box.extra", Value: edit.Str("n")},
		{Kind: edit.OpAdd, Path: "e:ech.extra", Value: edit.Str("n")},
		{Kind: edit.OpAdd, Path: "d:over.nodes", Value: edit.Obj{"name": edit.Str("c")}},
	} {
		spreadGrowUndo(t, undoView{[]string{"hot"}, ""}, op)
	}
}

// DECISIONS 466 (no edit layer only), 273, API.md W11, E22: an AddEntry into a field a spread
// supplies, made with an active edit layer while hot amends the field's source, is an amendment
// line of that layer, applied; its Undo gives the layer's lines, the base file and every value back.
func TestSpreadGrowEditLayer(t *testing.T) {
	for _, v := range []undoView{{[]string{"hot"}, "hot"}, {[]string{"hot", "dev"}, "dev"}} {
		for _, root := range []string{"d:derived", "d:chain"} {
			for _, op := range spreadGrowOps(root)[spreadGrowEntryOps:] {
				spreadGrowUndo(t, v, op)
			}
		}
	}
}

// spreadGrowUndo applies op in v, then its Undo, then the Undo's Undo: each gives back the values
// before it, in v's layers and with none; under an edit layer, its lines and the base file too.
func spreadGrowUndo(t *testing.T, v undoView, op edit.Operation) {
	t.Helper()
	name, fs := opName(op)+" "+op.Path, spreadGrowFS()
	f1, undo, ok := spreadGrowApply(t, name, fs, v, []edit.Operation{op})
	if !ok {
		return
	}
	f2, redo, ok := spreadGrowApply(t, name+", Undo", f1, v, undo)
	if !ok {
		return
	}
	spreadGrowSame(t, name+", Undo", fs, f2, v.layers)
	if v.edit != "" {
		layerFile := "law/d/" + v.edit + ".layer.canon"
		if w, g := amendLines(t, fs[layerFile].Data), amendLines(t, f2[layerFile].Data); !slices.Equal(w, g) {
			t.Errorf("%s, Undo: %s lines %q, want %q", name, layerFile, g, w)
		}
		if fileText(f2, spreadGrowFile) != spreadGrowSrc {
			t.Errorf("%s, Undo: %s changed:\n%s", name, spreadGrowFile, matrixHunk(spreadGrowSrc, fileText(f2, spreadGrowFile)))
		}
	}
	if f3, _, ok := spreadGrowApply(t, name+", Undo of the Undo", f2, v, redo); ok {
		spreadGrowSame(t, name+", Undo of the Undo", f1, f3, v.layers)
	}
}

// spreadGrowApply applies ops to fsys in view v: the files it writes, each write minimal (M6), and
// its Undo.
func spreadGrowApply(t *testing.T, name string, fsys mapFS, v undoView, ops []edit.Operation) (mapFS, []edit.Operation, bool) {
	t.Helper()
	s := open(t, fsys, v.layers, v.edit, spreadGrowPkgs...)
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Errorf("%s: %+v: %v", name, ops, err)
		return nil, nil, false
	}
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
	}
	return written(fsys, plan), plan.Undo, true
}

// spreadGrowSame reports every let of d and e whose value differs between want and got, read with
// layers active and with none.
func spreadGrowSame(t *testing.T, name string, want, got mapFS, layers []string) {
	t.Helper()
	for _, ls := range [][]string{layers, nil} {
		w, g := letTexts(t, open(t, want, ls, "", spreadGrowPkgs...).snap), letTexts(t, open(t, got, ls, "", spreadGrowPkgs...).snap)
		if !maps.Equal(w, g) {
			t.Errorf("%s, layers %v:\n%s", name, ls, valuesDiff(w, g))
		}
	}
}
