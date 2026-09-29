package live_test

// shop is a package whose records have views with titles, subtitles, `when` conditions on
// fields, methods and groups, `show` lines and view-named methods, an inline and a plain variant,
// dependent fields, and collections of records with `text` columns, in English and French.
var shop = map[string]string{
	"a/a.canon": `package a

enum Goal { kill, collect }

type Target(g: Goal) = match g {
  kill => String
  collect => Int
}

type Reward(item: Bool) = match item {
  false => Int
  true => String
}

type Wrap(m: ref modes) = match m.goal {
  kill => [Target(m.goal)](..=3)
  collect => Target(m.goal)
}

variant Kind {
  weapon { attack: Int, speed: Int, grip: Stats }
  food { heal: Int }
}

variant Effect {
  burn { power: Int }
  freeze { turns: Int }
}

record Ore {
  name: String
}

record Mode {
  goal: Goal
}

record Plan(m: ref modes) {
  aim: Target(m.goal)
}

record Metal {
  label: String
  ore: ref ores
}

record Stats {
  hp: Int
  shown: Bool
}

record Part {
  label: String
  size: Int
  stats: Stats
  goal: Goal
  target: Target(goal)
  link: ref metals | "none"
  kind: Kind @json(inline)
}

record Step {
  text: String
}

record Slot {
  size: Int
}

record Mark {
  size: Int
}

record Note {
  text: String
}

record Item {
  name: String
  shown: Bool
  count: Int
  kind: Kind @json(inline)
  effect: Effect
  stats: Stats
  goal: Goal
  target: Target(goal)
  bonus: Reward(stats.shown)
  parts: [Part]
  rows: [Part] = []
  spares: {String: Part}
  steps: [Step]
  slots: {String: Slot}
  marks: [Mark] = []
  markMap: {String: Mark} = {}
  notes: [Note]
  mode: ref modes
  plan: Plan(mode)
  plans: [Plan(mode)] = []
  wrapped: Wrap(mode)
  hint: Target(goal) where true

  fn doubled(self) -> Int {
    return count * 2
  }

  fn broken(self) -> Int {
    return count
  }
}

view Ore {
  title "Ore {name}"
}

view Metal {
  title "Metal {label} of {ore}"
  subtitle "{id} / {ore.id}"
}

view Stats {
  title "Stats"
  hp { when: shown }
  group vitals "Vitals" when shown { hp }
}

view Part {
  title "{label}"
  subtitle "{link}"
  columns { label, size, stats, target, link, grip }
}

view Step {
  title "Step {index}"
  text { when: index > 1 }
  show "Position" "{index}"
}

view Slot {
  title "Slot {key}"
}

view Mark {
  title "Mark"
}

view Effect.freeze {
  turns { when: turns > 5 }
  show "Turns" "{turns}"
}

view Item {
  title "Item {name}"
  subtitle "{goal}"
  show "Summary" "{name} x{count}"
  doubled "Twice" { when: shown }
  count { when: shown }
  speed { when: count > 1 }
  rows { control: text }
  group main "Main" when shown {
    name
    show power "Power" "{count}"
    doubled
    broken
    show "Extra" "{name}"
    speed
  }
  group rest "Rest" when count > 1 { stats }
  show "Tail" "{count}"
  show ident "Ident" "{id}"
}

let modes: table Mode = {
  m1 { goal: kill }
  m2 { goal: collect }
}

let plans: {m in modes: Plan(m)} = { "m1": { aim: "x" }, "m2": { aim: 3 } }

let ores: table Ore = {
  o1 { name: "Rock" }
  o2 { name: "Sand" }
}

let metals: table Metal = {
  iron { label: "Iron", ore: o1 }
  gold { label: "Gold", ore: o1 }
}

let pool: [Stats] = [{ hp: 1, shown: true }, { hp: 2, shown: false }]

let items: table Item = {
  sword {
    name: "Sword", shown: false, count: 3, kind: weapon { attack: 5, speed: 2, grip: { hp: 9, shown: true } },
    effect: freeze { turns: 2 }, stats: { hp: 10, shown: true }, goal: kill, target: "wolf",
    bonus: "gem",
    parts: [
      { label: "Blade", size: 2, stats: { hp: 1, shown: true }, goal: collect, target: 4, link: iron,
        kind: weapon { attack: 7, speed: 1, grip: { hp: 5, shown: true } } },
      { label: "Blade", size: 3, stats: { hp: 2, shown: true }, goal: kill, target: "orc", link: "none",
        kind: food { heal: 2 } },
      { label: "Hilt", size: 1, stats: { hp: 3, shown: true }, goal: kill, target: "elf", link: gold,
        kind: food { heal: 1 } }
    ],
    rows: [{ label: "Rim", size: 1, stats: { hp: 1, shown: true }, goal: kill, target: "x", link: iron,
      kind: food { heal: 1 } }],
    spares: { "left": { label: "Guard", size: 1, stats: { hp: 4, shown: false }, goal: collect, target: 1,
      link: iron, kind: food { heal: 3 } } },
    steps: [{ text: "cut" }, { text: "grind" }],
    slots: { "belt": { size: 2 } },
    marks: [{ size: 1 }, { size: 2 }],
    markMap: { "m": { size: 3 } },
    notes: [{ text: "sharp" }],
    mode: m2, plan: { aim: 5 }, plans: [{ aim: 8 }], wrapped: 6, hint: "h"
  }
  retired bread {
    name: "Bread", shown: true, count: 1, kind: food { heal: 4 }, effect: burn { power: 1 },
    stats: { hp: 0, shown: false }, goal: collect, target: 7, bonus: 5, parts: [], spares: {},
    steps: [], slots: {}, notes: [], mode: m1, plan: { aim: "y" }, wrapped: ["a"], hint: 3
  }
}
`,
	"a/a.fr.canon": `package a
translation fr

Item.title "Objet {name}"
Item.show.power "Puissance"
Stats.title "Statistiques {hp}"
Ore.title "Minerai"
Mark.title "Marque {key}"
`,
}

// languages is the project's languages: French has a translation file, German none.
const languages = "  languages: [en, fr, de]\n"
