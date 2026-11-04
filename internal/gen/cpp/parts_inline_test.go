package cppgen_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const (
	inlineWidth    = 300 // inline Reward variants in the generated-size record, one decoder line each
	inlineLimit    = 100 // the body budget the size test cuts at
	inlineRewards  = 8   // the Reward fields of the run test's record (r0..), then its Cost fields
	inlineCosts    = 4   // c0..
	inlineMinParts = 3   // Reward fields, Reward fields, Cost fields
	inlineSmall    = 5   // the body budget the run test cuts at: four units to a part, so Reward fields r0..r3, r4..r7, then Cost fields c0..c3
)

// inlineVariant is a variant read inline, at the key tag, with a case holding one Int member and a case holding none.
func inlineVariant(name, tag, full, field, empty string) *ir.Variant {
	return &ir.Variant{Pkg: partsPkg, Name: name, Tag: tag, Cases: []*ir.Case{
		{Name: full, Wire: full, Fields: []*ir.Field{field2(field)}}, {Name: empty, Wire: empty},
	}}
}

// field2 is an Int member of a case.
func field2(name string) *ir.Field { return field(name, name, "", tInt) }

// inlineOnly is a types package of one record Row whose only fields are rewards @json(inline) variants of Reward (tag "kind") then costs of Cost (tag "cost").
func inlineOnly(rewards, costs int) *ir.Package {
	reward, cost := inlineVariant("Reward", "kind", "gem", "n", "none"), inlineVariant("Cost", "cost", "coin", "amount", "free")
	rec := &ir.Record{Pkg: partsPkg, Name: "Row"}
	add := func(prefix string, n int, v *ir.Variant) {
		for i := range n {
			f := field(fmt.Sprintf("%s%d", prefix, i), "", "", ir.TypeRef{Kind: types.Variant, Named: v})
			f.WirePath, f.Inline = nil, true
			rec.Fields = append(rec.Fields, f)
		}
	}
	add("r", rewards, reward)
	add("c", costs, cost)
	return &ir.Package{
		Name: partsPkg, Dir: "demo/parts", Types: []ir.Type{reward, cost, rec},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/parts/out", Mode: ir.ModeTypes, Namespace: "demo::parts"}},
	}
}

// CODEGEN.md §2.11, §7.6: a C++ Decode whose only fields are @json(inline) variants is cut into Decode<N> parts like any other (a part takes the same document as Decode, so no object is bound or passed); none passes the budget.
func TestInlineOnlyDecoderIsCut(t *testing.T) {
	t.Parallel()
	files := generateAt(t, inlineOnly(inlineWidth, inlineWidth), inlineLimit)
	if src := sourceOf(t, files); !strings.Contains(src, partsNoPart) {
		t.Fatal("a decoder of inline variants only is not cut")
	}
	for _, f := range bodies(t, files) {
		if f.Body > inlineLimit && f.Statements > 1 {
			t.Errorf("%s has %d lines, over the budget of %d", f.Name, f.Body, inlineLimit)
		}
	}
}

// inlineDocs are the documents of the run test, by file name: the Reward fields come first, so a part holding a Cost field comes after the parts holding Reward ones.
func inlineDocs() map[string]string {
	const (
		goodReward = `"kind":"gem","n":4`
		badReward  = `"kind":"gem","n":"x"`
		goodCost   = `"cost":"coin","amount":9`
		badCost    = `"cost":"coin","amount":"x"`
	)
	doc := func(parts ...string) string { return "{" + strings.Join(parts, ",") + "}" }
	return map[string]string{
		"good.json":        doc(goodReward, goodCost),
		"free.json":        doc(`"kind":"none"`, `"cost":"free"`),
		"badreward.json":   doc(badReward, goodCost),
		"badcost.json":     doc(goodReward, badCost), // fails in a later part
		"both.json":        doc(badReward, badCost),  // the first part wins
		"nocost.json":      doc(goodReward),
		"unknowncost.json": doc(goodReward, `"cost":"gold"`),
		"nokind.json":      doc(goodCost),
		"root.json":        `[]`,
	}
}

