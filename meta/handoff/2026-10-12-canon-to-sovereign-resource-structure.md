# Resource's Canon structure: a cleanup plan in steps

2026-10-12, canon 0.1.5.

**To:** the Sovereign `Source` session, which writes the Canon packages under Resource `Config/`. Louis carries this to it; paste it as is.
**From:** the Canon project, after a read-only review of Resource `Config/` at the HEAD of 2026-10-12.
**Paths:** from the Resource root, or from the Source root where marked "Source".
**Audit ids:**
- `DA-x`/`DB-x` are Source `meta/resource-canon/audit/review-themes-a.md` and `-b.md`;
- `HW` is `…/audit/hwave-plan.md`;
- `ND H-n` is `…/night-decisions-2026-10-08.md`;
- `L119` is Source ADR L-0119.

**Companion:** Canon's plan, `services/configlang/meta/handoff/2026-10-12-canon-next-versions.md`. Items `F`/`N` below refer to it.

## Ground rule, and what is missing

Phase 1 is accepted as deliberate (L119:126, "1:1 with today's structure"). The following stay until a later step, because changing them now would force a Source refactor:
- ItemProp, MoverProp and skill rows with Hungarian wire names;
- numbered columns and `"="` sentinels;
- the legacy file shapes, define headers and CRLF files.

**No phase-2 plan exists.** It has no ADR, no owner and no order; it is defined only by L119 d.2 and d.4 plus deferral lists (DA "PHASE-2 queue", HW "Not done in wave H"). **Ask:** write phase 2 as an ADR (the next free id is L-0125), with an owner and an order. This handoff is its input.

Steps 0 to 2 change no Source line. The acceptance of each is the full gate:

```sh
canon fmt --check && canon lock check && canon check --max-warnings 0 \
  && canon test && canon build --check --max-warnings 0
```

The gate must stay green, and outputs must stay byte-identical except where a step says "loader-equal" (the L119 d.2 proof).

Work on the branch where the Canon expansion runs (`test`, 51 commits ahead of `main` at the time of writing), one family at a time.

## Constraints this plan respects

