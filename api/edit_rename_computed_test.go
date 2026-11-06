package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// directReadSrc is p: bb spreads aa's second, and rr refers to x through bb's own let path.
const directReadSrc = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by name\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ name: \"x\" }] },\n" +
	"  { id: \"second\", oo: [{ name: \"x\" }, { name: \"y\" }] },\n]\n\n" +
	"let bb: A = { ...aa.second, id: \"third\" }\n\nlet rr: ref bb.oo = bb.oo.x\n"

// literalListLayer replaces bb's oo with a literal list holding its own x and z.
const literalListLayer = "package p\nlayer dev\n\namend bb {\n  oo: [{ name: \"x\" }, { name: \"z\" }]\n}\n"

// API.md R7, E11, E12, E19 (DECISIONS 483, 489, 503): a layer replacing bb's oo with a literal makes
// rr's layered read bb's own x, its base one second's: active, rr's `.x` is listed from the base and
// the rename, which would retarget it and has rr's computed ref on its plan, is refused `computed`.
func TestRenameRefusedForBaseOnlyRead(t *testing.T) {
	src := formatted(t, srcMain, directReadSrc)
	files := map[string]string{srcMain: src, sharedLayer: literalListLayer}
	rows := []struct {
		layers       []string
		want, detail string
	}{
		{[]string{"dev"}, fmt.Sprintf("code %s:%d", srcMain, lineOf(src, "let rr:")), "p:rr, p:bb.oo[x]"},
		{nil, fmt.Sprintf("value %[1]s:%[2]d, code %[1]s:%[2]d", srcMain, lineOf(src, "let rr:")), "p:rr"},
	}
	for _, r := range rows {
		t.Run(fmt.Sprintf("active %v", r.layers != nil), func(t *testing.T) {
			sharedRow{l: sharedLevel{path: secondX}, layers: r.layers, before: files, want: r.want, refused: canon.ReasonComputed, to: "z"}.run(t)
			refusedDetail(t, files, r.layers, secondX, r.detail)
		})
	}
}

// API.md E11, E12 (DECISIONS 489, 503): through dd and ee, unamended copies of bb, the base read is
// not kept: the plan's computed ref rr and the reads of the layer's own x refuse the rename `computed`.
func TestRenameRefusedForPlanComputedRef(t *testing.T) {
	files := map[string]string{srcMain: formatted(t, srcMain, twoLetsSrc), sharedLayer: literalListLayer}
	p, m := sharedOpen(t, files, []string{"dev"})
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(secondX, canon.Key("z"))}})
	var ne *canon.NotEditableError
	const detail = "p:rr, p:dd.oo[x], p:ee[e].oo[x]"
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonComputed || ne.Detail != detail {
		t.Fatalf("API.md E12: the rename gives %v, want computed, Detail %s", err, detail)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if read(t, m, name) != files[name] {
			t.Errorf("API.md E19: %s was written", name)
		}
	}
}

// refusedDetail checks that renaming path to z with layers active is refused with Detail detail.
func refusedDetail(t *testing.T, files map[string]string, layers []string, path, detail string) {
	t.Helper()
	p, _ := sharedOpen(t, files, layers)
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(path, canon.Key("z"))}})
	var ne *canon.NotEditableError
	if !errors.As(err, &ne) || ne.Detail != detail {
		t.Errorf("API.md E12 (DECISIONS 503): the refusal is %v, want Detail %q", err, detail)
	}
}

// letName is a let's declaration name.
var letName = regexp.MustCompile(`(?m)^let (\w+)`)

// API.md E11, E12 (DECISIONS 489, 503): a computed ref has no token, so no rename, applied or undone,
// ever rewrites a let's declaration name on its behalf, whichever layers are active.
func TestRenameNeverRewritesLetNames(t *testing.T) {
	cases := []struct {
		name, src, layer, path string
	}{
		{"direct read, literal list", directReadSrc, literalListLayer, secondX},
		{"read through a copy, literal list", twoLetsSrc, literalListLayer, secondX},
		{"read through a copy, two amended lets", twoLetsSrc, "package p\nlayer dev\n\namend dd {\n  oo.y.w: 4\n}\n\namend bb {\n  oo.x.w: 5\n}\n", secondX},
		{"function-computed refs", fnRefSrc, "package p\nlayer dev\n", "p:tt[x]"},
		{"a ref only a layer computes", layerFnSrc, "package p\nlayer dev\n\namend pk {\n  pick: fx()\n}\n", "p:tt[x]"},
		{"spread copy, literal list", spreadLayerSrc, "package p\nlayer dev\n\n" + amendBB(`oo: [{ name: "x", w: 7 }]`), secondX},
		{"spread copy, literal list, first's x", spreadLayerSrc, "package p\nlayer dev\n\n" + amendBB(`oo: [{ name: "x", w: 7 }]`), "p:aa[first].oo[x]"},
		{"two amended lets, first's x", twoLetsSrc, "package p\nlayer dev\n\namend bb {\n  oo.x.w: 5\n}\n", "p:aa[first].oo[x]"},
	}
	applied := 0
	t.Cleanup(func() {
		if applied == 0 {
			t.Error("no row applied a rename: the guard checks refusals only")
		}
	})
	for _, c := range cases {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", c.name, layers != nil), func(t *testing.T) {
				files := map[string]string{srcMain: formatted(t, srcMain, c.src), sharedLayer: c.layer}
				p, m := sharedOpen(t, files, layers)
				res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(c.path, canon.Key("z"))}})
				sameLets(t, "the rename", files[srcMain], read(t, m, srcMain))
				if err != nil {
					return
				}
				applied++
				if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
					t.Fatalf("API.md E22: the Undo: %v", err)
				}
				sameLets(t, "the Undo", files[srcMain], read(t, m, srcMain))
			})
		}
	}
}

// sameLets fails when after declares other lets than before.
func sameLets(t *testing.T, what, before, after string) {
	t.Helper()
	names := func(src string) string {
		var out []string
		for _, m := range letName.FindAllStringSubmatch(src, -1) {
			out = append(out, m[1])
		}
		return strings.Join(out, " ")
	}
	if got, want := names(after), names(before); got != want {
		t.Errorf("API.md E11: after %s the lets are %q, want %q", what, got, want)
	}
}
