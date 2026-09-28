# M3 remaining units (2026-09-28)

From the read-only planner's scoping at 7be65fb; calls in
[decisions/log-2026-09-28.md](decisions/log-2026-09-28.md). ★ = critical path. Tier in brackets.
Tick with the landing SHA.

## Wave 1 (in flight at the start)

- [ ] check follow-ups: E3806 at a parameter declaration, union readings, progen cascades [opus]
- [ ] verify: dependent verification E3801/E3802/E3501/E3322, resolved values to outputs [opus]
- [x] pattern automaton (2b24d52, 8b5210b, 7df809a): ir table + gen/cpp iterative matcher [opus]
- [ ] gen consumers' dependent refusals (gen/go, gen/cpp, ir), all-Never → E8019, ir alias-chain
      patterns, `LoadInputs` hiding scope, example golden [opus] (after the three above)

## View model, i18n, API

- [x] U1 (cda1581) `api/vm` + `api/vm/internal/vmgen` + `vm-check` gate; ADR on union flattening [opus]
- [ ] U2 `internal/testkit/jsonschema` subset validator (26 keywords, unknown keyword fails) [sonnet]
- [x] U3 (f251383) golden harness finds nested examples; `balance.parity` MANIFEST + golden [sonnet]
- [x] U4a (a256213) `build` analysis handle (Program, Evaluator, bags, findings) for api/edit/views [sonnet]
- [ ] U4b `edit.Snapshot`/`Resolve` (API §6 P1–P10) + editability (API §7) [opus]
- [ ] ★U5 `check`: resolve/type views and translation files; E1703, E1633 [opus]
- [ ] ★U6 `i18n`: catalogue, translation files, W1701, E1702, E1704–E1707 [sonnet]
- [ ] ★U7 `views` V1: static checks E1601–E1632/E1634, W16xx [opus]
- [ ] ★U8 wire i18n/views checks into `build`; generic findings test over every example [sonnet]
- [ ] U9 `gen/view`: bytes from `*vm.ViewModel` (J1–J3, J10, WIRE §7) [sonnet]
- [ ] ★U10 `views` V2: `types` and controls (VIEWMODEL §4, §6, §12.3, §12.5) [sonnet→opus]
- [ ] ★U11 `views` V3: the rest of the model (§5, §7–§10, §12.4, §12.6–§12.11) [sonnet]
- [ ] ★U12 `build` writes the `view` target in phase 8, even with errors [sonnet]
- [ ] ★U13 view goldens (pipeline, farm, events) + schema validation; GEN-01 view diff [sonnet]
- [ ] U14 `api.Value` + `Origin.Replaced` (ADR-0007) + `cli explain` golden [opus]
- [ ] U15 `api.ViewModel` (JSON equals the emit bytes) [opus]
- [ ] U16 `gen/cpp` `types` mode; events compiles, Decode accepts EventConfig.json [sonnet]
- [ ] U17 `ir` `types` mode rules mirroring U16's refusals [opus]
- [x] U18 (bb2b756) `make check-real` (not gating) + realdata findings list [sonnet]
- [ ] U19 `benchgen` (IMPLEMENTATION-PLAN §7.6) [sonnet]
- [ ] U20 progen operators for every new code [sonnet]
- [ ] U21 spec syncs of the logged calls [orchestrator / docs]
- [ ] Cleanup wave: run-2 NITs + cleanup list (handoff/2026-09-25-cloud-run-2.md)

## Order (≤ 3 builders at once, disjoint packages)

1. U1 ∥ U2 ∥ U3 → U4a ∥ U19 ∥ U18 → ★U5 ∥ U4b ∥ U9 → ★U6 ∥ ★U7 ∥ U16 → ★U8 ∥ U17 →
   ★U10 ∥ U20 → ★U11 ∥ U14 → ★U12 → ★U13 ∥ U15.
