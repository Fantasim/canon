package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// rangeImportOrders is a data file of demo.app that every refinement of demo.base and demo.app accepts.
const rangeImportOrders = `{"$schema": "demo.app.Order@00000001", "rows": [
 {"id": "o1", "color": "blue", "at": {"x": 3}, "trail": [{"x": 1}, {"x": 2}], "kind": "solid", "tint": "green", "n": 1, "alt": null, "shade": null, "$hue": {"red": 1, "green": 2, "blue": 3}},
 {"id": "o2", "color": "red", "at": {"x": -1}, "trail": [], "kind": "clear", "alt": {"x": 9}, "shade": "warm", "$hue": {"red": 4, "green": 5, "blue": 6}}
]}`

// rangeImportPair is importPair with refinements on the classes demo.app reads through its readers: a field, a case field, both dependent branches and a lookup cell.
func rangeImportPair() (base, app *ir.Package) {
	base, app = importPair()
	base.Types[1].(*ir.Record).Fields[0].Type = within(tInt32, bound(-1, 9))
	paint := base.Types[2].(*ir.Variant)
	paint.Cases[0].Fields = append(paint.Cases[0].Fields, field("n", "n", "", within(tInt32, bound(0, 2))))
	shade := base.Types[3].(*ir.Dependent)
	shade.Branches[0].Type = within(tString, bound(1, 4))
	shade.Branches[1].Type = within(tInt32, bound(0, 9))
	app.Types[1].(*ir.Record).Methods[0].Result = within(tInt, bound(1, 6))
	return base, app
}

// importCases are rangeImportOrders with from replaced by to, and the loader's line.
var importCases = []struct{ name, from, to, want string }{
	{"ok", "", "", "ok 2"},
	{"field-hi", `"at": {"x": 3}`, `"at": {"x": 10}`, "error NAME.json: rows[0].at.x: expected a value in -1..=9"},
	{"field-lo", `"at": {"x": 3}`, `"at": {"x": -2}`, "error NAME.json: rows[0].at.x: expected a value in -1..=9"},
	{"field-in", `"at": {"x": 3}`, `"at": {"x": -1}`, "ok 2"},
	{"elem", `{"x": 2}`, `{"x": 12}`, "error NAME.json: rows[0].trail[1].x: expected a value in -1..=9"},
	{"optional", `"alt": {"x": 9}`, `"alt": {"x": 99}`, "error NAME.json: rows[1].alt.x: expected a value in -1..=9"},
	{"case-hi", `"n": 1`, `"n": 3`, "error NAME.json: rows[0].n: expected a value in 0..=2"},
	{"case-lo", `"n": 1`, `"n": -1`, "error NAME.json: rows[0].n: expected a value in 0..=2"},
	{"branch-str", `"shade": "warm"`, `"shade": "toolong"`, "error NAME.json: rows[1].shade: expected a byte length in 1..=4"},
	{"cell-hi", `"red": 1`, `"red": 7`, "error NAME.json: rows[0].$hue.red: expected a value in 1..=6"},
	{"cell-lo", `"blue": 6`, `"blue": 0`, "error NAME.json: rows[1].$hue.blue: expected a value in 1..=6"},
}

// CODEGEN.md §2.8, §5.15: a data loader refuses a value outside its refinement in a class of another package, a case, a dependent branch and a lookup cell it reads through its readers.
func TestCppRangeForeignReadersRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base, app := rangeImportPair()
	for _, p := range []*ir.Package{base, app} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "range_import_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	var args []string
	for _, c := range importCases {
		text := strings.Replace(rangeImportOrders, c.from, c.to, 1)
		if c.from != "" && text == rangeImportOrders {
			t.Fatalf("%s: %q not in the orders", c.name, c.from)
		}
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name)
	}
	sources := []string{"demo/base/out/base.gen.cpp", "demo/app/out/app.gen.cpp", "demo/app/out/app_conformance.gen.cpp", "main.cpp"}
	for _, out := range buildAndRun(t, dir, sources, append([]string{dir}, args...)...) {
		got := driverSections(strings.ReplaceAll(out, dir+"/", ""))
		for _, c := range importCases {
			if want := strings.ReplaceAll(c.want, "NAME", c.name); got[c.name] != want+"\n" {
				t.Errorf("%s: got %q, want %q", c.name, got[c.name], want)
			}
		}
	}
}
