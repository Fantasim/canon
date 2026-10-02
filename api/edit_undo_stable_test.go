package canon_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// codedLaw is editLaw with a @stable code on Code, first's 1, and its lock lines.
var codedLaw = map[string]string{
	"a/a.canon": strings.Replace(strings.Replace(editLaw["a/a.canon"],
		"record Code {\n  /// Its label.\n  label: String\n}",
		"record Code {\n  /// Its label.\n  label: String\n  /// Its code, locked.\n  code: Int @stable\n}", 1),
		`first { label: "First" }`, `first { label: "First", code: 1 }`, 1),
	"a/canon.lock": lockHeader + "field  a.codes.code  1  first\ntable  a.codes  first\n",
}

// zLaw is codedLaw with z, an entry the lock does not hold yet, of code 5.
var zLaw = map[string]string{
	"a/a.canon":    strings.Replace(codedLaw["a/a.canon"], `first { label: "First", code: 1 }`, `first { label: "First", code: 1 }`+"\n  z { label: \"Z\", code: 5 }", 1),
	"a/canon.lock": codedLaw["a/canon.lock"],
}

// retiredNew is the lock edited with each table line the lock before lacked retired: an id the
// edit locked that the before state lacked (API.md E23).
func retiredNew(before, edited string) string {
	var out []string
	for _, line := range strings.SplitAfter(edited, "\n") {
		if strings.HasPrefix(line, "table ") && !strings.Contains(before, line) {
			line = strings.TrimSuffix(line, "\n") + "  retired\n"
		}
		out = append(out, line)
	}
	return strings.Join(out, "")
}

// absent is a want of lockFactsKept: no value at that path.
const absent = "\x00absent"

// stableRoots are the values an Undo of an edit of a:codes gives back.
var stableRoots = []string{"a:statuses", "a:config", "a:picked", "a:twice", "a:codes.first"}

