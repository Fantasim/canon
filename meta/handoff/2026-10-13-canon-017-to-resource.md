# To Resource: canon 0.1.7 (2026-10-13)

From the Canon compiler session. canon 0.1.7 is installed. Scripts moves your pre-push pin
([its handoff](2026-10-13-canon-017-to-scripts.md)); until then the hook refuses the new binary.

Measured on copies of HEAD `43e46174` (0.1.6) and `3cd10875` (0.1.7), run with both versions
and outputs diffed:
- `canon check`: 0 errors and 0 warnings in 89 packages on 0.1.5, 0.1.6 and 0.1.7. No new finding.
- `build --check`: identical, except the C++ runtime header below.

## Required, after the pin lands

1. `canon build`. canon's only change to committed outputs is the C++ runtime header in Source,
   12 copies: `Engine/Resource/Generated/<pkg>/canon_runtime.h` ×11 and
   `Engine/Telemetry/Generated/canon_runtime.h` (new decoders and open enums). Source commits them
   ([its handoff](2026-10-13-canon-017-to-source.md)).
   - On `43e46174` the same `build --check` also listed stale telemetry outputs (`session.gen.h`,
     `telemetry.gen.h`, `Telemetry*.gen.h`, `Tools/telemetry/generated/*`). Every canon version
     listed them: they come from your telemetry commits not yet built, not from canon.
2. Nothing else. Keep `/Config/.canon/` git-ignored (it already is): canon now keeps writer locks
   there too.

## Website emit lines (website ADR-0033: the website session writes them)

The website session applies [the patch](2026-10-13-website-resource-emits.patch): 15 emit files,
Go for `world` and `content` (and a website copy of `content.vocab`), and TS for 12 packages.
It applies cleanly to HEAD `3cd10875`, and with it `canon check` is clean (0.1.7). It adds no
work for you.

## Optional cleanups 0.1.6 made possible

- `engine/numbers.canon` `duplicates(keys)` (41 calls in 20 files) → built-in `keys.duplicates()`.
  The order differs: yours lists keys by first repeat (`[a, b, b, a]` → `[b, a]`), the built-in
  by first occurrence (`[a, b]`). Only the order of listed duplicates in a message changes; re-run
  the `expect` tests that pin one.
- Format specs, checked on 0.1.7:
  - `engine/server/packManifest.canon` `hexByte(n)` → `"0x{n:02X}"` (prints `0x0A`).
  - `services/csn/cpp.canon` `hex(n)` → `"{n:X}"`, but only if `n` is never negative: your helper
    gives `"0"` for a negative number, while the spec prints a `-`. `{0:X}` is `0`.
- `compiler: "0.1.7"` in `project.canon`: then any other canon reports `E1015` in every editor,
  agent and command, not only at push. Bump it with the pin.
