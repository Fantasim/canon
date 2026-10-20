package cppgen_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// fileOf is the generated file ending in suffix.
func fileOf(t *testing.T, files []ir.File, suffix string) string {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return string(f.Content)
		}
	}
	t.Fatalf("no generated file ends in %s", suffix)
	return ""
}

// DECISIONS 383, CODEGEN.md §5.2, §7.1: the header declares each enum helper, with no per-member body; the .gen.cpp defines it once.
func TestEnumHelpersOnlyDeclaredInHeader(t *testing.T) {
	files := generate(t, helpersPackage())
	header, source := fileOf(t, files, ".gen.h"), fileOf(t, files, ".gen.cpp")
	noMemberBodies(t, header)
	helpers := []string{
		"std::string_view ToName(Tone v)", "std::string_view ToWire(Tone v)", "std::optional<Tone> ToneFromWire(std::string_view wire)",
		"std::optional<Element> ElementFromCode(uint8_t code)", "std::optional<Delta> DeltaFromCode(int16_t code)", "std::optional<Extreme> ExtremeFromCode(int64_t code)",
		"std::optional<NoCode> NoCodeFromCode(uint8_t code)", "std::optional<ShapeKind> ShapeKindFromWire(std::string_view wire)",
		"std::string_view ToName(const Stat& v)", "bool IsKnown(const Stat& v)", "std::optional<Stat> StatFromWire(std::string_view wire)",
		"std::optional<uint16_t> ToCode(const Stat& v)", "std::optional<Stat> StatFromCode(uint16_t code)", "bool IsKnown(const Blank& v)",
	}
	for _, sig := range helpers {
		if strings.Count(header, sig+";\n") != 1 || strings.Count(source, sig+" {\n") != 1 {
			t.Errorf("%s: want one declaration in the header and one definition in the .gen.cpp", sig)
		}
	}
	for _, ordinal := range []string{"ElementOrdinal", "DeltaOrdinal", "ExtremeOrdinal", "NoCodeOrdinal"} {
		if !strings.Contains(header, "inline constexpr size_t "+ordinal+"(") {
			t.Errorf("the header lacks detail::%s", ordinal)
		}
	}
	if strings.Contains(header, "StatOrdinal") || strings.Contains(header, "ToneOrdinal") {
		t.Error("only a closed @codes enum gets detail::<E>Ordinal")
	}
}

// noMemberBodies fails when a header holds a per-member body: a chain over wires or values, or a switch.
func noMemberBodies(t *testing.T, header string) {
	t.Helper()
	for _, chain := range []string{"if (wire ==", "if (v ==", "case ", "switch ("} {
		if strings.Contains(header, chain) {
			t.Errorf("the header holds a per-member body %q:\n%s", chain, header)
		}
	}
}

// DECISIONS 383, CODEGEN.md §5.3, §7.1: a baked emit's header declares its id enums' ToWire and <Rec>IdFromWire, with no per-key body; the .gen.cpp defines them.
func TestIDEnumHelpersOnlyDeclaredInHeader(t *testing.T) {
	files := generate(t, bakedPackage())
	header, source := fileOf(t, files, ".gen.h"), fileOf(t, files, ".gen.cpp")
	noMemberBodies(t, header)
	for _, sig := range []string{"std::string_view ToWire(ColumnId v)", "std::optional<ColumnId> ColumnIdFromWire(std::string_view key)"} {
		if strings.Count(header, sig+";\n") != 1 || strings.Count(source, sig+" {\n") != 1 {
			t.Errorf("%s: want one declaration in the header and one definition in the .gen.cpp", sig)
		}
	}
}

// helpersWant is what helpers_main.cpp prints (DECISIONS 383): the first declared of equal wires wins, a value that is no member gives "" or none.
const helpersWant = `Tone: warning/warning/self info/info/self series_1/série-1/self old/sz/self | none=[|] unknown=none
Element: FIRE/FIRE/self WATER/WATER/self EARTH/EARTH/self | none=[|] unknown=none
Delta: up/up/self down/down/self flat/flat/self | none=[|] unknown=none
Extreme: LOW/LOW/self MID/MID/self HIGH/HIGH/self | none=[|] unknown=none
Dup: a/x/self b/x/other c/w/self | none=[|] unknown=none
Nothing: | none=[|] unknown=none
NoCode: | none=[|] unknown=none
ShapeKind: circle/circle/self dot/DOT/self | none=[|] unknown=none
Element: 3/self/0 1/self/1 200/self/2 | unknown=none ordinal=3
Delta: 7/self/0 -5/self/1 0/self/2 | unknown=none ordinal=3
Extreme: -9223372036854775808/self/0 0/self/1 9223372036854775807/self/2 | unknown=none ordinal=3
NoCode: | unknown=none ordinal=0
Stat: atk/ATK/known/self def/SPA/known/self spd/SPÉ/known/self atk/ATK/known/self | ZZZ/unknown/none
Blank: | ZZZ/unknown/none
Stat codes: 4/self 1/self 2/self 4/self | 9=atk 77=none ZZZ=none
`

// DECISIONS 383, CODEGEN.md §5.2: every enum helper answers both ways, on non-members and empty enums, with every compiler and mode.
func TestEnumHelpersCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "out", "helpers")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, generate(t, helpersPackage()))
	copyFile(t, filepath.Join("testdata", "main", "helpers_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, got := range buildAndRun(t, dir, []string{"main.cpp", "out/helpers/helpers.gen.cpp"}) {
		if got != helpersWant {
			t.Errorf("got:\n%s\nwant:\n%s", got, helpersWant)
		}
	}
}

// sizeMembers is the member count of the size test's enum, past Resource's largest (11,351).
const sizeMembers = 12000

// DECISIONS 383, CODEGEN.md §5.2: a 12,000-member @codes enum's header is linear in its members, its .gen.cpp compiles at -O2 within cxx.Timeout.
func TestEnumHelpersSize(t *testing.T) {
	if testing.Short() {
		t.Skip("generates and compiles a 12,000-member enum")
	}
	lines := func(n int) int {
		return strings.Count(fileOf(t, generate(t, bigCodedPackage(n)), ".gen.h"), "\n")
	}
	quarter, half, full := lines(sizeMembers/4), lines(sizeMembers/2), lines(sizeMembers)
	if full-half != 2*(half-quarter) {
		t.Errorf("header lines %d, %d, %d for %d, %d, %d members: not linear", quarter, half, full, sizeMembers/4, sizeMembers/2, sizeMembers)
	}
	dir := t.TempDir()
	files := generate(t, bigCodedPackage(sizeMembers))
	writeFiles(t, dir, files)
	if header := fileOf(t, files, ".gen.h"); strings.Contains(header, "case ") {
		t.Fatal("the header switches over the members")
	}
	cc := cxx.Compilers(t)[0]
	flags := slices.DeleteFunc(slices.Clone(cxx.Flags), func(f string) bool { return strings.HasPrefix(f, "-O") })
	start := time.Now()
	if out, err := compile(cc, append(flags, "-O2", "-c", "-o", filepath.Join(dir, "big.o"), "big.gen.cpp"), dir); err != nil {
		t.Fatalf("%s -O2 big.gen.cpp: %v\n%s", filepath.Base(cc), err, out)
	}
	t.Logf("%s -O2 of a %d-member @codes enum's .gen.cpp: %s", filepath.Base(cc), sizeMembers, time.Since(start).Round(time.Millisecond))
}
