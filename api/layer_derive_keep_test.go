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

// deriveKeepSrc is p, held at one place: oo, mk, dsub and sv read no field, tag reads id, m n and yw
// oo; r, picks, same, pw, hasx, dsame and dw read p's entries. two holds one P at two places; the
// defaulted sel of w's inner holds a ref into w's oo, which ws and wsw read.
const deriveKeepSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord R {\n  pick: ref p.oo\n}\n\nrecord D {\n  pick: ref S\n}\n\n" +
	"record P {\n  id: String\n  n: Int = 1\n  oo: [S] keyed by k = [{ k: \"x\" }, { k: \"y\" }]\n  tag: String = self.id\n  m: Int = self.n + 1\n  mk: {ref S: Int} = { x: 1 }\n  dsub: [D] = [{ pick: y }]\n  yw: Int = self.oo.y.w\n  sv: V = {}\n}\n\nrecord V {\n  a: Int = 1\n  b: Int = 2\n  c: Int = self.a + 1\n}\n\n" +
	"record T2 {\n  l: [P]\n}\n\nfn dup(i: P) -> [P] {\n  return [i, i]\n}\n\n" +
	"let p: P = { id: \"a\" }\n\nlet r: ref p.oo = p.oo.x\n\nlet same: Bool = r == p.oo.x\n\n" +
	"let picks: [R] = [{ pick: p.oo.y }]\n\nlet pw: Int = picks[0].pick.w\n\nlet hasx: Bool = p.mk.keys().contains(p.oo.x)\n\n" +
	"let dsame: Bool = p.dsub[0].pick == p.oo.y\n\nlet dw: Int = p.dsub[0].pick.w\n\n" +
	"let two: T2 = { l: dup({ id: \"t\" }) }\n\nlet twox: Bool = two.l[1].oo.x == two.l[0].oo.x\n\n" +
	"record E {\n  pick: ref S\n}\n\nrecord Q {\n  id: String\n  sel: [E] = [{ pick: x }]\n}\n\nrecord W {\n  oo: [S] keyed by k\n  inner: Q\n}\n\n" +
	"let w: W = { oo: [{ k: \"x\", w: 1 }], inner: { id: \"a\" } }\n\nlet ws: [Bool] = [w.inner.sel[0].pick == w.oo.x]\n\nlet wsw: Int = w.inner.sel[0].pick.w\n"

