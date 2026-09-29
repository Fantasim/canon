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
	// goodShelves is strict_main's dump of testdata/main/shop/good/shelves.json, unedited.
	goodShelves = "front:slots=-1,delete=7,i=30 back:slots=12,delete=8,i=31"
	// goodAnvilAir is anvil's and air's segments of strict_main's dump of items.json, unedited.
	goodAnvilAir = "anvil:code=7,price=+1,weight=+150,bonus=-2,legacyMax=-1 air:code=12,price=+0,weight=+0,bonus=0,legacyMax=0"
	// goodConfig is strict_main's dump of config.json's scale field, unedited.
	goodConfig = "config:scale=+1.5"
	// escapedShelf is a shelf whose id is WIRE.md §7.3's sample, "a<TAB>é/\"<ESC>".
	escapedShelf = `{"id": "a\t\u00e9/\"\u001b", "slots": 1, "items": [], "next": null, "delete": 8, "i": 31, "flags": 0, "waits": []}`
)

// caseMode is where a case's files come from and how the drivers load them.
type caseMode int

const (
	// shopFiles: testdata/main/shop/good, one file edited.
	shopFiles caseMode = iota
	// missingFile: no file at all.
	missingFile
	// nestFile: testdata/main/nest/good/value.json, edited.
	nestFile
	// commaLocale: as shopFiles, loaded by the C++ driver under a locale whose decimal point is ','.
	commaLocale
)

// strictCase is a data file canon never writes, one edit from testdata/main/shop/good or
// testdata/main/nest/good (mode); want is the error after "<path>: ", dump the success line otherwise.
type strictCase struct {
	name, rule, file, old, new, want, dump string
	mode                                   caseMode
}

