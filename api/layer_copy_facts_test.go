package canon_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// copyFactsSrc is p: q2 and c2 plainly copy q1 (no collection) and c1 (a keyed list); h2 spreads h1
// holding both nested; pi copies b1's inner record; j.b and two.l[1] copy j.a and two.l[0] (79);
// pw's inner is its own. Every tag is self.id and every lab self.w + 1, defaults a source supplies.
const copyFactsSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n  lab: Int = self.w + 1\n}\n\n" +
	"record Q {\n  id: String\n  tag: String = self.id\n}\n\n" +
	"record C {\n  id: String\n  tag: String = self.id\n  oo: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n}\n\n" +
	"record H {\n  inner: Q\n  innc: C\n}\n\n" +
	"record B {\n  id: String\n  inner: C\n}\n\n" +
	"record J {\n  a: C\n  b: C = self.a\n}\n\n" +
	"record T2 {\n  l: [C]\n}\n\n" +
	"fn dup(i: C) -> [C] {\n  return [i, i]\n}\n\n" +
	"record MM {\n  m: {String: C}\n}\n\n" +
	"fn twoMap(i: C) -> {String: C} {\n  return { \"a\": i, \"b\": i }\n}\n\n" +
	"let q1: Q = { id: \"a\" }\n\nlet q2: Q = q1\n\n" +
	"let c1: C = { id: \"a\" }\n\nlet c2: C = c1\n\n" +
	"let h1: H = { inner: { id: \"a\" }, innc: { id: \"a\" } }\n\nlet h2: H = { ...h1 }\n\n" +
	"let b1: B = { id: \"b1\", inner: { id: \"i\" } }\n\nlet pi: C = b1.inner\n\nlet b5: B = { ...b1, id: \"b5\" }\n\n" +
	"let j: J = { a: { id: \"i\" } }\n\nlet jy: S = j.b.oo.y\n\n" +
	"let jsame: Bool = j.b.oo.x == j.a.oo.x\n\n" +
	"let pw: B = { id: \"pw\", inner: { id: \"own\" } }\n\nlet pwy: Bool = pw.inner.oo.y == b1.inner.oo.y\n\n" +
	"let two: T2 = { l: dup({ id: \"t\" }) }\n\nlet twox: Bool = two.l[1].oo.x == two.l[0].oo.x\n\n" +
	"let mm: MM = { m: twoMap({ id: \"t\" }) }\n\nlet mmtags: [String] = [mm.m[\"a\"].tag, mm.m[\"b\"].tag]\n"

// copyFactsCase is a set of layer files (name to amend blocks), the active ones, and values with the
// layers active.
type copyFactsCase struct {
	name   string
	layers map[string]string
	active []string
	want   map[string]string
}

