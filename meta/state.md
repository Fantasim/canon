# State — Canon compiler

Updated: 2026-10-01. **M4 accepted** (`c5324f6`, CI green on three platforms) and **post-M4 work
complete** on main (HEAD `37968e3`). Next: **M5 (LSP)**; M6 and M7 run in parallel (plan.md).
M3 accepted earlier (`ea3d7e2`). M4 report: [handoff/2026-10-01-m4-complete.md](handoff/2026-10-01-m4-complete.md).
Ledger: [m4-units.md](m4-units.md). Calls: [decisions/log-2026-09-29.md](decisions/log-2026-09-29.md)
"M4" and "U-E22-r" onward. Design: [ADR-0011](decisions/0011-incremental-memo.md), [ADR-0012](decisions/0012-formatter-region-settle.md).

## Current focus

Post-M4 is done (`f6114a2..37968e3`): spec syncs (DECISIONS 252-273), PS1-PS3, ME1, DV1, M4.1; units
in [m4-units.md](m4-units.md). Gates on that code: `make check` green, race tests at GOMAXPROCS 2
and 4 green, `make fuzz-edit` 2 min PASS after each M4.1 round. CI on `claude/post-m4-ci`: pushed,
result in the handoff (the orchestrator fills it).

**Next** (start prompt: [handoff/2026-10-01-next-start-prompt.md](handoff/2026-10-01-next-start-prompt.md)):
M5 (`lsp`, `editors/vscode`), with M6 (legacy C++/TS) and M7 (migration) in parallel waves per
[plan.md](plan.md); each needs its feature example first (§7.9: `ts` before M6, `pairs` still owed).
Logged later items (checker gaps, cleanups) are listed in the start prompt. M1.5's second wave stays parked.

Long fuzz/progen campaigns stay deferred by Louis ([decisions/log-2026-09-29.md](decisions/log-2026-09-29.md)
"Platforms and fuzzing"): a milestone runs only its own stated acceptance criteria.

## Milestones

M0, M1, M2, M3, **M4** accepted; post-M4 done. M1.5 foundation committed (`f498713`), still open
(second wave: type-directed + metamorphic progen suites; see `plan.md`), parked. M5, M6, M7 not started.

## What exists (committed)

spec + DECISIONS 1-273; `syntax`, `format` (+ §13 `Rewrite`; M9 and Rewrite judge a file in its role,
258), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form; a `load` given to a field decodes
in its scope, 268), `check`/`types` (dependent types, views, translations, broken-view/-translation
tracking; E1903 `variantCase`, E3015 `notConstant`/`budget` for phase 2's folds only, 263),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the project;
stage A forces the constants phase 2's folds read, 264), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/go`, `gen/cpp` (data and
`types` modes), `views`, `i18n`, `gen/view`, `conform`, `build`, `project`, `check.Session`,
`eval.Memo`, `workspace`, `views/live`, `edit` (+ ops, typing, codec, Refs), `api` over workspace,
`cli` (version/init/new/check/build/test/explain/fmt), `internal/testkit`; `tools/audit`.

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
only for edit layers and multi-op dependent requests; the bench has neither). The CI result for the
post-M4 code is not known to this update.

## Verify queue

Louis pushes `main` (HEAD `37968e3`); `claude/post-m4-ci` is pushed. Re-run `make bench-edit` on a
quiet machine or a CI-class runner when one exists.
