package canon_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

const shapesProject = "project a {\n  canon: \"0.1\"\n}\n"

const shapesLaw = `/// P.
package p

/// R.
record R {
  /// N.
  n: Int = 0
  /// X.
  x: Int = 10 / n
}

/// D.
let d: R = {}

/// K.
record K {
  /// Id.
  id: String
  /// X.
  x: Int(0..5)
}

/// Seven.
let seven: Int = 7

/// Ks.
let ks: [K] keyed by id = [{ id: "a b", x: seven }, { x: 1, id: "c" }]

/// Ten.
let ten: Int = 10

/// Num.
record Num {
  /// Id.
  id: Int
  /// X.
  x: Int
}

/// Km.
let km: [Num] keyed by id = [{ x: 1, id: ten / 0 }]

/// Zero.
let zero: Int = 0

/// Ls.
let ls: [K] keyed by id = [{ id: "z", x: ten / zero }]

/// Loaded.
record Loaded {
  /// V.
  v: Int where 10 / it >= 0
}

/// Rs.
let rs: [Loaded] = load("rs.json")

/// Layered.
record Layered {
  /// N.
  n: Int(0..5)
  /// Tags.
  tags: {String: Int} = {}
}

/// C.
let c: Layered = { n: 0 }

/// C2.
let c2: Layered = { n: 0 }
`

const shapesLayer = `package p
layer dev

amend c {
  n: 7 + seven
}

amend c2 {
  tags["a b"]: 10 / 0
}
`

// API.md F1, P1, P8: each finding raised evaluating a top-level value carries the canonical path of
// the value it is about, a keyed-list element named by its key once the key is known and the
// list's own path until then, and API.md R6: Value of the path names the same finding with it.
func TestFindingPathShapes(t *testing.T) {
	opt := project(map[string]string{
		"project.canon": shapesProject, "p/p.canon": shapesLaw, "p/dev.canon": shapesLayer,
		"p/rs.json": `[{"v": 1}, {"v": 0}]`,
	})
	opt.Layers = []string{"dev"}
	p, err := canon.Open("/law", opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	div, out := diag.E4102.Def().Code, diag.E3204.Def().Code
	want := map[string]diag.Code{
		"d.x": div, `ks["a b"].x`: out, "km": div, "ls[z].x": div, "rs[1].v": div,
		"c.n": out, `c2.tags["a b"]`: div,
	}
	got := map[string]string{}
	for _, f := range res.Findings {
		if f.Path != "" {
			got[f.Path] = f.Code
		}
		checkPoisonedPath(t, p, f)
	}
	for _, path := range slices.Sorted(maps.Keys(want)) {
		if code := want[path]; diag.Code(got[path]) != code {
			t.Errorf("finding at %q: %q, want %s; all: %v", path, got[path], code, got)
		}
	}
}

// checkPoisonedPath: Value of a finding's path is a value or ErrNoValue, and its R6 findings
// carry the same Path.
func checkPoisonedPath(t *testing.T, p *canon.Project, f canon.Finding) {
	t.Helper()
	if f.Path == "" {
		return
	}
	_, err := p.Value(context.Background(), f.Package+":"+f.Path)
	if err == nil {
		return
	}
	var pe *canon.PathError
	if !errors.Is(err, canon.ErrNoValue) || !errors.As(err, &pe) {
		t.Errorf("Value(%s:%s): %v, want a value or ErrNoValue", f.Package, f.Path, err)
		return
	}
	for _, w := range pe.Findings {
		if w.Code == f.Code && w.Path != f.Path {
			t.Errorf("Value(%s:%s) explains with Path %q, the finding has %q", f.Package, f.Path, w.Path, f.Path)
		}
	}
}
