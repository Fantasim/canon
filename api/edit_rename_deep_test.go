package canon_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// deepCase is aa's elements holding depth levels of keyed lists or tables below them, each
// level's field optional or not: oo holds S entries, S's inn holds T entries, T's dd holds U's.
type deepCase struct {
	name     string
	depth    int
	table    bool
	optional bool
}

// deepFields, deepRecords and deepKeys are each level's field, entry record and two keys: the
// first key of every level is on the renamed entry's path, the second its sibling's.
var (
	deepFields  = []string{"oo", "inn", "dd"}
	deepRecords = []string{"S", "T", "U"}
	deepKeys    = [][2]string{{"x", "z"}, {"q", "r"}, {"d", "e"}}
)

// deepRead is a let reading through aa: the element named head, then at each level the key of
// index idx[i]; dyn is the level whose key is a dynamic index (-1: none), 0 being aa's own; every
// writes `?.` before each field below an entry too, as `aa.second.oo?.x?.inn?.q` (W3401, still static).
type deepRead struct {
	name, head string
	idx        []int
	dyn        int
	every      bool
}

// deepGen writes the fixture of c in which the entry at level ren of second's path has key nk.
type deepGen struct {
	c   deepCase
	ren int
	nk  string
}

// key is the key of index j at level i (1-based), on the renamed path or not.
func (g deepGen) key(onPath bool, i, j int) string {
	if onPath && j == 0 && i == g.ren {
		return g.nk
	}
	return deepKeys[i-1][j]
}

// collType is the type of level i's field.
func (g deepGen) collType(i int) string {
	t := "[" + deepRecords[i-1] + "] keyed by k"
	if g.c.table {
		t = "table " + deepRecords[i-1]
	}
	if g.c.optional {
		return "(" + t + ")?"
	}
	return t
}

// record is level i's entry record.
func (g deepGen) record(i int) string {
	out := "record " + deepRecords[i-1] + " {\n"
	if !g.c.table {
		out += "  k: String\n"
	}
	out += "  w: Int = 0\n"
	if i < g.c.depth {
		out += "  " + deepFields[i] + ": " + g.collType(i+1) + "\n"
	}
	return out + "}\n\n"
}

// literal is level i's collection literal, on the renamed path or not.
func (g deepGen) literal(onPath bool, i int) string {
	var entries []string
	for j := range deepKeys[i-1] {
		k, body := g.key(onPath, i, j), "w: 1"
		if i < g.c.depth {
			body += ", " + deepFields[i] + ": " + g.literal(onPath && j == 0, i+1)
		}
		if g.c.table {
			entries = append(entries, k+" { "+body+" }")
		} else {
			entries = append(entries, "{ k: \""+k+"\", "+body+" }")
		}
	}
	if g.c.table {
		return "{ " + strings.Join(entries, ", ") + " }"
	}
	return "[" + strings.Join(entries, ", ") + "]"
}

// expr is r's read expression; a dynamic level's key is a let `k<level>` holding the same key.
func (g deepGen) expr(r deepRead) string {
	out := "aa." + r.head
	if r.dyn == 0 {
		out = "aa[k0]"
	}
	dot := "."
	if g.c.optional {
		dot = "?."
	}
	onPath := r.head == "second"
	for i := 1; i <= g.c.depth; i++ {
		j := r.idx[i-1]
		if r.every && i > 1 {
			out += "?"
		}
		out += "." + deepFields[i-1]
		switch {
		case r.dyn == i && g.c.optional:
			out += "![k" + fmt.Sprint(i) + "]"
		case r.dyn == i:
			out += "[k" + fmt.Sprint(i) + "]"
		default:
			out += dot + g.key(onPath, i, j)
		}
		onPath = onPath && j == 0
	}
	return out
}

// src is package p in canonical layout: the records, aa, the dynamic keys and reads.
func (g deepGen) src(t *testing.T, reads []deepRead) string {
	t.Helper()
	out, err := canon.Format(srcMain, []byte(g.text(reads)))
	if err != nil {
		t.Fatalf("the fixture does not format: %v", err)
	}
	return string(out)
}

