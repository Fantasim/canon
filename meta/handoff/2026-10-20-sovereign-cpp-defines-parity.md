# Sovereign -> Canon: `@cpp(defines:)` parity gaps (follow-up to 2026-10-20-canon-reply-cpp-gaps.md)

**From:** Sovereign Source (Louis, ADR L-0138 wave 0b). **Canon version:** 0.1.9.

## Why

We applied the reply's §1 to all 11 Sovereign vocab packages: `emit cpp { mode: baked }` plus
`@cpp(defines: "<PREFIX>")` on every id enum. Resource branch `louis/canon-w0b`, gate green.

The Source switch from the rendered `define*.h` to `*.defines.gen.h` needs the **same set of names**
in the generated file. The set of NAME/VALUE pairs matches for only 2 of 11 headers (`defineObj.h`,
`defineBadge.h`). No value ever differs; the gaps are names the C++ emit cannot write. A Sovereign
parity script compares the two sets.

## Asks

1. **An identity mode for enums with no common prefix.**
   - *Affected:* `ItemGrade`, `GuideEvent`, `BossDelay`, `SoundId` (SND_/VOC_, 2,299 names) and
     `Continent` have members that share no single prefix.
   - *Bug found:* on 0.1.9, `@cpp(defines: "")` writes **nothing**, with no finding. Per the reply's
     rule ("a name already starting with the prefix keeps it"), it should write every member
     unchanged.
   - *Wanted:* the empty prefix writes every member as is, or add an explicit `@cpp(defines)` with
     no argument.
2. **Validate the prefix.** `@cpp(defines: "*")` is accepted and writes `#define *CONT_NODATA 0`.
   Refuse a prefix that is not a valid identifier start.
3. **Constants and table keys in the defines file.**
   - *What is missing:* the rendered headers also carry names that are not enum members:
     - package `const`s (`MAX_JOB`, `MAX_ADJPARAMARY`, `MAX_ITEM_KIND3`, `QS_END`,
       `WI_INSTANCE_LAST_ID`, `PERIN_VALUE 100000000L`…);
     - table keys (`engine.vocab` `defineConstants` / `LimitId::LIM_*` / `LANG_MAX`, plus the quest,
       world and neuz constants);
     - aliases (`DIS_MULTY`, `DIS_PSYCHIC`, `WI_DUNGEON_SECRET_L`, `MCD_MCD_FLARIS_NPC53..59`,
       `SND_NPC_KERNIDLE1`…);
     - one expression, `JOB_ALL MAX_JOB`.
   - *Wanted:* a way to put each of these in a package's `*.defines.gen.h`. For example,
     `@cpp(defines)` on a `const`, on a table, or on an alias, plus a defines entry that refers to
     another name.
4. **`push_macro` coverage.**
   - *Problem:* the reply's coexistence guarantee covers only members of `@cpp(defines:)` enums. A TU
     that includes the legacy header and the generated `.gen.h` fails in 6 of 7 packages tried
     (quest, items.core, character, engine, world, ui.neuz). The causes are table-key id enums
     (`ConstantId::MAX_AS`, `LimitId::LIM_*`), package `inline constexpr` consts (`MAX_JOB`…) and the
     no-prefix enums.
   - *Wanted:* wrap **every** generated identifier that a defines file (or a legacy header) also
     names.
5. *(minor)* `withRetired`: the generated defines file includes retired members, while the text emit
   renders with `withRetired: false`, giving +62 `II_` and +140 `TID_` names. This is harmless for
   C++. An option to match the text emit would make the parity diff exact.

6. **Whole-project check/build time after the emits.** Once the 11 vocab packages emit baked
   C++ and 31 data packages emit `types` (branch `louis/canon-w0b`), a whole-project `canon check`
   rose from about 12 s to about 408 s, and `canon build` takes about 8m17s. The Resource gate now
   runs for about 40 minutes. Each package checked alone takes 11 s or less, so the time is spent
   at whole-project level.
7. **Ask 2 of the first handoff (out-of-line lookups) now blocks the first real loader.**
   `engine.gen.cpp` (package `engine`, 456/308 lines) includes the big vocab headers. It compiles
   in seconds at -O0, but at -O2 it was still running after more than 10 minutes. Please keep this
   at the top of v0.1.10.

## Accept when

For each of the 11 packages, the set of NAME/VALUE pairs in `*.defines.gen.h` equals the rendered
header's set, with hex vs decimal spellings counting as equal. And a TU including the legacy header
plus the package's `.gen.h` compiles.

## Paths

- **Sovereign ADR:** `Source/meta/decisions/L-0138-canon-generated-cpp-structures-and-loaders.md`.
- **Resource:** branch `louis/canon-w0b` (worktree `worktrees/canon-w0b/Resource`). The commits are
  `4f9abce5..bf563abe`, one per vocab package.