// inlineRun writes inlineOnly generated at limit, its driver and the documents under dir, compiles with every compiler and returns the driver's output for each.
func inlineRun(t *testing.T, dir string, limit int, docs map[string]string) []string {
	t.Helper()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, out, generateAt(t, inlineOnly(inlineRewards, inlineCosts), limit))
	copyFile(t, filepath.Join("testdata", "main", "inline_cut_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	names := slices.Sorted(maps.Keys(docs))
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(docs[n]), partsFileMode); err != nil {
			t.Fatal(err)
		}
	}
	return buildAndRun(t, dir, []string{"main.cpp", "out/parts.gen.cpp"}, slices.Concat([]string{dir}, names)...)
}

// CODEGEN.md §2.11, §7.6, DECISIONS 435: a Decode<N> part holding an @json(inline) variant's field compiles with every compiler and the cut decoder reads and fails as the whole one does, the first failure included: a bad Reward and a bad Cost together fail at the Reward, the part before.
func TestInlineOnlyCutCompilesAndRuns(t *testing.T) {
	t.Parallel()
	docs := inlineDocs()
	if src := sourceOf(t, generateAt(t, inlineOnly(inlineRewards, inlineCosts), partsHuge)); strings.Contains(src, partsNoPart) {
		t.Fatal("the whole generation holds a part")
	}
	checkInlineAlignment(t, sourceOf(t, generateAt(t, inlineOnly(inlineRewards, inlineCosts), inlineSmall)))
	whole := inlineRun(t, t.TempDir(), partsHuge, docs)
	cut := inlineRun(t, t.TempDir(), inlineSmall, docs)
	for i := range whole {
		if whole[i] != cut[i] {
			t.Errorf("run %d: the cut decoder's output differs\nwhole:\n%s\ncut:\n%s", i, whole[i], cut[i])
		}
		for _, want := range []string{"good.json: r0=gem4 r1=gem4 r2=gem4 r3=gem4 r4=gem4 r5=gem4 r6=gem4 r7=gem4 c0=coin9 c1=coin9 c2=coin9 c3=coin9", "free.json: r0=none r1=none r2=none r3=none r4=none r5=none r6=none r7=none c0=free c1=free c2=free c3=free", "badreward.json: error", "badcost.json: error", "both.json: error"} {
			if !strings.Contains(cut[i], want) {
				t.Errorf("run %d: the output lacks %q:\n%s", i, want, cut[i])
			}
		}
		checkFirstFailure(t, cut[i])
	}
}

// checkFirstFailure checks the answers of badreward.json, badcost.json and both.json in out: the last two are the errors of the first part's field and of a later part's, and both together answer as the first.
func checkFirstFailure(t *testing.T, out string) {
	t.Helper()
	answer := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		name, text, _ := strings.Cut(line, ": ")
		answer[name] = text
	}
	if answer["both.json"] != answer["badreward.json"] || answer["badcost.json"] == answer["badreward.json"] {
		t.Errorf("the first failure is not the first part's:\nbadreward %s\nbadcost %s\nboth %s", answer["badreward.json"], answer["badcost.json"], answer["both.json"])
	}
}

// partBody is the text of detail::Decode<n> of PartsAccess in src, up to its closing brace; false when there is none.
func partBody(src string, n int) (string, bool) {
	_, rest, ok := strings.Cut(src, fmt.Sprintf("bool PartsAccess::Decode%d(", n))
	if !ok {
		return "", false
	}
	body, _, _ := strings.Cut(rest, "\n}\n")
	return body, true
}

// checkInlineAlignment checks, whatever a field's line count, that the cut falls where the test needs it: Decode0 holds no Cost field and the last part none of Reward, so a Cost failure is a later part's, and a part called out of order shows.
func checkInlineAlignment(t *testing.T, src string) {
	t.Helper()
	var parts []string
	for n := 0; ; n++ {
		body, ok := partBody(src, n)
		if !ok {
			break
		}
		parts = append(parts, body)
	}
	if len(parts) < inlineMinParts {
		t.Fatalf("the decoder has %d parts, want at least %d", len(parts), inlineMinParts)
	}
	if first := parts[0]; strings.Contains(first, "out.c") || !strings.Contains(first, "out.r0_") {
		t.Errorf("Decode0 is not Reward fields only:\n%s", first)
	}
	if last := parts[len(parts)-1]; strings.Contains(last, "out.r") || !strings.Contains(last, "out.c3_") {
		t.Errorf("the last part is not Cost fields only:\n%s", last)
	}
}
