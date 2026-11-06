package canon_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// defaultCopySrc is p: r's defaults b, alt, deep and au yield an earlier field, an element of qs, a
// record in one of gs, an entry of uu; Q's picks key, an entry of R's oo, binds only when r completes.
// r is w; the p* lets compare through each copy, the o* lets its own entry, the r* lets read w.
const defaultCopySrc = "package p\n\nrecord T {\n  n: String\n  w: Int = 0\n}\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\n" +
	"record S {\n  name: String\n}\n\n" +
	"record Q {\n  id: String\n  tt: [T] keyed by n\n  best: ref T = \"t\"\n  picks: {ref S: Int} = { x: 2 }\n}\n\n" +
	"record G {\n  id: String\n  inner: Q\n}\n\n" +
	"record R {\n  oo: [S] keyed by name = [{ name: \"x\" }]\n  a: Q\n  b: Q = self.a\n  qs: [Q] keyed by id\n  alt: Q = qs.q\n" +
	"  gs: [G] keyed by id\n  deep: Q = gs.g.inner\n  uu: [U] keyed by n = [{ n: \"x\" }]\n  au: U = uu.x\n}\n\n" +
	"let r: R = w\n\n" +
	"let pb: [Bool] = [r.b.tt.t == r.a.tt.t, r.b.best == r.a.best, r.a.tt.contains(r.b.best), r.b.picks.keys().contains(r.oo.x)]\n\n" +
	"let palt: [Bool] = [r.alt.tt.t == r.qs.q.tt.t, r.alt.best == r.qs.q.best, r.qs.q.tt.contains(r.alt.best), r.alt.picks.keys().contains(r.oo.x), r.alt == r.qs.q]\n\n" +
	"let pdeep: [Bool] = [r.deep.tt.t == r.gs.g.inner.tt.t, r.deep.best == r.gs.g.inner.best, r.gs.g.inner.tt.contains(r.deep.best), r.deep.picks.keys().contains(r.oo.x)]\n\n" +
	"let pau: [Bool] = [r.au == r.uu.x, r.uu.contains(r.au)]\n\n" +
	"let ob: [Bool] = [r.b.tt.t == r.a.tt.t, r.b.best == r.a.best, r.b.tt.contains(r.b.best), r.b.best.w == 5]\n\n" +
	"let oalt: [Bool] = [r.alt.tt.t == r.qs.q.tt.t, r.alt.best == r.qs.q.best, r.alt.tt.contains(r.alt.best), r.alt.best.w == 5]\n\n" +
	"let odeep: [Bool] = [r.deep.tt.t == r.gs.g.inner.tt.t, r.deep.best == r.gs.g.inner.best, r.deep.tt.contains(r.deep.best), r.deep.best.w == 5]\n\n" +
	"let rb: Int = r.b.tt.t.w\n\nlet ralt: Int = r.alt.tt.t.w\n\nlet rdeep: Int = r.deep.tt.t.w\n\nlet rau: Int = r.au.w\n"

// defaultCopyLit and defaultCopyJSON give r's written fields, as a literal and as a JSON document.
const (
	defaultCopyLit  = `{ a: { id: "a", tt: [{ n: "t" }] }, qs: [{ id: "q", tt: [{ n: "t" }] }], gs: [{ id: "g", inner: { id: "q", tt: [{ n: "t" }] } }] }`
	defaultCopyJSON = `{"a": {"id": "a", "tt": [{"n": "t"}]}, "qs": [{"id": "q", "tt": [{"n": "t"}]}], "gs": [{"id": "g", "inner": {"id": "q", "tt": [{"n": "t"}]}}]}` + "\n"
)

// defaultCopyLayerLine is where the dev layer's one line sits in its file, which Refs lists.
var defaultCopyLayerLine = fmt.Sprintf("%v p/dev.layer.canon:5", canon.RefLayer)

// defaultCopyWays are how r is written: a literal, a load of defaultCopyJSON.
var defaultCopyWays = []struct{ name, w string }{{"literal", defaultCopyLit}, {"load", `load("r.json")`}}

// defaultCopyShape is one copy of r: its probe let, its own-entry probe let, the let reading w
// through it, its own path, and the first path of the entry that read reaches. A read naming the
// entry by its key token is a code ref of it (API.md R7); entry marks a copy that is an entry itself.
type defaultCopyShape struct {
	probe, own, read, copy, first string
	keyed, entry                  bool
}

