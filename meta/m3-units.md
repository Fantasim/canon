# M3 remaining units (2026-09-28)

From the read-only planner's scoping at 7be65fb; calls in
[decisions/log-2026-09-28.md](decisions/log-2026-09-28.md). ★ = critical path. Tier in brackets.
Tick with the landing SHA.

## Wave 1 (in flight at the start)

- [x] check follow-ups (2900de3): E3806 at a parameter declaration, union readings, progen cascades [opus]
- [x] verify (bd2c5b6): dependent verification E3801/E3802/E3501/E3322, resolved values to outputs [opus]
- [x] pattern automaton (2b24d52, 8b5210b, 7df809a): ir table + gen/cpp iterative matcher [opus]
- [x] gen consumers' dependent refusals (31dab01) (gen/go, gen/cpp, ir), all-Never → E8019, ir alias-chain
      patterns, `LoadInputs` hiding scope, example golden [opus] (after the three above)

## View model, i18n, API

- [x] U1 (cda1581) `api/vm` + `api/vm/internal/vmgen` + `vm-check` gate; ADR on union flattening [opus]
- [x] U2 (44f0770) `internal/testkit/jsonschema` subset validator (26 keywords, unknown keyword fails) [sonnet]
- [x] U3 (f251383) golden harness finds nested examples; `balance.parity` MANIFEST + golden [sonnet]
- [x] U4a (a256213) `build` analysis handle (Program, Evaluator, bags, findings) for api/edit/views [sonnet]
- [x] U4b (846e216) `edit.Snapshot`/`Resolve` (API §6 P1–P10) + editability (API §7) [opus]
- [x] ★U5 (844d07d) `check`: resolve/type views and translation files; E1703, E1633 [opus]
- [x] ★U6 (9c10bac, tick 90480f0) `i18n`: catalogue, translation files, W1701, E1702, E1704–E1707 [sonnet]
- [x] ★U7 (8873caa) `views` V1: static checks E1601–E1632/E1634, W16xx [opus]
- [x] ★U8 (1ec276d) wire i18n/views checks into `build`; generic findings test over every example [sonnet]
- [x] U9 (dfba53b) `gen/view`: bytes from `*vm.ViewModel` (J1–J3, J10, WIRE §7) [sonnet]
- [x] ★U10 (a1663da, tick 03859a8) `views` V2: `types` and controls (VIEWMODEL §4, §6, §12.3, §12.5) [sonnet→opus]
- [x] ★U11 (d06484d) `views` V3: the rest of the model (§5, §7–§10, §12.4, §12.6–§12.11) [sonnet]
- [x] ★U12 (c7ae4e2) `build` writes the `view` target in phase 8, even with errors [sonnet]
- [x] ★U13 (e7f2e74) view goldens (pipeline, farm, events) + schema validation; GEN-01 view diff [sonnet]
- [x] U14 (d20ec40) `api.Value` + `Origin.Replaced` (ADR-0007) + `cli explain` golden [opus]
- [x] U15 (fc86348, tick ab8dc52) `api.ViewModel` (JSON equals the emit bytes) [opus]
- [x] U16 (c368908, tick c920f92) `gen/cpp` `types` mode; events compiles, Decode accepts EventConfig.json [sonnet]
- [x] U17 (c368908, tick c920f92) `ir` `types` mode rules mirroring U16's refusals [opus]
- [x] U18 (bb2b756) `make check-real` (not gating) + realdata findings list [sonnet]
- [x] U19 (df4a3dc, tick 12fb95c) `benchgen` (IMPLEMENTATION-PLAN §7.6) [sonnet]
- [x] U20 (1eaf9b5) progen operators for every new code [sonnet]
- [x] U21 spec syncs of the logged calls [orchestrator / docs] — every call in
      [decisions/log-2026-09-24.md](decisions/log-2026-09-24.md) through
      [decisions/log-2026-09-29.md](decisions/log-2026-09-29.md) was synced into the spec as it
      was made (each log entry cross-references its own `docs(spec):` commit).
- [~] Cleanup wave A landed (f9802da); rest: run-2 NITs + cleanup list (handoff/2026-09-25-cloud-run-2.md)

## Hardening after acceptance (M3 acceptance items already green; landed after the ledger above)

- check follow-ups 3 (3aebf22): E2111, E3317 on literals and computed maps, variant-level
  methods, cascades.
- check follow-ups 4 (fb445ac): variant case filter, G16 qualified studio names, `@menu` names,
  `T??` recovery, error-typed fields not required.
- Variant-level methods end to end (84e6394): `eval`, `ir`, `rules`, `i18n` (DECISIONS 226).
- Alias-chain patterns (c9a56e7): every pattern of an input's alias chain checked at load
  (DECISIONS 225, ADR-0008).
- No `E8011` cascade / no `E1119` after an empty interpolation (94f3ef2).
- Progen triage at nightly size (fe54e03): 8 operator fixes, 3 open compiler archives.
- Broken views and translations decided by `check` (754a7eb): `Info.BrokenViews`,
  `Info.BrokenTranslations` (DECISIONS 228, ADR-0009).
- Drivers across the project (ea3d7e2): type-function drivers cover every package of the
  project, not only the selection (VIEWMODEL §12.3, J5, API R9).
- Cleanup wave (0955fac): one job splitter (`check`), one `Unparen`, one UTF-8/BOM check across
  `check`, `syntax`, `edit`, `eval`, `ir`, `jsonsrc`, `load`.
- libc++ in the C++ test matrix; Windows (MSVC `cl.exe`) and macOS CI jobs added to
  `.github/workflows/check.yml` (bc559e3) — not yet run (fire once Louis pushes).

## Order (≤ 3 builders at once, disjoint packages)

1. U1 ∥ U2 ∥ U3 → U4a ∥ U19 ∥ U18 → ★U5 ∥ U4b ∥ U9 → ★U6 ∥ ★U7 ∥ U16 → ★U8 ∥ U17 →
   ★U10 ∥ U20 → ★U11 ∥ U14 → ★U12 → ★U13 ∥ U15.
