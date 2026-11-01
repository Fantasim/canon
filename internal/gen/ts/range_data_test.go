package tsgen_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	goodTail    = `"$score":1,"$rank":{"lo":1,"hi":2}`
	itemsFile   = "items.json"
	itemsSchema = "$schema"
)

// itemsText is a data file of items with rows spliced in.
func itemsText(schema string, rows ...string) string {
	return fmt.Sprintf(`{"$schema": %q, "rows": [%s]}`, schema, strings.Join(rows, ","))
}

// itemRow is a row of items; tail is its stored `$` result and lookup.
func itemRow(name string, qty int, tail string) string {
	return fmt.Sprintf(`{"name":%q,"qty":%d,%s}`, name, qty, tail)
}

// dataErr is the data loader's line for a refusal at path of items.json.
func dataErr(path, message string) string { return "error " + itemsFile + ": " + path + ": " + message }

// rangeDataCases are the loader's refinements (CODEGEN.md §5.15): a row's fields, a stored `$` result and a lookup cell, then the rows' count; the text is in the case's `to`.
func rangeDataCases(schema string) []rangeCase {
	row := func(name string, qty int) string { return itemRow(name, qty, goodTail) }
	return []rangeCase{
		{"two", "", itemsText(schema, row("a", 1), row("b", 9)), rangeOK},
		{"none", "", itemsText(schema), rangeOK},
		{"three", "", itemsText(schema, row("a", 1), row("b", 2), row("c", 3)), dataErr("rows", countIn+"..=2")},
		{"name", "", itemsText(schema, row("a", 1), row("bcde", 2)), dataErr("rows[1].name", bytesIn+"1..=3")},
		{"qty", "", itemsText(schema, row("a", 10)), dataErr("rows[0].qty", valueIn+"0..=9")},
		{"dup-first", "", itemsText(schema, row("a", 1), row("a", 2), row("c", 3)), dataErr("rows[1].name", `duplicate id "a" (first at rows[0])`)},
		{"dup-before-count", "", itemsText(schema, row("a", 1), row("b", 2), row("b", 3)), dataErr("rows[2].name", `duplicate id "b" (first at rows[1])`)},
		{"row-before-dup", "", itemsText(schema, row("a", 1), row("a", 2), row("cdef", 3)), dataErr("rows[2].name", bytesIn+"1..=3")},
		{"row-first", "", itemsText(schema, row("a", 1), row("b", 2), row("cdef", 3)), dataErr("rows[2].name", bytesIn+"1..=3")},
		{"score-hi", "", itemsText(schema, itemRow("a", 1, `"$score":10,"$rank":{"lo":1,"hi":2}`)), dataErr("rows[0].$score", valueIn+"0..=9")},
		{"score-lo", "", itemsText(schema, itemRow("a", 1, `"$score":-1,"$rank":{"lo":1,"hi":2}`)), dataErr("rows[0].$score", valueIn+"0..=9")},
		{"rank-lo", "", itemsText(schema, itemRow("a", 1, `"$score":1,"$rank":{"lo":1,"hi":0}`)), dataErr("rows[0].$rank.hi", valueIn+"1..=6")},
		{"rank-hi", "", itemsText(schema, itemRow("a", 1, `"$score":1,"$rank":{"lo":7,"hi":2}`)), dataErr("rows[0].$rank.lo", valueIn+"1..=6")},
		// CODEGEN.md §5.13, WIRE.md §5.5, §5.11: a required key absent or null, a stored `$` result absent, a lookup cell absent.
		{"name-missing", "", itemsText(schema, `{"qty":1,`+goodTail+`}`), dataErr("rows[0].name", rangeMissing)},
		{"qty-null", "", itemsText(schema, `{"name":"a","qty":null,`+goodTail+`}`), dataErr("rows[0].qty", rangeNull)},
		{"score-missing", "", itemsText(schema, itemRow("a", 1, `"$rank":{"lo":1,"hi":2}`)), dataErr("rows[0].$score", rangeMissing)},
		{"rank-cell-missing", "", itemsText(schema, itemRow("a", 1, `"$score":1,"$rank":{"lo":1}`)), dataErr("rows[0].$rank.hi", rangeMissing)},
		{"rank-cell-null", "", itemsText(schema, itemRow("a", 1, `"$score":1,"$rank":{"lo":1,"hi":null}`)), dataErr("rows[0].$rank.hi", rangeNull)},
		{"rows-missing", "", fmt.Sprintf(`{"$schema": %q}`, schema), dataErr("rows", rangeMissing)},
		{"rows-null", "", fmt.Sprintf(`{"$schema": %q, "rows": null}`, schema), dataErr("rows", rangeNull)},
	}
}

