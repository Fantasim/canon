package progen_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	typedFile       = "typed/typed.canon"
	typedPackage    = "typed"
	typedOutRoot    = "out"
	typedModule     = "example.com/typed"
	typedGoImport   = typedModule + "/go"
	typedGoOut      = "@out/go/" // WIRE.md §2.3: a build's display path keeps its "@root" alias
	typedDataOut    = "@out/data/"
	typedBudget     = "budget: 200_000"
	shortTyped      = 3
	typedNightlyCap = 1000 // a case compiles and tests a Go module (about 0.3 s with a warm build cache)
	minRecords      = 1
	maxRecords      = 2
	minEnums        = 0
	maxEnums        = 2
	propTyped       = "typed"
	testTyped       = "TestTyped"
	zzIntBase       = "ZZ_INT_BASE" // a zero-valued const every Int/Range default adds a literal to
	zzStrBase       = "ZZ_STR_BASE" // a zero-valued const every String default adds a literal to
	harnessDir      = "progen/"     // an archive's files the compiler never reads (programOf)
	expectFile      = harnessDir + "expect.json"
	smokeFile       = harnessDir + "smoke_test.go"
)

// typedProjectSrc is every case's project: one root, mapped for Go (CODEGEN.md §2.8).
const typedProjectSrc = `project typed {
  canon: "0.1"
  ` + typedBudget + `
  roots {
    ` + typedOutRoot + `: "out"
  }
  go_module {
    ` + typedOutRoot + `: "` + typedModule + `"
  }
}
`

// TestTyped is DECISIONS 200 item 3: a program well-typed by construction checks clean, builds,
// its JSON holds the values it was written with, and its generated Go compiles and answers as
// the evaluator does: its values' getters, and an export fn against the evaluator's results.
func TestTyped(t *testing.T) {
	n := min(total(shortTyped), typedNightlyCap)
	if supervise(t, testTyped, n) {
		return
	}
	for i := *flagFrom; i < n; i++ {
		seed := caseSeed(suiteTyped, i)
		announce(suiteTyped, propTyped, i, seed)
		t.Run(fmt.Sprintf("%s_%d", propTyped, i), func(t *testing.T) {
			runTyped(t, i, seed)
		})
	}
}

func runTyped(t *testing.T, i int, seed uint64) {
	t.Helper()
	files := buildTyped(seed)
	v := verifyTyped(files)
	if v.Kind == "" || reported(t, suiteTyped, propTyped, seed, v) {
		return
	}
	report(t, typedArchive(i, seed, files, v), v)
}

// typedArchive is case i kept whole (decision log "A6 progen suites 3–4 — review calls": each
// case compiles Go, so it is not shrunk): the program and the harness files that judge it.
func typedArchive(i int, seed uint64, files *progen.Project, v verdict) *progen.Counterexample {
	return &progen.Counterexample{
		Suite: suiteTyped, Name: propTyped, Case: i, Seed: seed, Sig: v.Sig,
		Packages: []string{typedPackage}, Want: propTyped, Note: v.Text, Files: files,
	}
}

// typedCase is one seeded program: its model, record roots, a table root, a ref root into it,
// and an export fn with the calls the evaluator answers (DECISIONS 200 item 3).
type typedCase struct {
	mo    *typedModel
	roots []typedRoot
	table tableRoot
	ref   refRoot
	fn    typedFn
}

// buildTyped is one seeded case as an archive's files: the project, the program, what its JSON
// must hold (expectFile) and the Go test its generated package must pass (smokeFile).
func buildTyped(seed uint64) *progen.Project {
	r := progen.NewRand(seed)
	mo := genModel(r, r.Intn(maxEnums-minEnums+1)+minEnums, r.Intn(maxRecords-minRecords+1)+minRecords)
	ensureDefault(r, mo)
	tc := typedCase{mo: mo, roots: genTypedRoots(r, mo), table: genTable(r, mo, mo.records[0])}
	tc.ref = refRoot{name: "firstItem", recName: mo.records[0].name, key: tableKeys[0]}
	tc.fn = genFn(r)
	files := progen.NewProject()
	files.Set(projectFile, []byte(typedProjectSrc))
	files.Set(typedFile, renderTypedSource(tc))
	files.Set(expectFile, expectations(tc))
	files.Set(smokeFile, renderSmoke(tc))
	return files
}

// verifyTyped is "" when the program checks and builds clean, its JSON is expectFile's, and its
// Go passes smokeFile and the build's conformance test.
func verifyTyped(files *progen.Project) (v verdict) {
	defer recoverVerdict(&v)
	prog := programOf(files)
	opt := progen.RunOptions{Packages: []string{typedPackage}}
	if bad, ok := typedBroken(modeCheck, progen.Run(context.Background(), prog, opt)); ok {
		return bad
	}
	opt.Build, opt.Targets = true, goAndJSON
	built := progen.Run(context.Background(), prog, opt)
	if bad, ok := typedBroken(modeBuild, built); ok {
		return bad
	}
	expect, _ := files.Get(expectFile)
	if v := checkExpected(outputMap(built.Outputs), expect); v.Kind != "" {
		return v
	}
	smoke, _ := files.Get(smokeFile)
	return compileTyped(built.Outputs, smoke)
}

// typedBroken is the verdict of an outcome that panicked, errored or reported any finding: a
// program built to be well-typed must check and build clean.
func typedBroken(stage string, out progen.Outcome) (verdict, bool) {
	if v, bad := broken(out); bad {
		return v, true
	}
	if out.Err != nil || len(out.Findings) > 0 {
		kind := kindMissing + " " + stage
		return verdict{Kind: kind, Sig: kind + " " + shapes(out.Findings), Text: fmt.Sprintf("%s: got %v, findings %v", stage, out.Err, out.Findings)}, true
	}
	return verdict{}, false
}

// replayTyped re-runs a kept typed archive from its files alone.
func replayTyped(c *progen.Counterexample) verdict { return verifyTyped(c.Files) }