// defaultCopyShapes are b, alt, deep and au; alt and au are entries, and au, read through a field,
// has no key token in its read and no entry below it.
var defaultCopyShapes = []defaultCopyShape{
	{"p:pb", "p:ob", "rb", "p:r.b", "p:r.a.tt[t]", true, false},
	{"p:palt", "p:oalt", "ralt", "p:r.alt", "p:r.qs[q].tt[t]", true, true},
	{"p:pdeep", "p:odeep", "rdeep", "p:r.deep", "p:r.gs[g].inner.tt[t]", true, false},
	{"p:pau", "", "rau", "p:r.au", "p:r.uu[x]", false, true},
}

// defaultCopyLine is a layer line amending r, a path and the value it then holds, and the copy
// whose own entry it amends, if any.
type defaultCopyLine struct {
	text, at, want, below string
}

// defaultCopyLines amend each copy's id or source (a.id re-derives b), each source's entry (which
// re-derives its copy), and the entry below each copy.
var defaultCopyLines = []defaultCopyLine{
	{`a.id: "z"`, "p:r.b.id", "z", ""}, {`b.id: "z"`, "p:r.b.id", "z", ""},
	{`alt.id: "z"`, "p:r.alt.id", "z", ""}, {`deep.id: "z"`, "p:r.deep.id", "z", ""},
	{"a.tt.t.w: 5", "p:rb", "5", ""}, {"qs.q.tt.t.w: 5", "p:ralt", "5", ""},
	{"gs.g.inner.tt.t.w: 5", "p:rdeep", "5", ""}, {"uu.x.w: 5", "p:rau", "5", ""},
	{"b.tt.t.w: 5", "p:rb", "5", "p:r.b"}, {"alt.tt.t.w: 5", "p:ralt", "5", "p:r.alt"},
	{"deep.tt.t.w: 5", "p:rdeep", "5", "p:r.deep"},
}

// openDefaultCopy opens defaultCopySrc with r written as w, under a layer dev of the line amending r, active when on.
func openDefaultCopy(t *testing.T, w, line string, on bool) *canon.Project {
	t.Helper()
	src := strings.Replace(defaultCopySrc, "= w\n", "= "+w+"\n", 1)
	return openLayeredWith(t, src, map[string]string{"p/r.json": defaultCopyJSON}, "amend r {\n  "+line+"\n}\n", on)
}

// defaultCopyRefs are the kind, file and line of each ref Refs lists for the entry at id.
func defaultCopyRefs(t *testing.T, p *canon.Project, id string) []string {
	t.Helper()
	res, err := p.Refs(context.Background(), id)
	if err != nil {
		t.Fatalf("API.md R7: Refs of %s: %v", id, err)
	}
	var out []string
	for _, r := range res.Refs {
		out = append(out, fmt.Sprintf("%v %s:%d", r.Kind, r.File, r.Line))
	}
	return out
}

// checkErrors fails when p has an error finding.
func checkErrors(t *testing.T, p *canon.Project) {
	t.Helper()
	if errs := errorsOf(t, p); len(errs) > 0 {
		t.Fatalf("EVALUATION.md §3.4: errors\n%s", strings.Join(errs, "\n"))
	}
}

// checkDefaultCopy fails unless, in p, c's copy holds its source's entries: its probe all true, no
// Graph node of its own, the node at its first path (DECISIONS 478), and the read through the copy
// listed among that entry's refs (API.md R7).
func checkDefaultCopy(t *testing.T, p *canon.Project, c defaultCopyShape) {
	t.Helper()
	checkErrors(t, p)
	if got := probeText(t, p, c.probe); strings.Contains(got, "false") {
		t.Errorf("EVALUATION.md §4.2, DECISIONS 79, 492, 498: %s = %s, want every comparison true", c.probe, got)
	}
	nodes := elementNodes(t, p)
	if !slices.Contains(nodes, c.first) || slices.ContainsFunc(nodes, func(n string) bool { return strings.HasPrefix(n, c.copy+".") }) {
		t.Errorf("DECISIONS 478: element nodes %v, want %s and none below %s", nodes, c.first, c.copy)
	}
	if !c.keyed {
		return
	}
	line := slices.IndexFunc(strings.Split(defaultCopySrc, "\n"), func(l string) bool { return strings.HasPrefix(l, "let "+c.read+":") }) + 1
	if want := fmt.Sprintf("%v %s:%d", canon.RefCode, srcMain, line); !slices.Contains(defaultCopyRefs(t, p, c.first), want) {
		t.Errorf("API.md R7: Refs of %s lack %q: %v", c.first, want, defaultCopyRefs(t, p, c.first))
	}
}

