package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md V1, V2; TYPES.md 11.4 (a loaded wire value is a literal); DECISIONS 171: a FromJSON
// decoded on its computed branch is refused bound for Canon text that reads another branch, at the
// top level, under an edit layer and nested at any depth; nothing is written.
func TestFromJSONSecondBranchCanonRefused(t *testing.T) {
	kb := edit.Member("b")
	for _, c := range []struct {
		name string
		fsys mapFS
		view undoView
		ops  []edit.Operation
	}{
		{"top level, driver set before", branchFS(), undoView{}, []edit.Operation{setAt("cn.k", kb), setAt("cn.t", edit.FromJSON(`{"x": 1}`))}},
		{"top level, driver already b", branchFS(), undoView{}, []edit.Operation{setAt("cb.t", edit.FromJSON(`{"x": 1}`))}},
		{"nested", branchFS(), undoView{}, []edit.Operation{setAt("cn", edit.FromJSON(`{"k": "b", "t": {"x": 1}}`))}},
		{"nested, list element", branchFS(), undoView{}, []edit.Operation{setAt("cn", edit.FromJSON(`{"k": "b", "l": [{"x": 1}]}`))}},
		{"nested, two levels", branchFS(), undoView{}, []edit.Operation{setAt("ch", edit.FromJSON(`{"h": {"k": "b", "t": {"x": 1}}}`))}},
		{"nested, map value", branchFS(), undoView{}, []edit.Operation{setAt("cm", edit.FromJSON(`{"m": {"db": {"x": 1}}}`))}},
		{"edit layer, driver set before", layerFS(), undoView{[]string{"dev"}, "dev"}, []edit.Operation{setAt("lt.e2.k", kb), setAt("lt.e2.t", edit.FromJSON(`{"x": 4}`))}},
	} {
		s := open(t, c.fsys, c.view.layers, c.view.edit, "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: c.ops})
		if !errors.Is(err, edit.ErrBadValue) || plan != nil {
			t.Errorf("%s: planned %v, %v, want ErrBadValue and no plan", c.name, plan != nil, err)
		}
	}
}

// API.md V1, V2, E22; TYPES.md 11.4; DECISIONS 171: a FromJSON on the first branch into Canon text,
// or on any branch into a JSON source, applies, reads back as given, and its Undo restores.
func TestFromJSONBranchAllowed(t *testing.T) {
	for _, c := range []struct {
		name, root, want string
		op               edit.Operation
	}{
		{"Canon, first branch", "ch", "t: RA{x: 2}", setAt("ch.h.t", edit.FromJSON(`{"x": 2}`))},
		{"Canon, nested, first branch", "cn", "t: RA{x: 1}", setAt("cn", edit.FromJSON(`{"k": "a", "t": {"x": 1}}`))},
		{"Canon, into its typed literal", "ck", "t: RB{x: 2}", setAt("ck.h.t", edit.FromJSON(`{"x": 2}`))},
		{"JSON, second branch", "jh", "t: RB{x: 0}", setAt("jh.h", edit.FromJSON(`{"k": "b", "t": {"x": 0}}`))},
		{"JSON, second branch, map value", "jm", "db: RB{x: 0}", setAt("jm", edit.FromJSON(`{"m": {"db": {"x": 0}}}`))},
	} {
		edited, ok := applied(t, c.name, branchFS(), []edit.Operation{c.op})
		if !ok {
			continue
		}
		if got := values(open(t, edited.fsys, nil, "", "d"), false)["d:"+c.root]; !strings.Contains(got, c.want) {
			t.Errorf("%s: %s = %s, want it to hold %s", c.name, c.root, got, c.want)
		}
		checkClean(t, c.name, branchFS(), []edit.Operation{c.op})
		undoTwice(t, c.name, branchFS(), []edit.Operation{c.op})
	}
	view := undoView{[]string{"dev"}, "dev"}
	op := []edit.Operation{setAt("lt.e2.t", edit.FromJSON(`{"x": 4}`))}
	if _, fs, ok := applyIn(t, "edit layer, first branch", layerFS(), view, op); ok {
		if got := values(open(t, fs, view.layers, "", "d"), false)["d:lt"]; !strings.Contains(got, "e2: LT{k: a, t: RA{x: 4}}") {
			t.Errorf("edit layer, first branch: lt = %s under dev", got)
		}
	}
	undoIn(t, undoSpec{name: "edit layer, first branch", fsys: layerFS(), view: view, ops: op, views: [][]string{{"dev"}, nil}, lines: []string{"law/d/dev.layer.canon"}})
}
