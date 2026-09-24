# State — Canon compiler

Updated: 2026-09-24, end of day (Louis asked for a clean stop to check the project himself).

## Current focus

**M1 accepted** (items 1–5; item 6 — sovcommon teamboard — deferred by Louis until a full release after M7).
**M2 substantially landed, acceptance mixed** (§6 M2, `spec/IMPLEMENTATION-PLAN.md`):
1. pipeline byte-exact — done, `examples/pipeline/expected/` regenerated and reviewed (GEN-01,
   [handoff/2026-09-24-GEN-01-pipeline-diff.md](handoff/2026-09-24-GEN-01-pipeline-diff.md),
   5cb14f2).
2. C++ builds `-Werror` — done on the local g++/clang++. Louis (2026-09-24): the §7.8 matrix
   (GCC 9, Clang 10, MSVC, nlohmann 3.9) is deferred, not planned; the local toolchain is the gate.
3. generated Go conformance under `-race` — done (gen/go translated fns a0d6373, data mode
   08966cb; `go test -race` green per the GEN-01 handoff's verify log).
4. `canon test` — done (7c00520, ADR-0004 `api.TestResult.Check`).
5. `$schema` refusal + `Reload` keeps the old snapshot — done for both targets (Go and C++
   smoke/compile tests; the GEN-01 handoff's W2 adds `TestStale`).
6. FINGERPRINT.md test vectors — done: `internal/ir/fingerprint_test.go` reads and checks all 10
   vectors of FINGERPRINT.md §7 (serialization, SHA-256, `$schema`).
7. a finding in `data/II_POT_HEAL_L.json` points at the right line/col/pointer — **not pinned by
   a committed test**: no test under `internal/wire`, `internal/jsonsrc` or `internal/testkit`
   asserts a position against that file; `internal/wire/testdata/findings/E7110_*.txtar` use
   synthetic fixtures only. Open for next session.

**M1.5 foundation: committed (f498713).** `internal/testkit/progen/`: rule-mutation and
grammar/corruption suites (type-directed + metamorphic are the second wave), `make progen-nightly`.
73 counterexamples, each `open <owner>`: check 35, format 20, ir 9, syntax 3, build 2, eval 4 —
the next bug-fix wave, one delegation per owner. Final `make check` green on the main tree.

## What exists (committed)

spec + DECISIONS 1–207; `syntax`, `format`, `jsonsrc`, `wire` (decode + `load.dir`), `check`/
`types`, `eval`/`eval/std` + `value` (bounded memory/time), `verify`, `lock`, `rules`, `ir` (stage
E export fns, fingerprint, Go and C++ name plans), `gen/json`, `gen/go` (baked, data mode, stores,
translated fns, conformance), `gen/cpp` (data mode, stores, runtime, conformance, strict loaders),
`conform`, `load` (`load.dir` of JSON), `build` (whole pipeline, conformance wiring, `cppgen`
registered), `project`, `api` (Check/Build/Test), `cli` (version/init/new/check/build/test),
`internal/testkit` (+ `cxx` toolchain helper); `tools/audit`.

## Open Louis-calls

None. Technical gaps and the calls made on them: [decisions/log-2026-09-24.md](decisions/log-2026-09-24.md)
(the orchestrator decides, never asks, DECISIONS 207). Spec synced (15c01ce); ERRORS.md pass
pending (see "Next session" (d)).

## Next session, in order

a. **Owed correctness (loader parity):** Float32 double rounding (C++ decimal→double→float vs
   Go direct), a `-0` Float token (C++ +0.0 vs Go -0.0, one rule), duplicate row ids (neither
   target refuses; must fail the load), the missing-file message (targets still differ).
b. **M1.5 bug-fix wave**, by owner, from the 73 open counterexamples above (check first, largest).
c. **Consumer unit:** switch `gen/go` and `gen/cpp` to take names from the ir name plans (the
   switch-lists in decisions/log-2026-09-24.md, "ir name plans + support plan").
d. **ERRORS.md pass** (after the tree is quiet): a new general "cannot generate this construct"
   code; E8011 variants (override, unexported, derived, reserved-namespace); E7004/E7005 cause
   vocabularies; W7115 wording (dangling/looping link); a code for a scalar `-0.0` constant;
   E8012/E8151 for `Define`. Then spec sync #2 (everything the log marks "at the sync").
e. **Cleanups owed** (briefly; full list in the log): translated-fn classification exists three
   times (check/ir/eval); `internalError` moves to `internal/build/errors.go`; the ASCII-fold/
   inline-fold rule shared as one ir helper; `osFS.EvalSymlinks` moves from `hosts.go` to
   `write.go`; the compile-and-run loop still duplicated between two `_test.go` files (flagged).
f. **M1.5 second wave:** type-directed + metamorphic suites, once the foundation is committed.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G` (an uncapped eval
  probe took 24 GB twice and froze the laptop); a watchdog kills any test process over 5 GB.
- Agents share one working tree — **never `git stash`**; gate each unit in a clean worktree
  (HEAD + its own packages) before commit.
- The golangci-lint cache must be private per worktree (`GOLANGCI_LINT_CACHE`); a stale entry
  shared by content hash across `tools/audit` copies broke `audit-self` once (remedy: `cd
  tools/audit/toolchain && go tool golangci-lint cache clean`).
- Sonnet units that failed review twice moved to opus (gen/go, ir).

## What could not be verified

Windows and macOS real runs; MSVC 19.2x; GCC 9; Clang 10; nlohmann/json 3.9; CI on a runner.
