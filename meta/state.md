# State — Canon compiler

Updated: 2026-10-06. Branch `feat/past-and-ergonomics` (worktree, not yet on main) holds DECISIONS
304-324: [log-2026-10-05](decisions/log-2026-10-05.md), [log-2026-10-06](decisions/log-2026-10-06.md),
ADR-0015, -0016, [ADR-0017](decisions/0017-shared-records-make-hooks.md). M5 accepted (CI run 36973712977).

## Current focus

**v0.1.0 release prep.** Landed on the branch: `in` tests elements, `hasKey` tests keys (317); a
name in scope is never a symbolic key (318); one form per meaning (`= none` warns W3001, `values: []`
refused; 319); honest refusals, every generator limit refused by `check` with its way out (320);
hygiene, `canon help`, never-reported codes reported (322); shared records across packages in Go,
C++ and TS, all modes, through make hooks (323); precomputed results verified (324). Handoffs:
[Source](handoff/2026-10-05-telemetry-source.md), [showcase](handoff/2026-10-05-showcase-workarounds.md),
[audit](handoff/2026-10-06-design-audit.md).
**Next:** merge to main, push, tag v0.1.0, publish the GitHub release with archives built locally
(`make dist`; CI is down: Actions billing); remove the merged branches and worktrees.
**Owed for v0.2** (refused today with a way out): Go `embedded`/`types` and C++ `embedded` modes,
dependent values read through a ref or optional, legacy structs (M6), entry isolation of a poisoned
table (321), input defaults (321). Also later: `ordered_json`, kind constants, API S11 vs §3.4,
editor scope of calls inside interpolations. Long fuzz/progen campaigns stay deferred
([log-2026-09-29](decisions/log-2026-09-29.md)); progen operators for the new codes are a follow-up.
Environment for a cloud session: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (run memory-capped targets by hand under `ulimit -v`).

## Milestones

M0-M5 accepted; post-M4 done. M1.5 second wave and acceptance done in cloud run 1 (box unticked
only for the unverified 3 GB cap). M6: TypeScript done, C++ baked done, legacy C++ not started
(waits, Louis). M7 not started.

## What exists (committed)

spec + DECISIONS 1-324; `syntax`, `format` (+ §13 `Rewrite`; M9 and Rewrite judge a file in its role,
258), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form; a `load` given to a field decodes
in its scope, 268), `check`/`types` (dependent types, views, translations, broken-view/-translation
tracking; E1903 `variantCase`, E3015 `notConstant`/`budget` for phase 2's folds only, 263),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the project;
stage A forces the constants phase 2's folds read, 264), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go`, `gen/cpp` (data and
`types` modes), `views`, `i18n`, `gen/view`, `conform`, `build`, `project`, `check.Session`,
`eval.Memo`, `workspace`, `views/live`, `edit` (+ ops, typing, codec, Refs), `api` over workspace,
`cli` (version/init/new/check/build/test/explain/fmt/help), `internal/testkit`; `tools/audit`.

- Multi-destination emits (ME1, `051c3b7`): an `out` list per emit; `internal/check/emitout.go`
  (E8009 variants, E8004 `noEmit`/`noCopy`); `examples/features/copies`; ts copies checked only.
- M4.1 (`5541eab`): multi-op edits type each op against the earlier ops' state; every Undo is
  verified by a dry apply, restoring the smallest enclosing item or refusing NotEditable(computed);
  every edit under an EditLayer is verified. Older edit bugs fixed with it (layer AddEntry, valueDiff,
  region restore after a Rename, rename-then-remove, rename-then-re-add, swap); file Changes are
  each path's net effect (API.md N8).

## Open Louis-calls

Three, none blocking, in [the M4 handoff](handoff/2026-10-01-m4-complete.md): (a) O2, should dot-files
be skipped as sources; (b) optional frozen-contract change to `check.Info` (layered maps); (c) is a
target wanted for the studio's view-model refresh after an edit (240-470 ms).

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`. Temp dirs go to
  `/var/tmp`, never `/tmp`.
- Agents share one working tree: never `git stash`. Parallel units run in worktrees; commit
  there, cherry-pick, `make check` before every continue.
- `GOLANGCI_LINT_CACHE` is private per worktree. The audit fails on an unmeasured lane.
- Sonnet units that failed review twice moved to opus (`.claude/rules/orchestration.md`).

## What could not be verified

NFR-01 measured on this local reference machine only, not a 4-core CI runner, and quiet only on
`f53ba0c`. On the final code it was compared at equal load (no regression; Louis accepted it). The
macOS/Windows link tests skip where links cannot be made. Long fuzz/progen campaigns beyond the
stated acceptance, deferred by Louis. NFR-01 was not re-run after M4.1's Undo verification (it runs
only for edit layers and multi-op dependent requests; the bench has neither). The 2026-10-06 wave's
final `make check` result is the orchestrator's to record; this update did not run it.

## Verify queue

Louis pushes `main` after the merge; tag v0.1.0 afterwards. Re-run `make bench-edit` on a quiet
machine or a CI-class runner when one exists.
