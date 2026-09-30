package format_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// rewriteChecked is Rewrite of changes on f, failing when judged around the changed bytes it
// differs from Rewrite judged on the whole file: the same bytes, or the same refusal.
func rewriteChecked(t *testing.T, path string, f *syntax.File, changes []format.Change) ([]byte, error) {
	// FORMATTER.md §13, API.md M5, M6
	t.Helper()
	out, err := format.Rewrite(f, changes)
	want, werr := format.RewriteWhole(f, changes)
	if (err == nil) != (werr == nil) || err != nil && err.Error() != werr.Error() {
		t.Fatalf("%s: Rewrite refused with %v, the whole-file Rewrite with %v", path, err, werr)
	}
	if !bytes.Equal(out, want) {
		t.Fatalf("%s: Rewrite differs from the whole-file Rewrite:\n%s", path, lineDiff(want, out))
	}
	return out, err
}

// The table of monsterText: its rows, one in every tableGap after a blank line and a comment,
// one in every tableBroken written one field per line, one in every tableNoted with a comment.
const (
	tableRows, tableGap, tableBroken, tableNoted = 150, 23, 31, 17
	tableEdits, tableSeed, tableLevels, tableHP  = 160, 7, 200, 100000
	tableWide                                    = 70 // a name this long breaks its row
)

