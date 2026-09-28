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
  checks `project.canon`, applies `Options.Roots`, and scans the file set (every `.canon` file
  under the project root, skipping directories whose name starts with `.`, and the directory
  listing needed to map packages to directories). It parses no other source. Parsing, checking and
  loading happen on first use.
- **O3.** Errors from `project.canon` itself (syntax, `E1001` unsupported language version,
  `E1002` unknown key, a root that does not exist) make `Open` fail with a `*ProjectError`, which
  wraps `ErrProject` and carries the findings.
- **O4.** An unknown name in `Options.Layers` is not an `Open` error: layers are found per package
  (LAY-01), so the first call that loads packages returns `ErrUnknownLayer` if no loaded package
  has a file for that layer.
- **O5.** If `.canon/journal/` holds an unfinished edit (§10.3), `Open` rolls it back before
  anything else and logs one line at level Warn.
- **O6.** After `Close`, every method returns `ErrClosed`. `Close` stops every `Watch` and is
  idempotent.

### 2.1 Options

| Field | Default | Meaning |
|---|---|---|
| `Layers []string` | none | layers to apply, in order (SPEC §19, CLI `--layer`) |
| `Lang string` | source language | language of translated texts: check messages, titles, `show` labels (CLI `--lang`) |
| `EditLayer string` | `""` | when set, `Set`/`Reset`/`AddEntry` edits write amendments into this layer (§7.5) |
| `Roots map[string]string` | none | replaces the directory of a declared root (CLI `--root name=dir`); a relative directory is relative to the project root; a name that is not declared is `ErrProject` |
| `FS FS` | the OS | file system used for every read and write (tests use an in-memory one) |
| `Cache string` | `<root>/.canon/cache` | cache directory; `"off"` disables the cache |
| `Workers int` | `runtime.GOMAXPROCS(0)` | parallelism; results are byte-identical for every value (NFR-05) |
| `MaxFindings int` | 1000 | findings kept per package; the rest are counted (§4.3) |
| `Logger *slog.Logger` | discard | diagnostics about the compiler itself, never findings |

