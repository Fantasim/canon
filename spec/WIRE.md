# Canon wire format

Status: **normative** companion of [SPEC.md](../SPEC.md) v0.1. Covers how Canon values are read
from files (`load`) and written to JSON data files (`emit json`), the paths both use, and the
build manifest.

Owned AUDIT items: LOD-01..11, WIR-01..11, GEN-04 (paths), GEN-05 (JSON marker), GEN-06 (`bare`),
the wire and file-layout parts of EMT-01..05, the byte format of `expected/potions.json` (GEN-01),
NFR-03 (wire part), NFR-04, MOCKUP-GAPS 7 (`@json(int)`, `@json(bits)`) and DECISIONS 21 /
MOCKUP-GAPS 49 (`@json(pairs:)`, §5.14).

Related documents:

| Document | What this document assumes from it |
|---|---|
| [FINGERPRINT.md](FINGERPRINT.md) | the `$schema` value written by §8 |
| [LOCK.md](LOCK.md) | retired entries (§5.7 here) |
| GRAMMAR.md | the syntax of annotations (`@json("x", unit: s)`, symbol arguments such as `inline`, `int`, `bits`, `codes`) and `E1104` for an unknown annotation or argument |
| TYPES.md | the type rules and the codes they own: `E3201` (out of range), `E3202` (NaN/Inf), `E3301` (unknown field), `E3302` (missing required field), `E3312` (input field in data), `E3501`/`E3502` (refs), `E3701` (asset), `E3802` (dependent-type mismatch), `E3102` (duplicate key) |
| EVALUATION.md | build phases, poisoning of failed values, and the provenance format (file, 1-based line, 1-based byte column, RFC 6901 pointer) |
| CODEGEN.md | generated loaders and decoders, which implement §5 and §7 of this document; `E8001` (refuse to overwrite a file without a marker), `E8002` (two emits of one target), `E8003` (unknown emit option) |
| FORMATTER.md | the canonical layout of JSON *sources* (`canon fmt --json-sources`, FMT-02), which reuses the scalar encodings of §7.2 and §7.3 |

Words: **must** and **is an error** are requirements. A code in brackets (`E7103`) is the
diagnostic emitted when the requirement is broken. All codes are listed in §13.

---

## 1. Terms

| Term | Meaning |
|---|---|
| **wire form** | the JSON value that represents a Canon value of a given type, after all `@json` annotations are applied |
| **source wire** | the wire form read by `load` and by the decoders of `types` mode (EMT-05). Fields may be absent (defaults apply). Tables are JSON objects keyed by id. |
| **data wire** | the wire form written by `emit json` and read by the loaders of `data` and `embedded` modes. Every field is present. Top-level tables are arrays of rows with `$id`. Precomputed `export fn` results appear under `$` keys. |
| **document** | one JSON file: its top-level value |
| **`$` key** | an object key starting with `$`. Canon never derives such a key from a Canon name, so `$` keys cannot collide with fields. |

Source wire and data wire are the same mapping (§5) with the differences listed in §5.13.

---

## 2. Paths

Applies to every path string in `load`, `load.*`, `emit … { out: … }`, `asset(…)` roots and
`@files` templates.

### 2.1 Syntax

```
path       = [ "@" rootName [ "/" rest ] ] | rest
rootName   = identifier                      (a key of project.canon `roots`)
rest       = segment { "/" segment } [ "/" ]
segment    = 1*( any character except "/" and "\" )
```

- The separator is `/`. A `\` anywhere is an error [`E7001`], on every platform.
- An absolute path (`/x`, `C:/x`, `C:\x`, `//server/x`) is an error [`E7001`].
- An empty segment (`a//b`, a leading `/` after the root name such as `@services//x`) is an error
  [`E7001`]. The only allowed empty segment is a single trailing `/`, which marks a directory
  (`out: "out/"`).
- `@name` must name a declared root [`E7003`]. `@name` alone (no `/`) is the root directory itself.
- Paths are compared byte for byte. Letter case is significant on every platform.

### 2.2 Resolution

1. **Base.** A rooted path starts at the root's directory, itself resolved lexically relative to the
   project directory (`resource: "../../../Resource"`). An unrooted path starts at the directory of
   the `.canon` file that contains it.
2. **Lexical normalisation.** Segments `.` are dropped. Each `..` removes the previous segment.
   This is done on the text, before any file system access, and symbolic links are not consulted.
3. **Containment.** A rooted path must not remove more segments than its `rest` added (it may not
   climb above the root directory) [`E7001`]. An unrooted path must stay inside the project
   directory [`E7001`].
4. **Roots inside the project.** A root may point inside the project directory
   (`pipeline_go: "pipeline/out/go"`, `features: "features"` in `examples/project.canon`). A file
   there can then be reached both ways: `out/go/potions.gen.go` written in
   `pipeline/potion.canon`, or `@pipeline_go/potions.gen.go`. Both are valid and name the same
   file; each keeps the form it was written in (§2.3). Such a root exists for what needs the
   directory as a root, such as a Go module path (CODEGEN.md §2.8), not to change how paths are
   written.

| Written in `examples/pipeline/potion.canon` | Result |
|---|---|
| `"data/*.json"` | `pipeline/data/*.json` (project-relative) |
| `"out/"` | directory `pipeline/out` |
| `"@resource/Server/Define/defineitem.h"` | root `resource`, `Server/Define/defineitem.h` |
| `"@resource/Server/../Server/Define/x.h"` | `@resource/Server/Define/x.h` |
| `"@services/..//teamboard"` | `E7001`: `..` climbs above `@services` (and `//` is an empty segment) |
| `"../../outside.json"` | `E7001`: leaves the project |
| `"out\\potions.json"` | `E7001`: `\` is not a separator |

### 2.3 Display

Paths printed in findings, the view model, generated headers, the build manifest and the golden
harness's `expected/MANIFEST` (IMPLEMENTATION-PLAN.md §7.1) are in their **normalised written
form**: `@root/rest` for a path written with a root, project-relative for a path written without
one, `/` separators, no trailing `/` except for directories that were written with one. A file
is never re-addressed through another root, and an unrooted path is never re-addressed through a
root that contains it: the pipeline's outputs display as `pipeline/out/go/potions.gen.go`, not
`@pipeline_go/potions.gen.go`.

Comparisons that ask whether two paths are the same file (output collisions `E8152`, duplicate
`load.dir` matches, the read set) use the resolved paths, never the display forms. Inside one
build, a file keeps the display form of the first reference in the deterministic traversal order
(packages in package order, then source order) wherever a single form is needed (the manifest's
`file` lines, §10).

---

## 3. Reading JSON (LOD-02)

These rules apply to every JSON file read by the compiler (`load`, `load.dir`, `canon convert`,
the edit API reading JSON sources). They are the rules of RFC 8259 plus the choices below.

### 3.1 Bytes

- The file must be UTF-8 [`E7105`]. A UTF-8 byte order mark (`EF BB BF`) at offset 0 is skipped.
  A UTF-16 or UTF-32 byte order mark is `E7105`.
- The document is exactly one JSON value surrounded by optional whitespace (space, tab, LF, CR).
  Anything else is `E7109`: an empty file, trailing content, comments, trailing commas, single
  quotes, unquoted keys, `NaN`, `Infinity`, a leading `+`, leading zeros (`01`), a bare `.5`,
  control characters inside strings, invalid escapes.
- Nesting deeper than 512 arrays or objects is `E7109`.
- A syntax error stops the reading of that file: one `E7109` finding at the offending byte.

### 3.2 Strings

- Escapes are decoded (`\"` `\\` `\/` `\b` `\f` `\n` `\r` `\t` `\uXXXX`). A `\u` escape that
  produces an unpaired UTF-16 surrogate is `E7105`. `\u0000` is allowed.
- Two keys of one object that are equal after decoding are `E7104` (`"a"` and `"\u0061"` are
  equal), reported at the second key; the message names the first (`first at <line>:<col>`).

### 3.3 Numbers

- Numbers are parsed from their token **exactly** (arbitrary precision decimal) before any
  conversion. No implementation may parse through a `float64` first.
- Conversion to the expected type is in §5.1.

