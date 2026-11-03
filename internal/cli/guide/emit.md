# Emit

`canon build` runs `check`; with no error in any loaded package it writes every `emit` output of
the selected packages and appends new stable ids to `canon.lock`. `emit view` is written even
with errors. Outputs are written atomically and only when their bytes change.

```canon project.canon
project demo {
  canon: "0.1"
  roots {
    gen: "gen"
    mirror: "mirror"
  }
  go_module {
    gen: "example.com/demo/gen"
    mirror: "example.com/demo/mirror"
  }
  out_templates {
    go: {
      gen: "{dir}"
      mirror: "go/{dir}"
    }
  }
}
```

```canon store/store.canon
package store

/// A product.
record Product {
  /// Shown name.
  name: String(1..)
  /// Price in cents.
  price: Int(0..)
  /// Units in one pack.
  pack: Int(1..) = 1

  /// No runtime input: computed at build time, emitted as a getter.
  export fn unitPrice(self) -> Int { return price / pack }

  /// A runtime input (an Int): the body is translated into each target, with a conformance test.
  export fn priceFor(self, count: Int) -> Int { return price * max(count, 0) }
}

let products: table Product = {
  pen { name: "Pen", price: 120 }
  ink { name: "Ink", price: 900, pack: 3 }
}

/// Finite inputs (an enum, a Bool, a ref into a table): a lookup table over every input.
export fn isCheap(p: ref products) -> Bool { return p.price < 500 }

test "priceFor counts whole units" {
  expect products.pen.priceFor(3) == 360
  expect products.pen.priceFor(-1) == 0
}

emit go { out: [gen, mirror], package: "store", mode: baked }
emit json { out: "@gen/data/" }
```

## Targets and options

| Target | `out` | Options |
|---|---|---|
| `go` | directory | `mode`, `package` (default: last element of `out`), `values` |
| `cpp` | directory | `mode`, `namespace` (default: package path with `::`), `values` |
| `ts` | a `.ts` file | `mode`, `values` |
| `json` | a `.json` file (one value) or a directory (`<value>.json` each) | `values` |
| `view` | a file | none |
| `text` | a directory | none (`## Text files`) |

- One emit per target per package (`E8002`); unknown option `E8003`; bad value `E8009`.
- `out` is `@root/...` or relative to the file; a list of paths writes one copy per entry. Every
  segment of the project-relative path an `out` resolves to uses only `A-Z a-z 0-9 . _ -`, else
  `E8009` `outChars`. That path includes the root's own path and, for an `out` with no root, the
  directory of the file holding the emit. An `out` that does not resolve is `E7001` alone. The
  path is written into `#include`s, Go imports and TypeScript import specifiers, which take no
  escapes.
- `out` may name a root instead of a path (`out: [gen, mirror]`): the path comes from the
  `out_templates` of `project.canon`, as in the project above (`go: { gen: "{dir}" }`), where
  `{dir}` is the package path as a directory (`store`, `items/vocab`) and `{dotted}` the package
  path as written. A root name with no template for this target is `E8009` `outTemplate`; a
  quoted string equal to a root name that has one is `E8009` `outQuoted` (write the name
  unquoted, or `"./<name>"` for the directory beside the file). An invalid target or template is
  `E1019` (a placeholder other than `{dir}` and `{dotted}`, a template starting with `@`, a
  path with an empty, `.` or `..` segment, a `ts` template not ending in `.ts`).
- A code target always emits every public type, enum, constant and `export fn`; `values`
  (default: every public `let`) selects values.
- A package whose types another package's output uses must emit the same target (`E8004`).
- Go needs `go_module` for the root holding `out` (`E8007`).
- `values: []` is refused (`E8009`): omit `values` for every value.
- Generated: `go` and `cpp` in `baked`, `data` and `types`, `ts`, `json`, `view`.
  `canon check` refuses what no generator builds, with a message naming the way out (`E8019`):
  `embedded` in Go and C++ (use `baked` or `data`); legacy C++ structs
  (`@cpp(struct:)`); a dependent value decided through an optional, a record parameter, a ref
  into a `@reload` value or a ref into an enclosing record's table field (decide it by an enum
  field of the same record, or by a ref into a let that is not `@reload`). Through any other ref
  it is generated: the ref's targets are known at build time, so the code holds a private table
  from the ref's key to the branch, and a key it lacks fails with `no branch for this value`.
