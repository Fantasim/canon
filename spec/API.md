# Canon Go API

Status: **normative**, for language version 0.1. This document fixes the exported Go API of the
compiler library: package `canon`, source file [`api/canon.go`](../api/canon.go). The command
line (`cmd/canon`), the language server and the studio all use it. CLI.md §5 gives an overview.
Where CLI.md §5 and this document disagree, this document wins, and §16 lists the errata for
CLI.md.

Settled here: AUDIT API-01..06, VIEW-01, DIAG-02 (finding shape), and the parts of DECISIONS 12
and 20 (MOCKUP-GAPS 11, 13, 16, 17, 23, 31, 38, 45, 47, 48) that concern the API.

"MUST", "MUST NOT", "SHOULD" and "MAY" are used in their RFC 2119 sense. Every numbered rule
(`P3`, `E12`, …) is a test target: IMPLEMENTATION-PLAN.md §7.4 requires at least one test per rule.

Other documents this one depends on, and what it assumes from them:

| Document | Assumed here |
|---|---|
| GRAMMAR.md | identifiers, keyword-as-name rules (LEX-08), Canon literal syntax, `amend` path syntax |
| FORMATTER.md | the canonical layout of `.canon` files and of JSON *sources* (FMT-01, FMT-02, DECISIONS 18); `canon fmt` is idempotent |
| TYPES.md | assignability, type identity, value equality (§7.5), which fields are dependent (§11) |
| EVALUATION.md | provenance (EVL-07), poisoning, layer application (LAY-02), step budget, build phases (§1) |
| WIRE.md | the wire form of every type: `FromJSON` values and JSON-source writes use it |
| LOCK.md | when `canon.lock` gains lines (E20); that retirement is one-way (§4.3, §4.6), so `Unretire` is refused (E4) |
| VIEWMODEL.md | the view model bytes, the type encoding used in `TypeInfo.VM` and `EvalResult.Types` |
| I18N.md | translation keys of `show` lines, titles and check messages; fallback rules |

---

## Contents

