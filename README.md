# Canon (working name)

One language for every configuration: game data, service settings, taxonomies and the
studio's presentation. It is written once, checked once at build time, and **translated**
into typed code (Go, C++ and TypeScript), JSON data files and the studio's view model.
Runtimes never run Canon. They receive values that are already correct.

**Status:** language v0.1 **being locked**. The v0.1 draft was reviewed for implementability
([AUDIT.md](meta/spec-phase/AUDIT.md), 202 findings) and against a studio mockup ([MOCKUP-GAPS.md](meta/spec-phase/MOCKUP-GAPS.md),
50 view gaps); the answers are settled in [DECISIONS.md](DECISIONS.md) (decisions 17–25) and
[meta/spec-phase/review/ACCEPTED-CHOICES.md](meta/spec-phase/review/ACCEPTED-CHOICES.md), and are written into SPEC.md, CLI.md and
the companion documents under [spec/](spec). Nothing is implemented. The examples are the test
cases for the spec: each one rewrites a real file from this repository.

## Files

| File | What it is |
|---|---|
| [SPEC.md](SPEC.md) | the language: the normative overview, with a short summary of every subject and a pointer to the companion document that owns its details |
| [CLI.md](CLI.md) | the `canon` command: flags, output formats, exit codes, value paths, and what the embedding API does |
| [DECISIONS.md](DECISIONS.md) | why things are the way they are; wins over every other document |
| [review/](meta/spec-phase/review) | the consistency pass that locks v0.1: [ACCEPTED-CHOICES.md](meta/spec-phase/review/ACCEPTED-CHOICES.md), the choices DECISIONS 24 accepts and which answer won where documents disagreed (it wins over every document but DECISIONS), and [CONSISTENCY-TODO.md](meta/spec-phase/review/CONSISTENCY-TODO.md), the work list |
| [AUDIT.md](meta/spec-phase/AUDIT.md) | the pre-implementation review: 202 findings, each with a proposed answer (accepted unless DECISIONS overrides it) |
| [MOCKUP-GAPS.md](meta/spec-phase/MOCKUP-GAPS.md) | the view behaviours found while mocking the studio, and the examples that contradicted the spec |
| `meta/spec-phase/mockups/` | `studio.html`, a clickable mockup of the studio rendering the examples' views (git-ignored, local only) |
| [go.mod](go.mod) | the compiler's Go module, `github.com/fantasim/canonlang` (DECISIONS 23); for now it holds only the API stub [api/canon.go](api/canon.go) |
| [Makefile](Makefile), [tools/audit/](tools/audit) | the gate every change passes, and the code audit it runs (see Development below) |
| [examples/](examples) | the test cases (next section), with their own module [examples/go.mod](examples/go.mod) so that generated goldens never break `go build ./...` at the root |
| [examples/_fixtures/](examples/_fixtures) | trimmed copies of the real files the examples load; tests redirect every root to them or to a temporary directory (`--root`) |
| `examples/*/expected/` | per example, `findings.txt` (what `canon check` reports); for the pipeline, every generated file, a `MANIFEST` and its own `go/go.mod`; for teamboard, the `canon.lock` of its first build |
| [spec/GRAMMAR.md](spec/GRAMMAR.md) | the normative grammar, lexer modes, keyword-as-name rules, annotation catalogue, parse corpus |
| [spec/FORMATTER.md](spec/FORMATTER.md) | the canonical layout (no column alignment) and canonical JSON for sources |
| [spec/TYPES.md](spec/TYPES.md) | typing rules: strict optionals and narrowing, entries and refs, joins, refinements, dependent types |
| [spec/EVALUATION.md](spec/EVALUATION.md) | what is evaluated and when, step costs, poisoning, copy-on-write, provenance, layers |
| [spec/STDLIB.md](spec/STDLIB.md) | every standard function with its signature, costs and errors; the canonical text form |
| [spec/WIRE.md](spec/WIRE.md) | JSON reading and writing, `load.*` formats, the `emit json` file layout, with byte-exact samples |
| [spec/FINGERPRINT.md](spec/FINGERPRINT.md) | the schema fingerprint (`canon-fp v1`) and its test vectors |
| [spec/LOCK.md](spec/LOCK.md) | `canon.lock`: format, what is locked, retire/rename/reuse, merges |
| [spec/CODEGEN.md](spec/CODEGEN.md) | generated Go, C++17 and TypeScript: names, files, APIs, runtime helpers, legacy C++ structs |
| [spec/CONFORMANCE.md](spec/CONFORMANCE.md) | conformance tests of translated functions: vectors, checked arithmetic, error signalling |
| [spec/VIEWMODEL.md](spec/VIEWMODEL.md), [spec/viewmodel.schema.json](spec/viewmodel.schema.json) | the view model (`canon-vm/1`) and the studio behaviours it drives |
| [spec/I18N.md](spec/I18N.md) | translation keys, templates in translations, reporting of missing keys |
| [spec/API.md](spec/API.md) | the Go embedding API used by the studio: types, paths, edit operations, minimal writes |
| [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) | module map, frozen interfaces, milestones, fixtures, performance targets |
| [spec/ERRORS.md](spec/ERRORS.md) | the single source of every diagnostic (DECISIONS 27): severity, owning package and document, meaning, message templates with typed arguments; `internal/diag/codes.go` is generated from it |

