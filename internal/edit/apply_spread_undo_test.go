package edit_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// spreadUndoSrc has lets spreading base, whose url default reads host and port, at every depth.
const spreadUndoSrc = `package d

/// A server.
record Server {
  /// Port.
  port: Int = 1
  /// Host.
  host: String = "localhost"
  /// URL.
  url: String = "http://{host}:{port}"
}

/// A holder of a server.
record Holder {
  /// Its server.
  server: Server = {}
}

/// The base server.
let base: Server = { host: "b" }

/// A server spreading it.
let derived: Server = { ...base, port: 2 }

/// A server spreading it below a field.
let deep: Holder = { server: { ...base } }

/// Two spreads away from base.
let hop2: Server = { ...derived }

/// A nested spread of a nested spread.
let nest: Holder = { server: { ...deep.server } }

/// A server dev amends, no spread reads.
let other: Server = {}

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int = 1
    /// Colour.
    c: String = "k"
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
    /// Colour.
    c: String = "k"
  }
}

/// A picture.
record Pic {
  /// Its shape.
  shape: Shape = circle {}
  /// Tag.
  tag: String = "t"
}

/// The base picture.
let basePic: Pic = { shape: circle { r: 2, c: "red" } }

/// A picture its spread gives a shape.
let pic: Pic = { ...basePic, tag: "p" }

/// A picture beside a spread with its own shape.
let pic2: Pic = { ...basePic, shape: circle { r: 3, c: "blue" } }
`

const spreadUndoFile = "law/d/d.canon"

func spreadUndoFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), spreadUndoFile: file(spreadUndoSrc),
		"law/d/dev.layer.canon": file("package d\nlayer dev\n\namend other {\n  port: 7\n}\n")}
}

// spreadUndoOp is an edit beside a spread that changes a field another field's default reads, the
// let line it turns, and that line once written.
type spreadUndoOp struct {
	op       edit.Operation
	old, new string
}

var spreadUndoOps = []spreadUndoOp{
	{edit.Operation{Kind: edit.OpSet, Path: "d:derived.host", Value: edit.Str("x")},
		`let derived: Server = { ...base, port: 2 }`, `let derived: Server = { ...base, port: 2, host: "x" }`},
	{edit.Operation{Kind: edit.OpSet, Path: "d:deep.server.port", Value: edit.Int(5)},
		`let deep: Holder = { server: { ...base } }`, `let deep: Holder = { server: { ...base, port: 5 } }`},
	{edit.Operation{Kind: edit.OpSet, Path: "d:hop2.port", Value: edit.Int(5)},
		`let hop2: Server = { ...derived }`, `let hop2: Server = { ...derived, port: 5 }`},
	{edit.Operation{Kind: edit.OpSet, Path: "d:nest.server.port", Value: edit.Int(5)},
		`let nest: Holder = { server: { ...deep.server } }`, `let nest: Holder = { server: { ...deep.server, port: 5 } }`},
	{edit.Operation{Kind: edit.OpSet, Path: "d:derived", Value: edit.Source("{ port: 3 }")}, // W9: what it leaves out is written
		`let derived: Server = { ...base, port: 2 }`, `let derived: Server = { ...base, port: 3, host: "localhost", url: "http://localhost:3" }`},
	{edit.Operation{Kind: edit.OpReset, Path: "d:deep.server"}, // a whole record beside its spread, to its default
		`let deep: Holder = { server: { ...base } }`, `let deep: Holder = {}`},
}

// API.md W9, M6, E22, E23 (EDIT-SPREADUNDO): an edit of a field beside a spread writes that field
// alone, the others the spread supplies left to it; a whole Set writes what it leaves out (W9) and
// its Undo every old value. Each Undo gives every value back, with no layer and under dev.
func TestSpreadUndoRestores(t *testing.T) {
	for _, layers := range [][]string{nil, {"dev"}} {
		for _, c := range spreadUndoOps {
			spreadUndoIn(t, layers, c)
		}
	}
}

