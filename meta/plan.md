# Plan — compiler v0.1 (M0–M7)

The order of work, condensed from [spec/IMPLEMENTATION-PLAN.md](../spec/IMPLEMENTATION-PLAN.md)
§5–§7, which stays the authority (owners §5.1, phases §5.2, acceptance §6). Owners are the
plan's agent roles; each step names the package(s) it owns. Tick a box only when its acceptance
passes and `make check` is green. **Every milestone also requires** `make check` green, the
`maprange` analyzer clean, `go test -race ./...` green, the determinism job green (§7.5), and
every new diagnostic code tested (§7.2). Never start a step whose gate is not met: record the
block in [state.md](state.md).

**Gate for M0:** the consistency pass (`meta/spec-phase/review/CONSISTENCY-TODO.md`) is closed and v0.1 locked.

## M0 — Contracts

- [x] **M0.1 Audit rules first** (QA, `tools/audit`; addendum, DECISIONS 26–27):
  `diag-message-inline` (enforce), `diag-code-untested` (ratchet), `ignore-count` ratcheted,
  thresholds moved to `tools/audit/thresholds.tsv`. Accept: rules.md in sync, the tool gates
  itself, `make check` green.
- [x] **M0.2 Module skeleton** (QA): every package of §3 with `doc.go`, an Example test,
  `constants.go`/`errors.go` where needed; `internal/testkit/deps_test.go` enforcing §3's
  dependency rule. Accept: `go list -deps` matches the allowed graph.
- [x] **M0.3 Registry** (QA, `internal/diag`): `codes.go` generated from `spec/ERRORS.md`,
  diff-checked by `make check`. Accept: ERRORS.md regenerated from the registry is unchanged.
- [x] **M0.4 The eight frozen contracts** (§4), as compiling Go, each approved by its consumers:
  `source.go` + `ast.go` (SYN), `types.go` and `check/info.go` (TYP), `value.go` and
  `eval/host.go` (EVL), `diag.go` (QA), `ir.go` (IR), `edit/path.go` (API); lock format types
  (VER), `project` schema types (LOD).
- [x] **M0.5 API split** (API, `api/`): `api/canon.go` split by concern per §12.4, no API change.
- [x] **M0.6 Harness** (QA, `internal/testkit`): golden harness skeleton running one trivial
  golden; fixture extractor `fixturegen`; CI skeleton running `make check`.

## M1 — v0: `taxonomy.canon` to baked Go and JSON

- [x] Parser for every example, AST goldens (SYN, `syntax`). Accept: no `E11xx`, dumps equal
  `internal/syntax/testdata/ast/`.
- [x] Resolver + checker (TYP, `check`); evaluator + stdlib subset (EVL, `eval`); verify, lock,
  rules (VER); `project`, `wire` encode (LOD); `ir`, `gen/json`, `build` (IR); `gen/go` baked
  (GO); `cli` check/build/version/init/new (API); `goldens-check` wired in the Makefile (QA).
- [x] Accept (§6 M1): `canon check teamboard` prints its `findings.txt`; `canon build teamboard
  sovcommon...` equals the goldens and a new `expected/MANIFEST`; generated Go builds on the current
  Go (DECISIONS 205); `canon.lock` equals its golden, `E6001` on delete/rename, retire passes;
  the sovcommon `teamboard` integration is deferred by Louis until a full release after M7 (log-2026-09-24).

## M1.5 — Generated-program testing (QA, DECISIONS 200)

- [ ] `internal/testkit` program generator (grammar- and type-directed, seeded, shrinking) and
  the four nightly suites: rule mutation (every ERRORS.md rule), grammar + token mutation,
  well-typed programs (check ⇒ build ⇒ Go compiles ⇒ equals the evaluator), metamorphic.
  Accept: every rule has a mutation operator; one nightly run clean under a 3 GB cap; each
  counterexample kept as a txtar. Budget ~3k lines. Pulled forward, runs beside M2 (206).
  Its suites clean + conformance green = the gate before any consumer integration.
  **Foundation committed** (f498713, 2026-09-24): generator, shrinker, the rule-mutation and
  grammar/corruption suites; 73 open counterexamples (see [state.md](state.md)). Not ticked:
  the counterexamples must be fixed by their owners and the second wave (type-directed,
  metamorphic) built before this box ticks.

## M2 — Pipeline: data mode for Go and C++

- [x] First step: regenerate `examples/pipeline/expected/` (GEN-01); the orchestrator reviews the
  diff and continues, the diff is listed in `meta/handoff/` for Louis (DECISIONS 190). Done
  5cb14f2 ([handoff/2026-09-24-GEN-01-pipeline-diff.md](handoff/2026-09-24-GEN-01-pipeline-diff.md)).
