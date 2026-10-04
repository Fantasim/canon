# Handoff to Source: Canon is ready for telemetry (ADR L-0111 d.1.8)

From `services/configlang` (Canon). For the Source orchestrator and its paired monitoring/Scripts
sessions. Read-only for us: everything below is input to Source's own ADR work.

## What Canon now provides (main, 2026-10-04)

| L-0111 d.1.8 prerequisite | Status |
|---|---|
| M5 accepted, hardening pass | done (M5 CI run 36973712977; M1.5 acceptance clean on 3 seeds) |
| C++ `baked` generator | done: `emit cpp { mode: baked }` (DECISIONS 293, 296, 298) |
| constexpr metadata | done: a package-level lookup over enums (or precomputed fn) with a scalar result (integers, Float, Bool, String as `std::string_view`, Duration, enums, table ids) is `inline constexpr` in the header; usable in `static_assert`, template arguments, `if constexpr` (DECISIONS 293) |
| a way to write `.sql` files | done: `emit text { out: dir }` + `@text("file")` on a public, parameterless, String export fn; bytes verbatim; ownership by a `.canon-text` listing; `canon build --adopt <path>` takes a hand-written file over once (DECISIONS 294, 295, 297, 300) |
| `examples/telemetry` dry run | done: 7 real events + 2 tombstones, built, compiled (g++, clang++, libc++, `-Werror`), run |

Also available: Go `baked` (monitoring), `canon build --root name=path` (worktrees), `canon build
--check` (drift gate), `canon.lock` (renumbering or reuse of an id or code is a build error), enum
reflection `E.members` / `.retired` (DECISIONS 299).

## Louis's ruling that changes L-0111 d.1.7