// spreadUndoIn applies c's op with layers active, which turns c's line alone, then its Undo: every
// let's value comes back, with layers active and with none.
func spreadUndoIn(t *testing.T, layers []string, c spreadUndoOp) {
	t.Helper()
	op, fs := c.op, spreadUndoFS()
	s := open(t, fs, layers, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Errorf("%v %d %s: %v", layers, op.Kind, op.Path, err)
		return
	}
	f1 := written(fs, plan)
	if want := strings.Replace(spreadUndoSrc, c.old+"\n", c.new+"\n", 1); fileText(f1, spreadUndoFile) != want {
		t.Errorf("%v %d %s writes\n%swant\n%s", layers, op.Kind, op.Path, matrixHunk(spreadUndoSrc, fileText(f1, spreadUndoFile)), matrixHunk(spreadUndoSrc, want))
	}
	s1 := open(t, f1, layers, "", "d")
	up, err := edit.Apply(context.Background(), s1.env, s1.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Errorf("%v %d %s, Undo %+v: %v", layers, op.Kind, op.Path, plan.Undo, err)
		return
	}
	f2 := written(f1, up)
	for _, ls := range [][]string{layers, nil} {
		want, got := letTexts(t, open(t, fs, ls, "", "d").snap), letTexts(t, open(t, f2, ls, "", "d").snap)
		if !maps.Equal(got, want) {
			t.Errorf("%v %d %s, Undo %+v, layers %v:\nedit:\n%sundone:\n%svalues:\n%s", layers, op.Kind, op.Path, plan.Undo, ls,
				matrixHunk(fileText(fs, spreadUndoFile), fileText(f1, spreadUndoFile)), matrixHunk(fileText(fs, spreadUndoFile), fileText(f2, spreadUndoFile)), valuesDiff(want, got))
		}
	}
}

// valuesDiff is each let whose value differs between want and got.
func valuesDiff(want, got map[string]string) string {
	out := ""
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if want[k] != got[k] {
			out += "    " + k + ": " + got[k] + ", want " + want[k] + "\n"
		}
	}
	return out
}

// API.md E14, W9, E22, E23 (EDIT-SPREADUNDO): a SetCase of a shape beside a spread, supplied by it
// or stated, whose new case has no unwritten field (one kept, one given), writes the new case; its
// Undo gives every value back, with no layer and under dev, whose line no spread reads.
func TestSpreadUndoSetCase(t *testing.T) {
	for _, path := range []string{"d:pic.shape", "d:pic2.shape"} {
		for _, layers := range [][]string{nil, {"dev"}} {
			setCaseUndoIn(t, path, layers)
		}
	}
}

// setCaseUndoIn turns the shape at path into a square of side 4 with layers active, then undoes it.
func setCaseUndoIn(t *testing.T, path string, layers []string) {
	t.Helper()
	op := edit.Operation{Kind: edit.OpSetCase, Path: path, Case: "square", Value: edit.Obj{"side": edit.Int(4)}}
	fs := spreadUndoFS()
	s := open(t, fs, layers, "", "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Errorf("%v SetCase %s: %v", layers, path, err)
		return
	}
	f1 := written(fs, plan)
	if got := letTexts(t, open(t, f1, layers, "", "d").snap)[strings.TrimSuffix(path, ".shape")]; !strings.Contains(got, `square{side: 4, c: `) {
		t.Errorf("%v SetCase %s: %s, want a square of side 4 keeping its colour", layers, path, got)
	}
	s1 := open(t, f1, layers, "", "d")
	up, err := edit.Apply(context.Background(), s1.env, s1.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Errorf("%v SetCase %s, Undo %+v: %v", layers, path, plan.Undo, err)
		return
	}
	f2 := written(f1, up)
	for _, ls := range [][]string{layers, nil} {
		if want, got := letTexts(t, open(t, fs, ls, "", "d").snap), letTexts(t, open(t, f2, ls, "", "d").snap); !maps.Equal(got, want) {
			t.Errorf("%v SetCase %s, Undo, layers %v:\n%s", layers, path, ls, valuesDiff(want, got))
		}
	}
}
