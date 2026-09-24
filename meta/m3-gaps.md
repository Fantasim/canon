# M3 gap map — Load and the view model

Date: 2026-09-24. Base SHA: `1e56fd898df4c309fc8774cf4e81dc80b3047a58`.

Read-only recon (W0, IMPLEMENTATION-PLAN.md §6 M3, plan.md "M3 execution"). Method: built
`/tmp/canon-w0` (`go build ./cmd/canon`), ran `canon check <pkg>` for every example package with
every `project.canon` root redirected the way `internal/cli/examples_test.go` and
`internal/testkit/golden/examples_test.go` do, diffed stdout against `expected/findings.txt`;
grepped the registry (`internal/diag/codes.go`) against every non-`diag` package for M3-range
codes never constructed; read the owning packages named MISSING below to confirm (not assumed
from grep alone).

## §1 M3 acceptance items (IMPLEMENTATION-PLAN.md §6, verbatim) — status

1. *"Every example package, checked against `examples/_fixtures`, prints exactly its
   `expected/findings.txt`."* — **13 of 24 example packages fail wholesale** with
   `build.ErrLoad` ("load is not supported by this compiler yet: a load form other than
   load.dir"), because `internal/load/loader.go`'s `Load` refuses every method but `"dir"`.
   2 packages (`sovcommon.time`, `studio`) differ only by a missing `W1701` (i18n status)
   finding — i18n checking is not built. 9 pass today (`teamboard`, `pipeline`,
   `features.{lookup,matching,retirement,warns}`, `service.resourcestudio`,
   `sovcommon.{roles,ui}`) — see §2.
2. *"The view models of `farm`, `events` and `pipeline` validate against
   `spec/viewmodel.schema.json` and equal their goldens."* — **MISSING.** `internal/views`,
   `internal/gen/view`, `api/vm` are M0 skeletons (`doc.go` only, no logic).
   `internal/build/constants.go:80` wires `generators[ir.TargetView] = nil` — building the
   `view` target is a guaranteed nil-generator failure today. `farm` and `events` are also both
   blocked by item 1 (both force non-`load.dir` forms). `pipeline/expected/potion.view.json`
   exists as a pre-drafted golden, explicitly held per `Makefile` comment: *"until M3 wires
   gen/view, VIEWMODEL.md's golden link and GEN-01"* — it is the concrete target shape for the
   `gen/view` unit, not yet verified against real output.
3. *"`canon build balance.parity` output equals its golden."* — **MISSING.**
   `balance/parity/sweep_plan.canon` uses only `load.defines` (LOD-01, three calls, C header
   parsing) — 100% unimplemented; `expected/` has `findings.txt` but no `MANIFEST`, so no build
   golden exists yet to equal.
4. *"`canon explain config.server.port --layer louis` prints its golden (CLI.md §3.7
   format)."* — **MISSING.** `grep -rn explain internal/cli` finds no command; `cli.Main`'s
   usage text (confirmed by running `--help`) lists only `build check init new test version` —
   no `explain`.
5. *"`api.ViewModel(pkg).JSON()` equals the `emit view` file for every example with one."* —
   **MISSING.** `api/value.go:161` `Project.ViewModel` is `return nil, errUnimplemented()`; only
   the struct shape (`api.ViewModel`, from M0's frozen contract) exists.
6. *"The C++ `types`-mode output of `events` compiles and `Decode` accepts the fixture
   `EventConfig.json`."* — **MISSING.** `internal/ir` already has `ModeTypes` wired into its
   emit-validation stage (`rules_emit.go`: `modeRules[ModeTypes] = [...]{checkDecoders,
   checkTypesMode}`), but `internal/gen/cpp` has zero code that reads `ModeTypes` beyond an
   error-message name table (`errors.go:59`) — no `types`-mode C++ is generated. `events` is
   also blocked by item 1 (`load.dir` of non-JSON / dependent-typed files, see §3).
7. *"Real-data job (§7.3) runs `canon check resource... balance...` on the real `Resource/`
   tree ... stored as `realdata/findings.txt` (not gating)."* — **MISSING.** No `check-real` or
   `testdata-real` target in `Makefile` (`grep -n check-real Makefile` empty).

**None of the seven acceptance items pass today.** Items 1's failure is dominated by one root
cause (load forms); items 2, 4, 5, 6 share the same second root cause (views/i18n/api layer is
an M0 skeleton); item 3 is load.defines specifically; item 7 has no harness at all.

## §2 Per-example status

Run: `canon check <pkg>` from `examples/`, roots redirected exactly as
`internal/cli/examples_test.go`'s `exampleRoots` (resource/client → `_fixtures`; source,
services, sovcommon, web, parity, generated → a scratch dir). Findings compared to
`expected/findings.txt` (duration normalized to `(…)`).

