# Emit

`canon build` runs `check`; with no error in any loaded package it writes every `emit` output of
the selected packages and appends new stable ids to `canon.lock`. `emit view` is written even
with errors. Outputs are written atomically and only when their bytes change.

```canon project.canon
project demo {
  canon: "0.1"
  roots {
    gen: "gen"
  }
  go_module {
    gen: "example.com/demo/gen"
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

emit go { out: "@gen/store", package: "store", mode: baked }
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
- `out` is `@root/...` or relative to the file; a list of paths writes one copy per entry.
- A code target always emits every public type, enum, constant and `export fn`; `values`
  (default: every public `let`) selects values.
- A package whose types another package's output uses must emit the same target (`E8004`).
- Go needs `go_module` for the root holding `out` (`E8007`).
- `values: []` is refused (`E8009`): omit `values` for every value.
- Generated: `go` and `cpp` in `baked` and `data`, `cpp` also in `types`, `ts`, `json`, `view`.
  `canon check` refuses what no generator builds, with a message naming the way out (`E8019`):
  Go `embedded` and `types`, C++ `embedded` (use `baked` or `data`); legacy C++ structs
  (`@cpp(struct:)`); a dependent value decided through a ref, an optional or a record parameter,
  in any go/cpp emit (decide it by an enum field of the same record; owed for v0.2).
- Shared records: a record, variant, dependent type or table of another package is usable in
  values and data in every target and mode; do not copy the type. A package reached only through
  another's types needs the same emits in its imports. Still `E8019`: a foreign record whose ref
  the owner's loader resolves, and a lookup over a foreign table without an id enum. The
  generated make hooks (Go `Make_<T>`, C++ `detail::<P>Make`) are for generated code, not API.

## Modes

| Mode | Runtime gets | Table ids |
|---|---|---|
| `baked` (default) | values compiled into the binary | an id enum |
| `embedded` | refused by `check` in this release (`E8019`); use `baked` or `data` | an id enum |
| `data` | a separate data file plus a loader that checks its fingerprint | strings |
| `types` | types plus a decoder; the runtime reads its own file; C++ only (Go: `E8019`) | strings |

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
`Float`, `String`, `Duration`, enums (`E9006`); a translated method must be called by a `test`
of its package (`E9008`), which supplies the conformance vectors.

## What is written

- Generated files start with a marker (Go `// Code generated by canon`, C++ `// GENERATED by
  canon`, JSON a `"$schema": "<name>@<8 hex>"` key). Never edit them: change the source and
  rebuild. `canon build` refuses to overwrite a file without one (`E8001`; `--adopt <path>`
  takes over a legacy C++ header or a text file). `canon build --check` (CI) writes nothing, exit 1 if stale.
- JSON data: `{"$schema", "rows": [...]}` for a list, table or keyed list (table rows start
  with `"$id"`, then `"$retired": true`), `"value"` otherwise; every field written, wire names
  applied, precomputed `export fn` results as `"$<name>"` keys, package-level ones in `"$fns"`.

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
@gen/text/_version.sql
@gen/text/limits.json
```

- `@text("<file>")` goes on a public package-level `export fn` with no parameter and a result
  that is a `String` or has a wire form (`E8021` otherwise; `E8151`, `E8102` for a part with no
  wire form). The file name has no `/` or `\` and is not `.` or `..`; two names of one package
  equal ignoring case are `E8021`. `emit text` takes only `out` (a directory, or a list of
  copies); with no `@text` fn in the package it is `E8009`.
- A `String` result is written verbatim (UTF-8). Any other result is written as plain JSON
  (`canon guide data`, wire names): no envelope, no `$schema`; fields in declaration order under
  their wire names, a table as an object keyed by entry id, then one `\n`. A field that is
  `none` with `none` as its default is left out (no key); with `@json(none: X)` it is written
  `X`; `T? = d` holding `none` is `null`. `none` in a list or a map is `null`. A Float has one
  canonical spelling (`1.0` is written `1`): there is no option for indentation or float format,
  and `load` of the file at the result type gives the value back.
- A `@text` fn is a leaf: the go, cpp, ts and json emits skip it, and nothing but a `test` may
  call it (`E8021`).
- Prefer a typed result to building a string: the compiler checks the value, writes the names and
  omits absent fields, so no hand-written serializer (quotes, commas, escapes) is needed. A
  `String` result is for formats Canon does not write.
- Ownership: the package also writes `<pkg>/canon.outputs` (beside `canon.lock`): the paths of the
  text files it wrote. Commit it. The build overwrites a text file only if it is absent or listed
  there; an existing file that is not listed is `E8001`: `canon build --adopt <path>` takes it
  over, once. Files no longer written stay on disk (`canon` never deletes an output) and leave
  the list. Two packages may write into one directory; one path written twice is `E8152`. A
  `.canon-text` left in an output directory by a 0.1 build counts as ownership once and is
  deleted.
