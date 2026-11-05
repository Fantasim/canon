package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// The targets and referring values of spreadLayerSrc: bb's x, bb's own under a layer amending
// it, second's x and y, and rr, cc's and pk's refs converted from bb's x, computed.
const (
	bbX     = "p:bb.oo[x]"
	secondX = "p:aa[second].oo[x]"
	secondY = "p:aa[second].oo[y]"
	edgeRR  = "p:rr true"
	edgeCC  = "p:cc.subs[0].pick true"
	edgePK  = "p:pk.pick true"
)

// defaultedSrc is p: h's oo takes its field's default, whose x rr and pk convert to refs, and h's
// subs its default, whose pick states x in the default (a code ref only, API.md R7).
const defaultedSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record P {\n  pick: ref S\n}\n\n" +
	"record H {\n  id: String\n  oo: [S] keyed by name = [{ name: \"x\" }]\n  subs: [P] = [{ pick: \"x\" }]\n}\n\n" +
	"let h: H = { id: \"h\" }\n\nlet rr: ref h.oo = h.oo.x\n\nlet pk: P = { pick: h.oo.x }\n"

// nestedHolderSrc is spreadLayerSrc with b1, whose inner record's oo holds x, which rn refers to.
const nestedHolderSrc = spreadLayerSrc + "\nrecord B {\n  id: String\n  inner: A\n}\n\n" +
	"let b1: B = { id: \"b1\", inner: { id: \"in\", oo: [{ name: \"x\" }] } }\n\n" +
	"let rn: ref b1.inner.oo = b1.inner.oo.x\n"

// edgesTo is, for each id, its value and key edges in p's graph, as `<Path> <Computed>` in R14
// order; each id's are checked to be the value and key refs Refs lists for it (API.md R11).
func edgesTo(t *testing.T, p *canon.Project, ids ...string) map[string][]string {
	t.Helper()
	_, edges := graphOf(t, p)
	out := map[string][]string{}
	for _, id := range ids {
		var to []*canon.GraphEdge
		for _, e := range edges {
			if e.To == id {
				to = append(to, e)
				out[id] = append(out[id], fmt.Sprintf("%s %v", e.Path, e.Computed))
			}
		}
		checkEdgesTo(t, p, id, to)
	}
	return out
}

// layerEdgeRow is a layer body, active or not, and the edges wanted to each target.
type layerEdgeRow struct {
	body string
	on   bool
	want map[string][]string
}

// checkLayerEdges fails for each row of src whose graph does not have exactly the wanted edges.
func checkLayerEdges(t *testing.T, rule, src string, rows []layerEdgeRow) {
	t.Helper()
	for _, r := range rows {
		ids := slices.Sorted(maps.Keys(r.want))
		got := edgesTo(t, openLayered(t, src, r.body, r.on), ids...)
		for _, id := range ids {
			if !slices.Equal(got[id], r.want[id]) {
				t.Errorf("%s: layer %q active %v: edges to %s =\n%s\nwant\n%s", rule, r.body, r.on, id,
					strings.Join(got[id], "\n"), strings.Join(r.want[id], "\n"))
			}
		}
	}
}

// amendBB is a layer body amending bb by line.
func amendBB(line string) string {
	return "amend bb {\n  " + line + "\n}\n"
}

// API.md R7, R11 (DECISIONS 238, 478, 486): a layer line writing bb's oo as a list literal gives bb
// its own x; rr, cc's and pk's refs converted from it are its computed value refs and edges,
// whatever file the literal they were converted from is written in.
func TestLayerWrittenListRefs(t *testing.T) {
	body := amendBB(`oo: [{ name: "x", w: 7 }]`)
	checkLayerEdges(t, "API.md R11", spreadLayerSrc, []layerEdgeRow{
		{body, true, map[string][]string{bbX: {edgeRR, edgeCC, edgePK}, secondX: nil}},
		{body, false, map[string][]string{secondX: {edgeRR, edgeCC, edgePK}}},
	})
}

