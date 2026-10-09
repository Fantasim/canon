# State — Canon compiler

Updated: 2026-10-09. **v0.1.4** (Sovereign's 2026-10-08 asks) on main; v0.1.3 to v0.1.0 are
out; `main` is the only branch. DECISIONS 304-343:
[log-2026-10-06](decisions/log-2026-10-06.md), [log-2026-10-08](decisions/log-2026-10-08.md),
[ADR-0018](decisions/0018-frame-owned-collections.md),
[ADR-0019](decisions/0019-edit-memory-adopted-verdicts.md). M5 accepted (CI run 36973712977).

## Current focus

**v0.1.4, then telemetry as first real use.** Shipped: 339 open enums in Go `types` mode
(`open: [E]`, a string type keeping the wire); 340 `Decode<Fn>File` for a map/list `@text`
result (skipped, never refused, when it cannot be written); 341 `E8026` an output its own build
reads; 342 `E8027` no write through a symbolic link (fail closed); 343 `consumer_roots` and
`canon build --only-root` (`E8028`). Fix: `overlayFS.EvalSymlinks("/")` recursed forever (edit).
Stage E now judges go emits on each copy's view (`CopyOf`), as the generator sees them.
Handoffs: [Canon's reply](handoff/2026-10-09-canon-reply-open-ids-consumer-roots.md); [all](handoff/README.md).
**Next:** telemetry first use; Sovereign's cutover on Go types; fix every known bug that surfaces.
Release builds stay local (`make dist`) while CI is down.
**Fixed 40226d7:** `canon test` pinned one file table per expect (Sovereign whole test 7.76 GB/60 s → 2.15 GB/19 s). **Owed, performance:** edit time is ops × a full analysis (E1); FMT's whole-file judgement holds
the edit peak (per-item judgement, or the on-disk cache `Options.Cache`); a token-streaming JSON
reader for Go data/types loaders (~110 ms/MB today); warm re-check on entry add/remove.
**Owed from 2026-10-08 asks:** the cross-root import trap (E8004-like), `E8005` on several `table
Constant` lets, `ownerHooks` exact for data-mode owners, C++/TS open enums and text decoders.
**Owed for v0.2:** dependent types kept through static typing (lifts E3804 on dependent-map
`union`/literals); an amend path into a dependent map (E1905 today); Go and C++ `embedded`;
legacy structs (M6); entry isolation (321); input defaults (321); E3305 duplicated on nested
untyped literals. Also later: `ordered_json`, kind constants, API S11 vs §3.4, editor scope of calls
inside interpolations. Long fuzz/progen campaigns stay deferred.
Environment: OS watch tests in `internal/workspace` skip when inotify watches are exhausted (raise
`fs.inotify.max_user_watches`). Cloud: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci
--prefix tools/tsc`; no systemd (memory-cap targets by hand under `ulimit -v`).

## Milestones

M0-M5 accepted; post-M4 done. M1.5 done (box unticked only for the unverified 3 GB cap). M6:
TypeScript and C++ baked done, legacy C++ not started (waits, Louis). M7 not started.

## What exists (committed)

spec + DECISIONS 1-338; `syntax`, `format` (+ §13 `Rewrite`; M9 and Rewrite judge a file in its role,
258), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form; a `load` given to a field decodes
in its scope, 268), `check`/`types` (dependent types, views, translations, broken-view/-translation
tracking; E1903 `variantCase`, E3015 `notConstant`/`budget` for phase 2's folds only, 263),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the project;
stage A forces the constants phase 2's folds read, 264), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go` (baked, data, `types`), `gen/cpp` (data, `types`), `views`, `i18n`, `gen/view`, `conform`,
`build`, `project`, `check.Session`, `eval.Memo`, `workspace`, `views/live`, `edit` (+ ops, typing,
codec, Refs), `api` over workspace,
`cli` (version/init/new/check/build/test/explain/fmt/help), `internal/testkit`, `tools/audit`.

- Multi-destination emits (ME1, `051c3b7`): `examples/features/copies`. M4.1 (`5541eab`): verified
  multi-op edits and Undos; detail in [m4-units.md](m4-units.md) and [plan.md](plan.md).

## Open Louis-calls
Three, none blocking, in [the M4 handoff](handoff/2026-10-01-m4-complete.md): (a) O2, skip dot-files as
sources; (b) optional frozen-contract change to `check.Info` (layered maps); (c) a target for the
studio's view-model refresh after an edit (240-470 ms).

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`. Temp dirs go to
  `/var/tmp`, never `/tmp`.
- Agents share one working tree: never `git stash`; parallel units run in worktrees, cherry-picked
  after `make check`. `GOLANGCI_LINT_CACHE` is private per worktree; the audit fails on an
  unmeasured lane. Review rules: `.claude/rules/orchestration.md`.

## What could not be verified

NFR-01 measured on this local reference machine only, not a 4-core CI runner, and quiet only on
`f53ba0c`. On the final code it was compared at equal load (no regression; Louis accepted it). The
macOS/Windows link tests skip where links cannot be made. Long fuzz/progen campaigns beyond the
stated acceptance, deferred by Louis. NFR-01 was not re-run after M4.1's Undo verification (it runs
only for edit layers and multi-op dependent requests; the bench has neither). The 2026-10-06 wave's
final `make check` was green on `f4f7da5`. The v0.1.2 `make check` and the "one-value edit ~2 s" figure (the reply's bench) were not re-run
for this update. v0.1.3: Go types decoding was not run on Sovereign's real files (synthetic
5 MB only); edit memory was measured on synthetic tables, not the real `model` package.

## Verify queue
Re-run `make bench-edit` on a quiet machine or a CI-class runner when one exists.