| Example | Exit | vs golden | First difference |
|---|---|---|---|
| `teamboard` | 0 | **EQUAL** | — |
| `pipeline` | 0 | **EQUAL** | — |
| `balance.parity` | 2 | DIFFERS | `build.ErrLoad`: `sweep_plan.canon:16:18` (`load.defines`) |
| `features.codes` | 2 | DIFFERS | `build.ErrLoad`: `codes.canon:26:42` |
| `features.csv` | 2 | DIFFERS | `build.ErrLoad`: `csv.canon:20:41` (`load.csv`) |
| `features.embedded` | 2 | DIFFERS | `build.ErrLoad`: `embedded.canon:19:42` |
| `features.legacycpp` | 2 | DIFFERS | `build.ErrLoad`: `legacycpp.canon:10:19` |
| `features.lookup` | 0 | **EQUAL** | — (no `load` call; untested by this milestone's forms) |
| `features.matching` | 0 | **EQUAL** | — |
| `features.retirement` | 0 | **EQUAL** | — |
| `features.text` | 2 | DIFFERS | `build.ErrLoad`: `text.canon:10:33` (`load.text`) |
| `features.warns` | 0 | **EQUAL** | — |
| `game.items` | 2 | DIFFERS | `build.ErrLoad`: `item.canon:19:18`; has `item.view.canon` + `item.fr.canon` too |
| `resource.adventurequest` | 2 | DIFFERS | `build.ErrLoad`: `adventurequest.canon:198:3` |
| `resource.events` | 2 | DIFFERS | `build.ErrLoad`: `event.canon:18:22`; has 2 `view` blocks — an M3 acceptance-item-2 target |
| `resource.farm` | 2 | DIFFERS | `build.ErrLoad`: `farm.canon:15:19`; has `farm.view.canon` (6 `view` blocks) + `farm.fr.canon` — the other acceptance-item-2 target |
| `resource.heistia` | 2 | DIFFERS | `build.ErrLoad`: `heistia.canon:66:30` |
| `resource.rules` | 2 | DIFFERS | `build.ErrLoad`: `rules.canon:15:18` |
| `resource.vocab` | 2 | DIFFERS | `build.ErrLoad`: `vocab.canon:16:22` |
| `service.resourcestudio` | 0 | **EQUAL** | — |
| `sovcommon.roles` | 0 | **EQUAL** | — |
| `sovcommon.time` | 0 | DIFFERS | golden expects `warning[W1701]` (20 texts, no fr) + summary "1 warning"; got "0 warnings" |
| `sovcommon.ui` | 0 | **EQUAL** | — |
| `studio` | 0 | DIFFERS | golden expects `warning[W1701]` (13 texts, no fr); got "0 warnings" |

The 13 `build.ErrLoad` packages are exactly `internal/cli/examples_test.go`'s `loadExamples` map
(that map, and its sibling test `TestExamplesLoadUntilM3`, are the wholesale-refusal contract
that must be dismantled example-by-example as each load form lands — not a general test to
delete in one shot, since each entry leaves that map only once its own form is read).

## §3 Per-feature rule tables

Legend: file:func cites the implementation; "UNTESTED" = code exists but no test/golden
exercises it; "MISSING" = confirmed absent by reading the file, not just by an empty grep.

### Load forms (WIRE.md §6, LOD-01..08)

| Form / rule | Spec | Implemented | Tested | Notes |
|---|---|---|---|---|
| `load.dir` (JSON) | §6.5, LOD-05 | `internal/load/dir.go:dir` | `internal/load/dir_test.go`, `internal/testkit/golden` (pipeline) | Only its bare one-string-literal-argument form: `dirPattern` (dir.go:209) refuses any named arg outright — no `at:`, `partial:`, `format:` option reaches this form yet. |
| Format detection | §6.2, LOD-08 | `internal/load/dir.go:formatOf` | `dir_test.go` | JSON only decoded (`checkFormats`, dir.go:91, refuses csv/text matches with `unsupported`); extension-based, case-insensitive, done for the 3 known extensions. |
| `at:` path | §6.3, LOD-03 | **MISSING** | — | `diag.E7106` ("an `at:` path that does not match the document") has zero call sites outside `internal/diag`. `check/load.go:loadArgs` types `at:` as a String constant (generic default branch) but nothing in `load` reads it — `dirPattern` rejects any named arg. |
| `partial: true` | §6.4 | Checker-side only | — | `check/load.go:hasHeader`/`loadArgs` recognize `optionPartial` as a Bool-typed option (typed, not acted on); `load` package has no partial-read logic. |
| `load.csv` | §6.6, LOD-06 | **MISSING** (loader) | Checker-side tested | `internal/load/loader.go:Load` returns `unsupported` for any method ≠ `"dir"`. `check/load.go:load` already types `load.csv(header: true/false)` against `[[String]]` or a collection (E7116 on mismatch) — see LOD unit note below. `diag.E7113` ("malformed CSV") has zero call sites: CSV parsing itself doesn't exist. |
| `load.text` | §6.7, LOD-07 | **MISSING** (loader) | Checker-side tested | `check/load.go:loadText` types it against `String` already; `load` package has no text-file reader. |
| `load.defines` | §6.8, LOD-01 | **MISSING** (loader) | Checker-side tested | `check/load.go:load` returns `TableType{Elem: DefineType}` for `loadDefines` already (typed); no C-header `#define` parser exists anywhere under `internal/load`. This is `balance.parity`'s and 5+ `features`/`resource` examples' blocker. |
| Bare `load(...)` | §6.1 | **MISSING** (loader) | — | `loader.Load` refuses any `e.Method == nil` too (falls into the same `!= methodDir` branch). |
| Globs (`**`, `?`, `[...]`, `{...}`) | §6.5 | `internal/load/glob.go` | `glob_internal_test.go` | Built for `load.dir`'s pattern only; validated (`validateGlob`), not yet exercised by the still-unsupported forms. |
| Dependent/ref/variant/map element types through `load.dir` | §6.5 (DECISIONS 173) | Deliberately gated off | `internal/load/supported_internal_test.go` | `internal/load/supported.go:supported` refuses any ref, variant, map or dependent-default field — `unsupported("an element type this milestone cannot decode without the evaluator")`. This is a **named, intentional M2 boundary** (DECISIONS 173 / "load.dir review (M2)" in the decisions log), not a bug; M3's LOD unit removes it deliberately, coordinated with EVL/verify since decoding then needs the evaluator. |

### Dependent types, unions, assets (TYPES.md §11, §13.2, §13.4)

Existing foundation — **do not rebuild**:
- `internal/types/dependent.go`: `TypeAppType` (`F(args)`, DEP-02), `DepUnionType` (static view,
  DEP-01).
- `internal/check/typefunc.go:applyTypeFunc` (§11.1), `domain` (dependent maps, §11.5).
- `internal/check/maps.go:mapLit` (dependent maps, §5.2); `internal/check/convert.go:convertDepMap`,
  `internal/types/judge.go:assignToDepMap`/`sameTypeFunc`/`storesDependent` (assignability, TYP-18,
  TYP-02).
- `internal/check/chain.go:depName`/`unknownMember` (E3804 on a dependent value's field).
- `internal/check/ops.go:dependent`/`contextDependent` (operator refusal on dependent values,
  TYPES.md §11.4).
- String-literal unions: `internal/check/typeexpr.go` (comment: "union over one of these, or a
  dependent value") — present but not separately audited this pass; TYP-09 owner.
- Assets (TYPES.md §13.4, TYP-21): `types.AssetSpec`; verification in
  `internal/verify/refine.go` reports `E3701`/`E3702`/`E3703` (root/extension/path-not-clean);
  `check/refine.go` reports `E3704` (asset root not a load path). **Already mutation-tested**:
  `internal/testkit/progen/ops_values_test.go` has operators for all four codes citing
  "TYPES.md §13.4". Assets are functionally the most complete M3 feature.
- `internal/eval/host.go`'s dependent-type evaluation seam (DEP-02, §11.6) — not read this pass
  in full; flag for the TYP/EVL unit to confirm branch selection at eval time, not just typing.

What's open: whether §11's rules are *fully* covered (no rule-id-by-rule-id table was built —
budget did not allow reading TYPES.md §11 line by line against every check/*.go site); the W1
delegation should have its own agent read TYPES.md §11 in full against these files and produce
the rule-by-rule list, since dependent types are "partly" done per `meta/state.md`, not
"unstarted."

### Verification and inputs already landed (do not rebuild)

- Layers (EVALUATION.md §9, LAY-01..03): `internal/eval/layers.go` + `layers_test.go` exist.
- Runtime inputs (EVALUATION.md §11, LAY-04, TYP-17): `internal/check/inputs.go` (168+ lines,
  `dependent type anywhere in p`, `holdsIn`); codes `E1901` (build) and `E1902`-`E1911` (check)
  are each used in 2-4 files already — **not MISSING**, contrary to a naive "E19xx is a gap"
  read of the registry.
- `LoadInputs` for generated code: **MISSING** in both targets —
  `grep -rln LoadInputs internal/gen/go internal/gen/cpp` is empty. CODEGEN.md §5.12 "Runtime
  inputs" codegen does not exist yet; this is exactly W2's "unions + `LoadInputs` (`gen/go` then
  `gen/cpp`)" line.

### Views (VIEWMODEL.md)

**Entirely unbuilt.** `internal/views/` is `doc.go` only. Every `E16xx` code (`E1601`-`E1634`,
31 codes, all `Package: "views"` in the registry) has zero call sites outside `internal/diag`.
Two examples already carry real view syntax to check against: `resource/farm/farm.view.canon`
(6 `view` blocks — sections, groups, `_other`, widgets per a skim of VIEWMODEL.md §5) and
`resource/events/event.canon` (2 `view` blocks) and `pipeline/potion.canon` (1, matching the
pre-drafted `potion.view.json` golden) and `game/items/item.view.canon`.

### Translations / i18n (I18N.md)

**Entirely unbuilt.** `internal/i18n/` is `doc.go` only. `E1702`-`E1707` (6 codes) unused.
`W1701` ("missing translations") has exactly one reference in the whole tree —
`internal/cli/examples_test.go:157`, which only *strips* a hypothetical `W1701` block from
`pipeline`'s golden before comparing (defensive scaffolding for when it lands, not a producer).
Two goldens already assert `W1701` output content today and fail against it:
`sovcommon/time/expected/findings.txt` (20 texts, no fr) and `studio/expected/findings.txt` (13
texts, no fr); `resource/{events,farm,heistia,vocab,adventurequest}` and `game/items` also have
`W1701` lines in their (currently unreachable, load-blocked) goldens. `resource/farm/farm.fr.canon`
and `game/items/item.fr.canon` are real translation-file fixtures already checked in.

### `gen/view` and the view model (VIEWMODEL.md, viewmodel.schema.json)

**Entirely unbuilt.** `internal/gen/view/` is `doc.go` only. `api/vm` (package `vm`) is `doc.go`
only — the Go structs `ViewModel.Decode` is supposed to target (API.md §5.4) don't exist.
`ir.TargetView` and `check.TargetView` constants exist and are threaded through `ir`'s stage
machinery (`emits.go:129`, `precompute.go:31`, `constants.go:212,225`) and
`build/place.go:76`, `build/emit.go:161,166` — the *plumbing* recognizes a view target, but
`internal/build/constants.go:80` sets `generators[ir.TargetView] = nil`, so requesting it
crashes or no-ops (unverified which — worth a builder's first five minutes to confirm, not
assumed here). `spec/viewmodel.schema.json` exists (2824 lines) and is unread this pass beyond
its presence.

### C++ `types` mode (CODEGEN.md §5.13)

`internal/ir` validates `ModeTypes` emits (`checkDecoders`, `checkTypesMode` in
`rules_emit.go`) — the IR-side contract exists. `internal/gen/cpp` has 28 non-test `.go` files
covering `baked`/`embedded`/`data` modes (`decode.go`, `decode_wire.go`, `strict.go`,
`stored.go`, etc.) but none read `ir.ModeTypes` except the error-message name table
(`errors.go:59`). Nothing decode-only (no baked default values, per CODEGEN.md §5.13) is
generated. `events` is the target example (acceptance item 6) but is also load-blocked.

### TS data mode (CODEGEN.md, CONFORMANCE.md TS)

`internal/gen/ts/` is `doc.go` only — no TS codegen exists at all (not scoped to M3 by the plan
table (§5.1: TS's M3 row says "TS: data"), but confirmed empty here since M6 also depends on it
starting and the plan explicitly lists it under M3's phase row).

### `api` Check / Value / ViewModel, `api/vm`, `cli explain` (API.md, CLI.md §3.7)

| Symbol | Status | Evidence |
|---|---|---|
| `api.Project.Check` | **Done** (M1/M2 scope) | Used by every passing golden; not re-verified line-by-line this pass. |
| `api.Project.Value` | Stub | `api/value.go:47` `return nil, errUnimplemented()`. Struct shapes (`Value`, `Origin`, `Editability`, `TypeInfo`) are the frozen M0 contract and already carry the fields `explain` needs (`Origin.Kind/Span/Pointer/Layer/Via/Stack`). |
| `api.Project.ViewModel` | Stub | `api/value.go:161`. |
| `api.Project.Refs` | Stub (not in M3's acceptance list, but same package) | `api/value.go:136`, called out for completeness. |
| `api/vm` package | `doc.go` only | — |
| `cli explain` | **MISSING entirely** | `cli.Main --help` lists `build check init new test version`; no `explain` subcommand file under `internal/cli`. |

### `balance.parity` golden

No `expected/MANIFEST`; `sweep_plan.canon` uses only `load.defines` (3 calls) and one
`emit json { out: "@parity/sweep_plan.json", ... }`. Fully blocked by the `load.defines` gap
above; nothing else stands in its way as far as this pass found.

### `make check-real` / real-data job

**Missing.** No `check-real` target, no `testdata-real`-reading rule, in `Makefile`. DECISIONS
29 / CLAUDE.md rule 4 already fix where such data must live (`testdata-real/`, git-ignored,
opt-in target) — this is a QA-owned Makefile + harness addition, not a design question.

## §4 Codes defined but never produced (M3-relevant ranges)

Checked every code in `E16xx`, `E17xx`/`W17xx`, `E19xx`, `E37xx`, `E7xxx`/`W7xxx`, `E9xxx`
against `grep -rl "diag\.<CODE>\b" --include=*.go internal | grep -v internal/diag`.

- **All of `E1601`-`E1634`** (31 codes, views) — zero uses. Whole range MISSING.
- **All of `E1702`-`E1707`** (6 codes, i18n) — zero uses. Whole range MISSING.
- `W1701` (i18n status) — zero *producing* uses (one reference, and it's a test-side stripper).
- `E7106` (`at:` path mismatch, WIRE.md §6.3) — zero uses. MISSING (matches §3's `at:` finding).
- `E7113` (malformed CSV, WIRE.md §6.6) — zero uses. MISSING (matches §3's `load.csv` finding).
- `E1901`-`E1911` (layers/inputs) — **used**, 2-4 sites each. Not a gap.
- `E3701`-`E3704` (assets) — **used**, including in the progen mutation suite. Not a gap.
- `E9001`-on (`ir`) range not individually audited for usage this pass (M2-owned, out of scope
  here); spot check showed `ir` codes generally have call sites (`ir` package is mature per
  `meta/state.md`).

No M3-range code was found registered-but-silently-renamed or duplicated; the registry and the
grep are consistent with the file-level MISSING findings in §3.

## §5 Proposed unit scoping per wave

Cross-package dependency graph for the flags below: **load execution (LOD) blocks almost
everything** — dependent-type/asset decoding through `load.dir`, `balance.parity`, `farm`,
`events`, most `features.*`/`resource.*` checks, and therefore the view/i18n/API units that want
those examples as their test fixtures. The wave order in `meta/plan.md` already reflects this
(W1 loads first); this section only adds file-level detail.

### W1
- **`load` forms** (LOD, sonnet, `internal/load` + `internal/build/hosts.go` wiring, drop
  `build.ErrLoad` from `internal/cli/examples_test.go`'s `loadExamples` entries one form at a
  time): implement `load.defines` (new, no existing parser — biggest sub-unit, needed by
  `balance.parity` and several `features`/`resource` examples), `load.csv`, `load.text`,
  `at:`, `partial:`, and lift `internal/load/supported.go`'s dependent/ref/variant/map gate
  (coordinate with the dependent-types unit below: lifting the gate before dependent decoding
  exists in `load` would just move the failure into `wire`/`verify`). The checker side
  (`internal/check/load.go`) already types all four forms and both options — this unit is
  almost purely `internal/load` + a `internal/wire` decoder for CSV/text/defines, not a checker
  rewrite.
- **Dependent types** (`types`, `check`, opus): complete TYPES.md §11 against the existing
  `dependent.go`/`typefunc.go`/`judge.go` machinery (§3's open item — no rule-by-rule table
  exists yet; this unit should produce one as it goes, since W0 could not verify completeness
  claim by claim in budget). Coordinate with LOD: `load.dir`'s `supported()` gate is the join
  point.
- **Layers + inputs completion, explain provenance** (`eval`, opus): `layers.go`/`inputs.go`
  exist; scope is closing whatever E1901-E1911 gaps remain (not audited rule-by-rule this pass)
  plus building the `Origin`/`Frame` provenance data `api.Value` and `cli explain` need —
  `api/value.go`'s `Origin` struct already has `Stack []Frame`, so the shape is frozen; this
  unit's job is populating it from `eval`, not designing it.

### W2
- **Dependent verification + assets** (`verify`, opus): assets are essentially done (§3); scope
  is dependent-type verification specifically (branch selection at verify time) — read
  `internal/verify/refine.go` fully first, since `E3701`-`E3704` already live there.
- **View and translation checking** (`check`, after W1's `types`/`check` unit lands, opus): this
  is the entire `E16xx`/`E17xx` range from zero — the largest single unit in the milestone (31 +
  6 new codes, 0 existing call sites). `resource/farm/farm.view.canon` (6 blocks) and
  `resource/events/event.canon` (2 blocks) are real fixtures to check against once load-unblocked.
- **Unions + `LoadInputs`** (`gen/go` then `gen/cpp`, sonnet): `LoadInputs` is genuinely new in
  both targets (§3); "unions" here likely means the codegen side of string-literal unions /
  dependent unions for the generated structs, not the checker (which appears to already
  represent them) — confirm scope with TYP before starting, since `internal/types/dependent.go`'s
  `DepUnionType` may already be the shape `gen/go`/`gen/cpp` need to consume.
- **`types` mode in `ir`** (opus): mostly done — `rules_emit.go`'s `checkDecoders`/
  `checkTypesMode` already exist. Scope here is likely closing gaps found once `gen/cpp`'s types
  mode (W3) exercises it, so sequence this unit to land just before or alongside gen/cpp's.

### W3
- **`views`, `i18n`, `gen/view`** (sonnet, schema + goldens): all three packages are `doc.go`
  skeletons. `gen/view` additionally needs `internal/build/constants.go:80`'s
  `generators[ir.TargetView]` wired to a real function (currently `nil`) and `api/vm`'s Go
  structs generated from `spec/viewmodel.schema.json`. `pipeline/expected/potion.view.json` is
  the one pre-drafted target golden to regenerate-and-review (GEN-01 style) rather than write by
  hand; `farm`/`events` need their own goldens from scratch. This unit is downstream of W2's
  view/translation *checking* unit (view resolution needs `check`'s output) — sequence
  accordingly even though the plan lists it as W3.
- **C++ `types` mode and TS data** (`gen/cpp`, `gen/ts`, sonnet): `gen/cpp` needs new files (no
  existing types-mode logic to extend, only the IR-side validation is ready); `gen/ts` is a
  green field (`doc.go` only) — flag to the orchestrator whether the full TS data mode belongs
  in M3 given the plan's own M6 TS scope, since IMPLEMENTATION-PLAN.md's phase table lists "TS:
  data" under M3 but §5.1's TS row says "CODEGEN.md (TS)" generally; recommend confirming this
  isn't scope creep before the unit starts (see §6).
- **`api` Check/Value/ViewModel, `api/vm`, `cli explain`** (opus, frozen contract): `api.Check`
  is done; `Value` and `ViewModel` are stubs whose struct shapes are already frozen and don't
  need renegotiating — this unit fills the stub bodies and adds the `explain` CLI command
  (CLI.md §3.7 format, verbatim golden line `canon explain config.server.port --layer louis`).
  Depends on W1's provenance work and W3's `gen/view`/`api/vm` for `ViewModel`.

### W4
- **progen mutation operators** for every new code (all 31+6+2 = 39 new M3 codes need operators,
  following the pattern already used for assets in
  `internal/testkit/progen/ops_values_test.go`).
- **Real-data run**: build the `check-real`/`testdata-real` Makefile target and harness from
  scratch (§3) — no existing scaffolding to extend.
- **Every example equals its `findings.txt`**: by W4 this should mean deleting
  `internal/cli/examples_test.go`'s `loadExamples` map entirely (it should be empty) and
  possibly retiring `TestExamplesLoadUntilM3` itself, plus wiring the 21 examples that currently
  have `expected/findings.txt` but no `expected/MANIFEST` into `TestExamples`'s golden harness
  properly (or documenting why `findings.txt`-only checking, via
  `internal/cli/examples_test.go`, is the intended final harness for check-only examples — this
  wasn't resolved by the plan documents read this pass and should be confirmed, see §6).

## §6 Spec gaps / contradictions noticed

1. **TS scope inside M3 is stated two different ways.** IMPLEMENTATION-PLAN.md §5.2's phase
   table, M3 row: *"CPP: `types` mode; TS: data"* (implying TS data-mode codegen is M3 work).
   But §5.1's owner table gives TS the companion documents *"CODEGEN.md (TS), CONFORMANCE.md
   (TS)"* with no milestone qualifier, and M6's scope explicitly is *"Legacy C++ and
   TypeScript ... TS goldens pass `tsc --strict` and `node --test`"* (§6 M6), which reads as TS's
   real acceptance gate. `meta/plan.md`'s M3 section doesn't mention TS at all in its bullet
   list, only in the wave breakdown ("C++ `types` mode and TS data (`gen/cpp`, `gen/ts`;
   sonnet)"). Given `internal/gen/ts` is currently empty, building a first TS data-mode codegen
   inside M3 is a substantial, easy-to-underscope addition; flagging for the orchestrator to
   confirm depth (a minimal data-mode emitter vs. a fuller TS package) before W3's TS unit
   starts, rather than resolving it silently here.
2. **No `expected/MANIFEST` convention stated for check-only examples.** 21 of 24 example
   packages have `expected/findings.txt` but no `expected/MANIFEST`, so
   `internal/testkit/golden`'s `TestExamples` (which requires a MANIFEST) never runs them; only
   `internal/cli/examples_test.go`'s hand-written `TestTeamboardFindings`/`TestPipelineFindings`
   pattern checks a findings.txt today, and only for those two packages by name — there is no
   generic "check every example's findings.txt" test even for the 9 examples that already pass.
   IMPLEMENTATION-PLAN.md §6 M3 item 1 ("every example package ... prints exactly its
   `expected/findings.txt`") implies such a generic test should exist, but neither
   IMPLEMENTATION-PLAN.md §7.1/§7.2 nor `meta/plan.md` says whether that's a new generic loop in
   `cli`'s test file, an extension of `golden.TestExamples` to check-only examples (no build), or
   individual named tests per package. This is a real gap in the test-strategy documents, not
   just an implementation gap — the W4 unit that's supposed to make item 1 pass needs this
   decided first.

## What could not be verified this pass

TYPES.md §11 was not read rule-by-rule against the existing dependent-type code (§3 flags this
explicitly); `internal/eval/host.go`'s dependent-type evaluation branch-selection was not
opened; `spec/viewmodel.schema.json`'s content was not read beyond confirming its presence and
line count; whether `generators[ir.TargetView] = nil` panics or silently no-ops was not
exercised (no `emit view` block exists in any currently-buildable example to trigger it);
`internal/check/typeexpr.go`'s string-literal-union handling (TYP-09) was located but not read
in full. `go test ./internal/testkit/golden/...` was not run under the memory-capped runner this
pass (recon relied on the standalone `canon check` binary instead, which is representative for
findings-text comparison but does not exercise the build/MANIFEST path); a later unit's verify
step should still run the full `make check` / `golden` suite itself.
