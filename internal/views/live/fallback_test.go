package live_test

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/views/render"
)

// things is a table whose views interpolate enum labels, refs, `none` texts and a list with
// equal titles; French translates some titles only.
var things = map[string]string{
	"a/a.canon": `package a

enum Goal { kill, collect }

record Ore {
  name: String
}

view Ore {
  title "Ore {name}"
}

record Gem {
  name: String
}

view Gem {
  title "Gem {key}"
}

record Piece {
  label: String
  ore: ref ores
  note: String?
}

view Piece {
  title "{label} {ore}"
  subtitle "{note}"
  note { none: "No note" }
}

record Thing {
  name: String
  goal: Goal
  ore: ref ores
  gem: ref gems
  note: String?
  pieces: [Piece]
  gemMap: {String: Gem} = {}
}

view Thing {
  title "Thing {name}"
  subtitle "{goal} {ore} {note}"
  note { none: "Nothing" }
  show "Sum" "{name} {goal}"
}

record Tag {
  note: String?
  alt: String?
}

view Tag {
  title "Tag {note}"
  subtitle "Alt {alt}"
  note { none: "No note" }
  alt { none: "No alt" }
}

record Holder {
  gem: ref gems
}

view Holder {
  title "{gem}"
}

let ores: table Ore = {
  o1 { name: "Rock" }
}

let gems: table Gem = {
  g1 { name: "Ruby" }
}

let tags: table Tag = {
  t { note: none, alt: none }
}

let holders: table Holder = {
  h1 { gem: g1 }
}

let things: table Thing = {
  t1 { name: "A", goal: kill, ore: o1, gem: g1, pieces: [{ label: "p", ore: o1 }, { label: "p", ore: o1 }] }
}
`,
	"a/a.fr.canon": `package a
translation fr

Thing.title "Chose {name}"
Tag.title "Étiquette {note}"
Tag.subtitle "Autre {alt}"
Tag.alt.none "Aucune"
`,
}

// counting is standIn counting every evaluation it makes, the steps a view spends.
type counting struct {
	standIn
	n *int
}

func (c counting) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	*c.n++
	return c.standIn.Eval(ctx, e, self, m)
}

// API.md V8, V13 (log-2026-09-29 M4 U9): detecting Fallback adds no evaluation: French with
// its catalogues evaluates exactly what the render-only baseline does (French without them, where
// no text can fall back), while it falls back through an enum label, a ref's title, a `none` text.
func TestFallbackSpendsNothing(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", things)
	run := func(in live.Input) (*live.Result, int) {
		n := 0
		in.Eval = counting{standIn: in.Eval.(standIn), n: &n}
		return at(t, in, live.Target{Value: entry(t, p.let(t, "a", "things"), "t1"), Name: "t1", Lang: "fr"}), n
	}
	bare := p.input()
	bare.I18N = nil
	baseline, rendering := run(bare)
	fr, detecting := run(p.input())
	if detecting != rendering || rendering == 0 {
		t.Errorf("API.md V13: %d evaluations with detection, %d rendering only", detecting, rendering)
	}
	texts := func(r *live.Result) []any {
		return []any{r.Title, r.Subtitle, r.Show[0].Text, r.Headings["pieces[0]"].Title}
	}
	same(t, "API.md V8", []any{texts(fr), texts(baseline)}, `[[
	 {"Value":"Chose A","OK":true,"Fallback":false}, {"Value":"kill Ore Rock Nothing","OK":true,"Fallback":true},
	 {"Value":"A kill","OK":true,"Fallback":true}, {"Value":"p Ore Rock (#1)","OK":true,"Fallback":true}],[
	 {"Value":"Chose A","OK":true,"Fallback":false}, {"Value":"kill Ore Rock Nothing","OK":true,"Fallback":false},
	 {"Value":"A kill","OK":true,"Fallback":false}, {"Value":"p Ore Rock (#1)","OK":true,"Fallback":false}]]`)
}

