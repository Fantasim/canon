package edit_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// examplePkgs are the example packages the minimal-write fuzz edits: literals, keyed lists,
// tables, entry files, JSON sources loaded one by one and by load.dir.
var examplePkgs = []string{"teamboard", "pipeline", "resource.farm", "game.items", "features.dependent"}

// mutations is how many kinds of operation a kind byte draws modulo it: Set, Add, Rename,
// Remove; the kind byte moveKind alone draws a Move among moveSpan positions, so the committed
// regressions keep their operations (log-2026-09-29 M4 U1b-r).
const mutations, moveKind, moveSpan = 4, 4, 3

// opBytes is how many bytes of the fuzz's input draw one operation: two for the value it
// edits, one for the kind; maxOps is how many operations one input draws at most.
const (
	opBytes = 3
	maxOps  = 3
)

// allowed are the refusals Apply may give a random operation; any other error fails the test.
var allowed = []error{
	edit.ErrBadPath, edit.ErrNoPath, edit.ErrAmbiguousPath, edit.ErrNoValue, edit.ErrBadValue, edit.ErrBadOp,
	edit.ErrNotEditable, edit.ErrKeyExists, edit.ErrStableKey, edit.ErrPathCollision,
}

// candidate is a value of an example the fuzz may edit, by path, with the value there.
type candidate struct {
	path string
	v    value.Value
}

// draw is the values a mutation writes: an integer and a string, the fuzz's or the sweep's.
type draw struct {
	n int64
	s string
}

// exampleEdits opens the examples as the fuzz edits them, and lists every value of their lets.
func exampleEdits(t testing.TB) (*build.Project, *build.Analysis, []candidate) {
	t.Helper()
	dir, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	out := t.TempDir()
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
	}
	p, err := build.Open(project.OS(), filepath.ToSlash(dir), build.Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), examplePkgs)
	if err != nil {
		t.Fatal(err)
	}
	var cands []candidate
	for _, pkg := range a.Program().Packages {
		for _, obj := range pkg.Decls {
			if v, ok := a.Force(eval.Root{Pkg: pkg.Path, Name: obj.Name()}); ok && obj.Kind() == check.ObjLet {
				cands = collect(cands, pkg.Path+":"+obj.Name(), v)
			}
		}
	}
	return p, a, cands
}

// collect lists v and every value below it, by positional paths.
func collect(out []candidate, path string, v value.Value) []candidate {
	out = append(out, candidate{path: path, v: v})
	switch x := v.(type) {
	case *value.Record:
		for i, f := range verify.Fields(x.T) {
			if i < len(x.Fields) && x.Fields[i] != nil {
				out = collect(out, path+"."+f.Name, x.Fields[i])
			}
		}
	case *value.List:
		for i, e := range x.Elems {
			out = collect(out, path+"[#"+strconv.Itoa(i)+"]", e)
		}
	case *value.Table:
		for i, e := range x.Entries {
			out = collect(out, path+"[#"+strconv.Itoa(i)+"]", e)
		}
	case *value.Map:
		for i, e := range x.Vals {
			out = collect(out, path+"[#"+strconv.Itoa(i)+"]", e)
		}
	}
	return out
}

// mutation is an operation on candidate c: a Set of a scalar, an Add to a list of scalars, a
// Remove, a Rename of an entry, a Move; false when c takes none of them.
func mutation(c candidate, kind uint8, d draw) (edit.Operation, bool) {
	if kind == moveKind {
		return edit.Operation{Kind: edit.OpMove, Path: c.path, Index: int(uint64(d.n) % moveSpan)}, true
	}
	switch kind % mutations {
	case 0:
		lit, ok := scalar(c.v, d)
		return edit.Operation{Kind: edit.OpSet, Path: c.path, Value: lit}, ok
	case 1:
		l, ok := c.v.(*value.List)
		if !ok || len(l.Elems) == 0 {
			return edit.Operation{}, false
		}
		lit, ok := scalar(l.Elems[0], d)
		return edit.Operation{Kind: edit.OpAdd, Path: c.path, Value: lit}, ok
	case 2:
		return edit.Operation{Kind: edit.OpRename, Path: c.path, Key: edit.Key(d.s)}, true
	}
	return edit.Operation{Kind: edit.OpRemove, Path: c.path}, true
}

// scalar is a value of a scalar's type drawn from d.
func scalar(v value.Value, d draw) (edit.Lit, bool) {
	switch x := v.(type) {
	case *value.Int:
		return edit.Int(d.n), true
	case *value.Bool:
		return edit.Bool(d.n%2 == 0), true
	case *value.Str:
		return edit.Str(d.s), true
	case *value.Member:
		i := int(uint64(d.n) % uint64(len(x.Enum.Members)))
		return edit.Member(x.Enum.Members[i].Name), true
	}
	return nil, false
}

// sweepDraw is the sweep's values for v: another integer, another string, a fresh key.
func sweepDraw(v value.Value) draw {
	d := draw{s: "x"}
	switch x := v.(type) {
	case *value.Int:
		d.n = x.V + 1
	case *value.Bool:
		d.n = 1
		if !x.V {
			d.n = 0
		}
	case *value.Str:
		d.s = x.V + "x"
	case *value.Member:
		d.n = int64(x.Index + 1)
	}
	return d
}

