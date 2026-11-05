package canon_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// spreadLayerSrc is p: bb spreads aa's second, writing only id, so its oo holds second's entries;
// r reads x through bb, rw its w, rr refers to it through bb's let path, same compares both
// reads, and cc and pk hold refs converted from bb's x, which pw, pkw and eqp read.
const spreadLayerSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record P {\n  pick: ref S\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by name\n  subs: [P] = []\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ name: \"x\" }] },\n" +
	"  { id: \"second\", oo: [{ name: \"x\" }, { name: \"y\" }] },\n]\n\n" +
	"let bb: A = { ...aa.second, id: \"third\" }\n\n" +
	"let r: S = bb.oo.x\n\n" +
	"let rw: Int = bb.oo.x.w\n\n" +
	"let rr: ref bb.oo = bb.oo.x\n\n" +
	"let same: Bool = bb.oo.x == aa.second.oo.x\n\n" +
	"let cc: A = { id: \"c\", oo: bb.oo, subs: [{ pick: bb.oo.x }] }\n\n" +
	"let pk: P = { pick: bb.oo.x }\n\n" +
	"let pw: Int = cc.subs[0].pick.w\n\n" +
	"let pkw: Int = pk.pick.w\n\n" +
	"let eqp: Bool = cc.subs[0].pick == bb.oo.x\n"

// spreadLayerRW is the file and line of rw in spreadLayerSrc, whose code ref Refs lists.
var spreadLayerRW = fmt.Sprintf("%s:%d", srcMain, slices.Index(strings.Split(spreadLayerSrc, "\n"), "let rw: Int = bb.oo.x.w")+1)

// openLayered opens src as p's source with a layer dev of body, active when on.
func openLayered(t *testing.T, src, body string, on bool) *canon.Project {
	t.Helper()
	law := maps.Clone(editLaw)
	law[srcMain] = src
	law["p/dev.layer.canon"] = "package p\nlayer dev\n\n" + body
	opts := project(law)
	if on {
		opts.Layers = []string{"dev"}
	}
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// openSpreadLayer opens spreadLayerSrc with a layer dev amending bb by line, active when on.
func openSpreadLayer(t *testing.T, line string, on bool) *canon.Project {
	t.Helper()
	return openLayered(t, spreadLayerSrc, "amend bb {\n  "+line+"\n}\n", on)
}

// spreadLayerView is what the test compares of p: second's x's refs and the graph of package p
// below its roots, sorted.
func spreadLayerView(t *testing.T, p *canon.Project) (refs, graph []string) {
	t.Helper()
	res, err := p.Refs(context.Background(), "p:aa[second].oo[x]")
	if err != nil {
		t.Fatalf("API.md R7: Refs: %v", err)
	}
	for _, r := range res.Refs {
		refs = append(refs, fmt.Sprintf("%v %s:%d", r.Kind, r.File, r.Line))
	}
	nodes, edges := graphOf(t, p)
	for _, n := range nodes {
		if n.Package == "p" && n.Kind != canon.NodeRoot {
			graph = append(graph, fmt.Sprintf("%s %s %s", n.Kind, n.ID, n.Type))
		}
	}
	for _, e := range edges {
		if strings.HasPrefix(e.To, "p:") {
			graph = append(graph, fmt.Sprintf("%s %s %s %s %s %v", e.Kind, e.From, e.Path, e.Pattern, e.To, e.Computed))
		}
	}
	slices.Sort(graph)
	return refs, graph
}

// valueText is the text of the value at path in p.
func valueText(t *testing.T, p *canon.Project, path string) string {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil {
		t.Fatalf("API.md R1: Value of %s: %v", path, err)
	}
	return v.Text
}

// checkValues fails for each path of want whose value in p is not its text.
func checkValues(t *testing.T, p *canon.Project, rule string, want map[string]string) {
	t.Helper()
	for _, path := range slices.Sorted(maps.Keys(want)) {
		if got := valueText(t, p, path); got != want[path] {
			t.Errorf("%s: %s = %s, want %s", rule, path, got, want[path])
		}
	}
}

// DECISIONS 410, 478: an active layer line leaving bb's entries unchanged keeps second's identity
// on them: Refs of second's x are those without the layer, Graph gains only the entry amended.
func TestLayerSpreadCopyKeepsSourceEntries(t *testing.T) {
	for _, c := range []struct{ line, gains string }{{`id: "fourth"`, ""}, {`oo.y.w: 5`, "element p:bb.oo[y] p.S"}} {
		t.Run(c.line, func(t *testing.T) {
			refs0, graph0 := spreadLayerView(t, openSpreadLayer(t, c.line, false))
			p := openSpreadLayer(t, c.line, true)
			refs1, graph1 := spreadLayerView(t, p)
			if len(refs0) == 0 || !slices.Equal(refs0, refs1) {
				t.Errorf("DECISIONS 410: Refs of p:aa[second].oo[x] =\n%s\nwith the layer, want, as without\n%s",
					strings.Join(refs1, "\n"), strings.Join(refs0, "\n"))
			}
			if c.gains != "" {
				graph0 = append(graph0, c.gains)
				slices.Sort(graph0)
			}
			if !slices.Equal(graph0, graph1) {
				t.Errorf("DECISIONS 478: Graph =\n%s\nwith the layer, want\n%s", strings.Join(graph1, "\n"), strings.Join(graph0, "\n"))
			}
			if want := string(canon.RefCode) + " " + spreadLayerRW; !slices.Contains(refs1, want) {
				t.Errorf("API.md R7, DECISIONS 473: Refs with the layer lack bb.oo.x.w's %q:\n%s", want, strings.Join(refs1, "\n"))
			}
			checkValues(t, p, "EVALUATION.md §4.2", map[string]string{"p:same": "true", "p:pw": "0", "p:pkw": "0", "p:eqp": "true"})
		})
	}
}

// DECISIONS 478 (TYPES.md 6.2, 6.3, 7.5): an entry the layer amends below it or replaces is the
// amended bb's own, its key kept, and a ref converted from it dereferences to it.
func TestLayerSpreadCopyAmendedEntry(t *testing.T) {
	for _, c := range []struct{ line, w string }{{`oo.x.w: 5`, "5"}, {`oo.x: { name: "x", w: 9 }`, "9"}, {`oo.x: aa.first.oo.x`, "0"}} {
		t.Run(c.line, func(t *testing.T) {
			p := openSpreadLayer(t, c.line, true)
			_, graph := spreadLayerView(t, p)
			if !slices.Contains(graph, "element p:bb.oo[x] p.S") || slices.Contains(graph, "element p:bb.oo[y] p.S") {
				t.Errorf("DECISIONS 478: Graph, want bb's own x and second's y:\n%s", strings.Join(graph, "\n"))
			}
			checkValues(t, p, "TYPES.md §6.3", map[string]string{
				"p:same": "false", "p:rw": c.w, "p:pw": c.w, "p:pkw": c.w, "p:eqp": "true", "p:aa[second].oo[x].w": "0",
			})
		})
	}
}

// DECISIONS 478: a layer line writing bb's oo whole gives bb its own entries, as a field a spread
// writes holds: x through bb is bb's node and no longer second's x.
func TestLayerWrittenFieldOwnsEntries(t *testing.T) {
	for _, c := range []struct {
		line string
		want []string
	}{
		{`oo: aa.first.oo`, []string{"element p:bb.oo[x] p.S", "value p:rr p:rr p:rr p:bb.oo[x] true"}},
		{`oo: [{ name: "x", w: 7 }]`, []string{"element p:bb.oo[x] p.S"}},
	} {
		t.Run(c.line, func(t *testing.T) {
			p := openSpreadLayer(t, c.line, true)
			_, graph := spreadLayerView(t, p)
			for _, want := range c.want {
				if !slices.Contains(graph, want) {
					t.Errorf("DECISIONS 478: Graph lacks %q:\n%s", want, strings.Join(graph, "\n"))
				}
			}
			checkValues(t, p, "EVALUATION.md §4.2", map[string]string{"p:same": "false"})
		})
	}
}

// DECISIONS 478: second's x converts to bb's let path, whose unwritten oo holds it, without a
// layer and under one leaving it, active or checked inactive (EVALUATION.md 9.1); a layer amending
// it makes bb's x bb's own, and the conversion is the membership finding.
func TestSpreadLetPathHoldsSourceEntry(t *testing.T) {
	src := spreadLayerSrc + "\nlet r2: ref bb.oo = aa.second.oo.x\n"
	for _, c := range []struct {
		body string
		on   bool
		want bool
	}{
		{"", false, false},
		{"amend bb {\n  id: \"z\"\n}\n", false, false},
		{"amend bb {\n  id: \"z\"\n}\n", true, false},
		{"amend bb {\n  oo.x.w: 5\n}\n", true, true},
	} {
		res, err := openLayered(t, src, c.body, c.on).Check(context.Background(), "p")
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.ContainsFunc(res.Findings, isCode(diag.E3503.Def().Code)); got != c.want {
			t.Errorf("DECISIONS 478: layer %q active %v: %s %v, want %v", c.body, c.on, diag.E3503.Def().Code, got, c.want)
		}
	}
}

// copiedDefaultSrc is p: h applies H's defaults, hh spreads h, k applies its own.
const copiedDefaultSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record H {\n  id: String\n  tag: String = self.id\n  oo: [S] keyed by name = [{ name: \"x\" }]\n}\n\n" +
	"let h: H = { id: \"h\" }\n\nlet hh: H = { ...h, id: \"hh\" }\n\nlet k: H = { id: \"k\" }\n\n" +
	"let same: Bool = hh.oo.x == h.oo.x\n"

// EVALUATION.md 9.3 step 3: a field a spread copied counts as written, so amending the copy
// re-derives only the defaults it applied itself.
func TestLayerSpreadCopyKeepsCopiedDefaults(t *testing.T) {
	p := openLayered(t, copiedDefaultSrc, "amend hh {\n  id: \"q\"\n}\n\namend k {\n  id: \"q\"\n}\n", true)
	checkValues(t, p, "EVALUATION.md §9.3", map[string]string{"p:same": "true", "p:hh.tag": "h", "p:k.tag": "q"})
}

// layerRefsOf is the layer refs of path in p.
func layerRefsOf(t *testing.T, p *canon.Project, path string) []string {
	t.Helper()
	res, err := p.Refs(context.Background(), path)
	if err != nil {
		t.Fatalf("API.md R7: Refs of %s: %v", path, err)
	}
	var out []string
	for _, r := range res.Refs {
		if r.Kind == canon.RefLayer {
			out = append(out, fmt.Sprintf("%s:%d", r.File, r.Line))
		}
	}
	return out
}

// DECISIONS 484, 486, 489: active, `amend bb { oo.x.w: 1 }` reaches bb's own x, so its `.x` is a
// layer ref of bb's x once, and of second's x once, whose key bb's x carries (DECISIONS
// 491, amending 486); a line amending only bb's id leaves second's x reached as itself.
func TestLayerSpreadCopyLineKeyRef(t *testing.T) {
	const line5 = "p/dev.layer.canon:5"
	p := openSpreadLayer(t, `oo.x.w: 1`, true)
	if got := layerRefsOf(t, p, "p:aa[second].oo[x]"); !slices.Equal(got, []string{line5}) {
		t.Errorf("DECISIONS 486, 489: second's x has layer refs %v, want %s once", got, line5)
	}
	if got := layerRefsOf(t, p, "p:bb.oo[x]"); !slices.Equal(got, []string{line5}) {
		t.Errorf("DECISIONS 484: bb's x has layer refs %v, want %s once", got, line5)
	}
	p = openLayered(t, spreadLayerSrc, "amend bb {\n  id: \"q\"\n}\n\namend aa {\n  second.oo.x.w: 1\n}\n", true)
	if got, want := layerRefsOf(t, p, "p:aa[second].oo[x]"), []string{"p/dev.layer.canon:9"}; !slices.Equal(got, want) {
		t.Errorf("DECISIONS 484: second's x has layer refs %v, want %v", got, want)
	}
}
