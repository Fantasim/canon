package eval

import (
	"maps"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
)

// beginTestLog starts the log of a test of pkg: its stops and what poisons the values it forces (CLI.md §3.5).
func (e *Evaluator) beginTestLog(pkg string) {
	e.testStops, e.testFiles, e.testPkg = map[string]*diag.Bag{}, e.files(), e.key(pkg)
	e.logCauses()
}

// LogCauses makes the evaluator keep what poisons each value it forces, as a test's log does (EVALUATION.md §7.2).
func (e *Evaluator) LogCauses() {
	if e.testFiles == nil {
		e.testFiles = e.files()
	}
	e.logCauses()
}

// logCauses starts keeping what poisons each value, once.
func (e *Evaluator) logCauses() {
	if e.causes == nil {
		e.causes, e.via = map[*rootState]*diag.Bag{}, map[*rootState]*rootState{}
	}
}

// logBag is an unlimited bag of pkg for a test's log.
func (e *Evaluator) logBag(pkg string) *diag.Bag {
	b := diag.NewBag(e.testFiles, pkg)
	b.Truncate(math.MaxInt)
	return b
}

// stopBag is the running test's bag of stopping errors raised in a frame of pkg.
func (e *Evaluator) stopBag(pkg string) *diag.Bag {
	if e.testStops[pkg] == nil {
		e.testStops[pkg] = e.logBag(pkg)
	}
	return e.testStops[pkg]
}

// stopFindings is the running test's stopping errors, by package name.
func (e *Evaluator) stopFindings() []diag.Finding {
	var out []diag.Finding
	for _, pkg := range slices.Sorted(maps.Keys(e.testStops)) {
		out = append(out, e.testStops[pkg].Findings()...)
	}
	return out
}

// noteStop keeps, while a test runs, a hard error of its own statements, and, while causes are
// logged, the one that poisons the value a run evaluates or verifies; the bag gets it too.
func (r *run) noteStop(b *diag.Builder) {
	e := r.ev
	switch {
	case e.testStops != nil && r.sink == nil && r.test != nil:
		b.Report(e.stopBag(r.fr.pkg))
	case e.causes != nil && (r.free || r.emitted != nil) && e.aside == nil: // stage B; a host verifying aside keeps its own
		if e.stageB == nil {
			e.stageB = e.logBag(r.fr.pkg)
		}
		b.Report(e.stageB)
	case e.causes != nil && r.root != nil && e.causes[r.root] == nil:
		e.causes[r.root] = e.logBag(r.fr.pkg)
		b.Report(e.causes[r.root])
	}
}

// notePoisonedRead keeps, while causes are logged, that the value r evaluates is poisoned by reading root.
func (e *Evaluator) notePoisonedRead(r *run, root Root) {
	if e.causes == nil || r.root == nil || e.via[r.root] != nil {
		return
	}
	if st := e.rootState(root); st != nil {
		e.via[r.root] = st
	}
}

// noteOut keeps E4401 b as the stop of a test of its package and as the cause of r's value (API.md R6).
func (e *Evaluator) noteOut(r *run, b *diag.Builder) {
	pkg := r.charge.pkg
	if e.testStops != nil && e.testPkg == pkg {
		b.Report(e.stopBag(pkg))
	}
	if e.causes != nil && r.root != nil && e.causes[r.root] == nil {
		e.causes[r.root] = e.logBag(pkg)
		b.Report(e.causes[r.root])
	}
}

// outCause is the E4401 of pkg's spent budget, the cause of a value it left unevaluated (API.md R6).
func (e *Evaluator) outCause(st *rootState) []diag.Finding {
	b := e.out[e.key(st.root.Pkg)]
	if b == nil || !st.starved && st.status != idle {
		return nil
	}
	bag := diag.NewBag(e.files(), st.root.Pkg)
	b.Report(bag)
	return bag.Findings()
}

// Cause is the value first poisoned and its evaluation's hard error, as a test kept it (EVALUATION.md §7.2).
type Cause struct {
	Root     Root
	Findings []diag.Finding
}

// PoisonCause follows a poisoned value through the poisoned values it read to the first one.
func (e *Evaluator) PoisonCause(root Root) Cause {
	st := e.rootState(root)
	seen := map[*rootState]bool{}
	for st != nil && e.via[st] != nil && !seen[st] {
		seen[st] = true
		st = e.via[st]
	}
	if st == nil {
		return Cause{Root: root}
	}
	c := Cause{Root: st.root}
	if bag := e.causes[st]; bag != nil {
		c.Findings = bag.Findings()
	} else {
		c.Findings = e.outCause(st)
	}
	return c
}

// ReportAside sends the evaluator's findings to bags until restore is called: a verification
// whose findings its host keeps apart, as a vector's are (canon test's poison causes).
func (e *Evaluator) ReportAside(bags check.Bags) (restore func()) {
	saved := e.aside
	e.aside = bags
	return func() { e.aside = saved }
}
