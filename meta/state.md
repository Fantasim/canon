# State — Canon compiler

Updated: 2026-10-31. **v0.1.13** (`canon graph` / `Project.Graph`: every node and value/key edge from one analysis, patterns per schema location; DECISIONS 396-397, [log](decisions/log-2026-10-29.md), [reply](handoff/2026-10-29-canon-reply-v0113.md)) released 2026-10-29. **v0.1.12** (retired members in the enum API: C++ `IsRetired`/`k<E>LiveMembers`, Go `Retired()`/`<E>LiveMembers()`, TS `<E>LiveMembers`; DECISIONS 395, [log](decisions/log-2026-10-22.md), [reply](handoff/2026-10-22-canon-reply-v0112.md)) released 2026-10-22 (pushed, GitHub release with dist archives; Source handoff H-CANON-0112 drafted, not yet placed in `Source/meta/handoff/`). **v0.1.11** (optimisable emits: bounded-memory C++ bulk data, Go enum tables, the `make realdata-compile` release gate; DECISIONS 392-394, [log](decisions/log-2026-10-21.md), [reply](handoff/2026-10-21-canon-reply-v0111.md)) released 2026-10-22 (pushed, GitHub release with dist archives). **v0.1.10** (C++ gap release, DECISIONS 383-391, [ADR-0022](decisions/0022-v0110-cpp-gap-release.md), [log](decisions/log-2026-10-20.md), [reply](handoff/2026-10-20-canon-reply-v0110.md)) released 2026-10-20; v0.1.9 (robustness, 374-382) and earlier out; `main` is
the only branch. DECISIONS 344-369: [log-2026-10-12](decisions/log-2026-10-12.md),
[log-2026-10-13](decisions/log-2026-10-13.md), [ADR-0020](decisions/0020-api-016-overlay-ctx-call-lang-roots.md)
(earlier: 304-343, ADR-0018/0019, logs 10-06, 10-08, 10-09). M5 accepted (CI run 36973712977).

## Current focus

**In progress (2026-10-31): the v0.2 run** of [ADR-0023](decisions/0023-v02-trust-release.md): DECISIONS 398-422 (synced to the spec in S1), 36 units, release v0.2.0; [status](handoff/2026-10-31-v020-status.md), [start prompt](handoff/2026-10-31-v020-start-prompt.md). It also takes the owed items of logs 10-21, 10-22 and 10-29, and the [C++ macro collisions](handoff/2026-10-29-sovereign-cpp-macro-collisions.md) ask. v0.3 and v0.4 are outlines ([ADR-0024](decisions/0024-v03-lean-generated-code-outline.md), [ADR-0025](decisions/0025-v04-formulas-outline.md)); the why is in the [roadmap](handoff/2026-10-31-canon-roadmap-v02-v04.md) and [log](decisions/log-2026-10-31.md). Louis, 2026-10-31: formulas before binary data; the on-disk cache and N1 parked. Q1 (runtime values in schema packages) and ADR-0023 §4 Decision 9 (Go exhaustiveness) wait for his yes. `make realdata-compile` is required before every tag (394).
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
**v0.1.9, the robustness release** ([ADR-0021](decisions/0021-v019-robustness-release.md), DECISIONS 374-382): all 30 units and S1/S2 landed on `main` (`1f3d71b7`..`5413b3b2`, 62 commits, [status](handoff/2026-10-14-v019-status.md)). §8 accepted ([log-2026-10-15](decisions/log-2026-10-15.md)), [reply](handoff/2026-10-15-canon-reply-v019.md). **Waiting on Louis:** push `main` and `v0.1.9`, `gh release create`, Scripts' canon-bump. Open too: [Resource structure](handoff/2026-10-12-canon-to-sovereign-resource-structure.md);
[all handoffs](handoff/README.md). Release builds stay local (`make dist`) while CI is down.
**Performance effort (2026-10-14, [log](decisions/log-2026-10-14.md)):** halve `check`/`build` memory and time; token store done (371, −15-17% RSS); lambda captures not copied (−20% allocs per call); Resource `tidCode` handoff sent (−35% `check`); enum by-name lookup, `Prov`, parallel packages next. Robustness first: the stat-trust refresh fix and a safe on-disk cache are documented, not started ([log](decisions/log-2026-10-14.md)). **Owed, performance:** per-entry deps (an `items` edit re-checks 30 importers); stage-E memo; the §2.7
on-disk cache (a `dev` compiler must key by commit); go/cpp/ts emits sharing a dir generate for names; edit time is ops × a full analysis; FMT's whole-file
judgement holds the edit peak; F22 streaming JSON reader for Go loaders (~110 ms/MB); F21 warm re-check
on entry add/remove.
**Taken by ADR-0023** (v0.2): the 0.1.9 "owed, small" list (§9.1-§9.3, §9.6, §9.11, §10.1-§10.2), ADR-0021 §9's junction, single-file link, Windows refresh, `.canon` text-name and opaque-enum items (§4, §10.3-§10.6), E3804/E1905 (§8).
**Owed, small (ADR-0023 §13; not in v0.2):** `writelock` I/O errors name absolute paths (v0.3); a `load.dir` glob matching a `.json` text output not yet written is `E8026` only from the next run (unverified); `replacePlatforms` (`api/refresh_test.go:24`) is a third copy of `stampPlatforms`; dead `ModFloat` helpers in gen/go and gen/ts (v0.3, with the runtime goldens); §9.5's function-size items (v0.3); MSVC at scale (documented limit, CODEGEN §9); VIEWMODEL J13/J14 through a ref (next studio work); E9001's `while` variant (v0.4); the checked-widths ask's "generated size" part (v0.3). Parked: the cache's stat table (plan text only, §10.7), `evalAnalysis` → `KeptSelection` (§10.8); M4 call (b) declined (§10.9). Still open: the studio's full reload has no target.
**Documented limit:** `NoHook` is filled for the import closure only, the only packages read (ADR-0021 §2.5).
**Later:** Go and C++ `embedded` (deferred); legacy structs (M6/N1, parked); input defaults (321); `ordered_json`, kind constants (v0.3). Fuzz/progen campaigns deferred.
Environment: `internal/workspace` OS watch tests skip when inotify watches run out (raise
`fs.inotify.max_user_watches`). Cloud: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (memory caps by hand under `ulimit -v`).

