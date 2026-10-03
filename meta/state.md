# State — Canon compiler

Updated: 2026-10-03 (cloud run 1). Integration branch **`claude/m6-run-1`** (stands in for main; not
merged; report [handoff/2026-10-03-cloud-m6-run-1.md](handoff/2026-10-03-cloud-m6-run-1.md), calls
[log-2026-10-03](decisions/log-2026-10-03.md), DECISIONS 279-292). M5 accepted (CI run 36973712977).

## Current focus

Cloud run 1 landed 20 reviewed units, each green under local `make check`: M6 TypeScript target
(T1 `b15485d`); M5 gaps G3 `1f571ee`, G4 `f0a3dfa`, G5 `31de12e`; bugs M6-append `06f35c0`, B1
`916599f`, W1 `44bb9ce`, WB2 `2ed7c3d`, C1 `b8c1443`, CX `6f687d7`, L2 `e87e18b`; LSP later items LX
`f5ffd7c`; workspace flake F1 `4106f77`; Hardening: H1 `182eb81`, H2 `6303766`, HW1 `e4cef64`, N1
`4c44b5f`, Go/C++ table fields GG `9999f8a`, GC `b5c0d86`. No progen archive open; the M1.5
acceptance run is clean on 3 seeds at N=10000 (only the 3 GB MemoryMax cap is unverified: no systemd).
**Blocked:** CI. Every Actions job fails before a runner starts since ~05:10 UTC 2026-10-03 (likely
the minutes/spending limit): Louis to check billing, then run CI on the branch head and merge.
Next (needs a ruling or Louis): the later items in the run report (cross-package decoders §2.2 vs
§2.8, `ordered_json` overload, generators writing kind constants, API S11 vs §3.4). Not started:
M6's legacy C++ part, M7, telemetry. Long fuzz/progen campaigns stay deferred by Louis
([log-2026-09-29](decisions/log-2026-09-29.md) "Platforms and fuzzing").
Environment for a cloud session: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (run memory-capped targets by hand under `ulimit -v`).

## Milestones

M0-M5 accepted; post-M4 done. M1.5 second wave and acceptance done in cloud run 1 (box unticked only
for the unverified 3 GB cap). M6: TypeScript done, legacy C++ not started. M7 not started.

## What exists (committed)

spec + DECISIONS 1-292; `syntax`, `format` (+ §13 `Rewrite`; M9 and Rewrite judge a file in its role,
258), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form; a `load` given to a field decodes
in its scope, 268), `check`/`types` (dependent types, views, translations, broken-view/-translation
tracking; E1903 `variantCase`, E3015 `notConstant`/`budget` for phase 2's folds only, 263),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the project;
stage A forces the constants phase 2's folds read, 264), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go`, `gen/cpp` (data and
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
only for edit layers and multi-op dependent requests; the bench has neither).

## Verify queue

Louis pushes `main` (HEAD `37968e3`); `claude/post-m4-ci` is pushed. Re-run `make bench-edit` on a
quiet machine or a CI-class runner when one exists.
