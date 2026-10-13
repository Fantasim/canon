# canon v0.1.6 and v0.1.7: what each repo does now (2026-10-13)

To: the Sovereign sessions `Scripts`, `Resource`, `Source`, `website` and `admin`. v0.1.6 is
published (https://github.com/Fantasim/canon/releases/tag/v0.1.6) and installed at
`~/.local/bin/canon` from the release (sha256 `56647b58…d336e5b9`, equal to the release
archive's binary). What changed: [the reply](2026-10-13-canon-reply-v016.md).

Everything below was measured, not guessed: v0.1.5 and v0.1.6 run on a copy of Resource HEAD
`43e46174` and of each service's HEAD, outputs diffed, services compiled. Nothing in any
Sovereign repo was written.

## Scripts: first, `canon-bump` 0.1.6

The installed binary is now 0.1.6, so the Scripts gate and Resource's pre-push hook (both pinned
to 0.1.5's sha256) refuse until the pins move. Run the `canon-bump` skill now. Measured on
Resource with both versions (one run each, 6 GB cap):

| Step | v0.1.5 | v0.1.6 |
|---|---:|---:|
| `canon check` | 17.1 s, 2.56 GB | 18.5 s, 2.66 GB |
| `canon build --check` | 22.1 s, 2.98 GB | 22.3 s, 2.95 GB |

## Resource

Required: nothing. `canon check` is 0 errors, 0 warnings in 89 packages on both versions. No
new finding (`E8152`, `E8004`, `W8030`, `E8019`) appears.

After the bump, one `canon build` rewrites the C++ runtime header in Source, 12 copies:
`Engine/Resource/Generated/<pkg>/canon_runtime.h` × 11 and `Engine/Telemetry/Generated/canon_runtime.h`.
This is v0.1.6's only change to Resource's committed outputs. `build --check` also lists stale
telemetry outputs on **both** versions (`session.gen.h`, `telemetry.gen.h`, `Telemetry*.gen.h`,
`Tools/telemetry/generated/*`). Those come from HEAD's telemetry commit not yet being built, not
from canon.

Optional cleanups that v0.1.6 makes possible:
- `engine/numbers.canon` `duplicates(keys)` (41 calls in 20 files) can become the built-in
  `keys.duplicates()`. The order differs: yours lists keys by **first repeat**
  (`[a, b, b, a]` gives `[b, a]`), the built-in by **first occurrence** (`[a, b]`). Only the
  order of listed duplicates in a message changes; re-run the `expect` tests that pin one.
- Hex helpers become format specs. Checked on 0.1.6: `"0x{n:02X}"` gives `0x0A`, and `{0:X}` gives
  `0`.
  - `engine/server/packManifest.canon` `hexByte(n)` → `"0x{n:02X}"`.
  - `services/csn/cpp.canon` `hex(n)` → `"{n:X}"`, but only if `n` is never negative: your helper
    returns `"0"` for a negative `n`, the spec prints a `-`.
- `compiler: "0.1.6"` in `project.canon`: with it, another canon reports `E1015` in every editor,
  agent and command, not only at push. The hook's sha256 pin stays the stronger check.

Emit lines for the website (website ADR-0033 lets the website session write them): see below.

## Source

Commit the 12 regenerated `canon_runtime.h` after Resource's build. No Source code change is
needed, and no Source code calls anything v0.1.6 renamed. Not verified here: the header on the
Windows toolchain. canon's own tests compile it with clang and libc++ on Linux only, so the real
check is Source's Windows build.

## website

**H-14 #4 (dungeons) is unblocked.** On a copy of Resource with these three changes, `canon check`
is clean for all 89 packages. The website copy (`--only-root website`) generates
`internal/gen/world` and `internal/gen/content`, they pass `go vet`, and the website builds.
- `world/emit.canon`: `emit go { out: "@website/internal/gen/world"  package: "world"  mode: types }`
- `content/emit.canon`: `emit go { out: "@website/internal/gen/content"  package: "content"  mode: types }`
- `content/vocab/emit.canon`: the go emit's `out` becomes
  `["@monitoring/internal/platform/gen/content/vocab", "@website/internal/gen/content/vocab"]`.
  Without it, `E8004` says: "give the go emit of content.vocab a copy under website".

Then `dungeons.json` and the fallback world and difficulty constants can go.

**H-14 #6 (browser data) is still blocked, now on a canon bug.** One `emit ts` to
`@website/web/src/lib/generated/<pkg>.ts` on each of `ai`, `balance`, `ai.vocab`, `items.vocab`,
`items.vocab.core`, `text`, `text.vocab`, `ui.vocab`, `ui.vocab.neuz`, `character.vocab`,
`world.vocab` and `engine.vocab` (12 packages) checks clean. But the generated `ai.ts` fails
`tsc` with `TS2590: Expression produces a union type that is too complex to represent`, at
`moverTexts`, whose key type is a 3,256-member literal union. v0.1.5 generates the same failing
line. canon owes the fix; wait for it before adding these lines.

Generated decoder messages changed (`duplicate key`, `invalid JSON at byte N`). No website code
or test matches the old texts.

## admin

`make gen` (admin's own canon project) rewrites the committed `web/src/lib/generated/admin.ts`.
Its `canonTable` helper gains the `@stable` `findBy` lookups, and nothing it exports changes.
Commit it. The git-ignored Go copies change decoder bodies only; admin builds and its TS
type-checks.

## monitoring, config-studio

Nothing to do. monitoring's generated copies build and type-check. config-studio compiles
against v0.1.6 through its `replace`, and calls none of the changed API (`SetOverlay`,
`ClearOverlay`, `ProjectInfo` ==).

## v0.1.7 (same day): TS2590 fixed, website H-14 #6 unblocked

v0.1.7 changes only generated TypeScript (DECISIONS 370): every non-empty list of records, rows,
map entries or lists is written `canonList<T>(($r) => [$r(…), …])`, so `tsc` no longer fails with
TS2590 on large tables. Checked on the website case above: all 12 packages' TS (6.9 MB,
`moverTexts` 3,256 rows) type-check on tsc 5.9.3 (44 s) and 7.0.2 (4.6 s); both failed before.
Element checks are unchanged (an unknown field or a wrong type still fails).

- **Scripts:** bump to 0.1.7 (not 0.1.6) if the 0.1.6 bump has not run yet. Installed from the
  release: `~/.local/bin/canon` sha256 `e50b09d61972b8f70507781d83fae3a0b62783b77aad05f43cc77aa74f8b7b8b`
  (equal to the release archive's binary; tag v0.1.7 = 65f7ea8b).
- **Source:** Resource's build rewrites no `.ts` in Source; nothing beyond the 0.1.6 section.
- **website:** the H-14 #6 emit lines above can now be added.
- **admin, monitoring:** regenerated TS changes shape (rows wrapped in `$r(…)`), nothing it exports
  changes; admin commits its regenerated `admin.ts` once.
