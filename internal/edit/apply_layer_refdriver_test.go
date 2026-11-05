package edit_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/wire"
)

// refDriverD types lr, the loaded jr and lx by the kind of the definition their ref names
// (TYPES.md 11.4): lr and jr in this package's defs, lx in package e's edefs. RA and RB differ by
// x's default, so a value typed against the wrong one is printed without it.
const refDriverD = `package d

import e

/// A kind.
enum K { a, b }

/// A.
record RA {
  /// X.
  x: Int = 0
}

/// B.
record RB {
  /// X.
  x: Int = 5
}

/// What a kind holds.
type Pick(k: K) = match k {
  a => RA
  b => RB
}

/// A definition.
record Def {
  /// Its kind.
  k: K
}

/// The definitions.
let defs: table Def = { da { k: a }, db { k: b } }

/// Typed by the definition it names.
record RR {
  /// Its definition.
  d: ref defs
  /// What it holds.
  t: Pick(d.k)?
}

/// Holds two RRs.
record LR {
  /// The first.
  r1: RR
  /// The second.
  r2: RR
}

/// Written in Canon.
let lr: LR = { r1: { d: da }, r2: { d: db } }

/// Loaded.
let jr: LR = load("r.json")

/// What an e kind holds.
type EPick(k: e.EK) = match k {
  a => RA
  b => RB
}

/// Typed by the e definition it names.
record RX {
  /// Its definition.
  d: ref e.edefs
  /// What it holds.
  t: EPick(d.k)?
}

/// Holds two RXs.
record LX {
  /// The first.
  r1: RX
  /// The second.
  r2: RX
}

/// Written in Canon, its drivers in another package.
let lx: LX = { r1: { d: ea }, r2: { d: eb } }
`

const refDriverE = `package e

/// A kind.
enum EK { a, b }

/// An e definition.
record EDef {
  /// Its kind.
  k: EK
}

/// The e definitions.
let edefs: table EDef = { ea { k: a }, eb { k: b } }
`

// refDriverFS is refDriverD, e, and an empty layer dev in d only: a line amending e's edefs
// creates e/dev.layer.canon (W11).
func refDriverFS() mapFS {
	return mapFS{
		"law/project.canon": file(projectCanon), "law/d/d.canon": file(refDriverD), "law/e/e.canon": file(refDriverE),
		"law/d/r.json":          file("{\n  \"r1\": {\n    \"d\": \"da\"\n  },\n  \"r2\": {\n    \"d\": \"db\"\n  }\n}\n"),
		"law/d/dev.layer.canon": file("package d\nlayer dev\n"),
	}
}

// refDriverRun is one request: the driver an earlier op changes in the edit layer, then the
// dependent field its ref reads, set to v, on host.
type refDriverRun struct {
	name, driver, path string
	v                  edit.Lit
	host               func(*build.Analysis) wire.Host
}

// refDriverRoots are, per field r, the driver and dependent paths: through a ref into another root
// of the package, a JSON root's ref into it, a ref into another package, whose line goes to another
// layer file (W11).
func refDriverRoots(r string) [][3]string {
	da, ea := map[string]string{"r1": "da", "r2": "db"}[r], map[string]string{"r1": "ea", "r2": "eb"}[r]
	return [][3]string{
		{"ref into another root", "d:defs." + da + ".k", "d:lr." + r + ".t"},
		{"JSON root, ref into another root", "d:defs." + da + ".k", "d:jr." + r + ".t"},
		{"ref into another package", "e:edefs." + ea + ".k", "d:lx." + r + ".t"},
	}
}

// refDriverMatrix is every run for field r: each root, each value kind (x 4 on either branch),
// on the harness's host and on build's, the one api supplies.
func refDriverMatrix(r string) []refDriverRun {
	vals := []struct {
		name string
		v    edit.Lit
	}{{"Source", edit.Source("{ x: 4 }")}, {"Obj", edit.Obj{"x": edit.Int(4)}}, {"FromJSON", edit.FromJSON(`{"x": 4}`)}}
	hosts := map[string]func(*build.Analysis) wire.Host{"test host": hostOf, "build.EditHost": build.EditHost}
	var out []refDriverRun
	for _, hn := range slices.Sorted(maps.Keys(hosts)) {
		for _, v := range vals {
			for _, c := range refDriverRoots(r) {
				out = append(out, refDriverRun{hn + ", " + c[0] + ", " + v.name, c[1], c[2], v.v, hosts[hn]})
			}
		}
	}
	return out
}

var refDriverDev = []string{"dev"}

// refDriverOpen analyzes fsys with d and e selected, layers active, editLayer the edit layer.
func refDriverOpen(t *testing.T, fsys mapFS, layers []string, editLayer string) session {
	t.Helper()
	return open(t, fsys, layers, editLayer, "d", "e")
}

