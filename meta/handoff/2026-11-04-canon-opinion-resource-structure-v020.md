# Resource `Config/` on canon 0.2.0: Canon's opinion on a cleaner structure

2026-11-04, canon 0.2.0, Resource `test` at `d2f893f9`.

**To:** the agent that owns Resource `Config/` (the Source session, by Resource CLAUDE.md's write table).
Louis carries it; paste it as is.
**From:** the Canon project, after a read-only survey of `Config/` (three passes: what became of the
2026-10-12 plan, a feature census, the package and import map).
**Status: an opinion, not a request.** Nothing here is ordered. Each item says what Canon sees, what
it would do in Canon's own idiom, what that buys, and what it costs. Judge each against L-0147, the
2026-10-30 organization review and your own constraints, and drop what does not hold. Canon has no
stake beyond the language being used where it is strongest.
**Paths:** from `Resource/Config/` unless marked. Counts are grep-level unless marked "measured".

## Where this sits

- **The 2026-10-12 plan** (`services/configlang/meta/handoff/2026-10-12-canon-to-sovereign-resource-structure.md`)
  was never cited on the Sovereign side. L-0147 and the 2026-10-30 review (Source
  `meta/resource-canon/organization-review-2026-10-30.md`) took the work over, and they did it well.
  Done since: clean field names with `@json` keeping the wire (D1, wave N), `ItemPropFlag` bits (D3),
  the converter headers (S7), one `Tid`/`tidCode`, one `LiveMover`, the `skills`/`skillsData` split,
  authoring notes out of rendered files (D7, one `_comment` left at `ai/mover.canon:21`). This file
  replaces that plan as Canon's view. The items still open are folded in below.
- **Canon agrees with the review's verdict:** "well organized; a cleanup is worth doing, not needed".
  It also agrees with the review's "keep as is" list. Locked id enums apart from their rows stay right
  until Canon ships one declaration per id (N2, planned for v0.5). The checkups idiom, the thin
  L-0138 packages and the two feature Switches stay too.
- What follows is the language's side: places where `Config/` writes by hand something Canon now
  knows how to state, and why Canon thinks the stated form is better.

## The one point of timing

L-0147 moves the game from copy structs onto the generated records (waves C, then E for the props).
After a table moves, its Canon shape is Source's API: a field's type is a C++ member type, and a
variant is a C++ type. Wave N saw this ("before E, so the props' read sites are written once against
the final names") and applied it to names.

**Canon's opinion:** the same rule holds for types, with more force. Each reshape below is free for
Source while the table still has a copy struct, and is a Source refactor after it moves. So the cheapest
moment for a table's type pass is inside its own C or E batch, just before its read sites move.
- For ItemProp, MoverProp, the skill rows and CtrlProp, the window is open until wave E.
- For the tables already moved (C1-C4), a reshape now costs read-site edits. Weigh it per field.

## 1. A rule as a type, not a check (the largest lever)

Resource's own rule 1 says "at the smallest unit that holds it (a type, a refinement, a check)". The
census says about two thirds of the checks are type constraints written as code. A sample of 30 `fail`
calls found 11 uniqueness, 9 existence and 4 range or length checks, and only 6 genuine cross-field
rules.

**Why a type is the "more optimized" way, seen from the compiler:**
- A check exists only at `canon check`, as a message.
- A type travels:
  - every generated decoder refuses a value outside it, in Go, C++ and TS (DECISIONS 399);
  - each bound becomes a generated constant (400; C++ `T::k<F><Path><Min|Max|End>`);
  - `canon edit` refuses a bad value at the field with its path;
  - config-studio gets a picker or a range;
  - `canon refs`, `canon graph` and `canon rename` follow a `ref` but cannot follow a string a check
    tests.
- The same guarantee costs less to evaluate, and the check, its message and its test go away.

| Today | Canon's way | Sites | Note |
|---|---|---|---|
| `duplicateInts` (`engine/numbers.canon:57`), `var seen: {Int: …}` loops (`model/modelTable.canon:52,560`, `model/motion.canon:68`) | `keyed by <field>` where the field is the identity; `where it.isUnique()` / `[…].isUnique()` where it is not; `xs.duplicates()` (built in since 0.1.6) where the message must name the repeats | 50 call sites, about 98 "twice/unique" messages | `keyed by` changes identity: edit paths and refs name the key. Use it only where the field is the key. |
| `hasItemRow(x)` (`items/rows.canon:10`), `x in propMover`, "has no … row" checks | a `ref items` / `ref propMover` field, or the existing `LiveItem`/`LiveMover` aliases | 43 + 17, about 44 "unknown/has no" messages | A ref's wire form is the key, so rendered bytes stay identical. Check the generated member type first: if it equals today's id type, Source sees no change. |
| `if t.name.len() > 64 { fail(…) }` (`progression/vocab/timeFrame.canon:29`) and similar | `String(..=64)`, `[T](..=N)` | about 61 | Also gives the C++ side a length constant, which wave D wants. |
| `…Ms` / `…Sec` / `…Min` fields as integers (`events/wheelOfFortune.canon:27,179`, `events/eventConfig.canon:156`, `progression/cardTitle.canon:113`); `type DurationMs = UInt32Val` (`content/refs.canon:11`) | `Duration(lo..=hi) @json(unit: s)` (or `ms`, `m`, `h`, `d`) | 68 Ms, 42 Sec, 19 Min/Hours; 18 `Duration` fields today, 1 `unit:` | The JSON stays the same number. Literals read `30s`, not `30000`, and a seconds field can no longer be fed milliseconds. The generated member type changes: do it per table, before it moves. |
| Two `Percent` types of different meaning (`character/system/vip/vipConfig.canon:12` `Int(0..=1_000)`, `ai/drop/monsterDrops.canon:19` `Float(0..=100)`) | two names, e.g. `PerMille` and `Percent`, declared once in `engine` | 2 types, 26 `…Pct` fields | Naming only, and the cheapest item here. |
| String unions of enum names (`items/item.canon:34-60` `LinkKind`, `WeaponType`, `ReferStat`); `dungeonType: String @json("type")` (`balance/targets.canon:1005,1042`) | the enum, or a dependent type on the field that decides it (`kind3` alone decides each union in the data, measured 2026-10-12) | 2,481 values | The cross-root trap that blocked `dungeonType` was fixed in 0.1.6 (DECISIONS 347). |
| `parent: Int(-128..=127)`, "-1 by convention", with hand root/self checks (`guild/talentTree.canon:18,28,64,78`) | `parent: ref talentNodes? @json(none: -1)`; `cycles()` is already used at `:88` | 1 record | The data uses -1 for roots, so stricter is free (rulings #10 and #12). |
| `Int(0..=1)` flags (`ai/monsters/bossMechanics.canon:50`, `ai/moverEx.canon:71`, `pet/pet.canon:54`) | `Bool @json("…", int)` | 3 | Small. Listed so the set is complete. |

**What Canon would leave as checks:**
- uniqueness over composite string keys (`progression/activitypoints/activityPoints.canon:48-53`):
  Canon has no composite key, so `duplicates()` over the joined key is the right tool;
- genuine cross-field rules.

## 2. A kind tag beside optional fields is a variant

`events/eventConfig.canon:136-150` is the clearest case. It has `kind: EventKind @json("type")` and
optional fields documented as "required for spawn_monster" or "required except on
monster_drop_inject", and checks enforce those docs.

**Canon's way:** a variant with `@json(tag: "type")`, used through a field marked `@json(inline)`
(`canon guide wire`, "Inline variants"). The tag and the case fields stay in the parent object, so the
JSON keeps its bytes.
- Each "required for X" becomes structure: a case holds exactly its fields.
- The checks go.
- `match` over the case is exhaustive in Canon, in C++ (`-Wswitch`) and in TS.
- Canon's own `examples/resource/events/event.canon` models this file.

**Where else:**
- grep the docs for "required for", "only on" and "absent unless": each hit is a case field written as
  an optional;
- the props are the same pattern at scale (ItemProp's per-kind blocks, the 10-12 plan's Step 3.1).

**Cost and timing:** a variant is a different C++ type, so this is a reshape for §0's window. For
eventConfig it is worth it whenever its batch comes. For the props it is the largest reshape of the
campaign. Canon thinks the variant model is the end state, but it is a judgement on L-0147's scope and
not Canon's to make.

## 3. `…Problems` fns become `expect package` (cheap, no rendered change)

**Today:**
- 73 `…Problems` fns;
- 67 `for p in xProblems(..) { fail(x, p) }` loops;
- 3 `expect package` uses (`items/clientui/wikiTests.canon:60`, `ai/npc/tests.canon:215`).

The fns exist because package checks ran in no test before 0.2.0.

**Canon's way:** the recipe in the v0.2.0 reply (`2026-11-02-canon-reply-v020.md`, "Recipe for
Resource's …Problems workarounds"):
- keep the check where it is;
- test it through a substituted world: `expect package { rows: X } fails "…"`.

On a copy, four rewritten tests passed, and they failed when the check they guard was removed. That
is the point: today a test of `xProblems` stays green if someone deletes the loop that calls it.
- **Keep a fn** where it is a real derivation other code reads.
- **Inline it into the check** where it exists only to make the check testable.

This changes no rendered file and no generated file, so it is a good first move.

## 4. Wave D is unblocked on Canon's side

L-0147 parks wave D until Canon ships "checked widths, list lengths, exported bounds" (the 2026-10-30
ask). canon 0.2.0 shipped two of the three:
- **Decoders refuse a value outside its refinement** (399). The ~310 narrowing guards repeat what the
  decoder now proves.
- **Each finite bound is a generated constant** (400). A hand cap can `static_assert` against it, as
  `NMUpgradeConfig.h:54-55` already does by hand. That turns the review's item 7 into a general rule.

**What Canon will not do:** infer a width from a refinement (GEN-02). The remaining enabler is on
Resource's side: declare sized types (`Int32`, `UInt16`, …) where the C++ member must be narrow.
- The generated headers held 830 `int64_t` members at the 0.1.13 count.
- That sweep changes member types, so §0's timing applies.

Wave D stays Louis's call ("not urgent"). This only says the block is gone on Canon's side.

## 5. Text: one record, one way to name a key

**Measured:**
- 16 records declare the same `{us, fr, gr, es?, pt?}` shape (`text/keys.canon:5` `TextRow`,
  `text/records.canon:8` `TextEntry`, `items/item.canon:88` `ItemText`, `SkillText`, `MotionText`, …).
- L-0150 (es/pt) had to add two fields to each of them. That is the cost of the duplication, already
  paid once.
- `fileTexts` is still 6 copies.
- `ui/clientui/textKeys.canon` still checks a hand-kept list of tables.
- `Skill.name` is a raw `"IDS_PROPSKILL_TXT_…"` string (`character/skillsData.canon:8`), while
  `Item.name` is already `ref itemTexts` (`items/item.canon:108`).
- 2,127 `"IDS_` literals.
- `TextKey` has 0 uses.

**Canon's way:**
- one `record GameText` in `text`, which every table uses;
- a `ref` wherever a field names a text row;
- one `fileTexts`;
- a cross-table key check derived from the tables, not from a list.

The next language column would then be one edit. A text key becomes a `ref` that `canon refs` and
`rename` follow. Rendered files stay byte-identical. `services.wiki`'s 3-language `Text` is wiki's
package; mention it to them, don't change it.

## 6. Write the layering down; split `engine`

**Measured:**
- 516 import edges and no cycles. Most edges follow header < `*.vocab` < data < client/text/services.
- `Config/README.md` states no layering rule.

The breaks that matter:
- **`engine`** (fan-in 46) imports 7 domain vocabs through `engine/eventParam.canon:6-12`, and
  `character.vocab` through `engine/numbers.canon:4`. Every package that imports `engine` for a number
  helper reads those vocabs.
- **`text`** (a helper, fan-in 22) imports `items.vocab.core` for `Dst` (`text/records.canon:3`).
- **Data packages import the 79k-line `text.client`:** `ai/npc/npc.canon:12`,
  `character/system/nmBuff.canon:8`, `guild/ranknames/rankNames.canon:4`,
  `progression/quests/title.canon:5`, and every service.

**Why Canon cares:** an import puts the imported package in the importer's read set.
- An edit there re-checks every importer, because per-entry dependencies are still owed on Canon's side.
- An edit takes the lock of every package whose read set its writes change (DECISIONS 417).

So `textClient` imported by data packages means a text edit locks and re-checks them, and a number
helper inside `engine` drags 7 vocabs into 46 packages. Splitting `engine` into a pure helper package
and an event-parameter package, and moving `Dst` below `text`, are both Canon-internal changes.

**Proposed rule for the README:** helpers < vocab < data < projections; imports go down. Cross-package
check-ups keep their named home. Canon's `canon lint` (v0.3, N3) does not check layering, so the
written rule is the only guard for now.

## 7. Entry layout: the template should describe the files

**Measured:** `items/item.canon:402` declares `@files("entries/{id}.canon")`, and 0 of 7,266 entry
files sit at that path. They sit in the legacy `<Job>/<Kind>/…` tree, at depth 5 (710) and 6 (6,556).
The comment says folders carry no meaning, and new entries land flat. The three telemetry templates
match their files 125/125, 125/125 and 126/126.

**Why it matters:** v0.3's `canon lint` will report entry files off their `@files` path (DECISIONS 440;
the ADR's estimate is 7,266 of 7,266). Two layouts in one table also confuse agents about where to
write.

**Two ways out, Canon's preference first:**
- **(a) A template that matches the existing folders.** `@files` may name dependent fields (DECISIONS
  280). If the job and kind segments derive from typed fields, this moves zero files and keeps the tree
  people browse. The 10-12 survey found 0 mismatches over `dwItemJob`/`dwItemKind1`. The uneven depth
  says to check that the segments really derive before choosing this.
- **(b) One move to flat**, between two Zirk merges (`-X find-renames`).

The current mix is the one state Canon would not keep.

## 8. Small, concrete, and still open

- **B3, a real bug.** `services/csn/cpp.canon:39-45` (`str`, `chars`) writes C string literals with
  no escaping. A `"` or `\` in the data breaks the generated C.
- **Three hex writers** (`engine/vocab/headers.canon:33` `hex8`, `engine/server/packManifest.canon:38`
  `hexByte`, `services/csn/cpp.canon:23` `hex`) become format specs: `"{n:08X}"`, `"{b:02x}"`
  (DECISIONS 358, `canon guide syntax`).
- **B4, checks that can never fire:**
  - `character/skills.canon:347-349`: `skills` is `keyed by define`;
  - `items/item.canon:435-441` (the `seen` code map): `@codes` values are unique already;
  - `character/system/vip/vipConfig.canon:146`: map keys are unique already.
- **B5, stale README text.**
  - `Config/README.md:58` says `test` peaks at ~7.7 GB, while CLAUDE.md rule 6 says ~1.7 GB.
  - `:133` says `*/client` writes `.txt`.
  - `header/header.canon:3` says "no emit".
- **The converters.** Source `meta/resource-canon/converters/` keeps 7 files whose README says
  "re-runnable until W6b", with no guard.
- **Same names in several packages:** `EventType` (`events.vocab`, `telemetry`), `Header`, `Tier`,
  `AzureBonus`, `PresetGear`, `CardSet`, `SetItem`, `Teleport`, `Block`. Each is legal in Canon, but
  each is a wrong-import trap for an agent and a clash on a consumer that imports both.

## 9. What Canon would wait for

- **Go package names** (58 of 84 differ from the Canon path; `itemsupgrade` is used twice, `balance`
  twice). v0.3's output templates (DECISIONS 439, N8) give each root one layout. Rename once then, not
  twice.
- **The id/row twin** (`ItemId` beside `items`, `hasItemRow`). N2, one declaration per id, is planned
  for v0.5. Until then the locked enum beside the table is the right shape, as the review says.
- **Splitting the studio-edited monoliths** (`model/mdlDynaData.canon` 83.8k lines,
  `text/client/textClient.canon` 79.2k lines). The review says leave them, and Canon agrees for now:
  the cost is Canon's whole-file format judgement at edit time, which is Canon's to fix.
- **`items/texts.canon`, the exception (113.9k lines, doubled by es/pt).** Here Canon's opinion is that
  the text belongs in the item's own entry file, a projection fn writes the client text file, and the
  `itemTexts` table goes. That is a reshape of the props (§0) and depends on §5's single record first.

## If Canon ordered it (it does not)

| Order | Items | Rendered or generated change |
|---|---|---|
| 1 | §3 and §8 | none, except B3's fix |
| 2 | §5 and §6 | none |
| 3 | §1's naming items (`Percent`) and the `ref`s whose generated type is unchanged | none |
| 4 | the rest of §1, §2 and §4's sized types, per table inside its own C or E batch | generated member types change |
| 5 | §7 | file moves, if (b) |

## Not verified

- No `canon check`, `build` or `test` was run on Resource for this note. The counts come from grep and
  file reads.
- The C++ member type that a `ref items` field generates, against today's id type, was not probed.
- Whether the item folder segments derive from typed fields (§7a) was not re-checked since 2026-10-12.
