package edit_test

import (
	"bytes"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// checkFixedPoint is the first half of API.md M6: format(after) == after, for .canon files
// and for JSON sources in their canonical layout (FMT-02).
func checkFixedPoint(t *testing.T, name string, after []byte) {
	t.Helper()
	if again := formatted(t, name, after); !bytes.Equal(again, after) {
		t.Errorf("%s: after is not a fixed point of the formatter (API.md M5, M6):\n%s\n---\n%s", name, after, again)
	}
}

// formatted is b in the canonical layout of its kind of file.
func formatted(t *testing.T, name string, b []byte) []byte {
	t.Helper()
	var fs source.FileSet
	src, err := fs.Add(name, "/"+name, b)
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(&fs, "")
	if path.Ext(name) == ".json" {
		root, err := jsonsrc.Parse(src, bag)
		if err != nil {
			t.Fatalf("%s does not parse: %v", name, err)
		}
		return jsonsrc.Format(root)
	}
	kind := syntax.FileSource
	if path.Base(name) == "project.canon" {
		kind = syntax.FileProject
	}
	out, err := format.Source(src, kind, bag)
	if err != nil {
		t.Fatalf("%s does not format: %v", name, err)
	}
	return out
}

// hunk is a run of changed lines: before's lines b0 to b1 became after's a0 to a1.
type hunk struct{ b0, b1, a0, a1 int }

// lineHunks are the changed runs of a longest-common-subsequence line diff.
func lineHunks(before, after []string) []hunk {
	n, m := len(before), len(after)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if before[i] == after[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []hunk
	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && before[i] == after[j] {
			i, j = i+1, j+1
			continue
		}
		h := hunk{b0: i, a0: j}
		for (i < n || j < m) && !(i < n && j < m && before[i] == after[j]) {
			if j >= m || i < n && lcs[i+1][j] >= lcs[i][j+1] {
				i++
			} else {
				j++
			}
		}
		h.b1, h.a1 = i, j
		out = append(out, h)
	}
	return out
}

func lines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// checkSteps is the second half of API.md M6, over every change kind: the plan's writes of the
// file chain from Before to After, and each keeps every byte outside its regions.
func checkSteps(t *testing.T, plan *edit.Plan, ch edit.Change, oneLine bool) {
	t.Helper()
	steps := edit.Writes(plan, ch.Path)
	if len(steps) == 0 {
		if ch.Kind != edit.ChangeRenamed || !bytes.Equal(ch.Before, ch.After) || oneLine {
			t.Errorf("%s: a change of kind %d with no write recorded", ch.Path, ch.Kind)
		}
		return
	}
	if ch.Kind != edit.ChangeCreated && !bytes.Equal(steps[0].Before, ch.Before) {
		t.Errorf("%s: the first write does not start from Before", ch.Path)
	}
	for i, s := range steps {
		if i > 0 && !bytes.Equal(steps[i-1].After, s.Before) {
			t.Errorf("%s: write %d does not start where write %d ended", ch.Path, i, i-1)
		}
		checkRegions(t, ch.Path, s)
	}
	last := steps[len(steps)-1]
	if !bytes.Equal(last.After, ch.After) {
		t.Errorf("%s: the last write does not end at After", ch.Path)
	}
	if oneLine {
		checkOneLine(t, ch.Path, last)
	}
}

// checkOneLine is API.md M6: each one-value Set of a scalar changes exactly one line, and
// Apply re-prints no more than that line.
func checkOneLine(t *testing.T, name string, w edit.Write) {
	t.Helper()
	for _, h := range lineHunks(lines(w.Before), lines(w.After)) {
		if h.b1-h.b0 != 1 || h.a1-h.a0 != 1 {
			t.Errorf("%s: a one-value Set changed lines %+v, want one line (API.md M6)", name, h)
		}
	}
	for _, r := range regionLines(w) {
		if r[1]-r[0] != 1 {
			t.Errorf("%s: a one-value Set re-printed lines %d to %d, want one line (API.md M6)", name, r[0]+1, r[1])
		}
	}
}

// regionLines are w's regions as lines of its before, from the first to past the last.
func regionLines(w edit.Write) [][2]int {
	out := make([][2]int, len(w.Regions))
	for i, r := range w.Regions {
		lo := bytes.Count(w.Before[:r.Lo], []byte("\n"))
		hi := bytes.Count(w.Before[:max(r.Hi-1, r.Lo)], []byte("\n"))
		out[i] = [2]int{lo, hi + 1}
	}
	return out
}