- A `const` of an optional or a `ref` type is `E8019` in a go or cpp emit (`OptionalConstant`,
  `RefConstant`; TypeScript writes `T | null` or the key): make it a field of a record `let`. A
  `const` whose type holds a dependent type, in a list, a map or an optional too, is `E8019` in
  every code emit (`DependentConstant`): hold the value in a record field beside its
  discriminant, in a `let` of that record.
- Shared records: a record, variant, dependent type or table of another package is usable in
  values and data in every target and mode; do not copy the type. A package reached only through
  another's types needs the same emits in its imports. Still `E8019`: a foreign record whose ref
  the owner's loader resolves, and a lookup over a foreign table without an id enum. The
  generated make hooks (Go `Make_<T>`, C++ `detail::<P>Make`) are for generated code, not API.
- A `ref` into another package's table is typed with that package's id for the target: its id
  enum when that package emits `baked`, a string otherwise. A Go or C++ loader or decoder of the
  holding package refuses a key that names no id of an enum (`unknown value <key>`).

## Modes

| Mode | Runtime gets | Table ids |
|---|---|---|
| `baked` (default) | values compiled into the binary | an id enum |
| `embedded` | refused by `check` in this release (`E8019`); use `baked` or `data` | an id enum |
| `data` | a separate data file plus a loader that checks its fingerprint | strings |
| `types` | types plus a decoder per public record (Go `Decode<X>(raw)`); the runtime reads its own file, e.g. a `@text` file | strings |

In `data` mode a value change rebuilds only the data file. `@reload` on a `let` (data mode only,
`E8202`) lets a runtime swap it while running; it must be written by the package's `emit json`.
`data` mode cannot hold a package-level `export fn` or a finite method with a `ref` parameter
(`E8013`). `types` mode cannot hold precomputed results or computed defaults (`E8014`).

## export fn

| Parameters besides `self` | Emitted as |
|---|---|
| none | a getter returning the value computed at build time |
| only `Bool`, enums, `ref` into a table | a lookup table over every combination (at most 65 536 cells, `E9002`) |
| at least one `Int`, `Float`, `String` or `Duration` | the body translated to each target, plus a conformance test |

A translated body uses only `let`, `if`/`else`, `return`, arithmetic and comparisons, `Float(i)`,
`Int(f)`, `min`, `max`, `abs`, `floor`, `ceil`, `round`, `clamp`, string templates over `String`,
`Int` and enums, `and` `or` `not` `??`, fields of `self` and parameters, enum members and
calls to other `export fn` of the package (`E9001` otherwise). Parameters: `Bool`, integers,
`Float`, `Float32`, `String`, `Duration`, enums (`E9006`); a translated method must be called by a
`test` of its package (`E9008`), which supplies the conformance vectors.

## What is written

- Generated files start with a marker (Go `// Code generated by canon`, C++ `// GENERATED by
  canon`, JSON a `"$schema": "<name>@<8 hex>"` key). Never edit them: change the source and
  rebuild. `canon build` refuses to overwrite a file without one (`E8001`; `--adopt <path>`
  takes over a legacy C++ header or a text file). `canon build --check` (CI) writes nothing, exit 1 if stale.
- JSON data: `{"$schema", "rows": [...]}` for a list, table or keyed list (table rows start
  with `"$id"`, then `"$retired": true`), `"value"` otherwise; every field written, wire names
  applied, precomputed `export fn` results as `"$<name>"` keys, package-level ones in `"$fns"`.

- Generated C++ never lets source text change how C++ reads it: a doc line ending in `\` is
  written as a `/** */` block; in docs `??/` is written `?\?/`, and in string and character
  literals every `?` that follows a `?` is written `\?` (DECISIONS 423).
- A `const` of a variant's kind (`Shape.dot.kind`) is written as the kind enum's member in every
  code target: a Go `var`, a C++ `inline constexpr`, a typed TypeScript `export const`; a list or
  map of kinds follows its list or map row.
- Every C++ output directory holds a `canon_runtime.h` stamped by its content: regenerate every
  C++ output with one canon, because a translation unit that includes copies of two different
  runtime texts stops at compile time with canon's `#error`. A copy written by canon 0.2 or earlier
  has no stamp, so a mix with one is caught only when the stamped copy is included last: regenerate
  all at once.
