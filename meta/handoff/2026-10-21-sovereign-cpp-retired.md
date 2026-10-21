# Sovereign -> Canon: retiredness of an enum member in the C++ emit

**From:** Sovereign Source (Louis, ADR L-0138 ruling 8, unit D1). **Canon version:** 0.1.10.

## Why

Sovereign is deleting `Resource/Server/Define/` (the rendered `define*.h`). The game's runtime
define table (name -> number, used by CScript and the JSON loaders) will be built in C++ from the
generated id enums (`k<E>Members`, `ToName`, the code) instead of from those headers.

The rendered headers drop retired members (`fromCodes(..., withRetired: false)`), and the table
must match them: a retired id must not resolve by name. However, a C++ program cannot tell a
retired member from a live one:
- `k<E>Members` holds every member, retired ones included: `kItemIdMembers` has 7,306 members
  for 7,244 live ones, and `kTextIdMembers` has 11,351 for 11,211 live ones.
- `ToName`, `<E>FromCode` and `<E>FromWire` do not distinguish retired members either.
- Retiredness appears only as a `/// Retired.` doc comment on the enumerator.

Canon declined defines-parity ask 5 (`withRetired` on the defines file, reply v0.1.10), and that
is fine: the defines file can keep retired names. This ask is about the **enum API**, not the
defines file.

Go has the same gap for `@codes` enums. A table's id already has `Retired()` / `GetRetired()`,
but a `@codes` member does not.

## Ask

In every C++ mode (`baked`, `data`, `types`), for every `@codes` enum and table id enum, emit one
of the two forms below (either is enough; both would be welcome):

1. `bool IsRetired(E v);` (or `constexpr`), true for a member declared `retired`. It sits next to
   `ToName`/`ToWire`, defined in the `.gen.cpp` as a table read, like the 0.1.10 helpers
   (DECISIONS 383).
2. `inline constexpr std::array<E, N> k<E>LiveMembers`: the members that are not retired, in
   `k<E>Members` order.

The same in Go for `@codes` enums (`func (v E) Retired() bool`), for parity.

## Current workaround (what we use if this cannot ship soon)

`types` mode refuses a lookup `export fn` (`E8014`), and the vocab packages emit Go in `types`
mode. So the only way to get this today is a separate package with a single
`emit cpp { mode: baked }`, holding one function per enum:

```canon
export fn itemIdRetired(k: past ItemId) -> Bool { return k.retired }
```

This produces `ItemIdRetired(::sov::items::ItemId)` through `<E>Ordinal`. It works (probed on
0.1.10), but it means one hand-written function per enum, in a package that exists only for this,
and an `std::abort()` on a value not in the enum. A native emit would remove that package.

## Scope on Sovereign today

Retired members, counted from `canon.lock`:

| Enum | Retired |
|---|---|
| `items.vocab.ItemId` | 62 |
| `text.vocab.TextId` | 140 |
| `ui.AppletId` | 12 |
| `ui.vocab.neuz.MoverMenu` | 2 |
| `ui.vocab.neuz.MapComboId` | 2 |
| `world.vocab.WorldId` | 2 |

## Acceptance

`ItemId::II_DEFAULT` (retired) has `IsRetired == true`, and so on for every enum in the table above.
`kItemIdLiveMembers.size() == 7244` and `kTextIdLiveMembers.size() == 11211`. Nothing else in the
emitted API changes.
