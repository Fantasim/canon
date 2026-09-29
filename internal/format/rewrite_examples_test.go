package format_test

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// items are the nodes of f a Change may remove: the ones Rewrite accepts as items.
func items(f *syntax.File) []syntax.Node {
	var out []syntax.Node
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil && n != syntax.Node(f) && n.Kind() != syntax.KindDocComment && n.First() <= n.Last() {
			out = append(out, n)
		}
		return true
	})
	return out
}

// topOf is the byte range a change to n may touch: n's top-level declaration, with the blank
// line and the comments before it, and the blank line after it (API.md M6).
func topOf(f *syntax.File, n syntax.Node) (lo, hi int) {
	tops := slices.Concat(nodes(f.Decls), nodes(f.Amends), nodes(f.Entries))
	if f.Project != nil {
		tops = append(tops, f.Project)
	}
	at := f.Span(n).Start
	for _, d := range tops {
		s := f.Span(d)
		if s.Start <= at && at < s.End {
			lo, hi = int(s.Start), int(s.End)
			for _, tr := range f.Tokens[d.First()].Leading {
				lo = min(lo, int(tr.Start))
			}
			return max(lo-1, 0), hi + len("\n\n")
		}
	}
	return 0, len(f.Src.Content)
}

func nodes[N syntax.Node](ns []N) []syntax.Node {
	out := make([]syntax.Node, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// checkRewrite checks API.md M5 and M6 on one Rewrite of an example: a fixed point, which
// differs from the example only inside the declaration holding n. It returns ErrChange and
// ErrText, which the caller judges; any other error fails the test.
func checkRewrite(t *testing.T, ex example, f *syntax.File, n syntax.Node, c format.Change) error {
	t.Helper()
	out, err := format.Rewrite(f, []format.Change{c})
	if errors.Is(err, format.ErrChange) || errors.Is(err, format.ErrText) {
		return err
	}
	if err != nil {
		t.Fatalf("%s: change %d at %v: %v", ex.path, c.Kind, f.Span(n), err)
	}
	if again, err := formatText(t, ex.path, out); err != nil || !bytes.Equal(again, out) {
		t.Fatalf("%s: change %d at %v: not a fixed point: %v\n%s", ex.path, c.Kind, f.Span(n), err, lineDiff(out, again))
	}
	lo, hi := topOf(f, n)
	before := ex.data
	hi = min(hi, len(before))
	if !bytes.HasPrefix(out, before[:lo]) || !bytes.HasSuffix(out[lo:], before[hi:]) {
		t.Fatalf("%s: change %d at %v touched bytes outside %d to %d:\n%s", ex.path, c.Kind, f.Span(n), lo, hi, lineDiff(before, out))
	}
	var gone []syntax.Tok
	if c.Kind == format.Remove {
		gone = []syntax.Tok{n.First(), separatorOf(f, n)}
	}
	if lost := lostComments(f, parse(t, ex.path, out).file, gone); len(lost) > 0 {
		t.Fatalf("%s: change %d at %v lost comments, or moved them to another token: %q (API.md M6)", ex.path, c.Kind, f.Span(n), lost)
	}
	return nil
}

// separatorOf is the last token of item n, or the comma after it.
func separatorOf(f *syntax.File, n syntax.Node) syntax.Tok {
	next := int(n.Last()) + 1
	for next < len(f.Tokens)-1 && f.Tokens[next].Kind == syntax.TokNL {
		next++
	}
	if f.Tokens[next].Kind == syntax.TokComma {
		return syntax.Tok(next)
	}
	return n.Last()
}

// lostComments are the comments of before that after lacks, each with the text of the token the
// formatter attaches it to, but for the ones attached to a token of gone, a removed item's
// (log-2026-09-29 M4 U1r).
func lostComments(before, after *syntax.File, gone []syntax.Tok) []string {
	have := map[string]int{}
	for _, c := range hostedComments(after, nil, true) {
		have[c]++
	}
	var lost []string
	for _, c := range hostedComments(before, gone, false) {
		if have[c] == 0 {
			lost = append(lost, c)
		}
		have[c]--
	}
	return lost
}

// hostedComments are f's comments as "token => comment", but for those attached to gone[0]
// to gone[1]; after, each also as " => comment", which one a kept comma holds is alone: a
// DECISIONS 216 comma is kept for the item after it, and removing that item frees it.
func hostedComments(f *syntax.File, gone []syntax.Tok, after bool) []string {
	hosts, kept := format.CommentHosts(f), format.KeptCommas(f)
	var out []string
	for _, tok := range f.Tokens {
		for _, tr := range slices.Concat(tok.Leading, tok.Trailing) {
			s, ok := commentOf(f, tr)
			host, attached := hosts[int(tr.Start)]
			if !ok || !attached || len(gone) > 0 && gone[0] <= host && host <= gone[1] {
				continue
			}
			ht := f.Tokens[host]
			switch {
			case after:
				out = append(out, string(f.Src.Content[ht.Start:ht.End])+" => "+s, " => "+s)
			case kept[host]: // a DECISIONS 216 comma the settle may drop: its comment by text only
				out = append(out, " => "+s)
			default:
				out = append(out, string(f.Src.Content[ht.Start:ht.End])+" => "+s)
			}
		}
	}
	return out
}

// full runs the property tests on every node and position rather than on a sample.
var full = flag.Bool("format.full", false, "run the format property tests unsampled")

// every is the sampling step of a property test: step by default, 1 with -format.full.
func every(step int) int {
	if *full {
		return 1
	}
	return step
}

// The sampling steps of TestRewriteExamples, and the least it must rewrite with them: a Remove
// of every item sampled, a Replace of every sampled node an item holds, an Insert in every
// sampled list literal (log-2026-09-29 M4 U1r).
const (
	removeEvery, insertEvery                   = 43, 9
	floorRemoved, floorReplaced, floorInserted = 55, 250, 80
)

// rewrites counts the Rewrites TestRewriteExamples checked.
type rewrites struct{ removed, replaced, inserted int }

// FORMATTER.md §13, API.md M5 and M6 on the examples and the corpus: checkNode, insertCopies.
func TestRewriteExamples(t *testing.T) {
	var got rewrites
	for k, ex := range append(exampleFiles(t), corpusWants(t)...) {
		f := parse(t, ex.path, ex.data).file
		for i, n := range items(f) {
			if (i+k)%every(insertEvery) == 0 {
				got.inserted += insertCopies(t, ex, f, n)
			}
			if (i+k)%every(removeEvery) != 0 {
				continue
			}
			removed, replaced := checkNode(t, ex, f, n)
			got.removed += removed
			got.replaced += replaced
		}
	}
	t.Logf("%+v", got)
	if !*full && (got.removed < floorRemoved || got.replaced < floorReplaced || got.inserted < floorInserted) {
		t.Errorf("%+v: fewer rewrites than %d removed, %d replaced, %d inserted", got, floorRemoved, floorReplaced, floorInserted)
	}
}

// checkNode removes n, then writes its single-line text over it, and counts the Rewrites made:
// a Remove is refused for a node no list or file holds (ErrChange) or one the grammar needs, as
// the only argument of @codes(T) (ErrText); a Replace only in the header.
func checkNode(t *testing.T, ex example, f *syntax.File, n syntax.Node) (removed, replaced int) {
	t.Helper()
	if checkRewrite(t, ex, f, n, format.Change{Kind: format.Remove, Node: n}) == nil {
		removed = 1
	}
	flat, err := format.Flat(f, n)
	if err != nil {
		return removed, 0
	}
	lo, hi := topOf(f, n)
	err = checkRewrite(t, ex, f, n, format.Change{Kind: format.Replace, Node: n, Text: string(flat)})
	if err != nil && (lo != 0 || hi != len(f.Src.Content)) {
		t.Fatalf("%s: a Replace at %v inside a declaration was refused: %v", ex.path, f.Span(n), err)
	}
	if err == nil {
		replaced = 1
	}
	return removed, replaced
}

// insertCopies inserts the single-line text of the first item of list literal n first and
// last in it, and reports how many of these Rewrites were checked; each must be.
func insertCopies(t *testing.T, ex example, f *syntax.File, n syntax.Node) int {
	t.Helper()
	var first syntax.Node
	count := 0
	switch l := n.(type) {
	case *syntax.BraceLit:
		if len(l.Items) == 0 || len(l.Clauses) > 0 {
			return 0
		}
		first, count = l.Items[0], len(l.Items)
	case *syntax.ListLit:
		if len(l.Elems) == 0 {
			return 0
		}
		first, count = l.Elems[0], len(l.Elems)
	default:
		return 0
	}
	flat, err := format.Flat(f, first)
	if err != nil {
		return 0
	}
	positions := []int{0, count}
	for _, at := range positions {
		if err := checkRewrite(t, ex, f, n, format.Change{Kind: format.Insert, List: n.First(), At: at, Text: string(flat)}); err != nil {
			t.Fatalf("%s: an Insert in the list at %v was refused: %v", ex.path, f.Span(n), err)
		}
	}
	return len(positions)
}

// corpusWants are the expected outputs of the corpus, fixed points (FORMATTER.md §12).
func corpusWants(t testing.TB) []example {
	t.Helper()
	cases, err := golden.Load("testdata/fmt/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]example, 0, len(cases))
	for _, c := range cases {
		want, err := c.Want()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, example{path: c.Archive.Files[0].Name, data: want})
	}
	return out
}
