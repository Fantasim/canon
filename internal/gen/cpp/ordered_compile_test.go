package cppgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// orderedCase is one document and what the driver prints for it: through Decode of an nlohmann::json (entries in key byte order), then through Decode of an nlohmann::ordered_json parsed from the same text (entries in the file's order).
type orderedCase struct{ name, body, plain, ordered string }

// orderedBody is a Shelf document from its four members' texts.
func orderedBody(power, deep, slots, racks string) string {
	return `{"power": ` + power + `, "deep": ` + deep + `, "slots": ` + slots + `, "racks": ` + racks + `}`
}

const (
	orderedEmptyObject = `{}`
	orderedNoRacks     = `[]`
	orderedFileSlots   = `{"b": {"n": 2}, "a": {"n": 1}}`
	orderedFilePower   = `{"z": 1, "b": 2, "a": 3}`
	orderedFileDeep    = `{"y": {"q": 1, "p": 2}, "x": {"b": 3, "a": 4}}`
	orderedFileRacks   = `[{"tags": {"t": 1, "s": 2}}, {"tags": {"r": 3, "q": 4}}]`
	orderedNotInt      = ": expected an integer"
	orderedNoEntries   = "power= deep= slots= racks="
)

// orderedCases: a file order unlike byte order is kept at every depth; the first failure is the first in file order; a repeated key is never seen; an empty object; documents that give the same verdict whatever the order (CODEGEN.md §5.13, DECISIONS 437).
var orderedCases = []orderedCase{
	{"order", orderedBody(orderedFilePower, orderedFileDeep, orderedFileSlots, orderedFileRacks),
		"power=a:3,b:2,z:1 deep=x[a:4,b:3];y[p:2,q:1] slots=a:1,b:2 racks=[s:2,t:1];[q:4,r:3]",
		"power=z:1,b:2,a:3 deep=y[q:1,p:2];x[b:3,a:4] slots=b:2,a:1 racks=[t:1,s:2];[r:3,q:4]"},
	{"first", orderedBody(`{"z": 1, "b": "x", "a": "y"}`, orderedEmptyObject, orderedEmptyObject, orderedNoRacks),
		"error Shelf: $.power.a" + orderedNotInt, "error Shelf: $.power.b" + orderedNotInt},
	{"deeper", orderedBody(orderedEmptyObject, `{"y": {"q": 1, "p": "x"}, "x": {"b": "z"}}`, orderedEmptyObject, orderedNoRacks),
		"error Shelf: $.deep.x.b" + orderedNotInt, "error Shelf: $.deep.y.p" + orderedNotInt},
	{"inlist", orderedBody(orderedEmptyObject, orderedEmptyObject, orderedEmptyObject, `[{"tags": {"b": 1}}, {"tags": {"z": 1, "b": "x", "a": "y"}}]`),
		"error Shelf: $.racks[1].tags.a" + orderedNotInt, "error Shelf: $.racks[1].tags.b" + orderedNotInt},
	{"table", orderedBody(orderedEmptyObject, orderedEmptyObject, `{"b": {"n": "x"}, "a": {"n": "y"}}`, orderedNoRacks),
		"error Shelf: $.slots.a.n" + orderedNotInt, "error Shelf: $.slots.b.n" + orderedNotInt},
	{"dup", orderedBody(`{"a": 1, "a": 2}`, orderedEmptyObject, orderedEmptyObject, orderedNoRacks), "power=a:2 deep= slots= racks=", "power=a:2 deep= slots= racks="},
	{"empty", orderedBody(orderedEmptyObject, orderedEmptyObject, orderedEmptyObject, orderedNoRacks), orderedNoEntries, orderedNoEntries},
	{"sorted", orderedBody(`{"a": 1, "b": 2}`, orderedEmptyObject, `{"a": {"n": 1}, "b": {"n": 2}}`, orderedNoRacks), "power=a:1,b:2 deep= slots=a:1,b:2 racks=", "power=a:1,b:2 deep= slots=a:1,b:2 racks="},
	{"shape", `{"power": [], "deep": {}, "slots": {}, "racks": []}`, "error Shelf: $.power: expected an object", "error Shelf: $.power: expected an object"},
	{"null", `null`, "error Shelf: $: null", "error Shelf: $: null"},
}

// CODEGEN.md §5.13, DECISIONS 437: a types-mode Decode of an nlohmann::ordered_json keeps the order the value holds in a map, a map of maps, a nested table and the maps of a list's records, and gives the verdict and first failure of Decode of an nlohmann::json where the order does not matter.
func TestOrderedDecodeCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, generate(t, orderedPackage()))
	copyFile(t, filepath.Join("testdata", "main", "ordered_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	args := []string{dir}
	var want strings.Builder
	for _, c := range orderedCases {
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name+".json")
		fmt.Fprintf(&want, "%s.json: %s\nordered %s.json: %s\n", c.name, c.plain, c.name, c.ordered)
	}
	for _, got := range buildAndRun(t, dir, []string{"main.cpp", "out/ordered.gen.cpp"}, args...) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}
