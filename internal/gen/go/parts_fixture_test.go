package gogen_test

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The synthetic packages of the part-cutting tests (CODEGEN.md §2.11, DECISIONS 435): a baked package of rows and a types package of one wide record.
const (
	partsModule = "example.com/parts"
	partsRows   = "parts.rows"
	partsWide   = "parts.wide"
	rowKeyFmt   = "k%03d"
	wideField   = "f%02d"
	shapeCount  = 8
)

// partsRowsPackage is a baked package: a table of n Item rows, a keyed list of n Entry records and one Item value.
func partsRowsPackage(n int) *pkgSpec {
	items, named := valueSpec{Name: "items", Type: "table Item"}, valueSpec{Name: "named", Type: "[Entry keyed by label]"}
	var listed []map[string]json.RawMessage
	for i := range n {
		fields := map[string]json.RawMessage{
			"label": json.RawMessage(strconv.Quote(fmt.Sprintf("row%03d", i))),
			"n":     json.RawMessage(strconv.Itoa(i)),
			"tone":  json.RawMessage(strconv.Quote([]string{"low", "mid", "high"}[i%3])),
		}
		items.Entries = append(items.Entries, entrySpec{ID: fmt.Sprintf(rowKeyFmt, i), Fields: fields})
		listed = append(listed, fields)
	}
	raw, _ := json.Marshal(listed)
	named.Value = raw
	return &pkgSpec{
		Name: partsRows, Dir: "parts/rows",
		Emit:  emitSpec{Out: "out/", Dir: "parts/rows", Import: partsModule + "/rows", Package: "rows", Mode: "baked", Values: []string{"items", "named", "home"}},
		Enums: []enumSpec{{Name: "Tone", Members: []string{"low", "mid", "high"}}},
		Records: []recordSpec{
			{Name: "Item", Fields: []fieldSpec{{Name: "label", Type: "String"}, {Name: "n", Type: "Int"}, {Name: "tone", Type: "Tone"}}},
			{Name: "Entry", Fields: []fieldSpec{{Name: "label", Type: "String"}, {Name: "n", Type: "Int"}}},
		},
		Values: []valueSpec{items, named, {
			Name: "home", Type: "Item", Value: json.RawMessage(`{"label":"home","n":7,"tone":"mid"}`),
		}},
	}
}

// wideShapes are the types a wide record's fields cycle through, with a sample of each on the wire.
var wideShapes = [shapeCount]struct{ typ, wire string }{
	{"Int", "%d"}, {"String", `"s%d"`}, {"Bool", "true"}, {"Float", "%d.5"},
	{"Duration", "%d"}, {"Tone", `"mid"`}, {"String?", `"o%d"`}, {"[Int]", "[%d, 1]"},
}

// widePackage is a types package of one record Wide of n fields, cycling through wideShapes, and the enum Tone it reads.
func widePackage(n int) *pkgSpec {
	wide := recordSpec{Name: "Wide"}
	for i := range n {
		wide.Fields = append(wide.Fields, fieldSpec{Name: fmt.Sprintf(wideField, i), Type: wideShapes[i%shapeCount].typ})
	}
	return &pkgSpec{
		Name: partsWide, Dir: "parts/wide",
		Emit:    emitSpec{Out: "out/", Dir: "parts/wide", Import: partsModule + "/wide", Package: "wide", Mode: "types"},
		Enums:   []enumSpec{{Name: "Tone", Members: []string{"low", "mid", "high"}}},
		Records: []recordSpec{wide},
	}
}

// partsFixture is the rows package of rows rows and the wide package of wide fields.
func partsFixture(rows, wide int) *fixture {
	return &fixture{Module: partsModule, Packages: []*pkgSpec{partsRowsPackage(rows), widePackage(wide)}}
}

// partsRowsFixture is the rows package alone.
func partsRowsFixture(rows int) *fixture {
	return &fixture{Module: partsModule, Packages: []*pkgSpec{partsRowsPackage(rows)}}
}

// partsWideFixture is the wide package alone.
func partsWideFixture(wide int) *fixture {
	return &fixture{Module: partsModule, Packages: []*pkgSpec{widePackage(wide)}}
}
