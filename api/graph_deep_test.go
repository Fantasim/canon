package canon_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// graphDeepRecords are the records of deepKeyedSrc and deepTableSrc: S holds Q entries (%[1]s) that
// its bound level-1 ref pick and each Q's nx name; A and P hold S entries (%[5]s); Holder's
// applied-record field p defaults to %[2]s; %[3]s, %[4]s and %[6]s are Q's, S's and A's keys.
const graphDeepRecords = `package p

record Q {
  w: Int = 0%[3]s
  nx: ref Q
}

record S {
  inn: %[1]s
  pick: ref Q%[4]s
}

record A {
  oo: %[5]s%[6]s
}

record Ev {
  max: Int = 3
}

record P(e: Ev) {
  oo: %[5]s
}

record Holder {
  ev: Ev = {}
  p: P(ev) = { oo: %[2]s }
}

`

// deepKeyedSrc holds keyed lists three levels deep: aa's elements hold S elements holding Q
// elements; one holds them at level 1, two is one spread, a holds the shared let s.
var deepKeyedSrc = fmt.Sprintf(graphDeepRecords, "[Q] keyed by k", `[{ name: "d", inn: [{ k: "q", nx: "q" }], pick: "q" }]`,
	"\n  k: String", "\n  name: String", "[S] keyed by name", "\n  id: String") + `let aa: [A] keyed by id = [
  { id: "first", oo: [{ name: "x", inn: [{ k: "q", nx: "q" }], pick: "q" }] },
  { id: "second", oo: [{ name: "x", inn: [{ k: "q", nx: "q" }, { k: "r", nx: "q" }], pick: "r" }] },
]

let one: A = { id: "one", oo: [{ name: "x", inn: [{ k: "q", nx: "q" }], pick: "q" }] }

let two: A = { ...one, id: "two" }

let s: S = { name: "y", inn: [{ k: "q", nx: "q" }], pick: "q" }

let a: A = { id: "a", oo: [s] }

let holder: Holder = {}
`

// deepTableSrc is deepKeyedSrc with tables; a table entry is a literal, so a.oo.y spreads s.
var deepTableSrc = fmt.Sprintf(graphDeepRecords, "table Q", `{ d { inn: { q { nx: q } }, pick: q } }`,
	"", "", "table S", "\n  tag: String = \"\"") + `let aa: table A = {
  first { oo: { x { inn: { q { nx: q } }, pick: q } } }
  second { oo: { x { inn: { q { nx: q }, r { nx: q } }, pick: r } } }
}

let one: A = { tag: "one", oo: { x { inn: { q { nx: q } }, pick: q } } }

let two: A = { ...one, tag: "two" }

let s: S = { inn: { q { nx: q } }, pick: q }

let a: A = { tag: "a", oo: { y { ...s } } }

let holder: Holder = {}
`

// deepWant is TestGraphDeepEntries' nodes and edges below the roots, sorted: kind is the node
// kind of an entry, seg how a path names key k.
func deepWant(kind string, seg func(k string) string) []string {
	ax, bx, ox, hd := "aa"+seg("first")+".oo"+seg("x"), "aa"+seg("second")+".oo"+seg("x"), "one.oo"+seg("x"), "holder.p.oo"+seg("d")
	want := []string{
		kind + " p:aa" + seg("first") + " p.A", kind + " p:" + ax + " p.S", kind + " p:" + ax + ".inn" + seg("q") + " p.Q",
		kind + " p:aa" + seg("second") + " p.A", kind + " p:" + bx + " p.S",
		kind + " p:" + bx + ".inn" + seg("q") + " p.Q", kind + " p:" + bx + ".inn" + seg("r") + " p.Q",
		kind + " p:" + ox + " p.S", kind + " p:" + ox + ".inn" + seg("q") + " p.Q",
		kind + " p:s.inn" + seg("q") + " p.Q", kind + " p:a.oo" + seg("y") + " p.S",
		kind + " p:" + hd + " p.S", kind + " p:" + hd + ".inn" + seg("q") + " p.Q",
		"value p:" + ax + " p:" + ax + ".pick p:aa[*].oo[*].pick p:" + ax + ".inn" + seg("q") + " false",
		"value p:" + bx + " p:" + bx + ".pick p:aa[*].oo[*].pick p:" + bx + ".inn" + seg("r") + " false",
		"value p:" + ox + " p:" + ox + ".pick p:one.oo[*].pick p:" + ox + ".inn" + seg("q") + " false",
		"value p:s p:s.pick p:s.pick p:s.inn" + seg("q") + " false",
	}
	for _, q := range []struct{ from, to, pattern string }{
		{ax + ".inn" + seg("q"), ax + ".inn" + seg("q"), "aa[*].oo[*].inn[*]"},
		{bx + ".inn" + seg("q"), bx + ".inn" + seg("q"), "aa[*].oo[*].inn[*]"},
		{bx + ".inn" + seg("r"), bx + ".inn" + seg("q"), "aa[*].oo[*].inn[*]"},
		{ox + ".inn" + seg("q"), ox + ".inn" + seg("q"), "one.oo[*].inn[*]"},
		{"s.inn" + seg("q"), "s.inn" + seg("q"), "s.inn[*]"},
	} {
		want = append(want, "value p:"+q.from+" p:"+q.from+".nx p:"+q.pattern+".nx p:"+q.to+" false")
	}
	slices.Sort(want)
	return want
}

