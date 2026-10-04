# State — Canon compiler

Updated: 2026-10-04. Main (local, not pushed: CI down) holds cloud run 1 (`claude/m6-run-1`,
verified locally; [report](handoff/2026-10-03-cloud-m6-run-1.md)) plus telemetry readiness. Calls [log-2026-10-04](decisions/log-2026-10-04.md),
DECISIONS 293-298. M5 accepted (CI run 36973712977).

## Current focus

**Telemetry readiness: done** (Louis: before M6's legacy C++, which waits; Source ADR L-0111
d.1.8). Landed, each green under `make check`: C++ `baked` + constexpr lookups + define-branch
values (293, 296, 298; `6fcb139`, `43f1ff3`); the `text` target (294, 295, 297, 300; `8f8da42`,
`67e777f`); enum reflection `E.members`/`.retired` (299, `ffae68b`); `SQL` initialism (301); fixes
found by the dry run: keyword-after-dot items (302, `7616293`), `String(x)` Info leak, unknown-case
binder cascade, `ref == ref?` wrong answer, optional names never symbolic keys (303, `f96d2fa`,
`ca285e0`, `03fa330`); `examples/telemetry` (`9d53590`, `3b3c3a0`), the first real use: 7 events,
Canon-native design (Louis, "elegance over legacy"). ADR-0014. Handoff to Source:
[handoff/2026-10-04-telemetry-canon-ready.md](handoff/2026-10-04-telemetry-canon-ready.md).
Louis question pending: [louis-calls Q1](handoff/2026-10-04-louis-calls.md) (what "retired" forbids).
Not reviewed by a spec-reviewer (weekly limit hit): R4's three fixes and TE round 3 (orchestrator
read the diffs; `make check` green). Re-review when the limit resets (2026-10-06).
**Blocked:** CI (Actions billing, Louis), then push main.
Later items: cross-package decoders §2.2 vs §2.8, `ordered_json`, kind constants, API S11 vs §3.4,
go `types`/`embedded` and cpp `embedded` modes still refused at build (GM ruling). Long fuzz/progen
campaigns stay deferred ([log-2026-09-29](decisions/log-2026-09-29.md)).
Environment for a cloud session: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (run memory-capped targets by hand under `ulimit -v`).

## Milestones

M0-M5 accepted; post-M4 done. M1.5 second wave and acceptance done in cloud run 1 (box unticked only
for the unverified 3 GB cap). M6: TypeScript done, C++ baked done, legacy C++ not started (waits, Louis). M7 not started.

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
