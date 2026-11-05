package edit_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md V1, V2, M7, E22 (DECISIONS 171, 475): a FromJSON decoded on a branch a bare brace does not
// read as is written as a typed literal into Canon text, at the top level and at any depth: it
// applies, checks clean, and its Undo restores (an edit layer: TestTypedLiteralEditLayer).
func TestFromJSONSecondBranchCanon(t *testing.T) {
	kb := edit.Member("b")
	for _, c := range []typedLitCase{
		{"top level, driver set before", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn.k", kb), setAt("cn.t", edit.FromJSON(`{"x": 1}`))}},
		{"top level, driver already b", "cb", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cb.t", edit.FromJSON(`{"x": 1}`))}},
		{"nested", "cn", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("cn", edit.FromJSON(`{"k": "b", "t": {"x": 1}}`))}},
		{"nested, list element", "cn", "l: [RB{x: 1}]", "l: [RB { x: 1 }]", []edit.Operation{setAt("cn", edit.FromJSON(`{"k": "b", "l": [{"x": 1}]}`))}},
		{"nested, two levels", "ch", "t: RB{x: 1}", "t: RB { x: 1 }", []edit.Operation{setAt("ch", edit.FromJSON(`{"h": {"k": "b", "t": {"x": 1}}}`))}},
		{"nested, map value", "cm", "db: RB{x: 1}", "db: RB { x: 1 }", []edit.Operation{setAt("cm", edit.FromJSON(`{"m": {"db": {"x": 1}}}`))}},
	} {
		typedLitApplies(t, branchFS(), c)
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
