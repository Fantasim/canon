# canon 0.1.8: upgrade actions (2026-10-14)

From the Canon compiler session. canon **v0.1.8** is tagged (`c33919a4`) and installed at
`~/.local/bin/canon` from its release archive. Louis publishes the GitHub release
(https://github.com/Fantasim/canon/releases/tag/v0.1.8) with the `dist/` archives.

Installed binary: sha256 `f78343a17b245d8571d644ae719ac87584e848d8a716d407b985da50f0b90fd2`, equal
to the `canon` inside `canon_0.1.8_linux_amd64.tar.gz`; `canon version` prints
`canon 0.1.8 (c33919a4dca014bbe14621cd71ef9c2edb8a54f4)`.

Until the pins move, Scripts' gate and Resource's pre-push hook refuse the installed binary (both
pin 0.1.7, `e50b09d6…`). The v0.1.7 archive is still on its release page if a gate must run first.

## What changed

- About a quarter less memory, outputs byte-identical (findings, `explain`, view models, generated
  Go/C++/TS, JSON, `canon edit` writes). On Resource `456c1bcd`, peak RSS: `check` 2.52-2.62 GB →
  1.88-1.99 GB, `build --check` 2.81-3.01 GB → 2.07-2.09 GB. Wall time unchanged.
- New `E.named(s)`, and a warning `W3002` on scans of `E.members` by name. `named` is reserved on
  enums.
- Fixed: `E.members` and friends in a type bound written before the enum saw no members.

## Order

1. **Resource first:** fix the 12 `W3002` sites ([Resource handoff](2026-10-14-canon-to-resource-tidcode.md)).
   On 0.1.8 they are warnings, and every gate runs `-max-warnings 0`. The cached-map fix works on
   0.1.7, so it can land before the bump.
2. **Scripts:** run `canon-bump` with `V=0.1.8`, expected hash `H` above. Both pins:
   `lib/canon_gate.sh:144-145` and Resource `Tools/hooks/pre-push:49`. Step 3's runs should drop
   to about 1.9-2.1 GB peak; please update the memory comments (`lib/canon_gate.sh:118` says
   2580 MB for `check`). With Resource's `tidCode` fixed, `check` is also about a third faster
   (11-12 s against 17-20 s on a copy at load ~5).
3. **Source, website, admin, sovcommon:** nothing to do. `canon build` on Resource writes the same
   8,721 files byte for byte; only eight studio view models differ, by the W3002 findings, and
   they return to identical once Resource fixes the scans.

## Re-measure

`canon lsp` holds the same smaller analysis: Scripts' LSP memory gate should see less. One Resource
project idled near 0.9 GB on 0.1.6; expect less on 0.1.8 (not measured here).
