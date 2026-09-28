# Real-data findings (2026-09-28)

`make check-real` (M3 acceptance item 7, IMPLEMENTATION-PLAN §7.3) on the real `Resource/` tree
at commit `6f908d8c499eb64a05132f20d8eb827d7e444614`, canon at `bb2b756`. The full output is in
the git-ignored `testdata-real/realdata/findings.txt`. **List only**: no fix is proposed here;
the types win over the data (Resource migration plan, owed with M7). Not gating.

| Package | Errors | Warnings |
|---|---|---|
| balance.parity | 0 | 0 |
| features.legacycpp | 0 | 0 |
| features.text | 0 | 0 |
| game.items | 0 | 0 |
| resource.adventurequest | 0 | 0 |
| resource.events | 311 | 0 |
| resource.farm | 311 | 0 |
| resource.heistia | 311 | 0 |
| resource.rules | 120 | 0 |
| resource.vocab | 311 | 0 |

Distinct findings (the same file is reported by every package that imports it):

1. **E3302 ×311 — `@resource/Server/Item/propItem.json`**: `resource.vocab.Item needs field
   icon`. 311 item entries of the real file have no `icon`, a required field of
   `resource.vocab.Item` (the fixture's trimmed rows all have one). Reported once per package
   that loads vocab (events, farm, heistia, vocab: 4 × 311 = 1244 lines).
2. **E7110 ×120 — `@resource/Server/System/tower_config.json`**: `expected a string for String,
   found an object`. 120 values the example types as `String` are JSON objects in the real file
   (resource.rules).

Nothing else: no other code, no warning.