// applyChecked applies ops and checks what it writes, and with scalar the one line a Set of a
// scalar writes; a refusal outside allowed fails.
func applyChecked(t *testing.T, env edit.Env, a *build.Analysis, ops []edit.Operation, scalar bool) bool {
	t.Helper()
	plan, err := edit.Apply(context.Background(), env, edit.NewSnapshot(a), edit.Request{Ops: ops})
	if errors.Is(err, edit.ErrInternal) {
		t.Fatalf("%+v: an internal failure: %v", ops, err)
	}
	if err != nil {
		for _, want := range allowed {
			if errors.Is(err, want) {
				return false
			}
		}
		t.Fatalf("%+v: an error that is no refusal: %v", ops, err)
	}
	for _, ch := range plan.Changes {
		checkWritten(t, plan, ch)
		if scalar && len(plan.Dropped) == 0 {
			checkOneRun(t, plan, ch)
		}
	}
	return true
}

// checkOneRun is API.md M6 for a Set of a scalar whatever its value: one run of lines, from at
// most one line before. Its line is rewritten, removed at the default (E6), added when absent,
// or broken when the new value passes the formatter's width.
func checkOneRun(t *testing.T, plan *edit.Plan, ch edit.Change) {
	t.Helper()
	steps := edit.Writes(plan, ch.Path)
	if ch.Kind != edit.ChangeModified || len(steps) == 0 {
		t.Errorf("%s: a Set of a scalar made a change of kind %d", ch.Path, ch.Kind)
		return
	}
	last := steps[len(steps)-1]
	if h := lineHunks(lines(last.Before), lines(last.After)); len(h) != 1 || h[0].b1-h[0].b0 > 1 {
		t.Errorf("%s: a Set of a scalar changed %+v, want one run from one line (API.md M6)", ch.Path, h)
	}
}

// API.md M6 on every example: each Set of a scalar, Add to a list of scalars, Remove, Move and
// Rename is refused with a refusal Apply documents or writes files that are fixed points,
// N12-clean, and equal to the files before outside the items the plan rewrote.
func TestMinimalWriteOnExamples(t *testing.T) {
	p, a, cands := exampleEdits(t)
	env := edit.Env{Project: p, Host: hostOf}
	applied := map[edit.Op]int{}
	for _, c := range cands {
		for kind := range uint8(moveKind + 1) {
			op, ok := mutation(c, kind, sweepDraw(c.v))
			if ok && applyChecked(t, env, a, []edit.Operation{op}, op.Kind == edit.OpSet) {
				applied[op.Kind]++
			}
		}
	}
	for _, k := range []edit.Op{edit.OpSet, edit.OpAdd, edit.OpRemove, edit.OpRename, edit.OpMove} {
		if applied[k] == 0 {
			t.Errorf("no operation %d applied on the examples", k)
		}
	}
}

// IMPLEMENTATION-PLAN.md 7.7, API.md E1, M6: a sequence of up to three random operations on
// the examples, with random integers, strings and keys, is refused with a refusal Apply
// documents or writes files that are fixed points, N12-clean, and minimal.
func FuzzMinimalWrite(f *testing.F) {
	for _, seed := range []struct {
		ops []byte
		n   int64
		s   string
	}{
		{[]byte{0, 3, 0}, 7, "x"}, {[]byte{0, 17, 0}, -1, ""}, {[]byte{0, 40, 3}, 0, "a b"},
		{[]byte{0, 61, 1}, 1 << 40, "é"}, {[]byte{0, 90, 0, 0, 91, 0}, 42, "open"},
		{[]byte{0, 130, 3, 0, 131, 2}, 3, "fresh"}, {[]byte{0, 211, 1, 0, 212, 0, 1, 44, 2}, 9, "k"},
		{[]byte{0, 91, 4, 0, 130, 4}, 0, "m"}, {[]byte{0, 212, 4, 0, 213, 3, 1, 45, 4}, 2, "n"},
	} {
		f.Add(seed.ops, seed.n, seed.s)
	}
	for _, r := range fuzzRegressions {
		f.Add(r.raw, r.n, r.s)
	}
	p, a, cands := exampleEdits(f)
	env := edit.Env{Project: p, Host: hostOf}
	f.Fuzz(func(t *testing.T, raw []byte, n int64, s string) {
		if ops := drawOps(cands, raw, n, s); len(ops) > 0 {
			applyChecked(t, env, a, ops, false)
		}
	})
}

// drawOps are the operations FuzzMinimalWrite's input draws, opBytes bytes each.
func drawOps(cands []candidate, raw []byte, n int64, s string) []edit.Operation {
	var ops []edit.Operation
	for i := 0; i+opBytes <= len(raw) && len(ops) < maxOps; i += opBytes {
		at := int(raw[i])<<8 | int(raw[i+1])
		if op, ok := mutation(cands[at%len(cands)], raw[i+2], draw{n: n + int64(len(ops)), s: s}); ok {
			ops = append(ops, op)
		}
	}
	return ops
}

// fuzzRegressions are FuzzMinimalWrite's committed failures under testdata/fuzz: a Set of a
// string holding braces and one of invalid UTF-8 (log-2026-09-29 M4 U4b-r2, U4b-r3).
var fuzzRegressions = []struct {
	raw []byte
	n   int64
	s   string
}{{[]byte("270"), 64, "0{0"}, {[]byte("a20"), 7, "\xb9"}}

// log-2026-09-29 M4 U1b-r: each committed regression still draws the Set of its string that
// once failed, so no change to the draw orphans it, and is still refused or written minimally.
func TestFuzzRegressionsDrawTheirSet(t *testing.T) {
	p, a, cands := exampleEdits(t)
	env := edit.Env{Project: p, Host: hostOf}
	for _, r := range fuzzRegressions {
		ops := drawOps(cands, r.raw, r.n, r.s)
		if len(ops) != 1 || ops[0].Kind != edit.OpSet || ops[0].Value != edit.Str(r.s) {
			t.Errorf("%q draws %+v, want one Set of %q", r.raw, ops, r.s)
			continue
		}
		applyChecked(t, env, a, ops, false)
	}
}