// text is src before formatting.
func (g deepGen) text(reads []deepRead) string {
	out := "package p\n\n"
	for i := g.c.depth; i >= 1; i-- {
		out += g.record(i)
	}
	out += "record A {\n  id: String\n  oo: " + g.collType(1) + "\n}\n\n"
	out += "let aa: [A] keyed by id = [\n  { id: \"first\", oo: " + g.literal(false, 1) + " },\n" +
		"  { id: \"second\", oo: " + g.literal(true, 1) + " },\n]\n\n"
	out += "let k0: String = \"second\"\n\n"
	for i := 1; i < g.c.depth; i++ {
		out += fmt.Sprintf("let k%d: String = %q\n\n", i, deepKeys[i-1][0])
	}
	typ := deepRecords[g.c.depth-1]
	if g.c.optional {
		typ += "?"
	}
	var lets []string
	for _, r := range reads {
		lets = append(lets, "let "+r.name+": "+typ+" = "+g.expr(r)+"\n")
	}
	return out + strings.Join(lets, "\n")
}

// layer amends second's renamed path down to its leaf, and first's.
func (g deepGen) layer() string {
	second, first := "second", "first"
	for i := 1; i <= g.c.depth; i++ {
		second += "." + deepFields[i-1] + "." + g.key(true, i, 0)
		first += "." + deepFields[i-1] + "." + g.key(false, i, 0)
	}
	return "package p\nlayer dev\n\namend aa {\n  " + second + ".w: 2\n  " + first + ".w: 3\n}\n"
}

// path is the canonical path of the entry at level ren of second's path, with its old key.
func (c deepCase) path(ren int) string {
	out := "p:aa[second]"
	for i := 1; i <= ren; i++ {
		out += "." + deepFields[i-1] + "[" + deepKeys[i-1][0] + "]"
	}
	return out
}

// deepStatic are the static reads: second's renamed path, its leaf's sibling, second's other
// level-1 entry's and first's; through optional fields, the renamed path with `?.` everywhere too.
func deepStatic(c deepCase) []deepRead {
	zeros := make([]int, c.depth)
	sib := slices.Clone(zeros)
	sib[c.depth-1] = 1
	oth := slices.Clone(zeros)
	oth[0] = 1
	reads := []deepRead{
		{"hit", "second", zeros, -1, false},
		{"sib", "second", sib, -1, false},
		{"oth", "second", oth, -1, false},
		{"fst", "first", zeros, -1, false},
	}
	if c.optional {
		reads = append(reads, deepRead{"all", "second", zeros, -1, true})
	}
	return reads
}

// deepRefLines are the lines of the reads of src through the entry at level ren of second's path.
func deepRefLines(src string, reads []deepRead, ren int) string {
	var out []string
	for _, r := range reads {
		if r.head == "second" && !slices.ContainsFunc(r.idx[:ren], func(j int) bool { return j != 0 }) {
			out = append(out, fmt.Sprintf("%s:%d", srcMain, lineOf(src, "let "+r.name+":")))
		}
	}
	return strings.Join(out, ", ")
}

// lineOf is the 1-based line of src starting with prefix, 0 for none.
func lineOf(src, prefix string) int {
	for i, l := range strings.Split(src, "\n") {
		if strings.HasPrefix(l, prefix) {
			return i + 1
		}
	}
	return 0
}

// deepCases are two and three levels, plain and optional, keyed lists and tables.
func deepCases() []deepCase {
	var out []deepCase
	for _, depth := range []int{2, 3} {
		for _, table := range []bool{false, true} {
			for _, optional := range []bool{false, true} {
				name := fmt.Sprintf("depth %d", depth)
				name += map[bool]string{false: " keyed list", true: " table"}[table]
				name += map[bool]string{false: " plain", true: " optional"}[optional]
				out = append(out, deepCase{name, depth, table, optional})
			}
		}
	}
	return out
}

