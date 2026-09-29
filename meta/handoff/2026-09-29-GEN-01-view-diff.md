# GEN-01: view-model goldens (M3 acceptance 2), diff and choices

Informational (DECISIONS 190, log-2026-09-28 item 14): no action is needed from Louis.

`internal/testkit/golden` now builds `pipeline`, `resource.farm` and `resource.events` for the
`view` target too, and `go test ./internal/testkit/golden -update` wrote their view goldens and
MANIFEST lines. Every view golden, and the view model of every example with an `emit view`
(nine), validates against `spec/viewmodel.schema.json` (VIEWMODEL V2); `make goldens-check` no
longer skips `examples/pipeline/expected/potion.view.json`.

## The diff

| Golden | Status | Size |
|---|---|---|
| `examples/pipeline/expected/potion.view.json` | pre-drafted; the compiler's output is byte-identical (0-line diff) | 7 821 B |
| `examples/resource/farm/expected/generated/farm.view.json` | new, generated | 45 824 B |
| `examples/resource/events/expected/generated/events.view.json` | new, generated | 40 163 B |

Golden names follow the harness rule (`manifestGolden`): `@generated/farm.view.json` is frozen
as `generated/farm.view.json`, as teamboard's `@sovcommon/...` outputs are.

## Review against the normative passages

- **Farm, I18N §11:** 88 catalogue keys, broken down exactly as §11 lists (6 type docs, 27
  labels, 27 helps, 1 deprecation, 1 placeholder, 3 titles, 4 singulars, 11 group labels, 3
  intros, 3 named checks, `farm` and `farm.help`); French translates 49, `missing` is 39, and
  the missing set contains `ModelType.typeName.help`, `Level.level`, `Global.maxModels` and
  `farm`. `ItemCost`, `Bonus` and `ModelType` titles carry no key (L6). The model's `W1701`
  says 39; `W5001` carries its French message (J15).
- **Farm, VIEWMODEL:** `maxModels` in "Unused fields", read-only `deprecated` (L12, C43);
  `settings` flattened (L16); `productionTick` offers `s m h d` (X12: wire unit `s`, no upper
  bound); `modelTypes` a table keyed by `typeId` whose width 70 goes to `$entry` (T7), `levels`
  a `count` column (T8); `levels` the `ordered_steps` widget over its table fallback (C41);
  local define tables `texts` and `attributes` indexed as `ref` targets (S1), both `select`
  (C5, ≤ 10 entries); `items` a `search` (16 entries).
- **Events (I18N gives no number):** 70 keys, recounted by hand from I18N §3.3 (6 docs, 28
  labels incl. 3 cases and 2 enum members, 26 helps, 2 named case checks, 1 title, 1 singular,
  4 group labels, 1 intro, `eventConfig`); no French file, so 70 missing, as its findings.txt
  already said. Inline `kind` lays its 11 case fields out in the parent's `_other` as
  `kind.<case>.<field>` with `case`/`path` (L17, L18); usage per shape `kind=<case>` (L19);
  `sovcommon.time` types counted under their own names (L10); `version` read-only `single` 1
  (C34); the `kind` column in mode `case` (T8).

## Differences

None. No drafting difference and no compiler bug was found; nothing was kept against a draft.
