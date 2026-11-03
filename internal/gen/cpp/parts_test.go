package cppgen_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/fnsize"
	"github.com/fantasim/canonlang/internal/types"
)

const (
	partsGolden   = "decode-parts"
	partsTinyCut  = 2       // TestDecodeIsCut's body budget: every field of a 5-field record is a part
	partsHuge     = 1 << 20 // a budget nothing reaches: the whole function
	partsSource   = ".gen.cpp"
	partsDecode   = "Decode"
	partsBuild    = "Build"
	partsAccess   = "Access::"
	partsFields   = 25        // the width of the boundary fixture's record
	partsThird    = 3         // the divisor of the budget that cuts a function into several parts
	partsNoPart   = "Decode0" // the first part of a cut decoder
	partsBuildAt  = "Build0"  // the first part of a cut Build
	partsFileMode = 0o644
)

// generateAt is generate with a body budget of limit lines, run twice: the same bytes both times (CODEGEN.md §2.7, NFR-05).
func generateAt(t *testing.T, p *ir.Package, limit int) []ir.File {
	t.Helper()
	first, err := cppgen.GenerateWithPartLines(p, cppEmit(p), limit)
	if err != nil {
		t.Fatal(err)
	}
	again, err := cppgen.GenerateWithPartLines(p, cppEmit(p), limit)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range first {
		if !bytes.Equal(f.Content, again[i].Content) {
			t.Errorf("%s differs between two runs", f.Path)
		}
	}
	return first
}

// sourceOf is the .gen.cpp of files.
func sourceOf(t *testing.T, files []ir.File) string {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, partsSource) {
			return string(f.Content)
		}
	}
	t.Fatal("no .gen.cpp")
	return ""
}

