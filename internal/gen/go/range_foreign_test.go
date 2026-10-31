package gogen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// emberRanged is game.core and game.world (data) with a refinement on each position world's readers read through core's classes (CODEGEN.md §2.8, §5.15): a field, a list, its element, a case field, both dependent branches, a stored `$` result, a lookup cell and a table row's fields.
func emberRanged() []*ir.Package {
	c, w := emberfall(ir.ModeData)
	levels := c.levels.Fields
	levels[0].Type = within(intT, between(0, 9))
	levels[2].Type = within(listT(within(strT, between(1, 4))), upTo(2))
	c.reward.Cases[0].Fields[0].Type = within(intT, between(0, 100))
	c.param.Branches[0].Type = within(intT, between(0, 100))
	c.param.Branches[1].Type = within(strT, between(2, 8))
	c.width.Result = within(intT, between(0, 9))
	c.scaled.Result = within(intT, between(0, 9))
	c.marker.Fields[0].Type = within(strT, between(1, 3))
	c.marker.Fields[1].Type = within(intT, between(0, 9))
	return []*ir.Package{c.p, w}
}

// emberCases are good/zones.json with from replaced by to, and the loader's line.
var emberCases = []struct{ name, from, to, want string }{
	{"ok", "", "", "ok"},
	{"field-hi", `"min": 1, "max": 5`, `"min": 10, "max": 5`, "error NAME.json: rows[0].levels.min: expected a value in 0..=9"},
	{"field-lo", `"min": 1, "max": 5`, `"min": -1, "max": 5`, "error NAME.json: rows[0].levels.min: expected a value in 0..=9"},
	{"field-in", `"min": 1, "max": 5`, `"min": 0, "max": 5`, "ok"},
	{"labels-count", `"labels": ["a"]`, `"labels": ["a","b","c"]`, "error NAME.json: rows[0].levels.labels: expected an element count in ..=2"},
	{"labels-elem", `"labels": ["a"]`, `"labels": ["abcde"]`, "error NAME.json: rows[0].levels.labels[0]: expected a byte length in 1..=4"},
	{"band-elem", `"min": 2, "max": 3`, `"min": 20, "max": 3`, "error NAME.json: rows[0].bands[0].min: expected a value in 0..=9"},
	{"named-value", `"north": {"min": 4`, `"north": {"min": 40`, "error NAME.json: rows[0].named.north.min: expected a value in 0..=9"},
	{"case-hi", `"amount": 3`, `"amount": 300`, "error NAME.json: rows[0].reward.amount: expected a value in 0..=100"},
	{"case-lo", `"amount": 3`, `"amount": -1`, "error NAME.json: rows[0].reward.amount: expected a value in 0..=100"},
	{"branch-str", `"param": "wind"`, `"param": "w"`, "error NAME.json: rows[0].param: expected a byte length in 2..=8"},
	{"cell-hi", `"$scaled": {"false": 4, "true": 8}`, `"$scaled": {"false": 4, "true": 80}`, "error NAME.json: rows[0].levels.$scaled.true: expected a value in 0..=9"},
	{"stored-hi", `"$width": 4`, `"$width": 40`, "error NAME.json: rows[0].levels.$width: expected a value in 0..=9"},
	{"marker-heal", `"heal": 5`, `"heal": 50`, "error NAME.json: rows[0].markers.gate.heal: expected a value in 0..=9"},
	{"marker-code", `"code": "g"`, `"code": "gggg"`, "error NAME.json: rows[0].markers.gate.code: expected a byte length in 1..=3"},
}

// CODEGEN.md §2.8, §5.15: world's readers of core's classes refuse a value outside its refinement at every position they read.
func TestGoRangeForeignReadersRun(t *testing.T) {
	zones, err := os.ReadFile(filepath.Join("testdata", "datafiles", "emberfall", "good", "zones.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runs []rangeRun
	for _, c := range emberCases {
		text := string(zones)
		if c.from != "" {
			text = strings.Replace(text, c.from, c.to, 1)
			if text == string(zones) {
				t.Fatalf("%s: %q not in zones.json", c.name, c.from)
			}
		}
		runs = append(runs, rangeRun{c.name, text, strings.ReplaceAll(c.want, "NAME", c.name)})
	}
	data := casesFile(t, runs)
	for _, r := range runs {
		data[r.Name+".json"] = []byte(r.Text)
	}
	runData(t, generateData(t, emberRanged()...), emberWorld, "testdata/smoke/range_ember_test.go", data)
}