// checkOwnEntry fails unless, in p, the line below c's copy made the copy's tt[t] its own: its best
// moved there and the source's is unequal (DECISIONS 486, 498), a node at each path (478), and
// the line's key a layer ref of both (491).
func checkOwnEntry(t *testing.T, p *canon.Project, c defaultCopyShape) {
	t.Helper()
	checkErrors(t, p)
	checkValues(t, p, "DECISIONS 486, 498", map[string]string{c.own: "[false, false, true, true]"})
	own, nodes := c.copy+".tt[t]", elementNodes(t, p)
	if !slices.Contains(nodes, own) || !slices.Contains(nodes, c.first) {
		t.Errorf("DECISIONS 478: element nodes %v, want %s and %s", nodes, c.first, own)
	}
	for _, id := range []string{own, c.first} {
		if refs := defaultCopyRefs(t, p, id); !slices.Contains(refs, defaultCopyLayerLine) {
			t.Errorf("DECISIONS 491: Refs of %s lack %q: %v", id, defaultCopyLayerLine, refs)
		}
	}
}

// EVALUATION.md 4.2, DECISIONS 79, 478, 492, 498 (ruling of 2026-11-06): a default yielding an
// existing value, at any depth, shares its instance, literal or loaded: its entries and bound refs
// are the source's (== and membership agree), Graph's nodes and Refs' reads at the first path.
func TestDefaultCopySharesInstance(t *testing.T) {
	for _, w := range defaultCopyWays {
		for _, c := range defaultCopyShapes {
			t.Run(w.name+" "+c.copy, func(t *testing.T) {
				checkDefaultCopy(t, openDefaultCopy(t, w.w, `a.id: "a"`, false), c)
			})
		}
	}
}

// checkLine checks every copy of p, built with line active when on: the copy line amends below
// holds its own entry, every other one its source's.
func checkLine(t *testing.T, p *canon.Project, line defaultCopyLine, on bool) {
	t.Helper()
	for _, c := range defaultCopyShapes {
		if on && c.copy == line.below {
			checkOwnEntry(t, p, c)
			continue
		}
		checkCopyAfter(t, p, c, line.text, on)
	}
	if on {
		checkValues(t, p, "EVALUATION.md §9.3", map[string]string{line.at: line.want})
	}
}

// EVALUATION.md 9.3 step 3, DECISIONS 486, 491, 492, 501 (ruling of 2026-11-06): every line, active
// or inactive, literal or loaded, leaves every copy its source's entries, re-derives a copy whose
// source it amends, and makes an entry it amends below a copy that copy's own.
func TestDefaultCopySharesInstanceUnderLayer(t *testing.T) {
	for _, w := range defaultCopyWays {
		for _, line := range defaultCopyLines {
			t.Run(w.name+" "+line.text, func(t *testing.T) {
				checkLine(t, openDefaultCopy(t, w.w, line.text, false), line, false)
				checkLine(t, openDefaultCopy(t, w.w, line.text, true), line, true)
			})
		}
	}
}

// API.md W7, DECISIONS 492 (ruling of 2026-11-06): a build after an edit of another file, which
// replays r's file, keeps every copy fact of a cold build, the layer amending alt inactive or active.
func TestDefaultCopySharesInstanceWarm(t *testing.T) {
	for _, w := range defaultCopyWays {
		for _, on := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s layer %v", w.name, on), func(t *testing.T) {
				src := strings.Replace(defaultCopySrc, "= w\n", "= "+w.w+"\n", 1)
				more := map[string]string{"p/r.json": defaultCopyJSON, "p/n.canon": "package p\n\nlet n: Int = 1\n"}
				p := openLayeredWith(t, src, more, "amend r {\n  alt.id: \"z\"\n}\n", on)
				for step := 2; step <= 3; step++ {
					if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:n", canon.Int(int64(step)))}}); err != nil {
						t.Fatalf("API.md E19: %v", err)
					}
					for _, c := range defaultCopyShapes {
						checkCopyAfter(t, p, c, "alt.id", on)
					}
				}
			})
		}
	}
}