// API.md E23, E22, E20 (G2 rounds 2, 4): each request locks second, new; the Undo applies, the
// lock keeps every line the edit wrote, second retired, second keeps the result's code, and every
// other value comes back.
func TestEditUndoKeepsLockFacts(t *testing.T) {
	add := func(code int64) canon.Op {
		return canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S"), "code": canon.Int(code)})
	}
	setCodes := func(src string) canon.Op { return canon.Set("a:codes", canon.Source(src)) }
	firstOnly := `{ first { label: "First", code: 1 } }`
	code := func(v string) map[string]string { return map[string]string{"a:codes.second.code": v} }
	setZ := setCodes(`{ first { label: "First", code: 1 }, z { label: "Z", code: 6 } }`)
	zKept := map[string]string{"a:codes.second.code": "5", "a:codes.z.code": "6"}
	zGone := map[string]string{"a:codes.second.code": "5", "a:codes.z": absent}
	cases := []struct {
		name  string
		files map[string]string
		ops   []canon.Op
		want  map[string]string
	}{
		{"Set, then AddEntry", nil, []canon.Op{setCodes(`{ first { label: "F" } }`),
			canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S")})}, nil},
		{"Set adding an entry, then AddEntry", nil, []canon.Op{setCodes(`{ first { label: "F" }, third { label: "T" } }`),
			canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S")})}, nil},
		{"AddEntry, then a Set changing its code", codedLaw, []canon.Op{add(2),
			setCodes(`{ first { label: "First", code: 1 }, second { label: "S", code: 5 } }`)}, code("5")},
		{"AddEntry, Set dropping it, AddEntry again", codedLaw, []canon.Op{add(2), setCodes(firstOnly), add(5)}, code("5")},
		{"AddEntry, Set dropping it, AddEntry again, Set its code", codedLaw,
			[]canon.Op{add(2), setCodes(firstOnly), add(5), canon.Set("a:codes.second.code", canon.Int(7))}, code("7")},
		{"Set adding it, then Set its code (E20)", codedLaw, []canon.Op{
			setCodes(`{ first { label: "First", code: 1 }, second { label: "S", code: 2 } }`),
			canon.Set("a:codes.second.code", canon.Int(7))}, code("7")},
		{"a pending value the Set changes, then AddEntry taking it", zLaw, []canon.Op{setZ, add(5)}, zKept},
		{"the same, then a Set of the new entry", zLaw, []canon.Op{setZ, add(5), canon.Set("a:codes.second.label", canon.Str("W"))}, zKept},
		{"Remove z, then AddEntry taking its code", zLaw, []canon.Op{canon.Remove("a:codes.z"), add(5)}, zGone},
		{"Set dropping z, then AddEntry taking its code", zLaw, []canon.Op{setCodes(firstOnly), add(5)}, zGone},
		{"Remove z, another Set, then AddEntry", zLaw, []canon.Op{canon.Remove("a:codes.z"), canon.Set("a:config.port", canon.Int(1)), add(5)}, zGone},
		{"Rename z, Set its code, then AddEntry taking the old one", zLaw, []canon.Op{
			canon.Rename("a:codes.z", canon.Key("y")), canon.Set("a:codes.y.code", canon.Int(6)), add(5),
		}, map[string]string{"a:codes.second.code": "5", "a:codes.z": absent, "a:codes.y.code": "6"}},
	}
	for _, c := range cases {
		lockFactsKept(t, c.name, c.files, c.ops, c.want)
	}
}

// lockFactsKept applies ops to editLaw with files, then their Undo, and checks it as
// TestEditUndoKeepsLockFacts says; want are values after the Undo, by path.
func lockFactsKept(t *testing.T, name string, files map[string]string, ops []canon.Op, want map[string]string) {
	t.Helper()
	ctx := context.Background()
	p, m := openEdit(t, map[string]string{"a/canon.lock": lockFirst}, files)
	before := map[string]string{}
	for _, path := range stableRoots {
		v, err := p.Value(ctx, path)
		if err != nil {
			t.Fatalf("%s: %s: %v", name, path, err)
		}
		before[path] = v.Text
	}
	locked := lockOf(t, m, "a/canon.lock")
	res, err := p.Edit(ctx, canon.Edit{Ops: ops})
	if err != nil {
		t.Errorf("%s: Edit: %v", name, err)
		return
	}
	edited := lockOf(t, m, "a/canon.lock")
	undoRetires(t, p, res)
	lock := retiredNew(locked, edited)
	if got := lockOf(t, m, "a/canon.lock"); got != lock || !strings.Contains(edited, "table  a.codes  second\n") {
		t.Errorf("API.md E20, E23, %s: lock after the edit %q, after the Undo %q, want %q", name, edited, got, lock)
	}
	for _, path := range stableRoots {
		if v, err := p.Value(ctx, path); err != nil || v.Text != before[path] {
			t.Errorf("API.md E22, %s: %s after the Undo: %+v, %v, want %s", name, path, v, err, before[path])
		}
	}
	if _, err := p.Value(ctx, "a:codes.third"); err == nil {
		t.Errorf("API.md E22, %s: a:codes.third survives the Undo", name)
	}
	for _, path := range slices.Sorted(maps.Keys(want)) {
		v, err := p.Value(ctx, path)
		if want[path] == absent && err == nil || want[path] != absent && (err != nil || v.Text != want[path]) {
			t.Errorf("API.md E23, %s: %s after the Undo: %+v, %v, want %s", name, path, v, err, want[path])
		}
	}
}

// API.md E23, E20 (log-2026-10-02, G2 round 3): an entry added to a stable table and dropped by a
// later Set of the whole table was never locked: neither the edit nor its Undo, which takes no
// Retire, changes canon.lock, and the Undo gives the source back.
func TestEditUndoStableAddEntryDropped(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, map[string]string{"a/canon.lock": lockFirst})
	before := read(t, m, "a/a.canon")
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{
		canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S")}),
		canon.Set("a:codes", canon.Source(`{ first { label: "F" } }`)),
	}})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if slices.ContainsFunc(res.Undo, func(op canon.Op) bool { return op.Kind == canon.OpRetire }) {
		t.Errorf("API.md E23: Undo %+v retires an id never locked", res.Undo)
	}
	if got := lockOf(t, m, "a/canon.lock"); got != lockFirst {
		t.Errorf("API.md E20: lock after the edit %q, want %q", got, lockFirst)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo %+v: %v", res.Undo, err)
	}
	if got := lockOf(t, m, "a/canon.lock"); got != lockFirst {
		t.Errorf("API.md E20: lock after the Undo %q, want %q", got, lockFirst)
	}
	if got := read(t, m, "a/a.canon"); got != before {
		t.Errorf("API.md E22: the source does not come back:\n%s", got)
	}
}

// API.md E23, E4, E20 (G2 round 5): with AllowErrors, an id whose lines E20 skips, its code
// already first's, was never locked: its Undo is a Remove, which applies, and the lock is unchanged.
func TestEditUndoSkippedID(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, codedLaw)
	before := read(t, m, "a/a.canon")
	res, err := p.Edit(ctx, canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S"), "code": canon.Int(1)}),
	}})
	if err != nil || !res.Applied || res.Summary.Errors == 0 {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	if want := []canon.Op{{Kind: canon.OpRemove, Path: "a:codes.second"}}; !slices.EqualFunc(res.Undo, want, sameOp) {
		t.Errorf("API.md E23: Undo %+v, want %+v", res.Undo, want)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E4, E22: the Undo %+v: %v", res.Undo, err)
	}
	if got := read(t, m, "a/a.canon"); got != before {
		t.Errorf("API.md E22: the source does not come back:\n%s", got)
	}
	if got := lockOf(t, m, "a/canon.lock"); got != codedLaw["a/canon.lock"] {
		t.Errorf("API.md E20: lock %q, want it unchanged", got)
	}
}