// refDriverApply applies req to fsys under the active edit layer dev, on host: the plan and the
// files it writes, each write minimal (M6).
func refDriverApply(t *testing.T, fsys mapFS, host func(*build.Analysis) wire.Host, req edit.Request) (*edit.Plan, mapFS, error) {
	t.Helper()
	s := refDriverOpen(t, fsys, refDriverDev, refDriverDev[0])
	s.env.Host = host
	plan, err := edit.Apply(context.Background(), s.env, s.snap, req)
	if err != nil {
		return nil, nil, err
	}
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
	}
	return plan, written(fsys, plan), nil
}

// refDriverValue is the value of the root path names, analyzed from fsys under dev, every error
// that analysis holds in d or e reported.
func refDriverValue(t *testing.T, name string, fsys mapFS, path string) string {
	t.Helper()
	s := refDriverOpen(t, fsys, refDriverDev, "")
	got := values(s, false)[path[:strings.Index(path, ".")]]
	for _, pkg := range []string{"d", "e"} {
		for _, f := range s.a.Bag(pkg).Findings() {
			if f.Severity == diag.Error {
				t.Errorf("%s: after the edit: %s %s %s", name, f.Code, f.Path, f.Message)
			}
		}
	}
	return got
}

// API.md E1, V1, W11; DECISIONS 475: an edit layer's driver an earlier op changes, read through
// a ref, types the later Set: to b, RB's value is refused as a brace (475) or written typed, never RA's.
func TestLayerRefDriverToSecondBranch(t *testing.T) {
	for _, c := range refDriverMatrix("r1") {
		ops := []edit.Operation{setAt(c.driver, edit.Member("b")), setAt(c.path, c.v)}
		_, fs, err := refDriverApply(t, refDriverFS(), c.host, edit.Request{Ops: ops, AllowErrors: true})
		var ve *edit.ValueError
		switch {
		case errors.As(err, &ve) && ve.Expected == "d.RB?":
			continue // DECISIONS 475: RB's value, which Canon text does not read a brace as
		case err != nil:
			t.Errorf("%s: %v, want it refused as RB's or written as RB's", c.name, err)
			continue
		}
		if got := refDriverValue(t, c.name, fs, c.path); !strings.Contains(got, "RB{x: 4}") {
			t.Errorf("%s: %s = %s under dev, want it typed against RB, never RA", c.name, c.path, got)
		}
	}
}

// API.md E1, V1, W11, E22; DECISIONS 475: to a, RA's value, read as its brace, applies clean
// under dev, and its Undo round-trips the values and both packages' layer lines.
func TestLayerRefDriverToFirstBranch(t *testing.T) {
	for _, c := range refDriverMatrix("r2") {
		ops := []edit.Operation{setAt(c.driver, edit.Member("a")), setAt(c.path, c.v)}
		p1, f1, err := refDriverApply(t, refDriverFS(), c.host, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%s: %v, want it typed against RA and applied", c.name, err)
			continue
		}
		if got := refDriverValue(t, c.name, f1, c.path); !strings.Contains(got, "RA{x: 4}") {
			t.Errorf("%s: %s = %s under dev, want it typed against RA", c.name, c.path, got)
		}
		refDriverUndo(t, c, [2]mapFS{refDriverFS(), f1}, p1.Undo)
	}
}

// refDriverUndo applies undo to edited[1], then its Undo: the first gives back edited[0]'s values
// under dev and without it, and each layer file's lines; the second edited[1]'s values (E22).
func refDriverUndo(t *testing.T, c refDriverRun, edited [2]mapFS, undo []edit.Operation) {
	t.Helper()
	p2, f2, err := refDriverApply(t, edited[1], c.host, edit.Request{Ops: undo})
	if err != nil {
		t.Errorf("%s, Undo: %+v: %v", c.name, undo, err)
		return
	}
	refDriverSame(t, c.name+", Undo", edited[0], f2)
	for _, f := range []string{"law/d/dev.layer.canon", "law/e/dev.layer.canon"} {
		if w, g := amendLines(t, fileData(edited[0], f)), amendLines(t, fileData(f2, f)); !slices.Equal(w, g) {
			t.Errorf("%s, Undo: %s lines %q, want %q", c.name, f, g, w)
		}
	}
	if _, f3, err := refDriverApply(t, f2, c.host, edit.Request{Ops: p2.Undo}); err != nil {
		t.Errorf("%s, Undo of the Undo: %+v: %v", c.name, p2.Undo, err)
	} else {
		refDriverSame(t, c.name+", Undo of the Undo", edited[1], f3)
	}
}

// fileData is the bytes of fsys's file at name, nil when there is none.
func fileData(fsys mapFS, name string) []byte {
	if f := fsys[name]; f != nil {
		return f.Data
	}
	return nil
}

// refDriverSame reports every value of want that got does not hold, under dev and without it.
func refDriverSame(t *testing.T, name string, want, got mapFS) {
	t.Helper()
	for _, layers := range [][]string{refDriverDev, nil} {
		w, g := values(refDriverOpen(t, want, layers, ""), false), values(refDriverOpen(t, got, layers, ""), false)
		for _, k := range slices.Sorted(maps.Keys(w)) {
			if g[k] != w[k] {
				t.Errorf("%s, layers %v: %s = %s, want %s", name, layers, k, g[k], w[k])
			}
		}
	}
}
