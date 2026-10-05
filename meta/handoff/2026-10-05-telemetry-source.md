# Telemetry → Source: what changed in Canon, and one question (2026-10-05)

For the Source team (read-only for the compiler). Follows
[2026-10-04-telemetry-canon-ready.md](2026-10-04-telemetry-canon-ready.md).

## Question: the ledger classification of GrantKind 3 and 48

`LEVEL_UP_GIFT` (3) and `GUILD_DISBAND_BANK` (48) are retired, but the lake still holds rows with
those codes. Canon can now type a ledger role that names them (`past`, ADR-0015, DECISIONS 304),
but no ledger role was ever written for either code, so their classification cannot be recovered
on our side. For each code, please give what `LEDGER_KIND_TABLES`
(`Events/TelemetryItemsEvents.h`, around lines 156-217) says or said: currency, bucket, sign,
token, counted, filter (e.g. `COALESCE(via_mail, 0) = 0`), amount/item columns, dedupOf, note,
and any version bound. The two roles then go into `examples/telemetry/events.canon`.

## What is available now

- **Retired members in history** (`past`, DECISIONS 304): a slot typed `past GrantKind` (or `[past
  Member(of)]`) may hold retired members; plain slots still refuse them, and the error says so.
- **Enum reflection by value** (DECISIONS 306): `Member(v).members` and `.typeName` replace the
  per-enum copies. Telemetry's `enumOf` is one expression; outputs did not change.
- **A ledger role names its vocabulary, not its column.** `rows: some { of: grant_kind, kinds:
  [...] }`: the column is the one column of the event that holds that vocabulary (`columnOf`), and
  a check refuses a vocabulary held by no column or by several. Generated code: `RowsSome` loses
  `ColumnID()` (Go) and `GetColumnKey()` / `column_` (C++); use `Of()`. No caller was found in
  Engine, Tools or services/monitoring. SQL and catalog.json are unchanged.

## The catalog is now typed JSON: format 3

DECISIONS 308: `catalog.json` is generated from typed records (`@text` returning `CatalogDoc`), not
hand-escaped strings. Any Unicode text works; the printable-ASCII limit on descriptions, notes and
filters is gone. Changes from format 2, and nothing else:

1. `"format": 3`.
2. Each event starts with `"state": "live"` or `"state": "retired"` (the variant tag comes first).
   Live events no longer carry `"retired": false`; a tombstone is
   `{"state": "retired", "id": 20, "table": "mail_item_received"}` instead of `"retired": true`.
3. Arrays are one element per line (`producers`, ledger `kinds.values`); `[]` stays on one line.
4. Text is raw UTF-8: any Unicode in a description, note or filter is written as is (none of the
   current texts uses non-ASCII; they mirror Source).

Unchanged: key names and order, `null` for none, `kinds: null` for "every row", the column objects
`{name, type, kind, nullable, since, retiredIn, enum}`, the enum entries `{name, view, values:
[{code, name, retired}]}`, the final LF. SQL outputs are byte-identical. The reader in Source must
accept format 3 (switch on `state`).

```json
  "events": [
    {
      "state": "live",
      "id": 11,
      "table": "item_created",
      "version": 5,
      "producers": [
        "WorldServer",
        "DatabaseServer"
      ],
```

## Note, 2026-10-06 (DECISIONS 317-323)

- `examples/telemetry/telemetry.canon` now tests keys with `headerColumns.hasKey(c.name)` (317);
  the catalog output is unchanged (format 3 as above).
- Generated C++ headers gained `detail::<P>Make` structs and friend lines (323). No change to the
  API Source code calls. No regenerated Source output is required unless Source regenerates.