// strictCases are the rules of the loaders' shared vocabulary, each with a failing file.
func strictCases() []strictCase {
	deep := func(n int) string { return `"waits": [` + strings.Repeat("[", n) + strings.Repeat("]", n) + `]` }
	return append([]strictCase{
		{"good", "CODEGEN.md §7.6", shelvesFile, `"i": 30`, `"i": 30`, "", goodShelves, shopFiles},
		{"bom", "WIRE.md §3.1", shelvesFile, `{`, "\xef\xbb\xbf{", "", goodShelves, shopFiles},
		{"dupid", "WIRE.md §5.7, log-2026-09-24 loader parity (duplicate row ids)", shelvesFile,
			`"id": "back"`, `"id": "front"`, `rows[1].id: duplicate id "front" (first at rows[0])`, "", shopFiles},
		{"dupescaped", "WIRE.md §5.7, §7.3, log-2026-09-24 loader parity (an escaped key's token)", shelvesFile, "",
			shelvesOf + `"rows": [` + escapedShelf + `, ` + escapedShelf + `]}`,
			`rows[1].id: duplicate id "a\té/\"\u001b" (first at rows[0])`, "", shopFiles},
		{"reloadgood", "log-2026-09-24 loader parity (dump: every field, optionals, Config)", itemsFile,
			`"$schema": "demo.shop.Item@0000000a"`, `"$schema": "demo.shop.Item@0000000a"`, "",
			"sword:code=300,price=+12.5,weight=+3.25,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"negzerofloat", "WIRE.md §5.1, log-2026-09-24 loader parity (-0 Float reads as +0)", itemsFile,
			`"price": 12.5`, `"price": -0`, "",
			"sword:code=300,price=+0,weight=+3.25,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"negzerofloat32", "WIRE.md §5.1, log-2026-09-24 loader parity (-0.0 Float32 reads as +0)", itemsFile,
			`"weight": 3.25`, `"weight": -0.0`, "",
			"sword:code=300,price=+12.5,weight=+0,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"float32round", "WIRE.md §5.1, log-2026-09-24 loader parity (Float32 rounds once, from the token)", itemsFile,
			`"weight": 3.25`, `"weight": 1.000000059604644775400625`, "",
			"sword:code=300,price=+12.5,weight=+1.00000012,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"negzeroexp", "WIRE.md §5.1, log-2026-09-24 loader parity (-0e5 Float reads as +0)", itemsFile,
			`"price": 12.5`, `"price": -0e5`, "",
			"sword:code=300,price=+0,weight=+3.25,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"negunderflow", "WIRE.md §5.1, log-2026-09-24 loader parity (a negative underflow reads as +0)", itemsFile,
			`"weight": 3.25`, `"weight": -1e-400`, "",
			"sword:code=300,price=+12.5,weight=+0,bonus=none,legacyMax=9 " + goodAnvilAir + " " + goodConfig, shopFiles},
		{"missingfile", "CODEGEN.md §7.5, log-2026-09-24 loader parity (missing file)", shelvesFile,
			"", "", "", "", missingFile},
		{"case", "log-2026-09-24 key case", shelvesFile, `"slots": -1`, `"Slots": -1`, `rows[0].Slots: differs from "slots" only in letter case`, "", shopFiles},
		{"cases", "DOCTRINE §5", shelvesFile, `"slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "delete": 7, "i": 30`,
			`"Slots": -1, "items": ["sword", "anvil"], "next": "back", "b0": "warning", "v0": 3, "b1": "info", "v1": 5, "DELETE": 7, "I": 30`,
			`rows[0].DELETE: differs from "delete" only in letter case`, "", shopFiles},
		{"longs", "log-2026-09-24 loader parity (ASCII case rule)", shelvesFile, `"slots": -1`, "\"\u017flots\": -1", `rows[0].slots: missing`, "", shopFiles},
		{"kelvin", "log-2026-09-24 loader parity (ASCII case rule)", itemsFile, `"$kin": {"false": [], "true": ["air", "anvil"]}`, "\"$\u212ain\": {}", `rows[0].$kin: missing`, "", shopFiles},
		{"dup", "WIRE.md §3.2", shelvesFile, `"i": 30`, `"i": 30, "i": 31`, `rows[0].i: duplicate key`, "", shopFiles},
		{"dupschema", "WIRE.md §3.2", shelvesFile, `"rows": [`, `"$schema": "demo.shop.Shelf@0000000b", "rows": [`, `$schema: duplicate key`, "", shopFiles},
		{"envcase", "log-2026-09-24 envelope", shelvesFile, `"$schema"`, `"$Schema"`, `$Schema: differs from "$schema" only in letter case`, "", shopFiles},
		{"envrows", "log-2026-09-24 envelope", shelvesFile, `"rows"`, `"ROWS"`, `ROWS: differs from "rows" only in letter case`, "", shopFiles},
		{"noschema", "CODEGEN.md §6.3", shelvesFile, `"$schema": "demo.shop.Shelf@0000000b",`, ``, `built from schema <none>` + badSchema, "", shopFiles},
		{"emptyschema", "CODEGEN.md §6.3", shelvesFile, `"$schema": "demo.shop.Shelf@0000000b"`, `"$schema": ""`, `built from schema ` + badSchema, "", shopFiles},
		{"notobject", "WIRE.md §3.1", shelvesFile, "", `[]`, invalid, "", shopFiles},
		{"trailing", "WIRE.md §3.1", shelvesFile, "", shelvesOf + `"rows": []} {}`, invalid, "", shopFiles},
		{"utf8", "WIRE.md §3.1", shelvesFile, `"id": "front"`, "\"id\": \"fr\xffont\"", invalid, "", shopFiles},
		{"surrogate", "WIRE.md §3.2", shelvesFile, `"id": "front"`, `"id": "\ud800"`, invalid, "", shopFiles},
		{"hugenum", "WIRE.md §3.3", shelvesFile, `"delete": 7`, `"delete": 1e400`, invalid, "", shopFiles},
		{"deep", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK + 1),
			`rows[0].waits` + strings.Repeat("[0]", nestedOK+1) + `: nested deeper than 512 levels`, "", shopFiles},
		{"deepest", "WIRE.md §3.1", shelvesFile, `"waits": [1, 2]`, deep(nestedOK), `rows[0].waits[0]: expected an integer from -9223372036 to 9223372036`, "", shopFiles},
		{"rowsnull", "log-2026-09-24 null envelope", shelvesFile, "", shelvesOf + `"rows": null}`, `rows: null`, "", shopFiles},
		{"rowsmissing", "log-2026-09-24 null envelope", shelvesFile, "", `{"$schema": "demo.shop.Shelf@0000000b"}`, `rows: missing`, "", shopFiles},
		{"rowskind", "log-2026-09-24 loader parity (kind)", shelvesFile, "", shelvesOf + `"rows": {}}`, `rows: expected an array`, "", shopFiles},
		{"valuenull", "log-2026-09-24 null envelope", configFile, "", `{"$schema": "demo.shop.Config@0000000c", "value": null}`, `value: null`, "", shopFiles},
		{"valuekind", "log-2026-09-24 loader parity (kind)", configFile, "", `{"$schema": "demo.shop.Config@0000000c", "value": []}`, `value: expected an object`, "", shopFiles},
	}, fieldCases()...)
}