- **Locked ids never move or change** (Resource CLAUDE.md rule 4). A theme restructure was rejected for its 33,537 lock lines (L-0123 d.1). Prefer changes that touch no locked enum.
- **Go import paths in the services mirror Canon package paths.** Moving a package changes admin, website and monitoring imports, which each need a `make gen`.
- **Data writers address packages by name:** balance `canon_edit.py`, DevTools, Source `Tools/traininggrounds`. Moving data between packages means updating them in the same change.
- **Zirk's `dev` is ported by hand at each merge.** Large file moves need `-X find-renames` (precedent: L-0123 U11 moved 7,203 files).
- **Stricter than the loader is allowed** (Louis's rulings 2026-10-09 #10 and #12). A C++ change needs a `Config/Sys_*.h` flag or the d.7 structural exemption (Source CLAUDE.md).
- **`canon rename` also edits views,** and `Config/**/studio.canon` is config-studio's write scope. Tell config-studio before a rename wave.

## Step 0: bugs and one hazard (do first)

| # | Where | What | Fix | Tracked? |
|---|---|---|---|---|
| B1 | `Config/ui/clientui/texts.canon`, the treeHelp rows (323, starting at the first, `IDS_CLIENT_TREEHELP_INC_000000`) | German text in the `us` column, English in `gr` | Compare with the pre-port source to tell a converter bug from old data. Swap the columns, then add a test | no |
| B2 | `Config/progression/tutorial/file.canon:10` | The hand-written JSON escaper escapes only `\` and `"`. A line break or tab in a text writes invalid JSON | Escape `\n`, `\t` and control characters | no |
| B3 | `Config/services/csn/cpp.canon:39` (`str`, `chars`) | C string literals written without any escaping | Escape `\` and `"` | no |
| B4 | `Config/character/skills.canon:20621-20630`, `Config/items/item.canon:410-413`, `Config/character/system/vipConfig.canon:148` | Checks that can never fire: `skills` is already `keyed by define`; `@codes` values are already unique; map keys are already unique | Delete them, and the tests that only expect them | no |
| B5 | `Config/README.md`, `Config/header/header.canon:2-3` | Stale text: | Update the text | no |
| | | the memory figures predate the 2026-10-09 performance pass (the README says ~7.7 GB for `test`; the hook says ~2.0 GB); | | |
| | | "`*/client` writes `.txt`" (they write `.json`); | | |
| | | "no emit" (there is one) | | |
| H1 | Source `meta/resource-canon/converters/` (39 files, 13,385 lines) | **A re-run silently reverts Canon-side edits** (HW Fact 4). Zirk's dev29/dev30 merges were ported by hand into converter-owned files: Resource `1bc42fc3` changed 1,258 lines of `progression/achievementData.canon` under a GENERATED header. The converters also still write stale paths (`propItem.py:429`: `@files("Items/{id}.canon")`), and their legacy JSON inputs no longer exist | Freeze them now: a guard that refuses to run, and a README line "history, phase 1 only". Delete them family by family in phase 2. Then strip the headers (S7) | partly (converters/README "history") |

**Not a bug** (checked): all 311 items without an icon are female fashion boxes (307 `II_SYS_SYS_SCR_BXF*` and 4 others). The `Item_FashionBoxFemale.dds` default fits all of them. The default encoding one family is the real issue, handled by D6 (DA-E3).

## Step 1: structure (outputs byte-identical)

### S1. Layering: who may import whom

**Tracked:** only side effects (HW Facts 6, ND H-26); no item on layering.

**Problem.**
- The shared helper `engine` imports 7 domain vocab packages, so importing it for `duplicates` loads `ItemId`, `MoverId` and more.
- Text tables sit downstream of the data that names them. `character.client` imports `character`, so `Skill.name` (`Config/character/skills.canon:50`) holds a raw `"IDS_PROPSKILL_TXT_…"` string instead of a `ref` (DA-C2, config-studio R3). Across the project, 2,754 text keys are plain strings.

**Do.**
1. Write the rule in `Config/README.md`: pure helpers < `.vocab` (ids) < theme data < `client`/`clientui` projections. Imports only go down.
   - Cross-package "check-ups" stay as they are; HW Idioms 2 and DA-A1c accepted that idiom. Give them a named home, e.g. `<theme>/checkups.canon` in the package that may import both.
2. Split `engine` into a pure helper package and a domain one (the event-parameter resolver).
3. Move a text table next to, or below, the data that names it, so the field becomes a `ref`. Text tables are not locked, so this moves no lock line.

**Acceptance.**
- `^import` lines follow the rule.
- Every text field that points at a text table is a `ref`.

Canon N8 will later check the rule itself.

### S2. Text: one record, one way to reference it

**Tracked:**
- the row shape: DA-A2, DB-D6 (partial);
- the reference styles: DA-B3c, DA-C2, DB-L1, R3, never as one design;
- the hand-kept list: ND H-48, which made the list the fix; the debt itself is not tracked.

**Problem.**
- 15 main text tables (21 counting the small ones).
- The `{us, fr, gr}` record is declared 16 times: `TextRow` in `Config/text/keys.canon:5` is the same record as `ItemText` in `Config/items/item.canon:75`.
- Five reference spellings: `ref itemTexts`, `ref moverTexts?`, a raw String, `IdsKey`/`TextKey`, and the `TextId` enum.
- `type TextKey` (`Config/text/records.canon:20`) is used 0 times.
- `fileTexts` is written 6 times and `tidCode` 3 times. The copy in `Config/progression/vocab/recordbook/boards.canon:121` lacks the `not m.retired` filter of the others.
- `Config/ui/clientui/textKeys.canon` checks the client's first-wins merge through a hand-kept list of 21 tables, so a table missing from the list goes unchecked.

**Do.**
1. One `record GameText` in `text`, which every table uses.
2. One reference spelling per kind of key: a `ref`.
3. One copy of each helper.
4. Derive the cross-table key check from the tables rather than from a list. If that is not expressible, ask Canon through a handoff.

Keep game texts as data. Canon translations cover tool texts only (I18N L2), so DB-L5's "translations are the native shape" does not apply to them.

**Acceptance.** Text outputs byte-identical. One `us/fr/gr` record and one copy of each helper.

### S3. Split the monolith literals

**Tracked:** DB-P4 (which suggests `@files("layout/{id}.canon")` for `layoutData`), DA-D3, and HW "big-file splits (low value)". There is no unit.

**Problem.**
- 19 files over 5k lines hold 57% of the project:
  - `model/mdlDynaData.canon`: 83k lines;
  - `items/texts.canon`: 67k;
  - `ui/layoutData.canon`: 60k;
  - `text/client/textClient.canon`: 36k;
  - `ai/propMover.canon` and `character/skills.canon`: 20k each.
- Each is one legacy file transcribed as one `let`, "every block and row, in file order".
- Every addition conflicts in the same file: textClient took 16 commits since 09-15, `telemetry/events.canon` 23. A review means reading a slice of a 60k-line file. And Canon's formatter judges a whole file, which holds the edit memory peak.

**Do.**
- Use the shape telemetry already has (its `@files` templates match 354 of 354 entry files): a `table` or keyed list with `@files`, plus an export fn that assembles the runtime file in the order the engine needs.
- Start with the most-edited files.
- Where the value is a record of lists, model each list as a table.

**Acceptance.** Outputs byte-identical. No hand-maintained file over about 5k lines.

### S4. Item entry layout

**Tracked:** DA-D4 was closed by a comment. Flat versus templated was never decided.

**Problem.**
- The 7,203 entry files sit in the legacy 149-folder tree, while `@files("entries/{id}.canon")` (`Config/items/item.canon:374`) declares a flat layout. The first new item will land flat.
- Entries load in path order. The runtime row order comes from `layout`, but check that nothing else depends on entry order.
- The folders agree with the data today (0 mismatches over `dwItemJob` and `dwItemKind1`), and nothing checks it.

**Do.** Decide one of:
- **(a)** a template over the typed fields the folders encode, then move the few files that do not match;
- **(b)** a flat layout.

Either is one move of 7,203 files (merges need `-X find-renames`). Do it once, between two Zirk merges.

**Acceptance.** Every entry file sits at its template path. Canon's `canon lint` (N3) will flag drift.

### S5. Types that 0.1.5 already has

About 400 of the roughly 930 hand-written rules could already be types.

| Where | Today | Instead | Tracked |
|---|---|---|---|
| `Config/content/refs.canon:16`, 156 time fields | `type DurationMs = Int(0..=UINT32_MAX)` | `Duration`. ms is the default wire unit; the `*Sec` fields need `@json(unit: s)` (wire names fixed 2026-09-19, Resource `meta/handoffs/0001`). Literals are rewritten, bytes identical | DA-B4 |
| `Config/character/system/vipConfig.canon:9`, `Config/ai/drop/monsterDrops.canon:12` | Two `Percent` types: `Int(0..=1_000)` and `Float(0.0..=100.0)` | Distinct names (`PerMille`, `Percent`) | DA-C4 |
| `Config/balance/targets.canon:1073,1101` | `dungeonType: String` plus a check | `DungeonType @json("type")`, once Canon fixes the cross-root import trap (F3) and admin gets an `emit go` on `content.vocab` (ND H-37: typing it today made admin's Go import the monitoring module) | DB-L1 |
| `Config/guild/talentTree.canon:18,57-95` | `parent: Int(-128..=127)`, "-1 by convention", with hand-written uniqueness and root checks | A keyed list with `parent: ref TalentNode? @json(none: -1)` and `cycles()`. Roots -2..-128 are loader-valid, so check that the data uses -1 only (stricter is allowed, rulings #10 and #12) | DB-L2, ND H-27 |
| `Config/events/eventConfig.canon:137-260` | `kind` plus 18 optional fields and 12 rules | A variant with per-kind fields (Canon's `examples/resource/events/event.canon` models this file) | no |
| `hasItemRow(x)` (about 30 sites), `x in propMover` (12), live-mover alias ×5 under 4 names, item references spelled 3 ways | Existence checks | `ref items`, or one shared alias. Drops and VIP use `ItemId` (config-studio R5) | DA-C1, DA-A1 |
| `repeatedSlot` ×7 (`Config/model/modelTable.canon:50`) and similar loops | Hand-written uniqueness | `keyed by` | DB-D2 |

Keep each rule id in the test that proves it, rather than in a check message.

**Acceptance.** Fewer checks, the same failing tests, outputs byte-identical.

### S6. One shared helper library

**Tracked:** DA-A6 is the nearest item for the hex writers. The escapers and `tidCode` are not tracked.

**Duplicates to merge:**
- two JSON escapers that disagree: `Config/telemetry/telemetry.canon:548` escapes newlines; `Config/progression/tutorial/file.canon:10` does not (B2);
- three hex writers: `hex8`, `hexByte`, and services.csn's `hex`;
- `tidCode` ×3, `fileTexts` ×6 (see S2);
- 17 sets built by hand as `{k: true for …}` (use `.isUnique()`, `keyed by` or a map).

**Do.**
- Gather them in the pure helper package of S1, one tested copy each.
- `duplicates` and `duplicateInts` are a Canon gap (no generics), not a mistake: Canon v0.1.6 adds a `duplicates()` builtin (F18), plus hex and width in format specs.

### S7. Honest headers

**Tracked:** not as work.

**Problem.**
- 103 hand-maintained sources start with "GENERATED by Source … converters … edit through canon edit" (e.g. `Config/ui/textsData.canon:3`). The Canon guide tells agents never to edit a generated file, and these files are now the source of truth.
- Some hand-built outputs start with an imitation of Canon's own `GENERATED by canon` marker. Source `meta/resource-canon/audit/security-gate.md` H8 is related.

**Do.**
1. Replace the converter header with a doc comment: "converted from X on <date>; this file is the source".
2. Give hand-built outputs a marker that is not Canon's.

Do both after H1, so no converter re-adds a header.

### S8. Naming conventions

Pick one name per role and record it in `Config/README.md`:
- `rules.canon` or `checkups.canon`;
- `*Tests.canon` or `tests.canon`;
- what a `.vocab` package holds (ids only; `progression.vocab.ranking` holds no enum);
- Go package names from the Canon package path (16 of 41 are ad hoc; `itemsupgrade` reads like the package `items.upgrade`).

## Step 2: phase-1 debt that touches no Source line

Do this before the services write more code against the generated getters. Admin already calls `row.DwIDID()` (`services/admin/internal/gamedata/catalogue.go:110`). Coordinate one `make gen` per service.

| # | What | How | Note | Tracked |
|---|---|---|---|---|
| D1 | Clean field names: `items.Item` (128 of 129 Hungarian), `ai.Mover` (43/45), `character.TroupeSkill` (112/115), `SkillLegacy` | `canon rename items:Item.dwCost cost` adds `@json("dwCost")`, so runtime files stay byte-identical | DECISIONS 3 asks this. DA-B8's premise ("before code emits") has lapsed. Generated Go and TS names change, and the studio labels can then mostly go | DA-B8 |
| D2 | 18 fields of 0/1 integers (about 18k values) | `Bool @json("bX", int)` | `model` already does this 35 times | no (badge only, DB-L2) |
| D3 | `dwFlag` | `[ItemFlag] @json("dwFlag", bits)` | 49 rows set bit 0x08, which `IP_TYPE` does not define (Source `ProjectCmn.h`). Decide what it means first | no |
| D4 | String unions of enum names (`LinkKind`, `WeaponType`, `ReferStat`, `Config/items/item.canon:34-60`) | A dependent type on `dwItemKind3`, which alone decides each union in the data | 2,481 values | no |
| D5 | Icons and other file names as asset types | `asset("@resource/Icon/Item", …)` | Of 6,892 icon values, 1,649 differ from the file only in case, and 289 live in other folders. Canon matches case exactly (DECISIONS 19): fix the data once | no |
| D6 | Required fields | Type each file's `defaults` block as its own record, then make `kind3`, `icon` and `name` required where the data allows | The `defaults` block stays in the runtime file, because the engine refuses a file without it (Source `ProjectCmn.cpp:488`). This ends "an item with no kind is a scroll" and 10,483 `field: none` lines | DA-E3 |
| D7 | Authoring notes in runtime JSON: `_comment`, `_note`, `_doc_*` (about 27 fields; 940 of the 961 `_comment` values are in propMover) | Move them into `///` docs | **Loader-equal**, not byte-identical | DA-B6 |

## Step 3: phase 2 proper (in the L-0125 ADR)

1. **Items as an authoring model plus a projection.**
   - A variant per kind (weapon, armor, ride with its flight block, blink wing with its target, …).
   - `bonuses: [StatBonus](..=6)`, a `skin: Bool` in place of `layout` nVer 500, and the texts inside the item.
   - An export fn writes today's `propItem.json` row.
   - With Canon N1 (legacy C++ structs), a generated loader fills the existing `ItemProp`, so Source call sites stay as they are.
2. **One declaration per id** (Canon N2). This removes the ItemId/items twin, `layout` as a separate table, and the `hasItemRow` checks.
3. **Movers as one `Monster` aggregate.** Today a mover spans 5 files keyed by `MoverId`: propMover, moverTexts, propMoverEx, the mdlDyna rows and the drops.
4. **The other families:**
   - skills with dependent slot types (`AdjArg(destParam1)`, `StateArg(destParam1)`);
   - ui resdata with per-kind flag enums and `Bool`;
   - model as keyed collections.
5. **Loaders accept Canon's forms where Canon refuses a legacy one:**
   - an absent key instead of an explicit `null`;
   - a tagged variant instead of an untagged choice;
   - no placeholder default for a key that must always be written.

   This retires the tutorial's hand-written JSON and troupe's 105 placeholder defaults. It follows DECISIONS 327 and TYPES §13.2 on Canon's side, and "Change Resource, never work around it" (L-0118 d.7) on Sovereign's.
6. **Retire the converters,** family by family (H1).

## Asked of Canon, already on its list

- F1, the `canon lsp` memory leak. The Scripts handoff of 2026-10-12 is not yet in Canon's mailbox; send it.
- F3, website H-14 #4 and the Resource ADR 0007 owed items.
- N1 legacy C++ structs.
- N2 one declaration per id.
- N3 `canon lint`.
- N4 the wider `pairs` and record `inline`; per-field `codes` comes earlier, in v0.1.6 (F20).
- N5 testable package checks.
- N7 native package moves, which retire most of `canon.lock.moves`.
