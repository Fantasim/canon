package cppgen_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// denseHeaderLines are the lines the header of demo.dense holds for each dense enum (CODEGEN.md §5.2, DECISIONS 430): the ordinal subtracts the first code in uint64_t and clamps to the member count, the first code an integer literal, canon::kIntMin for the minimum.
var denseHeaderLines = []string{
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(-128);\n    return i < 3 ? static_cast<size_t>(i) : 3;\n",
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(6000);\n    return i < 3 ? static_cast<size_t>(i) : 3;\n",
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(9223372036854775806);\n    return i < 2 ? static_cast<size_t>(i) : 2;\n",
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(canon::kIntMin);\n    return i < 2 ? static_cast<size_t>(i) : 2;\n",
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(0);\n    return i < 4 ? static_cast<size_t>(i) : 4;\n",
	"    const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(1);\n    return i < 1 ? static_cast<size_t>(i) : 1;\n",
}

// CODEGEN.md §5.2, DECISIONS 430: an @codes enum whose codes are dense in declaration order has an ordinal that subtracts the first code, and no k<E>Codes or k<E>CodeOrdinals; a wrapping pair, a pair with a gap and an enum without member keep the search, k<E>CodeOrdinals in the smallest unsigned type.
func TestDenseOrdinalHeader(t *testing.T) {
	header := fileOf(t, generate(t, densePackage()), ".gen.h")
	for _, want := range denseHeaderLines {
		if strings.Count(header, want) != 1 {
			t.Errorf("the header lacks one dense ordinal %q", want)
		}
	}
	for _, dense := range []string{"Level", "Era", "Wide", "Floor", "Start", "Single", "Spelled"} {
		for _, array := range []string{"k" + dense + "Codes", "k" + dense + "CodeOrdinals"} {
			if strings.Contains(header, array) {
				t.Errorf("dense %s still has %s", dense, array)
			}
		}
	}
	for _, kept := range []string{
		"inline constexpr std::array<int64_t, 2> kGapCodes = {canon::kIntMin, canon::kIntMax};\n",
		"inline constexpr std::array<uint8_t, 2> kGapCodeOrdinals = {1, 0};\n",
		"inline constexpr std::array<uint8_t, 2> kSkipCodeOrdinals = {0, 1};\n",
		"inline constexpr std::array<uint8_t, 0> kVoidCodes = {};\n",
		"inline constexpr std::array<uint8_t, 0> kVoidCodeOrdinals = {};\n",
	} {
		if !strings.Contains(header, kept) {
			t.Errorf("the header lacks %q", kept)
		}
	}
}

// CODEGEN.md §5.2, DECISIONS 430: k<E>CodeOrdinals and k<E>ByWire take the smallest of uint8_t, uint16_t and uint32_t that holds every ordinal.
func TestEnumIndexTablesSmallestType(t *testing.T) {
	if testing.Short() {
		t.Skip("generates enums of 65,537 members")
	}
	for _, tc := range []struct {
		members int
		typ     string
	}{{256, "uint8_t"}, {257, "uint16_t"}, {65536, "uint16_t"}, {65537, "uint32_t"}} {
		files := generate(t, bigCodedPackage(tc.members))
		for _, want := range []struct{ suffix, array string }{{".gen.h", "kItemCodeOrdinals"}, {".gen.cpp", "kItemByWire"}} {
			if !strings.Contains(fileOf(t, files, want.suffix), "std::array<"+tc.typ+", "+strconv.Itoa(tc.members)+"> "+want.array+" = ") {
				t.Errorf("%d members: %s is not a %s array", tc.members, want.array, tc.typ)
			}
		}
	}
}

// denseWant is what dense_main.cpp prints (DECISIONS 430): every answer of today's search, kept by the subtraction.
const denseWant = `Level: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 m2/m2/2/code/wire/0 | -125=3/[|]/none/0 0=3/[|]/none/0 127=3/[|]/none/0
Era: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 m2/m2/2/code/wire/0 | 5999=3/[|]/none/0 6003=3/[|]/none/0 0=3/[|]/none/0 65535=3/[|]/none/0
Wide: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 | -9223372036854775808=2/[|]/none/0 0=2/[|]/none/0 9223372036854775805=2/[|]/none/0
Floor: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 | -9223372036854775806=2/[|]/none/0 0=2/[|]/none/0 9223372036854775807=2/[|]/none/0
Start: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 m2/m2/2/code/wire/0 m3/m3/3/code/wire/0 | 4=4/[|]/none/0 255=4/[|]/none/0
Single: m0/m0/0/code/wire/0 | 0=1/[|]/none/0 2=1/[|]/none/0 255=1/[|]/none/0
Spelled: a/x/0/code/wire/0 b/y/1/code/wire/0 | 0=2/[|]/none/0 3=2/[|]/none/0
Gap: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 | 0=2/[|]/none/0 -9223372036854775807=2/[|]/none/0
Skip: m0/m0/0/code/wire/0 m1/m1/1/code/wire/0 | 0=2/[|]/none/0 2=2/[|]/none/0 4=2/[|]/none/0
Void: | 0=0/[|]/none/0 255=0/[|]/none/0
lookups: 1 9 -7 8
`

// CODEGEN.md §5.2, DECISIONS 430: the dense ordinal equals the declaration position of a member and the member count for any other value, below the first code, past the last, at the type's minimum and maximum, constexpr and from a second package, with every compiler and mode; every helper answers as it did.
func TestDenseOrdinalCompileAndRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dense := densePackage()
	for _, p := range []*ir.Package{dense, denseUsePackage(dense)} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "dense_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	sources := []string{"demo/dense/out/dense.gen.cpp", "demo/denseuse/out/denseuse.gen.cpp", "main.cpp"}
	for _, got := range buildAndRun(t, dir, sources) {
		if got != denseWant {
			t.Errorf("got:\n%s\nwant:\n%s", got, denseWant)
		}
	}
}
