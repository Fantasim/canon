package gogen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
)

const (
	inlinePkgName   = "parts.inl"
	rewardField     = "r%02d"
	costField       = "c%02d"
	inlineFields    = 38 // the Reward fields of the run test's record, then its Cost fields: at inlineLimit, two parts of Reward fields then one of Cost ones
	inlineCosts     = 10
	inlineHuge      = 300
	inlineLimit     = 100
	inlineDecodeFn  = "decodeInl"
	inlineFile      = "inl/inl.gen.go"
	inlineSmokeFile = "inl/smoke_test.go"
	inlinePartsMin  = 2
)

// The documents the cut and the whole decoder of inline variants are both asked: the Reward fields come first, so a Cost field is in a later part than r00, and a Reward failure and a Cost failure land in different parts.
const (
	goodReward = `"kind":"gem","n":3`
	badReward  = `"kind":"gem","n":"x"`
	goodCost   = `"cost":"coin","amount":9`
	badCost    = `"cost":"coin","amount":"x"`
	// the answers of these three are compared: the first failure is the first part's
	docBadReward = `{` + badReward + `,` + goodCost + `}`
	docBadCost   = `{` + goodReward + `,` + badCost + `}`
	docBothBad   = `{` + badReward + `,` + badCost + `}`
)

var inlineDocs = []string{
	`{` + goodReward + `,` + goodCost + `}`, `{"kind":"none","cost":"free"}`, docBadReward, docBadCost, docBothBad,
	`{` + goodReward + `}`, `{` + goodReward + `,"cost":"gold"}`, `{"kind":"gem","cost":"free"}`, `{"kind":"other"}`, `{}`, `[]`,
}

// inlinePackage is a types package of one record Inl whose only fields are rewards @json(inline) variants of Reward (tag kind), then costs of Cost (tag cost): its decoder binds no object.
func inlinePackage(rewards, costs int) *pkgSpec {
	inl := recordSpec{Name: "Inl"}
	for i := range rewards {
		inl.Fields = append(inl.Fields, fieldSpec{Name: fmt.Sprintf(rewardField, i), Type: "Reward", Inline: true})
	}
	for i := range costs {
		inl.Fields = append(inl.Fields, fieldSpec{Name: fmt.Sprintf(costField, i), Type: "Cost", Inline: true})
	}
	return &pkgSpec{
		Name: inlinePkgName, Dir: "parts/inl",
		Emit:    emitSpec{Out: "out/", Dir: "parts/inl", Import: partsModule + "/inl", Package: "inl", Mode: "types"},
		Records: []recordSpec{inl},
		Variants: []variantSpec{
			{Name: "Reward", Cases: []recordSpec{{Name: "gem", Fields: []fieldSpec{{Name: "n", Type: "Int"}}}, {Name: "none"}}},
			{Name: "Cost", Tag: "cost", Cases: []recordSpec{{Name: "coin", Fields: []fieldSpec{{Name: "amount", Type: "Int"}}}, {Name: "free"}}},
		},
	}
}

// inlineSmoke is a test of package inl that decodes inlineDocs and writes what the decoder answered (value or error text) to the file $PARTS_OUT.
func inlineSmoke(rewards, costs int) []byte {
	var getters, docs []string
	for i := range rewards {
		getters = append(getters, fmt.Sprintf("fmt.Sprint(v.R%02d().Kind())", i))
	}
	for i := range costs {
		getters = append(getters, fmt.Sprintf("fmt.Sprint(v.C%02d().Kind())", i))
	}
	for _, d := range inlineDocs {
		docs = append(docs, "`"+d+"`")
	}
	return fmt.Appendf(nil, `package inl_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	inl "example.com/parts/inl"
)

func TestDump(t *testing.T) {
	var b strings.Builder
	for _, doc := range []string{%s} {
		v, err := inl.DecodeInl([]byte(doc))
		if err != nil {
			fmt.Fprintf(&b, "%%s: %%v\n", doc, err)
			continue
		}
		fmt.Fprintf(&b, "%%s: %%v\n", doc, []string{%s})
	}
	if err := os.WriteFile(os.Getenv("PARTS_OUT"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
`, strings.Join(docs, ", "), strings.Join(getters, ", "))
}

// inlineFilesAt generates the inline package of rewards Reward and costs Cost fields with gen.
func inlineFilesAt(t *testing.T, gen generator, rewards, costs int) map[string][]byte {
	t.Helper()
	return generateWith(t, gen, loadFixture(t, &fixture{Module: partsModule, Packages: []*pkgSpec{inlinePackage(rewards, costs)}}))
}

