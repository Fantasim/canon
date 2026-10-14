# To Resource: `tidCode` costs a third of `canon check` (2026-10-14)

From the Canon compiler session. Optional, no Canon version needed: works on 0.1.7.

## What

Three copies of `tidCode` turn a `TID_*` name into its number by scanning all 9,455 `TextId`
members, once per call:

- `events/vocab/display/display.canon:100` (six `*Tid` getters per row, and `check liveTids`)
- `progression/vocab/recordbook/boards.canon:120`
- `progression/vocab/ranking/names.canon:58` (also used by `events/vocab/display/paramNames.canon`)

```canon
return TextId.members.first(m => m.name == name and not m.retired)?.code ?? 0
```

That is about 20 million predicate calls per `check`: 55% of everything canon allocates.

## Fix

Build the map once. A top-level value is computed on first read and cached for the run
(EVALUATION.md §3.1). In each of the three files:

```canon
/// The TID number of a name, 0 for none or a name text.vocab does not hold.
fn tidCode(name: String?) -> Int {
  if name == none {
    return 0
  }
  return tidCodes.get(name) ?? 0
}

/// Every live text id's number by name, built once.
let tidCodes: {String: Int} = TextId.members.filter(m => not m.retired).toMap(m => m.name, m => m.code)
```

`boards.canon`'s copy has no retired filter today; keep `TextId.members.toMap(...)` there to keep
its meaning, or align it with the other two. Better still: one exported `tidCode` in `text.vocab`
instead of three copies.

## Measured

Copy of Resource `456c1bcd`, canon 0.1.7, `canon check`, three interleaved runs each at equal
load (22-26 on a 24-core host, so absolute times are inflated):

| | Before | After |
|---|---:|---:|
| Wall | 31.0-34.8 s | 20.2-21.7 s (−35%) |
| User CPU | 52-57 s | 30-32 s (−43%) |
| Peak RSS | 2.65 GB | 2.77-2.84 GB (+5%) |

Findings identical: 0 errors, 0 warnings in 89 packages.

## Why the names are strings, and what Canon owes

`Tid` is a `String` because `text.vocab` emits no C++ or baked Go, so a field typed `TextId` would
need an enum the consumers never get. Canon has no way to write such a field as its code without
the enum type (deferred F20, per-field `@json(codes)`). Canon also owes a direct name-to-member
lookup, so the obvious code is the fast code. Both are tracked on Canon's side
(`meta/decisions/log-2026-10-14.md`); the fix above does not wait on them.
