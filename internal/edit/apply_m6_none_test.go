package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// noneP has a one-line optional scalar field (rows.a.hint), a list field of several lines
// (rows.b.tags) and one of a line (rows.c.tags), each settable to none.
var noneP = mapFS{
	"law/project.canon": file(projectCanon),
	"law/p/p.canon": file("package p\n\n/// R.\nrecord R {\n  /// C.\n  count: Int\n  /// H.\n  hint: String? = \"x\"\n" +
		"  /// T.\n  tags: [Int]?\n}\n\n/// Rows.\nlet rows: table R = {\n  a {\n    count: 1\n    hint: \"set\"\n  }\n" +
		"  b {\n    count: 2\n    tags: [\n      1,\n      2,\n      3,\n    ]\n  }\n  c { count: 3, tags: [1, 2] }\n}\n"),
}

// log-2026-09-29 P20-r2 (API.md M6, M5): M6's one line covers a Set whose replaced item is written
// on one line, whatever the new value, none included; a Set replacing several lines (E7) is outside
// it. Each Set is applied under the check scalarSet decides, whichever way the none is written.
func TestSetNoneOneLineByReplacedItem(t *testing.T) {
	s := open(t, noneP, nil, "", "p")
	for _, c := range []struct {
		name, path string
		value      edit.Lit
		covered    bool
	}{
		{"none, one-line scalar field", "rows.a.hint", edit.None{}, true},
		{"Source none, one-line scalar field", "rows.a.hint", edit.Source("none"), true},
		{"JSON null, one-line scalar field", "rows.a.hint", edit.FromJSON("null"), true},
		{"scalar, one-line scalar field", "rows.a.hint", edit.Str("y"), true},
		{"none, one-line list", "rows.c.tags", edit.None{}, true},
		{"none, list of several lines", "rows.b.tags", edit.None{}, false},
		{"Source none, list of several lines", "rows.b.tags", edit.Source(" none "), false},
		{"JSON null, list of several lines", "rows.b.tags", edit.FromJSON("null"), false},
	} {
		ops := []edit.Operation{{Kind: edit.OpSet, Path: c.path, Value: c.value}}
		if got := scalarSet(s.a, ops, 0); got != c.covered {
			t.Errorf("%s: scalarSet = %v, want %v", c.name, got, c.covered)
		}
		if got := oneValueSet(s.a, ops); got != c.covered {
			t.Errorf("%s: oneValueSet = %v, want %v", c.name, got, c.covered)
		}
		if !applyChecked(t, s.env, s.a, ops, oneValueSet(s.a, ops)) {
			t.Errorf("%s: refused", c.name)
		}
	}
}
