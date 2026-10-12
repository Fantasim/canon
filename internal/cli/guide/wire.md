# Wire forms

A legacy file has shapes a model should not have. `@json` keeps them on the wire: the field has
a clean Canon name and type, the file has its own spelling (DECISIONS 3, 21). Annotations live on
the field; they never change what the value means, only how a file writes and reads it. One
project is in `canon guide port`.

| Form | On | Wire |
|---|---|---|
| `@json("szName")` | field, variant case, `@codes` member | the key (or name) in the file |
| `@json(case: snake)` | record, variant | default key of each field: `snake` `camel` `kebab` `upper_snake` |
| `@json(path: "req.szSkill")` | field | the value sits below `req` |
| `@json(none: -1)` | optional field | what `none` is written as (default `null`) |
| `@json(int)` | field with `Bool` | `0` / `1` |
| `@json(unit: s)` | field with `Duration` | a number in `ms` `s` `m` `h` `d` |
| `@json(bits)` | `[E]`, `E` a `@codes` enum | one integer OR-ing the member codes |
| `@json(codes)` | `@codes` enum | the code instead of the name |
| `@json(pairs: [k, v])` | `[R]`, `R` two fields | numbered parallel keys |
| `@json(inline)` | field with a variant | tag and case fields in the parent object |

Forms combine in one annotation: `@json("bOnce", int)`, but two of `inline` `int` `bits` `unit:`
do not. A form at a wrong position is `E1104`; a form on the wrong type is `E3316`, at the
annotation. Two fields with one wire name are `E3316` too.

## Names, numbers and nesting

```canon wire/names/names.canon
/// Names and scalar forms of a legacy rewards file.
package wire.names

/// Ranks; the file stores the number.
enum Rank @codes(UInt8) @json(codes) {
  recruit = 1
  captain = 2
}

/// A reward as the file spells it.
record Reward {
  /// Gold paid.
  gold: Int(0..) @json("dwGold")
  /// Paid once. 0 or 1 on the wire.
  once: Bool = false @json("bOnce", int)
  /// Time before it can be taken again. Whole minutes on the wire.
  cooldown: Duration = 0s @json("nCooldownMin", unit: m)
  /// Level needed. -1 on the wire means none.
  minLevel: Int(1..)? @json("nMinLevel", none: -1)
  /// Rank needed. The number on the wire.
  rank: Rank = recruit @json("nRank")
  /// Skill needed, one object down.
  skill: String? @json(path: "req.szSkill")
}

/// The file as it is.
let rewards: [Reward] = load("data/rewards.json")
```

```json wire/names/data/rewards.json
[
  { "dwGold": 50, "bOnce": 1, "nCooldownMin": 90, "nMinLevel": -1, "nRank": 2, "req": { "szSkill": "mining" } },
  { "dwGold": 10, "nCooldownMin": 5, "nMinLevel": 3 }
]
```

- An absent key is the field's default, or `none` for an optional field; a required field with
  no default is `E3302`. `null` is `none` for an optional field and `E3315` on any other; the
  `none:` marker is `none` (WIRE §5.4).
- `Duration` reads any number whose value is a whole number of milliseconds (`1.5` minutes is
  fine, `0.0005` seconds is `E3203`), and writes an integer count of its unit: `1500ms` under
  `unit: s` has no wire form (`E8102`). The default unit is `ms`.
- `@json(int)` reads only `0` and `1` (`E7110` for `true` or `2`).
- `@json(codes)` reads an integer that is a member's code (`E7111` otherwise). Use it when the
  file stores the number; without it the file holds the member name.
- Renaming a Canon field never touches a file: the wire name stays in `@json`.
- Mistake: copying the legacy name into the model (`dwGold: Int`). The model, the generated
  code and the studio all show it. Name it for what it is and keep `@json("dwGold")`.

## Bitmasks: `@json(bits)`

What: one integer in the file that ORs flags; the model has a list of enum members.
When: a field the legacy code tests with `flags & MASK`. Not for a value with more than one
bit per option: give each such option its own enum.

```canon wire/bits/bits.canon
/// A bitmask field.
package wire.bits

/// Item flags. Every code is a power of two.
enum Flag @codes(UInt32) {
  binds = 1
  undestroyable = 2
  equipBind = 4
}

/// An item with flags.
record Item {
  /// Property flags. One integer on the wire.
  flags: [Flag] where it.isUnique() = [] @json("dwFlag", bits)
}

/// The file as it is.
let items: [Item] = load("data/items.json")
```

```json wire/bits/data/items.json
[
  { "dwFlag": 5 },
  { "dwFlag": 0 }
]
```

- `5` reads as `[binds, equipBind]`: members in ascending code order. `[]` is `0`. The order
  of the list is not kept by the file, so declare it a set (`where it.isUnique()`): a list with
  a member twice has no encoding (`E8102`).
