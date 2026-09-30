package edit_test

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// benchProject is a benchgen project FuzzMinimalWriteAll edits too; make fuzz-edit sets it.
var benchProject = flag.String("edit.bench", "", "a benchgen project FuzzMinimalWriteAll also edits (make fuzz-edit)")

// fuzzTally is where each exec appends a line make fuzz-edit counts (IMPLEMENTATION-PLAN §7.6).
var fuzzTally = flag.String("edit.tally", "", "a file each FuzzMinimalWriteAll exec appends a line to (make fuzz-edit)")

// fuzzWorkerFlag is the flag go test gives a fuzzing worker process, which testing gives no
// F.Add seed (it runs only what the coordinator sends); tallyMode is the tally file's mode, and
// tallySeed the word of a seed's line.
const (
	fuzzWorkerFlag = "test.fuzzworker"
	tallyMode      = 0o644
	tallySeed      = "seed"
)

// outcomes are an exec's tally word: whether Apply wrote its edit or refused it.
var outcomes = map[bool]string{true: "applied", false: "refused"}

// opWords name the first operation's kind on a tally line: make fuzz-edit wants Set, Add and
// Remove applied on the benchmark.
var opWords = [...]string{
	edit.OpSet: "Set", edit.OpReset: "Reset", edit.OpAdd: "Add", edit.OpInsert: "Insert",
	edit.OpAddEntry: "AddEntry", edit.OpRemove: "Remove", edit.OpMove: "Move", edit.OpRename: "Rename",
	edit.OpRetire: "Retire", edit.OpUnretire: "Unretire", edit.OpSetCase: "SetCase",
}

// fuzzProjects are the examples, then the benchmark project when -edit.bench names one.
func fuzzProjects(t testing.TB) []*fuzzProject {
	t.Helper()
	dir, roots := exampleRoots(t)
	out := []*fuzzProject{openFuzzProject(t, dir, roots)}
	if *benchProject != "" {
		bench, err := filepath.Abs(*benchProject)
		if err != nil {
			t.Fatal(err)
		}
		fz := openFuzzProject(t, filepath.ToSlash(bench), nil)
		fz.name, fz.ops, fz.copies = projBench, benchOps, benchCopies
		out = append(out, fz)
	}
	return out
}

// API.md E1, M6 (M4 acceptance item 3): up to three random operations of every kind on every
// example, and with -edit.bench one on the benchmark project, are refused as Apply documents or
// write files that are fixed points, N12-clean and minimal; one Set of a scalar changes one line.
func FuzzMinimalWriteAll(f *testing.F) {
	projects := fuzzProjects(f)
	if !fuzzWorker() {
		addSeeds(f, projects)
	}
	f.Fuzz(func(t *testing.T, raw []byte, n int64, s string) {
		if len(raw) == 0 {
			return
		}
		fz := projects[int(raw[0])%len(projects)]
		if ops := fz.draw(raw[1:], n, s); len(ops) > 0 {
			applied := applyChecked(t, fz.env, fz.a, ops, oneValueSet(fz.a, ops))
			tally(t, fz.name, outcomes[applied], ops[0].Kind)
		}
	})
}

// oneValueSet is whether ops is one Set of a scalar replacing an item of one line, as the golden
// tests count it (scalarSet): none included, the item it replaces being what decides
// (log-2026-09-29 P20-r2, API.md M6, M5).
func oneValueSet(a *build.Analysis, ops []edit.Operation) bool {
	return len(ops) == 1 && scalarSet(a, ops, 0)
}

// fuzzWorker is whether this process is a fuzzing worker. Without the flag (a future Go), every
// process computes the seeds, as before: slower, never wrong.
func fuzzWorker() bool {
	fl := flag.Lookup(fuzzWorkerFlag)
	if fl == nil {
		return false
	}
	g, ok := fl.Value.(flag.Getter)
	if !ok {
		return false
	}
	on, _ := g.Get().(bool)
	return on
}

// addSeeds adds every project's seeds, after proving them on the benchmark (coversBench), and
// tallies each, for make fuzz-edit to count only what fuzzing applied. Only the coordinator
// needs them, so a worker spends its time fuzzing (log-2026-09-29 M4 P20).
func addSeeds(f *testing.F, projects []*fuzzProject) {
	for i, fz := range projects {
		seeds := fz.seeds(byte(i))
		if fz.name == projBench {
			fz.coversBench(f, seeds)
		}
		for _, seed := range seeds {
			f.Add(seed.raw, int64(seedN), seedS)
			tally(f, fz.name, tallySeed, fz.draw(seed.raw[1:], seedN, seedS)[0].Kind)
		}
	}
}

// tally appends "<project> <word> <operation>" to -edit.tally when set.
func tally(tb testing.TB, project, word string, kind edit.Op) {
	tb.Helper()
	if *fuzzTally == "" {
		return
	}
	out, err := os.OpenFile(*fuzzTally, os.O_APPEND|os.O_CREATE|os.O_WRONLY, tallyMode)
	if err != nil {
		tb.Fatal(err)
	}
	_, err = fmt.Fprintln(out, project, word, opWords[kind])
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		tb.Fatal(err)
	}
}

// coversBench applies the benchmark's seeds once: every package of the benchmark gets an
// operation applied, and so do Set, Add and Remove (log-2026-09-29 M4 U7b-r).
func (fz *fuzzProject) coversBench(tb testing.TB, seeds []fuzzSeed) {
	tb.Helper()
	applied, pkgs := map[edit.Op]int{}, map[string]int{}
	for _, seed := range seeds {
		ops := fz.draw(seed.raw[1:], seedN, seedS)
		pkgs[seed.pkg] += 0
		if _, err := edit.Apply(context.Background(), fz.env, edit.NewSnapshot(fz.a), edit.Request{Ops: ops}); err == nil {
			applied[ops[0].Kind]++
			pkgs[seed.pkg]++
		}
	}
	tb.Logf("benchmark seeds applied by operation %v; by package %v", applied, pkgs)
	for _, k := range []edit.Op{edit.OpSet, edit.OpAdd, edit.OpRemove} {
		if applied[k] == 0 {
			tb.Fatalf("operation %d applied to nothing on the benchmark", k)
		}
	}
	for _, pkg := range slices.Sorted(maps.Keys(pkgs)) {
		if pkgs[pkg] == 0 {
			tb.Fatalf("%s: no operation applied on the benchmark", pkg)
		}
	}
}