- A `load.dir` pattern that matches a path an emit of the build writes is `E8026` at the load,
  before the output exists: narrow the pattern, or move the emit's `out` outside the load's base.

## Decoders check refinements, and bounds are constants

Every decoder the generators write refuses a value outside the range or length refinement of its
type at its position, in Go, C++ and TypeScript: `data` loaders and snapshots, `types`-mode
decoders (`Decode`, `Parse`, `parse`, `Decode<Fn>File`), TypeScript `decode<V>` and `parse<V>`, and
the readers of other packages' classes. Baked data has no decoder, so nothing to check.

```canon limits/limits.canon
/// Limits of a profile, written as constants in every target.
package limits

/// How loud a profile is.
enum Tone {
  /// Quiet.
  warning
  /// Normal.
  info
  /// Loud.
  series_1
}

/// A profile.
record Profile {
  /// Display name, 1 to 24 bytes.
  name: String(1..=24)
  /// Tags, at most 8, each 0 to 5.
  tags: [Int(0..=5)](..=8) = []
  /// Level; the upper bound is exclusive.
  level: Int(1..120) = 1
  /// Time between two uses.
  cooldown: Duration(1s..=1h) = 1s
  /// How loud.
  tone: Tone = info
}

emit go { out: "@gen/limits/go", package: "limits", mode: types }
emit cpp { out: "@gen/limits/cpp", namespace: "limits", mode: types }
emit ts { out: "@gen/limits/limits.ts", mode: types }
```

- Checked: a record or case field, a list or keyed-list element, a map key and value, a dependent
  branch, the base of a literal union (its literals pass), a stored `$` result or lookup cell, a
  `@json(pairs:)` or `@json(bits)` list, the rows of a data file's top-level list or keyed list, a
  `@text` result. Measured: an integer of any width; a `Float`; a `Float32` as the binary32 value
  stored, compared as binary64 with the bound; a `Duration` in milliseconds after its unit; a
  `String` in UTF-8 bytes; a list, keyed list or map by element count.
- Not checked: `none` and a default taken for an absent key; patterns, `where`, assets, refs and
  checks, which run only in Canon; make hooks; `LoadInputs` keeps its own checks.
- The message is one of `expected a value in <R>`, `expected a byte length in <R>`,
  `expected an element count in <R>`, with `<R>` the refinement in Canon's text (`1..=120`, `0..1`,
  `..=8`, `1s..=1h`), byte for byte the same in every target and mode:
  `Profile: $.tags[2]: expected a value in 0..=5`,
  `items.json: rows: expected an element count in ..=500`.
  A refinement is judged after the value's own type rules, so `Int8(0..=5)` reading 200 keeps the
  width message and reading 7 fails the refinement; a collection reports its elements, keys and
  duplicates before its count (a map is decoded entry by entry, key then value); the first
  failure in decode order is the one reported.
- The `$schema` fingerprint does not include refinements. A data file built before a refinement
  narrowed still loads where its values fit: the loader refuses each value outside the new
  refinement at its path. Widening a refinement keeps every older file valid.
- TypeScript decoders also refuse a repeated id of a keyed list or a data table, with the bytes of
  Go and C++ (`items.json: rows[1].name: duplicate id "a" (first at rows[0])`), and a map key `-0`
  or any non-canonical integer key (`expected a canonical decimal integer key`). A canonical key
  outside the JS safe range reads `integer outside the TypeScript safe range` there, TypeScript's
  documented limit.
- A failure under an empty map key names the key as `<parent>.` in every target (`$.m.` for the
  key or a scalar value, `$.m..v` for a field of a record value).
- The C++ `Decode` of a `types`-mode record takes an already-parsed value, an `nlohmann::json` or an
  `nlohmann::ordered_json` (`Decode(const nlohmann::ordered_json&, std::string&)`). Handed a
  `json`, whose objects are ordered by key, a nested table or a map it decodes holds its entries
  in key byte order; handed an `ordered_json`, it holds them in the order that value holds, the
  file's for one parsed from text, as `Parse` (which reads the text itself) does. Neither sees a
  repeated key or a number token.

