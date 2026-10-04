# Telemetry recon for `examples/telemetry` (2026-10-04)

Read-only recon of `/home/louis/Desktop/Sovereign/Source` (paths relative to it) for the Canon
dry run (L-0111 d.1.8). Source stays read-only: any need there is a handoff.

## Facts that shape the dry run

- `views/_tables.sql` holds **no CREATE TABLE**: per table a read macro substituting the table
  name. The column DDL is only in C++ `CreateTableSql()` (`Engine/Telemetry/TelemetryEvents.cpp:231-258`),
  built from a provenance prefix (:190-195), the `TELEMETRY_SQL_*` macros
  (`Events/TelemetryCoreEvents.h:154-166`) and retired columns (appended after live ones).
- Tombstones 20 and 156 are **comments only** in `Config/TelemetryConfig.h` (enum EventType :256,
  comments :272, :378); `schema.lock.json` has `"retired_event_ids": []`; no output mentions them.
  Comment :272 holds a non-ASCII em dash (catalog.json refuses non-ASCII).
- Retired enumerators: deleted with a gap comment (GrantKind 15, `Events/TelemetryItemsTypes.h:64-65`)
  or kept with a "retired" comment (`LEVEL_UP_GIFT = 3` :27, `GUILD_DISBAND_BANK = 48` :131). No
  generator knows either form; kept ones are emitted as members.
- **Version history is not in the headers**: only the current version (sometimes a
  `#ifdef __TELEMETRY_LEDGER_V2` alternative). History is the lock's per-column `since` /
  `retired_in`; `since` is ≥ 2 for tables older than the lock and is not prefix-ordered
  (item_created: `item_instance_id` since 3 sits before `grant_kind` since 2). A Canon check
  "version N+1 starts with version N's columns" would refuse item_created, item_drops, npc_buys,
  item_consumed: L-0111 d.1.2's rule needs restating before the port (Source's call).

## Where the catalogue lives

- Event list `Engine/Telemetry/TelemetryEvents.h:29-115`: `X(C, Struct, Name, ENQ_PUBLIC|ENQ_INTERNAL, EMIT_*)`;
  producer masks `Client/Telemetry/TelemetryEmitters.h` (:41 `EMIT_SPINE = WS|CORE|DB|LOGIN`,
  :50/52 `EMIT_WS_PRE_LEDGER_V2`, 0 under V2).
- META: `M(Struct, EVENT_X, version, "table", QueueTier::T, has_instance_id, sizeof, "desc")`
  (sizeof feeds a static_assert). QueueTier HOT/WARM/COOL/TINY (+READONLY under V2,
  `TelemetryCoreEvents.h` ~195-210).
- Enums `Events/*Types.h` (`enum class X : uint8_t`; `ProducerKind`/`AckStatus` use
  `std::uint8_t`, read separately by `catalog_json.py:101-130`).
- Outputs `Tools/telemetry/`: `catalog.json`, `schema.lock.json`, `views/_tables.sql`,
  `_enums.sql`, `_timeline_union.sql`, `_ledger_map.sql`. Generators `Tools/telemetry/gen/`
  (`gen_views.py`: SQL + lock, `build_lock` :1902, `dump_lock` :1925; `catalog_json.py` `build`
  :523; `gen_catalog.py`; `schema_lock.py` L1-L9).

## Proposed events (7) + tombstones 20, 156

| id | table | ver | tier | producers | inst | covers |
|---|---|---|---|---|---|---|
| 151 | farm_events | 2 | COOL | WorldServer | no | retired column, enum column, doc sink with `ver=` |
| 152 | sovereign_events | 1 | TINY | CoreServer | no | 4 enum columns, doc mint, `L(*)` none row |
| 71 | trade_penya | 2 | COOL | WorldServer | no | counted transfer, 2 legs, `D()` defaults |
| 70 | penya_drops | 2 | HOT | WorldServer | no | counted mint |
| 110 | server_lifecycle | 1 | TINY | WS, Core, DB, Login | no | ENQ_INTERNAL, no ledger |
| 31 | item_consumed | 3 | READONLY | `[]` | yes | conditional META, doc sink |
| 11 | item_created | 5 | WARM | WS, DB | yes | GrantKind with retired members (62 enumerators, 62 ledger rows; optional) |

- farm_events: META `Events/TelemetryContentEvents.h:42-43` (V2) / :57-58; FIELDS :45-52; RETIRED
  :54-55 `R(int64_t, cost, TF_SCALAR, 2)`; LEDGER :70-84; FarmEventKind
  `Events/TelemetryContentTypes.h:50-61`. catalog.json object at line 15147; lock :7274;
  `_tables.sql` :1158, catalog row :1443; `_timeline_union.sql` :999-1009; `_ledger_map.sql`
  :420-423; `_enums.sql` :717-730, enum_columns :1013. DDL:
  `CREATE TABLE IF NOT EXISTS farm_events (event_version UTINYINT NOT NULL, producer_id UINTEGER NOT NULL, boot_id BIGINT NOT NULL, batch_seq BIGINT NOT NULL, row_no USMALLINT NOT NULL, timestamp_ns BIGINT NOT NULL, player_id UINTEGER NOT NULL, kind UTINYINT, slot_index UINTEGER, type_id UINTEGER, item_count UINTEGER, target_player_id UINTEGER, fact_id UBIGINT, cost BIGINT);`
  Ledger `"doc gold sink token=farm filter={cost <> 0} dedup=farm ver=..1"` →
  `('farm_events', 'Purchase', 'gold', 'sink', -1, 'farm', NULL, 'enum_farm_event_kind', 'kind', 'cost <> 0 AND event_version <= 1', 'cost', NULL, 'player_id', NULL, false, 'farm', '<note>'),`
