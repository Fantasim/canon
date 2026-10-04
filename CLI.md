# Canon command line and embedding API

Version: **0.1 (being locked)**, for the language described in [SPEC.md](SPEC.md). Nothing is
implemented yet.

The exact Go types and signatures of the embedding API (§5) are in [spec/API.md](spec/API.md),
which wins where the two differ; the text formats of findings follow SPEC §21. The sample outputs
in this document (findings, `test`, `explain`, `refs`, summaries) are the golden formats: an
implementation prints exactly these layouts. What this document leaves to the implementation is
fixed in [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) §8.

---

## Contents

1. [Overview](#1-overview)
2. [Common behaviour](#2-common-behaviour)
3. [Commands](#3-commands)
4. [Editor integration](#4-editor-integration)
5. [Embedding API (Go)](#5-embedding-api-go)
6. [Recipes](#6-recipes)

---

## 1. Overview

`canon` is one self-contained binary written in Go, built from the module
`github.com/fantasim/canonlang` (DECISIONS 23). It has no runtime dependencies and never touches
the network.

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
| `canon convert <value>` | turn a `load`ed JSON source into `.canon` source, proven lossless |
| `canon i18n stub \| status` | manage translation files |
| `canon lock check` | verify `canon.lock` against the sources without building |
| `canon lsp` | language server on stdio |
| `canon edit` | apply one edit request (JSON) through the edit API |
| `canon rename <name> <new>` | rename a Canon name everywhere it is used |
| `canon version` | compiler and language versions |
| `canon guide [topic]` | the agent guide: the language and the workflow, for an AI agent |

---

## 2. Common behaviour

### 2.1 Finding the project

`canon` looks for `project.canon` in the current directory, then in each parent. `--project <dir>`
overrides the search. Every path the commands print is relative to the project root, or
`@root/...` for files under a declared root.

### 2.2 Selecting packages

Commands that take packages accept, in any combination:

| Argument | Selects |
|---|---|
| (nothing) | every package of the project |
| `game.items` | one package |
| `game...` | a package and every package below it |
| `./game/items` | the package of a directory |
| `path/to/file.canon` | the package of a file |

Imports of the selected packages are always loaded; only the selected packages are emitted,
formatted or tested.

### 2.3 Global flags

| Flag | Meaning |
|---|---|
| `--project <dir>` | project root (default: search upward) |
| `--root <name>=<dir>` | use `<dir>` for the declared root `<name>` instead of the path in `project.canon`; repeatable. Tests and fixtures use it to redirect every root (`examples/_fixtures/README.md`); a name the project does not declare is a usage error (exit 2) |
| `--layer <name>` | apply a layer (SPEC §19); repeatable, applied in order; a name that matches no layer file of any loaded package is `E1901` (exit 2) |
| `--format text\|json` | output format (default `text`) |
| `--color auto\|always\|never` | colours the severity word and code of each finding header in text output (default `auto`: only on a terminal; a non-empty `NO_COLOR` turns `auto` off; `always` wins over it) |
| `-q`, `--quiet` | print errors only |
| `--max-warnings <n>` | exit with code 4 when there are more than `n` warnings (default: unlimited) |
| `--lang <code>` | language of check messages that have translations (default: the source language); a code with no translation file falls back to the source language |

Environment variables change how `canon` **prints**, never what a program computes (the purity
law applies to evaluation, SPEC §1.1).

### 2.4 Output

Text findings follow SPEC §21.1: `severity[CODE]`, two spaces, the location; detail lines
indented by 2 spaces; a blank line between findings; a summary line at the end. The exact text
templates (detail lines, related locations, the summary line) are fixed in
[spec/API.md](spec/API.md) §4.

```
error[E3501]  @resource/Server/System/farm_config.json:212:18
  farm.modelTypes[3].levels[2].productionItem: unknown key "II_SYS_SYS_SCR_FARM3" in resource.vocab.items
  expected by resource/farm/farm.canon:41 (productionItem: ref items)

2 errors, 5 warnings in 3 packages (1.4 s)
```

With `--format json`, each finding is one line of JSON, with its keys in a fixed order
([spec/API.md](spec/API.md) §4.2):

```json
{"severity":"error","code":"E3501","file":"@resource/Server/System/farm_config.json","line":212,"col":18,"endLine":212,"endCol":41,"pointer":"/modelTypes/3/levels/2/productionItemDefine","package":"resource.farm","path":"farm.modelTypes[3].levels[2].productionItem","message":"unknown key \"II_SYS_SYS_SCR_FARM3\" in resource.vocab.items","related":[{"file":"resource/farm/farm.canon","line":41,"col":3,"endLine":41,"endCol":28,"note":"productionItem: ref items"}]}
```

followed by one summary line `{"summary":{"errors":2,"warnings":5,"packages":3,"ms":1400}}`, which
gains `"truncated":[{"package":…,"errors":…,"warnings":…}]` when findings were dropped.

| Field | Content |
|---|---|
| `severity` | `"error"` or `"warning"` |
| `code` | `"E3501"` |
| `file`, `line`, `col`, `endLine`, `endCol` | the span; lines and columns are 1-based, columns count UTF-8 bytes (the LSP converts to UTF-16), the end is exclusive; omitted together when the finding has no file |
| `pointer` | the RFC 6901 pointer of the value, when `file` is a loaded JSON file |
| `package` | the package the finding belongs to |
| `path` | the value path (§2.6), starting at the value's name, without the package, when the finding is about a value |
| `message` | the message, in the `--lang` language when translated |
| `check` | the name of the `check` that produced the finding, if named |
| `layer` | the layer that set the value, if any |
| `related` | `[{file, line, col, endLine, endCol, note}]`, other locations |
| `stack` | `[{fn, file, line, col, endLine, endCol}]`, the Canon call stack of an evaluation error, innermost first (at most 16 frames) |
| `moreFrames` | the number of frames cut from `stack` (API.md F5, F13) |
| `reads` | the field paths a check read, which the studio highlights (API.md F5) |

Optional fields are omitted when empty. Findings are sorted by file, line, column, code and
message, findings without a file first. After 1,000 findings in one package, the rest are dropped
and counted in the summary.

`test`, `explain`, `refs` and `--watch` print the text layouts shown in §3. With `--format json`
every command prints one JSON object per line, then the summary line (`explain` prints its one
object and no summary, IMPLEMENTATION-PLAN §8.1); the object of each command
is given with it in §3.

### 2.5 Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | at least one error finding, or `--check` found something out of date |
| 2 | usage error: bad flag, unknown package, missing `project.canon`, a `--layer` name that matches no layer file (`E1901`) |
| 3 | internal compiler error (a bug in `canon`; the message says how to report it) |
| 4 | more warnings than `--max-warnings` |
| 130 | interrupted (SIGINT, SIGTERM): the command stops, and any write in progress is finished or rolled back, so no output is left half-written |

### 2.6 Value paths

`explain`, `refs`, `convert`, findings and the edit API address values with the same path
syntax:

```
// modelTypes is keyed by typeId: [3] is the key 3. levels is a plain list: [0] is a position.
farm.modelTypes[3].levels[0].productionItem
farm.modelTypes[#0] // the first element of a keyed list, by position
items.II_WEA_AXE_ANGEL.kind // table entry by key
items[II_WEA_AXE_ANGEL] // the same
adventureQuests.styles[daily].taskWeights // enum-keyed map: member name…
adventureQuests.styles["daily"].taskWeights // …or its wire value, quoted
config.paths["resource root"] // map key that is not an identifier
resource.vocab:Element.FIRE // an enum member, with the package qualifier
```

- On a **plain list**, `[n]` is a position (from 0).
- On a **table or keyed list**, `[k]` is always a **key** (an identifier, a quoted JSON string,
  or a number for integer keys), never a position; `.k` also names an entry whose key is an
  identifier.
- On a **map**, `[k]` is a key: an enum member by name or by quoted wire value, a ref by key, a
  string as an identifier or quoted, an integer in decimal.
- `[#n]` is a position (from 0) in any ordered collection. Paths printed by `canon` never use it.
- A path starts with a public value or constant of a package; `pkg:value` qualifies it when the
  name is ambiguous (`resource.farm:farm.global`) and may also name a `local` value. A root may
  also be an enum, in the form `Enum.member`.
- Paths printed by `canon` are canonical: keyed-list elements as `[key]`, table entries as
  `.key`, and findings without the package qualifier (it is the finding's `package`).

What a path may point to when editing (computed values, defaulted fields, spread-built values,
layered values) is defined in [spec/API.md](spec/API.md) §6 and §7.

### 2.7 Cache

Build results are cached in `.canon/cache/` at the project root, keyed by the hash of the build
manifest (SPEC §13.2): compiler version, language version, layers, `--lang`, and a SHA-256 of
every source and loaded file. The cache format is versioned and opaque. Deleting the directory is
always safe. `.canon/` belongs in `.gitignore`.

---

## 3. Commands

### 3.1 `canon init`

```
canon init [--name <project>]
```

Creates `project.canon` with `canon: "<current language version>"`, an empty `roots` block and
`languages: [en]`, plus a `.gitignore` entry for `.canon/`. Refuses to run where a
`project.canon` exists.

### 3.2 `canon new`

```
canon new <package>
```

Creates the package directory and `<last-segment>.canon` containing the `package` line and a doc
comment placeholder.

### 3.3 `canon check`

```
canon check [packages…] [--layer …] [--watch]
```

Runs every phase of a build except emit (SPEC §11.2, [spec/EVALUATION.md](spec/EVALUATION.md)
§1): every value of the selected packages is evaluated, verified and checked, and the results of
`export fn`s and the conformance vectors of translated functions are computed as `build` would
emit them (stage E), so `check` and `build` report the same findings, `E9008` and `E9009`
included. No file is written. Prints every finding, then the summary.

- `--watch`: re-check on every change to a source, a loaded file or a layer, printing only what
  changed since the previous run (cycles as spec/IMPLEMENTATION-PLAN.md §8.1 says).

Exit: 0, 1, 2, 3, 4.

### 3.4 `canon build`

```
canon build [packages…] [--layer …] [--target go|cpp|ts|json|view|text]… [--check] [--watch]
            [--adopt <path>]…
```

Runs `check`; if there is no error in any loaded package, selected or imported, writes every
`emit` output of the selected packages and appends new stable values to each `canon.lock`. An
imported package's error findings are then printed with the selection's, so nothing is refused
without its reason. View models (`emit view`) are written even when there are errors, so the
studio can show them; code, data and the lock are not. A build with `--layer` never writes
`canon.lock` (spec/LOCK.md §6.1).

| Flag | Meaning |
|---|---|
| `--target t` | emit only these targets (repeatable) |
| `--check` | write nothing; exit 1 if any output or lock would change (conformance tests included). For CI |
| `--watch` | rebuild on every change; used by a local server and by the studio |
| `--adopt <path>` | take over the hand-written file at `path`, which the build would otherwise refuse to overwrite (`E8001`): the header of a legacy C++ struct moving to `access: both` (SPEC §15.3). Only a C++ header can be adopted: a JSON output without a `$schema` is `E8001` even when listed. The build prints `adopting <path>`; from then on the file carries the marker. Under `--check` nothing is adopted: the output is reported and counted as stale. Repeatable |

- Outputs are written atomically: into a temporary file, then renamed. A failed build leaves every
  previous output untouched.
- A file is rewritten only when its content changes, so timestamps (and the C++ build) are not
  disturbed needlessly.
- Every generated file carries its generated marker on its first line (SPEC §15.1): Go
  `// Code generated by canon from <dir>/. DO NOT EDIT.`, C++ and TypeScript
  `// GENERATED by canon from <dir>/. DO NOT EDIT.`, where `<dir>` is the package directory
  (`resource/vocab/`); the runtime helper files have their own fixed markers; JSON data files
  start with a `$schema` key of the form `<name>@<8 hex digits>`. `canon build` refuses to
  overwrite an existing file without a marker (`E8001`), so hand-written code is never clobbered.
  A hand-written file is taken over only explicitly: `canon build --adopt` for a legacy C++
  header, `canon convert --adopt` for a JSON source (§3.10).
- A build with `--layer` writes the same `out` files as a plain build: do not commit outputs of a
  layered build.
- **Report.** After the findings and the summary line (§2.4), the text report prints one line
  `adopting <path>` per adopted output, then lists the outputs the build changed (with `--check`:
  would change), grouped by target in the order `go`, `cpp`, `ts`, `json`, `view`, `text` (DECISIONS 294): a line
  `<target>:`, then one line per output, indented two spaces, in byte order; an adopted output is
  listed in its group too. Unchanged outputs are not listed. A `canon.lock` is listed, under
  `lock:`, only when it gains lines. Under `--check` every listed output and lock is prefixed
  `stale `. Every printed path is a display path (§2.1), in error messages too:

  ```
  0 errors, 0 warnings in 1 package (0.2 s)
  json:
    @out/tiers.json
  lock:
    a/canon.lock
  ```

- With `--format json`: the findings, then one `output` line per output, `unchanged` ones
  included, then one `lock` line per appended lock line, then check's summary (`truncated` kept)
  followed by `"written"` and `"stale"`: the number of outputs and locks changed, or, under
  `--check`, that would change (spec/IMPLEMENTATION-PLAN.md §8.1). An output's `status` is
  `written`, `unchanged`, `stale` or `adopted`:

  ```json
  {"output":{"path":"@out/tiers.json","target":"json","package":"a","status":"written"}}
  {"lock":{"package":"a","file":"a/canon.lock","line":"table  a.tiers  low"}}
  {"summary":{"errors":0,"warnings":0,"packages":1,"ms":12,"written":2,"stale":0}}
  ```

- `-q` prints error findings and the summary only: warnings, the output listing and the lock
  listing are not printed, in text and JSON alike.

Exit: 0, 1, 2, 3, 4.

### 3.5 `canon test`

```
canon test [packages…] [--run <regex>] [--layer …]
```

Runs the `test` blocks whose name matches `--run` (an RE2 regular expression with search
semantics: `--run overlap` matches any name containing `overlap`; default: all), in package, file
and source order. Prints each failing `expect` with its location, the expected and actual values,
and for `fails`/`warns` the findings that were actually produced. `fails` and `warns` match a
message text, a check name or a code (`expect v fails E3204`, SPEC §18).

```
FAIL  resource/events/event.canon:176  overlapping windows are refused
  resource/events/event.canon:177  expect { ...sample, schedule: [at(Mon, 20, 22), at(Mon, 21, 23)] } fails "overlaps an earlier window"
  got: no finding

FAIL  pipeline/potion.canon:39  healFor never overheals and never goes negative
  pipeline/potion.canon:41  expect p.healFor(200) == 200
  expected: 200
  got: 500

12 passed, 2 failed (0.3 s)
```

The layout is exact (the second failure is `pipeline`'s test with `healFor` broken to
`return heal`, IMPLEMENTATION-PLAN.md M2):

- One block per failing test: `FAIL`, two spaces, `<file>:<line>` of the `test` keyword, two
  spaces, the test name. Passing tests print nothing (`-v` prints `ok  <file>:<line>  <name>`).
- Then, for each failing `expect` of the test in source order: two spaces, `<file>:<line>` of the
  `expect` keyword, two spaces, the `expect` statement's source text with every run of whitespace
  replaced by one space; then its detail lines, each indented two spaces:
  - `expect a == b`: `expected: <b>` then `got: <a>`; any other comparison `a op b`:
    `expected: <op> <b>` then `got: <a>`; any other Boolean: `got: false`. Values are written in
    their canonical text form (STDLIB.md §9);
  - `fails`/`warns`: `got: no finding`, or one line `got: <severity>[<CODE>]  <message>` per
    finding the subject produced, in F2 order (API.md §4.1);
  - `passes`: one line `got: <severity>[<CODE>]  <message>` per finding produced;
  - an `expect c` whose evaluation produced error findings (EVALUATION.md §10.3): one line
    `got: <severity>[<CODE>]  <message>` per finding, after the lines above.
- A hard error outside an `expect` subject, which stops the test (EVALUATION.md §10.4), is one
  more line after the failures: two spaces, `<file>:<line>` of the error, two spaces,
  `<severity>[<CODE>]  <message>`.
- A blank line after each block, then the summary `<p> passed, <f> failed (<duration>)`, with the
  duration as in API.md F15 (goldens write `(…)`).

JSON line per test: `{"test":{"package", "name", "file", "line", "status": "pass"|"fail",
"failures": [{"file", "line", "col", "expect", "expected", "got", "findings": [finding…]}]}}`,
then `{"summary":{"passed":12,"failed":1,"ms":300}}`.

Exit: 0 when every test passes, 1 otherwise.

### 3.6 `canon fmt`

```
canon fmt [paths…] [--check] [--diff] [--json-sources]
```

Rewrites `.canon` files in the canonical layout defined by [spec/FORMATTER.md](spec/FORMATTER.md).
Comments are kept. The layout never aligns columns, so a one-value edit is a one-line diff.
`fmt` evaluates nothing, except what `--json-sources` needs (below). A file with a syntax error is
left unchanged: its errors are printed and the exit code is 1; so is a `project.canon` with a
syntax error, and then nothing is written (exit 2 stays for a missing file). `fmt` does not report
naming conventions; `canon check` does (`W1003`).

| Flag | Meaning |
|---|---|
| `--check` | write nothing; exit 1 if any file is not formatted |
| `--diff` | print the changes instead of writing |
| `--json-sources` | also normalize every JSON file read by `load` to the canonical JSON source layout of [spec/FORMATTER.md](spec/FORMATTER.md) §14 (done once, before the studio edits them: see §5.3): 2-space indent, one member or element per line, existing key order and unknown keys preserved, numbers read into Canon fields re-printed canonically. This is not the layout of emitted data files (SPEC §14.3) |

- `--json-sources` learns which files are loaded, and the Canon field types of their numbers, by
  running the check of the packages holding the formatted files and evaluating only the `const`s
  and `let`s whose declaration holds a `load`; it reports nothing about them and writes nothing
  else. A number some Canon field reads is re-printed in the canonical text of WIRE.md §7.2. Every
  other number is kept as written: an unknown key's, a `none` marker, a `Duration` that is not a
  whole count, a token several loads read with different types or canonical texts or only an
  unforced reader reads, an `E7103` `1.0` in an `Int` field nothing reads. An explicit
  `load(…, format:)` decides whether a file is JSON; globs match as `load`'s do. A JSON source that
  is not UTF-8 is reported as `E7105`.
- A symbolic link is never replaced: its target is written when it lies inside the project or a
  root, else left alone.
- `--diff` alone writes nothing and exits 0; a writing run prints nothing; `-q` keeps the
  `--check` list and the diffs (they are the result, not findings). The JSON lines are in
  spec/IMPLEMENTATION-PLAN.md §8.1.

### 3.7 `canon explain`

```
canon explain <path> [--layer …] [--depth <n>]
```

Prints the final value at `path`, its type, and for every part of it the file and line that set
it, including which layer amended it.

```
config.server.port = 9000  Int(1024..=65535)
  set by  service/resourcestudio/louis.layer.canon:11  layer louis
  default  service/resourcestudio/resourcestudio.canon:29  8765
```

The first line is `<path> = <canonical text>`, two spaces, the canonical type text. Then one line
per origin, from the one that set the final value back to the one it replaced, each indented two
spaces: `<origin>  <file>:<line>  <detail>`. Columns are separated by two spaces and never padded
(DECISIONS 18). Origins (API.md §5.2 `Origin`): `set by` (a layer amendment; detail
`layer <name>`), `literal` (no detail), `default` (detail: the default's canonical text),
`loaded` (detail: the JSON pointer as `#/…` for JSON, `row <n>` for CSV, nothing for defines and
text), `spread` (detail: the spread
expression), `computed` (detail: the expression, then one `  in <fn> (<file>:<line>)` line per
stack frame, API.md F13).

For a computed value, `explain` shows the expression and the call stack that built it. For an
exported function, it shows how it is emitted (getter, lookup or translated) and why.

JSON line: `{"explain":{"path", "type", "text", "value", "origin", "parts"}}`, with `text` the
canonical Canon text, `value` the wire form, `origin` as `Value.Origin` in
[spec/API.md](spec/API.md) §5.2, and `parts` the same objects for the parts, down to `--depth`.

### 3.8 `canon refs`

```
canon refs <path> [--layer …]
```

Lists every place that references the entry or enum member at `path`: values holding a `ref`
to it, map keys, and the names in code (function bodies, defaults, constants), views, checks and
layer amendments. Every package of the project is searched.

```
resource.vocab:items.II_GEN_MAT_MOONSTONE is referenced 5 times
  @resource/Server/Event/EventConfig.json:47  value  resource.events:eventConfig.events[moonstone_rain].kind.itemId
  @resource/Server/Quest/adventure_quest_config.json:95  key  resource.adventurequest:adventureQuests.hourlyTargets[ECONOMY_DROP_ITEM].ratesBySpecific[II_GEN_MAT_MOONSTONE]
  @resource/Server/System/heistia_config.json:13  value  resource.heistia:heistia.tasks[1].filterParam
  resource/events/event.canon:172  value  resource.events:sample.kind.itemId
  resource/events/event.canon:186  code  resource.events
```

The first line is the target's qualified canonical path, ` is referenced `, the count and `times`
(`time` when it is 1). Then one line per reference in API.md R8 order: two spaces,
`<file>:<line>`, two spaces, the kind (`value`, `key`, `code`, `view`, `check`, `layer`), two
spaces, and `<package>:<path>` for `value` and `key`, or the package alone for the others. No
column is padded.

This answers "what breaks if I retire this?" before anyone tries.

JSON line per reference: `{"ref":{"kind", "package", "path", "file", "line", "col"}}`, with
`kind` one of `value`, `key`, `code`, `view`, `check`, `layer` (API.md §5.3), then
`{"summary":{"target", "count"}}`.

### 3.9 `canon infer` (dropped)

Not part of v0.1 (DECISIONS 188). A domain's first type is written from its JSON Schema, its
loader and its data, then proven by `canon check` over every file (§6.4).

### 3.10 `canon convert`

```
canon convert <value> [--layout <template>] [--adopt] [--dry-run]
```

Turns the JSON source of a `load`ed value into `.canon` source (DECISIONS 11). The value's type
must already be written in Canon, and the value must be read by `load` or `load.dir` of JSON or
CSV files; anything else is a usage error (exit 2).

**Before converting, the runtime must read emitted data.** The value must already be written by
its package's `emit json`, so the runtime reads a generated data file through a generated loader
(`data` mode, or the `types`-mode decoder of the emitted value), not the source files; otherwise
`convert` explains this and exits with 2. There is no `bare` output (SPEC §22), so an emitted file
never has the shape a hand-written loader of the legacy file expects.

1. Reads the value through its `load` as usual.
2. Prints it as `.canon` source with clean names, canonical literals, and without fields equal to
   their defaults: a table or keyed list becomes one `entry` file per entry (for a keyed list the
   key field is implied, SPEC §4.3), placed by `--layout` (default: the collection's `@files`
   annotation, else `<collection>/<key>.canon`), and the `@files` annotation is added to the
   `let` with the template used; any other value becomes a literal in the declaring file.
3. Replaces the `load(…)` expression with the new source (`{}` or `[]` for a table or keyed list
   whose entries went to their own files).
4. **`--adopt` keeps the old path.** For a single-file `load`, the package's `emit json` for the
   value is changed to write to the path of the converted source file, so deployments keep reading
   the same path, now a generated data file. That file has no generated marker yet: the build prints
   `adopting <path>` and takes it over. If that `emit json` writes several values, `--adopt` is a
   usage error. When that `emit json`'s `out` is a list (CODEGEN §2.1), the path is added as a
   further entry, unless an entry already equals it as a resolved path (WIRE §2.2); adding it is a
   usage error when it would share an owning root with another entry (`E8009` `outRoot`, CODEGEN
   §2.8) or when the list's entries are directories (`E8009` `outForm`), checked in that order:
   already an entry, then `outForm`, then `outRoot` (DECISIONS 269, 270). It is, with
   `canon build --adopt` (§3.4), the only way to take over a file (SPEC §15.1).
5. **Proves the conversion lossless.** It builds the project before and after, in memory, and
   compares (a) the converted value and every value that depends on it (equal values, entry order
   and retired flags), and (b) every emitted output, byte for byte, except provenance in view
   models (source file lists, `usage`, finding locations). This works also for a domain that emits
   only a view model. If anything differs, nothing is written and the differences are printed.

`--dry-run` prints what would be written. On success the files are written atomically; the JSON
sources are left in place (except an adopted file, which now holds the emitted data) and `convert`
prints the command to delete them (`git rm …`), so the conversion lands as one reviewable commit.
Details: [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) §8.3.

### 3.11 `canon i18n`

```
canon i18n stub <lang> [packages…]
canon i18n status [packages…] [--lang <lang>]… [--list] [--format text|json]
```

- `stub` adds every missing key of the selected packages to a translation file (by default
  `<dir>/<last segment>.<lang>.canon`), each with the source text as a comment and an empty
  translation `""`. Existing lines are never touched. `<lang>` must be a project language other
  than the source language (exit 2 otherwise).
- `status` prints, per package and non-source language (or only the `--lang` ones), the number of
  keys, translated, missing and unknown, and the translation files. `--list` adds one line per
  missing or unknown key; `--format json` prints one object per row, then a summary.
- `check` and `build` report missing keys only as one `W1701` per package and language, with the
  count, for packages that emit a view.

Key catalogue, stub layout and status layout: [spec/I18N.md](spec/I18N.md) §3, §9 and §10.

### 3.12 `canon lock check`

```
canon lock check [packages…]
```

Verifies that no locked id, code or `@stable` value was removed, renamed or reused (SPEC §12).
It evaluates only the stable collections and what they depend on (including their `load`s), and
runs no check. Values not locked yet are reported as one `W6006` per package ("run
`canon build`"), so a pre-commit hook catches a forgotten build. Usually fast enough for a
pre-commit hook; a stable table computed from large loaded data costs what evaluating that data
costs. Details: [spec/LOCK.md](spec/LOCK.md) §8.

### 3.13 `canon lsp`

```
canon lsp
```

Starts the language server on stdin/stdout (§4).

Exit: 0 after `shutdown` then `exit`; 1 when `exit` or the end of input comes before
`shutdown` (LSP 3.17), or when the input stream breaks (bad framing, a read error: its text on
stderr); 2 for an argument; 130 when interrupted.

### 3.14 `canon version`

Prints the compiler version and the build's commit, the language versions it supports (every
known minor of its major version), and the format versions it writes, on three lines:

```
canon 0.1.0 (<commit>)
language 0.1
formats canon-fp v1, canon-vm/1, canon.lock v1
```


### 3.15 `canon edit`

```
canon edit [request.json] [--layer …] [--edit-layer <name>]
```

Applies one edit request through the edit API (§5.3, [spec/API.md](spec/API.md) §7 to §10), so
the write is checked, atomic and minimal. The request is the JSON form of API.md §8.8, read from
the file or, without one, from stdin, plus one optional top-level key read by the command itself:
`"editLayer"`, which sets `Options.EditLayer` as `--edit-layer` does (both given and different is
a usage error). Before editing, the command checks every package, so its revision covers the
whole project and a printed `base` is current for the next run. A request with no ops (`{"ops":[]}`) writes nothing and
prints the current `revision`: the way to get a `base` before a first edit.

`canon edit` is a command for agents: its output is always JSON lines, whatever `--format` says
(DECISIONS 274). First one object:

```json
{"edit":{"applied":true,"revision":"r1:…","changes":[{"kind":"modified","path":"resource/farm/farm.canon"}],"dropped":[],"undo":{"base":"r1:…","ops":[{"op":"set","path":"farm.modelTypes[3].maxLevel","source":"9"}]}}}
```

then one JSON line per finding and the summary line (§2.4), with the elapsed time. `undo` is a
complete request that reverts the edit's values when given back to `canon edit` (API.md E22:
values, not layout); it carries `"editLayer"` when the edit had one. With `"dryRun": true`
nothing is written and `applied` is false. An edit refused for its findings (`ErrRejected`)
prints the object with `applied` false, empty `changes` and `dropped` and no `undo`, then the
findings, and exits 1. A `project.canon` with errors, or a poisoned value, prints its findings
as JSON lines and the summary. Any other API error prints its text (API.md X1) on stderr and
exits as API.md §15 says. A request whose top level is not a JSON object is `ErrBadOp`.

Exit: 0, 1, 2, 3.

### 3.16 `canon rename`

```
canon rename <name> <new-name> [--dry-run] [--layer …]
```

Renames a Canon name and everything that names it, in one atomic, checked and minimal edit: the
`RenameName` op of the edit API ([spec/API.md](spec/API.md) §8.9, DECISIONS 275). `<name>` is:

```
pipeline:Potion.heal                 // a field of a record
features.renames:Reward.item.count   // a field of a variant case
teamboard:Status                     // a type
teamboard:canTransition              // a function; teamboard:TimeOfDay.minutes, a method
resource.farm:farm                   // a let; FARM_MAX_MODELS, a const named by one package only
balance.parity:sweep.total           // a parameter or local, declared once in its function
balance/parity/sweep_plan.canon:42:9 // the identifier at that position
```

The declaration and every use follow in every package: qualified uses and `import` lists, record
literals in data, `entry` lines, named arguments, views, translation keys (I18N.md K4), amendment
paths in every layer file, `emit … values:` names and `@files` variables. A field on the wire
without a positional wire name gains `@json("<old wire name>")`, so no data file changes
(DECISIONS 3). Comments, texts, file paths, data files and `canon.lock` are never changed;
generated code and emitted data follow at the next `canon build`.

Refused: entry keys, enum members and variant cases, which are data (`ErrStableKey` when stable,
else `ErrBadOp`; keys change with `canon edit`'s `rename` op); a stable table's `let`, a `@codes`
enum or a `@stable` field (`ErrStableKey`, LOCK.md §4.6); a package holding a broken
declaration; a collision; and a rename that would make another name refer to something else
(`ErrNameClash`).

Like `canon edit`, its output is always JSON lines: one `{"rename":{…}}` object with the keys of
§3.15's `edit` object, whose `undo` is the reverse rename as a request for `canon edit`, then the
findings and the summary. Errors print and exit as §3.15 says.

Exit: 0, 1, 2, 3.


### 3.17 `canon guide`

```
canon guide [topic]
```

Prints the agent guide embedded in the binary (DECISIONS 276): with no topic, the index, which
lists the topics in one line each; with a topic, that topic. The text matches the binary's
version. An unknown topic is a usage error that lists the topics. Exit: 0, 2.

---

## 4. Editor integration

`canon lsp` implements the Language Server Protocol. An editor extension provides syntax
highlighting (a hand-written TextMate grammar, kept in step with [spec/GRAMMAR.md](spec/GRAMMAR.md))
and starts `canon lsp`. Positions are exchanged in UTF-16 code units, as the protocol requires;
findings are converted from their UTF-8 byte columns.

| Feature | Behaviour |
|---|---|
| diagnostics | every finding, as you type, including findings located in `load`ed JSON files, published even for files the editor has not opened |
| hover | type, doc comment, default, and for values the computed value |
| go to definition | from a use to its declaration; from a `ref` value to the entry, including into JSON |
| find references | same as `canon refs` |
| formatting | `canon fmt` |

The server is for reading: completion, code actions and rename are not offered (DECISIONS 274).
Values are edited in the studio or, by agents, with `canon edit`; names with `canon rename` (§6.5).

---

## 5. Embedding API (Go)

A studio (the generic editor of SPEC §16) links the compiler as a Go library. The same package
powers the CLI and the language server. This section describes what the API does; the exported
types and signatures (`Options`, `Project`, `CheckResult`, `Value`, `Edit`, `Op`, `Lit`,
`EvalRequest`, `EvalResult`, `Event`, `BuildOptions`, the revision type and the error values) are
fixed in [spec/API.md](spec/API.md) and its source file [api/canon.go](api/canon.go), which win
where this section differs.

### 5.1 Opening a project

The public package is `canon`, in the `api/` directory of the compiler module
`github.com/fantasim/canonlang` (DECISIONS 23).

```go
import canon "github.com/fantasim/canonlang/api"

root, err := canon.FindProject(".")
p, err := canon.Open(root, canon.Options{
    Layers: []string{"alice"},
    Lang:   "fr",
})
defer p.Close()
```

A `Project` holds the parsed sources and caches. It is safe for concurrent use: reads run in
parallel, and edits are serialized (a single writer). `Options.Roots` redirects declared roots
(the API side of `--root`), and `Options.EditLayer` sends edits to a layer (§5.3).

### 5.2 Reading

```go
// Findings of those packages (every build phase but emit), as a *CheckResult.
res, err := p.Check(ctx, "resource.farm")
// SPEC §16.10, as JSON bytes or decoded into Go structs.
vm, err := p.ViewModel(ctx, "resource.farm")
// A *canon.Value: the value with its type, text, origin and editability.
v, err := p.Value(ctx, "resource.farm:farm.modelTypes[3]")
// Everything that references an entry or enum member.
refs, err := p.Refs(ctx, "items.II_GEN_MAT_MOONSTONE")
```

`Check` returns `(*CheckResult, error)`: the findings are in the result, and `err` is set only
when the call itself fails (an unknown package or layer, a closed project, a cancelled context,
an internal error). Values and paths use the syntax of §2.6; every path the API returns is
canonical and, for values, package-qualified.

### 5.3 Editing

The studio never edits text. It sends **operations on values**:

```go
res, err := p.Edit(ctx, canon.Edit{
    Base: rev, // revision the studio last saw
    Ops: []canon.Op{
        canon.Set("farm.modelTypes[3].maxLevel", canon.Int(10)),
        canon.Add("farm.modelTypes[3].levels", canon.Obj{"level": canon.Int(10), …}),
        canon.Remove("farm.modelTypes[3].levels[11]"),
        canon.Move("farm.modelTypes[3].levels[2]", 0),
    },
})
```

Operation values are a closed set of typed constructors, never untyped Go values: `Bool`, `Int`,
`Float`, `Str`, `Dur`, `Member` (an enum member by Canon name), `Key` and `IntKey` (a key),
`Case`, `None`, `List`, `Obj`, `Map`, `FromJSON` (a value in its wire form) and `Source` (a Canon
literal). They are checked against the type expected at the path before anything is written: a
string for an `Int` field is refused immediately ([spec/API.md](spec/API.md) §8.2).

| Operation | Meaning |
|---|---|
| `Set(path, v)` | replace the value at `path`; setting a field to its default removes it from the source (or the JSON key) |
| `Reset(path)` | remove a field so that it takes its default |
| `Add(path, v)` / `Insert(path, i, v)` | append to, or insert at position `i` of, a list or keyed list |
| `AddEntry(path, key, v)` | add an entry to a table or map; a new entry of a collection filled by `entry` declarations gets a new file, placed by the `@files` template; on a stable table the new id is appended to `canon.lock` in the same atomic edit |
| `Remove(path)` | remove a list element, map entry or non-stable table entry; an entry that has its own file loses its file |
| `Move(path, i)` | reorder within a list, keyed list, map or table |
| `Rename(path, newKey)` | change a key and every reference to it (not on stable tables) |
| `Retire(path)` | retire a stable entry or `@codes` member (SPEC §12) |
| `Unretire(path)` | refused with `ErrStableKey`: a retired id comes back only through a reviewed hand edit of `canon.lock` (SPEC §12) |
| `SetCase(path, case, fields)` | change a variant's case, keeping the fields of the same name and type |

Paths follow §2.6, including `[#n]` for a position. The request also has `AllowErrors` (write a
draft even if it has errors), `DryRun` (compute everything, write nothing), `Normalize` (allow
re-printing a file that is not in canonical layout) and `Evaluate` (paths to evaluate after the
edit, as §5.6 does). Semantics:

- **Atomic.** All operations apply, or none do.
- **Checked.** After applying, the affected packages are re-checked. `res.Findings` holds the
  findings; if any is an error, the edit is refused and nothing is written, unless
  `AllowErrors` is set.
- **Cascades.** The compiler applies the consequences of an edit in the same edit: a case change
  keeps compatible fields, and an optional dependent field that no longer fits its driver is set
  to `none`. Every value dropped is listed in `res.Dropped`.
- **Editable values only.** A computed value is not editable: the edit fails with
  `ErrNotEditable`, naming the nearest editable source. A defaulted field is inserted into its
  literal in declaration order; a value built by a spread gets an override field in the spreading
  literal. Edits go to the base sources, and a value whose final value is set by an active layer
  is read-only and names the layer (MOCKUP-GAPS 48); with `Options.EditLayer` set to that layer,
  `Set`, `Reset` and `AddEntry` write amendments into its file instead.
- **Minimal writes.** Every value knows the file and span it came from (SPEC §11.5). The compiler
  re-prints only the smallest enclosing item whose text changes, in the canonical layout, keeping
  its comments ([spec/API.md](spec/API.md) §9, [spec/FORMATTER.md](spec/FORMATTER.md) §13). For a
  JSON source the same happens in canonical JSON. A file that is not in canonical layout is
  refused (`ErrNotCanonical`) unless `Normalize` is set. Since the layout never aligns columns,
  the diff is the change and nothing else.
- **Concurrency.** `Base` is the revision the studio last read. If a file read by the affected
  packages changed since (someone edited a file, pulled a commit), the edit is refused with
  `ErrStale` and the studio reloads. Staleness is decided per package; an empty `Base` disables
  the check.

The result lists the files changed, the new revision, the findings, the values dropped by
cascades and the operations that undo the edit (`res.Undo`). Exact types, errors and rules:
[spec/API.md](spec/API.md) §7 to §10.

### 5.4 Watching

```go
err := p.Watch(ctx, func(ev canon.Event) {
    // ev.Revision, ev.Cause (external, edit or overlay), ev.Files, ev.Packages
    // ev.Findings: every current finding of ev.Packages (replace, do not merge)
    // ev.Summary, ev.Err
})
```

`Watch` returns an error only if watching cannot start; events then arrive on one goroutine, in
revision order. Changes are coalesced over 100 ms (at most 1 s after the first change). An
external change during an edit makes that edit fail with `ErrStale`.

### 5.5 Building

```go
out, err := p.Build(ctx, canon.BuildOptions{Packages: []string{"game.items"}, Check: false})
```

Same behaviour as `canon build`. `BuildOptions.Adopt` lists the hand-written outputs the build may
take over (`canon build --adopt`); an adopted output has the status `adopted`.

### 5.6 Evaluate

```go
ev, err := p.Evaluate(ctx, canon.EvalRequest{Path: "resource.farm:farm.modelTypes[3]", Draft: ops})
```

`Evaluate` is how the studio shows what depends on Canon expressions without reimplementing them
(SPEC §16.10). For the value at `Path`, with the uncommitted operations `Draft` applied in memory,
it returns the `title`, `subtitle` and `preview`, the `when` flag of every field and group that
has one, the `show` lines (with their translation keys), a heading for each element of the
collections it holds (or of the value itself when it is a collection), the resolved types of
dependent fields, the findings located there, and the values a cascade would drop. A text whose
evaluation fails is marked not OK and shown as "—"; a failing `when` counts as true. The studio
calls it after each committed edit, not on each keystroke. Details:
[spec/API.md](spec/API.md) §11.

---

## 6. Recipes

### 6.1 Make targets in a consumer repository

```make
gen:        ## regenerate code from the law
	canon build --project ../law

gen-check:  ## CI: fail if generated code is stale
	canon build --project ../law --check
```

### 6.2 CI of the law repository

```
canon fmt --check
canon lock check
canon build --check --max-warnings 0     # or no limit while migrating
canon test
```

Then, in each consumer repository, its own test suite runs the generated conformance tests
(SPEC §15.6).

### 6.3 Pre-commit hook

```
canon fmt --check && canon lock check
```

Both are usually fast: `fmt` evaluates nothing, and `lock check` evaluates only the stable
collections and their dependencies (§3.12).

### 6.4 Converting a domain

One domain at a time, each with its round-trip proof (DECISIONS 11):

```
# 1. Write the first type (game/items/item.canon) from the domain's JSON Schema, its loader
#    and its data: constraints, clamps and defaults included (DECISIONS 188).

# 2. Read the JSON through `load`: every file is now validated.
canon check game.items

# 3. Add `emit json` for the value, move the runtime to the generated loader, and ship that.
canon build game.items

# 4. JSON to .canon, proven lossless.
canon convert items --layout "items/{itemKind1}/{id}.canon"

# 5. Delete the old sources, as printed by convert.
git rm -r ../Resource/Server/Item/Items
```

Step 3 is required: `convert` refuses a value that its package's `emit json` does not write yet
(§3.10). For a single-file source whose path deployments rely on, `canon convert --adopt` makes
the `emit json` write the data file at that same path: one `out` is changed to it, and a list of
`out` entries gains it as a further entry, unless one already resolves to it (§3.10 step 4).

### 6.5 Agents editing the law

An agent edits through the compiler rather than the text, so every write is checked and minimal
(DECISIONS 274):

```
canon check --format json                 # what is wrong, with value paths and related locations
canon explain <path> --format json        # a value, its type and where each part was set
canon refs <path>                         # what uses an entry, before removing or retiring it
canon edit fix.json                       # apply value changes; prints the Undo request
canon rename <name> <new-name>            # rename a field, type, function, let or local everywhere
canon fmt                                 # after any hand edit of the code itself
```

---

## 7. Installing and updating

Linux (amd64, arm64), from the GitHub releases (DECISIONS 276):

```
curl -fsSL https://raw.githubusercontent.com/Fantasim/canon/main/tools/install.sh | sh             # latest
curl -fsSL https://raw.githubusercontent.com/Fantasim/canon/main/tools/install.sh | sh -s -- v0.2.0 # a version
```

The script downloads `canon_<version>_linux_<arch>.tar.gz`, checks it against `checksums.txt`, and
installs `canon` into `$CANON_INSTALL_DIR` (default `~/.local/bin`). Running it again updates.
The version can also come from `$CANON_VERSION`; `$CANON_RELEASE_BASE` replaces the release
download URL (a mirror, or a local test release). A tag `vX.Y.Z-suffix` is a prerelease: it is
published but never installed as the latest.
`canon version` prints the installed version. A release is made by pushing a tag `v<semver>`.

