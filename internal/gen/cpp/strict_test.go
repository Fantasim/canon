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
	// badSchema is how a loader names a file whose $schema is not the shelves'.
	badSchema = ", this binary expects demo.shop.Shelf@0000000b. Rebuild the data or deploy the matching binary"
	invalid   = "not a canon data file (invalid JSON)"
	shelvesOf = `{"$schema": "demo.shop.Shelf@0000000b", `
)

// strictCase is a data file canon never writes, made from testdata/main/shop/good by one edit
// (old "" replaces the whole file), and the exact error its generated loaders must return after
// "<path>: " ("" for a file that loads). Shelves load alone; items and config load together.
type strictCase struct {
	name, rule, file, old, new, want string
}

// strictCases are the rules of the loaders' shared vocabulary, each with a failing file.
func strictCases() []strictCase {
	deep := func(n int) string { return `"waits": [` + strings.Repeat("[", n) + strings.Repeat("]", n) + `]` }
	return append([]strictCase{
		{"good", "CODEGEN.md §7.6", shelvesFile, `"i": 30`, `"i": 30`, ""},
		{"bom", "WIRE.md §3.1", shelvesFile, `{`, "\xef\xbb\xbf{", ""},
		{"case", "log-2026-09-24 key case", shelvesFile, `"slots": -1`, `"Slots": -1`, `rows[0].Slots: differs from "slots" only in letter case`},
		{"cases", "DOCTRINE §5", shelvesFile, `"slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "delete": 7, "i": 30`,
			`"Slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "DELETE": 7, "I": 30`,
			`rows[0].DELETE: differs from "delete" only in letter case`},
		{"longs", "log-2026-09-24 loader parity (ASCII case rule)", shelvesFile, `"slots": -1`, "\"\u017flots\": -1", `rows[0].slots: missing`},
		{"kelvin", "log-2026-09-24 loader parity (ASCII case rule)", itemsFile, `"$kin": {"false": [], "true": ["air", "anvil"]}`, "\"$\u212ain\": {}", `rows[0].$kin: missing`},
		{"dup", "WIRE.md §3.2", shelvesFile, `"i": 30`, `"i": 30, "i": 31`, `rows[0].i: duplicate key`},
		{"dupschema", "WIRE.md §3.2", shelvesFile, `"rows": [`, `"$schema": "demo.shop.Shelf@0000000b", "rows": [`, `$schema: duplicate key`},
		{"envcase", "log-2026-09-24 envelope", shelvesFile, `"$schema"`, `"$Schema"`, `$Schema: differs from "$schema" only in letter case`},
		{"envrows", "log-2026-09-24 envelope", shelvesFile, `"rows"`, `"ROWS"`, `ROWS: differs from "rows" only in letter case`},
		{"noschema", "CODEGEN.md §6.3", shelvesFile, `"$schema": "demo.shop.Shelf@0000000b",`, ``, `built from schema <none>` + badSchema},
		{"emptyschema", "CODEGEN.md §6.3", shelvesFile, `"$schema": "demo.shop.Shelf@0000000b"`, `"$schema": ""`, `built from schema ` + badSchema},
		{"notobject", "WIRE.md §3.1", shelvesFile, "", `[]`, invalid},
		{"trailing", "WIRE.md §3.1", shelvesFile, "", shelvesOf + `"rows": []} {}`, invalid},
		{"utf8", "WIRE.md §3.1", shelvesFile, `"id": "front"`, "\"id\": \"fr\xffont\"", invalid},
		{"surrogate", "WIRE.md §3.2", shelvesFile, `"id": "front"`, `"id": "\ud800"`, invalid},
		{"hugenum", "WIRE.md §3.3", shelvesFile, `"delete": 7`, `"delete": 1e400`, invalid},
		{"deep", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK + 1),
			`rows[0].waits` + strings.Repeat("[0]", nestedOK+1) + `: nested deeper than 512 levels`},
		{"deepest", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK), `rows[0].waits[0]: expected an integer from -9223372036 to 9223372036`},
		{"rowsnull", "log-2026-09-24 null envelope", shelvesFile, "", shelvesOf + `"rows": null}`, `rows: null`},
		{"rowsmissing", "log-2026-09-24 null envelope", shelvesFile, "", `{"$schema": "demo.shop.Shelf@0000000b"}`, `rows: missing`},
		{"rowskind", "log-2026-09-24 loader parity (kind)", shelvesFile, "", shelvesOf + `"rows": {}}`, `rows: expected an array`},
		{"valuenull", "log-2026-09-24 null envelope", configFile, "", `{"$schema": "demo.shop.Config@0000000c", "value": null}`, `value: null`},
		{"valuekind", "log-2026-09-24 loader parity (kind)", configFile, "", `{"$schema": "demo.shop.Config@0000000c", "value": []}`, `value: expected an object`},
	}, fieldCases()...)
}

