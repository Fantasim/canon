# Syntax

## Project and packages

- `project.canon` at the root holds only the `project` declaration. `canon init` writes one.
- Package `shop.items` lives in `shop/items/`; `canon new shop.items` creates
  `shop/items/items.canon`. A file declares its directory's package or an ancestor's (`E2006`),
  so `shop/items/axe/axe.canon` may say `package shop.items`. All files of a package share one
  namespace; declaration order does not matter.
- A file starting `package p` then `layer name` is a layer (`canon guide layers`); then
  `translation fr`, a translation (`canon guide views-i18n`).

```canon project.canon
/// The configuration of the demo project.
project demo {
  canon: "0.1"

  /// Named roots: paths written "@data/...". Relative to this file; may leave the project.
  roots {
    data: "../shared-data"
    gen: "gen"
  }
  /// Roots a machine may lack; any other root outside the project must exist (E1013).
  optional_roots: [data]

  languages: [en, fr]
  budget: 100_000_000
  go_module {
    gen: "example.com/demo/gen"
  }
}
```

`canon` is required. `languages`: the first is the source language (default `[en]`).
`studio: <package>` names the studio package (`views-i18n`). `go_module` maps a root to the Go
module path of the Go outputs under it. No expressions; unknown key `E1002`. Reading an absent
optional root is an error at the value; an output into it is skipped. `project.local.canon`
beside it (git-ignored, same grammar, only `roots`) places roots on one machine; `--root` wins.

## A source file

```canon shop/shop.canon
/// Package doc: the line(s) directly before `package`.
package shop

import shop.vocab { Kind }
import shop.vocab as vocab

/// The highest item level.
const MAX_LEVEL = 150

/// An item. Doc comments attach to the next declaration, field, member, case or entry.
record Item {
  /// Shown name.
  name: String(1..=64)
  /// Required level.
  level: Int(1..=MAX_LEVEL) = 1
  /// What it is.
  kind: Kind
  /// Price, written `nPrice` in data files.
  price: Int(0..) = 0 @json("nPrice")
}

// A plain comment. /* Block comments */ do not nest.

/// The items for sale.
let items: table Item = {
  axe { name: "Axe", level: 10, kind: vocab.Kind.weapon, price: 1_200 }
  tonic { name: "Tonic", kind: potion }
}
```

```canon shop/vocab/vocab.canon
/// Shared words.
package shop.vocab

/// What an item is.
enum Kind { weapon, potion, material }
```

- `import a.b` (use `b.Name`), `import a.b as x` (use `x.Name`), `import a.b { Name, Other }`
  (bare names). Imports are acyclic (`E2002`).
- `local` before `const`, `let`, `type`, `fn`, `record`, `enum`, `variant`: private to the
  package, never imported, never emitted.
- `///` doc comments become generated comments and studio help. A public type, field or
  `export fn` without one is `W1002`; a doc comment followed by a blank line is `W1001`.

## Names

Conventions (`W1003`): `UpperCamel` types; `lowerCamel` fields, functions, methods, lets,
parameters, locals, package segments; `UPPER_SNAKE` constants; `lower_snake` widgets. Enum
members, variant cases, table keys, check and layer names are data: free.

Reserved words cannot name declarations, fields, parameters or locals (`E1125`):
`and amend as asset break check const continue else emit entry enum export expect false fn for
if import in input is layer let load local match none not or package project record ref retired
return self stable table test translation true type var variant view warn where while widget`.
Data may use them: an enum member `check`, a table key `package`, a field name before `:` in a
literal. Bare in a value only the words that cannot start an expression (`icon: check`); others
are written qualified (`Icon.return`). `none true false self` never name a member, case or key
(`E1126`). A field whose wire name is reserved keeps a clean name: `kind: Kind @json("type")`.

## Separators

- Inside `{ }`: items end at a newline or a comma (mixable). Inside `( )` and `[ ]`: commas,
  newlines ignored. Trailing separators allowed. Two items on one line without one: `E1117`.
- A line continues after an operator, `=`, `:`, `,`, `.`, `=>`, `->`, `??`, `and`, `or`, `else`,
  and before a line starting with `.`, `?.`, `??`, a binary operator except `-`, `and`, `or`,
  `else`.
- In `if`, `while`, `for ... in`, `match` headers, a `{` always opens the body: parenthesize a
  typed literal there, `if (Point { x: 1, y: 2 }) == p { ... }`.

## Literals

| Kind | Examples | Notes |
|---|---|---|
| integer | `42` `1_000_000` `0xFF` `0b1010` `-3` | `_` only between digits |
| float | `0.15` `1e-3` `2.5e6` | needs `.digit` or an exponent; an integer literal is accepted as a Float |
| duration | `250ms` `90s` `1h30m` `2d` | units `d h m s ms`, largest first, whole numbers |
| string | `"Heal {heal} HP"` | escapes `\n \t \r \\ \" \{ \} \u{1F600}`; `{{` `}}` literal braces |
| raw string | `r"C:\path\{x}"` | no escapes, no interpolation, one line |
| multiline | `"""` ... `"""` | closing line's indent stripped from every line; `r"""` raw |
| regex | `/^II_[A-Z0-9_]+$/` | RE2, search semantics; only in a refinement or `.matches()` |
| other | `true` `false` `none` | |

Interpolation `{expr}` writes the canonical text: decimal ints, shortest floats, `1h30m`,
members by name, refs by key, `[a, b]`, `{k: v}`, `Type{f: v}`. Format specs on numbers only:
`{n:,}` groups thousands, `{x:.2}` fixed decimals, `{d:+}` always signed.

## Canonical layout

`canon fmt` has no options; every file is a fixed point of it. Indent 2 spaces, one space after
`:` and around `=`, never column alignment, lines up to 100 columns where breakable, a broken
brace list holds one item per line without commas, imports sorted, durations rewritten
(`90m` becomes `1h30m`). Annotations: on a field after its type and default; on a `record`,
`enum` or `variant` after its name; on other declarations on the lines before (`@reload`,
`@files(...)`). Run `canon fmt` after every hand edit; `canon edit` and `canon rename` write
canonical text themselves and refuse a file that is not (`ErrNotCanonical`).