// API.md V8: a Lang given without the project's languages is API misuse (ErrNoLanguages).
func TestLangWithoutLanguages(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", things)
	in := p.input()
	in.Languages = nil
	_, err := live.Evaluate(context.Background(), in, live.Target{Value: entry(t, p.let(t, "a", "things"), "t1"), Lang: "fr"})
	if !errors.Is(err, live.ErrNoLanguages) {
		t.Errorf("err = %v, want ErrNoLanguages", err)
	}
	if _, err := live.Evaluate(context.Background(), in, live.Target{Value: entry(t, p.let(t, "a", "things"), "t1")}); err != nil {
		t.Errorf("no Lang: %v", err)
	}
}

// API.md V8 (VIEWMODEL.md X4; log-2026-09-29 M4 U9): a `none` rendered through its field's
// `none` text that falls back sets the text's Fallback; a translated one does not.
func TestFallbackNone(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", things)
	res := evaluate(t, p, entry(t, p.let(t, "a", "tags"), "t"), "t", "fr")
	same(t, "API.md V8 none", []any{res.Title, res.Subtitle}, `[
	 {"Value":"Étiquette No note","OK":true,"Fallback":true}, {"Value":"Autre Aucune","OK":true,"Fallback":false}]`)
}

// API.md V8, S8 (log-2026-09-29 M4 U9): a ref whose target title fails renders its key, which
// is no fallback.
func TestFallbackFailedTarget(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", things)
	res := evaluate(t, p, p.let(t, "a", "holders"), "holders", "fr")
	same(t, "API.md V8 S8", res.Headings["h1"].Title, `{"Value":"g1","OK":true,"Fallback":false}`)
}

// API.md V8 (log-2026-09-29 M4 U9): a Lang naming the source language is the source; a Lang the
// project does not have renders the source with every keyed text fallen back.
func TestLangs(t *testing.T) {
	p := demo(t, "  languages: [en, fr]\n", things)
	thing := entry(t, p.let(t, "a", "things"), "t1")
	source, named, unknown := evaluate(t, p, thing, "t1", ""), evaluate(t, p, thing, "t1", "en"), evaluate(t, p, thing, "t1", "xx")
	if jsonOf(t, source) != jsonOf(t, named) {
		t.Errorf("API.md V8 source:\n %s\n %s", jsonOf(t, source), jsonOf(t, named))
	}
	same(t, "API.md V8 unknown", []any{unknown.Title, unknown.Show[0].Label}, `[
	 {"Value":"Thing A","OK":true,"Fallback":true}, "Sum"]`)
}

// API.md V8 (VIEWMODEL.md X6, I18N.md F3, F5, T2): the translation used is the first non-empty
// entry of its key; when that one is broken the source is, and falls back.
func TestTranslationChoice(t *testing.T) {
	files := map[string]string{
		"a/a.canon": `package a

enum Goal { kill, collect }

record Pt {
  x: Int
  g: Goal
}

view Pt {
  title "Point {x}"
}

record Qt {
  x: Int
}

view Qt {
  title "Q {x}"
}

let pts: table Pt = {
  p1 { x: 1, g: kill }
}

let qts: table Qt = {
  q1 { x: 2 }
}
`,
		"a/a.fr.canon": "package a\ntranslation fr\n\nPt.title \"\"\nPt.title \"Point {g}\"\nQt.title \"Q {nope}\"\nQt.title \"Q {x}\"\n",
	}
	fsys := mapFS{"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n")}}
	for name, src := range files { //canon:unordered a map copied into a map
		fsys["law/"+name] = &fstest.MapFile{Data: []byte(src)}
	}
	p := analyze(t, setup{fsys: fsys, dir: lawDir, opt: build.Options{}})
	pt := evaluate(t, p, entry(t, p.let(t, "a", "pts"), "p1"), "p1", "fr")
	qt := evaluate(t, p, entry(t, p.let(t, "a", "qts"), "q1"), "q1", "fr")
	same(t, "API.md V8 X6", []any{pt.Title, qt.Title}, `[
	 {"Value":"Point kill","OK":true,"Fallback":true}, {"Value":"Q 2","OK":true,"Fallback":true}]`)
}
