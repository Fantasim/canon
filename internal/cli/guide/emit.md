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
  takes over a legacy C++ header). `canon build --check` (CI) writes nothing, exit 1 if stale.
- JSON data: `{"$schema", "rows": [...]}` for a list, table or keyed list (table rows start
  with `"$id"`, then `"$retired": true`), `"value"` otherwise; every field written, wire names
  applied, precomputed `export fn` results as `"$<name>"` keys, package-level ones in `"$fns"`.
