package edit_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// undoView is a session: its active layers and its edit layer.
type undoView struct {
	layers []string
	edit   string
}

// applyIn applies ops to fsys in view v: the plan and the files it writes, each write minimal (M6).
func applyIn(t *testing.T, name string, fsys mapFS, v undoView, ops []edit.Operation) (*edit.Plan, mapFS, bool) {
	t.Helper()
	s := open(t, fsys, v.layers, v.edit, "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if err != nil {
		t.Errorf("%s: %+v: %v", name, ops, err)
		return nil, nil, false
	}
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
	}
	return plan, written(fsys, plan), true
}

// sameIn reports every value of want got does not hold, read with layers active.
func sameIn(t *testing.T, name string, want, got mapFS, layers []string) bool {
	t.Helper()
	w, g := values(open(t, want, layers, "", "d"), false), values(open(t, got, layers, "", "d"), false)
	ok := true
	for _, k := range slices.Sorted(maps.Keys(w)) {
		if g[k] != w[k] {
			t.Errorf("%s, layers %v: %s = %s, want %s", name, layers, k, g[k], w[k])
			ok = false
		}
	}
	return ok
}

// undoSpec is a request to undo twice: ops applied to fsys in view, the layer sets whose values
// must come back, and the files that must come back byte for byte.
type undoSpec struct {
	name  string
	fsys  mapFS
	view  undoView
	ops   []edit.Operation
	views [][]string
	files []string
	lines []string // layer files whose amendment lines, not bytes, come back (E22: values, not text)
}

// undoIn applies c's ops, then their Undo, then its Undo: the values come back under every layer
// set of c.views, and so do c.files, byte for byte.
func undoIn(t *testing.T, c undoSpec) {
	t.Helper()
	p1, f1, ok := applyIn(t, c.name, c.fsys, c.view, c.ops)
	if !ok {
		return
	}
	p2, f2, ok := applyIn(t, c.name+", Undo", f1, c.view, p1.Undo)
	if !ok {
		return
	}
	for _, ls := range c.views {
		sameIn(t, c.name+", Undo", c.fsys, f2, ls)
	}
	for _, f := range c.files {
		if string(f2[f].Data) != string(c.fsys[f].Data) {
			t.Errorf("%s, Undo: %s does not come back:\n%s", c.name, f, f2[f].Data)
		}
	}
	for _, f := range c.lines {
		if w, g := amendLines(t, c.fsys[f].Data), amendLines(t, f2[f].Data); !slices.Equal(w, g) {
			t.Errorf("%s, Undo: %s lines %q, want %q", c.name, f, g, w)
		}
	}
	if _, f3, ok := applyIn(t, c.name+", Undo of the Undo", f2, c.view, p2.Undo); ok {
		for _, ls := range c.views {
			sameIn(t, c.name+", Undo of the Undo", f1, f3, ls)
		}
	}
}

const reviewLayerSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// What it aims at.
  target: Target(goal)?
  /// A note.
  note: String = ""
}

/// The quests.
let quests: table Quest = {
  slay { goal: collect, target: 10 }
}
`

const reviewDev = `package d
layer dev

amend quests {
  slay.goal: kill
  slay.target: "boar"
}
`

const reviewTop = `package d
layer top

