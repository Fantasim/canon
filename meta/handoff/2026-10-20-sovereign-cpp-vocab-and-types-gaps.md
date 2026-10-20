# Sovereign -> Canon: C++ emit gaps for "every package emits C++" (ADR L-0138 amendment 2026-10-20)

**From:** Sovereign Source (Louis). **To:** the Canon project. **Canon version probed:** 0.1.9.

## Why

Louis ruled on 2026-10-20 that every Sovereign Canon package emits C++. The id lists
(`items.vocab` `ItemId`, `text.vocab` `TextId`, `character.vocab`, `ai.vocab`, `world.vocab`,
`engine.vocab`...) are to become the C++ source of those ids, replacing the rendered
`Server/Define/define*.h` macro headers. The ~110 hand-written loaders then read typed ids straight
from the generated `T::Parse`.

A probe on a scratch copy of `Resource/Config` added `emit cpp` everywhere. All 11 vocab packages
pass `baked`, 35/50 runtime data packages pass `types`, and the other 26 packages pass `baked`.
A live `pet.json` parse resolves `II_PET_WHITETIGER01` to 3551. The items below are what blocks the
rest. They are listed in priority order; items 1 and 2 block ruling 6 outright.

## Asks

1. **A vocab C++ emit that can replace `define*.h`.**
   - *Problem:* the generated `enum class ItemId { II_WEA_HAN_HAND = ..., ... }` uses the macro
     names. Any TU that also includes `defineitem.h` fails to compile, because the macro expands
     inside the enum. Source has about 20K uses of these ids as plain integers, in about 1,000 files,
     plus 11 `#if`/`#ifdef` sites on define names.
   - *Wanted:* an emit option that generates the names as unscoped `inline constexpr` constants of
     the id type, or as an unscoped enum that converts implicitly to an integer. The generated header
     can then be included **instead of** `define*.h`. The typed `enum class` stays for data fields;
     the constants and the enum share the same numbers.
   - *Accept when:* a TU including only the generated headers compiles Source code written against
     `define*.h`. Exception: `#ifdef II_...`-style sites, which Source rewrites.
2. **Name lookups out of the header, and fast to compile.**
   - *Problem:* `ItemIdFromWire` and `TextIdFromWire` are inline chains of 7,306 and 11,351 string
     compares, in the header. One small generated `pet.gen.cpp` takes 137 s at -O2 (9 s at -O0), and
     a TU calling both takes 682 s.
   - *Wanted:* `ToName`/`ToWire`/`FromWire`/`FromCode` declared in the header and defined in the
     `.gen.cpp`, using a sorted array plus binary search, or a perfect hash.
   - *Accept when:* `items/vocab.gen.cpp` and `text/vocab.gen.cpp` each compile at -O2 in under 10 s,
     and a TU that uses them pays nothing extra.
3. **C++ member-name override.**
   - *Problem:* E8011 refuses `ai.vocab` `_OI_DEFAULT`, `items.vocab.core` `ElementDefine._NONE`,
     `_FIRE`, `_WATER`, `_ELECTRICITY`, `_WIND`, `_EARTH`, and `world.clientworld`
     `IDS_DUOMINOUS__WLD_000000`. `canon.lock` forbids renaming them (E6001), and they are spelled
     bare in the data.
   - *Wanted:* `@cpp("name")` on a member, or a generator mangling rule for reserved names.
4. **Maps in C++ `types` mode.**
   - *Problem:* E8019 refuses maps. That is 94 of the 110 failures, across `balance`, `ui`,
     `progression.quests`, `combat`, `character`, `items.upgrade`, `character.system`, `events`,
     `items.catalog`, `items.clientui`, `ui.servernames` and `world`. The suggested keyed list would
     change the rendered JSON shape.
   - *Wanted:* string-, enum-, ref- and int-keyed maps decoded into an ordered container, with the
     wire shape unchanged.
5. **Smaller `types` gaps:**
   - a field default whose value is a record or variant (`ai.monsters`, `content.modes`,
     `progression.tutorial`, `balance`);
   - a list of optional elements (`character/skills.canon:197 destData1`);
   - a literal union over a ref (`events/exchange.canon:16 itemId: ref items | "PENYA"`,
     `ui/layout.canon:53`);
   - a root `Parse` for files whose top level is a table or list (about 15 TextEntry/TextRow/
     SkillText/HoF files).
6. **W8006 should also cover `<cstdint>`/`<climits>` macro names.** Public constants named
   `INT32_MAX`, `UINT32_MAX` (`engine`) and `NAME_MAX` (`items`) currently break the generated `.cpp`
   with no warning.
7. *(nice to have)* One shared `canon_runtime*.h` instead of a copy per output directory.

## Paths

- **Sovereign ADR:** `Source/meta/decisions/L-0138-canon-generated-cpp-structures-and-loaders.md`
  ("Amendment 2026-10-20").
- **Census:** `Source/meta/canon-loaders-census.md`.
- **Pilot consumer:** `Source/Game/Economy/ActivityPoints/ActivityPointsConfig.cpp` (types mode).