- Every code of the enum, retired ones included, must be a power of two between 1 and 2^62
  (`E3316`). A set bit that is not a member is `E7111` and names the bits (`0x100`); a negative
  number is `E7110`; `2.0` is `E7103`.
- Generated code has `rt.List[Flag]` (Go) and `std::vector<Flag>` (C++). Go has `Flag.Code()`;
  in C++ the enum's value is the code; both have `FlagFromCode`: use them to build or test the
  mask where the legacy code wants it. The studio shows checkboxes.
- Mistake: an integer field with the flag values in its docs. Declare the enum, so every code is
  named, retired when dropped, and kept by `canon.lock`.

## Parallel keys: `@json(pairs:)`

What: numbered keys in a legacy object (`dwDestParam0`, `nAdjParamVal0`, `dwDestParam1`, ...)
become one list of two-field records.
When: a short, fixed number of slots. Never write the numbered fields in the model.

```canon wire/pairs/pairs.canon
/// Parallel keys.
package wire.pairs

/// A stat and by how much it is raised.
record Bonus {
  /// The stat.
  stat: String(1..)
  /// The amount.
  amount: Int
}

/// An item with up to three bonuses.
record Item {
  /// Bonuses in slot order. Written dwStat0 / nAmount0, dwStat1 / nAmount1, ...
  bonuses: [Bonus](..=3) = [] @json(pairs: ["dwStat{i}", "nAmount{i}"])
}

/// The file as it is.
let items: [Item] = load("data/items.json")
```

```json wire/pairs/data/items.json
[
  { "dwStat0": "STR", "nAmount0": 5, "dwStat1": "DEX", "nAmount1": 2 },
  {}
]
```

- The number of slots is the upper length bound: `(..=3)` and `(1..=3)` give 3, `(..3)` gives 2.
  No finite bound is `E3316`. A non-empty default is `E3316` (absent slots decode as the empty
  list).
- The first field of the record goes under the first template, the second under the second;
  their own wire names are not used (their `unit:` and `int` forms are). Both templates hold
  `{i}` once; `{i}` counts from 0.
- A slot is both keys or neither. One key alone, a `null`, or a filled slot after an empty one is
  `E7117`: fix the legacy file once, Canon does not tolerate the gap. Keys of a slot past the
  bound are unknown keys (`E3301`).
- The data wire omits the keys of unused slots (an empty list writes none). Generated code,
  the studio and checks see a plain `[Bonus]`.
- Mistake: two fields `stat0`, `amount0`... in the model, or a `[Bonus]` with no upper bound.

## Inline variants: `@json(inline)`

What: a variant whose tag and case fields sit in the parent object, as legacy files flatten
a type tag and the fields that depend on it.
When: the file has a kind key beside the common keys. A variant that is a value of its own
(a reward, an event) keeps its object and needs no `inline`.

```canon wire/inline/inline.canon
/// An inline variant.
package wire.inline

/// What a mark shows. Fields depend on the type.
variant Shape @json(tag: "nShape") {
  /// A circle.
  circle {
    /// Radius.
    radius: Int(1..) @json("nRadius")
  }
  /// A rectangle.
  rect {
    /// Width.
    width: Int(1..) @json("nWidth")
    /// Height.
    height: Int(1..) @json("nHeight")
  }
}

/// A mark with its shape flat beside its label.
record Mark {
  /// Shown text.
  label: String @json("szLabel")
  /// The shape. No key of its own.
  shape: Shape @json(inline)
}

/// The file as it is.
let marks: [Mark] = load("data/marks.json")
```

```json wire/inline/data/marks.json
[
  { "szLabel": "start", "nShape": "circle", "nRadius": 3 },
  { "szLabel": "wall", "nShape": "rect", "nWidth": 4, "nHeight": 1 }
]
```

- The tag key comes from `@json(tag:)` (default `kind`). Encoding writes it where the field is,
  then the case's fields. A key of another case is `E3301` and names that case; a missing tag is
  `E7112`.
- The field must be a non-optional variant (`E3316`), and there is at most one per record. A tag
  key, a case field or a first path segment equal to a parent key is `E3318`, and so are two
  inline fields.
- Mistake: one record with every field of every kind, optional. Inline keeps the file flat and
  the model exact: a rectangle cannot carry a radius.

## Checking the wire

- `canon check` decodes every `load`ed file at its type: each finding names the file, line and
  key. A wrong form is `E3316` at the annotation.
- `canon build` writes Canon's data wire for `emit json` (`$schema`, `rows`, every field) and the
  legacy shape for a `@text` function (`canon guide port`). Both use the wire names.
- A `@json` form is part of the type's fingerprint: changing one changes the `$schema` of
  emitted data, and `data` runtimes refuse older files until rebuilt (`canon guide recipes`).
