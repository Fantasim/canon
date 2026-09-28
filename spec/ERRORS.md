# Canon diagnostic codes

Status: **normative**, and the **single source** of every diagnostic of Canon for language
version 0.1 (DECISIONS 27). For each code: its severity, the Go package that owns it, the document
section that says when it fires, a one-line meaning, and its English messages (one template per
variant, with typed arguments). `internal/diag/codes.go` is generated from this file (§2); no
other file declares a code or holds message text.

- **Ownership.** The owning document defines *when* a code fires: its diagnostics table lists the
  code, its severity and its trigger, and every other document that mentions the code points
  there. This file defines *what the finding says*. A new code is added to its owning document and
  to this file in the same change, and `codes.go` is regenerated (§2.3).
- **Severity.** `error` blocks emission (except `emit view`, VIEWMODEL.md J4); `warning` does not;
  `runtime` codes are signalled by generated code at run time, never reported by `canon`.
- **Package.** The package of IMPLEMENTATION-PLAN.md §3 (under `internal/`) that owns the code: it
  reports the code and holds the per-code tests (`internal/<package>/testdata/findings/`,
  IMPLEMENTATION-PLAN.md §7.2). When a trigger has a static and an evaluation form (`E3201`,
  `E3301`, `E1905`…), the package that meets the evaluation form reports that form with the same
  constructor. `gen` stands for the three code generators (`gen/go`, `gen/cpp`, `gen/ts`) and is
  used only by `runtime` codes. The number range of a code does not decide its package: `E4503`
  is static and belongs to `check`.
- **Text and JSON forms** of a finding: API.md §4.2 (JSON) and §4.4 (text: header, `expected by`
  lines, summary).
- **Retired numbers** are never reused: `E1624` and `E1625` (the view `row` item, removed by
  DECISIONS 21), `E8016` (merged into `E8153`). `E3014` and `E3319` were referenced once but never
  defined; their cases are `E6003` and `E3316`.

---

## 1. Messages

### 1.1 Tables

Each range section below has two tables.

- **Codes**: `Code | Severity | Package | Owner | Meaning`, one row per code. `Owner` is the
  defining document section; `Meaning` is a summary of its trigger.
- **Messages**: `Code | Variant | Args | Template`, one row per message, grouped by code in the
  order of the codes table.
  - `Variant` is `-` for a code with one message. A code with several messages has one row per
    variant, each named by a lowerCamel identifier unique within the code; no row is `-`.
  - `Args` is `-` or the ordered list `name:Type, name:Type…` of the message's arguments: at most
    4, names lowerCamel and unique within the row, types from §1.3. The order is the order of the
    constructor's parameters (§2.2).
  - `Template` is the exact English message, written as a code span.

Variants named `one` and `many` differ only in grammatical number: `one` is used when the count
is exactly 1, `many` otherwise (0 included).

### 1.2 Template grammar

A template is read left to right:

```ebnf
template    = { text | escape | placeholder } ;
escape      = "{{" | "}}" | "\\" | "\n" ;   (* a literal "{", "}", "\" or a line break *)
placeholder = "{" argName "}" ;              (* argName is declared in Args *)
text        = any character except "{", "}", "\", "`", "|" ;
```

- `{{` and `}}` are recognized before placeholders: `{{{name}}}` is a literal `{`, the
  placeholder `{name}`, and a literal `}` (it renders `{key}` for `name` = `key`).
- Every argument of `Args` is used at least once, and every placeholder names one of them. An
  argument may be used more than once (`W1604`).
- A `{` or `}` not part of an escape or a placeholder, a `\` not followed by `\` or `n`, a
  backtick and a `|` are invalid: messages are plain text and never use Markdown quoting.
- `\n` starts a new line of a multi-line message; API.md F10 indents each further line.

Rendering replaces each placeholder by its argument's rendering (§1.3); no other text is added.
The rendered message is `Finding.Message` (API.md §4.1); the path prefix, the location and the
`Related` lines are added by the text form (API.md §4.4), never by a template.

### 1.3 Argument types

| Type | Go parameter | Rendering |
|---|---|---|
| `Name` | `string` | verbatim: an identifier, a qualified name (`resource.vocab.Element`, qualified when it belongs to another package than the finding's), a value path (API.md §6.5), a keyword, an annotation or option name, a key as a name |
| `Names` | `[]string` | the elements as `Name`, joined by `, ` |
| `Chain` | `[]string` | the elements as `Name`, joined by ` -> ` (a cycle lists its first element again at the end) |
| `Type` | `diag.TypeArg` | the canonical type text (`types.Type.String()`, API.md `TypeInfo.Expr`) |
| `Types` | `[]diag.TypeArg` | the elements as `Type`, joined by `, ` |
| `Value` | `diag.ValueArg` | the canonical text form (STDLIB.md §9): strings quoted, durations as `1m30s` |
| `Expr` | `source.Span` | the source text of the span, each run of whitespace (line breaks included) replaced by one space |
| `Int` | `int64` | decimal, `-` for a negative number, no separators |
| `Rune` | `rune` | `U+` and at least four upper-case hexadecimal digits (`U+001F`) |
| `Path` | `string` | a display path (API.md §1.3): `@root/…` or project-relative; a directory outside the project and every root is written absolute, `/`-separated |
| `Loc` | `source.Span` | `<display path>:<line>` of the span's start (the form of API.md F12) |
| `Pointer` | `string` | an RFC 6901 JSON pointer in its URI fragment form: `#` then the pointer (`#/modelTypes/3`, `#` for the root) |
| `Kind` | `diag.Kind` | the word of §1.4 |
| `Text` | `string` | verbatim. Only for text the compiler does not write: source excerpts (a literal as written, an asset path, a pattern), data from loaded files, the message of a user `check` or `warn`, and error text from outside Canon (operating system, RE2, the JSON and CSV readers) |
| `Message` | `diag.Message` | another code's rendered message (§2.2, `Builder.Message`); used by `E1703` only |

A value is never passed as `Text` and English is never passed as an argument: an alternative
wording is a variant, and a closed set of words is a `Kind`.

### 1.4 Kinds

`diag.Kind` is a generated enum (`diag.KindField`…). A `Kind` argument renders as its word.

| Kind | Word | Used by |
|---|---|---|
| `Array` | an array | `E7110` |
| `Boolean` | true or false | `E7110` |
| `Builtin` | built-in | `E3016` |
| `Case` | case | `E1133`, `E1613`, `E1627`, `E3003`, `E3603`, `E6002` |
| `CaseField` | a case used as a type | `E8019` |
| `Check` | check | `E1133` |
| `Code` | code | `E3015`, `E6001` |
| `ComputedDefault` | computed default | `E8014` |
| `Const` | const | `W1003`, `E1125`, `E1133` |
| `ConstValue` | const value | `E3015` |
| `ConstantString` | a constant string | `E1119`, `E8009` |
| `CrossPackageBakedValue` | a value of a record, variant or table of another package | `E8019` |
| `DefineKey` | a lookup parameter that refs a load.defines table | `E8019` |
| `DefineTable` | define table | `E1627` |
| `DependentType` | a dependent type | `E8019` |
| `Emit` | emit | `E1133` |
| `Entry` | entry | `E1133`, `E6002` |
| `Enum` | enum | `W1002`, `W1003`, `E1125`, `E1133`, `E1627`, `E6001` |
| `EnumMember` | enum member | `E1118` |
| `Field` | field | `W1002`, `W1003`, `E1118`, `E1125`, `E1133`, `E1613`, `E2101`, `E3003`, `E3320`, `E6001` |
| `FieldlessCaseExportFn` | an export fn of a case without fields | `E8019` |
| `ForeignDataRecord` | a record or variant of another package read by a data loader | `E8019` |
| `ForeignPairsField` | a pairs field of a record of another package | `E8019` |
| `ForeignTableLookupParam` | a lookup whose parameter is a ref into a table of another package | `E8019` |
| `Function` | function | `W1002`, `W1003`, `E1125`, `E1133` |
| `GlobBrace` | an unclosed, empty or nested brace | `E7005` |
| `GlobBracket` | an unclosed bracket | `E7005` |
| `GlobDoubleStar` | a double star that is not a whole segment | `E7005` |
| `Icon` | icon | `E1610` |
| `Import` | import | `E1133` |
| `ImportAlias` | import alias | `W1003`, `E1125` |
| `InlineFoldedKey` | an inline variant key equal to a parent key but for letter case | `E8019` |
| `InputField` | an `input` field | `E8019` |
| `Integer` | an integer | `E1119`, `E7110` |
| `Layer` | layer | `E1125` |
| `Let` | let | `W1003`, `E1125`, `E1133` |
| `Literal` | a literal value | `E1119` |
| `Local` | local | `W1003`, `E1125`, `E2101` |
| `LookupFunction` | lookup function | `E8014` |
| `LowerCamel` | lowerCamel | `W1003` |
| `LowerSnake` | lower_snake | `W1003` |
| `Map` | map | `E3320` |
| `MapField` | a map | `E8019` |
| `Member` | member | `E1133`, `E1613`, `E3003`, `E3603`, `E6002` |
| `MemberValue` | member value | `E3015` |
| `Menu` | menu | `E1610` |
| `Method` | method | `W1003`, `E1125`, `E1613`, `E3003`, `E3016` |
| `MethodOrCheck` | method or check | `E1118` |
| `ModeName` | a mode name | `E8009` |
| `Null` | null | `E7110` |
| `Number` | a number | `E7110` |
| `Object` | an object | `E7110` |
| `OptionalElementList` | a list of optional elements | `E8019` |
| `OptionalMapValue` | a map with optional values | `E8019` |
| `PackageSegment` | package segment | `W1003`, `E1125` |
| `Parameter` | parameter | `W1003`, `E1125`, `E2101` |
| `ParameterDefault` | parameter default | `E3015` |
| `PrecomputedFunction` | precomputed function | `E8014` |
| `ReadIsDir` | is a directory | `E7004` |
| `ReadLinkLoop` | a symbolic link loop | `E7004` |
| `ReadMissing` | no such file or directory | `E7004` |
| `ReadNotDir` | not a directory | `E7004` |
| `ReadNotRegular` | not a regular file | `E7004` |
| `ReadPermission` | permission denied | `E7004` |
| `ReadTooLarge` | too large | `E7004` |
| `ReadUnreadable` | unreadable | `E7004` |
| `Record` | record | `W1002`, `W1003`, `E1125`, `E1133`, `E3320` |
| `RecordConstant` | a constant of a record or variant type | `E8019` |
| `RecordCycleThroughMethod` | a by-value cycle of records through a stored method result | `E8019` |
| `RecursiveVariantCase` | a variant that recurses through a case | `E8019` |
| `RefUnion` | a literal union over a ref | `E8019` |
| `RefinementBound` | refinement bound | `E3015` |
| `ResolvedLookupResult` | a lookup method whose result holds a ref resolved at load | `E8019` |
| `SelfReadNotAPath` | a read of self that is not a path of fields | `E8019` |
| `Spread` | spread | `E3320` |
| `StableValue` | stable value | `E6001` |
| `String` | a string | `E7110` |
| `Table` | table | `E3320`, `E6001` |
| `TableEntry` | table entry | `E1118`, `E3320` |
| `TableField` | a table field | `E8019` |
| `TemplateString` | a template string | `E1119` |
| `Test` | test | `E1133` |
| `Tone` | tone | `E1610` |
| `TopLevelDeclaration` | top-level declaration | `E1118` |
| `TypeAlias` | type alias | `W1002`, `W1003`, `E1125`, `E1133` |
| `TypeHeader` | type header | `E1118` |
| `TypeParameter` | type parameter | `W1003`, `E1125` |
| `Unit` | unit | `E1610` |
| `UpperCamel` | UpperCamel | `W1003` |
| `UpperSnake` | UPPER_SNAKE | `W1003` |
| `ValueNames` | a list of value names | `E8009` |
| `Variable` | variable | `W1003`, `E1125` |
| `Variant` | variant | `W1002`, `W1003`, `E1125`, `E1133`, `E1627` |
| `VariantCase` | variant case | `E1118` |
| `VariantMethod` | a method or check declared on a variant outside its cases | `E8019` |
| `View` | view | `E1133` |
| `Widget` | widget | `W1003`, `E1125`, `E1133`, `E1610` |

