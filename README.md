# Canon

One configuration language, checked once at build time and translated into typed Go, C++17 and
TypeScript, JSON data files and a studio view model. Runtimes never run Canon: they receive
values that are already correct.

`canon` is the compiler: one self-contained Go binary, no runtime dependencies, no network.

## A taste

`board/board.canon`: a record with a rule, a table whose rows reference each other, and a
package-level check.

```canon
/// Statuses of a ticket board.
package board

/// Colour of a badge.
enum Tone { neutral, info, success, warning }

/// One point of a ticket's lifecycle.
record Status {
  /// Name shown on the badge.
  label: String(1..)
  /// Colour of the badge.
  tone: Tone = neutral
  /// A terminal status closes the ticket.
  terminal: Bool = false
  /// The statuses a ticket may move to from here.
  next: [ref Status]

  check not next.contains(self) else "a status cannot move to itself"
}

/// Ids are stable: never renamed, never reused, retired instead of deleted.
let statuses: stable table Status = {
  open { label: "Open", tone: warning, next: [taken] }
  taken { label: "Taken", tone: info, next: [open, done] }
  done { label: "Done", tone: success, terminal: true, next: [open] }
}

check statuses.active().count(.terminal) >= 1 else "at least one status closes a ticket"

emit json { out: "out/" }
emit cpp { out: "out/", namespace: "board", mode: data }
```

Mistakes are build errors with a location, not runtime surprises:

```
$ canon check
error[E2102]  board/board.canon:23:54          # next: [taken, missing]
  unknown name missing

error[E5001]  board/board.canon:25:3           # done { ..., next: [done] }
  statuses.done: a status cannot move to itself
  expected by board/board.canon:18 (check)
```

When everything holds, `canon build` writes the outputs and records the stable ids in
`canon.lock`:

```
$ canon build
0 errors, 0 warnings in 1 package (4 ms)
cpp:
  board/out/board.gen.cpp
  board/out/board.gen.h
  board/out/canon_runtime.h
  board/out/canon_runtime_json.h
json:
  board/out/statuses.json
lock:
  board/canon.lock
```

```json
{
  "$schema": "board.Status@f98f84e0",
  "rows": [
    {"$id": "open", "label": "Open", "tone": "warning", "terminal": false, "next": ["taken"]},
    {"$id": "taken", "label": "Taken", "tone": "info", "terminal": false, "next": ["open", "done"]},
    {"$id": "done", "label": "Done", "tone": "success", "terminal": true, "next": ["open"]}
  ]
}
```

The generated C++ gives each row typed getters (`GetLabel()`, `GetNext()` returning resolved
`const Status*`), a `Statuses::Load(path, error)` that checks the schema fingerprint, and a
`Find(id)` by binary search. A larger real example is
[examples/teamboard/taxonomy.canon](examples/teamboard/taxonomy.canon).

## Why

Canon replaces the layers a configuration usually grows: a JSON Schema, a vocabulary of `x-`
keywords, rule scripts (Lua, Python), a hand-written loader per language, and editor overlays.

- **Checks sit next to the data.** A `check` is ordinary Canon code; there is no keyword list to
  extend and no separate rules engine.
- **Strict types remove guard code.** Unknown refs, enum members and ids are type errors.
  Optionals are strict: presence is proved with `!= none`, `??` or `!`.
- **Pure, build-time only, always finishes** ([SPEC.md](SPEC.md) §1). Evaluation has a step
  budget; the same sources always give the same outputs.
- **Generated code is read-only.** Types, values, getters and a fingerprint check on load; an
  `export fn` is precomputed or translated with a generated conformance test. The evaluator
  exists once, in the compiler.
- **One canonical layout, no alignment.** `canon fmt`, the studio and agents print the same
  layout, so a one-value edit is a one-line diff.
- **`load` is the migration bridge.** Existing JSON, CSV and `#define` headers are read and
  type-checked in place, so files move into Canon one at a time.

