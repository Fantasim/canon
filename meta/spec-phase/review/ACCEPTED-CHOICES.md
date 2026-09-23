# Accepted choices from the documentation agents

DECISIONS 24 accepts every choice below. Where two documents disagreed, the winner is named
here. Each owning document states the full rule.

## Conflicts settled

| Topic | Winner | Loser, which must be aligned |
|---|---|---|
| Parallel legacy fields | `@json(pairs: …)` (DECISIONS 21) | VIEWMODEL's view `row` |
| Default editor of a type | `widget … default` (DECISIONS 22) | the opt-in `time_of_day` in the examples |
| "More" threshold | 25 % (MOCKUP-GAPS 8) | VIEW-06's 10 % |
| Values set by an active layer | read-only unless `EditLayer` names the layer | VM-05 "editable with a warning" |
| `W1003` | naming conventions; deprecated uses `W3301` | TYP-22 |
| Reload | one `<Package>Snapshot` per emit, with one store | per-value `PotionsStore` |
| Go runtime error type | `*rt.EvalError` | `*canon.EvalError` |
| Dependent-type branch enum | `<Alias>Branch` | DEP-03's `<Alias>Kind` |
| Baked/embedded accessors | `Get<V>()` in Go and C++ | `func <V>()` |
| `entry` | allowed for tables and keyed lists | TYPES.md `E3103`, tables only |
| Un-retiring | `E6002` (hand edit of the lock only) | API.md `Unretire` succeeding |
| `editable` in the view model | `canon` / `json` / `none` + `reason` (API.md) | VM-05's `computed` / `layered` |
| Adopting hand-written files | one `--adopt` flag, on `canon convert` (JSON) and on `canon build` (C++ headers for `access: both`) | two separate mechanisms |

## Language (GRAMMAR, TYPES, EVALUATION, STDLIB)

- **Names and annotations**
  - Reserved words are allowed as the name before `:` in any brace literal, and as bare values in
    the positions GRAMMAR §4.3 lists.
  - Annotation placement follows GRAMMAR §8: prefix annotations on top-level declarations;
    annotations after the name on record, enum and variant declarations.
- **Source text**
  - A BOM in a `.canon` file is `E1123`; `fmt` removes it.
  - Widget names are lower_snake.
- **Checks**
  - The syntax is `check <cond> at <field> else "…"`.
- **New view syntax**
  - `plural`, `filters { x multi }`, `show [id] "Label" "tpl"`, and the `field` escape.
- **Annotations for views and legacy C++**
  - `@menu(m, icon:, label:)`, which wins over a view's `menu`.
  - `@cpp(value:)` and `@cpp(unit:)`.
- **Optionals**
  - `?.` skips the rest of the chain when it meets `none`.
  - Narrowing follows TYPES.md exactly, including `var`s, early `return` and comprehension `if`.
  - Case fields are readable only after narrowing with `is` or `match`.
- **Keyed collections**
  - On keyed lists and tables, `xs[k]`, `xs.k` and `get(k)` look up by key; position is
    `xs.at(i)`.
  - Amend paths gain `[#n]` for a position, as in API paths.
- **Types and values**
  - `Range` values are integers only. Duration and Float ranges exist only in refinements.
  - `Duration` values are limited to ±9,223,372,036,854 ms (the range of Go's `time.Duration`).
- **Layers and inputs**
  - When a layer changes a field, later fields that came from their defaults are recomputed.
  - An empty environment variable counts as unset.
- **Tests**
  - `expect v fails E3204` may target a code.
- **Floats**
  - Float `min`, `max` and `clamp` order −0.0 before +0.0.

## Formats (WIRE, FINGERPRINT, LOCK, FORMATTER)

- **Wire profiles**
  - There are two: source (read) and data (written).
  - Only top-level tables in data files use `rows` with `$id`.
- **Reading legacy files**
  - `$schema` in a loaded legacy file is ignored.
- **Annotation rules**
  - `@json(bits)` requires `@codes` with power-of-two codes.
  - `@json(inline)` is only allowed on non-optional fields.
- **Data files**
  - `E8102` (a value with no wire form) is reported during `check`.
  - There is no `bare` option.
  - Package `$fns` go in the file of the first value.
  - A `data`-mode value must be written by the package's `emit json` (`E8153`).
- **Fingerprint**
  - It hashes wire shape only, with no Canon names; types are numbered.
  - A ref contributes only its key's wire type.
  - Loaders compare the whole `$schema` string.
- **Lock**
  - `field` facts are named by the table value.
  - Retirement is recorded in the lock.
- **Formatter**
  - Broken `( )` and `[ ]` lists get a trailing comma.
  - Imports and imported names are sorted bytewise.
  - A list created by the edit API stays on one line only if it fits and holds no nested list.
  - JSON sources use jq-style layout and reuse WIRE.md §7.2 and §7.3 for numbers and strings.

## Code generation (CODEGEN, CONFORMANCE)

- **C++ classes**
  - Default constructors are public. Members stay private and there are no setters.
  - `@cpp(defines:)` goes in its own header, with macro guards.
  - Legacy `both` and `getters` getters return the member's own C++ type.
- **Enum helpers**
  - C++: `<Enum>FromWire` and `ToName`.
  - Go: `<E>Members()`.
  - TS: `<E>Members`, `Names` and `Index`.
- **TypeScript**
  - Tables are `CanonTable` objects.
- **Refs**
  - Refs resolve to pointers across any public value of the same package and emit.
- **Translated methods**
  - A translated method must be called by at least one test (`E9008`).
  - It takes no ref parameters (`E9006`).
- **Runtime helpers**
  - A second C++ runtime header, `canon_runtime_json.h`, keeps baked mode free of nlohmann.

## View model and i18n (VIEWMODEL, I18N)

- **Layout**
  - The `_other` group label is studio chrome, not a translation key.
  - Case fields of inline variants are laid out in the parent's sections.
- **Menus**
  - A view's `menu` also places public collections of that type.
- **Controls**
  - "Cards" apply to every map of records.
  - `unit` is not allowed on `Duration`, which has its own unit selector.
- **Relevance**
  - Usage counts instances reachable from public values, per variant shape.
- **Views**
  - Views on loaded define tables are allowed.
- **Translations**
  - A text is translatable only if it contains a letter outside its interpolations.
  - Unnamed `show` lines get ids `_0`, `_1`…
  - An empty translation counts as missing.
  - `W1701` fires once per (package, language), and only for packages that emit a view.

## API and plan (API, IMPLEMENTATION-PLAN)

- **Module**
  - The module is `github.com/fantasim/canonlang` (DECISIONS 23) and needs Go 1.25.
- **Libraries**
  - Hand-written parser (CST with trivia), cobra, glsp, go-json-experiment/jsontext, doublestar,
    fsnotify, go-udiff, txtar, go-cmp, x/term.
- **Options and exit codes**
  - `--root name=path` and `Options.Roots`.
  - An empty `Base` disables the staleness check.
  - An interrupt exits with code 130.
- **Edits**
  - A path through a computed value is not editable; the error names the editable source.
  - Cascades (`SetCase`, dependent fields) are applied by the compiler.
  - An edit refuses a file not in canonical layout unless `Normalize` is set.
- **Evaluate**
  - A failing `when` counts as true.
  - Target: p95 ≤ 150 ms.
- **Migration tools**
  - `convert` requires `emit json` first.
  - LSP rename adds `@json("old")` when the field has wire presence.
