# Overnight run: finish the open work, then all of M3 (brief written 2026-09-24)

Supersedes `2026-09-24-next-run.md`: its tasks 1–4 are Part A here, and its task 5 is now all of
M3. There is no budget limit. Louis is asleep and **nobody answers questions**. Run until M3's
acceptance (IMPLEMENTATION-PLAN §6 M3) passes, then stop: write the final report and end.

## Standing rules

- Follow CLAUDE.md and `.claude/rules/orchestration.md` to the letter: you orchestrate. `go-dev`
  builds one package per unit, `spec-reviewer` reviews every non-trivial diff, and you run
  `GOTOOLCHAIN=local make check` yourself before every commit. Commit granularly on `main`; never
  push.
- Tiers per orchestration.md: sonnet for goldens, gen, load, cli, views, tests and cleanups; opus
  for check/types, eval, verify, ir semantics, frozen contracts and determinism. A sonnet unit
  that fails review twice moves to opus.
- Every test and probe runs under `systemd-run --user --scope -p MemoryMax=3G`.
- **No questions to anyone.** A technical gap or contradiction is yours to decide: take the strictest
  consistent reading, then log both passages and your choice in `meta/decisions/log-<date>.md`.
  A direction question that can't wait goes to `meta/handoff/<date>-questions.md` with the
  choice you made meanwhile. Keep going.
- **Stuck unit:** after two failed review rounds on opus, stop that unit and write down why in
  `meta/state.md` (what, why, the last diff). Continue with the next unit that doesn't depend on it.
  Never loop on one problem all night.
- **Survive context compaction:** after every committed unit, update `meta/state.md` (focus,
  unit done, next unit) via `docs-updater` or directly, and tick `meta/plan.md`. The files are
  the memory, not your context.
- Louis's calls (log-2026-09-24):
  - C++ gate: the local g++/clang++ only, with nlohmann from `../../Source/External`.
  - Every known bug gets fixed, formatter included. Nothing is deferred.
  - No consumer integration (teamboard included) before a full release after M7.
  - The current resourcestudio is never touched or adapted (DECISIONS 191).
- Read-only outside this directory. `Resource/` data comes from the git-ignored `testdata-real/`
  snapshot (43 MB, `testdata-real/RESOURCE_COMMIT`). Never commit anything from it.

## Part A: close the open work (in order)

**A1. Loader parity (runtime safety).** The generated Go and C++ loaders must refuse duplicate
row ids in a data file (today neither does). In the same units, fix the other three differences
between the targets:
- Float32 double rounding: C++ decimal→double→float vs Go direct.
- `-0` Float token: one rule for both targets; log it.
- The missing-file message.

Two parallel units (`gen/go`, `gen/cpp`; sonnet, opus review), with conformance tests proving the
targets agree.

**A2. Demo bugs of 2026-09-24.** Repro: a package with `enum Rarity { common, rare, epic }`, a
record `Item { rarity: Rarity, price: Int, warn cheap_epic: not (rarity == epic and price < 1_000)
else "..." }`, loaded by `load.dir`.
1. `rarity == legendary` (an unknown enum member in a rule) → internal compiler error, exit 3.
   It must be a `check` finding. Write the failing test first, and fix the class, not the instance.
2. A broken JSON file (`{"id": "IT_X", "price": `) gives `E7109 JSON syntax error: ""`, with an
   empty detail.
3. Decode errors (unknown enum value, unknown field) hide the range and regex errors of the same
   file. Follow the spec's no-cascade rules: fix it, or log why it stays.
4. "is **a** Int"; the E4402 trace prints 16 identical frames. Collapse the repeats if the spec
   allows it.
5. E8007 (Go emit without a `go_module` root) should hint at the fix if ERRORS.md allows it;
   otherwise note it for the ERRORS.md pass.

**A3. M1.5 bug-fix wave.** All 73 counterexamples in
`internal/testkit/progen/testdata/counterexamples/` (check 35, format 20, ir 9, eval 4,
syntax 3, build 2):
- One `go-dev` per owner package: opus for check and eval, sonnet for the rest.
- Every fix gets a regression test; archives are marked fixed per the progen docs.
- Order: check, eval, format, ir, syntax, build.
- Afterwards, `make progen-nightly` once. Anything new goes into this same wave.

**A4. M2 close.** Commit a test that pins line, column and JSON pointer of a finding in
`examples/pipeline/data/II_POT_HEAL_L.json`. Tick M2 in `meta/plan.md`.

**A5. Owed from `meta/state.md` "Next session":**
- (c) the consumer unit: gen/go and gen/cpp take names from the ir name plans;
- (e) the cleanups;
- (d) the ERRORS.md pass plus spec sync #2. This is the orchestrator's work per DECISIONS 207, and
  the new M3 codes will join it at the end of M3.

**A6. M1.5 second wave:** the type-directed and metamorphic suites (QA). Fix what they find,
then tick M1.5 when its acceptance holds.

## Part B: M3, as planned in `meta/plan.md` "M3 execution"

Follow the waves W0–W4 written there exactly: one builder per package at a time, at most 3 in
parallel on disjoint packages (worktrees: fast-forward, cherry-pick, `make check` before every
continue). After each wave: an integration cleanup, the progen rule-mutation rerun, and a commit.
- **W0** gap map → `meta/m3-gaps.md`. Scope every later unit from it; don't rebuild what M1
  already built (layers, amend, inputs exist in check/eval).
- **W1** load forms ∥ dependent types ∥ layers/inputs completion + explain provenance.
- **W2** dependent verification + assets ∥ view/translation checking ∥ unions + `LoadInputs`
  in gen ∥ `types` mode in ir.
- **W3** views/i18n/gen/view ∥ C++ `types` mode + TS data ∥ api Check/Value/ViewModel + explain.
- **W4** mutation operators for every new diagnostic code; `make check-real` over
  `testdata-real/`; all §6 M3 acceptance items.

**Real-data findings** (acceptance 7) are a **list for Louis only**, grouped by code and by file
(counts plus a few examples each), in `meta/handoff/<date>-realdata-findings.md`. Propose no
fixes. The law never bends to fit the data: a rule that real data breaks massively is flagged in
the list, not relaxed. The example packages' types are not edited to make real data pass (for
example, to accept a legacy sentinel like `0` meaning "no item" in a `ref` field). Louis trusts the Canon
types over the data (`Resource/` can be very wrong). So for each massive pattern, an opus
`spec-reviewer` pass gives a verdict, "data wrong" (the default) or "type questionable" with the
reason. The list feeds the future Resource migration plan (log-2026-09-24). List it, then move on. The spec is final: build
it as written, and put an implementation concern about a feature in the report; never cut or
simplify the feature.

## End of run

When M3's acceptance passes (or nothing unblocked remains):
1. Run `GOTOOLCHAIN=local make check` and `go test -race ./...` (capped) one last time, and keep
   the log.
2. Update `meta/state.md` and `meta/plan.md`; write any ADRs owed.
3. Write `meta/handoff/<date>-overnight-report.md` for Louis:
   - what landed, with SHAs, per part and wave;
   - what is open or blocked, and why;
   - every decision taken, with links into the log;
   - direction questions waiting for him;
   - what could not be verified.
4. Commit, and end. Don't start M4.
