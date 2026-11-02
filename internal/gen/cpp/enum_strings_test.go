package cppgen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// reNameTables are the string tables of an enum in a .gen.cpp: k<E>Names and k<E>Wires, at namespace scope.
var reNameTables = regexp.MustCompile(`(?s)std::array<std::string_view, \d+> (k\w+(?:Names|Wires)) = \{.*?\};\n`)

// reLocalTables are the function-local tables DECISIONS 383 wrote and 430 replaced: a name table, a wire table, a sorted copy of the wires and the enumerators in that order.
var reLocalTables = regexp.MustCompile(`\b(kNames|kWires|kValues|kIds)\b|std::array<std::string_view, \d+> kKeys|\n    static constexpr std::array<std::string_view`)

// tableTexts are the enum string tables of a .gen.cpp by name.
func tableTexts(source string) map[string]string {
	out := map[string]string{}
	for _, m := range reNameTables.FindAllStringSubmatch(source, -1) {
		out[m[1]] = m[0]
	}
	return out
}

// literalCount is how many times the string literal s is in the enum string tables of source.
func literalCount(source, s string) int {
	n := 0
	for _, text := range tableTexts(source) { //canon:unordered a sum
		n += strings.Count(text, `"`+s+`"`)
	}
	return n
}

// CODEGEN.md §5.2, §5.3, DECISIONS 430: an enum string is written once in the .gen.cpp: the Canon names in k<E>Names, the wires in k<E>Wires only when one differs, an order in k<E>ByWire, no function-local table; a kind enum and an id enum the same.
func TestEnumStringsWrittenOnce(t *testing.T) {
	source := fileOf(t, generate(t, helpersPackage()), ".gen.cpp")
	for _, tc := range []struct {
		literal string
		count   int
	}{
		{"warning", 2}, {"info", 2}, {"series_1", 1}, {"série-1", 1}, {"old", 1}, {"sz", 1}, // Tone: a wire differs, so both tables hold the strings that are equal
		{"FIRE", 1}, {"WATER", 1}, {"EARTH", 1}, {"up", 1}, {"down", 1}, {"flat", 1}, {"LOW", 1}, {"MID", 1}, {"HIGH", 1}, // the wires are the names: one table
		{"circle", 2}, {"dot", 1}, {"DOT", 1}, // a kind enum
		{"SPA", 1}, {"SPÉ", 1}, {"def", 1}, {"spd", 1}, // an opened enum's wires and names
	} {
		if n := literalCount(source, tc.literal); n != tc.count {
			t.Errorf("%q is in the enum tables %d times, want %d", tc.literal, n, tc.count)
		}
	}
	for _, s := range []string{"série-1", "sz", "series_1", "EARTH", "HIGH", "SPÉ"} { // literals nothing else in the file spells
		if n := strings.Count(source, `"`+s+`"`); n != 1 {
			t.Errorf("%q is in the .gen.cpp %d times, want once", s, n)
		}
	}
	if m := reLocalTables.FindString(source); m != "" {
		t.Errorf("a function-local table remains: %q", m)
	}
	tables := tableTexts(source)
	for _, want := range []string{"kToneNames", "kToneWires", "kShapeKindNames", "kShapeKindWires", "kStatWires", "kStatWireNames", "kOpenDupWireNames", "kBlankWires"} {
		if tables[want] == "" {
			t.Errorf("the .gen.cpp lacks the table %s", want)
		}
	}
	for _, none := range []string{"kElementWires", "kDeltaWires", "kExtremeWires", "kNothingWires", "kNoCodeWires", "kBlankWireNames", "kStatNames", "kToneWireNames"} {
		if tables[none] != "" {
			t.Errorf("the .gen.cpp holds %s, a table nothing needs", none)
		}
	}
	for _, order := range []string{
		"constexpr std::array<uint8_t, 4> kToneByWire = {1, 3, 2, 0};\n", "constexpr std::array<uint8_t, 3> kDupByWire = {2, 0, 1};\n",
		"constexpr std::array<uint8_t, 0> kNothingByWire = {};\n", "constexpr std::array<uint8_t, 2> kShapeKindByWire = {1, 0};\n",
	} {
		if !strings.Contains(source, order) {
			t.Errorf("the .gen.cpp lacks %q", order)
		}
	}
	if !strings.Contains(source, "kWireAt") || strings.Contains(source, "std::array<std::string_view, 4> kWires") {
		t.Error("an opened enum's FromCode finds its wires through kWireAt in the one table")
	}
}

// CODEGEN.md §5.3, DECISIONS 430: a table's id enum holds its keys once, in k<Id>Wires, with k<Id>ByWire, and no names.
func TestIDEnumStringsWrittenOnce(t *testing.T) {
	source := fileOf(t, generate(t, bakedPackage()), ".gen.cpp")
	for _, s := range []string{"open", "taken", "closed", "todo", "done"} {
		if n := literalCount(source, s); n != 1 {
			t.Errorf("%q is in the enum tables %d times, want once", s, n)
		}
	}
	if m := reLocalTables.FindString(source); m != "" {
		t.Errorf("a function-local table remains: %q", m)
	}
	tables := tableTexts(source)
	if tables["kColumnIdWires"] == "" || tables["kStatusIdWires"] == "" || tables["kColumnIdNames"] != "" {
		t.Errorf("an id enum has k<Id>Wires and no k<Id>Names: %v", tables)
	}
	for _, order := range []string{"std::array<uint8_t, 2> kColumnIdByWire = {1, 0};\n", "std::array<uint8_t, 3> kStatusIdByWire = {2, 0, 1};\n"} {
		if !strings.Contains(source, order) {
			t.Errorf("the .gen.cpp lacks %q", order)
		}
	}
}

// sameOpenPackage is an opened enum whose names are its wires.
func sameOpenPackage() *ir.Package {
	same := &ir.Enum{Pkg: "demo.same", Name: "Same", Members: members([]string{"a", "a", "b", "b"}, nil)}
	return &ir.Package{
		Name: "demo.same", Dir: "demo/same", Types: []ir.Type{same},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/same", Mode: ir.ModeTypes, Namespace: "demo::same", Open: []string{"Same"}}},
	}
}

// CODEGEN.md §5.2, DECISIONS 430: an opened enum whose names are its wires has no k<E>WireNames, and ToName is ToWire.
func TestOpenEnumNamesOnlyWhenTheyDiffer(t *testing.T) {
	source := fileOf(t, generate(t, sameOpenPackage()), ".gen.cpp")
	if strings.Contains(source, "kSameWireNames") {
		t.Error("an opened enum whose names are its wires holds kSameWireNames")
	}
	if want := "std::string_view ToName(const Same& v) {\n    return ToWire(v);\n}\n"; !strings.Contains(source, want) {
		t.Errorf("ToName is not ToWire:\n%s", source)
	}
}