## Install

Linux amd64 and arm64, from the GitHub releases ([CLI.md](CLI.md) §7, DECISIONS 276):

```sh
# latest release
curl -fsSL https://raw.githubusercontent.com/Fantasim/canon/main/tools/install.sh | sh

# a pinned version
curl -fsSL https://raw.githubusercontent.com/Fantasim/canon/main/tools/install.sh | sh -s -- v0.2.0
```

The script downloads `canon_<version>_linux_<arch>.tar.gz`, verifies it against
`checksums.txt` and installs `canon` into `$CANON_INSTALL_DIR` (default `~/.local/bin`).
Running it again updates; the compiler itself never touches the network, so there is no
self-update command. Releases are tagged `v<semver>`.

```
$ canon version
canon 0.1.0 (<commit>)
language 0.1
formats canon-fp v1, canon-vm/1, canon.lock v1
```

From source (Go 1.25):

```sh
git clone https://github.com/Fantasim/canon && cd canon
go build ./cmd/canon
```

## Quick start

```sh
canon init --name demo     # project.canon, plus .canon/ in .gitignore
canon new board            # board/board.canon with its package line
$EDITOR board/board.canon  # write types, data, checks and emits
canon check                # parse, type-check, evaluate, run every check
canon build                # check, then write every emit output and canon.lock
canon fmt                  # rewrite sources in the canonical layout
```

`canon test` runs the `test` blocks; `canon explain <path>` shows a value, its type and where
each part was set. The global flag `--format json` gives machine-readable output.

## Commands at a glance

From [CLI.md](CLI.md) §1; each command has its own section in CLI.md §3.

| Command | What it does |
|---|---|
| `canon init` | create `project.canon` in the current directory |
| `canon new <package>` | create a package directory with a first file |
| `canon check` | parse, type-check, evaluate and run every check; print findings |
| `canon build` | `check`, then write every `emit` output and update `canon.lock` |
| `canon test` | run `test` blocks |
| `canon fmt` | rewrite sources in the canonical layout |
| `canon explain <path>` | show a value, its type, and where each part of it comes from |
| `canon refs <path>` | list everything that references an entry |
| `canon edit` | apply one edit request (JSON) through the edit API |
| `canon rename <name> <new>` | rename a Canon name everywhere it is used |
| `canon lsp` | language server on stdio |
| `canon version` | compiler and language versions |
| `canon guide [topic]` | the agent guide, embedded in the binary |
| `canon convert <value>` | turn a `load`ed JSON source into `.canon`, proven lossless (M7) |
| `canon i18n stub \| status` | manage translation files (M7) |
| `canon lock check` | verify `canon.lock` against the sources without building |

`canon` with no argument lists what the installed binary implements; `guide`, `convert`, `i18n` and
`lock check` are specified but not built yet.

## Editor

[editors/vscode](editors/vscode) is a VS Code extension: a TextMate grammar for highlighting,
and a client that starts `canon lsp` over stdio (`canon` on the `PATH`, or set
`canon.server.path`). The server is for reading code:

| Feature | Behaviour |
|---|---|
| diagnostics | every finding as you type, including findings inside `load`ed JSON files, for files not open too |
| hover | type, doc comment, default, and for values the computed value |
| go to definition | from a use to its declaration; from a `ref` value to its entry, into JSON too |
| find references | the same as `canon refs` |
| formatting | `canon fmt` |

Completion, code actions and rename are intentionally absent: values are edited in the studio,
and by agents through `canon edit` and `canon rename` (DECISIONS 274).

Package the extension:

```sh
cd editors/vscode
npm install && npx vsce package   # produces canon-0.1.0.vsix
```

## For AI agents