### 3.4 Errors after syntax

Once a file parses, decoding it against the expected type reports **every** mismatch (it does not
stop at the first), each at the JSON value it concerns. A value that has any error is poisoned
(EVALUATION.md): it is not used, and values depending on it report nothing more.

---

## 4. Annotations that change the wire

`@json` forms (syntax in GRAMMAR.md):

| Form | On | Effect |
|---|---|---|
| `@json("w")` | field, variant case, enum member | wire name |
| `@json(path: "a.b.c")` | field | the value is at `a` → `b` → key `c` (§5.5.3) |
| `@json(case: snake\|camel\|kebab\|upper_snake)` | record, variant, case | default wire names of its fields (§5.5.2) |
| `@json(tag: "t")` | variant | tag key, default `"kind"` |
| `@json(inline)` | field whose type is a variant | tag and case fields sit in the parent object (§5.6) |
| `@json(none: X)` | optional field | the wire's way of writing `none` (§5.4) |
| `@json(unit: ms\|s\|m\|h\|d)` | field containing `Duration` | number unit on the wire (§5.1) |
| `@json(int)` | field containing `Bool` | `Bool` written as `0` / `1` (§5.2) |
| `@json(bits)` | field of type `[E]` or `[E]?`, `E` a `@codes` enum | the list written as one bitmask integer (§5.3) |
| `@json(codes)` | enum with `@codes` | members written as their code (§5.3) |
| `@json(pairs: ["k{i}", "v{i}"])` | field of type `[R]`, `R` a two-field record | element `i` written as the two parallel keys `k{i}` and `v{i}` of the parent object (§5.14) |

Several forms may be combined in one annotation: `@json("dwCooldownMs", unit: ms)`,
`@json("bTwoHanded", int)`.

### 4.1 Where each form applies [`E3316`]

Positions and combinations are checked by GRAMMAR.md's annotation catalogue: an unknown form or a
form at a wrong position is `E1104`; the positional name together with `path:`, or two of
`inline`, `int`, `bits`, `unit:` in one `@json`, is a GRAMMAR.md error; `@json(codes)` on an enum
without `@codes` is `E1119`; `@json("w")` on a plain enum member (which uses `= "w"`) is a
position error. The rules that depend on the **type** are checked here. Breaking one is `E3316`,
at the annotation:

| Form | Required |
|---|---|
| `path:` | 2 or more non-empty segments separated by `.`; no segment starts with `$` |
| `inline` | the field's type is a non-optional variant (an alias of an optional does not count as a variant) |
| `none:` | the field's type is optional; `X` is an integer (optionally negative), a float, a string, `true`, `false`, `{}` or `[]` |
| `unit:` | the field's type contains `Duration` outside any named record, variant or enum (directly, or in `T?`, `[T]`, map values) |
| `int` | the field's type contains `Bool` outside any named type |
| `bits` | the field's type is `[E]` or `[E]?`, `E` has `@codes`, every code of `E` (retired included) is a power of two between 1 and 2^62 |
| `pairs:` | the field's type is a non-optional plain list `[R]` (not keyed, not a table) whose length refinement has a finite upper bound; `R` is a record with exactly two fields, no input field and no non-translated `export fn`; each field of `R` is non-optional and has a scalar wire form (`Bool`, an integer type, `Float`, `Float32`, `String`, `Duration`, an enum, a `ref`, an asset, a literal union); each template contains `{i}` exactly once, the two templates differ, and no expanded key starts with `$` or collides with another expanded key (§5.14) |

`unit:` and `int` apply to every `Duration` (resp. `Bool`) of the field's type that is not inside a
named type. `none:` applies only to the field's own outer optional, never to optionals nested in
lists or maps (those are always `null`).

### 4.2 Wire names must not collide [`E3316`, `E3318`]

For each record and each variant case, compute the set of **wire locations**: for each non-input
field, its key path (`["dwID"]`, or `["legacy","reqMp"]` for `path:`), or, for a `pairs:` field,
every expanded key of §5.14 (`["dwDestParam0"]` … `["nAdjParamVal5"]`); the `$` keys of its export
fns (§5.11). It is `E3316`, at the second field, when:

- two fields have the same wire name, or a field's wire name is a `$` key;
- a field's key path is a prefix of another's (`legacy: Int` next to `@json(path: "legacy.reqMp")`),
  or the same key path;
- two `$` keys are equal (cannot happen with distinct method names; listed for completeness).

For variants and inline fields it is `E3318`, at the field or case, when:

- a case field's wire name equals the variant's tag key (inline or not);
- with `@json(inline)`: the tag key, a field wire name of any case, or a case `$` key equals a wire
  name, first path segment or `$` key of the parent record;
- a record has more than one `@json(inline)` field.

---

## 5. The wire mapping

For every type, how a value is **encoded** (data wire, and the source wire unless §5.13 says
otherwise) and how a JSON value is **decoded** (both wires). "JSON kind" is one of null, boolean,
number, string, array, object. A JSON value of the wrong kind is `E7110`.

### 5.1 Scalars

| Type | Encode | Decode |
|---|---|---|
| `Bool` | `true` / `false` | boolean only |
| `Int`, `Int8`…`Int32`, `UInt8`…`UInt64` | decimal integer, `-` for negatives (§7.2) | a number token with no fraction and no exponent, else `E7103` (`2.0`, `1e3` and `-1.5` are `E7103`); value checked against the type (`E3201`); `-0` reads as `0` |
| `Float` | §7.2, float64 | any number token, rounded once to the nearest float64 (ties to even); an overflow to ±Inf is `E3202`; `-0`/`-0.0` read as `0` |
| `Float32` | §7.2, shortest decimal that reads back as the same float32 | any number token, rounded once from the exact decimal to the nearest float32; overflow `E3202` |
| `String` | a JSON string (§7.3) | string only |
| `Duration` | an **integer** count of the field's unit (default `ms`, WIR-03); a value that is not a whole number of units is `E8102` | any number token (integer or not); exact decimal × unit in ms must be a whole number of ms, else `E3203`; the result must fit the `Duration` range, ±9,223,372,036,854 ms (`E3201`, TYPES.md §7.2) |

Units: `ms` = 1, `s` = 1 000, `m` = 60 000, `h` = 3 600 000, `d` = 86 400 000 ms.

`E8102` is reported during verification (EVALUATION.md §1, stage B, phase 4) for every evaluated
value, not only when writing, so `canon check` finds it: a field's wire annotation is part of what
the value must satisfy.

| Field | JSON | Value |
|---|---|---|
| `cooldown: Duration @json("dwCooldownMs", unit: ms)` | `8000` | `8s` |
| same | `8000.0` or `8e3` | `8s` |
| `maxDuration: Duration(1m..) @json("maxDurationMin", unit: m)` | `1.5` | `1m30s` |
| `duration: Duration @json("durationSec", unit: s)` | `0.0005` | `E3203` (0.5 ms) |
| `productionTick: Duration @json("productionTickIntervalSec", unit: s)`, value `1500ms` | (encode) | `E8102` |
| `actionDelay: Duration @json("actionDelayMs", unit: ms)`, value `700ms` | (encode) | `700` |
| `heal: Int @json("nHeal")` | `2500.0` | `E7103` |

### 5.2 `Bool` as `0` / `1`: `@json(int)`

Encode `false` → `0`, `true` → `1`. Decode: only the integer tokens `0` and `1`; anything else
(`true`, `2`, `1.0`) is `E7110`.

### 5.3 Enums

- **Wire value** (WIR-04): the member's name, or its `= "…"` string (plain enums), or its
  `@json("…")` name (`@codes` enums, where `= N` is the code). `Tone.series_1` has wire
  `"series-1"`; `Element.FIRE` has wire `"FIRE"` and code `1`.
- **Encode**: a JSON string holding the wire value. With `@json(codes)`: a JSON integer, the code.
- **Decode**: a string equal, byte for byte, to one member's wire value, else `E7111`. The message
  names the closest member and, when the string is a Canon name whose wire differs
  (`"series_1"`), says so. With `@json(codes)`: an integer token equal to a member's code, else
  `E7111` (non-integer: `E7103`).