// bodies are the function bodies of the .gen.cpp of files, as fnsize measures them.
func bodies(t *testing.T, files []ir.File) []fnsize.Func {
	t.Helper()
	funcs, err := fnsize.Cpp(sourceOf(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return funcs
}

// CODEGEN.md §2.11, §7.6, DECISIONS 435 (D-SIZE): a decoder over the budget is cut into Decode<N> parts of detail::<P>Access that Decode calls in order; the access struct, with the declarations first, is written before the cut decoders. The golden is pipeline cut at 2 lines (-update writes it).
func TestDecodeIsCut(t *testing.T) {
	files := generateAt(t, pipeline(), partsTinyCut)
	dir := filepath.Join("testdata", "golden", partsGolden)
	if *update {
		rewrite(t, dir, files)
		return
	}
	for _, f := range files {
		want, err := os.ReadFile(filepath.Join(dir, f.Path))
		if err != nil {
			t.Fatalf("%v (run with -update)", err)
		}
		if !bytes.Equal(f.Content, want) {
			t.Errorf("%s differs from its golden (run with -update and read the diff)", f.Path)
		}
	}
	src := sourceOf(t, files)
	decl := strings.Index(src, "static bool Decode0(")
	first := strings.Index(src, "bool Decode(const nlohmann::json& v, canon::json::Decoder& dec, Potion& out) {")
	if decl < 0 || first < 0 || decl > first || strings.Index(src, "struct PipelineAccess {") > decl {
		t.Errorf("the access struct does not declare Decode0 before the cut decoder (%d, %d)", decl, first)
	}
	if !strings.Contains(src, "if (!PipelineAccess::Decode0(v, dec, out)) return false;") {
		t.Error("Decode does not call its first part")
	}
}

// partsDoc is one JSON document of the equivalence test, by member.
type partsDoc map[string]any

// partsDocs are the documents of TestCutDecodeCompilesAndRuns, by file name: valid ones, and invalid ones that fail early, in a middle part and in the last, and one that fails twice (CODEGEN.md §2.11: the first failure is the same).
func partsDocs() map[string]string {
	base := func(mut func(d partsDoc)) string {
		d := partsDoc{"s0": "a", "s1": "b", "s2": "c", "s4": "e", "s5": "f", "s6": "g", "s7": "h", "l0": []int{1}, "l1": []int{}, "shape": partsDocShape("dot")}
		for i := range partsInts {
			if i != partsInts/2 {
				d["i"+strconv.Itoa(i)] = i * 3
			}
		}
		for i := range partsBools {
			d["b"+strconv.Itoa(i)] = i%2 == 0
		}
		for i := range partsFloats {
			d["f"+strconv.Itoa(i)] = float64(i) + 0.5
		}
		mut(d)
		b, _ := json.Marshal(d)
		return string(b)
	}
	none := func(partsDoc) {}
	return map[string]string{
		"good.json": base(none),
		"blob.json": base(func(d partsDoc) { d["shape"] = partsDocShape("blob") }),
		"set.json": base(func(d partsDoc) {
			d["i8"], d["s3"], d["o1"], d["o3"], d["at"] = 77, "set", 5, 6, map[string]int{"x": 7, "y": 8}
		}),
		"early.json":    base(func(d partsDoc) { d["i1"] = "x" }),
		"middle.json":   base(func(d partsDoc) { d["f2"] = "x" }),
		"last.json":     base(func(d partsDoc) { delete(d, "shape") }),
		"lastbad.json":  base(func(d partsDoc) { d["shape"] = map[string]any{"kind": "ring"} }),
		"blobbad.json":  base(func(d partsDoc) { d["shape"] = partsDocBad(partsBlob - 1) }),
		"blobhead.json": base(func(d partsDoc) { d["shape"] = partsDocBad(0) }),
		"two.json":      base(func(d partsDoc) { d["s5"] = 5; delete(d, "shape") }),
		"lists.json":    base(func(d partsDoc) { d["l1"] = []string{"x"} }),
		"point.json":    base(func(d partsDoc) { d["at"] = map[string]any{"x": "x"} }),
		"root.json":     `[]`,
	}
}

// partsDocShape is a shape of kind kind with every member 1.
func partsDocShape(kind string) map[string]any {
	s := map[string]any{"kind": kind, "r": 4}
	for i := range partsBlob {
		s["p"+strconv.Itoa(i)] = i + 1
	}
	return s
}

// partsDocBad is a Blob whose member p<at> is a string.
func partsDocBad(at int) map[string]any {
	s := partsDocShape("blob")
	s["p"+strconv.Itoa(at)] = "x"
	return s
}

// partsRun writes the demo.parts package generated at limit, its driver and the documents under dir, compiles with every compiler and returns the driver's output for each.
func partsRun(t *testing.T, dir string, limit int, docs map[string]string) []string {
	t.Helper()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	files := generateAt(t, partsPackage(), limit)
	writeFiles(t, out, files)
	copyFile(t, filepath.Join("testdata", "main", "parts_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	names := slices.Sorted(maps.Keys(docs))
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(docs[n]), partsFileMode); err != nil {
			t.Fatal(err)
		}
	}
	return buildAndRun(t, dir, []string{"main.cpp", "out/parts.gen.cpp"}, slices.Concat([]string{dir}, names)...)
}

// CODEGEN.md §2.11, §7.6, DECISIONS 435 (D-SIZE): a 40-member record with a built default and a variant case, cut at 60 lines, builds with every compiler and reads, fails and reports exactly as the whole decoder does, the first failure included.
func TestCutDecodeCompilesAndRuns(t *testing.T) {
	t.Parallel()
	docs := partsDocs()
	whole := partsRun(t, t.TempDir(), partsHuge, docs)
	cutDir := t.TempDir()
	cut := partsRun(t, cutDir, partsLimit, docs)
	if src := sourceOf(t, generateAt(t, partsPackage(), partsLimit)); !strings.Contains(src, "Decode1(") || !strings.Contains(src, "static bool Decode0(") {
		t.Fatal("the fixture is not cut")
	}
	if src := sourceOf(t, generateAt(t, partsPackage(), partsHuge)); strings.Contains(src, partsNoPart) {
		t.Fatal("the whole generation holds a part")
	}
	for i := range whole {
		if whole[i] != cut[i] {
			t.Errorf("run %d: the cut decoder's output differs\nwhole:\n%s\ncut:\n%s", i, whole[i], cut[i])
		}
		for _, want := range []string{"last.json: error Wide: $.shape: missing", "good.json: i0=0 i1=3", "early.json: error Wide: $.i1: expected an integer", "blobbad.json: error Wide: $.shape.p63"} {
			if !strings.Contains(cut[i], want) {
				t.Errorf("run %d: the output lacks %q:\n%s", i, want, cut[i])
			}
		}
	}
}

// partsWide is a decoder's width: the test's record of n Int members, in types mode.
func partsWide(n int) *ir.Package {
	rec := &ir.Record{Pkg: partsPkg, Name: "Row"}
	for i := range n {
		name := fmt.Sprintf("m%d", i)
		rec.Fields = append(rec.Fields, field(name, name, "", ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}))
	}
	return &ir.Package{
		Name: partsPkg, Dir: "demo/parts", Types: []ir.Type{rec},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/parts/out", Mode: ir.ModeTypes, Namespace: "demo::parts"}},
	}
}

// decoderBodies are the line counts of detail's Decode functions and of its Decode<N> parts.
func decoderBodies(funcs []fnsize.Func) (whole, parts []int) {
	for _, f := range funcs {
		switch {
		case f.Name == partsDecode || strings.HasSuffix(f.Name, partsAccess+partsDecode):
			whole = append(whole, f.Body)
		case strings.Contains(f.Name, partsAccess+partsDecode):
			parts = append(parts, f.Body)
		}
	}
	return whole, parts
}

// boundary is one construct's exact-boundary check: whole measures the uncut function in lines of body, split returns the line counts of the functions that make it up and of its parts, and part is the generated text a cut leaves.
type boundary struct {
	p     *ir.Package
	split func([]fnsize.Func) (whole, parts []int)
	part  string
}

// check generates p at its own length (not cut), a line short (cut, every function within it) and a third of it (cut into several parts, every function within it).
func (b boundary) check(t *testing.T) {
	t.Helper()
	whole, _ := b.split(bodies(t, generateAt(t, b.p, partsHuge)))
	if len(whole) != 1 {
		t.Fatalf("%d uncut functions", len(whole))
	}
	exact := whole[0]
	if src := sourceOf(t, generateAt(t, b.p, exact)); strings.Contains(src, b.part) {
		t.Errorf("a body of exactly %d lines is cut", exact)
	}
	for _, c := range []struct{ limit, parts int }{{exact - 1, 1}, {exact / partsThird, 2}} {
		files := generateAt(t, b.p, c.limit)
		if !strings.Contains(sourceOf(t, files), b.part) {
			t.Fatalf("a body of %d lines is not cut at a budget of %d", exact, c.limit)
		}
		made, parts := b.split(bodies(t, files))
		if len(parts) < c.parts {
			t.Errorf("budget %d: %d parts, want at least %d", c.limit, len(parts), c.parts)
		}
		for _, n := range slices.Concat(made, parts) {
			if n > c.limit {
				t.Errorf("budget %d: a function of %d lines", c.limit, n)
			}
		}
	}
}

// CODEGEN.md §2.11, DECISIONS 435: a Decode whose body is exactly the budget is not cut; one line more is, into parts that are each within the budget, measured on the generated text.
func TestDecodeBoundary(t *testing.T) {
	boundary{partsWide(partsFields), decoderBodies, partsNoPart}.check(t)
}

// buildBodies are the line counts of the Build function and of its Build<N> parts.
func buildBodies(funcs []fnsize.Func) (whole, parts []int) {
	for _, f := range funcs {
		_, name, ok := strings.Cut(f.Name, partsAccess)
		switch {
		case !ok:
		case name == partsBuild:
			whole = append(whole, f.Body)
		case strings.HasPrefix(name, partsBuild):
			parts = append(parts, f.Body)
		}
	}
	return whole, parts
}

// CODEGEN.md §2.11, §7.3, DECISIONS 435: a Build whose body is exactly the budget is not cut; one line more is, into parts of at most the budget (the old rule cut at least that), measured on the generated text.
func TestBuildBoundary(t *testing.T) {
	boundary{bulkSmallPackage(), buildBodies, partsBuildAt}.check(t)
}

// inputsCase is the one driver case of TestAllInputDecoderIsNotCut: the variable NOTE set to "x" (hex 78).
const inputsCase = "CANON_TEST_NOTE 78\n"

// inputsRun writes inputsPackage generated at limit, its driver and one case, builds with every compiler and returns the driver's output for each.
func inputsRun(t *testing.T, limit int) []string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, generateAt(t, inputsPackage(), limit))
	copyFile(t, filepath.Join("testdata", "main", "inputs_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	file := filepath.Join(dir, "cases.txt")
	if err := os.WriteFile(file, []byte(inputsCase), partsFileMode); err != nil {
		t.Fatal(err)
	}
	return buildAndRun(t, dir, []string{"main.cpp", "demo.gen.cpp"}, file)
}

// CODEGEN.md §2.11, §7.6: a decoder whose fields are all inputs writes no statement, so there is nothing to cut at any budget and no part is left with its parameters unused: Config generated at a budget of 1 has no part, builds under -Werror and runs as the whole does.
func TestAllInputDecoderIsNotCut(t *testing.T) {
	t.Parallel()
	if src := sourceOf(t, generateAt(t, inputsPackage(), 1)); strings.Contains(src, partsNoPart) {
		t.Error("a decoder of input fields only is cut")
	}
	whole, cut := inputsRun(t, partsHuge), inputsRun(t, 1)
	for i := range whole {
		if whole[i] != cut[i] {
			t.Errorf("run %d: the output differs\nwhole:\n%s\ncut:\n%s", i, whole[i], cut[i])
		}
	}
}

// partsOrder are the markers of the demo.parts source in the order CODEGEN.md §2.7 gives: the decoders that are not cut, the access struct, then the cut decoders, each a part definition and its Decode, in declaration order.
var partsOrder = []string{
	"bool Decode(const nlohmann::json& v, canon::json::Decoder& dec, Point& out) {",
	"bool Decode(const nlohmann::json& v, canon::json::Decoder& dec, ShapeDot& out) {",
	"struct PartsAccess {",
	"bool PartsAccess::Decode0(const nlohmann::json& v, canon::json::Decoder& dec, ShapeBlob& out) {",
	"bool Decode(const nlohmann::json& v, canon::json::Decoder& dec, ShapeBlob& out) {",
	"bool PartsAccess::Decode0(const nlohmann::json& v, canon::json::Decoder& dec, Wide& out) {",
	"bool Decode(const nlohmann::json& v, canon::json::Decoder& dec, Wide& out) {",
}

// CODEGEN.md §2.7, §2.11: in a file with cut decoders the uncut decoders, the text helpers and the readers keep their place before <P>Access, and only the cut decoders follow it, in declaration order.
func TestCutDecodersFollowTheAccessStruct(t *testing.T) {
	src := sourceOf(t, generateAt(t, partsPackage(), partsLimit))
	at := -1
	for _, marker := range partsOrder {
		i := strings.Index(src, marker)
		if i < 0 || i <= at {
			t.Fatalf("%q is at %d, after %d: the decoders are out of order", marker, i, at)
		}
		at = i
	}
	if !strings.Contains(src, "static bool Decode0(const nlohmann::json& v, canon::json::Decoder& dec, Wide& out);") {
		t.Error("the access struct does not open with the part declarations")
	}
}
