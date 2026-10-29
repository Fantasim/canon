# Sovereign -> Canon: generated enum members that collide with defines Canon renders

**From:** Sovereign Source (Louis, ADR L-0138 tier 2, 2026-10-29). **Canon version:** 0.1.12.

## Why

Sovereign is replacing the rendered `define*.h` headers with Canon's own C++ (`@cpp(defines:)`
files and the generated id enums). In the same translation unit, a generated `enum class` member
and a `#define` of the same name collide: the preprocessor rewrites the enumerator
(`enum class BadgeKind { 5, 6 }`) and the compile fails.

Canon already guards the common case. The v0.1.10 reply
([2026-10-20-canon-reply-v0110.md](2026-10-20-canon-reply-v0110.md), DECISIONS 389-391) says
`push_macro` wraps "every identifier a defines file of the package, or of a package it reads,
defines", and the first reply
([2026-10-20-canon-reply-cpp-gaps.md](2026-10-20-canon-reply-cpp-gaps.md) §1) promised that a TU
may include a defines header before or after the matching `.gen.h`. That promise stops at the
package boundary and at Canon's own `@cpp(defines:)` output. Sovereign hits two cases outside it.
Both are worked around by hand today; the workarounds are the cost this ask removes.

## Case 1: a defines file of a package the enum's package does not read

`Engine/Resource/Generated/progression/quests.gen.h` declares (line ~1774):

```cpp
enum class BadgeKind : uint8_t {
    BADGE_REQ_USE_ITEM,
    BADGE_REQ_FAERY_RANK,
};
```

with no `push_macro` for either name. `Engine/Resource/Generated/items/badge.defines.gen.h`
(another package, which `progression.quests` does not read) holds:

```cpp
#define BADGE_REQ_USE_ITEM 5
#define BADGE_REQ_FAERY_RANK 6
```

A TU that includes the defines header first and `quests.gen.h` second gets
`enum class BadgeKind { 5, 6 }`. The "package it reads" rule does not cover this: no read edge
exists, yet both headers land in one TU. Sovereign worked around it by include order (Source
`f361a094f`), which breaks again the first time a TU includes them the other way round.

## Case 2: a header Canon renders as text, not as a defines file

`Engine/Resource/Generated/ui/ui.gen.h` declares `enum class WindowId : uint16_t` (1,051 `APP_*`
members, from line ~468) and `enum class ControlId` (~212 `WIDC_*` members, from line ~3681).
Resource also renders `Server/UI/WindowIds.h` through an `emit text`: plain C `#define`s of the
same `APP_*` / `WIDC_*` names. Neuz's PCH includes that header, so every PCH TU that includes
`ui.gen.h` fails. `ui.gen.h` already wraps its `WBS_*` constants (97 `push_macro` lines) but not
these enum members, because `WindowIds.h` is not an `@cpp(defines:)` output and Canon does not
treat it as a defines file.

Sovereign worked around it by splitting the theme records out of `ui` into a new package,
`ui.theme` (Resource `14f56ff8`, Source `9b3af8a9d`), so that the TUs which need the records do
not include `ui.gen.h`. That removes the include for those TUs only; any other TU that wants
`WindowId` still cannot be a PCH TU.

## Ask

In every C++ mode, any enum member or constant name that a generated header declares is wrapped
in `#pragma push_macro` / `#undef` ... `#pragma pop_macro` whenever **any** define of that name
exists in the project, from either source:

1. any package's `@cpp(defines:)` output, whether or not the declaring package reads that
   package (Case 1); and
2. any C header that a Canon `emit text` renders, such as `Server/UI/WindowIds.h`, with the
   defined names taken from its `#define` lines (Case 2).

If (2) is not detectable from the rendered text, accept a documented alternative that gives the
same result: for example an opt-in annotation on the `emit text` ("this file defines these
enums' names"), or an opt-in prefix for the enum members in C++ only (`@cpp(name:)` applied by
rule). Canon chooses the spelling. The one constraint is that no hand-maintained list of names
sits on Sovereign's side, since the 1,263 `APP_*` / `WIDC_*` names change with every UI edit.

The existing one rule (a TU where the macro is live cannot spell `Enum::NAME`) stays as
documented in the first reply.

## Acceptance

1. A TU that includes **every** rendered defines header (every package's `*.defines.gen.h` and
   every text-emitted C header, `WindowIds.h` included), then **every** `*.gen.h`, compiles, with
   `-std=c++20 -Wall -Wextra -Werror`. The reverse order compiles too.
2. A Canon test proves it for both cases: (a) two packages with no read edge, one with a
   `@cpp(defines:)` file and one with an enum of the same member names; (b) a package whose enum
   members share names with a C header rendered by an `emit text`.
3. Sovereign's own check: `Engine/Resource/Generated/progression/quests.gen.h` and
   `Engine/Resource/Generated/ui/ui.gen.h` compile when included after
   `items/badge.defines.gen.h` and `Server/UI/WindowIds.h` in one TU, with no include-order rule.

## Paths (from the Source repo root)

- `Engine/Resource/Generated/progression/quests.gen.h` (`BadgeKind`, ~:1774)
- `Engine/Resource/Generated/items/badge.defines.gen.h`
- `Engine/Resource/Generated/ui/ui.gen.h` (`WindowId` ~:468, `ControlId` ~:3681)
- `../Resource/Server/UI/WindowIds.h` (rendered, never hand-edited)
- Source workaround commits: `f361a094f` (include order), `9b3af8a9d` (ui.theme split);
  Resource `14f56ff8`.
