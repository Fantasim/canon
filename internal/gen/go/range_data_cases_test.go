package gogen_test

import (
	"fmt"
	"strings"
)

// rangeItemsFile is a data file of demo.items with rows spliced in.
func rangeItemsFile(rows ...string) string {
	return `{"$schema": "demo.items.Item@1", "rows": [` + strings.Join(rows, ",") + `]}`
}

const goodTail = `"$score":1,"$rank":{"lo":1,"hi":2}`

// itemRow is a row of demo.items; tail is its stored `$` result and lookup.
func itemRow(name string, qty int, tail string) string {
	return fmt.Sprintf(`{"name":%q,"qty":%d,%s}`, name, qty, tail)
}

// rangeDataCases are the loader's refinements (CODEGEN.md §5.15): a row's fields, a stored `$` result and a lookup cell, then the rows' duplicates, then their count.
var rangeDataCases = []struct{ name, text, want string }{
	{"two", rangeItemsFile(itemRow("a", 1, goodTail), itemRow("b", 9, goodTail)), "ok"},
	{"none", rangeItemsFile(), "ok"},
	{"three", rangeItemsFile(itemRow("a", 1, goodTail), itemRow("b", 2, goodTail), itemRow("c", 3, goodTail)), "error NAME.json: rows: expected an element count in ..=2"},
	{"name", rangeItemsFile(itemRow("a", 1, goodTail), itemRow("bcde", 2, goodTail)), "error NAME.json: rows[1].name: expected a byte length in 1..=3"},
	{"qty", rangeItemsFile(itemRow("a", 10, goodTail)), "error NAME.json: rows[0].qty: expected a value in 0..=9"},
	{"dup-first", rangeItemsFile(itemRow("a", 1, goodTail), itemRow("a", 2, goodTail), itemRow("c", 3, goodTail)), "error NAME.json: rows[1].name: duplicate id \"a\" (first at rows[0])"},
	{"row-first", rangeItemsFile(itemRow("a", 1, goodTail), itemRow("b", 2, goodTail), itemRow("cdef", 3, goodTail)), "error NAME.json: rows[2].name: expected a byte length in 1..=3"},
	{"score-hi", rangeItemsFile(itemRow("a", 1, `"$score":10,"$rank":{"lo":1,"hi":2}`)), "error NAME.json: rows[0].$score: expected a value in 0..=9"},
	{"score-lo", rangeItemsFile(itemRow("a", 1, `"$score":-1,"$rank":{"lo":1,"hi":2}`)), "error NAME.json: rows[0].$score: expected a value in 0..=9"},
	{"rank-lo", rangeItemsFile(itemRow("a", 1, `"$score":1,"$rank":{"lo":1,"hi":0}`)), "error NAME.json: rows[0].$rank.hi: expected a value in 1..=6"},
	{"rank-hi", rangeItemsFile(itemRow("a", 1, `"$score":1,"$rank":{"lo":7,"hi":2}`)), "error NAME.json: rows[0].$rank.lo: expected a value in 1..=6"},
}
