# Types

## Scalars

| Type | Values | Notes |
|---|---|---|
| `Bool` | `true` `false` | |
| `Int` | 64-bit signed | overflow is `E4101`, never a wrap |
| `Int8` `Int16` `Int32` `UInt8` `UInt16` `UInt32` `UInt64` | sized | `Int` in expressions; range checked when stored (`E3201`) |
| `Float` `Float32` | IEEE 754 | NaN and infinities invalid (`E3202`) |
| `String` | UTF-8 | length refinements count bytes |
| `Duration` | milliseconds | `250ms`, `1h30m`; no `Int` conversion: `n * 1s`, `Int(d / 1ms)` |

No implicit `Int`/`Float` conversion: an integer literal is accepted as a `Float`, an integer
expression is not (`E3311`); write `Float(i)`, `Int(f)` (truncates), `round(f)`.

## Refinements

```canon fragment
Int(0..=100)                  // inclusive; 0..100 is half-open; Int(1..) lower bound only
Float(0.0..=1.0)
Duration(1s..=10m)
String(1..=64)                // byte length
[Item](1..=64)                // element count of a list or map
String(/^II_[A-Z0-9_]+$/)     // RE2, matches anywhere: anchor with ^ and $
Int(0..) where it % 2 == 0    // `where` takes any Bool expression over `it`
```

One range or regex per type (`Int(0..)(..=5)` is `E1103`; add `where`). Refinements are checked
when a value is stored: `let`, field, element, argument, return, loaded or amended value.
Failures: `E3204` range or length, `E3205` regex, `E3206` `where`. A literal that breaks a range
or regex is a static error, so a test cannot `expect` it to fail.

## Declarations

```canon types/types.canon
package types

/// A weekday; members are data, named freely.
enum Day { Mon, Tue, Wed }

/// `ordered` enables < <= > >= by declaration order; `= "..."` sets a member's wire value.
enum Grade ordered { low, mid, high = "top" }

/// Numeric codes, stable forever: retire members, never remove or renumber them.
enum Element @codes(UInt8) { FIRE = 1, WATER = 2, retired WIND = 3 }

/// Gold, never negative.
type Price = Int(0..)

/// A time of day.
record TimeOfDay {
  /// 0 to 23.
  hour: Int(0..=23)
  /// 0 to 59.
  minute: Int(0..=59) = 0

  fn minutes(self) -> Int { return hour * 60 + minute }
}

/// What a quest gives: exactly one case, each with its own fields.
variant Reward @json(tag: "type") {
  gold {
    /// Coins given.
    amount: Int(1..)
  }
  item {
    /// Item given.
    name: String(1..)
    /// How many.
    count: Int(1..) = 1
  }
  nothing
}

/// A quest players take.
record Quest {
  /// Opening time.
  opens: TimeOfDay
  /// Days it runs.
  days: [Day] = []
  /// What it gives.
  reward: Reward = nothing
  /// Entry cost; none is free.
  fee: Price? = none
  /// Quest unlocked next.
  next: ref quests? = none
}

/// The quests, keyed by identifier.
let quests: table Quest = {
  intro { opens: { hour: 8 }, reward: gold { amount: 10 }, next: hunt }
  hunt { opens: TimeOfDay { hour: 20, minute: 30 }, days: [Mon, Wed], reward: item { name: "Bow" } }
}

/// A map with enum keys, written bare.
let weights: {Element: Int} = { FIRE: 2, WATER: 1 }
```

- Records: a field is required unless it has a default or is optional (`E3302` when missing).
  Records are closed (`E3301` for an unknown field) and nominal. Inside the body, fields are in
  scope by name and `self` is the value. A default may use constants, earlier fields and the stdlib.
  A `table` element may not declare `id` or `retired` (`E2105`): entries have `.id`, `.retired`.
- Enum members: `Day.Mon`, or bare (`Mon`) where the type is expected; `.name`, `.index`,
  `.wire`, and `.code` with `@codes`. A retired member stays for `match`; using it is `E3506`.