// API.md R7, R11 (DECISIONS 238, 483, 489): a ref a layer line writes, computed from an entry, is
// a computed value ref of that entry at its let's name, and the base value it replaced one of its
// own; a ref token the line states is a `layer` ref, no edge (API.md R7, R11; DECISIONS 483).
func TestLayerWrittenRefValue(t *testing.T) {
	body := "amend pk {\n  pick: aa.second.oo.y\n}\n"
	checkLayerEdges(t, "API.md R11", spreadLayerSrc, []layerEdgeRow{
		{body, true, map[string][]string{secondY: {edgePK}, secondX: {edgeRR, edgeCC, edgePK}}},
		{body, false, map[string][]string{secondY: nil, secondX: {edgeRR, edgeCC, edgePK}}},
	})
	p := openLayered(t, spreadLayerSrc, "amend cc {\n  subs[0].pick: \"y\"\n}\n", true)
	if got := edgesTo(t, p, "p:cc.oo[y]"); len(got) != 0 {
		t.Errorf("API.md R11: a layer's stated ref has edges %v, want none", got)
	}
	if got, want := layerRefsOf(t, p, "p:cc.oo[y]"), []string{"p/dev.layer.canon:5"}; !slices.Equal(got, want) {
		t.Errorf("API.md R7: cc's y has layer refs %v, want %v", got, want)
	}
}

// API.md R11 (DECISIONS 478): an entry of a keyed list in a nested record field is a node at any
// depth, with the layer active or not, and rn's computed ref is its edge.
func TestGraphNestedHolderUnderLayer(t *testing.T) {
	var rows []layerEdgeRow
	for _, body := range []string{"", "amend b1 {\n  inner.oo.x.w: 4\n}\n", "amend b1 {\n  inner.id: \"z\"\n}\n", "amend b1 {\n  id: \"q\"\n}\n"} {
		for _, on := range []bool{true, false} {
			rows = append(rows, layerEdgeRow{body, on, map[string][]string{"p:b1.inner.oo[x]": {"p:rn true"}}})
		}
	}
	checkLayerEdges(t, "DECISIONS 478", nestedHolderSrc, rows)
}

// API.md R7, R11, E12 (DECISIONS 238, 478, 483, 489): rr and pk convert h's x, which h's oo takes
// from its field's default: computed value refs and edges, without a layer and under one; the
// default's own stated `pick: "x"` is a code ref, no edge; renaming x is refused `computed`.
func TestRefsFromDefaultedField(t *testing.T) {
	want := map[string][]string{"p:h.oo[x]": {"p:rr true", "p:pk.pick true"}}
	body := "amend h {\n  id: \"q\"\n}\n"
	checkLayerEdges(t, "API.md R11", defaultedSrc, []layerEdgeRow{{"", false, want}, {body, false, want}, {body, true, want}})
	p := openLayered(t, defaultedSrc, "", false)
	res, err := p.Refs(context.Background(), "p:h.oo[x]")
	if err != nil {
		t.Fatalf("API.md R7: Refs: %v", err)
	}
	code := slices.ContainsFunc(res.Refs, func(r canon.Ref) bool {
		return r.Kind == canon.RefCode && r.File == srcMain && r.Line == lineOf(defaultedSrc, "  subs: [P]")
	})
	if !code {
		t.Errorf("API.md R7: Refs of p:h.oo[x] lack the default's `pick: \"x\"` code ref: %+v", res.Refs)
	}
	_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("p:h.oo[x]", canon.Key("z"))}})
	var ne *canon.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonComputed {
		t.Errorf("API.md E12: renaming p:h.oo[x]: %v, want a refusal %q", err, canon.ReasonComputed)
	}
}

// API.md R7, R11 (DECISIONS 238, 486): a layer line replacing bb's x whole, by an existing entry or
// a literal, makes bb's x its own: rr, cc's and pk's refs converted from it are its value refs and
// edges; inactive, they stay second's x's.
func TestLayerReplacedEntryRefs(t *testing.T) {
	var rows []layerEdgeRow
	for _, line := range []string{`oo.x: aa.second.oo.x`, `oo.x: aa.first.oo.x`, `oo.x: { name: "x", w: 9 }`} {
		rows = append(rows,
			layerEdgeRow{amendBB(line), true, map[string][]string{bbX: {edgeRR, edgeCC, edgePK}}},
			layerEdgeRow{amendBB(line), false, map[string][]string{secondX: {edgeRR, edgeCC, edgePK}}})
	}
	checkLayerEdges(t, "API.md R11", spreadLayerSrc, rows)
}

// API.md R7, R11 (DECISIONS 238): rr's one token reaches second's x in the base evaluation and bb's
// own x in the layered one: both refs are listed, each under its target, and both edges have
// rr's Path.
func TestLayerRefBaseAndLayeredTargets(t *testing.T) {
	checkLayerEdges(t, "DECISIONS 238", spreadLayerSrc, []layerEdgeRow{
		{amendBB(`oo.x.w: 5`), true, map[string][]string{bbX: {edgeRR, edgeCC, edgePK}, secondX: {edgeRR}}},
	})
}

