package eval_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// EVALUATION.md §10.5 item 2, §4.3: in testdata/worlds/nested.txtar, substituting each let with itself captures the bare findings.
func TestSelfSubstitutionIsBare(t *testing.T) {
	a, err := txtar.ParseFile("testdata/worlds/nested.txtar")
	if err != nil {
		t.Fatal(err)
	}
	b := runBuild(t, fromArchive(t, a), eval.Options{})
	tr := newTestRun(b, eval.Options{})
	bare, self := tr.run(t, "a", "fresh"), tr.run(t, "a", "each let substituted with itself")
	if len(bare.Expects) != 1 || len(self.Expects) != 1 {
		t.Fatalf("expects %d and %d, want 1 and 1", len(bare.Expects), len(self.Expects))
	}
	want, got := findingKeys(bare.Expects[0].Captured), findingKeys(self.Expects[0].Captured)
	if !slices.Equal(got, want) {
		t.Errorf("self-substituted:\n%s\nwant the bare expect's:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// faithfulTest substitutes x with the test's own x: a faithful copy builds the world a fresh
// build of the package is.
const faithfulTest = `
test "faithful" {
  expect package { x: x } passes
}
`

// The value kinds TestWorldCopyIsFaithful copies, each a package holding x, its findings in
// verification, instance checks and package checks.
var faithfulCases = []struct{ name, src string }{
	{"defaults", `/// Item.
record Item {
  /// Max.
  max: Int = 5
  /// Lim, derived.
  lim: Int(0..) = max - 7

  warn seen: false else "instance {max}"
}

/// X.
let x: [Item] = [{}, { max: 9 }]
`},
	{"applied records with dependent fields", `/// K.
enum K { num, sub }

/// Node.
record Node {
  /// K.
  k: K
  /// Max.
  max: Int = 5
}

/// Nodes.
let nodes: table Node = { a { k: num }, b { k: sub } }

/// Sub.
record Sub(e: Node) {
  /// Lim.
  lim: Int = e.max

  check lim <= e.max else "lim {lim} above {e.max} of {e.k}"
}

/// P.
type P(e: Node) = match e.k {
  num => Int
  sub => Sub(e)
}

/// Holder.
record Holder {
  /// Like.
  like: ref nodes
  /// P.
  p: P(like)
  /// By node.
  by: {y in nodes: P(y)} = {}
}

/// X.
let x: [Holder] = [{ p: { lim: 9 }, like: b }, { like: a, p: 3, by: { b: { lim: 7 } } }, { like: a, p: { lim: 1 } }]

/// Reads the value verification finds of the wrong branch, then fails.
let d: Int = [x[2].p].len() + [1][4]
`},
	{"cases", `/// Reward.
local variant Reward {
  gold { amount: Int }
  nothing

  fn worth(self) -> Int {
    return match self {
      gold(g) => g.amount
      nothing => 0
    }
  }

  check positive: worth() >= 0 else "{self.kind} is worth {worth()}"
}

/// X.
let x: [Reward] = [gold { amount: -1 }, nothing, gold { amount: 4 }]
`},
	{"keyed lists and let-path refs", `/// R.
record R {
  /// K.
  k: String
  /// X.
  n: Int

  warn seen: false else "keyed {k}"
}

/// X.
let x: [R] keyed by k = [{ k: "a", n: 1 }, { k: "b", n: 2 }]

/// Refs into x.
let picks: [ref x] = [a, zz]

/// Reads the ref verification finds unknown, then fails.
let d: Int = [picks[1]].len() + [1][4]

check {
  for r in x {
    if r.n > 1 {
      warn(r, "second {r.k}")
    }
  }
}
`},
	{"maps", `/// X.
let x: {String: Int(0..)} = { "a": 1, "b": zero() - 1 }

/// Zero, known only once evaluated.
local fn zero() -> Int { return 0 }

/// Y, derived.
let y: Int = x.len()
`},
	{"tables with retired entries", `/// Status.
record Status {
  /// Label.
  label: String(1..)

  warn seen: false else "status {label}"
}

/// X.
let x: stable table Status = {
  open { label: "Open" }
  retired old { label: blank() }
}

/// A ref to a retired entry.
let pick: ref x = old

/// Blank, known only once evaluated.
local fn blank() -> String { return "" }

/// Reads the ref verification refuses, then fails.
let d: Int = [pick].len() + [1][4]
`},
	{"level-1 refs", `/// Node.
record Node {
  /// Name.
  name: String

  warn seen: false else "node {name}"
}

/// Link.
record Link {
  /// To.
  to: ref Node
}

/// Tree.
record Tree {
  /// Nodes.
  nodes: [Node] keyed by name
  /// Links.
  links: [Link]
}

/// X.
let x: Tree = { nodes: [{ name: "a" }, { name: "b" }], links: [{ to: "a" }, { to: "zz" }, { to: "b" }] }

/// Reads the ref verification finds unknown, then fails.
let d: Int = [x.links[1].to].len() + [1][4]
`},
	{"nested refinements", `/// Level.
record Level {
  /// Index.
  index: Int(0..=24)
}

/// X, a field of its first level below its refinement.
let x: [Level] = [{ index: zero() - 1 }, { index: 3 }]

/// Reads the field its literal found outside its refinement, then fails.
let d: Int = [x[0].index].len() + [1][4]

/// Zero, known only once evaluated.
local fn zero() -> Int { return 0 }
`},
	{"literals kept as written", `/// K.
enum K { whole, real }

/// Node.
record Node {
  /// K.
  k: K
}

/// Nodes.
let nodes: table Node = { a { k: whole }, b { k: real } }

/// P: a Float for a real node.
type P(e: Node) = match e.k {
  whole => Int
  real => Float
}

/// Holder.
record Holder {
  /// Like.
  like: ref nodes
  /// P, an integer literal a real node's Float takes as written.
  p: P(like)
}

/// X.
let x: [Holder] = [{ like: b, p: 3 }, { like: a, p: 4 }]
`},
	{"one instance held twice", `/// Item.
record Item {
  /// W.
  w: Int(0..)

  warn seen: false else "instance {w}"
}

/// Twice the one instance.
local fn twice(i: Item) -> [Item] { return [i, i] }

/// X.
let x: [Item] = twice({ w: 2 }) + twice({ w: 3 })

/// Y, another let holding x's first instance.
let y: [Item] = [x[0]]
`},
}

// ownedCopy names the cases whose x owns a field collection: its copy's entries and refs name
// the copy as owner, so value.Equal, which compares identities by instance, tells them apart.
var ownedCopy = map[string]bool{"level-1 refs": true}

// EVALUATION.md §10.5 item 2: a copy verified in a world matches a fresh build's findings, instances and value.
func TestWorldCopyIsFaithful(t *testing.T) {
	for _, c := range faithfulCases {
		t.Run(c.name, func(t *testing.T) {
			b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+c.src+faithfulTest)
			want := findingKeys(b.bags["a"].Findings())
			tr := newTestRun(b, eval.Options{})
			res := tr.run(t, "a", "faithful")
			if len(res.Expects) != 1 || len(tr.worlds) != 1 {
				t.Fatalf("%d expects, %d worlds; want 1 and 1 (errors %v, internal %v, findings %v)", len(res.Expects), len(tr.worlds), res.Errors, tr.ev.Err(), want)
			}
			got := findingKeys(res.Expects[0].Captured)
			if !slices.Equal(got, want) {
				t.Errorf("world findings:\n%s\nwant a fresh build's:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if g, w := instances(got), instances(want); g != w {
				t.Errorf("%d instances checked in the world, %d in a fresh build", g, w)
			}
			x := eval.Root{Pkg: "a", Name: "x"}
			orig, _ := tr.ev.Settled(x)
			wv := tr.worlds[0].value(x)
			if orig == nil || wv == nil || wv == orig || wv.CanonText() != orig.CanonText() || !ownedCopy[c.name] && !value.Equal(wv, orig) {
				t.Errorf("world x %v, want a copy equal to %v", wv, orig)
			}
		})
	}
}

// findingKeys is each finding as code, place, path and message, sorted.
func findingKeys(fs []diag.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, fmt.Sprintf("%s %d:%d-%d %s: %s", f.Code, f.Span.File, f.Span.Start, f.Span.End, f.Path, f.Message))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// instances is how many instance checks warned: each case's `seen` warns once per instance.
func instances(keys []string) int {
	n := 0
	for _, k := range keys {
		if strings.HasPrefix(k, string(diag.W5001.Def().Code)) {
			n++
		}
	}
	return n
}

// value is the value the world settled for root, nil when none.
func (w worldSeen) value(root eval.Root) value.Value {
	if i := slices.Index(w.roots, root); i >= 0 {
		return w.values[i]
	}
	return nil
}

// isolationSrc substitutes a let a test then reads.
const isolationSrc = `/// Levels.
let levels: [Int] = [1, 2, 3]

/// Their count.
let total: Int = levels.len()

test "isolation" {
  expect package { levels: [] } passes
  expect levels.len() == 3
}
`

// EVALUATION.md §10.5 item 6: the world is discarded; the test's values do not change.
func TestPackageSubjectIsolation(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+isolationSrc)
	tr := newTestRun(b, eval.Options{})
	levels, total := eval.Root{Pkg: "a", Name: "levels"}, eval.Root{Pkg: "a", Name: "total"}
	before, _ := tr.ev.Force(context.Background(), levels)
	res := tr.run(t, "a", "isolation")
	after, _ := tr.ev.Settled(levels)
	if res.Failed || after != before || after.CanonText() != "[1, 2, 3]" {
		t.Errorf("failed %t; levels %v after the world, want %v unchanged", res.Failed, after, before)
	}
	if _, forced := tr.ev.Settled(total); forced {
		t.Errorf("the world's total reached the test")
	}
	if len(tr.worlds) != 1 || tr.worlds[0].value(total) == nil || tr.worlds[0].value(total).CanonText() != "0" {
		t.Errorf("worlds %v: want one, whose total follows the substitution", tr.worlds)
	}
}

// valueBuilder is a Builder that builds value subjects only.
type valueBuilder struct {
	subjects
	calls *int
}

func (v valueBuilder) Build(ctx context.Context, x value.Value, capture *diag.Bag) {
	*v.calls++
	if x != nil {
		v.subjects.Build(ctx, x, capture)
	}
}

// EVALUATION.md §10.5 (ADR-0023 Forecast 15): a Builder that is no PackageBuilder fails the expect, internally.
func TestPackageSubjectNeedsPackageBuilder(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "a/a.canon", pkgA+isolationSrc)
	tr := newTestRun(b, eval.Options{})
	calls := 0
	for _, cp := range b.selected([]string{"a"}) {
		for _, obj := range cp.Decls {
			if obj.Kind() != check.ObjTest {
				continue
			}
			res := tr.ev.Test(context.Background(), testDecl(t, obj), valueBuilder{subjects: tr.builder("a").subjects, calls: &calls})
			if len(res.Expects) == 0 || res.Expects[0].Passed || !res.Failed {
				t.Errorf("expects %v: want the package subject failed", res.Expects)
			}
		}
	}
	if tr.ev.Err() == nil || calls != 0 {
		t.Errorf("internal error %v, %d value builds: want one, and none", tr.ev.Err(), calls)
	}
}

// recordingBuilder counts every build it is asked for.
type recordingBuilder struct {
	packages
	values, worlds *int
}

func (r recordingBuilder) Build(ctx context.Context, x value.Value, capture *diag.Bag) {
	*r.values++
	r.packages.Build(ctx, x, capture)
}

func (r recordingBuilder) BuildPackage(ctx context.Context, w *eval.World, capture *diag.Bag) {
	*r.worlds++
	r.packages.BuildPackage(ctx, w, capture)
}

// recordingSrc calls a recorded fn in a substitution, in its world and in an expect.
const recordingSrc = potionSrc + `
/// Restored by the potion.
let restored: Int = p.healFor(40)

test "package subjects are not built for vectors" {
  expect package { restored: p.healFor(7) } passes
  expect p.healFor(8) == 8
  expect p passes
}
`

// EVALUATION.md §10.5 item 6, CONFORMANCE.md §6.1: computing vectors builds no package subject nor its values.
func TestTestCallsSkipPackageSubjects(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "potion/potion.canon", recordingSrc)
	heal := fnObj(t, b, potionPkg, "Potion", "healFor")
	ev, _, bags := freshTests(b, eval.Options{})
	values, worlds := 0, 0
	var seen []worldSeen
	rb := recordingBuilder{packages: packages{subjects: subjects{b: &build{prog: b.prog, checked: b.checked, ev: ev}, pkg: potionPkg}, worlds: &seen}, values: &values, worlds: &worlds}
	calls, err := ev.TestCalls(context.Background(), potionPkg, []check.Object{heal}, rb)
	if got := argsOf(calls); err != nil || !slices.Equal(got, []int64{200, 9000, -5, 8}) {
		t.Errorf("arguments %v, %v; want the expects' only", got, err)
	}
	if values != 1 || worlds != 0 {
		t.Errorf("%d value builds, %d worlds: want 1 and 0", values, worlds)
	}
	assertEmpty(t, bags)
}

