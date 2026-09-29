package live_test

import (
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/views/live"
)

// ranked is a keyed list whose view title reads `index`, and boxes of picks whose titles and
// `text` column refer to its elements; French translates the picks' title only.
var ranked = map[string]string{
	"a/a.canon": `package a

record Lv {
  name: String
}

view Lv {
  title "Level {index}"
}

let lvs: [Lv] keyed by name = [{ name: "a" }, { name: "b" }]

record Pick {
  label: String
  lv: ref lvs | "none"
}

view Pick {
  title "Pick {label} {lv}"
  columns { label, lv }
}

record Box {
  picks: [Pick]
}

view Box {
  title "Box"
}

let boxes: table Box = {
  b { picks: [{ label: "x", lv: b }] }
}
`,
	"a/a.fr.canon": `package a
translation fr

Pick.title "Choix {label} {lv}"
`,
}

// API.md V6a, V7, V8 (VIEWMODEL.md S8, 3.4; log-2026-09-29 M4 U9b-r): a ref to a keyed list's
// element renders its title in the element's own place, its `index`, in a title and in a `text`
// cell, as its own heading does; in French the untranslated target title makes both fall back.
func TestTargetPlace(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", ranked)
	box := entry(t, p.let(t, "a", "boxes"), "b")
	lvs := at(t, p.real(), live.Target{Value: p.let(t, "a", "lvs"), Name: "lvs"})
	same(t, "API.md V6 3.4", lvs.Headings["[b]"].Title, `{"Value":"Level 2","OK":true,"Fallback":false}`)
	for _, in := range []live.Input{p.input(), p.real()} {
		src := at(t, in, live.Target{Value: box, Name: "b"}).Headings["picks[0]"]
		fr := at(t, in, live.Target{Value: box, Name: "b", Lang: "fr"}).Headings["picks[0]"]
		same(t, "API.md V6a V7 V8 S8", []any{src.Title, src.Cells["lv"], fr.Title, fr.Cells["lv"]}, `[
		 {"Value":"Pick x Level 2","OK":true,"Fallback":false}, {"Value":"Level 2","OK":true,"Fallback":false},
		 {"Value":"Choix x Level 2","OK":true,"Fallback":true}, {"Value":"Level 2","OK":true,"Fallback":true}]`)
	}
}

// broken names two methods the checker marks broken in a view.
var broken = map[string]string{
	"a/a.canon": `package a

record Box {
  n: Int

  fn bad(self) -> Int { return n + "x" }

  fn lost(self) -> Int { return nope(n) }
}

view Box {
  title "Box {n}"
  bad "Bad"
  lost "Lost"
}

let boxes: table Box = {
  b { n: 1 }
}
`,
}

// EVALUATION.md 1, API.md V10, V11, X2 (VIEWMODEL.md X7; log-2026-09-29 M4 U9b-r): a view item
// naming a broken method keeps its view; the method's line renders "—" (OK false), the rest of
// the view renders, and no internal error is left for a later Evaluate.
func TestBrokenMethod(t *testing.T) {
	fsys := mapFS{"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")}}
	fsys["law/a/a.canon"] = &fstest.MapFile{Data: []byte(broken["a/a.canon"])}
	p := analyze(t, setup{fsys: fsys, dir: lawDir})
	for range 2 {
		res := at(t, p.real(), live.Target{Value: entry(t, p.let(t, "a", "boxes"), "b"), Name: "b"})
		same(t, "API.md V10 API.md V11", []any{res.Title, res.Show}, `[{"Value":"Box 1","OK":true,"Fallback":false}, [
		 {"Owner":"","Key":"Box.bad","Label":"Bad","Text":{"Value":"","OK":false,"Fallback":false}},
		 {"Owner":"","Key":"Box.lost","Label":"Lost","Text":{"Value":"","OK":false,"Fallback":false}}]]`)
		if err := p.a.ViewErr(); err != nil {
			t.Errorf("API.md X2: %v", err)
		}
	}
}

// API.md 11 (log-2026-09-29 M4 U9b-r): the analysis's live inputs are the project's languages
// and studio and phase 2's texts: Evaluate reads them as it reads the ones checked again.
func TestLiveInputs(t *testing.T) {
	p := shopProject(t)
	li := p.a.LiveInputs()
	same(t, "API.md 11", []any{li.Studio, li.Languages}, `["", ["en","fr","de"]]`)
	in := p.real()
	in.I18N = li.I18N
	sword := p.item(t, "sword")
	got := at(t, in, live.Target{Value: sword, Name: "sword", Lang: "fr"})
	want := at(t, p.real(), live.Target{Value: sword, Name: "sword", Lang: "fr"})
	if jsonOf(t, got) != jsonOf(t, want) {
		t.Errorf("API.md V8:\n %s\n %s", jsonOf(t, got), jsonOf(t, want))
	}
}