**Bound constants.** Each finite side of a field's refinement is a constant, for every field of a
public record or variant case an emit writes: `Min`, `Max` (for `..=`), `End` (for `..`), and for a
length `LenMin`, `LenMax`, `LenEnd`; one level down a list or keyed-list element, a map key or a
map value, the name gains `Elem`, `Key` or `Value` (`TagsElemMax`). The value is the bound as
written, never shifted or clamped to the width. For `Profile` above:

| | Go | C++ | TypeScript |
|---|---|---|---|
| `name` byte length, at most 24 | `Profile_NameLenMax` | `Profile::kNameLenMax` | `Profile_NameLenMax` |
| each of `tags`, at most 5 | `Profile_TagsElemMax` | `Profile::kTagsElemMax` | `Profile_TagsElemMax` |
| `level`, exclusive end | `Profile_LevelEnd` | `Profile::kLevelEnd` | `Profile_LevelEnd` |
| `cooldown`, at least 1s | `Profile_CooldownMin` (`1000 * time.Millisecond`) | `Profile::kCooldownMin` (`std::chrono::milliseconds`) | `Profile_CooldownMin` (ms) |

Integers and lengths are untyped Go constants, C++ `int64_t` and TypeScript `number` (`bigint` at a
`@ts(bigint)` position); floats are `float64`, `double` and `number`. Aliases, values, consts, fns
and dependent branches have none. Tie a hand-sized array to the bound, so a change of the Canon
refinement breaks the build instead of a buffer:

```c
std::array<int64_t, limits::Profile::kTagsLenMax> tags;
static_assert(limits::Profile::kNameLenMax <= 32, "Profile.name no longer fits the wire header");
```

- Every name is reserved whether or not it is written (`E8005`, `both`, at the field).
- A refinement never changes the generated type (GEN-02): `Int(1..=100_000)` is `int64`. Declare a
  width with a sized type: `UInt32(1..=100_000)` is a `uint32` in Go and `uint32_t` in C++.

## Generated enums

An enum without `@codes`, a variant's kind enum and a dependent type's branch enum are opaque
wherever their values would be declaration positions. A declaration position is not an id: it
changes when members move.

```text
// Go
type Tone struct{ position uint8 }
var ToneWarning = Tone{0}               // members are vars; the zero value is the first member
func (self Tone) String() string        // Canon name
func (self Tone) Wire() string          // wire value
func (self Tone) Position() int         // declaration position, for array indexes in memory
func ParseTone(wire string) (Tone, bool)
func ToneMembers() iter.Seq[Tone]       // declaration order, retired members included

// C++: enum class as before, but nlohmann/json refuses it
template <typename J> void to_json(J&, Tone) = delete;
template <typename J> void from_json(const J&, Tone&) = delete;

// TypeScript: no ToneIndex export
export const ToneMembers: ReadonlyArray<Tone>;
```

- Go: no integer converts to or from one, and `<`, arithmetic, `const`, `iota` and `%d` no longer
  mean a position. `==`, `switch` with member cases and map keys work; `encoding/json` writes the
  wire value (a branch enum writes the branch name); `database/sql` and `gob` refuse the type.
  A Canon `const` of such a type is a Go `var`. An `ordered` enum's members compare by
  `Position()`; use it for an array index in memory and never store it.
- C++: store `ToWire(v)` and read `ToneFromWire(s)`; casts stay legal.
- TypeScript: no enum exports `<E>Index`. A program that needs the order reads
  `ToneMembers.indexOf(v)` (`-1` for a value outside the enum).
- `@codes` enums keep their integer, the declared and locked code, in Go and C++; open enums keep
  their wire.
- Go's `exhaustive` linter no longer sees these types: they are not constants. Canon's `match`,
  C++ `-Wswitch`, TypeScript string-union narrowing and the generated tables stay exhaustive. If
  a rule says "a new enum value must break the build", give each Go `switch` a `default:` that
  returns an error, and a test that ranges over `ToneMembers()` through the switch, so a new member
  fails the test.

## TypeScript in a bundle

Every `Object.freeze`, `canonFreeze`, `canonList`, `canonTable`, `canonIndex`, `new CanonMap` and
`new Map` that builds a module-level value is written `/*#__PURE__*/ ` first, each call of a chain
on its own. A bundler that finds such a `const` unread drops it, so a program that imports one
constant of a generated module carries that constant alone, not every table of the module. A
generated module has no top-level expression statement and builds only immutable data.

Put a `package.json` in the output directory, which holds only generated files, so that a
bundler knows it:

```json
{ "type": "module", "sideEffects": false }
```

A bundler reads the nearest `package.json`: this one covers the files of its directory and below,
never the rest of the project (do not set `sideEffects` at the project root for it).
Without `type`, TypeScript (`module: nodenext`) compiles the generated `.ts` files, which are ESM, as
CommonJS (`TS1287` under `verbatimModuleSyntax`), and Node warns (`MODULE_TYPELESS_PACKAGE_JSON`):
keep `"type": "module"`.

- `removeComments` (a `tsconfig.json` option), a formatter or any step that strips comments
  before the bundler runs deletes the marks: the output is then as large as before v0.3, never
  wrong. Measured with esbuild, Rollup and Rolldown; webpack (`optimization.innerGraph`) and
  Terser were not measured, so look at a bundle before relying on them.
- A decoder's returned value is not marked (a bundler drops an unused function whole), and the
  `$union<N>` constant of a `data` or `types` decoder is the one top-level value no mark can cover.
- `<last>.conformance.test.ts` registers its `node:test` tests when loaded, by design: run it with Node's test
  runner and never import it from a program.
- Each generated file declares its own `CanonEvalError` and `CanonTable`. Tell an evaluation error
  by `e.name === "CanonEvalError"`, then read `e.code`, not by `instanceof` across files; and a barrel
  names its re-exports (`export { Item } from "./items.js"`), because `export *` over two generated
  modules is `TS2308`.

## C++ names and `#define`s

`@cpp(defines: "P")` on an enum or a `const` writes `<last>.defines.gen.h` for legacy code:
`#define APP_MAIN 0`. Generated headers guard the names a package, and the packages it reads,
define. A generated C++ name that another package, one this package does not read, writes as a
`#define` cannot be guarded, so it is `E8005` (`foreignDefine`, or `foreignId`, `foreignDerived` or
`foreignFixed` by how the name is built), at the declaration, whatever the selection:
`APP_MAIN is declared for hud.Screen.APP_MAIN and written as a #define for windows.Window.MAIN, in a
package this one does not read, so its headers cannot guard it; rename hud.Screen.APP_MAIN with
@cpp(name:), or use a type of that package here`.

```canon windows/windows.canon
/// Window ids shared with legacy C++ code.
package windows

/// The windows. `@cpp(defines:)` writes `APP_MAIN 0` and `APP_OPTIONS 1` into a defines header
/// and guards every generated header against those macros.
enum Window @cpp(defines: "APP_") {
  /// The main window.
  MAIN
  /// The options window.
  OPTIONS
}

emit cpp { out: "@gen/windows", namespace: "windows", mode: types }
```

```text out/gen/windows/windows.defines.gen.h
// GENERATED by canon from windows/. DO NOT EDIT.
#pragma once
#define APP_MAIN 0
#define APP_OPTIONS 1
```

```canon hud/hud.canon
/// A package that does not read `windows`.
package hud

/// Screens.
enum Screen {
  /// Spelled like a define of `windows`; renamed in C++.
  APP_MAIN @cpp(name: "AppMain")
  /// Another.
  CREDITS
}

emit cpp { out: "@gen/hud", namespace: "hud", mode: types }
```

- Ways out, the one the finding names: `@cpp(name:)` on the declaration (`foreignDefine`, as `hud`
  does); the Canon rename of the name a derived C++ name is built on, which `@cpp(name:)` does not
  change (`foreignDerived`: a value's, field's, fn's or parameter's name); renaming a table id's
  entry (`foreignId`); or, always, a use of a type of the defining package in this one, which
  makes this package read it and so guard its defines (the only way for `foreignFixed`, a name
  the generator fixes, such as `Get` or `Decode`).
- A package writes defines when its first `emit cpp` is `baked`, `data` or `types`. A local or a
  parameter of a translated fn is no generated name: the C++ compiler reports a clash there.
- A name met only in a comment, a string literal or a character literal is not wrapped.
- A header that `emit text` renders is plain text to Canon: it cannot be guarded. When it defines
  the names of enum members, use `@cpp(defines:)` on the enum instead (`canon guide recipes`).

## Text files: `emit text` and `@text`

For a runtime that reads its own plain format (a legacy JSON file, SQL, a header): the file is
computed at build time and written as is. No marker, no `$schema`, no loader.

