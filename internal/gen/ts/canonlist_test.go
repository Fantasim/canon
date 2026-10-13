package tsgen_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	bigCount    = 3000 // DECISIONS 370: tsc 5.9 and 7 fail from about a thousand elements
	smallCount  = 3
	excessProp  = "may only specify known properties|is not assignable to" // TS2353 on latest; TS2345 on 5.0, whose first line says "not assignable"
	wrongType   = "is not assignable to"                                   // TS2322, or TS2345 on 5.0
	bigFile     = "big/big.canon"
	bigHead     = "/// A big package.\npackage big\n\n/// Kinds.\nenum Kind { alpha, beta, gamma, delta }\n\n/// A row.\nrecord Row {\n  /// Label.\n  label: String\n  /// Kind.\n  kind: Kind\n}\n\n/// A listed record.\nrecord Rec {\n  /// Key.\n  name: String\n  /// Kind.\n  kind: Kind\n  /// A row.\n  of: ref rows\n}\n\n"
	bigTail     = "emit ts { out: \"@features/big/big.ts\" }\n"
	listCall    = "canonList<"
	constPrefix = "export const "
	lineFormat  = "big.ts(%d,"
	wantSep     = "|"
	rowOpen     = "$r({ "
	rowExcess   = "$r({ extra: 1, "
	labelRight  = `label: "x0"`
	labelWrong  = "label: 5"
	blockOpen   = " = {\n"
	blockClose  = "}\n\n"
	listOpen    = " = [\n"
	listClose   = "]\n\n"
	tableDecl   = "/// The rows.\nlet rows: stable table Row"
	keyedDecl   = "/// The keyed records.\nlet keyed: [Rec] keyed by name"
	listDecl    = "/// The records.\nlet recs: [Rec]"
	recMapDecl  = "/// A map of records.\nlet byName: {String: Rec}"
	lolDecl     = "/// Lists of lists.\nlet lol: [[ref rows]]"
	lolEntry    = "  [r%[1]d, r%[1]d],\n"
	scalarDecl  = "/// A map of scalars.\nlet counts: {ref rows: Int}"
	rowFormat   = "  r%[1]d { label: \"x%[1]d\", kind: %[2]s }\n"
	recFormat   = "  { name: \"n%[1]d\", kind: %[2]s, of: r%[1]d },\n"
	recMapEntry = "  \"k%[1]d\": { name: \"n%[1]d\", kind: %[2]s, of: r%[1]d },\n"
	scalarEntry = "  r%[1]d: %[1]d,\n"
)

var kindNames = []string{"alpha", "beta", "gamma", "delta"}

// part is one value of the big package: its declaration, its delimiters and the format of each of its entries.
type part struct{ decl, open, entry, closer string }

// section writes p with n entries.
func section(b *strings.Builder, n int, p part) {
	b.WriteString(p.decl + p.open)
	for i := range n {
		fmt.Fprintf(b, p.entry, i, kindNames[i%len(kindNames)])
	}
	b.WriteString(p.closer)
}

// bigSource is a package with a stable table, a keyed list, a plain list of records, a list of lists, a map of records and a map of scalars, each of n elements.
func bigSource(n int) string {
	var b strings.Builder
	b.WriteString(bigHead)
	section(&b, n, part{tableDecl, blockOpen, rowFormat, blockClose})
	section(&b, n, part{keyedDecl, listOpen, recFormat, listClose})
	section(&b, n, part{listDecl, listOpen, recFormat, listClose})
	section(&b, n, part{recMapDecl, blockOpen, recMapEntry, blockClose})
	section(&b, n, part{lolDecl, listOpen, lolEntry, listClose})
	section(&b, n, part{scalarDecl, blockOpen, scalarEntry, blockClose})
	b.WriteString(bigTail)
	return b.String()
}

// bigGenerated builds bigSource(n) and writes its TypeScript under dir; it returns the file and its text.
func bigGenerated(t *testing.T, dir string, n int) (path, text string) {
	t.Helper()
	w := buildWorld(t, []txtar.File{{Name: bigFile, Data: []byte(bigSource(n))}})
	if all, errors := w.findings(t); errors > 0 {
		t.Fatalf("does not build: %v", all)
	}
	outs := w.generate(t)
	paths := writeOutputs(t, dir, outs)
	if len(paths) != 1 {
		t.Fatalf("%d files, want 1", len(paths))
	}
	return paths[0], string(outs[0].Content)
}

