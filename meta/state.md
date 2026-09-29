# State — Canon compiler

Updated: 2026-09-29. **M3 accepted** (main, HEAD `ea3d7e2`). Session stops at the M4 boundary.
Full report: [handoff/2026-09-29-m3-complete.md](handoff/2026-09-29-m3-complete.md). Unit ledger:
[m3-units.md](m3-units.md). Calls of the final wave:
[decisions/log-2026-09-28.md](decisions/log-2026-09-28.md),
[decisions/log-2026-09-29.md](decisions/log-2026-09-29.md).

## Current focus

M3 (plan.md "M3 — Load and the view model") is accepted, 2026-09-29, all 7 acceptance items,
each proved by a test:
1. every example prints its `findings.txt` — `TestExampleFindings` (`internal/cli`).
2. view models validate against the schema and equal goldens — `TestExamples` +
   `TestExamplesViewModelsValidate` (`internal/testkit/golden`).
3. `balance.parity` golden — `TestExamples/balance.parity`.
4. `canon explain` golden — `TestExplainExamples` (`internal/cli`).
5. `api.ViewModel` (JSON equals the emit bytes) — `TestViewModelEqualsEmitView` (`api`).
6. C++ `types` mode decode — `TestEventsTypesDecode` (`internal/testkit/golden`).
7. real-data job, not gating — `make check-real`, findings refreshed in
   [handoff/2026-09-28-realdata-findings.md](handoff/2026-09-28-realdata-findings.md).

**Next milestone: M4** (formatter and the edit API, `plan.md` "M4 — Formatter and the edit API",
IMPLEMENTATION-PLAN §6 M4). Not started; no unit scoped yet.

Landed after M3's acceptance list closed but before the stop, all on `main`:
- libc++ joins the C++ test matrix (`clang++ -stdlib=libc++`, header-detected, gated under
  `CANON_REQUIRE_CXX`).
- `check.yml` gains `windows-latest` (MSVC: `go test` without `CANON_REQUIRE_CXX`, plus
  `TestGoldensCompileMSVC` compiling every committed C++ golden with `cl.exe`) and
  `macos-latest` (Apple clang/libc++, `CANON_REQUIRE_CXX` on). **Neither has run**: both fire only
  once Louis pushes, e.g. `git push origin main:refs/heads/claude/m3-ci`.
- Long fuzz/progen campaigns are deferred by Louis ("not now… I need this language to be ready
  soon"): a milestone runs only its own stated acceptance criteria, never a multi-hour nightly
  campaign — [decisions/log-2026-09-29.md](decisions/log-2026-09-29.md) "Platforms and fuzzing".

## Milestones

M0, M1, M2, **M3** accepted. M1.5 foundation committed (`f498713`), still open (second wave:
type-directed + metamorphic progen suites; see `plan.md`). M4 next.

## What exists (committed)

spec + DECISIONS 1–228; `syntax`, `format` (stub), `jsonsrc`, `wire`, `load` (every WIRE §6 form),
`check`/`types` (dependent types, views, translations, broken-view/-translation tracking),
`eval`/`eval/std` + `value` (layers, provenance, variant-level methods, drivers across the
project), `verify`, `lock`, `rules`, `ir` (stage E, fingerprint, Go/C++/`types`-mode name plans,
pattern automaton, alias-chain patterns), `gen/json`, `gen/go` (baked, data, stores, translated
fns, conformance, runtime inputs, unions, define refs), `gen/cpp` (data mode, `types` mode,
stores, runtime, conformance, strict loaders, define refs), `views`, `i18n`, `gen/view`,
`conform`, `build` (writes the `view` target in phase 8), `project`, `api` (Check/Build/Test,
`Value`+`Origin`, `ViewModel`, `edit.Snapshot`/`Resolve`), `cli` (version/init/new/check/build/
test/explain), `internal/testkit` (+ `cxx`, `progen`, `jsonschema`, `benchgen`); `tools/audit`.

## Open Louis-calls

None open. `handoff/2026-09-24-questions.md` does not exist.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`. Temp dirs go to
  `/var/tmp`, never `/tmp`.
- Agents share one working tree: never `git stash`. Parallel units run in worktrees; commit
  there, cherry-pick, `make check` before every continue.
- `GOLANGCI_LINT_CACHE` is private per worktree. The audit fails on an unmeasured lane.
- Sonnet units that failed review twice moved to opus (`.claude/rules/orchestration.md`).

## What could not be verified

Windows and macOS real CI runs (MSVC `cl.exe`, Apple clang) — the jobs exist in `check.yml` but
have not executed; only local g++ 15.2 and clang++ 21.1 (now also `-stdlib=libc++`) ran. Long
fuzz/progen campaigns beyond default size, deferred by Louis. NFR-01 performance targets (§7.6)
are M4's own gate, not measured against M3 code. `canon explain`'s input fields among its parts,
deferred to M4 (log-2026-09-29 "U15 api.ViewModel").