Run `canon guide` first, once it ships ([CLI.md](CLI.md) §3.17, DECISIONS 276): it prints an index of topics,
and `canon guide <topic>` prints one, matched to the binary's version. Agents read with
`canon check --format json`, `canon explain` and `canon refs`, and write through `canon edit`
(value changes) and `canon rename` (names). Both take and print JSON, check the result before
writing, write atomically and minimally, and print an `undo` request that reverts the change
([CLI.md](CLI.md) §6.5).

## Status

Language v0.1. Milestones M0–M5 are built: parser, type checker, evaluator, build and emit (Go,
C++17 data and types modes, JSON, view model), formatter, edit API, language server, `canon edit`
and `canon rename`. Next: M6 (legacy C++ struct modes and the TypeScript target) and M7
(migration: `canon convert`, `canon i18n`). Details in [meta/state.md](meta/state.md).

## Documentation

Precedence when documents disagree: DECISIONS.md, then
[ACCEPTED-CHOICES.md](meta/spec-phase/review/ACCEPTED-CHOICES.md), then the owning companion
document in `spec/`, then the summaries in SPEC.md and CLI.md.

| Document | What it holds |
|---|---|
| [SPEC.md](SPEC.md) | the language: normative overview, pointing to the companion document that owns each subject |
| [CLI.md](CLI.md) | the `canon` command: flags, output formats, exit codes, value paths, install, the embedding API |
| [DECISIONS.md](DECISIONS.md) | every decision and its reason; wins over every other document |
| [spec/GRAMMAR.md](spec/GRAMMAR.md) | grammar, lexer modes, keyword-as-name rules, annotation catalogue |
| [spec/TYPES.md](spec/TYPES.md) | typing: strict optionals, entries and refs, joins, refinements, dependent types |
| [spec/EVALUATION.md](spec/EVALUATION.md) | what is evaluated and when, step costs, poisoning, provenance, layers |
| [spec/STDLIB.md](spec/STDLIB.md) | every standard function: signature, costs, errors |
| [spec/FORMATTER.md](spec/FORMATTER.md) | the canonical layout, and canonical JSON for sources |
| [spec/WIRE.md](spec/WIRE.md) | JSON reading and writing, `load.*` formats, `emit json` layout |
| [spec/FINGERPRINT.md](spec/FINGERPRINT.md) | the schema fingerprint `canon-fp v1` and its test vectors |
| [spec/LOCK.md](spec/LOCK.md) | `canon.lock`: format, retire/rename/reuse, merges |
| [spec/CODEGEN.md](spec/CODEGEN.md) | generated Go, C++17 and TypeScript: names, files, APIs, legacy C++ structs |
| [spec/CONFORMANCE.md](spec/CONFORMANCE.md) | conformance tests of translated functions |
| [spec/VIEWMODEL.md](spec/VIEWMODEL.md), [viewmodel.schema.json](spec/viewmodel.schema.json) | the view model `canon-vm/1` and the studio behaviours it drives |
| [spec/I18N.md](spec/I18N.md) | translation keys, templates, missing-key reports |
| [spec/API.md](spec/API.md) | the Go embedding API: types, paths, edit operations, minimal writes |
| [spec/ERRORS.md](spec/ERRORS.md) | every diagnostic code; `internal/diag` is generated from it |
| [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) | module map, frozen interfaces, milestones, performance targets |
| [DOCTRINE.md](DOCTRINE.md) | project law for the code and for contributors |
| [tools/audit/DOCTRINE-code.md](tools/audit/DOCTRINE-code.md) | the code doctrine the audit enforces |
| [meta/](meta/README.md) | state, plan, implementation ADRs and handoffs |
| [meta/spec-phase/](meta/spec-phase) | the v0.1 spec reviews: [AUDIT.md](meta/spec-phase/AUDIT.md), [AUDIT-2.md](meta/spec-phase/AUDIT-2.md), [MOCKUP-GAPS.md](meta/spec-phase/MOCKUP-GAPS.md), [review/](meta/spec-phase/review) |
| [editors/vscode/README.md](editors/vscode/README.md) | the VS Code extension: grammar subset, dependencies, tests |
| [CLAUDE.md](CLAUDE.md) | entry point for coding agents working on the compiler |