- Retired members keep their wire value and code: the decoder recognises them (not `E7111`), then
  TYPES.md reports a retired member used in a value (`E3506`). Generated loaders decode them.

**`@json(bits)`** (MOCKUP-GAPS 7) on `flags: [Flag]`:

- Encode: the bitwise OR of the members' codes, as a JSON integer (`[tradable, soulbound]` with
  codes 1 and 4 → `5`; `[]` → `0`). A list holding the same member twice has no encoding: `E8102`.
- Decode: an integer token ≥ 0 (non-integer `E7103`, negative `E7110`). Every set bit must be a
  member code, else `E7111` naming the unknown bits in hex (`0x100`). The result lists the members
  in ascending code order.
- The order of a bits list is therefore not preserved by the wire. Such fields should be declared
  `where it.isUnique()`; `canon convert` compares them as sets.

### 5.4 Optional values and `none` (WIR-02)

- **Encode**: every field is written. `none` is written as `null`, or as the field's
  `@json(none: X)` marker. `none` inside a list or map value (`[T?]`, `{K: V?}`) is always `null`.
- **Decode** of a record field:

  | JSON | Field `T?` without default, or `T? = none` | Field `T? = d` (non-`none` default) | Field `T = d` | Field `T` (required) |
  |---|---|---|---|---|
  | key absent | `none` | `d` | `d` | `E3302` |
  | `null` | `none` | `none` | `E3315` | `E3315` |
  | the `none:` marker (JSON-equal) | `none` | `none` | n/a | n/a |
  | another value | decoded as `T` | decoded as `T` | decoded as `T` | decoded as `T` |

- `null` anywhere a non-optional value is expected (list element, map value, top-level value) is
  `E3315`.
- JSON equality for the marker: same kind; numbers equal by exact value; strings equal after
  decoding; `{}` and `[]` match only an empty object and an empty array.
- A non-`none` value whose encoding equals the field's marker has no encoding [`E8102`], checked in
  verification (`parent: ref TalentNode? @json(none: -1)` with a node whose id is `-1`).

### 5.5 Records

#### 5.5.1 Object shape

A record is a JSON object. **Encode** writes its members in this order:

1. in a top-level table row only: `"$id"`, then `"$retired": true` if the entry is retired (§5.7);
2. the fields in declaration order, skipping `input` fields (WIR-11). A `@json(inline)` field is
   expanded in place (§5.6). A `path:` field is written inside its nested objects (§5.5.3); a
   `pairs:` field writes its slot keys in place (§5.14);
3. the `$` keys of the record's export functions, in declaration order (§5.11).

**Decode** reads the whole object, then decodes the fields in declaration order (so a dependent
field sees the fields it depends on). Keys that no field claims are `E3301`, one finding per key,
unless the `load` has `partial: true`. `@deprecated` fields are read and written like any other
field (WIR-11). An `input` field's key present in the data is `E3312`.

JSON key order never matters when decoding.

#### 5.5.2 Wire names and `@json(case:)` (WIR-08)

A field's wire name is, in order of precedence: its `@json("…")`; else its Canon name converted by
the nearest `@json(case:)` (the field's case, then its variant, for case fields; the record, for
record fields); else its Canon name. `case:` never propagates into nested record types
(`LevelDiff` in adventurequest stays camelCase inside a snake_case `Global`), and never applies
to variant case names or enum members.

The conversion splits the Canon name into words, then joins them:

1. `_` is a separator and is dropped; empty words are dropped.
2. A word boundary falls before character `c[i]` (`i > 0`) when:
   - `c[i]` is upper case and `c[i-1]` is lower case or a digit; or
   - `c[i]` and `c[i-1]` are upper case and `c[i+1]` exists and is lower case.
3. Digits never start a word: they stay with the preceding word.
4. `snake`: words lower-cased, joined with `_`. `kebab`: lower-cased, joined with `-`.
   `upper_snake`: upper-cased, joined with `_`. `camel`: the Canon name unchanged.

"Upper case" and "lower case" mean ASCII `A-Z` and `a-z` (identifiers are ASCII).

| Canon name | `snake` | `kebab` | `upper_snake` |
|---|---|---|---|
| `targetTime` | `target_time` | `target-time` | `TARGET_TIME` |
| `maxQuestsPerStyle` | `max_quests_per_style` | `max-quests-per-style` | `MAX_QUESTS_PER_STYLE` |
| `ratesBySpecific` | `rates_by_specific` | `rates-by-specific` | `RATES_BY_SPECIFIC` |
| `isDailyFlat` | `is_daily_flat` | `is-daily-flat` | `IS_DAILY_FLAT` |
| `stage1Rate` | `stage1_rate` | `stage1-rate` | `STAGE1_RATE` |
| `level2` | `level2` | `level2` | `LEVEL2` |
| `v2beta` | `v2beta` | `v2beta` | `V2BETA` |
| `HTTPServer` | `http_server` | `http-server` | `HTTP_SERVER` |
| `userID` | `user_id` | `user-id` | `USER_ID` |
| `parseHTTP2Response` | `parse_http2_response` | `parse-http2-response` | `PARSE_HTTP2_RESPONSE` |
| `XMLHttpRequest` | `xml_http_request` | `xml-http-request` | `XML_HTTP_REQUEST` |
| `getX` | `get_x` | `get-x` | `GET_X` |
| `ABC` | `abc` | `abc` | `ABC` |
| `a_b` | `a_b` | `a-b` | `A_B` |
| `_private` | `private` | `private` | `PRIVATE` |
| `x` | `x` | `x` | `X` |

Two fields that convert to the same wire name are `E3316`.

#### 5.5.3 `@json(path:)` (LOD-09)

`reqMp: Int = 0 @json(path: "legacy.reqMp")`: the value is the key `reqMp` of the object under the
key `legacy`. Segments are used verbatim (`case:` does not apply).

- **Decode**: a missing intermediate object means the field is absent (its default applies). An
  intermediate that is present but not an object is `E7110` (`null` included). Keys of an
  intermediate object that no field path claims are `E3301` (unless `partial`).
- **Encode**: the intermediate objects are created at the position of the **first** field (in
  declaration order) whose path starts with that prefix; inside them, members follow the
  declaration order of the fields that share the prefix. Two fields `a.b.c` and `a.d` give
  `"a": {"b": {"c": …}, "d": …}` at the position of the first of them.

#### 5.5.4 Input and deprecated fields (WIR-11)

`input` fields have no wire form: never written, and `E3312` when present in loaded data.
`@deprecated` fields are read, written, and fingerprinted like any other field; loading one from a
JSON source produces no finding (TYPES.md decides whether a Canon literal setting one warns).

### 5.6 Variants (WIR-07)

- **Encode**: always a JSON object. The tag member comes first (`"kind": "<case wire>"`, key from
  `@json(tag:)`), then the case's fields in declaration order, then the case's `$` keys. A case
  without fields is `{"kind": "nothing"}`.
- A case's wire name is its Canon name or its `@json("…")`. `@json(case:)` on a variant converts the
  field names of all its cases; on a case, of that case only; never the case names.
- **Decode**: the object must have the tag key [`E7112` if missing], holding a string [`E7110`]
  equal to a case's wire name [`E7112` if unknown, naming the cases]. Keys that belong to another
  case are `E3301` with a hint naming that case. A retired case decodes, then `E3506` applies
  (TYPES.md).
- **`@json(inline)`**: the field has no key of its own. Encode writes, at the field's position in
  the parent, the tag member, the case's fields, and the case's `$` keys. Decode reads the tag and
  the selected case's fields from the parent object. A parent key is claimed if it is a parent
  field, the tag, or a field of the selected case.

`Event` from `resource/events/event.canon` (`kind: EventKind @json(inline)`, tag `"type"`), the
`sample` value, encoded compactly (§7.4):

```json
{"id": "sample", "worldId": 1, "targetCount": 10, "type": "monster_drop_inject", "itemId": "II_DEFAULT", "itemCount": [1, 1], "levelMin": 0, "levelMax": 0, "schedule": [{"day": "Mon", "startUtc": {"hour": 20, "minute": 0}, "endUtc": {"hour": 22, "minute": 0}}], "rollMode": "local_budget"}
```

