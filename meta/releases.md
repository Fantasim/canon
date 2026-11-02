# Older releases and long-running notes

Moved out of [state.md](state.md) on 2026-11-02 to keep it under its 80-line cap. Newest facts live in state.md and the reply of each release in [handoff/README.md](handoff/README.md).

## Release headlines (state.md, as of 2026-10-31)

Updated: 2026-10-31. **v0.1.13** (`canon graph` / `Project.Graph`: every node and value/key edge from one analysis, patterns per schema location; DECISIONS 396-397, [log](decisions/log-2026-10-29.md), [reply](handoff/2026-10-29-canon-reply-v0113.md)) released 2026-10-29. **v0.1.12** (retired members in the enum API: C++ `IsRetired`/`k<E>LiveMembers`, Go `Retired()`/`<E>LiveMembers()`, TS `<E>LiveMembers`; DECISIONS 395, [log](decisions/log-2026-10-22.md), [reply](handoff/2026-10-22-canon-reply-v0112.md)) released 2026-10-22 (pushed, GitHub release with dist archives; Source handoff H-CANON-0112 drafted, not yet placed in `Source/meta/handoff/`). **v0.1.11** (optimisable emits: bounded-memory C++ bulk data, Go enum tables, the `make realdata-compile` release gate; DECISIONS 392-394, [log](decisions/log-2026-10-21.md), [reply](handoff/2026-10-21-canon-reply-v0111.md)) released 2026-10-22 (pushed, GitHub release with dist archives). **v0.1.10** (C++ gap release, DECISIONS 383-391, [ADR-0022](decisions/0022-v0110-cpp-gap-release.md), [log](decisions/log-2026-10-20.md), [reply](handoff/2026-10-20-canon-reply-v0110.md)) released 2026-10-20; v0.1.9 (robustness, 374-382) and earlier out; `main` is
the only branch. DECISIONS 344-369: [log-2026-10-12](decisions/log-2026-10-12.md),
[log-2026-10-13](decisions/log-2026-10-13.md), [ADR-0020](decisions/0020-api-016-overlay-ctx-call-lang-roots.md)
(earlier: 304-343, ADR-0018/0019, logs 10-06, 10-08, 10-09). M5 accepted (CI run 36973712977).

## Older notes (as of 2026-10-31)

**v0.1.6 shipped F1-F19** (the Resource review's [next versions](handoff/2026-10-12-canon-next-versions.md)
§2; [reply](handoff/2026-10-13-canon-reply-v016.md)): LSP releases a project on its last close (368);
`ProjectInfo.Roots`, `ValueWith`/`Edit.Lang`, overlays take a ctx (344-346, breaking, ADR-0020); F3 (347-350)
consumer-copy `E8004`, shared string id types, dependent value through a ref; F15, F11 (351-357) unique
staging, per-package writer locks, `compiler:` pin, package-less files, `E8152` in check, stray generated
files, case-only text rename; F7, F8 (360, 361, 364) no repeated `E3305`, every text decoder refuses
repeated keys and names the first failure in document order; F14 (362, 363, 365, 369) C++/TS open enums and
`@text` decoders, exact owner hooks, `E8019`, TS `findBy<F>`; F2 (366) reserved-word members in edits; F18
(358, 359) format specs, `duplicates()`; F4, F19, F6 guide topics; F12, F5; F9, F10: no bug. Deferred: F20
(per-field `@json(codes)`), F21 (warm re-check), F22 (streaming JSON reader). Pre-tag: E8152's json names
planned, not encoded (2.1× edit regression gone), M6 oracle joins, killed-build wedge (336).
**v0.1.9, the robustness release** ([ADR-0021](decisions/0021-v019-robustness-release.md), DECISIONS 374-382): all 30 units and S1/S2 landed on `main` (`1f3d71b7`..`5413b3b2`, 62 commits, [status](handoff/2026-10-14-v019-status.md)). §8 accepted ([log-2026-10-15](decisions/log-2026-10-15.md)), [reply](handoff/2026-10-15-canon-reply-v019.md). (v0.1.9 has since been released, see the headlines above.) Open too: [Resource structure](handoff/2026-10-12-canon-to-sovereign-resource-structure.md);
[all handoffs](handoff/README.md). Release builds stay local (`make dist`) while CI is down.
**Performance effort (2026-10-14, [log](decisions/log-2026-10-14.md)):** halve `check`/`build` memory and time; token store done (371, −15-17% RSS); lambda captures not copied (−20% allocs per call); Resource `tidCode` handoff sent (−35% `check`); enum by-name lookup, `Prov`, parallel packages next. Robustness first: the stat-trust refresh fix and a safe on-disk cache are documented, not started ([log](decisions/log-2026-10-14.md)). **Owed, performance:** per-entry deps (an `items` edit re-checks 30 importers); stage-E memo; the §2.7
on-disk cache (a `dev` compiler must key by commit); go/cpp/ts emits sharing a dir generate for names; edit time is ops × a full analysis; FMT's whole-file
judgement holds the edit peak; F22 streaming JSON reader for Go loaders (~110 ms/MB); F21 warm re-check
on entry add/remove.
**Taken by ADR-0023** (v0.2): the 0.1.9 "owed, small" list (§9.1-§9.3, §9.6, §9.11, §10.1-§10.2), ADR-0021 §9's junction, single-file link, Windows refresh, `.canon` text-name and opaque-enum items (§4, §10.3-§10.6), E3804/E1905 (§8).
**Owed, small (ADR-0023 §13; not in v0.2):** `writelock` I/O errors name absolute paths (v0.3); a `load.dir` glob matching a `.json` text output not yet written is `E8026` only from the next run (unverified); `replacePlatforms` (`api/refresh_test.go:24`) is a third copy of `stampPlatforms`; dead `ModFloat` helpers in gen/go and gen/ts (v0.3, with the runtime goldens); §9.5's function-size items (v0.3); MSVC at scale (documented limit, CODEGEN §9); VIEWMODEL J13/J14 through a ref (next studio work); E9001's `while` variant (v0.4); the checked-widths ask's "generated size" part (v0.3). Parked: the cache's stat table (plan text only, §10.7), `evalAnalysis` → `KeptSelection` (§10.8); M4 call (b) declined (§10.9). Still open: the studio's full reload has no target.
**Documented limit:** `NoHook` is filled for the import closure only, the only packages read (ADR-0021 §2.5).
**Later:** Go and C++ `embedded` (deferred); legacy structs (M6/N1, parked); input defaults (321); `ordered_json`, kind constants (v0.3). Fuzz/progen campaigns deferred.

## Older "what could not be verified" caveats (as of 2026-10-31)

NFR-01 measured on this local machine only, not a 4-core CI runner, quiet only on `f53ba0c`; at equal load
on the final code (no regression; Louis accepted it); not re-run after M4.1's Undo verification. macOS/
Windows link tests skip where links cannot be made. Long fuzz/progen campaigns deferred by Louis. The v0.1.2
"one-value edit ~2 s" figure was not re-run. v0.1.3: edit memory measured on synthetic tables only.
v0.1.6: Windows and macOS lock and rename behaviour (vet only); a real editor day on Resource for F1 (the
bound is on a synthetic Resource-shaped project); Go types decoding on Sovereign's real files (synthetic
only); edit p95 0.32 s / evaluate 0.24 s miss 0.30 / 0.15 as v0.1.5 does (load 2-10).
