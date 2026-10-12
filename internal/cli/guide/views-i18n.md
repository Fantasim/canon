# Views and translations

Views say how the studio (the generic editor) presents data. They never change validation,
data or generated runtime code. `emit view { out: "..." }` writes the package's view model.

## The studio package

`project.canon` names it with `studio: studio`. View props `unit`, `icon`, `tone`, `menu` and
`widget` resolve against it without an import (unknown: `E1610`).

```canon project.canon
project demo {
  canon: "0.1"
  languages: [en, fr]
  studio: studio
}
```

```canon studio/studio.canon
package studio

/// Navigation sections.
enum Menu { items, system }

/// UI icons.
enum Icon { gem, sword, gear }

/// UI tones.
enum Tone { neutral, info, warning, danger }

/// How a number is shown.
record UnitSpec {
  /// Shown after the number.
  suffix: String = ""
  /// Shown = stored x scale.
  scale: Float = 1.0
  /// Group thousands.
  thousands: Bool = true
  /// Fraction digits.
  decimals: Int(0..=6) = 0
}

let units: table UnitSpec = {
  gold { suffix: " gold" }
  pct { suffix: " %" }
}

/// A custom editor the studio implements under this name.
widget level_bar(value: Int)
```

## A view

```canon shop/shop.canon
package shop

/// What an item is.
enum Kind { weapon, potion }

/// An item for sale.
record Item {
  /// Shown name.
  name: String(1..)
  /// What it is.
  kind: Kind
  /// Required level.
  level: Int(1..=99) = 1
  /// Price in gold.
  price: Int(0..) = 0
}

let items: table Item = {
  axe { name: "Axe", kind: weapon, level: 10, price: 300 }
}

view Item {
  title "{name}"
  subtitle "{id}"
  menu items icon gem
  singular "item"
  plural "items"
  search { name, id }
  filters { kind multi, level }
  columns { name 200, kind 100, price 90 }

  group main "Main" { name, kind }
  group trade "Trade" "What it costs." advanced {
    price { unit: gold }
    level { widget: level_bar }
  }
  show tier "Tier" "{if level > 50 { "high" } else { "low" }}"
}

view Kind {
  weapon "Weapon" { icon: sword, tone: danger }
  potion "Potion" { icon: gem, tone: info }
}

emit view { out: "out/shop.view.json" }
```

| Item | Meaning |
|---|---|
| `title`, `subtitle "<template>"` | how an entry is named in lists and pickers; `{id}` is the table key |
| `menu <Menu> icon <Icon>` | navigation of the values of this type (`@menu(...)` on a `let` overrides) |
| `singular`, `plural "<text>"` | "Add item", "3 items" |
| `search { exprs }`, `filters { field multi? }`, `columns { field width? }` | pickers, table filters, table columns (fields only) |
| `preview <asset expr>` | thumbnail in lists |
| `group id "Label" "intro"? advanced? when cond? { members }` | ordered sections; others go to a final "Other" |
| `show id? "Label" "<template>"` | read-only computed line |
| `<field> "Label"? { props }` | per-field presentation; `field title { }` for a field named like a view word |

Field props: `help`, `unit`, `control`, `widget`, `readonly`, `hidden`, `placeholder`, `when`,
`none` (label of `none`), `step` (`"Level {index}"`). Controls: `switch` `checkbox` (Bool),
`segmented` `radio` `select` `search` (enum, ref, variant), `checkboxes` `chips` (lists of enums
or refs), `input` `textarea` `code` (String), `number` `stepper` `slider` (numbers; `slider` needs
both bounds), `color`, `text` (read-only). Rules: one view per target (`E1607`), in the target's
package (`E1603`); unknown field `E1602`; a name placed twice `E1605`; `view Variant.case { }`
lays out one case; `check ... at field` pins a finding to a field.

## Controls and their JSON

Every field of a view has a resolved `control` in the view model, at
`views.<qualified type>.fields.<field>.control` (VIEWMODEL.md §12.4, §12.5). The compiler picks it
from the type (§4.3) unless `control:` or `widget:` says otherwise; the studio renders what is
written and never decides again. Members are present only where they apply: an absent member is
absent, never `null`. `kind` is one of the closed list of VIEWMODEL §12.5, and
`spec/viewmodel.schema.json` validates the whole file. Four controls carry the collections of
plain values and maps:

```canon shop/loot.canon
package shop

/// Damage elements.
enum Element { fire, ice, poison }

/// An effect on an item.
record Perk {
  /// Shown name.
  name: String(1..)
  /// Strength.
  power: Int(0..=9) = 1
}

/// What a chest holds.
record Loot {
  /// Words people search for.
  tags: [String] = []
  /// Up to four ranks, filled from the first; the first is required.
  ranks: [Int(0..)](1..=4) = [1]
  /// Resistance per element; an element without a value has none.
  resist: {Element: Int(0..=100)} = {}
  /// Effects by name.
  perks: {String: Perk} = {}
}

view Perk {
  title "{name}"
  singular "perk"
}

/// All loot.
@menu(items, icon: gem)
let loot: table Loot = {
  chest {
    tags: ["gold"]
    ranks: [2, 3]
    resist: { fire: 10 }
    perks: { "sharp": { name: "Sharp" } }
  }
}
```

