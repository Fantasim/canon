# Consistency pass: work list

Everything the eight documentation agents reported for files they did not own, plus the
decisions of meta/spec-phase/review/ACCEPTED-CHOICES.md and DECISIONS 21–24. Three agents apply it: **K1**
(SPEC.md, CLI.md, README.md), **K2** (spec/*.md, spec/viewmodel.schema.json, api/canon.go,
go.mod) and **K3** (examples/).

**Rule for everyone:** DECISIONS.md wins, then meta/spec-phase/review/ACCEPTED-CHOICES.md, then the companion
document that owns the topic. SPEC keeps short normative summaries and points to the companion
docs for detail; it must never contradict them. Don't edit DECISIONS.md, AUDIT.md,
MOCKUP-GAPS.md or this file.

## K1: SPEC.md, CLI.md, README.md

### SPEC.md
- **§2 (lexical)**
  - §2.3: newlines follow GRAMMAR §3 (the innermost bracket decides; `@` continuation lines).
  - §2.4: naming conventions are `W1003`, reported by `canon check`. Widgets are lower_snake.
    Name positions: pointer to GRAMMAR §4.3.
  - §2.5/2.6: pointers to GRAMMAR §2 (durations, `r"""`, format specs up to 20, `!`, `_`) and
    STDLIB §9 (canonical text).
- **§3.1**: pointer to GRAMMAR §7 for the project schema. It includes `go_module`, keyed by root
  name.
- **§4.2 and §6.1**: the sample `{ open { … }  taken { … } }` must parse. Use one entry per line,
  or commas.
- **§5 (types)**
  - §5.6, §7.1, §7.5: must match TYPES.md exactly:
    - `?.` short-circuits the rest of the chain;
    - narrowing is per TYPES.md;
    - `.` or `[ ]` on a `T?` is a static `E3402`.
  - §5.7: keyed lists and tables index by key; add `.at(i)`. `m.k` on a map is an error.
    `entry` works for keyed lists.
  - §5.9: `Range` is Int only.
  - §5.11: the branch enum is `<Alias>Branch`.
  - §5.12: pointer to the GRAMMAR §8 annotation catalogue. The table must list `@json(pairs:)`
    (DECISIONS 21), `@json(int)`, `@json(bits)`, `@json(codes)`, `@menu`, `@cpp(value:)` and
    `@cpp(unit:)`.
- **§11 (evaluation)**: §11.2 and §11.6 are replaced by pointers to EVALUATION.md §1 (stages)
  and §12 (step cost).
- **§12 (lock)**
  - The sample header is exactly `# canon.lock v1`.
  - The field fact is `field  resource.vocab.eventTypes.code  0  COMBAT_KILL_MONSTER`.
  - Retirement is recorded in the lock; un-retiring is `E6002`.
- **§14 (emit)**
  - §14.4: name rule for anonymous root types (`<package>.<value>`).
- **§15 (generated code)**
  - Generated headers per CODEGEN.
  - Go: `Load<V>` and `Get<V>`.
  - C++: `<Enum>FromWire` and `ToName`.
  - TS: `CanonTable`.
  - Reload: per-package `<Package>Snapshot` and store.
  - Conformance file names per CODEGEN.
  - Go errors are `*rt.EvalError`.
  - C++ default constructors are public, members private.
  - `canon_runtime_json.h`.
  - `--adopt`.
- **§16 (views)**
  - Fold in VIEWMODEL.md: thresholds, new view items (`plural`, `filters { x multi }`,
    `show [id]`, `field` escape, `view Variant.case`, `check … at`), `$schema` = `canon-vm/1`,
    and `widget … default` (DECISIONS 22).
  - Remove the `row` item if present.
  - §16.11 must copy examples/studio/studio.canon exactly, after K3's change. It includes
    `widget time_of_day(value: TimeOfDay) default` and `weight_share(value: Int, siblings: [Int])`.
- **§17 (translations)**: key table per I18N.md, including `Type.plural`, `Type.show.<id>`
  (unnamed: `_0`…), `units.<unit>.suffix` and the kind-word escapes.
- **§18 (tests)**: the `fails E####` form.
- **§19.2 (inputs)**: no default, no `where`, an empty variable counts as unset, and the allowed
  types per EVALUATION.md.
- **§20 (stdlib)**: `at`, keyed `get`/`find`, `keys() -> [ref T]`, `Range.len()` and
  `contains()`. List `min`/`max` only on numbers and durations. Float −0.0 ordering.
- **§21**
  - §21.1: finding fields per API.md (`pointer`, `package`, paths starting at the value name).
  - §21.2: the ranges table includes E10xx, E11xx, E16xx/W16xx, E17xx/W17xx, E19xx, W3xxx, W6xxx,
    E81xx, E82xx, E83xx and E9xxx. Note `E8150`–`E8153` and `E8102` (WIRE) and `E8301`–`E8303`
    (runtime-signalled).
- **§23**: Q "bare JSON" is closed (no `bare`). Keep only what is still open.
- **Appendix A/B**: pointers only. Contextual words include `plural multi at default siblings
  field`.

### CLI.md
- **§2 (common behaviour)**
  - §2.3: `--root name=path`. An unknown `--layer` is `E1901`, exit 2.
  - §2.5: exit code 130 on interrupt.
- **§3 (commands)**
  - §3.3: `check` also computes export fn results.
  - §3.4:
    - view outputs are written even when the build has errors;
    - `--adopt`;
    - the generated-file marker per format.
  - §3.5: `--run` is an RE2 search; `fails` can take a code.
  - §3.6: layout is per spec/FORMATTER.md (not SPEC §2.1).
  - §3.9 (`infer`): `--by a,b`, `--schema`.
  - §3.10 (`convert`) and §6.4: convert requires the value to be written by `emit json` first,
    with `--adopt` to keep the old path.
  - §3.11: pointer to I18N.md §9–§10. `status` has `--lang`, `--list`, `--format`.
- **§5 (Go API)**: aligned with API.md §16, items 1–8:
  - typed op values;
  - `(*CheckResult, error)`;
  - new ops;
  - `Evaluate`;
  - `Watch` signature;
  - finding JSON;
  - `[#n]`.
  Module path `github.com/fantasim/canonlang`.

### README.md
- Files table:
  - examples: `game/items`, `features/*`, `_fixtures`, `balance/parity`,
    `service/resourcestudio`;
  - `review/`;
  - `go.mod`.
- Remove the stale "named Go rules" wording. Since ADR-0009 (2026-09-19), Lua is the only
  implementation of those rules.
- Module path `github.com/fantasim/canonlang`.

## K2: companion documents (spec/*), api/canon.go, go.mod

### GRAMMAR.md
- Carry the view grammar from VIEWMODEL §3.1:
  - the `field` escape;
  - `show [id]` inside groups;
  - `plural`, `filters { x multi }`;
  - `widget … default`;
  - `siblings` parameters;
  - the `step:` prop.
- Remove view `row` (DECISIONS 21).
- Annotations: add `@menu`, `@cpp(value:)`, `@cpp(unit:)`. Keep `@json(pairs:)`.
- Translation key segments may be `_0`-style ids.
- Amend paths accept `[#n]`.
- `@files` template variables: align with API.md N2 (nested `{f.g}` and a variant field's case
  wire name are allowed).
- Format spec `.N` ≤ 20 is enforced here (`E1101`). STDLIB must drop E4503 for it, or keep it
  only as unreachable.

### TYPES.md
- `entry` for keyed lists (`E3103` must allow them).
- `E3316` also covers annotations that don't apply to the field's type.
- Duration limit ±9,223,372,036,854 ms.

### EVALUATION.md
- Stage names are the reference. IMPLEMENTATION-PLAN's "phase 6" must become the stage name.

### STDLIB.md
- Float −0.0 ordering for `min`, `max`, `clamp`.
- The format spec limit is owned by GRAMMAR.

### WIRE.md, FINGERPRINT.md, LOCK.md
- WIRE: define `@json(pairs:)` precisely:
  - the `{i}` template;
  - index range (from the list's upper bound);
  - how absent or zero pairs are written and read;
  - fingerprint contribution.
- FORMATTER: its JSON-source layout reuses WIRE §7.2 and §7.3.
- LOCK: removing `retired` from an entry is `E6002`.

### API.md and api/canon.go
- Module path `github.com/fantasim/canonlang` (go.mod at the configlang root). The api package's
  import path follows.
- `Unretire` returns `ErrStableKey`. Drop the claim that un-retiring is not a reuse (API.md:24,
  :756).
- `Evaluate` must satisfy VIEWMODEL §13 Q1–Q5:
  - `ShowLine.Key` = `Type.show.<id>` (unnamed `_<n>`), or `Type.<method>` for methods;
  - `Heading.Cells map[string]Text`;
  - `Evaluate` on a collection path returns headings for its elements;
  - `Finding.Reads []string`;
  - disambiguated `Heading.Title`.
- Equality lives in TYPES.md §7.5 (API.md L22, L694).
- Go errors: `*rt.EvalError` wording.
- The stub must still compile: `go vet ./api/...`.

### IMPLEMENTATION-PLAN.md
- Module path.
- The IR is defined in CODEGEN §4.5.
- Dependent branches are `Branch`.
- M6 must not expect TS goldens for pipeline, which has no TS emit. Add a TS emit to a small
  example, or point M6 to features/.
- "phase 6" becomes the EVALUATION stage names.
- Equality points to TYPES.md.

### IMPLEMENTATION-PLAN.md: code doctrine (DECISIONS 25)
- Add a "Code doctrine" section. The compiler's Go code follows
  `/home/louis/Desktop/Sovereign/services/sovcommon/tools/sovaudit/fleet/DOCTRINE-code.md`; read
  it and `rules.md` next to it.
- Define `make check`: gofmt, vet, test, golden diff, `sovaudit check`.
- The auditor is the project's own copy, `tools/audit` (agent K4 copies and customises it). Cite
  its customised doctrine, `tools/audit/DOCTRINE-code.md`. Do not specify a separate
  golangci-lint equivalent.
- Ratchet baseline in `.sovaudit/`.
- `doc.go` and an example test per package.
- Constants and errors files per package. For example, diagnostic codes live in
  `internal/diag/codes.go`, so no bare numbers appear elsewhere.
- Check that the module layout respects the size limits (e.g. parser split by construct).
- Say which of DOCTRINE-code's rules apply to the canonlang repo's own agent docs (CLAUDE.md
  ≤ 80 lines).

### CODEGEN.md and CONFORMANCE.md
- CODEGEN:
  - the domain order of lookup tables (EVALUATION stage E relies on it);
  - inputs per EVALUATION §11.3;
  - `types`-mode decoder behaviour on invalid input;
  - where `embedded` data goes;
  - the name of the retired getter;
  - `@json(pairs:)` codegen: a list of records.
- CONFORMANCE: its own step cap per vector.

### VIEWMODEL.md, viewmodel.schema.json, I18N.md
- Replace view `row` with `@json(pairs:)` (DECISIONS 21): the list of records is shown with the
  normal list rules.
- `widget … default` (DECISIONS 22) is already chosen; make sure it matches GRAMMAR.
- `editable` = `canon|json|none` + `reason`.
- Layered values are read-only unless `EditLayer`.
- I18N: `Type.plural`, `Type.show.<id>`; `W1701` only for packages that emit a view.
- Re-validate potion.view.json against the schema after any change.

## K3: examples/

- **Formatter fixed points (FORMATTER §15)**: reformat every `.canon` file:
  - one item per line in broken brace lists (no several fields per line);
  - sorted imports;
  - `60s` → `1m` (canonical durations);
  - lines ≤ 100 columns;
  - trailing commas in broken `( )`/`[ ]` lists.
- **Fingerprint**: replace `pipeline.Potion@00000000` with `pipeline.Potion@f750790e` in
  `pipeline/expected/potions.json`, `pipeline.gen.h` and `go/potions.gen.go` (and anywhere else in
  expected/). potions.json must equal WIRE.md §8.3 byte for byte. Re-run the golden builds
  (g++ -std=c++17 with `-I/home/louis/Desktop/Sovereign/Source/External`; go vet/test in a scratch
  module) and report.
- **examples/project.canon**
  - Roots per CODEGEN: `services` plus the GEN-04 roots, and `pipeline_go: "pipeline/out/go"` if
    CODEGEN requires it.
  - `go_module { services: "github.com/sovereign/services", pipeline_go: "example.com/potions" }`,
    or whatever CODEGEN specifies.
- **Module setup**: add `examples/go.mod` (`module github.com/fantasim/canonlang/examples`) so
  `go build ./...` at the configlang root passes.
- **sovcommon/time/time.canon**: `emit cpp { mode: types }` (E8004/E8018), if not present.
- **teamboard**
  - Add an `emit json`.
  - Add a golden `teamboard/expected/canon.lock` with the exact bytes of LOCK.md §9.1.
- **pipeline/expected/**
  - Add `findings.txt`: only `W1701` if pipeline has no fr file; check that it matches
    potion.view.json's finding.
  - Add `MANIFEST` per IMPLEMENTATION-PLAN.
- **studio/studio.canon**
  - `widget time_of_day(value: TimeOfDay) default` (DECISIONS 22).
  - Remove explicit `widget: time_of_day` uses in views, which are now redundant.
- **game/items**: keep `@json(pairs:)` (DECISIONS 21) and make sure it matches WIRE/GRAMMAR.
- **Optional view additions**
  - heistia `rewardPools { step: "Winner {index}" }`.
  - farm `check unreachable_levels: … at maxLevel else …`.
- **expected/findings.txt**: `W1701` only for packages that emit a view. Update every
  findings.txt.