// CODEGEN.md §2.11, DECISIONS 435: a types-mode decoder whose only read fields are @json(inline) variants, which binds no object, is cut like any other: a record of 600 of them has no function over 1,000 lines (measured by fnsize), and its decoder has parts.
func TestInlineOnlyDecoderIsCut(t *testing.T) {
	funcs := partFuncs(t, inlineFilesAt(t, gogen.Generate, inlineHuge, inlineHuge)[inlineFile])
	for name, fn := range funcs { //canon:unordered each function is checked alone
		if !fits(fn, ir.PartLines) {
			t.Errorf("%s has %d lines, over the budget of %d", name, fn.Body, ir.PartLines)
		}
	}
	if n := len(partsOf(funcs, inlineDecodeFn)); n < inlinePartsMin {
		t.Errorf("%s has %d parts, want at least %d", inlineDecodeFn, n, inlinePartsMin)
	}
}

// CODEGEN.md §2.11 (Same behaviour): the cut decoder of inline variants vets and answers as the whole did, the same values and the same first failure.
func TestInlineOnlyCutRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	dump := func(gen generator) string {
		files := inlineFilesAt(t, gen, inlineFields, inlineCosts)
		mod, out := t.TempDir(), filepath.Join(t.TempDir(), dumpName)
		writeNoncedModule(t, mod, partsModule, files, map[string][]byte{inlineSmokeFile: inlineSmoke(inlineFields, inlineCosts)})
		goCmd(t, mod, "vet", "./...")
		t.Setenv(dumpEnv, out)
		goCmd(t, mod, "test", "-count=1", "./inl")
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	whole, cut := dump(gogen.Generate), dump(cutAt(inlineLimit))
	if whole != cut {
		t.Errorf("the cut program answers differently:\n%s\nvs\n%s", whole, cut)
	}
	if !strings.Contains(whole, `{`+goodReward+`,`+goodCost+`}: [`) {
		t.Errorf("the dump has no good answer:\n%s", whole)
	}
	checkInlineFirstFailure(t, cut)
	checkInlineAlignment(t, string(inlineFilesAt(t, cutAt(inlineLimit), inlineFields, inlineCosts)[inlineFile]))
}

// checkInlineAlignment checks, whatever a field's line count, that the cut falls where the test needs it: the first part holds no Cost field and the last none of Reward, so a Cost failure is a later part's, and a part called out of order shows.
func checkInlineAlignment(t *testing.T, src string) {
	t.Helper()
	var parts []string
	for n := 0; ; n++ {
		body, ok := funcText(src, ir.GoPart(inlineDecodeFn, n))
		if !ok {
			break
		}
		parts = append(parts, body)
	}
	if len(parts) < inlinePartsMin+1 {
		t.Fatalf("%s has %d parts, want at least %d", inlineDecodeFn, len(parts), inlinePartsMin+1)
	}
	if first := parts[0]; strings.Contains(first, "decodeCost(") || !strings.Contains(first, "decodeReward(") {
		t.Errorf("the first part is not Reward fields only:\n%s", first)
	}
	if last := parts[len(parts)-1]; strings.Contains(last, "decodeReward(") || !strings.Contains(last, "decodeCost(") {
		t.Errorf("the last part is not Cost fields only:\n%s", last)
	}
}

// funcText is the text of the function name in src, up to the next function; false when there is none.
func funcText(src, name string) (string, bool) {
	_, rest, ok := strings.Cut(src, "func "+name+"(")
	if !ok {
		return "", false
	}
	body, _, _ := strings.Cut(rest, "\nfunc ")
	return body, true
}

// checkInlineFirstFailure checks the answers to the bad Reward, the bad Cost (a later part's) and both: both answer as the Reward, the first part's, and the Cost's differs.
func checkInlineFirstFailure(t *testing.T, dump string) {
	t.Helper()
	answer := map[string]string{}
	for _, line := range strings.Split(dump, "\n") {
		doc, text, _ := strings.Cut(line, ": ")
		answer[doc] = text
	}
	reward, cost, both := answer[docBadReward], answer[docBadCost], answer[docBothBad]
	if reward == "" || reward == cost || both != reward {
		t.Errorf("the first failure is not the first part's:\nbad reward: %s\nbad cost: %s\nboth: %s", reward, cost, both)
	}
}
