# Canon implementation plan

Status: **normative for the implementation team**, for language version 0.1. This document says
how the compiler is built: the module layout, the interfaces frozen before work starts, who owns
what, the milestones and their acceptance tests, the test strategy, the behaviour of the commands
that are not language semantics, versioning, platforms and libraries.

Settled here: AUDIT ORG-01..03, NFR-01..05, CLI-01..05, the parts of DIAG-01 about the code
registry, and DECISIONS 25 (the code doctrine, §12). The choices this plan once marked
**[confirm]** are accepted (DECISIONS 24, meta/spec-phase/review/ACCEPTED-CHOICES.md).

Inputs, in order of authority: DECISIONS.md, AUDIT.md (accepted answers), the companion documents
in `spec/`, SPEC.md, CLI.md. When a companion document and this plan disagree about *behaviour*,
the companion document wins; this plan wins about *code organisation*.

---

## Contents

1. [Principles](#1-principles)
2. [Repository and module layout](#2-repository-and-module-layout)
3. [Package map](#3-package-map)
4. [Interfaces frozen first](#4-interfaces-frozen-first)
5. [Owners and order of work](#5-owners-and-order-of-work)
6. [Milestones](#6-milestones)
7. [Test strategy](#7-test-strategy)
8. [Command behaviours](#8-command-behaviours)
9. [Versioning](#9-versioning)
10. [Platforms](#10-platforms)
11. [Libraries](#11-libraries)
12. [Code doctrine](#12-code-doctrine)
13. [Open issues](#13-open-issues)

---

## 1. Principles

1. **One evaluator.** Canon expressions are evaluated only in `internal/eval`. The CLI, the
   language server, the studio (through `Evaluate`) and conformance vectors all call it.
2. **Contracts before code.** The interfaces of §4 are written, reviewed and frozen in M0. Each
   module is then built against them, with hand-built fixtures, before its producers exist.
3. **Tests are the spec made executable.** Every numbered rule in a companion document has a test
   that names it: by its rule id where the document numbers its rules (`// API.md E14`,
   `// VIEWMODEL.md G19`), else by its section (`// WIRE.md §5.4`).
4. **Deterministic by construction.** No output depends on Go map order, goroutine scheduling,
   directory listing order, the clock, the environment or the absolute checkout path (NFR-05).
5. **No cgo, no network.** `canon` is one static binary (CLI.md §1).
6. **One code doctrine.** The compiler's own Go code follows the fleet's code doctrine, enforced by
   the project's copy of the auditor and by `make check` (§12, DECISIONS 25).

---

## 2. Repository and module layout

The compiler lives in `services/configlang/` (this directory), as one Go module.

```
services/configlang/
  go.mod                      module github.com/fantasim/canonlang, go 1.25
  Makefile                    `make check` (§12.2)
  SPEC.md CLI.md DECISIONS.md AUDIT.md README.md MOCKUP-GAPS.md
  spec/                       companion documents (normative), spec/ERRORS.md the code catalogue
  tools/audit/                the code auditor, its own Go module (§12)
  .sovaudit/                  the auditor's ratchet baseline and per-repo state (§12.3)
  api/                        package canon: the public API (API.md)
  api/vm/                     view-model Go structs, generated from spec/viewmodel.schema.json
  cmd/canon/                  main package: flag parsing only, calls internal/cli
  internal/…                  every other package (§3)
  editors/vscode/             TextMate grammar and the VS Code extension (M5)
  examples/                   language examples = golden tests (own go.mod, see below)
  examples/_fixtures/         minimal copies of the files the examples load (ORG-02, §7.3)
  examples/features/          one small example per feature no other example covers (ORG-03, §7.9)
  bench/                      generated benchmark project (not committed; §7.6)
```

- **Module path.** `github.com/fantasim/canonlang` (DECISIONS 23). The repository is public, so
  every contributor runs the same gate (§12).
- **Go version.** The compiler is written for Go 1.25 (`go 1.25` in go.mod). Generated code
  targets the current Go release only (CG-10, DECISIONS 205); the two are independent.
- **Import path of the API.** The public package is `github.com/fantasim/canonlang/api`, package
  name `canon` (API.md §1.1). ORG-01 named it `api`; the directory keeps that name, the package is
  `canon` so callers write `canon.Open`.
- **examples/ is its own module.** `examples/pipeline/expected/go/*.go` are Go files; as part of the
  compiler module, a broken golden would break `go build ./...`. `examples/go.mod`
  (`module github.com/fantasim/canonlang/examples`) excludes them. The golden harness compiles
  generated Go in a temporary module instead (§7.8). It is needed from the start: the pipeline
  golden `examples/pipeline/expected/go/potions.gen.go` imports `example.com/potions/rt`, so
  `go build ./...` at the module root fails without `examples/go.mod`.
- **tools/audit/ is its own module** too (§12.1): the compiler never imports it, and its
  dependencies (golangci-lint as a tool) stay out of the compiler's `go.mod`.

---

## 3. Package map

ORG-01's table, adjusted: `source` (file set and positions) is split out so every module shares
one position type; JSON sources get their own syntax tree (`jsonsrc`) because both `load` and
`edit` need positions and lossless re-printing; `wire`, `lock`, `i18n`, `views`, `build`,
`workspace`, `convert`, `cli` and `testkit` are added (`infer` dropped, DECISIONS 188).

| Package (under `internal/` unless noted) | Owns | Implements | Consumes |
|---|---|---|---|
| `source` | file set, file ids, byte positions, line tables, display paths (`@root/...`) | API.md §1.3 | — |
| `diag` | code registry generated from `spec/ERRORS.md` (`cmd/diaggen`), finding builders, `Finding`, sorting, truncation, text and JSON rendering | DIAG-01..03, ERRORS.md, API.md §4 | source |
| `syntax` | lexer (modes, interpolation, regex rule), parser, lossless token stream with trivia, AST | GRAMMAR.md | source, diag |
| `format` | printer over the AST and trivia; comment attachment | FORMATTER.md (FMT-01) | syntax |
| `jsonsrc` | JSON reader with byte spans and RFC 6901 pointers; canonical source-JSON printer | WIRE.md (LOD-02), FORMATTER.md (FMT-02) | source, diag |
| `project` | `project.canon` schema, roots, path normalization (`E7001`), package discovery, file set scan | SPEC §3, GRM-07, GEN-04 | syntax, source |
| `types` | type representation, aliases, dependent type functions, assignability, joins | TYPES.md | syntax |
| `value` | immutable values, identity, `Prov`, equality, canonical text form | EVALUATION.md, STD-06 | types, source, diag |
| `check` | resolver + bidirectional type checker for declarations, bodies, views, translations, layers; the `Folder` interface through which it folds constants with `eval` (§4.7) | TYPES.md, GRAMMAR.md (names), RES-01..09 | syntax, types, project, value |
| `eval` | interpreter, budget, freezing / copy-on-write, poisoning, layer application; `eval/std` stdlib; the `Host` interface through which it reaches `load` and `verify` (§4.8) | EVALUATION.md, STDLIB.md | check, value |
| `wire` | wire decode (JSON, CSV cells) into values; canonical JSON encode | WIRE.md | value, types, jsonsrc |
| `load` | `load`, `load.dir`, `load.defines`, `load.csv`, `load.text`, `at:`, globs, assets listing cache | SPEC §13, LOD-01..11, TYP-21 | wire, project, value |
| `verify` | refinements, refs, keys, dependent types, assets (EVALUATION.md §1: stage B, verify) | TYPES.md (verification), EVALUATION.md §5 | eval, value, load, wire (E8102, DECISIONS 283) |
| `lock` | `canon.lock` parse, print, append, rules `E6xxx` | LOCK.md | value, project |
| `rules` | record and package checks, `fail`/`warn`, test blocks and `expect` | EVALUATION.md (checks, tests), SPEC §10, §18 | eval, verify |
| `i18n` | key catalogue, translation files, fallback, `i18n stub`/`status` | I18N.md | check |
| `api/vm` (package `vm`) | view-model Go structs, generated from `spec/viewmodel.schema.json`; `ViewModel.Decode` targets them (API.md §5.4) | VIEWMODEL.md, viewmodel.schema.json | — |
| `views` | view resolution (groups, controls, labels, `when`/`show`, usage, search index) shared by `gen/view` and `Evaluate` | VIEWMODEL.md, MOCKUP-GAPS | check, eval, i18n, api/vm |
| `ir` | target-neutral emit IR (§4.5 of this plan), fingerprint, emit validation (stage E), portable-subset check (`E9xxx`) | §4.5, FINGERPRINT.md, CODEGEN.md (what the IR must carry) | check, verify, value, types, project, wire (E8102, DECISIONS 283) |
| `conform` | conformance vector selection and expected results | CONFORMANCE.md | ir, eval |
| `gen/json` | `emit json` files | WIRE.md (emit layout) | ir, wire |
| `gen/go` | Go code, `rt` package, Go conformance tests | CODEGEN.md (Go), CONFORMANCE.md | ir, conform |
| `gen/cpp` | C++17 code, `canon_runtime.h`, legacy struct modes, C++ conformance | CODEGEN.md (C++), CONFORMANCE.md | ir, conform |
| `gen/ts` | TypeScript code, TS conformance | CODEGEN.md (TS), CONFORMANCE.md | ir, conform |
| `gen/view` | view-model JSON | VIEWMODEL.md, viewmodel.schema.json | views, ir, api/vm |
| `build` | phase and stage orchestration, build manifest, cache, atomic output writes, GENERATED markers (`E8001`), `--adopt`; implements `eval.Host` with `load` and `verify` (§4.8) | EVALUATION.md §1, SPEC §14, CLI §3.4, LOD-11, GEN-05 | all of the above |
| `edit` | paths, ops, editability, cascades, minimal writes, file placement, journal | API.md §6-§10 | format, jsonsrc, value, wire, lock, build (re-check) |
| `workspace` | snapshots, revisions, refresh, overlays, watching, incremental invalidation | API.md §3, §12 | build, edit |
| `api/` (package `canon`) | public API; thin adapter over `workspace` | API.md | workspace |
| `convert` | `canon convert` | §8.3 | edit, build, format |
| `lsp` | language server | §8.4 | workspace, check, format |
| `cli` | command implementations, text/JSON output, exit codes | CLI.md, §8 | api, internal packages for fmt, convert, i18n, explain of functions |
| `cmd/canon` (top level) | `main`: reads the working directory, wires signals, calls `cli` (standard `flag`, DECISIONS 139) | CLI.md | cli |
| `testkit` | golden harness, fixture FS, shuffling FS, benchmark generator | §7 | api |

Dependency rule: an arrow may only point up this table (a package imports packages listed above
it, except `testkit`, which may import anything). `go list -deps` is checked in CI against the
allowed graph in `internal/testkit/deps_test.go`. The Consumes column binds as well: a package
imports only the packages its row lists or packages reachable through them
(`TestConsumesColumn`, whose allowlist of known deviations only shrinks). `testkit` is exempt from
the Consumes column (its "may import anything" is the specific rule; the rank rule still binds),
and `cli`'s prose cell means every package listed above it. Where a package must call one listed
below it, it declares the interface it needs and receives an implementation at construction, from
`build` or `workspace`. There are exactly two such seams: `check.Folder` (constant folding during
checking, §4.7) and `eval.Host` (loading and verification during evaluation, §4.8).

---

## 4. Interfaces frozen first

These eight contracts are written in M0 as compiling Go, reviewed by every consumer owner, and then
frozen. A change after M0 needs a pull request from the owner, approved by the owner of every
consuming package (the "Consumes" column of §3). The sketches below fix the shape; the owner
completes them in the listed file.

### 4.1 Syntax tree — `internal/source/source.go`, `internal/syntax/ast.go`

The parser is **lossless**: the token stream plus trivia reproduces the file byte for byte
(after `\r\n` normalization). The formatter and the edit engine work from it (FMT-01, API-03).

```go
package source

type FileID uint32
type Pos int32                      // byte offset in the file's (normalized) content

type File struct {
    ID      FileID
    Path    string                  // display path: "@resource/…" or project-relative, '/'-separated
    Abs     string                  // absolute path, '/'-separated
    Content []byte
    lines   []Pos                   // start of each line
}
func (f *File) Position(p Pos) (line, col int)   // 1-based; col in UTF-8 bytes (API.md §1.3)

type Span struct { File FileID; Start, End Pos }  // End exclusive
type FileSet struct { /* append-only, safe for concurrent reads */ }
func (s *FileSet) File(id FileID) *File
```

```go
package syntax

type TokenKind uint16   // one per token of GRAMMAR.md: Ident, Int, Float, Duration, String parts, Regex, punctuation, keywords…
type Token struct {
    Kind       TokenKind
    Start, End source.Pos
    Leading    []Trivia  // whitespace, newlines and comments before the token
    Trailing   []Trivia  // comments and spaces after it on the same line
}
type TriviaKind uint8   // Space, Newline, LineComment, BlockComment, DocComment
type Trivia struct { Kind TriviaKind; Start, End source.Pos }

type Tok int32          // index into File.Tokens

// Every node records its first and last token, so its exact text and its trivia are known.
type Node interface { First() Tok; Last() Tok; node() }

type File struct {
    Src     *source.File
    Tokens  []Token
    Kind    FileKind           // Source | Layer | Translation | Project
    Doc     *DocComment        // package doc (GRM-08)
    Package *QualifiedName     // nil for project.canon
    Imports []*Import
    Decls   []Decl             // source files
    Layer   *Ident; Amends []*AmendBlock           // layer files
    Lang    *Ident; Entries []*TranslationEntry    // translation files
    Project *ProjectDecl                           // project.canon (GRM-07)
}
```

Node set (GRAMMAR.md is authoritative for fields; the owner adds nothing outside this list without
a contract change):

| Group | Nodes |
|---|---|
| declarations | `ConstDecl` `TypeDecl` `EnumDecl` `EnumMember` `RecordDecl` `FieldDecl` `VariantDecl` `VariantCase` `FnDecl` (with `Export`, `Self`) `Param` `LetDecl` `EntryDecl` `CheckDecl` `ViewDecl` `WidgetDecl` `TestDecl` `EmitDecl` `ProjectDecl` `Import` `AmendBlock` `Amendment` `TranslationEntry` |
| shared | `Ident` `QualifiedName` `DocComment` `Annotation` `AnnotationArg` `Modifiers` (`local`, `retired`) |
| view items | `ViewTitle` `ViewSubtitle` `ViewSingular` `ViewMenu` `ViewColumns` `ViewSearch` `ViewFilters` `ViewPreview` `ViewShow` `ViewGroup` `ViewField` |
| types | `NamedType` (with args: refinement or parameters, decided by `check`, GRM-05) `ListType` `MapType` `DepMapType` `TableType` `RefType` `OptionalType` `KeyedType` `WhereType` `UnionType` `LiteralType` `MatchType` `AssetType` `AnyType` `FnType` |
| expressions | `IdentExpr` `IntLit` `FloatLit` `StringLit` (parts: text, `Interp{Expr, Spec}`) `RawStringLit` `DurationLit` `RegexLit` `BoolLit` `NoneLit` `SelfExpr` `ItExpr` `ParenExpr` `UnaryExpr` `BinaryExpr` `IsExpr` `RangeExpr` `SelectorExpr` (with `Optional` for `?.`) `IndexExpr` `CallExpr` `Arg` `ForceExpr` (postfix `!`, DECISIONS 17) `LambdaExpr` `ShorthandLambda` `ListLit` `ListComp` `BraceLit` (items: `FieldItem` `MapItem` `EntryItem` `SpreadItem`; classified by `check`, GRM-10) `MapComp` `CompClause` `TypedLit` `IfExpr` `MatchExpr` `MatchArm` `Pattern` `LoadExpr` |
| statements | `Block` `LetStmt` `VarStmt` `AssignStmt` `IfStmt` `ForStmt` `WhileStmt` `BreakStmt` `ContinueStmt` `ReturnStmt` `ExpectStmt` `MatchStmt` `ExprStmt` |

`Decl`, `Expr`, `Type`, `Stmt` and `ViewItem` are interfaces over these nodes. The parser also
returns its findings (`E11xx`) and recovers at declaration and item boundaries so the language
server gets a tree for broken files.

### 4.2 Types — `internal/types/types.go`

```go
package types

type Kind uint8   // one per kind of TYPES.md §2, which is authoritative
const (
    Bool Kind = iota; Int; Float; String; Duration
    Enum; Record; Variant; Case
    VariantKind                     // TYPES.md `Kind`: the type of `v.kind`
    Optional; List; Map; DepMap; Table; Ref
    LitUnion; Never; Range; Func; Pair
    TypeApp                         // a dependent type `Param(eventType)`, or a record applied to arguments
    DepUnion                        // the static view of a dependent value, printed `Param(*)`
    Define                          // the element record of `load.defines` tables
    Any; None; Error
)

type Type interface {
    Kind() Kind
    String() string      // canonical type text, qualified names (API.md TypeInfo.Expr)
    Underlying() Type    // aliases expanded; refinements kept
    Base() Type          // refinements, `where` and aliases removed (TYP-04, VIEW-07)
}

type Basic struct { K Kind; Bits int; Signed bool }   // Int8..UInt64 are Int + bits (TYP-03); Float32 is Float + 32
type Refined struct {                                   // not a kind: Kind() is Of's (TYPES.md §2, refinements)
    Of Type; Ranges []Bound; Pattern *regexp.Regexp; Where *Predicate
    Asset *AssetSpec                                    // asset(root, ext: […]): a String refinement (TYPES.md §13.4)
}                                                       // checked at storage points (EVALUATION.md §4.3)
type Alias struct { Pkg, Name string; Params []*Param; Def Type; Decl *syntax.TypeDecl }

type EnumType struct { Pkg, Name string; Ordered bool; Codes *Basic; Members []*Member; Decl *syntax.EnumDecl }
type Member struct { Name, Wire string; Index int; Code int64; HasCode, Retired bool; Doc string }

type RecordType struct { Pkg, Name string; Params []*Param; Fields []*Field; Methods []*Method; Checks []*syntax.CheckDecl; JSONCase string; Decl *syntax.RecordDecl }
type Field struct {
    Name, Wire string; WirePath []string; Index int
    Type Type; Default syntax.Expr; HasDefault bool; DependsOn []int  // earlier fields its type uses (TYPES.md §11)
    Input *Input; Deprecated string; Stable bool; Unit string; Inline bool; NoneWire []byte
    Doc string; Annotations []*syntax.Annotation
}
type VariantType struct { Pkg, Name, Tag string; Cases []*CaseType; Decl *syntax.VariantDecl }
type CaseType struct { Variant *VariantType; Name, Wire string; Index int; Fields []*Field; Retired bool }

type ListType struct { Elem Type; KeyedBy *Field }   // TYPES.md §2: a keyed list is a List with KeyedBy != nil
type MapType struct { Key, Value Type }
type DepMapType struct { Coll *Collection; Param string; Value *TypeFunc }
type TableType struct { Elem Type; Stable bool }
type RefType struct { Target *Collection }
type OptionalType struct { Elem Type }
type LitUnionType struct { Of Type; Literals []string }       // TYP-09; `Base` would clash with the method (DECISIONS 77)
type AssetSpec struct { Root string; Exts []string }
type FuncType struct { Params []Type; Result Type }             // GRM-15
type PairType struct { A, B Type }                              // TYPES.md §12.5
type VariantKindType struct { Variant *VariantType }            // an enum whose members are the cases (TYPES.md §8.3)
type TypeAppType struct { Fn *TypeFunc; Rec *RecordType; Args []syntax.Expr } // erased statically (TYP-18)
type DepUnionType struct { Fn *TypeFunc }
type TypeFunc struct { Pkg, Name string; Params []*Param; Match *syntax.MatchType }

var (                                                           // singletons
    DefineType *RecordType                                      // `record Define { value: Int }`, kind Define
    AnyType, NoneType, ErrorType, NeverType, RangeType Type
)

// A collection a ref can target (RES-03).
type Collection struct {
    Kind      CollKind   // Let | EnclosingField | Defines
    Pkg, Name string     // the let
    FieldPath []string   // EnclosingField: path from the enclosing record
    Elem      Type
}

func Identical(a, b Type) bool
func Assignable(from, to Type) bool     // TYPES.md judgments
func Join(a, b Type) (Type, bool)       // TYP-05
```

### 4.3 Values and provenance — `internal/value/value.go`

```go
package value

type Value interface {
    Type() types.Type
    Prov() *Prov
    CanonText() string   // canonical text form (STD-06); satisfies diag.ValueArg (ERRORS.md §1.3)
}

type (
    Bool   struct { V bool; P *Prov }
    Int    struct { V int64; P *Prov; T types.Type }        // every integer type (TYP-03)
    Float  struct { V float64; P *Prov; T types.Type }
    Str    struct { V string; P *Prov; T types.Type }       // String, asset, literal-union literal
    Dur    struct { Ms int64; P *Prov }
    Member struct { Enum *types.EnumType; Index int; P *Prov }
    None   struct { T types.Type; P *Prov }
    Record struct {                                          // records and variant cases
        T      types.Type        // *RecordType, *CaseType or *TypeAppType
        Fields []Value           // declaration order; absent optional fields hold None
        Set    []bool            // field present in the source (VM-06 usage, API-06)
        Ident  *Identity         // non-nil for table entries and keyed-list elements (TYP-02)
        P      *Prov
    }
    List   struct { T types.Type; Elems []Value; P *Prov }   // plain and keyed lists
    Map    struct { T types.Type; Keys, Vals []Value; P *Prov }  // insertion order
    Table  struct { T *types.TableType; Entries []*Record; P *Prov }
    Ref    struct { T *types.RefType; Key Key; P *Prov }
    Range  struct { Start, End int64; HasEnd bool; P *Prov } // TYP-12
)

type Key struct { S string; I int64; IsInt bool }
type Identity struct { Coll *types.Collection; Key Key; Retired bool }

type ProvKind uint8   // Literal, JSON, CSV, Defines, Text, Default, Spread, Computed, Layer (API.md Origin)
type Prov struct {
    Kind    ProvKind
    Span    source.Span   // literal node, JSON value's first byte..end, default expr, building expr, amendment
    Pointer string        // RFC 6901 for loaded JSON
    Layer   string
    Via     *Prov         // Default: the literal/object that omitted the field; Spread: original
    Stack   []diag.Frame  // Computed: ≤ 16 frames (EVL-07)
}

func Equal(a, b Value) bool             // TYPES.md §7.5 equality (TYP-08)
func Text(v Value) string               // v.CanonText()
```

Values are immutable once they leave the call that built them (freezing, EVL-04). The
evaluator's copy-on-write buffers are internal to `eval`.

For the edit engine, `Prov.Span` of a `Literal` or `JSON` value is exactly the span of the CST
node that states it; `edit` maps the span back to the node.

### 4.4 Findings and the code registry — `internal/diag/diag.go`, `internal/diag/codes.go`

[ERRORS.md](ERRORS.md) is the single source of every diagnostic (DECISIONS 27): code, severity,
owning package, owning document, meaning, and each message's template with its typed arguments.
`internal/diag/codes.go` is **generated** from it by `go run ./internal/diag/cmd/diaggen` and is
never edited by hand. ERRORS.md §2 is the contract of the generator (what it reads, what it
refuses, what it writes) and of the call convention; this section only places it in the module.

```go
package diag

type Severity uint8   // Error, Warning, Runtime
type Code string      // "E3501"

// Generated (ERRORS.md §2.2): the registry, one type and variable per code, the Kind enum.
type Def struct { Code Code; Severity Severity; Package string; Variants []Variant }
var Registry = []Def{ /* sorted by Code */ }
var E3501 codeE3501   // func (codeE3501) At(span source.Span, key ValueArg, coll string) *Builder

// Hand-written.
type Builder struct { /* a finding under construction: Path, Pointer, Related, Check, Layer, Stack, Report, Message */ }
func (b *Builder) Detached() *Builder   // a copy holding every argument as the text it renders, so a memoized
                                        // finding keeps no value or type alive (addition, DECISIONS 250)
func (f Finding) Restated(files Files, args ...any) Finding // f with its own template re-rendered from args (DECISIONS 281)
type Finding struct {
    Code     Code
    Severity Severity
    Span     source.Span
    Pointer  string
    Package  string
    Path     string        // canonical value path (API.md F1)
    Message  string        // rendered from the template (ERRORS.md §1.2)
    Check    string
    Layer    string
    Related  []Related
    Stack    []Frame       // innermost first, ≤ 16
    MoreFrames int         // frames cut from Stack (API.md F13, DECISIONS 82)
    Reads    []string      // VIEWMODEL.md J15
}
type Related struct { Span source.Span; Note string }   // notes: ERRORS.md §1.5
type Frame struct { Fn string; Span source.Span }       // also used by value.Prov

// Resolves spans for sorting and rendering; *source.FileSet implements it (DECISIONS 81).
type Files interface {
    Path(id source.FileID) string                            // "" is no location
    Position(id source.FileID, p source.Pos) (line, col int)
    Content(id source.FileID) []byte
}
type Bag struct { /* collects findings concurrently; Findings() is sorted (API.md F2), deduplicated and truncated (F7) (DECISIONS 83), whatever the order of the reports (DECISIONS 105) */ }
func NewBag(files Files, pkg string) *Bag
func Render(w io.Writer, files Files, findings []Finding, opt RenderOptions) error   // text (DIAG-03) or JSON (API.md F5); the error wraps ErrWrite

// A finding with every span resolved (API.md §1.3): what Write renders, and what the API
// converts to and from canon.Finding (DECISIONS 106).
type Located struct {
    Code Code; Severity Severity; Loc source.Location
    Pointer, Package, Path, Message, Check, Layer string
    Related []RelatedLoc; Stack []FrameLoc; MoreFrames int; Reads []string
}
func Locate(files Files, findings []Finding) []Located
func Write(w io.Writer, findings []Located, opt RenderOptions) error   // Render over resolved findings
func (l *Located) AppendJSON(buf []byte) []byte                       // one F5 object, for canon.Finding.MarshalJSON
```

- **One way to report.** A package reports a finding only as
  `diag.E3501.At(span, key, coll).Related(…).Report(bag)` (ERRORS.md §2.3). No other file writes
  a code as a literal or holds message text (audit rule `diag-message-inline`, addendum), and
  no code builds a `Finding` by hand. The `Package` column of ERRORS.md names the package that
  owns each code; a code's number range does not.
- **Adding or changing a code.** Edit the owning document (when it fires) and ERRORS.md (its
  row and messages) in one change, regenerate `codes.go`, add the test that produces it. QA
  (the owner of `diag`) reviews every change to ERRORS.md.
- **Gate.** `make check` regenerates `codes.go` into a temporary file and fails if it differs
  from the committed one (target `diag-check`, §12.2); the generator also checks the runtime
  helper texts against ERRORS.md §1.6.
- The audit rule `diag-code-untested` fails when a code the compiler reports has no test case
  that expects it (§7.2, DECISIONS 55).
- **Order.** `Findings()` and `Write` sort in a total order: the F2 key, then every other field
  (severity, end position, package, path, pointer, check, layer, related, stack, cut frames,
  reads). Of duplicates (EVALUATION.md §14), the least in that order is kept, so the result
  never depends on which goroutine reported first (NFR-05, DECISIONS 105).

The public `canon.Finding` (API.md §4.1) is converted from `diag.Finding` at the API boundary
through `Locate`, and back to a `Located` when the API writes findings (`canon.WriteFindings`,
`Finding.MarshalJSON`), so every text and JSON form of a finding has one writer, in `diag`.

### 4.5 Emit IR — `internal/ir/ir.go`

```go
package ir

type Package struct {
    Name     string          // "pipeline"
    Dir      string          // display path of the package directory
    Doc      string
    Imports  []*PackageRef   // packages whose types or values are referenced, with their emits per target; one a ts emit reaches only through other packages' types carries its ts emits only (DECISIONS 279)
    Types    []Type          // every public type, in declaration order (files in path order)
    Consts   []*Const
    Values   []*Value        // public values selected by the emit's `values`
    Fns      []*ExportFn     // package-level export fns, @text fns excluded
    TextFns  []*ExportFn     // @text fns, written only by emit text (additive, DECISIONS 300)
    Emits    []*Emit
}

type Type interface{ QName() string }   // *Record, *Enum, *Variant, *Dependent; QName() is Pkg + "." + Name (DECISIONS 80)
type Record struct {
    Pkg, Name, Doc string
    Fields  []*Field
    Methods []*ExportFn
    Cpp     CppOptions       // struct, header, access, field/type/name per field (CODEGEN.md §7.8, CPP-01)
    Go, TS  NameOptions
    Fingerprint string       // "pkg.Type@xxxxxxxx" when the record is a root of an emitted value
}
type Field struct {
    Name, Wire string; WirePath []string; Doc string
    Type TypeRef; Optional bool; NoneWire []byte; Unit string; Inline bool
    Pairs *Pairs             // @json(pairs:): slot key templates and slot count (WIRE.md §5.14)
    Default value.Value      // nil when required
    Deprecated string; Stable bool; Input *Input
    Cpp CppFieldOptions; Go, TS NameOptions
}
type TypeRef struct {
    Kind    types.Kind
    Named   string           // qualified name for Enum/Record/Variant/Dependent
    Elem, Key *TypeRef
    Bits    int; Signed bool
    KeyedBy string
    Ref     *RefTarget       // collection, same-value or by-key (CG-03), define value
}
type Pairs struct { Keys [2]string; Slots int }
type Enum struct { Pkg, Name, Doc string; Members []EnumMember; Codes int; CppDefines string; JSONCodes bool }
type EnumMember struct { Name, Wire, Doc string; Index int; Code int64; Retired bool }
type Variant struct { Pkg, Name, Doc, Tag string; Cases []*Case }
type Case struct { Name, Wire, Doc string; Fields []*Field; Retired bool }
type Dependent struct { Pkg, Name string; BranchEnum string; BranchNames []string; Branches []Branch } // "<Alias>Branch" (CODEGEN.md §5.6)

type Value struct {
    Name, Doc string
    Type   TypeRef
    V      value.Value       // fully evaluated and verified
    Reload bool
    IDs    []string          // table ids in entry order, for id enums (EMT-04)
}
type ExportFn struct {
    Name string; Receiver *Record; Params []Param; Result TypeRef
    Kind    FnKind           // Precomputed | Lookup | Translated (SPEC §9.4)
    Results []value.Value    // Precomputed: one per receiver value, in value order
    Table   *LookupTable     // Lookup: dense, in CODEGEN.md §5.10 domain order (CG-08)
    Body    PExpr            // Translated: portable-subset expression tree, typed
    Vectors []Vector         // Translated: from internal/conform
}
type PExpr interface{ pexpr() }   // Lit, Param, SelfField, Unary, Binary, Call(builtin), CallExport, If, Let, Template, Coalesce, EnumMember
type Emit struct {
    Target Target; Out string              // Out as written, for messages only
    Dir      string                        // output directory (a ts emit's: its file's), project-relative through the declared roots
    FileName string                        // ts: the file name in Dir
    GoImport string                        // go: the import path of Dir (CODEGEN.md §2.8)
    Mode Mode; Values []string; GoPackage, Namespace string   // typed options (DECISIONS 80)
}
```

Each code generator is a pure function `func Generate(p *ir.Package, e *ir.Emit) ([]File, error)`
with `type File struct { Path string; Content []byte }`, `Path` relative to the emit's `Dir`.
`Package.Emits` and `PackageRef.Emits` may hold several emits of one target, the copies of an
`out` list (CODEGEN.md §2.1), and a generator is called with `ir.CopyOf(proj, p, e)`, the package
as copy `e` sees it, each import's emits narrowed to the copy it uses (CODEGEN.md §2.8): a caller
precondition, not a type change, added under §4's review rule (DECISIONS 270).
Codegen agents start from hand-built `ir.Package` fixtures in `internal/gen/<t>/testdata/`,
before the front end exists. **Generators never resolve roots** (DECISIONS 108): stage E fills
every emit's `Dir`, `FileName` and `GoImport`, those of the imported packages' emits
(`PackageRef.Emits`) included, from `project.canon`'s declared roots and `go_module`, never from
a `--root` override, so a Go import (`<GoImport>/rt`, an imported package's `GoImport`) is read
off the IR and a C++ or TS relative include is the path from one emit's `Dir` to another's.

### 4.6 Edit paths — `internal/edit/path.go`

```go
package edit

type SegKind uint8  // Field (".f"), Key ("[k]"), Pos ("[#n]")
type KeyLit struct { Kind KeyLitKind; Text string; Int int64; Raw string }  // Word | Int | String (API.md §6.1); Raw as written (DECISIONS 87)
type Seg struct { Kind SegKind; Name string; Key KeyLit; Pos int }
type Path struct { Package string; Root string; Segs []Seg }

func Parse(s string) (Path, error)              // API.md §6.1; an error is a *SyntaxError
type SyntaxError struct { Offset int; Reason error }   // wraps edit.ErrBadPath (the API's canon.ErrBadPath) and Reason
func (p Path) String() string                   // as parsed

type Step struct { Seg Seg; Container types.Type; Value value.Value }
type Resolved struct { Canonical string; Steps []Step; Target value.Value }
func Resolve(s *Snapshot, p Path) (Resolved, error)  // API.md §6.2-§6.5; with Snapshot, at M4
```

`Step` and `Resolved` are declared now. `Snapshot` is `edit`'s own concrete type, not an
interface: the part of one build's results (the checked program and its evaluated values) that
`Resolve` reads, which `workspace` assembles from `build`'s results and hands to `edit`. `build`
sits above `edit` in §3, so no third injection seam exists (DECISIONS 107). Its fields follow
`build`'s result type, so `Snapshot` and `Resolve` are written with that type, not before.

### 4.7 The checked program — `internal/check/info.go`

What the type checker hands to every later package (`eval`, `views`, `i18n`, `ir`, `conform`,
`lsp`, `edit`). It is modelled on `go/types.Info`: the checker records every conclusion once, and
no consumer resolves a name, types an expression or classifies a literal again.

```go
package check

// Check runs phase 2 (EVALUATION.md §1, TYPES.md §1) over the parsed files of the loaded
// packages. Static findings go to the package bags; the result is immutable and safe for
// concurrent reads.
func Check(ctx context.Context, proj *project.Project, files []*syntax.File, bags Bags, fold Folder) *Program

type Bags map[string]*diag.Bag   // one bag per loaded package, by path; a map, not a third seam (DECISIONS 34)

// Folder evaluates a constant expression (TYPES.md §15) once check has typed it: refinement
// bounds, parameter defaults, codes and member values, `const` initializers. `eval` implements
// it (eval.NewFolder), so constants are computed by the one evaluator (principle 1). owner is
// the declaration e belongs to: its package's bag takes the findings, its file locates them.
type Folder interface {
    Fold(ctx context.Context, owner Object, e syntax.Expr, info *Info) (value.Value, bool)
}

type Program struct {
    Packages []*Package   // dependencies first, ties by qualified name (TYPES.md §3.1)
    Info     *Info
}

type Package struct {
    Path    string          // "resource.farm"
    Files   []*syntax.File  // source, layer and translation files, in path order
    Decls   []Object        // top-level declarations in (file path, position) order (EVALUATION.md §2.1)
    Imports []*Package
    Layers  map[string][]*syntax.AmendBlock   // by layer name (EVALUATION.md §9)
}

type Info struct {
    Types      map[syntax.Expr]types.Type           // every expression, after narrowing (TYPES.md §6.6), before Conv
    TypeExprs  map[syntax.Type]types.Type           // every written type, resolved; a ref carries its *types.Collection (TYPES.md §10.2)
    Defs       map[*syntax.Ident]Object             // each declaring identifier
    Uses       map[*syntax.IdentExpr]Object         // each name in value position, contextual names included (TYPES.md §4)
    NameUses   map[*syntax.Ident]Object             // each other identifier naming an object: type names, qualified-name parts,
                                                    // patterns, `.name`, literal fields, named arguments, amend segments, imports…
    Selections map[*syntax.SelectorExpr]*Selection  // each `.name` on a value (TYPES.md §3.5); a qualified form (§4.3) has none
    Conv       map[syntax.Expr]*Conversion          // the implicit conversion at a use (TYPES.md §6.2); absent: none
    Keys       map[syntax.Expr]*types.Collection    // symbolic keys and key literals, checked at evaluation (TYPES.md §1, §4.1)
    Symbols    map[*syntax.IdentExpr]bool           // identifiers kept as symbols: a dependent value's (§11.4), a load format (WIRE.md §6.1)
    Calls      map[*syntax.CallExpr]*Callee         // the resolved callee of each call
    Literals   map[*syntax.BraceLit]LitKind         // brace-literal classification (TYPES.md §5.2)
    Matches    map[syntax.Node]*MatchInfo           // each `match` expression or statement (TYPES.md §12.6)
    Broken     map[Object]bool                      // declarations with a static error, or naming one (TYPES.md §1)
    BrokenViews map[*syntax.ViewDecl]bool           // views holding an error (VIEWMODEL.md J4; ADR-0009, DECISIONS 228)
    BrokenTranslations map[*syntax.TranslationEntry]bool // translation entries holding an error (I18N T2; ADR-0009)
}

func (i *Info) ObjectOf(n syntax.Node) Object   // an *Ident in Defs or NameUses, an *IdentExpr in Uses; else nil
func ViewBroken(info *Info, d *syntax.ViewDecl) bool   // BrokenViews[d], or a parser recovery node in d (J4)

type ObjKind uint8   // Const Let Fn Method Param Local Field Member Case Entry Builtin TypeName Package Layer Check Test Widget
type Object interface {   // implementations are pointers, one per declaration, so == is identity
    Kind() ObjKind
    Name() string
    Pkg() string          // the declaring package; "" for built-ins
    Type() types.Type     // declared or inferred; types.ErrorType when broken; nil for a package, layer, check or test
    Decl() syntax.Node    // the declaring node; nil for built-ins
    File() *syntax.File   // the file holding Decl; nil for built-ins
}

type SelKind uint8   // Field, Entry (a table or keyed-list key), BuiltinMember (id, retired, kind, code…), Method
type Selection struct { Kind SelKind; Obj Object; Recv types.Type; Deref bool }  // Deref: through a ref;
                     // Obj nil: an entry of a collection with dynamic keys, looked up by name at evaluation

type ConvKind uint8  // Wrap (T to T?), Deref (ref T to T), EntryToRef (T to ref T; E3503 deferred),
                     // IntLitToFloat, CaseToVariant, ToList (table T or keyed list to [T], identities kept),
                     // Elements (lists, maps, pairs: Key and Inner per element),
                     // Present (S? to T?: Inner applied to a present value)
type Conversion struct { Kind ConvKind; From, To types.Type; Inner, Key *Conversion }  // Key: map keys, a pair's first

type CalleeKind uint8   // Fn, Method, Builtin (STDLIB.md), Convert (`Int(f)`, STDLIB.md §2.1), Lambda
type Callee struct {
    Kind     CalleeKind
    Obj      Object        // Fn, Method
    Builtin  string        // "filter", "min"…
    Overload int           // the row of the STDLIB.md signature table that matched
    TypeArgs []types.Type  // bound type parameters (TYPES.md §12.2)
}

type LitKind uint8   // RecordLit, TableLit, MapLit, MapComp, ErrorLit
type MatchInfo struct {
    Scrutinee   types.Type
    Covers      [][]int   // per arm: the member or case indexes it covers (declaration order; Bool: 0 false,
                          // 1 true), NoneIndex for `none`
    Exhaustive  bool
    Unreachable []int     // arms reported W3601 or E3602
}
const NoneIndex = -1
```

- Every map is complete for the nodes of every declaration that is not broken: a consumer that
  finds no entry has found a checker bug. Maps are looked up, never iterated for output
  (`maprange`, §7.5). Every identifier is in `Defs`, `NameUses`, `Uses`, `Keys` or `Symbols`,
  unless it names no object (an annotation, a project key, an emit target, an option or a
  built-in's parameter, an `expect` outcome or code); `emit … values:` names and the `{f}`
  variables of `@files` templates are in `Uses` (the `.g` of `{f.g}` in `NameUses`; a `g` that
  several cases of a variant `f` declare is an occurrence of each, marked ambiguous, as is an
  amend segment through such a field; a field typed by one case is not a variant and is never
  ambiguous; the names after an ambiguous segment are resolved in every candidate's type, plain when
  they name one declaration across them, else ambiguous under each) (DECISIONS 275). `Occurrence` is `{File, Span, Kind, Site,
  Ambiguous}`. `Program.Occurrences(Object)`
  lists every recorded occurrence of an object, built lazily once (additive, DECISIONS 275).
- A recovery node (`BadExpr`, `BadType`, `BadStmt`, `BadDecl`, GRAMMAR.md §10) makes the
  declaration holding it broken; a `BadExpr` or `BadType` is typed `types.ErrorType` (TYPES.md
  §1) and nothing else is recorded for any of them; a top-level `BadDecl` declares nothing.
- Constants needed during checking (refinement bounds, `..=FARM_MAX_MODELS`, TYPES.md §1 step 2
  and §15) are folded through `Folder` as soon as their expression is typed; a cycle between
  constants is `E4301`. `build` passes `eval.NewFolder(bags, opt)`, and its evaluator then spends
  that folder's counter (`UseFolder`, §4.8: one counter per invocation, DECISIONS 104); tests of
  `check` pass a fixture folder that knows only literals.
- `Check` and `Bags` land with the checker in M1; the rest is written in M0 (DECISIONS 101).

### 4.8 The evaluator's host — `internal/eval/host.go`

`eval` sits above `load` and `verify` in §3 and may not import them, yet the evaluator forces
`load` expressions (EVALUATION.md §1, §3.1) and verifies at once a value first forced after
stage B has started (EVALUATION.md §1). It declares what it needs; `build` implements it with
`load` and `verify` and injects it at construction.

```go
package eval

type Host interface {
    // Load forces one load expression against its expected type (WIRE.md §6). Read and decode
    // findings go to the bags; false means the value is poisoned (EVALUATION.md §7).
    Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool)
    // Verify runs stage B (EVALUATION.md §5) on a top-level value; false means it is invalid.
    Verify(ctx context.Context, root Root, v value.Value) bool
}

type Root struct { Pkg, Name string }   // a top-level value (EVALUATION.md §2.1)

type Options struct {
    Budget int64      // project.budget; 0: the default, 10^8 steps (EVALUATION.md §12.2)
    Layers []string   // the active layers, in stack order (EVALUATION.md §9.1)
}

func NewFolder(bags check.Bags, opt Options) check.Folder                   // constants only (§4.7)
func New(prog *check.Program, host Host, bags check.Bags, opt Options) *Evaluator
func (e *Evaluator) Force(ctx context.Context, root Root) (value.Value, bool)   // stage A for one root
func (e *Evaluator) BeginVerification(ctx context.Context)   // stage B: host.Verify on each value forced so far, in the
                                                             // order its evaluation completed; from now on a first Force verifies too
func (e *Evaluator) MarkInvalid(v value.Value)               // verify's soft findings (EVALUATION.md §7.3, DECISIONS 79)
func (e *Evaluator) Invalid(v value.Value) bool              // marked by a conversion or by verify; rules skips checks above it
func (e *Evaluator) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool)

// Additions (DECISIONS 250), each additive: New and NewFolder are unchanged.
func (e *Evaluator) UseFolder(f check.Folder) bool   // e spends NewFolder's counter f: one per invocation (DECISIONS 104);
                                                     // false, a no-op, once e has evaluated anything or for another Folder
type Folds struct { /* the folds one NewFolder folder made up to a point, in order */ }
func FoldsOf(f check.Folder) Folds                   // the zero Folds for another Folder
func (fs Folds) Replay(ctx context.Context, bags check.Bags) check.Folder   // a folder on a counter of its own that made
                                                     // fs's folds again, each seeing what it first saw (EVALUATION.md §12.2)
func (fs Folds) Reads() []Root                       // every constant fs's folds read, once each, in fold order:
                                                     // stage A's item 3 (EVALUATION.md §2.1; DECISIONS 271)
type LoadMemo interface {                            // optional capability of a Host whose loads the memo replays (§7.6)
    LoadRecorded(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool, LoadInputs)
    LoadReplay(ctx context.Context, e *syntax.LoadExpr, in LoadInputs) (done func(), ok bool)
}
type LoadInputs any                                  // what a LoadMemo recorded of one load
```

- `build` wires the seams at run time, never at import time: `prog := check.Check(…,
  eval.NewFolder(bags, opt))`; then it creates its host, `ev := eval.New(prog, host, bags,
  opt)`, and `host.verifier = verify.New(ev, loader, …)`; `verify` imports `eval` (it is below
  it) to force the targets of refs and to mark invalid values.
- The evaluator returns no Go error for a language failure: those are findings. `ctx`
  cancellation stops it, and the caller reports the interrupt (§8.5).
- Tests of `eval` use a fixture host (`internal/eval/testdata/`) that serves values from txtar
  archives, before `load` and `verify` exist (§5.3).
- `Host` and `Root` are written in M0; `Options`, `NewFolder`, `New` and the `Evaluator` land
  with the evaluator in M1 (DECISIONS 101).

---

## 5. Owners and order of work

### 5.1 Agents

One owner agent per package. An owner writes the package, its tests and its goldens, and reviews
every change to its frozen contract.

| Agent | Packages | Companion documents implemented |
|---|---|---|
| **SYN** | `source`, `syntax`, `format`, `jsonsrc` | GRAMMAR.md, FORMATTER.md |
| **TYP** | `types`, `check` | TYPES.md |
| **EVL** | `value`, `eval` (+ `eval/std`), `conform` | EVALUATION.md, STDLIB.md, CONFORMANCE.md (vectors) |
| **VER** | `verify`, `rules`, `lock` | TYPES.md (verification), EVALUATION.md (checks, tests), LOCK.md |
| **LOD** | `project`, `wire`, `load` | WIRE.md, SPEC §3, §13 |
| **IR** | `ir`, `gen/json`, `build` | §4.5 (the IR), FINGERPRINT.md, CODEGEN.md §2 (files, headers), WIRE.md (emit layout) |
| **GO** | `gen/go` | CODEGEN.md (Go), CONFORMANCE.md (Go) |
| **CPP** | `gen/cpp` | CODEGEN.md (C++), CONFORMANCE.md (C++) |
| **TS** | `gen/ts` | CODEGEN.md (TS), CONFORMANCE.md (TS) |
| **VM** | `views`, `gen/view`, `i18n`, `api/vm` | VIEWMODEL.md, viewmodel.schema.json, I18N.md |
| **API** | `edit`, `workspace`, `api`, `cli`, `cmd/canon` | API.md, CLI.md, §8.1 of this plan |
| **LSP** | `lsp`, `editors/vscode` | §8.4 |
| **MIG** | `convert` | §8.3, §8.5 |
| **QA** | `diag`, `testkit`, `examples/_fixtures`, `examples/features`, CI, benchmarks, `make check`, `tools/audit` baseline | §7, §12, DIAG-01 |

### 5.2 Order of work

Phases run in order; agents inside a phase run in parallel. A phase ends when its milestone's
acceptance tests pass.

| Phase | SYN | TYP | EVL | VER | LOD | IR | GO / CPP / TS | VM | API | LSP / MIG | QA |
|---|---|---|---|---|---|---|---|---|---|---|---|
| **M0** contracts | `source`, `ast.go` | `types.go`, `check/info.go` | `value.go`, `eval/host.go` | lock format types | `project` schema types | `ir.go` | review `ir.go`; IR fixtures | review | `api/canon.go` (done), `edit/path.go` | — | `diag`, `diaggen` and `codes.go` generated from ERRORS.md, harness skeleton, CI skeleton, fixture extractor |
| **M1** taxonomy | parser (all examples), AST goldens | checker | eval + std subset, layers (DECISIONS 187, 196) | verify, lock, rules | `project`, `wire` encode | `ir`, `gen/json`, `build` | GO: baked; CPP/TS: baked from IR fixtures | — | `cli` check/build/version/init/new | — | fixtures, findings files, determinism job |
| **M1.5** generated programs | — | — | — | — | — | — | — | — | — | — | program generator and the four suites of §7.7 (beside M2) |
| **M2** pipeline | `jsonsrc` | export fns | `conform` | `rules` tests | `wire` decode, `load.dir` | fingerprint, reload IR, portable subset check | GO/CPP: data mode, stores, conformance | — | `cli` test | — | toolchain CI jobs |
| **M3** load + view | `format` (start) | dependent types, views, i18n, layers | — | dependent verification, assets | `load` (all forms) | `types` mode | CPP: `types` mode | `views`, `gen/view`, `i18n`, `api/vm` | `Check`/`Value`/`ViewModel` in `api` | — | real-data job, benchmark generator |
| **M4** fmt + edit | `format` done, JSON source printer | `check.Session` (incremental re-check) | incremental memo | — | — | — | — | `Evaluate` support | `edit`, `workspace`, full `api`, fmt/explain/refs/watch | — | edit goldens, fuzzing, perf gates |
| **M5** LSP + agent CLI | recovery hardening | position queries; Canon-name rename | — | — | — | — | — | — | `cli` edit/rename | LSP: server, grammar, extension | LSP transcripts, edit/rename goldens |
| **M6** legacy C++ + TS | — | — | — | — | — | legacy IR | CPP: `fields`/`both`/`getters`; TS: data, then complete | — | — | — | feature examples |
| **M7** migration | — | — | — | — | — | — | — | `i18n stub`/`status` | `cli` wiring | MIG: `convert` | real-data runs |

M5, M6 and M7 run in parallel once M4 is accepted. M6's C++ work may start after M3. M1.5 runs
beside M2 (DECISIONS 206).

### 5.3 Rules for parallel agents

- An agent edits only its packages. A needed change elsewhere is an issue to the owner.
- Tests that need another module's output use fixtures (`testdata/`), never the other module's
  unfinished code.
- A frozen contract (§4, `api/canon.go`) changes only through §4's review rule.
- A new or changed diagnostic is a change to `spec/ERRORS.md` (and to its owning document),
  reviewed by QA; `codes.go` is regenerated, never edited (§4.4).

---

## 6. Milestones

Each milestone lists its scope and acceptance tests. "Golden" means byte-for-byte comparison
(§7.1). All milestones also require: `make check` green (§12.2: gofmt, vet, tests, goldens, the
code auditor with its ratchet, `maprange` included), `go test -race ./...` green, the
determinism job green (§7.5), and every new registry code tested (§7.2).

### M0 — Contracts

- Scope: the module skeleton of §2-§3, following the code doctrine (§12: a `doc.go` and an example
  test per package, `constants.go` and `errors.go` where needed); the eight contracts of §4 as
  compiling Go; `api/canon.go` split by concern (§12.4); `diaggen` and the `codes.go` it generates
  from `spec/ERRORS.md` (ERRORS.md §2), with the `diag-check` gate; the golden harness; `make
  check` and CI running it; the audit baseline in `.sovaudit/` (§12.3).
- Accepted when: the contracts compile, each is approved by its consumers, `internal/diag/codes.go`
  regenerated from `spec/ERRORS.md` is unchanged (`diag-check`), every message of ERRORS.md renders
  from its constructor in a table test, `make check` passes, and the harness runs one trivial
  golden.

### M1 — v0: `taxonomy.canon` to baked Go and JSON

- Scope: full parser; resolver and type checker; evaluator with the stdlib subset used by
  `teamboard`, `sovcommon/ui`, `sovcommon/roles`; verification; stable tables and `canon.lock`;
  record and package checks; IR; `gen/json`; `gen/go` in `baked` mode; `canon check`, `build`,
  `lock check`, `version`, `init`, `new`.
- Accepted when:
  1. Every `.canon` file under `examples/` parses with no `E11xx` finding, and its AST dump equals
     `internal/syntax/testdata/ast/<example path>.txt`.
  2. `canon check teamboard` (fixtures, §7.3) exits 0 and prints exactly
     `examples/teamboard/expected/findings.txt`.
  3. `canon build --target go --target json teamboard sovcommon...` writes outputs equal to their
     goldens: Go for `teamboard`, `ui`, `roles`, and JSON for the taxonomy values (taxonomy.canon's
     `emit json`). The `emit ts` of these packages waits for `gen/ts` (M6). This milestone adds
     `examples/teamboard/expected/MANIFEST` and the output goldens it lists (§7.1), including
     `teamboard/canon.lock canon.lock`.
  4. The generated Go compiles with the current Go (DECISIONS 205), passes `go vet`, and a smoke
     test reads every value through the getters.
  5. `canon.lock` of `teamboard` equals its golden `examples/teamboard/expected/canon.lock`
     (LOCK.md §9.1), compared through the MANIFEST like every output (§7.1); deleting a status
     gives `E6001`; renaming one gives `E6001`; retiring one and building passes.
- M1 is accepted on items 1–5. The `teamboard` integration (in a branch of `services/sovcommon`,
  its hand-written loader replaced by the generated package, the service tests passing) is not
  part of it: like every consumer integration, it waits for the integration gate of M1.5
  (DECISIONS 189, 206).

### M1.5 — Generated-program testing

- Scope: the program generator and the four suites of §7.7 (DECISIONS 200), built by QA in
  `internal/testkit`. It runs beside M2.
- Accepted when: every ERRORS.md rule has a mutation operator; one nightly run of the four suites
  is clean under its memory cap; every counterexample found is shrunk and kept as a txtar.
- **Integration gate.** No consumer integration (sovcommon `teamboard` included) starts before
  the four suites run clean (every ERRORS.md rule mutated, well-typed programs agreeing between
  the evaluator and the generated code, metamorphic variants unchanged) and conformance is green
  (DECISIONS 206).

### M2 — Pipeline: data mode for Go and C++, with goldens

- Scope: `load.dir` of JSON, wire decode, keyed lists, `@json(unit:)`, export fns of the three
  kinds, conformance vectors and tests for Go and C++, `data` mode loaders, `@reload` stores and
  snapshots, fingerprints, `canon test`, `jsonsrc` positions in findings.
- Accepted when:
  1. At the start of M2 the compiler regenerates `examples/pipeline/expected/` (GEN-01); the
     orchestrator reviews the diff against the conventions of the existing Go and C++ code
     (sovcommon, Source) and decides (DECISIONS 190); from then on `canon build pipeline` equals
     them byte for byte.
  2. Generated C++ compiles with `-std=c++17 -Wall -Wextra -Werror` on GCC 9 and current, Clang 10
     and current, and MSVC 19.2x `/std:c++17 /W4 /WX`, with nlohmann/json 3.9 and current.
     `RunPipelineConformance()` returns 0 (CPP-06).
  3. `go test` of the generated Go package passes (conformance) with `-race`.
  4. `canon test pipeline` reports 1 passed; breaking `healFor` in a copy reports the failing
     `expect` in the CLI.md §3.5 format.
  5. Both loaders refuse a data file whose `$schema` differs, with a message naming both
     fingerprints; `Reload` keeps the old snapshot on failure (Go test with `-race`, C++ test).
  6. FINGERPRINT.md's test vectors pass.
  7. A finding in `data/II_POT_HEAL_L.json` points at the right line and column and pointer.

### M3 — Load and the view model

- Scope: every `load` form; `at:`, `partial:`; `load.defines`; assets; dependent types, dependent
  maps, parameterized records; inline variants; layers and inputs; views, translations, `gen/view`;
  C++ `types` mode; `canon explain` for values; `api.Check`, `Value`, `ViewModel`.
- Accepted when:
  1. Every example package, checked against `examples/_fixtures`, prints exactly its
     `expected/findings.txt`.
  2. The view models of `farm`, `events` and `pipeline` validate against
     `spec/viewmodel.schema.json` and equal their goldens.
  3. `canon build balance.parity` output equals its golden.
  4. `canon explain config.server.port --layer louis` prints its golden (CLI.md §3.7 format).
  5. `api.ViewModel(pkg).JSON()` equals the `emit view` file for every example with one.
  6. The C++ `types`-mode output of `events` compiles and `Decode` accepts the fixture
     `EventConfig.json`.
  7. Real-data job (§7.3) runs `canon check resource... balance...` on the real `Resource/` tree
     and its findings are reviewed and stored as `testdata-real/realdata/findings.txt` (not gating, §7.3).

### M4 — Formatter and the edit API

- Scope: `canon fmt` (`.canon` and `--json-sources`), `format` fixed points, the complete public API
  (API.md), `Evaluate`, `Watch`, `canon explain`, `refs`, `--watch`; the incremental memoization
  needed by NFR-01.
- Accepted when:
  1. Every example is a fixed point of `canon fmt`; `fmt --check` passes; formatter fuzzing finds
     no non-idempotent input in 10 minutes.
  2. Every numbered rule of API.md has at least one test naming it (§7.4); all pass.
  3. The minimal-write invariant (API.md M6) holds under fuzzing: random `Set`/`Add`/`Remove` on
     every example and on the benchmark project, under §7.6's fuzz gate.
  4. NFR-01 edit and evaluate targets hold on the benchmark (§7.6).
  5. Crash test: an FS that fails at each rename in turn leaves, after `Open`, every file as before
     the edit.
  6. Stress test under `-race`: 8 readers, 1 editor and 1 watcher for 60 s, no race, no stale read
     after an edit returns. The duration is a minimum: the run goes on until its work floor is met
     (at least 10 edits, every reader observing after an edit), failing only at a hard cap, and it
     checks content, not only revisions, on every kind of read.
- M4 has no studio integration spike: the current resourcestudio is not adapted, and a new
  studio is built on the Canon API (DECISIONS 191).

### M5 — Language server and agent CLI

- Scope: §8.4; `canon edit` and `canon rename` (CLI.md §3.15, §3.16; DECISIONS 274).
- Accepted when: scripted JSON-RPC transcripts (`internal/lsp/testdata/*.txtar`) pass for every
  feature of CLI.md §4; UTF-16 conversion tests with non-ASCII text pass; diagnostics for an edited
  entry file of the benchmark project are published within 500 ms of the last change; findings in
  JSON files are published without the file being open; `canon edit` applies a request and its
  printed `undo` restores every value (API.md E22), and every file byte for byte where the edit
  kept the layout, on every example of `examples/features/edits`;
  `canon rename` has a golden per name kind (field on the wire, type, function, let, local) and a
  refused stable id, each undone by its printed undo byte for byte (DECISIONS 275).

### M6 — Legacy C++ modes and TypeScript

- Scope: `@cpp(struct, header, access: fields | both | getters, field, type, name)`, `@cpp(defines)`,
  `E8201`; the TS target in every mode, with conformance tests.
- Accepted when: the `examples/features/legacycpp` `ItemProp` example compiles in all three
  modes against its hand-written header fixture, and in `getters` mode the expected compile errors
  are the listed reads; TS goldens pass `tsc --strict --noEmit` (TS 5.0 and current) and
  `node --test` (Node 20 and current) for every example with a TS emit: `sovcommon/ui`,
  `sovcommon/roles` and `teamboard` (baked), `features/embedded` (embedded) and `features/lookup`
  (lookup tables); the `data` mode and translated functions through the `features/ts` example of
  §7.9 (the pipeline has no TS emit); `E8101` is tested.

### M7 — Migration tools

- Scope: `canon convert`, `canon i18n stub|status` (`canon infer` dropped, DECISIONS 188).
- Accepted when: `convert potions` on a copy
  of `pipeline` passes its proof and writes its golden files; a convert whose proof fails writes
  nothing and prints the differences; `i18n stub fr` on `farm` equals its golden.

---

## 7. Test strategy

### 7.1 Goldens from `examples/`

- **Harness.** `internal/testkit/golden` runs, for each example directory with an `expected/`
  folder: copy the project and `examples/_fixtures` into a temporary directory; open it with
  `Options.Roots` redirecting the roots as `examples/_fixtures/README.md` says (a root with a
  fixture directory, `resource` and `client`, at the copied fixtures; every other root outside the
  project at an empty temporary directory; the roots inside the project, `pipeline_go` and
  `features`, unchanged, since the project is already a copy); run `Check`, then `Build`.
- **Manifest.** Outputs are compared only for an example that has `expected/MANIFEST`: today
  `pipeline`; `teamboard` gets one in M1 (M1 item 3), and each later example when its outputs are
  frozen. The manifest lists one line per output: `<display path written> <golden path relative
  to expected/>`, e.g. `pipeline/out/potions.json potions.json`, the display path being the
  written form of WIRE.md §2.3 (project-relative for `emit … { out: "out/" }`). Every written
  output must be listed and every listed output written. A `canon.lock` that the build writes or
  appends to is an output like the others: it is listed as `<package dir>/canon.lock canon.lock`
  (for teamboard, `teamboard/canon.lock canon.lock`). Without a manifest, only
  `expected/findings.txt` is compared (§7.2).
- **Comparison.** Byte for byte. The one exception is the transition of GEN-01: until the start of
  M2, the illustrative pipeline goldens are compared semantically (parsed JSON; Go and C++ modulo
  comments and whitespace) and the test is marked "illustrative".
- **Update.** `go test ./internal/testkit/golden -update` rewrites goldens (`Check`, then `Build`
  into the fixture roots). CI never updates; a golden change is reviewed in the diff like code.
- **Smoke tests.** `expected/` holds only compiler output. The smoke test of an example's
  generated Go module lives in `internal/testkit/golden/testdata/smoke/<example>/`, and
  `goldens-vet` copies it beside a temporary copy of the module (DECISIONS 201).
- **Command output goldens.** The samples in CLI.md (§2.4, §3.5, §3.7, §3.8) are frozen as
  goldens in `internal/cli/testdata/` (CLI-03), run against the examples.

### 7.2 Expected findings

- **Per example.** `examples/<ex>/expected/findings.txt`: the output of `canon check <packages>
  --color never` against the fixtures: the text form of API.md §4.4, sorted per API.md F2,
  followed by the summary line with the duration replaced by `(…)`. An example that must be clean
  has only the summary line.
- **Per code.** Every registry code has at least one test case:
  `internal/<pkg>/testdata/findings/<CODE>_<n>.txtar`, a txtar archive holding a minimal project
  and a `findings.txt` section. The audit rule `diag-code-untested` fails when a code the
  compiler reports has none (DECISIONS 55).
- **Negative cases live in tests, not in fixtures.** The fixtures are clean except on purpose
  (`examples/_fixtures/README.md`): no fixture produces an error, and the few warnings they
  produce mirror findings of the real data (farm's unreachable levels, heistia's duplicate
  description), each listed in the example's `findings.txt`. Errors firing are tested by the
  per-code txtar cases above, by `expect … fails` / `warns` in the examples' `test` blocks, and by
  `examples/features/`.

### 7.3 Fixture tree (ORG-02)

`examples/_fixtures/<root>/` mirrors each root the examples read (`resource`, `client`) with the
smallest files that exercise each example: real rows, trimmed, key names and value spellings
kept. [`examples/_fixtures/README.md`](../examples/_fixtures/README.md) is the authority: it lists
every fixture and what reads it, how the harness redirects roots, and the rules (real shapes,
UTF-8 and `\n`, placeholder icons named exactly as the data names them, clean except on purpose,
small). Total size ≤ 300 KB. From M0, QA regenerates them with
`go run ./internal/testkit/cmd/fixturegen` (deterministic: fixed list of ids, fixed order) and
commits the result; the tool is re-run only on purpose. Files an example reads by a relative path
(`pipeline/data/`, `features/*/data/`) live next to the example.

A **real-data job** (`make check-real`, non-gating, runs where `testdata-real/` holds the real
`Resource/` tree, DECISIONS 29) checks the examples against it, `client` still redirected to the
fixtures, and records `testdata-real/realdata/findings.txt` (git-ignored) for review; the reviewed
list goes to `meta/handoff/` as a list only.

### 7.4 API rule tests

Every numbered rule of API.md (O1…X2) has at least one test that cites it. A citation has one
form, `API.md <id>` (a list, `API.md E1, E2`, is allowed; a range is not), and counts only in the
doc comment or body of a `Test`, `Example` or `Fuzz` function or in the comment of a txtar archive a
harness reads; it must prove its rule. Edit tests are txtar goldens in
`internal/edit/testdata/edits/`: the files before, the `Edit` in its JSON form (API.md §8.8), the
files after, and the `EditResult` through a JSON projection of the tests' own (without `Revision`:
`EditResult` has no public JSON form). A golden records an error if and only if its name starts
with `refuse_`; an unexpected error fails it. On every case that applies, the test asserts the
minimal-write invariant M6 (its one-line clause wherever M6 applies it, never by opt-in), N12 and
the `Undo`. A coverage test lists the rule ids found in API.md and fails when one has no test.

### 7.5 Determinism (NFR-05)

- **Twice, differently.** CI builds every example twice, with `GOMAXPROCS=1, Workers=1` and
  `GOMAXPROCS=8, Workers=8`, through a test FS that returns directory listings in a different
  random order each time, from two different absolute checkout paths, with different `TZ` and
  `LANG`. All outputs, findings (text and JSON), view models and lock files must be identical.
- **Across platforms.** Linux, macOS and Windows runners build the examples; outputs must equal the
  Linux goldens byte for byte (NFR-04).
- **No map order.** The audit rule `maprange` (`tools/audit`, run by `make check`, DECISIONS
  110) reports every `range` over a Go map, or over `maps.Keys`, `maps.Values` or `maps.All`,
  tests included, in the packages `api`, `build`, `diag`, `edit`, `format`, `gen`, `i18n`, `ir`,
  `jsonsrc`, `lock`, `views`, `wire` and the packages under them. A loop that is
  order-independent is annotated `//canon:unordered` with a reason.
- **Race.** Every test runs under `-race` in CI.

### 7.6 Performance (NFR-01)

- **Benchmark project.** `go run ./internal/testkit/cmd/benchgen -seed 1 -out bench/` generates a
  project that looks like the item domain: a `stable table Item` with **7,000 entry files** placed
  by `@files("items/{kind}/{id}.canon")`; `Item` has 12 shared fields and a variant of 8 cases
  with about 40 fields each (about 50 set per entry); refs into a 7,000-define header and a
  500-entry monster table; an asset field with 7,000 icon files; a view with `title`, `search`,
  `filters`, `columns`; 3 record checks and 2 package checks (uniqueness, group-by). A twin package
  reads the same data as 7,000 JSON files with `load.dir`.
- **Targets**, measured on the reference machine, Louis's local machine (Linux x86-64, AMD Ryzen AI
  9 HX 370, 24 cores; the benchmark prints the CPU model; a recorded number states its machine and
  the load average at the run's start (printed by the bench) and at its end (recorded by hand), so
  runs compare only at equal load, DECISIONS 261), for the benchmark project plus all examples:

  | Measure | Target |
  |---|---|
  | cold `canon check` (empty cache) | ≤ 10 s |
  | warm `canon check` (unchanged manifest) | ≤ 1 s |
  | `Edit` of one entry field, re-check and write, p95 over 200 random `Set`s | ≤ 300 ms |
  | `Evaluate` of one entry after a committed edit, p95 | ≤ 150 ms |
  | peak RSS during cold check | ≤ 1.5 GB |
  | view model per package, without search index | ≤ 5 MB |

- **Gates.** The full benchmark is an opt-in make target run on the reference machine, never part of
  `make check`; exceeding a target fails it, and its numbers are recorded in `meta/`. A 4-core CI
  runner (Linux x86-64, 4 cores, 16 GB) may run it, or a 1,000-entry version, as a report that never
  gates (DECISIONS 261). The edit benchmark (`bench-edit`) gates the `Edit` p95, the `Evaluate` p95,
  cold check, cold RSS and the view-model size. It calls `Value` before each timed `Set`, as the
  studio refreshes its view model after a committed edit (API.md V14); that refresh is outside
  NFR-01's gated measures (DECISIONS 261). Warm `canon check` is reported, not gated, until the
  on-disk cache is turned on (`Options.Cache` is accepted and inert in M4). A gate never passes
  silently:
  - cold check and RSS are measured per project (the benchmark and each example), each against its
    target; a project with errors is not measured;
  - the 200 timed `Set`s draw every editable scalar field kind present (assets, as another existing
    file of the type not held before, and `String`s of named or literal-union types included)
    with fresh valid values, the fields the package's checks read included; rejected `Set`s are
    reported apart and not counted, as are `ErrNotEditable` refusals, each bounded at 10% on the
    benchmark (more fails the run), while on the examples, whose checks tie fields together on
    purpose, the share is reported; on the benchmark, a kind present but never drawn fails the run;
  - the edit fuzz run with `-edit.bench` fails if the benchmark gets no `Set`, `Add` or `Remove`
    applied, if its baseline coverage never completes, or if, past the baseline, execs fall under
    2,000 per minute or applied edits under 200 per minute per project (about a fifth of the
    reference machine's rate). On the benchmark each input is one op copying at most 1,000 values;
    multi-op sequences and whole-collection copies are fuzzed on the examples only (DECISIONS 261).
- **Architecture it forces (NFR-02).** Package-level invalidation alone cannot re-check a
  7,000-entry package in 300 ms. From M4, `workspace` keeps: a per-file cache of parse and type-check
  results keyed by (file hash, hash of the package's declaration signatures); per-entry evaluation
  memoized by (entry source hash, hashes of the values it reads); record-check results memoized by
  value hash for checks that statically read no package value, a requirement met by replaying a
  check result while every value its run read keeps its fingerprint, which memoizes checks
  reading package values too (DECISIONS 261). The `value` store is designed for
  this from M1 (values addressable by (root, key) and hash-consed provenance). Incremental equals
  cold in everything observable: values, findings, steps and budget charges, provenance (EVL-07),
  identities (`check.Info` object identity included), invalid and written marks, bound arguments.

### 7.7 Fuzzing and generated programs

Native Go fuzz targets, run 10 minutes nightly each: the lexer and parser (no panic; every error
has a span), `Format` idempotence, `FormatJSONSource` idempotence, wire round trip
(`decode(encode(v)) == v`), path `Parse`/`String` round trip, and edit invariants (API.md M6).
The edit fuzz (`make fuzz-edit`) fails when it never leaves its baseline or falls under its
throughput floor (§7.6): a fuzz run that does not fuzz is not a pass. The other targets have no
such judge yet (DECISIONS 261).

A program generator in `internal/testkit` (M1.5, DECISIONS 200) drives four nightly property
suites, each under a memory cap:

1. **Rule mutation** of the examples: one operator per ERRORS.md rule; each mutant gets exactly
   the code its rule names.
2. **Grammar-driven generation with token mutations**: valid programs parse and survive a
   format-reparse unchanged; corrupted ones get a located finding, never a panic.
3. **Type-directed well-typed programs**: a clean `check` implies a successful `build`, the
   generated Go compiles, and its answers equal the evaluator's.
4. **Metamorphic variants**: renames, reordering, comments and whitespace change no finding and
   no output.

A counterexample is shrunk and kept as a txtar case.

### 7.8 Target toolchains

Generated code is compiled and its tests run in CI: the current Go (in a temporary module);
GCC 9 and current, Clang 10 and current, MSVC 19.2x; nlohmann/json 3.9 and current (vendored under
`internal/testkit/third_party/`); TypeScript 5.0 and current with Node 20 and current. C++ is built
with `-ffp-contract=off` (CNF-03).

### 7.9 Missing feature examples (ORG-03)

Before the module that implements each feature starts, QA makes sure a small example with goldens
exists under `examples/features/<name>/` (the directory names, DECISIONS 262):

| Example | Feature | Needed by | State |
|---|---|---|---|
| `retirement` | `retired` entries and `@codes` members, `E3502`, `.active()` | M1 | present |
| `matching` | value-level `match` on enums and variants, exhaustiveness | M1 | present |
| `codes` | `@codes` enums with `@json(codes)` | M2 | present |
| `embedded` | `embedded` mode in Go and TS | M2 | present |
| `lookup` | finite-input export fns (lookup tables), including a table-keyed parameter | M2 | present |
| `warns` | `expect … warns` and `fails name` | M2 | present |
| `csv`, `text` | `load.csv` with and without header, `load.text` | M3 | present |
| `dependent` | dependent types: enum, `Bool` and match-path discriminants, lists, baked Go literals, C++ data loader | M3 | present |
| `legacycpp` | `@cpp(struct, access)` with the hand-written header fixture `ProjectCmn.h` | M6 | present |
| `entries` | `entry` files, `@files`, entry order, entries of a keyed list; error-free, so duplicate keys are `internal/check` `E3101` (table) and `E3102` (keyed list) cases and `ErrKeyExists` refusals (DECISIONS 262) | M1 (build), M4 (edit) | present |
| `pairs` | `@json(pairs:)`: slots, gaps (`E7117`), fingerprint | M2 | to add |
| `ts` | TS goldens for data mode and translated functions | M6 | to add |
| `edits` | a project used only by edit tests: defaults, spread, layered values, JSON sources, dependent fields, variants | M4 | present |

---

## 8. Command behaviours

This section fixes what CLI.md leaves to the implementation. Language semantics are elsewhere.

### 8.1 Output formats (CLI-03)

Text formats are the CLI.md samples, frozen as goldens (§7.1). Findings use the text form of
API.md §4.4 (DIAG-03, rules F9–F15): the header `<severity>[<CODE>]`, two spaces, the location
`file:line:col`; detail lines indented two spaces (the path and message, the layer, one
`expected by` line per related location, the stack); a blank line between findings; then the
summary. Colour only when `--color` allows it; `NO_COLOR` is respected.

With `--format json`, every command prints one JSON object per line (JSON Lines), with keys in the
order shown, and ends with one `summary` object:

| Command | Lines |
|---|---|
| `check`, `lock check` | one finding per line (API.md F5), then `{"summary":{"errors":2,"warnings":5,"packages":3,"ms":1400,"truncated":[…]}}` |
| `build` | findings, then `{"output":{"path":…,"target":"go","package":…,"status":"written"}}` per output, `unchanged` ones included, `{"lock":{"package":…,"file":…,"line":…}}` per appended line, then check's summary (`truncated` kept) followed by `"written"` and `"stale"`, the outputs and locks changed or, under `--check`, that would change; with `-q`, only the error findings and the summary (CLI.md §3.4) |
| `test` | `{"test":{"package":…,"name":…,"file":…,"line":…,"status":"pass"\|"fail","failures":[{"file","line","col","expect","expected","got","findings":[…]}]}}` per test, then `{"summary":{"passed":12,"failed":1,"ms":300}}` |
| `explain` | `{"explain":{"path":…,"type":…,"text":…,"value":<wire JSON>,"origin":{…},"parts":[…]}}` (parts recursive to `--depth`) |
| `refs` | `{"ref":{"kind":…,"package":…,"path":…,"file":…,"line":…,"col":…}}` per ref (`path` omitted for `code`, `view`, `check` and `layer` refs, which have none), then `{"summary":{"target":…,"count":14}}` |
| `fmt` | in every mode: findings, then `{"file":…,"formatted":false}` per unformatted file (with `"diff":…` under `--diff`), then `{"summary":{"files":n,"unformatted":k}}` |
| `i18n status` | `{"i18n":{"package":…,"lang":…,"translated":…,"missing":…,"unknown":…}}` per pair |
| `version` | `{"version":{"compiler":…,"languages":[…],"fingerprint":"canon-fp v1","viewModel":"canon-vm/1","lock":"canon.lock v1","commit":…}}` |
| `convert` | `{"convert":{"written":[…],"adopted":[…],"delete":[…]}}` |
| `--watch` (check, build) | per cycle: `{"cycle":{"revision":…,"files":[…],"packages":[…]}}`, then each finding that appeared with `"change":"added"` and each that disappeared with `"change":"removed"` (`change` the last key), then, for `build`, the outputs and locks as the one-shot build prints them, then the summary; outputs that did not settle: a `cycle` line with `"settled":false`, then the last build's summary |

- In text mode, `--watch` prints per cycle a header `-- <n> files changed, <m> packages re-checked`
  (the count is not made singular: `1 files`), the findings that appeared, one line
  `fixed: <severity>[<CODE>]  <file:line:col>` per finding that disappeared, and the summary.
  Findings are matched between cycles by (code, file, path, message).
- A watched re-check that fails (an error value, not findings) is a cycle too: its failure's
  findings shown as added. It proves nothing about the findings before it: the state it leaves is
  the last good state's findings plus the failure's, so the repair reports the failure's findings
  removed and re-adds nothing that did not change. `cycle.revision` is the revision of the result
  shown (a build's, after its writes).
- A rebuild caused only by the previous build's own writes runs at most once in a row (chained
  loads settle); if it changes outputs again, the watch prints that outputs did not settle and
  waits for an outside change, so a self-feeding project cannot spin.
- `ms` and durations are the only non-deterministic fields; golden tests replace them.

### 8.2 `canon infer` (dropped)

Not part of v0.1 (DECISIONS 188).

### 8.3 `canon convert` (CLI-02)

`canon convert <value> [--layout tpl] [--adopt] [--dry-run]`:

1. The value must be read by `load` or `load.dir` of JSON or CSV files and have a declared type;
   `load.defines`, `load.text` and computed values are usage errors (exit 2).
2. The value must already be written by the package's `emit json`, so the runtime reads an
   emitted data file through a generated loader, not the source files; otherwise exit 2 with that
   explanation. This replaces CLI.md §3.10 step 4: WIRE.md §8.1 has no `bare` option (GEN-06), so an
   emitted file never has the legacy shape a hand-written loader expects, and a runtime must move
   to a generated loader (`data` mode, or `types`-mode `Decode` of the emitted value) before its
   domain is converted.
3. **Printing.** A table or keyed list becomes one `entry` per element (keyed lists are allowed,
   the key field is omitted: `entry potions.II_POT_HEAL_L { … }`), placed by `--layout`, else the
   `@files` template, else `<value>/<key>.canon`; the `@files` annotation is added to the `let`
   with the template used. Any other value becomes a literal replacing the `load(…)` expression.
   Printing follows API.md §9.3: contextual names, canonical literals, fields equal to their
   defaults omitted.
4. The `load(…)` of a table or keyed list becomes `{}` or `[]`.
5. **Adoption.** With `--adopt`, for a single-file `load`, the package's `emit json` for the value
   is changed to write to the path of the converted source file (file mode; if that `emit json`
   writes several values, `--adopt` is a usage error). After conversion the build writes the data
   file there and **adopts** it: it prints `adopting <path>` and overwrites a file without the
   GENERATED marker, which only `--adopt` allows (GEN-05, CODEGEN.md §2.4). Deployment paths stay
   the same.
6. **Proof.** The project is built before and after in memory. The proof passes when (a) the
   evaluated values are equal (TYPES.md §7.5 equality, plus entry order and retired flags), and
   (b) every emitted output is byte-identical, except the view model's `values.*.sources`,
   `usage`, and finding locations. (`usage` counts fields present in the source (VM-06); omitting
   fields equal to their defaults changes it by design.) If the proof fails, nothing is written and
   the differences are printed as a unified diff of the canonical JSON of the differing values.
7. On success, `convert` writes the new files atomically (API.md §10.3), leaves the JSON sources in
   place (except an adopted file, which now holds the emitted data), and prints `git rm` commands
   for the others. `--dry-run` prints what would be written.

### 8.4 `canon lsp` (CLI-04)

- **Transport.** LSP 3.17 over stdio. One `api.Project` per `project.canon` found above an opened
  file; files outside any project get syntax diagnostics only.
- **Positions.** LSP positions are UTF-16 code units; conversion is done in `lsp` only, from the
  1-based UTF-8 byte columns of findings.
- **Sync.** Full-document sync; each change is an overlay (API.md §3.4). Diagnostics are recomputed
  for the affected packages 150 ms after the last change and published for every file with
  findings, including loaded JSON, CSV and header files that are not open; files that no longer
  have findings get an empty list.
- **Features**, per CLI.md §4:
  - **hover:** canonical type text, doc comment, default, and for a let, const or table entry its
    canonical text truncated to 20 lines and 4,096 bytes, a cut ending with a `…` line.
  - **definition:** declarations; from a `ref` value to the entry, including into JSON and from a
    position inside a JSON buffer; a ref stated once and evaluated per instance (a field default
    in a field's collection) answers every instance's entry; a value a layer replaced is still
    found, as `Refs` finds it (DECISIONS 285).
  - **references:** `Refs`: entries, keyed elements and enum members (API.md R7). References of
    Canon names come with `canon rename`'s name index (A2). With `includeDeclaration`, the entry's own location comes
    first. References start from a ref key naming the target, or a ref in a JSON buffer. In a `*.layer.canon` file, hover shows
    the base value, labelled as such.
  - **formatting:** `Format`.
  - Completion, code actions and rename are not offered (DECISIONS 274). The rename of a Canon
    name is `canon rename` (CLI.md §3.16, API.md §8.9, DECISIONS 275).
- **Highlighting.** A hand-written TextMate grammar (`editors/vscode/syntaxes/canon.tmLanguage.json`),
  tested against every example with a snapshot of scopes.
- **Extension.** `editors/vscode` starts `canon lsp` (no extra argument) and watches
  `**/*.{canon,json,csv,h,txt}` in the workspace folders for `workspace/didChangeWatchedFiles`. Files
  read under another extension (`format:`), assets, and roots outside the workspace are not watched:
  their changes show after the next edit of a buffer (DECISIONS 274).

### 8.5 Other command behaviour

- **Exit codes.** CLI.md §2.5, and API errors map as API.md §15 says. An interrupt (SIGINT,
  SIGTERM) cancels the context, finishes or rolls back any write, and exits with 130.
- **`--root <name>=<dir>`** (repeatable) sets `Options.Roots` (API.md §2.1).
- **Cache.** `.canon/cache/` holds, keyed by the build manifest hash (LOD-11), the findings of
  `check` and the output hashes of `build`, plus a stat table `(path, size, mtime, sha256)` so an
  unchanged file is not re-hashed. The format has a version in its first line; another version is
  ignored and rebuilt. Deleting the directory is always safe.
- **`canon build`** prints, after the summary, the changed outputs grouped by target, one per line,
  in byte order, then the locks that gained lines; under `--check` each is prefixed `stale `;
  unchanged outputs are listed only in the JSON form (CLI.md §3.4, DECISIONS 201).
- **`canon version`** prints three lines: `canon <compiler version> (<commit>)`, `language 0.1`,
  `formats canon-fp v1, canon-vm/1, canon.lock v1`.

---

## 9. Versioning (NFR-03)

| What | Version | Rule |
|---|---|---|
| language | `MAJOR.MINOR` in `project.canon` (`canon: "0.1"`) | A compiler lists the versions it accepts. It accepts a project whose major it knows and whose minor is ≤ the highest it knows; a newer minor or another major is `E1001`. While the major is 0, each minor may break the previous one and a compiler supports only the minors it lists. |
| compiler | semver `0.x.y` | Printed by `canon version`. A new language minor is a new compiler minor. |
| fingerprint | `canon-fp v1` (FINGERPRINT.md) | Changing the algorithm is a new compiler major. |
| view model | `canon-vm/N` in `$schema` | A breaking change bumps `N`; the studio refuses a version it does not know. |
| lock | `# canon.lock v1` header (LOCK.md) | A new format is read by the next compiler and rewritten by `build`. |
| runtime helpers | C++ `namespace canon { inline namespace rt_v1 }`; Go `rt` package with `const Version = 1`; TS header comment | A breaking change creates `rt_v2`; old and new can link into one binary. |
| cache | first line of each cache file | Unknown versions are ignored. |
| Go API (`api`) | module semver | No compatibility promise before 1.0 (API.md §1.2). |

---

## 10. Platforms (NFR-04)

- `canon` runs on Linux, macOS and Windows, on amd64 and arm64. It is built with `CGO_ENABLED=0`
  and `-trimpath`, so the binary does not contain the build machine's paths.
- Every path the compiler prints or writes into an output uses `/`. Paths in Canon sources use `/`;
  a `\` in a `load` or `emit` path is `E7001`.
- Every file the compiler writes uses `\n` and ends with one `\n`. Consumer repositories mark
  generated files `eol=lf` in `.gitattributes`, otherwise `build --check` sees CRLF checkouts as
  stale; `canon build` warns once when it reads an output with `\r\n`.
- Sources with `\r\n` are accepted and normalized (SPEC §2.1). `canon fmt` rewrites them with `\n`.
- Path order is the byte order of `/`-separated root-relative paths, on every platform.
- Names are matched case-sensitively everywhere (packages, entry files, globs, assets), including
  on case-insensitive file systems: existence checks compare against directory listings, never
  against a successful `Stat` (TYP-21, DECISIONS 19).
- Windows: paths longer than 260 characters are handled with the `\\?\` prefix inside the OS FS
  implementation; file replacement uses `MoveFileEx` semantics through `os.Rename`.

---

## 11. Libraries

Every third-party dependency is listed here. Adding one needs an update of this table.

| Need | Choice | Why |
|---|---|---|
| parser | **hand-written** recursive descent, with a Pratt parser for expressions | The grammar is context-sensitive in ways generator libraries handle badly: regex vs division by the previous token (LEX-01), interpolation lexer modes (LEX-02), newlines decided by the innermost bracket and the previous token (GRM-02, GRM-03), no typed literal in `if`/`match`/`when` headers (GRM-01), keywords usable as names in some positions (LEX-08). The formatter and the edit API need a lossless tree with trivia (§4.1) and the language server needs error recovery. participle builds an AST from struct tags, drops trivia, and recovers poorly; tree-sitter needs cgo. A tree-sitter grammar may still be written from GRAMMAR.md for editors, as a separate artifact. |
| CLI | standard `flag` (DECISIONS 139) | no third-party dependency; Louis ruled out cobra (2026-09-24). |
| JSON with positions | `github.com/go-json-experiment/json` (`jsontext`) | Token-level decoder with `InputOffset()` for spans and `StackPointer()` for RFC 6901 pointers, rejects duplicate names by default (LOD-02 `E7104`), raw number tokens for exact parsing. It is the upstream of Go's experimental `encoding/json/v2`; switch to the standard library when it is no longer behind `GOEXPERIMENT`. |
| JSON number and float text | standard library (`strconv`, `math/big`) | Floats are formatted as ECMAScript `Number::toString` (WIR-01, STD-06) on top of `strconv.FormatFloat(f, 'e', -1, bits)`; exact decimal parsing uses `math/big`. |
| globs | `github.com/bmatcuk/doublestar/v4` | `**` semantics of LOD-05; ordering is done by the compiler. |
| file watching | `github.com/fsnotify/fsnotify` | Cross-platform; directories are watched one by one (it is not recursive), which the benchmark's ~20 directories allow. |
| diffs (`fmt --diff`, convert) | `github.com/aymanbagabas/go-udiff` | Unified diffs; an exported copy of `golang.org/x/tools/internal/diff`. |
| multi-file test cases | `golang.org/x/tools/txtar` | Standard format for Go test archives. |
| VS Code LSP client (npm, `editors/vscode` only) | `vscode-languageclient` ^9 | The official client that starts `canon lsp` over stdio; not part of the Go module (Louis, 2026-10-02). |
| TypeScript conformance toolchain (npm, tests and CI only) | `typescript` 5.0 and current | `tsc --strict --noEmit` over the TS goldens (§6 M6); a test toolchain like `g++`, never imported by the compiler (DECISIONS 277). |
| VS Code packaging (npm dev, `editors/vscode` only) | `@vscode/vsce` ^3 | Builds the `.vsix`. Highlighting is tested by the pure-Go TextMate engine in `internal/testkit/vscodegrammar`, not by `vscode-textmate` (Louis, 2026-10-02). |
| test comparisons | `github.com/google/go-cmp` | Readable diffs of results. |
| terminal detection | `golang.org/x/term` | `--color auto`. |
| regex | standard `regexp` (RE2) | SPEC §2.5; search semantics (STD-03). |
| CSV | standard `encoding/csv` | RFC 4180 (LOD-06). |
| hashing | standard `crypto/sha256` | fingerprints, revisions, manifest. |
| C++ JSON in generated code and tests | nlohmann/json ≥ 3.9 (vendored for tests) | SPEC §15.3. |

---

## 12. Code doctrine

DECISIONS 25. The compiler's own Go code follows the fleet's code doctrine
(`sovcommon/tools/sovaudit`, `fleet/DOCTRINE-code.md`), in the version customised for this
repository: [tools/audit/DOCTRINE-code.md](../tools/audit/DOCTRINE-code.md), with every rule, its
mode and its reason in [tools/audit/rules.md](../tools/audit/rules.md). Those two files are the
authority on the rules; this section says how the compiler applies them.

### 12.1 The auditor

- `tools/audit/` is the project's own copy of sovaudit (`canon audit`), a separate Go module
  (`github.com/fantasim/canonlang/tools/audit`) that the compiler never imports
  ([tools/audit/README.md](../tools/audit/README.md)). It is customised for a standalone compiler
  repository: the fleet-only lanes are gone (comparisons with sovcommon and other services, the
  Svelte and front-end rules, the meta/ census), "sovcommon first" became "standard library
  first", and generated goldens and test data are not judged (`examples/**/expected/`,
  `examples/_fixtures/`, `testdata/`, files marked `// Code generated … DO NOT EDIT.` or named
  `*.gen.go`).
- Because it is its own module, it runs from its directory:
  `cd tools/audit && go run . check --repo ../..` (the gate) and
  `cd tools/audit && go run . audit --repo ../..` (the census, `make audit`). DECISIONS 25 writes
  the gate `go run ./tools/audit check`; the Makefile is the reference form.
- The linters the stock lane needs (golangci-lint, deadcode) are pinned in
  `tools/audit/toolchain/` and configured there. The compiler has no separate golangci-lint
  configuration, and §11's library table does not list them: they are not compiler
  dependencies.

### 12.2 `make check`

The one gate, the same for every contributor (the repository is public) and in CI. The
[Makefile](../Makefile) runs, in order, stopping at the first failure:

1. **format**: `gofmt -l` prints nothing for the hand-written Go files;
2. **vet**: `go vet ./...`;
3. **tests**: `go test ./...` (CI adds `-race`);
4. **goldens**: each golden Go module under `examples/**/expected/` is vetted and tested in a
   temporary copy, with its smoke test from `internal/testkit/golden/testdata/smoke/<example>/`
   (DECISIONS 201);
   every `expected/MANIFEST` line names a golden that exists; and the goldens are diff-clean:
   every example rebuilt by the compiler writes exactly its `expected/` files (§7.1; the rebuild
   is wired when the compiler can build, M1);
5. **diagnostics**: `internal/diag/codes.go` equals what `diaggen` generates from
   `spec/ERRORS.md`, and the runtime helper texts use only the pairs of ERRORS.md §1.6 (target
   `diag-check`, from M0, when `internal/diag` exists); `api/vm/vm.gen.go` equals what
   `api/vm/internal/vmgen` generates from `spec/viewmodel.schema.json` (target `vm-check`, M3);
6. **audit**: the auditor checks its own code against its own baseline, then the repository:
   `cd tools/audit && go run . check --repo ../..`, which fails on any finding of an `enforce`
   rule and on any `ratchet` finding that is new or grew against the baseline (§12.3).

### 12.3 The ratchet baseline

- `.sovaudit/baseline.tsv` records the findings that existed when it was taken; `.sovaudit/state.tsv`
  holds this repository's rule modes; `.sovaudit/root-allow.txt` its intentional root entries.
- Only `make audit-tighten` (`baseline --tighten`) writes the baseline, and it only shrinks; the
  `baseline-guard` rule fails a baseline or state loosened against the base revision.
- One finding is silenced only by `// sovaudit:ignore <rule> -- <reason>`, and every ignore is
  counted (`ignore-count` only goes down).

### 12.4 What every package does

- **Size.** Functions ≤ 60 lines, ≤ 5 parameters, ≤ 3 results, nesting ≤ 3; files ≤ 500 lines.
  Past a limit, split by concern; never raise the limit.
- **Constants.** No literal twice and no bare number but 0 and 1; constants live in
  `<pkg>/constants.go`. Diagnostic codes and their message templates live only in
  `spec/ERRORS.md` and the `internal/diag/codes.go` generated from it (`diag.E3501`, §4.4), so no
  code string or message text appears in any other file. The
  numbers the documents fix are named constants of the package that enforces them: the call-depth
  limit 10 000 and the default budget (`eval`), the 65 536 lookup cells and the 1 000 000 steps per
  conformance vector (`conform`), the width 100 (`format`), the 20 digits of a format spec
  (`syntax`), the 512 JSON nesting levels (`jsonsrc`).
- **Errors.** Sentinels in `<pkg>/errors.go`, wrapped with `%w`, compared with `errors.Is`. The
  API's sentinels and error types (API.md §15) are `api/errors.go`.
- **Surface.** Nothing exported that only its own package uses (an Example test counts as a user);
  no dead code, commented-out code or TODO: work is tracked outside the code.
- **Comments.** At most 3 lines, saying why; no file header outside `doc.go`. A rule of a
  companion document is cited, never narrated: `// GRAMMAR.md §3.1`, `// API.md E14`,
  `// decision 25`.
- **Every package** has a `doc.go` holding its package comment, and an `example_test.go` with at
  least one `Example…` function that runs.
- **Standard library first**, then code already in this repository; §11 lists every third-party
  dependency.

The module layout keeps packages inside the limits; each owner plans files per construct in M0:

| Package | Files |
|---|---|
| `syntax` | the lexer by token family (`lexer.go` modes, `lexer_string.go` strings and interpolation, `lexer_number.go` numbers, durations and regexes), `separate.go` (the separator pass, GRAMMAR.md §3), the parser by construct (`parse_file.go`, `parse_decl.go`, `parse_annotation.go`, `parse_type.go`, `parse_expr.go`, `parse_stmt.go`, `parse_view.go`, `parse_project.go`), `recover.go`, and the AST by node group (`ast_decl.go`, `ast_type.go`, `ast_expr.go`, `ast_stmt.go`, `ast_view.go`) |
| `check` | one file per concern: scopes and resolution, contextual names, brace-literal classification, expressions by operator family, calls, flow narrowing, refs, dependent types, views, translations, layers |
| `eval`, `eval/std` | the interpreter by construct; the stdlib by receiver (`seq.go`, `keyed.go`, `map.go`, `string.go`, `math.go`, `graph.go`, `sort.go`) and `format.go` (canonical text, format specs) |
| `wire` | decoding by kind (`decode_scalar.go`, `decode_record.go`, `decode_variant.go`, `decode_pairs.go`), `encode.go`, `float.go` (ECMAScript number text) |
| `gen/go`, `gen/cpp`, `gen/ts` | one file per construct of CODEGEN.md §5. The runtime helper texts are **data, not Go**: `runtime/rt.go.txt`, `runtime/canon_runtime.h.txt`, `runtime/canon_runtime_json.h.txt`, `runtime/canon_runtime.ts.txt`, embedded with `//go:embed` and written out under their real names (`rt/rt.go`, …). No `.go` file of the compiler module holds them, so `go vet`, `go build` and the auditor never see them and no exemption exists (DECISIONS 26); the goldens pin their bytes, and `diag-check` their (code, text) pairs (ERRORS.md §1.6) |
| `api` | `doc.go` (the package comment), `project.go` (kept as `canon.go` until the documents linking it are next edited, DECISIONS 68: options, FS, project, open and close, packages, revisions, overlays), `findings.go`, `value.go`, `ops.go`, `edit.go`, `evaluate.go`, `watch.go`, `build.go` (build and test, and the helpers `Format`, `FormatJSONSource` and `Version` of API.md §14), `errors.go`, `constants.go` (the enum types with their values, DECISIONS 69, and the unexported texts), `example_test.go` and the `example_<concern>_test.go` files beside it: the frozen contract `api/canon.go` (a single file of over a thousand lines; its package comment is already in `api/doc.go`) is split this way in M0, with no API change but one: the sentinel `ErrSyntax` that `*SyntaxError` wraps (API.md §15) is added to `api/errors.go`. The split lands with the example test and the first `cmd/canon` path that reaches the API, which use `ErrSyntax` and the stubs: added earlier, the sentinel alone would be an exported name no other package uses, and a test alone would make the auditor's `deadcode -test` report every stub unreachable |

### 12.5 Agent docs

Of the fleet doctrine's rules on agent docs (its rule 7), these apply to this repository:
`CLAUDE.md` at most 80 lines, and so is any `.claude/` rule or agent file. The project's auditor
has no meta lane (DECISIONS 25), so review keeps these caps. `meta/` holds the working state
(`meta/state.md`, capped at 80 lines like `CLAUDE.md`), the plan, the handoffs to Louis and the
orchestrator's decision logs (`meta/decisions/log-<date>.md`). Language and direction decisions
are recorded in `DECISIONS.md`, one short entry each; a technical call the orchestrator makes
where the spec is silent goes to the decision log (DECISIONS 207). The normative documents (SPEC.md,
CLI.md, `spec/*.md`, DECISIONS.md) are not agent docs and have no cap; like every Markdown file,
they are checked for dead links (the `dead-link` rule).

---

## 13. Open issues

1. **Converting a domain whose runtime has a hand-written loader.** WIRE.md has no `bare` option
   (GEN-06), so §8.3 requires the runtime to use a generated loader before its domain is
   converted; CLI.md §3.10 states it. Each domain's migration plan must schedule that loader
   change first.
2. **Real-data fixtures.** The fixture extractor needs read access to the real `Resource/` tree;
   the extracted subset should be reviewed for anything that must not be committed.
3. **Overlay imports.** DECISIONS 8 also mentions importing help texts from the studio overlays
   (`resourcestudio/internal/overlay/modules/*.json`); with `infer` dropped (DECISIONS 188), the
   agent writing a domain's first type reads them directly.
4. **Reference machine.** Settled (DECISIONS 261): Louis's local machine, the gate an opt-in make
   target there; a CI runner only reports.
5. **Legacy data that `@json(pairs:)` refuses.** In the real `propItem.json` (6,944 items), 6 items
   leave a gap between filled slots and 118 fill only one key of a slot (123 items in all); both
   are `E7117` (WIRE.md §5.14). Its `defaults` block also writes `"="` ("not set") in every slot.
   A one-off script fixes both (and turns `"="` slots into absent keys) before `game.items` reads
   the file (DECISIONS 13).
6. **`@json(pairs:)` on legacy structs.** A list of records cannot map onto a legacy C++ struct
   yet (`E8109`, CODEGEN.md §7.8.1), so `ItemProp`'s `dwDestParam[6]` and `nAdjParamVal[6]` need a
   mapping rule before `game.items` targets `@cpp(struct: "ItemProp")`.

## Addendum: DECISIONS 26 and 27 (binding)

- **M0 audit work.** Before any compiler package lands, tools/audit gains these rules:
  - `diag-message-inline`: a diagnostic message outside `internal/diag` is an enforce finding;
  - `diag-code-untested`: a catalogued code with no test that produces it is a ratchet finding
    from M0, with a target of zero at v0.1;
  - `ignore-count`: the ignore count is ratcheted and may not grow;
  - rule thresholds move from Go constants into one data file that the tool reads
    (`tools/audit/thresholds.tsv`).
- **Generated diagnostics.** `internal/diag/codes.go` is generated from spec/ERRORS.md by
  `internal/diag/cmd/diaggen`, whose contract is ERRORS.md §2, and `make check` diff-checks it
  (§4.4, §12.2 step 5).
- **Parser construction.** Dispatch over token and node kinds is table-driven. There are no
  size-limit exemptions: a function that would exceed a limit is split.