(`items.first()` is `II_DEFAULT`, the first `II_` define of the real `defineitem.h`.)

### 5.7 Lists, keyed lists and tables (LOD-04, WIR-06)

| Type | Encode | Decode |
|---|---|---|
| `[T]` | JSON array, in order | array only |
| `[T] keyed by f` | JSON array of `T` objects; the key is the field `f` | array only; duplicate keys `E3102` |
| `table T`, `stable table T`, **top level of a data file** | array of rows: each row is the record's object with `"$id": "<key>"` first and `"$retired": true` second when retired (§8.2) | (data wire loaders only) |
| `table T` anywhere else (nested in a value, and every source wire) | JSON object: key = entry id, value = the record's object, in entry order; a retired entry's object starts with `"$retired": true` | object only; each key must be a Canon identifier [`E7114`]; `"$retired"` is accepted in a row object and must be `true` [`E7110`]; entry order = key order in the file |

Retired entries are written everywhere (LOCK.md §7): in top-level rows and in nested tables. Keyed
lists and plain lists have no retirement.

`load.dir` builds a list, keyed list or table from one file per element (§6.5).

### 5.8 Maps (WIR-05)

A map is a JSON object whose members follow the map's insertion order. Keys:

| Key type | Wire key |
|---|---|
| `String` | the string, verbatim |
| integer types | canonical decimal (`-5`, `0`, `42`); decode requires exactly `-?(0\|[1-9][0-9]*)`, else `E7103` |
| enum | the member's wire value; with `@json(codes)`, the code in decimal |
| `ref` into a table or a define table | the entry key |
| `ref` into a keyed list | the key field's wire value, as text (decimal for integer keys) |
| literal union `A \| "lit"` | the literal, or `A`'s wire key; on decode the literal wins (TYP-09) |
| dependent key (`SpecificKey(e)`) | the wire key of the branch selected by the key's parameter |

Two keys that encode to the same text are `E3317`, at the second key (for example a literal
`"default"` and a ref key `default`). Decoding a key applies the same rules as decoding a value of
the key type (`E7111` for an unknown member, `E3501` for an unknown ref key).

A dependent map `{e in eventTypes: HourlyTarget(e)}` is an object keyed by the ref key; each value
is decoded with the parameter bound to that key's entry.

### 5.9 References, assets, unions, dependent types

- **`ref`** (§5.8 of SPEC): the key. A table or define-table ref is a JSON string. A keyed-list ref
  is the key field's wire form (`ref TalentNode` with `id: Int` is a JSON integer). A key that does
  not exist is `E3501` (verification).
- **asset**: a JSON string, the path relative to the asset root (rules and `E3701` in TYPES.md).
- **Literal union** `A | "lit"`: the literal string, or `A`'s wire form. On decode a string equal
  to a literal is the literal.
- **Dependent type** `Param(e)`: the wire form of the branch selected by the discriminant, with no
  tag. The discriminant (an earlier field, a record parameter or a map key) is decoded first. A
  `Never` branch accepts only `none` (absent, `null`, or the marker); any other value is `E3802`.
- **`Range` and function types** have no wire form: `load` into them is `E7116`, emitting them is
  `E8151`.

### 5.10 Aliases and refinements

Aliases are transparent. Refinements and `where` never change the wire; they are checked after
decoding (TYPES.md).

### 5.11 Export functions: `$` keys (WIR-09)

For every `export fn` of a record or variant case that is **not** translated (no parameter besides
`self`, or only finite parameters, SPEC §9.4), each encoded value of that type carries one `$` key,
data wire only:

- key `"$<canonName>"` (`"$isStrong"`), regardless of `@json(case:)`;
- no extra parameter: the result's wire form (a `Duration` result is in `ms`; `@json` annotations
  cannot apply to results);
- finite parameters: nested objects, one level per parameter in parameter order, keyed by the wire
  key (§5.8 rules) of each argument value. Domain order: enum members in declaration order,
  retired included; table or keyed-list entries in entry order, retired included; `Bool` as
  `"false"` then `"true"`.

**Package-level** export functions of the same two kinds go under the top-level key `"$fns"` of
the data file of the **first value** listed in the package's `emit json` (§8.1): an object keyed
by the function's Canon name (no `$`), each holding the result or the nested objects above.
Translated functions never appear in data.

### 5.12 Summary of `$` keys

| Key | Where | Wire |
|---|---|---|
| `$schema` | first key of a data file (§8.2); ignored in the root object of a source file | data |
| `$id` | first key of a top-level table row | data |
| `$retired` | after `$id` in rows; first key of a nested or source table row | both |
| `$<fn>` | after the fields of a record or case object | data |
| `$fns` | after `rows` / `value` in a data file | data |

Any other `$` key in a source file is an unknown key (`E3301`, or ignored with `partial`).

### 5.13 Source wire vs data wire

| | Source wire (`load`, `types`-mode decoders) | Data wire (`emit json`, `data`/`embedded` loaders) |
|---|---|---|
| absent fields | allowed: default, `none`, or `E3302` (§5.4) | never: every field is written (except an empty `pairs:` list, §5.14) |
| top-level table | object keyed by id | `rows` array with `$id` |
| `$` keys | only `$schema` (root, ignored) and `$retired` (table rows) | `$schema`, `$id`, `$retired`, `$<fn>`, `$fns` |
| numbers | any token the type accepts (§5.1) | canonical (§7.2) |
| layout | any | canonical bytes (§7) |

A `types`-mode decoder (EMT-05) implements the source-wire structure rules: defaults, `none`,
units, paths, inline variants, `int`, `bits`, `codes`. It does not re-validate refinements or refs:
the file it reads is the one `canon check` validated. Data-mode loaders may assume the data wire.

### 5.14 Parallel keys: `@json(pairs:)` (DECISIONS 21)

Legacy files write a short list of pairs as numbered parallel keys (`dwDestParam0` with
`nAdjParamVal0`, `dwDestParam1` with `nAdjParamVal1`, …). `@json(pairs: [k, v])` maps them onto a
list of two-field records, so code, checks and the studio see one list (DECISIONS 21):

```
record StatBonus {
  attribute: ref attributes
  value: Int
}
stats: [StatBonus](..=6) = [] @json(pairs: ["dwDestParam{i}", "nAdjParamVal{i}"])
```

- **Slots.** `N`, the number of slots, is the upper bound of the list's length refinement:
  `(..=6)` and `(1..=6)` give 6, `(..6)` gives 5. A list without a finite upper bound is `E3316`
  (§4.1). Slot `i` (`0 ≤ i < N`) owns the keys `k(i)` and `v(i)`: the templates with `{i}` replaced
  by `i` in decimal (`dwDestParam0` … `dwDestParam5`). `@json(case:)` never applies to them.
- **Fields.** The first field of `R`, in declaration order, is written under `k(i)`, the second
  under `v(i)`. The fields' own wire names (`@json("…")`, `case:`) are not used; their `unit:` and
  `int` forms apply to the values.
- **Encode.** Element `i` of the list is written as `k(i)` then `v(i)`, at the field's position
  in the parent object, elements in order: `"dwDestParam0": "DST_STR", "nAdjParamVal0": 5,
  "dwDestParam1": …`. Slots from the list's length to `N − 1` are **not written**: an empty list
  writes no key. This is the only data-wire field that may have no key (§5.13).
- **Decode** (both wires). For each slot, both keys absent means the slot is **empty**; both
  present and not `null` means it is **filled**, and each value is decoded against its field's
  type. One key without the other, or a `null`, is `E7117` at the slot's first key present. Filled
  slots must be contiguous from slot 0: a filled slot after an empty one is `E7117` (Canon is the
  law, DECISIONS 6; legacy data with gaps is fixed by a one-off script, DECISIONS 13). The list is
  the filled slots, in slot order. Keys matching a template with `i ≥ N` are not claimed: `E3301`
  unless `partial`.
- **Defaults.** An absent field (every slot empty) decodes as the empty list, whatever the
  field's default; a field with `pairs:` must therefore have no default or the default `[]` (a
  non-empty default is `E3316`).