// testDecl is obj's test declaration.
func testDecl(t *testing.T, obj check.Object) *syntax.TestDecl {
	t.Helper()
	d, ok := obj.Decl().(*syntax.TestDecl)
	if !ok {
		t.Fatalf("%s is no test", obj.Name())
	}
	return d
}

// amendedSrc is a let a layer amends, then a test substituting it with itself.
const amendedSrc = `/// Server.
record Server {
  /// Port.
  port: Int = 80
}

/// Config.
record Config {
  /// Server.
  server: Server = {}
}

/// X.
let x: Config = {}

test "faithful" {
  expect package { x: x } passes
}
`

// amendedLayer amends x's port.
const amendedLayer = `package a
layer dev

amend x {
  server.port: 9000
}
`

// EVALUATION.md §10.5 item 2, CLI.md §3.7: a layered copy keeps what its amendments replaced and copied, as History and Produced read them.
func TestWorldCopyKeepsAmendments(t *testing.T) {
	opt := eval.Options{Layers: []string{"dev"}}
	b := buildFiles(t, opt, "a/a.canon", pkgA+amendedSrc, "a/dev.canon", amendedLayer)
	tr := newTestRun(b, opt)
	if res := tr.run(t, "a", "faithful"); res.Failed || len(tr.worlds) != 1 {
		t.Fatalf("failed %t, %d worlds", res.Failed, len(tr.worlds))
	}
	x := eval.Root{Pkg: "a", Name: "x"}
	orig, _ := tr.ev.Settled(x)
	w := tr.worlds[0]
	got, want := amendments(w.ev, w.value(x)), amendments(tr.ev, orig)
	if got != want || !strings.Contains(want, "80") {
		t.Errorf("world history and produced value:\n%s\nwant the test's:\n%s", got, want)
	}
}

// amendments is the history of x.server.port, then x as its literal produced it.
func amendments(ev *eval.Evaluator, x value.Value) string {
	rec, ok := x.(*value.Record)
	if !ok {
		return "no record"
	}
	server, _ := rec.Fields[0].(*value.Record)
	if server == nil {
		return "no server"
	}
	var parts []string
	for _, v := range ev.History(rec, server, server.Fields[0]) {
		parts = append(parts, v.CanonText())
	}
	return strings.Join(parts, " <- ") + "; produced " + ev.Produced(rec).CanonText()
}