// openFacts opens copyFactsSrc with c's layer files, active when on.
func openFacts(t *testing.T, c copyFactsCase, on bool) *canon.Project {
	t.Helper()
	law := maps.Clone(editLaw)
	law[srcMain] = copyFactsSrc
	for _, name := range slices.Sorted(maps.Keys(c.layers)) {
		law["p/"+name+".layer.canon"] = "package p\nlayer " + name + "\n\n" + c.layers[name]
	}
	opts := project(law)
	if on {
		opts.Layers = c.active
	}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// EVALUATION.md 9.3 step 3, DECISIONS 486 (rulings of 2026-11-05): an amended copy, a plain or a
// spread one, at any depth, whatever its record holds, by one line or several, re-derives none of
// the defaults its source supplied; an entry amended below a copy keeps its lab too.
func TestLayerCopyKeepsSuppliedDefaults(t *testing.T) {
	two := "amend pi {\n  id: \"z\"\n  oo.x.w: 5\n}\n"
	for _, c := range []copyFactsCase{
		{"plain copy without collection", map[string]string{"dev": "amend q2 {\n  id: \"b\"\n}\n"}, []string{"dev"}, map[string]string{"p:q2.tag": "a", "p:q2.id": "b"}},
		{"plain copy with collection", map[string]string{"dev": "amend c2 {\n  id: \"b\"\n}\n"}, []string{"dev"}, map[string]string{"p:c2.tag": "a"}},
		{"nested in a spread", map[string]string{"dev": "amend h2 {\n  inner.id: \"b\"\n  innc.id: \"b\"\n}\n"}, []string{"dev"},
			map[string]string{"p:h2.inner.tag": "a", "p:h2.innc.tag": "a", "p:h2.innc.id": "b"}},
		{"one line", map[string]string{"dev": "amend pi {\n  id: \"z\"\n}\n"}, []string{"dev"}, map[string]string{"p:pi.tag": "i"}},
		{"two lines", map[string]string{"dev": two}, []string{"dev"}, map[string]string{"p:pi.tag": "i", "p:pi.oo[x].w": "5"}},
		{"two lines reversed", map[string]string{"dev": "amend pi {\n  oo.x.w: 5\n  id: \"z\"\n}\n"}, []string{"dev"}, map[string]string{"p:pi.tag": "i"}},
		{"two layers", map[string]string{"dev": "amend pi {\n  id: \"z\"\n}\n", "qa": "amend pi {\n  oo.x.w: 5\n}\n"}, []string{"dev", "qa"},
			map[string]string{"p:pi.tag": "i", "p:pi.id": "z"}},
		{"entry below a copy", map[string]string{"dev": "amend b5 {\n  inner.oo.x.w: 5\n}\n"}, []string{"dev"},
			map[string]string{"p:b5.inner.oo[x].w": "5", "p:b5.inner.oo[x].lab": "1", "p:b1.inner.oo[x].lab": "1"}},
		{"a right-hand side's read", map[string]string{"dev": "amend pw {\n  inner: b1.inner\n}\n", "qa": "amend pw {\n  inner.id: \"z\"\n}\n"},
			[]string{"dev", "qa"}, map[string]string{"p:pwy": "true", "p:pw.inner.tag": "i", "p:pw.inner.id": "z"}},
		{"a map's first place", map[string]string{"dev": "amend mm {\n  m[\"a\"].id: \"z\"\n}\n"}, []string{"dev"},
			map[string]string{"p:mmtags": `["z", "t"]`}},
		{"a map's first place after a new key", map[string]string{"dev": "amend mm {\n  m[\"c\"]: { id: \"c\" }\n  m[\"a\"].id: \"z\"\n}\n"},
			[]string{"dev"}, map[string]string{"p:mmtags": `["z", "t"]`}},
		{"a map's first place after a layer's new key", map[string]string{"dev": "amend mm {\n  m[\"c\"]: { id: \"c\" }\n}\n", "qa": "amend mm {\n  m[\"a\"].id: \"z\"\n}\n"},
			[]string{"dev", "qa"}, map[string]string{"p:mmtags": `["z", "t"]`}},
		{"a map's second place after a new key", map[string]string{"dev": "amend mm {\n  m[\"c\"]: { id: \"c\" }\n  m[\"b\"].id: \"z\"\n}\n"},
			[]string{"dev"}, map[string]string{"p:mmtags": `["t", "t"]`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			checkValues(t, openFacts(t, c, true), "DECISIONS 486", c.want)
		})
	}
}

// DECISIONS 79, 486 (ruling of 2026-11-05): j.b, a default yielding j.a, is a copy, so a line amending
// its id leaves the entries it holds j.a's and re-derives no default: no node of its own, its reads
// still refs of j.a's entries, active or inactive.
func TestLayerCopyWithinALet(t *testing.T) {
	c := copyFactsCase{"", map[string]string{"dev": "amend j {\n  b.id: \"z\"\n}\n"}, []string{"dev"}, nil}
	off, on := openFacts(t, c, false), openFacts(t, c, true)
	checkValues(t, on, "DECISIONS 486", map[string]string{"p:jsame": "true", "p:j.b.id": "z", "p:j.b.tag": "i", "p:j.a.tag": "i"})
	checkValues(t, off, "DECISIONS 79", map[string]string{"p:jsame": "true"})
	if nodes := elementNodes(t, on); slices.ContainsFunc(nodes, func(n string) bool { return strings.HasPrefix(n, "p:j.b.") }) {
		t.Errorf("DECISIONS 478: j.b holds nodes of its own: %v", nodes)
	}
	refs := func(p *canon.Project) string {
		res, err := p.Refs(context.Background(), "p:j.a.oo[y]")
		if err != nil {
			t.Fatalf("API.md R7: %v", err)
		}
		var out []string
		for _, r := range res.Refs {
			out = append(out, fmt.Sprintf("%v %s:%d", r.Kind, r.File, r.Line))
		}
		return strings.Join(out, ", ")
	}
	if a, b := refs(off), refs(on); a != b || a == "" {
		t.Errorf("DECISIONS 410: Refs of p:j.a.oo[y] = %q with the layer, want %q as without", b, a)
	}
}

// DECISIONS 79, 478, 486: two.l holds one record at l[0], then l[1]. Amending l[1] re-derives none of
// a copy's defaults, l[0] its own; untouched entries keep their identity at both, nodes at l[0] when
// l[1] is amended (with l[0] amended Graph's placement is edit's, unchecked).
func TestLayerCopyAtASecondPlace(t *testing.T) {
	for _, c := range []struct {
		line, tag0, tag1 string
		nodes            bool
	}{{`l[1].id: "z"`, "t", "t", true}, {`l[0].id: "z"`, "z", "t", false}} {
		t.Run(c.line, func(t *testing.T) {
			fc := copyFactsCase{"", map[string]string{"dev": "amend two {\n  " + c.line + "\n}\n"}, []string{"dev"}, nil}
			on := openFacts(t, fc, true)
			checkValues(t, on, "DECISIONS 486", map[string]string{"p:twox": "true", "p:two.l[0].tag": c.tag0, "p:two.l[1].tag": c.tag1})
			if !c.nodes {
				return
			}
			nodes := elementNodes(t, on)
			for _, want := range []string{"p:two.l[0].oo[x]", "p:two.l[0].oo[y]"} {
				if !slices.Contains(nodes, want) {
					t.Errorf("DECISIONS 478: no node %s: %v", want, nodes)
				}
			}
			if slices.ContainsFunc(nodes, func(n string) bool { return strings.HasPrefix(n, "p:two.l[1].") }) {
				t.Errorf("DECISIONS 478: l[1] holds nodes of its own: %v", nodes)
			}
		})
	}
}

// memoCopyA declares b1 as an entry of bs; memoCopyB, another file, spreads it into b5 and converts
// b1's y to b5's let path; an edit of n re-evaluates memoCopyB's file, replaying memoCopyA's values.
const (
	memoCopyA = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nrecord I {\n  id: String\n  oo: [S] keyed by name\n  tag: String = self.id\n}\n\n" +
		"record B {\n  id: String\n  inner: I\n}\n\nlet bs: [B] keyed by id = []\n\n" +
		"entry bs.e { inner: { id: \"i\", oo: [{ name: \"x\" }, { name: \"y\" }] } }\n\nlet b1: B = bs.e\n"
	memoCopyB = "package p\n\nlet n: Int = 1\n\nlet b5: B = { ...b1, id: \"b5\" }\n\nlet r5: ref b5.inner.oo = b1.inner.oo.y\n\n" +
		"let s5: [Bool] = [b5.inner.oo.y == b1.inner.oo.y, b5.inner.tag == \"i\"]\n"
)

// memoView is p's errors and s5, as text.
func memoView(t *testing.T, p *canon.Project) string {
	t.Helper()
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range res.Findings {
		if f.Severity == "error" {
			out = append(out, f.Code+" "+f.Path)
		}
	}
	return strings.Join(append(out, valueText(t, p, "p:s5")), "; ")
}

// API.md W7, DECISIONS 486: under an active layer, a build after an edit, which replays the values
// of an unchanged file, gives the copy facts a cold build gives: no false E3503, s5 the same.
func TestLayerCopyWarmEqualsCold(t *testing.T) {
	law := maps.Clone(editLaw)
	law["p/p.canon"], law["p/b.canon"] = memoCopyA, memoCopyB
	law["p/dev.layer.canon"] = "package p\nlayer dev\n\namend b5 {\n  inner.id: \"z\"\n}\n"
	opts := project(law)
	opts.Layers = []string{"dev"}
	warm, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = warm.Close() })
	const want = "[true, true]"
	if got := memoView(t, warm); got != want {
		t.Fatalf("DECISIONS 486: cold %q, want %q", got, want)
	}
	for step := 2; step <= 3; step++ {
		if _, err := warm.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:n", canon.Int(int64(step)))}}); err != nil {
			t.Fatalf("API.md E19: %v", err)
		}
		m, _ := opts.FS.(*memFS)
		after := maps.Clone(law)
		after["p/b.canon"] = read(t, m, "p/b.canon")
		cold := project(after)
		cold.Layers = opts.Layers
		c, err := canon.Open("/law", cold)
		if err != nil {
			t.Fatal(err)
		}
		if w, cv := memoView(t, warm), memoView(t, c); w != cv || cv != want {
			t.Errorf("API.md W7: after edit %d warm %q, cold %q, want %q", step, w, cv, want)
		}
		_ = c.Close()
	}
}