A constructor is given only a Kind whose row lists its code; the per-code tests cover each
listed pair.

### 1.5 Related notes

A `Related` location (API.md F4, F12) carries a note, built by a constructor of `diag` from this
table; its text follows §1.2 and §1.3.

| Note | Args | Template |
|---|---|---|
| `source` | `decl:Expr` | `{decl}`: the declaration, annotation or refinement the value was checked against, its span covering the name and the type for a field (`productionItem: ref items`) |
| `check` | `name:Name` | `check {name}` |
| `checkUnnamed` | `-` | `check` |

The stack lines (`in <fn> (<file>:<line>)`, `(<n> more frames)`) and the summary line are fixed
by API.md F13 and F15 and written by `diag`'s renderer.

### 1.6 Texts signalled by generated code

Generated code signals a code at run time with a fixed text (CONFORMANCE.md §3): the `runtime`
codes, and the evaluation codes a translated function or a checked helper can meet. The runtime
helper texts (CODEGEN.md §6.3 `rt.go`, §7.4 `canon_runtime.h`, §8.2 the TypeScript helper block)
use exactly the pairs below, and no other; `E8301` and `E8302` are rendered from their templates
by the generated accessors.

| Code | Text |
|---|---|
| E3201 | `value does not fit its sized integer type` |
| E3202 | `value overflows Float32` |
| E3204 | `value outside its refinement range` |
| E4101 | `integer overflow in +` |
| E4101 | `integer overflow in -` |
| E4101 | `integer overflow in *` |
| E4101 | `integer overflow in /` |
| E4101 | `integer overflow in unary -` |
| E4101 | `integer overflow in abs` |
| E4102 | `integer division by zero` |
| E4102 | `integer division by zero in %` |
| E4102 | `duration division by zero` |
| E4103 | `float out of the Int range` |
| E4104 | `float result is not finite` |
| E4108 | `clamp with lo > hi` |
| E8303 | `integer outside the TypeScript safe range` |

---

## 2. Generating the registry

### 2.1 The generator

`go run ./internal/diag/cmd/diaggen` reads `spec/ERRORS.md` and writes `internal/diag/codes.go`,
gofmt-formatted, whose first line is `// Code generated by diaggen from spec/ERRORS.md. DO NOT
EDIT.` (so the auditor skips it, IMPLEMENTATION-PLAN.md §12.1). The same input always gives the
same bytes. The generator reads only the two tables of each `##` section whose title starts with
a code range (their header rows are exactly those of §1.1) and fails, writing nothing, when:

- a code appears in two codes rows, or does not match `^[EW][0-9]{4}$`; its letter disagrees with
  its severity (`W` for `warning`, `E` for `error` and `runtime`); it is one of the retired
  numbers;
- the severity is not `error`, `warning` or `runtime`; the package is not in IMPLEMENTATION-PLAN.md
  §3 (or `gen` for a `runtime` code);
- a codes row has no message row, or a message row names no code of its section;
- the variant rules of §1.1, the `Args` rules (count, names, types) or the grammar of §1.2 are
  broken, or a placeholder and the `Args` list disagree;
- a row of §1.4 lists no code, or a code without a `Kind` argument; a code with a `Kind`
  argument is listed by no row;
- the count sentence ("The catalogue holds …") disagrees with the tables;
- with `-runtime <dir>`, a `Fail`, `OnEvalError` or `canonFail` call in a runtime helper text
  under `<dir>` uses a (code, text) pair that §1.6 does not list (`make check` runs it on
  `internal/gen/*/runtime/`).

`make check` runs the generator into a temporary file and fails when it differs from the
committed `internal/diag/codes.go` (target `diag-check`, IMPLEMENTATION-PLAN.md §12.2). CI never
regenerates; a change to this file and to `codes.go` land in the same commit.

### 2.2 What it generates

```go
package diag

type Severity uint8                  // Error, Warning, Runtime
type Code string                     // "E3501"

type Def struct {
    Code     Code
    Severity Severity
    Package  string                  // "verify"
    Variants []Variant               // one per message row, in table order
}
type Variant struct {
    Name     string                  // "" for a single-message code
    Args     []Arg                   // in constructor order
    Template string                  // §1.2, as written between the backticks: escapes kept (DECISIONS 57)
}
type Arg struct { Name string; Type ArgType }
type ArgType uint8                   // one constant per type of §1.3
type Kind uint8                      // one constant per row of §1.4, with Word()

var Registry = []Def{ /* every code, sorted by Code */ }

// One unexported type per code; the exported variable is the only way to report it.
type codeE3501 struct{}
var E3501 codeE3501
func (codeE3501) Def() *Def
func (codeE3501) At(span source.Span, key ValueArg, coll string) *Builder

// A code with variants has one method per variant: At + the variant's name, UpperCamel.
func (codeE2103) AtNone(span source.Span, typ string) *Builder
func (codeE2103) AtSeveral(span source.Span, typ string, colls []string) *Builder
```

- Each constructor takes the span first, then one parameter per `Args` entry, in order, with the
  Go type of §1.3: at most 5 parameters (DECISIONS 25). An argument name is never a Go keyword or
  predeclared identifier (the generator refuses `type`, `case`, `range`, `package`, `new`, `len`,
  `max`, `error`…); the tables write `typ`, `caseName`, `bound`, `pkg`, `renamed`, `length`,
  `capacity`, `cause`.
- `runtime` codes get a `Def` and no constructor: `canon` never reports them. The code
  generators render their templates into the generated code, or copy them into the runtime helper
  texts (CODEGEN.md §6.3, §7.4, §8.2); the goldens pin both.
- `TypeArg` is `interface{ String() string }`, which `types.Type` satisfies; `ValueArg` is
  `interface{ CanonText() string }`, which `value.Value` satisfies (IMPLEMENTATION-PLAN.md §4.3).
  `diag` imports neither package.
- The builder:

  ```go
  func (b *Builder) Path(p string) *Builder              // canonical value path (API.md F1)
  func (b *Builder) Pointer(p string) *Builder           // RFC 6901 pointer into a loaded JSON file
  func (b *Builder) Related(span source.Span, n Note) *Builder
  func (b *Builder) Check(name string) *Builder
  func (b *Builder) Layer(name string) *Builder
  func (b *Builder) Stack(frames []Frame) *Builder      // innermost first, cut to 16 (EVL-07)
  func (b *Builder) Report(bag *Bag)                     // renders the message and adds the finding
  func (b *Builder) Message() Message                    // renders without reporting (E1703)

  func NoteSource(decl source.Span) Note
  func NoteCheck(name string) Note                       // "" gives the checkUnnamed note
  ```

  `Report` renders the template with the bag's file set (for `Expr` and `Loc`), sets the severity
  from the `Def` and the package from the bag (a `Bag` collects the findings of one package,
  IMPLEMENTATION-PLAN.md §4.4), and never fails. `Message` is an opaque value only `diag` can
  build.

### 2.3 Rules for callers

- A diagnostic is reported only as `diag.<CODE>.At…(…).….Report(bag)`. No other file writes a
  code as a literal, formats a message, or builds a `Finding` by hand (audit rule
  `diag-message-inline`, IMPLEMENTATION-PLAN.md addendum).
- Every code has at least one test that produces it; `TestEveryCodeIsTested` and the audit rule
  `diag-code-untested` enforce it (DECISIONS 27).
- Go-level failures (I/O, a stale revision, API misuse) are sentinel errors in `errors.go`, never
  diagnostics (DECISIONS 27).

---

The catalogue holds 301 codes: 280 errors, 18 warnings and 3 run-time codes, with 465 messages.

## E10xx, W10xx: Project file, doc comments and naming