// monsterText is the shape of the benchmark's monster.canon, small, with the comments, blank
// lines and broken rows a table may hold, in canonical layout.
func monsterText(t testing.TB) []byte {
	// IMPLEMENTATION-PLAN §7.6
	t.Helper()
	var b strings.Builder
	b.WriteString("/// Monsters.\npackage monster\n\n/// A monster.\nrecord Monster {\n  /// Its name.\n")
	b.WriteString("  name: String(1..)\n  /// Its level.\n  level: Int(1..=200)\n  /// Its hit points.\n")
	b.WriteString("  hp: Int(1..=100000)\n}\n\nlet monsters: table Monster = {\n")
	for i := range tableRows {
		if i%tableGap == tableGap-1 {
			fmt.Fprintf(&b, "\n  // group %d\n", i)
		}
		key, level, hp := fmt.Sprintf("MON_%05d", i+1), 1+i%tableLevels, 1+i*i%tableHP
		switch {
		case i%tableBroken == tableBroken-1:
			fmt.Fprintf(&b, "  %s {\n    name: \"m%d\"\n    level: %d\n    hp: %d\n  }\n", key, i, level, hp)
		case i%tableNoted == tableNoted-1:
			fmt.Fprintf(&b, "  %s { name: \"m%d\", level: %d, hp: %d } // noted\n", key, i, level, hp)
		default:
			fmt.Fprintf(&b, "  %s { name: \"m%d\", level: %d, hp: %d }\n", key, i, level, hp)
		}
	}
	b.WriteString("}\n")
	out, err := formatText(t, "monster/monster.canon", []byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// valueText is a new text for node n: another number, a string short or wide enough to break
// its row, a record written loosely, or n's own text.
func valueText(r *rand.Rand, f *syntax.File, n syntax.Node) string {
	switch n.Kind() {
	case syntax.KindIntLit:
		return fmt.Sprint(r.IntN(tableHP))
	case syntax.KindStringLit:
		return fmt.Sprintf("%q", strings.Repeat("w", r.IntN(tableWide)))
	case syntax.KindBraceLit:
		return fmt.Sprintf("{name:%q,level:%d,hp:%d}", strings.Repeat("v", r.IntN(tableWide)), r.IntN(tableLevels), r.IntN(tableHP))
	}
	s := f.Span(n)
	return string(f.Src.Content[s.Start:s.End])
}

// On a table in canonical layout, Rewrite of random values, one or two at a time, is byte for
// byte the whole-file Rewrite, and a fixed point.
func TestRewriteAroundTable(t *testing.T) {
	// FORMATTER.md §13, API.md M5, M6
	data := monsterText(t)
	f := parse(t, "monster/monster.canon", data).file
	var ns []syntax.Node
	for _, n := range items(f) {
		if f.Span(n).Start > f.Span(f.Decls[1]).Start {
			ns = append(ns, n)
		}
	}
	r := rand.New(rand.NewPCG(tableSeed, tableSeed))
	around := 0
	for range tableEdits {
		changes := []format.Change{{Kind: format.Replace, Node: ns[r.IntN(len(ns))]}}
		if r.IntN(tableGap) == 0 {
			changes = append(changes, format.Change{Kind: format.Replace, Node: ns[r.IntN(len(ns))]})
		}
		for i := range changes {
			changes[i].Text = valueText(r, f, changes[i].Node)
		}
		out, err := rewriteChecked(t, "monster/monster.canon", f, changes)
		if err != nil {
			continue
		}
		if again, err := formatText(t, "monster/monster.canon", out); err != nil || !bytes.Equal(again, out) {
			t.Fatalf("not a fixed point (API.md M5): %v\n%s", err, lineDiff(out, again))
		}
		if format.SettledAround(f, out) {
			around++
		}
	}
	t.Logf("%d of %d judged around their changes", around, tableEdits)
	if around < tableEdits/2 {
		t.Errorf("%d of %d Rewrites judged around their changes, fewer than half", around, tableEdits)
	}
}

// The least number of sections TestSectionsPrintAlone checks.
const floorSections = 1500

// In a fixed point, each section a broken list or the file prints alone, printed alone, is its
// own bytes: the ground on which Rewrite judges a change around it.
func TestSectionsPrintAlone(t *testing.T) {
	// FORMATTER.md §7.1, §13
	total := 0
	inputs := append(exampleFiles(t), corpusWants(t)...)
	for _, ex := range append(inputs, example{path: "monster/monster.canon", data: monsterText(t)}) {
		checked, bad := format.SectionsAlone(parse(t, ex.path, ex.data).file)
		if len(bad) > 0 {
			t.Errorf("%s: the sections at %v do not print alone as their bytes", ex.path, bad)
		}
		total += checked
	}
	if total < floorSections {
		t.Errorf("%d sections checked, fewer than %d", total, floorSections)
	}
}

// On the examples and the corpus, a Replace of a sampled node by a string or a number, or by
// its own text written loosely, is byte for byte the whole-file Rewrite.
func TestRewriteAroundExamples(t *testing.T) {
	// FORMATTER.md §13, API.md M5
	r := rand.New(rand.NewPCG(tableSeed, tableSeed))
	for k, ex := range append(exampleFiles(t), corpusWants(t)...) {
		f := parse(t, ex.path, ex.data).file
		for i, n := range items(f) {
			if (i+k)%every(insertEvery) != 0 {
				continue
			}
			c := format.Change{Kind: format.Replace, Node: n, Text: valueText(r, f, n)}
			rewriteChecked(t, ex.path, f, []format.Change{c})
		}
	}
}

// aroundEdges are texts whose sections end at their edges: commas kept on their own line, a
// list's closing comments, the file's closing comments after its last declaration, members and
// cases named by reserved words, whose commas depend on their neighbours.
var aroundEdges = []string{
	keptDoc, keptAfter, statuses,
	"package p\n\nlet t: table T = {\n  a { x: 1 }\n  b { x: 2 } // two\n  // closing\n}\n// end\n",
	"package p\n\nconst A = 1 /* a\n  b */\n\n/// B.\nconst B = [\n  1, // one\n  2,\n]\n",
	"package p\n\nenum K2 {\n  a,\n  in,\n  b\n}\n",
	"package p\n\nenum K {\n  plain\n  first,\n  and,\n  or,\n  not,\n  in,\n  is,\n  else,\n  where,\n  as,\n  last\n}\nvariant C {\n  a,\n  where,\n  b\n}\n",
}

// wordTexts are names a member or a case may take: reserved words that make or drop the comma
// of the item before or after them, and a plain word.
var wordTexts = []string{"in", "not", "as", "where", "plain"}

// At the edges of sections, every node replaced by a number, a word or its own text written on
// one line gives the whole-file Rewrite's bytes.
func TestRewriteAroundEdges(t *testing.T) {
	// FORMATTER.md §13, DECISIONS 211, 216, API.md M5
	r := rand.New(rand.NewPCG(tableSeed, tableSeed))
	for _, src := range aroundEdges {
		data, err := formatText(t, "a/a.canon", []byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		f := parse(t, "a/a.canon", data).file
		for _, n := range items(f) {
			rewriteChecked(t, "a/a.canon", f, []format.Change{{Kind: format.Replace, Node: n, Text: valueText(r, f, n)}})
			if flat, err := format.Flat(f, n); err == nil {
				rewriteChecked(t, "a/a.canon", f, []format.Change{{Kind: format.Replace, Node: n, Text: string(flat)}})
			}
			for _, w := range wordTexts {
				rewriteChecked(t, "a/a.canon", f, []format.Change{{Kind: format.Replace, Node: n, Text: w}})
			}
		}
	}
}

// looseAround are texts in which the section of the item holding the last number is not its own
// bytes printed alone: the item written loosely, a comment ending the line before it that the
// printer moves onto its own line, an item sharing the line of the one before.
var looseAround = []string{
	"package p\n\nlet t: table T = {\n  a { x: 1 }\n  b {x: 2}\n}\n",
	"package p\n\nlet t: table T = {\n  a { x: 1 } /* c\n  d */\n  b { x: 2 }\n}\n",
	"package p\n\nlet t: table T = {\n  a { x: 1 } b { x: 2 }\n}\n",
}

// Judged around its change, a text whose section of f does not print alone as f's own bytes is
// refused, left to the whole-file settle: f's side is the ground of that judgement.
func TestAroundNeedsItsGround(t *testing.T) {
	// FORMATTER.md §7.1, §13, API.md M5
	for _, src := range looseAround {
		f := parse(t, "a/a.canon", []byte(src)).file
		at := bytes.LastIndex(f.Src.Content, []byte("2"))
		content := slices.Concat(f.Src.Content[:at], []byte("3"), f.Src.Content[at+1:])
		if format.SettledAround(f, content) {
			t.Errorf("%q: judged around a section of f that is not its own bytes", src)
		}
	}
}

// Canonical judges a file as a fresh look does, a fixed point or not, refused or not, and keeps
// that judgement for the file.
func TestCanonicalKept(t *testing.T) {
	// API.md M9, FORMATTER.md §13
	inputs := []example{
		{path: "a/cr.canon", data: []byte("package a\r\n\r\nconst A = 1\r\n")},
		{path: "a/lone.canon", data: []byte("package a\n\nconst A = 1\r\n")},
		{path: "a/tab.canon", data: []byte("package a\n\nenum E {\n\tA\n}\n")},
		{path: "a/bad.canon", data: []byte("package a\n\nconst = 1\n")},
		{path: "a/lex.canon", data: []byte("package a\n\nconst A = \"\\q\"\n")},
		{path: "a/loose.canon", data: []byte("package a\nconst   A=1\n")},
	}
	inputs = append(append(inputs, corpusCases(t)...), exampleFiles(t)...)
	for _, in := range inputs {
		f := parse(t, in.path, in.data).file
		want, wantErr := freshLayout(t, f)
		got, err := format.Canonical(f)
		if got != want || !errors.Is(err, wantErr) || (err == nil) != (wantErr == nil) || !format.Judged(f) {
			t.Fatalf("%s: Canonical %v, %v; a fresh look %v, %v", in.path, got, err, want, wantErr)
		}
		if again, err := format.Canonical(f); again != got || !errors.Is(err, wantErr) {
			t.Fatalf("%s: judged %v then %v", in.path, got, again)
		}
	}
}

// freshLayout is a fresh look at f: Rewrite's refusal of it, else whether it is a fixed point,
// the text canon fmt prints for it being the text itself.
func freshLayout(t *testing.T, f *syntax.File) (bool, error) {
	t.Helper()
	if _, err := format.RewriteWhole(f, nil); err != nil {
		return false, err
	}
	out, err := formatText(t, f.Src.Path, f.Src.Content)
	return err == nil && bytes.Equal(out, f.Src.Content), nil
}

// corpusCases are the inputs of the corpus, most of them not in canonical layout.
func corpusCases(t testing.TB) []example {
	t.Helper()
	cases, err := golden.Load("testdata/fmt/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	out := make([]example, 0, len(cases))
	for _, c := range cases {
		out = append(out, example{path: c.Archive.Files[0].Name, data: c.Archive.Files[0].Data})
	}
	return out
}

// Rewrite and Canonical from many goroutines share the layout cache.
func TestCanonicalConcurrent(t *testing.T) {
	// DOCTRINE §5, API.md M9
	data := monsterText(t)
	f := parse(t, "monster/monster.canon", data).file
	n := f.Decls[1].(*syntax.LetDecl).Value.(*syntax.BraceLit).Items[0]
	var wg sync.WaitGroup
	for range tableGap {
		wg.Go(func() {
			if ok, err := format.Canonical(f); !ok || err != nil {
				t.Errorf("Canonical: %v, %v", ok, err)
			}
			if _, err := format.Rewrite(f, []format.Change{{Kind: format.Replace, Node: n, Text: "MON_00001 { name: \"x\", level: 1, hp: 1 }"}}); err != nil {
				t.Errorf("Rewrite: %v", err)
			}
		})
	}
	wg.Wait()
}

// BenchmarkRewriteTable is a one-value Set in a table: Rewrite around the value and the
// whole-file Rewrite.
func BenchmarkRewriteTable(b *testing.B) {
	// IMPLEMENTATION-PLAN §7.6
	f := parse(b, "monster/monster.canon", monsterText(b)).file
	row := f.Decls[1].(*syntax.LetDecl).Value.(*syntax.BraceLit).Items[tableRows/2].(*syntax.EntryItem)
	level := row.Value.Items[1].(*syntax.FieldItem).Value
	changes := []format.Change{{Kind: format.Replace, Node: level, Text: "77"}}
	for _, run := range []struct {
		name    string
		rewrite func(*syntax.File, []format.Change) ([]byte, error)
	}{{"around", format.Rewrite}, {"whole", format.RewriteWhole}} {
		b.Run(run.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := run.rewrite(f, changes); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
