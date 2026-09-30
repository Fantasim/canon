# Canon language specification

Version: **0.1 (being locked)**. Nothing is implemented yet. The files in `examples/` are the
test cases for this document: when an example and this text disagree, one of them is a bug. The
generated files in `examples/pipeline/expected/` are **illustrative** until the v0 compiler
regenerates them; until then tests compare them semantically (parsed JSON, Go/C++ ASTs modulo
comments), not byte for byte.

This document is the normative overview of the language. Where a companion document below owns
the details of a subject, this document keeps a short normative summary and points to it; if the
two disagree, the companion document wins for the details it owns and this document wins for
everything else. [DECISIONS.md](DECISIONS.md) records why things are the way they are, and wins
over both; the choices it accepts are listed in
[meta/spec-phase/review/ACCEPTED-CHOICES.md](meta/spec-phase/review/ACCEPTED-CHOICES.md) (DECISIONS 24), which also wins over both.

"Canon" is a working name. The compiler is the Go module `github.com/fantasim/canonlang`
(DECISIONS 23).

### Companion documents

| Document | Owns |
|---|---|
| [CLI.md](CLI.md) | the `canon` command: flags, output formats, exit codes, value paths |
| [spec/GRAMMAR.md](spec/GRAMMAR.md) | the normative grammar: lexer modes, newline rules, keyword-as-name rules, `project.canon` grammar, the annotation catalogue, naming conventions, parse corpus |
| [spec/FORMATTER.md](spec/FORMATTER.md) | the canonical layout printed by `canon fmt` and the edit API, and canonical JSON for sources |
| [spec/TYPES.md](spec/TYPES.md) | typing rules: assignability, optionals and narrowing, entries and refs, joins, refinements, dependent types, `match` exhaustiveness |
| [spec/EVALUATION.md](spec/EVALUATION.md) | which values are evaluated, when checks run, step costs, poisoning, copy-on-write, provenance, finding order, layer application |
| [spec/STDLIB.md](spec/STDLIB.md) | every standard function and method with its generic signature, costs, errors and the canonical text form |
| [spec/WIRE.md](spec/WIRE.md) | JSON decoding and canonical encoding, `load.*` formats, `at:` paths, globs, the `emit json` file layout |
| [spec/FINGERPRINT.md](spec/FINGERPRINT.md) | the schema fingerprint: canonical shape serialization, inclusions, test vectors |
| [spec/LOCK.md](spec/LOCK.md) | `canon.lock`: file grammar, sort order, what is locked, retire/rename/reuse, layers, edits and merges |
| [spec/CODEGEN.md](spec/CODEGEN.md) | generated Go, C++17 and TypeScript: naming, file layout, exact APIs, runtime helpers, legacy C++ structs |
| [spec/CONFORMANCE.md](spec/CONFORMANCE.md) | conformance tests of translated `export fn`s: vector selection, checked arithmetic, error signalling |
| [spec/VIEWMODEL.md](spec/VIEWMODEL.md) | the view model and the studio behaviours it drives: controls, relevance, pickers, groups, filters |
| [spec/viewmodel.schema.json](spec/viewmodel.schema.json) | the JSON Schema (2020-12) of the view model (`canon-vm/1`) |
| [spec/I18N.md](spec/I18N.md) | the translation key catalogue, templates in translations, `W1701` reporting, `i18n stub` output |
| [spec/API.md](spec/API.md) | the Go embedding API used by the studio: types, edit paths and operations, minimal writes, revisions |
| [spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) | module map, frozen interfaces, milestones, fixtures, performance targets, determinism CI |

[AUDIT.md](meta/spec-phase/AUDIT.md) is the pre-implementation review this version answers, and
[MOCKUP-GAPS.md](meta/spec-phase/MOCKUP-GAPS.md) the studio behaviours found while mocking the studio.

---

## Contents