- **Fingerprint.** The field line names the templates and `N` (FINGERPRINT.md §4.5).
- **Codegen and views.** Generated code and the view model see an ordinary list of records
  (CODEGEN.md §4.2, VIEWMODEL.md §5.9); only loaders and decoders know the slots.

---

## 6. `load` forms (LOD-01, LOD-03 … LOD-08)

### 6.1 Forms, options and expected types

| Form | Options | Expected type | Result |
|---|---|---|---|
| `load(path)` | `at`, `partial`, `format`, `header` (csv only) | required [`E7002`] | decoded per format |
| `load.dir(glob)` | `at`, `partial`, `format` | `[T]`, `[T] keyed by f`, `table T` [`E7116`] | one element per file (§6.5) |
| `load.csv(path)` | `header`, `partial` | with `header: true`: `[R]`, `[R] keyed by f`, `table R`; without: `[[String]]` | §6.6 |
| `load.text(path)` | none | `String` (or an alias or refinement of it) | §6.7 |
| `load.defines(path)` | `prefix` | none needed (fixed table type) | §6.8 |

- An option not listed for the form, or not valid for the detected format (`at:` on csv or text,
  `header:` on json, `partial:` on text), is `E7006`.
- `load` must receive its expected type directly from its context: an annotation, a field, an
  argument, a return type (TYP-06). `load(…).filter(…)` has none [`E7002`].
- The path must exist and be a readable regular file [`E7004`].
- The option values: `at` a string (§6.3), `partial` a Bool constant, `format` one of the symbols
  `json`, `csv`, `text`, `header` a Bool constant, `prefix` a string.

### 6.2 Format detection (LOD-08)

From the extension of the file name, compared ASCII case-insensitively: `.json` → json, `.csv` →
csv, `.txt` → text. Any other extension (including `.h`, `.hpp`, which only `load.defines` reads)
is `E7007` unless `format:` is given. `load.csv` and `load.text` fix their format; `format:` on
them is `E7006`.

### 6.3 The `at:` path (LOD-03)

```
at      = first { next }
first   = name | "*" | index
next    = "." ( name | "*" ) | index
index   = "[" ( "0" | nonZeroDigit { digit } ) "]"
name    = 1*( char other than "." "*" "[" "]" "\" | "\" ( "." | "*" | "[" | "]" | "\" ) )
```

- Applied to the document's top-level value, before decoding against the expected type.
- `name` selects an object member; `[n]` selects an array element (0-based); `*` over an object
  yields a map from each key (in document order) to the rest of the path applied to its value; `*`
  over an array yields a list. Each `*` adds one map or list level.
- A missing member, an index out of range, a name on a non-object, an index on a non-array, a `*`
  on a scalar, an empty or malformed path: `E7106`, at the JSON value where it failed. Under `*`,
  every member must match (Canon is the law).
- Keys produced by `*` are decoded as map keys of the expected map type (§5.8).

| Option | Document | Expected type |
|---|---|---|
| `at: "items"` | `{"items": [ … ]}` | `[Item] keyed by id` |
| `at: "*.us"` | `{"JOB_VAGRANT": {"us": "Vagrant", "fr": …}, …}` | `{String: String}` |
| `at: "canonicalBuilds.*.weapon"` | `{"canonicalBuilds": {"JOB_KNIGHT": {"weapon": "IK3_SWD"}, …}}` | `{String: String}` |
| `at: "solver.kAnchors"` | `{"solver": {"kAnchors": [15, 60]}}` | `[Int]` |
| `at: "a\\.b"` (Canon string `"a\\.b"`) | `{"a.b": 1}` | `Int` |

### 6.4 `partial: true`

Unknown keys are ignored at **every** depth of the loaded value: records, variant cases, inline
parents, `path:` intermediates. It never excuses anything else (unknown tags or members, wrong
kinds, duplicate keys).

### 6.5 Globs and `load.dir` (LOD-05)

Pattern syntax, per `/`-separated segment:

| Syntax | Matches |
|---|---|
| `**` (a whole segment) | zero or more directories |
| `*` | zero or more characters other than `/` |
| `?` | exactly one character other than `/` |
| `[abc]`, `[a-z]`, `[!a-z]` | one character in (not in) the set; `]` first in the set is literal |
| `{a,b,c}` | one of the alternatives (no nesting, no `/` inside) |

