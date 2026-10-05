package canon_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// refKeysHead is ms, keyed by name, and S, keyed by a ref to ms.
const refKeysHead = "package p\n\nrecord M {\n  hp: Int\n}\n\nlet ms: table M = {\n  wolf { hp: 1 }\n  bear { hp: 2 }\n}\n\nrecord S {\n  m: ref ms\n  n: Int\n}\n\n"

// renameWolfChecked renames p:ms.wolf to timber in law, checks each want is in its file after
// it, and that the Undo gives every file back byte for byte.
func renameWolfChecked(t *testing.T, rule string, law map[string]string, want map[string][]string) {
	t.Helper()
	ctx := context.Background()
	p, m := openEdit(t, law)
	files := slicesSorted(want)
	var before []string
	for _, f := range files {
		before = append(before, read(t, m, f))
	}
	res, err := p.Edit(ctx, renameWolf)
	if err != nil {
		t.Errorf("%s: %v", rule, err)
		return
	}
	for _, f := range files {
		for _, w := range want[f] {
			if got := read(t, m, f); !strings.Contains(got, w) {
				t.Errorf("%s: %q missing from %s after the rename:\n%s", rule, w, f, got)
			}
		}
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Errorf("API.md E23, %s: the Undo: %v", rule, err)
		return
	}
	for i, f := range files {
		if got := read(t, m, f); got != before[i] {
			t.Errorf("API.md E22, %s: %s after the Undo:\n%s", rule, f, got)
		}
	}
}

// slicesSorted are the keys of m in order.
func slicesSorted(m map[string][]string) []string {
	var out []string
	for k := range m { //canon:unordered sorted below
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// API.md 5.3, E11, DECISIONS 316 (310 review 1): a key token typed `ref C` in a layer's path
// (`ts[wolf]`, ts keyed by a ref to spawns) is a layer ref of the entry it names, so a two-level
// rename cascade rewrites it, the layer inactive.
func TestEditRefKeyInLayerPath(t *testing.T) {
	src := refKeysHead + "let spawns: [S] keyed by m = [{ m: wolf, n: 2 }]\n\nrecord T {\n  s: ref spawns\n  k: Int\n}\n\n" +
		"record Y {\n  ts: [T] keyed by s\n}\n\nlet y: Y = { ts: [{ s: wolf, k: 1 }] }\n"
	law := srcLaw(src, "p/dev.layer.canon", "package p\nlayer dev\n\namend y {\n  ts[wolf].k: 3\n}\n")
	p, _ := openEdit(t, law)
	refs, err := p.Refs(context.Background(), "p:spawns[wolf]")
	if err != nil || !slices.ContainsFunc(refs.Refs, func(r canon.Ref) bool { return r.Kind == canon.RefLayer && r.Span.File == "p/dev.layer.canon" }) {
		t.Errorf("API.md 5.3, DECISIONS 316: Refs(p:spawns[wolf]) %+v, %v, want the layer key", refs, err)
	}
	renameWolfChecked(t, "API.md E11, DECISIONS 316", law, map[string][]string{
		srcMain: {"[{ m: timber, n: 2 }]", "ts: [{ s: timber, k: 1 }]"}, "p/dev.layer.canon": {"ts[timber].k: 3"},
	})
}

// API.md 5.3, E11, DECISIONS 315, 316 (310 review 2): refs typed `ref z.spawns`, a let path to a
// record field's keyed list, are refs of that instance's elements; the cascade follows them.
func TestEditRefKeyThroughFieldList(t *testing.T) {
	src := refKeysHead + "record Z {\n  spawns: [S] keyed by m\n}\n\nlet z: Z = { spawns: [{ m: wolf, n: 2 }] }\n\n" +
		"record T {\n  s: ref z.spawns\n  k: Int\n}\n\nrecord Y {\n  ts: [T] keyed by s\n}\n\nlet y: Y = { ts: [{ s: wolf, k: 1 }] }\n\nlet pz: ref z.spawns = wolf\n"
	p, _ := openEdit(t, srcLaw(src))
	refs, err := p.Refs(context.Background(), "p:z.spawns[wolf]")
	if paths := refPaths(refs); err != nil || !slices.Equal(paths, []string{"y.ts[wolf].s", "pz"}) {
		t.Errorf("API.md 5.3, DECISIONS 315: Refs(p:z.spawns[wolf]) %v, %v", paths, err)
	}
	renameWolfChecked(t, "API.md E11, DECISIONS 315", srcLaw(src), map[string][]string{
		srcMain: {"spawns: [{ m: timber, n: 2 }]", "ts: [{ s: timber, k: 1 }]", "pz: ref z.spawns = timber"},
	})
}

// refPaths are the paths of r's refs.
func refPaths(r *canon.RefsResult) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, x := range r.Refs {
		out = append(out, x.Path)
	}
	return out
}

// DECISIONS 316, ERRORS.md E3012 (310 review 3): a keyed list whose ref key leads back to itself
// has no base key type; reading or editing one of its elements is an error, never a crash.
func TestEditSelfKeyedList(t *testing.T) {
	ctx := context.Background()
	self := "package p\n\nrecord N {\n  p: ref ns\n  v: Int\n}\n\nlet ns: [N] keyed by p = [{ p: a, v: 1 }]\n"
	for _, c := range []struct{ src, path string }{
		{self, "p:ns[a]"},
		{"package p\n\nrecord A {\n  b: ref bs\n}\n\nrecord B {\n  a: ref as\n}\n\nlet as: [A] keyed by b = [{ b: x }]\n\nlet bs: [B] keyed by a = [{ a: x }]\n", "p:as[x]"},
		{self + "\nlet mm: {ref ns: Int} = {}\n", "p:mm[a]"}, // a healthy root whose path key is read through ns's keys
	} {
		p, _ := openEdit(t, srcLaw(c.src))
		if _, err := p.Value(ctx, c.path); err == nil {
			t.Errorf("DECISIONS 316: Value(%s) has a value", c.path)
		}
		for _, op := range []canon.Op{canon.Rename(c.path, canon.Key("b")), canon.Remove(c.path), canon.Set(c.path+".v", canon.Int(2))} {
			if _, err := p.Edit(ctx, canon.Edit{AllowErrors: true, DryRun: true, Ops: []canon.Op{op}}); err == nil {
				t.Errorf("DECISIONS 316: %+v applied", op)
			}
		}
	}
}