// keptEntrySrc is p: r's kept defaults au, ap and al hold, refer to and list an entry of its own uu.
const keptEntrySrc = "package p\n\nrecord U {\n  n: String\n  w: Int = 0\n}\n\n" +
	"record R {\n  id: String\n  uu: [U] keyed by n = [{ n: \"x\" }]\n  au: U = uu.x\n  ap: ref U = uu.x\n  al: [U] = [uu.x]\n}\n\n" +
	"let r: R = { id: \"a\" }\n\nlet pk: [Bool] = [r.au == r.uu.x, r.uu.contains(r.au), r.ap == r.uu.x, r.al[0] == r.uu.x, r.al.contains(r.uu.x)]\n"

// DECISIONS 498, 501 (ruling of 2026-11-06): a default an amendment of another field keeps holds the
// entry its source holds after the amendment, so == and membership agree, active or inactive.
func TestKeptDefaultFollowsAdoption(t *testing.T) {
	for _, on := range []bool{false, true} {
		t.Run(fmt.Sprintf("layer %v", on), func(t *testing.T) {
			p := openLayered(t, keptEntrySrc, "amend r {\n  id: \"z\"\n}\n", on)
			checkValues(t, p, "DECISIONS 498", map[string]string{"p:pk": "[true, true, true, true, true]"})
		})
	}
}

// The depths of TestKeptDefaultSharedDepth, and the allocations re-owning may add from the first to
// the second: a walk per node adds about 32 a level, one per path at least 2^16.
const (
	sharedDepthLow, sharedDepthHigh, sharedDepthMax = 8, 16, 20
	sharedDepthGrowth                               = 5000
)

// sharedDepthSrc is p: r's kept default w0 lists uu's x twice, and each w_i lists w_{i-1} twice, to depth d.
func sharedDepthSrc(d int) string {
	var b strings.Builder
	b.WriteString("package p\n\nrecord U {\n  n: String\n}\n\nrecord R {\n  id: String\n  uu: [U] keyed by n = [{ n: \"x\" }]\n  w0: [U] = [uu.x, uu.x]\n")
	typ := "[U]"
	for i := 1; i <= d; i++ {
		typ = "[" + typ + "]"
		fmt.Fprintf(&b, "  w%d: %s = [w%d, w%d]\n", i, typ, i-1, i-1)
	}
	fmt.Fprintf(&b, "}\n\nlet r: R = { id: \"a\" }\n\nlet pk: [Bool] = [r.w0[0] == r.uu.x, r.w%d[1][0] == r.w%d[0][1]]\n", d, d)
	return b.String()
}

// sharedDepthBuild opens sharedDepthSrc at depth d with r's id amended, active when on, and checks pk.
func sharedDepthBuild(t *testing.T, d int, on bool) {
	t.Helper()
	p := openLayered(t, sharedDepthSrc(d), "amend r {\n  id: \"z\"\n}\n", on)
	checkValues(t, p, "DECISIONS 498", map[string]string{"p:pk": "[true, true]"})
}

// heldAllocs are the allocations of a build at depth d with the layer active beyond those without it.
func heldAllocs(t *testing.T, d int) float64 {
	t.Helper()
	off := testing.AllocsPerRun(1, func() { sharedDepthBuild(t, d, false) })
	return testing.AllocsPerRun(1, func() { sharedDepthBuild(t, d, true) }) - off
}

// DECISIONS 498, 501 (ruling of 2026-11-06): re-owning a kept default's entries walks a value held
// at several places once, so its cost grows with the nodes, not the paths, and the answer is right.
func TestKeptDefaultSharedDepth(t *testing.T) {
	low, high := heldAllocs(t, sharedDepthLow), heldAllocs(t, sharedDepthHigh)
	t.Logf("re-owning adds %.0f allocations at depth %d, %.0f at %d", low, sharedDepthLow, high, sharedDepthHigh)
	if high-low > sharedDepthGrowth {
		t.Errorf("DECISIONS 498: re-owning adds %.0f allocations at depth %d, %.0f at %d: it walks paths", low, sharedDepthLow, high, sharedDepthHigh)
	}
	sharedDepthBuild(t, sharedDepthMax, true)
}