// API.md R7, E11, E19, E22 (DECISIONS 473): a deep key read, plain or optional, is a code ref of the
// entry it reaches; renaming it, or the level-1 entry above, rewrites only the reads through it and
// its layer line, and the Undo restores both files byte for byte.
func TestEditRenameDeepKeyRead(t *testing.T) {
	for _, c := range deepCases() {
		for _, ren := range []int{c.depth, 1} {
			t.Run(fmt.Sprintf("%s rename level %d", c.name, ren), func(t *testing.T) {
				reads := deepStatic(c)
				old := deepGen{c: c, ren: ren, nk: deepKeys[ren-1][0]}
				renamed := deepGen{c: c, ren: ren, nk: "y"}
				src, layer := old.src(t, reads), old.layer()
				p, m := openEdit(t, srcLaw(src, "p/dev.layer.canon", layer))
				ctx := context.Background()
				refs, err := p.Refs(ctx, c.path(ren))
				if err != nil {
					t.Fatalf("API.md R7: Refs: %v", err)
				}
				if got, want := optionalCodeRefs(refs.Refs), deepRefLines(src, reads, ren); got != want {
					t.Errorf("API.md R7: the code refs of %s are %q, want %q", c.path(ren), got, want)
				}
				res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(c.path(ren), canon.Key("y"))}})
				if err != nil {
					t.Fatalf("API.md E19: the rename is refused: %v", err)
				}
				if got, want := read(t, m, srcMain), renamed.src(t, reads); got != want {
					t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", srcMain, got, want)
				}
				if got, want := read(t, m, "p/dev.layer.canon"), renamed.layer(); got != want {
					t.Errorf("API.md E11, R7: the layer after the rename:\n%s\nwant:\n%s", got, want)
				}
				if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
					t.Fatalf("API.md E22: the Undo: %v", err)
				}
				if got := read(t, m, srcMain); got != src {
					t.Errorf("API.md E22: %s after the Undo:\n%s", srcMain, got)
				}
				if got := read(t, m, "p/dev.layer.canon"); got != layer {
					t.Errorf("API.md E22: the layer after the Undo:\n%s", got)
				}
			})
		}
	}
}

// API.md R7, E18, E19 (DECISIONS 473): a read whose receiver is dynamic at any level is no code
// ref, though it reaches the renamed entry; the re-check finds the read the rename breaks (E4002)
// and the rename is refused, nothing written.
func TestEditRenameDeepDynamicReceiver(t *testing.T) {
	for _, c := range deepCases() {
		for dyn := range c.depth {
			t.Run(fmt.Sprintf("%s dynamic level %d", c.name, dyn), func(t *testing.T) {
				reads := append(deepStatic(c), deepRead{"dyn", "second", make([]int, c.depth), dyn, false})
				g := deepGen{c: c, ren: c.depth, nk: deepKeys[c.depth-1][0]}
				src := g.src(t, reads)
				p, m := openEdit(t, srcLaw(src))
				ctx := context.Background()
				refs, err := p.Refs(ctx, c.path(c.depth))
				if err != nil {
					t.Fatalf("API.md R7: Refs: %v", err)
				}
				if got, want := optionalCodeRefs(refs.Refs), deepRefLines(src, reads[:len(reads)-1], c.depth); got != want {
					t.Errorf("API.md R7: the code refs are %q, want %q (the dynamic read is none)", got, want)
				}
				res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(c.path(c.depth), canon.Key("y"))}})
				var re *canon.RejectedError
				if !errors.As(err, &re) || res == nil || res.Applied {
					t.Fatalf("API.md E19: %v, want the rename rejected", err)
				}
				code := string(diag.E4002.Def().Code)
				if !slices.ContainsFunc(re.Findings, func(f canon.Finding) bool { return f.Code == code && f.Line == lineOf(src, "let dyn:") }) {
					t.Errorf("API.md E18: findings %v, want %s at the dynamic read", re.Findings, code)
				}
				if got := read(t, m, srcMain); got != src {
					t.Errorf("API.md E19: %s written by a refused rename:\n%s", srcMain, got)
				}
			})
		}
	}
}
