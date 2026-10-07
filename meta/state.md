# State — Canon compiler

Updated: 2026-10-07. **v0.1.2 released** (tag on main; archives from the tag workflow). v0.1.0
(tag on `f4f7da5`) and v0.1.1 (stale `canon guide`) are out; `main` is the only branch. DECISIONS
304-334: [log-2026-10-05](decisions/log-2026-10-05.md), [log-2026-10-06](decisions/log-2026-10-06.md),
ADR-0015, -0016, [ADR-0017](decisions/0017-shared-records-make-hooks.md),
[ADR-0018](decisions/0018-frame-owned-collections.md). M5 accepted (CI run 36973712977).

## Current focus

**v0.1.2 (the Resource port's findings), then telemetry as first real use.** Shipped in v0.1.2
(DECISIONS in DECISIONS.md): 326 text-output ownership in `<pkg>/canon.outputs`; 327 a JSON text file
leaves an absent field out; 328 one step budget per package; 329 files take a plain create's mode
(umask); 330 an edit analyses only what it affects, found statically (E7008); 331 W4001 says what a
broken type silences; 332 roots per machine (`optional_roots`, `project.local.canon`, E1013, E8022,
E8023, E8025); 333 an unknown name beside a literal is E2102; 334 `it` and keys, three readings
fixed. Perf: frame-owned collections (ADR-0018), reporting N findings linear. Bugs: A1
`hasKey(it)` in a `where`; E3028 for a type after a union's first alternative; a warm re-check kept
the break of a let whose fold failed. v0.1.0 was 317-324.
Handoffs: Resource port [findings](handoff/2026-10-07-sovereign-resource-port.md), answered by
[Canon's reply](handoff/2026-10-07-canon-reply-resource-port.md); [all](handoff/README.md).
**Next:** telemetry first use; the Resource port resumes on Sovereign's side; fix every known bug
that surfaces. Release builds stay local (`make dist`) while CI is down (Actions billing).
**Owed, performance** (the reply): warm re-check when an edit adds or removes an entry (key-set
dependencies in the checker; ~3-4 s cold today); the on-disk cache (`Options.Cache`).
**Owed for v0.2** (refused today with a way out): Go `embedded`/`types` and C++ `embedded` modes,
dependent values read through a ref or optional, legacy structs (M6), entry isolation of a poisoned
table (321), input defaults (321). Also later: `ordered_json`, kind constants, API S11 vs §3.4,
editor scope of calls inside interpolations. Long fuzz/progen campaigns stay deferred
([log-2026-09-29](decisions/log-2026-09-29.md)); progen operators for new codes follow.
Environment: OS watch tests in `internal/workspace` skip when inotify watches are exhausted (raise
`fs.inotify.max_user_watches`). Cloud: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci
--prefix tools/tsc`; no systemd (memory-cap targets by hand under `ulimit -v`).

## Milestones

M0-M5 accepted; post-M4 done. M1.5 done (box unticked only for the unverified 3 GB cap). M6:
TypeScript and C++ baked done, legacy C++ not started (waits, Louis). M7 not started.

## What exists (committed)

spec + DECISIONS 1-334; `syntax`, `format` (+ §13 `Rewrite`; M9 and Rewrite judge a file in its role,
258), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form; a `load` given to a field decodes
in its scope, 268), `check`/`types` (dependent types, views, translations, broken-view/-translation
tracking; E1903 `variantCase`, E3015 `notConstant`/`budget` for phase 2's folds only, 263),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the project;
stage A forces the constants phase 2's folds read, 264), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go`, `gen/cpp` (data, `types`), `views`, `i18n`, `gen/view`, `conform`,
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
for this update.

## Verify queue
Re-run `make bench-edit` on a quiet machine or a CI-class runner when one exists.
