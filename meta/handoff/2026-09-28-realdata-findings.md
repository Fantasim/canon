# Real-data findings (2026-09-28, refreshed 2026-09-29)

`make check-real` (M3 acceptance item 7, IMPLEMENTATION-PLAN §7.3) on the real `Resource/` tree
at commit `6f908d8c499eb64a05132f20d8eb827d7e444614`. The full output is in the git-ignored
`testdata-real/realdata/findings.txt` (its header now names canon at `5cc4f4b`, after the view
and translation checks landed). **List only**: no fix is proposed here; the types win over the
data (Resource migration plan, owed with M7). Not gating.

| Package | Errors | Warnings |
|---|---|---|
| balance.parity | 0 | 0 |
| features.legacycpp | 0 | 0 |
| features.text | 0 | 0 |
| game.items | 0 | 1 |
| resource.adventurequest | 311 | 1 |
| resource.events | 311 | 1 |
| resource.farm | 311 | 1 |
| resource.heistia | 311 | 1 |
| resource.rules | 120 | 0 |
| resource.vocab | 311 | 1 |

Distinct findings (the same file is reported by every package that imports it):

1. **E3302 ×311 — `@resource/Server/Item/propItem.json`**: `resource.vocab.Item needs field
   icon`. 311 item entries of the real file have no `icon`, a required field of
   `resource.vocab.Item` (the fixture's trimmed rows all have one). Reported once per package
   that loads vocab — now 5 packages (adventurequest, events, farm, heistia, vocab):
   5 × 311 = 1555 lines in `findings.txt`. Same data issue as before; the extra packages are new
   `check-real` coverage (i18n/view checking landed since), not a new error.
2. **E7110 ×120 — `@resource/Server/System/tower_config.json`**: `expected a string for String,
   found an object`. 120 values the example types as `String` are JSON objects in the real file
   (resource.rules). Unchanged.
3. **W1701 — missing `fr` translations**, one line per package that declares text and has no
   `fr` catalogue for it (`game.items`, `resource.adventurequest`, `resource.events`,
   `resource.farm`, `resource.heistia`, `resource.vocab`; 6 lines total). New since i18n checking
   (U6/U8) was wired into `build`/`check`: these are real translation gaps in `Resource/`, not a
   compiler bug — each names the language and points at `canon i18n status <pkg> --lang fr
   --list`.

Nothing else: no other code, no other warning. Still two underlying data issues (missing
`icon`, `tower_config` typed as objects) plus the newly-visible translation gap; no compiler bug.