## Where the data lives

A `.canon` file holds types, rules, and **optionally the data itself**. The data can live in
one of two places, chosen per file:

| Data written in | Examples | Canon's job | What is generated |
|---|---|---|---|
| **`.canon`** | teamboard taxonomy, roles, service config, event types | Source of truth | Go/C++/TS code, and JSON data files |
| **An existing JSON file**, read with `load` | farm, events, heistia, adventure quests | Validates the file, gives it types | Types plus a decoder (`mode: types`); the runtime keeps reading its JSON |

Computed data is a third case: [sweep_plan.canon](examples/balance/parity/sweep_plan.canon) reads
Resource files and **writes** the bot's plan, replacing the Python generator.

Every source moves to `.canon` (DECISIONS 10 and 11). Until its domain is converted, legacy JSON
is read with `load`.
The server never reads sources, only what `canon build` emits, so converting changes nothing
for it.

For the full path from source files to the running server, with every generated file, see
[examples/pipeline/](examples/pipeline). Its `expected/` files are illustrative until the v0
compiler regenerates them.

## Reading order

1. [SPEC.md](SPEC.md) §1: the three laws (pure, build-time only, always finishes). The full
   language is in SPEC.md and the companion documents it points to, the `canon` command in
   [CLI.md](CLI.md), the studio's Go API in [spec/API.md](spec/API.md).
2. [examples/teamboard/taxonomy.canon](examples/teamboard/taxonomy.canon): records, stable ids, references, checks.
3. [examples/resource/events/event.canon](examples/resource/events/event.canon) and [sovcommon/time](examples/sovcommon/time/time.canon): functions, a variant, a view, tests.
4. [examples/resource/farm/](examples/resource/farm): the studio question, with types and a view kept separate.
5. The rest of the spec, then [DECISIONS.md](DECISIONS.md) for the reasons.

## What each example tests

