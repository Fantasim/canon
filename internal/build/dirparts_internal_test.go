package build

import (
	"fmt"
	"path"
	"strings"
	"testing"
)

const (
	dirPartsCase = "testdata/incremental/dirparts.txtar"
	dirI1        = "resource/items/i1.json"
	dirI2        = "resource/items/i2.json"
	dirI3        = "resource/items/i3.json"
	dirI4        = "resource/items/i4.json"
	dirI5        = "resource/items/i5.json"
	dirR1        = "resource/rows/r1.json"
	dirR9        = "resource/rows/r9.json"
	dirE1        = "resource/extras/e1.json"
	dirD0        = "resource/dots/d0.json"
	dirL1        = "resource/lims/l1.json"
	dirServed    = 3 // the loads whose every element is kept apart: items, rows, dots

	dirCap = "a/caps/main.canon"
)

// dirStep is a change of the case's files, and what the warm analysis after it must have
// replayed: its load.dir elements served, and the elements whose stages B and C it replayed.
type dirStep struct {
	name                     string
	do                       func(m roFS)
	parts, verified, checked int
}

// dirFiles are the case's file edits: a file put, one removed, one renamed, one edited in place.
type dirFiles struct {
	m roFS
}

func (d dirFiles) name(n string) string { return strings.TrimPrefix(path.Join(archiveRoot, n), "/") }

func (d dirFiles) put(n, data string) func(roFS) {
	return func(m roFS) { m[d.name(n)] = srcFile(data) }
}

func (d dirFiles) remove(n string) func(roFS) {
	return func(m roFS) { delete(m, d.name(n)) }
}

func (d dirFiles) rename(from, to string) func(roFS) {
	return func(m roFS) {
		m[d.name(to)] = m[d.name(from)]
		delete(m, d.name(from))
	}
}

func (d dirFiles) edit(n, old, new string) func(roFS) {
	return func(m roFS) {
		m[d.name(n)] = srcFile(strings.Replace(string(m[d.name(n)].Data), old, new, 1))
	}
}

// dirSteps are TestDirPartsEqualsCold's steps. Unchanged, the elements of items, rows and dots
// are served (8), stage B replays them and caps.main (9), stage C those of items and dots, each
// the last value of its package to complete (6).
func dirSteps(d dirFiles) []dirStep {
	i2 := string(d.m[d.name(dirI2)].Data)
	return []dirStep{
		{"unchanged", func(roFS) {}, 8, 9, 6},
		// the lims wait for the second pass: decoded again, none served
		{"lim edited", d.edit(dirL1, `"lim": 2`, `"lim": 3`), 8, 9, 6},
		// the extras replay their whole record until a file changes: e2 is then served, e1 decoded
		{"extra edited", d.edit(dirE1, `"e1"`, `"e1b"`), 9, 10, 6},
		{"json edited", d.edit(dirI2, `"cost": 3`, `"cost": 5`), 7, 8, 5},
		{"json added", d.put(dirI4, `{"sku": "i4", "cost": 1, "cat": "blue"}`), 8, 9, 6},
		{"json removed", d.remove(dirI1), 8, 9, 6},
		{"json renamed", d.rename(dirI3, dirI5), 7, 8, 5},
		{"row renamed", d.rename(dirR1, dirR9), 7, 8, 6},
		{"json broken", d.edit(dirI2, `"cost": 5`, `"cost": }`), 7, 6, 3}, // items poisoned: none verified
		{"json put back", d.put(dirI2, i2), 7, 8, 5},
		{"check fails", d.edit(dirI4, `"cost": 1`, `"cost": 9`), 7, 8, 5},
		{"value out of range", d.edit(dirI4, `"cost": 9`, `"cost": 120`), 7, 8, 5},
		// caps.main and each check reading caps run again; i4, out of range, runs none (EVALUATION.md §7.3)
		{"cap lowered", d.edit(dirCap, "most: 6", "most: 2"), 8, 8, 4},
		// the dots' indexes move: their paths, so stages B and C, change (API.md P8)
		{"dot added first", d.put(dirD0, `{"n": 2}`), 8, 6, 3},
		{"unchanged again", func(roFS) {}, 9, 10, 7},
	}
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, WIRE.md §6.5, EVALUATION.md §5, §8.1: load.dir files edited, warm as cold.
func TestDirPartsEqualsCold(t *testing.T) {
	z := archiveAnalyzer(t, dirPartsCase)
	d := dirFiles{m: z.fs.base.(roFS)}
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	if n := warm.r.ev.PartsReplayed(); n != 0 {
		t.Errorf("first: %d elements served from an empty memo", n)
	}
	for _, st := range dirSteps(d) {
		st.do(d.m)
		warm, cold = z.pair(t)
		same(t, st.name, warm, cold)
		v, c := replaysOf(warm)
		if n := warm.r.ev.PartsReplayed(); n != st.parts || v != st.verified || c != st.checked {
			t.Errorf("%s: %d elements served, stages B and C replayed %d and %d; want %d, %d and %d",
				st.name, n, v, c, st.parts, st.verified, st.checked)
		}
		if n := warm.r.ev.LoadsServed(); strings.HasPrefix(st.name, "unchanged") && n != dirServed {
			t.Errorf("%s: %d loads served whole from their elements, want %d", st.name, n, dirServed)
		}
	}
}

// EVALUATION.md §12.2, IMPLEMENTATION-PLAN §7.6 NFR-02: every budget up to the largest cold need of any step, warm as cold.
func TestDirPartsBudgets(t *testing.T) {
	need := 0
	for k := range len(dirSteps(dirFiles{m: archiveAnalyzer(t, dirPartsCase).fs.base.(roFS)})) + 1 {
		need = max(need, dirNeed(t, k))
	}
	t.Logf("a cold analysis needs up to %d steps", need)
	for n := 1; n <= need; n++ {
		z := dirBudgeted(t, n)
		d := dirFiles{m: z.fs.base.(roFS)}
		warm, cold := z.pair(t)
		same(t, fmt.Sprintf("budget %d, first", n), warm, cold)
		for _, st := range dirSteps(d) {
			st.do(d.m)
			warm, cold = z.pair(t)
			same(t, fmt.Sprintf("budget %d, %s", n, st.name), warm, cold)
		}
	}
}

// dirNeed is the least budget a cold analysis of the case after its first k steps does not exhaust.
func dirNeed(t *testing.T, k int) int {
	t.Helper()
	low, high := 1, budgetCeiling
	for low < high {
		mid := (low + high) / 2
		z := dirBudgeted(t, mid)
		d := dirFiles{m: z.fs.base.(roFS)}
		for _, st := range dirSteps(d)[:k] {
			st.do(d.m)
		}
		p, err := Open(z.fs, z.dir, z.opt)
		if err != nil {
			t.Fatal(err)
		}
		cold, err := p.Analyze(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if exhaustedIn(cold) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low == budgetCeiling {
		t.Fatalf("after %d steps: still exhausted at %d steps", k, budgetCeiling)
	}
	return low
}

// dirBudgeted is the load.dir case with project.canon's budget set to n (EVALUATION.md §12.2).
func dirBudgeted(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, dirPartsCase)
	z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, budgetFormat, n))
	return z
}
