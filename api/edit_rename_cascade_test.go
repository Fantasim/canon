package canon_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// cascadeSrc is COMPILER-ISSUES #3's package p, grown: ms's wolf keys z.spawns and spawns, pick
// refers to spawns[wolf], and ts is keyed by a ref to spawns: a two-level cascade.
const cascadeSrc = "package p\n\nrecord M {\n  hp: Int\n}\n\nlet ms: table M = {\n  wolf { hp: 1 }\n  bear { hp: 2 }\n}\n\n" +
	"record S {\n  m: ref ms\n  n: Int\n}\n\nrecord Z {\n  spawns: [S] keyed by m\n}\n\nlet z: Z = { spawns: [{ m: wolf, n: 2 }] }\n\n" +
	"let spawns: [S] keyed by m = [{ m: wolf, n: 2 }, { m: bear, n: 1 }]\n\nlet pick: ref spawns = wolf\n\n" +
	"record T {\n  s: ref spawns\n  k: Int\n}\n\nlet ts: [T] keyed by s = [{ s: wolf, k: 1 }]\n"

// cascadeLayer amends z.spawns[wolf]: a layer path into an element the rename moves.
const cascadeLayer = "package p\nlayer dev\n\namend z {\n  spawns[wolf].n: 3\n}\n"

// renameWolf renames p:ms.wolf to timber.
var renameWolf = canon.Edit{Ops: []canon.Op{canon.Rename("p:ms.wolf", canon.Key("timber"))}}

// API.md E11, E12, 7.2, E15, E23, DECISIONS 310 (COMPILER-ISSUES #3): a rename rewrites the
// keyed-list keys it names; each element moves with every ref and path into it, cascading in
// turn; nothing is dropped, and the Undo restores every file byte for byte.
func TestEditRenameCascadesKeys(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, srcLaw(cascadeSrc, "p/dev.layer.canon", cascadeLayer))
	if _, err := p.Edit(ctx, canon.Edit{DryRun: true, Ops: renameWolf.Ops}); err != nil {
		t.Fatalf("API.md E12, DECISIONS 310: dry run: %v", err)
	}
	before := []string{read(t, m, srcMain), read(t, m, "p/dev.layer.canon")}
	res, err := p.Edit(ctx, renameWolf)
	if err != nil {
		t.Fatalf("API.md E11: %v", err)
	}
	src := read(t, m, srcMain)
	for _, want := range []string{"spawns: [{ m: timber, n: 2 }]", "[{ m: timber, n: 2 }, { m: bear, n: 1 }]", "pick: ref spawns = timber", "[{ s: timber, k: 1 }]"} {
		if !strings.Contains(src, want) {
			t.Errorf("API.md E11: %q missing after the rename:\n%s", want, src)
		}
	}
	if got := read(t, m, "p/dev.layer.canon"); !strings.Contains(got, "spawns[timber].n: 3") {
		t.Errorf("API.md E11: layer path after the rename:\n%s", got)
	}
	for path, want := range map[string]string{"p:z.spawns[timber].n": "2", "p:ts[timber].k": "1", "p:spawns[timber].n": "2"} { //canon:unordered each path is read alone
		if v, err := p.Value(ctx, path); err != nil || v.Text != want {
			t.Errorf("API.md E11: %s: %+v, %v, want %s", path, v, err, want)
		}
	}
	if len(res.Dropped) != 0 {
		t.Errorf("API.md E15: a rewritten key names the same entry, yet %+v dropped", res.Dropped)
	}
	if !slices.EqualFunc(res.Undo, []canon.Op{canon.Rename("p:ms.timber", canon.Key("wolf"))}, sameOp) {
		t.Errorf("API.md E23: Undo %+v, want the inverse Rename alone", res.Undo)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E23: the Undo: %v", err)
	}
	if after := []string{read(t, m, srcMain), read(t, m, "p/dev.layer.canon")}; !slices.Equal(after, before) {
		t.Errorf("API.md E22: the files after the Undo:\n%s\n%s", after[0], after[1])
	}
}

// API.md E1, E11, E23, DECISIONS 310: a later op of the request reaches a moved element by its
// new path, and the Undo gives both back.
func TestEditRenameCascadesThenSet(t *testing.T) {
	ctx := context.Background()
	p, m := openEdit(t, srcLaw(cascadeSrc))
	before := read(t, m, srcMain)
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{renameWolf.Ops[0], canon.Set("p:spawns[timber].n", canon.Int(5)), canon.Set("p:ts[timber].k", canon.Int(6))}})
	if err != nil {
		t.Fatalf("API.md E1, E11: %v", err)
	}
	if v, err := p.Value(ctx, "p:ts[timber].k"); err != nil || v.Text != "6" {
		t.Errorf("API.md E11: p:ts[timber].k: %+v, %v", v, err)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E23: the Undo %+v: %v", res.Undo, err)
	}
	if got := read(t, m, srcMain); got != before {
		t.Errorf("API.md E22: source after the Undo:\n%s", got)
	}
}

// API.md E11, N8, E23, DECISIONS 310: a moved element of an @files keyed list keeps its file
// named by the template: it is renamed with its `entry` line, and back by the Undo.
func TestEditRenameCascadesFiles(t *testing.T) {
	ctx := context.Background()
	src := strings.Replace(cascadeSrc, "let spawns: [S] keyed by m = [{ m: wolf, n: 2 }, { m: bear, n: 1 }]", "@files(\"sp/{m}.canon\")\nlet spawns: [S] keyed by m = []", 1)
	wolf := "package p\n\nentry spawns.wolf { n: 2 }\n"
	p, m := openEdit(t, srcLaw(src, "p/sp/wolf.canon", wolf, "p/sp/bear.canon", "package p\n\nentry spawns.bear { n: 1 }\n"))
	res, err := p.Edit(ctx, renameWolf)
	if err != nil {
		t.Fatalf("API.md E11: %v", err)
	}
	if got := read(t, m, "p/sp/timber.canon"); got != "package p\n\nentry spawns.timber { n: 2 }\n" {
		t.Errorf("API.md E11, N8: the moved element's file: %q", got)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E23: the Undo: %v", err)
	}
	if got := read(t, m, "p/sp/wolf.canon"); got != wolf {
		t.Errorf("API.md E22: the element's file after the Undo: %q", got)
	}
}

// API.md E3, E13, DECISIONS 310: a stable table still refuses any rename with ErrStableKey,
// and a new key the table holds with ErrKeyExists, before any cascade. (A cascaded element
// whose new key its list already holds needs a dangling key, which leaves ms without a value.)
func TestEditRenameCascadesRefused(t *testing.T) {
	stable := strings.Replace(cascadeSrc, "let ms: table M", "let ms: stable table M", 1)
	taken := canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Rename("p:ms.wolf", canon.Key("bear"))}}
	for _, c := range []struct {
		name string
		law  map[string]string
		edit canon.Edit
		want error
	}{
		{"a stable table", srcLaw(stable, "p/canon.lock", "# canon.lock v1\ntable  p.ms  bear\ntable  p.ms  wolf\n"), canon.Edit{AllowErrors: true, Ops: renameWolf.Ops}, canon.ErrStableKey},
		{"a key the table holds", srcLaw(cascadeSrc), taken, canon.ErrKeyExists},
	} {
		p, m := openEdit(t, c.law)
		before := read(t, m, srcMain)
		_, err := p.Edit(context.Background(), c.edit)
		if !errors.Is(err, c.want) || read(t, m, srcMain) != before {
			t.Errorf("API.md E3, E13, %s: %v, want %v and nothing written", c.name, err, c.want)
		}
	}
}
