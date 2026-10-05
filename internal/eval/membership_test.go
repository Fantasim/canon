package eval_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"golang.org/x/tools/txtar"
)

// STDLIB.md §5, TYPES.md §10.3, DECISIONS 315, 317: every expect of testdata/membership passes.
func TestKeyedMembership(t *testing.T) {
	expectAllPass(t, "testdata/membership/*.txtar")
}

// STDLIB.md §3, TYPES.md §4.3, §11.6, DECISIONS 306, 307: every expect of testdata/typefn passes.
func TestTypeFunctionReflection(t *testing.T) {
	expectAllPass(t, "testdata/typefn/*.txtar")
}

// reflectCostSrc has a type function with a scrutinee path of one field and one with a plain body; PROBE is a let.
const reflectCostSrc = `/// A.
package a

/// Size.
enum Size { small, large }

/// Shape.
enum Shape { round, square, oval }

/// Ev.
record Ev {
  /// Big.
  big: Bool
}

/// Evs.
let evs: table Ev = { one { big: true } }

/// By the big flag.
type ByBig(e: Ev) = match e.big {
  true => Size
  false => Shape
}

/// Flat.
type Flat(e: Ev) = Shape

/// P.
PROBE
`

// EVALUATION.md §12.1: F(a…).members costs 1, its argument nodes, 1 + the scrutinee path, a step per member.
func TestTypeReflectionCost(t *testing.T) {
	steps := func(let string) int64 {
		b := buildFiles(t, eval.Options{}, "a/a.canon", strings.Replace(reflectCostSrc, "PROBE", let, 1))
		assertBuilt(t, b)
		return b.ev.StepsSpent()
	}
	name, members := steps("let p: String = Flat(evs.one).typeName"), steps("local let p = Flat(evs.one).members")
	scrutinee := steps("let p: String = ByBig(evs.one).typeName")
	plain := steps("let p: String = Shape.typeName")
	arg, lit := steps("let p: Ev = evs.one"), steps("let p: Int = 0")
	const shapes, path = 3, 1
	if members-name != shapes || scrutinee-name != 1+path || name-plain != 1+arg-lit+1 {
		t.Errorf("typeName %d, members %d, through a path %d, E.typeName %d, argument %d against a literal %d", name, members, scrutinee, plain, arg, lit)
	}
}

// costSrc is five entries in a field reached by a let path and in a keyed-list let; PROBE reads k in COLL.
const costSrc = `/// A.
package a

/// M.
record M {
  /// Hp.
  hp: Int
}

/// Ms.
let ms: table M = { a { hp: 1 }, b { hp: 1 }, c { hp: 1 }, d { hp: 1 }, e { hp: 1 } }

/// S.
record S {
  /// M.
  m: ref ms
  /// N.
  n: Int
}

/// Z.
record Z {
  /// Spawns.
  spawns: [S] keyed by m
}

/// Z.
let z: Z = { spawns: [{ m: a, n: 1 }, { m: b, n: 1 }, { m: c, n: 1 }, { m: d, n: 1 }, { m: e, n: 1 }] }

/// Ss.
let ss: [S] keyed by m = [{ m: a, n: 1 }, { m: b, n: 1 }, { m: c, n: 1 }, { m: d, n: 1 }, { m: e, n: 1 }]

/// K.
let k: ref COLL = KEY

/// P.
let p: Bool = PROBE
`

// EVALUATION.md §12, DECISIONS 197, 315: `in` costs one step per pair, keys equal or not, through a let path as on a let.
func TestMembershipCost(t *testing.T) {
	steps := costProbe(t, "k in COLL")
	const pairs = 4 // the last of five entries is four pairs past the first
	pathFirst, pathLast := steps("z.spawns", "a"), steps("z.spawns", "e")
	letFirst, letLast := steps("ss", "a"), steps("ss", "e")
	if pathLast-pathFirst != pairs || letLast-letFirst != pairs || pathLast-letLast != pathFirst-letFirst {
		t.Errorf("let path %d, %d; let %d, %d: want %d more steps at the last entry, alike", pathFirst, pathLast, letFirst, letLast, pairs)
	}
}

// STDLIB.md §5, DECISIONS 317: hasKey costs one step like get, wherever the key is, through a let path as on a let.
func TestHasKeyCost(t *testing.T) {
	has, get := costProbe(t, "COLL.hasKey(k)"), costProbe(t, "COLL.get(k) != none")
	for _, coll := range []string{"z.spawns", "ss"} {
		first, last := has(coll, "a"), has(coll, "e")
		if first != last || get(coll, "a") != get(coll, "e") {
			t.Errorf("%s: hasKey %d at the first entry, %d at the last; get %d, %d: want alike", coll, first, last, get(coll, "a"), get(coll, "e"))
		}
	}
}

// costProbe is the steps costSrc spends with PROBE as probe, k a ref of key into coll.
func costProbe(t *testing.T, probe string) func(coll, key string) int64 {
	t.Helper()
	return func(coll, key string) int64 {
		src := strings.NewReplacer("COLL", coll, "KEY", key).Replace(strings.Replace(costSrc, "PROBE", probe, 1))
		b := buildFiles(t, eval.Options{}, "a/a.canon", src)
		assertBuilt(t, b)
		return b.ev.StepsSpent()
	}
}

// assertBuilt fails on any finding of a probe: its parse's, then its packages'.
func assertBuilt(t *testing.T, b *build) {
	t.Helper()
	if fs := b.prog.parse.Findings(); len(fs) > 0 {
		t.Errorf("parse: %d findings, first %s %s", len(fs), fs[0].Code, fs[0].Message)
	}
	assertEmpty(t, b.bags)
}

// expectAllPass runs the tests of each archive: every expect passes, and the build has no error.
func expectAllPass(t *testing.T, glob string) {
	t.Helper()
	files, err := filepath.Glob(glob)
	if err != nil || len(files) == 0 {
		t.Fatalf("%s: %v", glob, err)
	}
	for _, file := range files {
		a, err := txtar.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := fromArchive(t, a)
		opt := eval.Options{Layers: strings.Fields(string(archiveFile(a, layersFile)))}
		b := runBuildWith(t, p, opt, jsonLoader(t, p, a))
		out, found := runTests(t, b), b.findings(t)
		bad := strings.Contains(out, "passed false") || strings.Contains(out, "failed true") ||
			strings.Contains(out, "stopped true") || strings.Contains(out, "broken true")
		if bad || !strings.Contains(out, "passed true") || strings.Contains(found, "error[") {
			t.Errorf("%s:\n%s\n%s", file, out, found)
		}
	}
}