Owner: GRAMMAR.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E1001 | error | project | GRAMMAR.md §7.1 | `canon` names a language version this compiler does not support |
| W1001 | warning | syntax | GRAMMAR.md §9.1 | a doc comment attaches to nothing (blank line, wrong position, after code) |
| E1002 | error | project | GRAMMAR.md §7.1 | unknown key in `project.canon` |
| W1002 | warning | check | GRAMMAR.md §9.1 | a public type, field or `export fn` has no doc comment |
| E1003 | error | project | GRAMMAR.md §7.1 | no `project.canon` in the directory or its parents (exit 2) |
| W1003 | warning | check | GRAMMAR.md §9.2 | a declared name breaks its naming convention |
| E1004 | error | project | GRAMMAR.md §7.1 | `project.canon` does not declare `canon` |
| E1005 | error | project | GRAMMAR.md §7.1 | a project key or root name given twice |
| E1006 | error | project | GRAMMAR.md §7.1 | a project value of the wrong kind or out of range |
| E1007 | error | project | GRAMMAR.md §7.1 | invalid root name or path (absolute, empty, `\`) |
| E1008 | error | project | GRAMMAR.md §7.1 | invalid or duplicate language code in `languages` |
| E1009 | error | project | GRAMMAR.md §7.1 | `go_module` key is not a declared root, or its module path is empty or invalid |
| E1010 | error | project | GRAMMAR.md §7.1 | `canon` is not `"MAJOR.MINOR"` |
| E1011 | error | syntax | GRAMMAR.md §5.2 | a `project` declaration outside `project.canon`, or anything else inside it |
| E1012 | error | project | GRAMMAR.md §7.1 | `studio` names a package that does not exist |

| Code | Variant | Args | Template |
|---|---|---|---|
| E1001 | - | version:Text, supported:Names | `project requires Canon {version}; this compiler supports {supported}` |
| W1001 | - | - | `doc comment is not attached to anything` |
| E1002 | - | key:Name | `unknown project key "{key}"` |
| W1002 | - | kind:Kind, name:Name | `{kind} {name} has no doc comment` |
| E1003 | - | dir:Path | `no project.canon found in {dir} or its parents` |
| W1003 | - | kind:Kind, name:Name, convention:Kind | `{kind} name "{name}" should be {convention}` |
| E1004 | - | - | `project.canon must declare "canon"` |
| E1005 | key | key:Name | `duplicate key "{key}" in project.canon` |
| E1005 | root | name:Name | `duplicate root "{name}" in project.canon` |
| E1006 | canon | - | `"canon" must be a string` |
| E1006 | roots | - | `"roots" must be a map of root names to path strings` |
| E1006 | languages | - | `"languages" must be a non-empty list of language codes` |
| E1006 | studio | - | `"studio" must be a package name` |
| E1006 | budget | - | `"budget" must be an integer of at least 1` |
| E1006 | goModule | - | `"go_module" must be a map of root names to module path strings` |
| E1007 | name | name:Name | `invalid root name "{name}": a root name is an identifier` |
| E1007 | empty | name:Name | `invalid root "{name}": the path is empty` |
| E1007 | absolute | name:Name | `invalid root "{name}": the path is absolute` |
| E1007 | backslash | name:Name | `invalid root "{name}": "\\" is not a path separator; write "/"` |
| E1008 | invalid | code:Name | `invalid language code "{code}"` |
| E1008 | duplicate | code:Name | `language "{code}" listed twice` |
| E1009 | root | name:Name | `go_module: "{name}" is not a declared root` |
| E1009 | path | name:Name | `go_module: the module path of root "{name}" is empty or contains a space` |
| E1010 | - | value:Text | `"canon" must be "MAJOR.MINOR", found "{value}"` |
| E1011 | outside | - | `a project declaration is only allowed in project.canon` |
| E1011 | inside | - | `project.canon may only contain the project declaration` |
| E1012 | - | pkg:Name | `studio package "{pkg}" does not exist` |

## E11xx: Lexer and parser

Owner: GRAMMAR.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E1101 | error | syntax | GRAMMAR.md §2.6 | invalid format spec in an interpolation (`[+][,][.N]`, N ≤ 20) |
| E1102 | error | syntax | GRAMMAR.md §2.6 | multiline string indentation mixes tabs and spaces |
| E1103 | error | syntax | GRAMMAR.md §5.9 | a type takes a second refinement; use `where` |
| E1104 | error | syntax | GRAMMAR.md §8.2 | unknown annotation or annotation argument |
| E1105 | error | syntax | GRAMMAR.md §5.10 | `fail` or `warn` called outside a `check { }` block |
| E1106 | error | syntax | GRAMMAR.md §1 | a character no token can start with, or a `#` outside an amend-path position segment |
| E1107 | error | syntax | GRAMMAR.md §2.6 | unterminated string |
| E1108 | error | syntax | GRAMMAR.md §2.2 | unterminated block comment |
| E1109 | error | syntax | GRAMMAR.md §2.6 | invalid escape sequence |
| E1110 | error | syntax | GRAMMAR.md §2.4 | invalid number literal (misplaced `_`, leading zero, bad suffix) |
| E1111 | error | syntax | GRAMMAR.md §2.5 | invalid duration literal (unit order, repeated unit, fraction, overflow) |
| E1112 | error | syntax | GRAMMAR.md §2.6 | invalid interpolation (empty, newline or comment inside, unterminated, lone `}`) |
| E1113 | error | syntax | GRAMMAR.md §2.7 | unterminated regular expression |
| E1114 | error | syntax | GRAMMAR.md §2.7 | the regular expression is not valid RE2 |
| E1115 | error | syntax | GRAMMAR.md §2.7 | a regular expression outside a type refinement or `matches()` |
| E1116 | error | syntax | GRAMMAR.md §12 | any other syntax error ("expected …, found …") |
| E1117 | error | syntax | GRAMMAR.md §3.1 | two items on one line without a separator |
| E1118 | error | syntax | GRAMMAR.md §8.1 | an annotation in a position it does not allow |
| E1119 | error | syntax | GRAMMAR.md §8.2 | an annotation argument of the wrong kind or value, missing, or exclusive with another |
| E1120 | error | syntax | GRAMMAR.md §8.1 | the same annotation twice at one position |
| E1121 | error | syntax | GRAMMAR.md §6.7 | a positional argument after a named one, or an argument given twice |
| E1122 | error | syntax | GRAMMAR.md §2.6 | malformed multiline string layout |
| E1123 | error | syntax | GRAMMAR.md §1 | the file starts with a byte order mark (`canon fmt` removes it) |
| E1124 | error | syntax | GRAMMAR.md §1 | invalid UTF-8, a control character, or a stray carriage return |
| E1125 | error | syntax | GRAMMAR.md §4.3 | a reserved word used as a name where it is not allowed |
| E1126 | error | syntax | GRAMMAR.md §4.3 | `none`, `true`, `false` or `self` names an enum member, case or table key |
| E1127 | error | syntax | GRAMMAR.md §5.2 | file structure (no `package`, import after a declaration, a declaration in a layer or translation file) |
| E1128 | error | syntax | GRAMMAR.md §5.11 | comparisons chained (`a < b < c`) |
| E1129 | error | syntax | GRAMMAR.md §6.1 | an `if`/`match` expression or a brace literal at depth 0 of a header |
| E1130 | error | syntax | GRAMMAR.md §5.10 | `expect` outside a test block |
| E1131 | error | syntax | GRAMMAR.md §5.4 | `input` without `from env`, or `from env` without `input` |
| E1132 | error | syntax | GRAMMAR.md §2.6 | a string that must be constant has an interpolation |
| E1133 | error | syntax | GRAMMAR.md §5.3 | `local`, `export` or `retired` on a declaration that does not take it |
| E1134 | error | syntax | GRAMMAR.md §5.10 | `break` or `continue` outside a loop |
| E1135 | error | syntax | GRAMMAR.md §5.10 | `return` outside a function |
| E1136 | error | syntax | GRAMMAR.md §5.11 | a comprehension brace literal without exactly one item |
| E1137 | error | syntax | GRAMMAR.md §5.9 | `keyed by` after a type that is not a list |

| Code | Variant | Args | Template |
|---|---|---|---|
| E1101 | - | spec:Text | `invalid format spec "{spec}": expected [+][,][.N] with N at most 20` |
| E1102 | - | - | `multiline string indentation mixes tabs and spaces` |
| E1103 | - | - | `a type takes one refinement; use "where" for more` |
| E1104 | annotation | name:Name | `unknown annotation @{name}` |
| E1104 | argument | name:Name, arg:Name | `@{name} has no argument "{arg}"` |
| E1105 | - | fn:Name | `{fn}() may only be called inside a check block` |
| E1106 | - | char:Text | `unexpected character {char}` |
| E1107 | - | - | `unterminated string` |
| E1108 | - | - | `unterminated block comment` |
| E1109 | - | seq:Text | `invalid escape sequence "{seq}"` |
| E1110 | - | text:Text | `invalid number literal "{text}"` |
| E1111 | order | text:Text | `invalid duration "{text}": units must decrease (d, h, m, s, ms)` |
| E1111 | repeated | text:Text, unit:Name | `invalid duration "{text}": unit {unit} is repeated` |
| E1111 | fraction | text:Text | `invalid duration "{text}": a fraction is not allowed; use a smaller unit` |
| E1111 | trailing | text:Text | `invalid duration "{text}": digits without a unit` |
| E1111 | overflow | text:Text | `invalid duration "{text}": it does not fit a signed 64-bit count of milliseconds` |
| E1112 | empty | - | `invalid interpolation: {{}} is empty` |
| E1112 | newline | - | `invalid interpolation: a line break inside it` |
| E1112 | comment | - | `invalid interpolation: a comment inside it` |
| E1112 | unterminated | - | `invalid interpolation: no closing }}` |
| E1112 | brace | - | `invalid interpolation: a lone }} in text; write }}}} for a brace` |
| E1113 | - | - | `unterminated regular expression` |
| E1114 | - | cause:Text | `invalid regular expression: {cause}` |
| E1115 | - | - | `a regular expression is only allowed as a type refinement or the argument of matches()` |
| E1116 | token | expected:Names, found:Text | `expected {expected}; found {found}` |
| E1116 | eof | expected:Names | `expected {expected}; found the end of the file` |
| E1117 | - | - | `missing separator: put "," or a new line between items` |
| E1118 | - | name:Name, position:Kind | `@{name} is not allowed on this {position}` |
| E1119 | missing | name:Name, arg:Name | `@{name}: argument "{arg}" is missing` |
| E1119 | kind | name:Name, arg:Name, expected:Kind | `@{name}({arg}): expected {expected}` |
| E1119 | value | name:Name, arg:Name, allowed:Names | `@{name}({arg}): expected one of {allowed}` |
| E1119 | exclusive | name:Name, arg:Name, other:Name | `@{name}: "{arg}" and "{other}" exclude each other` |
| E1119 | needs | name:Name, arg:Name, other:Name | `@{name}: "{arg}" needs "{other}"` |
| E1119 | codes | - | `@json(codes) needs @codes on the same enum` |
| E1120 | - | name:Name | `@{name} given twice; combine the arguments` |
| E1121 | order | - | `positional argument after a named argument` |
| E1121 | twice | name:Name | `argument "{name}" given twice` |
| E1122 | open | - | `multiline string: text after the opening """` |
| E1122 | close | - | `multiline string: the closing """ must be the first text of its line` |
| E1122 | indent | - | `multiline string: this line does not start with the indentation of the closing """` |
| E1123 | - | - | `file starts with a byte order mark` |
| E1124 | utf8 | - | `invalid UTF-8` |
| E1124 | control | char:Rune | `control character {char}` |
| E1124 | cr | - | `stray carriage return` |
| E1125 | - | word:Name, kind:Kind | `"{word}" is a reserved word and cannot name this {kind}` |
| E1126 | - | word:Name | `"{word}" cannot name an enum member, variant case or table key` |
| E1127 | package | - | `a file must start with "package"` |
| E1127 | imports | - | `imports must precede declarations` |
| E1127 | layer | - | `layer files contain only amend declarations` |
| E1127 | translation | - | `translation files contain only translation entries` |
| E1127 | secondPackage | - | `a file has exactly one package clause` |
| E1128 | - | - | `comparisons do not chain: write "a < b and b < c"` |
| E1129 | - | - | `parenthesize this expression: "{{" here would start the block` |
| E1130 | - | - | `expect is only allowed in a test block` |
| E1131 | missing | - | `an input field needs from env "VAR"` |
| E1131 | noInput | - | `"from env" requires "input"` |
| E1132 | - | - | `this string must be constant (no interpolation)` |
| E1133 | - | modifier:Name, kind:Kind | `"{modifier}" is not allowed on this {kind}` |
| E1134 | - | keyword:Name | `{keyword} outside a loop` |
| E1135 | - | - | `return outside a function` |
| E1136 | - | - | `a comprehension literal has exactly one item` |
| E1137 | - | - | `"keyed by" applies to a list type` |

## E16xx, W16xx: Views

Owner: VIEWMODEL.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E1601 | error | views | VIEWMODEL.md §4.5 | a `control:` hint that does not accept the field's type |
| E1602 | error | views | VIEWMODEL.md §3.3 | a view names no field, method, case or member of its target |
| E1603 | error | views | VIEWMODEL.md §3.2 | a view declared outside the package of its target |
| W1604 | warning | views | VIEWMODEL.md §3.4 | a field named `key` or `index` hides the view's magic name |
| E1605 | error | views | VIEWMODEL.md §3.3 | a name, or a view-level item (`title`, `columns`, `filters`, …), placed twice in one view |
| E1606 | error | views | VIEWMODEL.md §3.3 | `columns` or `filters` names something that is not a field |
| E1607 | error | views | VIEWMODEL.md §3.2 | a second view for one target |
| E1608 | error | views | VIEWMODEL.md §3.8 | a widget used on a field whose type does not match its `value` parameter |
| E1609 | error | views | VIEWMODEL.md §4.5 | unknown built-in control name |
| E1610 | error | views | VIEWMODEL.md §3.5 | unknown unit, widget, menu, icon or tone of the studio package |
| E1611 | error | views | VIEWMODEL.md §3.5 | `unit` on a field that is not a number or a list or map of numbers |
| E1612 | error | views | VIEWMODEL.md §3.4 | `preview` is not an asset |
| E1613 | error | views | VIEWMODEL.md §3.5 | unknown or misplaced property of a view item (or a column width out of range) |
| E1614 | error | views | VIEWMODEL.md §3.3 | an item's label declared twice (in one view or two) |
| E1615 | error | views | VIEWMODEL.md §3.4 | an unescaped `{` in a plain view text |
| E1616 | error | views | VIEWMODEL.md §3.3 | a view names a method that takes parameters |
| E1617 | error | views | VIEWMODEL.md §3.6 | duplicate group or show id in a view |
| E1618 | error | views | VIEWMODEL.md §3.6 | a group or show id starting with `_` (reserved) |
| E1619 | error | views | VIEWMODEL.md §4.5 | `slider` on a type without both bounds |
| E1620 | error | views | VIEWMODEL.md §7.3 | a filter on a type that cannot be filtered |
| E1621 | error | views | VIEWMODEL.md §7.3 | `multi` on a filter that is not a choice, case or contains filter |
| E1622 | error | views | VIEWMODEL.md §3.4 | a `search` term of a type that cannot be searched |
| E1623 | error | views | VIEWMODEL.md §3.5 | a `step` text uses a name other than `{index}` |
| E1626 | error | views | VIEWMODEL.md §3.2 | a view on a `let` that is not a `load.defines` table, or on a name that is not a record, variant or enum |
| E1627 | error | views | VIEWMODEL.md §3.2 | a view item not allowed for its target kind |
| E1628 | error | views | VIEWMODEL.md §3.3 | a name matches case fields of different types |
| E1629 | error | views | VIEWMODEL.md §3.8 | two default widgets for one type |
| E1630 | error | views | VIEWMODEL.md §3.8 | a default widget whose `value` type is not a named record, variant or enum |
| E1631 | error | views | VIEWMODEL.md §3.8 | a widget's `siblings` is not a list of its `value` type |
| E1632 | error | views | VIEWMODEL.md §3.9 | `@menu` on something other than a public top-level value |
| E1633 | error | check | VIEWMODEL.md §3.7 | `check … at f` names no field of the record or case |
| E1634 | error | views | VIEWMODEL.md §3.5 | a field has both `control` and `widget` |
| W1640 | warning | views | VIEWMODEL.md §10 | an editable public value has no menu (packages with an `emit view` only) |
| W1641 | warning | views | VIEWMODEL.md §5.5 | a required field without a default is `hidden` |
| W1642 | warning | views | VIEWMODEL.md §5.2 | a deprecated field is named in a group |

| Code | Variant | Args | Template |
|---|---|---|---|
| E1601 | - | control:Name, typ:Type, field:Name | `control {control} does not accept {typ} (field {field})` |
| E1602 | - | target:Name, name:Name | `{target} has no field, method, case or member named {name}` |
| E1603 | - | target:Name, pkg:Name | `a view of {target} must be declared in package {pkg}` |
| W1604 | - | name:Name, typ:Name | `field {name} of {typ} hides the view name {name}: {{{name}}} reads the field` |
| E1605 | - | name:Name, view:Name, first:Loc | `{name} is placed twice in view {view} (first at {first})` |
| E1606 | columns | name:Name, typ:Name | `{name} is not a field of {typ}: columns name fields only` |
| E1606 | filters | name:Name, typ:Name | `{name} is not a field of {typ}: filters name fields only` |
| E1607 | - | target:Name, first:Loc | `{target} already has a view at {first}` |
| E1608 | - | widget:Name, param:Type, typ:Type, field:Name | `widget {widget} takes {param}, not {typ} (field {field})` |
| E1609 | - | name:Name | `{name} is not a built-in control (VIEWMODEL.md §4.5 lists them)` |
| E1610 | - | kind:Kind, name:Name, pkg:Name | `unknown {kind} {name}: not declared in studio package {pkg}` |
| E1611 | - | typ:Type | `unit applies to numbers and to lists and maps of numbers, not {typ}` |
| E1612 | - | typ:Type | `preview must be an asset, not {typ}` |
| E1613 | property | property:Name, item:Kind | `{property} is not a property of this {item}` |
| E1613 | width | width:Int | `column width {width} is out of range: widths go from 16 to 2000` |
| E1613 | twice | property:Name, first:Loc | `{property} is given twice for this item (first at {first})` |
| E1613 | literal | property:Name | `{property} takes a literal here, not an expression` |
| E1614 | - | field:Name, first:Loc | `the label of {field} is declared twice (first at {first})` |
| E1615 | - | - | `{{ in plain text: labels, help, intros, placeholders and none texts are not templates (write {{{{)` |
| E1616 | - | name:Name | `method {name} takes parameters: a view may name only methods without parameters` |
| E1617 | group | id:Name, view:Name, first:Loc | `duplicate group id {id} in view {view} (first at {first})` |
| E1617 | show | id:Name, view:Name, first:Loc | `duplicate show id {id} in view {view} (first at {first})` |
| E1618 | - | id:Name | `{id} is reserved: ids starting with _ belong to the compiler` |
| E1619 | lower | typ:Type | `slider needs both bounds, and {typ} has no lower bound` |
| E1619 | upper | typ:Type | `slider needs both bounds, and {typ} has no upper bound` |
| E1620 | - | typ:Type, field:Name | `cannot filter on {typ} (field {field})` |
| E1621 | - | typ:Type | `multi applies to enum, ref, case and list filters, not {typ}` |
| E1622 | - | typ:Type | `a search term must be text, a number, an enum, a ref or an asset, not {typ}` |
| E1623 | - | name:Name | `a step text may only use {{index}}, not {{{name}}}` |
| E1626 | let | name:Name | `{name} is a value that does not come from load.defines: only define tables take a view` |
| E1626 | type | name:Name | `{name} is not a record, variant or enum: a view targets one of these or a load.defines table` |
| E1627 | - | item:Name, target:Kind | `{item} is not allowed in the view of this {target}` |
| E1628 | - | name:Name, cases:Names | `{name} names case fields of different types ({cases}): name them in the case views` |
| E1629 | - | typ:Name, first:Name, second:Name | `{typ} has two default widgets: {first} and {second}` |
| E1630 | - | typ:Type | `a default widget needs a named record, variant or enum type, not {typ}` |
| E1631 | - | param:Type, typ:Type | `siblings must be a list of the value type: expected [{param}], got {typ}` |
| E1632 | - | - | `@menu applies to public top-level values only` |
| E1633 | - | field:Name, typ:Name | `at {field}: {typ} has no field {field}` |
| E1634 | - | field:Name | `field {field} has both control and widget` |
| W1640 | - | value:Name | `value {value} has no menu: the studio lists it under "No menu"` |
| W1641 | - | field:Name | `required field {field} is hidden: new values cannot be completed in the studio` |
| W1642 | - | field:Name, group:Name | `deprecated field {field} is named in group {group}; it is shown under "Unused fields"` |

## E17xx, W17xx: Translations

Owner: I18N.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| W1701 | warning | i18n | I18N.md §5 | a package that emits a view has translations missing in a language (one per package and language) |
| E1702 | error | i18n | I18N.md §4 | a translation key that matches no text of the package |
| E1703 | error | check | I18N.md §7 | a translated template does not type-check |
| E1704 | error | i18n | I18N.md §4 | a translation file for a language not in `project.languages` |
| E1705 | error | i18n | I18N.md §4 | a translation key given twice |
| E1706 | error | i18n | I18N.md §4 | a translation file for the source language |
| E1707 | error | i18n | I18N.md §4 | the translation of a plain text interpolates |

| Code | Variant | Args | Template |
|---|---|---|---|
| W1701 | one | pkg:Name, lang:Name | `1 text of package {pkg} has no {lang} translation (canon i18n status {pkg} --lang {lang} --list)` |
| W1701 | many | n:Int, pkg:Name, lang:Name | `{n} texts of package {pkg} have no {lang} translation (canon i18n status {pkg} --lang {lang} --list)` |
| E1702 | plain | key:Name, pkg:Name | `translation key {key} matches no text of package {pkg}` |
| E1702 | noLetter | key:Name, pkg:Name | `translation key {key} matches no text of package {pkg}: its source text has no letter, so it is not translatable` |
| E1702 | form | key:Name, pkg:Name, hint:Name | `translation key {key} matches no text of package {pkg}; write {hint}` |
| E1703 | type | key:Name, detail:Message | `translation of {key}: {detail}` |
| E1703 | step | key:Name, expr:Expr | `translation of {key}: a step text may only use {{index}}, not {{{expr}}}` |
| E1704 | - | lang:Name, languages:Names | `language {lang} is not in project.languages ({languages})` |
| E1705 | - | key:Name, first:Loc | `translation key {key} is given twice (first at {first})` |
| E1706 | - | lang:Name | `{lang} is the source language: its texts are the sources, not a translation file` |
| E1707 | - | key:Name | `{key} is plain text: its translation cannot interpolate (write {{{{ for a brace)` |

## E19xx: Layers and runtime inputs

Owner: EVALUATION.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E1901 | error | build | EVALUATION.md §9.1 | `--layer` names a layer no loaded package declares (exit 2) |
| E1902 | error | check | EVALUATION.md §9.2 | `amend` targets a `const`, function or type |
| E1903 | error | check | EVALUATION.md §11.1 | a record with input fields is not reached from exactly one public value through record fields |
| E1904 | error | check | EVALUATION.md §11.3 | an input pattern outside the portable RE2 ∩ ECMAScript subset |
| E1905 | error | check | EVALUATION.md §9.2 | an amend path that does not fit the type, or does not exist when applied |
| E1906 | error | check | EVALUATION.md §9.1 | one package declares a layer name twice |
| E1907 | error | check | EVALUATION.md §11.1 | an input field has a default |
| E1908 | error | check | EVALUATION.md §9.2 | an amend path set twice, or overlapping another, in one layer |
| E1909 | error | check | EVALUATION.md §9.2 | a layer amends a value of another package |
| E1910 | error | check | EVALUATION.md §11.1 | an input type that cannot be read or checked at runtime |
| E1911 | error | check | EVALUATION.md §11.1 | invalid environment variable name |

| Code | Variant | Args | Template |
|---|---|---|---|
| E1901 | - | name:Name | `no loaded package declares layer {name}` |
| E1902 | - | name:Name | `amend {name}: only a let can be amended` |
| E1903 | - | record:Name | `{record} has input fields, so it must be reached from exactly one public value through record fields` |
| E1904 | - | re:Text, construct:Text | `pattern /{re}/ is not in the portable subset: {construct}` |
| E1905 | field | path:Name | `amend path {path}: no such field` |
| E1905 | key | path:Name, key:Value | `amend path {path}: key {key} does not exist` |
| E1905 | index | path:Name, index:Int | `amend path {path}: index {index} out of range` |
| E1905 | none | path:Name | `amend path {path}: passes through none` |
| E1905 | caseField | path:Name, caseName:Name, field:Name | `amend path {path}: case {caseName} has no field {field}` |
| E1906 | - | pkg:Name, name:Name, first:Loc | `package {pkg} declares layer {name} twice (first at {first})` |
| E1907 | - | field:Name | `input {field} cannot have a default` |
| E1908 | twice | path:Name, layer:Name | `amend path {path} is set twice in layer {layer}` |
| E1908 | overlap | path:Name, other:Name, layer:Name | `amend path {path} overlaps {other} in layer {layer}` |
| E1909 | - | name:Name | `amend {name}: layers amend values of their own package only` |
| E1910 | - | field:Name, typ:Type | `input {field}: type {typ} cannot be read or checked at run time` |
| E1911 | - | name:Text | `{name} is not a valid environment variable name` |

## E2xxx: Packages and names

Owner: TYPES.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E2001 | error | build | TYPES.md §3.1 | one directory holds files of two packages, neither being the directory's |
| E2002 | error | check | TYPES.md §3.1 | import cycle |
| E2003 | error | check | TYPES.md §3.1 | import of a package that does not exist |
| E2004 | error | check | TYPES.md §3.1 | selective import of a missing or `local` name |
| E2005 | error | check | TYPES.md §3.1 | one name bound twice in one file by imports, or by an import and a declaration of the package |
| E2006 | error | check | TYPES.md §3.1 | a file declares a package that is not its directory's or an ancestor's |
| E2101 | error | check | TYPES.md §4.2 | a contextual name is ambiguous with a local, parameter or field |
| E2102 | error | check | TYPES.md §3.3 | unknown name |
| E2103 | error | check | TYPES.md §10.2 | `ref T` finds no collection of `T`, or several |
| E2104 | error | check | TYPES.md §3.6 | a field and a method with the same name |
| E2105 | error | check | TYPES.md §3.6 | a reserved member name: `id`/`retired` on a table element, `kind` on a case |
| E2106 | error | check | TYPES.md §3.2 | a name declared twice in one namespace |
| E2107 | error | check | TYPES.md §3.4 | a name declared twice in one block |
| E2108 | error | check | TYPES.md §3.4 | `self` outside a record, case or variant body |
| E2109 | error | check | TYPES.md §3.4 | `it` outside a refinement predicate |
| E2110 | error | check | TYPES.md §3.2 | a type name used as a value, or a value name as a type |
| E2111 | error | check | TYPES.md §3.7 | an emitted public declaration exposes a `local` type |

| Code | Variant | Args | Template |
|---|---|---|---|
| E2001 | - | dir:Path, a:Name, b:Name | `directory {dir} holds files of packages {a} and {b}` |
| E2002 | - | cycle:Chain | `import cycle: {cycle}` |
| E2003 | - | pkg:Name | `unknown package {pkg}` |
| E2004 | - | pkg:Name, name:Name | `{pkg} has no public {name}` |
| E2005 | - | name:Name, first:Loc | `{name} is bound twice in this file (first at {first})` |
| E2006 | - | dir:Path, pkg:Name | `a file in {dir} declares package {pkg}, which is neither that directory's package nor an ancestor of it` |
| E2101 | - | name:Name, qualified:Name, kind:Kind | `{name} is ambiguous: {qualified} or the {kind} {name}; write it qualified` |
| E2102 | plain | name:Name | `unknown name {name}` |
| E2102 | hint | name:Name, hint:Name | `unknown name {name} (did you mean {hint}?)` |
| E2103 | none | typ:Name | `ref {typ}: there is no collection of {typ}; write ref <collection name>` |
| E2103 | several | typ:Name, colls:Names | `ref {typ}: several collections of {typ} ({colls}); write ref <collection name>` |
| E2104 | - | record:Name, name:Name | `{record} has a field and a method named {name}` |
| E2105 | entry | name:Name | `{name} is reserved on table entries` |
| E2105 | case | name:Name | `{name} is reserved on variant cases` |
| E2106 | - | name:Name, first:Loc | `{name} is declared twice (first at {first})` |
| E2107 | - | name:Name | `{name} is already declared in this block` |
| E2108 | - | - | `self is only valid in a record, case or variant body` |
| E2109 | - | - | `it is only valid in a refinement predicate` |
| E2110 | type | name:Name | `{name} is a type, not a value` |
| E2110 | value | name:Name | `{name} is a value, not a type` |
| E2111 | - | decl:Name, typ:Name | `{decl} is public but exposes the local type {typ}: make {typ} public or {decl} local` |

## E3xxx, W3xxx: Types, literals, refs, assets, dependent types

Owner: TYPES.md, WIRE.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E3001 | error | check | TYPES.md §15 | a public top-level `let` without a type annotation |
| E3002 | error | check | TYPES.md §6.2 | a value of one type where another is expected |
| E3003 | error | check | TYPES.md §3.5 | unknown field, method or member (including a case field on an un-narrowed variant) |
| E3004 | error | check | TYPES.md §12.2 | too many, missing or unknown arguments in a call |
| E3005 | error | check | TYPES.md §12.2 | calling something that is not a function |
| E3006 | error | check | TYPES.md §12.1 | a function can finish without returning |
| E3007 | error | check | TYPES.md §7.1 | an operator not defined for its operand types |
| E3008 | error | check | TYPES.md §5.1 | the type of an expression cannot be inferred |
| E3010 | error | check | TYPES.md §15 | a field default reads something other than constants, earlier fields, parameters and built-ins |
| E3011 | error | check | TYPES.md §9.2 | a type that cannot be a map key |
| E3012 | error | check | TYPES.md §9.1 | `keyed by` names no field, or a field whose type cannot be a key |
| E3013 | error | check | TYPES.md §9.3 | a table of something that is not a record |
| E3015 | error | check | TYPES.md §15 | a position that needs a constant expression gets something else |
| E3016 | error | check | TYPES.md §12.3 | a method or built-in used as a value |
| E3017 | error | check | TYPES.md §12.7 | assignment to something that is not a `var` |
| E3018 | error | check | TYPES.md §12.7 | `for a, b in` over something that is not a map or pairs |
| E3019 | error | check | TYPES.md §12.1 | `return` without a value in a function |
| E3020 | error | check | TYPES.md §12.7 | an expression statement that has no effect |
| E3021 | error | check | TYPES.md §13.1 | a type alias refers to itself |
| E3022 | error | check | TYPES.md §13.1 | a record contains itself and can never be built |
| E3023 | error | check | TYPES.md §7.4 | a refinement not valid on its type, or an empty range |
| E3025 | error | check | TYPES.md §13.3 | a `Range` value without `Int` bounds or without a start |
| E3101 | error | check | TYPES.md §9.3 | duplicate table key |
| E3102 | error | verify | TYPES.md §9.1 | a key, `@codes` code or `@stable` value used twice |
| E3103 | error | check | TYPES.md §9.3 | `entry t.k` where `t` is not a table or keyed list of the package or not initialized by a table or list literal, or `retired entry` on a keyed list |
| E3201 | error | check | TYPES.md §7.2 | a value outside its sized integer type, the `Duration` range or the `@codes` type |
| E3202 | error | verify | TYPES.md §7.3 | a NaN or infinite value stored, or a `Float32` overflow |
| E3203 | error | wire | WIRE.md §5.1 | a unit-scaled duration on the wire that is not a whole number of milliseconds |
| E3204 | error | verify | TYPES.md §7.4 | a value outside its range or length refinement |
| E3205 | error | verify | TYPES.md §7.4 | a string that does not match its regex refinement |
| E3206 | error | verify | TYPES.md §7.4 | a value that fails its `where` predicate |
| E3301 | error | check | TYPES.md §5.2 | unknown field in a literal or in loaded data |
| W3301 | warning | check | TYPES.md §16 | use of a deprecated field, member or entry in Canon source |
| E3302 | error | check | TYPES.md §5.2 | missing required field |
| E3303 | error | check | TYPES.md §5.2 | a spread of another type |
| E3304 | error | check | TYPES.md §5.2 | an identifier map key where the key type is `String` or an integer |
| E3305 | error | check | TYPES.md §5.2 | a brace literal whose kind cannot be decided |
| E3306 | error | check | TYPES.md §12.3 | a function type where it is not allowed |
| E3307 | error | check | TYPES.md §12.7 | assignment to a field (records are values) |
| E3308 | error | check | TYPES.md §6.4 | branches with incompatible types |
| E3309 | error | check | TYPES.md §7.5 | comparison of refs into different collections |
| E3310 | error | check | TYPES.md §7.5 | ordering on a type that is not orderable |
| E3311 | error | check | TYPES.md §5.3 | an `Int` expression where a `Float` is expected |
| E3312 | error | check | TYPES.md §14 | a value given to an input field (literal or loaded data) |
| E3313 | error | check | TYPES.md §14 | an input field read at build time |
| E3314 | error | check | TYPES.md (STDLIB.md §4.3) | `sum()` of a list whose element type is unknown |
| E3315 | error | wire | WIRE.md §5.4 | `null` where a non-optional value is required |
| E3316 | error | check | WIRE.md §4.1 | an `@json` form that does not apply to the field's type, or colliding wire keys |
| E3317 | error | wire | WIRE.md §5.8 | two map keys that encode to the same text |
| E3318 | error | check | WIRE.md §4.2 | an inline variant's or a tag's keys collide with the parent's |
| E3320 | error | check | TYPES.md §5.2 | an item kind not allowed in that kind of brace literal |
| E3321 | error | check | TYPES.md §5.2 | a field given twice in a literal |
| E3322 | error | check | TYPES.md §5.2 | a key given twice in a map literal or comprehension |
| E3323 | error | check | TYPES.md §5.2 | a spread that is not first, or appears twice |
| E3401 | error | check | TYPES.md §2 | `T??` is not a type |
| W3401 | warning | check | TYPES.md §6.5 | `!`, `?.`, `??` or a `none` test on a value that is never `none` |
| E3402 | error | check | TYPES.md §6.5 | member access, call, index, iteration or arithmetic on a value that may be `none` |
| E3403 | error | check | TYPES.md §6.5 | a `T?` where a `T` is expected |
| E3501 | error | verify | TYPES.md §10.3 | a ref to a key that does not exist |
| E3502 | error | verify | TYPES.md §10.3 | a live entry references a retired one |
| E3503 | error | eval | TYPES.md §6.2 | a `T` converted to `ref T` is not an entry of the target |
| E3504 | error | check | TYPES.md §10.2 | `ref X` where `X` is not a collection or a record type |
| E3505 | error | eval | TYPES.md §10.2 | a per-instance ref with no enclosing instance to resolve against |
| E3506 | error | verify | TYPES.md §8.1 | a retired enum member or variant case used in a value |
| E3601 | error | check | TYPES.md §12.6 | a `match` that does not cover every member or case |
| W3601 | warning | check | TYPES.md §12.6 | an unreachable `_` arm |
| E3602 | error | check | TYPES.md §12.6 | a `match` pattern already covered |
| E3603 | error | check | TYPES.md §12.6 | a pattern that is not a member or case of the scrutinee's type |
| E3604 | error | check | TYPES.md §12.6 | `match` on a type that cannot be matched |
| E3605 | error | check | TYPES.md §8.3 | `is` with a name that is not a case of the variant, or on a non-variant |
| E3701 | error | verify | TYPES.md §13.4 | an asset file that does not exist (letter case included) |
| E3702 | error | verify | TYPES.md §13.4 | an asset with an extension not allowed |
| E3703 | error | verify | TYPES.md §13.4 | an asset path that is not a clean relative path |
| E3704 | error | check | TYPES.md §13.4 | invalid `asset(…)` type arguments |
| E3801 | error | verify | TYPES.md §11.6 | a non-optional field whose computed type is `Never` |
| E3802 | error | verify | TYPES.md §11.6 | a value that does not match its computed dependent type |
| E3803 | error | check | TYPES.md §11.2 | a type-function argument or scrutinee that is not a path rooted at a parameter |
| E3804 | error | check | TYPES.md §11.4 | an operation not available on a dependent value |
| E3805 | error | check | TYPES.md §11.1 | a field type that uses a later field |
| E3806 | error | check | TYPES.md §11.1 | wrong number or types of arguments to a parameterized type, or a parameter of another kind |

| Code | Variant | Args | Template |
|---|---|---|---|
| E3001 | - | name:Name | `public value {name} needs a type annotation` |
| E3002 | - | expected:Type, found:Type | `expected {expected}, found {found}` |
| E3003 | - | typ:Type, kind:Kind, name:Name | `{typ} has no {kind} {name}` |
| E3004 | many | fn:Name | `too many arguments in call to {fn}` |
| E3004 | missing | param:Name, fn:Name | `missing argument {param} in call to {fn}` |
| E3004 | unknown | param:Name, fn:Name | `unknown argument {param} in call to {fn}` |
| E3005 | - | typ:Type | `{typ} is not callable` |
| E3006 | - | fn:Name, result:Type | `{fn} can finish without returning a {result}` |
| E3007 | binary | op:Name, a:Type, b:Type | `operator {op} is not defined for {a} and {b}` |
| E3007 | unary | op:Name, a:Type | `operator {op} is not defined for {a}` |
| E3008 | - | - | `cannot infer the type of this expression; add a type` |
| E3010 | - | - | `a default may use only constants, earlier fields, parameters and built-ins` |
| E3011 | - | typ:Type | `{typ} cannot be a map key` |
| E3012 | field | field:Name | `keyed by {field}: no such field` |
| E3012 | type | field:Name, typ:Type | `keyed by {field}: type {typ} cannot be a key` |
| E3013 | - | typ:Type | `a table holds records; {typ} is not a record` |
| E3015 | - | what:Kind | `this {what} must be a constant expression` |
| E3016 | - | kind:Kind, name:Name | `{kind} {name} cannot be used as a value; write a lambda` |
| E3017 | - | name:Name | `cannot assign to {name}: it is not a var` |
| E3018 | - | a:Name, b:Name, typ:Type | `for {a}, {b} in needs a map or pairs, found {typ}` |
| E3019 | - | fn:Name, result:Type | `return in {fn} needs a value of type {result}` |
| E3020 | - | - | `this expression has no effect` |
| E3021 | - | alias:Name | `type alias {alias} refers to itself` |
| E3022 | - | record:Name | `{record} contains itself and can never be built` |
| E3023 | invalid | refinement:Expr, typ:Type | `refinement {refinement} is not valid on {typ}` |
| E3023 | empty | refinement:Expr | `range {refinement} is empty` |
| E3025 | - | - | `a Range value must have Int bounds and a start` |
| E3101 | - | key:Name, table:Name, first:Loc | `duplicate key {key} in {table} (first at {first})` |
| E3102 | key | key:Value, first:Loc | `key {key} is used twice (first at {first})` |
| E3102 | code | code:Int, first:Loc | `code {code} is used twice (first at {first})` |
| E3102 | stable | value:Value, first:Loc | `@stable value {value} is used twice (first at {first})` |
| E3103 | target | table:Name, key:Name | `entry {table}.{key}: {table} is not a table or keyed list of this package` |
| E3103 | literal | table:Name, key:Name | `entry {table}.{key}: {table} must be initialized with a table or list literal` |
| E3103 | retired | - | `retired entry needs a table` |
| E3201 | - | value:Value, typ:Type | `{value} does not fit {typ}` |
| E3202 | nonFinite | value:Value | `{value} is not a finite number` |
| E3202 | float32 | value:Value | `{value} overflows Float32` |
| E3203 | - | value:Text, unit:Name | `{value} is not a whole number of milliseconds ({unit} on the wire)` |
| E3204 | - | value:Value, bound:Expr | `{value} is outside {bound}` |
| E3205 | - | value:Value, re:Text | `{value} does not match /{re}/` |
| E3206 | - | value:Value, pred:Expr | `{value} fails where {pred}` |
| E3301 | - | typ:Type, field:Name | `{typ} has no field {field}` |
| W3301 | plain | name:Name | `{name} is deprecated` |
| W3301 | reason | name:Name, reason:Text | `{name} is deprecated: {reason}` |
| E3302 | - | typ:Type, field:Name | `{typ} needs field {field}` |
| E3303 | - | from:Type, to:Type | `cannot spread a {from} into a {to}` |
| E3304 | - | name:Name | `map key {name} would be a variable; write "{name}": or ({name}):` |
| E3305 | - | - | `cannot tell what this {{ … }} is; give it a type` |
| E3306 | - | - | `a function type is not allowed here` |
| E3307 | - | field:Name | `fields cannot be assigned; rebuild the value with {{ ...x, {field}: … }}` |
| E3308 | - | a:Type, b:Type | `branches have incompatible types {a} and {b}` |
| E3309 | - | a:Type, b:Type | `{a} and {b} reference different collections` |
| E3310 | - | op:Name, typ:Type | `{op} needs an orderable type, found {typ}` |
| E3311 | - | expr:Expr | `{expr} is an Int; write Float({expr})` |
| E3312 | - | field:Name | `{field} is an input: it cannot be given a value` |
| E3313 | - | field:Name | `{field} is an input: it has no value at build time` |
| E3314 | - | - | `cannot sum a list whose element type is unknown` |
| E3315 | field | field:Name | `null for {field}, which is not optional` |
| E3315 | type | typ:Type | `null where {typ} is required` |
| E3316 | collision | key:Name, field:Name, other:Name | `wire key {key} of {field} collides with {other}` |
| E3316 | prefix | field:Name, other:Name | `the key path of {field} is a prefix of the key path of {other}` |
| E3316 | dollar | key:Name, field:Name | `wire key {key} of {field} starts with $, which only export functions use` |
| E3316 | form | form:Name, typ:Type | `@json({form}) does not apply to {typ}` |
| E3316 | path | path:Text | `@json(path: "{path}") needs two or more non-empty segments, none starting with $` |
| E3316 | bits | enum:Name | `@json(bits) needs every code of {enum} to be a power of two from 1 to 2^62` |
| E3316 | pairsBound | field:Name | `@json(pairs:) on {field} needs a list type with a finite upper length bound` |
| E3316 | pairsTemplate | template:Text | `@json(pairs:) template "{template}" must contain {{i}} exactly once, differ from the other one and expand to keys that start with no $ and collide with no other key` |
| E3316 | pairsDefault | field:Name | `@json(pairs:) on {field} allows no default but []` |
| E3317 | - | a:Value, b:Value, text:Text | `map keys {a} and {b} both encode to "{text}"` |
| E3318 | inline | key:Name, variant:Name, field:Name | `{key} of inline variant {variant} collides with {field}` |
| E3318 | twoInline | - | `a record may have only one @json(inline) field` |
| E3318 | tag | field:Name, tag:Name | `case field {field} uses the tag key "{tag}"` |
| E3320 | - | item:Kind, literal:Kind | `{item} items are not allowed in {literal} literals` |
| E3321 | - | field:Name | `field {field} is given twice` |
| E3322 | - | key:Value | `key {key} is given twice` |
| E3323 | - | - | `a spread must come first and appear once` |
| E3401 | - | typ:Type | `{typ}? is not a type: {typ} is already optional` |
| W3401 | - | expr:Expr, op:Name | `{expr} is never none: {op} has no effect` |
| E3402 | - | expr:Expr | `{expr} may be none: use ?., !, ?? or test != none first` |
| E3403 | - | typ:Type, found:Type | `expected {typ}, found {found}: prove it is present (!= none, !, ??)` |
| E3501 | - | key:Value, coll:Name | `unknown key {key} in {coll}` |
| E3502 | - | key:Value, entry:Name | `{key} is retired; live {entry} cannot reference it` |
| E3503 | - | typ:Type, coll:Name | `this {typ} is not an entry of {coll}` |
| E3504 | - | name:Name | `ref {name}: {name} is not a collection or a record type` |
| E3505 | - | typ:Name, record:Name | `ref {typ} has no enclosing {record} to resolve against` |
| E3506 | - | typ:Name, name:Name | `{typ}.{name} is retired` |
| E3601 | - | missing:Names | `match does not cover {missing}; add them or _` |
| W3601 | - | - | `_ is unreachable: every case is covered` |
| E3602 | - | pattern:Expr | `pattern {pattern} is already covered` |
| E3603 | - | pattern:Expr, kind:Kind, typ:Type | `{pattern} is not a {kind} of {typ}` |
| E3604 | - | typ:Type | `cannot match on {typ}` |
| E3605 | case | name:Name, variant:Type | `{name} is not a case of {variant}` |
| E3605 | notVariant | typ:Type | `is needs a variant, found {typ}` |
| E3701 | - | path:Text, root:Path | `asset {path} not found under {root}` |
| E3702 | - | path:Text, exts:Names | `asset {path}: extension must be one of {exts}` |
| E3703 | - | path:Text | `asset path {path} is not a clean relative path` |
| E3704 | root | root:Text | `invalid asset(…): "{root}" is not a load path` |
| E3704 | ext | ext:Text | `invalid asset(…): "{ext}" is not a bare extension` |
| E3704 | missing | - | `invalid asset(…): it needs a root and ext` |
| E3801 | - | field:Name, dep:Expr | `{field} has type {dep} = Never: this value cannot be built` |
| E3802 | - | dep:Expr, typ:Type | `value does not match {dep} = {typ}` |
| E3803 | argument | - | `a type-function argument must be a path rooted at a parameter` |
| E3803 | scrutinee | - | `a type function must match on a path rooted at a parameter, of an enum or Bool type` |
| E3804 | - | op:Name, fn:Name | `{op} is not available on a dependent value ({fn}(*))` |
| E3805 | - | field:Name | `field type may only use earlier fields: {field}` |
| E3806 | arity | fn:Name, n:Int, types:Types | `{fn} takes {n} arguments of types {types}` |
| E3806 | param | name:Name, typ:Type | `type parameter {name} must be a record, a ref, an enum or Bool, found {typ}` |

## E4xxx: Evaluation and the standard library

Owner: EVALUATION.md, STDLIB.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E4001 | error | eval | EVALUATION.md §7.1 | postfix `!` on `none` (the rule: TYPES.md §6.5) |
| E4002 | error | eval | EVALUATION.md §4.1 | missing key, index out of range, open range without an end, or slice out of range |
| E4101 | error | eval | EVALUATION.md §6.1 | integer or duration overflow |
| E4102 | error | eval | EVALUATION.md §6.1 | division by zero |
| E4103 | error | eval | EVALUATION.md §6.2 | a float converted to an integer does not fit `Int` |
| E4104 | error | eval | EVALUATION.md §6.2 | a float operation produces NaN or an infinity |
| E4105 | error | eval/std | STDLIB.md §4.2 | `zip` of sequences of different lengths |
| E4106 | error | eval/std | STDLIB.md §7 | `split` or `replace` with an empty separator or pattern |
| E4107 | error | eval/std | STDLIB.md §7 | a string slice bound inside a UTF-8 character |
| E4108 | error | eval/std | STDLIB.md §2.2 | `clamp` with `lo > hi` |
| E4201 | error | eval | EVALUATION.md §4.5 | internal: write to a frozen value (a compiler bug, exit 3) |
| E4301 | error | eval | EVALUATION.md §3.2 | a cycle between values |
| E4401 | error | eval | EVALUATION.md §12.2 | the evaluation step budget is exhausted |
| E4402 | error | eval | EVALUATION.md §3.3 | call depth exceeds 10 000 |
| E4501 | error | eval/std | STDLIB.md §2.3 | `topoSort` meets a cycle |
| E4502 | error | eval/std | STDLIB.md §4.2 | `toMap` produces a key twice |
| E4503 | error | check | STDLIB.md §9.5 | a value that cannot be formatted (format spec on a non-number, a function value) |

| Code | Variant | Args | Template |
|---|---|---|---|
| E4001 | - | expr:Expr | `{expr} is none` |
| E4002 | index | index:Int, n:Int | `index {index} out of range for length {n}` |
| E4002 | key | key:Value, coll:Name | `no key {key} in {coll}` |
| E4002 | open | - | `open range has no end` |
| E4002 | slice | a:Int, b:Int | `slice {a}..{b} out of range` |
| E4101 | integer | expr:Expr | `integer overflow in {expr}` |
| E4101 | duration | expr:Expr | `duration overflow in {expr}` |
| E4102 | - | - | `division by zero` |
| E4103 | - | value:Value | `{value} does not fit an Int` |
| E4104 | - | expr:Expr | `{expr} is not a finite number` |
| E4105 | - | a:Int, b:Int | `zip: lengths differ ({a} and {b})` |
| E4106 | separator | method:Name | `{method}: the separator is empty` |
| E4106 | pattern | method:Name | `{method}: the pattern is empty` |
| E4107 | - | index:Int | `slice bound {index} is inside a UTF-8 character` |
| E4108 | - | lo:Value, hi:Value | `clamp: lower bound {lo} is above upper bound {hi}` |
| E4201 | - | - | `internal: write to a frozen value` |
| E4301 | - | cycle:Chain | `cycle between values: {cycle}` |
| E4401 | - | n:Int, root:Name, k:Int | `evaluation budget of {n} steps exhausted\nheaviest: {root} ({k} steps)` |
| E4402 | - | - | `call depth exceeds 10000` |
| E4501 | - | cycle:Chain | `topoSort: cycle {cycle}` |
| E4502 | - | key:Value | `toMap: key {key} produced twice` |
| E4503 | plain | typ:Type | `cannot format {typ}` |
| E4503 | spec | typ:Type, spec:Text | `cannot format {typ} with :{spec}` |

## E5xxx, W5xxx: Checks and tests

Owner: EVALUATION.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E5001 | error | rules | EVALUATION.md §8.3 | a one-line `check` is false |
| W5001 | warning | rules | EVALUATION.md §8.3 | a one-line `warn` is false |
| E5002 | error | rules | EVALUATION.md §8.3 | `fail(at, message)` in a check block |
| W5002 | warning | rules | EVALUATION.md §8.3 | `warn(at, message)` in a check block |
| E5003 | error | rules | EVALUATION.md §8.3 | a check name used twice in one scope |
| E5004 | error | rules | EVALUATION.md §10.3 | `expect … fails name` names no check that applies |
| E5005 | error | rules | EVALUATION.md §10.1 | a test name declared twice in one package |

| Code | Variant | Args | Template |
|---|---|---|---|
| E5001 | - | message:Text | `{message}` |
| W5001 | - | message:Text | `{message}` |
| E5002 | - | message:Text | `{message}` |
| W5002 | - | message:Text | `{message}` |
| E5003 | - | name:Name, scope:Name | `check name {name} is used twice in {scope}` |
| E5004 | - | name:Name, typ:Type | `no check named {name} applies to {typ}` |
| E5005 | - | name:Text, pkg:Name | `test "{name}" is declared twice in {pkg}` |

## E6xxx, W6xxx: Stable ids and canon.lock

Owner: LOCK.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E6001 | error | lock | LOCK.md §4.1 | a locked id, code or stable value was removed or renamed |
| E6002 | error | lock | LOCK.md §4.2 | a locked value reused, moved, changed or un-retired, or conflicting lock lines |
| E6003 | error | check | LOCK.md §1 | `stable table` or `@stable` in a position that does not allow it |
| E6004 | error | lock | LOCK.md §6.1 | a layer adds a stable entry or changes a stable value |
| E6005 | error | lock | LOCK.md §2.4 | `canon.lock` cannot be read (header, version, syntax, merge markers) |
| W6006 | warning | lock | LOCK.md §8 | stable values not yet in `canon.lock` (`canon lock check` only) |

| Code | Variant | Args | Template |
|---|---|---|---|
| E6001 | removed | holder:Name | `{holder} was removed; retire it instead` |
| E6001 | renamed | previous:Name, renamed:Name, key:Name | `{previous} was removed; {renamed} is new: renaming a stable id is not allowed.\nKeep {key} and retire it: retired {key} {{ … }}` |
| E6001 | held | previous:Name, kind:Kind, value:Value, renamed:Name | `{previous} was removed; its {kind} {value} is now held by {renamed}: renaming is not allowed` |
| E6001 | gone | kind:Kind, name:Name | `stable {kind} {name} is gone (renamed or no longer stable); update canon.lock in the same commit if this is a rename` |
| E6001 | remove | key:Name | `stable entry {key} cannot be removed, only retired` |
| E6002 | unretire | kind:Kind, holder:Name | `retired {kind} {holder} cannot come back` |
| E6002 | renumber | member:Name, code:Int | `{member} renumbered: code {code} is locked` |
| E6002 | codeTaken | code:Int, member:Name | `code {code} belongs to {member}` |
| E6002 | changed | field:Name, key:Name, previous:Value, renamed:Value | `stable value {field} of {key} changed from {previous} to {renamed}` |
| E6002 | valueTaken | value:Value, field:Name, key:Name | `value {value} of {field} belongs to {key}` |
| E6002 | conflict | value:Name, a:Text, b:Text | `canon.lock holds two facts for {value}: {a} and {b}` |
| E6003 | table | - | `stable table must be the whole type of a top-level let` |
| E6003 | field | - | `@stable needs a field of the element of a stable table` |
| E6003 | type | typ:Type | `@stable field must be an integer type or String, not {typ}` |
| E6004 | entry | layer:Name, table:Name | `layer {layer} cannot add an entry to stable table {table}` |
| E6004 | field | layer:Name, field:Name | `layer {layer} cannot change stable field {field}` |
| E6005 | header | - | `canon.lock: missing header` |
| E6005 | version | version:Text | `canon.lock: version {version} is newer than this canon` |
| E6005 | syntax | - | `canon.lock: cannot parse line` |
| E6005 | kind | kind:Text | `canon.lock: unknown kind {kind}` |
| E6005 | package | name:Text, pkg:Name | `canon.lock: {name} is not in package {pkg}` |
| E6005 | merge | - | `canon.lock: merge conflict marker` |
| W6006 | one | - | `1 stable value is not in canon.lock yet: run canon build` |
| W6006 | many | n:Int | `{n} stable values are not in canon.lock yet: run canon build` |

## E7xxx, W7xxx: Loading and decoding

Owner: WIRE.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E7001 | error | project | WIRE.md §2 | invalid path (leaves its root or the project, empty segment, absolute, `\`) |
| E7002 | error | check | WIRE.md §6.1 | `load` without an expected type |
| E7003 | error | project | WIRE.md §2.1 | unknown root |
| E7004 | error | load | WIRE.md §6.1 | a loaded file cannot be read |
| E7005 | error | load | WIRE.md §6.5 | invalid glob |
| E7006 | error | load | WIRE.md §6.1 | a `load` option not valid for the form or format |
| E7007 | error | load | WIRE.md §6.2 | the format of a loaded file cannot be told from its extension |
| W7101 | warning | load | WIRE.md §6.8 | `load.defines` skipped defines it cannot evaluate (once per file) |
| E7102 | error | load | WIRE.md §6.8 | a define redefined with a different value |
| E7103 | error | wire | WIRE.md §5.1 | a JSON number that must be an integer is not |
| E7104 | error | jsonsrc | WIRE.md §3.2 | duplicate JSON key or CSV column |
| E7105 | error | load | WIRE.md §3.1 | a loaded file is not valid UTF-8 |
| E7106 | error | load | WIRE.md §6.3 | an `at:` path that does not match the document |
| W7107 | warning | load | WIRE.md §6.5 | a glob matches no file |
| E7108 | error | wire | WIRE.md §6.6 | a CSV cell that does not parse as its field's type |
| E7109 | error | jsonsrc | WIRE.md §3.1 | JSON syntax error |
| E7110 | error | wire | WIRE.md §5 | a JSON value of the wrong kind |
| E7111 | error | wire | WIRE.md §5.3 | a wire value that is not a member of the enum |
| E7112 | error | wire | WIRE.md §5.6 | a variant object without its tag, or with an unknown case |
| E7113 | error | load | WIRE.md §6.6 | malformed CSV |
| E7114 | error | wire | WIRE.md §5.7 | a table key that is not a Canon identifier |
| W7115 | warning | load | WIRE.md §6.5 | a symbolic link skipped: outside the roots, dangling, looping or unreadable |
| E7116 | error | check | WIRE.md §6.1 | a load form that cannot produce the expected type |
| E7117 | error | wire | WIRE.md §5.14 | an `@json(pairs:)` slot with one key but not the other, a `null`, or a filled slot after an empty one |

| Code | Variant | Args | Template |
|---|---|---|---|
| E7001 | root | path:Text, root:Name | `path {path}: leaves root @{root}` |
| E7001 | project | path:Text | `path {path}: leaves the project` |
| E7001 | empty | path:Text | `path {path}: empty segment` |
| E7001 | dot | path:Text, seg:Text | `path {path}: "{seg}" is not a path segment` |
| E7001 | absolute | path:Text | `path {path}: absolute path` |
| E7001 | backslash | path:Text | `path {path}: "\\" is not a separator` |
| E7002 | - | - | `load needs an expected type here: annotate the let or the field` |
| E7003 | - | name:Name, roots:Names | `unknown root @{name}; roots: {roots}` |
| E7004 | - | path:Path, cause:Kind | `cannot read {path}: {cause}` |
| E7005 | - | pattern:Text, cause:Kind | `invalid glob {pattern}: {cause}` |
| E7006 | option | option:Name, form:Name, format:Name | `option {option} is not valid for {form} ({format})` |
| E7006 | format | format:Name | `format {format} is not one of json, csv and text` |
| E7007 | - | path:Path | `cannot tell the format of {path}; add format: json, csv or text` |
| W7101 | one | path:Path, name:Name | `1 define skipped in {path} ({name}): not a supported integer expression` |
| W7101 | many | n:Int, path:Path, name:Name | `{n} defines skipped in {path} (first: {name}): not a supported integer expression` |
| E7102 | - | name:Name, renamed:Int, previous:Int, first:Loc | `{name} redefined with a different value ({renamed}, first {previous} at {first})` |
| E7103 | number | json:Text | `{json} is not an integer` |
| E7103 | mapKey | key:Text | `map key "{key}" is not a canonical integer` |
| E7104 | json | key:Text, first:Loc | `duplicate key "{key}" (first at {first})` |
| E7104 | csv | name:Text | `duplicate CSV column "{name}"` |
| E7105 | file | path:Path, offset:Int | `{path} is not valid UTF-8 at byte {offset}` |
| E7105 | surrogate | char:Rune | `unpaired surrogate {char}` |
| E7106 | member | at:Text, name:Text, pointer:Pointer | `at: "{at}": no member "{name}" at {pointer}` |
| E7106 | index | at:Text, index:Int, pointer:Pointer | `at: "{at}": index {index} out of range at {pointer}` |
| E7106 | notObject | at:Text, pointer:Pointer | `at: "{at}": not an object at {pointer}` |
| E7106 | notArray | at:Text, pointer:Pointer | `at: "{at}": not an array at {pointer}` |
| E7106 | scalar | at:Text, pointer:Pointer | `at: "{at}": * applied to a scalar at {pointer}` |
| E7106 | syntax | at:Text | `at: "{at}": malformed path` |
| W7107 | - | pattern:Text | `glob {pattern} matches no file` |
| E7108 | - | text:Text, typ:Type | `CSV cell "{text}" is not a valid {typ}` |
| E7109 | eof | - | `JSON syntax error: unexpected end of input` |
| E7109 | depth | limit:Int | `JSON syntax error: nested deeper than {limit} levels` |
| E7109 | char | char:Text | `JSON syntax error: unexpected character {char}` |
| E7110 | kind | expected:Kind, typ:Type, found:Kind | `expected {expected} for {typ}, found {found}` |
| E7110 | int | found:Text | `expected 0 or 1 for @json(int), found {found}` |
| E7110 | bits | found:Text | `expected a non-negative bitmask for @json(bits), found {found}` |
| E7110 | retired | found:Text | `expected true for $retired, found {found}` |
| E7111 | member | wire:Text, enum:Name | `"{wire}" is not a member of {enum}` |
| E7111 | hint | wire:Text, enum:Name, hint:Name | `"{wire}" is not a member of {enum} (did you mean {hint}?)` |
| E7111 | bits | bits:Int, enum:Name | `bits {bits} are not members of {enum}` |
| E7112 | tag | tag:Text, variant:Name | `missing tag "{tag}" for variant {variant}` |
| E7112 | case | wire:Text, variant:Name, cases:Names | `unknown case "{wire}" of {variant}; cases: {cases}` |
| E7113 | bareQuote | n:Int | `CSV: a quote inside an unquoted field at record {n}` |
| E7113 | unclosed | n:Int | `CSV: an unterminated quoted field at record {n}` |
| E7113 | afterQuote | n:Int | `CSV: text after a closing quote at record {n}` |
| E7113 | fieldCount | n:Int, got:Int, want:Int | `CSV: record {n} has {got} fields, the first has {want}` |
| E7113 | bareCR | n:Int | `CSV: a carriage return inside an unquoted field at record {n}` |
| E7113 | noHeader | - | `CSV: header: true but the file has no header record` |
| E7114 | - | key:Text | `"{key}" is not a valid table key (an identifier)` |
| W7115 | outsideRoots | path:Path | `symbolic link {path} points outside the roots; skipped` |
| W7115 | dangling | path:Path | `symbolic link {path} points to nothing; skipped` |
| W7115 | looping | path:Path | `symbolic link {path} is part of a loop; skipped` |
| W7115 | statFailed | path:Path | `symbolic link {path} cannot be resolved or read; skipped` |
| E7116 | - | form:Name, typ:Type | `{form} cannot produce {typ}` |
| E7117 | key | slot:Int, field:Name, k:Name, v:Name | `@json(pairs:) slot {slot} of {field}: {k} without {v}` |
| E7117 | null | slot:Int, field:Name | `@json(pairs:) slot {slot} of {field}: null value` |
| E7117 | gap | slot:Int, field:Name, empty:Int | `@json(pairs:) slot {slot} of {field}: filled after empty slot {empty}` |

## E80xx–E83xx, W80xx: Emit and generated code

Owner: CODEGEN.md, WIRE.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E8001 | error | build | CODEGEN.md §2.4 | refuses to overwrite a file that `canon` did not generate |
| E8002 | error | check | CODEGEN.md §2.1 | two emits of one target in one package |
| E8003 | error | check | CODEGEN.md §2.1 | unknown emit target or emit option |
| E8004 | error | ir | CODEGEN.md §2.8 | a type of a package that has no emit for the same target |
| E8005 | error | ir | CODEGEN.md §3.5 | two generated names collide in one scope |
| W8006 | warning | ir | CODEGEN.md §3.5 | a generated C++ name is a common platform macro |
| E8007 | error | ir | CODEGEN.md §2.8 | a Go output under no root mapped by `go_module` |
| E8008 | error | ir | CODEGEN.md §2.3 | two Go emits write into one directory |
| E8009 | error | check | CODEGEN.md §2.1 | invalid emit option value (mode, package, namespace, out, values) or option of the wrong kind |
| E8010 | error | ir | CODEGEN.md §5.2 | an `ordered` enum whose codes do not increase |
| E8011 | error | ir | CODEGEN.md §3.5 | an override or a derived name that is not a usable identifier in the target |
| E8012 | error | ir | CODEGEN.md §4.4 | an emitted type or value with no representation in generated code |
| E8013 | error | ir | CODEGEN.md §5.10 | an export fn that `data` mode cannot hold |
| E8014 | error | ir | CODEGEN.md §5.13 | something `types` mode cannot emit (precomputed data, computed default) |
| E8015 | error | ir | CODEGEN.md §2.2 | a `data`/`embedded` value that is not a table, keyed list or record |
| E8017 | error | ir | CODEGEN.md §5.6 | a dependent-type branch that is not a scalar, String, enum or ref |
| E8018 | error | ir | CODEGEN.md §2.8 | a type decoded from JSON whose package is emitted in `baked` mode |
| E8019 | error | ir | EVALUATION.md §1 | an emit whose generator cannot produce a construct valid Canon allows |
| E8020 | error | ir | CODEGEN.md §5.1 | a Go constant of -0.0, which Go constants cannot hold |
| E8101 | error | ir | CODEGEN.md §4.1 | an emitted integer outside the TypeScript safe range without `@ts(bigint)` |
| E8102 | error | wire | WIRE.md §5.1 | a value with no wire form for its field (not a whole unit, equals the `none` marker, repeated bits member) |
| E8103 | error | ir | CODEGEN.md §7.8.1 | a string or list longer than its fixed-size legacy C++ array |
| E8104 | error | ir | CODEGEN.md §5.12 | a TypeScript emit of a package with input fields |
| E8106 | error | ir | CODEGEN.md §7.8.1 | a value outside the range of its legacy C++ member |
| E8107 | error | ir | CODEGEN.md §7.8.1 | unknown or incompatible `@cpp(type:)` |
| E8108 | error | ir | CODEGEN.md §7.8.1 | a case of an inline variant on a legacy struct lacks `@cpp(value:)` |
| E8109 | error | ir | CODEGEN.md §7.8 | invalid `@cpp(struct:)` mapping (missing header, bad access, unmappable field) |
| E8150 | error | check | WIRE.md §8.1 | `emit json` file mode with other than one value |
| E8151 | error | ir | WIRE.md §5.9 | a value with no wire form (`Range`, function) emitted to JSON |
| E8152 | error | build | WIRE.md §8.1 | two outputs of a build with the same path, or paths differing only in letter case |
| E8153 | error | ir | WIRE.md §8.1 | a `data`-mode or `@reload` value not written by the package's `emit json` as `<value>.json` |
| E8201 | error | ir | CODEGEN.md §5.11 | `@reload` on data mapped to a legacy struct in `fields` or `both` mode |
| E8202 | error | ir | CODEGEN.md §5.11 | `@reload` on a value emitted in a mode that has no file to reload |
| E8301 | runtime | gen | CODEGEN.md §5.9 | embedded data failed to decode (signalled at run time) |
| E8302 | runtime | gen | CODEGEN.md §5.12 | an input getter called before `LoadInputs` (signalled at run time) |
| E8303 | runtime | gen | CODEGEN.md §8.2 | an integer outside the TypeScript safe range in translated code (signalled at run time) |

| Code | Variant | Args | Template |
|---|---|---|---|
| E8001 | - | path:Path | `{path} was not generated by canon; refusing to overwrite it` |
| E8002 | - | pkg:Name, target:Name | `package {pkg} has two emits for target {target}` |
| E8003 | target | target:Name | `unknown emit target {target}: expected go, cpp, ts, json or view` |
| E8003 | option | option:Name, target:Name | `unknown option {option} for emit {target}` |
| E8004 | - | typ:Name, pkg:Name, target:Name | `{typ} of package {pkg} is used by this {target} emit, but {pkg} has no {target} emit` |
| E8005 | - | target:Name, name:Name, a:Name, b:Name | `{target}: {name} is generated for both {a} and {b}` |
| W8006 | - | name:Name, item:Name | `C++ name {name} (from {item}) is a macro in common platform headers` |
| E8007 | - | out:Path | `Go output {out} is under no root listed in go_module: map its root in project.canon go_module` |
| E8008 | - | dir:Path, a:Name, b:Name | `Go directory {dir} is written by two emits ({a}, {b})` |
| E8009 | mode | mode:Name, target:Name, modes:Names | `invalid mode {mode} for emit {target}: expected one of {modes}` |
| E8009 | package | value:Text | `invalid package "{value}" for emit go: not a Go identifier, or a Go keyword` |
| E8009 | namespace | value:Text | `invalid namespace "{value}" for emit cpp: expected ident{{::ident}} with no C++ keyword` |
| E8009 | tsOut | value:Text | `invalid out "{value}" for emit ts: it must end in .ts` |
| E8009 | values | name:Name, target:Name | `values of emit {target}: {name} is not a public let of this package` |
| E8009 | valuesTwice | name:Name, target:Name | `values of emit {target}: {name} is listed twice` |
| E8009 | kind | option:Name, target:Name, expected:Kind | `option {option} of emit {target} must be {expected}` |
| E8009 | missing | option:Name, target:Name | `emit {target} is missing {option}` |
| E8009 | reservedNamespace | value:Text | `invalid namespace "{value}" for emit cpp: canon, std and nlohmann are reserved` |
| E8010 | - | enum:Name | `ordered enum {enum} has codes that do not increase in declaration order` |
| E8011 | override | name:Text, target:Name | `{name} is not a valid {target} identifier for @{target}(name:)` |
| E8011 | unexported | name:Text | `{name} is not exported: an @go(name:) override starts with an upper-case letter` |
| E8011 | derived | name:Text, origin:Name, target:Name | `{name}, the {target} name derived for {origin}, is not a valid identifier` |
| E8011 | reserved | name:Text, origin:Name | `{name}, the C++ name derived for {origin}, is a keyword or a reserved name` |
| E8012 | type | typ:Name, what:Type | `{typ} cannot be emitted: {what} has no representation in generated code` |
| E8012 | define | typ:Name | `{typ} cannot be emitted: a load.defines table or record has no representation in generated code` |
| E8013 | package | fn:Name | `{fn} needs baked or embedded mode: data files cannot hold package functions` |
| E8013 | refParam | fn:Name | `{fn} needs baked or embedded mode: data files cannot hold a method with a ref parameter` |
| E8014 | - | what:Kind, typ:Name | `types mode cannot emit the {what} of {typ}: there is no data to read it from` |
| E8015 | - | value:Name, typ:Type | `{value} has type {typ}: data and embedded modes emit tables, keyed lists and records` |
| E8017 | - | branch:Name, alias:Name, typ:Type | `branch {branch} of {alias} has type {typ}: dependent types may only have scalar, String, enum or ref branches` |
| E8018 | - | typ:Name, emit:Name, pkg:Name | `{typ} is decoded from JSON by {emit}, but package {pkg} is emitted in baked mode` |
| E8019 | - | target:Name, mode:Name, what:Kind | `emit {target} in {mode} mode cannot generate {what}` |
| E8020 | - | name:Name | `constant {name} is -0.0, which a Go constant cannot hold: make it a let` |
| E8101 | - | value:Value, field:Name | `{value} does not fit a TypeScript number; add @ts(bigint) to {field}` |
| E8102 | unit | value:Value, field:Name, unit:Name | `{value} has no wire form for {field}: not a whole number of {unit}` |
| E8102 | none | value:Value, field:Name, marker:Text | `{value} has no wire form for {field}: it equals the none marker {marker}` |
| E8102 | bits | value:Value, field:Name, member:Name | `{value} has no wire form for {field}: {member} appears twice in a bits set` |
| E8103 | - | member:Name, ctype:Text, capacity:Int, length:Int | `{member} ({ctype}) holds at most {capacity}; this value needs {length}` |
| E8104 | - | field:Name | `input field {field} cannot be emitted to TypeScript` |
| E8106 | - | value:Value, member:Name, ctype:Text | `{value} does not fit {member} ({ctype})` |
| E8107 | - | ctype:Text, field:Name, typ:Type | `unknown or incompatible @cpp(type: {ctype}) for {field} of type {typ}` |
| E8108 | - | caseName:Name, variant:Name, member:Name | `case {caseName} of {variant} needs @cpp(value:) to fill {member}` |
| E8109 | header | typ:Name, access:Name | `@cpp(struct:) on {typ}: access {access} needs header` |
| E8109 | access | typ:Name, access:Text | `@cpp(struct:) on {typ}: access must be fields, both or getters, not {access}` |
| E8109 | field | typ:Name, field:Name | `@cpp(struct:) on {typ}: field {field} cannot map onto a legacy struct (a map, a non-inline variant, a nested record or a list of records)` |
| E8150 | - | out:Text, n:Int | `emit json out "{out}" is a file but values has {n} elements` |
| E8151 | type | value:Name, typ:Type | `value {value} of type {typ} has no wire form` |
| E8151 | define | value:Name | `value {value} is a load.defines table or record, which has no wire form` |
| E8152 | - | a:Path, b:Path | `outputs collide: {a} and {b}` |
| E8153 | notWritten | value:Name | `{value} is emitted in data mode but not written by emit json` |
| E8153 | fileName | value:Name | `{value} is emitted in data mode but its file must be {value}.json` |
| E8153 | directory | value:Name | `{value} is emitted in data mode but @reload files must share a directory` |
| E8201 | - | value:Name, typ:Name, legacy:Name, access:Name | `@reload on {value}: {typ} maps onto legacy struct {legacy} in {access} mode, whose pointers legacy code keeps` |
| E8202 | - | value:Name, target:Name, mode:Name | `@reload on {value}: the {target} emit is in {mode} mode, which has no file to reload` |
| E8301 | - | value:Name, cause:Text | `embedded data of {value} failed to decode: {cause}` |
| E8302 | - | field:Name | `{field} is a runtime input and LoadInputs has not been called` |
| E8303 | - | - | `integer outside the TypeScript safe range` |

## E9xxx: Translated functions

Owner: CONFORMANCE.md.

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E9001 | error | ir | CONFORMANCE.md §2.2 | a construct outside the portable subset in a translated function |
| E9002 | error | ir | CONFORMANCE.md §8 (CODEGEN.md §5.10) | a lookup table over 65 536 cells |
| E9003 | error | ir | CONFORMANCE.md §2.1 | an optional parameter in an export fn with parameters |
| E9004 | error | ir | CONFORMANCE.md §2.1 | a translated function returns a type that is not allowed |
| E9005 | error | ir | CONFORMANCE.md §2.2 | a `Float` or `Duration` interpolated in a translated template |
| E9006 | error | ir | CONFORMANCE.md §2.1 | a translated function parameter of a type that is not allowed |
| E9007 | error | ir | CONFORMANCE.md §2.1 | a refinement on a translated parameter or result that cannot be checked at run time |
| E9008 | error | conform | CONFORMANCE.md §6.1 | a translated method no test of its package calls |
| E9009 | error | conform | CONFORMANCE.md §6.5 | a conformance vector exceeds its step cap or the call-depth limit |

| Code | Variant | Args | Template |
|---|---|---|---|
| E9001 | - | construct:Expr, fn:Name | `{construct} is not allowed in translated function {fn}: it is outside the portable subset` |
| E9002 | - | fn:Name, n:Int | `lookup table of {fn} would have {n} cells (limit 65536)` |
| E9003 | - | param:Name, fn:Name | `parameter {param} of export fn {fn} is optional` |
| E9004 | - | fn:Name, typ:Type | `translated function {fn} returns {typ}: only scalars, strings, enums and refs are allowed` |
| E9005 | - | expr:Expr, typ:Type | `{expr} has type {typ}: translated templates may only interpolate String, integer and enum values` |
| E9006 | - | param:Name, fn:Name, typ:Type | `parameter {param} of {fn} has type {typ}: translated functions take Bool, integers, Float, String, Duration and enums` |
| E9007 | param | refinement:Expr, param:Name, fn:Name | `refinement {refinement} of parameter {param} of {fn} cannot be checked at run time` |
| E9007 | result | refinement:Expr, fn:Name | `refinement {refinement} of the result of {fn} cannot be checked at run time` |
| E9008 | - | typ:Name, fn:Name, pkg:Name | `translated method {typ}.{fn} is called by no test of package {pkg}: add one, it gives the conformance test its receiver` |
| E9009 | steps | n:Int, fn:Name, limit:Int | `conformance vector {n} of {fn} exceeds {limit} steps` |
| E9009 | depth | n:Int, fn:Name | `conformance vector {n} of {fn} exceeds the call-depth limit` |