- `**` not forming a whole segment, an unclosed `[` or `{`, or an empty alternative is `E7005`.
  There is no escape character (`\` is `E7001`): match a literal `*` with `[*]`.
- **Base.** The segments before the first one containing a magic character form a literal path,
  resolved by §2.2; it must stay inside its root or the project [`E7001`]. The walk starts there.
- Matching is case-sensitive on every platform. Characters are Unicode code points of UTF-8 names.
- **Dotfiles.** A name starting with `.` matches only a pattern segment that starts with a literal
  `.`. `**` never enters a directory whose name starts with `.`.
- Only regular files match (after following links). Symbolic links are followed only when their
  target resolves inside a declared root or the project; any other link is skipped with `W7115`. A
  directory link already on the current walk path is not followed again.
- **Order**: ascending byte order of the matched path relative to the base, with `/` separators.
- Zero matches is `W7107` at the `load.dir`; the result is empty.

`load.dir` decodes each matched file (format from its own extension, or `format:`), applying `at:`
and `partial` to each. With expected type:

- `[T]`: the elements in path order;
- `[T] keyed by f`: the elements in path order; keys from field `f`; duplicates `E3102` naming both
  files;
- `table T`: the key is the file's **stem** (the name without its last extension:
  `II_POT_HEAL_L.json` → `II_POT_HEAL_L`), which must be a Canon identifier [`E7114`]; the file's
  content is the entry's record (a top-level `"$retired": true` retires it); duplicate stems in
  different directories are `E3102`.

### 6.6 `load.csv` (LOD-06)

- RFC 4180 with `,` as separator. UTF-8 [`E7105`]; a UTF-8 BOM is skipped. Records end with LF or
  CR LF; the last record may omit it. A field is either unquoted (no `"`, `,`, CR or LF inside) or
  quoted with `""` for a quote; quoted fields may contain `,`, CR and LF. A bare `"` in an unquoted
  field, an unterminated quote, text after a closing quote, or a record with a different number of
  fields than the first is `E7113`.
- **Without `header`**: the result is `[[String]]`, one list per record, cells verbatim.
- **With `header: true`**: the first record names the columns. Each name must be the wire name of a
  field of the element record `R` (`E3301` otherwise, unless `partial`); duplicate names are
  `E7104`; for `table R`, a column named `$id` holds the keys (`E7114` rules). A required field
  without a column is `E3302`, reported once at the header.
- **Cells.** Only scalar, enum, `ref`, asset and literal-union fields (optionally optional) may be
  read from CSV; any other field type is `E7116`. A cell is read as text:

  | Field type | Cell text |
  |---|---|
  | integer types | a Canon integer literal with an optional leading `-` (`42`, `-1`, `1_000`, `0x1F`) |
  | `Float`, `Float32` | a Canon integer or float literal with an optional leading `-` |
  | `Duration` | a Canon integer or float literal in the field's unit (default `ms`), §5.1 rules |
  | `Bool` | `true` or `false`; with `@json(int)`, `0` or `1` |
  | `String`, asset | the cell, verbatim |
  | enum | the wire value (the code in decimal with `@json(codes)`) |
  | `ref` | the key (decimal for integer keys) |

  A cell that does not parse is `E7108`. An **empty cell** means the field is absent (§5.4 table:
  default, `none`, or `E3302`). A cell equal to the field's `none:` marker (as text) is `none`.
- Findings point to the file, the record's line and the cell's column.

### 6.7 `load.text` (LOD-07)

The file's bytes as a `String`: strict UTF-8 [`E7105`], a leading BOM removed, every CR LF replaced
by LF, nothing trimmed. A lone CR is kept.

### 6.8 `load.defines` (LOD-01)

Reads C `#define`s. There is no preprocessor: every line is read, whatever `#if` block it sits in.

**Reading.**

1. Bytes are decoded as Latin-1 (every byte is one character; this never fails). Legacy headers
   hold CP949 or UTF-8 comments; names are ASCII.
2. Line splicing: a `\` immediately followed by LF or CR LF joins the next line.
3. Comments are replaced by one space: `//` to end of line, `/* … */` (may span lines), outside
   string and character literals.
4. Each line matching `^[ \t]*#[ \t]*define[ \t]+NAME` is a define, where
   `NAME = [A-Za-z_][A-Za-z0-9_]*`. Its line (for provenance) is the line of the `#`.

**Classification.**

| After `NAME` | Result |
|---|---|
| `(` immediately (function-like macro) | skipped, counted |
| nothing (only whitespace) | skipped silently (include guards: `#define __DEFINE_JOB`) |
| an expression of the grammar below whose names are all earlier accepted defines of the same file | accepted |
| anything else (strings, `'A'` characters, casts, `sizeof`, unknown names, later defines, operators not listed) | skipped, counted |

```
expr     = or
or       = and    { "|" and }
and      = shift  { "&" shift }
shift    = add    { ( "<<" | ">>" ) add }
add      = unary  { ( "+" | "-" ) unary }
unary    = "-" unary | primary
primary  = integer | NAME | "(" expr ")"
integer  = ( "0" ( "x" | "X" ) hexDigit { hexDigit }     (* hexadecimal *)
           | "0" { octDigit }                             (* octal, "0" alone is zero *)
           | nonZeroDigit { digit } )                     (* decimal *)
           { "u" | "U" | "l" | "L" }                      (* suffixes ignored *)
```

- Evaluation uses unbounded integers; `>>` is arithmetic; a shift count must be 0..63. A result
  outside `Int` (int64), or a shift count out of range, makes the define skipped and counted.
- **Count.** At most one `W7101` per file per build: "N defines skipped", located at the first
  skipped define, counting only skipped defines whose name starts with the `prefix` of some
  `load.defines` of that file (all of them if one has no prefix).
- **Duplicates.** The same name defined again with the same value is ignored. With a different
  value it is `E7102`, at the second definition; the message names the first (all `#if`
  branches count). `#undef` and every other directive are ignored; `#include` is not followed.

**Result.** A table keyed by the full define name (the prefix is not stripped), one entry per
accepted define whose name starts with `prefix` (all if no prefix), in file order of first
definition. Each entry is a built-in record with `value: Int`. A `ref` into it has the key as wire
form (a JSON string). The table is not stable and has no lock.

Samples from the real headers (`@resource/Server/Define/`):

| Line | Result |
|---|---|
| `#ifndef __DEFINE_JOB` then `#define __DEFINE_JOB` (defineJob.h:1-2) | skipped silently |
| `#define JTYPE_EXPERT 1` | `JTYPE_EXPERT` = 1 |
| `#define MAX_GENERAL_LEVEL			150		` (trailing tabs) | 150 |
| `#define JOB_VAGRANT                 0 ` | 0 |
| `#define JOB_ALL					MAX_JOB` (after `#define MAX_JOB 16`) | 16 |
| `#define DIS_SWORD                   1 //` | 1 |
| `#define CHS_DEBUFFALL				0x00200000	// <CP949 bytes>` (defineAttribute.h:194) | 2097152 |
| `#define	PERIN_VALUE		100000000L` (define.h:244) | 100000000 |
| `#define	LIM_HIDDENBONDS_MAX_REGISTRATION_COST_PENYA	999900000000LL	// …` (defineLimits.h:30) | 999900000000 |
| `#define Normal 1` (defineUpgradeType.h, loaded without prefix) | `Normal` = 1 |
| `#define II_DEFAULT 10` (defineitem.h:4, first `II_` define) | `II_DEFAULT` = 10 |
| `#define X (1 << 4) \| 0x2` | 18 |
| `#define NAME "text"` | skipped, counted |
| `#define MAKE(a) ((a) + 1)` | skipped, counted |

---

## 7. Canonical JSON bytes (WIR-01)

Every JSON file `canon` writes (data files; the view model follows VIEWMODEL.md but uses §7.1–§7.3)
is canonical: two correct implementations produce the same bytes.

### 7.1 File

- UTF-8, no BOM. Lines end with LF (`\n`), never CR LF. The file ends with exactly one LF.
- Indentation is spaces only.

### 7.2 Numbers

- **Integers** (all integer types, `Duration` counts, enum codes, bitmasks): decimal digits, `-`
  prefix for negatives, no leading zeros, no `+`, no exponent, no fraction. `0` for zero.
- **Float** (float64): the ECMAScript `Number::toString(x)` algorithm (ECMA-262, radix 10), which
  is also what `JSON.stringify` writes:
  1. If `x` is `+0` or `-0`, write `0`.
  2. If `x < 0`, write `-` and continue with `-x`.
  3. Let `k`, `s`, `n` be integers with `k ≥ 1`, `10^(k-1) ≤ s < 10^k`, `s × 10^(n-k) = x` exactly
     after reading back as float64, and `k` as small as possible (shortest round trip; when several
     `s` qualify, the one closest to `x`). Let `d` be the `k` digits of `s`.
  4. If `k ≤ n ≤ 21`: `d` followed by `n - k` zeros.
  5. Else if `0 < n ≤ 21`: the first `n` digits of `d`, `.`, the remaining `k - n` digits.
  6. Else if `-6 < n ≤ 0`: `0.`, then `-n` zeros, then `d`.
  7. Else: the first digit of `d`; if `k > 1`, `.` and the remaining digits; then `e`, then `+` if
     `n - 1 ≥ 0` else `-`, then `|n - 1|` in decimal.
- **Float32**: the same layout, with `d` and `n` the shortest digits that read back as the same
  float32.
- Go note: `strconv.FormatFloat(x, 'e', -1, 64)` (or `…, 32` for Float32) gives `d` and `n - 1`;
  apply steps 4–7 to it. `strconv.FormatFloat(x, 'g', -1, 64)` does **not** have the same layout.

| Value | Bytes |
|---|---|
| `0.1` | `0.1` |
| `1.0` | `1` |
| `-0.0` | `0` |
| `-2.5` | `-2.5` |
| `100.0` | `100` |
| `123.456` | `123.456` |
| `1/3` | `0.3333333333333333` |
| `0.000001` | `0.000001` |
| `1e-7` | `1e-7` |
| `1.5e-7` | `1.5e-7` |
| `1e20` | `100000000000000000000` |
| `1e21` | `1e+21` |
| `1e300` | `1e+300` |
| `1.7976931348623157e308` | `1.7976931348623157e+308` |
| `5e-324` | `5e-324` |
| `9007199254740994.0` | `9007199254740994` |
| `Float32` `0.1` | `0.1` (not `0.10000000149011612`) |
| `Int` `9223372036854775807` | `9223372036854775807` |

### 7.3 Strings

Exactly as ECMAScript `JSON.stringify` for well-formed strings:

- `"` → `\"`, `\` → `\\`;
- U+0008 → `\b`, U+0009 → `\t`, U+000A → `\n`, U+000C → `\f`, U+000D → `\r`;
- other U+0000–U+001F → `\u00xx` with **lower-case** hex (`\u001b`);
- everything else raw UTF-8: `/`, U+007F, U+2028, U+2029, `<`, `>`, `&` and non-ASCII are not
  escaped.

`"a<TAB>é/\"<ESC>"` is written `"a\té/\"\u001b"`.

### 7.4 Compact and pretty forms

**Compact** (`compact(v)`):

- scalars as above; `null`, `true`, `false`;
- empty array `[]`, empty object `{}`;
- array: `[` + elements joined by `, ` + `]`;
- object: `{` + members `"key": value` joined by `, ` + `}` (one space after `:`, one after `,`,
  none inside the brackets).

**Pretty** at indentation `i` (`pretty(v, i)`), the layout of `JSON.stringify(v, null, 2)`:

- scalars, `[]`, `{}` as in compact;
- non-empty array: `[`, LF, then each element as `i + 2` spaces + `pretty(e, i + 2)`, joined by
  `,` LF, then LF, `i` spaces, `]`;
- non-empty object: `{`, LF, then each member as `i + 2` spaces + `"key": ` + `pretty(v, i + 2)`,
  joined by `,` LF, then LF, `i` spaces, `}`.

No alignment, no padding, anywhere.

---

## 8. `emit json` (WIR-10, GEN-06, EMT-03)

### 8.1 Declaration and file layout

```
emit json { out: "<path>", values: [v1, v2, …] }
```

- Options: `out` (required) and `values` (optional). Any other option, including `mode`, is
  `E8003`. One `emit json` per package (`E8002`, CODEGEN.md).
- `values` lists public top-level `let`s of the package. Default: every public `let`, in
  declaration order (files in path byte order, then source order).
- **File mode**: `out` ends in `.json` (case-sensitive). `values`, explicit or default, must then
  have exactly one element [`E8150`]; the file is `out`.
- **Directory mode**: any other `out` (with or without a trailing `/`). Each value `v` is written
  to `<out>/<v>.json`.
- **No `bare` option** (GEN-06): every data file has the `$schema` wrapper of §8.2, so every one
  carries the marker of §8.4 and the fingerprint check. A non-Canon consumer reads `value` or
  `rows` (`plan.value.combos` for the sweep plan's Lua).
- Two outputs of the build (any target, any package) with the same path, or with paths that differ
  only in letter case, are `E8152` (they would overwrite each other on Windows and macOS). The
  runtime helper files of CODEGEN.md §2.3, written with identical content by several emits into one
  directory, are not a collision. (Two Go emits writing different files into one directory are
  CODEGEN.md's `E8008`.)
- A value whose type contains `Range` or a function type is `E8151`.
- **Data mode link** (EMT-03/05, RLD-01): every value emitted by a `data`-mode code target of the
  package must be written by the package's `emit json`, and each `@reload` value must be written to
  a file named `<value>.json`, all `@reload` values of the package in the same directory
  [`E8153`]. This code also covers a `@reload` value that no `emit json` writes. The generated
  loaders read those files (CODEGEN.md §5.9, §5.11). `embedded` mode embeds the same document
  (§8.2) in the generated code (CODEGEN.md §2.3).

The pipeline example: `emit json { out: "out/potions.json", values: [potions] }` is file mode,
file `pipeline/out/potions.json`; `potions` is `@reload`, and its file is named `potions.json`.

### 8.2 Document

The top-level value is an object with, in order:

1. `"$schema"`: the value's schema identifier (FINGERPRINT.md §2);
2. `"rows"` when the value is a list, a keyed list or a table (the elements, each encoded per §5,
   table rows with `$id`/`$retired`), else `"value"` (records, variants, maps, scalars, enums,
   `none`);
3. `"$fns"` when this file carries the package-level export functions (§5.11).

Layout, byte for byte:

```
{
  "$schema": <compact string>,
  "rows": [
    <compact(row 1)>,
    …
    <compact(row n)>
  ]
}
```

- `rows` with no element is written `  "rows": []`.
- Each row is on one line, indented 4 spaces, in `compact` form.
- `"value": ` is followed by `pretty(value, 2)`.
- `"$fns": ` is followed by `pretty(fns, 2)`.
- Members of the top-level object are separated by `,` LF; the object closes with LF `}` LF.

Every default is filled in, every field is present (§5.4), `input` fields are absent.

### 8.3 Byte-exact samples

**The pipeline** (`examples/pipeline`, GEN-01). `potions` is `[Potion] keyed by id` loaded from
`data/II_POT_HEAL_L.json` then `data/II_POT_HEAL_S.json` (path order). `II_POT_HEAL_S` has no
`nStack`: the default `99` is written. `$isStrong` is `heal >= 2000`. The schema identifier is
`pipeline.Potion@f750790e` (FINGERPRINT.md, vector 1). `out/potions.json` is exactly these 334 bytes
(SHA-256 `2a51028fc9ef1b8b783b4a1c6f8470f61cfc81e06c3f06ef96c738ab7d29bb75`):

```json
{
  "$schema": "pipeline.Potion@f750790e",
  "rows": [
    {"dwID": "II_POT_HEAL_L", "szName": "IDS_PROPITEM_TXT_POT_L", "nHeal": 2500, "dwCooldownMs": 8000, "nStack": 20, "$isStrong": true},
    {"dwID": "II_POT_HEAL_S", "szName": "IDS_PROPITEM_TXT_POT_S", "nHeal": 500, "dwCooldownMs": 3000, "nStack": 99, "$isStrong": false}
  ]
}
```

`examples/pipeline/expected/potions.json` holds exactly these bytes.

**A record value.** If taxonomy.canon had `emit json { out: "out/deck.json", values: [deck] }`
(SHA-256 `3d47a2ebd8ea6ff139c1479016e382b989bc81678df76ee2c974ca2f00c9d87c`):

```json
{
  "$schema": "teamboard.Deck@02af81fb",
  "value": {
    "layouts": [
      "4:1",
      "2:2"
    ],
    "maxHidden": 2
  }
}
```

**A scalar value**, `let assigneeMinRole: Role = maintainer` (the root type is
`sovcommon.roles.Role`, FINGERPRINT.md §2.2):

```json
{
  "$schema": "sovcommon.roles.Role@dc935485",
  "value": "maintainer"
}
```

**A table with a retired entry, a method and a package function.** Package `flow`:

```
package flow