// deriveKeepView is what a row compares of p: the refs of p's entries and the graph of package p
// below its roots, sorted.
func deriveKeepView(t *testing.T, p *canon.Project) (refs, graph []string) {
	t.Helper()
	for _, path := range []string{"p:p.oo[x]", "p:p.oo[y]"} {
		res, err := p.Refs(context.Background(), path)
		if err != nil {
			t.Fatalf("API.md R7: Refs of %s: %v", path, err)
		}
		for _, r := range res.Refs {
			refs = append(refs, fmt.Sprintf("%s %v %s:%d", path, r.Kind, r.File, r.Line))
		}
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

// deriveKeepCase is a layer block and the values it leaves active.
type deriveKeepCase struct {
	name, block string
	want        map[string]string
}

// EVALUATION.md §9.3 step 3, DECISIONS 486, 501: unchanged defaults keep values, refs, Refs and Graph.
func TestLayerDeriveKeepsUnchangedDefaults(t *testing.T) {
	clean := map[string]string{"p:same": "true", "p:pw": "0", "p:hasx": "true", "p:r": "x", "p:twox": "true", "p:dsame": "true", "p:dw": "0", "p:p.yw": "0", "p:ws": "[true]", "p:wsw": "1"}
	with := func(more map[string]string) map[string]string {
		out := maps.Clone(clean)
		maps.Copy(out, more)
		return out
	}
	for _, c := range []deriveKeepCase{
		{"an unread field", "amend p {\n  id: \"z\"\n}\n", with(map[string]string{"p:p.tag": "z", "p:p.m": "2"})},
		{"a read field", "amend p {\n  n: 5\n}\n", with(map[string]string{"p:p.tag": "a", "p:p.m": "6"})},
		{"an entry, then an unread field", "amend p {\n  oo.y.w: 3\n  id: \"z\"\n}\n",
			with(map[string]string{"p:pw": "3", "p:dw": "3", "p:p.yw": "3", "p:p.tag": "z", "p:p.oo[y].w": "3"})},
		{"an outer entry, then an unread field", "amend w {\n  oo.x.w: 4\n  inner.id: \"z\"\n}\n",
			with(map[string]string{"p:wsw": "4", "p:w.inner.id": "z"})},
		{"two places", "amend two {\n  l[0].id: \"z\"\n}\n", with(map[string]string{"p:two.l[0].tag": "z", "p:two.l[1].tag": "t"})},
	} {
		t.Run(c.name, func(t *testing.T) {
			off := openLayered(t, deriveKeepSrc, c.block, false)
			on := openLayered(t, deriveKeepSrc, c.block, true)
			for _, p := range []*canon.Project{off, on} {
				if errs := errorsOf(t, p); len(errs) > 0 {
					t.Fatalf("DECISIONS 486: errors %v, want none", errs)
				}
			}
			checkValues(t, off, "DECISIONS 486", clean)
			checkValues(t, on, "EVALUATION.md §9.3", c.want)
			refs0, graph0 := deriveKeepView(t, off)
			refs1, graph1 := deriveKeepView(t, on)
			if !slices.Equal(refs0, refs1) || len(refs0) == 0 {
				t.Errorf("API.md R7: Refs of p's entries =\n%s\nwith the layer, want, as without\n%s", strings.Join(refs1, "\n"), strings.Join(refs0, "\n"))
			}
			if !slices.Equal(graph0, graph1) {
				t.Errorf("API.md R11: Graph =\n%s\nwith the layer, want, as without\n%s", strings.Join(graph1, "\n"), strings.Join(graph0, "\n"))
			}
		})
	}
}

// originText is o and the origins it came through, one per line.
func originText(o *canon.Origin) string {
	var out []string
	for ; o != nil; o = o.Via {
		out = append(out, fmt.Sprintf("%s %s:%d:%d layer %q replaced %v", o.Kind, o.File, o.Line, o.Col, o.Layer, o.Replaced != nil))
	}
	return strings.Join(out, "\n")
}

// EVALUATION.md §13, DECISIONS 464: unchanged defaults keep their origin, changed ones the layer, via the omitter.
func TestLayerDeriveKeepsProvenance(t *testing.T) {
	for _, c := range []struct {
		block   string
		kept    []string
		derived string
	}{
		{"amend p {\n  id: \"z\"\n}\n", []string{"p:p.oo", "p:p.oo[x]", "p:p.mk", "p:p.sv", "p:p.sv.b", "p:p.m"}, "p:p.tag"},
		{"amend p {\n  sv.a: 5\n}\n", []string{"p:p.sv.b", "p:p.oo", "p:p.m"}, "p:p.sv.c"},
		{"amend p {\n  n: 5\n}\n", []string{"p:p.oo", "p:p.sv.b"}, "p:p.m"},
	} {
		off := openLayered(t, deriveKeepSrc, c.block, false)
		on := openLayered(t, deriveKeepSrc, c.block, true)
		for _, path := range c.kept {
			o0, o1 := mustValue(t, off, path).Origin, mustValue(t, on, path).Origin
			if a, b := originText(&o0), originText(&o1); a != b {
				t.Errorf("EVALUATION.md §13: %s after %q has origin\n%s\nwant, as without the layer,\n%s", path, c.block, b, a)
			}
		}
		o0, o1 := mustValue(t, off, c.derived).Origin, mustValue(t, on, c.derived).Origin
		if o1.Kind != canon.OriginDefault || o1.Layer != "dev" {
			t.Errorf("DECISIONS 464: %s after %q has origin %s, want a default in layer dev", c.derived, c.block, originText(&o1))
		}
		svLine := slices.Index(strings.Split(deriveKeepSrc, "\n"), "  sv: V = {}") + 1
		if a, b := originText(o0.Via), originText(o1.Via); a != b || o1.Via == nil || o1.Via.Kind != canon.OriginLiteral ||
			c.derived == "p:p.sv.c" && o1.Via.Line != svLine {
			t.Errorf("EVALUATION.md §13: %s after %q comes via\n%s\nwant the literal that omitted it, as without the layer,\n%s", c.derived, c.block, b, a)
		}
	}
}