Louis (2026-10-04): Resource/Source's current shapes are the pain Canon removes; the Canon package
is designed the Canon-native way and does **not** reproduce `gen_views.py` byte for byte. Source
refactors its integration to the new outputs. L-0111 d.1.7's byte-for-byte proof against today's
four SQL files, `catalog.json` and the lock is therefore to be revised by Source; the lake-facing
facts stay exact (each `CREATE TABLE` equals today's `CreateTableSql()` bytes).

## The reference package: `examples/telemetry`

Read it in this repository (`telemetry.canon`, `vocab.canon`, `events.canon`, `sql.canon`,
`catalog.canon`) and its generated outputs under `examples/telemetry/expected/`. Design:

- **EventType**: `enum EventType @codes(UInt8)`, one member per event named after its lake table, so
  one name is the event id, the table name and the C++/Go enumerator. Tombstones are retired members
  (`retired mail_items_received = 20`, `retired world_boss_spawns = 156`); `canon.lock` pins every
  code.
- **Vocabularies**: Tier, Producer, Enqueue, SqlType, FieldKind, Currency, Bucket, Leg are enums; the
  telemetry enums (FarmEventKind, SovereignEvent*, LifecycleKind, GrantKind) are `@codes`, member
  descriptions are doc comments (they reach C++ and Go), removed values are retired members. A
  `Vocabulary` enum names the enums a column can hold; labels come from `E.members`, retired
  included. The dry run trims GrantKind to 14 of 62 members.
- **Columns**: records `{name, sql: SqlType, kind, values: Vocabulary?, since?, retiredIn?}`;
  nullable, DDL fragment and byte width are derived; the header columns and provenance prefix are
  declared once.
- **Events**: `let events: {EventType: Event}`; each event has description, version, since (first
  known version), tier, producers, enqueue, columns (keyed by name, wire order, a retired column left
  in place), ledger roles, ledgerKinds. A package check refuses a live type without a table.
- **Ledger roles**: records; `rows` is a variant (`every`, or `some { column, of, kinds: [Member(of)] }`
  where `Member(v)` is a dependent type, so a kind outside the column's enum is a type error);
  amount, item, player, counterparty are refs into the event's own columns. When `ledgerKinds` is
  set, every live kind must be covered (L-0111's coverage rule, as a Canon check).
- **Lookups** (constexpr in C++): `IsRetired`, `VersionOf`, `TierOf`, `TableOf`, `HasInstanceId`
  (derived), `WireSize` (packed size, replaces META's pinned sizeof), `Emits(e, p)` (replaces the EMIT
  masks), `CreateTableSql(e)` (a constexpr `string_view`: the ingester preflight can become a
  compile-time check; Go `CreateTableSQL`). Tombstones answer neutrally (version 0, READONLY, size 0).
- **Outputs**: C++ baked to `@source/Engine/Telemetry/Generated` (namespace `Telemetry`); Go baked to
  monitoring; `emit text` writes `tables.sql`, `enums.sql`, `catalog.json`.

## Version history: proposal for L-0111 d.1.2

Canon checks today, in `Event`: V1 column names unique (retired included); V2 `since` in (first
known, version] and `retiredIn` in (since, version]; V3 each version after the first adds or retires
a column; V4 a ledger role never reads a retired column unless `untilVersion` is below its
retirement; V5 header columns implicit, an enum8 column is one vocabulary and UTINYINT; coverage.

L-0111's "version N+1 starts with version N's columns" refuses item_created, item_drops, npc_buys
and item_consumed (since values not prefix-ordered). Proposed instead: the column set only grows
(columns(N) ⊆ columns(N+1) ∪ retired(N+1)); names and types never change; retired columns stay in
the DDL after the live ones; each bump changes the layout; wire order is free (Deserialize is
strict per version, the lake reads by name). Not checkable across builds yet (see open points).

## Where the outputs differ from today's Source files

- **tables.sql** is new (today the DDL exists only in C++ `CreateTableSql`): one `CREATE TABLE` per
  live table, id order; bytes equal today's (farm_events verified against the C++ bytes, all 7
  against the lock-derived rule).
- **enums.sql**: no banners, no Vocab-derived views (`enum_event_type`, `enum_time_frame`); only
  vocabularies a column holds, in first-use order; every member listed, retired codes included
  (today's gap-comment enumerators become labelled rows); `event_catalog_static(source_table,
  event_type, tier, has_instance_id, event_version)` with plain integers (no `CAST AS SMALLINT`),
  no `event_type_name` or `columns` (now in catalog.json): the hand-written `event_catalog` macro
  needs adapting.
- **catalog.json**: a new documented format 1 (2-space indent, documented key order; no
  schema_hash, views hashes or generated_by); every event type in id order, tombstones as
  `{id, table, retired: true}` (replaces `retired_event_ids`); live events carry since, enqueue,
  wireSize, columns (header first, retiredIn in place), ledger roles; enums `{name, view, values:
  [{code, name, retired}]}`.
- **Not produced**: `_timeline_union.sql`, `_ledger_map.sql`, `schema.lock.json`, the producer_kinds
  listing (Source decides whether to derive them from Canon or drop them).
- **C++**: `Telemetry::EventType` is an `enum class` with table-name members, not `EVENT_X`;
  metadata from functions (`VersionOf(T::TYPE)`) rather than `T::VERSION` members; no SQL/JSON
  strings in the header.

## Open points

- **Retired members in history data (Louis Q1, pending).** A retired enum member cannot appear in
  stored data (`E3506`), so a ledger role cannot name GrantKind 3 or 48; their labels survive in
  `enums.sql` and `catalog.json`. See `meta/handoff/2026-10-04-louis-calls.md`.
- **MSVC never compiled Canon output** (L-0111 risk 2). Linux g++/clang++ (libc++ too) under
  `-Werror` only; Windows file names of text outputs are guarded (DECISIONS 297).
- **CI is down** (GitHub Actions billing); every result above is from local `make check`. Pin the
  `canon` version to a commit after CI is green again.
- Canon does not yet check version history across builds (nested keyed lists have no lock facts):
  a language change if wanted.

## How to consume

1. `Resource/project.canon` declares roots for the subscribers (`source: "../Source"`,
   `monitoring: "../services/monitoring"`); an `emit` outside a root is `E7001`/`E7003`.
2. `Resource/Telemetry/*.canon` (the package) with emits: `emit cpp { out: "@source/Engine/Telemetry/Generated", namespace: "Telemetry" }`
   (baked is the default), `emit go { out: "@monitoring/…" }`, `emit text { out: "@source/Tools/telemetry/generated" }`.
3. `canon build` in Resource writes into the sibling trees; outputs are committed in Source;
   `canon build --check` joins `sov telemetry check`. First build over existing hand-written files:
   `canon build --adopt <path>` per file.
