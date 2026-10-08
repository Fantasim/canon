# Sovereign needs Go `types` mode (2026-10-08)

From the Sovereign `Source` session (ADR L-0119, Resource into Canon, canon v0.1.2). Paste as is.
Louis's ruling: each service gets Canon-GENERATED Go to read Resource; the hand-written readers
in the shared Go library `sovcommon` (~10.5K lines) are deleted.

## What is needed

The same thing `emit cpp` already gives in `types` mode, for Go: a generated Go type for a
package's records PLUS a generated loader that reads a runtime file Canon itself renders, so a
service reads **the very files the game reads** (no second data set). Today:

```
error[E8019] emit go cannot use types mode yet; use baked mode
```

## Shape wanted

- `emit go { mode: types, out: [<copies>], package: "<name>" }` on a package that renders runtime
  files through `@text` fns returning typed values (Sovereign phase 1: plain JSON, wire names,
  declaration order, absent optional omitted, `@json(none: X)` honoured -- your 2026-10-07 reply).
- Per such `@text` file: a Go type for the file value and `Load<File>(path string) (<Type>, error)`
  that decodes exactly what the `@text` fn writes (wire names, `none` spellings like `"="`,
  optional/absent, enum fields by wire name or code, maps keyed by enum). No package global.
- Enums: `type X uint16` + `XFromName/XFromCode` + `Name()`, as `combat.vocab`'s Go emit has today.
- `out` as a list of copies (one per consumer project root, each with its own `go_module`),
  like your list-out copies for TS. Sovereign adds roots `admin`, `website` (optional roots).
- A refusal when the file on disk does not match the generated type (unknown key / wrong type),
  with path + reason; the `$schema` fingerprint idea of data mode is fine if cheap.

## Acceptance (Sovereign side)

1. `items` package (renders `Server/Item/propItem.json`, ~7.2K rows, full rows + `defaults`)
   with `emit go { mode: types }` builds; `LoadPropItem` on the real 23.7 MB file returns every
   row, equal (field by field) to what the C++ loader reads.
2. Same for: `Server/Text/serverNames.json` (ui.servernames), the job registry
   (character.vocab Job + defineJob values), `System/vip_config.json`, `Balance/expTable.json`,
   `Map/WorldLevel.json`, `Quest/SeasonPass.json`, `Quest/MilestoneRB2.json`,
   `System/GuildTalentTree.json`, `System/NMBuff.json`, `Text/DSTString.json`, the DST and TID
   registries (engine.vocab / text.vocab enums), `Client/textClient.json` (`us` column).
3. `canon build --check` stays green with the Go copies in admin, website and monitoring.

## Who uses it (census, Sovereign `meta/resource-canon/sovcommon-plan.md` section 2)

- admin: item catalogue (id, symbol, name via serverNames, description key `_TXT_`->`_TXTD_`,
  icon), VIP level formula inputs, guild member cap, job names/tiers, Faery max level, guild
  ranks, season-pass rewards, live-bonus DST list with English labels, client-text picker,
  talent tree + server-buff names.
- website: item names/icons, job list, monthly donate milestones.
- monitoring: item names (inspector search), DST symbols + rate flags, element display names.

## Priority

Blocking for Sovereign's cutover: on the campaign branch, monitoring already lost its item
names at boot (it read a vocab JSON that Canon now owns), so its switch must ship with the cutover.
