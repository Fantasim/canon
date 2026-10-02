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