// fieldCases break one field, key or row of a canon-written file.
func fieldCases() []strictCase {
	return append([]strictCase{
		{"nullfield", "WIRE.md §5.4", shelvesFile, `"slots": -1`, `"slots": null`, `rows[0].slots: null`, "", shopFiles},
		{"narrow", "TYPES.md §7.2", shelvesFile, `"slots": -1`, `"slots": 300`, `rows[0].slots: expected an integer from -128 to 127`, "", shopFiles},
		{"fraction", "WIRE.md §5.1", shelvesFile, `"slots": -1`, `"slots": 1.5`, `rows[0].slots: expected an integer from -128 to 127`, "", shopFiles},
		{"int", "WIRE.md §5.1", shelvesFile, `"delete": 7`, `"delete": "7"`, `rows[0].delete: expected an integer`, "", shopFiles},
		{"duration", "TYPES.md §7.2", shelvesFile, `"waits": [1, 2]`, `"waits": [1, 9223372037]`, `rows[0].waits[1]: expected an integer from -9223372036 to 9223372036`, "", shopFiles},
		{"gap", "WIRE.md §5.14", shelvesFile, `"b0": "warning", "v0": 3, `, ``, `rows[0].b1: pairs slot after an empty one`, "", shopFiles},
		{"incomplete", "WIRE.md §5.14", shelvesFile, `"v0": 3, `, ``, `rows[0].b0: incomplete pairs slot`, "", shopFiles},
		{"incompletev", "WIRE.md §5.14", shelvesFile, `"b0": "warning", `, ``, `rows[0].v0: incomplete pairs slot`, "", shopFiles},
		{"nullslot", "WIRE.md §5.14", shelvesFile, `"v0": 3`, `"v0": null`, `rows[0].b0: null in a pairs slot`, "", shopFiles},
		{"nullslotk", "WIRE.md §5.14", shelvesFile, `"b0": "warning"`, `"b0": null`, `rows[0].b0: null in a pairs slot`, "", shopFiles},
		{"bits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": 13`, `rows[0].flags: unknown bits 0x8`, "", shopFiles},
		{"negbits", "WIRE.md §5.3", shelvesFile, `"flags": 5`, `"flags": -1`, `rows[0].flags: expected an integer from 0 to 9223372036854775807`, "", shopFiles},
		{"nullelem", "WIRE.md §5.4", shelvesFile, `"waits": [1, 2]`, `"waits": [1, null]`, `rows[0].waits[1]: null`, "", shopFiles},
		{"nullref", "WIRE.md §5.4", shelvesFile, `"items": ["sword", "anvil"]`, `"items": ["sword", null]`, `rows[0].items[1]: null`, "", shopFiles},
		{"nullrow", "WIRE.md §5.4", shelvesFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`, "", shopFiles},
		{"rowkind", "log-2026-09-24 loader parity (kind)", shelvesFile, `"rows": [`, `"rows": [3, `, `rows[0]: expected an object`, "", shopFiles},
		{"int01", "WIRE.md §5.2", itemsFile, `"isTradable": 1`, `"isTradable": 2`, `rows[0].isTradable: expected an integer from 0 to 1`, "", shopFiles},
		{"float32", "WIRE.md §5.1", itemsFile, `"weight": 3.25`, `"weight": 1e39`, `rows[0].weight: expected a number that fits Float32`, "", shopFiles},
		{"float", "log-2026-09-24 loader parity (kind)", itemsFile, `"price": 12.5`, `"price": "x"`, `rows[0].price: expected a number`, "", shopFiles},
		{"bool", "log-2026-09-24 loader parity (kind)", itemsFile, `"$heavy": false`, `"$heavy": 1`, `rows[0].$heavy: expected true or false`, "", shopFiles},
		{"uint64", "TYPES.md §7.2", itemsFile, `"amount": 9000000000`, `"amount": -1`, `rows[1].reward.amount: expected an integer from 0 to 9223372036854775807`, "", shopFiles},
		{"code", "WIRE.md §5.3", itemsFile, `"element": 300`, `"element": 5`, `rows[0].element: unknown value 5`, "", shopFiles},
		{"coderange", "WIRE.md §5.3", itemsFile, `"element": 300`, `"element": 70000`, `rows[0].element: expected an integer from 0 to 65535`, "", shopFiles},
		{"enum", "WIRE.md §5.3", itemsFile, `"tone": "series-1"`, `"tone": "loud"`, `rows[0].tone: unknown value loud`, "", shopFiles},
		{"enumkind", "log-2026-09-24 loader parity (kind)", itemsFile, `"tone": "series-1"`, `"tone": 3`, `rows[0].tone: expected a string`, "", shopFiles},
		{"order", "CODEGEN.md §7.6 (declaration order)", itemsFile, `"code": 300, "label": "Sword"`, `"label": 3, "code": "x"`, `rows[0].code: expected an integer from 0 to 65535`, "", shopFiles},
		{"orderconv", "CODEGEN.md §7.6 (declaration order)", itemsFile, `"tone": "series-1", "element": 300`, `"tone": "loud", "element": "x"`, `rows[0].tone: unknown value loud`, "", shopFiles},
	}, keyCases()...)
}

