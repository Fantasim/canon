package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// baseKeySrc is p: aa's second holds x (w 3) and y, bb spreads second; {more} reads x beside them.
const baseKeySrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by name\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ name: \"x\", w: 2 }] },\n" +
	"  { id: \"second\", oo: [{ name: \"x\", w: 3 }, { name: \"y\" }] },\n]\n\n" +
	"let bb: A = { ...aa.second, id: \"third\" }\n\n{more}"

// baseKeyLayer replaces bb's oo with a literal holding its own x (w 1) and z (w 9).
const baseKeyLayer = "package p\nlayer dev\n\namend bb {\n  oo: [{ name: \"x\", w: 1 }, { name: \"z\", w: 9 }]\n}\n"

// baseKeyRow is a read of x beside baseKeySrc: let names the let holding the token, listed whether
// Refs lists it with the layer active, detail the active refusal's Detail.
type baseKeyRow struct {
	name, more, let string
	listed          bool
	detail          string
	renamed         bool // the let holds the key: its value follows the rename
}

// API.md R7, E11, E12, E19, E22 (DECISIONS 483, 489, 503): with bb's oo a layer literal, each key read of x
// through bb (listed) or through a copy with no base kept is refused `computed` naming it, nothing written;
// inactive, the rename applies, each value kept or following it, and the Undo gives the bytes back.
func TestRenameBaseOnlyKeyPositions(t *testing.T) {
	const dd = "let dd: A = { ...bb }\n\n"
	rows := []baseKeyRow{
		{"hasKey", "let hk: Bool = bb.oo.hasKey(\"x\")\n", "hk", true, "p:bb.oo[x]", false},
		{"get", "let g: Bool = bb.oo.get(\"x\") != none\n", "g", true, "p:bb.oo[x]", false},
		{"find", "let g: Bool = bb.oo.find(\"x\") != none\n", "g", true, "p:bb.oo[x]", false},
		{"string index", "let ix: Int = bb.oo[\"x\"].w\n", "ix", true, "p:bb.oo[x]", false},
		{"map word key", "let mk: {ref bb.oo: Int} = { x: 1 }\n", "mk", true, "p:bb.oo[x]", true},
		{"map string key", "let mk: {ref bb.oo: Int} = { \"x\": 1 }\n", "mk", true, "p:bb.oo[x]", true},
		{"nested map key", "let ml: [{ref bb.oo: Int}] = [{ x: 1 }]\n", "ml", true, "p:bb.oo[x]", true},
		{"read through a copy", dd + "let rw: Int = dd.oo.x.w\n", "rw", false, "p:dd.oo[x]", false},
		{"word ref through a copy", dd + "let q: ref dd.oo = x\n", "q", false, "p:dd.oo[x]", true},
		{"read through a copy's entry", "let ee: [A] keyed by id = [{ ...bb, id: \"e\" }]\n\nlet rw2: Int = ee.e.oo.x.w\n", "rw2", false, "p:ee[e].oo[x]", false},
		{"string index through a copy's entry", "let ee: [A] keyed by id = [{ ...bb, id: \"e\" }]\n\nlet ix: Int = ee.e.oo[\"x\"].w\n", "ix", false, "p:ee[e].oo[x]", false},
	}
	for _, r := range rows {
		src := formatted(t, srcMain, strings.ReplaceAll(baseKeySrc, "{more}", r.more))
		files := map[string]string{srcMain: src, sharedLayer: baseKeyLayer}
		t.Run(r.name+" active true", func(t *testing.T) {
			want := ""
			if r.listed {
				want = fmt.Sprintf("code %s:%d", srcMain, lineOf(src, "let "+r.let+":"))
			}
			sharedRow{l: sharedLevel{path: secondX}, layers: []string{"dev"}, before: files, want: want, refused: canon.ReasonComputed, to: "z"}.run(t)
			refusedDetail(t, files, []string{"dev"}, secondX, r.detail)
		})
		t.Run(r.name+" active false", func(t *testing.T) { baseKeyInactive(t, files, r.let, r.renamed) })
	}
}

// baseKeyInactive renames second's x to z with the layer inactive: applied, let's value kept (renamed: x now z),
// and its Undo gives every file back (API.md E11, E22).
func baseKeyInactive(t *testing.T, files map[string]string, let string, renamed bool) {
	t.Helper()
	p, m := sharedOpen(t, files, nil)
	ctx := context.Background()
	before, err := p.Value(ctx, "p:"+let)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(secondX, canon.Key("z"))}})
	if err != nil {
		t.Fatalf("API.md E11: the rename is refused: %v", err)
	}
	want := before.Text
	if renamed {
		want = strings.ReplaceAll(want, "x", "z")
	}
	if after, err := p.Value(ctx, "p:"+let); err != nil || after.Text != want {
		t.Errorf("API.md E11: %s is %v, %v after the rename, want %s", let, after, err, want)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo: %v", err)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if read(t, m, name) != files[name] {
			t.Errorf("API.md E22: %s after the Undo:\n%s", name, read(t, m, name))
		}
	}
}

// API.md E12, R8 (DECISIONS 503): a refusal's Detail lists the refs not editable in R8 order, the
// session's and the plan's merged (rr before rlate), then the shared paths.
func TestRenameRefusalDetailMergedInOrder(t *testing.T) {
	more := "let rr: ref bb.oo = bb.oo.x\n\nlet cc: A = { ...aa.second, id: \"c\" }\n\nlet rlate: ref cc.oo = cc.oo.x\n"
	files := map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(baseKeySrc, "{more}", more)), sharedLayer: baseKeyLayer}
	refusedDetail(t, files, []string{"dev"}, secondX, "p:rr, p:rlate, p:bb.oo[x]")
	refusedDetail(t, files, nil, secondX, "p:rr, p:rlate")
	p, _ := sharedOpen(t, files, []string{"dev"})
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(secondX, canon.Key("z"))}})
	var ne *canon.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonComputed {
		t.Errorf("API.md E12: the refusal is %v, want computed", err)
	}
}
