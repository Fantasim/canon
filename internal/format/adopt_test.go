package format_test

import (
	"bytes"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	monsterPath  = "monster/monster.canon" // the benchmark-shaped file
	adoptSamples = 6                       // the nodes of each canonical input the test replaces
	layoutParts  = 2                       // what a layout holds: the refusal and the fixed-point flag
)

// An adopted tree gives what a fresh tree of its bytes gives, Canonical and Rewrite against RewriteWhole (API.md M9, M5).
func TestAdopt(t *testing.T) {
	if got := format.LayoutFields(); got != layoutParts {
		t.Fatalf("a layout holds %d parts: Adopt sets %d", got, layoutParts)
	}
	inputs := append(append([]example{{path: monsterPath, data: monsterText(t)}}, corpusCases(t)...), exampleFiles(t)...)
	adopted := 0
	for _, in := range inputs {
		fresh := parse(t, in.path, in.data).file
		if want, err := freshLayout(t, fresh); err != nil || !want {
			continue
		}
		adopted++
		f := parse(t, in.path, in.data).file
		format.Adopt(f)
		if !format.Judged(f) {
			t.Fatalf("%s: Adopt kept no layout", in.path)
		}
		format.Adopt(f) // one held is left alone
		got, err := format.Canonical(f)
		if !got || err != nil {
			t.Fatalf("%s: Canonical after Adopt %v, %v", in.path, got, err)
		}
		for _, change := range adoptChanges(in.path) {
			out, err := format.Rewrite(f, change.on(f))
			want, werr := format.RewriteWhole(fresh, change.on(fresh))
			if !sameOutcome(out, err, want, werr) {
				t.Fatalf("%s: Rewrite after Adopt %q, %v; the whole-file Rewrite of a fresh tree %q, %v", in.path, out, err, want, werr)
			}
		}
	}
	if adopted == 0 {
		t.Fatal("no canonical input")
	}
}

// Adopt leaves a layout already judged as it is (API.md M9).
func TestAdoptKeepsJudged(t *testing.T) {
	f := parse(t, "a/loose.canon", []byte("package a\nconst   A=1\n")).file
	if got, _ := format.Canonical(f); got {
		t.Fatal("a loose file is canonical")
	}
	format.Adopt(f)
	if got, err := format.Canonical(f); got || err != nil {
		t.Errorf("Adopt replaced a layout held: %v, %v", got, err)
	}
}

// adoptChange is a Replace of the node pick finds, by its own text when text is empty.
type adoptChange struct {
	pick func(*syntax.File) syntax.Node
	text string
}

// on is the change on the nodes of f.
func (c adoptChange) on(f *syntax.File) []format.Change {
	n := c.pick(f)
	text := c.text
	if text == "" {
		s := f.Span(n)
		text = string(f.Src.Content[s.Start:s.End])
	}
	return []format.Change{{Kind: format.Replace, Node: n, Text: text}}
}

// adoptChanges are adoptSamples Replaces spread over the nodes of a file and, in the benchmark's
// file, a new level for a row of its table.
func adoptChanges(path string) []adoptChange {
	var out []adoptChange
	for i := range adoptSamples {
		out = append(out, adoptChange{pick: func(g *syntax.File) syntax.Node {
			ns := items(g)
			return ns[len(ns)*i/adoptSamples]
		}})
	}
	if path == monsterPath {
		out = append(out, adoptChange{text: "77", pick: func(g *syntax.File) syntax.Node {
			row := g.Decls[1].(*syntax.LetDecl).Value.(*syntax.BraceLit).Items[tableRows/2].(*syntax.EntryItem)
			return row.Value.Items[1].(*syntax.FieldItem).Value
		}})
	}
	return out
}

// sameOutcome is two Rewrites giving the same bytes, or both refusing alike.
func sameOutcome(out []byte, err error, want []byte, werr error) bool {
	if err != nil || werr != nil {
		return err != nil && werr != nil && err.Error() == werr.Error()
	}
	return bytes.Equal(out, want)
}