## Examples

[examples/](examples) is one project (its [project.canon](examples/project.canon)) and the
compiler's test suite: each directory's `expected/` holds what `canon check` and `canon build`
produce, written by the compiler and diffed by `make check`. Most rewrite a real configuration
file of Sovereign, the project Canon was first built for; loaded files are trimmed copies in
[examples/_fixtures/](examples/_fixtures).

| Example | Shows | Replaces |
|---|---|---|
| [pipeline/](examples/pipeline) | the whole path on a tiny object: per-item JSON in; data file, C++ and Go loaders, view model out | a hand-merged item table |
| [teamboard/](examples/teamboard/taxonomy.canon) | tables, `stable` ids, `ref`, derived values, package checks | a JSON taxonomy plus ~650 lines of loading and validation Go |
| [resource/events/](examples/resource/events) + [sovcommon/time](examples/sovcommon/time) | functions, loops, a variant, `load` of legacy JSON, a view, tests | a JSON Schema, a Lua rule and an editor overlay |
| [resource/vocab/](examples/resource/vocab) | event types with the parameter kind they take, item pickers | vocabulary JSON and generated define headers |
| [resource/heistia/](examples/resource/heistia) | a field whose type depends on another field | nine `if`/`then` schema branches |
| [resource/adventurequest/](examples/resource/adventurequest) | records with a value parameter, maps whose value type depends on the key | a schema and an overlay |
| [resource/rules/](examples/resource/rules) | graph walk, group-by, set parity, range join | four Lua rules |
| [resource/farm/](examples/resource/farm) | the studio: types, a separate view, a French translation | a schema and an overlay |
| [balance/parity/](examples/balance/parity) | data computed from data, layers as CLI flags | a Python generator |
| [service/resourcestudio/](examples/service/resourcestudio) | a service config, a per-machine layer, a secret input | a TOML file and its loader |
| [sovcommon/](examples/sovcommon) | shared enums, written once | hand-kept lists in Go and TS |
| [studio/](examples/studio/studio.canon) | the studio's vocabulary: menus, icons, tones, units, widgets | (new) |
| [game/items/](examples/game/items) | a migrated domain: one `entry` file per item, `@json(pairs:)`, `@json(bits)`, a view | a 6,944-row item table with its loader and schema |
| [features/](examples/features) | one small project per feature: `codes`, `copies`, `csv`, `dependent`, `edits`, `embedded`, `entries`, `legacycpp`, `lookup`, `matching`, `renames`, `retirement`, `text`, `warns` | |

## Development

- One Go module, `github.com/fantasim/canonlang`, Go 1.25. The embedding API is package `canon`
  in [api/](api); the binary is [cmd/canon](cmd/canon); everything else is under `internal/`.
  [examples/](examples) and [tools/audit](tools/audit) are separate modules.
- `GOTOOLCHAIN=local make check` is the gate for every change: `gofmt`, `go vet`, tests, a
  short race stress test, the generated goldens (vetted, tested and diff-clean), the diagnostics
  registry, and the code audit on itself and on the repository. It needs a C++ compiler and
  the nlohmann/json headers, for the generated-C++ tests.
- The code audit ([tools/audit](tools/audit)) enforces
  [DOCTRINE-code.md](tools/audit/DOCTRINE-code.md): short functions and files, no magic values,
  sentinel errors, a `doc.go` and a running example per package, and a baseline in `.sovaudit/`
  that only shrinks.
- Goldens under `examples/**/expected/` are written by the compiler (`-update`), never by hand.
  Diagnostics come only from the registry generated from [spec/ERRORS.md](spec/ERRORS.md).
- Contributors start at [DOCTRINE.md](DOCTRINE.md); the build order and milestones are in
  [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) and [meta/plan.md](meta/plan.md).

There is no license file yet.