- **O7.** `Options` never changes what a program computes, except `Layers` (which is part of the
  program, SPEC §19) and `Roots` (which changes the files read). Both are part of the build
  manifest (LOD-11).

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
  display path bytes. The read set is `project.canon`, every source, translation and layer file
  of the packages loaded so far, every file read by `load` (globs expanded), and, whatever the
  packages loaded, the existing `canon.lock` of every directory below the project root that
  holds a source file or is an ancestor of one (every place a package's lock can be, LOCK.md §2.1):
  the revision cannot know the packages of files not yet parsed. Overlays (§3.4) replace file
  contents in the listing.
- **S4.** The project remembers the per-file hashes of at least the last 64 revisions it has
  produced. A revision it does not remember, or one from another `Project`, is stale.
- **S5.** A revision is compared per package: an edit or an evaluation against `Base` is **stale**
  when a file in the read set of the packages it touches (§8.6) has a different hash now than at
  `Base`. Changes to unrelated files do not make it stale.
- **S6.** `Base == ""` disables the staleness check. It is for scripts and tests; the studio MUST
  send the revision it last read.

### 3.3 Concurrency (API-05)

- **S7.** A `Project` is safe for concurrent use by any number of goroutines.
- **S8.** Reads (`Check`, `Value`, `Refs`, `ViewModel`, `Evaluate`, `Test`, `LockCheck`,
  `Build` with `Check: true`) run in parallel with each other and with a writer. Identical
  concurrent reads of the same snapshot share one computation.
- **S9.** There is one writer at a time: `Edit`, `Build` that writes, `SetOverlay` and
  `ClearOverlay` take the project's write lock. A writer never blocks readers; readers that
  started before the writer finished keep their snapshot.
- **S10.** A writer publishes its new snapshot atomically when it returns. The revision in its
  result is that snapshot's.
- **S11.** Every method takes a `context.Context`. Cancelling it makes the call return
  `ctx.Err()` promptly; a cancelled `Edit` writes nothing, or, if cancellation arrives after the
  first rename, completes (§10.3).

### 3.4 Overlays

```go
func (p *Project) SetOverlay(file string, content []byte) error
func (p *Project) ClearOverlay(file string) error
```

An overlay replaces a file's content in memory without writing it; the language server uses it for
unsaved buffers. `file` is a display path or an absolute path. Overlays take part in every read
and in the revision.

- **S12.** `Edit` refuses to write a file that has an overlay (`ErrOverlay`): the buffer and the
  disk would disagree.

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
  `Value`.
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
  are kept. The counts in `Summary.Errors`/`Warnings` include the dropped ones;
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
  ` (<note>)` when the note is not empty. For example:

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
  were dropped, one more line `  (<n> more frames)`.
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
| `JSON() []byte` | the wire form (WIRE.md), exactly as `emit json` would write it inside `value` |

- **R4.** A `*Value` holds its snapshot: it stays valid and unchanged after later edits.
- **R5.** `Value` on a path that does not exist is `ErrNoPath`; a syntax error is `ErrBadPath`;
  an ambiguous unqualified root is `ErrAmbiguousPath` (§6.3).
- **R6.** A value that could not be computed (poisoned, EVALUATION.md) is `ErrNoValue`, which
  wraps the findings that explain why.

`Origin` records where a value comes from (EVL-07):

```go
type Origin struct {
    Kind    OriginKind // literal | json | csv | defines | text | default | spread | computed | layer
    Span               // the literal, the JSON value's first byte, the default expression, the
                       // building expression, or the amendment
    Pointer string     // RFC 6901, for json
    Layer   string     // for layer
    Via     *Origin    // default: the literal or JSON object that omitted the field;
                       // spread: the origin of the copied value
    Stack   []Frame    // computed: the Canon call stack that built it, at most 16 frames
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

type RefsResult struct { Target string; Refs []Ref }
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
  the target's importers.
- **R8.** Order: F2 order of the spans. `canon refs` prints them in this order (CLI.md §3.8).

### 5.4 ViewModel

```go
func (p *Project) ViewModel(ctx context.Context, pkg string) (*ViewModel, error)

type ViewModel struct { Package string; Revision Revision }
func (vm *ViewModel) JSON() []byte
func (vm *ViewModel) Decode(v any) error
```

- **R9.** `JSON()` returns exactly the bytes `emit view` would write for `pkg` in this snapshot
  (VIEWMODEL.md), whether or not the package declares `emit view`. It is produced even when the
  package has errors (VM-07).
- **R10.** Typed Go structs for the view model are generated from `spec/viewmodel.schema.json`
  into package `github.com/fantasim/canonlang/api/vm` (IMPLEMENTATION-PLAN M3). `Decode` accepts
  them.

### 5.5 Packages and project facts

```go
func (p *Project) Root() string
func (p *Project) Packages(ctx context.Context) ([]PackageInfo, error)   // sorted by Name
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

- **P8.** Fields: `.name`. Table entries: `.key`. Keyed-list elements: `[key]`. Plain-list elements:
  `[n]`. Map entries: `[key]`. Nothing uses `[#n]` in canonical form.
- **P9.** A key is written as a word when it is one and the key type is not an integer type;
  integers are written in decimal; enum keys are written as the Canon member name; every other key
  is written as a JSON string, escaped as WIRE.md escapes strings. For a literal-union key type
  (`T | "lit"`), a key that is one of the literals is always written as a JSON string (P2 reads a
  word as a `T`), so the canonical form resolves to itself.
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
| `format` | the source is `load.csv`, `load.defines` or `load.text` (not editable in v0) |
| `input` | an `input` field (it has no value at build time) |
| `key` | the key field of a keyed-list element, or a key of a dependent map (MOCKUP-GAPS 17); keys change only with `Rename` |
| `pseudo` | `.id`, `.retired`, `.kind` (P3) |
| `order` | `Insert` or `Move` in a collection whose order comes from file paths (`load.dir`, entry files) |
| `layer` | an operation that cannot be written as an amendment when `EditLayer` is set (§7.5) |

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
  `base`.

### 7.5 Layers (MOCKUP-GAPS 48)

- **W10.** Without `EditLayer`, edits go to base sources. A path whose final value is set by an
  active layer is `layered` (W5), so the studio shows it read-only with the layer's name. Paths the
  layers do not touch are edited normally, even while layers are active.
- **W11.** With `EditLayer: "x"`, `Set`, `Reset` and `AddEntry` write into the `x` layer file of
  the package that declares the value's root: `Set` adds or replaces the amendment line for that
  path (`amend <root> { <path>: <expr> }`), `Reset` removes it, `AddEntry` adds an amendment that
  creates the key (LAY-02). If the package has no `x` layer file, it is created as
  `<package dir>/<x>.layer.canon`. Every other op is `ErrNotEditable` with reason `layer`. A layer
  cannot add entries to a stable table (LCK-04), which the re-check reports as a finding.
- **W11a.** A layer may not hold a path and one of its prefixes (EVALUATION.md §9.2, `E1908`). So
  when the layer already amends an ancestor of the path, `Set` edits the value inside that
  amendment's right-hand side (a structural edit of that literal, §9); when it amends descendants
  of the path, `Set` replaces them all with the one new amendment line.

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
| `FromJSON(raw)` | the wire form | any type: decoded by WIRE.md's rules for that type |
| `Source(text)` | a Canon literal | any type: parsed as a Canon expression that contains only literals (no names but contextual ones, SPEC §6.2), typed against the expected type |

- **V1.** A value that does not fit the expected type is `*ValueError` (wraps `ErrBadValue`) naming
  the op, the path, the expected type and what was given. Nothing is written.
- **V2.** Only the static type is checked at this point. Refinements (`Int(1..=100)`), references
  (`E3501`), keys and checks are verified by the re-check (§8.6), like any other value.
- **V3.** `Obj` omits fields to leave them at their defaults. A required field that is missing is a
  finding of the re-check (`E3302`), not a `ValueError`.
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

- **E1.** Ops apply in order to the state left by the previous ops; paths in later ops see earlier
  changes (a key added by op 0 can be addressed by op 1).
- **E2.** An op on a container kind it does not support (for example `Add` on a table, `AddEntry`
  on a list, `Reset` on a required field) is `ErrBadOp`.
- **E3.** `Add`, `Insert` and `AddEntry` with a key that already exists are `ErrKeyExists`.
  `Set` on a map key or table entry that does not exist is `ErrNoPath` (use `AddEntry`).
- **E4.** `Remove` on an entry of a stable table, and `Rename` of such an entry, are
  `ErrStableKey` (SPEC §12: retire it instead). `Remove` of an `@codes` enum member is not an op.
  `Unretire` is always `ErrStableKey`: retirement is one-way, and bringing an id back is a reviewed
  hand edit of `canon.lock` (LOCK.md §4.6), which the build would otherwise report as `E6002`.
- **E5.** `Set` of the whole value of an entry, element or map entry keeps its key: a `v` whose
  key field differs from the current key is `ErrNotEditable` with reason `key`.
- **E6.** `Set(path, v)` where `v` equals the field's default removes the field from its literal or
  JSON object (API-06), exactly as `Reset(path)`. Equality is the value equality of TYPES.md §7.5;
  for a default that depends on earlier fields (TYP-15), the default is computed from the edited
  record.
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
  - the entry's file, when the table has `@files` and the file's path equals the template
    instantiated for the old entry: the file is renamed to the template instantiated for the new
    entry (§10.2).
- **E12.** If any reference of kind `value` or `key` is not editable (for example a ref computed by
  a function), the whole rename fails with `*NotEditableError` for that reference; `Detail` lists
  every such reference.
- **E13.** `Rename` of a key in a dependent map is `ErrNotEditable` with reason `key`
  (MOCKUP-GAPS 17). `Rename` in a stable table is `ErrStableKey` (E4).

### 8.5 Cascades: case changes and dependent fields

Some edits make other values invalid in a predictable way. The compiler applies the consequence
inside the same edit and reports it, so every client behaves the same.

- **E14.** `SetCase(path, case, fields)` builds the new case value from: every field of the old
  case whose name exists in the new case with the same type after removing refinements (VIEW-07's
  comparison) and whose value satisfies the new field's refinements; then `fields`, which override.
  Every old field not kept is reported in `Dropped` (MOCKUP-GAPS 13). `SetCase` to the current case
  only applies `fields`.
- **E15.** After all ops are applied, for every record touched by an op, every field whose type is
  a dependent type (SPEC §5.11) computed from a field that changed is re-typed. If its value no
  longer matches the new type and the field is optional, it is set to `none` (E7 rules) and
  reported in `Dropped` (MOCKUP-GAPS 16). A required field that no longer matches is left as is,
  and the re-check reports `E3802`.
- **E16.** Cascades never apply to anything but the records touched by the ops.

### 8.6 Checking and refusal

- **E17.** The **affected packages** of an edit are the packages that own a file the edit writes,
  plus every package that imports one of them. The staleness check (S5) covers their read set.
- **E18.** After applying the ops in memory, the affected packages are re-checked (phases 1–7).
  `Findings` holds all their findings.
- **E19.** If any finding is an error and `AllowErrors` is false, nothing is written, `Applied` is
  false, and `Edit` returns the result together with a `*RejectedError` (wraps `ErrRejected`).
  With `AllowErrors`, the files are written and the result has `Applied: true` and the error
  findings (drafts in progress, MOCKUP-GAPS 47).
- **E20.** An edit that adds an id to a stable table, or a new value of a `@stable` field, appends
  the new lock lines to the package's `canon.lock` inside the same edit (LCK-04), and `Retire` adds
  `retired` to its lock line, according to LOCK.md §5.
- **E21.** Order of checks before writing: path syntax, path resolution, editability, op/container
  kind, value types (V1), staleness (S5), apply and cascades, canonical-layout check (§9.4),
  re-check (E18), write (§10). The first failing step decides the error.

### 8.7 Undo

- **E22.** `Undo` is a list of ops that, applied with `Base` set to the result's `Revision`,
  restores every value the edit changed (including cascades). It restores values, not text:
  comments of removed items and a deleted entry file's doc comment are not restored, and a
  recreated entry file is placed by the `@files` template.
- **E23.** Inverses: `Set` → `Set(old)`, or `Reset` if the field was absent; `Reset` → `Set(old)`;
  `Add`/`Insert`/`AddEntry` → `Remove`; `Remove` → `Insert(parent, oldPosition, old)` or
  `AddEntry(parent, key, old)` followed by a `Move` to the old position; `Move` → `Move` back;
  `Rename` → `Rename` back; `SetCase` → `Set(old whole variant value)`. `Retire` has no inverse
  (E4): `Undo` restores every other change of the edit, and the studio warns before retiring.
  `Undo` lists the inverses in reverse order of the ops. Old values are carried as `Source` literals.

### 8.8 JSON form of an edit

The studio's web client sends edits as JSON. `Edit` has this form through its struct tags
(`base`, `ops`, then `allowErrors`, `dryRun`, `normalize` and `evaluate`, each omitted when
false or empty), and `Op` implements `json.Marshaler` and `json.Unmarshaler` (E24–E26):

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
  setCase`. A value is given either as `value` (wire form, decoded as `FromJSON`) or as `source`
  (Canon literal text, decoded as `Source`), never both. `"value": null` is `None`.
- **E25.** `key` is a JSON string or integer; it is read as a path key (P1, P2), so for an enum
  key a string is first matched as a Canon member name, then as a wire value.
- **E26.** Marshalling writes `source` for every `Lit` except `FromJSON`, which is written as
  `value`. `Source` text produced by the API is canonical (FORMATTER.md), single-line.

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
  around it are then collapsed as FORMATTER.md requires.
- **M5.** After the splices, the file MUST be a fixed point of the formatter. If it is not (a line
  grew past the width, a single-line literal must now break), the smallest enclosing item that
  makes it a fixed point is re-printed instead, going up to the top-level declaration if needed.
- **M6.** Invariant, checked by the edit golden tests: `format(after) == after`, and every byte of
  `after` outside the re-printed and inserted items equals the corresponding byte of `before`.
  Because the formatter never aligns columns (DECISIONS 18), a one-value `Set` of a scalar changes
  exactly one line.

### 9.3 Printing values

- **M7.** Values are printed with contextual names (SPEC §6.2): bare enum members, case names and
  keys. Durations use their canonical text (LEX-04). Fields equal to their defaults are omitted in
  newly printed records (E6 applied recursively to new values); fields of an existing literal are
  kept unless the op sets them to their default.
- **M8.** JSON sources are written with wire names and the wire form of WIRE.md. A new key is placed
  after the previous declared field's key in declaration order (FMT-02); unknown keys kept by
  `partial` are left in place. `@json(path: …)` fields create the intermediate objects LOD-09
  describes.

### 9.4 Files that are not in canonical layout

- **M9.** Before writing a file, the edit checks that its current content is a fixed point of the
  formatter (`canon fmt` for `.canon`, the canonical source layout of FMT-02 for JSON). If not, the
  edit fails with `*NotCanonicalError` (wraps `ErrNotCanonical`) listing the files, unless
  `Edit.Normalize` is true, in which case each such file is first normalized entirely, as part of
  the same edit. Migration normalizes JSON sources once with `canon fmt --json-sources`
  (DECISIONS 12), after which this never happens.

---

## 10. Writing files

### 10.1 New entry files (`@files`)

- **N1.** `AddEntry` on a table, or `Add`/`Insert` on a keyed list, creates a new **file** when the
  value's `let` has a `@files` annotation, or when at least one existing entry is declared with
  `entry` in its own file (or, for `load.dir`, always). Otherwise the entry is added to the literal.
- **N2.** The template of `@files("items/{itemKind1}/{id}.canon")` is expanded with the new
  entry's value: `{id}` is the key; `{f}` and `{f.g}` are fields (nested through records and the
  current case of variants). A value is written as: enum → wire value; ref → key; variant → case
  wire name; integer → decimal; `Bool` → `true`/`false`; `String` → itself. A `none` value, a
  string that is empty or contains `/`, `\`, a control character, or is `.` or `..`, is
  `ErrBadValue`.
- **N3.** The expanded path is relative to the directory of the package that declares the value.
  Missing directories are created. An existing file at that path is `ErrPathCollision`.
- **N4.** Without `@files`: a `.canon` entry goes to `<package dir>/<value name>/<key>.canon`
  (SPEC §4.3); a `load.dir` element goes to the directory of the glob's first wildcard segment,
  named `<key>.json`, and a glob where that is not determined (`**` before the file name, several
  wildcard directories) requires `@files`, else `ErrNotEditable` with reason `order`.
- **N5.** A new `.canon` entry file contains exactly: the `package` line, a blank line, and the
  `entry <value>.<key> { … }` declaration printed canonically, ending with one `\n`. A new JSON
  element file contains the element's wire form in canonical source layout.
- **N6.** `Remove` of an entry declared in its own file deletes the file when the entry is the
  file's only declaration, else removes the declaration (M4). Directories left empty are removed,
  up to but not including the package directory.
- **N7.** `Retire` inserts `retired ` before the `entry` keyword, before the entry's key in a
  table literal, or before the member's name in an `@codes` enum declaration.

### 10.2 Renamed files

- **N8.** A file renamed by `Rename` (E11) keeps its content except the changed key; its old path
  is reported as `OldPath` of a `renamed` change.

### 10.3 Atomicity and crash safety

- **N9.** Before writing, every file to be written or deleted is re-read and its hash compared with
  the snapshot. A difference is `*StaleError` (wraps `ErrStale`) listing the files: someone changed
  them while the edit ran (API-05).
- **N10.** The edit writes a journal `.canon/journal/<new revision>.json` listing, for each file,
  its path and previous content (or its absence), and syncs it. It then writes every new content to
  a temporary file in the same directory, renames each over its target, performs deletions, removes
  the journal, and publishes the new snapshot.
- **N11.** If a step after the journal fails, every file already changed is restored from the
  journal and the edit returns the error. If the process dies, the next `Open` restores them (O5).
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

- **V4a. JSON form.** `EvalRequest`, `EvalResult` and every type they hold have a JSON form, the
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
  `show` lines, and methods named in the view (MOCKUP-GAPS 24, as `show` lines).
- **V6.** Every element of every collection that is a field of a value in the form gets a
  `Heading` (its view's `title`, `subtitle`, `preview`), so the studio can list rows. When `Path`
  itself names a collection, each of its elements gets a `Heading` too, so one `Evaluate` serves a
  table screen (VIEWMODEL.md Q4). Elements deeper than that are evaluated when the studio opens
  them with another `Evaluate`.
- **V6a.** `Heading.Cells` holds, for each column of mode `text` of the collection's table control
  (VIEWMODEL.md T8), keyed by field key, the rendered cell: a record's title, a ref's target
  title, a value's canonical text. `Heading.Title` is disambiguated as VIEWMODEL.md S9 says: when
  two elements of one collection render the same title, each becomes `<title> (<key>)`, or
  `<title> (#<n>)` in a plain list.
- **V7.** A title of a ref in a template uses the target's `title` if its type has a view, else its
  key; `{x.id}` forces the key (MOCKUP-GAPS 22). A title with no view is the key (entries,
  elements), or the value name (top-level values).
- **V8.** Texts are in `Lang`, falling back to the source language; `Fallback` is true when the
  fallback was used (MOCKUP-GAPS 45).

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
  empty `Value`; it produces **no finding** (MOCKUP-GAPS 38). The studio shows "—".
- **V12.** A `when` condition whose evaluation fails counts as **true** (the item is shown) and
  produces no finding, so a field with a finding is never hidden by a failing condition
  (MOCKUP-GAPS 33).
- **V13.** `Evaluate` runs on the snapshot (plus the draft); it counts steps against the budget
  like any evaluation (EVL-03). It never writes and never changes the revision.
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
  the directories listed by globs, and `project.canon`, then returns. It returns an error only if
  watching cannot start. Watching stops when `ctx` is cancelled or the project is closed.
- **W13.** `fn` is called on one goroutine, never concurrently with itself, in revision order.
- **W14.** Changes are coalesced: after a change, the watcher waits until 100 ms pass without a new
  change (API-05), but never more than 1 s after the first change, then refreshes, re-checks the
  affected packages and calls `fn` once.
- **W15.** An `Edit` that writes files produces exactly one event with cause `edit`, after the
  edit returns; the watcher does not report the edit's own writes as `external`. Overlay changes
  produce `overlay` events.
- **W16.** Several `Watch` calls may be active; each receives every event.

---

## 13. Building and testing

### 13.1 Build

```go
func (p *Project) Build(ctx context.Context, o BuildOptions) (*BuildResult, error)

type BuildOptions struct {
    Packages []string   // selectors (R1); none = all
    Targets  []Target   // go cpp ts json view; none = all
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
- **B1b.** A `Target` in `BuildOptions.Targets` that is not `go`, `cpp`, `ts`, `json` or `view` is
  refused with `*ValueError` (wraps `ErrBadValue`; `Expected` is `go, cpp, ts, json or view`),
  never dropped. A `Build` that writes holds the project's write lock (S9) and returns the
  revision read after its writes (S10), in `Check.Revision`.
- **B2.** An output that exists without the generated-file marker is taken over only when it is
  listed in `BuildOptions.Adopt` (CLI `canon build --adopt <path>`: the header of an
  `access: both` struct, CODEGEN.md §7.8.3), or by `canon convert --adopt` (IMPLEMENTATION-PLAN.md
  §8.3). Its `Status` is then `adopted` (GEN-05). Any other unmarked output is `E8001`.
  `BuildOptions.Adopt` takes only a C++ header: a JSON output without its `$schema` is `E8001`
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
  returns a `*SyntaxError` with its findings. `Format(Format(x)) == Format(x)`.
- **T2.** `FormatJSONSource` returns the canonical source layout of a JSON file (FMT-02); key
  order is preserved. Invalid JSON returns a `*SyntaxError`.

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
| `ErrProject` | `*ProjectError` | `project.canon` has errors, bad `Roots` (O3) | 2 |
| `ErrUnsupportedVersion` | `*ProjectError` | `E1001` | 2 |
| `ErrUnknownPackage` | | a selector matches no package (R1) | 2 |
| `ErrUnknownLayer` | | a layer name matches no file (O4, LAY-01) | 2 |
| `ErrBadPath` | `*PathError` | path syntax, unsupported segment (§6) | 2 |
| `ErrNoPath` | `*PathError` | nothing at that path | 2 |
| `ErrAmbiguousPath` | `*PathError` | unqualified root matches several packages (P6) | 2 |
| `ErrNoValue` | `*PathError` | the value is poisoned (R6) | 1 |
| `ErrBadOp` | `*PathError` | op not valid for that container (E2) | 2 |
| `ErrBadValue` | `*ValueError` | value does not fit the type; bad template value (V1, N2); unknown build `Target` (B1b) | 2 |
| `ErrKeyExists` | `*PathError` | E3 | 1 |
| `ErrStableKey` | `*PathError` | E4 (`Remove`, `Rename`, `Unretire` of a stable id), E13 | 1 |
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