// API.md R11, R12, R13, P10, DECISIONS 410, 473: a node per entry at any depth, at its identity's
// path only (none under two or a, holder.p's included); one edge per bound ref, from its nearest
// node; every node reads back through Value, and its edges are its Refs' value and key refs.
func TestGraphDeepEntries(t *testing.T) {
	for _, c := range []struct {
		name, src, kind string
		seg             func(k string) string
	}{
		{"keyed list", deepKeyedSrc, "element", func(k string) string { return "[" + k + "]" }},
		{"table", deepTableSrc, "entry", func(k string) string { return "." + k }},
	} {
		t.Run(c.name, func(t *testing.T) {
			checkGraphOfP(t, c.src, deepWant(c.kind, c.seg))
		})
	}
}

// checkGraphOfP compares package p's nodes below the roots and its edges, sorted, with want; every
// node of p reads back through Value, and its edges are its Refs' value and key refs (R11, R14).
func checkGraphOfP(t *testing.T, src string, want []string) {
	t.Helper()
	p, _ := openEdit(t, srcLaw(src))
	nodes, edges := graphOf(t, p)
	var got []string
	for _, n := range nodes {
		if n.Package == "p" && n.Kind != canon.NodeRoot {
			got = append(got, fmt.Sprintf("%s %s %s", n.Kind, n.ID, n.Type))
		}
	}
	for _, e := range edges {
		if strings.HasPrefix(e.To, "p:") {
			got = append(got, fmt.Sprintf("%s %s %s %s %s %v", e.Kind, e.From, e.Path, e.Pattern, e.To, e.Computed))
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("R11: Graph =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	byTo := map[string][]*canon.GraphEdge{}
	for _, e := range edges {
		byTo[e.To] = append(byTo[e.To], e)
	}
	for _, n := range nodes {
		if n.Package != "p" {
			continue
		}
		checkNodeValue(t, p, n)
		if n.Kind != canon.NodeRoot {
			checkEdgesTo(t, p, n.ID, byTo[n.ID])
		}
	}
}

// sharedRecords are records and functions for TestGraphSharedRecordHome: mk, mt and mtt compute a
// record holding a keyed list or a table.
const sharedRecords = `package p

record S {
  name: String
}

record Pl {
  pl: [S] keyed by name = []
}

record Tg {
  tag: String
  pl: [S] keyed by name = []
}

record At {
  oo: [Tg] keyed by tag
}

record Tt {
  tt: table S
}

record Ah {
  h: Tt
}

record Wr {
  w: Tg
}

fn mk(n: String) -> Pl {
  return { pl: [{ name: n }] }
}

fn mt() -> Tg {
  return { tag: "t", pl: [{ name: "x" }] }
}

fn mtt() -> Tt {
  return { tt: { x { name: "x" } } }
}

`

// API.md R11, R7, DECISIONS 410: a record a let computes is at home in that let, never where
// another let holds it by name (an element, a field or the whole let, declared first or not);
// a spread copy's let path names its source's entries, in both directions.
func TestGraphSharedRecordHome(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      []string
	}{
		{"element of a computed let", "let at: At = { oo: [t] }\nlet t: Tg = mt()\nlet r: ref t.pl = t.pl.x\n", []string{
			"element p:at.oo[t] p.Tg", "element p:t.pl[x] p.S", "value p:r p:r p:r p:t.pl[x] true",
		}},
		{"let read before its source", "let aw: Pl = z\nlet z: Pl = mk(\"x\")\n", []string{"element p:z.pl[x] p.S"}},
		{"let read from a field", "let a3: Tg = wr.w\nlet wr: Wr = { w: mt() }\n", []string{"element p:wr.w.pl[x] p.S"}},
		{"chain of lets", "let a4: Pl = a5\nlet a5: Pl = z0\nlet z0: Pl = mk(\"x\")\n", []string{"element p:z0.pl[x] p.S"}},
		{"field of a computed let", "let ah: Ah = { h: u }\nlet u: Tt = mtt()\nlet ru: ref u.tt = u.tt.x\n", []string{
			"entry p:u.tt.x p.S", "value p:ru p:ru p:ru p:u.tt.x true",
		}},
		{"spread copy", "let one: Tg = { tag: \"t\", pl: [{ name: \"x\" }] }\nlet two: Tg = { ...one }\n" +
			"let r1: ref two.pl = one.pl.x\nlet r2: ref one.pl = two.pl.x\n", []string{
			"element p:one.pl[x] p.S", "value p:r1 p:r1 p:r1 p:one.pl[x] true", "value p:r2 p:r2 p:r2 p:one.pl[x] true",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			checkGraphOfP(t, sharedRecords+c.src, c.want)
		})
	}
}