amend quests {
  slay.note: "top"
}
`

func reviewLayerFS(layers ...string) mapFS {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(reviewLayerSrc), "law/d/dev.layer.canon": file(reviewDev)}
	if slices.Contains(layers, "top") {
		fs["law/d/top.layer.canon"] = file(reviewTop)
	}
	return fs
}

// API.md E22, W11, W11a (log-2026-10-01 M4.1 review rulings): under an edit layer, active below
// another or inactive, the Undo gives the layer its own lines back, never a merged value; the
// values come back in the edit layer's view and the session's, and the layer file byte for byte.
func TestUndoEditLayerLines(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	files := []string{"law/d/dev.layer.canon"}
	undoIn(t, undoSpec{"below another layer", reviewLayerFS("top"), undoView{[]string{"dev", "top"}, "dev"},
		[]edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))},
		[][]string{{"dev", "top"}, {"dev"}, nil}, files, nil})
	inactive := undoView{nil, "dev"}
	undoIn(t, undoSpec{"inactive", reviewLayerFS(), inactive,
		[]edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}, [][]string{{"dev"}, nil}, files, nil})
	undoIn(t, undoSpec{"inactive, dependent first", reviewLayerFS(), inactive,
		[]edit.Operation{setAt("quests.slay.target", edit.Str("x")), setAt("quests.slay.goal", kill)}, [][]string{{"dev"}, nil}, files, nil})
}

// API.md E22, W11 (log-2026-10-01 M4.1 review rulings): random requests through an active and an
// inactive edit layer; every Undo applies and gives every value back in both views.
func TestUndoEditLayerRandom(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	pool := []edit.Operation{
		setAt("quests.slay.goal", m("collect")), setAt("quests.slay.goal", m("kill")),
		setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.target", edit.Str("boar")),
		setAt("quests.slay.bonus", edit.Int(4)), setAt("quests.slay.bonus", edit.Str("elk")),
		{Kind: edit.OpReset, Path: "quests.slay.target"}, {Kind: edit.OpReset, Path: "quests.slay.goal"},
		setAt("quests.slay.note", edit.Str("n")),
		setAt("quests.gather.goal", m("kill")), setAt("quests.gather.target", edit.Str("x")), setAt("quests.gather.target", edit.Int(8)),
		{Kind: edit.OpReset, Path: "quests.gather.goal"},
	}
	dev := "package d\nlayer dev\n\namend quests {\n  slay.goal: collect\n  slay.target: 7\n  slay.bonus: 3\n}\n"
	for _, layers := range [][]string{{"dev"}, nil} {
		next := lcg(4242)
		for range randomUndos {
			fs := questFS()
			fs["law/d/dev.layer.canon"] = file(dev)
			randomUndo(t, fs, undoView{layers, "dev"}, pickOps(next, pool), [][]string{{"dev"}, nil})
		}
	}
}

// API.md E1, E22 (log-2026-09-29 U-E22-r): random requests on quests; no Undo is refused, none
// leaves a value other than it was, and no request fails inside the compiler.
func TestUndoRandom(t *testing.T) {
	m := func(s string) edit.Lit { return edit.Member(s) }
	pool := []edit.Operation{
		setAt("quests.slay.goal", m("collect")), setAt("quests.slay.goal", m("kill")),
		setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.target", edit.Str("boar")),
		setAt("quests.slay.bonus", edit.Int(4)), setAt("quests.slay.bonus", edit.Str("elk")),
		{Kind: edit.OpReset, Path: "quests.slay.target"}, setAt("quests.slay.note", edit.Str("n")),
		setAt("quests.slay.items", edit.Bool(true)), setAt("quests.slay.items", edit.Bool(false)),
		{Kind: edit.OpAdd, Path: "quests.slay.rewards", Value: edit.Int(7)},
		{Kind: edit.OpAdd, Path: "quests.slay.rewards", Value: edit.Str("gem")},
		{Kind: edit.OpRemove, Path: "quests.slay.rewards[0]"},
		{Kind: edit.OpInsert, Path: "quests.slay.rewards", Index: 0, Value: edit.Int(9)},
		setAt("quests.slay.rewards[0]", edit.Str("orb")),
		setAt("quests.gather.act.goal", m("collect")), setAt("quests.gather.act.target", edit.Int(5)),
		{Kind: edit.OpSetCase, Path: "quests.gather.act", Case: "wait"},
		{Kind: edit.OpSetCase, Path: "quests.slay.act", Case: "hunt", Value: edit.Obj{"goal": m("collect")}},
		setAt("quests.slay.act.target", edit.Int(3)),
		{Kind: edit.OpRemove, Path: "quests.gather"},
		{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("gather"), Value: edit.Source(`{ goal: kill, target: "hare", act: wait {} }`)},
		{Kind: edit.OpRename, Path: "quests.gather", Key: edit.Key("pick")},
		{Kind: edit.OpMove, Path: "quests.slay", Index: 1},
		setAt("quests.gather.goal", m("kill")), setAt("quests.gather.target", edit.Str("x")),
	}
	next := lcg(98765)
	for range randomUndos {
		randomUndo(t, questFS(), undoView{}, pickOps(next, pool), [][]string{nil})
	}
}

// randomUndos is how many random requests TestUndoRandom and TestUndoEditLayerRandom apply.
const randomUndos = 150

// lcg is a deterministic generator of indexes below n.
func lcg(seed uint64) func(n int) int {
	return func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
}

// pickOps are two to four operations of pool.
func pickOps(next func(int) int, pool []edit.Operation) []edit.Operation {
	ops := make([]edit.Operation, 2+next(3))
	for i := range ops {
		ops[i] = pool[next(len(pool))]
	}
	return ops
}

// randomUndo applies ops: a refusal is fine, an internal failure is not; the Undo applies and
// gives every value back under each layer set of views.
func randomUndo(t *testing.T, fsys mapFS, v undoView, ops []edit.Operation, views [][]string) {
	t.Helper()
	s := open(t, fsys, v.layers, v.edit, "d")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
	if errors.Is(err, edit.ErrInternal) {
		t.Errorf("%+v: %v", ops, err)
	}
	if err != nil {
		return
	}
	f1 := written(fsys, plan)
	_, f2, ok := applyIn(t, "Undo of "+opsText(ops), f1, v, plan.Undo)
	for _, ls := range views {
		if ok && !sameIn(t, "Undo of "+opsText(ops), fsys, f2, ls) {
			return
		}
	}
}

func opsText(ops []edit.Operation) string {
	var b strings.Builder
	for _, op := range ops {
		b.WriteString(op.Path + " ")
	}
	return b.String()
}

// API.md E11, E22 (log-2026-10-01 M4.1 review rulings): a Rename between or around the changes of
// a driver and its dependent, a swap of two keys included, is undone after the region it renames
// is restored under its name then; the values come back.
func TestUndoAroundRenames(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	ren := func(p, k string) edit.Operation {
		return edit.Operation{Kind: edit.OpRename, Path: p, Key: edit.Key(k)}
	}
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"rename first", []edit.Operation{ren("quests.slay", "slain"), setAt("quests.slain.goal", collect), setAt("quests.slain.target", edit.Int(25))}},
		{"rename between", []edit.Operation{setAt("quests.slay.goal", collect), ren("quests.slay", "slain"), setAt("quests.slain.target", edit.Int(25))}},
		{"swap", []edit.Operation{ren("quests.slay", "tmp"), ren("quests.gather", "slay"), ren("quests.tmp", "gather"),
			setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("x")), setAt("quests.gather.goal", collect), setAt("quests.gather.target", edit.Int(3))}},
		{"swap, cascade", []edit.Operation{setAt("quests.slay.target", edit.Str("boar")), ren("quests.slay", "tmp"), ren("quests.gather", "slay"),
			ren("quests.tmp", "gather"), setAt("quests.gather.goal", collect)}},
		{"rename back and forth", []edit.Operation{setAt("quests.slay.goal", collect), ren("quests.slay", "a"), setAt("quests.a.target", edit.Int(2)),
			ren("quests.a", "slay"), setAt("quests.slay.goal", kill)}},
	} {
		undoIn(t, undoSpec{name: c.name, fsys: questFS(), ops: c.ops, views: [][]string{nil}})
	}
}

// amendLines are the amendment lines of a layer file, `path: right-hand side` as the Undo's
// verification compares them, sorted.
func amendLines(t *testing.T, text []byte) []string {
	t.Helper()
	var fs source.FileSet
	src, err := fs.Add("l.layer.canon", "/l.layer.canon", text)
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, ""))
	var out []string
	for _, b := range f.Amends {
		for _, it := range b.Items {
			from, to, rhs := f.Span(it.Path[0]), f.Span(it.Path[len(it.Path)-1]), f.Span(it.Value)
			out = append(out, b.Target.Name+"."+string(text[from.Start:to.End])+": "+edit.LineText(string(text[rhs.Start:rhs.End])))
		}
	}
	slices.Sort(out)
	return out
}