// tagRow is a row of the table tags, its id and label.
func tagRow(id string) string { return fmt.Sprintf(`{"$id":%s,"label":"x"}`, id) }

// tagsText is the data file tags.json with rows spliced in.
func tagsText(schema string, rows ...string) string {
	return fmt.Sprintf(`{"$schema": %q, "rows": [%s]}`, schema, strings.Join(rows, ","))
}

// tagDupCases are the ids of a table, unique as WIRE.md §5.7 says: the token is the id as the data wire writes a String (WIRE.md §7.3).
func tagDupCases(schema string) []rangeCase {
	err := func(path, message string) string { return "error tags.json: " + path + ": " + message }
	return []rangeCase{
		{"tags-ok", "", tagsText(schema, tagRow(`"a"`), tagRow(`"b"`)), rangeOK},
		{"tags-dup", "", tagsText(schema, tagRow(`"a"`), tagRow(`"b"`), tagRow(`"a"`)), err("rows[2].$id", `duplicate id "a" (first at rows[0])`)},
		{"tags-dup-escaped", "", tagsText(schema, tagRow(`"a\t\u001b\""`), tagRow(`"a\t\u001b\""`)), err("rows[1].$id", `duplicate id "a\t\u001b\"" (first at rows[0])`)},
		// WIRE.md §5.7, CODEGEN.md §5.13: `$retired` must be true when present, else `expected true` at its path.
		{"tags-retired-true", "", tagsText(schema, `{"$id":"a","$retired":true,"label":"x"}`), rangeOK},
		{"tags-retired-false", "", tagsText(schema, `{"$id":"a","$retired":false,"label":"x"}`), err("rows[0].$retired", "expected true")},
		{"tags-retired-null", "", tagsText(schema, `{"$id":"a","$retired":null,"label":"x"}`), err("rows[0].$retired", "expected true")},
		{"tags-retired-text", "", tagsText(schema, `{"$id":"a","$retired":"true","label":"x"}`), err("rows[0].$retired", "expected true")},
		{"tags-id-missing", "", tagsText(schema, `{"label":"x"}`), err("rows[0].$id", rangeMissing)},
		{"tags-id-null", "", tagsText(schema, tagRow(`null`)), err("rows[0].$id", rangeNull)},
	}
}

// schemaOf is the `$schema` of the data file the json emit of w writes under file.
func schemaOf(t *testing.T, w *world, file string) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(w.generateJSON(t)[file], &doc); err != nil {
		t.Fatal(err)
	}
	schema, ok := doc[itemsSchema].(string)
	if !ok {
		t.Fatalf("%s has no %s: %v", file, itemsSchema, doc)
	}
	return schema
}

// CODEGEN.md §5.15, §5.9: a data-mode parse<V> refuses a row field outside its refinement at its path, a stored `$` result and a lookup cell too, and the rows' count after their elements.
func TestTSRangeDataRun(t *testing.T) {
	var runs []rangeRun
	w := rangeWorld(t, "range_data")
	for _, c := range rangeDataCases(schemaOf(t, w, itemsFile)) {
		runs = append(runs, rangeRun{"range_data/" + rangeModule, c.name, "parseItems", c.to, c.want})
	}
	for _, c := range tagDupCases(schemaOf(t, w, "tags.json")) {
		runs = append(runs, rangeRun{"range_data/" + rangeModule, c.name, "parseTags", c.to, c.want})
	}
	runRange(t, []string{"range_data"}, runs)
}