| Example | Tests | Replaces today |
|---|---|---|
| [pipeline/](examples/pipeline) | the whole pipeline on a tiny object: per-item JSON in, data file + C++/Go loaders + view model out (`expected/`, illustrative) | the propItem merge, done by Canon |
| [teamboard/taxonomy.canon](examples/teamboard/taxonomy.canon) | tables, `stable` ids, `ref`, derived values, package checks | taxonomy.json (85 lines) + its loading and validation Go (~650 lines) + the TS generator |
| [resource/events/event.canon](examples/resource/events/event.canon) + [sovcommon/time](examples/sovcommon/time/time.canon) | functions, loops, a variant, `load` of legacy JSON, view, tests, a shared package | eventConfig.schema.json (215) + event.schedule-overlap.lua (96) + overlay event.json (279) |
| [resource/vocab/vocab.canon](examples/resource/vocab/vocab.canon) | event types with the parameter kind they take, item pickers that search by name | Vocab/*.json + generated define headers + EventTypeDisplay.h's param column |
| [resource/heistia/heistia.canon](examples/resource/heistia/heistia.canon) | a field whose type depends on another field | heistia_config.schema.json's nine if/then branches |
| [resource/adventurequest/adventurequest.canon](examples/resource/adventurequest/adventurequest.canon) | records with a value parameter, maps whose value type depends on the key | adventureQuestConfig.schema.json + overlay |
| [resource/rules/rules.canon](examples/resource/rules/rules.canon) | graph walk, group-by, set parity, range join | the other four Lua rules (230) |
| [resource/farm/](examples/resource/farm) | the studio: types, a separate view, a French translation file | farm_config.schema.json (207) + overlay farm.json (348) |
| [balance/parity/](examples/balance/parity) | data computed from data, layers as CLI flags | gen_sweep_plan.py (683) |
| [service/resourcestudio/](examples/service/resourcestudio) | an ordinary service config, per-machine layer, secret input | config.example.toml + its loader and precedence rules |
| [sovcommon/](examples/sovcommon) | shared enums, written once | roles.go's ladder and the tone/icon lists checked by scraping TS |
| [studio/studio.canon](examples/studio/studio.canon) | the studio's vocabulary: menus, icons, tones, units, widgets, and the default editor of `TimeOfDay` | nothing (it is new) |
| [game/items/](examples/game/items) | a domain after migration and `canon convert`: one `entry` file per item, nested kinds, `@json(pairs:)`, `@json(bits)`, a view and a translation | propItem.json's 6,944 rows and their loader, schema and overlay |
| [features/](examples/features) | one small example per feature no other example covers: `@codes` on the wire (`codes`), `load.csv` (`csv`), `embedded` mode (`embedded`), legacy C++ structs (`legacycpp`), finite-input `export fn` (`lookup`), value `match` (`match`), `retired` ids (`retired`), `load.text` (`text`), `expect … warns` (`warns`) | |

Line counts are indicative. The bigger gain is that each subject is written in **one** place:
today the event rules are spread across a JSON Schema, `x-` keywords, a Lua file, a C++ loader and
a studio overlay.

## Main decisions

- **Checks replace every validation layer.** JSON Schema, the `x-invariant` / `x-format`
  vocabularies, the Lua rules (since ADR-0009 of 2026-09-19, the only implementation of the named
  rules) and the Python enforcers all become `check`s written next to the data. There is no
  keyword list to extend.
- **Types remove the guard code.** A check only runs on well-typed data, so the
  `type(x) == "table" and x ~= host.null` half of every Lua rule disappears. Unknown refs, enum
  members and ids are type errors, with no code to write. Optionals are strict: code proves
  presence with a `!= none` test, `??` or `!` (DECISIONS 17).
- **Generated code is read-only and contains no hand-written logic.** Types, values, getters and
  a schema fingerprint check on load; `export fn` methods are precomputed, or translated with a
  generated conformance test. The evaluator exists once, in the compiler.
- **Views are separate from types, but in the same language.** The studio becomes a generic
  renderer. Types already give it most of the editor, and views only add French labels, groups,
  units, columns and custom widgets. A view that names a missing field is a compile error.
- **`load` is the migration bridge.** Existing JSON, CSV and `#define` headers are read and
  type-checked where they are. Files can move into Canon one at a time.
- **One canonical layout, no alignment.** `canon fmt` and the studio's edits print the same
  layout, and a one-value edit is a one-line diff (DECISIONS 18).

## Development

- The compiler is one Go module, `github.com/fantasim/canonlang` (DECISIONS 23), written for Go
  1.25. The embedding API is the package `canon` in `api/`; the examples and `tools/audit` are
  separate modules.
- `make check` is the gate every change passes (DECISIONS 25): `gofmt -l` empty, `go vet`,
  `go test`, the generated goldens diff-clean, and the code audit `go run ./tools/audit check`
  (the Makefile runs it from `tools/audit/`, which is its own module). The audit enforces the code
  doctrine of [tools/audit/DOCTRINE-code.md](tools/audit/DOCTRINE-code.md): short functions and
  files, no magic values, sentinel errors, a `doc.go` and an example test per package, and a
  ratchet baseline in `.sovaudit/`.
- The build order, packages and milestones are in
  [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md).

## Next steps

1. Finish locking v0.1: the consistency pass of [review/](meta/spec-phase/review) aligns SPEC.md, CLI.md, the
   companion documents and the examples with DECISIONS 17–25; then answer the remaining open
   questions (SPEC §23).
2. Keep the examples complete: fixtures for every loaded path, an expected findings file per
   example, and goldens regenerated by the v0 compiler (see spec/IMPLEMENTATION-PLAN.md).
3. Compiler v0 in Go, following spec/IMPLEMENTATION-PLAN.md: parser, name resolution and type
   checker, evaluator, checks, `json` and `go` emitters. Done when `taxonomy.canon` builds and its
   generated Go replaces `sovcommon/teamboard`'s loader with the service tests still green.
4. `load` + `cpp` (`data` and `types` modes) + `view`: `event.canon` and `farm.canon` validate the
   real Resource files, and the studio renders the farm from the view model.
5. `canon fmt` and the edit API, then `canon lsp` (diagnostics first).