1. [Overview](#1-overview)
2. [Lexical structure](#2-lexical-structure)
3. [Projects, packages and files](#3-projects-packages-and-files)
4. [Declarations](#4-declarations)
5. [Types](#5-types)
6. [Values and literals](#6-values-and-literals)
7. [Expressions](#7-expressions)
8. [Statements](#8-statements)
9. [Functions](#9-functions)
10. [Checks](#10-checks)
11. [Evaluation](#11-evaluation)
12. [Stable ids and `canon.lock`](#12-stable-ids-and-canonlock)
13. [Reading files: `load`](#13-reading-files-load)
14. [Output: `emit`](#14-output-emit)
15. [Generated code](#15-generated-code)
16. [Views](#16-views)
17. [Translations](#17-translations)
18. [Tests](#18-tests)
19. [Layers and runtime inputs](#19-layers-and-runtime-inputs)
20. [Standard library](#20-standard-library)
21. [Diagnostics](#21-diagnostics)
22. [Out of scope](#22-out-of-scope)
23. [Open questions](#23-open-questions)
- [Appendix A: Grammar](#appendix-a-grammar) (normative text in spec/GRAMMAR.md)
- [Appendix B: Keywords](#appendix-b-keywords) (normative text in spec/GRAMMAR.md)

---

## 1. Overview

Canon is a language for configuration of any kind: application data, service settings,
taxonomies, and how all of it is presented in an editor. It is written once, checked once at build time,
and **translated** into:

- typed, read-only code for **Go**, **C++** and **TypeScript**;
- **JSON** data files that runtimes read;
- a **view model** that a generic editor (the **studio**) renders.

Canon is not tied to one project. The examples in this document come from the first project
that uses it (an online game: items, monsters, events), because real data makes better tests;
nothing in the language depends on them.

Runtimes never interpret Canon. They receive values that are already correct and a typed,
read-only way to read them.

### 1.1 The three laws

1. **Pure.** A Canon program has no clock, randomness, network, environment or process access.
   Its only inputs are files inside the project's roots, read through `load` (§13). The same
   sources always produce the same outputs, byte for byte.
2. **Build-time only.** Functions, checks and loops run inside the compiler. Generated code
   contains types, values, lookups and getters. The one exception is an `export fn` with
   runtime inputs (§9.4): its body is translated from a small portable subset, and every
   translation ships with a generated conformance test against the compiler's evaluator.
3. **Always finishes.** Evaluation runs under a step budget (§11.6). Running out is a
   diagnostic with a Canon stack trace. `while` and recursion are allowed because of it.

Anything that needs logic at runtime beyond the portable subset (a damage formula, a
matchmaking rule) is runtime code, not config.

### 1.2 Terms

| Term | Meaning |
|---|---|
| project | A directory tree with a `project.canon` at its root (§3.1). The law repository. |
| package | A named set of `.canon` files (§3.2). The unit of import and of emission. |
| value | Anything a Canon expression produces. Values are immutable once built. |
| public value | A top-level `let` not marked `local`. What `emit` can output. |
| entry | An element of a table or keyed list: a value plus an identity (collection, key) (§5.8). |
| wire form | How a value is written in a data file (JSON). May differ from Canon names (§5.12). |
| finding | A diagnostic produced by the compiler or by a `check` (§21). |
| target | An output kind: `go`, `cpp`, `ts`, `json`, `view` (§14). |
| runtime | A program that uses generated code: a server, a Go service, a web page. |

---

## 2. Lexical structure

### 2.1 Source text

Source files are UTF-8 with `\n` line endings (`\r\n` is accepted and normalized). Invalid UTF-8
is `E1124`; a byte order mark is `E1123`, and `canon fmt` removes it. Outside strings and comments
only ASCII is allowed (`E1106`). Tabs and spaces are whitespace. Details:
[spec/GRAMMAR.md](spec/GRAMMAR.md) §1.

`canon fmt` rewrites every file in the single canonical layout; there are no formatting options.
The layout never aligns columns: one space after `:`, one space around `=`, no padding to line
values up, so a one-value edit is a one-line diff (DECISIONS 18). Indentation is 2 spaces, lines
are at most 100 columns where they can be broken, and a broken brace list holds one item per line
without commas. Every example is a fixed point of `canon fmt`, and so is every code sample in
this document, apart from its `…` elisions and `<placeholders>`. Details:
[spec/FORMATTER.md](spec/FORMATTER.md).

### 2.2 Comments

```
// line comment
/* block comment, may span lines, does not nest */
/// doc comment
```

A **doc comment** (`///`, possibly several consecutive lines) attaches to the next declaration,
field, enum member, table entry, variant case, view item or `export fn`. A doc comment followed
by a blank line or by something it cannot attach to is a warning (`W1001`). Doc comments are
part of the output: they become generated code comments and the default help text in the
studio (§16). A public type, field or `export fn` without one is a warning (`W1002`).

- A doc comment **before the `package` line** is the package's doc. When several files of a
  package have one, they are concatenated in byte order of their paths, separated by a blank line.
  Generated files do not carry it: their header is at most two fixed lines (spec/CODEGEN.md
  §2.5).
- Doc text is normalized: `///` and one following space are stripped, trailing whitespace is
  removed, consecutive lines are joined with `\n`, and leading and trailing empty lines are
  dropped. `////` is an ordinary comment, and so is `///` written after code on the same line
  (`W1001`). Generated code copies the text verbatim (spec/CODEGEN.md). Attachment rules:
  [spec/GRAMMAR.md](spec/GRAMMAR.md) §9.1.

### 2.3 Newlines and separators

A newline ends a field, entry, statement, declaration or other list item, **except**:

- when the innermost open bracket is `(` or `[` (or a string interpolation): newlines there are
  insignificant. Only the innermost bracket decides, so inside a `{` nested in `[` or `(` newlines
  separate items again;
- after a token that cannot end an item: a binary operator, `??`, `=` and the compound
  assignments, `=>`, `->`, `.`, `?.`, `,`, `:`, `|`, `(`, `[`, `{`, `...`, or one of the keywords
  `and or not in is else where as`;
- before a line whose first token continues the previous one: `.`, `?.`, `??`, a binary operator
  other than `-`, `=` and the compound assignments, `=>`, `->`, `|`, or one of the keywords
  `and or in is else where`;
- before a line that starts with annotations (`@name`, `@name(…)`), unless the annotations are
  followed by a declaration keyword: `@reload` on its own line before `let` starts a new
  declaration, while an own-line `@deprecated(…)` under a field belongs to that field.

Inside `{ }`, items are separated by newlines or commas, which may be mixed (at most one comma
between two items); blank lines are allowed between items, and a trailing separator is allowed.
Inside `( )` and `[ ]`, items are separated by commas, and a trailing comma is allowed. Two items
on one line without a separator are `E1117`. Details: [spec/GRAMMAR.md](spec/GRAMMAR.md) §3.

### 2.4 Identifiers

```
identifier = letter { letter | digit | "_" }
letter = "A".."Z" | "a".."z" | "_"
```

Identifiers are ASCII and case-sensitive. A lone `_` is not an identifier: it is a punctuation
token (the `match` wildcard, an ignored binder, and the any-type of §5.9).

**Naming conventions** are reported by `canon check` as warnings (`W1003`), on the selected
packages only, never by `canon fmt`:

| Kind | Convention | Example |
|---|---|---|
| types, enums, records, variants, aliases | `UpperCamel` | `ModelType` |
| fields, functions, methods, lets, parameters, locals, import aliases, package segments | `lowerCamel` | `farmPurchasePrice` |
| constants | `UPPER_SNAKE` | `FARM_MAX_MODELS` |
| widgets | `lower_snake` | `weekly_timeline` |
| enum members, table keys, variant cases, check names, group and show ids, layer names | free: they are data | `gm_junior`, `Mon`, `II_WEA_AXE_ANGEL` |

**Keywords.** Contextual keywords (`keyed`, `by`, the view vocabulary, …) are keywords only where
the grammar expects them. Reserved keywords cannot name declarations, fields, parameters or locals
(`E1125`), but data may use them as names: as an enum member, variant case, table key or
translation-key segment (`enum Icon { check, package }`), as the name before `:` in any brace
literal (`emit go { package: "teamboard" }`), after `.` (`Icon.check`), and bare in value position
for the words that cannot start an expression (`icon: check`). `none`, `true`, `false` and `self`
never name a member, case or key (`E1126`). The exact positions and word lists are
[spec/GRAMMAR.md](spec/GRAMMAR.md) §4.

### 2.5 Literals

| Kind | Examples | Notes |
|---|---|---|
| integer | `42` `1_000_000` `0x1F` `0b1010` | `_` only between digits; `-` is an operator, folded into the literal |
| float | `3.5` `0.15` `1e-3` `2.5e6` | a float literal always has `.` followed by a digit, or an exponent |
| string | `"Heal {heal} HP"` | interpolation `{expr}`; `{{` and `}}` for literal braces |
| raw string | `r"C:\path\{x}"` | no escapes, no interpolation, on one line |
| multiline string | `"""` … `"""`, `r"""` … `"""` | the closing line's indentation is stripped from every line; first and last newline removed; `r` makes it raw |
| duration | `250ms` `90s` `5m` `1h` `2d` `1h30m` | units `d h m s ms`, each at most once, largest first, no spaces, whole numbers only (`1.5h` is `E1111`) |
| regex | `/^II_[A-Z0-9_]+$/` | RE2 syntax, search semantics (§5.2); only in refinements and `matches` |
| boolean | `true` `false` | |
| none | `none` | the absent value of an optional |

String escapes: `\n \t \r \\ \" \{ \} \u{1F600}`.

- **Integers** are arbitrary precision until checked against their expected type;
  `-9223372036854775808` folds as one negative literal. A literal that does not fit its type is
  `E3201` at compile time.
- **Durations** allow any integer per unit (`90m`, `1500ms`); `canon fmt` rewrites them in
  canonical form (`90s` becomes `1m30s`, `60s` becomes `1m`).
- **Regex vs division.** `/` starts a regex literal only when the previous significant token is `(`
  or `,`, and a regex is accepted only as the sole argument of a type refinement or of
  `.matches(`. Everywhere else `/` divides.
- **Interpolation** lexes a full expression inside `{…}`, including nested strings and brackets,
  up to the matching `}`, on one line and without comments (`E1112`). A format spec is a `:` at
  bracket depth 0 followed by `[+][,][.N]`, with `N` at most 20, and `}`; anything else is
  `E1101`.
- **Multiline strings** mixing tabs and spaces in the stripped prefix are `E1102`; escapes and
  interpolation work as in ordinary strings.

Interpolation writes a value in its **canonical text form**: integers in decimal, floats as
ECMAScript `Number::toString` (integral values without `.0`), durations as canonical literals
(largest unit first, zero parts omitted, `0s` for zero, `-` prefix when negative: `1h30m`), enum
members by name, refs by key, `none` as `none`, lists as `[a, b]`, maps as `{k: v}`, records as
`Type{f: v}`. A format spec may follow a colon: `{heal:,}` (groups of 3 with `,`), `{ratio:.2}`
(fixed decimals, rounded half away from zero from the exact binary value), `{delta:+}` (always
signed, `+0` for zero); specs apply to numbers only. Details:
[spec/GRAMMAR.md](spec/GRAMMAR.md) §2 (lexing), [spec/STDLIB.md](spec/STDLIB.md) §9 (the text form
and format specs).

### 2.6 Operators and punctuation

```
...  ..=  ..  ??  ?.  =>  ->  ==  !=  <=  >=  +=  -=  *=  /=
+  -  *  /  %  <  >  =  !  ?  .  ,  :  (  )  [  ]  {  }  @  |  _
```

Tokens are matched longest first. `!` is only the postfix presence assertion (§5.6): `x!=y` is
`x != y`. `?` appears only in types (`T?`). `_` is a token of its own, never an identifier.
`@` is directly followed by the annotation name. Details: [spec/GRAMMAR.md](spec/GRAMMAR.md) §2.8.

---

## 3. Projects, packages and files

### 3.1 The project file

The root of the law repository contains `project.canon`. It is the only file without a `package`
line: it may start with doc comments, then holds exactly one `project` declaration.

```
/// The law repository of the Acme project.
project acme {
  canon: "0.1"

  /// Named roots used by `load` and `emit` paths: "@resource/Server/...".
  roots {
    resource: "../Resource"
    source: "../Source"
    web: "../web"
  }

  languages: [en, fr]
  studio: studio
  budget: 100_000_000
  go_module {
    web: "example.com/acme/web"
  }
}
```

The keys form a built-in schema, given in full in [spec/GRAMMAR.md](spec/GRAMMAR.md) §7. The
project file holds no expressions: only strings, integers, identifiers, lists and maps. An
unknown key is `E1002`.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `canon` | `String`, `"MAJOR.MINOR"` | required | language version this project is written for |
| `roots` | `{identifier: String}` | `{}` | named roots; paths are relative to the project directory and may point outside it |
| `languages` | `[identifier](1..)` | `[en]` | the first one is the source language (§17); codes match `[a-z]{2,3}(_[A-Z][a-z]{3})?(_[A-Z]{2})?` |
| `studio` | package name, optional | `none` | package holding the studio vocabulary (§16.11); it must exist (`E1012`) |
| `budget` | `Int(1..)` | `100_000_000` | evaluation steps per invocation (§11.6) |
| `go_module` | `{root name: String}` | `{}` | the Go module path of each root that receives Go code, keyed by a declared root name (`E1009`), so imports between generated packages can be derived (§14.6) |

- `key { … }` is the canonical spelling of a map-valued key (`roots { … }`); `key: { … }` means
  the same and `canon fmt` rewrites it.
- A path in `load` or `emit` is either `@root/...`, or relative to the directory of the file that
  contains it. `..` is resolved lexically first; then the path must stay inside the project
  directory or inside a declared root, and an empty segment (`//`) is invalid (`E7001`). Written
  paths always use `/`. An unknown root is `E7003`.
- `canon` states the language version. A compiler accepts the minor versions it knows within its
  major version, refuses newer ones and other majors (`E1001`), and never silently reinterprets
  older code.
- Tests and tools can redirect a root without editing the file (CLI `--root name=path`).

### 3.2 Packages

A package is named by a dotted path that mirrors directories under the project root:
`game.items` lives in `game/items/`.

```
package game.items

import resource.vocab as vocab // alias
import shared.roles { Role } // brings names into scope
import shared.ui // use qualified: ui.Tone
```

- Every file except `project.canon` starts with a `package` line, optionally preceded by the
  package doc comment (§2.2). A file may declare the package of its own directory or of an
  ancestor directory, never of a descendant, so a large package can keep entries in
  subdirectories (`game/items/weapon/axe/II_WEA_AXE_ANGEL.canon` may declare
  `package game.items`); any other package line is `E2006`. A directory cannot contain files of
  two unrelated packages (`E2001`).
  Layer and translation files follow the same rule.
- All files of a package share one namespace. Declaration order does not matter.
- Imports name packages by their dotted path. Imports are acyclic (`E2002`). `canon fmt` sorts
  them by package path, and the names inside `{ }`, as bytes.
- A package's **public names** are its top-level declarations not marked `local`.

### 3.3 Kinds of files

| First lines | Kind | Contains |
|---|---|---|
| `project name` (file `project.canon`) | project | the project declaration (§3.1) |
| `package p` | source | declarations (§4) |
| `package p` then `layer name` | layer | `amend` blocks only (§19) |
| `package p` then `translation lang` | translation | translation entries only (§17) |

By convention views go in `<name>.view.canon` and translations in `<name>.<lang>.canon`, next
to the source they describe. The convention is not enforced.

---

## 4. Declarations

| Declaration | Section | Emitted? |
|---|---|---|
| `const NAME = expr` | 4.1 | yes, as a constant |
| `type Name = Type` / `type Name(params) = Type` | 5.10, 5.11 | as the underlying type |
| `enum Name { … }` | 5.3 | yes |
| `record Name { … }` / `record Name(params) { … }` | 5.4 | yes |
| `variant Name { … }` | 5.5 | yes |
| `fn name(…) -> T { … }` | 9 | never |
| `export fn name(…) -> T { … }` | 9.4 | yes, as a getter, a lookup or a translated function |
| `let name: T = expr` | 4.2 | yes, as a value |
| `entry coll.key { … }` | 4.3 | as part of its table or keyed list |
| `check …` / `warn …` | 10 | never |
| `view Type { … }` / `view Variant.case { … }` | 16 | to the view model |
| `widget name(value: T)` / `widget name(value: T, siblings: [T])`, optionally followed by `default` | 16.11 | to the view model |
| `test "name" { … }` | 18 | never |
| `emit target { … }` | 14 | controls emission |

`local` before `const`, `type`, `fn`, `let`, `record`, `enum` or `variant` makes it private to
the package: usable by the package's own code, never imported, never emitted.

### 4.1 Constants

```
const FARM_MAX_MODELS = 100
const DAY = 1440
const WEEK = 7 * DAY
```

A constant is a value computable without `load`: literals, other constants, operators and the
standard library, but no user function calls. Its type is inferred, and may be composite (a list
or a map). Constants may be used in types (`[ModelType](..=FARM_MAX_MODELS)`) and are emitted in
every target (Go: a `const`, or an accessor for lists and maps; C++: `inline const`; TypeScript: a
frozen `const`). A constant cannot be amended by a layer.

### 4.2 Values

```
let statuses: stable table Status = { open { … }, taken { … } }
let farm: FarmConfig = load("@resource/Server/System/farm_config.json")
local let jobs = load.defines("@resource/Server/Define/defineJob.h", prefix: "JOB_")
```

A top-level public `let` requires a type annotation (`E3001`): generated code needs a declared
type. `local` lets may infer it, except from `load`, `load.dir` and `load.csv`, which need an
expected type (§13.2); `load.defines` and `load.text` have fixed types.
`canon check` and `canon build` evaluate every top-level `let` of the selected packages, public
and `local`, so every value is validated even if nothing emits it (§11.1).

### 4.3 Entries in other files

The entries of a table or keyed list may be spread over many files, one entry per file if wanted.
This is the form `canon convert` produces for per-item data:

```
// game/items/items.canon
package game.items

let items: stable table Item = {} // entries come from `entry` declarations

// game/items/weapon/axe/II_WEA_AXE_ANGEL.canon
package game.items

/// Angel Axe: the level 90 mercenary axe.
entry items.II_WEA_AXE_ANGEL {
  name: "IDS_PROPITEM_TXT_003930"
  level: 90
  kind: IK1_WEAPON { … }
}
```

- `entry t.key { fields }` adds an entry to table or keyed list `t` of the same package, whose
  literal must be a table literal or a list literal (possibly empty); on anything else it is
  `E3103`. The literal may hold entries too; both are merged.
- For a keyed list (`[T] keyed by f`), the key is the entry's field `f`: the body omits it, and
  writing it is `E3321` (`entry potions.II_POT_HEAL_L { … }`; TYPES.md §9.3).
- Entry order: the table literal's entries first, then entry files in path order, then source
  order within a file.
- A duplicate key is an error naming both locations: `E3101` for a table, `E3102` for a keyed
  list (TYPES.md §9.3).
- `retired entry t.key { … }` retires an entry (§12).
- `@files("items/{itemKind1}/{id}.canon")` on the table's `let` says where a **new** entry's file
  goes (used by `canon convert` and by the studio's `Add`). The template is relative to the
  package directory and may use `{id}` and any field of the entry, including nested fields
  (`{f.g}`, through records and the current case of variants): enums give their wire value, refs
  their key, variants their case's wire name. Without it, new entries go to
  `<table>/<id>.canon`. Moving a file never changes the entry. Details:
  [spec/API.md](spec/API.md) §10.1.

---

## 5. Types

### 5.1 Scalars

| Type | Meaning | Go | C++ | TypeScript |
|---|---|---|---|---|
| `Bool` | `true` / `false` | `bool` | `bool` | `boolean` |
| `Int` | 64-bit signed | `int64` | `int64_t` | `number` (§15.4) |
| `Int8` `Int16` `Int32` | sized signed | `int8`… | `int8_t`… | `number` |
| `UInt8` `UInt16` `UInt32` `UInt64` | sized unsigned | `uint8`… | `uint8_t`… | `number` / `bigint` |
| `Float` | 64-bit IEEE 754 | `float64` | `double` | `number` |
| `Float32` | 32-bit IEEE 754 | `float32` | `float` | `number` |
| `String` | UTF-8 text | `string` | `std::string` | `string` |
| `Duration` | a span of time, millisecond precision | `time.Duration` | `std::chrono::milliseconds` | `number` (ms) |

A value that does not fit a sized type is an error (`E3201`). NaN and infinities are not valid
`Float` values (`E3202`).

- **Sized integers in expressions.** Every integer type is `Int` in expressions (64-bit signed
  arithmetic, §7.2). A sized type is `Int` plus an implicit range, checked where a value is stored
  (§5.2). In v0, `UInt64` is limited to `0..=INT64_MAX`.
- `Float32` values are rounded to nearest when stored; the only check is finiteness. Emitted JSON
  uses the shortest decimal that round-trips as a 32-bit float.
- `Duration` values are limited to ±9,223,372,036,854 ms, the range of Go's `time.Duration`;
  beyond is `E3201`.

### 5.2 Refinements

A refinement narrows a type. It is reported at the value's source location.

```
Int(0..=100) // inclusive range; `0..100` is half-open
Int(1..) // lower bound only
Float(0.0..=1.0)
Duration(1s..=10m)
String(1..=64) // on String: length in bytes
[Window](1..=64) // on a list or map: number of elements
String(/^II_[A-Z0-9_]+$/) // regex: RE2, search semantics; anchor with ^ and $
Float where it > 0.0 and it < 1.0 // general form; `it` is the value
[Int] where it.len() == 2 and it[0] <= it[1]
```

- Range bounds are constant expressions.
- A regex refinement uses RE2 syntax with **search** semantics, like Go's `MatchString`: the
  pattern must match somewhere in the string. `String(/^IDS_/)` accepts `"IDS_TXT_1"`; write
  `^…$` to match the whole string. `matches` (§20.2) uses the same semantics.
- `Name(args)` after a built-in scalar (or an alias of one) is a refinement and takes exactly one
  range or regex; after a parameterized record or alias it passes values (§5.4, §5.11). Chaining
  refinements (`Int(0..)(..=5)`) is `E1103`: use `where` for more.
- A `where` predicate is any pure expression over `it` returning `Bool`. It may call functions. It
  applies to the non-`none` value (`Int? where it > 0` is "none, or a positive Int"); in a union it
  binds to the last alternative.
- Refinements compose: `Int(0..) where it % 2 == 0`.
- **When refinements are checked.** Refinements play no part in static typing (`Int(0..)` is
  `Int` to the type checker), except that a constant literal out of bounds is `E3201` at compile
  time. At evaluation, they are checked at every conversion into a declared type: an annotated
  `let`, a record field, an element of a typed list or map, a function argument or return value,
  an amended value and a loaded value.
- Generated code uses the **base type**: `Int(0..=100)` is emitted as `int64_t` / `int64`, never
  narrowed from its bounds, because the fingerprint (§14.4) does not cover refinements. Narrow
  types come only from the explicit sized types of §5.1 (`Int32`, `UInt16`, …). The check never
  runs at runtime (§14.5), except for runtime inputs (§19.3).

### 5.3 Enums

```
enum Weekday { Sun, Mon, Tue, Wed, Thu, Fri, Sat }

/// `ordered`: declaration order is rank, so `<` compares positions.
enum Role ordered { member, gm_junior, gm_senior, maintainer, owner, admin }

/// A member may have a different wire value.
enum Tone { warning, info, series_1 = "series-1", series_2 = "series-2" }

/// Persisted numeric codes: stable forever (§12).
enum Element @codes(UInt8) { FIRE = 1, WATER = 2, ELECTRICITY = 3, WIND = 4, EARTH = 5 }
```

- Members are referred to as `Weekday.Mon`, or bare (`Mon`) where the expected type is known
  (§6.2).
- Every member has `.name` (its Canon name), `.index` (position from 0) and `.wire` (its wire
  value). With `@codes`, `.code` is its number.
- `ordered` enables `<`, `<=`, `>`, `>=`.
- An enum with `@codes` is stable: members may be retired (`retired FIRE = 1`) but never
  removed, renumbered or reused (§12). A retired member stays for `match` exhaustiveness and in
  generated code; using it in a value is `E3506`, except inside a retired entry.
- **Wire value.** Without `@codes`, a member's wire value is its name, or the string after `=`.
  With `@codes`, `=` gives the code, so a different wire name is written `@json("…")` on the
  member; the wire value is still the name unless the enum has `@json(codes)`, in which case the
  wire holds the number. Codes otherwise appear only in generated code and in `canon.lock`.
- `@cpp(defines: "IK1_")` additionally emits the members as C `#define`s for legacy code (§15.3).

### 5.4 Records

```
/// A weekly schedule window.
record Window {
  /// Day the window opens (UTC).
  day: Weekday
  startUtc: TimeOfDay
  endUtc: TimeOfDay
  note: String? = none

  check startUtc != endUtc else "zero-length window"

  fn lengthMinutes(self) -> Int { … }
  export fn isOvernight(self) -> Bool { return endUtc.minutes() < startUtc.minutes() }
}
```

- A field is `name: Type`, optionally `= default`, optionally followed by annotations.
- A field is **required** unless it has a default or an optional type. An optional field without
  a default defaults to `none`.
- A default may use constants, earlier fields of the same record and the standard library; not
  package values or user functions.
- Records are **closed**: an unknown field in a literal or a loaded file is an error (`E3301`),
  unless the load uses `partial: true` (§13).
- Records are **nominal**: two records with the same fields are different types.
- Inside a record body, fields are in scope by name and `self` is the record value.
- A record may contain `check`/`warn` (§10), `fn` methods (§9) and `export fn` (§9.4).
- Field names are clean camelCase (`heal`, `itemKind`); legacy names go in `@json` (§5.12).
- A record used as the element of a `table` may not declare fields named `id` or `retired`
  (`E2105`): entries already have them (§5.7). A keyed-list element may. A record may not have a
  field and a method of the same name (`E2104`).
- Record annotations: `@json(case: snake)` (§5.12), `@cpp(…)` (§15.3).

**Parameterized records** take values, not types:

```
record HourlyTarget(e: EventType) {
  ratesBySpecific: {Param(e) | "default": {Stage: Int(0..)}}? = none
}
```

The parameter is in scope in field types, defaults and checks. `HourlyTarget(x)` is a type
wherever `x` is known. Statically, every instance is the erased type `HourlyTarget(_)`; field types
that depend on the parameter are checked at evaluation (§5.11).

### 5.5 Variants

A variant is a closed choice between named cases, each with its own fields:

```
variant Reward @json(tag: "type") {
  item { define: ref items, count: Int(1..) = 1 }
  gold { amount: Int(1..) }
  nothing
}
```

- A case with no fields is written alone (`nothing`).
- Literal: `item { define: II_GEN_GOLD, count: 3 }`, or `Reward.item { … }` (always accepted);
  `nothing`. A case whose fields all have defaults may be written bare (`item` is `item {}`); a
  case with required fields cannot (`E3302`).
- A case may contain checks and methods like a record. A case may not declare a field named
  `kind` (a record field `kind` holding a variant is fine: `e.kind.kind`).
- Wire form: always an object whose tag key (default `kind`, configurable with
  `@json(tag: "…")`) comes first and holds the case's wire name, followed by the case's fields; a
  case without fields is `{"kind": "nothing"}`. A field whose type is a variant can be written
  **inline** in its parent object with `@json(inline)`: the tag and the case fields then sit
  directly in the parent. Only a non-optional field can be inline (`E3316`), a record has at most
  one inline variant, and a wire name that collides with a parent field is `E3318`.
- `v.kind` is the case, as a value of the generated enum `<Variant>Kind`. `v is item` tests it.
  Pattern matching: §7.8.
- The fields and methods of a case are readable only on a value known to be that case: a case
  literal, a `match` binding (`item(r) => r.count`), or a path narrowed by `is`
  (`if reward is item { total += reward.count }`). On a plain `V` they are `E3003`. Details:
  [spec/TYPES.md](spec/TYPES.md) §8.3.

Variants replace every "field X is required when type is Y" rule: each case has exactly the
fields that make sense for it.

### 5.6 Optional values

`T?` is a `T` or `none`. Optionals are **strict** (DECISIONS 17): a `T?` is never accepted where a
`T` is expected (`E3403`), and `.f`, `.m(…)`, `[ ]`, calls, iteration and arithmetic do not apply
to a `T?` (`E3402`). Both are compile-time errors. A `T` is accepted where a `T?` is expected.
Code proves presence in one of four ways:

- **Narrowing.** A test `x != none` narrows `x` from `T?` to `T` wherever the test is known to
  hold: in the right operand of `and`, the branch of an `if` statement or `if` expression, the body
  of `while`, the later clauses of a comprehension after an `if` clause, the other arms of a
  `match` with a `none` arm. Symmetrically, `x == none` narrows `x` in the right operand of `or`
  and in the `else` branch, and after `if x == none { return … }` for the rest of the block.
  `x` is a **stable path**: a local, `var`, parameter, loop variable, field, `self` or top-level
  value, followed by field or entry-key segments (`at.jobBase`, `flags.sensitive.hint`), never an
  index or a call. Assigning a `var` ends the narrowing of paths rooted at it.
- **Fallback.** `x ?? fallback` is `x` when present, else `fallback`; its type is `T` when
  `fallback` is a `T`.
- **Chaining.** `x?.f` and `x?.m(…)`: when `x` is `none`, the **whole rest of the chain** is
  skipped and the chain is `none`. The chain's type is made optional once, at its end (never
  `T??`): `w?.item.id` is `String?` when `w` is optional and `item` is not. Parentheses end a
  chain.
- **Assertion.** Postfix `x!` is `x` as a `T`. If `x` is `none` it is `E4001` at that expression.

```
let tier = tiers.get(level) // Tier?
check tier == none or tier.totems else "…" // narrowed in the right operand of `or`
let name = parent?.name ?? "(root)" // String
let first: ref Severity = severities.first()! // asserted: E4001 if the table is empty
```

`T??` is not a type (`E3401`). A `!`, `?.`, `??` or `none` comparison on a value that is never
`none` is `W3401`. The full rules (facts through `not`, `and`, `or` and `==`, early exits, loops,
`var` kills) are [spec/TYPES.md](spec/TYPES.md) §6.5 and §6.6.

### 5.7 Collections

| Type | Meaning |
|---|---|
| `[T]` | list, ordered |
| `{K: V}` | map, keeps insertion order; `K` is `String`, an integer type, an enum, a `ref`, or a literal union over one of these (§5.9) |
| `table T` | ordered collection of `T` keyed by **identifier**; every entry gets `.id` |
| `stable table T` | a `table` whose ids are a permanent contract (§12) |
| `[T] keyed by f` | a list whose field `f` is unique; entries can be referenced by `f` |

- Table and keyed-list keys are unique (`E3101` for tables, `E3102` for keyed lists). A keyed
  list's key field is a `String`, an integer type, an enum or a `ref` (`E3012`).
- **Access by key.** On a table or keyed list, `xs[k]` and `xs.k` (for identifier keys) read the
  entry with key `k`, and `xs.get(k)` returns it as an optional; position is `xs.at(i)`. A `Range`
  index is still a positional slice. On a plain list, `xs[i]` and `xs.get(i)` are positions (from
  0, negative from the end). On a map, `m[k]` reads the entry and `m.get(k)` returns an optional;
  `m.k` is `E3003` (write `m[k]`). A missing key or index is an evaluation error (`E4002`).
- `x.k` without parentheses is always a field or an entry, and `x.k(…)` always a method, so a key
  named `count` or `filter` is harmless.
- Every table entry has `.id` (its key, an identifier) and `.retired` (§12).
- Tables and keyed lists accept every list method over their entries (§20.3), and their entries
  may be declared in other files with `entry` (§4.3).
- In generated code, the keys of a public table form an id enum (Go `ItemID`, `StatusID`; C++ and
  TypeScript `ItemId`, `StatusId`) in `baked` and `embedded` modes. In `data` and `types` modes
  ids are a distinct string type, so adding an entry changes only the data file (§14.2).

Details: [spec/TYPES.md](spec/TYPES.md) §9, [spec/STDLIB.md](spec/STDLIB.md) §5.

### 5.8 References

`ref T` holds the key of an entry in a table or keyed list whose element type is `T`. `ref` is
followed by a (qualified) name only: a type name `T`, or a collection name. `?`, refinements and
`where` then apply to the ref type, so `ref Node?` is `(ref Node)?`.

- **Which collection.** `ref name` naming a collection (including `local` ones and define tables
  from `load.defines`) targets it: `ref areas`, `ref items`. `ref T` naming a type searches, in
  order, and stops at the first level that has any candidate collection of `T`:
  1. collection-typed fields of the enclosing record types (`parent: ref Node?` next to
     `nodes: [Node] keyed by id`); the ref is resolved per enclosing value;
  2. top-level lets of the package, `local` included;
  3. public lets of imported packages.

  The level must have exactly one candidate, else `E2103` (write `ref name`).
- A public type may refer to a `local` collection; other packages validate against it, and
  generated code holds such refs by key (§15.1).
- A key is written as an identifier or, for a keyed list with a non-`String` key field, as a
  literal of that field's type (`modelType: 3`).
- A dangling reference is an error (`E3501`) at the reference's location. A reference from a live
  entry to a retired one is an error (`E3502`).
- **Entries, refs and values.** Statically there are `T` and `ref T`. An entry of a table or keyed
  list is a `T` that also carries an identity (collection, key); `self` in a record body is an
  entry when the value is one. `ref T` converts implicitly to `T` (dereference), so a ref behaves
  like the entry it points to: `s.next[0].label`. `T` converts implicitly to `ref T`, which is
  valid only if the value is an entry of the ref's target collection (`E3503` otherwise). `==`
  between a ref and an entry compares identity; `r.id` is the key.
- Wire form: the key (for a keyed list, the key field's wire form).
- References may form cycles (`next: [ref Status]`).

Details: [spec/TYPES.md](spec/TYPES.md).

### 5.9 Special types

| Type | Meaning |
|---|---|
| `Never` | has no values. `Never?` accepts only `none`; a non-optional field whose type is `Never` makes its value unbuildable (`E3801`). Used by dependent types (§5.11). |
| `A \| "lit"` | an `A`, or exactly the string `"lit"`. Only string literals may be alternatives, and `A` must have a string wire form (`String`, an enum, a `ref`). On decode, the literal wins over a same-spelled key. |
| `Range` | the value of `a..b`: an **integer** range with `.start`, `.end` (end exclusive; `a..=b` stores `b + 1`), `.len()`, `.isEmpty()` and `.contains(x)`. `.end` and `.len()` of an open range (`0..`) are `E4002`. `Float` and `Duration` ranges exist only in refinements, and `..b` only as a slice index (`E3025` as values). |
| `fn(A, B) -> R` | a function type, for parameters that take a function or lambda (§9.1). |
| `_` | "any type", only in widget parameter types (§16.11). |
| `asset(root, ext: […])` | a file name under an asset root; the file must exist, with exact letter case (§16.5). |

### 5.10 Aliases

```
type Price = Int(0..)
type ItemCount = [Int(1..=100_000)] where it.len() == 2 and it[0] <= it[1]
```

An alias is the same type as its definition. It exists for naming and documentation.

### 5.11 Types that depend on a value

A type may take value parameters and choose its shape with `match`:

```
type Param(e: EventType) = match e.param {
  monster => ref monsters
  item => ref items
  none_, stat => Never
}

record Task {
  eventType: ref eventTypes
  filterParam: Param(eventType)? = none // a field type may use earlier fields
}

record QuestConfig {
  hourlyTargets: {e in eventTypes: HourlyTarget(e)} // a value type that depends on the key
}
```

- A field type may refer to **earlier** fields of the same record. The field is checked against
  the type computed from that value.
- `{k in coll: T(k)}` is a map keyed by `ref coll` whose value type depends on the key.
- **Restrictions.** A type-level `match` must be exhaustive (checked statically). Its scrutinee is
  an enum- or `Bool`-typed path rooted at a type parameter or an earlier field; parameters are
  records, refs, enums or `Bool`; result types have no refinement that depends on the parameter.
- **Checking.** The type function is evaluated for each value when values are verified (stage B,
  §11.2), and counts toward the step budget. A mismatch is `E3802` ("value does not match
  `Param(COMBAT_KILL_FFA)` = `Never`"). `none` is always valid for an optional dependent field.
- **In expressions**, a dependent field has a synthesized union type (`Param(*)`) that supports
  only `==`, `!= none`, interpolation, `String(x)` and `match` on the discriminant. There is no
  narrowing by discriminant in v0.
- **Generated code**: a dependent type is emitted as a generated union named after the alias
  (`Param`), with a branch enum `<Alias>Branch` (`ParamBranch`) whose members are the first
  pattern of each arm (`Never` arms have none); TypeScript discriminates on `branch`. The loader
  reads the discriminant first; the wire is unchanged (untagged). A dependent map is a map from
  the ref key to the erased value type; dependent key types become `String` keys. Which branch
  applies to which value is a build-time guarantee.
- Studio: the editor of a dependent field follows the value it depends on (§16.2).

Details: [spec/TYPES.md](spec/TYPES.md), [spec/CODEGEN.md](spec/CODEGEN.md).

### 5.12 Wire names and annotations

Canon names are clean (`heal`, `itemKind`). Files may use other names.

| Annotation | On | Effect |
|---|---|---|
| `@json("nHeal")` | field, case, member of a `@codes` enum | wire name (other members use `= "wire"`, §5.3) |
| `@json(path: "legacy.reqMp")` | field | the value sits deeper in the wire object; a missing intermediate object reads as the default |
| `@json(case: snake)` | record, variant, case | default wire names of fields are converted: `snake` (`targetTime` ↔ `target_time`), `camel` (identity), `kebab`, `upper_snake` |
| `@json(tag: "type")` | variant | tag key (default `kind`) |
| `@json(inline)` | non-optional field of variant type | tag and case fields sit in the parent object |
| `@json(none: -1)` / `@json(none: "")` / `@json(none: {})` | optional field | the wire's way of writing `none` (default `null`) |
| `@json(unit: s)` | field holding `Duration`s | the wire holds a number in `ms`, `s`, `m`, `h` or `d` (default: integer `ms`); a value that is not a whole number of milliseconds is `E3203` on read, and a value not exact in the unit is `E8102` |
| `@json(codes)` | `@codes` enum | the wire holds the member's code instead of its name |
| `@json(int)` | field holding `Bool`s | legacy form: the wire holds `0` or `1` (`bIsTradable: 1`) |
| `@json(bits)` | `[E]` field, `E` a `@codes` enum whose codes are powers of two | legacy form: the wire holds one integer, the bitwise OR of the members' codes (`dwFlags: 5` holds the members coded 1 and 4) |
| `@json(pairs: ["dwDestParam{i}", "nAdjParamVal{i}"])` | list of a two-field record | legacy parallel fields (DECISIONS 21): element `i` is written as the two numbered wire keys; code, checks and the studio all see a list of records (`stats: [StatBonus](..=6)`). `{i}` counts from 0, and the list's upper length bound gives the range of indexes |
| `@codes(UInt8)` | enum | persisted numeric codes (§5.3, §12) |
| `@stable` | field of a `stable table` element | its values are recorded in `canon.lock` like ids (§12) |
| `@files("…/{id}.canon")` | table or keyed-list `let` | where new entry files go (§4.3) |
| `@reload` | `let` | runtimes may swap this value while running (§15.5); default is load once |
| `@menu(m, icon: i, label: "…")` | public `let` | studio navigation entry of this value; it wins over its type's view `menu` (§16.6) |
| `@deprecated("why")` | field, member, case, entry | still loaded, checked, emitted and fingerprinted; setting it in a literal is `W3301`; read-only in the studio |
| `@since(7)` | anything | documentation: the format version that introduced it |
| `@cpp(struct:, header:, access:)`, `@cpp(field:, type:)` | record, field | legacy C++ struct mapping (§15.3) |
| `@cpp(value: 1)` | variant case | the integer written to the legacy member that holds an inline variant's case (§15.3) |
| `@cpp(unit: s)` | `Duration` field mapped to a legacy member | the member holds the duration in that unit (default `ms`) |
| `@cpp(defines: "IK1_")` | enum | also emit the members as C `#define`s (§15.3) |
| `@cpp(name:)`, `@go(name:)`, `@ts(name:)` | type, field, member, case, method, `let`, `const`, `fn` | override the generated name in one target (§15.1) |
| `@ts(bigint)` | integer field | emitted as a TypeScript `bigint` (§15.4) |

- Annotation arguments are strings, integers, literals of the annotated field's wire form
  (`none:`), or **symbols** from a closed set per annotation (`s`, `snake`, `inline`, `fields`).
  Symbols are never resolved in scope. An unknown annotation or argument is `E1104`; an
  annotation at a position it does not allow is `E1118`; one that does not apply to the field's
  type is `E3316`. The full catalogue (positions, parameters, kinds, which `@json` forms combine)
  is [spec/GRAMMAR.md](spec/GRAMMAR.md) §8.
- Annotations never change what a value means; they only change how it is written or emitted.
  Renaming a Canon field never changes a data file, because the wire name stays in `@json`.
- Encoding details (none, units, maps, variants, snake-case algorithm, `path`, `bits`, `pairs`):
  [spec/WIRE.md](spec/WIRE.md).

---

## 6. Values and literals

### 6.1 Literals of composite values

```
[1, 2, 3] // list
{ hour: 8, minute: 30 } // record (type from context)
TimeOfDay { hour: 8, minute: 30 } // record, explicit type
{ "Cap": [a, b], "Boots": [c] } // map with string keys
{ Stage_1: 20, Stage_2: 20 } // map with enum keys
{ open { … }, taken { … } } // table: entries are `key { fields }`
item { define: II_GEN_GOLD } // variant case
0..10 // a half-open range
0..=10 // an inclusive range
```

- In a record literal, every required field must be given (`E3302`) and no unknown field may be
  (`E3301`).
- **Spread** copies a record and overrides fields: `{ ...base, label: "Other" }`. It is the only
  form of inheritance. A literal has at most one spread, and it comes first. Its type must equal
  the expected type (`E3303`); a case may spread a value of the same case. Refinements and checks
  run again on the result.
- Table entries may carry doc comments and the `retired` prefix. Inside `{ }`, entries are
  separated by newlines or commas (§2.3).
- **What a `{ … }` is** is decided by the type checker, from its items and its expected type:
  items `key { … }` make a table; `expr: expr` followed by `for` is a map comprehension; with an
  expected record or case, `name:` is a field; with an expected map whose key is an enum or a
  ref, `name:` is resolved as in §6.2; with an expected map whose key is `String` or an integer,
  `name:` is `E3304` (write `"name":` or `(name):`). Without an expected type only `"str": v`
  items or a comprehension are allowed, and `{}` is `E3305`. Details:
  [spec/TYPES.md](spec/TYPES.md).

### 6.2 Contextual names

Name resolution and type checking are one bidirectional pass: the expected type of each
position is known when its names are resolved. A bare identifier in value position is resolved
in this order:

1. against the **expected type**: a member of the expected enum, a case of the expected variant,
   a key of the collection targeted by the expected `ref`;
2. against scope: locals and parameters, then fields and `self` members (inside record bodies),
   then package names, then imports, then built-in types and functions. Methods are looked up
   only after `.`.

`tone: warning` therefore needs no `Tone.` prefix, and `next: [taken, wont_do]` resolves to
status keys.

- **A step-1 match wins.** When the expected type is an enum, a variant or a ref and step 1
  matches, that meaning is used silently, even if a package or imported name has the same
  spelling (`icon: columns` is the `Icon` member even beside `let columns`). It is an ambiguity
  error (`E2101`) only when a **local or parameter** of the same name has a type assignable to
  the expected type; write it qualified.
- **Keys not yet known.** When the expected type is `ref C` and the identifier matches no
  declared name, it is recorded as a symbolic key and checked once `C` is evaluated (`E3501` if
  absent). This is how keys of loaded collections (`load.defines`, `load.dir`) are written. The
  ambiguity test uses declared names only, never loaded keys.

### 6.3 Equality and ordering

- `==` needs the same base type up to optionality; `none == none` is true.
- Scalars, strings and durations compare by value. Floats compare by IEEE value, also inside
  records.
- Records, variants and lists compare structurally (deep equality). Maps compare as sets of keys
  with equal values, ignoring order. A keyed list equals a list with equal elements.
- Refs compare by key; entries compare by identity (collection and key); a ref and an entry compare
  by identity. Refs into different collections cannot be compared (`E3309`).
- `<` and friends are defined on numbers, durations, strings (byte order) and `ordered` enums;
  elsewhere (`Bool`, refs, lists) they are `E3310`, and on optionals `E3403`.

Details: [spec/TYPES.md](spec/TYPES.md) §7.5.

---

## 7. Expressions

### 7.1 Precedence

From lowest to highest:

| Level | Operators | Associativity |
|---|---|---|
| 0 | lambda `x => e` | right (the body extends as far as possible) |
| 1 | `??` | right |
| 2 | `or` | left |
| 3 | `and` | left |
| 4 | `not` (prefix) | — |
| 5 | `==` `!=` `<` `<=` `>` `>=` `in` `is` | none (no chaining) |
| 6 | `..` `..=` | none |
| 7 | `+` `-` | left |
| 8 | `*` `/` `%` | left |
| 9 | unary `-` | — |
| 10 | postfix: `.f` `?.f` `[i]` `(args)` `!` | left |

A `?.` that meets `none` skips the rest of its postfix chain (§5.6).

### 7.2 Arithmetic

- `Int` arithmetic is 64-bit. Overflow is an evaluation error (`E4101`), never a wrap.
- `/` on two integers truncates toward zero; `%` has the sign of the left operand (as in Go and
  C++). Division by zero is `E4102`.
- All integer types compute as `Int` (§5.1); the result is range-checked only when stored.
- There is no implicit conversion between `Int` and `Float`, except that an integer **literal
  token** (optionally with a leading `-`) is accepted where a `Float` is expected; an integer
  constant is not (`E3311`, write `Float(N)`). Convert explicitly: `Float(i)`, `Int(f)`
  (truncates; `E4103` if out of range), `round(f)`.
- A `Float` operation that produces NaN or an infinity (`x / 0.0`, `sqrt(-1.0)`) is `E4104`.
- `Duration ± Duration`, `Duration * Int` and `Int * Duration`, `Duration / Int` (truncates toward
  zero at the millisecond), `Duration / Duration` (→ `Float`) are defined; `Duration * Float` is
  not. Durations may be negative in expressions; refinements decide whether they are valid in
  data.
- `+` concatenates strings and lists. Prefer interpolation for strings.

### 7.3 Comparison and membership

- `a in xs` tests membership in a list, the keys of a map or table, or a range.
- `v is case` tests a variant's case. The right side is a case name (possibly qualified), resolved
  against the variant of the left side, not an expression.
- Comparisons do not chain: `a < b < c` is an error; write `a < b and b < c`.

### 7.4 Boolean logic

`and`, `or` short-circuit. `not` negates. Operands must be `Bool` (no truthiness).

### 7.5 Access

- `x.f` reads a field, or the entry with key `f` of a table or keyed list; `x.m(…)` calls a
  method (§5.7). On a map, `.f` is `E3003`.
- `x[i]` indexes a plain list by position (from 0; negative indexes count from the end), and
  looks up a table, a keyed list or a map by key. `xs.at(i)` is the position in a table or keyed
  list. A missing index or key is `E4002`.
- `.` and `[ ]` on a `T?` are `E3402`, at compile time (§5.6).
- `x?.f` and `x?.m(…)` are `none` if `x` is `none`, and so is the rest of the chain; the result
  is optional.
- `x!` is `x` without its optionality; `E4001` if `x` is `none` (§5.6).
- Indexing with a `Range` value slices lists and strings by position: `xs[a..b]`, `xs[a..]`,
  `xs[..b]`.

### 7.6 Calls

`f(a, b)`, `f(a, name: b)`. Named arguments follow positional ones. Methods: `x.m(args)`.
A method with no arguments still needs `()`.

### 7.7 Conditional expression

```
if level <= 15 { vagrant } else if level <= 60 { expert } else { pro }
```

`if` is an expression when every branch is a single expression and `else` is present. At the
start of a statement, `if` is always the statement form (§8). The branch types are joined (as for
list elements, `??` and `match` arms): refinements are dropped, `none` and `T` give `T?`, and an
empty list or map takes the other branch's type; with only empties and no expected type it is
`E3308`.

**Header expressions.** In the header of `if`, `else if`, `while`, `for … in`, `match` (value and
type level) and a view's `when`, a name immediately followed by `{` is never a typed literal: the
`{` opens the body. `if settings.includeExtra { … }` therefore reads the field and opens the
block. Parenthesize to use a typed literal there: `if (Point { x: 1, y: 2 }) == p { … }`.

### 7.8 `match`

```
match reward {
  item(r) => r.count // binds the case's value as `r`
  gold(g) => g.amount
  nothing => 0
}
match e.param {
  monster, item => true // several patterns
  _ => false
}
```

- The scrutinee is an enum, a variant, a `Bool`, or an optional of one of these (with a `none`
  pattern). Such a `match` must cover every member or case, retired ones included, or have `_`
  (`E3601`). On any other type `_` is required. Literal patterns are not allowed in v0.
- In a value `match`, a `{` after `=>` starts a brace literal; in a statement `match` it starts a
  block.

### 7.9 Lambdas

`x => expr`, `(a, b) => expr`, `(i, x) => expr` (destructures a pair from `enumerate`, `zip`, map
iteration). The shorthand `.field` or `.method()` means `x => x.field`. Lambdas are pure and
capture by value. A lambda needs an expected function type (from the parameter of a user
function or of a standard function, §9.1): its parameter types come from it and its body is
inferred.

### 7.10 Comprehensions

```
[s.id for s in skills if s.level <= lv]
{ j.id: j.lineage() for j in jobs }
[combo(j, w) for level in levels for j in jobsAt(level) if ok(j) let w = pick(j, level)]
```

Clauses `for`, `if` and `let` may repeat, in any order after the first `for`, and apply left to
right.

---

## 8. Statements

Statements appear in function bodies, `check { }` blocks and `test { }` blocks.

| Statement | Meaning |
|---|---|
| `let x = e` / `let x: T = e` | immutable local |
| `var x = e` | mutable local |
| `x = e`, `x += e`, `-=`, `*=`, `/=`, `x[k] = e` | assignment to a `var`, or to an element (at any depth) of a list or map held by a `var`; `x.f = e` is `E3307` |
| `if c { … } else if … { … } else { … }` | conditional |
| `for x in xs { … }`, `for i, x in xs.enumerate()`, `for k, v in m` | iteration |
| `while c { … }` | loop |
| `break`, `continue` | loop control |
| `return e` | return from a function |
| `match e { … }` | as a statement, arms may be blocks |
| an expression | evaluated for its effect: `fail(…)`, `warn(…)` (§10.2) |

**Value semantics.** Lists and maps have value semantics: after `var xs = [1]; let ys = xs;
xs[0] = 2`, `ys` is still `[1]` (the evaluator copies on write). `xs += e` rebinds
(`xs = xs + e`). Only a `var` can be mutated, so a value that leaves the call (returned, stored in
a record, captured) can never change; `E4201` remains as an internal safety net. Evaluation order
therefore never changes a result. Details: [spec/EVALUATION.md](spec/EVALUATION.md).

---

## 9. Functions

### 9.1 Declaration

```
fn occupancy(w: Window) -> [Range] { … }
fn clampLevel(level: Int, max: Int = 150) -> Int { return min(level, max) }
```

- Parameter types and the return type are required.
- Parameters may have constant defaults. Arguments may be passed by name.
- Functions and lambdas may be passed to functions whose parameter has a **function type**
  `fn(A, B) -> R`:

  ```
  fn countWhere(xs: [Int], pred: fn(Int) -> Bool) -> Int { return xs.count(pred) }
  ```

  Function values cannot be stored in records, lets or collections, and are never emitted
  (`E3306`).
- Recursion is allowed; the step budget bounds it.

### 9.2 Methods

A `fn` inside a record, variant or case body takes `self` first:

```
record TimeOfDay {
  hour: Int(0..=23)
  minute: Int(0..=59)

  fn minutes(self) -> Int { return hour * 60 + minute }
}
```

Inside a method, fields are in scope by name.

### 9.3 Package functions

A top-level `fn` sees every value of its package and imports. It can read `load`ed values, so
data can be computed from data (`examples/balance/parity/sweep_plan.canon`).

### 9.4 Exported functions

`export fn` makes a function or method available in generated code. What is emitted depends on
its parameters:

| Parameters (besides `self`) | Example | Emitted as | Logic at runtime |
|---|---|---|---|
| none | `isStrong(self) -> Bool` | a value computed at build time, returned by a getter | none |
| only **finite** types: `Bool`, enums, `ref` into a table | `canTransition(from: ref Status, to: ref Status)`, `areasVisibleTo(role: Role)` | a lookup table covering every input | none |
| at least one `Int`, `Float`, `String` or `Duration` | `healFor(self, missingHp: Int)` | the body **translated** into each target | yes, with a conformance test |

A `ref` into a keyed list is not finite: a function taking one falls in the third kind, whose
parameters exclude refs (`E9006`).

Return types: any type for the first two kinds. A lookup table is a dense array over every input
combination, retired members included; more than 65,536 cells is `E9002`, and optional
parameters are `E9003`.

A **translated** function takes only `Bool`, integer, `Float`, `String`, `Duration` and enum
parameters (a `ref`, record, list, map or variant parameter is `E9006`) and returns one of these
or a `ref` (`E9004`); refinements of its parameters and result are limited to ranges and sized
types (`E9007`). A translated **method** must be called by at least one `test` of its package,
which gives the conformance test its receivers (`E9008`).

**The portable subset.** A translated body may contain only:

- `let`, `if` / `else`, `return`;
- arithmetic and comparisons on `Int`, `Float` and `Duration`; `Float(i)`, `Int(f)`, `min`,
  `max`, `abs`, `floor`, `ceil`, `round`, `clamp`;
- string templates over `String`, `Int` and enum values (no `Float`s, no format specs);
- `and`, `or`, `not`, `??`;
- reading fields of `self` and of parameters; enum members and variant cases;
- calls to other exported functions of the same package.

Anything else (loops, collections, string functions, non-exported calls) is `E9001`, naming the
construct.

**Same result everywhere.** Integer overflow, division by zero, rounding and conversions behave
differently in Go, C++ and TypeScript, so a translated body never uses the target's raw operators
for them: it calls generated **checked helpers** that reproduce Canon's semantics and report
Canon's error code where Canon's evaluator would fail (`E4101`, `E4102`, …):

- C++ uses no exceptions: a helper calls `canon::OnEvalError(code, message)`, a replaceable
  handler whose default logs and calls `std::abort()` (DECISIONS 19);
- Go panics with a `*rt.EvalError`, from the generated `rt` helper package (§15.2);
- TypeScript throws a `CanonEvalError`; integer inputs outside `Number.isSafeInteger` throw as
  well (`E8303`, §15.4).

**Conformance.** For each translated function, `canon build` emits a test per target. The
translated body is emitted as a pure function that both the method and the test call, so the test
checks the code that runs. The test feeds input vectors and compares with the results the Canon
evaluator computed, including errors: a vector on which Canon fails expects the same error code.
Vectors are chosen deterministically:

- receivers: the distinct `self` values used by the package's `test` calls of that function,
  projected on the fields the body reads;
- per parameter: the values used in tests, plus 0, 1, −1, the type's limits, each refinement
  bound −1/0/+1, and each `Int` field of `self` read by the body −1/0/+1, kept within the
  parameter's domain;
- combined as a cartesian product capped at 256 per receiver, else pairwise; test vectors first,
  then the generated ones sorted and deduplicated.

Floats must match bit for bit. Details: [spec/CONFORMANCE.md](spec/CONFORMANCE.md).

---

## 10. Checks

Checks are the whole validation story: there is no separate schema, keyword vocabulary or rule
registry.

### 10.1 Forms

```
check <bool expr> else "message" // error
warn <bool expr> else "message" // warning
check name: <bool expr> else "message" // named
check <bool expr> at <field> else "message" // reported at one field of the record
check { …statements… } // block form
```

- In a record, variant or case, a check runs **once per instance** of that type, with the fields
  in scope: each instance reachable from an evaluated top-level `let` once it is fully built, and
  from each `expect` subject (§18). Two equal values built by two literals are two instances,
  checked twice (EVALUATION.md §4.2, §8.1). Temporaries inside functions are not checked (their
  refinements are, §5.2).
- At package level, a check runs **once**, with every value of the package and its imports in
  scope.
- The message is a string template over the same scope.
- `at <field>` (one-line checks of a record or case only) reports the finding at that field's
  value instead of at the record, so the studio shows it next to the field. It is written on the
  line where the condition ends; an unknown field is `E1633`. From the farm example:

  ```
  warn unreachable_levels: maxLevel <= levels.len() at maxLevel
    else "maxLevel is {maxLevel} but only {levels.len()} levels exist"
  ```

### 10.2 Block form

Inside `check { }`, two functions report findings:

```
fail(at, "message") // error
warn(at, "message") // warning
```

`at` is any value. The finding points to **that value's source location**: every value remembers
where it was written, including values read from JSON (§11.5). Nobody builds a JSON pointer by
hand. `fail` and `warn` may appear only lexically inside a `check { }` block; calling them from a
helper function is `E1105`. `check {` always starts a block form.

### 10.3 Rules

- A one-line check reports at the value it belongs to: the record literal's opening token, the
  key of a table entry, or the `{` of a JSON object; with `at <field>`, at that field's value. A
  package-level one-line check reports at the check.
- Codes: a one-line `check`/`warn` is `E5001`/`W5001`; `fail`/`warn` in a block is
  `E5002`/`W5002`. The check's name, if any, is the finding's `check` field.
- A **name** makes the check addressable: translation key `Type.check.name`, or `check.name` at
  package level (§17), and test target `expect v fails name` (§18).
- **Every check runs** and every finding is reported. A build never stops at the first error.
- Checks run after types, refinements and references are verified. A check never needs to guard
  against a missing or badly typed value. A check that reads a value whose evaluation failed is
  skipped silently (§11.4).
- **Canon is the law.** A check states what is correct, not what a runtime happens to tolerate.
  A deliberate exception is written as a named `warn`.
- Errors block code and data outputs of `canon build` (not `emit view`, §14.1) and a studio save.
  Warnings do not.

---

## 11. Evaluation

### 11.1 Order

`canon check` and `canon build` evaluate **every** top-level `let` (public and `local`) of the
selected packages, plus what they need from imports; `canon test` evaluates on demand. Values are
evaluated **lazily**, when first needed, in dependency order. A cycle between values is `E4301`,
showing the cycle; a table entry that reads another entry of the same table is such a cycle (refs
are keys and do not count). The order in which independent values are evaluated has no
observable effect (§8).

### 11.2 Phases of a build

The phases of a build are defined in [spec/EVALUATION.md](spec/EVALUATION.md) §1: parse, resolve
and type-check (one bidirectional pass, §6.2), then **stage A** (evaluate, applying layers to each
value right after its base evaluation), **stage B** (verify types, refinements, references, keys,
dependent types and stable ids), **stage C** (instance checks), **stage D** (package checks),
**stage E** (precompute `export fn` results), and emit. `canon check` runs everything but emit;
`canon build` also emits: code and data only if there is no error, `emit view` always (§14.1).

### 11.3 Determinism

- Map iteration follows insertion order; table iteration follows entry order (§4.3).
- `load.dir` reads files in byte order of their paths.
- Float formatting and parsing are exact (shortest round-trip).
- Findings are sorted by file path (bytes), line, column, code and message; findings without a
  file come first.
- Every written path uses `/`, every output uses `\n` line endings, and path order is the byte
  order of `/`-separated relative paths, on every platform.
- The same sources and the same compiler version produce byte-identical outputs.

### 11.4 Evaluation errors

An error during evaluation (index out of range, `x!` on `none`, overflow, division by zero) is a
finding at the failing expression, with the Canon call stack. The failed value is **poisoned**:
values that depend on it are not evaluated and report nothing more, and checks that read it are
skipped. Other values keep being evaluated and other checks keep running.

### 11.5 Provenance

Every value carries the location it came from: a span in a `.canon` file, or a file, line and
column (1-based, byte columns) and an RFC 6901 JSON pointer in a loaded file. A field filled by
its default points at the default expression, with the literal or JSON object as a related
location; fields copied by a spread keep their original provenance; values computed by functions
carry the location of the expression that built them, plus a call stack of at most 16 frames;
amended values record the layer. Findings, `canon explain` and the studio's edit API all use it.

### 11.6 Step budget

Evaluation runs under a step budget: `project.budget` steps (10⁸ if not set) for one `canon`
invocation, or one re-check through the API, shared by constant folding during checking, values,
verification, checks, precomputation and tests. Exceeding it is `E4401`, with the stack and the
heaviest value (the top-level value charged the most steps). What costs a step, and to which value
it is charged, is [spec/EVALUATION.md](spec/EVALUATION.md) §12; the cost of each standard function
is in [spec/STDLIB.md](spec/STDLIB.md).

Details: [spec/EVALUATION.md](spec/EVALUATION.md).

---

## 12. Stable ids and `canon.lock`

`stable table`, `@codes` enums and `@stable` fields make identifiers a permanent contract: never
renamed, never reused, retired instead of deleted.

- Each package that has any of them keeps a committed `canon.lock` next to its sources. It lists,
  for each stable table, enum and `@stable` field, every value it has ever had and which entry
  held it. For a `@codes` enum the "entry" is the member name, so renaming a member keeps its
  code but is still `E6001`.
- `@stable` is allowed only on fields of `stable table` elements (`E6003`); its values are unique
  among all entries, retired ones included (`E3102`).
- `canon build` appends new values to the lock, and so does a studio `Add` on a stable table,
  inside the same atomic edit. Nothing ever removes one. Layers never write the lock and may not
  add entries to stable tables (`E6004`).
- **Removing** a locked id or code: `E6001`. **Renaming** is a removal plus an addition: `E6001`.
- **Retiring**: prefix the entry, member or case with `retired`. A retired entry stays everywhere: in
  generated id enums (so stored data still decodes), in data files (`"$retired": true`), in baked
  values, in the view model and search (flagged), and in plain iteration. It is left out of
  `.active()` only. A reference from a live entry to it is `E3502`. Retirement is recorded in the
  lock: the value's line ends with `retired`.
- **Reusing** a retired id, or moving a `@stable` value to another entry: `E6002`. So is
  **un-retiring**: removing the `retired` prefix brings a retired id back, which is `E6002`. An id
  comes back only through a reviewed hand edit of the lock (LOCK.md §4.6); the studio's
  `Unretire` is refused. After a git merge, two different entries claiming the same new id is also
  `E6002`.
- The lock is plain text, one line per value, sorted by kind, qualified name, value (numerically
  for integers), then holder, so merges are trivial. A `field` fact is named by the stable table's
  `let`, then the field.

Two excerpts, from `resource/vocab/canon.lock` and from `teamboard/canon.lock` after retiring the
`duplicate` status:

```
# canon.lock v1
enum   resource.vocab.Element  1  FIRE
enum   resource.vocab.Element  2  WATER
field  resource.vocab.eventTypes.code  0  COMBAT_KILL_MONSTER
field  resource.vocab.eventTypes.code  1  COMBAT_KILL_GIANT
```

```
# canon.lock v1
table  teamboard.statuses  duplicate  retired
table  teamboard.statuses  fixed
table  teamboard.statuses  open
```

The exact file grammar, and what `canon lock check` evaluates (stable collections and their
dependencies only), are in [spec/LOCK.md](spec/LOCK.md).

---

## 13. Reading files: `load`

`load` reads files that are not Canon sources: data still in JSON, C headers, CSV. Values it
produces are type-checked like literals, with findings pointing into the loaded file.

### 13.1 Forms

```
load("@resource/Server/System/farm_config.json") // format from extension
load("@resource/Server/Quest/adventure_quest_config.json", at: "styles")
load("@resource/Server/Item/propItem.json", at: "items", partial: true)
load.dir("@resource/Server/Item/Items/**/*.json") // one entry per file
load.defines("@resource/Server/Define/defineObj.h", prefix: "MI_")
load.csv("@resource/Server/Item/propItem.csv", header: true)
load.text("@resource/Server/Text/notice.txt")
```

| Option | Meaning |
|---|---|
| `at: "a.b.c"` | read only that part: `.`-separated segments, `[n]` an array index, `\.` an escaped dot; `*` over an object keeps its keys and yields a map (`at: "canonicalBuilds.*.weapon"`), over an array a list. A missing path is `E7106` |
| `partial: true` | ignore wire fields the expected type does not declare, at every depth |
| `prefix: "MI_"` | `load.defines` only: keep names starting with it |
| `header: true` | `load.csv` only: the first row names the columns (by wire name) |
| `format: json` | force a format: `json`, `csv` or `text` (otherwise from the extension `.json`, `.csv`, `.txt`) |

### 13.2 Semantics

- The **expected type** drives parsing: `let farm: FarmConfig = load(…)` reads the file as a
  `FarmConfig`, applying wire names, units and defaults. `load`, `load.dir` and `load.csv` must
  get their expected type directly from their context (an annotation, a field, an argument, a
  return type), else `E7002`: write `local let all: [Weapon] = load(…)`, then filter it, rather
  than `load(…).filter(…)`.
- **JSON** is strict RFC 8259 (a BOM is tolerated; non-UTF-8 is `E7105`, duplicate keys `E7104`).
  An absent key means the default; `null` means `none` for an optional field and is `E3315` for a
  required one. An `Int` accepts only integer tokens (`2.0` is `E7103`); numbers are parsed exactly
  before conversion. A `$schema` key at the root of a loaded file is ignored; any other `$` key
  except `$retired` on table rows is an unknown key.
- `load.defines` returns a table keyed by the full define name; each entry has `.value: Int`. A
  `ref` into it checks that a name exists. There is no preprocessor: every `#define NAME EXPR`
  whose EXPR is an integer expression (literals, parentheses, `- | & + << >>`, earlier defines of
  the same file) is read, in every `#if` branch; other defines are skipped with one `W7101` per
  file; the same name with two values is `E7102`. Files are decoded as Latin-1.
- `load.dir` takes a doublestar glob (dotfiles skipped, zero matches `W7107`) and returns one
  element per file, in path order. With an expected `[T] keyed by f`, keys come from field `f`;
  with `table T`, the key is the file stem. A `table T` read from one file is a JSON object keyed
  by id.
- `load.csv` follows RFC 4180; cells are parsed as Canon literals of the expected field type
  (`E7108`), and an empty cell means the default or `none`. `load.text` is strict UTF-8 with
  `\r\n` normalized.
- Each loaded file is recorded in the build manifest with its hash, alongside the compiler and
  language versions, the layers and `--lang`. An unchanged manifest means an unchanged output, so
  builds can be cached.
- `load` is a migration bridge: when a domain is converted (`canon convert`), its `load` becomes
  Canon source.

Details: [spec/WIRE.md](spec/WIRE.md).

---

## 14. Output: `emit`

### 14.1 Declaration

A package states what it produces, usually in a `build.canon` file:

```
emit go { out: "@web/backend/config", package: "config", mode: baked }
emit ts { out: "@web/src/lib/config.generated.ts" }
emit cpp { out: "@source/Generated/items", namespace: "acme::gen", mode: data }
emit json { out: "@resource/Server/Item/items.json", values: [items] }
emit view { out: "@web/studio/generated/items.view.json" }
```

`emit json { out: "@sovcommon/teamboard/data/" }` (teamboard) is the directory form: one
`<value>.json` per public value.

| Option | Targets | Meaning |
|---|---|---|
| `out` | all | go, cpp: a directory; ts: a file ending in `.ts`; json: a file when it ends in `.json`, else a directory (§14.3); view: a file |
| `values` | go, cpp, ts, json | which public values to emit; default: all public values |
| `mode` | go, cpp, ts | `baked` (default), `embedded`, `data` or `types` (§14.2) |
| `package` | go | Go package name (default: the last element of `out`) |
| `namespace` | cpp | C++ namespace (default: the package path with `.` replaced by `::`) |

- A package has at most one `emit` per target (`E8002`). An unknown option, including `mode` on
  `emit json`, is `E8003`; an invalid value is `E8009`.
- A code target (`go`, `cpp`, `ts`) always emits **every** public type, constant and `export fn`
  of the package, used or not; `values` only selects values. Imported types are referenced, never
  re-emitted (§14.6).
- `emit view` writes the package's whole view model (§16.10).
- Code and data outputs are written only when the build has no error. `emit view` is written even
  when there are errors, so the studio can show them.

### 14.2 Modes

| Mode | Runtime gets | Table ids | Use for |
|---|---|---|---|
| `baked` (default) | values compiled into the binary as constants; no file, no parser | enum | small sets that change with code: enums, taxonomies, event types |
| `embedded` | the data file embedded in the binary plus a generated decoder | enum | medium data in Go (`go:embed`) or TS bundles |
| `data` | a separate data file plus a generated loader that checks the fingerprint | string | large or often-tuned data: items, monsters, drop tables |
| `types` | the read-only types plus a generated in-memory **decoder**; the runtime keeps reading its own file and passes the parsed JSON to the decoder. No file I/O, no fingerprint check (legacy files have no `$schema`) | string | migration, while a hand-written loader still reads the file |

Changing a value in `data` mode rebuilds only the data file: the runtime picks it up on restart or
reload, without recompiling. Because table ids are a string type in `data` and `types` modes
(§5.7), adding or retiring an entry is such a change too. New types or fields always require
recompiling, since code must use them.

### 14.3 Data files

`emit json` writes one file per value. When `out` ends in `.json`, it is that file, and `values`
(explicit or default) must name exactly one value (`E8150`); otherwise `out` is a directory
holding `<valueName>.json` for each value. Two outputs of a build with the same path, or paths
that differ only in letter case, are `E8152`.

```json
{
  "$schema": "game.items.Item@9f3c2a71",
  "rows": [
    {"$id": "II_WEA_AXE_ANGEL", "szName": "IDS_PROPITEM_TXT_003930", "nLevel": 90, "$isStrong": true},
    {"$id": "II_WEA_AXE_OLD", "$retired": true, "szName": "IDS_PROPITEM_TXT_000120", "nLevel": 1, "$isStrong": false}
  ]
}
```

(The fingerprint in samples is illustrative.)

- `$schema` is the fingerprint of the value's type (§14.4). It also marks the file as generated
  (§15.1).
- `rows` holds a list, table or keyed list, in order. Any other value (a record, a map) is under
  `value`.
- A row of a **top-level table** starts with `"$id": "<key>"`, followed by `"$retired": true` when
  the entry is retired (§12). A table nested in a value is an object keyed by id, as in source
  files. A keyed-list row adds nothing: its key is one of its fields.
- Wire names, units and tags follow `@json` (§5.12). Every field is written and every default is
  filled in, so loaders never need to know defaults. `none` is written `null`, or as its
  `@json(none: …)` value. A `Duration` without `@json(unit:)` is integer milliseconds. Input
  fields (§19.2) are omitted; `@deprecated` fields are written.
- Precomputed `export fn` results are extra keys `$<canonName>` (`"$isStrong": true`), whatever
  the wire case. A method with finite inputs is an object keyed by the wire form of its argument.
  Package-level exported functions go under a top-level `"$fns"` key, in the file of the first
  value the package's `emit json` writes.
- A value with no wire form for a field (a duration not exact in its `@json(unit:)`, a value equal
  to its `none` marker, a member twice in a `bits` list) is `E8102`, reported by `canon check`
  like any other finding. A value whose type contains a `Range` or a function type is `E8151`.
- Every value that a `data`-mode code emit reads must be written by the package's `emit json`,
  and each `@reload` value to `<value>.json`, all in one directory (`E8153`).
- Output is canonical: UTF-8, `\n` line endings and a final newline; the top-level keys on their
  own lines with a 2-space indent; each row on one line, indented 4 spaces (`": "` and `", "`, no
  alignment); a `value` or `$fns` pretty-printed like `JSON.stringify(v, null, 2)`; floats written
  as ECMAScript `Number::toString`.

Details, with byte-exact samples: [spec/WIRE.md](spec/WIRE.md).

### 14.4 The schema fingerprint

`$schema` is `<name>@<hash>`. `<hash>` is the first 8 lowercase hex digits of a SHA-256 over a
canonical text serialization (`canon-fp v1`) of the **wire shape** of the emitted type graph:
wire names, field types, optionality and `none` encoding, `@json(path)`, `inline`, units, `int`
and `bits`, enum wire values and codes, variant tags and cases, and the `$` keys with their types,
in order. Types are numbered, not named, so recursive and cross-package types serialize the same
way everywhere; a `ref` contributes only the wire type of its key.

`<name>` is for people reading the file. It is the package-qualified name of the value's type,
after removing one optional and one level of list, keyed list or table (`pipeline.Potion` for
`let potions: [Potion] keyed by id`). When that type has no name of its own (a list of integers, a
map), `<name>` is `<package>.<value>` (`balance.parity.anchors`).

Canon names, doc comments, refinements and defaults are **not** part of the hash. Renaming a Canon
field therefore never changes the fingerprint, just as it never changes a data file (DECISIONS 3).
Values do not affect it either.

Every generated `data`-mode loader has the whole `$schema` string compiled in and refuses a file
whose `$schema` differs, byte for byte, with a message naming both. This catches the one dangerous
deployment: data built from one schema read by a binary built from another, where a changed wire
name or unit would otherwise load silently wrong.

Details and test vectors: [spec/FINGERPRINT.md](spec/FINGERPRINT.md).

### 14.5 No validation at runtime

Generated code contains no validation. A loader checks the fingerprint and copies fields. The
runtime can trust the data because it can only receive data this exact schema has validated.

### 14.6 Outputs of several packages

Generated code for a package uses the generated code of the packages it imports; a type is never
duplicated across outputs.

- If a package's output uses a type of another package, that package must emit the same target
  (`E8004`).
- Go import paths are derived from the imported package's `out` directory and the module path
  that `project.go_module` gives for the closest root containing it (§3.1): with
  `go_module { services: "github.com/sovereign/services" }`, `@services/teamboard` is imported as
  `github.com/sovereign/services/teamboard`. A Go emit under no mapped root is `E8007`.
- C++ includes the imported package's header by its path relative to the including file's
  directory (`#include "../vocab/vocab.gen.h"`).
- TypeScript uses relative imports computed from the `out` paths, with `.js` specifiers.
- A record or variant of package P held inside a type that another package decodes from JSON
  needs P's emit for that target in `data`, `embedded` or `types` mode, which provides decoders
  (`E8018`).

Details: [spec/CODEGEN.md](spec/CODEGEN.md).

---

## 15. Generated code

This section fixes the shape of generated code. Exact names, signatures, file layouts and the
runtime helper files are in [spec/CODEGEN.md](spec/CODEGEN.md), which wins on details.

### 15.1 Common rules

- **Read-only, always.** Every field of a generated type is private and has a getter. There are no
  setters. Only generated loaders and decoders fill values.
- **Files.** Per emit: Go `<out>/<gopkg>.gen.go` and the helper package `<out>/rt/rt.go`; C++
  `<last>.gen.h`, `<last>.gen.cpp` and the runtime headers `canon_runtime.h` (always) and
  `canon_runtime_json.h` (`embedded`, `data` and `types` modes), plus `<last>.defines.gen.h` when
  an enum has `@cpp(defines:)`; TypeScript the `out` file. `<last>` is the last segment of the
  package; conformance files are listed in §15.6.
- **Generated marker.** The first line of every generated file names the package directory it
  comes from: Go `// Code generated by canon from <dir>/. DO NOT EDIT.` (Go's own convention),
  C++ and TypeScript `// GENERATED by canon from <dir>/. DO NOT EDIT.` (`resource/vocab/`), JSON
  its `$schema` key. Runtime helper files have their own fixed markers. At most one more line
  follows the marker: the Go package doc line of the main Go file and of `rt.go`, or the one-line
  header of a conformance file (spec/CODEGEN.md §2.5). `canon build` refuses to
  overwrite a file without a marker (`E8001`), so hand-written code is never clobbered. Taking over
  a hand-written file is always explicit, with `--adopt`: `canon convert --adopt` for a JSON
  source, `canon build --adopt <path>` for the header of a legacy struct in `access: both` mode
  (§15.3).
- **Stable output.** Generated code never contains line numbers, timestamps, absolute paths or the
  compiler version, so a rebuild on another machine does not rewrite files. Go output goes through
  `go/format`.
- **Doc comments** are copied verbatim (§2.2). Go prefixes the first line with the name only if it
  does not already start with it; there is no other rewriting.
- **Names.** Types keep their Canon name. Other names split words at `_`, lower→upper and
  letter↔digit boundaries, then follow each language's convention; Go applies an initialism list
  (`ID`, `URL`, `API`, `HTTP`, `JSON`, `UI`, `DB`, `IP`, `HP`, `MP`, `TS`), C++ does not (Go `ID()`,
  C++ `GetId()`). Table id types are `<Element>ID` in Go (`ItemID`), `<Element>Id` in C++ and
  TypeScript (`ItemId`); a value's container is
  `<UpperCamel(value)>` (`Potions`); variant kinds are `<Variant>Kind`, even when the name repeats;
  the branch enum of a dependent type is `<Alias>Branch`. A name that is reserved in the target
  gets a `_` suffix (Go field `default_`). Two names that collide after conversion, or a getter
  that collides with a generated method (`ID`, `Retired`, `Kind`, `Branch`, `As<Case>`, `Len`,
  `At`, `All`, `Find`, `Get`, `String`, `Wire`), are `E8005`, fixed with `@go(name:)`,
  `@cpp(name:)` or `@ts(name:)`; a `@go(name:)` override also renames the names derived from it
  (spec/CODEGEN.md §3.5). A `@go(name:)` override must be an exported identifier, and a derived
  name that is not an identifier in its target is refused like a bad override (`E8011`). Known
  platform macro names (`ERROR`, `DELETE`, `min`…) are `W8006`.
- **Tables** expose, in every target: the number of entries, iteration in order, lookup by
  position, lookup by key, and lookup by each `@stable` field (`FindByCode`).
- **References.** A ref field has a key getter (Go `XxxID()`, C++ `GetXxxKey()`), and a getter
  that returns the target entry itself (a pointer or reference) when the target is in the same
  value, in the same reload snapshot, or, in `baked` and `embedded` modes, in any public value of
  the same package and emit. Refs into another package, a `local` collection or a collection of
  an enclosing record are keys only. A ref into a define table also exposes the define's integer
  (`XxxValue()`, `GetXxxValue()`), which legacy code needs.
- `export fn` without runtime inputs becomes a getter; with finite inputs, a lookup function;
  with runtime inputs, a translated function plus its conformance test (§9.4).
- **Toolchains.** The current Go release only; C++17 on GCC ≥ 9, Clang ≥ 10 and MSVC 19.20
  (VS 2019), with nlohmann/json ≥ 3.9 in the JSON modes; TypeScript ≥ 5.0, ES modules.

### 15.2 Go

| Canon | Getter returns |
|---|---|
| scalar, `String`, enum | the value |
| `Duration` | `time.Duration` |
| `T?` scalar, `String`, enum | `(T, bool)` |
| `T?` record, variant | `*T` (nil when `none`) |
| `T?` list, map | `(rt.List[T], bool)`, `(rt.Map[K, V], bool)` |
| record, variant | `*T` |
| `ref T` | `XxxID()` always returns the key; `Xxx()` returns `*T` when §15.1 allows it |
| `[T]` | `rt.List[T]` (`rt.List[*R]` for records): `Len()`, `At(i)`, `All() iter.Seq[T]` |
| `[T] keyed by f`, a `table` field | `rt.KeyedList[K, R]` |
| `{K: V}` | `rt.Map[K, V]`: `Len()`, `Get(k) (V, bool)`, `All() iter.Seq2[K, V]` |
| variant | `Kind()`, and `As<Case>() (*<Variant><Case>, bool)` per case with fields |

- Fields are unexported; getters are `Heal()`, not `GetHeal()`.
- Enums are typed integers (the index, or the code with `@codes`) with `String()` (the Canon
  name), `Wire()`, `Parse<Enum>(wire string) (<Enum>, bool)` and `<Enum>Members()` (declaration
  order, retired members included); `@codes` enums add `Code()` and `<Enum>FromCode`.
- `rt` is a small read-only helper package emitted at `<out>/rt/rt.go` (no dependency). Runtime
  errors of generated code are `*rt.EvalError` panics.
- Table and keyed-list containers: `Len()`, `At(i)`, `All() iter.Seq[*T]`,
  `Find(key) (*T, bool)`, `FindBy<Field>(v) (*T, bool)` per `@stable` field, and in `baked` and
  `embedded` modes `Get(id) *T` over the id enum. `FindBy<Field>` reads a map index built once,
  never a scan.
- For each value `v`: `data` mode has `Load<V>(path string) (*<V>, error)` (`LoadPotions`);
  `baked` and `embedded` have `Get<V>()` (`GetStatuses()`), built once on first use through
  `sync.OnceValue`, with no `init()` side effects except reading inputs (§19.2); `types` mode has a
  decoder per public record and variant, `Decode<T>(raw []byte) (*T, error)`.
- Go cannot make a whole object read-only: runtime code can write `pkg.Potion{}`, an empty value
  nobody can fill. So Go has no legacy-struct mode: hand-written config structs are replaced by
  generated types.

### 15.3 C++

Generated C++ targets **C++17** and depends only on the standard library and, in `data`,
`embedded` and `types` modes, nlohmann/json. It never throws and builds with exceptions disabled.

**Getters** are `GetXxx() const` and never copy. Exported functions keep their own name in
UpperCamel, without `Get`: `IsStrong()`, `HealFor(missingHp)`.

| Canon | Getter returns |
|---|---|
| `Bool`, integers, floats, enums | by value |
| `Duration` | `std::chrono::milliseconds` by value |
| `String` | `const std::string&` |
| record, variant | `const T&` |
| `ref T` | `GetXxxKey()` always returns the key; `GetXxx()` returns `const T&` when §15.1 allows it |
| `T?` scalar, enum | `std::optional<T>` by value |
| `T?` string, record, variant | `const T*` (nullptr when `none`) |
| `T?` list, map | `const std::vector<T>*`, `const canon::FlatMap<K, V>*` |
| `[T]` | `const std::vector<T>&` |
| `[T] keyed by f`, a `table` field | `const canon::KeyedList<K, R>&` |
| `{K: V}` | `const canon::FlatMap<K, V>&`: sorted, `Find(key) -> const V*`, iteration in source order |
| variant | `GetKind()`, and `As<Case>() -> const <Variant><Case>*` per case with fields |

**Classes.** Members are private (`heal_`), and there are no setters. Default constructors are
public: a default-constructed value holds zeros and defaults and can never be modified, and
generated classes hold each other by value. Values are built by `detail::<P>Access`, defined only
in `<last>.gen.cpp`.

**Lookups never allocate.** Containers keep a sorted index: `Find(std::string_view key) -> const
T*` is a binary search over `std::string_view`s, and `FindByCode(…)` likewise. `At(i)`, `Len()` and
`All()` (a `const std::vector<T>&`) give positions and iteration; `baked` and `embedded` tables
add `Get(<Element>Id) -> const T&`.

**Values.** Each table or keyed-list value `v` has a container class `<V>` (`Potions`). `baked`
and `embedded` modes emit `const <V>& Get<V>()`, whose data is a function-local `static const`
(no static initialization order problem). `data` mode emits
`static std::shared_ptr<const <V>> <V>::Load(const std::string& path, std::string& error)`; the
shared pointer to const is a whole snapshot, so a runtime can reload by swapping the pointer.
`types` mode emits `static std::optional<T> T::Decode(const nlohmann::json& json, std::string&
error)` for each public record and variant.

**Enums** are `enum class` whose underlying type is `uint8_t`, `uint16_t` or `uint32_t` by member
count, or the `@codes` type. Members keep their Canon names (a reserved word gets a `_` suffix).
`std::string_view ToName(E)` and `std::string_view ToWire(E)` are overloads in the namespace;
`std::optional<E> <E>FromWire(std::string_view)` carries the enum's name, since C++ cannot overload
on the return type; `k<E>Members` lists the members, and `@codes` enums add `<E>FromCode`.
`@cpp(defines: "IK1_")` also emits `#define IK1_WEAPON 1` lines for legacy code, in their own
header `<last>.defines.gen.h`: the define is the member name if it already starts with the prefix,
else prefix + member; its value is the code with `@codes`, else the index. Generated code guards
the enumerators it names against those macros (`#pragma push_macro`).

**No exceptions.** Where generated code must fail at runtime (a checked helper in a translated
function, §9.4; an `embedded` file that does not decode, `E8301`; an input read before
`LoadInputs`, `E8302`), it calls `canon::OnEvalError(code, message)`, a replaceable handler whose
default logs and calls `std::abort()` (DECISIONS 19).

`canon::FlatMap`, `canon::KeyedList`, `canon::OnEvalError` and the other helpers live in the
generated header `canon_runtime.h`; the JSON helpers of loaders and decoders live in a second
header, `canon_runtime_json.h`, so `baked` mode never needs nlohmann/json. Both are written once
per output directory inside `namespace canon { inline namespace rt_v1 { … } }`, with version
guards instead of `#pragma once`, so two output directories linked into one binary cannot
violate the one-definition rule. Bit-exact conformance of floats requires building with
`-ffp-contract=off` (GCC, Clang) or `/fp:precise` (MSVC).

#### Legacy structs

An existing hand-written struct (in the examples, `ItemProp`) can be targeted instead of a
generated class. Each record chooses how runtime code reaches its fields, with a toggle
(DECISIONS 5):

```
record Item @cpp(struct: "ItemProp", header: "ItemProp.h", access: fields) {
  kind3: ItemKind3 @json("dwItemKind3") @cpp(field: "dwItemKind3")
  …
}
```

| `access:` | The struct | Existing reads `p->dwItemKind3` | Getters `GetKind3()` |
|---|---|---|---|
| `fields` | stays hand-written; Canon generates `class ItemPropTable`, which owns a `std::vector<ItemProp>` filled by generated code, plus a `static_assert(std::is_same_v<decltype(ItemProp::m), T>)` per mapped member | compile | none |
| `both` | **generated** by Canon into `@cpp(header:)`, with the legacy public member names and types **and** getters; the first build needs `canon build --adopt <header>` | compile | available, returning the member's own C++ type |
| `getters` | generated; members are private | fail to compile: the compiler lists what remains | the only way |

- In every mode, lookups return `const` pointers or references, so runtime code cannot modify
  config. A write is a compile error.
- The intended path, struct by struct: `fields` (Canon fills the existing struct) → `both` (new
  code uses getters, old reads keep working) → `getters` (the last reads are migrated, guided by
  the compiler).
- Members whose type differs from the Canon type use `@cpp(type: "DWORD")`; a fixed array such as
  `@cpp(type: "char[64]")` gets a build-time length check (`E8103`), and a value outside a
  member's range is `E8106`. Unmapped members are value-initialized. A ref into a define table
  stores the define's value. A `Duration` member holds milliseconds unless `@cpp(unit: s)` says
  otherwise. The fields of an inline variant map to members named by `@cpp(field:)` on each case
  field, and the member that holds the case gets each case's `@cpp(value: N)` (`E8108` when
  missing). Getter names can be overridden with `@cpp(name: "GetID")`.

Worked `ItemProp` example: [spec/CODEGEN.md](spec/CODEGEN.md) §7.8.

### 15.4 TypeScript

- Each `emit ts` writes one `.ts` file (an ES module) that uses only erasable syntax, so it also
  runs under Node's type stripping.
- Records are `interface`s with `readonly` properties; lists are `ReadonlyArray`; maps are
  `CanonMap` values whose mutators throw (`Object.freeze` does not freeze a `Map`). Every other
  emitted value is deeply frozen, so writes fail at compile time and at runtime. A `readonly`
  property is TypeScript's getter.
- Enums are unions of their **wire** strings, plus frozen `<E>Members`, `<E>Names` and `<E>Index`
  (and `<E>Codes` with `@codes`). Variants are `{ readonly kind: "<case wire>", … }`; dependent
  unions discriminate on `branch`. Refs are key strings, resolved with the container's
  `find(key)`. An optional is `T | null`.
- Tables and keyed lists are `CanonTable` values (`statuses.find(key)`, `at(i)`, `length`, `all`);
  in `baked` and `embedded` modes their key types are literal unions.
- `Int` is `number`. A value outside `Number.MAX_SAFE_INTEGER` in an emitted value is `E8101`,
  unless the field is annotated `@ts(bigint)` (the only way to get a `bigint`, also for `UInt64`).
  Translated functions check integer inputs with `Number.isSafeInteger` and throw
  `CanonEvalError` (`E8303`) otherwise.
- `data` mode exports `decode<V>(json: unknown)` per value, and `types` mode `decode<T>` per public
  record and variant; neither does I/O: the page or process fetches or reads the file itself.
- Precomputed `export fn` results are properties; translated ones are functions taking the value as
  their first argument.

### 15.5 Hot reload

A value may change while a runtime is running only if its `.canon` file says so. Reloadability is
a design property of the data, declared next to it, not a runtime option (DECISIONS 16):

```
/// Tuned live during balance sessions: runtimes keep ItemIds, never Potion pointers.
@reload
let potions: [Potion] keyed by id = load.dir("@resource/Server/Item/Potions/*.json")
```

- **Default: not reloadable.** A value without `@reload` is loaded once at startup with its plain
  `Load`. Its generated code has no reload entry point, so nobody can swap it by accident.
- **What `@reload` promises.** Runtime code never keeps a pointer or reference obtained from the
  value beyond one piece of work (a tick, a request, a command); long-lived state keeps the **id**
  (`ItemId`), or the snapshot itself. The doc comment should say why that holds.
- **What Canon enforces.** `@reload` on a value whose type (or any type it contains) maps onto a
  legacy C++ struct in `fields` or `both` mode is an error (`E8201`): legacy code keeps raw
  pointers to such structs (members, index arrays), which a swap would leave dangling. It becomes
  allowed once the record is in `getters` mode, when its access is generated code. `@reload` in
  `baked`, `embedded` or `types` mode is `E8202`: there is no file to reload. A `@reload` value
  must be written by the package's `emit json` as `<value>.json` (`E8153`).
- **One package, one snapshot.** All `@reload` values of a package's Go or C++ emit are loaded
  together into one immutable `<Package>Snapshot` (`PipelineSnapshot`), with one getter per value,
  and swapped together. References between them are resolved inside the snapshot, so they always
  agree. References to values outside the snapshot are held by key, never by pointer (§15.1).
  `@reload` values have no per-value loader.
- **One store per snapshot.** `<Package>Store` has `Current()` (an atomic load) and `Reload(dir)`,
  which reads `<dir>/<value>.json` for every `@reload` value (§14.3) (C++
  `static bool Reload(const std::string& dir, std::string& error)`; Go
  `func (self *PipelineStore) Reload(dir string) error`, with a package variable `Store`).
  `Reload` builds the new snapshot beside the old one, then swaps it atomically (C++17
  `std::atomic_load`/`std::atomic_store` on the `shared_ptr`, or `std::atomic<std::shared_ptr>`
  where available; Go `atomic.Pointer`). Readers that still hold the old snapshot keep using it;
  it is freed when the last one lets go. If loading fails (unreadable file, fingerprint mismatch),
  the old snapshot stays and the error is returned. The first load is also a `Reload`:
  `Current()` returns null before it. TypeScript has no store in v0.
- **Only values reload.** A schema change changes the fingerprint (§14.4), so the new file is
  refused until the runtime is rebuilt.
- **Live runtime state keeps the values it copied** (a running effect keeps its duration). Stable
  ids guarantee that a reload never removes an entry that live state refers to.

### 15.6 Conformance tests

For every translated `export fn` of a package, `canon build` writes a test per target, next to
the generated code: Go `<gopkg>_conformance_test.go` (one `Test<T><Fn>Conformance` per function),
C++ `<last>_conformance.gen.cpp` (an entry point `int <namespace>::conformance::Run<P>Conformance()`
returning the number of failures, where `<P>` is the package's last segment in UpperCamel) and
TypeScript `<last>.conformance.test.ts` beside the emitted file (using `node:test`). They are
meant to run in each runtime's test suite, and `canon build --check` fails when one is stale.
Details: [spec/CONFORMANCE.md](spec/CONFORMANCE.md).

---

## 16. Views

Views describe **what data means to an editor**, not where pixels go. They live next to the types,
in the same language, so a view that names a missing field is a compile error. Views never affect
validation, generated runtime code or data: deleting every view changes only how the studio looks.

This section states the vocabulary and the rules a view author relies on. The view model, and
every studio behaviour it drives (controls, relevance, pickers, groups, filters, findings), are
specified in [spec/VIEWMODEL.md](spec/VIEWMODEL.md), which wins on details.

### 16.1 What goes where

The rule: **`.canon` states facts about the data and editorial intent; the studio owns
rendering.** To decide, ask: *if the studio were rebuilt with another UI toolkit, would this still
be true?* If yes, it belongs in `.canon`.

| In `.canon` | In the studio |
|---|---|
| types, ranges, defaults, units, references, assets: what a value **is** | the design system: components, colours, spacing, typography, themes |
| doc comments: what a value **means** | icon artwork, image decoding (DDS → thumbnails), caching |
| labels, groups, order, `advanced`: how an editor **thinks** about the data | layout: tabs or sections, panels, responsive behaviour |
| how an entry is **named and recognised**: `title`, `subtitle`, `preview`, `search` | interaction: undo/redo, shortcuts, drag and drop, autosave, drafts |
| which fields are worth **filtering** a large collection by | bulk-edit mechanics, table virtualization for large collections |
| **control hints** from a closed vocabulary (§16.3) | how each control looks and behaves |
| **presentation of enum members**: label, icon, tone (§16.4) | history and diffs (from git), review workflow |
| read-only **computed lines** shown next to the data (§16.6) | per-user preferences: language, column widths, collapsed groups |
| **translations** of all of the above (§17) | |
| the **catalog** of custom widgets: names, accepted types, and the default editor of a type (§16.11) | the widgets' implementation |

### 16.2 Choosing a control

With no hint, the control follows from the type, with fixed thresholds, so the result is
predictable. The thresholds count **active** members, cases or entries. The **compiler** resolves
the control and writes it into the view model; the studio never re-derives it. A field's `widget`
wins, then its `control`, then a default widget declared for its type (§16.11), then this table.

| Data | Control |
|---|---|
| `Bool` | switch in a form; checkbox in a table cell |
| `Bool?` | segmented: Yes / No / Unset |
| enum or variant case, up to 4 choices | segmented buttons: every choice visible, one click |
| enum or variant case, 5 to 10 choices | select |
| enum or variant case, more than 10 | searchable select |
| `[Enum]` declared a set (`where it.isUnique()`, or `@json(bits)`), up to 6 choices | checkbox group |
| `[Enum]` otherwise | chips with a searchable picker |
| `ref T`, target with up to 10 entries | select, each option with its `title` and `preview` |
| `ref T`, larger target | searchable picker (§16.8) |
| `ref T` into a collection of the record being edited | select over that collection's current keys |
| `[ref T]` | chips with a searchable picker |
| `String` | single-line input |
| `String(/re/)` | single-line input, pattern checked as you type |
| `Int`, `Float` | number input with bounds and unit suffix; stepper buttons only on an integer whose range has both bounds and at most 20 values |
| `Duration` | number plus a unit selector (ms, s, m, h, d), shown in its largest exact unit |
| asset (§16.5) | file picker with thumbnail |
| record with up to 6 fields | inline section |
| larger record | collapsible card |
| `[scalar] where it.len() == 2 and it[0] <= it[1]` | a min–max range |
| `[scalar]` with an upper length bound of 6 or less | that many inputs in a row, labelled by position |
| other `[scalar]` | tag input |
| `[Record]`, `table`, keyed list | table with inline editing of scalar columns, plus a detail panel |
| other lists | one control per element, numbered |
| `{Enum: scalar}` | one input per member, in a row (`Stage_1`, `Stage_2`, `Stage_3`) |
| `{K: Record}` (dependent maps included) | one card per entry, titled by the value's `title` or its key; "Add" asks for a key first (for an enum key, the members not yet present) |
| other maps | key/value table; the key and the value use the rules above |
| `T \| "lit"` | the control of `T`, plus one pinned option per literal |
| `T?` | the control of `T`, a clear button, and the default as placeholder; as segmented buttons, a final "Unset" segment instead of the clear button |
| a field that admits a single value | read-only |
| dependent field | the control of the type computed from the value it depends on |
| `@deprecated` field | read-only, in a collapsed "Unused fields" group |
| check finding | shown on the value it reports at; a field with a finding is always shown (§16.12) |

### 16.3 Control hints

A view may ask for another built-in control with `control:`. The compiler checks that the control
accepts the field's type (`E1601`), comparing types after stripping refinements, `keyed by` and
`where`; a `T?` field matches a control or widget for `T`. An unknown control is `E1609`, and
`control` together with `widget` on one field is `E1634`.

| `control:` | Accepts |
|---|---|
| `switch`, `checkbox` | `Bool` |
| `segmented`, `radio`, `select`, `search` | enum, `ref`, variant (its case selector) |
| `checkboxes`, `chips` | `[Enum]`, `[ref T]` |
| `input`, `textarea`, `code` | `String` |
| `number`, `stepper`, `slider` | integer types, `Float`, `Duration` (`slider` needs both bounds, `E1619`) |
| `color` | `String(/^#[0-9a-fA-F]{6}$/)`, `UInt32` |
| `text` | anything: shown read-only |

```
view Global {
  visitCost { control: slider } // Int(0..=1_000_000): slider plus number
  description { control: textarea }
  flags { control: checkboxes }
}
```

A control a type needs that is not in this list is a **widget** (§16.11), not a new hint.

### 16.4 Enum members and table entries

An enum's members can carry a label, a help text, an icon and a tone, in a view of the enum:

```
view Element {
  FIRE "Fire" { icon: flame, tone: danger }
  WATER "Water" { icon: droplet, tone: info }
  ELECTRICITY "Electricity" { icon: zap, tone: warning }
  WIND "Wind" { icon: wind, tone: series_2 }
  EARTH "Earth" { icon: mountain, tone: series_3 }
}
```

- Every control that shows the enum (segmented buttons, selects, chips, table cells) uses them.
  A member without a view shows its Canon name.
- `icon` is a member of the studio's UI icon set, and `tone` a member of its tones (§16.11). These
  are UI symbols, not data.
- Labels and help are translatable (`Element.FIRE`, `Element.FIRE.help`).
- A view of a variant (`view Reward { … }`) presents its cases the same way: label, `help`,
  `icon`, `tone`.
- A table's entries are presented by the view of the entry type (`title`, `subtitle`, `preview`),
  computed from each entry's own data.

### 16.5 Assets: images and files

A value that names a file (an icon, a model, a sound) has an **asset type**:

```
type ItemIcon = asset("@resource/Icon/Item", ext: [dds, png])

record Item {
  icon: ItemIcon @json("szIcon") // "Itm_WeaAxeAngel.dds"
}
```

- The wire form is the file name, or a `/`-separated path, relative to the asset root; `..` is not
  allowed (`E3703`). `ext` lists extensions as symbols (`E3702` for another extension).
- **The file must exist**, with exactly that name: matching is exact and case-sensitive, byte for
  byte. A missing file, or one that differs only in letter case, is an error at build time
  (`E3701`) (DECISIONS 19). Legacy case drift in data is fixed once by a script, not by the
  language.
- The studio shows a thumbnail and edits the value with a file picker limited to that root and
  those extensions. Decoding formats such as DDS is the studio's job.
- `preview <expr>` in a view names the asset that represents an entry (§16.6), for example
  `preview icon`.

### 16.6 View vocabulary

The samples of §16.6 and §16.8 use these self-contained types (an illustration, not an example
file):

```
local let jobs = load.defines("@resource/Server/Define/defineJob.h", prefix: "JOB_")
local let names: {String: String} = load("@resource/Server/Text/names.json")

variant ItemKind @json(tag: "type") {
  weapon { attackMin: Int(0..), attackMax: Int(0..), attackSpeed: Float(0.0..) }
  armor { defense: Int(0..) }
  material
}

record Item {
  define: String // "II_GEN_MAT_MOONSTONE": the key
  nameKey: String
  icon: ItemIcon // §16.5
  level: Int(1..=150)
  job: ref jobs? = none
  kind: ItemKind @json(inline)

  fn name(self) -> String { return names.get(nameKey) ?? nameKey }
}

let items: [Item] keyed by define = load.dir("@resource/Server/Item/Items/**/*.json")
```

```
view Item {
  title "{name()}"
  subtitle "{define}"
  preview icon
  singular "item"
  plural "items"
  search { name(), define }
  filters { kind multi, job, level }
  columns { icon 48, nameKey 220, kind 120, level 70 }

  group identity "Identity" { nameKey, icon, level, job }
}

/// A view of one case: shown when the item is a weapon.
view ItemKind.weapon {
  group combat "Combat" { attackMin, attackMax, attackSpeed }
  show avg "Average hit" "{(attackMin + attackMax) / 2}"
}
```

| Item | Meaning |
|---|---|
| `title "<template>"` | how an entry is named in lists, pickers, headers and card titles |
| `subtitle "<template>"` | second line in lists and pickers |
| `preview <expr>` | the asset shown next to the entry in lists and pickers (`E1612` if not an asset) |
| `singular "<text>"` | used in "Add …" actions |
| `plural "<text>"` | the noun after the count of a collection of this type ("9 levels") |
| `menu <Menu> icon <Icon>` | where the public values of this type, and the public collections of it, appear in the studio navigation |
| `columns { field width?, … }` | columns when a collection of this type is a table (widths are defaults, 16 to 2000 pixels); fields only (a method is `E1606`, use `show`) |
| `search { expr, … }` | what a picker matches (§16.8) |
| `filters { field multi?, … }` | filter controls above a large collection of this type (§16.9); `multi` makes a choice filter multi-select |
| `group id "Label" "intro"? advanced? (when expr)? { … }` | ordered sections; `id` is the translation key; `advanced` groups start collapsed |
| `show id? "Label" "<template>"` | a read-only computed line, at top level or inside a group; the optional `id` names it for translations |
| `<field> "Label"? { props }` | per-field presentation, at top level or inside a group |
| `field <name> "Label"? { props }` | the same, for a field whose name is a view word (`field title "Title"`) |

Field props: `help` (overrides the doc comment), `unit` (§16.11), `control` (§16.3), `widget`
(§16.11), `readonly`, `hidden`, `placeholder`, `when` (a condition), `none` (the label of `none`
on an optional field) and `step` (the name of each element of a list, over `{index}` only:
`step: "Winner {index}"`). Their exact rules are
[spec/VIEWMODEL.md](spec/VIEWMODEL.md) §3.5.

- **Scope of templates and conditions**: the magic names `id` (a table entry's key, or a define's
  name), `key` (a map value's key) and `index` (a list element's position, from 1), then the fields
  and methods of the type, then the package scope. A field named `key` or `index` hides the magic
  name (`W1604`); a table element cannot declare `id`.
- A view may name a method without parameters; it is shown like a `show` line (a method with
  parameters is `E1616`).
- **Variants.** Prefer a variant to a `when` whenever the condition describes what the data *is*
  (`kind is weapon`): `view Variant.case { … }` lays out one case, and a view of the record that
  holds an inline variant may name fields that exist in only some cases, the group hiding when the
  current case lacks them. `view Variant { … }` labels the cases (§16.4).
- `when` shows a field or group only while the condition holds. It is **display only**: it never
  changes validation or data.
- `check … at <field>` (§10.1) pins a record's finding to one field.
- Groups: named groups come first, in view order. A name placed twice in one view is `E1605`;
  `hidden` wins over group membership. Fields not named in any group go to a final group labelled
  "Other" (id `_other`, studio chrome, not a translation key), ordered by usage (§16.7). Ids
  starting with `_` are reserved (`E1618`).
- Every name and value in a view is checked: fields must exist (`E1602`), `unit` must be a studio
  unit, `control` and `widget` must accept the field's type, `preview` must be an asset.
- `menu` applies to every public value of the type's package whose type is that type, or a list,
  keyed list or table of it; a value can override it with `@menu(…)`, which wins.
- A view is declared in its target's own package (`E1603`), and a target has at most one view
  (`E1607`, §23); `view V` and `view V.c` are different targets. A view may also be declared on a
  `let` that comes from `load.defines` (`view jobs { title "…" }`), to name its entries in pickers.

### 16.7 Relevance

A record with many fields is rarely edited as a whole: in the example project, an item type has
97 possible fields and a material uses about 24. Two layers decide what the studio shows:

1. **Types say what can exist.** Real kinds are a variant, so fields that don't apply to a kind are
   not part of it. The studio never shows them and Canon refuses them.
2. **Usage says what usually matters.** `emit view` records, per type and per **shape** (the
   current case of each inline variant field), how many values **set** each field, a field being
   set when it is present in the source (a literal field or a JSON key), not merely different from
   its default. Only values reachable from the package's public values count. A field is shown if
   at least 25 % of the shape's values set it (MOCKUP-GAPS 8), or if the value being edited sets
   it or carries a finding; the rest go under "More", in usage order. Explicit `group`s override
   this order.

For legacy data, the first version of such a variant is written from the domain's schema, loader
and data ([CLI.md](CLI.md) §6.4): each kind allows the fields its entries set. Tightening is then a
series of small reviewed changes.

### 16.8 Pickers and search

A `ref` field is edited with a picker. The picker **stores the key** (`II_GEN_MAT_MOONSTONE`) and
**shows and searches** what the target type's view declares:

```
view Item {
  title "{name()}" // "Moonstone"
  subtitle "{define}" // "II_GEN_MAT_MOONSTONE"
  search { name(), define } // "moon" and "MOONST" both find it
  preview icon // its thumbnail next to each result
}
```

`emit view` builds the search index at build time: one row per entry with its key, title,
subtitle, search terms and preview. It is written only in the view model of the collection's own
package, and other packages reference it. The studio searches it locally (about 1 MB for 7,000
entries). Retired entries are never offered. A picker shows the first 60 matches, ranked by title
prefix, word prefix, title substring, then key or search term. Two entries with the same title are
shown as `<title> (<key>)`. Details: [spec/VIEWMODEL.md](spec/VIEWMODEL.md) §8.

### 16.9 Large collections

A collection with many entries (items, monsters) is a table the studio virtualizes.

- `filters { … }` lists the fields offered as filters above it. Each filter follows the type: an
  enum, `ref` or variant-case filter is a single choice plus "Any" (multi-select with `multi`), a
  `Bool` filter Any / Yes / No, a number or duration filter a min–max range bounded by the
  observed values, a list of enums or refs a "contains" choice, and optional fields get a "(none)"
  choice. Filtering on another type is `E1620`, and `multi` on a filter that is not a choice
  `E1621`.
- A text search above the table reuses the type's `search { }`.
- Scalar columns are editable inline. Selecting several rows and setting one column on all of
  them is a studio feature; each change goes through the edit API as ordinary operations, so every
  check still runs.
- Sorting by any column is always available.

### 16.10 View model

`emit view` writes one JSON document per package, even when the build has errors. Its format is
versioned (`"$schema": "canon-vm/1"`), specified in [spec/VIEWMODEL.md](spec/VIEWMODEL.md) §12 and
[spec/viewmodel.schema.json](spec/viewmodel.schema.json).

| Key | Content |
|---|---|
| `$schema` | the view model format version, `canon-vm/1` |
| `package`, `language` | the package's qualified name and the project's language version |
| `requires` | the packages whose view models this one refers to |
| `types` | every type of the package, by qualified name (`resource.farm.Global`): fields (name, type, refinements, default, doc, wire name), cases, members; types of other packages are referenced, not copied |
| `views` | every view, resolved (sections, labels, controls, conditions, computed lines) |
| `values` | every public value: type, source files, menu, whether and how it is editable |
| `usage` | per type and shape: how many values set each field, split into shown and "More" |
| `search` | per collection of this package: the picker index |
| `assets`, `units`, `widgets` | the asset roots, units and widgets the package's views use, and the default widgets that apply |
| `studio` | the studio package's catalogue (menus, icons, tones, units, widgets), only in that package's view model |
| `i18n` | every text, per language |
| `findings` | findings of the last build |

The studio is a generic renderer of this model, its built-in controls and the widget catalog.
Conditions (`when`), computed lines (`show`), titles and draft findings are evaluated by the
compiler through the embedding API's `Evaluate` call ([spec/API.md](spec/API.md)), so the studio
never reimplements Canon expressions. Patterns are marked as RE2.

### 16.11 Studio vocabulary and widgets

The package named by `project.studio` declares what views may use, with ordinary Canon. This is
`examples/studio/studio.canon`, in full:

```
/// The studio's presentation vocabulary (SPEC §16.11).
///
/// Resource Studio is a generic renderer. This package is its contract with every view:
/// navigation sections, UI icons and tones, units, and the custom widgets it implements.
/// A view can only use what is declared here, and the compiler checks each use.
///
/// Built-in controls (switch, segmented, select, search, checkboxes, slider...) are not
/// declared: they are part of the language (SPEC §16.2, §16.3). Adding a widget is a
/// two-part change: declare it here, and register a Svelte component under the same name.
package studio

import sovcommon.time { TimeOfDay, Window }

/// Top-level sections of the studio navigation, in display order. A value whose view
/// names no menu is listed under "No menu".
enum Menu { events, instances, progression, economy, items, gear, monsters, guild, world, system }

/// UI symbols, not game data: navigation, enum members, menus.
enum Icon {
  flame
  droplet
  zap
  wind
  mountain
  farm
  event
  tower
  chest
  map
  sword
  shield
  gem
  gear
}

/// UI tones, mapped to the design system's colour tokens.
enum Tone { neutral, info, success, warning, danger, series_1, series_2, series_3 }

/// How numbers are shown. `scale` converts the stored value into the shown one.
record UnitSpec {
  /// Appended to the number. Translatable, like every other studio text.
  suffix: String = ""
  /// Shown value = stored value × scale.
  scale: Float = 1.0
  /// Group digits by thousands.
  thousands: Bool = true
  /// Digits after the decimal separator.
  decimals: Int(0..=6) = 0
}

/// The units a view may name with `unit:`.
let units: table UnitSpec = {
  penya { suffix: " penya" }
  hp { suffix: " HP" }
  pct { suffix: " %" }
  /// Probabilities stored out of 1 000 000 000 (random options, drop rates).
  ppb { suffix: " %", scale: 0.0000001, decimals: 4 }
  weight {}
  count {}
  seconds { suffix: " s" }
}

/// A week-long timeline; windows are bars, dragged to move or resize. Wrapping windows
/// are drawn across midnight and across the week boundary.
widget weekly_timeline(value: [Window])

/// An ordered list where position has meaning (level 1, 2, 3...): numbered steps,
/// drag to reorder, with the position shown as the step's name (`step:` renames it).
widget ordered_steps(value: [_])

/// A weight shown with its share of the list it belongs to ("225 000 = 22.5 %"). The
/// studio passes the same field of every element of the enclosing list as `siblings`.
widget weight_share(value: Int, siblings: [Int])

/// Grid editor for [magnitude, cumulative probability] pairs, showing the implied
/// per-step probability and the remainder to 1 000 000 000.
widget probability_ladder(value: [[Int]])

/// One "HH:MM" input for a time of day, instead of two bare numbers. The default editor
/// of every TimeOfDay field, in every package; a view can still choose another control.
widget time_of_day(value: TimeOfDay) default

// Every package's view model refers to this vocabulary (menus, icons, tones, units,
// widgets); the studio reads it once from here.
emit view { out: "@generated/studio.view.json" }
```

- The view props `unit`, `icon`, `tone`, `menu` and `widget` resolve against the studio package
  implicitly; importing it is unnecessary but harmless. `unit: ppb` is a reference to `units`: an
  unknown unit, widget, menu, icon or tone is `E1610`.
- `unit` applies to integer and `Float` fields, and to each element or value of a list or map of
  these; shown = stored × `scale`. It does not apply to `Duration`, which has its own unit
  selector (`E1611`).
- A **widget** is a custom component for a type that needs its own editor (a timeline, a
  probability ladder). It is implemented once in the studio under the same name; a studio that
  lacks it renders the control the field would have had without it. A view opts in with
  `field { widget: name }`, and the compiler checks the field's type against the `value`
  parameter (`_` matches any type, `E1608` otherwise). A widget that needs the same field of the
  other elements of the enclosing list declares a second parameter, `siblings: [T]` (`E1631` for
  another type).
- **Default widgets** (DECISIONS 22). `widget … default` makes the widget the editor of every
  field whose type matches `value`, in every package, unless the field's view gives it a
  `control` or `widget`. The `value` type must be a named record, variant or enum, optionally in a
  list (`E1630`), and a type has at most one default widget (`E1629`). So every `TimeOfDay` field
  gets the `time_of_day` input without any view naming it.
- The line between a control hint and a widget: hints are the studio's built-in controls, usable on
  any compatible type; a widget exists for a specific data shape. The widget catalog grows only
  when a type truly needs one.

### 16.12 Studio behaviours

The studio mockup (`meta/spec-phase/mockups/studio.html`) settled the following behaviours (MOCKUP-GAPS.md,
accepted by DECISIONS 20; asset letter case follows DECISIONS 19, parallel legacy fields
DECISIONS 21, default widgets DECISIONS 22). Their normative text is in
[spec/VIEWMODEL.md](spec/VIEWMODEL.md):

- **Controls:** shared thresholds for enums and variant cases; the "Unset" segment; sets of enums;
  short positional lists and min–max ranges; time-of-day values through the default
  `time_of_day` widget; stepper buttons; the legacy wire forms `@json(int)` and `@json(bits)`
  (§5.12).
- **Relevance and grouping:** where "More" starts (25 %); order of sections (named groups, `_other`,
  "More", "Unused fields"); nested kinds (a variant inside a case); optional fields with a
  non-`none` default (default, set, explicit `none`); the label of `none`.
- **Variants and dependent fields:** changing a case (fields of the same name and type kept, the
  rest listed and dropped, with Undo); views naming case-only fields and `view Variant.case`; the
  read-only case column; a dependent field whose driver changes; fixed keys of dependent maps;
  editing `T | "lit"`.
- **Pickers and titles:** views on define tables; picker ranking tiers and the 60-result limit;
  retired entries; refs inside title templates; keys of keyed collections and "Rename…" through
  `refs`; methods in views; collection-valued columns and `plural`; duplicate titles; naming the
  steps of lists with `step`.
- **Widgets, units, findings:** widgets that read sibling values; cards for every map of records;
  `unit` on lists and maps; record-level findings as a banner, and pinning a check to a field with
  `at`; package-level checks over collections; findings on hidden fields; asset letter case
  (exact).
- **Structure and text:** flattening a group that holds one record field; the final "Other" group;
  `advanced` groups collapsed and remembered per user; `show` inside groups and "—" on failure;
  default labels humanized from camelCase; single-value fields read-only; durations in their
  largest exact unit; the "No menu" section; filters, including multi-select; text search; English
  fallback markers; translated unit suffixes and number separators; when `when` and `show` are
  re-evaluated (after each committed edit); editing while a layer applies (edits go to base
  sources; a value set by an active layer is read-only unless `EditLayer` names that layer);
  parallel legacy fields, which are lists of records through `@json(pairs:)` and use the ordinary
  list controls.

---

## 17. Translations

Source texts are written in the project's first language (English): doc comments, view labels,
check messages. Other languages live in translation files, keyed by **path**:

```
package resource.farm
translation fr

Global.farmPurchasePrice "Prix d'achat de la ferme"
Global.farmPurchasePrice.help "Prix payé une fois pour obtenir une ferme."
ModelType.group.unlock "Déblocage"
ModelType.check.unreachable_levels "maxLevel vaut {maxLevel} mais seuls {levels.len()} paliers existent"
```

| Key | Text |
|---|---|
| `Type.help` | a type's help (its doc comment) |
| `Type.field` | a field's label |
| `Type.field.help`, `.deprecated`, `.placeholder`, `.none`, `.step` | its help (the `help` prop, else the doc comment), `@deprecated` reason, placeholder, label of `none`, step name |
| `Type.method`, `Type.method.help` | the label and help of a method a view names |
| `Variant.case`, `Variant.case.help`, `Variant.case.field…` | a variant case's label and help, and its fields as above; the items of `view Variant.case` follow the same pattern under `Variant.case.` |
| `Enum.member`, `Enum.member.help` | an enum member's label and help |
| `Type.title`, `Type.subtitle`, `Type.singular`, `Type.plural` | view texts |
| `Type.group.<id>`, `Type.group.<id>.intro` | group texts (the final `_other` group is studio chrome, not a key) |
| `Type.show.<id>`, `Type.show.<id>.text` | a `show` line's label and template; an unnamed line's id is `_0`, `_1`… by position among the view's unnamed lines |
| `Type.check.<name>`, `check.<name>` | a named check's message, in a type or at package level |
| `value`, `value.help` | a public value's label (its `@menu(label:)`, else its humanized name) and doc |
| `Menu.member`, `units.<unit>.suffix` | in the studio package only: menu labels and unit suffixes |

- A field, method, case or member whose name is one of the key words (`help`, `title`, `group`,
  `check`, `show`, `field`, …) is written with its kind word in front: `Quest.field.title`,
  `Icon.member.check.help` (`field.`, `method.`, `case.`, `member.`). The reserved words and the
  full catalogue are [spec/I18N.md](spec/I18N.md) §3.
- A text is translatable, and gets a key, only if it contains a letter outside its
  interpolations: `"{name()}"` and `" %"` are not keys. Labels always exist: a label humanized from
  a field name, or an enum member shown by its Canon name, is a key like any other.
- A text that is a template in the source may be a template in a translation; it is type-checked
  in the source text's scope (`E1703`). A translation of a plain text cannot interpolate
  (`E1707`).
- A missing key is a warning, reported once per package and language with a count (`W1701`), and
  only for packages that emit a view. An empty translation (`""`) counts as missing. `canon i18n
  status` lists the keys, and `canon i18n stub` adds them. The studio falls back to the source
  language and marks the fallback.
- A key that matches nothing is an error (`E1702`), so renaming a field shows up in its
  translations immediately. Keys refer to the file's own package; a language missing from
  `project.languages` is `E1704`, the source language `E1706`; a duplicate key is `E1705`.
- These are **tool texts**. Application texts (item names, quest texts) are data and stay in the
  application's own text tables; `search` expressions can read them.

The full key catalogue, fallbacks, and the `canon i18n stub` and `status` outputs are in
[spec/I18N.md](spec/I18N.md).

---

## 18. Tests

```
test "healFor never overheals" {
  let p: Potion = { id: "II_POT_T", name: "IDS_T", heal: 500, cooldown: 1s }
  expect p.healFor(200) == 200
  expect p.healFor(9000) == 500
}

test "overlapping windows are refused" {
  expect { ...sample, schedule: [at(Mon, 20, 22), at(Mon, 21, 23)] } fails "overlaps"
  expect { ...sample, schedule: [at(Mon, 20, 22), at(Mon, 22, 23)] } passes
}
```

| Form | Passes when |
|---|---|
| `expect a == b` (any comparison) | the comparison holds |
| `expect v passes` | building `v` gives no error |
| `expect v fails "text"` / `fails name` / `fails E3204` | building `v` gives an error whose message contains `text`, whose `check` field is `name`, or whose code is `E3204` |
| `expect v warns "text"` / `warns name` / `warns W5001` | same, for a warning |

- `fails` and `warns` match the **evaluation** findings (dynamic `E3xxx`, `E4xxx`, `E5xxx` and
  `W5xxx`) of building `v` and the values reachable from it; record checks run on them (§10.1). A
  static type error in a test is a compile error, never an expected failure.
- After `fails` or `warns`, an identifier of the form `E1234` or `W1234` is a code; any other
  identifier names a check that applies to `v` (`E5004` otherwise).
- A failing `expect` does not stop the test; later statements still run.
- Tests run with `canon test` (`--run` selects them by an RE2 search on their name). They may use
  `local` helpers and values. They are never emitted, except as input vectors of conformance tests
  (§9.4). Details: [spec/EVALUATION.md](spec/EVALUATION.md) §10.

---

## 19. Layers and runtime inputs

### 19.1 Layers

A layer changes values, never types. It is how one law gets its per-environment or per-machine
variations.

```
package service.editor
layer alice

amend config {
  paths.dataRoot: "/home/alice/work/data"
  server.port: 9000
}
```

- `canon build --layer alice` applies every `layer alice` file of the loaded packages. Layers
  stack in the order given. A package has at most one file per layer name (`E1906`); a name that
  matches no file of any loaded package is `E1901` (exit code 2). Every layer file is type-checked,
  active or not, so renaming a field breaks a stale layer at once.
- `amend <value> { path: expr, … }` overrides a top-level `let` of the layer's own package
  (`E1909` for another package, `E1902` for a `const`) at each path. The override applies to the
  value right after its base evaluation, before anything reads it, so it works on loaded and
  computed values too, and values derived from it follow. Within the amended record, later fields
  that took their default are recomputed from the new value.
- A path is made of `.f` (a field, or a table or keyed-list entry), `[k]` (a key, or a plain
  list's index) and `[#n]` (a position in any ordered collection, as in API paths). It must exist,
  except that its last segment may be a new map key or table key (the expression is then the whole
  entry); otherwise `E1905`. List elements may be replaced, not appended. Map and table entries
  cannot be removed, entries cannot be added to a stable table (`E6004`), and one layer cannot set
  a path twice or a path and its prefix (`E1908`). The expression is typed against the path.
- Every check runs again on the result: a layer cannot produce an invalid config.
- `canon explain <path> --layer …` shows the final value and where each part was set; provenance
  records the layer (§11.5).
- A layered build writes the same `out` files as a plain one.

Details: [spec/EVALUATION.md](spec/EVALUATION.md) §9.

### 19.2 Runtime inputs

Secrets and machine-local values are never written in files. A field can be an **input**:

```
record EditorConfig {
  apiKey: input String(1..)? from env "EDITOR_API_KEY"
  dbUrl: input String(1..) from env "APP_DB_URL"
}
```

- An input's type is `Bool`, an integer type, `Float`, `Float32`, `String`, `Duration` or an enum,
  possibly optional, refined only by ranges, lengths and portable patterns; anything else,
  including a `where`, is `E1910`. An input has no default (`E1907`); `input T?` means "`none`
  when unset". The variable name matches `[A-Za-z_][A-Za-z0-9_]*` (`E1911`).
- Inputs are allowed only in records reachable from a single public value that is not a
  collection, through record fields only (`E1903`), so one variable is one value.
- Input fields never appear in literals or loaded data (`E3312`) and are absent from Canon values:
  checks, functions, views and other values cannot read them (`E3313`). Their values do not exist
  at build time. They are omitted from data files.
- A variable that is unset **or empty** is absent: an optional input gives `none`, a required one
  makes loading fail with the variable's name. Otherwise the text is read as a literal of the
  field's type (`true`/`false`, a number, a duration literal such as `1h30m`, an enum member by
  its wire value, a string byte for byte); the accepted text is
  [spec/EVALUATION.md](spec/EVALUATION.md) §11.3.
- Generated code reads inputs in an explicit call made at startup: Go `LoadInputs() error`, C++
  `bool LoadInputs(std::string& error)`; every failure is reported, not only the first. Reading an
  input before `LoadInputs` is `E8302`: Go panics with `*rt.EvalError`, C++ calls
  `canon::OnEvalError`.
- TypeScript has no environment: a package with inputs cannot have a TypeScript emit (`E8104`).

### 19.3 The only runtime validation

For an input, `LoadInputs` checks the field's **own refinement** (the range of a sized type,
ranges, lengths in bytes, patterns with search semantics, enum membership); `where` predicates
are never checked at runtime. That check is generated from the type, and a pattern must use the
subset shared by RE2 and ECMAScript regexes (`E1904`). It is the only validation that ever runs at
runtime.

---

## 20. Standard library

The library is closed and versioned with the language. Anything not in it is written in Canon.
This section lists it; every generic signature, empty-collection result, error code, ordering
guarantee and step cost is in [spec/STDLIB.md](spec/STDLIB.md).

### 20.1 Conversions and math

| Function | Meaning |
|---|---|
| `Int(x: Float) -> Int` | truncate toward zero; `E4103` if out of range |
| `Float(x: Int) -> Float` | exact when representable |
| `String(x) -> String` | canonical text form (§2.5) |
| `abs(x)` `min(a, b, …)` `max(a, b, …)` `clamp(x, lo, hi)` | on numbers and durations, all arguments of the same base type; `min` and `max` take at least 2 arguments and return the first of equal ones; `clamp` is `min(max(x, lo), hi)`, and `lo > hi` is `E4108`. On floats, −0.0 orders before +0.0, so every target gives the same bits |
| `floor(x)` `ceil(x)` `round(x)` | `Float -> Int`; `round` rounds half away from zero; out of range is `E4103` |
| `sqrt(x)` `pow(x, y)` | `Float`; a NaN or infinite result is `E4104` |

There is no conversion between `Int` and `Duration`: write `n * 1s` and `Int(d / 1ms)`.

### 20.2 Strings

`len()` (bytes), `isEmpty()`, `contains(s)`, `startsWith(s)`, `endsWith(s)`, `split(sep)`
(`split("")` is `E4106`), `trim()` (ASCII whitespace), `lower()`, `upper()` (ASCII only),
`replace(a, b)` (every occurrence), `matches(/re/)` (RE2, search semantics, §5.2),
`find(s) -> Int?` (a byte index), `[a..b]` (byte slice; must fall on character boundaries, else
`E4107`).

### 20.3 Lists

These methods also apply to tables and keyed lists, over their entries (which keep their
identity). Methods that return a list return a plain list.

| Method | Meaning |
|---|---|
| `len()`, `isEmpty()` | size |
| `first()`, `last()` | `T?` |
| `first(pred)` | first element matching, `T?` |
| `get(i)` | `T?`, by position (on a table or keyed list, `get` takes a key, §20.4) |
| `contains(x)`, `indexOf(x) -> Int?` | membership |
| `map(f)`, `filter(pred)`, `flatMap(f)`, `flatten()` | transformation |
| `any(pred)`, `all(pred)`, `count(pred)` | predicates |
| `sum()`, `min()`, `max()` | on numbers and durations only; `sum()` of an empty list is `0`, `0.0` or `0s`; `min`/`max` are `T?`, the first of ties |
| `minBy(f)`, `maxBy(f)` | `T?` |
| `sortBy(f)`, `reverse()` | stable sort |
| `groupBy(f) -> {K: [T]}` | keys in first-seen order |
| `unique()`, `isUnique()` | by equality |
| `enumerate()` | pairs `(i, x)` |
| `pairs()` | every unordered pair `(a, b)` with `a` before `b` |
| `zip(other)` | pairs; lists of different lengths are `E4105` |
| `join(sep)` | on `[String]` only |
| `intersect(b)`, `union(b)`, `diff(b)` | order of the left list |
| `toMap(keyF, valF)` | `{K: V}`; duplicate keys are an error |

### 20.4 Maps, tables and ranges

**Maps:** `m[k]`, `len()`, `isEmpty()`, `keys() -> [K]`, `values() -> [V]`, `get(k) -> V?`,
`contains(k)`, `map(f)` (values), `filter(pred)`, `all(pred)`, `any(pred)`, `count(pred)`; map
predicates take `(k, v)`.

**Tables and keyed lists** add key access to the list methods: `xs[k]` and `xs.k` (the entry with
key `k`, `E4002` if missing), `get(k) -> T?` and `find(k) -> T?` (by key), `at(i) -> T` (by
position, negative from the end), `keys() -> [ref T]`, `values() -> [T]`, and, on tables,
`active()` (the entries not retired, as a `[T]`). `x in xs` accepts an entry or a key.

**Ranges:** `r.start`, `r.end`, `r.len()` (`max(end − start, 0)`), `r.isEmpty()`,
`r.contains(x)` (same as `x in r`); `.end` and `.len()` of an open range are `E4002`.

### 20.5 Graphs

| Function | Meaning |
|---|---|
| `reachable(from: x, next: f) -> [T]` | every element reachable from `x` following `f` (which returns `T?` or `[T]`), including `x` first, in depth-first preorder |
| `cycles(xs, next: f) -> [T]` | elements of `xs` that lie on a cycle (a self-loop counts), in input order |
| `topoSort(xs, next: f) -> [T]` | dependency order, input order breaking ties; `E4501` on a cycle, naming it |

### 20.6 Findings

`fail(at, message)` and `warn(at, message)`, only inside `check { }` blocks (§10.2).

---

## 21. Diagnostics

### 21.1 Format

```
error[E3501]  @resource/Server/System/farm_config.json:212:18
  farm.modelTypes[3].levels[2].productionItem: unknown key "II_SYS_SYS_SCR_FARM3" in resource.vocab.items
  expected by resource/farm/farm.canon:41 (productionItem: ref items)
```

- `severity[CODE]`, two spaces, the location; then, each indented by 2 spaces, the value path and
  message, and the related locations. A blank line separates findings, and a summary line ends
  the output. The exact text templates are fixed in [spec/API.md](spec/API.md) §4.
- A finding has: severity (`error`, `warning`), code, file, line, column, end line and end column
  (1-based; columns count UTF-8 bytes; the end is exclusive), the RFC 6901 `pointer` when the file
  is a loaded JSON file, the `package` it belongs to, the value `path` (starting at the value's
  name, without the package, CLI.md §2.6), the message, the related locations (file, span, note),
  the check's name when a `check` produced it, the Canon call stack for evaluation errors (at most
  16 frames), the layer when a layer set the value, and `reads` (the fields a check read, which
  the studio highlights).
- `canon … --format json` prints one JSON object per finding with these fields, in a fixed key
  order, for tools (CLI.md §2.4, [spec/API.md](spec/API.md) §4).
- Findings are sorted (§11.3). After 1,000 findings in one package, the rest are counted in the
  summary.

### 21.2 Codes

The single source of every code, with its severity, owning package and document, meaning and
messages, is [spec/ERRORS.md](spec/ERRORS.md). The ranges (a range names the owning document; the
Go package that reports a code is ERRORS.md's `Package` column):

| Range | Area | Owner |
|---|---|---|
| `E10xx` | the project file | GRAMMAR.md |
| `E11xx` | lexer and parser | GRAMMAR.md |
| `W10xx` | doc comments and naming conventions | GRAMMAR.md |
| `E16xx` / `W16xx` | views | VIEWMODEL.md |
| `E17xx` / `W17xx` | translations | I18N.md |
| `E19xx` | layers and runtime inputs | EVALUATION.md |
| `E2xxx` | names, imports, packages; resolution (`E21xx`) | TYPES.md |
| `E3xxx` / `W3xxx` | types, keys and entries (`E31xx`), numbers and refinements (`E32xx`), literals and wire typing (`E33xx`), optionals (`E34xx`), references (`E35xx`), `match` (`E36xx`), assets (`E37xx`), dependent types (`E38xx`) | TYPES.md; `E3203` and `E3315`–`E3318` WIRE.md |
| `E4xxx` | evaluation: `!` and indexes (`E40xx`), arithmetic (`E41xx`), an internal safety net, cycles, call depth and budget (`E42xx`–`E44xx`), standard library (`E45xx`) | EVALUATION.md, STDLIB.md |
| `E5xxx` / `W5xxx` | user checks and tests: `E5001`/`W5001` one-line, `E5002`/`W5002` block form | EVALUATION.md |
| `E6xxx` / `W6xxx` | stable ids and the lock | LOCK.md |
| `E7xxx` / `W7xxx` | paths and `load` (`E71xx`) | WIRE.md |
| `E80xx` / `W80xx` | `emit` and generated code | CODEGEN.md |
| `E81xx` | values that generated code or data files cannot hold; `E8102` and `E8150`–`E8153` are WIRE.md's | CODEGEN.md, WIRE.md |
| `E82xx` | `@reload` | CODEGEN.md |
| `E83xx` | signalled at runtime by generated code (`E8301`–`E8303`), never reported by the compiler | CODEGEN.md |
| `E9xxx` | `export fn` portability and conformance | CONFORMANCE.md |

Each companion document lists the codes of its area and says when each fires; the messages,
with their typed arguments, are defined only in [spec/ERRORS.md](spec/ERRORS.md), the single
source from which the compiler's registry is generated (DECISIONS 27). A code never changes
meaning once released.

---

## 22. Out of scope

- Running Canon at runtime. The only logic a target receives is `export fn` in its portable
  subset, with its conformance test.
- Any I/O beyond `load`; any environment access outside `input` fields.
- User-defined generics, macros, operator overloading, record inheritance (use spread).
- Removing entries from stable tables.
- Formatting options, and column alignment.
- Built-in data migrations: renaming a Canon field never touches data; other changes are
  one-off scripts.
- Case-insensitive asset matching: legacy case drift is fixed in the data (DECISIONS 19).
- Exceptions in generated C++ (DECISIONS 19).
- Code targets other than Go, C++ and TypeScript.
- Unwrapped JSON output: every data file has the `$schema` wrapper, so every one carries the
  generated marker and the fingerprint check; a non-Canon reader reads `value` or `rows`
  (WIRE.md §8.1, no `bare` option).

---

## 23. Open questions

1. **Name.** "Canon" is a placeholder.
2. **Several views per type.** A compact one for pickers and a full one for editing? (v0 allows one
   view per type, plus views of variant cases.)
3. **Binary layouts.** Is `@cpp(type: …)` enough for legacy C++ structs, or does Canon need a
   `layout` declaration for binary formats?

Closed since the previous draft: unwrapped JSON (no `bare` option, §22) and the outputs of layered
builds (a `--layer` build writes the same files as a plain one, by design: EVALUATION.md §9.4).

---

## Appendix A: Grammar

The normative grammar is [spec/GRAMMAR.md](spec/GRAMMAR.md): the lexer and its modes (§2), the
newline and separator rules (§3), keywords (§4), the EBNF of files, declarations, views, types,
statements and expressions (§5), the disambiguation rules (§6), the `project.canon` grammar (§7),
the annotation catalogue (§8), doc comments and naming conventions (§9), and what the parser
produces (§10). Whether a `{ … }` is a record, map or table literal is decided by the type checker
([spec/TYPES.md](spec/TYPES.md) §5.2), not by the parser.

## Appendix B: Keywords

- **Reserved words:** [spec/GRAMMAR.md](spec/GRAMMAR.md) §4.1. Where a reserved word may still be
  used as a name: GRAMMAR.md §4.3, summarized in §2.4. A field whose wire name is a reserved word
  keeps a clean Canon name and uses `@json`: `kind: RewardType @json("type")`.
- **Contextual words** are identifiers everywhere except in one position (GRAMMAR.md §4.2):
  `keyed` and `by` after a list type; `ordered` after an enum's name; `from` and `env` after an
  input's type; `fails`, `warns` and `passes` after an `expect` subject; `at` in a one-line check;
  the view words `title`, `subtitle`, `singular`, `plural`, `menu`, `icon`, `preview`, `search`,
  `filters`, `columns`, `group`, `show`, `field`, `advanced`, `when` and `multi`; `default` after a
  widget's parameters (which are named `value` and, optionally, `siblings`); `ext` in `asset(…)`.
  `it`, `fail` and the standard functions are ordinary names with predeclared meanings.
