# Fixtures for the examples

The examples read files outside this repository: `@resource/Server/...`, `@client/Client/...`
(see the roots in `../project.canon`). Tests never read the real repositories. They read these
fixtures instead: small copies of the real files, trimmed to the rows the examples use.

## How tests use them

The test harness opens `examples/` with **every root redirected**:

- a root that has a directory here (`_fixtures/resource/`, `_fixtures/client/`) points at it;
- every other root outside the project (`source`, `services`, `sovcommon`, `web`, `parity`,
  `generated`) is only written by `emit`, and points at a fresh, empty temporary directory that
  the harness creates (a required root must exist, DECISIONS 332), so a test never writes into a
  real repository;
- the roots inside the project (`pipeline_go`, `features`) are not redirected: build tests
  already work on a copy of the project.

`project.canon` itself is not changed, and the harness's copy leaves out any
`project.local.canon`. The override is a harness option: `canon.Options{Roots: map[string]string{"resource": "_fixtures/resource", …}}`
in the Go API, and `--root resource=_fixtures/resource` (repeatable) on the command line:

```
mkdir -p "$TMP"/{source,services,sovcommon,web,parity,generated}
canon check --project examples \
  --root resource=_fixtures/resource --root client=_fixtures/client \
  --root source=$TMP/source --root services=$TMP/services --root sovcommon=$TMP/sovcommon \
  --root web=$TMP/web --root parity=$TMP/parity --root generated=$TMP/generated
```

Relative paths are relative to `examples/`. A root the project does not declare is a usage
error (exit 2). Each example's `expected/findings.txt` lists the findings `canon check`
produces under this setup.

`canon check` and `canon test` write nothing, so they may point at `_fixtures/` directly.
`canon build` may write under a fixture root too (game.items emits
`@resource/Server/Item/items.json`), so build tests first copy `_fixtures/` to a temporary
directory and point the roots at the copy.

## Layout

`_fixtures/<root>/` mirrors the root: `@resource/Server/Item/propItem.json` is
`_fixtures/resource/Server/Item/propItem.json`.

| Fixture | Read by |
|---|---|
| `resource/Server/Define/define*.h` | every `load.defines` (vocab, farm, events, rules, balance, game.items, features) |
| `resource/Server/Item/propItem.json` | resource.vocab (`items`), balance.parity (weapons) |
| `resource/Icon/Item/*.dds` | the `ItemIcon` asset root: one placeholder per icon the rows above name |
| `resource/Server/Text/serverNames.json` | resource.vocab and game.items (`name()`) |
| `resource/Server/System/farm_config.json` | resource.farm |
| `resource/Server/Event/EventConfig.json` | resource.events |
| `resource/Server/System/heistia_config.json` | resource.heistia |
| `resource/Server/Quest/adventure_quest_config.json` | resource.adventurequest |
| `resource/Server/System/{GuildTalentTree,JobChangeV4,tower_config}.json`, `resource/Server/Skill/skills.json` | resource.rules |
| `resource/Server/Skill/skills.json`, `resource/Server/Npc/etc.json`, `resource/Server/Balance/{presets,targets}.json`, `client/Client/Text/etc.json` | balance.parity |
| `resource/Server/resource.txt` | features.text (`load.text`) |

Files an example reads by a relative path (`pipeline/data/`, `features/*/data/`) live next to
the example and need no fixture. So does the hand-written C++ header a legacy-struct example
compiles against: `features/legacycpp/ProjectCmn.h`, a minimal `struct ItemProp` with the five
members `legacycpp.canon` maps (`dwID`, `dwItemKind1`, `dwItemLV`, `dwPackMax`, `bPermanence`)
and one it does not (`szIcon`), used by the M6 test of the three access modes
(IMPLEMENTATION-PLAN.md M6).

## Rules for fixtures

- **Real shapes.** Rows are copied from the real files (Resource at 376dd9e6) and trimmed; key
  names, value spellings (`"="` for "not set", `0`/`1` booleans, tabs in `#define` lines) and the
  header guards of define files are kept, because the loaders must handle them.
- **UTF-8, `\n`.** Real define headers contain CP949 bytes in comments; the fixtures do not.
- **Icons are placeholders.** Each `.dds` holds the 4 bytes `DDS `. Only existence is checked
  (SPEC §16.5), and the name matches the data exactly, letter case included (DECISIONS 19).
- **Clean except on purpose.** A fixture produces no error. The warnings it produces mirror
  findings the real data has, and each one is listed in the example's `expected/findings.txt`:
  - farm: the Wheat model declares `maxLevel` 3 with 2 levels (`unreachable_levels`; six real
    models do the same);
  - heistia: two tasks share the description "Drop moonstone" (the real file does).
- **Small.** Add only what a new `load` needs. When an example starts reading a new path, add
  the smallest file that type-checks, then add the rows a check needs.

## What the real data adds

Checked against the real repositories, the same examples report more. The largest: 1,891 of
the 6,944 items name an icon that does not exist with that exact name (1,649 of them exist with
another letter case, like `itm_WeaSwoTurtle.dds` for `Itm_WeaSwoTurtle.DDS`). Each is `E3701`
(DECISIONS 19); the fix is a one-off rename script, not a language rule.
