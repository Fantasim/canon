# State — Canon compiler

Updated: 2026-10-01. **M4 accepted** (main `f6114a2`, CI green on all three platforms); M3 accepted
earlier (`ea3d7e2`). First after M4: M4.1, dependent fields in multi-op edits (handoff).
Full report: [handoff/2026-10-01-m4-complete.md](handoff/2026-10-01-m4-complete.md). Unit ledger:
[m4-units.md](m4-units.md). Calls: [decisions/log-2026-09-29.md](decisions/log-2026-09-29.md) "M4".
Design: [ADR-0011](decisions/0011-incremental-memo.md), [ADR-0012](decisions/0012-formatter-region-settle.md).

## Current focus

M4 (plan.md "M4 — Formatter and the edit API") is accepted, 2026-10-01, all 6 acceptance items, the
§7.9 feature examples `edits` and `entries` in, CI green (`claude/m4-ci8`). The proofs:
1. fixed points: gated in `make check`; `FuzzFormat`, `FuzzRewrite`, `FuzzRewriteAround` 10 min clean
   on the final code, the `jsonsrc` fuzzes earlier.
2. every API.md rule tested: `TestEveryAPIRuleHasATest` (`internal/testkit`), 148 rules.
3. minimal writes under fuzzing: `make fuzz-edit`, 10 min, 84,242 execs past baseline, 19,538
   example and 6,359 benchmark edits applied.
4. NFR-01 on the quiet reference machine (AMD Ryzen AI 9 HX 370): Edit p95 0.267 s, Evaluate p95
   15 ms, cold check 2.49 s, RSS 1.22 GB, view models <= 0.49 MB (`make bench-edit`).
5. crash test: `TestCommitCrashFailedRename` and its siblings (`internal/edit`).
6. `make stress`, 60 s, PASS.

**Next**: per [plan.md](plan.md): M5 (LSP), M6 (legacy C++/TS) and M7 (migration) run in parallel now
that M4 is accepted; multi-destination emits (DECISIONS 229) are implemented after M4 (Louis's
call), spec sync first. The post-M4 spec sync (DECISIONS 207) and the later units are listed in the
handoff. M1.5's second wave (type-directed + metamorphic progen suites) stays parked.

CI on `claude/m4-ci8` is green on all three platforms; the merge to `main` is Louis's (on GitHub, or
by pushing `main`). Long fuzz/progen campaigns stay deferred by Louis ([decisions/log-2026-09-29.md](decisions/log-2026-09-29.md)
"Platforms and fuzzing"): a milestone runs only its own stated acceptance criteria.

## Milestones

M0, M1, M2, M3, **M4** accepted. M1.5 foundation committed (`f498713`), still open (second wave:
type-directed + metamorphic progen suites; see `plan.md`), parked. M5, M6, M7 not started.

## What exists (committed)

spec + DECISIONS 1–251; `syntax`, `format` (+ §13 `Rewrite`), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every WIRE §6 form),
`check`/`types` (dependent types, views, translations, broken-view/-translation tracking),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the
project), `verify`, `lock`, `rules`, `ir` (stage E, fingerprint, Go/C++/`types`-mode name plans,
pattern automaton, alias-chain patterns), `gen/json`, `gen/go` (baked, data, stores, translated
fns, conformance, runtime inputs, unions, define refs), `gen/cpp` (data mode, `types` mode,
stores, runtime, conformance, strict loaders, define refs), `views`, `i18n`, `gen/view`,
`conform`, `build` (writes the `view` target in phase 8), `project` (+ parse reuse), `check.Session`,
`eval.Memo`, `workspace` (snapshots, revisions, overlays, one writer), `views/live`, `edit` (+ ops,
typing, codec, Refs), `api` over workspace (Check/Build/Test, `Value`+`Origin`, `ViewModel`,
overlays), `cli` (version/init/new/check/build/test/explain/fmt), `internal/testkit` (+ `cxx`, `progen`, `jsonschema`, `benchgen`); `tools/audit`.

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
stated acceptance, deferred by Louis.

## Verify queue

Louis merges `claude/m4-ci8` (CI green) into `main`. Re-run `make bench-edit` on a quiet machine or a
CI-class runner when one exists.
