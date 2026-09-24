package cppgen_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

const (
	shelvesFile = "shelves.json"
	itemsFile   = "items.json"
	configFile  = "config.json"
	// nestedOK arrays inside rows[0].waits make 512 levels with the file's object, rows, the row and waits.
	nestedOK = 508
)

// strictCase is a data file canon never writes, made from testdata/main/shop/good by one edit
// (old "" replaces the whole file), and the exact error its generated loader must return after
// "<path>: " ("" for a file that loads). Shelves load alone; items and config load together.
type strictCase struct {
	name, rule, file, old, new, want string
}

func strictCases() []strictCase {
	deep := func(n int) string { return `"waits": [` + strings.Repeat("[", n) + strings.Repeat("]", n) + `]` }
	return []strictCase{
		{"good", "CODEGEN.md §7.6", shelvesFile, `"i": 30`, `"i": 30`, ""},
		{"case", "log-2026-09-24 key case", shelvesFile, `"slots": -1`, `"Slots": -1`, `rows[0].Slots: differs from "slots" only in letter case`},
		{"cases", "DOCTRINE §5", shelvesFile, `"slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "delete": 7, "i": 30`,
			`"Slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "DELETE": 7, "I": 30`,
			`rows[0].DELETE: differs from "delete" only in letter case`},
		{"dup", "WIRE.md §3.2", shelvesFile, `"i": 30`, `"i": 30, "i": 31`, `rows[0].i: duplicate key`},
		{"dupschema", "WIRE.md §3.2", shelvesFile, `"rows": [`, `"$schema": "demo.shop.Shelf@0000000b", "rows": [`, `$schema: duplicate key`},
		{"envcase", "log-2026-09-24 envelope", shelvesFile, `"$schema"`, `"$Schema"`, `$Schema: differs from "$schema" only in letter case`},
		{"envrows", "log-2026-09-24 envelope", shelvesFile, `"rows"`, `"ROWS"`, `ROWS: differs from "rows" only in letter case`},
		{"notobject", "WIRE.md §3.1", shelvesFile, "", `[]`, `not a canon data file (invalid JSON)`},
		{"trailing", "WIRE.md §3.1", shelvesFile, "", `{"$schema": "demo.shop.Shelf@0000000b", "rows": []} {}`, `not a canon data file (invalid JSON)`},
		{"deep", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK + 1),
			`rows[0].waits` + strings.Repeat("[0]", nestedOK+1) + `: nested deeper than 512 levels`},
		{"deepest", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK), `rows[0].waits[0]: expected an integer`},
		{"gap", "WIRE.md §5.14", shelvesFile, `"b0": "warning", "v0": 3, `, ``, `rows[0].b1: pairs slot after an empty one`},
		{"incomplete", "WIRE.md §5.14", shelvesFile, `"v0": 3, `, ``, `rows[0].b0: incomplete pairs slot`},
		{"nullslot", "WIRE.md §5.14", shelvesFile, `"v0": 3`, `"v0": null`, `rows[0].v0: missing`},
		{"bits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": 13`, `rows[0].flags: unknown bits 0x8`},
		{"negbits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": -1`, `rows[0].flags: expected a non-negative integer`},
		{"nullelem", "WIRE.md §5.4", shelvesFile, `"waits": [1, 2]`, `"waits": [1, null]`, `rows[0].waits[1]: null`},
		{"nullref", "WIRE.md §5.4", shelvesFile, `"items": ["sword", "anvil"]`, `"items": ["sword", null]`, `rows[0].items[1]: null`},
		{"nullrow", "WIRE.md §5.4", shelvesFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`},
		{"int", "WIRE.md §5.2", itemsFile, `"isTradable": 1`, `"isTradable": 2`, `rows[0].isTradable: expected 0 or 1, not 2`},
		{"nullcell", "WIRE.md §5.11", itemsFile, `"$rank": {"warning": {"false": 0`, `"$rank": {"warning": {"false": null`, `rows[0].$rank.warning.false: null`},
		{"nocell", "WIRE.md §5.11", itemsFile, `"$labelIn": {"1": "fire", "300": null}`, `"$labelIn": {"1": "fire"}`, `rows[0].$labelIn.300: missing`},
		{"domaincase", "log-2026-09-24 key case", itemsFile, `"$rank": {"warning"`, `"$rank": {"WARNING"`, `rows[0].$rank.WARNING: differs from "warning" only in letter case`},
		{"nulllistcell", "WIRE.md §5.11", itemsFile, `"$kin": {"false": [], "true": ["air", "anvil"]}`, `"$kin": {"false": null, "true": ["air", "anvil"]}`, `rows[0].$kin.false: null`},
		{"badlistcell", "log-2026-09-24 lookup cells", itemsFile, `"true": ["air", "anvil"]`, `"true": ["air", "nope"]`, `rows[0].$kin.true[1]: no entry nope`},
		{"badcell", "log-2026-09-24 lookup cells", itemsFile, `"$pairFor": {"warning": "air"`, `"$pairFor": {"warning": "nope"`, `rows[0].$pairFor.warning: no entry nope`},
		{"nullrecord", "WIRE.md §5.4", itemsFile, `{"x": 3, "y": 4}]`, `null]`, `rows[0].path[1]: null`},
		{"nullid", "WIRE.md §5.7", itemsFile, `{"$id": "sword"`, `{"$id": null`, `rows[0].$id: null`},
		{"retired", "WIRE.md §5.7", itemsFile, `"$retired": true`, `"$retired": false`, `rows[1].$retired: not true`},
		{"tablerow", "WIRE.md §5.4", itemsFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`},
		{"nullpath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": null`, `rows[0].legacy: null`},
		{"pathcase", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": {"MAX": 9}`, `rows[0].legacy.MAX: differs from "max" only in letter case`},
		{"nopath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}, `, ``, `rows[0].legacy.max: missing`},
		{"tagcase", "WIRE.md §5.6", itemsFile, `{"type": "item", "itemId": "II_GEM"`, `{"Type": "item", "itemId": "II_GEM"`, `rows[0].reward.Type: differs from "type" only in letter case`},
		{"casefield", "WIRE.md §5.6", itemsFile, `"itemId": "II_GEM"`, `"ItemId": "II_GEM"`, `rows[0].reward.ItemId: differs from "itemId" only in letter case`},
		{"inlinecase", "WIRE.md §5.6", configFile, `"itemId": "II_KEY"`, `"itemID": "II_KEY"`, `value.itemID: differs from "itemId" only in letter case`},
		{"fnscase", "WIRE.md §5.11", configFile, `"value": {`, `"$FNS": {}, "value": {`, `$FNS: differs from "$fns" only in letter case`},
	}
}

// write writes the case's files under dir and returns the driver's argument and expected line.
func (c strictCase) write(t *testing.T, dir string) (arg, want string) {
	t.Helper()
	caseDir := filepath.Join(dir, c.name)
	files := []string{itemsFile, configFile}
	if c.file == shelvesFile {
		files = []string{shelvesFile}
	}
	for _, f := range files {
		copyFile(t, filepath.Join("testdata", "main", "shop", "good", f), filepath.Join(caseDir, f), func(b []byte) []byte {
			switch {
			case f != c.file:
				return b
			case c.old == "":
				return []byte(c.new)
			case !bytes.Contains(b, []byte(c.old)):
				t.Fatalf("%s: %s holds no %s", c.name, f, c.old)
			}
			return bytes.Replace(b, []byte(c.old), []byte(c.new), 1)
		})
	}
	path, want := filepath.Join(caseDir, c.file), "ok"
	if c.want != "" {
		want = path + ": " + c.want
	}
	if c.file == shelvesFile {
		return "shelves=" + path, want
	}
	return "reload=" + caseDir, want
}

// CODEGEN.md §7.6: a loader refuses every file canon never writes, naming the key's path, in every build.
func TestStrictLoads(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, generate(t, constructs()), "strict_main.cpp")
	cases := strictCases()
	args, wants := make([]string, len(cases)), make([]string, len(cases))
	for i, c := range cases {
		args[i], wants[i] = c.write(t, dir)
	}
	for _, out := range buildAndRun(t, dir, []string{"shop.gen.cpp", "main.cpp"}, args...) {
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(got) != len(cases) {
			t.Fatalf("%d lines for %d cases:\n%s", len(got), len(cases), out)
		}
		for i, c := range cases {
			if got[i] != wants[i] {
				t.Errorf("%s (%s):\n got %s\nwant %s", c.name, c.rule, got[i], wants[i])
			}
		}
	}
}