// fieldCases break one field, key or row of a canon-written file.
func fieldCases() []strictCase {
	return append([]strictCase{
		{"nullfield", "WIRE.md §5.4", shelvesFile, `"slots": -1`, `"slots": null`, `rows[0].slots: null`},
		{"narrow", "TYPES.md §7.2", shelvesFile, `"slots": -1`, `"slots": 300`, `rows[0].slots: expected an integer from -128 to 127`},
		{"fraction", "WIRE.md §5.1", shelvesFile, `"slots": -1`, `"slots": 1.5`, `rows[0].slots: expected an integer from -128 to 127`},
		{"int", "WIRE.md §5.1", shelvesFile, `"delete": 7`, `"delete": "7"`, `rows[0].delete: expected an integer`},
		{"duration", "TYPES.md §7.2", shelvesFile, `"waits": [1, 2]`, `"waits": [1, 9223372037]`, `rows[0].waits[1]: expected an integer from -9223372036 to 9223372036`},
		{"gap", "WIRE.md §5.14", shelvesFile, `"b0": "warning", "v0": 3, `, ``, `rows[0].b1: pairs slot after an empty one`},
		{"incomplete", "WIRE.md §5.14", shelvesFile, `"v0": 3, `, ``, `rows[0].b0: incomplete pairs slot`},
		{"incompletev", "WIRE.md §5.14", shelvesFile, `"b0": "warning", `, ``, `rows[0].v0: incomplete pairs slot`},
		{"nullslot", "WIRE.md §5.14", shelvesFile, `"v0": 3`, `"v0": null`, `rows[0].b0: null in a pairs slot`},
		{"nullslotk", "WIRE.md §5.14", shelvesFile, `"b0": "warning"`, `"b0": null`, `rows[0].b0: null in a pairs slot`},
		{"bits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": 13`, `rows[0].flags: unknown bits 0x8`},
		{"negbits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": -1`, `rows[0].flags: expected an integer from 0 to 9223372036854775807`},
		{"nullelem", "WIRE.md §5.4", shelvesFile, `"waits": [1, 2]`, `"waits": [1, null]`, `rows[0].waits[1]: null`},
		{"nullref", "WIRE.md §5.4", shelvesFile, `"items": ["sword", "anvil"]`, `"items": ["sword", null]`, `rows[0].items[1]: null`},
		{"nullrow", "WIRE.md §5.4", shelvesFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`},
		{"rowkind", "log-2026-09-24 loader parity (kind)", shelvesFile, `"rows": [`, `"rows": [3, `, `rows[0]: expected an object`},
		{"int01", "WIRE.md §5.2", itemsFile, `"isTradable": 1`, `"isTradable": 2`, `rows[0].isTradable: expected an integer from 0 to 1`},
		{"float32", "WIRE.md §5.1", itemsFile, `"weight": 3.25`, `"weight": 1e39`, `rows[0].weight: expected a number that fits Float32`},
		{"float", "log-2026-09-24 loader parity (kind)", itemsFile, `"price": 12.5`, `"price": "x"`, `rows[0].price: expected a number`},
		{"bool", "log-2026-09-24 loader parity (kind)", itemsFile, `"$heavy": false`, `"$heavy": 1`, `rows[0].$heavy: expected true or false`},
		{"uint64", "TYPES.md §7.2", itemsFile, `"amount": 9000000000`, `"amount": -1`, `rows[1].reward.amount: expected an integer from 0 to 9223372036854775807`},
		{"code", "WIRE.md §5.3", itemsFile, `"element": 300`, `"element": 5`, `rows[0].element: unknown value 5`},
		{"coderange", "WIRE.md §5.3", itemsFile, `"element": 300`, `"element": 70000`, `rows[0].element: expected an integer from 0 to 65535`},
		{"enum", "WIRE.md §5.3", itemsFile, `"tone": "series-1"`, `"tone": "loud"`, `rows[0].tone: unknown value loud`},
		{"enumkind", "log-2026-09-24 loader parity (kind)", itemsFile, `"tone": "series-1"`, `"tone": 3`, `rows[0].tone: expected a string`},
		{"order", "CODEGEN.md §7.6 (declaration order)", itemsFile, `"code": 300, "label": "Sword"`, `"label": 3, "code": "x"`, `rows[0].code: expected an integer from 0 to 65535`},
		{"orderconv", "CODEGEN.md §7.6 (declaration order)", itemsFile, `"tone": "series-1", "element": 300`, `"tone": "loud", "element": "x"`, `rows[0].tone: unknown value loud`},
	}, keyCases()...)
}

// keyCases break a row's `$` key, a variant, a path or a list.
func keyCases() []strictCase {
	return []strictCase{
		{"tag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"type": "gem", "itemId": "II_GEM"`, `rows[0].reward.type: unknown case gem`},
		{"nulltag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"type": null, "itemId": "II_GEM"`, `rows[0].reward.type: null`},
		{"notag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"itemId": "II_GEM"`, `rows[0].reward.type: missing`},
		{"nullcell", "WIRE.md §5.11", itemsFile, `"$rank": {"warning": {"false": 0`, `"$rank": {"warning": {"false": null`, `rows[0].$rank.warning.false: null`},
		{"nocell", "WIRE.md §5.11", itemsFile, `"$labelIn": {"1": "fire", "300": null}`, `"$labelIn": {"1": "fire"}`, `rows[0].$labelIn.300: missing`},
		{"cellkind", "WIRE.md §5.11", itemsFile, `"$labelIn": {"1": "fire"`, `"$labelIn": {"1": 3`, `rows[0].$labelIn.1: expected a string`},
		{"levelnull", "WIRE.md §5.11", itemsFile, `{"warning": {"false": 0, "true": 1}, `, `{"warning": null, `, `rows[0].$rank.warning: null`},
		{"levelkind", "WIRE.md §5.11", itemsFile, `{"warning": {"false": 0, "true": 1}, `, `{"warning": 3, `, `rows[0].$rank.warning: expected an object`},
		{"tablenull", "WIRE.md §5.11", itemsFile, `"$rank": {"warning"`, `"$rank": null, "$ranks": {"warning"`, `rows[0].$rank: null`},
		{"domaincase", "log-2026-09-24 key case", itemsFile, `"$rank": {"warning"`, `"$rank": {"WARNING"`, `rows[0].$rank.WARNING: differs from "warning" only in letter case`},
		{"nulllistcell", "WIRE.md §5.11", itemsFile, `"$kin": {"false": [], "true": ["air", "anvil"]}`, `"$kin": {"false": null, "true": ["air", "anvil"]}`, `rows[0].$kin.false: null`},
		{"badlistcell", "log-2026-09-24 lookup cells", itemsFile, `"true": ["air", "anvil"]`, `"true": ["air", "nope"]`, `rows[0].$kin.true[1]: no entry nope`},
		{"badcell", "log-2026-09-24 lookup cells", itemsFile, `"$pairFor": {"warning": "air"`, `"$pairFor": {"warning": "nope"`, `rows[0].$pairFor.warning: no entry nope`},
		{"noentry", "CODEGEN.md §5.8", itemsFile, `"$best": "anvil"`, `"$best": "nope"`, `rows[0].$best: no entry nope`},
		{"nullrecord", "WIRE.md §5.4", itemsFile, `{"x": 3, "y": 4}]`, `null]`, `rows[0].path[1]: null`},
		{"recelem", "log-2026-09-24 loader parity (kind)", itemsFile, `"path": [{"x": 1, "y": 2}, {"x": 3, "y": 4}]`, `"path": [1]`, `rows[0].path[0]: expected an object`},
		{"list", "log-2026-09-24 loader parity (kind)", itemsFile, `"tags": ["sharp", "metal"]`, `"tags": "x"`, `rows[0].tags: expected an array`},
		{"elem", "log-2026-09-24 loader parity (kind)", itemsFile, `"tags": ["sharp", "metal"]`, `"tags": ["sharp", 3]`, `rows[0].tags[1]: expected a string`},
		{"nullid", "WIRE.md §5.7", itemsFile, `{"$id": "sword"`, `{"$id": null`, `rows[0].$id: null`},
		{"noid", "WIRE.md §5.7", itemsFile, `{"$id": "sword", `, `{`, `rows[0].$id: missing`},
		{"idkind", "WIRE.md §5.7", itemsFile, `{"$id": "sword"`, `{"$id": 7`, `rows[0].$id: expected a string`},
		{"retired", "WIRE.md §5.7", itemsFile, `"$retired": true`, `"$retired": false`, `rows[1].$retired: expected true`},
		{"retirednull", "WIRE.md §5.7", itemsFile, `"$retired": true`, `"$retired": null`, `rows[1].$retired: expected true`},
		{"tablerow", "WIRE.md §5.4", itemsFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`},
		{"nullpath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": null`, `rows[0].legacy: null`},
		{"pathkind", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": 3`, `rows[0].legacy: expected an object`},
		{"pathcase", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": {"MAX": 9}`, `rows[0].legacy.MAX: differs from "max" only in letter case`},
		{"nopath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}, `, ``, `rows[0].legacy.max: missing`},
		{"tagcase", "WIRE.md §5.6", itemsFile, `{"type": "item", "itemId": "II_GEM"`, `{"Type": "item", "itemId": "II_GEM"`, `rows[0].reward.Type: differs from "type" only in letter case`},
		{"casefield", "WIRE.md §5.6", itemsFile, `"itemId": "II_GEM"`, `"ItemId": "II_GEM"`, `rows[0].reward.ItemId: differs from "itemId" only in letter case`},
		{"inlinecase", "WIRE.md §5.6", configFile, `"itemId": "II_KEY"`, `"itemID": "II_KEY"`, `value.itemID: differs from "itemId" only in letter case`},
		{"fnscase", "WIRE.md §5.11", configFile, `"value": {`, `"$FNS": {}, "value": {`, `$FNS: differs from "$fns" only in letter case`},
	}
}

// write writes the case's files under dir and returns the drivers' argument and expected line.
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