// keyCases break a row's `$` key, a variant, a path or a list.
func keyCases() []strictCase {
	return []strictCase{
		{"tag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"type": "gem", "itemId": "II_GEM"`, `rows[0].reward.type: unknown case gem`, "", shopFiles},
		{"nulltag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"type": null, "itemId": "II_GEM"`, `rows[0].reward.type: null`, "", shopFiles},
		{"notag", "WIRE.md §5.6", itemsFile, `"type": "item", "itemId": "II_GEM"`, `"itemId": "II_GEM"`, `rows[0].reward.type: missing`, "", shopFiles},
		{"nullcell", "WIRE.md §5.11", itemsFile, `"$rank": {"warning": {"false": 0`, `"$rank": {"warning": {"false": null`, `rows[0].$rank.warning.false: null`, "", shopFiles},
		{"nocell", "WIRE.md §5.11", itemsFile, `"$labelIn": {"1": "fire", "300": null}`, `"$labelIn": {"1": "fire"}`, `rows[0].$labelIn.300: missing`, "", shopFiles},
		{"cellkind", "WIRE.md §5.11", itemsFile, `"$labelIn": {"1": "fire"`, `"$labelIn": {"1": 3`, `rows[0].$labelIn.1: expected a string`, "", shopFiles},
		{"levelnull", "WIRE.md §5.11", itemsFile, `{"warning": {"false": 0, "true": 1}, `, `{"warning": null, `, `rows[0].$rank.warning: null`, "", shopFiles},
		{"levelkind", "WIRE.md §5.11", itemsFile, `{"warning": {"false": 0, "true": 1}, `, `{"warning": 3, `, `rows[0].$rank.warning: expected an object`, "", shopFiles},
		{"tablenull", "WIRE.md §5.11", itemsFile, `"$rank": {"warning"`, `"$rank": null, "$ranks": {"warning"`, `rows[0].$rank: null`, "", shopFiles},
		{"domaincase", "log-2026-09-24 key case", itemsFile, `"$rank": {"warning"`, `"$rank": {"WARNING"`, `rows[0].$rank.WARNING: differs from "warning" only in letter case`, "", shopFiles},
		{"nulllistcell", "WIRE.md §5.11", itemsFile, `"$kin": {"false": [], "true": ["air", "anvil"]}`, `"$kin": {"false": null, "true": ["air", "anvil"]}`, `rows[0].$kin.false: null`, "", shopFiles},
		{"badlistcell", "log-2026-09-24 lookup cells", itemsFile, `"true": ["air", "anvil"]`, `"true": ["air", "nope"]`, `rows[0].$kin.true[1]: no entry nope`, "", shopFiles},
		{"badcell", "log-2026-09-24 lookup cells", itemsFile, `"$pairFor": {"warning": "air"`, `"$pairFor": {"warning": "nope"`, `rows[0].$pairFor.warning: no entry nope`, "", shopFiles},
		{"noentry", "CODEGEN.md §5.8", itemsFile, `"$best": "anvil"`, `"$best": "nope"`, `rows[0].$best: no entry nope`, "", shopFiles},
		{"nullrecord", "WIRE.md §5.4", itemsFile, `{"x": 3, "y": 4}]`, `null]`, `rows[0].path[1]: null`, "", shopFiles},
		{"recelem", "log-2026-09-24 loader parity (kind)", itemsFile, `"path": [{"x": 1, "y": 2}, {"x": 3, "y": 4}]`, `"path": [1]`, `rows[0].path[0]: expected an object`, "", shopFiles},
		{"list", "log-2026-09-24 loader parity (kind)", itemsFile, `"tags": ["sharp", "metal"]`, `"tags": "x"`, `rows[0].tags: expected an array`, "", shopFiles},
		{"elem", "log-2026-09-24 loader parity (kind)", itemsFile, `"tags": ["sharp", "metal"]`, `"tags": ["sharp", 3]`, `rows[0].tags[1]: expected a string`, "", shopFiles},
		{"nullid", "WIRE.md §5.7", itemsFile, `{"$id": "sword"`, `{"$id": null`, `rows[0].$id: null`, "", shopFiles},
		{"noid", "WIRE.md §5.7", itemsFile, `{"$id": "sword", `, `{`, `rows[0].$id: missing`, "", shopFiles},
		{"idkind", "WIRE.md §5.7", itemsFile, `{"$id": "sword"`, `{"$id": 7`, `rows[0].$id: expected a string`, "", shopFiles},
		{"retired", "WIRE.md §5.7", itemsFile, `"$retired": true`, `"$retired": false`, `rows[1].$retired: expected true`, "", shopFiles},
		{"retirednull", "WIRE.md §5.7", itemsFile, `"$retired": true`, `"$retired": null`, `rows[1].$retired: expected true`, "", shopFiles},
		{"tablerow", "WIRE.md §5.4", itemsFile, `"rows": [`, `"rows": [null, `, `rows[0]: null`, "", shopFiles},
		{"nullpath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": null`, `rows[0].legacy: null`, "", shopFiles},
		{"pathkind", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": 3`, `rows[0].legacy: expected an object`, "", shopFiles},
		{"pathcase", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}`, `"legacy": {"MAX": 9}`, `rows[0].legacy.MAX: differs from "max" only in letter case`, "", shopFiles},
		{"nopath", "WIRE.md §5.5.3", itemsFile, `"legacy": {"max": 9}, `, ``, `rows[0].legacy.max: missing`, "", shopFiles},
		{"tagcase", "WIRE.md §5.6", itemsFile, `{"type": "item", "itemId": "II_GEM"`, `{"Type": "item", "itemId": "II_GEM"`, `rows[0].reward.Type: differs from "type" only in letter case`, "", shopFiles},
		{"casefield", "WIRE.md §5.6", itemsFile, `"itemId": "II_GEM"`, `"ItemId": "II_GEM"`, `rows[0].reward.ItemId: differs from "itemId" only in letter case`, "", shopFiles},
		{"inlinecase", "WIRE.md §5.6", configFile, `"itemId": "II_KEY"`, `"itemID": "II_KEY"`, `value.itemID: differs from "itemId" only in letter case`, "", shopFiles},
		{"fnscase", "WIRE.md §5.11", configFile, `"value": {`, `"$FNS": {}, "value": {`, `$FNS: differs from "$fns" only in letter case`, "", shopFiles},
	}
}

// write writes the case's files under dir (none for a missing-file case) and returns the
// drivers' argument and expected line: the dump of the loaded values, or the error text
// (log-2026-09-24 loader parity: a missing file's own vocabulary carries no "<path>: " prefix).
func (c strictCase) write(t *testing.T, dir string) (arg, want string) {
	t.Helper()
	caseDir := filepath.Join(dir, c.name)
	path := filepath.Join(caseDir, c.file)
	// CODEGEN.md §5.11, §7.6: the snapshot loader's own dir+file join is always "/".
	if c.mode != nestFile && c.file != shelvesFile {
		path = caseDir + "/" + c.file
	}
	if c.mode != missingFile {
		c.writeFiles(t, caseDir)
	}
	want = c.dump
	switch {
	case c.mode == missingFile:
		want = "cannot open " + path
	case c.want != "":
		want = path + ": " + c.want
	}
	switch {
	case c.mode == nestFile:
		return "nested=" + path, want
	case c.file == shelvesFile:
		return "shelves=" + path, want
	case c.mode == commaLocale:
		return "reload:" + commaLocaleName + "=" + caseDir, want
	}
	return "reload=" + caseDir, want
}

// writeFiles copies shelves.json alone, items.json and config.json together, or (nested)
// demo.nest's single value.json, into caseDir, applying the case's one edit to c.file.
func (c strictCase) writeFiles(t *testing.T, caseDir string) {
	t.Helper()
	if c.mode == nestFile {
		copyFile(t, filepath.Join("testdata", "main", "nest", "good", c.file), filepath.Join(caseDir, c.file), func(b []byte) []byte {
			return c.applyEdit(t, b)
		})
		return
	}
	files := []string{itemsFile, configFile}
	if c.file == shelvesFile {
		files = []string{shelvesFile}
	}
	for _, f := range files {
		copyFile(t, filepath.Join("testdata", "main", "shop", "good", f), filepath.Join(caseDir, f), func(b []byte) []byte {
			if f != c.file {
				return b
			}
			return c.applyEdit(t, b)
		})
	}
}

// applyEdit is the case's one edit: old "" replaces the whole file with new.
func (c strictCase) applyEdit(t *testing.T, b []byte) []byte {
	t.Helper()
	switch {
	case c.old == "":
		return []byte(c.new)
	case !bytes.Contains(b, []byte(c.old)):
		t.Fatalf("%s: %s holds no %s", c.name, c.file, c.old)
	}
	return bytes.Replace(b, []byte(c.old), []byte(c.new), 1)
}
