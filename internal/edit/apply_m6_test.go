package edit_test

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
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
// file chain from Before to After, each keeps every byte outside its regions, and each for
// which oneLine holds changes one line.
func checkSteps(t *testing.T, plan *edit.Plan, ch edit.Change, oneLine func(edit.Write) bool) {
	t.Helper()
	steps := edit.Writes(plan, ch.Path)
	if len(steps) == 0 {
		if ch.Kind != edit.ChangeRenamed || !bytes.Equal(ch.Before, ch.After) {
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
		if oneLine(s) {
			checkOneLine(t, ch.Path, s)
		}
	}
	if last := steps[len(steps)-1]; !bytes.Equal(last.After, ch.After) {
		t.Errorf("%s: the last write does not end at After", ch.Path)
	}
}

// checkOneLine is API.md M6: a one-value Set of a scalar changes exactly one line, and Apply
// re-prints no more than that line.
func checkOneLine(t *testing.T, name string, w edit.Write) {
	t.Helper()
	if fault := oneLineFault(name, w); fault != "" {
		t.Errorf("%s: %s (API.md M5, M6)", name, fault)
	}
	for _, r := range regionLines(w) {
		if r[1]-r[0] != 1 {
			t.Errorf("%s: a one-value Set re-printed lines %d to %d, want one line (API.md M6)", name, r[0]+1, r[1])
		}
	}
}

// oneLineFault is why w breaks M6's one line, "" when it keeps it: its one line of before
// becomes one line, or several only when its single-line form passes the formatter's width
// (API.md M5 over M6, log-2026-09-29 M4 B3-r2).
func oneLineFault(name string, w edit.Write) string {
	h := lineHunks(lines(w.Before), lines(w.After))
	switch {
	case len(h) != 1 || h[0].b1-h[0].b0 != 1 || h[0].a1 == h[0].a0:
		return fmt.Sprintf("a one-value Set changed lines %+v, want one line", h)
	case h[0].a1-h[0].a0 == 1:
		return ""
	}
	single, ok := singleLine(name, w.After, h[0])
	switch {
	case !ok:
		return fmt.Sprintf("a one-value Set broke lines %+v into no item printable on one line", h)
	case utf8.RuneCountInString(single) <= format.Width:
		return fmt.Sprintf("a one-value Set broke a line that fits the width: %q", single)
	}
	return ""
}

// singleLine is the one line hunk h of after would be without M5's break: the outermost node
// its lines hold printed flat, with the text around it on its first and last lines. A JSON
// source has no such break.
func singleLine(name string, after []byte, h hunk) (string, bool) {
	if path.Ext(name) == ".json" {
		return "", false
	}
	var fs source.FileSet
	src, err := fs.Add(name, "/"+name, after)
	if err != nil {
		return "", false
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, ""))
	starts := lineStarts(after)
	lo, hi := starts[h.a0], starts[h.a1]-1
	var best syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil || best != nil {
			return false
		}
		sp := f.Span(n)
		if int(sp.Start) >= lo && int(sp.End) <= hi && int(sp.Start) < starts[h.a0+1] && int(sp.End) > starts[h.a1-1] {
			best = n
		}
		return best == nil
	})
	if best == nil {
		return "", false
	}
	flat, err := format.Flat(f, best)
	if err != nil {
		return "", false
	}
	sp := f.Span(best)
	return string(after[lo:sp.Start]) + string(flat) + string(after[sp.End:hi]), true
}

// lineStarts are the offsets each line of b starts at, then len(b).
func lineStarts(b []byte) []int {
	out := []int{0}
	for i, c := range b {
		if c == '\n' {
			out = append(out, i+1)
		}
	}
	if out[len(out)-1] != len(b) {
		out = append(out, len(b))
	}
	return out
}

// API.md M5, M6 (log-2026-09-29 M4 B3-r2): a one-value Set may break its line only when the
// line with the new value in place passes the formatter's width.
func TestOneLineAllowsOnlyAWidthBreak(t *testing.T) {
	const before = "package p\n\nlet rows: table Row = {\n  a { count: 1, note: \"x\" }\n}\n"
	long := strings.Repeat("n", format.Width)
	for _, tc := range []struct {
		after string
		fits  bool
	}{
		{"package p\n\nlet rows: table Row = {\n  a { count: 1, note: \"y\" }\n}\n", true},
		{"package p\n\nlet rows: table Row = {\n  a {\n    count: 1\n    note: \"y\"\n  }\n}\n", false},
		{"package p\n\nlet rows: table Row = {\n  a {\n    count: 1\n    note: \"" + long + "\"\n  }\n}\n", true},
	} {
		fault := oneLineFault("p/p.canon", edit.Write{Before: []byte(before), After: []byte(tc.after)})
		if (fault == "") != tc.fits {
			t.Errorf("%q: fault %q, want one only for a needless break", tc.after, fault)
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

// scalarSet reports operation i of ops a one-value Set of a scalar, none included, replacing an
// item written on one line (API.md M6, M5): one of several lines is held to M6's regions only
// (log-2026-09-29 P20-r2).
func scalarSet(a *build.Analysis, ops []edit.Operation, i int) bool {
	return scalarValue(ops, i) && replacedOneLine(a, ops[i])
}

// replacedOneLine reports the item op replaces holding no line end in a's sources: a value
// written elsewhere (default, spread, layer) or an unreadable path is not one of several lines.
func replacedOneLine(a *build.Analysis, op edit.Operation) bool {
	path, err := edit.Parse(op.Path)
	if err != nil {
		return true
	}
	res, err := edit.Resolve(edit.NewSnapshot(a), path)
	if err != nil || res.Target == nil {
		return true
	}
	p := res.Target.Prov()
	if p == nil || (p.Kind != value.ProvLiteral && p.Kind != value.ProvJSON) {
		return true
	}
	src := a.Files().Content(p.Span.File)
	return p.Span.Start > p.Span.End || int(p.Span.End) > len(src) ||
		!bytes.Contains(src[p.Span.Start:p.Span.End], []byte("\n"))
}

// scalarValue reports operation i of ops a Set of one scalar value, whatever it replaces.
func scalarValue(ops []edit.Operation, i int) bool {
	if i < 0 || i >= len(ops) || ops[i].Kind != edit.OpSet {
		return false
	}
	switch v := ops[i].Value.(type) {
	case edit.FromJSON:
		b := bytes.TrimSpace(v)
		return len(b) > 0 && b[0] != '{' && b[0] != '['
	case edit.Source:
		return scalarSource.MatchString(strings.TrimSpace(string(v)))
	case edit.List, edit.Obj, edit.Map, edit.Case, nil:
		return false
	}
	return true
}

// scalarSource is a Canon scalar literal: a string, or one token without brackets or spaces.
var scalarSource = regexp.MustCompile(`^("([^"\\]|\\.)*"|[^\s{}\[\]"(),]+)$`)
