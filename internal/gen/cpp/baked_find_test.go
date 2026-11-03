package cppgen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// bakedFindWant is what testdata/main/baked_find_main.cpp prints for the baked-find fixture: every
// row of every container found at its address (hits), every key it lacks not found (misses).
const bakedFindWant = "names=8 numbers=9 tones=3 ranks=3 kinds=4 byKind=3 links=4 empties=0\n" +
	"hits=50 misses=62 failures=0\n"

// CODEGEN.md §5.9, §7.3, DECISIONS 432: a baked container finds every key at the address At(i) gives and nothing for a key it lacks, whatever the key's kind (a String by bytes, an Int by value, an enum by its underlying value, a ref held as a table's id by position, a ref into a keyed list by its key) and the order of its rows; FindBy over UInt16 and String and a keyed list of no row too.
func TestBakedFindTables(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, finderPackage()), "baked_find_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"finder.gen.cpp", "main.cpp"}) {
		if out != bakedFindWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedFindWant)
		}
	}
}

// bakedKeyShapes are what a baked container holds no more (CODEGEN.md §5.9): a key copied into the heap, a run-time sorted index, a cast that writes through a const row, a key list.
var bakedKeyShapes = []string{"FromRows(", "SortedIndex(", "FindSorted(", "const_cast<", "canon::KeyedList<"}

// generatedTexts are the header and the source of the package's C++ emit.
func generatedTexts(t *testing.T, p *ir.Package) (header, source string) {
	t.Helper()
	for _, f := range generate(t, p) {
		switch {
		case strings.HasSuffix(f.Path, ".gen.h"):
			header = string(f.Content)
		case strings.HasSuffix(f.Path, ".gen.cpp") && !strings.HasSuffix(f.Path, "_conformance.gen.cpp"):
			source = string(f.Content)
		}
	}
	return header, source
}

// CODEGEN.md §5.9, §7.3, DECISIONS 432: neither generated file of a baked container copies a key, sorts an index or casts a row's const away; the container holds a std::vector of rows.
func TestBakedContainersHoldNoKeys(t *testing.T) {
	t.Parallel()
	for _, fx := range []fixture{{"baked-bulk", bulkSmallPackage}, {"baked-find", finderPackage}} {
		header, source := generatedTexts(t, fx.build())
		for _, shape := range bakedKeyShapes {
			if strings.Contains(header, shape) || strings.Contains(source, shape) {
				t.Errorf("%s: a generated file holds %q", fx.name, shape)
			}
		}
		if !strings.Contains(header, "std::vector<Item> rows_;") && fx.name == "baked-bulk" {
			t.Errorf("%s: the Items container does not declare std::vector<Item> rows_;", fx.name)
		}
	}
}

// bakedFindShapes are, for the baked-find fixture, what a header declares and what the source defines (CODEGEN.md §5.9, DECISIONS 432): Find and FindBy<F> declared with the key's name, defined in the source with the parameter named key; a table's Find through its id enum's FromWire; the sorted keys beside the rows' positions, a @codes enum's by code, another enum's by position and not by wire, a ref held as an id by entry position.
var bakedFindShapes = []struct {
	rule, file, text string
}{
	{"header declares Find", "header", "    const Entry* Find(std::string_view name) const;\n"},
	{"header declares FindBy", "header", "    const Entry* FindByCode(uint16_t code) const;\n"},
	{"header declares a table's Find", "header", "    const Kind* Find(std::string_view id) const;\n"},
	{"header holds the rows", "header", "    std::vector<Entry> rows_;\n"},
	{"source defines Find", "source", "const Entry* Names::Find(std::string_view key) const {\n"},
	{"source defines FindBy", "source", "const Entry* Names::FindByAlias(std::string_view key) const {\n"},
	{"table's Find is its id enum's FromWire", "source", "const std::optional<KindId> id = KindIdFromWire(key);"},
	{"a String key sorted by bytes", "source", "constexpr std::array<std::string_view, 8> kNamesFindKeys = {\"Z\", \"_\", \"a\", std::string_view(\"a\\000b\", 3), \"ab\", \"pre\", \"prefix\", \"é\"};"},
	{"an Int key sorted by value", "source", "constexpr std::array<int64_t, 9> kNumbersFindKeys = {\n    canon::kIntMin,\n    -4611686018427387904,\n    -6,\n    -5,\n    0,\n    6,\n    7,\n    4611686018427387904,\n    canon::kIntMax,\n};\n"},
	{"an Int key sorted by value: rows", "source", "constexpr std::array<uint32_t, 9> kNumbersFindRows = {4, 5, 8, 1, 2, 7, 0, 3, 6};"},
	{"an enum key by position", "source", "constexpr std::array<Tone, 3> kTonesFindKeys = {Tone::first, Tone::second, Tone::third};"},
	{"an enum key by position: rows", "source", "constexpr std::array<uint32_t, 3> kTonesFindRows = {1, 2, 0};"},
	{"a @codes enum key by code", "source", "constexpr std::array<Rank, 3> kRanksFindKeys = {Rank::low, Rank::mid, Rank::high};"},
	{"a ref held as an id by position", "source", "constexpr std::array<KindId, 3> kByKindFindKeys = {KindId::zeta, KindId::mid, KindId::beta};"},
	{"no row", "source", "constexpr std::array<std::string_view, 0> kEmptiesFindKeys = {};"},
}

// CODEGEN.md §5.9, §7.3, DECISIONS 432: Find and FindBy<F> of a baked container are declared in the header and defined in the source over sorted constant arrays of the .gen.cpp's detail namespace, in C++'s order of each key kind.
func TestBakedFindDefinedInSource(t *testing.T) {
	t.Parallel()
	header, source := generatedTexts(t, finderPackage())
	for _, c := range bakedFindShapes {
		text := header
		if c.file == "source" {
			text = source
		}
		if !strings.Contains(text, c.text) {
			t.Errorf("%s: the %s lacks\n%s", c.rule, c.file, c.text)
		}
	}
}
