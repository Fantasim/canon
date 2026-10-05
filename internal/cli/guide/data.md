# Data

## Constants and values

- `const NAME = expr`: literals, constants, operators, stdlib; no `load`, no user function;
  inferred type; usable in types (`Int(..=NAME)`); never amended by a layer.
- `let name: T = expr`: a public `let` needs its type (`E3001`); a `local let` may infer it,
  except from `load`. Every top-level `let` is evaluated and checked, emitted or not.

## Literals of data

```canon shop/shop.canon
/// Items, ranks and limits of the shop.
package shop

/// What an item is.
enum Kind { weapon, potion }

/// An item for sale.
record Item {
  /// Shown name.
  name: String(1..)
  /// What it is.
  kind: Kind
  /// Price in gold.
  price: Int(0..) = 0 @json("nPrice")
  /// Sold in bundles of this size.
  bundle: Int(1..) = 1
}

/// Every item. New entry files go to items/<kind>/<key>.canon.
@files("items/{kind}/{id}.canon")
let items: stable table Item = {
  axe { name: "Axe", kind: weapon, price: 120 }
  // Replaced by the bow; its id stays reserved.
  retired sling { name: "Sling", kind: weapon }
}

/// A shared base and a variant of it: spread first, then overrides.
let promo: Item = { ...items.axe, name: "Promo axe", price: 99 }

/// A rank, keyed by its code field.
record Rank {
  /// The key.
  code: String(1..)
  /// Shown title.
  title: String(1..)
}

let ranks: [Rank] keyed by code = [
  { code: "private", title: "Private" },
  { code: "captain", title: "Captain" },
]

let starter: {ref ranks: [ref items]} = { private: [axe], captain: [axe, bow] }
```

```canon shop/items/weapon/bow.canon
package shop

/// One entry per file is fine; the body is the entry's fields.
entry items.bow {
  name: "Bow"
  kind: weapon
  price: 150
}
```

- Table literal: `key { fields }` per entry; keys are identifiers (`retired key { ... }`).
- `entry t.key { ... }` adds an entry to table or keyed list `t` of the same package from any
  file; `t`'s literal must be a table or list literal (`{}` / `[]` allowed). For a keyed list the
  key is the entry name and the body omits the key field (`E3321`). Order: the literal's
  entries, then entry files by path, then source order. `@files("dir/{field}/{id}.canon")` on
  the `let` places new entry files (`canon edit` `addEntry`); default `<let>/<key>.canon`.
- Spread `{ ...base, f: v }`: at most one, first, same type (`E3303`); checks run again. Map
  keys: enum members and refs bare, `String` keys quoted (`"a": 1`, `E3304`).

## load

```canon shop/sources.canon
package shop

/// Server limits.
record Limits {
  /// Players at once.
  players: Int(1..) @json("nPlayers")
  /// Queue length.
  queue: Int = 10 @json("nQueue")
  /// Session length, written in seconds.
  timeout: Duration @json("timeoutSec", unit: s)
  /// Message of the day; "" in the file means none.
  motd: String? = none @json("szMotd", none: "")
}

/// The whole file, read as the expected type.
let limits: Limits = load("data/limits.json")

/// Only part of a file: `at: "a.b[0]"` selects a member.
let seats: Int = load("data/limits.json", at: "nPlayers")

/// One entry per file, in path order; keys come from the key field.
let extraRanks: [Rank] keyed by code = load.dir("data/ranks/*.json")

/// A level curve from a spreadsheet; the header row names wire names.
record LevelRow {
  /// Character level.
  level: Int(1..)
  /// Experience to the next level.
  exp: Int(1..) @json("exp_to_next")
}

let levels: [LevelRow] keyed by level = load.csv("data/levels.csv", header: true)
```

```json shop/data/limits.json
{
  "nPlayers": 100,
  "timeoutSec": 1800,
  "szMotd": ""
}
```

```json shop/data/ranks/sergeant.json
{
  "code": "sergeant",
  "title": "Sergeant"
}
```

```csv shop/data/levels.csv
level,exp_to_next
1,100
2,250
```

| Form | Gives |
|---|---|
| `load(path)` | a file read as the expected type; `.json`, `.csv`, `.txt` by extension, or `format: json` |
| `load(path, at: "a.b", partial: true)` | `at:` reads one member (`[n]` index, `*` every member); `partial:` ignores keys the type lacks |
| `load.dir(glob)` | one element per file, path order: `[T]`, `[T] keyed by f`, or `table T` (key = file stem) |
| `load.csv(path, header: true)` | rows; cells parsed as Canon literals of the field type; empty cell = default |
| `load.defines(path, prefix: "JOB_")` | a C header's integer `#define`s as a table keyed by name, entries have `.value: Int`; `ref jobs` checks a name exists |
| `load.text(path)` | `String` |

- The expected type must come from the context (annotation, field, argument): `E7002` otherwise.
- JSON is strict: an absent key is the default, `null` is `none` (`E3315` on a required field),
  `2.0` in an `Int` is `E7103`, an unknown key is `E3301` unless `partial: true`.
- Paths: `@root/...` or relative to the file; never outside the project or a root (`E7001`).

## Wire names: `@json`

| Annotation | Effect |
|---|---|
| `@json("nHeal")` | the field's (or case's) name in data files |
| `@json(case: snake)` on a record/variant | default wire names converted: `snake`, `camel`, `kebab`, `upper_snake` |
| `@json(path: "legacy.reqMp")` | value nested deeper in the wire object |
| `@json(tag: "type")` on a variant | tag key (default `kind`) |
| `@json(inline)` on a variant field | tag and case fields sit in the parent object |
| `@json(none: -1)` / `(none: "")` | the wire spelling of `none` (default `null`) |
| `@json(unit: s)` on a `Duration` | number in `ms` `s` `m` `h` `d` (default integer ms) |
| `@json(codes)` on a `@codes` enum | the wire holds the code number |
| `@json(int)` on a `Bool`; `@json(bits)` on `[E]` | legacy: wire `0`/`1`; one integer OR-ing member codes |

Renaming a Canon field never changes data: the wire name stays in `@json` (`canon rename` adds
it when needed).

## Stable ids and canon.lock

- `stable table T`, `@codes(UInt8)` enums and `@stable` fields (on a stable table's element)
  are permanent: never removed (`E6001`), renamed (`E6001`), reused or un-retired (`E6002`).
- Retire instead: `retired key { ... }` in a table, `retired NAME = 3` in a `@codes` enum, or
  `canon edit` op `retire`. A retired entry stays in data files (`"$retired": true`), generated
  id enums and iteration; `.active()` skips it; a ref to it from a stored value is `E3502`
  unless it sits in a retired entry or a `past ref` slot.
- `canon.lock` (next to the package) records every stable value ever seen, one sorted line each:
  `table  shop.items  axe`, then `retired` when retired. `canon build` and `canon edit` append;
  nothing removes. A `--layer` build never writes it, and a layer cannot add stable entries
  (`E6004`). Commit it with the sources.
