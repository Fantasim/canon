package views_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// studioFull is a studio package whose units have every member of VIEWMODEL.md 12.9.
const studioFull = `/// The studio.
package studio

/// Menus.
enum Menu { items, system }

/// Icons.
enum Icon { gem, gear }

/// Tones.
enum Tone { info, danger }

/// A unit.
record UnitSpec {
  /// Appended.
  suffix: String = ""
  /// Shown = stored × scale.
  scale: Float = 1.0
  /// Grouped.
  thousands: Bool = true
  /// Digits.
  decimals: Int(0..=6) = 0
}

/// Units.
let units: table UnitSpec = {
  hp { suffix: " HP" }
  pct { suffix: " %" }
  ppb { scale: 0.5, thousands: false, decimals: 2 }
  weight {}
}

/// A time of day.
record Clock {
  /// Hours.
  h: Int
}

/// One input for a time of day.
widget clock(value: Clock) default

/// A weight and its share.
widget weight_share(value: Int, siblings: [Int])
`

// valued are public values of every source form, with a menu from a view or `@menu`, a layer
// amending one of them, and fields naming units and widgets.
var valued = map[string]string{
	"studio/studio.canon": studioFull,
	"a/a.canon": `package a

import studio

record Pt {
  x: Int
  hp: Int = 0
  share: Int = 0
  ratio: Int = 0
  at: studio.Clock?
}

view Pt {
  menu items icon gem
  hp { unit: hp }
  share { widget: weight_share }
  ratio { unit: pct }
}

/// Every point.
@reload
let points: [Pt] = [{ x: 1, hp: 3 }, { x: 2 }, { x: 3 }]

@menu(system, icon: gear, label: "All settings")
let settings: Pt = { x: 1 }

let sum: Int = 1 + 2

local let hidden: Int = 3

let loaded: [Pt] = load("data/pts.json")

let many: [Pt] = load.dir("many/*.json")
`,
	"a/data/pts.json":     `[{"x": 1}]`,
	"a/many/p1.json":      `{"x": 1}`,
	"a/many/p2.json":      `{"x": 2, "hp": 4}`,
	"a/night.layer.canon": "package a\nlayer night\n\namend settings {\n  x: 9\n}\n",
}

// VIEWMODEL.md 12.6, N1, N3, T2, API.md W1: each public value's order, type, label, help, menu,
// control, edit mode and reason, `@reload`, amending layers and sources (loaded file, glob and
// files matched).
func TestValues(t *testing.T) {
	m := tree(t, "", valued, build.Options{Layers: []string{"night"}}).model(t, demoPkg)
	if _, ok := m.Values["a:hidden"]; ok || len(m.Values) != 5 {
		t.Errorf("12.6: values %v", keysOf(m.Values))
	}
	expect(t, "12.6 N1 T2", m.Values["a:points"], `{"name":"points","order":0,"type":{"kind":"list","of":{"kind":"record","ref":"a.Pt"}},
	 "label":"a:points","help":"a:points.help","menu":{"menu":"items","icon":"gem"},
	 "control":{"kind":"table","of":"a.Pt","orderable":true,"columns":[{"field":"x","mode":"edit","cell":{"kind":"number"}},
	  {"field":"hp","mode":"edit","cell":{"kind":"number","unit":"hp"}},
	  {"field":"share","mode":"edit","cell":{"kind":"widget","widget":"weight_share","siblings":true,"fallback":{"kind":"number"}}},
	  {"field":"ratio","mode":"edit","cell":{"kind":"number","unit":"pct"}}]},
	 "editable":"canon","reload":true,"sources":{"files":["a/a.canon"]}}`)
	expect(t, "N1 N3 12.6 layers", []any{m.Values["a:settings"].Menu, m.Values["a:settings"].Layers}, `[{"menu":"system","icon":"gear"},["night"]]`)
	expect(t, "12.6 reason", []any{m.Values["a:sum"].Editable, m.Values["a:sum"].Reason, m.Values["a:sum"].Order}, `["none","computed",2]`)
	expect(t, "12.6 sources", []any{m.Values["a:loaded"].Sources, m.Values["a:loaded"].Editable}, `[{"files":["a/a.canon","a/data/pts.json"]},"json"]`)
	expect(t, "12.6 T2 glob", []any{m.Values["a:many"].Sources, m.Values["a:many"].Control.Orderable}, `[{"files":["a/a.canon"],"glob":"a/many/*.json","count":2},false]`)
	if m.I18N.Languages["en"].Texts["settings"] != "All settings" {
		t.Errorf("N3: the label of settings is %q", m.I18N.Languages["en"].Texts["settings"])
	}
}

