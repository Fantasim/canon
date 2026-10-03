# Canon code generation

Version: **0.1 (draft)**, companion to [SPEC.md](../SPEC.md) §14–§15. Normative.

This document fixes, for the three code targets (Go, C++17, TypeScript), the names, files and
exact public API of everything `canon build` generates, the runtime helper files in full, and
the legacy C++ struct mapping. It replaces the prose of SPEC §15.1–§15.5 wherever the two
differ (see [§11](#11-changes-needed-in-other-documents)). Conformance tests of translated
functions are specified in [CONFORMANCE.md](CONFORMANCE.md).

"Must", "must not" and "is an error" are requirements on the compiler. Error codes are listed
in [§12](#12-diagnostics).

---

## Contents

1. [Scope, dependencies and stability](#1-scope-dependencies-and-stability)
2. [What an emit produces](#2-what-an-emit-produces)
3. [Naming](#3-naming)
4. [Type mapping](#4-type-mapping)
5. [Constructs](#5-constructs)
6. [Go](#6-go)
7. [C++](#7-c)
8. [TypeScript](#8-typescript)
9. [Toolchain floors](#9-toolchain-floors)
10. [Goldens](#10-goldens)
11. [Changes needed in other documents](#11-changes-needed-in-other-documents)
12. [Diagnostics](#12-diagnostics)

---

## 1. Scope, dependencies and stability

### 1.1 What this document owns

- the public API of generated Go, C++ and TypeScript: type, function, method and constant names,
  signatures and return types;
- the files each emit writes, their names and headers;
- the runtime helper files (`rt/rt.go`, `canon_runtime.h`, `canon_runtime_json.h`, the TS helper
  block), given in full;
- the mapping of records onto legacy C++ structs (`@cpp(struct:, access:)`) and `@cpp(defines:)`;
- the codes `E80xx`, `E81xx` (except `E8102` and `E8150`–`E8153`, which WIRE.md owns), `E82xx`,
  `E83xx` and `W80xx` ([§12](#12-diagnostics)).

### 1.2 What it relies on

| Document | Assumption made here |
|---|---|
| WIRE.md | The bytes of `emit json` files, the `$schema` / `rows` / `value` envelope, `$id` and `$retired` on table rows (WIR-06), `$<fn>` keys for precomputed export fns (WIR-09), `null` for `none` (WIR-02), durations as integers in their `@json(unit:)` (WIR-03, LOD-10), the slot keys of `@json(pairs:)` (§5.14), and the decode rules for loaded sources (LOD-02). Generated loaders read exactly what WIRE.md writes; `types`-mode decoders read what WIRE.md's decode rules accept. |
| FINGERPRINT.md | The value of `$schema` (`<package>.<Type>@<8 hex>`). Generated code compares it as an opaque string. |
| TYPES.md | Sized integers are `Int` plus a range (TYP-03); `UInt64` is limited to `0..=INT64_MAX`; a value outside a sized type is `E3201`, a non-finite or `Float32`-overflowing value `E3202`, a range-refinement violation `E3204`. Stored `Duration` values are limited to ±9 223 372 036 854 ms, the range of Go's `time.Duration` (TYPES.md §7.2), so every Go getter can return a `time.Duration`. |
| EVALUATION.md / STDLIB.md | The meaning of every operator and function of the portable subset, as restated in CONFORMANCE.md §3. |
| GRAMMAR.md | The annotation catalogue (§8) accepts the `@go`, `@cpp` and `@ts` arguments of [§3.5](#35-overrides-and-collisions) and [§7.8](#78-legacy-structs); `project.canon` has `go_module` (§7.1, root name → module path). |

### 1.3 Stability

- The **public API** is everything a runtime may name: exported Go identifiers except those of
  generated `wire*`/`decode*`/`load*` helpers, C++ declarations outside namespaces named `detail`,
  TypeScript exports. It is normative.
- Private members, `detail` namespaces, unexported Go identifiers and non-exported TS helpers are
  the **reference layout**. Runtimes must not use them. The goldens ([§10](#10-goldens)) fix the
  reference layout byte for byte; once GEN-01 freezes them, the compiler's output must match
  them exactly.
- Generated code never contains line numbers, timestamps, absolute paths or the compiler version,
  so an unrelated edit or a rebuild on another machine does not rewrite files.

---

## 2. What an emit produces

### 2.1 Targets, modes and options

| Target | `out` | Modes | Default mode | Other options |
|---|---|---|---|---|
| `go` | directory (one Go package) | `baked`, `embedded`, `data`, `types` | `baked` | `package` (default: last element of `out`, the same for every entry of a list) |
| `cpp` | directory | `baked`, `embedded`, `data`, `types` | `baked` | `namespace` (default: package path with `.` → `::`) |
| `ts` | file ending in `.ts` | `baked`, `embedded`, `data`, `types` | `baked` | none |
| `json` | WIRE.md | — | — | `values` |
| `view` | VIEWMODEL.md | — | — | — |

- A package has at most one `emit` per target (`E8002`). A target word not in this table, or an
  unknown option, is `E8003`. An invalid option value is `E8009`: a mode the target does not
  have, a `package` that is not a Go identifier or is a Go keyword, a `namespace` that is not
  `ident{::ident}` or uses a C++ keyword, a `namespace` with a segment `canon`, `std` or
  `nlohmann` (reserved: the runtime's and the libraries' namespaces), a `ts` `out` not ending in
  `.ts`, a `values` item that is not a public top-level `let` of the package or is listed twice.
  `out` is required for every target (`E8009` `missing` without it).
- **Copies** (DECISIONS 229). For `go`, `cpp`, `ts` and `json`, `out` is a string or a non-empty
  list of strings; `view` takes one string. Each entry is a **copy** of the emit, written at that
  place as if it were the emit's only `out`. Every copy has the emit's mode and options; copies
  differ only in the import paths or includes of imported packages and, in Go, the import path of
  its own `rt` package (§2.8). An empty list is `E8009` `outEmpty`, and two entries sharing an
  owning root (§2.8) are `E8009` `outRoot`, reported at the later entry against the first one that
  root owns; each entry is checked as an `out` (a `ts` entry must end in `.ts`). Without `package`,
  every entry of a Go list must end in the same element once resolved, the default package name
  (`E8009` `outPackage` otherwise); the entries of a `json` list are all files or all directories
  (`E8009` `outForm` otherwise, WIRE.md §8.1); a list given to `view` is `E8009` `kind` (DECISIONS
  269, 270).
- **Typing of the options.** The options of each target form a built-in schema, checked in
  phase 2 like `project.canon` (GRAMMAR.md §7): nothing in an `emit` is an expression, is
  evaluated, or is resolved in scope.
  - `package` and `namespace` are constant strings, and `out` a constant string or, except for
    `view`, a list of them (`E1132` for an interpolation, `E8009` for another kind of value; a
    non-string element of an `out` list is `E8009` `kind` with `OutPaths`, at the element,
    DECISIONS 270).
  - `mode` is a bare word from the target's modes (`E8009` otherwise). It is never looked up in
    scope, so `mode: baked` is not an unknown name (`E2102`) and a `let baked` changes nothing.
  - `values` is a list of bare words, each the name of a public top-level `let` of the package;
    the names are matched against the package's declarations, not resolved as expressions.
- **When emit rules are checked.** Every code of this document's §12 except `E8001` is found in
  stage E (EVALUATION.md §1), which builds the IR of every emit and validates it without writing:
  `canon check` reports them exactly as `canon build` does (CLI.md §3.3). `E8001` needs the output
  file system and is reported in phase 8 (emit) only, like WIRE.md's `E8152`.
- `values` (default: every public value) selects the values emitted; an explicit `values: []` is
  the default too, every public value in declaration order. Every emit always contains
  **every public type, enum, constant and export fn** of the package, used or not (EMT-07).
  Imported types are referenced, never re-emitted.
- TypeScript `embedded` produces exactly the `baked` output (a bundle has no separate file).

### 2.2 What each mode contains

| | `baked` | `embedded` | `data` | `types` |
|---|---|---|---|---|
| types, enums, constants | yes | yes | yes | yes |
| table ids ([§5.3](#53-table-ids)) | enum | enum | string | string |
| value accessors `Get<V>()` | static data | decoded once from the embedded file | — | — |
| `Load<V>` / `<V>::Load` | — | — | non-`@reload` values | — |
| snapshot and store ([§5.11](#511-reloadable-values-snapshot-and-store)) | `E8202` | `E8202` | `@reload` values | `E8202` |
| public decoders ([§5.13](#513-types-mode-decoders)) | — | — | — | yes |
| precomputed getters | yes | yes | yes (from `$` keys) | `E8014` |
| package-level export fns without runtime input | yes | yes | `E8013` | `E8014` |
| methods with finite parameters | yes | yes | enum and `Bool` parameters only (`E8013`) | `E8014` |
| translated functions | yes | yes | yes | yes |
| `LoadInputs` ([§5.12](#512-runtime-inputs)) | yes | yes | yes | — |
| C++ needs nlohmann/json | no | yes | yes | yes |

In `data` and `embedded` mode, an emitted value must be a table, a keyed list or a record
(`E8015`); wrap anything else in a record.

### 2.3 Files

The compiler writes, per emit:

| Target | Files (`<last>` = last segment of the Canon package; `<gopkg>` = Go package name) |
|---|---|
| Go | `<out>/<gopkg>.gen.go`; `<out>/<gopkg>_conformance_test.go` if the package has a translated function; `<out>/rt/rt.go`; in `embedded` mode, `<out>/<value>.json` per emitted value |
| C++ | `<out>/<last>.gen.h`, `<out>/<last>.gen.cpp`; `<out>/<last>_conformance.gen.cpp` if the package has a translated function; `<out>/<last>.defines.gen.h` if an enum has `@cpp(defines:)`; `<out>/canon_runtime.h`; `<out>/canon_runtime_json.h` in `embedded`, `data` and `types` mode |
| TS | `<out>` (for example `generated.ts`); `<dir of out>/<last>.conformance.test.ts` if the package has a translated function |

- Two outputs with the same path, or with paths that differ only in letter case, are WIRE.md's
  `E8152` (WIRE.md §8.1), whatever their targets. The runtime files are the exception: identical
  content may be written by several emits. Two Go emits writing into the same directory, which is
  one Go package, are `E8008` even when their file names differ; `E8008` compares different emits
  only, and copies of one emit sharing a directory are `E8009` `outRoot` alone (DECISIONS 270).
- **Embedded data.** In `embedded` mode each emitted value's data is the data-wire document of
  WIRE.md §8.2 for that value (the bytes `emit json` would write for it, `$schema` included),
  whether or not the package has an `emit json`. Go writes it to `<out>/<value>.json` and embeds it
  with `//go:embed`; C++ writes no file and places the same bytes in `.gen.cpp` as a
  `static const unsigned char k<Value>Data[]` with its length; TypeScript `embedded` is `baked`
  (the values are object literals). The embedded copy is regenerated by every build, like any
  output, so it cannot go stale.
- `.gen.cpp` is always written, even when it holds nothing but its header and includes.
- The runtime files are identical in every output directory for a given runtime version.
- **Copies.** Each copy of an emit (§2.1) writes this whole set at its own `out` and is an output
  like any other: its files are compared for `E8152` and `E8008`, carry their generated markers
  (§2.4, WIRE.md §8.4), and are written with the lock, or not at all (LOCK.md §5).

### 2.4 Header lines

The first line of every generated file is its **marker**:

| Files | Marker |
|---|---|
| `.go` | `// Code generated by canon from <dir>/. DO NOT EDIT.` |
| `.h`, `.cpp`, `.ts` | `// GENERATED by canon from <dir>/. DO NOT EDIT.` |
| runtime files | `// Code generated by canon: Go runtime rt v1. DO NOT EDIT.`, `// GENERATED by canon: C++ runtime rt_v1. DO NOT EDIT.`, `// GENERATED by canon: C++ runtime rt_v1 (JSON). DO NOT EDIT.` |
| `.json` | the top-level `"$schema"` (WIRE.md) |

`<dir>` is the package directory relative to the project root (`pipeline`, `resource/vocab`),
`/`-separated (CG-07). The Go marker matches Go's `^// Code generated .* DO NOT EDIT\.$`.

`canon build` overwrites a file only if its first line matches
`^// (Code generated|GENERATED) by canon\b.* DO NOT EDIT\.$`, or, for `.json`, if its top-level
`$schema` matches WIRE.md's pattern. Otherwise the build fails with `E8001` and writes nothing.
The one way to take over a hand-written file is the `--adopt` flag: `canon convert --adopt` for a
JSON source that becomes the emitted data file (IMPLEMENTATION-PLAN.md §8.3), and
`canon build --adopt <path>` for the header of an `access: both` struct
([§7.8.3](#783-access-both)); the output is then reported `adopted` (API.md B2).

### 2.5 File headers and fixed comments

A generated file's header holds one or two comment lines, in every language. Line 1 is the
marker (§2.4). At most one more comment line follows it:

- Go main file (`<gopkg>.gen.go`): the marker, a blank line, then
  `// Package <gopkg> is generated by canon from package <canon.pkg>.`, the Go package doc, directly
  above `package`.
- Go runtime `rt/rt.go` (§6.3): the marker, a blank line, then
  `// Package rt is the runtime of the Go code canon generates.`, directly above `package`.
- Go conformance file: the marker, a blank line, T7, then a blank line before `package`, so T7 is
  not a package doc.
- C++ and TS conformance files: the marker, then T7 on the next line.
- Every other file (`.gen.h`, `.gen.cpp`, `.defines.gen.h`, the main `.ts` file, the C++ runtime
  headers): the marker alone.

The Canon package doc (GRM-08: the `///` lines before `package`) is not copied into generated
files. Generated code follows the conventions of the code beside it: Go those of sovcommon, C++
those of Source's recent code, never its legacy style (no `m_` members, no `C` class prefix).

The explanatory comments in generated code are fixed templates, listed below; no other prose is
generated. Each template is written with exactly the line breaks shown (`/` separates lines):
substituted names never re-wrap it. Placeholders: `<pkg>` the Canon package, `<P>` its
UpperCamel last segment (`Pipeline`), `<gopkg>` the Go package name, `<last>` the output file
stem, `<dir>/<file>` the Canon source of a function, `<Type>.<fn>` its Canon name, `<goFn>` the Go
name of the pure function, `<Const>` the Go schema constant, `<file>` a data file name, `<files>`
the data files of a snapshot joined by `, `, `<key>` the key field's Canon name. They are
normative: a golden that holds other prose is wrong.

| Id | Target | Where | Text |
|---|---|---|---|
| T2 | C++ | schema constant | `/// Fingerprint of the schema of <file> (types, wire names, units), not of` / `/// its values. The loader refuses a file built from another schema.` |
| T2 | Go | schema constant | `// <Const> is the fingerprint of the schema of <file> (types, wire names,` / `// units), not of its values. The loader refuses a file built from another schema.` |
| T3 | C++ | pure translated function (in `detail`) | `// Translated from <dir>/<file> (<Type>.<fn>). The method and the` / `// conformance test both call it, so the test checks the code that runs.` |
| T3 | Go | pure translated function | `// <goFn> is translated from <dir>/<file> (<Type>.<fn>). The method` / `// and the conformance test both call it, so the test checks the code that runs.` |
| T4 | C++ | snapshot class | `/// Every @reload value of package <pkg>, loaded together and never modified.` / `/// A reload builds a new snapshot; readers keep the one they hold.` |
| T4 | Go | snapshot type | `// <P>Snapshot holds every @reload value of package <pkg>, loaded together and` / `// never modified. A reload builds a new snapshot; readers keep the one they hold.` |
| T5 | C++ | store class | `/// The current snapshot of package <pkg>.` / `///   auto snap = <P>Store::Current();   // hold it for one tick or request` / `///   <P>Store::Reload(dir, error);      // the old snapshot lives while someone holds it` / `/// Current() is nullptr until the first successful Reload.` |
| T5 | Go | store type | `// <P>Store holds the current snapshot of package <pkg>.` / `//` / `//	snap := <gopkg>.Store.Current()   // keep it for one tick or request` / `//	err := <gopkg>.Store.Reload(dir)  // the old snapshot lives while someone holds it` (a tab after `//` on the two code lines) |
| T6 | C++ | snapshot `Load` | `` /// Reads <files> from `dir`. Returns nullptr and sets `error` if a file is `` / `/// missing, is not JSON, or was built from another schema. Values are not` / `` /// validated here: `canon build` already did, and filled every default. `` |
| T6 | Go | `Load<P>Snapshot` | `// Load<P>Snapshot reads <files> from dir. It fails if a file is missing, is` / `` // not JSON, or was built from another schema. Values are not validated here: `canon `` / `` // build` already did, and filled every default. `` |
| T7 | C++, TS | conformance file, right after the marker | `// Conformance vectors of package <pkg>, computed by the Canon evaluator.` |
| T7 | Go | conformance file, after the marker and a blank line, followed by a blank line | `// Conformance vectors of package <pkg>, computed by the Canon evaluator.` |
| T8 | C++ | container `Find` | `/// Binary search by <key>. Never allocates; nullptr when absent.` |
| T8 | Go | container methods | `// Len returns the number of entries.`; `// At returns entry i, in source order.`; `// All yields every entry in source order.`; `// Find returns the entry whose <key> is key.` |
| T9 | C++ | store `Reload` | `` /// Loads a new snapshot from `dir` and swaps it in. On failure the current `` / `` /// snapshot stays, `error` is set and the result is false. `` |
| T9 | Go | store methods | `// Current returns the current snapshot, or nil before the first successful Reload.`; `// Reload loads a new snapshot from dir and swaps it in. On failure the current` / `// snapshot stays and the error is returned.`; on the store variable, `// Store is the store of package <pkg>.` |
| T10 | C++ | conformance entry point, in the header | `/// Defined in <last>_conformance.gen.cpp: runs every conformance vector of the` / `/// package and returns the number of failures (0 when the translation agrees).` |
| T11 | C++ | conformance file | on the captured code, `// first evaluation error of the current call, empty if none`; on a vector's code member, `` // expected error code; empty when `want` is expected `` |
| T11 | Go | conformance file | on `canonCatch`, `// canonCatch runs f and returns its result, or the code of the *rt.EvalError it` / `// panicked with. Any other panic is re-raised.`; on a vector's code field, `// expected error code; empty when want is expected` |

The runtime helper files (`rt.go`, `canon_runtime.h`, `canon_runtime_json.h`, the TypeScript
helper block) are fixed texts, reproduced in §6.3, §7.4, §7.5 and §8.2; their comments are part
of those texts.

### 2.6 Doc comments

Doc text is the normalized text of LEX-07 (`///` and one space stripped, lines joined with `\n`,
trailing blank lines trimmed). It is copied **verbatim** (CG-06):

| Target | Form |
|---|---|
| Go | `// ` + line; an empty line is `//`. The first line is prefixed with `<GoName>: ` unless it already starts with `<GoName>` followed by a space. The file is then passed through `go/format` (which turns indented doc lines into `//\t` code blocks). |
| C++ | `/// ` + line; an empty line is `///`. |
| TS | one line: `/** text */`; several: `/**`, ` * ` + line (` *` for an empty line), ` */`. `*/` inside the text is written `*\/`. |

The doc of a field goes on its getter (and on its key getter only when there is no resolved
getter); a record's or enum's doc on the type; a member's on the constant or enumerator; a
value's on its accessor, container class and snapshot getter; an export fn's on its method or
function; a variant case's on its case type, or, for a case without fields (which has no type),
on its kind-enum member (§5.5). Undocumented items get no comment (the build warns with `W1002`).

### 2.7 Order and determinism

- **Declaration order** means: files of the package in byte order of their path, then source
  order. Everything is emitted in declaration order, except C++ classes, which are topologically
  sorted so that a class comes before the first class holding it by value (depth-first, stable
  with respect to declaration order). A stored result held by value orders classes like a field;
  it never closes a cycle (a result type reaching its receiver is E8019, DECISIONS 284).
- Within a file, the order of sections is fixed: constants, enums (declaration order, then kind
  enums of variants, branch enums of dependent types, id enums of tables), records and variants,
  containers, value accessors, export fns, snapshot and store, inputs, decoders (those of
  classes in declaration order, then those of dependent types a decoded class holds).
- **C++ declares before use**, so a header refines that order: (1) constants, then the schema
  constants (T2); (2) enums, as above; (3) a forward declaration `class <T>;` of every class of
  the header, in declaration order; (4) `namespace detail {`: `struct <P>Access;`, the decoder
  declarations (one per class decoded from JSON, declaration order), then the pure translated
  functions (T3, declaration order), `}  // namespace detail`; (5) the classes: records and
  variants (topologically sorted, above), containers, value accessors, package-level export fns,
  snapshot and store, inputs; (6) `namespace conformance {` with the entry point declaration
  (T10), when the package has translated functions. The `.gen.cpp` holds, inside `namespace
  detail`, the decoder definitions then the `<P>Access` definition, then the out-of-line member
  definitions in class order.
- **C++ includes**, in three groups separated by one blank line, each sorted bytewise:
  - a `.gen.cpp` or `_conformance.gen.cpp` first includes its own header `"<last>.gen.h"`, alone
    in its group;
  - the standard headers of every standard-library name the file's own text uses, whether or not
    an included header already provides it, from this table: `std::chrono` `<chrono>`, `size_t`
    `<cstddef>`, `int64_t` and the other fixed-width integers `<cstdint>`, `std::fprintf`
    `<cstdio>`, `std::shared_ptr`/`std::make_shared` `<memory>`, `std::optional` `<optional>`,
    `std::string`/`std::to_string` `<string>`, `std::string_view` `<string_view>`, `std::move`
    `<utility>`, `std::vector` `<vector>`, `std::array` `<array>`, `errno`/`ERANGE` `<cerrno>`,
    `DBL_MIN` `<cfloat>`, `std::fabs`/`std::isfinite` `<cmath>`, `std::getenv`/`std::strtoll`
    `<cstdlib>`, `std::locale` `<locale>`, `std::istringstream`
    `<sstream>`;
  - then, in a `.gen.h`, `<nlohmann/json_fwd.hpp>` (in its own group) when the header declares a
    decoder, and `"canon_runtime.h"` followed by the headers of imported packages (§2.8) in byte
    order; in a `.gen.cpp` of a mode that decodes JSON, `"canon_runtime_json.h"`.
- The generator never iterates a Go map or any unordered collection when producing output
  (NFR-05). CI builds every example twice, with `GOMAXPROCS=1` and `GOMAXPROCS=8`, and diffs the
  outputs.
- Go output is passed through `go/format.Source`; a formatting failure is an internal compiler
  error (exit 3).

### 2.8 Cross-package references

A generated type may use a type of an imported package (EMT-06). The imported package must have
an emit for the same target, else `E8004`. An imported type is referenced, never re-emitted.
A TypeScript file imports every package its types reach (fields, cases, export fns, dependent
branches, followed through other packages), since its literals and decoders may write any of them,
not only its direct imports; such a package needs a `ts` emit (`E8004`), and
the stage-E name plan reserves the same imports (DECISIONS 279).

- **Owning root and copies** (DECISIONS 229). An output's owning root is the declared root
  (GRAMMAR.md §7.1 `roots`) whose directory is the output's directory (a file's directory, for a
  `ts` or a `json` file) or its closest ancestor, judged lexically on project-relative paths as
  `GoImport` and `E8007` are, so a root reached through `..` never owns an output inside the project
  (of two roots with the same directory, the one declared first in `roots`); an output under no
  declared root is owned by the project itself, written `project <name>` in findings
  (`project acme { … }` gives `project acme`; DECISIONS 269, 270). Two entries of one emit's `out` may not share an
  owning root (`E8009`). A copy of package P owned by R (a root, or the project) uses, for each
  imported package Q, Q's copy owned by R, or Q's only copy when Q has one; otherwise `E8004`. The
  rules below then apply between the two copies.

`project.canon`'s `go_module` key (GRAMMAR.md §7.1) maps a root name to the Go import path of
the root's directory. From `examples/project.canon`:

```
roots {
  services: "../.."
  sovcommon: "../../sovcommon"
  pipeline_go: "pipeline/out/go"
  features: "features"
}
go_module {
  services: "gitlab.com/sovereign15"
  sovcommon: "gitlab.com/sovereign15/sovcommon"
  pipeline_go: "example.com/potions"
  features: "example.com/features"
}
```

- **Go.** A Go output directory `D` (resolved to a directory, however the `out` path was written)
  belongs to the mapped root whose directory is `D` or its closest ancestor; its import path is
  that root's module path joined with `D`'s path relative to the root directory:
  `@services/resourcestudio/internal/config` → `gitlab.com/sovereign15/resourcestudio/internal/config`;
  `@sovcommon/teamboard` → `gitlab.com/sovereign15/sovcommon/teamboard` (the `sovcommon` root is a
  closer ancestor than `services`); the pipeline's `out/go/` → `example.com/potions`;
  `features/embedded/out/go/` → `example.com/features/embedded/out/go`. A Go emit under no mapped
  root is `E8007`. The `rt` package of `D` is imported as `<import path of D>/rt`. Imports are
  grouped as Go does: standard library, blank line, others; each group sorted.
- **C++.** A header of another package is included by its path relative to the including file's
  directory (`#include "../vocab/vocab.gen.h"`). Every output directory includes its own
  `canon_runtime.h` as `#include "canon_runtime.h"`. A name of an imported package is written
  fully qualified from the global namespace (`::sov::vocab::Element`), so no class or namespace
  of the including package can hijack it (log-2026-09-25).
- **TS.** Imports are relative paths from the importing file to the imported file, with `.ts`
  replaced by `.js`, always starting with `./` or `../`. Types are imported with `import type`.
- **Decoders across packages.** If a record or variant of package P is held by value inside a type
  that a `data`, `embedded` or `types` emit of package Q decodes from JSON, P's emit for that
  target must be in `data`, `embedded` or `types` mode too, so that it provides decoders
  (`E8018`). `sovcommon.time` (only constants and types) is therefore emitted with `mode: types`.

---

## 3. Naming

### 3.1 Word split

A Canon identifier is split into **words**:

1. Split on `_`; drop empty pieces.
2. Inside each piece, a word boundary falls between two characters when: a lowercase letter is
   followed by an uppercase letter (`minRole` → `min|Role`); a letter is followed by a digit or a
   digit by a letter (`series1` → `series|1`); an uppercase letter is followed by an uppercase
   letter that is itself followed by a lowercase letter (`HTTPServer` → `HTTP|Server`).

Examples: `II_WEA_AXE_ANGEL` → `II WEA AXE ANGEL`; `gm_junior` → `gm junior`; `Stage_1` →
`Stage 1`; `none_` → `none`; `stage1Rate` → `stage 1 Rate`.

### 3.2 Casing functions

- **Cap(w)** (C++ and TS): the first character uppercased, the rest lowercased (ASCII only).
  Digit words are unchanged.
- **GoCap(w)**: `upper(w)` if `lower(w)` is in the initialism list, else `Cap(w)`. The list is
  closed and versioned with the language: `ID URL API HTTP JSON UI DB IP HP MP TS`.
- **UpperCamel(x)** = concatenation of Cap (or GoCap) of every word.
- **lowerCamel(x)** = the first word lowercased, then UpperCamel of the remaining words.

| Canon | C++/TS UpperCamel | Go UpperCamel | lowerCamel (Go) |
|---|---|---|---|
| `id` | `Id` | `ID` | `id` |
| `minRole` | `MinRole` | `MinRole` | `minRole` |
| `apiKey` | `ApiKey` | `APIKey` | `apiKey` |
| `series_1` | `Series1` | `Series1` | `series1` |
| `II_WEA_AXE_ANGEL` | `IiWeaAxeAngel` | `IIWeaAxeAngel` | `iiWeaAxeAngel` |
| `none_` | `None` | `None` | `none` |

### 3.3 Names of generated items

`T` is a Canon type name, `f` a field, `v` a value, `m` an enum member, `c` a variant case, `fn` an
export fn, `P` = UpperCamel(last segment of the package) in each target's casing.

| Item | Go | C++ | TS |
|---|---|---|---|
| record, enum, variant, dependent type | `T` (first letter uppercased) | `T` | `T` |
| enum member | `T` + UpperCamel(m) (`ToneSeries1`) | `m` verbatim (`Tone::series_1`) | the wire string (`"series-1"`) |
| variant kind enum | `TKind` (`EventKindKind`) | `TKind` | `TKind` (union of case wires) |
| variant case type | `T` + UpperCamel(c) (`EventKindSpawnMonster`) | same | same |
| dependent-type branch enum | `TBranch` (`ParamBranch`) | `TBranch` | `TBranch` |
| table id type | `<Element>ID` (`StatusID`) | `<Element>Id` | `<Element>Id` |
| id member | `<Element>ID` + UpperCamel(key) (`StatusIDOpen`) | key verbatim (`StatusId::open`) | the key string |
| field getter | UpperCamel(f) (`MinRole()`) | `Get` + UpperCamel(f) (`GetMinRole()`) | property `f` |
| ref key getter | UpperCamel(f) + `ID` (`GroupID()`); list: `+ IDs` | `Get` + UpperCamel(f) + `Key` (`GetGroupKey()`); list: `+ Keys` | (the property is the key) |
| define value getter | UpperCamel(f) + `Value` | `Get` + UpperCamel(f) + `Value` | — |
| table entry `.id`, `.retired` | `ID()`, `Retired()` | `GetId()`, `GetRetired()` | `id`, `retired` |
| precomputed or finite-parameter method | UpperCamel(fn) (`IsStrong()`) | UpperCamel(fn) (`IsStrong()`) | property `fn` |
| package-level export fn | UpperCamel(fn) | UpperCamel(fn) | lowerCamel(fn) |
| translated method, public | method UpperCamel(fn) | method UpperCamel(fn) | `lowerCamel(T) + UpperCamel(fn)`, value first (`potionHealFor(p, hp)`) |
| translated method, pure | `lowerCamel(T) + UpperCamel(fn)` (`potionHealFor`) | `detail::T_fn` (`detail::Potion_healFor`) | `$` + the public name (`$potionHealFor`) |
| constant | UpperCamel(name) (`FarmMaxModels`) | name verbatim (`FARM_MAX_MODELS`) | name verbatim |
| container of value `v` | UpperCamel(v) (`Potions`) | UpperCamel(v) | — (a `CanonTable`) |
| value accessor (`baked`, `embedded`) | `Get` + UpperCamel(v) (`GetStatuses()`) | `Get` + UpperCamel(v) | `const v` |
| value loader (`data`) | `Load` + UpperCamel(v) (`LoadPotions`) | static `<Container>::Load` | `decode` + UpperCamel(v) |
| schema constant | UpperCamel(v) + `Schema` | `k` + UpperCamel(v) + `Schema` | v + `Schema` |
| snapshot, store | `PSnapshot`, `PStore`, `var Store` | `PSnapshot`, `PStore` | — |
| conformance entry point | `Test` + T + UpperCamel(fn) + `Conformance` | `conformance::Run` + P + `Conformance` | `test("<T>.<fn> conformance")` |
| internal access struct | — | `detail::PAccess` | — |

Parameters and locals keep their Canon names in every target (escaped, [§3.4](#34-escaping)).

### 3.4 Escaping

A generated name that is reserved in its position gets a `_` suffix:

- **Go** lowercase positions (unexported fields, parameters, locals): Go keywords, predeclared
  identifiers (`any append bool byte cap clear close comparable complex complex64 complex128
  copy delete error false float32 float64 imag int int8 int16 int32 int64 iota len make max min
  new nil panic print println real recover rune string true uint uint8 uint16 uint32 uint64
  uintptr`), the package names the generated file imports, and `self`. The imports are the Go
  packages of imported Canon packages and, of `rt json fmt iter os filepath atomic sync time
  errors strconv strings math regexp embed`, those the generated code uses (`math` only where a
  `-0.0` literal is written), so a Canon name equal to a standard package name is escaped only
  where that package is imported. A struct field `default` is `default_`; its getter is
  `Default()`. An escaped name that is itself an import (`q` and `q_` both imported) is `E8005`.
- **C++** verbatim positions (enumerators, id members, constants, parameters): C++20 keywords,
  the alternative tokens (`and and_eq bitand bitor compl not not_eq or or_eq xor xor_eq`), and
  `detail`, `conformance`, `canon`, `std`, `nlohmann`. Private members are `f_` and never collide.
  A name starting with `_` followed by an uppercase letter, or containing `__`, is `E8011`
  (a reserved name, not a collision).
- **TS** top-level bindings and parameters: the ES2020 reserved words (`await break case catch
  class const continue debugger default delete do else enum export extends false finally for
  function if import in instanceof new null return super switch this throw true try typeof var
  void while with yield`, and the strict-mode `implements interface let package private
  protected public static`), `undefined`, `NaN`, `Infinity`, `arguments`, `eval`. Properties are
  never escaped. A `@ts(name:)` override that is one of these words is `E8011` (§3.5). A TS type
  name also may not be a predefined type name (`string number boolean symbol bigint object any
  unknown never void`): such an override is `E8011`.

### 3.5 Overrides and collisions

`@go(name: "…")`, `@cpp(name: "…")` and `@ts(name: "…")` replace the whole generated name of a
field getter, type, enum member, case, value accessor, constant or export fn in that target
(`@cpp(name: "GetID")`). A `@cpp(name:)` override on a type carries into the names C++ derives
from it as `@go(name:)` does below (the branch enum `<T>Branch`, `Decode<Alias>`, the kind enum
`<T>Kind`). The argument must be an identifier that is not reserved (`E8011`); a
`@go(name:)` override names public API (§1.3), so it must also be exported (`E8011`).

A `@go(name:)` override renames exactly what the Go name plan derives from it:

- a type's (record, enum, variant, dependent type): every name §3.3 builds on `T`, the variant
  kind enum `TKind` and its members, the enum member constants, the table id type
  `<Element>ID` and its members, `Parse<E>`, `<E>Members`, `<E>FromCode` and the branch enum
  `TBranch`;
- a case's: its case type is the override, and the override replaces UpperCamel(c) in its
  `As<Case>` accessor and in its kind-enum member (`@go(name: "Spawn")` gives `Spawn`, `AsSpawn`,
  `EventKindKindSpawn`);
- a field's: its key getter (`@go(name: "Group")` gives `GroupID()`) and its `FindBy<F>`;
- a method's or value's: the key getter of a ref result;
- storage names, each lowerCamel of the effective name (the override when there is one), escaped
  (§3.4), interior-`_` suffixes kept (§6.1): a field's and a precomputed or finite method's
  storage, a value's data storage, the `FindBy<F>` index variable and a package fn's
  `<fn>Table`.

A value's container name (§5.9) is UpperCamel of its Canon name, whatever its override.

A name derived without an override that is not a valid identifier in its target (a field `_1`,
whose Go getter would be `1`; a field `__`, whose name would be empty) is `E8011` at the
declaration, as a bad override is, so `canon check` refuses what `canon build` would. A
declaration gets one `E8011`: for its override when that is invalid, else for its first invalid
derived name. A name built on a type whose override is refused (its members, id type, case types,
and its methods' generated helpers: Go pure functions and conformance tests, C++ `<Class>_<fn>`
and conformance vectors) is that type's `E8011` when the same name built on the type's default
name is valid; otherwise it is the item's own (DECISIONS 213).

`E8005` is reported when two generated names of one scope are equal after conversion and
escaping. Scopes: a Go package; a C++ namespace (all packages emitted into it that the build
loads); a TS module; the members of one class, struct or enum; a Go struct's storage and its
methods together (one selector namespace). Two items that collide in several derived names (a
getter and its storage) give one `E8005`, at the getter. Examples: `series_1` and `series1`
in one enum; fields `fooBar` and `foo_bar`; a field `strong` (`GetStrong`) next to an export fn
`getStrong` (`GetStrong`); a value `potion: [Potion] keyed by id` whose container `Potion`
collides with the record; a Go method colliding with a generated one (`ID`, `Retired`, `Kind`,
`Branch`, `As<Case>`, `Len`, `At`, `All`, `Find`, `Get`, `String`, `Wire`).

Every name a generator writes is in these scopes, fixed names included: the names of dependent types
(class or struct, branch enum and its members, `As<Branch>`, `As<Branch>Value`,
`GetBranch`/`Branch`, Go's `branch`/`value` storage and `decode<T>`, C++'s `Decode<Alias>`),
`LoadInputs`, the per-package input namespace, input slots and flags (§5.12, §7.7) and the §7.7
input helpers a package uses (`EnvText`, `IsDecDigit`, `AllDigits`, `DurationDigits`,
`Parse<Kind>Literal`, `MatchPattern`). C++ adds the scopes name lookup crosses (C++17 [basic.scope.class],
[basic.scope.declarative]): a class member equal to a namespace-scope type that the class body names
(`Tone Tone() const;` changes the meaning of `Tone`), overrides included; a parameter, or a member
of a conformance vector struct, equal to a type the same signature names (`Echo(Tone Tone, Tone
other)`); and likewise a member of a container, of the snapshot or of the `detail::<P>Access` struct
against that scope's own name and the types its body names; a nested namespace segment equal to any
name its enclosing namespace declares (a class `gen`, from `@cpp(name: "gen")`, emitted into `sov`,
and a package emitted into `sov::gen`); and a name one package's namespace declares that another
package emitted into the same namespace also declares (two packages with runtime inputs in one
namespace both declare `LoadInputs`; their §7.7 helpers, local to each `.gen.cpp`, never meet each
other but meet every other name of the namespace). Each is `E8005`: at the user's declaration when
it meets a fixed name, at the later package's `emit` when two packages meet.

`W8006` warns when a generated C++ identifier is a macro name that common platform headers define:
`min max near far IN OUT OPTIONAL ERROR DELETE TRUE FALSE VOID CONST interface small TEXT
ABSOLUTE RELATIVE TRANSPARENT OPAQUE CALLBACK WINAPI PASCAL CDECL EXPORT DOMAIN OVERFLOW UNDERFLOW
NO_ERROR IGNORE INFINITE BOOL BYTE WORD DWORD INT UINT LONG major minor unix linux i386`, and the
Win32 A/W names `GetObject GetMessage GetClassName GetCommandLine GetCurrentTime GetUserName
GetFileAttributes GetProp SetProp CreateWindow CreateFile DeleteFile CopyFile MoveFile LoadImage
LoadString DrawText SendMessage PostMessage RegisterClass`. Generated code itself writes
`(std::min)(a, b)` and `(std::max)(a, b)`, so function-like macros cannot break it.

---

## 4. Type mapping

### 4.1 Scalars

| Canon | Go | C++ getter returns | TS |
|---|---|---|---|
| `Bool` | `bool` | `bool` | `boolean` |
| `Int`, any `Int(…)` | `int64` | `int64_t` | `number` |
| `Int8` `Int16` `Int32` | `int8` `int16` `int32` | `int8_t` `int16_t` `int32_t` | `number` |
| `UInt8` `UInt16` `UInt32` | `uint8` `uint16` `uint32` | `uint8_t` `uint16_t` `uint32_t` | `number` |
| `UInt64` | `uint64` | `uint64_t` | `number`, or `bigint` with `@ts(bigint)` |
| `Float`, `Float32` | `float64`, `float32` | `double`, `float` | `number` |
| `String`, `String(…)`, asset | `string` | `const std::string&` | `string` |
| `Duration` | `time.Duration` | `std::chrono::milliseconds` | `number` (ms) |
| enum | the enum type | the enum type | union of wire strings |
| `A \| "lit"` | `string` (the wire text) | `const std::string&` | `A \| "lit"` when `A` is an enum or String, else `string` |

A refinement never changes the generated type (GEN-02): `Int(1..=100_000)` is `int64`. Only the
sized types narrow. In TypeScript an `Int` value outside `Number.MAX_SAFE_INTEGER` in an emitted
value is `E8101`, unless the field has `@ts(bigint)`. A `types`-mode emit holds no values, so it
has no `E8101`.
`@ts(bigint)` makes every integer position of its field `bigint` except a ref, which takes its
target key's form ([§5.8](#58-references)); the type, the literal and every decoder, map keys
included, follow that one decision. `E8101` covers every integer the file writes: values, consts,
ref keys, precomputed export fn results and finite-parameter lookup tables; a fn result cannot
carry `@ts(bigint)`, so its finding names the fn; a fn or value the mode already refuses (E8013,
E8014) has no `E8101` (DECISIONS 279).

### 4.2 Composite types

`K` is the key type of a map ([§5.8](#58-references) for ref keys), `E` the element's getter type.

| Canon | Go getter returns | C++ getter returns | TS property |
|---|---|---|---|
| record `R` | `*R` | `const R&` | `R` |
| variant `V` | `*V` | `const V&` | `V` |
| dependent type `D(…)` | `*D` | `const D&` | `D` |
| `[T]` (scalar or enum) | `rt.List[T]` | `const std::vector<T>&` | `ReadonlyArray<T>` |
| `[R]` (record or variant) | `rt.List[*R]` | `const std::vector<R>&` | `ReadonlyArray<R>` |
| `[T] keyed by f` | `rt.KeyedList[K, R]` | `const canon::KeyedList<K, R>&` | `ReadonlyArray<R>` |
| `table T` (as a field) | `rt.KeyedList[<id type>, T]` | `const canon::KeyedList<<id type>, T>&` | `ReadonlyArray<T>` |
| `{K: V}` | `rt.Map[K, E]` | `const canon::FlatMap<K, V>&` | `ReadonlyMap<K, V>` |
| `{e in t: R(e)}` | `rt.Map[<key of t>, *R]` | `const canon::FlatMap<<key of t>, R>&` | `ReadonlyMap<<key of t>, R>` |
| `{D(e): V}` (dependent key) | `rt.Map[string, E]` | `const canon::FlatMap<std::string, V>&` | `ReadonlyMap<string, V>` |
| `ref T` | [§5.8](#58-references) | [§5.8](#58-references) | the key |

Lists of lists nest: `[[Reward]]` is `rt.List[rt.List[*Reward]]`, `const
std::vector<std::vector<Reward>>&`, `ReadonlyArray<ReadonlyArray<Reward>>`.

A `@json(pairs:)` field (WIRE.md §5.14, DECISIONS 21) is an ordinary list of records in every
target: `stats: [StatBonus](..=6) @json(pairs: …)` is `Stats() rt.List[*StatBonus]`,
`const std::vector<StatBonus>& GetStats() const`, `readonly stats: ReadonlyArray<StatBonus>`.
Only loaders and decoders know the slot keys: they read slot 0 to `N − 1` and stop at the first
empty one (WIRE.md's rules, which `canon build` already enforced). A `pairs` field cannot map onto
a legacy struct in this version (`E8109`, [§7.8](#78-legacy-structs)).

### 4.3 Optional values

| Canon `T?` | Go | C++ | TS |
|---|---|---|---|
| scalar, enum, `Duration` | `(T, bool)` | `std::optional<T>` | `T \| null` |
| `String` | `(string, bool)` | `const std::string*` | `string \| null` |
| record, variant, dependent type | `*T` (nil) | `const T*` (nullptr) | `T \| null` |
| list | `(rt.List[E], bool)` | `const std::vector<T>*` | `ReadonlyArray<T> \| null` |
| map | `(rt.Map[K, E], bool)` | `const canon::FlatMap<K, V>*` | `ReadonlyMap<K, V> \| null` |
| keyed list | `(rt.KeyedList[K, R], bool)` | `const canon::KeyedList<K, R>*` | `ReadonlyArray<R> \| null` |
| `ref T` | [§5.8](#58-references) | [§5.8](#58-references) | key `\| null` |

### 4.4 Types that are not emitted

`Range`, `Pair` (TYPES §12.5), function types, `_` and a non-optional `Never` have no
representation. An emitted type or value that contains one is `E8012`. A constant of a
variant's kind is `E8019` (DECISIONS 292). An optional `Never?` field is not emitted at all. The
`Define` record of `load.defines`, and so a define table (`table Define`), has no `canon-fp` form
(FINGERPRINT.md §8), so a `data` or `embedded` emit, whose loader checks the fingerprint, cannot
carry one: a type or value of such an emit that contains one is `E8012` (for `emit json`, WIRE.md's
`E8151`). A `ref` into a define table is a key and is emitted.
A parameterized record `R(p)` is emitted once, erased (`HourlyTarget`); its dependent field types
follow [§5.6](#56-dependent-types). A plain alias (`type Layout = String(/…/)`) has no name in
generated code: every use is emitted as its definition (SPEC §4).

---

## 5. Constructs

The samples are excerpts; complete generated files are in [§10](#10-goldens).

### 5.1 Constants

| Canon | Go | C++ | TS |
|---|---|---|---|
| `const FARM_MAX_MODELS = 100` | `const FarmMaxModels int64 = 100` | `inline constexpr int64_t FARM_MAX_MODELS = 100;` | `export const FARM_MAX_MODELS = 100;` |
| `String` | `const X = "…"` | `inline constexpr std::string_view X = "…";` | `export const X = "…";` |
| `Duration` | `const X = 90000 * time.Millisecond` | `inline constexpr std::chrono::milliseconds X{90000};` | `export const X = 90000;` |
| enum member | `const X = ToneInfo` | `inline constexpr Tone X = Tone::info;` | `export const X: Tone = "info";` |
| list or map (TYP-14) | `func X() rt.List[int64]` | `inline const std::vector<int64_t> X = {…};` | frozen `export const` |

### 5.2 Enums

```
enum Tone { warning, info, series_1 = "series-1" }
enum Element @codes(UInt8) { FIRE = 1, WATER = 2 }
```

**Go**

```go
type Tone uint8

const (
	ToneWarning Tone = 0
	ToneInfo    Tone = 1
	ToneSeries1 Tone = 2
)

func (self Tone) String() string        // Canon name: "series_1"
func (self Tone) Wire() string          // wire value: "series-1"
func ParseTone(wire string) (Tone, bool)
func ToneMembers() iter.Seq[Tone]       // declaration order, retired members included

type Element uint8                      // values are the codes: ElementFire Element = 1
func (self Element) Code() uint8
func ElementFromCode(code uint8) (Element, bool)
```

**C++**

```cpp
enum class Tone : uint8_t { warning, info, series_1 };
inline constexpr std::array<Tone, 3> kToneMembers = {Tone::warning, Tone::info, Tone::series_1};
std::string_view ToName(Tone v);                       // "series_1"
std::string_view ToWire(Tone v);                       // "series-1"
std::optional<Tone> ToneFromWire(std::string_view wire);

enum class Element : uint8_t { FIRE = 1, WATER = 2 };  // the underlying value is the code
std::optional<Element> ElementFromCode(uint8_t code);  // static_cast<uint8_t>(e) is the code
```

`ToName`, `ToWire` and `<E>FromWire` are `inline` in the header. `ToName`/`ToWire` are overloads
in the package namespace; `FromWire` carries the enum name because C++ cannot overload on the
return type (this replaces SPEC §15.3's `FromWire`).

**TS**

```ts
export type Tone = "warning" | "info" | "series-1";
export const ToneMembers: ReadonlyArray<Tone> = Object.freeze(["warning", "info", "series-1"]);
export const ToneNames: Readonly<Record<Tone, string>> = Object.freeze({ warning: "warning", info: "info", "series-1": "series_1" });
export const ToneIndex: Readonly<Record<Tone, number>> = Object.freeze({ warning: 0, info: 1, "series-1": 2 });
export const ElementCodes: Readonly<Record<Element, number>> = Object.freeze({ FIRE: 1, WATER: 2 });  // @codes only
```

Rules (GO-05, CPP-04, WIR-04):

- The underlying type is the `@codes` type, else the smallest of `uint8`, `uint16`, `uint32` that
  holds every index. Without `@codes`, the value is the declaration index (`.index`).
- With `@codes`, `<E>FromCode` exists and the value of each constant is its code.
- `ordered` enums compare with the native operators. `ordered` with `@codes` whose codes do not
  increase in declaration order is `E8010`, because generated `<` compares codes.
- Retired members stay in every target (SPEC §12), with the doc line `Retired.` added. The kind
  member of a retired variant case gets the same line (§5.5).

### 5.3 Table ids

The keys of a public `table` value (EMT-04):

| Mode | Go | C++ | TS |
|---|---|---|---|
| `baked`, `embedded` | `type StatusID uint8` + constants `StatusIDOpen…` in entry order, `String()` (the key), `ParseStatusID(key) (StatusID, bool)` | `enum class StatusId : uint8_t { open, taken, … }`, inline `ToWire(StatusId)`, `StatusIdFromWire(std::string_view)` (binary search, in `.gen.cpp`) | `export type StatusId = "open" \| "taken" \| …` and `StatusIdIndex: Readonly<Record<StatusId, number>>` |
| `data`, `types` | `type StatusID string` | `std::string` (getter `const std::string&`, lookups take `std::string_view`) | `export type StatusId = string` |

Keyed lists have no id type: their key is the key field, with the field's own type. Retired
entries keep their id (LCK-05) and are flagged by the **retired getter** of their record: Go
`Retired() bool`, C++ `bool GetRetired() const`, TS `readonly retired: boolean` (LOCK.md §7).
Keyed lists have no retirement, so their elements have no retired getter.

### 5.4 Records

```go
// Status: One point of the post lifecycle.
type Status struct { /* unexported fields, reference layout */ }

func (self *Status) ID() StatusID            // table entries only
func (self *Status) Retired() bool           // table entries only
func (self *Status) Label() string
func (self *Status) Next() rt.List[*Status]  // resolved refs (§5.8)
func (self *Status) NextIDs() rt.List[StatusID]
func (self *Status) By() (Actor, bool)       // optional enum
```

```cpp
class Status {
public:
    StatusId GetId() const;                                // table entries only
    bool GetRetired() const;                               // table entries only
    const std::string& GetLabel() const;
    const std::vector<const Status*>& GetNext() const;     // resolved refs (§5.8)
    const std::vector<StatusId>& GetNextKeys() const;
    std::optional<Actor> GetBy() const;
private:
    friend struct detail::TeamboardAccess;
    // members: <field>_ , reference layout
};
```

```ts
export interface Status {
  readonly id: StatusId;          // table entries only
  readonly retired: boolean;      // table entries only
  readonly label: string;
  readonly next: ReadonlyArray<StatusId>;
  readonly by: Actor | null;
}
```

In TypeScript, a record that is a table row and also a plain value elsewhere (a field, a list
element, a `let`) declares `id` and `retired` optional (`readonly id?: StatusId`); they are present
on table entries and absent elsewhere, and a decoder reads `$id` only in table position
(DECISIONS 278).

- Every field has exactly one getter (two for refs, [§5.8](#58-references)); there are no
  setters and no public members (DECISIONS 4).
- C++ getters are defined inline in the class; they return scalars and enums by value and
  everything else by `const&` or `const T*` (DECISIONS 14). Records are held by value.
- **C++ default constructors are public** (implicitly declared). A default-constructed value holds
  zeros and defaults and can never be modified, so it cannot forge configuration; making them
  private would forbid holding generated classes by value in other generated classes. Every class
  befriends `detail::<P>Access`, the struct (defined only in `<last>.gen.cpp`) that builds values,
  and, in JSON modes, its `detail::Decode` overload ([§7.2](#72-classes)). Go has the same
  limitation (GO-03): `potions.Potion{}` compiles and is empty.
- Precomputed export fns (no parameter besides `self`) are getters named after the fn, in field
  order after the fields: Go `IsStrong() bool`, C++ `bool IsStrong() const`, TS `readonly
  isStrong: boolean`.

### 5.5 Variants

```
variant EventKind @json(tag: "type") {
  spawn_monster { monsterId: ref monsters, spawnRegion: Rect, … }
  spawn_item { … }
  monster_drop_inject { … }
}
```

| | Go | C++ | TS |
|---|---|---|---|
| kind enum | `type EventKindKind uint8` with `EventKindKindSpawnMonster…`, `String()`, `Wire()`, `ParseEventKindKind` | `enum class EventKindKind : uint8_t { spawn_monster, … }`, `ToName`, `ToWire`, `EventKindKindFromWire` | `type EventKindKind = "spawn_monster" \| …` |
| case with fields | `type EventKindSpawnMonster struct`, with getters | `class EventKindSpawnMonster`, with getters | `interface EventKindSpawnMonster { readonly kind: "spawn_monster"; … }` |
| variant | `type EventKind struct`; `Kind() EventKindKind`; `AsSpawnMonster() (*EventKindSpawnMonster, bool)` per case with fields | `class EventKind`; `EventKindKind GetKind() const`; `const EventKindSpawnMonster* AsSpawnMonster() const` per case with fields | `type EventKind = EventKindSpawnMonster \| …`; a case without fields is `{ readonly kind: "nothing" }` |

- `As<Case>` returns the case (nil or nullptr unless `Kind()` is that case). A case without fields
  has no `As` method; test `Kind()`.
- A default-constructed C++ variant is the first case with default fields.
- A case's doc goes on its case type. A case without fields has no type, so its doc goes on its
  kind-enum member, followed by `Retired.` when the case is retired; the kind member of any other
  case carries only `Retired.`, when retired.
- `@json(inline)` changes only the wire, never the generated API.
- TS uses the **wire** name of the case as the `kind` discriminant.

### 5.6 Dependent types

`type Param(e: EventType) = match e.param { monster => ref monsters, item => ref items, element
=> Element, game_mode => String, none_, stat => Never }` (DEP-03):

- The union is emitted once, named after the alias (`Param`), with a **branch enum** `ParamBranch`
  whose members are the first pattern of each non-`Never` arm, in arm order; a wildcard `_ =>` arm
  is named after the first member it covers, in the discriminant enum's declaration order. (The
  name is `<Alias>Branch`, not `<Alias>Kind` as DEP-03 proposed: `ParamKind` is the natural name
  of the discriminating enum and already exists in `resource.vocab`.)
- Each branch has an accessor. A `ref` branch exposes the key. A branch that is a ref into a
  `load.defines` table is E8019 `DependentType` in every generator (no `As<Branch>Value` is
  written; `monster` below stands for a ref into an ordinary collection). A branch whose type is
  not a scalar, `String`, enum or `ref` is `E8017`.

```go
type ParamBranch uint8 // ParamBranchMonster, ParamBranchItem, ParamBranchElement, ParamBranchGameMode
type Param struct{ /* … */ }
func (self *Param) Branch() ParamBranch
func (self *Param) AsMonster() (string, bool)       // ref monsters: the key
func (self *Param) AsItem() (string, bool)
func (self *Param) AsElement() (Element, bool)
func (self *Param) AsGameMode() (string, bool)
```

```cpp
enum class ParamBranch : uint8_t { monster, item, element, game_mode };
class Param {
public:
    ParamBranch GetBranch() const;
    const std::string* AsMonster() const;           // nullptr unless GetBranch() == monster
    const std::string* AsItem() const;
    std::optional<Element> AsElement() const;
    const std::string* AsGameMode() const;
};
```

```ts
export type ParamBranch = "monster" | "item" | "element" | "game_mode";
export type Param =
  | { readonly branch: "monster"; readonly value: string }
  | { readonly branch: "item"; readonly value: string }
  | { readonly branch: "element"; readonly value: Element }
  | { readonly branch: "game_mode"; readonly value: string };
```

The wire is untagged; loaders pick the branch from the discriminant through a static table
generated from the `match` (the discriminant is read first). Dependent maps and keys follow
§4.2 (DEP-04). A loader reads the discriminant down the argument's then the match's path from what
it has already decoded: earlier required fields of the class, then fields of records held by
value, none a `ref` or optional; a `Bool` discriminant's cases are `false` and `true`. A list of
dependent values shares its field's discriminant. A discriminant member no branch covers (a `Never` arm, WIRE.md §5.9) refuses
the value with the loader text `no branch for this value` at its pointer, in every loader. Baked Go writes a dependent value (a field's, or
its list's elements) as `&T{branch, value}` in the branch the record's own fields select, the value
typed as `As<Branch>` returns it. What a generator does not write is E8019 `DependentType` at
stage E: a discriminant read through a `ref` or a record parameter, a dependent value in a map or
in a literal union a loader reads, another package's dependent type in a Go loader or baked Go
literal, and a dependent type every arm of which is `Never`.

- The Go struct stores `branch` and `value` (unexported); the Go branch enum has no `String`,
  `Wire` or `Parse<…>` (it is never on the wire). A Go data-mode loader writes `decode<T>`
  (§6.1) only for the dependent types a decoded class holds (through lists and optionals); C++
  writes `Decode<Alias>`. The C++ branch enum likewise has no `k<E>Members`, `<E>FromWire` or
  `ToName`/`ToWire` overload. Every name here comes from the name plan and collides under §3.5.

### 5.7 Parameterized records

`record HourlyTarget(e: EventType) { … }` is emitted as one class `HourlyTarget`, without the
parameter. Fields whose type depends on the parameter use §5.6 and §4.2.

### 5.8 References

A `ref` has a **key** (the key of the target entry) and, sometimes, a **resolved** getter that
returns the entry itself (CG-03, RES-03). A field `f: ref T` gets:

| The target is | Resolved getter | Key getter |
|---|---|---|
| in the same value, or another `@reload` value of the same snapshot | yes | yes |
| in `baked` or `embedded` mode, any public value of the same package and emit | yes | yes |
| any other value, a value of another package, a `local` collection | no | yes |
| a collection-typed field of an enclosing record (RES-03 case 1) | no | yes |
| a `load.defines` table | no | yes, plus the define value getter |

Resolved getters return what a record getter returns (Go `*T`, C++ `const T&`); for `ref T?`,
Go `*T` (nil), C++ `const T*`; for `[ref T]`, Go `rt.List[*T]`, C++ `const std::vector<const T*>&`.
Refs inside maps (keys or values) and inside nested lists are keys only.

The **key type** is the target collection's id type in the target package's emit for the same
target ([§5.3](#53-table-ids)), the key field's type for a keyed list, the field's `<id type>` for a
table field ([§4.2](#42-composite-types), DECISIONS 288), and `String` for `local` and define tables. Key getters return: Go `K`, `(K, bool)` for `ref T?`, `rt.List[K]` for
`[ref T]`; C++ `K` by value when it is an enum, `const std::string&` / `const std::string*` /
`const std::vector<K>&` otherwise. TypeScript properties hold the key only; resolution is
`container.find(key)`.

The **define value getter** (`XxxValue()`, `GetXxxValue()`) returns the integer value of the
define as `int64`/`int64_t`. Define values are compile-time facts of the runtime (the header is
compiled into it), so every Go and C++ emit carries a baked, sorted `(name, value)` table holding
every define of each define table a field of the emit's own classes refs (whether or not a value
uses it), and loaders resolve the value right after reading the key. A key missing from
that table (data built with a newer header than the binary) is a load error.

- **The table.** One per define table the emit's refs use, named from the table's `let`
  (`UpperCamel(let)`, [§3.3](#33-names-of-generated-items)): Go an unexported package variable
  `defines<Table>` of `[]struct{ name string; value int64 }`; C++ `detail::k<Table>Defines`, a
  `constexpr std::array<std::pair<std::string_view, int64_t>, N>`. Entries are sorted by the
  define name's bytes and looked up by binary search. The names join the package's generated
  names ([§3.5](#35-overrides-and-collisions)): a colliding user name is `E8005`. A baked emit
  writes the table too and reads no key from it at run time. `<Table>` in the load error below is
  C++'s UpperCamel of the `let` in both loaders ([§3.2](#32-casing-functions)); Go's variable name
  `defines<Table>` uses Go's UpperCamel.
- **Getters.** For `f: ref D` (D a define table): the key getter as above (`String`), plus the
  value getter — Go `FValue() int64`, C++ `int64_t GetFValue() const`. For `ref D?`: Go
  `FValue() (int64, bool)`, C++ `std::optional<int64_t> GetFValue() const`. For `[ref D]`: Go
  `FValues() rt.List[int64]`, C++ `const std::vector<int64_t>& GetFValues() const`, in the list's
  order; for `[ref D]?`, Go `FValues() (rt.List[int64], bool)`, C++ `const std::vector<int64_t>*
  GetFValues() const` ([§4.3](#43-optional-values)). Value getters exist only for fields (a pairs
  record's fields included); a value, constant or `export fn` result that is a ref into a define
  table exposes its key only. Refs to defines inside maps and nested lists are keys only (as every ref there).
- **Loading** (data, embedded and types modes). The loader looks the key up when it reads it and
  stores the value; a key missing from the table is the load error
  `<pointer>: define <name> is not in this program's <Table> table` (its text is part of this
  document, like every loader text). Go and C++ read the same keys, accept and refuse the same
  data, and report the same texts.

### 5.9 Values: containers and accessors

For each emitted value `v` of type `X`, the **container** is:

- a table or keyed list: a generated class `UpperCamel(v)` (`Potions`, `Statuses`);
- a record or variant: the type itself (CG-04); in a `data` emit of Go or C++, a value whose
  record (or whose rows' record) belongs to another package has no container here and is `E8019`
  ForeignDataRecord (DECISIONS 291);
- anything else (`baked` only, [§2.2](#22-what-each-mode-contains)): no container; the accessor
  returns what a field getter of type `X` returns.

**Table and keyed-list containers**

```go
func (self *Statuses) Len() int
func (self *Statuses) At(i int) *Status                 // source order
func (self *Statuses) All() iter.Seq[*Status]           // source order, retired entries included
func (self *Statuses) Find(key string) (*Status, bool)  // key type for a keyed list
func (self *Statuses) Get(id StatusID) *Status          // baked/embedded tables only
func (self *Statuses) FindByCode(code uint16) (*Status, bool) // per @stable field `code`, its Go type
```

```cpp
size_t Len() const;
const Status& At(size_t i) const;
const std::vector<Status>& All() const;
const Status* Find(std::string_view key) const;          // binary search, never allocates
const Status& Get(StatusId id) const;                    // baked/embedded tables only
const Status* FindByCode(uint16_t code) const;           // per @stable field, its C++ type
```

```ts
export interface CanonTable<K, T> { readonly length: number; readonly all: ReadonlyArray<T>; at(i: number): T; find(key: K): T | null; }
```

A `@stable` field `f` gets `FindBy` + UpperCamel(f). `Find` on a keyed list takes the key field's
type (`string`/`std::string_view`, `int64`/`int64_t`). In Go, `FindBy<F>` reads a
`map[<type>]int` index from the value to the entry's position, built once, never a scan.

**By mode**

| Mode | Go | C++ | TS |
|---|---|---|---|
| `baked` | `func GetStatuses() *Statuses`, `func GetDeck() *Deck`, `func GetAssigneeMinRole() roles.Role` | `const Statuses& GetStatuses();` `const Deck& GetDeck();` `sovcommon::roles::Role GetAssigneeMinRole();` | `export const statuses: CanonTable<StatusId, Status>`, `export const deck: Deck`, `export const assigneeMinRole: Role` |
| `embedded` | same accessors; the data is `//go:embed <v>.json`, decoded on first use | same accessors; the data file is a `static const unsigned char[]` in `.gen.cpp`, decoded on first use | as `baked` |
| `data` | `func LoadPotions(path string) (*Potions, error)` | `static std::shared_ptr<const Potions> Potions::Load(const std::string& path, std::string& error);` | `export function decodePotions(json: unknown): CanonTable<string, Potion>` |

- `baked` data (CPP-08, GO-04): C++ builds every value of the package once, inside one
  function-local `static const` object of `detail::<P>Access` in `.gen.cpp` (thread-safe, no
  static-initialization-order problem); accessors return references into it. Go builds them in one
  unexported function run once through `sync.OnceValue`; accessors return pointers into it. There is
  no `init()` side effect except input reading ([§5.12](#512-runtime-inputs)). A value of type
  `ref T` gets a resolved accessor when §5.8 allows it (`GetInitialStatus()`) and always a key
  accessor (`GetInitialStatusID()` in Go, `GetInitialStatusKey()` in C++).
- `embedded`: a decode failure is a build defect. It is signalled with `E8301` (C++
  `canon::OnEvalError`, Go panic with `*rt.EvalError`).
- `data`: loaders read one file, check `$schema`, fill every field and never validate (SPEC
  §14.5). Refs inside the file are resolved at load. `@reload` values have no public per-value
  loader: they load through their snapshot ([§5.11](#511-reloadable-values-snapshot-and-store)).

### 5.10 Exported functions

**Precomputed** (no parameter): a getter (§5.4) or, at package level, a function with no
parameters returning the value (Go `func Foo() T`, C++ `T Foo()`, TS `export function foo(): T`).
In `data` mode, precomputed methods read their `$<fn>` key (WIR-09).

**Lookup** (every parameter finite: `Bool`, enum, `ref` into a table) (CG-08). A `ref` into a keyed
list is not finite (a keyed list has no id enum to index by): a function taking one is translated,
and a translated function takes no `ref`, so it is `E9006` (CONFORMANCE.md §2.1):

```go
func CanTransition(from StatusID, to StatusID) bool
func ColumnOf(s StatusID) *Column
func AreasVisibleTo(role roles.Role) rt.List[*Area]
```

```cpp
bool CanTransition(StatusId from, StatusId to);
const Column* ColumnOf(StatusId s);                     // ref Column?: nullptr for none (§5.8)
const std::vector<const Area*>& AreasVisibleTo(sovcommon::roles::Role role);
```

```ts
export function canTransition(from: StatusId, to: StatusId): boolean;
export function areasVisibleTo(role: Role): ReadonlyArray<AreaId>;
```

- **Domain order.** Each parameter's domain is enumerated in a fixed order: enum members in
  declaration order, retired members included; table entries in entry order, retired entries
  included; `Bool` as `false` then `true`. The result table is dense, indexed by these ordinals,
  row-major in parameter order (the first parameter varies slowest). EVALUATION.md §2.3 (stage E)
  computes the cells in exactly this order, and WIRE.md §5.11 writes `$` keys and `$fns` in it.
  More than 65 536 cells is `E9002`; an optional parameter is `E9003`.
- Results are returned like field getters of the return type; list results are read-only views.
- A method with finite parameters stores one small table per entry. In `data` mode it is read
  from `"$<fn>": {<wire of arg>: result}` (WIR-09); only enum and `Bool` parameters are
  supported there (`E8013`). In TypeScript it is a property named after the fn holding that same
  nested object, frozen: `area.canPost["maintainer"]` (`Readonly<Record<Role, boolean>>`; a `Bool`
  argument is keyed `"true"`/`"false"`).
- Package-level export fns (precomputed or lookup) need `baked` or `embedded` mode: `E8013` in
  `data`, `E8014` in `types`.

**Translated** (at least one runtime parameter): the body is emitted once as a **pure function**
whose parameters are the paths of `self` the body reads (named as in CONFORMANCE.md §2.3:
`heal`, `startUtc_hour`) followed by the declared parameters. The
public method calls it; the conformance test calls it too (SPEC §9.4).

```go
func (self *Potion) HealFor(missingHp int64) int64 { return potionHealFor(self.heal, missingHp) }
func potionHealFor(heal int64, missingHp int64) int64 { return min(heal, max(missingHp, 0)) }
```

A package-level translated fn is public and pure at once (`func Foo(...)`, `Foo(...)`,
`export function foo(...)`). Parameter and return types, the checks made on entry and exit, and
the checked helpers used for every operation are in CONFORMANCE.md §2–§3. Go `time.Duration`
arguments are converted with `rt.DurationToMs`; results with `rt.DurationFromMs`.

### 5.11 Reloadable values: snapshot and store

All `@reload` values of a package's Go or C++ emit form **one snapshot** (RLD-01, SPEC §15.5):

```cpp
class PipelineSnapshot {
public:
    static std::shared_ptr<const PipelineSnapshot> Load(const std::string& dir, std::string& error);
    const Potions& GetPotions() const;          // one getter per @reload value
};
class PipelineStore {
public:
    static std::shared_ptr<const PipelineSnapshot> Current();  // atomic load; nullptr before the first Reload
    static bool Reload(const std::string& dir, std::string& error);
};
```

```go
type PipelineSnapshot struct{ /* … */ }
func (self *PipelineSnapshot) Potions() *Potions
func LoadPipelineSnapshot(dir string) (*PipelineSnapshot, error)
type PipelineStore struct{ /* atomic.Pointer[PipelineSnapshot] */ }
func (self *PipelineStore) Current() *PipelineSnapshot  // nil before the first Reload
func (self *PipelineStore) Reload(dir string) error
var Store PipelineStore
```

- `Load(dir)` reads, for each `@reload` value, `<dir>/<name>`, where `<name>` is the file name
  under which the package's `emit json` writes that value (`<value>.json`, WIRE.md §8.1). A
  `@reload` value that no `emit json` of the package writes is WIRE.md's `E8153`.
- `Reload` builds the new snapshot beside the old one, then swaps it atomically
  (`canon::AtomicSharedPtr`, which uses `std::atomic<std::shared_ptr>` where available and the
  C++17 `std::atomic_load`/`std::atomic_store` otherwise; Go `atomic.Pointer`). On failure the
  current snapshot stays and the error is returned. The first load is also a `Reload`.
- References between values of one snapshot are resolved inside it; references to values outside
  it are keys (§5.8).
- `@reload` on a value emitted by a Go or C++ emit in `baked`, `embedded` or `types` mode is
  `E8202` (RLD-02). `@reload` on data whose type maps onto a legacy C++ struct in `fields` or
  `both` mode is `E8201`. TypeScript has no store in v0: `decode<V>` is all it offers.

### 5.12 Runtime inputs

Inputs are allowed only in records reachable from a single non-collection public value (LAY-04,
`E1903`, EVALUATION.md §11.1). Each input field has one runtime slot in the package.

```go
func LoadInputs() error                    // reads every input of the package; errors.Join of all failures
func (self *Gen) APIKey() (string, bool)   // input getter; panics with *rt.EvalError E8302 before LoadInputs
```

```cpp
bool LoadInputs(std::string& error);        // false: `error` lists every failure, one per line
const std::string* GetApiKey() const;       // before LoadInputs: canon::OnEvalError("E8302", …), then none
```

- `LoadInputs` must be called once at startup, before any getter of an input and before other
  threads read configuration. Calling it again re-reads the environment.
- An unset **or empty** variable is unset: an optional input becomes `none`; a required one is an
  error naming the variable.
- The text is read exactly as EVALUATION.md §11.3 says: integers as decimal digits with an
  optional leading `-` (no separator, no `0x`); `Float` as
  `-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?`, finite; `Bool` as `true`/`false`; `Duration` as a Canon
  duration literal (`1h30m`), optionally prefixed by `-`; `String` byte for byte, valid UTF-8; an
  enum by a member's wire value, exactly, retired members refused. `rt.ParseIntLiteral` and its
  siblings ([§6.3](#63-the-rt-package)) and the C++ template of [§7.7](#77-runtime-inputs)
  implement it.
- The field's own refinement is checked (EVALUATION.md §11.3): the implicit range of a sized type
  (a `Duration` beyond ±`DurationLimit` is not a valid literal), ranges, length in bytes, then every distinct pattern of its alias chain (TYPES.md §7.4; a repeated
  pattern is kept once), innermost first (alias-chain order), with search semantics (Go `regexp.MatchString`, C++ `MatchPattern`, §7.7; patterns are limited to the portable subset by
  `E1904`). The C++ loader never uses `std::regex`: ir compiles the pattern to its Thompson
  automaton over code points (the states of Go's own compiled program, `regexp/syntax`
  `Simplify` then `Compile`, reachable from its start, captures and no-ops dropped), which the
  `.gen.cpp` holds as a constant table `kPattern` (`kPattern2`, … for further patterns) in the
  variable's block; `MatchPattern` searches
  it with one set of states per position, iteratively, in memory proportional to the table and
  time linear in the text, decoding UTF-8 as Go does, so it accepts exactly the texts
  `regexp.MatchString` accepts, at any length. Go's `regexp` size limits, which `check` applies
  when it compiles the pattern, bound the table. `where` is never checked at runtime. A failed check is
  an error naming the variable.
- A TS emit whose package declares an input field is `E8104`.

### 5.13 `types`-mode decoders

`types` mode (EMT-05) emits the read-only types plus a public decoder per public record and
variant, for runtimes that keep reading their legacy file:

```go
func DecodeEventConfig(raw []byte) (*EventConfig, error)
```

```cpp
static std::optional<EventConfig> EventConfig::Decode(const nlohmann::json& v, std::string& error);
```

```ts
export function decodeEventConfig(json: unknown): EventConfig;
```

- The C++ decoder takes an already-parsed `nlohmann::json`, whose objects are ordered by key: a
  nested table it decodes holds its entries in key byte order, not file order (the `data`-mode
  loader, which reads the text, keeps file order; DECISIONS 290).
- A decoder reads the JSON that the `load` reads at that position (after `at:`), with WIRE.md's
  decode rules for sources: absent means the default, `null` or the `@json(none:)` value means
  `none`, units, `path`, `pairs`, tags and `inline` are applied. Unknown keys are ignored (the file
  may be read with `partial: true`). There is no `$schema`.
- **Invalid input.** A decoder fails on anything it cannot represent: invalid JSON, a wrong JSON
  kind, a missing required key, an unknown enum member or case tag, a number that does not fit the
  generated type (a sized integer's range, the `Duration` range, a duration that is not a whole
  number of milliseconds, a fraction for an integer), an incomplete or non-contiguous `pairs`
  slot. It then returns **no value** and an error naming the JSON path of the first failure: Go
  `nil` and an `error`; C++ `std::nullopt` with `error` set; TS throws an `Error`. It never returns
  a partly filled value. What it can represent it accepts without checking refinements, refs, keys
  or checks: the file is the one `canon check` validated (WIRE.md §5.13).
- Defaults must be constant; a computed default (one that reads other fields, or the instance through
  its type's arguments, DECISIONS 282) in a type emitted in `types` mode is `E8014`, as
  are precomputed export fns and finite-parameter methods (there is no precomputed data to read).
- Refs are keys; table ids are strings.

---

## 6. Go

### 6.1 Files and package

- `<gopkg>.gen.go` holds everything except the conformance test. Its package doc is the one line
  of §2.5.
- Receivers are named `self` (Canon reserves `self`, so it never collides with a parameter).
  Getters use pointer receivers; enums use value receivers.
- Field storage: unexported, lowerCamel of the field's effective name (its `@go(name:)` override
  when it has one, else its Canon name), escaped (§3.4). Storage the generator adds beside a
  field (a presence flag, a ref's keys) carries an interior `_` (`hint_ok`, `next_ids`), which
  lowerCamel never produces, so it cannot collide with a field. Records are held by value inside
  containers and by pointer where a getter returns `*T`.
- Loaders use `encoding/json` with generated wire structs whose fields are pointers, so a missing
  key is reported (`rt.Missing`) instead of silently read as zero.

### 6.2 Sample: `baked` table with lookups (teamboard, abridged)

```go
type StatusID uint8

const (
	StatusIDOpen  StatusID = 0
	StatusIDTaken StatusID = 1
)

func (self StatusID) String() string
func ParseStatusID(key string) (StatusID, bool)

type Statuses struct{ rows []Status }

func (self *Statuses) Get(id StatusID) *Status { return &self.rows[id] }

type teamboardData struct {
	statuses      Statuses
	initialStatus *Status
}

var teamboardValues = sync.OnceValue(buildTeamboard) // builds rows, then links refs

func GetStatuses() *Statuses       { return &teamboardValues().statuses }
func GetInitialStatus() *Status    { return teamboardValues().initialStatus }
func GetInitialStatusID() StatusID { return teamboardValues().initialStatus.id }

var canTransitionTable = [6][6]bool{ /* row = from, column = to (DECISIONS 122) */ }

// CanTransition: sovcommon's CanTransition.
func CanTransition(from StatusID, to StatusID) bool {
	return canTransitionTable[from][to]
}
```

### 6.3 The `rt` package

Written to `<out>/rt/rt.go` for every Go emit (CG-09), whether or not the package uses it. It is
the only place generated Go code gets helpers from.

```go
// Code generated by canon: Go runtime rt v1. DO NOT EDIT.

// Package rt is the runtime of the Go code canon generates.
package rt

import (
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Version is the version of this runtime. Generated code of one output directory
// always uses the rt package written next to it.
const Version = 1

// ---------------------------------------------------------------------------
// Read-only collections
// ---------------------------------------------------------------------------

// List is a read-only list. The zero value is an empty list.
type List[T any] struct{ s []T }

// MakeList is for generated code. The list keeps s: the caller must not modify it.
func MakeList[T any](s []T) List[T] { return List[T]{s: s} }

// Len returns the number of elements.
func (l List[T]) Len() int { return len(l.s) }

// At returns element i. It panics if i is out of range, like a slice.
func (l List[T]) At(i int) T { return l.s[i] }

// All yields the elements in order.
func (l List[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range l.s {
			if !yield(v) {
				return
			}
		}
	}
}

// Clone returns a new slice holding the elements, which the caller may modify.
func (l List[T]) Clone() []T { return append([]T(nil), l.s...) }

// Map is a read-only map that iterates in source order. The zero value is empty.
type Map[K comparable, V any] struct {
	keys  []K
	vals  []V
	index map[K]int
}

// MakeMap is for generated code: keys are unique, vals[i] belongs to keys[i]. The map
// keeps both slices: the caller must not modify them.
func MakeMap[K comparable, V any](keys []K, vals []V) Map[K, V] {
	index := make(map[K]int, len(keys))
	for i, k := range keys {
		index[k] = i
	}
	return Map[K, V]{keys: keys, vals: vals, index: index}
}

// Len returns the number of entries.
func (m Map[K, V]) Len() int { return len(m.keys) }

// Get returns the value of k, and whether k is present.
func (m Map[K, V]) Get(k K) (V, bool) {
	i, ok := m.index[k]
	if !ok {
		var zero V
		return zero, false
	}
	return m.vals[i], true
}

// All yields the entries in source order.
func (m Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i, k := range m.keys {
			if !yield(k, m.vals[i]) {
				return
			}
		}
	}
}

// KeyedList is a read-only list of records that are unique by a key.
type KeyedList[K comparable, T any] struct {
	rows  []T
	index map[K]int
}

// MakeKeyedList is for generated code: keys[i] is the key of rows[i], keys are unique.
// The list keeps rows: the caller must not modify it.
func MakeKeyedList[K comparable, T any](rows []T, keys []K) KeyedList[K, T] {
	index := make(map[K]int, len(keys))
	for i, k := range keys {
		index[k] = i
	}
	return KeyedList[K, T]{rows: rows, index: index}
}

// Len returns the number of rows.
func (l KeyedList[K, T]) Len() int { return len(l.rows) }

// At returns row i. It panics if i is out of range, like a slice.
func (l KeyedList[K, T]) At(i int) *T { return &l.rows[i] }

// All yields every row in source order.
func (l KeyedList[K, T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for i := range l.rows {
			if !yield(&l.rows[i]) {
				return
			}
		}
	}
}

// Find returns the row whose key is k.
func (l KeyedList[K, T]) Find(k K) (*T, bool) {
	i, ok := l.index[k]
	if !ok {
		return nil, false
	}
	return &l.rows[i], true
}

// ---------------------------------------------------------------------------
// Evaluation errors (DECISIONS 19): translated code panics with *EvalError.
// ---------------------------------------------------------------------------

// EvalError is an error the Canon evaluator would also report, with its code.
type EvalError struct {
	Code    string // Canon diagnostic code, e.g. "E4101"
	Message string
}

func (e *EvalError) Error() string { return e.Code + ": " + e.Message }

// Fail panics with an *EvalError.
func Fail(code, message string) { panic(&EvalError{Code: code, Message: message}) }

// ---------------------------------------------------------------------------
// Checked arithmetic (CONFORMANCE.md §3). Int is int64, Float is float64,
// Duration is int64 milliseconds.
// ---------------------------------------------------------------------------

func AddInt(a, b int64) int64 {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		Fail("E4101", "integer overflow in +")
	}
	return a + b
}

func SubInt(a, b int64) int64 {
	if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		Fail("E4101", "integer overflow in -")
	}
	return a - b
}

func MulInt(a, b int64) int64 {
	var overflow bool
	if a > 0 {
		if b > 0 {
			overflow = a > math.MaxInt64/b
		} else {
			overflow = b < math.MinInt64/a
		}
	} else if b > 0 {
		overflow = a < math.MinInt64/b
	} else {
		overflow = a != 0 && b < math.MaxInt64/a
	}
	if overflow {
		Fail("E4101", "integer overflow in *")
	}
	return a * b
}

func DivInt(a, b int64) int64 {
	if b == 0 {
		Fail("E4102", "integer division by zero")
	}
	if a == math.MinInt64 && b == -1 {
		Fail("E4101", "integer overflow in /")
	}
	return a / b
}

func ModInt(a, b int64) int64 {
	if b == 0 {
		Fail("E4102", "integer division by zero in %")
	}
	if b == -1 {
		return 0
	}
	return a % b
}

func NegInt(a int64) int64 {
	if a == math.MinInt64 {
		Fail("E4101", "integer overflow in unary -")
	}
	return -a
}

func AbsInt(a int64) int64 {
	if a == math.MinInt64 {
		Fail("E4101", "integer overflow in abs")
	}
	if a < 0 {
		return -a
	}
	return a
}

func ClampInt(x, lo, hi int64) int64 {
	if lo > hi {
		Fail("E4108", "clamp with lo > hi")
	}
	return min(max(x, lo), hi)
}

// CheckFloat: any Float operation whose result is NaN or infinite is E4104.
func CheckFloat(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		Fail("E4104", "float result is not finite")
	}
	return x
}

func AddFloat(a, b float64) float64 { return CheckFloat(a + b) }
func SubFloat(a, b float64) float64 { return CheckFloat(a - b) }
func MulFloat(a, b float64) float64 { return CheckFloat(a * b) }
func DivFloat(a, b float64) float64 { return CheckFloat(a / b) }
func ModFloat(a, b float64) float64 { return CheckFloat(math.Mod(a, b)) }
func NegFloat(a float64) float64    { return -a }
func AbsFloat(a float64) float64    { return math.Abs(a) }

// MinFloat and MaxFloat order -0.0 before +0.0 (IEEE 754-2019 minimum and maximum).
func MinFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	if b < a {
		return b
	}
	if math.Signbit(a) {
		return a
	}
	return b
}

func MaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	if b > a {
		return b
	}
	if math.Signbit(a) {
		return b
	}
	return a
}

func ClampFloat(x, lo, hi float64) float64 {
	if lo > hi {
		Fail("E4108", "clamp with lo > hi")
	}
	return MinFloat(MaxFloat(x, lo), hi)
}

func IntToFloat(i int64) float64 { return float64(i) }

// FloatToInt is Int(f): it truncates toward zero; E4103 when the result does not fit.
func FloatToInt(f float64) int64 {
	if !(f >= -9223372036854775808.0 && f < 9223372036854775808.0) {
		Fail("E4103", "float out of the Int range")
	}
	return int64(f)
}

func FloorFloat(f float64) int64 { return FloatToInt(math.Floor(f)) }
func CeilFloat(f float64) int64  { return FloatToInt(math.Ceil(f)) }

// RoundFloat rounds half away from zero.
func RoundFloat(f float64) int64 { return FloatToInt(math.Round(f)) }

// DivDuration is Duration / Duration, a Float; a zero divisor is E4102.
func DivDuration(a, b int64) float64 {
	if b == 0 {
		Fail("E4102", "duration division by zero")
	}
	return CheckFloat(float64(a) / float64(b))
}

// CheckIntRange checks the range refinement of a parameter or a return value: E3204.
func CheckIntRange(v, lo, hi int64) int64 {
	if v < lo || v > hi {
		Fail("E3204", "value outside its refinement range")
	}
	return v
}

func CheckFloatRange(v, lo, hi float64) float64 {
	if v < lo || v > hi {
		Fail("E3204", "value outside its refinement range")
	}
	return v
}

// CheckIntWidth checks the range of a sized integer type (Int8 ... UInt64): E3201.
func CheckIntWidth(v, lo, hi int64) int64 {
	if v < lo || v > hi {
		Fail("E3201", "value does not fit its sized integer type")
	}
	return v
}

// ToFloat32 stores a Float into a Float32, rounding to nearest-even; overflow is E3202.
func ToFloat32(v float64) float32 {
	if math.Abs(v) >= 3.4028235677973366e+38 {
		Fail("E3202", "value overflows Float32")
	}
	return float32(v)
}

// CheckFloatArg: a Float argument that is NaN or infinite is E4104 on entry.
func CheckFloatArg(v float64) float64 { return CheckFloat(v) }

// DurationToMs converts a runtime argument to Canon's millisecond count; any
// sub-millisecond part is truncated toward zero.
func DurationToMs(d time.Duration) int64 { return d.Milliseconds() }

// DurationFromMs converts a Canon duration to a time.Duration.
func DurationFromMs(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }

// ---------------------------------------------------------------------------
// Data files
// ---------------------------------------------------------------------------

// DataFile is the envelope of a data file written by `canon build`.
type DataFile struct {
	Schema string          `json:"$schema"`
	Rows   json.RawMessage `json:"rows"`
	Value  json.RawMessage `json:"value"`
}

// ReadDataFile reads a data file and checks its $schema against schema.
func ReadDataFile(path, schema string) (*DataFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDataFile(path, raw, schema)
}

// ParseDataFile parses a data file (name is used in messages) and checks its $schema.
func ParseDataFile(name string, raw []byte, schema string) (*DataFile, error) {
	var f DataFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: not a canon data file: %w", name, err)
	}
	if f.Schema != schema {
		got := f.Schema
		if got == "" {
			got = "<none>"
		}
		return nil, fmt.Errorf("%s: built from schema %s, this binary expects %s. Rebuild the data or deploy the matching binary", name, got, schema)
	}
	return &f, nil
}

// Missing is the error of a required key absent from a data file.
func Missing(name, path, key string) error {
	return fmt.Errorf("%s: %s%s: missing", name, path, key)
}

// ---------------------------------------------------------------------------
// Runtime inputs (EVALUATION.md §11.3): environment text is read as the literal of
// the field's type.
// ---------------------------------------------------------------------------

// Env returns the variable's value. An unset or empty variable is unset.
func Env(name string) (string, bool) {
	v, ok := os.LookupEnv(name)
	return v, ok && v != ""
}

func isDec(c byte) bool { return c >= '0' && c <= '9' }

// allDigits reports whether s is one or more decimal digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDec(s[i]) {
			return false
		}
	}
	return true
}

// digits removes the '_' separators of a Canon decimal literal (duration parts).
func digits(s string) (string, bool) {
	if s == "" || !isDec(s[0]) || !isDec(s[len(s)-1]) {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case isDec(c):
			b.WriteByte(c)
		case c == '_' && isDec(s[i+1]):
		default:
			return "", false
		}
	}
	return b.String(), true
}

// ParseIntLiteral reads an integer input: decimal digits with an optional leading '-',
// no separator and no prefix.
func ParseIntLiteral(s string) (int64, error) {
	if !allDigits(strings.TrimPrefix(s, "-")) {
		return 0, fmt.Errorf("%q is not an integer", s)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is out of the Int range", s)
	}
	return n, nil
}

// ParseFloatLiteral reads a Float input: -?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?, finite.
func ParseFloatLiteral(s string) (float64, error) {
	bad := fmt.Errorf("%q is not a number", s)
	body := strings.TrimPrefix(s, "-")
	mant, exp, hasExp := strings.Cut(body, "e")
	if !hasExp {
		mant, exp, hasExp = strings.Cut(body, "E")
	}
	intPart, frac, hasFrac := strings.Cut(mant, ".")
	if !allDigits(intPart) || (hasFrac && !allDigits(frac)) {
		return 0, bad
	}
	if hasExp {
		if exp != "" && (exp[0] == '+' || exp[0] == '-') {
			exp = exp[1:]
		}
		if !allDigits(exp) {
			return 0, bad
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%q is out of the Float range", s)
	}
	return f, nil
}

// ParseStringLiteral reads a String input: the text byte for byte, which must be UTF-8.
func ParseStringLiteral(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("%q is not valid UTF-8", s)
	}
	return s, nil
}

// ParseBoolLiteral reads true or false.
func ParseBoolLiteral(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("%q is not true or false", s)
}

// ParseDurationLiteral reads a Canon duration literal: 250ms, 90s, 1h30m, -5m.
// Units d h m s ms appear at most once each, largest first.
func ParseDurationLiteral(s string) (time.Duration, error) {
	bad := fmt.Errorf("%q is not a duration", s)
	body := strings.TrimPrefix(s, "-")
	units := []struct {
		suffix string
		ms     int64
	}{{"d", 86_400_000}, {"h", 3_600_000}, {"ms", 1}, {"m", 60_000}, {"s", 1_000}}
	order := map[string]int{"d": 0, "h": 1, "m": 2, "s": 3, "ms": 4}
	var total int64
	last := -1
	for body != "" {
		i := 0
		for i < len(body) && (isDec(body[i]) || body[i] == '_') {
			i++
		}
		d, ok := digits(body[:i])
		if !ok {
			return 0, bad
		}
		rest := body[i:]
		matched := false
		for _, u := range units {
			if !strings.HasPrefix(rest, u.suffix) {
				continue
			}
			if order[u.suffix] <= last {
				return 0, bad
			}
			last = order[u.suffix]
			n, err := strconv.ParseInt(d, 10, 64)
			if err != nil || n > (math.MaxInt64-total)/u.ms {
				return 0, fmt.Errorf("%q is out of the Duration range", s)
			}
			total += n * u.ms
			body = rest[len(u.suffix):]
			matched = true
			break
		}
		if !matched {
			return 0, bad
		}
	}
	if last < 0 {
		return 0, bad
	}
	if total > math.MaxInt64/int64(time.Millisecond) {
		return 0, fmt.Errorf("%q is out of the Duration range", s)
	}
	if strings.HasPrefix(s, "-") {
		total = -total
	}
	return DurationFromMs(total), nil
}

// InputError is the error of a runtime input that is missing or invalid.
func InputError(name, reason string) error {
	return fmt.Errorf("environment variable %s: %s", name, reason)
}
```

---

## 7. C++

### 7.1 Files, namespaces, includes

- `<last>.gen.h` starts with `#pragma once`, as `<last>.defines.gen.h` does (§7.9; recent Source
  code's form, log-2026-09-24), then standard headers (sorted), then
  `<nlohmann/json_fwd.hpp>` in JSON modes, then imported packages' headers (sorted), then
  `"canon_runtime.h"`. Runtime headers use version guards (`CANON_RUNTIME_RT_V1`), never
  `#pragma once`, so that identical copies from two output directories can meet in one
  translation unit.
- Everything is in the emit's `namespace` (C++17 nested form `namespace sov::gen {`). Internals are
  in `<namespace>::detail`; conformance entry points in `<namespace>::conformance`.
- The header declares; `.gen.cpp` defines loaders, accessors, lookup tables, baked data, id-enum
  lookups and `detail::<P>Access`. Getters, the helpers of declared enums and pure translated
  functions are inline in the header.
- Generated C++ never throws and never catches. It builds with exceptions disabled.

### 7.2 Classes

- Members are `f_`, private, initialized (`int64_t heal_ = 0;`). Storage is reference layout:
  optional records use `std::optional<T>`, or `std::unique_ptr<T>` when `T` contains the
  enclosing class; variants use `std::variant` of the case classes; keyed lists
  `canon::KeyedList`.
- Every class declares `friend struct detail::<P>Access;`. In `data`, `embedded` and `types`
  mode, each public record and variant also has
  `bool detail::Decode(const nlohmann::json& v, canon::json::Decoder& dec, T& out);`, declared in
  the header and befriended by `T`, which other packages' decoders call (§2.8).
- In `types` mode, the public `static std::optional<T> T::Decode(const nlohmann::json&,
  std::string& error)` wraps it.

### 7.3 Sample: `baked` table (resource.vocab, abridged)

```cpp
/// The key of an entry of eventTypes.
enum class EventTypeId : uint8_t { COMBAT_KILL_MONSTER, ECONOMY_DROP_ITEM /* … */ };
std::string_view ToWire(EventTypeId v);
std::optional<EventTypeId> EventTypeIdFromWire(std::string_view key);

class EventType {
public:
    EventTypeId GetId() const { return id_; }
    bool GetRetired() const { return retired_; }
    /// Stored in the database: never renumbered, never reused (`@stable`).
    uint16_t GetCode() const { return code_; }
    const std::string& GetDisplay() const { return display_; }
    ParamKind GetParam() const { return param_; }
    /// Compile flag the event only exists under.
    const std::string* GetFlag() const { return flag_ ? &*flag_ : nullptr; }
private:
    friend struct detail::VocabAccess;
    EventTypeId id_{};
    bool retired_ = false;
    uint16_t code_ = 0;
    std::string display_;
    ParamKind param_{};
    std::optional<std::string> flag_;
};

class EventTypes {
public:
    size_t Len() const;
    const EventType& At(size_t i) const;
    const std::vector<EventType>& All() const;
    const EventType& Get(EventTypeId id) const;
    const EventType* Find(std::string_view key) const;
    const EventType* FindByCode(uint16_t code) const;
private:
    friend struct detail::VocabAccess;
    std::vector<EventType> rows_;
    std::vector<uint16_t> codes_;
    std::vector<uint32_t> byCode_;
};

const EventTypes& GetEventTypes();

// vocab.gen.cpp
namespace detail {
struct VocabAccess {
    struct Data { EventTypes eventTypes; };
    static Data Build();                      // fills rows_, codes_, byCode_
    static const Data& Get() { static const Data data = Build(); return data; }
};
}  // namespace detail
const EventTypes& GetEventTypes() { return detail::VocabAccess::Get().eventTypes; }
```

### 7.4 `canon_runtime.h`

Written to every C++ output directory. Namespace `canon::rt_v1` (inline), so code uses
`canon::FlatMap`; two runtime versions linked into one binary do not collide (CPP-05). It holds the
evaluation-error handler of DECISIONS 19, the checked arithmetic of CONFORMANCE.md §3, the
read-only containers and the atomic snapshot pointer.

```cpp
// GENERATED by canon: C++ runtime rt_v1. DO NOT EDIT.
#ifndef CANON_RUNTIME_RT_V1
#define CANON_RUNTIME_RT_V1

#include <algorithm>
#include <atomic>
#include <cmath>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <limits>
#include <memory>
#include <string>
#include <string_view>
#include <type_traits>
#include <utility>
#include <vector>

namespace canon {
inline namespace rt_v1 {

namespace json {
class Decoder;  // canon_runtime_json.h
}  // namespace json

// ---------------------------------------------------------------------------
// Evaluation errors (DECISIONS 19). Generated code never throws.
// ---------------------------------------------------------------------------

/// Called when translated code hits an error the Canon evaluator would report
/// (overflow, division by zero, ...). `code` is the Canon diagnostic code ("E4101");
/// both views point to string literals.
using EvalErrorHandler = void (*)(std::string_view code, std::string_view message);

namespace detail {
inline void DefaultEvalErrorHandler(std::string_view code, std::string_view message) {
    std::fprintf(stderr, "canon: evaluation error %.*s: %.*s\n", static_cast<int>(code.size()),
                 code.data(), static_cast<int>(message.size()), message.data());
    std::fflush(stderr);
    std::abort();
}
inline std::atomic<EvalErrorHandler> g_evalErrorHandler{&DefaultEvalErrorHandler};
}  // namespace detail

/// Replaces the handler and returns the previous one. nullptr restores the default,
/// which prints the error to stderr and calls std::abort().
inline EvalErrorHandler SetEvalErrorHandler(EvalErrorHandler handler) noexcept {
    return detail::g_evalErrorHandler.exchange(handler != nullptr ? handler
                                                                  : &detail::DefaultEvalErrorHandler);
}

/// Reports an evaluation error. If the handler returns, the helper that called it
/// returns 0 (or 0.0) and the translated function carries on with that value.
inline void OnEvalError(std::string_view code, std::string_view message) {
    detail::g_evalErrorHandler.load()(code, message);
}

// ---------------------------------------------------------------------------
// Checked arithmetic (CONFORMANCE.md §3). Int is int64_t, Float is double,
// Duration is int64_t milliseconds.
// ---------------------------------------------------------------------------

inline constexpr int64_t kIntMin = std::numeric_limits<int64_t>::min();
inline constexpr int64_t kIntMax = std::numeric_limits<int64_t>::max();

inline int64_t AddInt(int64_t a, int64_t b) {
    if ((b > 0 && a > kIntMax - b) || (b < 0 && a < kIntMin - b)) {
        OnEvalError("E4101", "integer overflow in +");
        return 0;
    }
    return a + b;
}

inline int64_t SubInt(int64_t a, int64_t b) {
    if ((b < 0 && a > kIntMax + b) || (b > 0 && a < kIntMin + b)) {
        OnEvalError("E4101", "integer overflow in -");
        return 0;
    }
    return a - b;
}

inline int64_t MulInt(int64_t a, int64_t b) {
    bool overflow;
    if (a > 0) {
        overflow = b > 0 ? a > kIntMax / b : b < kIntMin / a;
    } else {
        overflow = b > 0 ? a < kIntMin / b : (a != 0 && b < kIntMax / a);
    }
    if (overflow) {
        OnEvalError("E4101", "integer overflow in *");
        return 0;
    }
    return a * b;
}

inline int64_t DivInt(int64_t a, int64_t b) {
    if (b == 0) {
        OnEvalError("E4102", "integer division by zero");
        return 0;
    }
    if (a == kIntMin && b == -1) {
        OnEvalError("E4101", "integer overflow in /");
        return 0;
    }
    return a / b;
}

inline int64_t ModInt(int64_t a, int64_t b) {
    if (b == 0) {
        OnEvalError("E4102", "integer division by zero in %");
        return 0;
    }
    if (b == -1) return 0;
    return a % b;
}

inline int64_t NegInt(int64_t a) {
    if (a == kIntMin) {
        OnEvalError("E4101", "integer overflow in unary -");
        return 0;
    }
    return -a;
}

inline int64_t AbsInt(int64_t a) {
    if (a == kIntMin) {
        OnEvalError("E4101", "integer overflow in abs");
        return 0;
    }
    return a < 0 ? -a : a;
}

inline int64_t MinInt(int64_t a, int64_t b) { return (std::min)(a, b); }
inline int64_t MaxInt(int64_t a, int64_t b) { return (std::max)(a, b); }

inline int64_t ClampInt(int64_t x, int64_t lo, int64_t hi) {
    if (lo > hi) {
        OnEvalError("E4108", "clamp with lo > hi");
        return 0;
    }
    return x < lo ? lo : (x > hi ? hi : x);
}

/// Any Float operation whose result is NaN or infinite is E4104.
inline double CheckFloat(double x) {
    if (!std::isfinite(x)) {
        OnEvalError("E4104", "float result is not finite");
        return 0.0;
    }
    return x;
}

inline double AddFloat(double a, double b) { return CheckFloat(a + b); }
inline double SubFloat(double a, double b) { return CheckFloat(a - b); }
inline double MulFloat(double a, double b) { return CheckFloat(a * b); }
inline double DivFloat(double a, double b) { return CheckFloat(a / b); }
inline double ModFloat(double a, double b) { return CheckFloat(std::fmod(a, b)); }
inline double NegFloat(double a) { return -a; }
inline double AbsFloat(double a) { return std::fabs(a); }

/// min and max order -0.0 before +0.0 (IEEE 754-2019 minimum and maximum).
inline double MinFloat(double a, double b) {
    if (a < b) return a;
    if (b < a) return b;
    return std::signbit(a) ? a : b;
}

inline double MaxFloat(double a, double b) {
    if (a > b) return a;
    if (b > a) return b;
    return std::signbit(a) ? b : a;
}

inline double ClampFloat(double x, double lo, double hi) {
    if (lo > hi) {
        OnEvalError("E4108", "clamp with lo > hi");
        return 0.0;
    }
    return MinFloat(MaxFloat(x, lo), hi);
}

inline double IntToFloat(int64_t i) { return static_cast<double>(i); }

/// Int(f): truncates toward zero; E4103 when the result does not fit.
inline int64_t FloatToInt(double f) {
    if (!(f >= -9223372036854775808.0 && f < 9223372036854775808.0)) {
        OnEvalError("E4103", "float out of the Int range");
        return 0;
    }
    return static_cast<int64_t>(f);
}

inline int64_t FloorFloat(double f) { return FloatToInt(std::floor(f)); }
inline int64_t CeilFloat(double f) { return FloatToInt(std::ceil(f)); }
/// Rounds half away from zero.
inline int64_t RoundFloat(double f) { return FloatToInt(std::round(f)); }

/// Duration / Duration is a Float; a zero divisor is E4102.
inline double DivDuration(int64_t a, int64_t b) {
    if (b == 0) {
        OnEvalError("E4102", "duration division by zero");
        return 0.0;
    }
    return CheckFloat(static_cast<double>(a) / static_cast<double>(b));
}

/// Range refinement of a parameter or a return value (lo and hi inclusive): E3204.
inline int64_t CheckIntRange(int64_t v, int64_t lo, int64_t hi) {
    if (v < lo || v > hi) {
        OnEvalError("E3204", "value outside its refinement range");
        return 0;
    }
    return v;
}

inline double CheckFloatRange(double v, double lo, double hi) {
    if (v < lo || v > hi) {
        OnEvalError("E3204", "value outside its refinement range");
        return 0.0;
    }
    return v;
}

/// Range of a sized integer type (Int8 ... UInt64) of a parameter or a return value: E3201.
inline int64_t CheckIntWidth(int64_t v, int64_t lo, int64_t hi) {
    if (v < lo || v > hi) {
        OnEvalError("E3201", "value does not fit its sized integer type");
        return 0;
    }
    return v;
}

/// Stores a Float into a Float32, rounding to nearest-even; overflow is E3202.
inline float ToFloat32(double v) {
    if (std::fabs(v) >= 3.4028235677973366e+38) {
        OnEvalError("E3202", "value overflows Float32");
        return 0.0f;
    }
    return static_cast<float>(v);
}

/// A Float argument that is NaN or infinite is E4104 on entry.
inline double CheckFloatArg(double v) { return CheckFloat(v); }

// ---------------------------------------------------------------------------
// Read-only containers
// ---------------------------------------------------------------------------

/// The type a lookup takes: std::string_view for std::string keys, the key itself
/// otherwise. Lookups never allocate.
template <class K>
using LookupKey = std::conditional_t<std::is_same_v<K, std::string>, std::string_view, K>;

namespace detail {
template <class K>
LookupKey<K> AsLookup(const K& k) {
    return LookupKey<K>(k);
}

/// Positions of `keys`, sorted by key.
template <class K>
std::vector<uint32_t> SortedIndex(const std::vector<K>& keys) {
    std::vector<uint32_t> index(keys.size());
    for (uint32_t i = 0; i < index.size(); ++i) index[i] = i;
    std::sort(index.begin(), index.end(), [&keys](uint32_t a, uint32_t b) {
        return AsLookup(keys[a]) < AsLookup(keys[b]);
    });
    return index;
}

/// Binary search of `key` among `keys` through `index`; returns the position or -1.
template <class K>
std::ptrdiff_t FindSorted(const std::vector<K>& keys, const std::vector<uint32_t>& index,
                          LookupKey<K> key) {
    auto it = std::lower_bound(index.begin(), index.end(), key,
                               [&keys](uint32_t i, LookupKey<K> k) { return AsLookup(keys[i]) < k; });
    if (it == index.end() || !(AsLookup(keys[*it]) == key)) return -1;
    return static_cast<std::ptrdiff_t>(*it);
}
}  // namespace detail

/// A read-only map: iteration in source order, Find by binary search.
template <class K, class V>
class FlatMap {
public:
    using Entry = std::pair<K, V>;

    FlatMap() = default;

    /// For generated code: `entries` in source order, keys unique.
    static FlatMap FromEntries(std::vector<Entry> entries) {
        FlatMap m;
        m.entries_ = std::move(entries);
        m.keys_.reserve(m.entries_.size());
        for (const Entry& e : m.entries_) m.keys_.push_back(e.first);
        m.index_ = detail::SortedIndex(m.keys_);
        return m;
    }

    size_t Len() const noexcept { return entries_.size(); }
    const Entry& At(size_t i) const { return entries_[i]; }
    const std::vector<Entry>& All() const noexcept { return entries_; }
    typename std::vector<Entry>::const_iterator begin() const noexcept { return entries_.begin(); }
    typename std::vector<Entry>::const_iterator end() const noexcept { return entries_.end(); }

    /// nullptr when the key is absent.
    const V* Find(LookupKey<K> key) const {
        std::ptrdiff_t i = detail::FindSorted(keys_, index_, key);
        return i < 0 ? nullptr : &entries_[static_cast<size_t>(i)].second;
    }

private:
    std::vector<Entry> entries_;
    std::vector<K> keys_;
    std::vector<uint32_t> index_;
};

/// A read-only list whose elements are unique by a key: `[T] keyed by f` fields.
template <class K, class T>
class KeyedList {
public:
    KeyedList() = default;

    /// For generated code: `rows` in source order and their keys, keys unique.
    static KeyedList FromRows(std::vector<T> rows, std::vector<K> keys) {
        KeyedList l;
        l.rows_ = std::move(rows);
        l.keys_ = std::move(keys);
        l.index_ = detail::SortedIndex(l.keys_);
        return l;
    }

    size_t Len() const noexcept { return rows_.size(); }
    const T& At(size_t i) const { return rows_[i]; }
    const std::vector<T>& All() const noexcept { return rows_; }
    typename std::vector<T>::const_iterator begin() const noexcept { return rows_.begin(); }
    typename std::vector<T>::const_iterator end() const noexcept { return rows_.end(); }

    /// nullptr when the key is absent.
    const T* Find(LookupKey<K> key) const {
        std::ptrdiff_t i = detail::FindSorted(keys_, index_, key);
        return i < 0 ? nullptr : &rows_[static_cast<size_t>(i)];
    }

private:
    std::vector<T> rows_;
    std::vector<K> keys_;
    std::vector<uint32_t> index_;
};

// ---------------------------------------------------------------------------
// Snapshots (SPEC §15.5)
// ---------------------------------------------------------------------------

/// An atomically replaceable std::shared_ptr. Readers keep the snapshot they loaded;
/// the old one is freed when its last reader lets go.
template <class T>
class AtomicSharedPtr {
public:
    constexpr AtomicSharedPtr() noexcept = default;
    AtomicSharedPtr(const AtomicSharedPtr&) = delete;
    AtomicSharedPtr& operator=(const AtomicSharedPtr&) = delete;

#if defined(__cpp_lib_atomic_shared_ptr) && __cpp_lib_atomic_shared_ptr >= 201711L
    std::shared_ptr<T> Load() const noexcept { return p_.load(std::memory_order_acquire); }
    void Store(std::shared_ptr<T> p) noexcept { p_.store(std::move(p), std::memory_order_release); }

private:
    std::atomic<std::shared_ptr<T>> p_;
#else
    std::shared_ptr<T> Load() const noexcept {
        return std::atomic_load_explicit(&p_, std::memory_order_acquire);
    }
    void Store(std::shared_ptr<T> p) noexcept {
        std::atomic_store_explicit(&p_, std::move(p), std::memory_order_release);
    }

private:
    std::shared_ptr<T> p_;
#endif
};

}  // namespace rt_v1
}  // namespace canon

#endif  // CANON_RUNTIME_RT_V1
```

### 7.5 `canon_runtime_json.h`

Written next to `canon_runtime.h` when the emit is in `embedded`, `data` or `types` mode. Requires
nlohmann/json ≥ 3.9 on the include path as `<nlohmann/json.hpp>`. Nothing in it throws: parsing
uses `allow_exceptions=false` and every typed read checks the JSON type first, so it also builds
with exceptions disabled.

```cpp
// GENERATED by canon: C++ runtime rt_v1 (JSON). DO NOT EDIT.
#ifndef CANON_RUNTIME_JSON_RT_V1
#define CANON_RUNTIME_JSON_RT_V1

#include <chrono>
#include <cstdint>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

#include <nlohmann/json.hpp>

#include "canon_runtime.h"

namespace canon {
inline namespace rt_v1 {
namespace json {

using Json = nlohmann::json;

/// Reads the whole file. False (with `error` set) when it cannot be read.
inline bool ReadFile(const std::string& path, std::string& out, std::string& error) {
    std::ifstream in(path, std::ios::binary);
    if (!in) {
        error = "cannot open " + path;
        return false;
    }
    out.assign(std::istreambuf_iterator<char>(in), std::istreambuf_iterator<char>());
    if (in.bad()) {
        error = "cannot read " + path;
        return false;
    }
    return true;
}

/// Parses a data file written by `canon build` and checks its "$schema" against the
/// fingerprint compiled into this binary. `name` is used in messages (a path, or
/// "embedded potions.json").
inline bool ParseDataFile(const std::string& name, const std::string& text, std::string_view schema,
                          Json& doc, std::string& error) {
    doc = Json::parse(text, nullptr, /*allow_exceptions=*/false);
    if (doc.is_discarded() || !doc.is_object()) {
        error = name + ": not a canon data file (invalid JSON)";
        return false;
    }
    auto it = doc.find("$schema");
    std::string got = "<none>";
    if (it != doc.end() && it->is_string()) got = it->get_ref<const std::string&>();
    if (got != schema) {
        error = name + ": built from schema " + got + ", this binary expects " + std::string(schema) +
                ". Rebuild the data or deploy the matching binary";
        return false;
    }
    return true;
}

/// Reads JSON values into generated classes. It records the first error with the
/// path of the value, and never validates: values were checked by `canon build`.
class Decoder {
public:
    explicit Decoder(std::string name) : name_(std::move(name)) {}

    bool Ok() const noexcept { return error_.empty(); }
    const std::string& Error() const noexcept { return error_; }

    /// Path tracking for messages: Push("rows[3]") ... Pop().
    void Push(std::string segment) { path_.push_back(std::move(segment)); }
    void Pop() { path_.pop_back(); }

    /// Records an error at the current path (only the first one is kept).
    void Fail(std::string_view key, std::string_view what) {
        if (!error_.empty()) return;
        error_ = name_ + ": ";
        for (size_t i = 0; i < path_.size(); ++i) {
            if (i > 0) error_ += '.';
            error_ += path_[i];
        }
        if (!key.empty()) {
            if (!path_.empty()) error_ += '.';
            error_ += key;
        }
        error_ += ": ";
        error_ += what;
    }

    /// The value of `key`, or nullptr when the key is absent or null.
    const Json* Optional(const Json& obj, const char* key) {
        if (!obj.is_object()) {
            Fail(key, "expected an object");
            return nullptr;
        }
        auto it = obj.find(key);
        if (it == obj.end() || it->is_null()) return nullptr;
        return &*it;
    }

    /// The value of `key`; an error when it is absent or null.
    const Json* Required(const Json& obj, const char* key) {
        const Json* v = Optional(obj, key);
        if (v == nullptr) Fail(key, "missing");
        return v;
    }

    bool AsString(const Json& v, std::string_view key, std::string& out) {
        if (!v.is_string()) return Fail(key, "expected a string"), false;
        out = v.get_ref<const std::string&>();
        return true;
    }

    bool AsInt(const Json& v, std::string_view key, int64_t& out) {
        if (v.is_number_integer() && !(v.is_number_unsigned() && v.get<uint64_t>() > uint64_t(kIntMax))) {
            out = v.get<int64_t>();
            return true;
        }
        return Fail(key, "expected an integer"), false;
    }

    /// A Float may be written as an integer token ("1" for 1.0).
    bool AsFloat(const Json& v, std::string_view key, double& out) {
        if (!v.is_number()) return Fail(key, "expected a number"), false;
        out = v.get<double>();
        return true;
    }

    bool AsBool(const Json& v, std::string_view key, bool& out) {
        if (!v.is_boolean()) return Fail(key, "expected true or false"), false;
        out = v.get<bool>();
        return true;
    }

    /// `unitMs` is the length of one wire unit: 1 for ms, 1000 for s, 60000 for m...
    bool AsDuration(const Json& v, std::string_view key, int64_t unitMs, std::chrono::milliseconds& out) {
        int64_t n = 0;
        if (!AsInt(v, key, n)) return false;
        out = std::chrono::milliseconds(n * unitMs);
        return true;
    }

    /// `parse` is a generated <Enum>FromWire or <Enum>FromCode function.
    template <class E, class W>
    bool AsEnum(const Json& v, std::string_view key, std::optional<E> (*parse)(W), E& out) {
        std::optional<E> e;
        if constexpr (std::is_same_v<W, std::string_view>) {
            if (v.is_string()) e = parse(std::string_view(v.get_ref<const std::string&>()));
        } else {
            int64_t n = 0;
            if (v.is_number_integer()) {
                n = v.get<int64_t>();
                if (n >= int64_t(std::numeric_limits<W>::min()) && n <= int64_t(std::numeric_limits<W>::max()))
                    e = parse(static_cast<W>(n));
            }
        }
        if (!e) return Fail(key, "unknown enum value"), false;
        out = *e;
        return true;
    }

    /// The array at `key` (or nullptr with an error).
    const Json* Array(const Json& obj, const char* key) {
        const Json* v = Required(obj, key);
        if (v != nullptr && !v->is_array()) return Fail(key, "expected an array"), nullptr;
        return v;
    }

    /// The object at `key` (or nullptr with an error).
    const Json* Object(const Json& obj, const char* key) {
        const Json* v = Required(obj, key);
        if (v != nullptr && !v->is_object()) return Fail(key, "expected an object"), nullptr;
        return v;
    }

    // Field shortcuts: Required + As*.
    bool String(const Json& obj, const char* key, std::string& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsString(*v, key, out);
    }
    bool Int(const Json& obj, const char* key, int64_t& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsInt(*v, key, out);
    }
    bool Float(const Json& obj, const char* key, double& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsFloat(*v, key, out);
    }
    bool Bool(const Json& obj, const char* key, bool& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsBool(*v, key, out);
    }
    bool Duration(const Json& obj, const char* key, int64_t unitMs, std::chrono::milliseconds& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsDuration(*v, key, unitMs, out);
    }
    template <class E, class W>
    bool Enum(const Json& obj, const char* key, std::optional<E> (*parse)(W), E& out) {
        const Json* v = Required(obj, key);
        return v != nullptr && AsEnum(*v, key, parse, out);
    }

private:
    std::string name_;
    std::vector<std::string> path_;
    std::string error_;
};

}  // namespace json
}  // namespace rt_v1
}  // namespace canon

#endif  // CANON_RUNTIME_JSON_RT_V1
```

### 7.6 Loader shape

The pipeline golden ([§10](#10-goldens)) is the reference for `data` mode: `detail::Decode` per
record reads each field with the `Decoder` shortcuts in field declaration order, then the `$`
keys; `detail::<P>Access::Load<V>` reads the file, checks `$schema`, decodes `rows` (or `value`)
with `Push("rows[<i>]")` around each row, and builds the container; the snapshot's `Load` calls
one loader per `@reload` value with `dir + "/" + <file name>`.

### 7.7 Runtime inputs

When a package has input fields, `.gen.cpp` contains, in an anonymous namespace, the fixed helpers
below (the C++ counterpart of `rt.Env` and `rt.Parse*Literal`), followed by `LoadInputs`, which
reads the variables in field declaration order and sets `error` to one line per failure (it
replaces the caller's text, as every `Load` of §7.5 does).
Only the helpers the package's inputs use are written, so the file stays warning-free under
`-Wall` (§9): `EnvText` always; `IsDecDigit` and `AllDigits` for `Int`, `Float` and `Duration`;
`DurationDigits` for `Duration`; `Parse<Kind>Literal` per input kind; `MatchPattern`, last, when a
`String` input has a pattern. A patterned input's block declares one table per distinct pattern of the field's alias chain
(a repeated pattern is kept once), innermost first (alias-chain order): `static constexpr uint32_t kPattern[]`, then `kPattern2[]`, `kPattern3[]` …, each the state count, one row `{op, out, arg, count}` per state (state 0 starts; op 0
accepts; 1 continues at `out` and at `arg`; 2 continues at `out` where the position `arg` names
holds, 1 the start of the text and 2 its end; 3 reads one code point within the `count` sorted
pairs at `kPattern + arg` and continues at `out`), then the code point pairs, each distinct set
once in first-use order; the checks run in that order after the range and length checks,
`!MatchPattern(kPattern, val)` first, and the first that fails gives the variable's one line. The loaded values live in
slots `detail::<P>Inputs::<Class>`, with the flag `detail::<P>InputsLoaded` beside them; Go's
counterparts are the package variables `input_<T>_<store>`, `input_<T>_<store>_OK` and
`input_<T>_<store>_Pattern` (then `_Pattern2`, `_Pattern3` …, one per pattern, same order), and
the flag `inputsLoaded_`. All these names come from the name
plan (§3.5).

```cpp
namespace {
// Unset or empty is unset.
bool EnvText(const char* name, std::string& out) {
    const char* v = std::getenv(name);
    if (v == nullptr || *v == '\0') return false;
    out = v;
    return true;
}
bool IsDecDigit(char c) { return c >= '0' && c <= '9'; }
bool AllDigits(std::string_view s) {
    if (s.empty()) return false;
    for (char c : s) {
        if (c < '0' || c > '9') return false;
    }
    return true;
}
// Integer input: decimal digits with an optional leading '-' (EVALUATION.md §11.3).
bool ParseIntLiteral(std::string_view s, int64_t& out) {
    std::string_view body = s;
    if (!body.empty() && body.front() == '-') body.remove_prefix(1);
    if (!AllDigits(body)) return false;
    std::string text(s);
    errno = 0;
    char* end = nullptr;
    long long v = std::strtoll(text.c_str(), &end, 10);
    if (errno == ERANGE || *end != '\0') return false;
    out = static_cast<int64_t>(v);
    return true;
}
// Float, Bool, Duration and String parsers follow rt.ParseFloatLiteral, rt.ParseBoolLiteral,
// rt.ParseDurationLiteral and rt.ParseStringLiteral exactly (same accepted text, same
// results); floats are read with std::istringstream imbued with std::locale::classic().
}  // namespace
```

### 7.8 Legacy structs

A record can target a hand-written C++ struct instead of a generated class (DECISIONS 5, SPEC
§15.3):

```
record Item @cpp(struct: "ItemProp", header: "ItemProp.h", access: fields) { … }
```

| Argument | Meaning |
|---|---|
| `struct` | the C++ type name (may be qualified) |
| `header` | the header that declares it, as written in `#include "…"`; required for `fields` and `both` (`E8109`) |
| `access` | `fields`, `both` or `getters` (`E8109` for anything else) |

Per field: `@cpp(field: "dwItemKind3")` names the member (default: the Canon field name), and
`@cpp(type: "DWORD")` gives its C++ type (default: the type of §4.1). On a variant case,
`@cpp(value: N)` gives the integer written to the member of the variant field. `@cpp(unit: s)` on a
`Duration` field stores the duration in that unit (default `ms`).

#### 7.8.1 Member types

`@cpp(type:)` takes one of these names; anything else is `E8107`, and so is a name incompatible
with the Canon type. A value that does not fit the member is `E8106` (checked at build time on
every emitted value); a string or list longer than a fixed-size array is `E8103`.

| `@cpp(type:)` | Range | Canon types it accepts |
|---|---|---|
| `bool` | — | `Bool` |
| `BOOL` | 0, 1 | `Bool` |
| `int8_t` | −128..127 | integers, enums, `ref` to defines |
| `BYTE`, `uint8_t`, `unsigned char` | 0..255 | same |
| `short`, `SHORT`, `int16_t` | −32 768..32 767 | same |
| `WORD`, `USHORT`, `uint16_t`, `unsigned short` | 0..65 535 | same |
| `int`, `INT`, `LONG`, `long`, `int32_t` | −2³¹..2³¹−1 | same, and `Duration` |
| `DWORD`, `UINT`, `ULONG`, `unsigned`, `unsigned int`, `unsigned long`, `uint32_t` | 0..2³²−1 | same, and `Duration` |
| `int64_t`, `__int64`, `LONGLONG`, `long long` | 64-bit | same, and `Duration` |
| `uint64_t`, `ULONGLONG`, `unsigned long long`, `DWORD64` | 0..2⁶³−1 (TYP-03) | same, and `Duration` |
| `float`, `double` | finite | `Float`, `Float32` |
| `char[N]` | at most N−1 bytes | `String`, `ref` (the key) |
| `std::string` | — | `String`, `ref` (the key) |
| `T[N]` with `T` above | at most N elements (the rest value-initialized) | lists of the accepted types |

An enum stored in an integer member holds its code (`@codes`) or its index. A `ref` to a define
table stores the define's value. The variant field of an inline variant stores the case's
`@cpp(value:)` (missing: `E8108`); each case field maps to its own member, left value-initialized
when the row's case lacks it. A record containing a map, a non-inline variant, a nested record or
a list of records (a `@json(pairs:)` field included) cannot map onto a legacy struct (`E8109`).

#### 7.8.2 `access: fields`

The struct stays hand-written. Canon generates, in `<last>.gen.h`:

- `#include "<header>"`;
- `static_assert(std::is_default_constructible_v<S> && std::is_move_assignable_v<S>, …)`;
- one `static_assert(std::is_same_v<decltype(S::m), T>, "S::m must be T (<dir>: <Type>.<field>)")`
  per mapped member;
- `bool Fill(const nlohmann::json& row, canon::json::Decoder& dec, S& out);` in the package
  namespace (public: a legacy loader in `types` mode calls it per row). `Fill` starts with
  `out = S{};`, so unmapped members are value-initialized;
- the container of each value of that type, holding `std::vector<S>`, whose lookups return
  `const S*` and `const S&`, so every read compiles and every write is an error.

Worked example: the item table, with the hand-written struct

```cpp
// ItemProp.h (hand-written)
struct ItemProp {
    DWORD dwID;
    char  szName[64];
    DWORD dwItemKind1;
    DWORD dwItemKind3;
    DWORD dwAbilityMin;
    DWORD dwAbilityMax;
    int   nHeal;
    DWORD dwFlags;        // not mapped: value-initialized
};
```

and the Canon source

```
package game.items

local let itemDefines = load.defines("@resource/Server/Define/defineItem.h", prefix: "II_")

enum ItemKind3 @codes(UInt32) { SWORD = 201, AXE = 203, MATERIAL = 400 }

variant ItemKind @json(tag: "dwItemKind1") {
  IK1_WEAPON @cpp(value: 1) {
    attackMin: Int(0..) @json("dwAbilityMin") @cpp(field: "dwAbilityMin", type: "DWORD")
    attackMax: Int(0..) @json("dwAbilityMax") @cpp(field: "dwAbilityMax", type: "DWORD")
  }
  IK1_GENERAL @cpp(value: 4)
}

record Item @cpp(struct: "ItemProp", header: "ItemProp.h", access: fields) {
  define: ref itemDefines  @json("dwID")        @cpp(field: "dwID", type: "DWORD")
  name:   String(1..=63)   @json("szName")      @cpp(field: "szName", type: "char[64]")
  kind3:  ItemKind3        @json("dwItemKind3") @cpp(field: "dwItemKind3", type: "DWORD")
  heal:   Int(0..) = 0     @json("nHeal")       @cpp(field: "nHeal", type: "int")
  kind:   ItemKind         @json(inline)        @cpp(field: "dwItemKind1", type: "DWORD")
}

let items: stable table Item = {}
emit cpp { out: "@source/Generated/items", namespace: "sov::gen", mode: data }
```

generate:

```cpp
// items.gen.h
#include "ItemProp.h"

static_assert(std::is_default_constructible_v<ItemProp> && std::is_move_assignable_v<ItemProp>,
              "ItemProp must be default-constructible and move-assignable (game/items: Item)");
static_assert(std::is_same_v<decltype(ItemProp::dwID), DWORD>, "ItemProp::dwID must be DWORD (game/items: Item.define)");
static_assert(std::is_same_v<decltype(ItemProp::szName), char[64]>, "ItemProp::szName must be char[64] (game/items: Item.name)");
static_assert(std::is_same_v<decltype(ItemProp::dwItemKind1), DWORD>, "ItemProp::dwItemKind1 must be DWORD (game/items: Item.kind)");
static_assert(std::is_same_v<decltype(ItemProp::dwItemKind3), DWORD>, "ItemProp::dwItemKind3 must be DWORD (game/items: Item.kind3)");
static_assert(std::is_same_v<decltype(ItemProp::dwAbilityMin), DWORD>, "ItemProp::dwAbilityMin must be DWORD (game/items: ItemKind.IK1_WEAPON.attackMin)");
static_assert(std::is_same_v<decltype(ItemProp::dwAbilityMax), DWORD>, "ItemProp::dwAbilityMax must be DWORD (game/items: ItemKind.IK1_WEAPON.attackMax)");
static_assert(std::is_same_v<decltype(ItemProp::nHeal), int>, "ItemProp::nHeal must be int (game/items: Item.heal)");

namespace sov::gen {

inline constexpr std::string_view kItemsSchema = "game.items.Item@…";  // FINGERPRINT.md §2

/// Fills `out` from one row of items.json. Members that no field maps stay
/// value-initialized. Returns false (and records the error in `dec`) on a bad row.
bool Fill(const nlohmann::json& row, canon::json::Decoder& dec, ItemProp& out);

class Items {
public:
    static std::shared_ptr<const Items> Load(const std::string& path, std::string& error);
    size_t Len() const;
    const ItemProp& At(size_t i) const;
    const std::vector<ItemProp>& All() const;
    const ItemProp* Find(std::string_view key) const;   // by table key: "II_WEA_AXE_ANGEL"
private:
    friend struct detail::ItemsAccess;
    canon::KeyedList<std::string, ItemProp> rows_;
};

}  // namespace sov::gen
```

```cpp
// items.gen.cpp (Fill)
bool Fill(const nlohmann::json& row, canon::json::Decoder& dec, ItemProp& out) {
    out = ItemProp{};
    std::string s;
    int64_t n = 0;
    if (dec.String(row, "dwID", s)) {
        if (DefineValue(s, n)) out.dwID = static_cast<DWORD>(n);      // baked table of II_* (§5.8)
        else dec.Fail("dwID", "unknown define " + s);
    }
    if (dec.String(row, "szName", s)) std::memcpy(out.szName, s.c_str(), s.size() + 1);  // <= 63 bytes (E8103)
    if (dec.Int(row, "dwItemKind3", n)) out.dwItemKind3 = static_cast<DWORD>(n);           // fits (E8106)
    if (dec.Int(row, "nHeal", n)) out.nHeal = static_cast<int>(n);
    if (dec.String(row, "dwItemKind1", s)) {
        if (s == "IK1_WEAPON") {
            out.dwItemKind1 = 1;
            if (dec.Int(row, "dwAbilityMin", n)) out.dwAbilityMin = static_cast<DWORD>(n);
            if (dec.Int(row, "dwAbilityMax", n)) out.dwAbilityMax = static_cast<DWORD>(n);
        } else if (s == "IK1_GENERAL") {
            out.dwItemKind1 = 4;
        } else {
            dec.Fail("dwItemKind1", "unknown case " + s);
        }
    }
    return dec.Ok();
}
```

Existing code keeps `const ItemProp* p = items->Find(key); p->dwItemKind3` and cannot write
`p->nHeal = 0`.

#### 7.8.3 `access: both`

Canon **generates** the struct and writes it to `@cpp(header:)`, taking the file over (CPP-02):
the first build prints `adopting <path>` and requires `canon build --adopt <path>` (the same
explicit ownership transfer as `canon convert`, GEN-05); afterwards the file carries the marker.
The struct keeps the legacy member names and types, public, followed by getters:

```cpp
struct ItemProp {
    DWORD dwID = 0;
    char  szName[64] = {};
    DWORD dwItemKind1 = 0;
    DWORD dwItemKind3 = 0;
    DWORD dwAbilityMin = 0;
    DWORD dwAbilityMax = 0;
    int   nHeal = 0;

    const std::string& GetDefineKey() const;       // the key, kept by the container
    DWORD GetDefineValue() const { return dwID; }
    std::string_view GetName() const { return szName; }
    ItemKind3 GetKind3() const { return static_cast<ItemKind3>(dwItemKind3); }
    int GetHeal() const { return nHeal; }
    ItemKindKind GetKind() const;                  // from dwItemKind1 through @cpp(value:)
    DWORD GetAttackMin() const { return dwAbilityMin; }   // meaningful when GetKind() is IK1_WEAPON
    DWORD GetAttackMax() const { return dwAbilityMax; }
};
```

Getters of legacy members return the member's own type for integer, float and `bool` members
(so migrated reads keep their signedness), the enum type for enum fields, `std::string_view` for
`char[N]`, `const std::string&` for `std::string`, and `std::chrono::milliseconds` for durations.
Members no Canon field maps disappear from the generated struct; the build lists them.

#### 7.8.4 `access: getters`

The same struct, as a `class` whose members are private: every remaining legacy read fails to
compile, which lists the reads left to migrate. `@reload` becomes allowed (`E8201` no longer
applies).

### 7.9 `@cpp(defines:)`

`enum ItemKind1 @codes(UInt32) @cpp(defines: "IK1_") { … }` also writes `<last>.defines.gen.h`:

```cpp
// GENERATED by canon from game/items/. DO NOT EDIT.
#pragma once
#define IK1_WEAPON 1
#define IK1_GENERAL 4
```

- The define name is the member's name if it already starts with the prefix, else prefix + name;
  the value is the code with `@codes`, else the index, in decimal (CPP-03). Lines are in
  declaration order, retired members included.
- The defines live in their own header because a macro and an enumerator with the same name
  cannot meet: a translation unit that sees `#define IK1_WEAPON 1` cannot spell
  `ItemKind1::IK1_WEAPON`. Legacy code includes `<last>.defines.gen.h` (or keeps its own header
  when the values match token for token); new code uses the enum.
- Every generated header and source that names such an enumerator wraps that region in
  `#pragma push_macro("IK1_WEAPON")` / `#undef IK1_WEAPON` / `#pragma pop_macro("IK1_WEAPON")`,
  so a legacy header included earlier cannot break generated code.

---

## 8. TypeScript

### 8.1 File

- One `.ts` file per emit, ESM, no default export. It starts with the marker (§2.5), then
  imports, then the helper block (§8.2), then declarations in the order of §2.7.
- Records are `interface`s with `readonly` properties; values are deep-frozen object literals
  (`canonFreeze`); maps are `CanonMap` (writes throw at runtime); tables and keyed lists are
  `CanonTable` values. `readonly` makes writes fail at compile time, freezing at run time (TS-01).
- The file uses only erasable syntax (no `enum`, no `namespace`, no parameter properties), so it
  also runs under Node's type stripping.
- `data` mode exports, per value, `parse<V>(text: string)` and `decode<V>(json: unknown)`, and
  performs no I/O (TS-01); both throw on a `$schema` mismatch. `parse` reads the JSON text with the
  file's own tokenizer, so it reads exactly what WIRE.md writes: integers past 2^53 into
  `@ts(bigint)` fields, map members in file order, `2.0` or `1e3` for an `Int` refused (`E7103`),
  durations by their exact decimal. `decode` takes an already parsed value and cannot see those:
  a `@ts(bigint)` integer past 2^53 is refused, integer-like map keys come in JavaScript's order,
  and number tokens are judged by value (DECISIONS 278).
- `types` mode exports `parse<T>(text)` beside `decode<T>(json)` for every type it decodes, with
  the same exactness (DECISIONS 278).
- Decoders use private helpers (`dec…`), written after the export fns, only those a file uses;
  §8.2's block is the shared runtime, and these are the decoders' own (DECISIONS 278).

### 8.2 Helper block

The file contains the helpers below that it uses, in this order and with this exact text, and no
others (so that `noUnusedLocals` passes). `CanonEvalError` and `CanonTable` are always present
and exported.

```ts
// ---------------------------------------------------------------------------
// canon runtime, TypeScript v1 (CODEGEN.md §8.2). Identical in every generated file.
// ---------------------------------------------------------------------------

/** An error the Canon evaluator would also report, with its code ("E4101"). */
export class CanonEvalError extends Error {
  readonly code: string;
  constructor(code: string, message: string) {
    super(code + ": " + message);
    this.name = "CanonEvalError";
    this.code = code;
  }
}

function canonFail(code: string, message: string): never {
  throw new CanonEvalError(code, message);
}

/** Every Int crossing a translated function is a safe integer; -0 becomes 0. */
function canonInt(x: number): number {
  if (!Number.isSafeInteger(x)) canonFail("E8303", "integer outside the TypeScript safe range");
  return x === 0 ? 0 : x;
}

function canonAdd(a: number, b: number): number { return canonInt(a + b); }
function canonSub(a: number, b: number): number { return canonInt(a - b); }
function canonMul(a: number, b: number): number { return canonInt(a * b); }
function canonDiv(a: number, b: number): number {
  if (b === 0) canonFail("E4102", "integer division by zero");
  return canonInt(Number(BigInt(a) / BigInt(b)));
}
function canonMod(a: number, b: number): number {
  if (b === 0) canonFail("E4102", "integer division by zero in %");
  return canonInt(Number(BigInt(a) % BigInt(b)));
}
function canonNeg(a: number): number { return canonInt(-a); }
function canonAbs(a: number): number { return canonInt(Math.abs(a)); }
function canonClamp(x: number, lo: number, hi: number): number {
  if (lo > hi) canonFail("E4108", "clamp with lo > hi");
  return x < lo ? lo : x > hi ? hi : x;
}

/** Any Float operation whose result is NaN or infinite is E4104. */
function canonF(x: number): number {
  if (!Number.isFinite(x)) canonFail("E4104", "float result is not finite");
  return x;
}
function canonMinF(a: number, b: number): number {
  if (a < b) return a;
  if (b < a) return b;
  return Object.is(a, -0) ? a : b;
}
function canonMaxF(a: number, b: number): number {
  if (a > b) return a;
  if (b > a) return b;
  return Object.is(a, -0) ? b : a;
}
function canonClampF(x: number, lo: number, hi: number): number {
  if (lo > hi) canonFail("E4108", "clamp with lo > hi");
  return canonMinF(canonMaxF(x, lo), hi);
}
/** Int(f): truncates toward zero. */
function canonToInt(f: number): number {
  if (!(f >= -9223372036854775808 && f < 9223372036854775808)) canonFail("E4103", "float out of the Int range");
  return canonInt(Math.trunc(f));
}
function canonFloor(f: number): number { return canonToInt(Math.floor(f)); }
function canonCeil(f: number): number { return canonToInt(Math.ceil(f)); }
/** Rounds half away from zero. */
function canonRound(f: number): number { return canonToInt(Math.sign(f) * Math.round(Math.abs(f))); }
function canonDivDuration(a: number, b: number): number {
  if (b === 0) canonFail("E4102", "duration division by zero");
  return canonF(a / b);
}
/** Range refinement of a parameter or a result: E3204. */
function canonCheckRange(v: number, lo: number, hi: number): number {
  if (v < lo || v > hi) canonFail("E3204", "value outside its refinement range");
  return v;
}
/** Range of a sized integer type: E3201. */
function canonCheckWidth(v: number, lo: number, hi: number): number {
  if (v < lo || v > hi) canonFail("E3201", "value does not fit its sized integer type");
  return v;
}
/** Stores a Float into a Float32 (nearest-even); overflow is E3202. */
function canonF32(v: number): number {
  const f = Math.fround(v);
  if (!Number.isFinite(f)) canonFail("E3202", "value overflows Float32");
  return f;
}

/** A read-only Map: set, delete and clear throw. */
class CanonMap<K, V> extends Map<K, V> {
  constructor(entries: ReadonlyArray<readonly [K, V]>) {
    super();
    for (const [k, v] of entries) super.set(k, v);
    Object.freeze(this);
  }
  override set(_k: K, _v: V): this { throw new TypeError("canon: read-only map"); }
  override delete(_k: K): boolean { throw new TypeError("canon: read-only map"); }
  override clear(): void { throw new TypeError("canon: read-only map"); }
}

/** A table or keyed list: entries in source order, lookup by key. */
export interface CanonTable<K, T> {
  readonly length: number;
  readonly all: ReadonlyArray<T>;
  at(i: number): T;
  find(key: K): T | null;
}

function canonTable<K, T>(rows: ReadonlyArray<T>, keyOf: (row: T) => K): CanonTable<K, T> {
  const index = new Map<K, number>();
  rows.forEach((row, i) => index.set(keyOf(row), i));
  return Object.freeze({
    length: rows.length,
    all: rows,
    at: (i: number): T => rows[i],
    find: (key: K): T | null => {
      const i = index.get(key);
      return i === undefined ? null : rows[i];
    },
  });
}

/** Deep-freezes plain objects and arrays (CanonMaps are already read-only). */
function canonFreeze<T>(x: T): T {
  if (typeof x === "object" && x !== null && !Object.isFrozen(x)) {
    Object.freeze(x);
    for (const v of Object.values(x)) canonFreeze(v);
  }
  return x;
}

/** Checks the "$schema" of a data file against the fingerprint compiled in. */
function canonEnvelope(json: unknown, schema: string, name: string): Record<string, unknown> {
  const doc = json as Record<string, unknown> | null;
  const got = doc !== null && typeof doc === "object" && typeof doc["$schema"] === "string" ? doc["$schema"] : "<none>";
  if (got !== schema) {
    throw new Error(name + ": built from schema " + got + ", this binary expects " + schema + ". Rebuild the data or deploy the matching binary");
  }
  return doc as Record<string, unknown>;
}
// ---------------------------------------------------------------------------
```

### 8.3 Sample (teamboard, abridged)

```ts
// GENERATED by canon from teamboard/. DO NOT EDIT.
import type { Icon, Tone } from "../ui.generated.js";
import type { Role } from "../roles.generated.js";

export type StatusId = "open" | "taken" | "fixed" | "verified" | "wont_do" | "duplicate";

/** One point of the post lifecycle. */
export interface Status {
  readonly id: StatusId;
  readonly retired: boolean;
  readonly tone: Tone;
  readonly label: string;
  /** A terminal status closes the post. It can still be reopened through `next`. */
  readonly terminal: boolean;
  /** The closed set of statuses a post may move to from here. */
  readonly next: ReadonlyArray<StatusId>;
  /** Inputs a transition INTO this status must collect. */
  readonly requires: ReadonlyArray<PostField>;
  readonly optional: ReadonlyArray<PostField>;
  /** Who, besides a triager, may make the transition into this status. */
  readonly by: Actor | null;
}

export const statuses: CanonTable<StatusId, Status> = canonTable(canonFreeze<ReadonlyArray<Status>>([
  { id: "open", retired: false, tone: "warning", label: "Open", terminal: false, next: ["taken", "wont_do", "duplicate"], requires: [], optional: [], by: null },
  // …
]), (e) => e.id);

/** sovcommon's CanTransition. */
export function canTransition(from: StatusId, to: StatusId): boolean {
  return CAN_TRANSITION[StatusIdIndex[from] * 6 + StatusIdIndex[to]];
}
```

---

## 9. Toolchain floors

| Target | Floor | Why |
|---|---|---|
| Go | the current release only: generated Go is built, vetted and smoke-tested with the installed Go; no older release is targeted (DECISIONS 205) | no consumer needs an older release |
| C++ | C++17 on GCC ≥ 9, Clang ≥ 10, MSVC 19.20 (VS 2019) | inline variables, `std::optional`, `std::string_view`, `if constexpr` |
| nlohmann/json | ≥ 3.9 | `json_fwd.hpp`, `parse(…, allow_exceptions)` |
| TypeScript | ≥ 5.0, ESM, target ES2020 or later | `override`, `BigInt`; conformance tests use `node:test` (Node ≥ 20) |

Float exactness flags for C++ (`-ffp-contract=off`, `/fp:precise`, SSE2 on 32-bit x86) are in
CONFORMANCE.md §5. Generated code compiles warning-free with `-Wall -Wextra -Wpedantic` (GCC,
Clang) and `/W4` (MSVC).

---

## 10. Goldens

`examples/pipeline/expected/` holds the reference output of `canon build` for
`examples/pipeline/potion.canon`, regenerated for this document:

| File | Owner | Notes |
|---|---|---|
| `pipeline.gen.h`, `pipeline.gen.cpp` | this document | C++ `data` mode, snapshot and store |
| `pipeline_conformance.gen.cpp` | CONFORMANCE.md | vectors of CONFORMANCE.md §6 |
| `canon_runtime.h`, `canon_runtime_json.h` | this document | §7.4, §7.5 |
| `go/potions.gen.go` | this document | Go `data` mode, snapshot and store |
| `go/potions_conformance_test.go` | CONFORMANCE.md | |
| `go/rt/rt.go` | this document | §6.3 |
| `potions.json` | WIRE.md | regenerated here following WIR-01 and WIR-06; its byte layout is WIRE.md's to fix |
| `potion.view.json` | VIEWMODEL.md | not touched |

- The fingerprint in every golden is `pipeline.Potion@f750790e` (FINGERPRINT.md vector 1), the
  same in `potions.json`, `kPotionsSchema` and `PotionsSchema`.
- The Go goldens assume a root `pipeline_go: "pipeline/out/go"` and `go_module { pipeline_go:
  "example.com/potions" }` in `project.canon`, so `rt` is imported as `example.com/potions/rt`.
- Verified with: `g++` 15 and `clang++` 21, `-std=c++17 -Wall -Wextra -Wpedantic -Werror`, with and
  without `-fno-exceptions`, also `-std=c++20` and `-fsanitize=address,undefined`; loading the
  data, the schema-mismatch and missing-file errors, `RunPipelineConformance() == 0`. Go 1.25 with
  `go 1.23` in `go.mod`: `gofmt -l` clean, `go vet ./...` clean, `go test -race ./...` passing.
  (This record predates DECISIONS 205, which drops the Go 1.23 floor.)

---

## 11. Changes needed in other documents

This section listed what the other documents had to restate. The consistency pass of 2026-09-23
applied every item; each rule now lives in its owning document:

| Rule | Where it is stated |
|---|---|
| generated headers per §2.4, no line numbers (§1.3) | SPEC §15.1 |
| Go `Load<V>(path)`, baked accessors `Get<V>()`, `At`, `Get(id)`, the `rt` package | SPEC §15.2 |
| C++ `<Enum>FromWire`, `ToName`, `const X& GetX()`, `canon_runtime_json.h`, public default constructors | SPEC §15.3 |
| TS `CanonTable` values, the `branch` discriminant | SPEC §15.4 |
| one `<P>Snapshot` and one `<P>Store` per package, `Reload(dir)` | SPEC §15.5 |
| conformance file names | SPEC §15.6, CONFORMANCE.md §7.1 |
| the branch enum `<Alias>Branch` | TYPES.md §11.7, SPEC §5.11 |
| Go errors are `*rt.EvalError` | DECISIONS 19 |
| `--adopt` on `canon build` and `canon convert` | CLI.md §3.4, §3.10 |
| `@cpp(value:)`, `@cpp(unit:)` | GRAMMAR.md §8.3 |
| the `Duration` range ±9 223 372 036 854 ms | TYPES.md §7.2, EVALUATION.md §6.3 |
| `-0.0` before `+0.0` in `min`, `max`, `clamp` | STDLIB.md §2.2 |
| the pipeline fingerprint `pipeline.Potion@f750790e` | WIRE.md §8.3, FINGERPRINT.md §7, the goldens (§10) |
| a root and a `go_module` entry for every Go emit | `examples/project.canon` |
| `emit cpp { mode: types }` for `sovcommon.time` (`E8004`, `E8018`) | `examples/sovcommon/time/time.canon` |

---

## 12. Diagnostics

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E8001 | error | the target file exists and its first line is not a canon marker (§2.4) |
| E8002 | error | more than one `emit` per target per package |
| E8003 | error | target or option not in §2.1 |
| E8004 | error | cross-package type without an emit for the same target, or without a copy this copy can use (§2.8) |
| E8005 | error | two generated names equal in one scope (§3.5) |
| W8006 | warning | §3.5 list |
| E8007 | error | Go emit outside every mapped root |
| E8008 | error | two Go emits write into one directory (one Go package); two outputs with one path are WIRE.md's `E8152` |
| E8009 | error | bad mode, package, namespace, out or values; an option value of the wrong kind; an invalid `out` list: empty, two entries sharing an owning root, different last elements without `package`, JSON files mixed with directories (§2.1, §2.8) |
| E8010 | error | `ordered` + `@codes` out of order |
| E8011 | error | override not an identifier, reserved, or (Go) not exported; a name derived without override that is not an identifier (§3.5) |
| E8012 | error | `Range`, `Pair`, function type, `_`, non-optional `Never` in an emitted type or value; a `Define` record or define table in a `data` or `embedded` emit (§4.4) |
| E8013 | error | package-level export fn, or a finite method with a `ref` parameter, in `data` mode |
| E8014 | error | precomputed or finite export fn, or computed default, in `types` mode |
| E8015 | error | other value types in `data`/`embedded` |
| E8017 | error | §5.6 |
| E8018 | error | §2.8 decoders across packages |
| E8101 | error | emitted Int outside ±(2⁵³−1) (SPEC §15.4) |
| E8103 | error | string or list longer than a fixed-size C++ array |
| E8104 | error | package with inputs has a TS emit |
| E8106 | error | value outside a legacy member's range |
| E8107 | error | §7.8.1 |
| E8108 | error | inline variant on a legacy struct |
| E8109 | error | missing `header`, bad `access`, unmappable field |
| E8201 | error | SPEC §15.5 |
| E8202 | error | RLD-02 |
| E8301 | runtime | signalled by `embedded` accessors |
| E8302 | runtime | input getter before `LoadInputs` |
| E8303 | runtime | TS translated code (CONFORMANCE.md §4) |

Codes `E9001`–`E9009` (portability of translated functions) are in CONFORMANCE.md §8; `E8102` and
`E8150`–`E8153` are in WIRE.md §13. The single catalogue is [ERRORS.md](ERRORS.md).