- Variant literal: `gold { amount: 10 }`, `Reward.gold { ... }`; a case whose fields all have
  defaults may be bare (`item` needs `name`: `E3302`). `v.kind` is the case; `v is gold` tests
  it. Case fields are readable only on a narrowed value: `match`, `if v is gold { v.amount }`
  (else `E3003`). Wire form: `{"type": "gold", "amount": 10}` (tag default `kind`).
- Contextual names: a bare identifier is first resolved against the expected type (enum member,
  variant case, key of the expected `ref`'s collection), then against scope.

## Optionals

`T?` is `T` or `none`. Strict: a `T?` is never accepted where `T` is (`E3403`), and `.f`, calls,
`[ ]`, iteration and arithmetic on a `T?` are `E3402`. Prove presence:

| Form | Meaning |
|---|---|
| `if x != none { x.f }` | narrowing: also after `and`, in `while`, after `if x == none { return ... }` |
| `x ?? fallback` | `x` when present, else `fallback` |
| `x?.f`, `x?.m()` | `none` if `x` is `none`, and so is the rest of the chain |
| `x!` | `x` as `T`; `E4001` at evaluation if `none` |

Narrowing works on stable paths (locals, params, fields, `self.f.g`), never on indexes or calls:
`let t = tiers.get(k)` first. `!`, `?.`, `??` on a value that is never `none` is `W3401`.

## Collections

| Type | Access |
|---|---|
| `[T]` | `xs[i]` position from 0, negative from the end; `xs.get(i)` is `T?` |
| `{K: V}` | `m[k]`, `m.get(k)`; `K` is `String`, an integer, an enum or a `ref`; `m.k` is `E3003` |
| `table T` | keys are identifiers: `t.axe`, `t[axe]`, `t.get(axe)` (`T?`), `t.at(0)` by position |
| `stable table T` | a table whose keys are permanent (`data` topic) |
| `[T] keyed by f` | field `f` is unique (`E3102`): `xs[k]`, `xs.k`, `xs.get(k)`, `xs.at(i)` |

A missing key or index is `E4002`. Duplicate keys: `E3101` table, `E3102` keyed list.

## References

`ref T` holds the key of an entry. `ref items` names a collection; `ref Item` finds the one
collection of `Item` (enclosing record's collection fields, then the package, then imports;
none or several is `E2103`). A ref behaves like its entry (`q.next.opens`, `r.id` is the key),
an entry converts to a ref when it belongs to the target (`E3503` otherwise). Unknown key:
`E2102` when the collection's keys are written in source, `E3501` when loaded or computed.
A ref from a live table entry to a retired one is `E3502`. Wire form: the key.

## Special types

| Type | Meaning |
|---|---|
| `A \| "lit"` | an `A` or exactly the string `"lit"`; `A` has a string wire form |
| `Never` | no values; `Never?` accepts only `none` |
| `Range` | value of `a..b`: `.start`, `.end`, `.len()`, `.contains(x)`; never in emitted data (`E8151`) |
| `asset("@root/dir", ext: [png, dds])` | a file name under that root; the file must exist, exact case (`E3701`) |

## Dependent types

```canon types/dependent.canon
package types

/// What an objective counts.
enum Goal { kill, collect, visit }

/// What an objective aims at, chosen by its goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
  visit => Never
}

/// A daily objective.
record Objective {
  /// What it counts.
  goal: Goal
  /// Typed by the earlier field `goal`; none when the goal aims at nothing.
  target: Target(goal)? = none
}
```

- A field type may use earlier fields; `match` over an enum or `Bool` must be exhaustive.
- `{e in coll: T(e)}` is a map keyed by `ref coll` whose value type depends on the key.
- A value that does not fit its computed type is `E3802`; a required field computed as `Never`
  is `E3801`. In expressions a dependent value supports `==`, `!= none`, interpolation, `match`.