- sovereign_events: META :129-130, FIELDS :132-142, LEDGER :144-157; `_enums.sql` :732/741/752/765;
  ledger :425-426 (`L(*)` → kind NULL, `'none','none',0`); catalog 15309; lock 7359.
- trade_penya: `Events/TelemetryEconomyEvents.h:34-48`; ledger :68-69 (`leg` out/in, reconciles
  true); catalog 7536; lock 4368; timeline :460.
- penya_drops: :17-29; ledger :66.
- server_lifecycle: `Events/TelemetrySpineEvents.h:97-102`; LifecycleKind
  `Events/TelemetrySpineTypes.h:14-17`; `_enums.sql:178-183`; catalog 8345; lock 4874; timeline
  :559-568.
- item_consumed: `Events/TelemetryItemsEvents.h:885-900`; lock :3688 (since 2,2,3,2); ledger :249.
- item_created: META :84-108; GrantKind ledger coverage (`LEDGER_KIND_TABLES`) :156-217.

## Formats

- catalog.json: keys contract_version (2), enums, error_tokens (3), format (1), generated_by
  (`"Tools/telemetry/gen/gen_catalog.py"`), partition_column, producer_kinds (7: id/name/emits),
  provenance_columns (5), schema_hash, tables (86, id order), views (23). Table: columns,
  description, event_enum, event_id, has_instance_id, ledger, macro (`tbl_<t>`), name, producers,
  struct, tier, version. Column: enum, kind, lookup_view, name, nullable (= kind not in TF_TS,
  TF_ID), since, type, vocabulary, `retired_in` only when retired. Ledger entry = a `_ledger_map`
  row minus source_table and note. Enums sorted by name `{backing, lookup_view, name,
  values:[{id,name}], used_by:[{column,table}]}` (no member descriptions). `views[]` holds the
  sha256 of every `views/*.sql`, hand-written ones included: a 5-10 event subset can match a
  subset catalog only, never the real bytes.
- schema.lock.json: `{breaks:[{date,reason,violations}], enums:[{backing,name,values:[[name,id]]}],
  events:[{columns:[{enum,kind,name,since,type}], event_enum, event_id, retired_columns?, table,
  version}], format:1, retired_event_ids:[]}`; events by id, enums by name; `breaks`,
  `retired_event_ids` and every `since` are hand-owned history.
- `_timeline_union.sql`: banner, `CREATE OR REPLACE VIEW _timeline_union_stubs AS` with one
  `SELECT '<t>' AS source_table FROM read_parquet(getvariable('lake_dir') || '/<t>/_schema/date=1970-01-01/schema.parquet')`
  per table joined by `\nUNION ALL\n`; then `timeline_union(from_day, to_day)` as one SQL string
  literal (quotes doubled), per table `detail = to_json(struct_pack(c := c, ...))` over columns
  after the first two, retired last (`to_json(NULL)` if none), `@DAYS@/@FROM@/@TO@` placeholders.
- `_ledger_map.sql`: banner (:1-5), `LEDGER_MAP_STATIC` (`gen_views.py:1694-1751`), `_ledger_map AS
  SELECT * FROM (VALUES` with `    -- <table> (TELEMETRY_LEDGER_<Struct>)` before each struct's rows,
  17 `LEDGER_COLUMNS`, comma after every row but the last; then `_ledger_enum_values` aligned with
  `ljust(max+3)`, then fixed `ledger_map`, `_ledger_map_coverage_gaps`, `_ledger_map_unresolved`.

## Byte-for-byte quirks

- Two banners: `_tables`/`_enums`/`_timeline_union` say `Scripts/lanes/telemetry/gen_views.py`,
  `_ledger_map` says `Tools/telemetry/gen/gen_views.py`; banner ends `\n` and is joined with `\n`
  (blank line 4).
- Order: SQL per-table sections follow `FROZEN_EVENT_ORDER` (`gen_views.py:308-331`; unlisted last
  by id); SQL enums `FROZEN_ENUM_ORDER` (:287-306); catalog/lock id order, enums by name; ledger rows
  header include order then line order. Retired columns after live ones.
- Every file ends in exactly one `\n`; JSON = `json.dumps(indent="\t", sort_keys=True,
  ensure_ascii=True) + "\n"`; `schema_hash` = `sha256:` over compact sorted JSON without the hash,
  pure ASCII.
- `event_catalog_static` writes ids as `CAST(n AS SMALLINT)`.
- `_enums.sql` adds vocabulary views (`enum_event_type`, `enum_time_frame` from
  `Resource/Server/Vocab/*.json`) and two vocabulary rows in `enum_columns`.
- Ledger: `ver=..1` → `AND event_version <= 1`; doc rows `false`, counted `true`.
- `parse_ledgers` errors unless gold_changes, item_removed, item_created and
  points_balance_changes are present with every kind enumerator covered: a subset needs the rule
  relaxed or those tables included.
- `#ifdef __TELEMETRY_LEDGER_V2` decides the active META, tier and producers.
