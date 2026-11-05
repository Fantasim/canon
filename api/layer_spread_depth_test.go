package canon_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// spreadDepthSrc is p: b5 and c5 spread b1 and c1, whose entries x and y sit two and three levels
// down; pi is a plain copy of b1's inner record and hb holds one in a field. Each copy has r* (a
// let-path conversion of the source's x), s* (x == the source's), w* (x's w), k* (m's keys hold x).
const spreadDepthSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record I {\n  id: String\n  oo: [S] keyed by name\n  m: {ref S: Int} = { x: 1 }\n}\n\n" +
	"record B {\n  id: String\n  inner: I\n}\n\n" +
	"record C {\n  id: String\n  b: B\n}\n\n" +
	"let b1: B = { id: \"b1\", inner: { id: \"i\", oo: [{ name: \"x\" }, { name: \"y\" }] } }\n\n" +
	"let b5: B = { ...b1, id: \"b5\" }\n\n" +
	"let r5: ref b5.inner.oo = b1.inner.oo.x\n\n" +
	"let s5: Bool = b5.inner.oo.x == b1.inner.oo.x\n\n" +
	"let w5: Int = b5.inner.oo.x.w\n\n" +
	"let c1: C = { id: \"c1\", b: { id: \"b\", inner: { id: \"i\", oo: [{ name: \"x\" }, { name: \"y\" }] } } }\n\n" +
	"let c5: C = { ...c1, id: \"c5\" }\n\n" +
	"let r3: ref c5.b.inner.oo = c1.b.inner.oo.x\n\n" +
	"let s3: Bool = c5.b.inner.oo.x == c1.b.inner.oo.x\n\n" +
	"let w3: Int = c5.b.inner.oo.x.w\n\n" +
	"let pi: I = b1.inner\n\n" +
	"let rp: ref pi.oo = b1.inner.oo.x\n\n" +
	"let sp: Bool = pi.oo.x == b1.inner.oo.x\n\n" +
	"let wp: Int = pi.oo.x.w\n\n" +
	"let hb: B = { id: \"hb\", inner: b1.inner }\n\n" +
	"let rh: ref hb.inner.oo = b1.inner.oo.x\n\n" +
	"let sh: Bool = hb.inner.oo.x == b1.inner.oo.x\n\n" +
	"let wh: Int = hb.inner.oo.x.w\n\n" +
	"let kb5: [Bool] = [b5.inner.m.keys().contains(b1.inner.oo.x), b5.inner.m.keys().contains(b5.inner.oo.x)]\n\n" +
	"let kc5: [Bool] = [c5.b.inner.m.keys().contains(c1.b.inner.oo.x), c5.b.inner.m.keys().contains(c5.b.inner.oo.x)]\n\n" +
	"let kpi: [Bool] = [pi.m.keys().contains(b1.inner.oo.x), pi.m.keys().contains(pi.oo.x)]\n\n" +
	"let khb: [Bool] = [hb.inner.m.keys().contains(b1.inner.oo.x), hb.inner.m.keys().contains(hb.inner.oo.x)]\n"

// spreadDepthCase is one layer line amending the copy at root, and what it gives with the layer
// active: whether the copy's x is its own (s false, E3503 on the conversion, a node of its own),
// the w read through it, and the nodes the graph gains.
type spreadDepthCase struct {
	root, line string
	own        bool
	w          string
	gains      []string
}

// spreadDepthView is p's graph below its roots and the refs of b1's and c1's x, sorted.
func spreadDepthView(t *testing.T, p *canon.Project) (graph, refs []string) {
	t.Helper()
	nodes, edges := graphOf(t, p)
	for _, n := range nodes {
		if n.Package == "p" && n.Kind != canon.NodeRoot {
			graph = append(graph, fmt.Sprintf("%s %s", n.Kind, n.ID))
		}
	}
	for _, e := range edges {
		if strings.HasPrefix(e.To, "p:") {
			graph = append(graph, fmt.Sprintf("%s %s %s %s", e.Kind, e.From, e.Path, e.To))
		}
	}
	slices.Sort(graph)
	for _, path := range []string{"p:b1.inner.oo[x]", "p:c1.b.inner.oo[x]"} {
		res, err := p.Refs(context.Background(), path)
		if err != nil {
			t.Fatalf("API.md R7: Refs of %s: %v", path, err)
		}
		for _, r := range res.Refs {
			refs = append(refs, fmt.Sprintf("%s %v %s:%d", path, r.Kind, r.File, r.Line))
		}
	}
	return graph, refs
}

// conversionFails reports the E3503 of the let converting a source's x, at line, in p's check.
func conversionFails(t *testing.T, p *canon.Project, let string) bool {
	t.Helper()
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(res.Findings, func(f canon.Finding) bool {
		return f.Code == string(diag.E3503.Def().Code) && f.Path == let
	})
}

