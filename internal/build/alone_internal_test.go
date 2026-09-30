package build

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strconv"
	"testing"
)

const (
	aloneNone   = "testdata/incremental/readers_none.txtar"
	aloneLet    = "testdata/incremental/readers_let.txtar"
	aloneView   = "testdata/incremental/readers_view.txtar"
	aloneList   = "testdata/incremental/readers_list.txtar"
	aloneCond   = "testdata/incremental/readers_cond.txtar"
	aloneLoad   = "testdata/incremental/readers_load.txtar"
	aloneBare   = "testdata/incremental/readers_bare.txtar"
	aloneTwo    = "a/items/two.canon"
	aloneSwitch = "s/switches/main.canon"
	aloneOn     = 1
	aloneOff    = 0
	aloneWeight = 5
)

// aloneCase is a project whose later package reads a.items in one way, analyzed along edits.
type aloneCase struct {
	archive string
	sel     []string
	steps   []aloneStep
}

// aloneStep sets every number of one file to n, then wants the warm analysis to replay stage C
// in checked entries at the full budget.
type aloneStep struct {
	file    string
	n       int
	checked int
}

// aloneCases: a later value holds a part of a.items only if its evaluation read it, directly or
// through values read (log-2026-09-29 M4 P18); a.items has three entries, one edited at a time.
var aloneCases = []aloneCase{
	// b imports a, never reads a.items: a's two unchanged entries and b's own replay
	{archive: aloneNone, steps: []aloneStep{{aloneTwo, aloneWeight, 3}, {aloneTwo, aloneWeight + 1, 3}}},
	// b reads a.items through m.all, m loaded but not traversed
	{archive: aloneLet, sel: []string{"a", "b"}, steps: []aloneStep{{aloneTwo, aloneWeight, 0}, {aloneTwo, aloneWeight + 1, 0}}},
	// only b's view reads a.items, after stage C
	{archive: aloneView, steps: []aloneStep{{aloneTwo, aloneWeight, 3}, {aloneTwo, aloneWeight + 1, 3}}},
	// b holds a's records in a list, a map and a let of one's part
	{archive: aloneList, steps: []aloneStep{{aloneTwo, aloneWeight, 0}, {aloneTwo, aloneWeight + 1, 0}}},
	// b's load decodes rows whose default reads a.items
	{archive: aloneLoad, steps: []aloneStep{{aloneTwo, aloneWeight, 0}, {aloneTwo, aloneWeight + 1, 0}}},
	// verification fills b's bare case with a default reading a.items (stage B, charged to b.things)
	{archive: aloneBare, steps: []aloneStep{{aloneTwo, aloneWeight, 0}, {aloneTwo, aloneWeight + 1, 0}}},
	// b reads a.items while the switch is on
	{archive: aloneCond, steps: []aloneStep{
		{aloneSwitch, aloneOn, 0}, {aloneSwitch, aloneOff, 3}, {aloneTwo, aloneWeight, 2},
		{aloneSwitch, aloneOn, 0}, {aloneTwo, aloneWeight + 1, 0}, {aloneSwitch, aloneOff, 2},
	}},
}

// EVALUATION.md §8.1, log-2026-09-29 M4 P12-r, P18: stage C replays a table read by no other value traversed.
func TestAloneReaders(t *testing.T) {
	for _, c := range aloneCases {
		t.Run(path.Base(c.archive), func(t *testing.T) {
			z := archiveAnalyzer(t, c.archive)
			warm, cold := z.pairSel(t, c.sel)
			same(t, "first", warm, cold)
			for i, s := range c.steps {
				z.edit(t, s.file, s.n)
				warm, cold = z.pairSel(t, c.sel)
				same(t, fmt.Sprintf("step %d", i), warm, cold)
				settled(t, warm)
				if _, checked := replaysOf(warm); checked != s.checked {
					t.Errorf("step %d, %s set to %d: stage C replayed %d entries, want %d", i, s.file, s.n, checked, s.checked)
				}
			}
		})
	}
}

