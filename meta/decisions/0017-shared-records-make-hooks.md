# ADR-0017 — Shared records across packages: make hooks, own readers, honest refusals

Date: 2026-10-06. Status: accepted (DECISIONS 317-324; branch `feat/past-and-ergonomics`).

## Context

The design audit (handoff 2026-10-06) found Canon accepting programs its generators refused: a
record of another package could not be used inside a value or data file (`E8019`
ForeignDataRecord, CrossPackageBakedValue), and several modes passed `check` and failed `build`.
Louis asked for both fixed before v0.1.0 (DECISIONS 320, 323). The rulings of the wave are in
[log-2026-10-06](log-2026-10-06.md); this ADR records the durable implementation choices.

## Decision

- **The owner builds, the holder reads.** A package P exposes one "make hook" per record, case,
  case type, dependent branch, table entry and row type (Go `Make_<T>`…, C++ members of
  `detail::<P>Make`, CODEGEN §5.14). Hooks take storage, never `id`/`retired`; entry and row hooks
  take those. A package Q that holds P's values reads P's wire with its own readers
  (`decode_<gopkg>_<T>`, `detail::Read_<path>_<T>` with internal linkage, TS `read_<alias>_<T>`)
  and calls P's hooks; it never calls P's public decoders. No frozen contract changed.
- **Names come from one plan.** `internal/ir` computes every reserved name (hooks, rows, readers,
  aliases, `<gopkg>rt`, `OwnerAccessor`) and the foreign uses (`ForeignUses`, `ForeignRows`,
  `EmitDefines`); generators consume them, and plan-vs-generated tests compare the two per target.
  `RefTarget` gained `Cpp NameOptions` (additive).
- **TS writes other packages' names qualified** through one namespace import per package named
  (`game_core.StatusId`); every TS table holds `CanonRow<T, K>`; generator-made locals are
  `$`-prefixed and escaped user params `$`-suffixed, so no Canon name can meet them.
- **Readers are as strict as the owner's.** Unknown keys, letter case, `$id`/`$retired` by
  position, and `no entry <key>` for refs (fields, lookup cells, map keys) into an owner table whose
  ids the generator knows.
- **What a generator cannot build is refused at stage E**, never at build: E8019 carries a way
  out (`Way<Kind>`), `unbuilt` names the mode that builds, E8013 never names a failing mode; every
  backstop left in a generator is `ErrMalformed` with the rule that refuses first. E8018 retired.
- **Precomputed results are verified** like expect subjects (DECISIONS 324): no value path, the
  precomputation's frame always outermost with an exact hidden-frame count, charged to the
  precomputation's root; eval gained additive hooks (`CallFrame`, `Outermost`, `RunUnder`).

## Consequences

Emberfall's copied types can move back to `game.core`. Owed for v0.2: Go `embedded`/`types`,
C++ `embedded`, dependent values decided through a ref or optional, legacy structs (M6). The
hooks are "for generated code" (DECISIONS 4), not an API contract.