1. [Package and conventions](#1-package-and-conventions)
2. [Opening a project](#2-opening-a-project)
3. [Snapshots, revisions and concurrency](#3-snapshots-revisions-and-concurrency)
4. [Findings](#4-findings)
5. [Reading](#5-reading)
6. [Value paths](#6-value-paths)
7. [Where a value can be edited](#7-where-a-value-can-be-edited)
8. [Editing](#8-editing)
9. [Minimal writes](#9-minimal-writes)
10. [Writing files](#10-writing-files)
11. [Evaluate: live view state](#11-evaluate-live-view-state)
12. [Watching](#12-watching)
13. [Building and testing](#13-building-and-testing)
14. [Formatting helpers](#14-formatting-helpers)
15. [Errors](#15-errors)
16. [Errata for CLI.md](#16-errata-for-climd)

---

## 1. Package and conventions

### 1.1 Import path

The public package is named `canon` and lives in the `api/` directory of the compiler module:

```go
import canon "github.com/fantasim/canonlang/api"   // module path: DECISIONS 23, IMPLEMENTATION-PLAN §2
```

Everything else in the module is under `internal/`. An embedder uses only package `canon`. The
contract is the package, not a file: M0 split the single `api/canon.go` by concern into the
files of `api/` to meet the code doctrine's size limits, with no change to the API
(IMPLEMENTATION-PLAN §12.4 lists the files; `api/canon.go` keeps the options and the project,
DECISIONS 68).

### 1.2 Stability

Until compiler 1.0 the API may change in any minor release. Every change to the API (`api/`) is
reviewed by the owners of `cmd/canon`, `internal/lsp` and the studio (IMPLEMENTATION-PLAN §5).

### 1.3 Positions and spans

- `Line` and `Col` are 1-based. `Col` counts **UTF-8 bytes** from the start of the line (DIAG-02).
  The language server converts to UTF-16 (CLI-04).
- A span is `File, Line, Col, EndLine, EndCol`. The end is **exclusive**: `EndCol` is the column of
  the first byte after the span. An empty span has `EndLine == Line` and `EndCol == Col`.
- `File` is the display path of CLI.md §2.1: `@root/...` for a file under a declared root, else
  relative to the project root. It uses `/`, whatever the platform (NFR-04).

### 1.4 Ordering

Every slice the API returns has a defined order, stated where it is declared. Nothing depends on
Go map iteration (NFR-05). Maps in results (`EvalResult.When`, …) are for lookup only;
`MarshalJSON` of a result writes their keys in byte order.

---

## 2. Opening a project

```go
func FindProject(dir string) (root string, err error)
func Open(root string, opts Options) (*Project, error)
func (p *Project) Close() error
```

- **O1.** `FindProject` looks for `project.canon` in `dir` and then in each parent (CLI.md §2.1).
  It returns `ErrNoProject` when none is found.
- **O2.** `Open` requires `root` to contain `project.canon`; it does not search. It reads and
  checks `project.canon`, then `project.local.canon` when it exists (GRAMMAR.md §7.2). It places
  the roots, root by root: `Options.Roots` over `project.local.canon` over `project.canon`. It
  judges which roots are present (SPEC §3.1). It then scans the file set (every `.canon` file
  under the project root except `project.canon` and `project.local.canon` at its top, skipping
  directories whose name starts with `.`, and the directory listing needed to map packages to
  directories). It parses no other source. Parsing, checking and
  loading happen on first use.
- **O3.** These make `Open` fail with a `*ProjectError`, which wraps `ErrProject` and carries the
  findings:
  - errors from `project.canon` itself (syntax, `E1001` unsupported language version, `E1002`
    unknown key, …);
  - errors from `project.local.canon` (syntax, `E1011`, `E1014`);
  - a root name `Options.Roots` does not declare (`E7003`, DECISIONS 141);
  - a required root outside the project whose directory does not exist (`E1013`), whichever of
    the three places set its path (DECISIONS 332).

  An absent optional root does not: what reads it fails at the value (`E7009`, `E3705`), and what
  writes into it is skipped (`W8024`). Every new snapshot (S1) reads both files and judges
  presence again, so a root cloned after `Open` counts from then on.
- **O4.** An unknown name in `Options.Layers` is not an `Open` error: layers are found per package
  (LAY-01), so the first call that loads packages returns `ErrUnknownLayer` if no loaded package
  has a file for that layer.
- **O5.** If `.canon/journal/` holds an unfinished edit (§10.3), `Open` rolls it back before
  anything else and logs one line at level Warn, however many journals it rolled back, whenever
  it removed a journal that decodes (even if no file needed restoring). A journal whose writer
  still runs on this host is left alone. A journal it must keep (written on another host, a file
  changed since the edit, a journal that fails N11's checks) makes `Open` fail with an error
  wrapping `ErrProject` that names the journal and the files.
- **O6.** After `Close`, every method returns `ErrClosed`, except `Revision()`, which has no
  error and returns the revision of the newest snapshot read (S10). `Close` stops every `Watch`
  and is idempotent.

### 2.1 Options

| Field | Default | Meaning |
|---|---|---|
| `Layers []string` | none | layers to apply, in order (SPEC §19, CLI `--layer`) |
| `Lang string` | source language | language of translated texts: check messages, titles, `show` labels (CLI `--lang`) |
| `EditLayer string` | `""` | when set, `Set`/`Reset`/`AddEntry` edits write amendments into this layer (§7.5) |
| `Roots map[string]string` | none | replaces the directory of a declared root (CLI `--root name=dir`); a relative directory is relative to the project root; a name that is not declared is `ErrProject`; it wins over `project.local.canon` for the roots it names (O2) |
| `FS FS` | the OS | file system used for every read and write (tests use an in-memory one) |
| `Cache string` | `<root>/.canon/cache` | cache directory; `"off"` disables the cache |
| `Workers int` | `runtime.GOMAXPROCS(0)` | parallelism; results are byte-identical for every value (NFR-05) |
| `MaxFindings int` | 1000 | findings kept per package; the rest are counted (§4.3) |
| `Logger *slog.Logger` | discard | diagnostics about the compiler itself, never findings |

- **O7.** `Options` never changes what a program computes, except `Layers` (which is part of the
  program, SPEC §19) and `Roots` (which changes the files read). `project.local.canon` is the
  same kind of input as `Roots`: it changes the files read and where outputs go, never the bytes
  generated (CODEGEN.md §2.8). Both are part of the build manifest (LOD-11).

### 2.2 The FS interface

```go
type FS interface {
    ReadFile(name string) ([]byte, error)
    Stat(name string) (fs.FileInfo, error)
    ReadDir(name string) ([]fs.DirEntry, error)
    WriteFile(name string, data []byte) error   // MUST be atomic: temp file + rename
    Rename(oldname, newname string) error
    Remove(name string) error
    MkdirAll(name string) error
}
```

Names are absolute and use `/` (on Windows, `C:/...`). `ReadDir` order is not trusted: the
compiler sorts every listing by byte order (NFR-04), so an FS MAY return entries in any order.
The determinism tests use an FS that shuffles listings on purpose.

On Windows, every path given to the API (`Open`'s root, `Options.Roots`, overlay files, §3.4)
follows one rule: `\` is a separator like `/`; a rooted path without a volume (`/law/a.canon`)
takes the project directory's volume; cleaning keeps the volume and never climbs above it; the
drive letter is written upper-case (the OS ignores its case; letter case stays significant in
every name, WIRE.md §2.1). UNC project directories (`//server/share/…`) are supported; device
paths (`//?/…`, `//./…`) and a bare `//server` with no share are not volumes: they are rooted names
on the project directory's volume.

The OS `WriteFile` stages the content in a temporary file of the target's directory (a hidden
name holding the target's base name, so `Recover` can clear leftovers, §10.3), syncs it and
renames it over the target. When the target is a symbolic link, the link's target is written; the
link is never replaced. A writer that stages to a fixed temporary name first removes whatever
entry sits there (a link itself, never its target), so a planted link cannot redirect the bytes.
On Windows it cannot replace a file another process holds open. Two optional FS capabilities:
`SyncDir(dir string) error`, which makes an edit's renames durable (§10.3), and
`OSBacked() bool`, through which an embedder's FS opts into OS change notifications (§12).

---

## 3. Snapshots, revisions and concurrency

### 3.1 Snapshots

A `Project` holds an immutable **snapshot**: the content of every file it has read, and everything
computed from them (syntax trees, types, values, findings). Each public call runs against one
snapshot from start to end, so it never sees a half-applied change.

- **S1.** At the start of each call, unless a `Watch` is active, the project **refreshes**: it
  stats every file of the current read set and every directory a glob or package scan listed.
  A file whose size or modification time changed is re-hashed; a changed hash makes a new
  snapshot. While a `Watch` is active, the watcher keeps the snapshot fresh instead (§12).
- **S2.** A new snapshot recomputes only what depends on the changed files. In v0 the unit is the
  package: a package whose files changed, and every package that imports it, are re-checked
  (NFR-02). Values are stored so that finer invalidation can come later without an API change.

### 3.2 Revisions (API-04)

```go
type Revision string
func (p *Project) Revision() Revision
```

- **S3.** A revision is `"r1:"` followed by the lowercase hex SHA-256 of the **read-set listing**:
  one line per file, `<display path> NUL <lowercase hex SHA-256 of the content> LF`, sorted by
  display path bytes. The listing covers the project's **static read set**, the same whatever a
  call analysed: `project.canon`, and `project.local.canon` when it exists; every source, translation and layer file the scan finds (O2); the
  existing `canon.lock` of every directory below the project root that holds a source file or is
  an ancestor of one (LOCK.md §2.1); and every file a `load` of any package names, its path or, for
  `load.dir`, each file its glob matches (WIRE.md §6.5), counting the `load`s of source and layer
  files alike, evaluated or not. A `load` path is a string literal (WIRE.md §6.1), so this set is
  known from the parsed sources before any checking or evaluation (DECISIONS 330). A named file
  that does not exist or cannot be read is listed as `<display path> NUL unreadable` (DECISIONS
  143). Overlays (§3.4) replace file contents in the listing. A file reached through several
  display paths is listed once, by the smallest display (bytes) the `load`s give it; a file no
  display path names is listed by its project-relative path. A snapshot therefore has one
  revision, a function of its files alone.
- **S4.** The project remembers the per-file hashes of at least the last 64 revisions it has
  produced. A revision equal to the current one is current, whoever produced it: the listing
  covers the whole static read set (S3), so another `Project` or another process reading the same
  files computes the same revision. Any other revision the project does not remember is stale,
  since it cannot be compared file by file (S5). So a revision printed by an earlier process is
  current exactly when no file of the project's static read set has changed since (CLI.md §3.15).
- **S5.** A revision is compared per package: an edit or an evaluation against `Base` is **stale**
  when a file in the read set of the packages it touches (§8.6; an edit's affected packages, E17)
  has a different hash now than at `Base`. Changes to unrelated files do not make it stale. A
  package's read set is its **static read set**: its source, translation and layer files, its own
  `canon.lock` (LOCK.md §2.1), and the files its `load`s name with the directories their globs
  walk (S3), each by real path; plus the directory listings its asset checks consulted (TYPES.md
  §13.4). It includes its directory listing, restricted to the names the scan takes as sources (O2): a source file added
  to a touched package after `Base` makes the edit stale, an editor swap file never does. A file
  first read after `Base` is compared with the first content the project read for it. Since any
  source can reshape the import graph, the edit or evaluation is also stale when any scanned
  source file, `project.canon` or `project.local.canon` changed since `Base`; the unrelated files are non-source files
  (other packages' loads, assets). An `Evaluate` without a draft touches the packages its
  `Summary` covers (V13). Per-package comparison needs `Base`'s per-file hashes, which only the
  `Project` that produced it holds (S4).
- **S6.** `Base == ""` disables the staleness check. It is for scripts and tests; the studio MUST
  send the revision it last read.

### 3.3 Concurrency (API-05)

- **S7.** A `Project` is safe for concurrent use by any number of goroutines.
- **S8.** Reads (`Check`, `CheckWith`, `Info`, `Value`, `Refs`, `ViewModel`, `Evaluate`, `Test`,
  `LockCheck`, `Build` with `Check: true`) run in parallel with each other and with a writer. Identical
  concurrent reads of the same snapshot share one computation. A view model (R9) is kept per
  snapshot and package; a new snapshot invalidates it (DECISIONS 313).
- **S9.** There is one writer at a time: `Edit`, `Build` that writes, `SetOverlay` and
  `ClearOverlay` take the project's write lock. A writer never blocks readers; readers that
  started before the writer finished keep their snapshot.
- **S10.** A writer publishes its new snapshot atomically when it returns. The revision in its
  result is that snapshot's. `Revision()` is monotonic: it only moves to a snapshot at least as new
  as the last one published, so a read that finishes late never rolls it back. Without an active `Watch`, the snapshot an `Edit`
  publishes reads again every name its commit changed or pinned: each file written, renamed or
  removed, each directory created (N3) or removed (N6), every ancestor listing of those up to the
  file-system root, and the journal's directory with its ancestors (N10); it equals a fresh read of
  the disk the edit left, except external changes made during the write, which the next refresh (S1)
  sees; a watcher being restarted counts as active (DECISIONS 254).
- **S11.** Every method takes a `context.Context`. Cancelling it makes the call return
  `ctx.Err()` promptly; a cancelled `Edit` writes nothing, or, if cancellation arrives after the
  first rename, completes (§10.3).

### 3.4 Overlays

```go
func (p *Project) SetOverlay(file string, content []byte) error
func (p *Project) ClearOverlay(file string) error
```

An overlay replaces a file's content in memory without writing it; the language server uses it for
unsaved buffers. `file` is a display path or an absolute path (on Windows, §2.2's rule applies);
a path outside the project is a `*PathError` wrapping `ErrBadPath`. Overlays are keyed by the
file's `/` display form, so `SetOverlay` and `ClearOverlay` match however the path was written.
Overlays take part in every read of their own display path and in the revision; a read of the
same real file under another name sees the file system (DECISIONS 253).

- **S12.** `Edit` refuses to write a file that has an overlay (`ErrOverlay`): the buffer and the
  disk would disagree. Files are compared by real path (resolved through the snapshot's links as
  WIRE.md §6.5 does, a dangling link through its link text), so a file an overlay covers under any
  display path is refused (DECISIONS 253).

---

## 4. Findings

### 4.1 The Finding type (DIAG-02)

```go
type Finding struct {
    Severity Severity   // "error" | "warning"
    Code     string     // "E3501"
    Span                // File, Line, Col, EndLine, EndCol (§1.3); embedded
    Pointer  string     // RFC 6901 pointer when File is a loaded JSON file, else ""
    Package  string     // package the finding belongs to
    Path     string     // canonical value path (§6.5), "" when not about a value
    Message  string     // in Options.Lang when a translation exists (I18N.md)
    Check    string     // name of the check or warn that produced it, "" if unnamed or not a check
    Layer    string     // layer whose amendment produced the value, "" if none
    Related  []Related  // other locations, in the order the producer adds them
    Stack    []Frame    // Canon call stack for evaluation findings, innermost first, at most 16
    MoreFrames int      // frames cut from Stack (F13); 0 when none were
    Reads    []string   // one-line record check without `at`: the fields its condition reads (VIEWMODEL.md J15)
}

type Related struct { Span; Note string }
type Frame   struct { Fn string; Span }
```

- **F1.** `Path` is the full canonical path of the value the finding is about, starting at the
  public or local value name (`farm.modelTypes[3].levels[2].productionItem`), without the
  package qualifier (the package is in `Package`). `Package + ":" + Path` always resolves with
  `Value`; inside a poisoned root, `Value` returning `ErrNoValue` counts as resolving, so such a
  finding keeps its precise sub-path. A finding raised while evaluating a value is about the value
  being built, down to the literal: a hard error, `E4401` (the value evaluated at the exhausting
  step) and a constant cycle `E4301` take its path; a finding in a top-level `let` initializer
  takes the let's name; in a default expression, the field's path; in an amendment's right-hand
  side or a layered conversion, the amended path; in a `where` on a loaded value, the value's
  path; in a check run on an instance, the instance's path, on replay too. An element of a keyed
  list is named by its key once the key is known, before that by the list's own path. Every copy
  of a finding carries its `Path`, the causes of R6 included.
- **F2.** Findings are sorted by (`File` bytes, `Line`, `Col`, `Code`, `Message`) (EVL-09).
  Findings with no file sort first, by `Code` then `Message` (EVALUATION.md §14).
- **F3.** Codes and messages come from [ERRORS.md](ERRORS.md), the single source of diagnostics
  (DECISIONS 27): `Message` is the template of the finding's code and variant, rendered with its
  typed arguments (ERRORS.md §1), through the registry generated from it (IMPLEMENTATION-PLAN
  §4.4).
  User checks are `E5001`/`W5001` (one-line) and `E5002`/`W5002` (block form) (CHK-01).
- **F4.** For a one-line check in a record, the span is the one given by CHK-02; `Related` holds
  the check's own location with note `"check <name>"`, or `"check"` when the check is unnamed.

### 4.2 JSON form

`Finding.MarshalJSON` writes one object with keys in this order; empty optional keys are omitted:

```json
{"severity":"error","code":"E3501","file":"@resource/Server/System/farm_config.json","line":212,"col":18,"endLine":212,"endCol":41,"pointer":"/modelTypes/3/levels/2/productionItemDefine","package":"resource.farm","path":"farm.modelTypes[3].levels[2].productionItem","message":"unknown key \"II_SYS_SYS_SCR_FARM3\" in resource.vocab.items","check":"","layer":"","related":[{"file":"resource/farm/farm.canon","line":41,"col":3,"endLine":41,"endCol":28,"note":"productionItem: ref items"}],"stack":[],"reads":[]}
```

- **F5.** Key order: `severity code file line col endLine endCol pointer package path message
  check layer related stack moreFrames reads`. Omitted when empty: `pointer`, `path`, `check`,
  `layer`, `related`, `stack`, `moreFrames` (when 0), `reads`. `file`, `line`, `col`, `endLine`, `endCol` are omitted together when there is no file.
  (The example above shows every key for completeness.) Each `related` element has
  `file line col endLine endCol note`; each `stack` element has `fn file line col endLine endCol`.
- **F6.** The CLI prints exactly this object, one per line (CLI.md §2.4); `UnmarshalJSON` reads it
  back, and refuses a key F5 does not list or a severity other than `error` and `warning`.

### 4.3 Results that hold findings

```go
type Summary struct { Errors, Warnings, Packages int; Truncated []Truncation }
type Truncation struct { Package string; Errors, Warnings int }   // counts NOT kept
```

- **F7.** At most `Options.MaxFindings` findings are kept per package; the first ones in F2 order
  are kept, chosen on the source-language messages so the kept set is the same under every
  `Options.Lang` (DECISIONS 281). The counts in `Summary.Errors`/`Warnings` include the dropped ones;
  `Truncated` lists each package that dropped some, in package-name order.
- **F8.** `Summary.Packages` is the number of packages checked.

### 4.4 Text form (DIAG-03)

The CLI prints findings in text with these templates (CLI.md §2.4, SPEC §21.1 refer here), so
goldens (`expected/findings.txt`) and every tool agree byte for byte:

```
<severity>[<code>]  <file>:<line>:<col>
  <path>: <message>
  set by layer <layer>
  expected by <file>:<line> (<note>)
  in <fn> (<file>:<line>)

<E> errors, <W> warnings in <P> packages (<duration>)
```

- **F9.** Header: the severity (`error` or `warning`), `[`, the code, `]`; then, when the finding
  has a file, two spaces and `<file>:<line>:<col>` (display path, 1-based line, 1-based column in
  UTF-8 bytes, §1.3). No line ends with a space.
- **F10.** Message: two spaces, `<path>: ` when `Path` is not empty, then the message. Each
  further line of a multi-line message is indented two spaces too
  (`  heaviest: plan (98412330 steps)`, EVALUATION.md §12.2).
- **F11.** When `Layer` is set: `  set by layer <layer>`.
- **F12.** One line per `Related` element, in order: `  expected by <file>:<line>`, then
  ` (<note>)` when the note is not empty; a related element without a file is
  `  expected by (<note>)`. For example:

  ```
    expected by resource/farm/farm.canon:41 (productionItem: ref items)
    expected by resource/farm/farm.canon:109 (check unreachable_levels)
    expected by resource/heistia/heistia.canon:62 (check)
  ```

  A related location is always something the value was checked against: the field declaration
  whose type it failed (note: the declaration, `productionItem: ref items`), the check (note: F4),
  an annotation or a refinement. A second location that is not an expectation (the first of two duplicates)
  goes into the message (`… (first at <file>:<line>)`), never into `Related`.
- **F13.** One line per `Stack` frame, innermost first: `  in <fn> (<file>:<line>)`; when frames
  were dropped, one more line `  (<n> more frames)`. `<fn>` is `f` for a function, `T.m` for a record
  or variant-level method, `V.c.m` for a case method.
- **F14.** Findings are separated by one blank line. After the last finding comes one blank line
  and the summary line; a run without findings prints the summary line alone.
- **F15.** Summary: `<E> errors, <W> warnings in <P> packages (<duration>)`, where each count is
  followed by the singular noun when it is exactly 1 (`1 error`, `1 warning`, `1 package`) and
  the plural otherwise, 0 included. A clean run prints `0 errors, 0 warnings in 1 package (…)`.
  The counts include dropped findings (F7); when some were dropped, `, <n> not shown` follows the
  package count: `2 errors, 5 warnings in 3 packages, 1234 not shown (1.4 s)`. `<duration>` is
  `<n> ms` below one second and `<s.d> s` (one decimal, rounded down) from one second on; golden
  files write it `(…)` (IMPLEMENTATION-PLAN.md §7.2).

### 4.5 Writing findings

```go
func WriteFindings(w io.Writer, findings []Finding, o WriteOptions) error
type WriteOptions struct {
    JSON     bool          // the JSON lines of §4.2 and CLI.md §2.4 instead of the text form
    Summary  Summary
    Duration time.Duration
    Golden   bool          // the text form writes the duration `(…)` (F15)
}
```

- **F16.** `WriteFindings` writes the findings in F2 order, then the summary line, in the text
  form of §4.4 or the JSON form of §4.2; it is how the CLI prints findings, and its output is the
  one the compiler's own renderer gives (IMPLEMENTATION-PLAN.md §4.4). Findings that tie on the F2
  key are ordered by every other field, so the output never depends on the order of `findings`.
  A severity other than `error` and `warning` is an error.

---

## 5. Reading

### 5.1 Check

```go
func (p *Project) Check(ctx context.Context, packages ...string) (*CheckResult, error)
func (p *Project) CheckWith(ctx context.Context, r CheckRequest) (*CheckResult, error)

type CheckRequest struct {
    Packages []string        // selectors, as `packages` above
    Lang     string          // overrides Options.Lang; "" keeps it
}

type CheckResult struct {
    Revision Revision
    Packages []string        // selected packages, sorted
    Findings []Finding       // F2 order
    Summary  Summary
    Duration time.Duration
}
func (r *CheckResult) HasErrors() bool
```

- **R1.** `packages` uses the selectors of CLI.md §2.2 (`game.items`, `game...`, `./game/items`,
  `path/to/file.canon`); none selects every package. An unknown selector is `ErrUnknownPackage`.
- **R2.** `Check` runs build phases 1 to 7 (EVALUATION.md §1, EVL-01) on the selected packages and
  their imports. Findings are reported for the selected packages only.
- **R3.** Findings in the result are errors and warnings of the snapshot, not a failure of the
  call: `err` is non-nil only for `ErrUnknownPackage`, `ErrUnknownLayer`, `ErrClosed`, `ctx.Err()`
  or `*InternalError`.
- **R3a.** `CheckWith` takes a per-call language, like `EvalRequest.Lang` (§11); `Check(ctx,
  packages...)` is `CheckWith` with `Packages` and no `Lang`, and `Lang` `""` means
  `Options.Lang`. Only the messages are re-rendered in that language, from the kept analysis: the
  finding set and the counts do not change (F7, DECISIONS 281). The result does not echo the
  language (DECISIONS 313).

### 5.2 Value

```go
func (p *Project) Value(ctx context.Context, path string) (*Value, error)
```

`Value` returns the final value at `path` (after layers), its type and where it comes from. It
serves `canon explain` and the studio's detail panels.

```go
type Value struct {
    Path     string        // canonical path (§6.5), package-qualified
    Kind     ValueKind
    Type     TypeInfo
    Text     string        // canonical Canon literal text (STD-06 text form for scalars)
    Origin   Origin
    Editable Editability
}
type TypeInfo struct {
    Expr string            // canonical type text, qualified: "[resource.farm.Level]"
    VM   json.RawMessage   // the VIEWMODEL.md type encoding
}
```

Methods, all cheap, never re-evaluating:

| Method | Returns |
|---|---|
| `Bool() (bool, bool)`, `Int() (int64, bool)`, `Float() (float64, bool)`, `Str() (string, bool)`, `Dur() (time.Duration, bool)` | the scalar, and whether the kind matches |
| `Member() (enum, name string, ok bool)` | enum member (Canon name) |
| `Case() (string, bool)` | current case of a variant |
| `Key() (string, bool)` | key of a ref, entry or keyed element, in its canonical text |
| `IsNone() bool` | the value is `none` |
| `Len() int` | elements of a list, keyed list, table or map; fields of a record, or of a variant's current case; 0 otherwise |
| `Children() []*Value` | fields in declaration order (absent optional fields included, as `none`), elements in order, map entries in insertion order |
| `Child(seg string) (*Value, error)` | one child by a path segment (`.f`, `[k]`, `[#n]`) |
| `JSON() []byte` | the wire form (WIRE.md §5), compact: the canonical bytes of WIRE.md §7.1–§7.3 without §7.4's layout (`nil` for a value with no wire form: a pair, a range) |

- **R4.** A `*Value` holds its snapshot: it stays valid and unchanged after later edits.
- **R5.** `Value` on a path that does not exist is `ErrNoPath`; a syntax error is `ErrBadPath`;
  an ambiguous unqualified root is `ErrAmbiguousPath` (§6.3); a path ending at an `input` field is
  `ErrInputField` (the field has no value at build time, EVALUATION.md §11.2), with `Detail` its
  environment variable.
- **R6.** A value that could not be computed (poisoned, EVALUATION.md) is `ErrNoValue`, which
  wraps the findings that explain why: the cause evaluation or verification recorded for its root
  (its own hard error or, through taint, the first poisoned value it read, in any package), else
  the checker's findings in the root's declaration and in its active layers' amendments, else those
  of the checker-broken declarations it depends on.

`Origin` records where a value comes from (EVL-07):

```go
type Origin struct {
    Kind    OriginKind // literal | json | csv | defines | text | default | spread | computed | layer
    Span               // the literal, the JSON value's first byte, the default expression, the
                       // building expression, or the amendment
    Pointer string     // RFC 6901 for json; "/<row>/<column>" for a csv cell (EVALUATION.md §13)
    Layer   string     // for layer
    Via     *Origin    // default: the literal or JSON object that omitted the field;
                       // spread: the origin of the copied value
    Stack   []Frame    // computed: the Canon call stack that built it, at most 16 frames
    Text       string  // canonical text (STDLIB STD-06) of the value this origin produced, before
                       // any later amendment of it or its descendants; "" when not known
    MoreFrames int     // frames cut from Stack (F13); 0 when none
    Replaced   *Origin // the origin of the value an amendment replaced with this one (EVALUATION.md
                       // §9.3), and so on back to the base value, newest first; nil if none
}
```

`Editable` says whether and how the value can be edited (§7):

```go
type Editability struct {
    Mode   EditMode   // canon | json | none
    Reason Reason     // why not, when Mode == none
    File   string     // file an edit would write
    Origin string     // for Reason == computed: canonical path of the nearest editable source, or ""
    Layer  string     // for Reason == layered
}
```

### 5.3 Refs

```go
func (p *Project) Refs(ctx context.Context, path string) (*RefsResult, error)

type RefsResult struct { Target string; Refs []Ref }   // a key token typed `ref` in a path (a layer's
                                                       // `ts[wolf]`) is a ref of that entry (DECISIONS 316)
type Ref struct {
    Kind    RefKind   // value | key | code | view | check | layer
    Package string
    Path    string    // for value and key: canonical path of the referring value or map entry
    Span
}
```

- **R7.** `path` must name a table entry, a keyed-list element or an enum member (P7a); anything
  else is `ErrBadOp`. `Refs` lists: `ref` values that hold its key (`value`), map keys of `ref` type
  (`key`), names in function bodies, defaults and constants (`code`), in views (`view`), in checks
  (`check`), and in amendments (`layer`). Every loaded package of the project is searched, not only
  the target's importers. For an enum member, the `value` refs are the values holding the member
  and the `key` refs the map keys equal to it.
  - A `ref` or member value is *stated* when its provenance span (EVALUATION.md §13) is a key or
    name token: an identifier, a selector name, a string or integer literal, a JSON string or key.
    Any other provenance (a function body, any other expression, a ref converted from an entry
    record) is *computed*: the value is still a `value` ref, it is not editable (so `Rename`
    refuses it, E12), and one found inside a `let` is reported at that let's name. The names that
    compute it are `code` refs.
  - Names in defaults, constants and spreads are `code` only (renaming the token fixes every
    user); names in translations are `view`, names in `test` declarations `code`. A string literal
    keys a ref only for a `String` key or a literal of a literal union (TYPES.md §4.1); a member
    name keys an enum-keyed ref.
  - A key lookup through a per-instance collection (a keyed-list or table field) is a `code` ref
    of an element only when its receiver resolves statically to that element's instance (a `let`
    root, literal indices and keys); a dynamic receiver is not a ref (the re-check, E18, catches
    what a rename breaks).
  - Value refs come from the base evaluation and the layered one, merged by span. A value whose
    static type could hold a ref to the target but that cannot be computed makes `Refs` fail,
    naming it, so a partial list never looks complete. A `let` whose type is the error type holds
    no ref value; its written names are still `code` refs.
- **R8.** Order: F2 order of the spans. `canon refs` prints them in this order (CLI.md §3.8).

### 5.4 ViewModel

```go
func (p *Project) ViewModel(ctx context.Context, pkg string) (*ViewModel, error)

type ViewModel struct { Package string; Revision Revision }
func (vm *ViewModel) JSON() []byte
func (vm *ViewModel) Decode(v any) error
```

- `pkg` is a package name, not a selector (R1): anything else is `ErrUnknownPackage`. `Decode` is
  strict: a member that matches a field of `v` only by letter case, or a `null` where `v` holds no
  `json.RawMessage` or interface, is an error; members `v` does not read are skipped (VIEWMODEL J6).
- **R9.** `JSON()` returns exactly the bytes `emit view` would write for `pkg` in this snapshot
  (VIEWMODEL.md), whether or not the package declares `emit view`. It is produced even when the
  package has errors (VM-07).
- **R10.** Typed Go structs for the view model are generated from `spec/viewmodel.schema.json`
  into package `github.com/fantasim/canonlang/api/vm` (IMPLEMENTATION-PLAN M3). `Decode` accepts
  them.

### 5.5 Packages and project facts

```go
func (p *Project) Root() string
func (p *Project) Info(ctx context.Context) (ProjectInfo, error)
func (p *Project) Packages(ctx context.Context) ([]PackageInfo, error)   // sorted by Name
type ProjectInfo struct {
    Name string
    Doc  string    // the project doc comment's text (GRAMMAR.md §2.2), source language
}                  // Info fails like Packages: an invalid project.canon is a *ProjectError
type PackageInfo struct {
    Name    string
    Dir     string    // display path
    Files   []string  // sources, translations and layers, byte order
    Imports []string  // sorted
    Layers  []string  // layer names that have a file in this package, sorted
}
```

---

## 6. Value paths

### 6.1 Grammar (API-02)

```ebnf
path     = [ package ":" ] name { segment } ;
package  = word { "." word } ;
segment  = "." name                   (* record or case field; table entry; pseudo-field *)
         | "[" key "]"                (* key; or index of a plain list *)
         | "[#" digits "]" ;          (* position in any ordered collection *)
key      = word | [ "-" ] digits | jsonString ;
name     = word ;
word     = letter { letter | digit | "_" } ;       (* SPEC §2.4; "_" alone is not a word *)
digits   = digit { digit } ;                          (* no "_" separators, no sign, no leading zeros except "0" *)
jsonString = (* a JSON string literal, RFC 8259 *) ;
```

No whitespace is allowed anywhere in a path. Reserved words are valid `word`s (LEX-08).

### 6.2 Meaning of each segment

| Container at that point | `.name` | `[k]` | `[#n]` |
|---|---|---|---|
| record, variant case | field `name` | — | — |
| variant | field of the **current** case (`ErrNoPath` if the case lacks it) | — | — |
| `[T]` (plain list) | — | `k` must be `digits`: the element at index `k` | same |
| `[T] keyed by f` | the element whose key is `name` (keys that are words) | the element whose key is `k` (**P1**) | the element at position `n` |
| `table T` | the entry `name` | the entry `k` (`k` is a word or a JSON string holding one) | the entry at position `n` |
| `{K: V}` | — | the entry whose key is `k` (**P2**) | the entry at position `n` (insertion order) |
| `T?` | as for `T` if the value is not `none`; `ErrNoPath` otherwise | | |

- **P1.** `[k]` on a keyed list is a **key**, never a position, whatever the key type:
  `farm.modelTypes[3]` is the model whose `typeId` is 3. `k` is read as a literal of the key
  field's type: `digits` (with optional `-`) for integer types; a word or JSON string for `String`;
  for an enum, a word is a member name and a JSON string is a wire value; for a `ref`, the target's
  key.
- **P2.** A map key `k` is read according to `K`: `String`: a word is that string, a JSON string
  is its value; integer types: `digits`; enum: a word is a Canon member name, a JSON string is a
  wire value (`styles[daily]`, `styles["daily"]`); `ref`: as P1 for the target collection; a
  literal union `T | "lit"`: a JSON string equal to a literal is the literal, anything else is
  read as `T` (TYP-09). A dependent map `{e in c: T(e)}` has `ref c` keys.
- **P3.** Pseudo-fields are readable and never editable: `.id` and `.retired` on table entries,
  `.kind` on a variant (its case, as a member of `<Variant>Kind`). A record field of the same name
  wins over the pseudo-field where TYPES.md allows such a field (RES-08).
- **P4.** `[#n]` counts from 0; negative positions are not allowed (`ErrBadPath`). An index or
  position past the end is `ErrNoPath`.
- **P5.** A segment the container does not support (for example `.f` on a list, `[k]` on a
  record) is `ErrBadPath` naming the segment.

### 6.3 The root

- **P6.** Without a `package:` prefix, `name` is looked up among the **public** top-level values
  and constants of every package of the project. Exactly one match is required; several is
  `ErrAmbiguousPath` listing the qualified candidates; none is `ErrNoPath`.
- **P7.** With a prefix, `name` may be any top-level `let` or `const` of that package, `local`
  included.
- **P7a.** A root may also name an enum (`Element`, `resource.vocab:Element`, looked up like P6 and
  P7 among enums). The path is then exactly `Enum.member` and names that member. Such a path is
  accepted by `Value`, `Refs` and `Retire` (`Unretire` refuses it like any path, E4); every other
  operation is `ErrBadOp`. A
  value and an enum with the same name in scope are `ErrAmbiguousPath` unless qualified.

### 6.4 Paths in edits of layers

When `Options.EditLayer` is set (§7.5), the edit path is translated into an `amend` path
(GRAMMAR.md). A path that has no `amend` equivalent is `ErrNotEditable` with reason `layer`.

### 6.5 Canonical form

Every path the API returns (`Value.Path`, `Finding.Path`, `Ref.Path`, keys of `EvalResult`) is
canonical, so it can be compared as a string:

- **P8.** Fields: `.name`. Table entries: `.key`, or `[n]` when the key is an integer. Keyed-list
  elements: `[key]`. Plain-list elements: `[n]`, a plain list holding copies of table entries
  included. Map entries: `[key]`. A keyed-list element whose key is not known (not computed yet,
  or its computation failed) is named by the list's own path, never by a position. Nothing uses
  `[#n]` in canonical form.
- **P9.** A key is written as a word when it is one and the key type is not an integer type;
  integers are written in decimal; enum keys are written as the Canon member name; every other key
  is written as a JSON string, escaped as WIRE.md escapes strings. For a literal-union key type
  (`T | "lit"`), a key that is one of the literals is always written as a JSON string (P2 reads a
  word as a `T`), so the canonical form resolves to itself. The key type is the one declared at
  the path's location, computed for the instance when it is dependent (TYPES.md §11.5), never the
  stored type of a value assigned from elsewhere, so a value has exactly one path.
- **P10.** `Value.Path` and `EvalResult.Path` carry the `package:` prefix; `Finding.Path` and
  `Ref.Path` do not (their package is a separate field).

```
teamboard:statuses.open.next[0]
resource.farm:farm.modelTypes[3].levels[1].productionItem
resource.adventurequest:adventureQuests.styles[daily].taskWeights
service.resourcestudio:config.paths.resourceRoot
pipeline:potions[II_POT_HEAL_L].cooldown
```

---

## 7. Where a value can be edited

Every edit is decided by the value's **source**: the place in a file whose text states it. The
studio edits values, never text (DECISIONS 12). This section says which values have an editable
source and where the edit goes.

### 7.1 Source trees

- **W1.** A top-level `let` or `const` has a **source tree** when its initializer, after removing
  parentheses, is:
  - a literal (scalar, list, map, record, typed record, variant case, table), whose items are
    themselves source trees or other expressions; or
  - `load(…)` or `load.dir(…)` of JSON files (possibly with `at:` and `partial:`), with no method
    call or operator applied to the result. The JSON values are the source tree.
- **W2.** The entries declared with `entry t.key { … }` (SPEC §4.3, and keyed lists per CLI-02)
  are children of `t`'s source tree, in entry order.
- **W3.** A path is **structural** when, walking it from its root, every segment moves from a
  source-tree node to a child node of the same tree, or to a field that is absent from its record
  literal or JSON object because it has a default (§7.3), or that is absent because a spread
  supplies it (§7.4).
- **W4.** A value whose path is structural is editable, with `Mode` `canon` for a `.canon` source
  and `json` for a JSON source.

### 7.2 Values that are not editable

`Editability.Reason` names the first rule that applies:

| Reason | When |
|---|---|
| `computed` | the path leaves the source tree: a function call, comprehension, operator, `if`, `match`, identifier or method on the way (`Origin` gives the canonical path of the value's own source when that source is structural, else `""`) |
| `layered` | an active layer amends this path or an ancestor, and `EditLayer` is not that layer (MOCKUP-GAPS 48) |
| `format` | the source is `load.csv`, `load.defines` or `load.text` (not editable in v0); a JSON source whose file extension is not `.json` (`load(…, format: json)` of `x.cfg`), or a value whose defining file has a path segment starting with `.`, `.canon` files included (`Detail` names the path): the journal could not recover such writes (§10.3); below a `load` `at:` path past a `*` (whose `*`-level member is the item structural ops act on), an op when that member holds more than the selected remainder, or when the remainder's index is above 0: it would touch data outside the selection or another load's view of the file (`Detail` names the outside datum as `<display>#<pointer>`) |
| `input` | an `input` field (it has no value at build time) |
| `key` | the key field of a keyed-list element, or a key of a dependent map (MOCKUP-GAPS 17); keys change only with `Rename`, and this reason never refuses a `Rename` that rewrites a keyed-list key (E11, DECISIONS 310) |
| `pseudo` | `.id`, `.retired`, `.kind` (P3) |
| `order` | `Insert` or `Move` in a collection whose order comes from file paths (`load.dir`, entry files) |
| `layer` | an operation that cannot be written as an amendment when `EditLayer` is set (§7.5) |
| `broken` | a `RenameName` whose packages hold a broken declaration (E32) |

- **W5.** An edit on a non-editable path fails with `*NotEditableError` (wraps `ErrNotEditable`)
  carrying the op index, the path, the reason and, for `computed`, `Origin`.
- **W6.** `@deprecated` fields are editable through the API (SPEC §5.12 makes them read-only in the
  studio only). Setting one in a `.canon` source produces `W3301` (TYPES.md §16).

### 7.3 Defaulted fields

- **W7.** Setting a field that is absent from its record literal (or JSON object) because it has a
  default **inserts** it, in declaration order: after the nearest present field declared before
  it, or first if there is none (API-02, FMT-02).
- **W8.** When the absent field is itself a record that only exists through its default
  (`let config: Config = {}` and `Set("config.server.port", Int(9000))`), the edit
  **materializes** it: it inserts `server: { port: 9000 }`, where the inserted literal is the
  current value of `server` with the change applied, printed without the fields that equal their
  own defaults. The result is `let config: Config = { server: { port: 9000 } }`. Materializing
  never changes any other value.

### 7.4 Spread

- **W9.** In `{ ...base, label: "Other" }`, a field present after the spread is edited in place. A
  field that comes from `base` is edited by inserting an override field into the spreading literal
  (API-02); `base` itself is never changed through the derived value. Editing `base` directly edits
  `base`. Under a spread, `Reset`, a `Set` to the default and a field a record-level `Set` leaves
  out write the default explicitly: removing the field would let the spread supply a value.

### 7.5 Layers (MOCKUP-GAPS 48)

- **W10.** Without `EditLayer`, edits go to base sources. A path whose final value is set by an
  active layer is `layered` (W5), so the studio shows it read-only with the layer's name. Paths the
  layers do not touch are edited normally, even while layers are active.
- **W11.** With `EditLayer: "x"`, `Set`, `Reset` and `AddEntry` write into the `x` layer file of
  the package that declares the value's root: `Set` adds or replaces the amendment line for that
  path (`amend <root> { <path>: <expr> }`), `Reset` removes it, `AddEntry` adds an amendment that
  creates the key (LAY-02). If the package has no `x` layer file, it is created as
  `<package dir>/<x>.layer.canon`. Every write of these ops, defaults materialized by W8 included,
  goes to the layer, never to the base. `Remove` of an entry the layer itself added removes that
  amendment; every other op (`SetCase` included) is `ErrNotEditable` with reason `layer`. A layer
  cannot add entries to a stable table (LCK-04), which the re-check reports as a finding.
- **W11a.** A layer may not hold a path and one of its prefixes (EVALUATION.md §9.2, `E1908`). So
  when the layer already amends an ancestor of the path, `Set` edits the value inside that
  amendment's right-hand side (a structural edit of that literal, §9); when it amends descendants of
  the path, `Set` replaces them all with the one new amendment line. An `AddEntry` into a table the
  layer already amends adds one line and keeps the others. Under an `EditLayer` that is not active
  in the session, an `AddEntry` is refused as `ErrNotEditable` with reason `layer`: no path reaches
  the new entry to undo it (DECISIONS 273).

---

## 8. Editing

### 8.1 Request and result

```go
func (p *Project) Edit(ctx context.Context, e Edit) (*EditResult, error)

type Edit struct {
    Base        Revision  // revision the caller last read (S5, S6)
    Ops         []Op      // applied in order, all or none
    AllowErrors bool      // write even if the result has error findings (drafts)
    DryRun      bool      // compute everything, write nothing
    Normalize   bool      // allow writing files that are not in canonical layout (§9.4)
    Evaluate    []string  // paths to evaluate after the edit (§11), returned in Eval
}

type EditResult struct {
    Applied  bool                    // true when files were written (false for DryRun or refusal)
    Revision Revision                // snapshot after the edit (the Base snapshot if nothing was written)
    Changes  []FileChange            // byte order of Path
    Findings []Finding               // findings of the affected packages after the edit (F2 order)
    Summary  Summary
    Dropped  []Dropped               // values removed by cascades (§8.5), in path order
    Undo     []Op                    // ops that restore the previous values (§8.7)
    Eval     map[string]*EvalResult  // one per Edit.Evaluate path
}

type FileChange struct {
    Path    string       // display path
    Kind    ChangeKind   // modified | created | deleted | renamed
    OldPath string       // for renamed
    Before  []byte       // DryRun only; nil when created
    After   []byte       // DryRun only; nil when deleted
}
type Dropped struct { Path string; Value json.RawMessage }   // wire form of the removed value
```

### 8.2 Values in operations (API-01)

Operation values are a closed set of Go types implementing `Lit`. They are checked against the
type expected at the path **before** anything is written (CLI.md §5.3 "values, not text").

| Constructor | Canon value | Accepted where the expected type is |
|---|---|---|
| `Bool(b)` | `true`/`false` | `Bool` |
| `Int(n)` | integer | any integer type; also `Float` (as an integer literal would be, TYP-10) |
| `Float(f)` | float | `Float`, `Float32` |
| `Str(s)` | string | `String`, an asset type, a literal union's literal |
| `Dur(d)` | duration | `Duration`; `d` MUST be a whole number of milliseconds |
| `Member(name)` | enum member by **Canon name** | an enum; a literal union whose base is an enum |
| `Key(k)` | a key | `ref T` (String-keyed target), a table key, a String map key |
| `IntKey(n)` | a key | `ref T` whose target is keyed by an integer field (RES-09) |
| `Case(name, fields)` | variant case | a variant; `fields` may be `nil` for a case without fields |
| `None` | `none` | any `T?` |
| `List(elems...)` | list | `[T]`, `[T] keyed by f` |
| `Obj{...}` | record fields by Canon name | a record, a case's fields, a table entry |
| `Map(KV{k, v}...)` | map, in the given order | `{K: V}` |
| `FromJSON(raw)` | the wire form | any type: decoded by WIRE.md's rules for that type and the destination field's `@json` forms (`unit`, `int`, `bits`, `none`) |
| `Source(text)` | a Canon literal | any type: parsed as a Canon expression that contains only literals (no names but contextual ones, SPEC §6.2), typed against the expected type |

- **V1.** A value that does not fit the expected type is `*ValueError` (wraps `ErrBadValue`) naming
  the op, the path, the expected type and what was given. Nothing is written. So is a value holding
  invalid UTF-8 (a `Str`, a key, a literal; GRAMMAR.md §1). Bound for a JSON source, a value with no
  wire form is a `ValueError` too: a `Duration` that is not a whole number of its field's
  `@json(unit:)`, an `@json(pairs:)` list past its slot bound or with an element missing a required
  field (WIRE.md §5.14); bound for a `.canon` source, the same value is left to the re-check
  (`E8102`, `E3302`). A dependent symbol the op's literal gives is resolved to what it names in its
  record's computed branch, computed exactly as the decoder computes it (WIRE.md §5.9, DEP-02;
  defaults from the record's parameters); a symbol it introduces into a `Never` branch, or that
  names nothing in a branch that can be computed, is a `ValueError`. A scalar literal given to a
  dependent type fits when a branch takes it (TYPES.md §11.4; DECISIONS 267). Under an `EditLayer`
  that is not active in the session, a layer `Set` is typed against the declared type at the path,
  since the session cannot see the layer's own drivers (W11, DECISIONS 273).
- **V2.** Only the static type is checked at this point. Refinements (`Int(1..=100)`), references
  (`E3501`), keys and checks are verified by the re-check (§8.6), like any other value. For
  `FromJSON`, a wire value whose shape or type does not fit is a `ValueError` (and invalid JSON,
  an object repeating a key `E7104` included); a missing required field (`E3302`), a value out of
  range (`E3201`), a `Float32` overflow (`E3202`), a duplicate key (`E3102`) and map keys that
  encode alike (`E3317`) are findings of the re-check, their values kept. Three decode failures
  cannot be kept and stay a `ValueError`: a dependent field whose driver was left unset by a kept
  failure, a CSV table missing its `$id` column, and a `Duration` out of range.
- **V3.** `Obj` omits fields to leave them at their defaults. A required field that is missing is a
  finding of the re-check (`E3302`), not a `ValueError`; so is one a `FromJSON` object omits.
- **V4.** Integer and ref key constructors never coerce: `Str("3")` is not accepted for an `Int`,
  `Key("warning")` is not accepted for an enum (use `Member`).

### 8.3 Operations

| Constructor | Target | Effect |
|---|---|---|
| `Set(path, v)` | any editable value | replace the value (with W7/W8 for absent fields) |
| `Reset(path)` | a field that has a default | remove the field from its literal or JSON object, so it takes its default |
| `Add(path, v)` | `[T]`, `[T] keyed by f` | append `v`; for a keyed list the key comes from `v` |
| `Insert(path, i, v)` | `[T]`, `[T] keyed by f` | insert `v` at position `i` (`0 ≤ i ≤ len`) |
| `AddEntry(path, key, v)` | `table T`, `{K: V}` | add a new entry `key` with value `v` |
| `Remove(path)` | a list element, a map entry, an entry of a non-stable table | remove it |
| `Move(path, i)` | a list element, a keyed-list element, a map entry, a table entry | move it to position `i` among its siblings (position after removal) |
| `Rename(path, newKey)` | an entry of a non-stable table, a keyed-list element, a map entry | change the key and every reference to it (§8.4) |
| `Retire(path)` | an entry of a stable table, a member of an `@codes` enum | mark it `retired` (SPEC §12) |
| `Unretire(path)` | a retired entry or member | always refused: `ErrStableKey` (E4) |
| `SetCase(path, case, fields)` | a variant | change the case, keeping compatible fields (§8.5) |
| `RenameName(name, newName)` | a Canon name (§8.9) | rename its declaration and every name naming it |

- **E1.** Ops apply in order to the state left by the previous ops; paths in later ops see earlier
  changes (a key added by op 0 can be addressed by op 1). Each op is resolved against, and applied
  in memory to, the re-analysed state the previous ops left, so a path under an intermediate value
  that failed to compute is `ErrNoValue`, even if the final state would be valid, except for an
  edit that only removes or rewrites source (E19). A value under a dependent type is typed against
  the branch its driver has in that state (DECISIONS 257).
- **E2.** An op on a container kind it does not support (for example `Add` on a table, `AddEntry`
  on a list, `Reset` on a required field) is `ErrBadOp`.
- **E3.** `Add`, `Insert` and `AddEntry` with a key that already exists are `ErrKeyExists`.
  `Set` on a map key or table entry that does not exist is `ErrNoPath` (use `AddEntry`).
- **E4.** `Remove` on an entry of a stable table whose id the lock holds or the request locks, and
  `Rename` of such an entry, and either op on any stable entry under an `EditLayer` (LOCK.md §6.1),
  are
  `ErrStableKey` (SPEC §12: retire it instead). `Remove` of an `@codes` enum member is not an op.
  `Unretire` is always `ErrStableKey`: retirement is one-way, and bringing an id back is a reviewed
  hand edit of `canon.lock` (LOCK.md §4.6), which the build would otherwise report as `E6002`.
- **E5.** `Set` of the whole value of an entry, element or map entry keeps its key: a `v` whose
  key field differs from the current key is `ErrNotEditable` with reason `key`; one that leaves the
  key field out keeps the current key.
- **E6.** `Set(path, v)` where `v` equals the field's default removes the field from its literal or
  JSON object (API-06), exactly as `Reset(path)`. Equality is the value equality of TYPES.md §7.5;
  for a default that depends on earlier fields (TYP-15), the default is computed from the edited
  record. The removed field then tracks its default: a later op of the same edit that changes
  what the default depends on changes the field's value too (E1, DECISIONS 256).
- **E7.** `Set(path, None)`:
  - if the field's default is `none`, it removes the field (E6);
  - otherwise it writes an explicit `none` (MOCKUP-GAPS 11: default, value and explicit `none`
    are three states): `none` in a `.canon` source, and in a JSON source the field's
    `@json(none: X)` value if it has one, else `null` (WIRE.md);
  - in a JSON source, a field with `@json(none: X)` whose default is `none` is written as `X`
    rather than removed (API-06).
- **E8.** `Reset` of a field that is already absent does nothing and is not an error.
- **E9.** `Move` requires the element and its new position to be in one literal or one JSON
  array or object; otherwise `ErrNotEditable` with reason `order`.
- **E10.** `Retire` of an entry that comes from a JSON source writes `"$retired": true` as the
  first key of the entry's object (the file's top-level object for a `load.dir` table), which
  WIRE.md §5.7 accepts in source wire.

### 8.4 Rename

`Rename(path, newKey)` changes a key and everything that names it, in one atomic edit
(MOCKUP-GAPS 23).

- **E11.** The target is the entry or element at `path`; `newKey` has the key's type. What changes:
  - the key itself: the entry name in a table literal, the `entry t.key` line, the key field of a
    keyed-list element, or the map key;
  - every reference that `Refs` returns with kind `value` or `key`: the `ref` value or map key is
    set to the new key, with the editing rules of this document;
  - every reference of kind `code`, `view`, `check` or `layer`: the identifier token is replaced;
  - the entry's file, when the value (a table or a keyed list) has `@files` and the file's path
    equals the template instantiated for the old entry: the file is renamed to the template
    instantiated for the new entry (§10.2); a file placed by N4's default path, without `@files`,
    keeps its name (DECISIONS 256); the file of a `load.dir` table's entry, whose key is the file's
    stem (WIRE.md §6.5), is renamed to the new key with the same extension, in its directory;
  - a reference that is the key field of a keyed-list element (DECISIONS 310): that key is
    rewritten, and the element's path moves (`z.spawns[wolf]` → `z.spawns[timber]`) with every ref
    and path into it; its `@files` file follows the template rule above; a keyed list keyed by a
    ref to that element cascades in turn.
- **E12.** If any reference of kind `value` or `key` is not editable (for example a ref computed by
  a function), the whole rename fails with `*NotEditableError` for that reference; `Detail` lists
  every such reference. The key field of a keyed-list element is not refused: E11 rewrites it
  (DECISIONS 310).
- **E13.** `Rename` of a key in a dependent map is `ErrNotEditable` with reason `key`
  (MOCKUP-GAPS 17). `Rename` in a stable table is `ErrStableKey` (E4).

### 8.5 Cascades: case changes and dependent fields

Some edits make other values invalid in a predictable way. The compiler applies the consequence
inside the same edit and reports it, so every client behaves the same.

- **E14.** `SetCase(path, case, fields)` builds the new case value from: every field of the old
  case whose name exists in the new case with the same type after removing refinements (VIEW-07's
  comparison) and whose value satisfies the new field's type whole; then `fields`, which override.
  Every old field not kept is reported in `Dropped` (MOCKUP-GAPS 13), an old field written at its
  default included. `SetCase` to the current case only applies `fields`. "Satisfies whole" is
  judged when the `SetCase` runs, by re-analysis of the settled state after it (so op grouping
  never changes it, E1): a kept field with any fit error (type, every refinement kind, `where`,
  asset) at or inside its place is dropped, pre-existing or not, also when the new case value is
  unreadable (a given field broke it, a required field is missing), so even an `AllowErrors`
  `SetCase` never writes a kept value that breaks the new type. Kept fields are judged together;
  only a field whose type differs by refinements (compared structurally) needs judging; a variant
  left at its default is judged too, located from its nearest ancestor written in the source.
- **E15.** After all ops are applied, for every record touched by an op, every field whose type is
  a dependent type (SPEC §5.11) computed from a field that changed is re-typed. If its value no
  longer matches the new type and the field is optional, it is set to `none` (E7 rules) and
  reported in `Dropped` (MOCKUP-GAPS 16). A required field that no longer matches is left as is,
  and the re-check reports `E3802`. A field changed only when its value differs (a `Set` to an
  equal value changes nothing). A value no longer matches when the re-typing, refinements
  included, gives a type, refinement (range, length, pattern), `where`, asset, shape (`E3301`,
  `E3302`, `E3315`) or decoding (`E7110`, `E7111`, `E7112`) finding at its exact path; a
  pre-existing dangling ref (`E3501`) does not drop it. A dependent-keyed map whose keys no longer
  fit the new branch is dropped to its default. A value **held** at that place, a decoded
  dependent symbol "as the file wrote it" (DECISIONS 175), before the edit or given by the op as
  `FromJSON`, is not dropped: it stays with its `E3802` as before, so `Undo` restores it (E22),
  until a later op of the edit changes a field it depends on. Only the symbols the op's literal
  gives are resolved by V1; kept and carried values are left to this rule. An element a `Rename`
  moves (E11) is addressed here by its new path; the key a `Rename` cascade rewrites still names
  the same entry, so it is not a changed driver for dependent fields (DECISIONS 310).
- **E16.** Cascades never apply to anything but the records touched by the ops.

### 8.6 Checking and refusal

- **E17.** The **affected packages** of an edit are: the packages whose static read set (S3, S5)
  holds a file the edit writes or removes, or a directory listing the edit changes; each package
  declaring an `asset` type whose root holds, by real path, a name the edit creates or removes (its
  asset checks list that directory, TYPES.md §13.4); and every package importing one of these,
  directly or not. Static read sets are known from the parsed sources, never by evaluation, so a
  package whose `load` names a written file is affected even if that `load` is never evaluated.
  Files are compared by real path, resolved through the snapshot's links as WIRE.md §6.5 does, so a
  package reading a written file through a link or another root is affected. A directory the edit
  creates or removes changes the listing of each ancestor up to the first that existed. The
  staleness check (S5) covers their read sets (DECISIONS 252, 330).
- **E17a.** An edit analyses (phases 2–7) only these packages (DECISIONS 330):
  - before its first op, the **scope** of its ops: the package declaring each op's path root (§6.3)
    or `RenameName` target (E27), found from the parsed declarations, and, for a `Rename` or
    `RenameName`, every package importing one of those, directly or not, where its references and
    occurrences lie (E11, E33). A root that names no package adds none; its op fails in E21's
    order. Each op is resolved and applied against the scope's analysis (E1);
  - after the ops, the affected packages (E17, E18);
  - for both, every package they import, directly or not, and `project.studio`'s package with its
    imports when one of them declares a view, a `@menu` or an `emit view` (DECISIONS 227).

  An active layer (`Options.Layers`) is judged against the layer headers of every scanned package
  (O4): a layer amends only its own package (`E1909`), so a package declaring it outside these sets
  changes nothing they compute. Every other package is only scanned and parsed, for the import
  graph, the path roots and the static read sets: it is neither checked nor evaluated, its findings
  are not reported, and a failure in it does not fail the edit. An import these packages force
  lazily is evaluated as `canon check <pkg>` evaluates it (EVALUATION.md §2.1 item 2). When these
  packages are every package, the edit analyses the whole project, as before DECISIONS 330.
- **E18.** After applying the ops in memory, the affected packages are re-checked (phases 1–7)
  with what E17a loads for them. `Findings` holds all their findings and the project's own (package `""`), and only those: the
  findings of a package the edit does not affect, errors included, are not reported and never
  refuse the edit (E19). `canon check` stays the verdict of the whole project (DECISIONS 330).
- **E19.** If any finding is an error and `AllowErrors` is false, nothing is written, `Applied` is
  false, and `Edit` returns the result together with a `*RejectedError` (wraps `ErrRejected`).
  With `AllowErrors`, the files are written and the result has `Applied: true` and the error
  findings (drafts in progress, MOCKUP-GAPS 47). A `DryRun` applies this rule too. Clients send
  new values complete, with their required fields (VIEWMODEL.md D3); `AllowErrors` is for what
  cannot be known in advance. An edit that only removes or rewrites source (`Remove`, a `Set` of a
  field of the broken entry, `Undo`) works while the value is poisoned (R6), so a project broken
  by a missing required field (`E3302`) stays repairable through the API (DECISIONS 309). Each op
  is judged on its own, in any request. On a root table with no value, `Remove`, `Retire`,
  `AddEntry`, `Move`, and `Set`/`Reset` of an entry or its field are written from source; any
  other root with no value takes a `Set` of the whole root. Such an op is refused with the root's
  error when it would need a value: a field that drives a dependent field (E15), an entry with a
  spread (W9), or an inverse whose old text is not literal-only (§8.2). A field set to its default
  is written explicitly there (no E6/E7 omission), and E4 is judged before anything else.
- **E20.** An edit that adds an id to a stable table, or a new value of a `@stable` field, appends
  the new lock lines to the package's `canon.lock` inside the same edit (LCK-04), and `Retire` adds
  `retired` to its lock line, according to LOCK.md §5. The lines come from the edit's plan, the ids
  its ops add or retire, with the facts of the post-edit analysis; a `Set` or `Reset` on a root
  stable-table entry the lock does not hold yet records that entry's id (on a held one, a changed
  `@stable` value is `E6002`, LOCK.md §5). An edit that adds or retires no id writes no lock line
  and never creates or touches a `canon.lock`. An id's lines are written only whole, and only when
  none of its facts conflicts with the lock as read (LOCK.md §4.2) or duplicates another fact the
  post-edit sources would lock; otherwise the id is skipped and its finding (`E6002`, `E3102`)
  stays. Ids the edit adds that conflict among themselves are all skipped. The uniqueness test
  applies only to facts the lock does not hold yet: a `Retire` of a held id records `retired` even
  when an unlocked holder duplicates its value. Under `AllowErrors`, an id's lines are written
  whenever its own entry evaluates (LOCK.md §6.2).
- **E21.** Order of checks before writing: for each op in turn, path syntax, path resolution,
  editability, op/container kind, value types (V1), then its application in memory (E1); then the
  cascades (§8.5); then, over the files the ops write (known only now), overlays (S12), staleness
  (S5) and the canonical-layout check (§9.4); then the re-check (E18, E19); then the write (§10).
  The first failing step decides the error. For `RenameName`: request (E31), name (E27), kind
  (E28, E29), new name (E30), base (E32), application (E33, E34), overlays, staleness and layout,
  re-check, preservation (E35), write.

### 8.7 Undo

- **E22.** `Undo` is a list of ops that, applied with `Base` set to the result's `Revision`,
  restores every value the edit changed (including cascades), whatever E23's order would give
  (DECISIONS 257, 273). It restores values, not text: comments of removed items and a deleted entry
  file's doc comment are not restored, a recreated entry file may lie at any path its `@files`
  template gives for its key, whatever its templated fields, and a defaulted parent the Undo empties
  may remain written as `{}`. For every edit with an `EditLayer` (W11), active or not, the Undo
  restores that layer's own lines, each amendment line as it was or a `Reset` where there was none,
  never merged values, and is always verified, in the layer's own view (equality of its lines, whose
  order counts only for lines on overlapping paths) and in the session's layers; a request whose
  Undo would have to write back a computed line (one whose text cannot be a `Source`, a spread
  included) is refused before anything is written, with `*NotEditableError` reason `computed`, as W5
  gives for a `Reset` of that line. Off edit layers, the Undo is verified when the request has more
  than one op and touches a dependent field or its driver or fires a cascade (§8.5); a single op
  never pays for it (NFR-01). An entry's file path is part of the before state: a `Rename` inverse
  runs after the region restores of the item it renames, and verification compares the before path
  of entries that still exist or are renamed back; an entry the request removes and re-adds counts
  as recreated (N1–N4; DECISIONS 273). A value the before state held in error (a dependent mismatch
  that is not held, `E3802`, E15) cannot be written by any op: the Undo leaves it as the cascade
  drops it, and verification excludes it. When the edit's own result leaves a compared root
  uncomputable (only with `AllowErrors`), the plain Undo is returned unverified. When no verified
  Undo is found within the repair rounds, `Edit` fails with an `*InternalError` (`ErrInternal`) and
  writes nothing (DECISIONS 273).
- **E23.** Inverses: `Set` → `Set(old)`, or `Reset` if the field was absent (for an absent
  required field, only possible on a broken entry, `Set` of the enclosing entry to its old text,
  DECISIONS 257, 309); `Reset` → `Set(old)`;
  `Add`/`Insert`/`AddEntry` → `Remove`, except an `AddEntry` into a stable table → `Retire` of the
  new key (its id is in `canon.lock` and is never removed, E4; DECISIONS 277); the inverses of
  the request's other ops on paths inside that entry are left out, and verification (E22) ignores
  the retired entry's values: it did not exist before the edit; an inverse that writes a whole stable table (a `Set`
  back, or a region restore of the table) keeps every entry the request added to it, with the value
  the edit left, so its `Retire` finds it and no locked id is dropped (LOCK.md §4.1); an `AddEntry` whose entry the edit's
  result no longer holds (a later op of the request dropped it) was never locked (E20): it takes no
  `Retire`, and every write back of that table leaves the entry out; in general, an Undo never undoes a lock
  fact: every id and `@stable` value the edit locked (E20) keeps, in the Undo's target, the form the
  edit's result holds (an id the before state lacked is retired), every write back of a stable
  table or of a `@stable` field takes those values from the edit's result, and when the edit
  wrote lock lines the Undo's verification (E22) checks the lock too; a before value whose restore
  would contradict a lock fact the edit made (a `@stable` value now locked to another id) keeps the
  result's form, and verification excludes it; an id the edit tried to lock but whose lines E20
  skipped (an `AllowErrors` conflict, or an entry that does not evaluate) was never locked, and
  its inverse is a `Remove` when the result still holds the entry, none when it does not; a before entry the edit removed or renamed
  whose restore would contradict a lock fact stays as the result holds it, and verification excludes
  it; an `AddEntry` that re-creates a pending entry locks it, as the next build would (LOCK.md §4.4); `Remove` → `Insert(parent, oldPosition, old)` or
  `AddEntry(parent, key, old)` followed by a `Move` to the old position (where file paths fix the
  order, reason `order`, an `Add` or `AddEntry` alone, placed by N1–N4); `Move` → `Move` back;
  `Rename` → `Rename` back, a cascade through keyed-list keys included (E11, DECISIONS 310);
  `RenameName` → `RenameName(<canonical new name>, <old name>)` (E37); `SetCase` → `Set(old whole variant value)`. `Retire` has no inverse
  (E4): `Undo` restores every other change of the edit, and the studio warns before retiring. `Undo`
  lists the inverses in reverse order of the ops, then every cascade's inverse. Where E22 requires
  it, the Undo is verified by a dry apply against the after state; where that does not restore the
  before state, the smallest enclosing item is restored whole instead (DECISIONS 257). Old values
  are carried as `Source` literals, except a JSON-sourced value holding a decoded dependent symbol,
  carried as `FromJSON` of its wire form, encoded with the scope it is decoded in (the field's unit,
  `int`, `bits`, `none`), so it comes back as the file wrote it. Further inverses:
  - a `Set` of a field a spread supplied → `Set(old)`, not `Reset` (W9); under `EditLayer`, a `Set`
    on a path the layer had no line for → `Reset` (it removes the amendment), an `AddEntry` or
    `Remove` on a map inside an amendment → a `Set` of the whole map, and a `Remove` of an entry the
    layer added → an `AddEntry` that puts its line back at its place in the `amend` block, with no
    `Move` (W5 `layer`; DECISIONS 273);
  - `Add`, `Insert` or `AddEntry` into a collection that existed only through its default → a
    `Reset` of that collection;
  - any op on an `@json(pairs:)` list, and an element `Set` in an `@json(bits)` list →
    `Set(old list)`, or `Reset` when the list was defaulted and unwritten;
  - a dependent field E15 re-typed is restored after the field that drives it, typed at that
    intermediate state; a held value (E15) a re-type leaves in place is restated from the base.

### 8.8 JSON form of an edit

The studio's web client sends edits as JSON. `Edit` has this form through its struct tags
(`base`, `ops`, then `allowErrors`, `dryRun`, `normalize` and `evaluate`, each omitted when
false or empty), and `Op` implements `json.Marshaler` and `json.Unmarshaler` (E24–E26); `Edit`
implements `json.Unmarshaler` too, and JSON of either that does not decode is `ErrBadOp` (§15):

```json
{
  "base": "r1:5f0c…",
  "allowErrors": false,
  "ops": [
    {"op": "set", "path": "farm.modelTypes[3].maxLevel", "value": 10},
    {"op": "add", "path": "farm.modelTypes[3].levels", "value": {"level": 10, "modelName": "obj_UC017", "productionItemDefine": "II_GEN_MAT_MOONSTONE", "productionPerHour": 5, "maxStorage": 50}},
    {"op": "addEntry", "path": "statuses", "key": "blocked", "source": "{ tone: danger, label: \"Blocked\", terminal: false, next: [open] }"},
    {"op": "setCase", "path": "eventConfig.events[moonstone_rain].kind", "case": "spawn_item", "source": "{ spawnRegion: { left: 0, top: 0, right: 10, bottom: 10 } }"},
    {"op": "move", "path": "farm.modelTypes[3].levels[2]", "index": 0},
    {"op": "rename", "path": "statuses.blocked", "key": "on_hold"},
    {"op": "reset", "path": "config.server.port"},
    {"op": "set", "path": "config.paths.iconDir", "value": null}
  ]
}
```

- **E24.** `op` is one of `set reset add insert addEntry remove move rename retire unretire
  setCase renameName`; for `renameName`, `path` is the name and `name` the new name
  (`{"op":"renameName","path":"pipeline:Potion.heal","name":"hp"}`). A value is given either as `value` (wire form, decoded as `FromJSON`) or as `source`
  (Canon literal text, decoded as `Source`), never both. `"value": null` is `None`.
- **E25.** `key` is a JSON string or integer; it is read as a path key (P1, P2), so for an enum
  key a string is first matched as a Canon member name, then as a wire value. A key of a dependent
  key type is read against its computed branch (a bare name stays symbolic, TYPES.md §11.4,
  §11.5); on a `Never` or uncomputable branch it is kept as data.
- **E26.** Marshalling writes `source` for every `Lit` except `FromJSON`, which is written as
  `value`. `Source` text produced by the API is canonical (FORMATTER.md), single-line.

### 8.9 Renaming a Canon name

```go
func RenameName(name, newName string) Op
```

- **E27.** `name` is `[package ":"] word {"." word}`, or `file ":" line ":" col` (a display path,
  §1.3, and a byte column inside an identifier the checker resolved, declaring or using; it names
  that identifier's declaration). The first word names a top-level type, function, `let` or
  `const`: unprefixed, among the public top-level declarations of every package (several is
  `ErrAmbiguousPath` listing the qualified candidates, none `ErrNoPath`); prefixed, any top-level
  declaration of that package. Then, on a record, a field or method; on a variant, a case, then a
  field or method of that case; on a function or method, a parameter or local, `ErrAmbiguousPath`
  listing positions when the function declares that name more than once. Aliases are followed.
- **E28.** Renamable: fields of records and cases, methods, types (records, variants, enums,
  aliases, type functions), functions, `let`s, `const`s, parameters and block locals. A name
  reaching an entry key, an enum member or a variant case names data: `ErrStableKey` for an entry
  of a stable table or a `@codes` member (E4), else `ErrBadOp`, whose detail names `Rename` (§8.4)
  for an entry key, a keyed-list key or a map key, and says that an enum member or a variant case
  cannot be renamed for any other (DECISIONS 277).
  Any other declaration is `ErrBadOp`.
- **E29.** A stable table's `let`, a `@codes` enum and a `@stable` field are named by `canon.lock`
  lines: renaming one is `ErrStableKey` (LOCK.md §4.6). `RenameName` never writes `canon.lock`.
- **E30.** `newName` (in JSON, `name`, an empty string included) is a `word`, not `_`, not a reserved word that cannot name a declaration
  (GRAMMAR.md §4); else `*ValueError` (`ErrBadValue`), `Expected` `a name`. The old name again
  changes nothing.
- **E31.** A `RenameName` op is alone in its request and `AllowErrors` is false (`ErrBadOp`
  otherwise); with an `EditLayer` it is `*NotEditableError` reason `layer`.
- **E32.** If the target's package, a package importing it, or (for a parameter or local) its
  function holds a broken declaration, view or translation entry, the op is `*NotEditableError`
  reason `broken`, `Detail` listing them as `<file>:<line>`.
- **E33.** Every occurrence the checker records for the target is replaced in place (E11, M2–M6):
  the declaration; uses in value and type position, qualified forms and `import` lists; selector
  names, literal field names (in `let` values and entry files), named arguments, patterns,
  `keyed by`, `ref` and `entry` lines; view items; `amend` targets and segments in every layer
  file, active or not; translation key segments, which gain or lose their kind word when the
  segment becomes or stops being reserved (I18N.md K4); `emit … values:` names; `{f}` variables of
  `@files` templates. Comments, doc comments, string texts, file paths (an entry file stays in its
  directory, as DECISIONS 256), data files, outputs and `canon.lock` are not changed. A field
  declaration whose text changes is an item (§9.1), and so is any top-level declaration holding a
  renamed token.
- **E34.** A renamed field gets its old wire name (WIRE.md §5.5.2, `@json(case:)` applied) as
  `@json`'s positional argument when it has no positional wire name, `path:`, `pairs:` or `inline`,
  and its record or case is reachable (through fields, case fields, list, map and optional
  elements and aliases, never `ref`) from the expected type of a `load`, `load.dir` or `load.csv`,
  or from the declared type of a value an `emit json`, or a `go`/`cpp`/`ts` emit in mode
  `embedded`, `data` or `types`, writes. The argument is merged first into an existing `@json(…)`,
  else `@json("…")` is appended. Conversely a positional wire name equal to the new name's default
  wire name is removed, with the annotation if nothing is left in it.
- **E35.** The affected packages are re-checked (E18); an error refuses the op (E19), collisions
  included (`E2106`, `E2107`, `E2104`, `E2105`, `E2005`, `E2101`, `E1705`). Then every identifier of
  those packages must name the same declaration as before (the target's occurrences the renamed
  one; a built-in compared by name), else `*PathError` wrapping `ErrNameClash`, `Detail` naming the
  first captured identifier as `<file>:<line>:<col>` and what it would now name. An identifier
  that named nothing before and names something after is a capture too. An occurrence
  the index marks ambiguous (a `{f.g}` template variable or an amend segment that several cases' or
  branches' fields named `g` could mean) is `ErrNameClash` too, `Detail` naming it: renaming one case's field
  would leave the others' text meaning something else.
- **E36.** Generated identifiers, emitted `$fns` and `$`-function keys and file names follow at
  the next build; the op writes none of them (DECISIONS 275).
- **E37.** `Undo` is the single op `RenameName(<canonical new name>, <old name>)`; with E34's
  converse it restores every byte unless M5 re-printed an enclosing item, or the field carried a
  positional wire name equal to its default wire name, which the reverse rename drops (E34; the
  wire is unchanged) (E22).

---

## 9. Minimal writes

An edit changes the bytes that express the changed values, and nothing else (API-03, DECISIONS 12
and 18). Files are handled through their concrete syntax trees, which keep comments and blank
lines (IMPLEMENTATION-PLAN §4.1).

### 9.1 Items

An **item** is the unit of re-printing:

- in `.canon`: a field of a record literal (`name: value`), an element of a list literal, an entry
  of a map literal (`key: value`), an entry of a table literal (`key { … }`), an `entry`
  declaration, a top-level `let` or `const` declaration, an amendment line;
- in JSON: an object member (`"key": value`) or an array element.

An item owns its attached comments as FORMATTER.md attaches them: leading comment lines and doc
comments, and a trailing comment on its last line.

### 9.2 The algorithm

- **M1.** For `Set`, the old and new values are compared structurally. Records and cases of the
  same case: field by field. Maps, tables and keyed lists: key by key. Plain lists of equal length:
  position by position. Anything else, or a different case, is a replacement of the whole item.
  The result is the smallest set of items whose value changed.
- **M2.** Each changed item is re-printed alone, in the canonical layout at its nesting depth, and
  its text replaces the old item's text. Its attached comments are kept. Comments **inside** a
  re-printed item are kept for every sub-item M1 did not mark as changed; they are lost only with
  the sub-item they belong to.
- **M3.** A new item (W7, W8, `Add`, `Insert`, `AddEntry`) is printed canonically and inserted on
  its own line at the place W7 or the operation defines. In a literal written on one line, it is
  inserted on that line if the literal still fits the formatter's width and FORMATTER.md keeps it
  single-line; otherwise the enclosing item is re-printed (M5).
- **M4.** A removed item is deleted together with its attached comments and its line; blank lines
  around it are then collapsed as FORMATTER.md requires. Removing from a broken `( )`/`[ ]` list may
  re-print the whole list (FORMATTER.md §6.2 lays it out by width, so a removal can re-join it).
  Removing a JSON object's last member or an array's last element drops the comma on the kept item
  before it, which M6 counts as re-printing that item (DECISIONS 255).
- **M5.** After the splices, the file MUST be a fixed point of the formatter. If it is not (a line
  grew past the width, a single-line literal must now break), the smallest enclosing item that
  makes it a fixed point is re-printed instead, going up to the top-level declaration if needed.
- **M6.** Invariant, checked by the edit golden tests: `format(after) == after`, and every byte of
  `after` outside the re-printed and inserted items equals the corresponding byte of `before`.
  Because the formatter never aligns columns (DECISIONS 18), a one-value `Set` of a scalar that
  re-prints only its item changes exactly one line. M5 is the more specific rule: such a `Set` may
  break its line only when the single-line form (the line before, with the new value in place)
  exceeds the formatter's width, and then the enclosing item is re-printed. This one-line clause
  covers a `Set` of a scalar or `none` whose replaced item is written on one line; a `Set` replacing
  an item that spans several lines is held to the region rules only. A comma DECISIONS 211 or 216
  requires, or JSON's grammar forces, added or removed on a kept neighbour, or a byte the M5 settle
  changes there, counts as re-printing that neighbour (Insert, Remove, Move and a `Set` to the
  default alike, which is then not a one-line case; DECISIONS 255); a DECISIONS 216 comma line with
  a trailing comment may be rewritten whole by the settle.

### 9.3 Printing values

- **M7.** Values are printed with contextual names (SPEC §6.2): bare enum members, case names and
  keys. Durations use their canonical text (LEX-04). Fields equal to their defaults are omitted in
  newly printed records (E6 applied recursively to new values); fields of an existing literal are
  kept unless the op sets them to their default. Strings are printed with `{` and `}` escaped as
  `\{` and `\}` (GRAMMAR.md §2.6), so a string holding a brace is never read as interpolation.
- **M8.** JSON sources are written with wire names and the wire form of WIRE.md. A new key is placed
  after the previous declared field's key in declaration order (FMT-02); unknown keys kept by
  `partial` are left in place. `@json(path: …)` fields create the intermediate objects LOD-09
  describes. A list's or map's items take their field's unit and `int` form (WIRE.md §4.1). A
  dependent symbol is written as the wire form of what it names in its record's computed branch,
  at record level as at field level (WIRE.md §5.9); a held value on a `Never` branch (E15) is
  written back as its original JSON token (DECISIONS 175). An `@json(bits)` or `@json(pairs:)`
  list is written whole at its parent record, rewriting only the slot members or scalars whose
  value changed (M1 compares them by value, never by bytes).

### 9.4 Files that are not in canonical layout

- **M9.** Before writing a file, the edit checks that its current content is a fixed point of the
  formatter (`canon fmt` for `.canon`; the canonical source layout of FMT-02 for JSON, its numbers
  in their typed-canonical text, FORMATTER.md §14.1 and DECISIONS 165, a token two loads of the same
  real file read as different base types or canonical texts left as written, DECISIONS 259), judged
  on the raw bytes (a CR or a tab indentation is not canonical, DECISIONS 258). A `.canon` file is
  judged in its role: one other than `project.canon` opening with a `project` declaration holds
  `E1011` and is not a fixed point (DECISIONS 258). This rule governs FORMATTER.md
  §13. If not, the edit fails with `*NotCanonicalError` (wraps `ErrNotCanonical`) listing the files,
  unless `Edit.Normalize` is true, in which case each such file is first normalized entirely, as
  part of the same edit. Migration normalizes JSON sources once with `canon fmt --json-sources`
  (DECISIONS 12), after which this never happens.

---

## 10. Writing files

### 10.1 New entry files (`@files`)

- **N1.** `AddEntry` on a table, or `Add`/`Insert` on a keyed list, creates a new **file** when the
  value's `let` has a `@files` annotation, or when at least one existing entry is declared with
  `entry` in a file other than the `let`'s (or, for `load.dir`, always). Otherwise the entry is
  added to the literal, and `entry` declarations in the `let`'s own file do not order it.
- **N2.** The template of `@files("items/{itemKind1}/{id}.canon")` is expanded with the new
  entry's value: `{id}` is the key; `{f}` and `{f.g}` are fields (nested through records and the
  current case of variants, and through a dependent field's current branch). A value is written as: enum → wire value; ref → key; variant → case
  wire name; integer → decimal; `Bool` → `true`/`false`; `String` → itself. A `none` value, a
  string that is empty or contains `/`, `\`, a control character, or starts with `.` (it would name
  a hidden path, which the journal refuses, §10.3), is `ErrBadValue`.
- **N3.** The expanded path is relative to the directory of the package that declares the value.
  Missing directories are created. An existing file at that path is `ErrPathCollision`, naming the
  colliding files. An expanded path with a segment starting with `.` is refused, `DryRun`
  included, with an error wrapping `ErrProject` (§10.3).
- **N4.** Without `@files`: a `.canon` entry goes to `<package dir>/<value name>/<key>.canon`
  (SPEC §4.3); a `load.dir` element goes to the directory of the glob's first wildcard segment,
  named `<key>.json`, and a glob where that is not determined (`**` before the file name, several
  wildcard directories) requires `@files`, else `ErrNotEditable` with reason `order`.
- **N5.** A new `.canon` entry file contains exactly: the `package` line, a blank line, and the
  `entry <value>.<key> { … }` declaration printed canonically, ending with one `\n`. A new JSON
  element file contains the element's wire form in canonical source layout.
- **N6.** `Remove` of an entry declared in its own file deletes the file when the entry is the
  file's only declaration, else removes the declaration (M4); a `load.dir` element's file likewise.
  Directories left empty are removed, up to but not including the package directory, deepest
  first, after the file deletions; they are journaled and recreated on rollback, and a directory
  no longer empty at commit time is left.
- **N7.** `Retire` inserts `retired ` before the `entry` keyword, before the entry's key in a
  table literal, or before the member's name in an `@codes` enum declaration.

### 10.2 Renamed files

- **N8.** A file renamed by `Rename` (E11) keeps its content except the changed key; its old path
  is reported as `OldPath` of a `renamed` change. A request's `Changes` are each path's net effect,
  its base state against its final one: a path that existed and exists with other bytes is
  `modified`, one that existed only is `deleted`, one that exists only is `created`. `renamed` is
  only an optional pairing of a `deleted` and a `created` path whose bytes came from it, when
  neither path is otherwise involved. So a swap, or a rename into a path freed earlier in the same
  request, is valid and commits: a path the request frees is not an existing file for N3
  (DECISIONS 273).

### 10.3 Atomicity and crash safety

- **N9.** Before writing, every file to be written or deleted is re-read and its hash compared with
  the snapshot. A difference is `*StaleError` (wraps `ErrStale`) listing the files: someone changed
  them while the edit ran (API-05).
- **N10.** The edit writes a journal `.canon/journal/<hex>.json`, `<hex>` being the new revision
  without its scheme prefix (`r1:`; `:` is not allowed in Windows file names), listing, for each
  file, its path, its previous content (or its absence) and mode, and the SHA-256 of its new content
  (or its absence); the directories the edit creates and removes (N6), with their modes; and the
  writer's host and process id. It syncs it. It then writes every new content to a temporary file in
  the same directory, renames each over its target, performs deletions, removes emptied directories,
  removes the journal, and publishes the new snapshot. When the FS offers `SyncDir` (§2.2), the
  directories are synced before the first rename and before the journal is removed.
- **N11.** If a step after the journal fails, every file already changed is restored from the
  journal and the edit returns the error. If the process dies, the next `Open` restores them (O5).
  A journal is untrusted input (`Open` runs `Recover`, and a cloned repository could ship one), so
  every rollback and recovery holds to these checks:
  - it restores or removes only files still in the new state the journal records (the digest of
    their new content, or their absence), all or nothing: a file changed since keeps the journal,
    and the error names the files, so no one overwrites or deletes a file through a journal
    without knowing its exact bytes;
  - it restores only what this edit changed and recreates the directories it removed; a directory
    the edit created must lie above a file the edit created, and is removed with its contents only
    when it holds nothing but the edit's new files, their staged copies and temporary leftovers;
  - it accepts only paths inside the project directory or a declared root as this `Open` places
    it (O2: `Options.Roots`, `project.local.canon`, `project.canon`), paths outside the project
    being stored absolute. A journal written before a root moved names paths outside every
    current root: it is kept, with an error naming it and the files (O5), and nothing is written.
    A journal holding a path under an absent optional root is kept too, since applying it could
    create the root's directory (DECISIONS 332). Accepted paths also have no segment starting
    with `.`, and with an extension an edit
    writes (`.canon`, `.json`, `.lock`); it never traverses a symbolic link (every component it
    resolves must equal the lexical path; a link is removed, never descended);
  - it applies modes masked to the read and write permission bits (`mode & 0o666`), and refuses
    whole a journal holding a setuid, setgid or sticky bit;
  - it keeps, and never applies automatically, a journal written on another host (O5), and leaves
    alone one whose writer still runs on this host; a journal written by this process counts as
    running only while one of its commits runs there, so `Close` then `Open` recovers one a failed
    rollback left;
  - it keeps a journal that decodes but fails a check, writing nothing, and removes one that does
    not decode (only a torn write makes one, before any file changed).

  While any journal is present, every `Edit` that writes is refused with an error wrapping
  `ErrProject` that names it. `Edit` also refuses, with an error wrapping `ErrProject` naming the
  files, a change through a symbolic link below the project or a root (a symlinked package
  directory cannot be edited through the API) and a change to a path with a segment starting with
  `.`, so no commit is made whose rollback would be refused. A renamed file keeps its mode. Residual
  risks, accepted: a reused process id keeps a dead writer's journal "running"; the cross-process
  idle check is not atomic; a forged same-host journal can create new `.canon`, `.json` and `.lock`
  files, but overwriting or deleting one still needs its exact content.
- **N12.** Written files use `\n` line endings and end with exactly one `\n` (NFR-04).

---

## 11. Evaluate: live view state

The studio never evaluates Canon expressions itself (SPEC §16.10). `Evaluate` computes what the
view of one value shows, for the current sources or for a draft (VIEW-01).

```go
func (p *Project) Evaluate(ctx context.Context, r EvalRequest) (*EvalResult, error)

type EvalRequest struct {
    Base  Revision   // optional; stale → ErrStale
    Path  string     // the value the studio shows: an entry, an element, a top-level value
    Draft []Op       // applied in memory, never written; same rules as Edit (§8)
    Lang  string     // overrides Options.Lang
}

type EvalResult struct {
    Revision Revision
    Path     string                     // canonical, package-qualified
    Title    Text                       // view `title`, else the key, else the value name
    Subtitle Text
    Preview  string                     // wire value of the `preview` asset, "" if none
    When     map[string]bool            // item key → shown (§11.2)
    Show     []ShowLine                 // in view order, nested views after their owner
    Headings map[string]Heading         // relative path → heading of a collection element
    Types    map[string]json.RawMessage // relative path → resolved type of a dependent field (VM encoding)
    Findings []Finding                  // findings located at Path or below, F2 order
    Summary  Summary                    // over the affected packages, with the draft applied
    Dropped  []Dropped                  // cascades caused by the draft (§8.5)
}

type Text      struct { Value string; OK bool; Fallback bool }
type ShowLine  struct { Owner, Key, Label string; Text Text }
type Heading   struct { Title, Subtitle Text; Preview string; Retired bool; Cells map[string]Text }
```

- **V4a.** `EvalRequest`, `EvalResult` and every type they hold have a JSON form, the
  one the studio's web client exchanges (VIEWMODEL.md §13): the struct tags of `api/evaluate.go`
  (`Dropped` in `api/edit.go`, `Finding` and `Summary` in `api/findings.go`),
  lowerCamel keys (`revision`, `path`, `title`, `subtitle`, `preview`, `when`, `show`,
  `headings`, `types`, `findings`, `summary`, `dropped`; `value`, `ok`, `fallback` in a `Text`;
  `owner`, `key`, `label`, `text` in a `ShowLine`; `title`, `subtitle`, `preview`, `retired`,
  `cells` in a `Heading`; `path`, `value` in a `Dropped`), in declaration order. A `Finding` is
  written as F5 says and a `Summary` as the CLI summary object. Every key of a result is written,
  an empty collection as `[]` or `{}` (never `null`); in a request, `base`, `draft` and `lang` may
  be omitted. Map keys are written in byte order (§1.4). An `Op` of `Draft` is written as §8.8.

### 11.1 What is evaluated

- **V5.** The **form** of `Path` is the value at `Path` and every record or variant value reached
  from it through fields only (not through a list, table or map). Every view item of every value
  in the form is evaluated: `title`, `subtitle`, `preview`, `when` conditions on fields and groups,
  `show` lines, and methods named in the view (MOCKUP-GAPS 24, as `show` lines). `Types` holds each
  dependent field of the form whose type can be resolved: a field whose driver is `none` or cannot
  be resolved has no entry.
- **V6.** Every element of every collection that is a field of a value in the form gets a
  `Heading` (its view's `title`, `subtitle`, `preview`), so the studio can list rows. When `Path`
  itself names a collection, each of its elements gets a `Heading` too, so one `Evaluate` serves a
  table screen (VIEWMODEL.md Q4). Elements deeper than that are evaluated when the studio opens
  them with another `Evaluate`.
- **V6a.** `Heading.Cells` holds, for each column of mode `text` of the collection's table control
  (VIEWMODEL.md T8), keyed by field key, the rendered cell: a record's title, a ref's target
  title, a value's canonical text. `Heading.Title` is disambiguated as VIEWMODEL.md S9 says: when
  two elements of one collection render the same title, each becomes `<title> (<key>)`, or
  `<title> (#<n>)` in a plain list. Disambiguation is relative to a list: the title of the one
  element an `Evaluate` names (`EvalResult.Title`) is never disambiguated.
- **V7.** A title of a ref in a template uses the target's `title` if its type has a view, else its
  key; `{x.id}` forces the key (MOCKUP-GAPS 22). A title with no view is the key (entries,
  elements), or the value name (top-level values). A plain-list element has no key: its title with
  no view is `#<n>`, its 1-based position (VIEWMODEL.md S9, T24), even when it holds a copy of a
  table entry (only a keyed list names its elements by key).
- **V8.** Texts are in `Lang`, falling back to the source language; `Fallback` is true when the
  fallback was used (MOCKUP-GAPS 45). It is true for every text when the package has no
  translation file for `Lang`, or when `Lang` is neither the source language nor a project
  language. A nested text that falls back (an enum label, a ref's target title, a field's `none`
  text, a method's line) sets the flag of the text holding it. Detecting it spends no step: a text
  costs the same steps in every language.

### 11.2 Keys and failures

- **V9.** Keys of `When`, `Headings` and `Types` are canonical paths **relative** to `Path`
  (P8, without the leading `.`): `cooldown`, `global.visitCost`, `levels[0]`. A group's key is the
  relative path of its record followed by `#` and the group id: `#combat` for a group of the value
  at `Path`, `global#unlock` for a group of a nested record. Only items that have a `when` appear
  in `When`; an item absent from `When` is shown.
- **V10.** `ShowLine.Owner` is the relative path of the value whose view declares the line;
  `ShowLine.Key` is its label's translation key (I18N.md §3.3): `Type.show.<id>` for a `show` line
  (`Type.show._<n>` for the `n`-th unnamed one, from 0), or `Type.<method>` for a method item.
- **V10a.** `Findings` carry `Reads` (§4.1) like the findings of a build, so live findings
  highlight fields too (VIEWMODEL.md Q5).
- **V11.** A `show` line, title, subtitle or preview whose evaluation fails has `OK: false` and an
  empty `Value`; it produces **no finding** (MOCKUP-GAPS 38). The studio shows "—". A title,
  subtitle or preview the view does not declare is not a failure: `OK` is true, the title is V7's,
  the others are empty (DECISIONS 243). A method that is broken (TYPES.md §1) is never run: its
  line fails, and the view naming it is not itself broken.
- **V12.** A `when` condition whose evaluation fails counts as **true** (the item is shown) and
  produces no finding, so a field with a finding is never hidden by a failing condition
  (MOCKUP-GAPS 33). When an item is given `when` twice (`E1613`), the first written is used.
- **V13.** `Evaluate` runs on the snapshot (plus the draft): without a draft, its values are those
  of the snapshot's every-package analysis, the one `Value` reads and `Check` gives; with one, those
  of that analysis with the draft applied. Running only the touched packages is allowed only where
  it provably gives the same values, budget exhaustion included (DECISIONS 252). It counts steps
  against the budget like any evaluation (EVL-03): each template, `when` condition and method it
  runs is a run of its own, capped at the project's budget, and a run past it fails that text (V11)
  or makes that condition true (V12). Runs never share one budget, so no result depends on
  evaluation order. It never writes and never changes the revision. Without a draft, `Summary`
  covers the path's package and every package importing it, and those are the packages it touches
  (S5).
- **V14.** The studio calls `Evaluate` after each committed edit, including edits written with
  `AllowErrors`, not on each keystroke (MOCKUP-GAPS 47). `Edit.Evaluate` saves the extra call.

---

## 12. Watching

```go
func (p *Project) Watch(ctx context.Context, fn func(Event)) error

type Event struct {
    Revision Revision
    Cause    EventCause   // external | edit | overlay
    Files    []string     // files that changed, display paths, byte order
    Packages []string     // packages re-checked, sorted
    Findings []Finding    // ALL current findings of Packages (replace, do not merge)
    Summary  Summary
    Err      error        // non-nil when the re-check failed (*InternalError, ErrUnknownLayer)
}
```

- **W12.** `Watch` starts watching the read set of every loaded package, the package directories,
  the directories listed by globs, `project.canon` and `project.local.canon` (whether it exists
  or not), then returns. The presence of a root is judged again at each new snapshot; a root that
  only appears or disappears, with nothing in the read set changing, is seen at the next `Build`
  or the next change. It returns an error only if
  watching cannot start. Watching stops when `ctx` is cancelled or the project is closed. Starting
  runs one check of every package, so the read set of each is known (one package at a time when they
  cannot be checked together). The watcher follows the OS's change notifications for the OS FS or an
  FS whose `OSBacked()` is true (§2.2), and falls back to polling, with one Warn line, when they
  cannot start or later fail. Whatever the notifications deliver, any change to the read set is seen
  within the resync interval (at most 1 s): the watcher also stats the read set, watches the
  directories of link targets and retries directories it could not watch.
- **W13.** `fn` is called on one goroutine, never concurrently with itself, in revision order. An
  event's `Files`, and what makes an event, are only read-set files and names the scan takes as
  sources (the S5 predicate): editor swap files, `.canon/` and build outputs produce none; a listed
  directory's change counts only when a name it gains or loses is a source or matches the glob of a
  watching `load`. Events compare snapshots, so `Files` may name one written file under each display
  path that reaches it (DECISIONS 253). An event is delivered when its cause is not `external`, when
  a package was re-checked or removed, when the revision changed, or when there is an error. The
  event's revision is computed after its re-check. A package removed since the `Watch` started is
  listed in `Packages` with no findings. A `project.canon` broken while watching gives an event
  whose `Err` is a `*ProjectError`, its findings in `Findings`, and no packages; a load this
  compiler cannot run gives that error in `Err`. A panic in `fn` is recovered and becomes an
  `*InternalError` in the next event's `Err`; a panic in the watcher reaches every `Watch` as an
  event with cause `external` and an `*InternalError`, and the watcher restarts (at most once per
  resync interval).
- **W14.** Changes are coalesced: after a change, the watcher waits until 100 ms pass without a new
  change (API-05), but never more than 1 s after the first change, then refreshes, re-checks the
  affected packages and calls `fn` once.
- **W15.** An `Edit` that writes files produces exactly one event with cause `edit`; the watcher
  does not report the edit's own writes as `external`. The event is published inside the write
  (S10), so it may reach `fn` before `Edit` returns; its revision equals the result's. An external
  change made during the write is folded into that event. Overlay changes produce `overlay`
  events. Edit and overlay events are always delivered, even when they touch nothing the watch
  follows; a refused or `DryRun` edit publishes no event.
- **W16.** Several `Watch` calls may be active; each receives every event.

---

## 13. Building and testing

### 13.1 Build

```go
func (p *Project) Build(ctx context.Context, o BuildOptions) (*BuildResult, error)

type BuildOptions struct {
    Packages []string   // selectors (R1); none = all
    Targets  []Target   // go cpp ts json view text; none = all
    Check    bool       // write nothing; report what would change (CLI --check)
    Adopt    []string   // outputs the build may take over without a marker (CLI --adopt, B2)
}
type BuildResult struct {
    Check   *CheckResult
    Outputs []Output      // byte order of Path
    Lock    []LockChange  // lines appended (or that would be), per package
    Stale   bool          // Check mode: an output or lock would change
}
type Output struct { Path string; Target Target; Package string; Status OutputStatus } // written | unchanged | stale | adopted
type LockChange struct { Package string; File string; Lines []string }
```

- **B1.** `Build` behaves as CLI.md §3.4: code and data emits run only without errors; `emit view`
  runs even with errors (VM-07); outputs are written atomically and only when their content
  changes; a target without the GENERATED marker of GEN-05 is `E8001` (a finding, not an error
  value).
- **B1a.** An error in any loaded package, selected or imported, blocks code, data and the lock;
  the error findings of an imported package are then reported with the selection's (beyond R2),
  so nothing is refused without its reason. A build with layers never writes `canon.lock`
  (LOCK.md §6.1).
- **B1b.** A `Target` in `BuildOptions.Targets` that is not `go`, `cpp`, `ts`, `json`, `view` or `text` is
  refused with `*ValueError` (wraps `ErrBadValue`; `Expected` is `go, cpp, ts, json, view or text`),
  never dropped. A `Build` that writes holds the project's write lock (S9) and returns the
  revision read after its writes (S10), in `Check.Revision`.
- **B2.** An output that exists without the generated-file marker is taken over only when it is
  listed in `BuildOptions.Adopt` (CLI `canon build --adopt <path>`: the header of an
  `access: both` struct, CODEGEN.md §7.8.3, or a file of an `emit text`, §2.9), or by `canon convert --adopt` (IMPLEMENTATION-PLAN.md
  §8.3). Its `Status` is then `adopted` (GEN-05). Any other unmarked output is `E8001`.
  `BuildOptions.Adopt` takes only those two kinds: a JSON output without its `$schema` is `E8001`
  even when listed.

### 13.2 Test and lock check

```go
func (p *Project) Test(ctx context.Context, o TestOptions) (*TestResult, error)
type TestOptions struct { Packages []string; Run string }   // Run: RE2, matched against the test name
type TestResult struct { Tests []TestCase; Passed, Failed int; Duration time.Duration }
type TestCase struct { Package, Name string; Span; Passed bool; Failures []ExpectFailure }
type ExpectFailure struct { Span; Expect, Expected, Got string; Findings []Finding }

func (p *Project) LockCheck(ctx context.Context, packages ...string) (*CheckResult, error)
```

- **B3.** `Tests` are in (package, file, line) order. `Expect` is the source text of the `expect`
  statement, `Expected`/`Got` are canonical text forms (STD-06), `Findings` are the findings that
  were actually produced (for `fails`/`warns`).
- **B4.** `LockCheck` evaluates only stable collections and what they depend on (LCK-03).

---

## 14. Formatting helpers

```go
func Format(filename string, src []byte) ([]byte, error)
func FormatJSONSource(src []byte) ([]byte, error)
```

- **T1.** `Format` returns the canonical layout of one `.canon` file (FORMATTER.md). A syntax error
  returns a `*SyntaxError` with its findings. `Format(Format(x)) == Format(x)`. A file whose base
  name is `project.canon` is formatted as a project file.
- **T2.** `FormatJSONSource` returns the canonical source layout of a JSON file (FMT-02); key
  order is preserved. Invalid JSON returns a `*SyntaxError`; having no file name, its findings use
  the display name `<json>`.

```go
func Version() VersionInfo
type VersionInfo struct {
    Compiler    string   // semver, "0.1.0"
    Languages   []string // supported language versions, ascending: ["0.1"]
    Fingerprint string   // "canon-fp v1"
    ViewModel   string   // "canon-vm/1"
    Lock        string   // "canon.lock v1"
    Commit      string   // VCS revision of the compiler build, "" if unknown
}
```

- **T3.** `Commit` is the compiler's revision, never the embedding program's: the `vcs.revision`
  the go command stamped when the compiler is the main module and `vcs.modified` is not `true`;
  the revision a pseudo-version of `github.com/fantasim/canonlang` names when a program depends
  on it; `""` for a tagged or replaced dependency and for a build without VCS information.

---

## 15. Errors

Errors are Go errors, distinct from findings. Every error type wraps one sentinel, so callers use
`errors.Is`.

| Sentinel | Error type | When | CLI exit |
|---|---|---|---|
| `ErrNoProject` | | no `project.canon` found (O1) | 2 |
| `ErrProject` | `*ProjectError` | `project.canon` or `project.local.canon` has errors, bad `Roots`, a required root absent (O3); wrapped by the error of a journal `Open` must keep (O5), and of an edit refused because a journal exists or a path goes through a symbolic link or a hidden segment (N3, N11), which name the journal or the files | 2 |
| `ErrUnsupportedVersion` | `*ProjectError` | `E1001` | 2 |
| `ErrUnknownPackage` | | a selector matches no package (R1) | 2 |
| `ErrUnknownLayer` | | a layer name matches no file (O4, LAY-01) | 2 |
| `ErrBadPath` | `*PathError` | path syntax, unsupported segment (§6); an overlay path outside the project (§3.4) | 2 |
| `ErrNoPath` | `*PathError` | nothing at that path | 2 |
| `ErrAmbiguousPath` | `*PathError` | unqualified root matches several packages (P6); a rename name (E27) | 2 |
| `ErrNoValue` | `*PathError` | the value is poisoned (R6) | 1 |
| `ErrInputField` | `*PathError` | the path names an `input` field (R5); `Detail` is its environment variable | 2 |
| `ErrBadOp` | `*PathError` | op not valid for that container (E2); `Op` or `Edit` JSON that does not decode (§8.8), with the reason; E28, E31 | 2 |
| `ErrBadValue` | `*ValueError` | value does not fit the type; bad template value (V1, N2); unknown build `Target` (B1b); a rename's new name (E30) | 2 |
| `ErrKeyExists` | `*PathError` | E3 | 1 |
| `ErrStableKey` | `*PathError` | E4 (`Remove`, `Rename`, `Unretire` of a stable id), E13, E28, E29 | 1 |
| `ErrNameClash` | `*PathError` | E35: a rename would change what another name refers to | 1 |
| `ErrNotEditable` | `*NotEditableError` | §7.2 | 1 |
| `ErrStale` | `*StaleError` | S5, N9 | 1 |
| `ErrRejected` | `*RejectedError` | the edit produced errors (E19) | 1 |
| `ErrNotCanonical` | `*NotCanonicalError` | M9 | 1 |
| `ErrPathCollision` | `*PathError` | N3 | 1 |
| `ErrOverlay` | `*PathError` | S12 | 1 |
| `ErrSyntax` | `*SyntaxError` | `Format` or `FormatJSONSource` on input that does not parse (§14); added to `api/errors.go` by the M0 split (IMPLEMENTATION-PLAN.md §12.4) | 1 |
| `ErrClosed` | | O6 | 3 |
| `ErrInternal` | `*InternalError` | a compiler bug; carries a Go stack | 3 |

- **X1.** Error types carry `Op int` (index in `Edit.Ops`, `-1` when not an op), `Path` (as given)
  and, where relevant, `Detail` (a sentence for humans). Every error type's `Error()` text has
  one form, `"<op N: ><path: ><sentinel text><: detail>"`: `op N: ` only when `Op` is 0 or more,
  the path and its `: ` only when the path is not empty, `: ` and the detail only when the detail
  is not empty. A type without an `Op` or a `Path` field starts with its sentinel's text, so every
  text holds the text of the sentinel `errors.Is` matches. The detail is: `Detail` for a
  `*PathError`; `expected <Expected>, got <Got>`, then `: <Detail>` if any, for a `*ValueError`;
  the `Reason`, then `: <Detail>` if any, for a `*NotEditableError`; the files joined by `, `
  for a `*StaleError` and a `*NotCanonicalError`; the number of error findings (`1 error`,
  `2 errors`) for a `*RejectedError`; the first finding, as `<file>:<line>:<col>: <message>`
  (its message alone when it has no file), for a `*ProjectError` and a `*SyntaxError`; `Msg`
  for an `*InternalError`.
- **X2.** A panic inside the compiler is recovered at the API boundary and returned as
  `*InternalError`; the project stays usable for reads, and an edit in progress is rolled back.

---

## 16. Errata for CLI.md

These changes to CLI.md follow from this document (owned by the SPEC/CLI errata agent):

1. §5.2: `Check` returns `(*CheckResult, error)`; `Value` returns `*canon.Value`.
2. §5.3: the example uses typed values: `canon.Set("farm.modelTypes[3].maxLevel", canon.Int(10))`,
   `canon.Add(…, canon.Obj{"level": canon.Int(10), …})`. `Add(path, v, key)` is `AddEntry(path,
   key, v)`. Add `Insert`, `Reset`, `Rename`, `Unretire`, `SetCase` to the table, and `DryRun`,
   `Normalize`, `Evaluate` to the request. `ErrStale` is decided per package (S5).
3. §5.3: "Minimal writes" refers to API.md §9; "values, not text" refers to §8.2.
4. §5.4: `Watch` returns an `error`; events carry `Revision`, `Cause`, `Files`, `Packages`,
   `Findings` (complete, replace), `Summary`.
5. Add §5.6 `Evaluate` (VIEW-01), referring to API.md §11.
6. §2.3: add `--root <name>=<dir>` (repeatable; `Options.Roots`), used by tests and fixtures.
7. §2.4 and SPEC §21.1: the JSON finding gains `endLine`, `endCol`, `pointer`, `package`,
   `check`, `layer`, `stack`, and `related[].col/endLine/endCol` (F5). Its `path` starts with the
   value name (`farm.modelTypes[3]…`, F1).
8. §2.6: add `[#n]` (position) and state that `[n]` on a keyed list or table is a key (P1).
9. §5: the module path is `github.com/fantasim/canonlang` (DECISIONS 23), the API package
   `github.com/fantasim/canonlang/api`.
10. §3.4 and §5: `canon build --adopt <path>` is `BuildOptions.Adopt` (B2); `Unretire` is always
    refused (E4).