| Control | Chosen for (VIEWMODEL §4.3) | Meaning (§7.5, §7.6) |
|---|---|---|
| `tags` (C25) | `[S]`, `S` a scalar, with no other rule | the elements as tags: each edited with `element`, reordered, removed; "Add" appends |
| `positional` (C24) | `[S]` whose length has an upper bound `n` of at most 6 | `count` inputs in a row, labelled `1`..`count` (or by the field's `step`); filled from the first, the first `minCount` required |
| `enumRow` (C28) | `{E: S}`, `E` an enum, `S` a scalar | one input per active member of `source`, in member order; an empty input means the key is absent |
| `cards` (C29) | `{K: R}`, `R` a record or variant | one collapsible card per entry, in map order, titled by the value view's `title` with `{key}` |

`tags`: `element` is the control of one element, `orderable` is `true`.

```json view/shop.Loot.tags
{"kind": "tags", "element": {"kind": "input"}, "orderable": true}
```

`positional`: `element` as above; `count` is the length bound (1 to 6); `minCount` is the lower
length bound and is absent when it is 0.

```json view/shop.Loot.ranks
{"kind": "positional", "element": {"kind": "number", "min": 0}, "count": 4, "minCount": 1}
```

`enumRow`: `source` is `{"enum": "<qualified enum>"}` (the keys); `value` is the control of every
input. It has no `keyControl`: the keys are the members.

```json view/shop.Loot.resist
{"kind": "enumRow", "source": {"enum": "shop.Element"}, "value": {"kind": "number", "min": 0, "max": 100}}
```

`cards` on a map `{K: R}` that is not dependent: `value` is the control of an entry's value
(`section`, `card` or `variant`); `of` is the qualified name of a record or variant value (it is
also present for a `match` type function, VIEWMODEL §12.5); `keyControl` is the control of the
key, asked first by "Add" (`input` for a `String` key; `segmented`, `select` or `search` over
`source` for an enum key, by the thresholds of VIEWMODEL §4.2); `singular` is the text reference
of the value view's `singular`, when it has one. Keys are read-only once written: they change
through a rename.

```json view/shop.Loot.perks
{"kind": "cards", "of": "shop.Perk", "singular": "shop:Perk.singular", "keyControl": {"kind": "input"}, "value": {"kind": "section", "of": "shop.Perk"}}
```

- A dependent map `{k in c: T(k)}` is `cards` too, with `keyFixed: true`, a `source` naming the
  collection of its keys, and `value` the control of `T(k)` (`dependent` for a type function) (VIEWMODEL §6.5, D10,
  §12.5): "Add" offers the active keys of `c` not yet present, and a key cannot be renamed (API.md E13).
- Every other member of a control is in VIEWMODEL §12.5. Read the member, not the type: a studio
  that switches on `kind` and reads `element`, `count`, `minCount`, `source`, `keyControl`,
  `value`, `of` and `orderable` as above is correct for these four.
- A map that matches no rule above (`{K: V}` with a non-enum key and a scalar value) is `map`,
  with `keyControl` and `value`; a list of lists, maps or optionals is `list`.
- A list of enums is `checkboxes` or `chips` and a list of refs is `chips` (C20 to C22), not
  `tags`. A `widget:` replaces any control and keeps the default one as `fallback` (C41).

## Translations

Texts (docs, labels, check messages) are written in the first of `languages`; others go in
translation files, keyed by path:

```canon shop/shop.fr.canon warns=W1701
package shop
translation fr

Item.help "Un objet en vente."
Item.name "Nom"
Item.price "Prix"
Item.group.trade "Commerce"
Item.group.trade.intro "Ce qu'il coûte."
Item.show.tier "Rang"
Kind.weapon "Arme"
```

| Key | Text |
|---|---|
| `T.help`, `T.f`, `T.f.help` | type doc, field label, field help |
| `T.f.placeholder`, `.none`, `.step`, `.deprecated` | field props, `@deprecated` reason |
| `Enum.member`, `Enum.member.help`, `Variant.case`, `Variant.case.field` | members and cases |
| `T.title`, `T.subtitle`, `T.singular`, `T.plural` | view texts |
| `T.group.<id>`, `T.group.<id>.intro`, `T.show.<id>`, `T.show.<id>.text` | groups and show lines (`_0`, `_1` for unnamed) |
| `T.check.<name>`, `check.<name>` | named check messages, in a type or at package level |
| `value`, `value.help` | a public value's label and doc |

- A name that is a reserved segment (`help title subtitle singular plural group show check intro
  text deprecated placeholder none step field method case member`) takes its kind word:
  `Quest.field.title`, `Icon.member.check`.
- Only texts with a letter outside interpolations are keys. A translated template may
  interpolate what the source can (`E1703`); a plain text may not (`E1707`).
- Unknown key `E1702` (`canon rename` rewrites keys); duplicate `E1705`; a language not in
  `languages` `E1704`. Missing keys: one `W1701` per package and language with `emit view` (it
  names an `i18n status` command this build lacks).
