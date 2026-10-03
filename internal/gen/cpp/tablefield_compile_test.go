package cppgen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tableFileCase is one data file of a table-field package and the driver's line for it.
type tableFileCase struct{ name, body, want string }

// tableCases: WIRE.md §5.7's nested table in file order (b before a), its `$retired`, an empty table and an absent optional one, the refs of its rows resolved to their shelves (CODEGEN.md §5.8), and the refusals at the place they occur, texts as the Go loader's (CODEGEN.md §5.8).
var tableCases = []tableFileCase{
	{"good", `{"$id": "s1", "slots": {"b": {"n": 2, "home": "s2"}, "a": {"$retired": true, "n": 1, "home": "s1"}}, "spare": {"z": {"n": 9, "home": "s2"}}}, {"$id": "s2", "slots": {}}`,
		"s1[slots=b:2@s2->s2,a:1r@s1->s1 spare=z:9@s2->s2] s2[slots= spare=none]"},
	{"nullspare", `{"$id": "s1", "slots": {"a": {"n": 1, "home": "s1"}}, "spare": null}`, "s1[slots=a:1@s1->s1 spare=none]"},
	{"nohome", `{"$id": "s1", "slots": {"a": {"n": 1, "home": "nowhere"}}}`, "error good: rows[0].slots.a.home: no entry nowhere"},
	{"nohomespare", `{"$id": "s1", "slots": {}, "spare": {"c": {"n": 1, "home": "gone"}}}`, "error good: rows[0].spare.c.home: no entry gone"},
	{"key", `{"$id": "s1", "slots": {"a b": {"n": 1, "home": "s1"}}}`, "error good: rows[0].slots: " + badTableKey("a b")},
	{"retired", `{"$id": "s1", "slots": {"a": {"$retired": false, "n": 1, "home": "s1"}}}`, "error good: rows[0].slots.a.$retired: expected true"},
	{"array", `{"$id": "s1", "slots": [{"n": 1}]}`, "error good: rows[0].slots: expected an object"},
	{"row", `{"$id": "s1", "slots": {"a": {"n": 1, "home": "s1"}, "b": {"n": "x", "home": "s1"}}}`, "error good: rows[0].slots.b.n: expected an integer"},
	{"dup", `{"$id": "s1", "slots": {"a": {"n": 1, "home": "s1"}, "a": {"n": 2, "home": "s1"}}}`, "error good: rows[0].slots.a: duplicate key"},
}

// CODEGEN.md §4.2, §5.3, §5.8, WIRE.md §5.7: a table field compiles and its loader reads, orders, retires and resolves a nested table, as the Go loader does.
func TestTableFieldDataCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, tablesPackage()), "tables_main.cpp")
	args := []string{dir}
	var want strings.Builder
	for _, c := range tableCases {
		body := `{"$schema": "` + tablesSchema + `", "rows": [` + c.body + `]}`
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name+".json")
		fmt.Fprintf(&want, "%s.json: %s\n", c.name, strings.Replace(c.want, "error good:", "error "+c.name+".json:", 1))
	}
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "tables.gen.cpp"}, args...) {
		if out != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", out, want.String())
		}
	}
}

// typesTableCases: the public decoder of a types-mode record reads a nested table (rows in byte order of their keys, nlohmann's), a default empty table and an optional one, and refuses as the loader does.
var typesTableCases = []tableFileCase{
	{"good", `{"title": "t", "slots": {"a": {"n": 1}, "b": {"$retired": true, "n": 2}}, "spare": {"c": {"n": 3}}}`, "slots=a:1,b:2r extra= spare=c:3"},
	{"order", `{"title": "t", "slots": {"b": {"n": 2}, "a": {"n": 1}}}`, "slots=a:1,b:2 extra= spare=none"},
	{"extra", `{"title": "t", "slots": {}, "extra": {"x": {"n": 5}}}`, "slots= extra=x:5 spare=none"},
	{"key", `{"title": "t", "slots": {"a b": {"n": 1}}}`, "error Shelf: $.slots: " + badTableKey("a b")},
	{"retired", `{"title": "t", "slots": {"a": {"$retired": 1, "n": 1}}}`, "error Shelf: $.slots.a.$retired: expected true"},
	{"row", `{"title": "t", "slots": {"a": {"n": "x"}}}`, "error Shelf: $.slots.a.n: expected an integer"},
	{"array", `{"title": "t", "slots": []}`, "error Shelf: $.slots: expected an object"},
}

// CODEGEN.md §4.2, §5.13, WIRE.md §5.7: a types-mode table field compiles and its public decoder reads a nested table.
func TestTableFieldTypesCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, generate(t, tableTypesPackage()))
	copyFile(t, filepath.Join("testdata", "main", "tabletypes_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	args := []string{dir}
	var want strings.Builder
	for _, c := range typesTableCases {
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.name+".json")
		fmt.Fprintf(&want, "%s.json: %s\n", c.name, c.want)
	}
	for _, got := range buildAndRun(t, dir, []string{"main.cpp", "out/tabletypes.gen.cpp"}, args...) {
		if got != want.String() {
			t.Errorf("got:\n%s\nwant:\n%s", got, want.String())
		}
	}
}
