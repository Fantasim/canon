# Layers and runtime inputs

## Layers

A layer changes values, never types: per-environment or per-machine variations of one law.

```canon app/app.canon
package app

/// The HTTP server.
record Server {
  /// Listening host.
  host: String = "127.0.0.1"
  /// Listening port.
  port: Int(1024..=65535) = 8080
  /// Extra response headers.
  headers: {String: String} = {}
}

/// Every setting.
record Config {
  /// The server.
  server: Server = {}
  /// Allowed origins.
  origins: [String] = ["https://example.com"]
  /// Database password: read from the environment by generated code, never written.
  dbPassword: input String(1..) from env "APP_DB_PASSWORD"
  /// Optional input: none when unset or empty.
  debugToken: input String? from env "APP_DEBUG_TOKEN"
}

let config: Config = {}
```

```canon app/staging.layer.canon
// canon check --layer staging; canon build --layer staging
package app
layer staging

amend config {
  server.port: 9000
  server.headers["X-Env"]: "staging"
  origins[0]: "https://staging.example.com"
}
```

- File: `package p`, then `layer <name>`, then `amend` blocks only (`E1127`). Convention:
  `<name>.layer.canon`. At most one file per layer name per package (`E1906`).
- `amend <let> { path: expr }`: a top-level `let` of the layer's own package (`E1909`), never a
  `const` (`E1902`). Path segments: `.field` or `.entry`, `[key]` or a plain list's `[index]`,
  `[#n]` a position. The path must exist (`E1905`), except that its last segment may add a new
  map or table key. List elements can be replaced, not appended; entries cannot be removed; a
  stable table cannot gain entries (`E6004`). A path set twice, or with its prefix, is `E1908`.
- The expression is typed against the path; the amended value then passes every refinement and
  check again: a layer cannot produce an invalid config. Defaults of later fields that depend on
  an amended field are recomputed.
- Every layer file is type-checked on every run, active or not, so a renamed field breaks a
  stale layer at once.
- `--layer <name>` (repeatable, applied in order) on `check`, `build`, `test`, `explain`,
  `refs`, `edit`, `rename`. An unknown name is `E1901` (exit 2). A `--layer` build writes the
  same `out` files and never writes `canon.lock`: never commit its outputs.
- `canon explain config.server.port --layer staging` shows `set by  <file>:<line>  layer
  staging` before the replaced origins.

## Editing under layers

Without `--edit-layer`, `canon edit` writes base sources, and a value an active layer sets is
read-only (`ErrNotEditable`, reason `layered`). With `--edit-layer staging` (or the request key
`"editLayer": "staging"`), `set`, `reset` and `addEntry` write amendment lines into
`<package dir>/staging.layer.canon`, created if missing; other ops are refused (reason `layer`).

## Runtime inputs

`name: input T from env "VAR"` is a field read by generated code at startup (Go
`LoadInputs() error`, C++ `bool LoadInputs(std::string& error)`), never at build time.

- `T`: `Bool`, integers, `Float`, `Float32`, `String`, `Duration` or an enum, optionally `?`,
  refined only by ranges, lengths and portable patterns (`E1910`). No default (`E1907`).
- Allowed only in records reached from one public, non-collection value through record fields
  (`E1903`). Never written in literals or data (`E3312`); checks, functions and views cannot
  read them (`E3313`); omitted from data files. A TypeScript emit refuses a package with
  inputs (`E8104`).
- Unset or empty: `none` for `T?`, a load failure naming the variable otherwise. The field's own
  refinement is the only validation that ever runs at runtime.