// API.md E4 (G2 round 5): Remove and Rename refuse an entry of a stable table whose id the lock
// holds or the request locks, and take one only pending in the sources.
func TestEditStableKeyLocked(t *testing.T) {
	add := canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S"), "code": canon.Int(2)})
	cases := []struct {
		name string
		ops  []canon.Op
		want error
	}{
		{"Remove, held", []canon.Op{canon.Remove("a:codes.first")}, canon.ErrStableKey},
		{"Rename, held", []canon.Op{canon.Rename("a:codes.first", canon.Key("one"))}, canon.ErrStableKey},
		{"Remove, locked by the request", []canon.Op{add, canon.Remove("a:codes.second")}, canon.ErrStableKey},
		{"Rename, locked by the request", []canon.Op{add, canon.Rename("a:codes.second", canon.Key("two"))}, canon.ErrStableKey},
		{"Remove, pending", []canon.Op{canon.Remove("a:codes.z")}, nil},
		{"Rename, pending", []canon.Op{canon.Rename("a:codes.z", canon.Key("y"))}, nil},
	}
	for _, c := range cases {
		p, _ := openEdit(t, zLaw)
		if _, err := p.Edit(context.Background(), canon.Edit{Ops: c.ops, DryRun: true}); !errors.Is(err, c.want) && (c.want != nil || err != nil) {
			t.Errorf("API.md E4, %s: %v, want %v", c.name, err, c.want)
		}
	}
}

// API.md E23 (G2 round 6): with AllowErrors, an id a later operation drops is never locked and,
// the result lacking it, takes no inverse; the Undo, applied without AllowErrors, gives the
// values and the lock back.
func TestEditUndoDroppedSkippedID(t *testing.T) {
	ctx := context.Background()
	add := canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S"), "code": canon.Int(2)})
	set := func(label string) canon.Op {
		return canon.Set("a:codes", canon.Source(`{ first { label: "`+label+`", code: 1 } }`))
	}
	for _, ops := range [][]canon.Op{{add, set("F")}, {set("First"), add, set("G")}} {
		p, m := openEdit(t, codedLaw)
		res, err := p.Edit(ctx, canon.Edit{AllowErrors: true, Ops: ops})
		if err != nil || !res.Applied {
			t.Errorf("%+v: Edit: %+v, %v", ops, res, err)
			continue
		}
		if slices.ContainsFunc(res.Undo, func(op canon.Op) bool { return op.Kind == canon.OpRemove || op.Kind == canon.OpRetire }) {
			t.Errorf("API.md E23, %+v: Undo %+v names second", ops, res.Undo)
		}
		if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
			t.Errorf("API.md E22, %+v: the Undo %+v: %v", ops, res.Undo, err)
			continue
		}
		if v, err := p.Value(ctx, "a:codes.first.label"); err != nil || v.Text != "First" {
			t.Errorf("API.md E22, %+v: a:codes.first.label %+v, %v", ops, v, err)
		}
		if got := lockOf(t, m, "a/canon.lock"); got != codedLaw["a/canon.lock"] {
			t.Errorf("API.md E20, %+v: lock %q, want it unchanged", ops, got)
		}
	}
}

// API.md E23, LOCK.md 4.4 (G2 round 6): the Undo of a Remove of a pending entry adds it back,
// which locks it as the next build would; its values come back.
func TestEditUndoRemovePending(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, zLaw)
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Remove("a:codes.z")}})
	if err != nil {
		t.Fatalf("API.md E4: %v", err)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo %+v: %v", res.Undo, err)
	}
	if v, err := p.Value(ctx, "a:codes.z"); err != nil || v.Text != `Code{label: "Z", code: 5}` {
		t.Errorf("API.md E22: a:codes.z after the Undo: %+v, %v", v, err)
	}
	lock := lockOf(t, m, "a/canon.lock")
	if !strings.Contains(lock, "table  a.codes  z\n") || !strings.Contains(lock, "field  a.codes.code  5  z\n") {
		t.Errorf("API.md E23: lock after the Undo %q, want z locked", lock)
	}
}

// API.md E4, E21, LOCK.md 6.1 (G2 round 6): under an edit layer, Remove and Rename of any entry
// of a stable table, pending included, are refused; E21 judges editability first, reason layer.
func TestEditStableKeyLayer(t *testing.T) {
	law := map[string]string{"a/dev.layer.canon": "package a\nlayer dev\n\namend config {\n  port: 1\n}\n"}
	for name, text := range zLaw { //canon:unordered each file is copied under its own name
		law[name] = text
	}
	files := map[string]string{}
	for name, text := range editLaw { //canon:unordered each file is copied under its own name
		files[name] = text
	}
	for name, text := range law { //canon:unordered each file is copied under its own name
		files[name] = text
	}
	opts := project(files)
	opts.Layers, opts.EditLayer = []string{"dev"}, "dev"
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, op := range []canon.Op{canon.Remove("a:codes.z"), canon.Rename("a:codes.z", canon.Key("y")), canon.Remove("a:codes.first")} {
		_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{op}, DryRun: true})
		var ne *canon.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != canon.ReasonLayer {
			t.Errorf("API.md E4, E21, %+v under an edit layer: %v, want NotEditable reason layer", op, err)
		}
	}
}