- [x] `jsonsrc` (SYN), `wire` decode + `load.dir` (LOD), `conform` (EVL), fingerprint + reload IR
  (IR), data mode + stores + conformance (GO, CPP), `cli test` (API). All landed 2026-09-24
  (see [state.md](state.md) "What exists").
- [ ] Accept (§6 M2): pipeline byte-exact (done); C++ builds with `-Werror`
  (done on the local g++/clang++; the §7.8 matrix is deferred by Louis, 2026-09-24); conformance
  green with `-race` (done); `canon test` output format (done); fingerprint refusal (done);
  FINGERPRINT.md vectors (done, `internal/ir/fingerprint_test.go`); finding positions in JSON
  sources (open — no committed test pins `data/II_POT_HEAL_L.json`'s position). Box ticks once
  every sub-item does.

## M3 — Load and the view model

- [ ] Every `load` form (LOD); dependent types, views, i18n, layers (TYP, VM: `views`,
  `gen/view`, `i18n`, `api/vm`); C++ `types` mode (CPP); `Check`/`Value`/`ViewModel` (API);
  real-data job + `make check-real` over `testdata-real/` (QA).
- [ ] Accept (§6 M3): every example prints its `findings.txt`; view models validate against the
  schema and equal goldens; `balance.parity` golden; `canon explain` golden; C++ `types` decode.

**M3 execution (agreed with Louis 2026-09-24).** Layers/amend/inputs already exist in
`check`/`eval` (M1), and dependent types are partly in `check`/`types`, so no unit rebuilds from the spec
blindly. Waves; one builder per package at a time, ≤ 3 in parallel on disjoint packages
(worktrees); review + `make check` + commit per unit; cleanup + progen rule-mutation rerun per
wave; every funded run ends on a wave boundary with a report in `meta/handoff/`.
- [ ] **W0 gap map** (sonnet, read-only): per M3 feature, spec rule ids × implemented × tested,
  and what each example fails on today → `meta/m3-gaps.md`. Every later unit is scoped from it.
- [ ] **W1** load forms (`load`, `wire`, drop `build.ErrLoad`; sonnet) ∥ dependent types
  (`types`, `check`; opus) ∥ layers + inputs completion and explain provenance (`eval`; opus).
- [ ] **W2** dependent verification + assets (`verify`; opus) ∥ view and translation checking
  (`check`, after W1's check unit; opus) ∥ unions + `LoadInputs` (`gen/go` then `gen/cpp`;
  sonnet) ∥ `types` mode in `ir` (opus).
- [ ] **W3** `views`, `i18n`, `gen/view` (sonnet, schema + goldens) ∥ C++ `types` mode and TS
  data (`gen/cpp`, `gen/ts`; sonnet) ∥ `api` Check/Value/ViewModel, `api/vm`, `cli explain`
  (opus: frozen contract).
- [ ] **W4 acceptance**: progen mutation operators for every new code; real-data run; every
  example equals its `findings.txt`. Real-data findings in `Resource/` go to Louis as a **list
  only** (`meta/handoff/<date>-realdata-findings.md`), no fixes proposed, not gating.

## M4 — Formatter and the edit API

- [ ] `format` (SYN), incremental memo (EVL, API), `edit`, `workspace`, full `api` (API).
- [ ] Accept (§6 M4): examples are `fmt` fixed points, 10-minute fuzz clean; every API.md rule
  tested; minimal-write invariant under fuzzing; NFR-01 targets (§7.6); crash and race stress
  tests; studio integration spike postponed (DECISIONS 191: a new studio will be built).

## M5, M6, M7 — in parallel once M4 is accepted

- [ ] **M5 LSP** (LSP, `lsp`, `editors/vscode`): txtar transcripts, UTF-16, 500 ms diagnostics.
- [ ] **M6 Legacy C++ and TypeScript** (CPP, TS; may start after M3): `legacycpp` in three modes;
  TS goldens pass `tsc --strict` and `node --test`; `E8101` tested.
- [ ] **M7 Migration** (MIG, `convert`; VM for `i18n stub|status`; `infer` dropped, DECISIONS
  188): convert proof; `i18n stub fr` golden.
- [ ] **Resource migration plan** (owed before or with M7; Louis, log-2026-09-24): how dirty
  `Resource/` data is brought to the Canon types (types win over data), built from the
  real-data findings lists of M3 onward.

## Feature examples owed (§7.9, QA)

`entries` (before M1), `pairs` (before M2), `edits` (before M4), `ts` (before M6).
