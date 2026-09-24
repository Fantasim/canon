# State — Canon compiler

Updated: 2026-09-24, overnight run in progress (brief:
[handoff/2026-09-24-overnight-run.md](handoff/2026-09-24-overnight-run.md)). Decisions of the run:
[decisions/log-2026-09-24.md](decisions/log-2026-09-24.md) "Overnight run".

## Current focus — overnight run tracker

Part A (close open work) then Part B (M3 waves W0–W4). Ticked = committed on `main`.
- [ ] A1 loader parity: gen/go landed (2eb3294); gen/cpp merging main for full parity.
- [x] A2 demo bugs: (1) E2102, a broken check breaks its record (428fbad, DECISIONS 209);
  (2) E7109 variants (6534f56, DECISIONS 208); (3) stays (poisoning, logged); (4) "is a Int" →
  ERRORS pass, E4402 frames stay (logged); (5) E8007 hint → ERRORS pass.
- [ ] A3 M1.5 bug-fix wave (open archives on main: check 35, format 20, eval 4, syntax 3,
  build 2, ir 1). Landed: eval 5bbd911, syntax af5a768, ir 5e42c65 (DECISIONS 209–214). In
  flight: format (opus, round 4), build, check C1 (DECISIONS 214 + queued items); then the QA
  judge redesign (log "Progen archives after a fix"), then check C2 (archive triage).
  Also: audit gate-hole cleanup (opus, round 3); gen/cpp loader parity in review.
- [x] A4 M2 close: position test on `examples/pipeline/data/II_POT_HEAL_L.json`; M2 ticked.
- [ ] A5 consumer unit (name plans) · cleanups · ERRORS.md pass + spec sync #2.
- [ ] A6 M1.5 second wave (type-directed, metamorphic).
- [x] W0 gap map → `meta/m3-gaps.md` (1b0a332).
- [ ] W1 · [ ] W2 · [ ] W3 · [ ] W4 (see [plan.md](plan.md) "M3 execution").

Milestones: M0, M1, M2 accepted (M1 item 6 deferred to after M7). M1.5 foundation committed
(f498713), not ticked.

## What exists (committed)

spec + DECISIONS 1–207; `syntax`, `format`, `jsonsrc`, `wire` (decode + `load.dir`), `check`/
`types`, `eval`/`eval/std` + `value` (bounded memory/time), `verify`, `lock`, `rules`, `ir` (stage
E export fns, fingerprint, Go and C++ name plans), `gen/json`, `gen/go` (baked, data mode, stores,
translated fns, conformance), `gen/cpp` (data mode, stores, runtime, conformance, strict loaders),
`conform`, `load` (`load.dir` of JSON), `build` (whole pipeline, conformance wiring, `cppgen`
registered), `project`, `api` (Check/Build/Test), `cli` (version/init/new/check/build/test),
`internal/testkit` (+ `cxx` toolchain helper, `progen`); `tools/audit`.

## Open Louis-calls

None. Direction questions of the run, if any: `handoff/2026-09-24-questions.md`.

## Owed (feeds A5; full lists in the log)

- **ERRORS.md pass:** a general "cannot generate this construct" code; E8011 variants; E7004/
  E7005 cause vocabularies; W7115 wording; a code for a scalar `-0.0` constant; E8012/E8151 for
  `Define`; `is a {typ}` templates reworded; E8007 names its fix. Then spec sync #2.
- **Cleanups:** translated-fn classification ×3 (check/ir/eval); `internalError` →
  `internal/build/errors.go`; ASCII-fold/inline-fold rule as one ir helper; `osFS.EvalSymlinks`
  → `write.go`; the compile-and-run loop duplicated between two `_test.go` files; the `"$id"`
  wire key has no shared constant (wire vs gen/cpp const-dup); tools/audit: an unmeasured lane
  fails, the lint lane waits for the lock; one fixture FS in testkit (4 copies).

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G` (an uncapped eval
  probe took 24 GB twice and froze the laptop); a watchdog kills any test process over 5 GB.
- Agents share one working tree — **never `git stash`**; parallel units run in worktrees
  (fast-forward, commit there, cherry-pick, `make check` before every continue).
- The golangci-lint cache must be private per worktree (`GOLANGCI_LINT_CACHE`); a stale entry
  shared by content hash across `tools/audit` copies broke `audit-self` once (remedy: `cd
  tools/audit/toolchain && go tool golangci-lint cache clean`).
- Sonnet units that failed review twice moved to opus (gen/go, ir).

## What could not be verified

Windows and macOS real runs; MSVC 19.2x; GCC 9; Clang 10; nlohmann/json 3.9; CI on a runner.