// VIEWMODEL.md 12.9, 12.1: the units and widgets the controls name (a default widget included),
// suffixes keyed, neutral or absent; the studio's model holds its whole vocabulary; the studio
// is required.
func TestStudioSections(t *testing.T) {
	x := tree(t, "", valued, build.Options{Layers: []string{"night"}})
	m := x.model(t, demoPkg)
	expect(t, "12.9 units", m.Units, `{"hp":{"suffix":"studio:units.hp.suffix","scale":1,"thousands":true,"decimals":0},
	 "pct":{"suffix":{"text":" %"},"scale":1,"thousands":true,"decimals":0}}`)
	expect(t, "12.9 widgets G22", m.Widgets, `{"clock":{"value":{"kind":"record","ref":"studio.Clock"},"default":true,"help":"One input for a time of day."},
	 "weight_share":{"value":{"kind":"int","bits":64,"signed":true},"siblings":{"kind":"list","of":{"kind":"int","bits":64,"signed":true}},"help":"A weight and its share."}}`)
	expect(t, "12.1 requires", m.Requires, `["studio"]`)
	s := x.model(t, studioPkg).Studio
	expect(t, "12.9 studio", []any{s.Menus, s.Icons, s.Tones, s.Units["ppb"], s.Units["weight"], len(s.Widgets)},
		`["studio.Menu",["gem","gear"],["info","danger"],{"scale":0.5,"thousands":false,"decimals":2},{"scale":1,"thousands":true,"decimals":0},2]`)
	if m.Studio != nil {
		t.Error("12.9: a package other than the studio has a studio section")
	}
}

// VIEWMODEL.md L7, L8, L10, L19, 12.7: the shapes of the instances reachable from public values:
// the fields each instance sets (a default is not set), `main` holding those set in at least
// 25 % of the instances, `more` the others, by set count then declaration order.
func TestUsage(t *testing.T) {
	m := tree(t, "", valued, build.Options{Layers: []string{"night"}}).model(t, demoPkg)
	expect(t, "L7 L10 L19", m.Usage["a.Pt"], `{"shapes":{"":{"count":7,
	 "fields":{"at":0,"hp":2,"ratio":0,"share":0,"x":7},"main":["x","hp"],"more":["share","ratio","at"]}}}`)
}

// VIEWMODEL.md 3.5 `icon`, `tone` (log-2026-09-28 views V3 calls): a studio member written
// qualified (`Icon.gem`, `studio.Tone.danger`) is written as the bare member, like a bare one.
func TestQualifiedIcons(t *testing.T) {
	files := map[string]string{"studio/studio.canon": studioFull, "a/a.canon": `package a

import studio
import studio { Icon, Tone }

/// Rarities.
enum Rarity { common, rare, epic }

view Rarity {
  common "Common" { icon: Icon.gem, tone: Tone.info }
  rare "Rare" { tone: studio.Tone.danger }
  epic "Epic" { icon: gear }
}
`}
	var got [][2]string
	for _, m := range tree(t, "", files, build.Options{}).model(t, demoPkg).Types["a.Rarity"].Members {
		got = append(got, [2]string{m.Icon, m.Tone})
	}
	expect(t, "3.5", got, `[["gem","info"],["","danger"],["gear",""]]`)
}