## Milestones

M0-M5, post-M4, M1.5 done (3 GB cap unverified). M6: TS, C++ baked done; legacy C++ waits (Louis). M7 not started.

## What exists (committed)

spec + DECISIONS 1-382; `syntax`, `format` (+ §13 `Rewrite`), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every
WIRE §6 form), `check`/`types` (dependent types, views, translations, broken-view tracking), `eval`/`eval/std`
+ `value` (layers, provenance, drivers across the project), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go` (baked,
data, `types`), `gen/cpp` (data, `types`), `views`, `i18n`, `gen/view`, `conform`, `build`, `project`,
`check.Session`, `eval.Memo`, `workspace`, `views/live`, `edit` (+ ops, typing, codec, Refs), `api` over
workspace, `cli` (version/init/new/check/build/test/explain/fmt/guide/help), `internal/testkit`, `tools/audit`.

- Multi-destination emits (ME1, `051c3b7`): `examples/features/copies`. M4.1 (`5541eab`): verified
  multi-op edits and Undos; see [m4-units.md](m4-units.md), [plan.md](plan.md).

## Open Louis-calls
None blocking. [The M4 handoff](handoff/2026-10-01-m4-complete.md)'s (a) O2 and (c) refresh target landed in
v0.1.9 (ADR-0021), (b) `check.Info` layered maps in v0.2 (log-2026-10-14). The v0.2 order after N5 is Louis's.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`; temp dirs in `/var/tmp`.
- Agents share one working tree: never `git stash`; parallel units run in worktrees, cherry-picked
  after `make check`. `GOLANGCI_LINT_CACHE` is private per worktree; the audit fails on an
  unmeasured lane. Review rules: `.claude/rules/orchestration.md`.
- At most two agents run heavy tests (eval, `-race`, fuzz) at once (a session-local semaphore did it).

## What could not be verified

NFR-01 measured on this local machine only, not a 4-core CI runner, quiet only on `f53ba0c`; at equal load
on the final code (no regression; Louis accepted it); not re-run after M4.1's Undo verification. macOS/
Windows link tests skip where links cannot be made. Long fuzz/progen campaigns deferred by Louis. The v0.1.2
"one-value edit ~2 s" figure was not re-run. v0.1.3: edit memory measured on synthetic tables only.
v0.1.6: Windows and macOS lock and rename behaviour (vet only); a real editor day on Resource for F1 (the
bound is on a synthetic Resource-shaped project); Go types decoding on Sovereign's real files (synthetic
only); edit p95 0.32 s / evaluate 0.24 s miss 0.30 / 0.15 as v0.1.5 does (load 2-10).
## Verify queue
Re-run `make bench-edit` on a quiet machine or a CI-class runner when one exists.