// copiedSrc is spreadLayerSrc with D, whose st dd and d2 set from bb's x and y, rr2 holding bb's y,
// and M, whose mood mm holds calm, m2 holding angry: values a layer line copies by name.
const copiedSrc = spreadLayerSrc + "\nrecord D {\n  st: ref bb.oo\n}\n\n" +
	"let rr2: ref bb.oo = bb.oo.y\n\nlet dd: D = { st: bb.oo.x }\n\nlet d2: D = { st: bb.oo.y }\n\n" +
	"enum Mood { calm, angry }\n\nrecord M {\n  mood: Mood\n}\n\nlet mm: M = { mood: calm }\n\nlet m2: Mood = angry\n"

// API.md R7, R11 (DECISIONS 481, 489): a layer line copying a ref or member through a let or field,
// parens or not, states no key: a computed value ref at its let's name and an edge, the base value
// it replaced one of its own; a member name the line states, in parens or not, stays a `layer` ref.
func TestLayerCopiedValueRefs(t *testing.T) {
	const angry, calm = "p:Mood.angry", "p:Mood.calm"
	base := []string{edgeRR, edgeCC, edgePK, "p:dd.st true"}
	var rows []layerEdgeRow
	for _, line := range []string{"st: rr2", "st: d2.st", "st: ((rr2))"} {
		body := "amend dd {\n  " + line + "\n}\n"
		rows = append(rows,
			layerEdgeRow{body, true, map[string][]string{secondY: {"p:rr2 true", "p:dd.st true", "p:d2.st true"}, secondX: base}},
			layerEdgeRow{body, false, map[string][]string{secondY: {"p:rr2 true", "p:d2.st true"}, secondX: base}})
	}
	m2 := "amend mm {\n  mood: m2\n}\n"
	rows = append(rows,
		layerEdgeRow{m2, true, map[string][]string{angry: {"p:mm.mood true", "p:m2 false"}, calm: {"p:mm.mood false"}}},
		layerEdgeRow{m2, false, map[string][]string{angry: {"p:m2 false"}, calm: {"p:mm.mood false"}}})
	checkLayerEdges(t, "API.md R11", copiedSrc, rows)
	for _, c := range []struct {
		value string
		on    bool
	}{{"angry", true}, {"angry", false}, {"(angry)", true}, {"((angry))", true}, {"(angry)", false}} {
		p := openLayered(t, copiedSrc, "amend mm {\n  mood: "+c.value+"\n}\n", c.on)
		if got, want := layerRefsOf(t, p, angry), []string{"p/dev.layer.canon:5"}; !slices.Equal(got, want) {
			t.Errorf("API.md R7: mood: %s active %v: angry has layer refs %v, want %v", c.value, c.on, got, want)
		}
		if got := edgesTo(t, p, angry)[angry]; !slices.Equal(got, []string{"p:m2 false"}) {
			t.Errorf("API.md R11: mood: %s active %v: edges to angry %v, want m2's only", c.value, c.on, got)
		}
	}
}

// boardSrc is a board2 that a function builds, whose start and weights key name statuses' entries.
const boardSrc = "\n/// A board.\nrecord Board {\n  /// Start.\n  start: ref statuses\n  /// Weights.\n" +
	"  weights: {ref statuses: Int}\n}\n\n/// Makes a board.\nfn mk() -> Board {\n" +
	"  return { start: open, weights: { done: 2 } }\n}\n\n/// Board 2.\nlet board2: Board = mk()\n"

// API.md R7, R8, R11 (DECISIONS 238, 489): computed refs merge by span, kind and path: with the
// layer active, board2's layered start and its base weights key both name done at board2's name,
// both listed, the value first.
func TestRefsComputedKindsAtOneName(t *testing.T) {
	law := maps.Clone(refLaw)
	law["a/a.canon"] += boardSrc
	law["a/dev.layer.canon"] = "package a\nlayer dev\n\namend board2 {\n  start: statuses.done\n}\n"
	const done = "a:statuses.done"
	for _, c := range []struct {
		on   bool
		want []string
	}{{false, []string{"a:board2.weights[done] true"}}, {true, []string{"a:board2.start true", "a:board2.weights[done] true"}}} {
		opts := project(law)
		if c.on {
			opts.Layers = []string{"dev"}
		}
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		var got []string
		for _, e := range edgesTo(t, p, done)[done] {
			if strings.HasPrefix(e, "a:board2") {
				got = append(got, e)
			}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("DECISIONS 489: active %v: board2's edges to %s = %v, want %v", c.on, done, got, c.want)
		}
	}
}
