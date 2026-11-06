package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
)

const (
	layerBrokenMain  = "p/p.canon"
	layerBrokenLayer = "p/dev.layer.canon"
	layerBrokenLet   = "p/k.canon" // kk alone: a Recheck checks an annotated let again, never a fn
	// layerBrokenHead is p: pk picks a tt entry; uu is another S list; rt and ru refs into each.
	layerBrokenHead = "package p\n\nlocal record S {\n  name: String\n}\n\nlocal record P {\n  pick: ref tt\n}\n\n" +
		"local let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n\n" +
		"local let uu: [S] keyed by name = [{ name: \"x\" }]\n\nlocal let pk: P = { pick: tt.y }\n\n" +
		"local let rt: ref tt = tt.x\n\nlocal let ru: ref uu = uu.x\n\n" +
		"local fn gx() -> ref tt {\n  return fx()\n}\n\nlocal let aa: S = { name: 1 }\n\n" +
		"local record Bad {\n  n: Int\n\n  check n in 3 else \"x\"\n}\n\n"
	pickFx = "amend pk {\n  pick: fx()\n}\n"
	pickGx = "amend pk {\n  pick: gx()\n}\n"
)

// layerBrokenWorld is p with fx returning body, kk initialized by kk in a file of its own, and a
// layer of the amend blocks amends; aa and Bad are broken, gx calls fx.
func layerBrokenWorld(t *testing.T, body, kk, amends string) *world {
	t.Helper()
	return newWorld(t, map[string]string{
		layerBrokenMain:  layerBrokenHead + "local fn fx() -> ref tt {\n  " + body + "\n}\n",
		layerBrokenLet:   "package p\n\nlocal let kk: ref tt = " + kk + "\n",
		layerBrokenLayer: "package p\nlayer dev\n\n" + amends,
	})
}

// layerBroken reports the layer file of w broken in prog.
func layerBroken(t *testing.T, w *world, prog *check.Program) bool {
	t.Helper()
	f := w.files[layerBrokenLayer]
	o := prog.Info.Defs[f.Layer]
	if o == nil {
		t.Fatal("the layer file has no object")
	}
	return prog.Info.Broken[o]
}

// TYPES.md §1, DECISIONS 514: a layer naming a broken declaration is broken; the let it amends is no such name.
func TestLayerNamingBrokenFnIsBroken(t *testing.T) {
	rows := []struct {
		name, body, amends string
		broken             bool
	}{
		{"a record of another type", "return P { pick: tt.x }", pickFx, true},
		{"an untyped record literal", "return { name: \"x\" }", pickFx, true},
		{"a ref of another collection", "let r: ref uu = uu.x\n  return r", pickFx, true},
		{"a sound fn calling the broken one", "return P { pick: tt.x }", pickGx, true},
		{"a broken record named", "return tt.x", "amend pk {\n  pick: [tt.x, tt.y][Bad { n: 1 }.n]\n}\n", true},
		{"an entry of tt", "return tt.x", pickFx, false},
		{"a sound fn calling a sound one", "return tt.x", pickGx, false},
		{"a record of tt's element type, judged at evaluation (DECISIONS 185)", "return S { name: \"x\" }", pickFx, false},
		{"a broken let amended", "return tt.x", "amend aa {\n  name: \"a\"\n}\n\n" + pickFx, false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			w := layerBrokenWorld(t, row.body, "rt", row.amends)
			prog, _, _ := w.session()
			if got := layerBroken(t, w, prog); got != row.broken {
				t.Errorf("TYPES.md §1, DECISIONS 514: the layer is broken=%v, want %v", got, row.broken)
			}
		})
	}
}

// TYPES.md §1, DECISIONS 514, NFR-02: a Recheck breaking or mending a let a layer reads does so to the layer.
func TestRecheckLayerReadingBrokenLet(t *testing.T) {
	w := layerBrokenWorld(t, "return tt.x", "rt", "amend pk {\n  pick: kk\n}\n")
	prog, s, _ := w.session()
	if layerBroken(t, w, prog) {
		t.Fatal("DECISIONS 514: the layer reading a sound kk is broken")
	}
	for _, st := range []struct {
		old, new string
		broken   bool
	}{
		{"ref tt = rt", "ref tt = ru", true}, // a ref of uu is no ref of tt (TYPES.md §6.2)
		{"ref tt = ru", "ref tt = rt", false},
	} {
		prog, next, bags, ok := w.recheck(s, w.edit(layerBrokenLet, st.old, st.new))
		if !ok {
			t.Fatalf("%q to %q: Recheck refused a let's body edit", st.old, st.new)
		}
		if got := layerBroken(t, w, prog); got != st.broken {
			t.Errorf("TYPES.md §1, DECISIONS 514: after %q, the layer reading kk is broken=%v, want %v", st.new, got, st.broken)
		}
		if got, want := canonical(w.fs, prog, bags), w.cold(); got != want {
			t.Fatalf("NFR-02: after %q, Recheck differs from Check:\n%s", st.new, firstDiff(got, want))
		}
		s = next
	}
}
