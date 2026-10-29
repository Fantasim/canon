package edit_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// graphG holds every shape of API.md R11, R12, R13: retired entries and members, keyed lists in entries
// and records, plain lists, ref and member keys, cases sharing a field, a variant root, copies,
// let paths (DECISIONS 315, 316), computed refs and an optional field.
const graphG = `/// G.
package g

/// A tone.
enum Tone {
  calm
  loud
  retired old
}

/// A slot.
record Slot {
  /// Its number.
  n: Int
  /// What it points at.
  to: ref items?
}

/// An item.
record Item {
  /// Its tone.
  tone: Tone = calm
  /// Its slots.
  slots: [Slot] keyed by n = []
  /// What follows it.
  next: [ref items] = []
}

/// The items.
let items: table Item = {
  a { tone: loud, slots: [{ n: 1, to: b }, { n: 2 }] }
  b { next: [a, b] }
  retired gone {}
}

/// A zone.
record Zone {
  /// Its spawns.
  spawns: [Slot] keyed by n
  /// Its rooms.
  rooms: table Item
}

/// The zone.
let zone: Zone = { spawns: [{ n: 7, to: a }], rooms: { hall { next: [b] } } }

/// A copy of the zone.
let copy: Zone = zone

/// A holder of an entry.
record Holder {
  /// The entry it holds.
  held: Item
}

/// It holds a.
let holder: Holder = { held: items.a }

/// Refs through a let path.
let spawnRefs: [ref zone.spawns] = [7]

/// The same through the copy.
let copyRefs: [ref copy.spawns] = [7]

/// A shape.
variant Shape {
  dot { at: ref items }
  line { at: ref items, to: ref items }
}

/// Shapes.
let shapes: [Shape] = [dot { at: a }, line { at: b, to: a }]

/// A shape at the root.
let one: Shape = dot { at: b }

/// Shapes by item.
let byItem: {ref items: Shape} = { a: line { at: a, to: b } }

/// Weights by tone.
let byTone: {Tone: Int} = { loud: 1, old: 2 }

/// Picks an item.
fn pick() -> ref items {
  return b
}

/// Computed.
let picked: ref items = pick()

/// Picks a pair.
fn pair() -> {ref items: ref items} {
  return { a: b }
}

/// A computed pair, its key and value at one span.
let pairs: {ref items: ref items} = pair()

/// Plain lists of slots.
let lists: [[Slot]] = [[{ n: 3, to: b }]]
`

// graphFixture is a snapshot of graphG followed by extra, with more files, every package analyzed.
func graphFixture(t *testing.T, extra string, more mapFS) fixture {
	t.Helper()
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/g/g.canon":     file(graphG + extra),
		"law/g/d.json":      file("[\"a\", \"zz\"]\n"),
	}
	maps.Copy(fsys, more)
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, nil, false)
}

var nodeKinds = map[edit.NodeKind]string{edit.NodeRoot: "root", edit.NodeEntry: "entry", edit.NodeElement: "element", edit.NodeMember: "member"}

// graphLines renders g: a node as `N <kind> <id> <type> [retired] <line>`, an edge as
// `E <kind> <from> <path> <pattern> <to> [computed] <line> <text>`.
func graphLines(f fixture, g *edit.Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		line, _ := f.files.Position(n.Span.File, n.Span.Start)
		out = append(out, fmt.Sprintf("N %s %s %v%s %d", nodeKinds[n.Kind], n.ID, n.Type, mark(n.Retired, " retired"), line))
	}
	for _, e := range g.Edges {
		line, _ := f.files.Position(e.Span.File, e.Span.Start)
		out = append(out, fmt.Sprintf("E %s %s %s %s %s%s %d %s", refKinds[e.Kind], g.Nodes[e.From].ID, e.Path,
			g.Patterns[e.Pattern], g.Nodes[e.To].ID, mark(e.Computed, " computed"), line, f.text(e.Span)))
	}
	return out
}

func mark(on bool, text string) string {
	if on {
		return text
	}
	return ""
}