```canon tools/tools.canon
/// Plain files for a runtime that reads its own format.
package tools

/// Server limits.
record Limits {
  /// Players at once.
  players: Int(1..)
  /// Queue length; absent from the file when none.
  queue: Int?
  /// Session length, written in seconds.
  timeout: Duration = 30s @json("timeoutSec", unit: s)
  /// Ratio, written `1` for `1.0`.
  ratio: Float = 1.0
}

/// A `String` result is the file's bytes: nothing added, not even a final newline.
@text("_version.sql")
export fn versionSql() -> String { return "SELECT 3 AS contract;\n" }

/// Any other result is plain JSON.
@text("limits.json")
export fn limitsFile() -> Limits { return { players: 100 } }

emit text { out: "@gen/text" }
```

Written by `canon build` (a text fence of the guide tests holds each file's exact bytes):

```text out/gen/text/limits.json
{
  "players": 100,
  "timeoutSec": 30,
  "ratio": 1
}
```

```text out/gen/text/_version.sql
SELECT 3 AS contract;
```

```text out/tools/canon.outputs
# GENERATED by canon from tools/. DO NOT EDIT.
d3667e8ea17079fba34688b9c6d7f6f4261a4dd41896eebe8d5afdeea8f0cea1  @gen/text/_version.sql
127be29e8cc168459e7d23643b466527f11117bd4667840da6df79cce24e60f9  @gen/text/limits.json
```

- `@text("<file>")` goes on a public package-level `export fn` with no parameter and a result
  that is a `String` or has a wire form (`E8021` otherwise; `E8151`, `E8102` for a part with no
  wire form). The file name has no `/` or `\` and is not `.` or `..`; two names of one package
  equal ignoring case are `E8021`; so is a name ending in `.canon`, which the next run would read
  as a source of the project: `invalid @text file name "gen.canon": a .canon file is read as a
  source of the project; give it another extension`. `emit text` takes only `out` (a directory,
  or a list of copies); with no `@text` fn in the package it is `E8009`.
- A `String` result is written verbatim (UTF-8). Any other result is written as plain JSON
  (`canon guide data`, wire names): no envelope, no `$schema`; fields in declaration order under
  their wire names, a table as an object keyed by entry id, then one `\n`. A field that is
  `none` with `none` as its default is left out (no key); with `@json(none: X)` it is written
  `X`; `T? = d` holding `none` is `null`. `none` in a list or a map is `null`. A Float has one
  canonical spelling (`1.0` is written `1`): there is no option for indentation or float format,
  and `load` of the file at the result type gives the value back.
- An optional result (`T?`, `String?` included) that is `none` writes no file and is not listed
  in `canon.outputs`: gate a file by a flag that way. The file an earlier build wrote is removed.
- A `@text` fn is a leaf: the go, cpp, ts and json emits skip it, and nothing but a `test` may
  call it (`E8021`).
- In `types` mode, a public `@text` fn whose result is a map, a list or a keyed list also gets a
  decoder for its file: Go and C++ `Decode<Fn>File`, TypeScript `decode<Fn>File` and
  `parse<Fn>File`. `<Fn>` is the fn's name in the target without a final `File`: `tipFile` gives
  `DecodeTipFile`, never `DecodeTipFileFile`, and its errors name `TipFile`. `items` and
  `itemsFile` in one package reach one name and are `E8005`: put `@go(name:)`, `@cpp(name:)` or
  `@ts(name:)` on one of them.
- Prefer a typed result to building a string: the compiler checks the value, writes the names and
  omits absent fields, so no hand-written serializer (quotes, commas, escapes) is needed. A
  `String` result is for formats Canon does not write.
- Ownership: the package also writes `<pkg>/canon.outputs` (beside `canon.lock`): each text
  file it wrote, as its SHA-256 and its path. Commit it. The build overwrites a text file only if it is absent or listed
  there; an existing file that is not listed is `E8001`: `canon build --adopt <path>` takes it
  over, once. A listed file no longer written (a `none` result, a renamed or removed `@text`
  fn) is removed when its bytes are still the ones `canon` wrote, and leaves the list; an
  unlisted or hand-edited file is never touched. Two packages may write into one directory; one path written twice is `E8152`. A
  `.canon-text` left in an output directory by a 0.1 build counts as ownership once and is
  deleted.