// EVALUATION.md §8.1, §12.2, log-2026-09-29 M4 P12-r, P18: at every budget, stage C findings in order as cold.
func TestAloneReadersBudgets(t *testing.T) {
	for _, c := range aloneCases {
		t.Run(path.Base(c.archive), func(t *testing.T) {
			need := c.need(t)
			t.Logf("a cold analysis needs %d steps", need)
			for n := 1; n <= need; n++ {
				z := c.budgeted(t, n)
				warm, cold := z.pairSel(t, c.sel)
				same(t, fmt.Sprintf("budget %d, first", n), warm, cold)
				for i, s := range c.steps {
					z.edit(t, s.file, s.n)
					warm, cold = z.pairSel(t, c.sel)
					same(t, fmt.Sprintf("budget %d, step %d", n, i), warm, cold)
				}
			}
		})
	}
}

// settled fails unless every value a traverses completed: a case whose later package does not
// evaluate proves nothing about its reads.
func settled(t *testing.T, a *Analysis) {
	t.Helper()
	for _, root := range a.r.order {
		if _, ok := a.r.ev.Settled(root); !ok {
			t.Fatalf("%s.%s did not complete", root.Pkg, root.Name)
		}
	}
}

// need is the least budget c's cold analysis, along its edits, does not exhaust.
func (c aloneCase) need(t *testing.T) int {
	t.Helper()
	low, high := 1, budgetCeiling
	for low < high {
		mid := (low + high) / 2
		if c.exhausts(t, mid) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low == budgetCeiling {
		t.Fatalf("still exhausted at %d steps", budgetCeiling)
	}
	return low
}

// exhausts reports a cold analysis of c at budget n that runs out of steps at any of its edits.
func (c aloneCase) exhausts(t *testing.T, n int) bool {
	t.Helper()
	z := c.budgeted(t, n)
	if _, cold := z.pairSel(t, c.sel); exhaustedIn(cold) {
		return true
	}
	for _, s := range c.steps {
		z.edit(t, s.file, s.n)
		if _, cold := z.pairSel(t, c.sel); exhaustedIn(cold) {
			return true
		}
	}
	return false
}

// budgeted is c's project with its budget set to n (EVALUATION.md §12.2).
func (c aloneCase) budgeted(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, c.archive)
	abs := path.Join(archiveRoot, memoProject)
	text, err := z.fs.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	z.fs.set(abs, bytes.Replace(text, []byte(memoBudgetLine), fmt.Appendf(nil, "%s  budget: %d\n", memoBudgetLine, n), 1))
	return z
}

// EVALUATION.md §8.1, log-2026-09-29 M4 P18: the benchmark's items replay stage C beside twin, as cold.
func TestAloneBenchItems(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a benchmark project with go run")
	}
	z := benchAnalyzer(t)
	entries := entryFiles(t, z)
	sel := []string{"items", "twin"}
	z.pairSel(t, sel)
	bumpLast(t, z, entries[0])
	warm, cold := z.pairSel(t, sel)
	same(t, "edited", warm, cold)
	if _, checked := replaysOf(warm); checked != 2*len(entries)-1 {
		t.Errorf("stage C replayed %d entries and elements, want %d", checked, 2*len(entries)-1)
	}
}

// bumpLast adds one to the last number a field of the file abs sets.
func bumpLast(t *testing.T, z *analyzer, abs string) {
	t.Helper()
	text, err := z.fs.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	at := fieldNumber.FindAllSubmatchIndex(text, -1)
	if len(at) == 0 {
		t.Fatalf("%s sets no number", abs)
	}
	last := at[len(at)-1]
	n, err := strconv.Atoi(string(bytes.ReplaceAll(text[last[2]:last[3]], []byte("_"), nil)))
	if err != nil {
		t.Fatal(err)
	}
	z.fs.set(abs, slices.Concat(text[:last[2]], []byte(strconv.Itoa(n+1)), text[last[3]:]))
}