// DECISIONS 486, 478 at any depth: a line amending a sibling field of a spread or plain copy, or
// of its nested record, leaves its untouched entries the source's; one amending at or below an
// entry, or replacing the record holding them, makes them the copy's own, active or inactive.
func TestLayerSpreadCopyAtDepth(t *testing.T) {
	for _, c := range spreadDepthCases {
		t.Run(c.root+" "+c.line, func(t *testing.T) { spreadDepthRow(t, c, spreadDepthOpener(spreadDepthSrc, nil)) })
	}
}

// spreadDepthCases is each layer line TestLayerSpreadCopyAtDepth amends a copy with.
var spreadDepthCases = []spreadDepthCase{
	{"b5", `inner.id: "z"`, false, "0", nil},
	{"b5", `inner.oo.x.w: 5`, true, "5", []string{"element p:b5.inner.oo[x]"}},
	{"b5", `inner: { id: "z", oo: [{ name: "x", w: 9 }] }`, true, "9", []string{"element p:b5.inner.oo[x]"}}, // regression guard: passes on the base
	{"c5", `b.inner.id: "z"`, false, "0", nil},
	{"c5", `b.id: "z"`, false, "0", nil}, // regression guard: passes on the base
	{"c5", `b.inner.oo.x.w: 5`, true, "5", []string{"element p:c5.b.inner.oo[x]"}},
	{"pi", `id: "z"`, false, "0", nil},
	{"pi", `oo.x.w: 5`, true, "5", []string{"element p:pi.oo[x]"}},
	{"hb", `inner.id: "z"`, false, "0", nil},
	{"hb", `inner.oo.x.w: 5`, true, "5", []string{"element p:hb.inner.oo[x]"}},
	{"pi", `m[x]: 3`, false, "0", nil},
	{"hb", `inner.m[x]: 3`, false, "0", nil},
	{"b5", `inner.m[x]: 3`, false, "0", nil},
	{"c5", `b.inner.m[x]: 3`, false, "0", nil},
}

// spreadDepthOpener opens src, with the files more beside it, under a layer dev of body, active when on.
func spreadDepthOpener(src string, more map[string]string) func(t *testing.T, body string, on bool) *canon.Project {
	return func(t *testing.T, body string, on bool) *canon.Project {
		t.Helper()
		return openLayeredWith(t, src, more, body, on)
	}
}

// spreadDepthRow checks c opened by open: the graph and refs active are those inactive plus what c
// gains, the conversion fails exactly when the copy's x is its own, and the reads.
func spreadDepthRow(t *testing.T, c spreadDepthCase, open func(t *testing.T, body string, on bool) *canon.Project) {
	body := "amend " + c.root + " {\n  " + c.line + "\n}\n"
	off, on := open(t, body, false), open(t, body, true)
	graph0, refs0 := spreadDepthView(t, off)
	graph1, refs1 := spreadDepthView(t, on)
	graph0 = append(graph0, c.gains...)
	slices.Sort(graph0)
	if !slices.Equal(nodesOf(graph0), nodesOf(graph1)) {
		t.Errorf("DECISIONS 478: Graph nodes =\n%s\nwith the layer, want\n%s", strings.Join(nodesOf(graph1), "\n"), strings.Join(nodesOf(graph0), "\n"))
	}
	if !c.own && (!slices.Equal(graph0, graph1) || !slices.Equal(refs0, refs1)) {
		t.Errorf("DECISIONS 410: Graph and Refs with the layer =\n%s\n%s\nwant, as without\n%s\n%s",
			strings.Join(graph1, "\n"), strings.Join(refs1, "\n"), strings.Join(graph0, "\n"), strings.Join(refs0, "\n"))
	}
	reads := map[string][4]string{
		"b5": {"r5", "p:s5", "p:w5", "p:kb5"}, "c5": {"r3", "p:s3", "p:w3", "p:kc5"},
		"pi": {"rp", "p:sp", "p:wp", "p:kpi"}, "hb": {"rh", "p:sh", "p:wh", "p:khb"},
	}[c.root]
	let, same, w, keys := reads[0], reads[1], reads[2], reads[3]
	for _, p := range []*canon.Project{off, on} {
		if got := conversionFails(t, p, let); got != c.own {
			t.Errorf("DECISIONS 486: %s on %s %v, want %v", diag.E3503.Def().Code, let, got, c.own)
		}
	}
	// m's key ref names the x the copy holds: the source's, or the copy's own (DECISIONS 410)
	wantKeys := fmt.Sprintf("[%v, true]", !c.own)
	checkValues(t, on, "DECISIONS 486", map[string]string{same: fmt.Sprint(!c.own), w: c.w, keys: wantKeys})
	checkValues(t, off, "DECISIONS 410", map[string]string{same: "true", w: "0", keys: "[true, true]"})
}

// nodesOf is the node lines of a sorted graph view.
func nodesOf(graph []string) []string {
	return slices.DeleteFunc(slices.Clone(graph), func(s string) bool { return !strings.HasPrefix(s, "element ") })
}