record Status {
  label: String
  next:  [ref Status] = []
  export fn isTerminal(self) -> Bool { return next.isEmpty() }
}

let statuses: stable table Status = {
  open   { label: "Open", next: [closed] }
  closed { label: "Closed" }
  retired stale { label: "Stale" }
}

export fn canTransition(from: ref Status, to: ref Status) -> Bool { return from.next.contains(to) }

emit json { out: "out/statuses.json", values: [statuses] }
```

`out/statuses.json` (SHA-256 `43bc3a5f90c30a159149a5df47092f0e9a4710997841bb53161e64fbc9dd7821`):

```json
{
  "$schema": "flow.Status@b330a789",
  "rows": [
    {"$id": "open", "label": "Open", "next": ["closed"], "$isTerminal": false},
    {"$id": "closed", "label": "Closed", "next": [], "$isTerminal": true},
    {"$id": "stale", "$retired": true, "label": "Stale", "next": [], "$isTerminal": true}
  ],
  "$fns": {
    "canTransition": {
      "open": {
        "open": false,
        "closed": true,
        "stale": false
      },
      "closed": {
        "open": false,
        "closed": false,
        "stale": false
      },
      "stale": {
        "open": false,
        "closed": false,
        "stale": false
      }
    }
  }
}
```

The same table with no entry:

```json
{
  "$schema": "flow.Status@b330a789",
  "rows": []
}
```

**Every legacy encoding in one row** (FINGERPRINT.md vector 6, package `fpdemo`):

```
enum Element @codes(UInt8) @json(codes) { FIRE = 1, WATER = 2, retired WIND = 3 }
enum Flag @codes(UInt32) { tradable = 1, droppable = 2, soulbound = 4 }
enum Side { left, right = "RIGHT" }

record Skill @json(case: snake) {
  id: String @json("dwID")
  reqMp: Int = 0 @json(path: "legacy.reqMp")
  reqFp: Int = 0 @json(path: "legacy.reqFp")
  element: Element? = none @json(none: 0)
  flags: [Flag] where it.isUnique() = [] @json(bits)
  twoHanded: Bool = false @json("bTwoHanded", int)
  side: Side | "both" = "both"
  castTime: Duration? = none @json(unit: s)
  weights: {Element: Float} = {}
  export fn isFree(self) -> Bool { return reqMp == 0 and reqFp == 0 }
}