// mustGraph is Graph of f, failing the test on an error.
func mustGraph(t *testing.T, f fixture) *edit.Graph {
	t.Helper()
	g, err := f.Graph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// graphWant is graphG's graph: nodes at home only, no copy (R11); owners (R12); [*], cases and a
// key's uncased value (R13); value before key at one span (R14); computed refs at the let's name,
// a copy's refs merged by span with the original's (R7); let-path targets (DECISIONS 315, 316).
var graphWant = []string{
	"N member g:Tone.calm g.Tone 6",
	"N member g:Tone.loud g.Tone 7",
	"N member g:Tone.old g.Tone retired 8",
	"N root g:items table g.Item 30",
	"N entry g:items.a g.Item 31",
	"N element g:items.a.slots[1] g.Slot 31",
	"N element g:items.a.slots[2] g.Slot 31",
	"N entry g:items.b g.Item 32",
	"N entry g:items.gone g.Item retired 33",
	"N root g:zone g.Zone 45",
	"N element g:zone.spawns[7] g.Slot 45",
	"N entry g:zone.rooms.hall g.Item 45",
	"N root g:copy g.Zone 48",
	"N root g:holder g.Holder 57",
	"N root g:spawnRefs [ref g.zone.spawns] 60",
	"N root g:copyRefs [ref g.copy.spawns] 63",
	"N root g:shapes [g.Shape] 72",
	"N root g:one g.Shape 75",
	"N root g:byItem {ref g.items: g.Shape} 78",
	"N root g:byTone {g.Tone: Int} 81",
	"N root g:picked ref g.items 89",
	"N root g:pairs {ref g.items: ref g.items} 97",
	"N root g:lists [[g.Slot]] 100",
	"E value g:items.a g:items.a.tone g:items[*].tone g:Tone.loud 31 loud",
	"E value g:items.a.slots[1] g:items.a.slots[1].to g:items[*].slots[*].to g:items.b 31 b",
	"E value g:items.b g:items.b.next[0] g:items[*].next[*] g:items.a 32 a",
	"E value g:items.b g:items.b.next[1] g:items[*].next[*] g:items.b 32 b",
	"E value g:zone.spawns[7] g:zone.spawns[7].to g:zone.spawns[*].to g:items.a 45 a",
	"E value g:zone.rooms.hall g:zone.rooms.hall.next[0] g:zone.rooms[*].next[*] g:items.b 45 b",
	"E value g:spawnRefs g:spawnRefs[0] g:spawnRefs[*] g:zone.spawns[7] 60 7",
	"E value g:copyRefs g:copyRefs[0] g:copyRefs[*] g:zone.spawns[7] 63 7",
	"E value g:shapes g:shapes[0].at g:shapes[*]<dot>.at g:items.a 72 a",
	"E value g:shapes g:shapes[1].at g:shapes[*]<line>.at g:items.b 72 b",
	"E value g:shapes g:shapes[1].to g:shapes[*]<line>.to g:items.a 72 a",
	"E value g:one g:one.at g:one<dot>.at g:items.b 75 b",
	"E key g:byItem g:byItem[a] g:byItem[*] g:items.a 78 a",
	"E value g:byItem g:byItem[a].at g:byItem[*]<line>.at g:items.a 78 a",
	"E value g:byItem g:byItem[a].to g:byItem[*]<line>.to g:items.b 78 b",
	"E key g:byTone g:byTone[loud] g:byTone[*] g:Tone.loud 81 loud",
	"E key g:byTone g:byTone[old] g:byTone[*] g:Tone.old 81 old",
	"E value g:picked g:picked g:picked g:items.b computed 89 picked",
	"E value g:pairs g:pairs[a] g:pairs[*] g:items.b computed 97 pairs",
	"E key g:pairs g:pairs[a] g:pairs[*] g:items.a computed 97 pairs",
	"E value g:lists g:lists[0][0].to g:lists[*][*].to g:items.b 100 b",
}

// API.md R11, R12, R13, R14 on graphG, twice: the same snapshot gives the same graph.
func TestGraphNodesAndEdges(t *testing.T) {
	f := graphFixture(t, "", nil)
	first := graphLines(f, mustGraph(t, f))
	if !slices.Equal(first, graphWant) {
		t.Errorf("Graph =\n%s\nwant\n%s", strings.Join(first, "\n"), strings.Join(graphWant, "\n"))
	}
	if again := graphLines(f, mustGraph(t, f)); !slices.Equal(again, first) {
		t.Errorf("R14: a second Graph differs:\n%s", strings.Join(again, "\n"))
	}
}

// API.md R11: the graph holds what the analysis computed; a loaded ref whose key names no
// entry (E3501) is no edge, the others are.
func TestGraphDanglingRefIsNoEdge(t *testing.T) {
	f := graphFixture(t, "\n/// Loaded refs, one dangling.\nlet dangling: [ref items] = load(\"d.json\")\n", nil)
	var got []string
	for _, l := range graphLines(f, mustGraph(t, f)) {
		if strings.Contains(l, "dangling") {
			got = append(got, l)
		}
	}
	want := []string{"N root g:dangling [ref g.items] 103", `E value g:dangling g:dangling[0] g:dangling[*] g:items.a 1 "a"`}
	if !slices.Equal(got, want) {
		t.Errorf("Graph with a dangling ref =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// API.md R15: a let that could hold a ref but has no value, and a let typed Error, fail the
// graph, naming the first in (package, name) byte order; a let that holds none does not.
func TestGraphFailsOnMissingValues(t *testing.T) {
	broken := "\n/// A pool.\nlet pool: [ref items] = [a]\n\n/// Broken.\nlet zbroken: ref items = pool[5]\n"
	for _, c := range []struct {
		extra string
		more  mapFS
		want  string // the let named, "" for a graph
	}{
		{broken, nil, "g:zbroken"},
		{broken + "\n/// Bad.\nlet bad = nosuchname\n", nil, "g:bad"},
		{broken, mapFS{"law/h/h.canon": file("/// H.\npackage h\n\n/// Bad.\nlet abad = nosuchname\n")}, "g:zbroken"},
		{"\n/// Numbers.\nlet nums: [Int] = [1]\n\n/// Broken.\nlet count: Int = nums[5]\n", nil, ""},
	} {
		_, err := graphFixture(t, c.extra, c.more).Graph(context.Background())
		checkNoValue(t, err, c.want)
	}
}

// checkNoValue is R15's error naming the let want, or no error when want is "".
func checkNoValue(t *testing.T, err error, want string) {
	t.Helper()
	var pe *edit.PathError
	switch {
	case want == "" && err != nil:
		t.Errorf("Graph: %v, want a graph", err)
	case want == "":
	case !errors.As(err, &pe) || !errors.Is(err, edit.ErrNoValue):
		t.Errorf("Graph: %v, want ErrNoValue naming %s", err, want)
	case pe.Root.Pkg+":"+pe.Root.Name != want:
		t.Errorf("Graph names %s:%s, want %s", pe.Root.Pkg, pe.Root.Name, want)
	}
}

// graphShapes are entries of collections no let or field owns (in an optional field, a case
// field, a nested keyed list, a map of tables) and a load.defines table copied into a map.
const graphShapes = `
/// A slotted thing.
record Slotted {
  /// Its slots, maybe.
  slots: [Slot] keyed by n?
}

/// An optional keyed field.
let slotted: Slotted = { slots: [{ n: 8, to: a }] }

/// A poly.
variant Poly {
  pts { at: [Slot] keyed by n }
  empty {}
}

/// Polys.
let polys: [Poly] = [pts { at: [{ n: 4, to: a }] }]

/// Nested keyed lists.
let nested: [[Slot] keyed by n] = [[{ n: 5, to: b }]]

/// Tables by name.
let byName: {String: table Item} = { "x": { w { next: [a] } } }

/// Monsters.
local let mons = load.defines("m.h", prefix: "MI_")

/// Monsters grouped.
let grouped: {String: table Define} = { "x": mons }

/// Refs to monsters.
let toMons: [ref mons] = [MI_A]
`

// API.md R11: an entry of a collection no let or field owns is a node where it is met, a
// load.defines entry only under its own let; parity and completeness hold (checkParity).
func TestGraphNodesOfEveryCollection(t *testing.T) {
	f := graphFixture(t, graphShapes, mapFS{"law/g/m.h": file("#define MI_A 1\n#define MI_B 2\n")})
	g := mustGraph(t, f)
	var got []string
	for _, l := range graphLines(f, g) {
		if strings.HasPrefix(l, "N element") || strings.HasPrefix(l, "N entry") {
			got = append(got, strings.Fields(l)[2])
		}
	}
	for _, id := range []string{"g:slotted.slots[8]", "g:polys[0].at[4]", "g:nested[0][5]", "g:byName[x].w", "g:mons.MI_A", "g:mons.MI_B"} {
		if !slices.Contains(got, id) {
			t.Errorf("R11: no node %s among %v", id, got)
		}
	}
	if slices.ContainsFunc(got, func(id string) bool { return strings.HasPrefix(id, "g:grouped") }) {
		t.Errorf("R11: a copy of a load.defines table has nodes: %v", got)
	}
	checkParity(t, f, g)
}
