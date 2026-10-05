# ADR-0015 — `past E`: a type whose slots may name retired members

Date: 2026-10-05. Status: accepted (Louis, answering [louis-calls Q1](../handoff/2026-10-04-louis-calls.md));
the rule becomes DECISIONS 304 when the spec is synced. Not implemented yet.

## Context

A retired enum member or variant case may not appear in any stored value (`E3506`, TYPES.md
§8.1), except inside a retired table entry. Computing with retired members is already allowed
(DECISIONS 299): the check runs on stored values only (`internal/verify/scalars.go`). Telemetry,
the first real use, hit the gap: GrantKind 3 (`LEVEL_UP_GIFT`) and 48 (`GUILD_DISBAND_BANK`) are
retired, but the lake still holds rows with those codes, and the ledger must keep classifying
them. A role with `kinds: [LEVEL_UP_GIFT]` stores a retired member, so it is `E3506`. Today the
example drops those roles, and the gap is listed in the Source handoff.

Describing the past is a legitimate need of any data that outlives its vocabulary (ledgers,
analytics, migrations). The language must express it. Otherwise users either never retire a
member, or move the description out of Canon and lose its typing.

## Decision

- **Retired means "no new use", not "unnameable".** By default a slot typed `E` still refuses
  retired members. A slot typed `past E` also admits them. New data keeps the ban; a description of
  history declares that it is one.
- **The marker is a type, not a field annotation.** `past E` goes wherever a type goes: a field, a
  stored `let`, a list element (`[past GrantKind]`), a map key or value, an optional. A field
  annotation (`@history`, the first proposal) would not cover a stored `let` or a nested
  position, and it would need special cases to do so. The existing retired-entry exception is the
  same principle (only something about the past may name something retired), applied to a scope.
- **Syntax.** `past` is a contextual keyword (GRAMMAR §4.2) at the start of a type, before a
  name of an enum or variant: `primType = … | "past" namedType`. Prefixing follows `stable table`
  and `ref`. It stays an identifier everywhere else, so no existing name breaks. `past` on any
  other type is a new type error, in the TYPES range, assigned when ERRORS.md is synced.
- **Statically, `past E` is `E`.** Members, `match` (already exhaustive over retired members),
  operators and assignability are unchanged in both directions. Retirement stays a check on
  stored values: the stage-B walker skips `E3506` for the members or cases of `E` inside a slot
  declared `past E`, the same way it skips them inside a retired entry. Values nested inside a
  `past V` case keep their own rules. This adds no new kind to the type system.
- **Loaded data follows the slot.** A JSON or CSV value loaded into a `past E` slot may carry a
  retired code. This is how the lake's history comes in.
- **Erased in output.** `gen/go`, `gen/cpp` and `gen/ts`, the JSON data, views and the view model
  emit `past E` as `E`; the generated enums already hold every code. The formatter and the LSP
  print it as written. The lock is unaffected: it guards codes, not slot types.

## Consequences

Refs have the same gap: a live entry referencing a retired one is `E3502` (TYPES §10.3). The
design review ([2026-10-05](../handoff/2026-10-05-design-review.md) item 1) proposes
`past ref T` in the same unit; it is decided together with DECISIONS 304.

Telemetry's ledger types `kinds` as `[past GrantKind]` and restores the roles for codes 3 and 48.
Retirement stays meaningful: a retired member can come back only through a slot that says it is
about the past, and a reviewer sees that in the type. The rejected option, retired members
nameable everywhere, would let them creep back into live data unnoticed.

Work: spec sync (DECISIONS 304; GRAMMAR §4.2 and `primType`; TYPES §8.1; ERRORS `E3506` and the
new code; the guide docs), then one unit per package: `syntax` (parse, print, fuzz corpus),
`types` (resolve, the new error), `verify` (scope the skip), a mechanical erasure sweep over the
backends checked by goldens, and the telemetry example. Estimate: about a day of agent work.
Implementation waits until spec-reviewer is available again (2026-10-06).