let skills: [Skill] keyed by id = load.dir("skills/*.json")
emit json { out: "out/skills.json", values: [skills] }
```

The value
`{ id: "SI_FIREBALL", reqMp: 12, element: FIRE, flags: [tradable, soulbound], castTime: 2s,
weights: { FIRE: 0.5, WATER: 1.25 } }` is the row:

```json
{"dwID": "SI_FIREBALL", "legacy": {"reqMp": 12, "reqFp": 0}, "element": 1, "flags": 5, "bTwoHanded": 0, "side": "both", "cast_time": 2, "weights": {"1": 0.5, "2": 1.25}, "$isFree": false}
```

### 8.4 The generated-file marker for JSON (GEN-05)

JSON has no comments, so the marker of a Canon-written JSON file is its `$schema`. Before writing
`out`, `canon build` checks any existing file at that path: it may be overwritten only if, after an
optional UTF-8 BOM, it is a JSON object whose **first** member is `"$schema"` with a string value
matching

```
^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+@[0-9a-f]{8}$
```

Otherwise the write is refused with `E8001` (CLI.md §3.4), and `canon convert` is the only command
that may adopt the file. The view model's marker is its `$schema` (`canon-vm/N`, VIEWMODEL.md).
This regex is wider than the one proposed in GEN-05, which rejected upper-case letters in package
names.

---

## 9. Decode samples

Expected type and JSON → result.

| Expected | JSON | Result |
|---|---|---|
| `Potion` | `{"dwID": "II_POT_HEAL_S", "szName": "IDS_PROPITEM_TXT_POT_S", "nHeal": 500, "dwCooldownMs": 3000}` | `stack` = 99 (default) |
| `Potion` | same plus `"nStack": null` | `E3315` (`stack` is not optional) |
| `Potion` | same plus `"nstack": 5` | `E3301` unknown key `nstack` (unless `partial`) |
| `Potion` | `{"dwID": "X", "dwID": "Y", …}` | `E7104` at the second `dwID` |
| `Potion` | `{"dwID": "II_POT_T", "nHeal": 1e3, …}` | `E7103` |
| `Potion` | `{…, "dwCooldownMs": 1500.5}` | `E3203` (1500.5 ms) |
| `Task` (heistia) | `{"eventType": "COMBAT_KILL_FFA", "filterParam": "", …}` | `filterParam` = `none` (marker `""`) |
| `Task` | `{"eventType": "COMBAT_KILL_FFA", "filterParam": "MI_AIBATT1", …}` | `E3802` (the branch is `Never`) |
| `Task` | `{"eventType": "ECONOMY_DROP_ITEM", "filterParam": "II_GEN_MAT_MOONSTONE", …}` | a `ref items` |
| `Event` | `{…, "type": "spawn_item", "monsterId": "MI_AIBATT1", …}` | `E3301` (`monsterId` belongs to case `spawn_monster`) |
| `Event` | `{…}` without `"type"` | `E7112` |
| `Intent` (teamboard) | `{"tone": "series_1", …}` | `E7111` (`series_1` is the Canon name; the wire is `"series-1"`) |
| `TalentNode` | `{"id": 7, "parent": -1}` | `parent` = `none` |
| `Style` (adventurequest) | `{…, "rewards": {}}` | `rewards` = `none` (marker `{}`) |
| `SkillRow` (sweep_plan) | `{"define": "SI_X", …}` without `legacy` | `reqMp` = 0 |
| `SkillRow` | `{…, "legacy": null}` | `E7110` |
| any | file starting with `EF BB BF` | BOM skipped |
| any | `{"a": 1,}` | `E7109` |
| `table Status` (source wire) | `{"open": {…}, "taken": {"$retired": true, …}}` | two entries, `taken` retired |

---

## 10. Build manifest (LOD-11)

Every `canon check`, `build` and `test` computes a manifest: the exact set of inputs its results
depend on. Its SHA-256 keys the cache (`.canon/cache/`, CLI.md §2.7): equal manifests mean equal
outputs and findings. The manifest is text, UTF-8, LF line endings:

```
canon-manifest v1
compiler <compiler version>
language <MAJOR.MINOR from project.canon>
layer <name>                       one line per --layer, in the order given
lang <code>                        the --lang value (source language if absent)
command <check|build|test>
target <go|cpp|ts|json|view>       one line per selected target, sorted; all when none selected
package <qualified name>           one line per selected package, sorted
file <sha256> <path>               every file read, sorted by path bytes
glob <sha256> <pattern>            every glob evaluated, sorted by pattern bytes
list <sha256> <directory>          every directory listing consulted, sorted by path bytes
```

- `file` lines cover `project.canon`, every `.canon` source, layer and translation file parsed,
  every `canon.lock` read, and every file read by `load`, `load.*` and asset checks. `<path>` is the
  display form (§2.3). `<sha256>` is the lower-case hex SHA-256 of the file's bytes.
- `glob` hashes the list of matched paths (display form, one per line, each followed by LF, in match
  order), so adding or removing a matching file changes the manifest.
- `list` hashes the sorted names (one per line, LF-terminated) of a directory whose listing was used
  (asset existence, TYP-21).
- The cache format behind the key is opaque and versioned by the compiler. Deleting `.canon/` is
  always safe.

---

## 11. Output files, paths and newlines (NFR-04)

- `canon` runs on Linux, macOS and Windows with identical outputs.
- Every file `canon` writes (JSON, view model, generated code, `canon.lock`) is UTF-8 without BOM,
  uses LF line endings only, and ends with exactly one LF.
- Every path written inside a file (headers, view model sources, findings in JSON, the manifest)
  uses `/` and the display form of §2.3. Platform separators are used only at the operating-system
  call boundary.
- Wherever a list of paths is ordered (glob matches, `load.dir`, entry files, findings), the order
  is ascending byte order of the `/`-separated relative path.
- `canon build --check` compares bytes. Repositories that receive generated files should mark them
  `text eol=lf` in `.gitattributes` so that checkouts on Windows do not rewrite line endings.

---

## 12. Versioning (NFR-03, wire part)

- The data wire has no version number of its own: `$schema` identifies the shape, and the
  fingerprint version (`canon-fp v1`, FINGERPRINT.md §6) covers the encoding rules of this
  document. A change to this document that changes the data wire bytes a loader must accept for an
  unchanged shape (a new encoding of `none`, of enums, of rows…) requires a new fingerprint version,
  so that old binaries refuse the new files.
- A change of layout only (whitespace, §7.4) does not: loaders do not depend on it. It still
  changes the goldens.
- The manifest has its own version line (`canon-manifest v1`).

---

## 13. Diagnostics

Codes owned by this document. Location is where the finding points.

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E3203 | error | §5.1: a unit-scaled wire number that is not whole ms (code named by LOD-10) |
| E3315 | error | §5.4 |
| E3316 | error | §4.1 (any `@json` form on a field whose type it does not apply to, `pairs:` rules included), §4.2 (duplicate wire name, path prefix, reserved `$` key), §5.14 |
| E3317 | error | §5.8 |
| E3318 | error | §4.2 |
| E7001 | error | §2 |
| E7002 | error | §6.1 (TYP-06) |
| E7003 | error | §2.1 |
| E7004 | error | §6.1: missing file, directory, permission |
| E7005 | error | §6.5 |
| E7006 | error | §6.1, §6.2 |
| E7007 | error | §6.2 |
| W7101 | warning | §6.8 |
| E7102 | error | §6.8 |
| E7103 | error | §5.1, §5.3, §5.8 |
| E7104 | error | §3.2, §6.6 |
| E7105 | error | §3.1, §3.2, §6.6, §6.7 |
| E7106 | error | §6.3 |
| W7107 | warning | §6.5 |
| E7108 | error | §6.6 |
| E7109 | error | §3.1 |
| E7110 | error | §5 (any wrong JSON kind, `@json(int)` not 0/1, negative bitmask, non-object path intermediate, `$retired` not `true`) |
| E7111 | error | §5.3 |
| E7112 | error | §5.6 |
| E7113 | error | §6.6 |
| E7114 | error | §5.7, §6.5, §6.6 |
| W7115 | warning | §6.5 |
| E7116 | error | §5.9, §6.1, §6.6, §6.7 |
| E7117 | error | §5.14 |
| E8102 | error | §5.1, §5.3, §5.4; reported during verification (code named by LOD-10) |
| E8150 | error | §8.1 |
| E8151 | error | §5.9, §8.1 |
| E8152 | error | §8.1 |
| E8153 | error | §8.1 |

Codes used here and owned elsewhere: `E1104`, `E1119` (GRAMMAR), `E3102`, `E3201`, `E3202`, `E3301`,
`E3302`, `E3312`, `E3501`, `E3506`, `E3701`, `E3802` (TYPES), `E8001`, `E8002`, `E8003`, `E8008`
(CODEGEN). The single catalogue is [ERRORS.md](ERRORS.md).