// DECISIONS 370, CODEGEN.md §8.1: a table, a keyed list, a list of records, a list of lists and maps of thousands of elements type-check on every compiler: no union of the elements' literal types, so no TS2590.
func TestBigListsTypecheck(t *testing.T) {
	tc := tsc.Find(t)
	path, text := bigGenerated(t, t.TempDir(), bigCount)
	if !strings.Contains(text, listCall) {
		t.Fatal("the lists are not written through canonList")
	}
	typecheck(t, tc, path)
}

// breakage is a change to one section's first element and what tsc must say about that line.
type breakage struct{ section, old, new, want string }

var (
	excesses = []breakage{
		{"rows", rowOpen, rowExcess, excessProp},
		{"keyed", rowOpen, rowExcess, excessProp},
		{"recs", rowOpen, rowExcess, excessProp},
		{"byName", `$r(["k0", `, `$r(["k0", 1, `, wrongType},
		{"lol", `$r(["r0", `, `$r(["r0", "zz", `, wrongType},
		{"counts", `$r(["r0", `, `$r(["r0", 1, `, wrongType},
	}
	wrongs = []breakage{
		{"rows", labelRight, labelWrong, wrongType},
		{"keyed", `name: "n0"`, "name: 5", wrongType},
		{"recs", `name: "n0"`, "name: 5", wrongType},
		{"byName", `$r(["k0"`, "$r([5", wrongType},
		{"lol", `$r(["r0"`, "$r([5", wrongType},
		{"counts", `$r(["r0"`, "$r([5", wrongType},
	}
)

// apply breaks the first element of each section's declaration in text, and returns the broken text and the line of each break.
func apply(t *testing.T, text string, bs []breakage) (string, []int) {
	t.Helper()
	lines := strings.Split(text, "\n")
	var broken []int
	for _, b := range bs {
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, constPrefix+b.section+":") })
		for i >= 0 && i < len(lines) && lines[i] != "" && !strings.Contains(lines[i], b.old) {
			i++ // a table's rows are on the lines below its declaration
		}
		if i < 0 || i >= len(lines) || !strings.Contains(lines[i], b.old) {
			t.Fatalf("%s: no %q in its declaration", b.section, b.old)
		}
		lines[i] = strings.Replace(lines[i], b.old, b.new, 1)
		broken = append(broken, i+1)
	}
	return strings.Join(lines, "\n"), broken
}

// DECISIONS 370, CODEGEN.md §8.1: each element is still checked against the declared type on its own, in every kind of list (table, keyed list, list of records, map, list of lists): an excess property or member and a wrong type fail on the element's line.
func TestListElementsAreStillChecked(t *testing.T) {
	tc := tsc.Find(t)
	runs := []struct {
		name string
		bs   []breakage
	}{{"excess", excesses}, {"wrong type", wrongs}}
	for _, r := range runs {
		name, bs := r.name, r.bs
		path, text := bigGenerated(t, t.TempDir(), smallCount)
		broken, lines := apply(t, text, bs)
		writeFile(t, path, []byte(broken))
		for _, comp := range tc.Compilers {
			args := append([]string{comp.Script, "--noEmit"}, tsc.Args(tc.Typings, path)...)
			out, err := run(tc, args, "")
			if err == nil {
				t.Errorf("%s on %s: tsc passes", name, comp.Name)
			}
			expectErrors(t, name+" on "+comp.Name, out, bs, lines)
		}
	}
}

// expectErrors requires tsc's output to report each breakage on one line: the file's line number, then one of the alternatives (separated by "|") of the message.
func expectErrors(t *testing.T, label, out string, bs []breakage, lines []int) {
	t.Helper()
	for i, b := range bs {
		at := fmt.Sprintf(lineFormat, lines[i])
		if !slices.ContainsFunc(strings.Split(out, "\n"), func(l string) bool { return strings.Contains(l, at) && containsAny(l, strings.Split(b.want, wantSep)) }) {
			t.Errorf("%s: no %q error on line %d (%s):\n%s", label, b.want, lines[i], b.section, out)
		}
	}
}

// containsAny reports a line holding one of words.
func containsAny(line string, words []string) bool {
	return slices.ContainsFunc(words, func(w string) bool { return strings.Contains(line, w) })
}
