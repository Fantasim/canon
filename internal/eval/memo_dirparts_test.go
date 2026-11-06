package eval_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// nestedPartsArchive is a load.dir of I, whose qs hold a Q owning tt: binding its picks' default copies it (EVALUATION.md §3.4).
const nestedPartsArchive = `-- a/a.canon --
/// A.
package a

/// An entry of oo.
record S {
  /// Its key.
  name: String
}

/// An entry of tt.
record T {
  /// Its key.
  name: String
}

/// Owns tt; its picks' default names an entry of the enclosing I's oo.
record Q {
  /// Its key.
  name: String
  /// Owned entries.
  tt: [T] keyed by name
  /// An entry of its own tt.
  best: ref T = "t"
  /// A default ref key.
  picks: {ref S: Int} = { x: 2 }
}

/// One file each.
record I {
  /// Its key.
  id: String
  /// The entries picks names.
  oo: [S] keyed by name
  /// The nested owners.
  qs: [Q] keyed by name
}

/// The elements.
let ii: [I] keyed by id = load.dir("ii/*.json")

/// Each element's default key, bound to its own oo.
let held: [Bool] = [ii.a.qs.q.picks.keys().contains(ii.a.oo.x), ii.b.qs.q.picks.keys().contains(ii.b.oo.x), ii.c.qs.q.picks.keys().contains(ii.c.oo.x)]
`

// nestedPartsHeld is held as each element binds its default key to its own oo.
const nestedPartsHeld = "a.held = [true, true, true]"

// nestedPkg is the package of nestedPartsArchive, whose loads its bags report.
const nestedPkg = "a"

// nestedPart is the file holding the element id of ii, under scopeDir.
func nestedPart(id string) string { return scopeDir[1:] + "/ii/" + id + ".json" }

// nestedPartJSON is the element of ii in file ii/<id>.json, its oo holding x and more.
func nestedPartJSON(id, more string) []byte {
	return []byte(`{"id": "` + id + `", "oo": [{"name": "x"}, {"name": "` + more + `"}], "qs": [{"name": "q", "tt": [{"name": "t"}]}]}` + "\n")
}

// dirMemoHost serves loads through one Loader over a changing file system, as a build's host:
// an unchanged file keeps its source and parsed tree, so the memo serves its element again.
type dirMemoHost struct {
	*host
	p      *program
	l      *load.Loader
	srcs   map[[sha256.Size]byte]*source.File
	parsed map[*source.File]*jsonsrc.Node
}

// newDirMemoHost is a dirMemoHost over files, a MapFS rooted at scopeDir.
func newDirMemoHost(t *testing.T, p *program, files fstest.MapFS) *dirMemoHost {
	layout, ok := project.NewLayout(&project.Project{}, scopeDir, nil, diag.NewBag(p.fs, nestedPkg))
	if !ok {
		t.Fatal("no layout")
	}
	d := &dirMemoHost{p: p, srcs: map[[sha256.Size]byte]*source.File{}, parsed: map[*source.File]*jsonsrc.Node{}}
	d.l = &load.Loader{FS: rootedFS{files}, Layout: layout, Set: p.fs, Add: d.add, Parse: d.parse}
	return d
}

func (d *dirMemoHost) add(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
	if src := d.srcs[sum]; src != nil {
		return src, nil
	}
	src, err := d.p.fs.Add(display, abs, data)
	if err == nil {
		d.srcs[sum] = src
	}
	return src, err
}

func (d *dirMemoHost) parse(src *source.File, bag *diag.Bag) (*jsonsrc.Node, error) {
	if n := d.parsed[src]; n != nil {
		return n, nil
	}
	n, err := jsonsrc.Parse(src, bag)
	if err == nil {
		d.parsed[src] = n
	}
	return n, err
}

// request is a load's request in nestedPkg, decoded through the evaluator.
func (d *dirMemoHost) request() load.Request {
	return load.Request{Pkg: nestedPkg, Bag: diag.NewBag(d.p.fs, nestedPkg)}.Through(d.ev)
}

func (d *dirMemoHost) Load(ctx context.Context, e *syntax.LoadExpr, t types.Type) (value.Value, bool) {
	v, ok, err := d.l.Load(ctx, d.request(), e, t)
	return v, ok && err == nil
}

func (d *dirMemoHost) LoadRecorded(ctx context.Context, e *syntax.LoadExpr, t types.Type) (value.Value, bool, eval.LoadInputs) {
	out, err := d.l.Recorded(ctx, d.request(), e, t)
	if err != nil || out.Inputs == nil {
		return out.Value, out.OK && err == nil, nil
	}
	return out.Value, out.OK, out.Inputs
}

func (d *dirMemoHost) LoadReplay(_ context.Context, _ *syntax.LoadExpr, in eval.LoadInputs) (func(), bool) {
	li, ok := in.(*load.Inputs)
	return func() {}, ok && d.l.Replay(li)
}

// IMPLEMENTATION-PLAN §7.6 NFR-02, EVALUATION.md §3.4: an element binding copies a nested owner of is kept, warm as cold.
func TestMemoDirPartsNestedBinding(t *testing.T) {
	files := fstest.MapFS{}
	for _, id := range []string{"a", "b", "c"} {
		files[nestedPart(id)] = &fstest.MapFile{Data: nestedPartJSON(id, "y")}
	}
	p := fromArchive(t, txtar.Parse([]byte(nestedPartsArchive)))
	m := checkOnce(t, p, eval.Options{})
	var d *dirMemoHost
	m.as = func(h *host) eval.Host { d.host = h; return d }
	d = newDirMemoHost(t, p, files)
	memo := eval.NewMemo()
	for i, step := range []struct {
		file         string
		served, made int
	}{{"", 0, 3}, {"", 3, 0}, {"b", 2, 1}, {"", 3, 0}} {
		if step.file != "" {
			files[nestedPart(step.file)] = &fstest.MapFile{Data: nestedPartJSON(step.file, fmt.Sprint("w", i))}
		}
		cold := m.evaluate(t, nil, 0)
		got := m.evaluate(t, memo, 1)
		same(t, fmt.Sprint("step ", i), got, cold)
		if !strings.Contains(got.dump, nestedPartsHeld) {
			t.Errorf("step %d: EVALUATION.md §3.4: want %s in\n%s", i, nestedPartsHeld, got.dump)
		}
		if made := got.parts.stored + got.parts.unkept; got.parts.hits != step.served || made != step.made || got.parts.unkept != 0 {
			t.Errorf("step %d: %d elements served, %d decoded, %d of them unkept; want %d, %d and none",
				i, got.parts.hits, made, got.parts.unkept, step.served, step.made)
		}
	}
}
