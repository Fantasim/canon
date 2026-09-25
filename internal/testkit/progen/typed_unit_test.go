package progen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// TestGenModelBounds proves genModel stays inside its shape (IMPLEMENTATION-PLAN.md §7.7 item 3).
func TestGenModelBounds(t *testing.T) {
	for seed := uint64(1); seed < 50; seed++ {
		r := progen.NewRand(seed)
		mo := genModel(r, r.Intn(maxEnums-minEnums+1)+minEnums, r.Intn(maxRecords-minRecords+1)+minRecords)
		if len(mo.records) < minRecords || len(mo.records) > maxRecords {
			t.Fatalf("seed %d: %d records, want %d..%d", seed, len(mo.records), minRecords, maxRecords)
		}
		checkModelFields(t, seed, mo)
	}
}

// checkModelFields is TestGenModelBounds's field-level pass, kept apart to stay within the
// audit's nesting limit.
func checkModelFields(t *testing.T, seed uint64, mo *typedModel) {
	t.Helper()
	for _, rec := range mo.records {
		if len(rec.fields) == 0 {
			t.Fatalf("seed %d: record %s has no field", seed, rec.name)
		}
		for _, f := range rec.fields {
			if f.ft.kind == tyEnum && len(mo.enums) == 0 {
				t.Fatalf("seed %d: field %s is an enum but the model has none", seed, f.name)
			}
		}
	}
}

// TestBuildTypedDeterministic proves a case's archive is a pure function of its seed (DOCTRINE.md §5).
func TestBuildTypedDeterministic(t *testing.T) {
	seed := uint64(4242)
	a, b := buildTyped(seed), buildTyped(seed)
	for _, name := range a.Names() {
		x, _ := a.Get(name)
		y, _ := b.Get(name)
		if string(x) != string(y) {
			t.Fatalf("seed %d: two builds wrote different %s", seed, name)
		}
	}
}

// reProbe is a probe declaration: a public value the evaluator computes by calling the fn.
var reProbe = regexp.MustCompile(`(?m)^let ` + probePrefix + `\d+: Int = ` + fnName + `\(`)

// DECISIONS 200 item 3 and the review call "its answers equal the evaluator's": every program
// exports a fn and values that call it, the smoke test compares those calls in Go with the
// evaluator's JSON, and the first root leaves a defaulted field to the evaluator.
func TestTypedCaseComputes(t *testing.T) {
	for seed := uint64(1); seed < 40; seed++ {
		files := buildTyped(seed)
		src, _ := files.Get(typedFile)
		smoke, _ := files.Get(smokeFile)
		switch {
		case !strings.Contains(string(src), "export fn "+fnName+"("):
			t.Fatalf("seed %d: no export fn:\n%s", seed, src)
		case len(reProbe.FindAll(src, -1)) != probeCount:
			t.Fatalf("seed %d: want %d probes:\n%s", seed, probeCount, src)
		case !strings.Contains(string(smoke), "probe(t, \""+probePrefix+"0\")"):
			t.Fatalf("seed %d: the smoke test reads no probe:\n%s", seed, smoke)
		}
		r := progen.NewRand(seed)
		mo := genModel(r, r.Intn(maxEnums-minEnums+1)+minEnums, r.Intn(maxRecords-minRecords+1)+minRecords)
		ensureDefault(r, mo)
		if !omitsDefault(genTypedRoots(r, mo)[0]) {
			t.Fatalf("seed %d: the first root spells every defaulted field", seed)
		}
	}
}

// omitsDefault tells a root whose literal leaves a defaulted field out.
func omitsDefault(root typedRoot) bool {
	for _, f := range root.rec.fields {
		if f.def != nil && !strings.Contains(root.text, f.name+":") {
			return true
		}
	}
	return false
}

// TYPES.md §5.1 (E3008): every operator genIntExpr writes has an operand that reads a parameter.
func TestGenIntExprReadsParams(t *testing.T) {
	lit := regexp.MustCompile(`\(\d+ [-+*] \d+\)|\(\d+\)`)
	for seed := uint64(1); seed < 200; seed++ {
		e := genIntExpr(progen.NewRand(seed), fnParamNames, fnMaxDepth)
		names := strings.NewReplacer("abs(", "(", "min(", "(", "max(", "(").Replace(e)
		if lit.MatchString(e) || !strings.ContainsAny(names, "ab") {
			t.Fatalf("seed %d: %s", seed, e)
		}
	}
}

// A gofail signature names what failed, free of what a seed draws (the review's "real
// signatures").
func TestGoFailSig(t *testing.T) {
	out := "# example.com/typed/go\ngo/typed.go:12:3: undefined: rt.Foo7\n" +
		"--- FAIL: TestSmoke (0.00s)\n    smoke_test.go:9: " + smokeMark + "Int/list f2: got [1]\n" +
		"--- FAIL: TestCalcConformance (0.00s)\n"
	want := kindGoFail + " Int/list, TestCalcConformance, TestSmoke, undefined: rt.FooN"
	if got := goFailSig(out); got != want {
		t.Errorf("goFailSig = %q, want %q", got, want)
	}
}
