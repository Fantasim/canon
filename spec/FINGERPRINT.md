# Canon schema fingerprint

Status: **normative** companion of [SPEC.md](../SPEC.md) v0.1 (§14.4). Defines the `$schema`
identifier of every data file, the canonical shape serialization `canon-fp v1` it is hashed from,
and test vectors.

Owned AUDIT items: EMT-01, EMT-02, NFR-03 (fingerprint versioning), GEN-01 (the fingerprint in the
goldens).

Related documents: [WIRE.md](WIRE.md) (the wire mapping the fingerprint describes, and the JSON
string encoding used below), CODEGEN.md (loaders compile the identifier in and compare it),
TYPES.md (aliases, dependent types, parameterised records).

---

## 1. Purpose

A generated loader must refuse a data file written for another shape: a renamed wire key, a changed
unit or a new field would otherwise load silently wrong. The fingerprint is a hash of everything
that decides how the data wire (WIRE.md §5) of one value must be decoded, and of nothing else.
Values never affect it.

---

## 2. The `$schema` identifier

### 2.1 Format

```
<name>@<hash>
```

- `<hash>` is the first 8 characters of the lower-case hexadecimal SHA-256 of the value's
  `canon-fp v1` text (§4), taken over its UTF-8 bytes.
- `<name>` is defined in §2.2.
- The whole identifier matches `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)+@[0-9a-f]{8}$`
  (the JSON marker of WIRE.md §8.4).

Example: `pipeline.Potion@f750790e`.

### 2.2 The name part

Let `T` be the declared type of the emitted value, aliases expanded.

1. If `T` is `U?`, let `T = U`.
2. If `T` is `[U]`, `[U] keyed by f`, `table U` or `stable table U`, let `T = U`; then if `T` is
   `V?`, let `T = V`. (One level only: `[[Reward]]` stops at `[Reward]`.)
3. If `T` is a record (a parameterised record counts, by its name), a variant or an enum, `<name>`
   is its package-qualified Canon name: `pipeline.Potion`, `teamboard.Status`,
   `sovcommon.roles.Role` (for `let assigneeMinRole: Role` in package `teamboard`).
4. Otherwise `<name>` is `<emitting package>.<value name>`: `let anchors: [Int]` in
   `balance.parity` gives `balance.parity.anchors`.

The name part is for people reading the file. The hash is computed without any Canon type name.

### 2.3 Comparison

A generated loader has the full identifier of its value compiled in and compares it byte for byte
with the file's `$schema` before reading anything else. On a mismatch it refuses the file and names
both identifiers (CODEGEN.md). Two consequences:

- renaming a record type changes the identifier (through its name part) and so requires the
  runtime to be rebuilt, which it must be anyway since the generated class name changed;
- two values with the same shape and the same name part share an identifier, which is correct:
  one loader reads both.

---

## 3. What the fingerprint covers (EMT-01, EMT-02)

| Included | Excluded |
|---|---|
| the value's container: list, keyed list (and the key field's wire path), table, map, optional | Canon names of fields, cases, members, types and values (EMT-02: wire names only) |
| for each record and case field, in declaration order: its wire key path, its type, optionality, `none` encoding, `inline`, the templates and slot count of `@json(pairs:)`, the wire unit of its `Duration`s, `@json(int)` / `@json(bits)` | doc comments, `@since`, `@deprecated` (a deprecated field is still covered as a field) |
| scalar types exactly (`Int` vs `Int32`, `Float` vs `Float32`) | refinements, ranges, regexes, `where` predicates (except the slot count a `@json(pairs:)` list takes from its upper length bound) |
| enum members' wire values, in declaration order, retired included; `@codes` codes and code type; whether the wire holds codes (`@json(codes)`) | defaults (every default is written in data files) |
| variant tag key and cases' wire names, in order, retired included, with their fields | `retired` flags of members, cases and entries; table entries and their keys (data) |
| `$` keys of export functions and the wire type of their results (and argument domains, as map keys) | translated export functions and every function body |
| `$fns` of the package, in the file that carries it | `input` fields (not on the wire); record and package `check`s and `warn`s |
| dependent types: what the branch depends on, the discriminant path, and each branch's wire type | `stable` (a stable table has the same wire as a table), `ordered`, `@reload`, `@cpp`, `@go`, `@ts`, `@files`, views, translations |
| key types of maps and refs (`String`, `Int`, an enum) | the target collection of a `ref` (only its key's wire type matters to the decoder) |
| record parameters (their count) and how each application binds them | asset roots and extensions (an asset is a `String` on the wire) |
| types of other packages reached from the value (visited like local ones) | the data of other packages (for example which event has which `param`) |

The fingerprint is **structural**: it is computed from the type graph reachable from the value,
across packages, with recursive types handled by numbering (§4.3).

---

## 4. The serialization `canon-fp v1`

### 4.1 Text

- UTF-8. Each line ends with LF (`\n`), the last one included. No blank lines, no trailing spaces.
- Indentation is exactly 2 spaces per level.
- `<jstr>` is a JSON string encoded as in WIRE.md §7.3 (`"dwID"`, `"series-1"`, `"$isStrong"`).
- `<path>` is a JSON array of `<jstr>` written with no spaces: `["dwID"]`, `["legacy","reqMp"]`.
- Integers are canonical decimal.

### 4.2 Grammar

```
fp        = "canon-fp v1" LF
            "root " type LF
            { "fn " jstr " " type LF }                      (* package $fns, §4.5 *)
            { block } ;

block     = "type @" N " " ( recordHead | variantHead | enumHead ) LF { line } ;
recordHead  = "record params=" N ;
variantHead = "variant tag=" jstr ;
enumHead    = "enum wire=" ( "string" | "code" ) " codes=" ( "-" | intType ) ;

line      = "  " field                                      (* in a record block *)
          | "  " fnLine                                     (* in a record block *)
          | "  case " jstr LF { "    " field | "    " fnLine }   (* in a variant block *)
          | "  member " jstr " code=" ( "-" | integer ) LF ; (* in an enum block *)

compactJson = (* the marker's JSON, compact form of WIRE.md §7.4 *) ;
field     = "field " ( path | "inline" | pairs ) " " type
            " opt=" ( "0" | "1" )
            " none=" ( "-" | "null" | compactJson )
            " unit=" ( "-" | "ms" | "s" | "m" | "h" | "d" )
            " enc=" ( "-" | "int" | "bits" ) LF ;
fnLine    = "fn " jstr " " type LF ;
pairs     = "pairs(" jstr "," jstr "," N ")" ;               (* @json(pairs:), §4.5 *)

type      = scalar
          | "@" N [ "<" source { "," source } ">" ]         (* named type, with parameter bindings *)
          | "list(" type ")"
          | "keyed(" path "," type ")"
          | "table(" type ")"
          | "map(" type "," type ")"
          | "opt(" type ")"
          | "ref(" type ")"
          | "union(" type { "," jstr } ")"
          | "dep(" source "," path { "," jstr "=" type } ")"
          | "never" ;
source    = "field" path | "param" N | "key" ;
scalar    = "Bool" | "Int" | "Int8" | "Int16" | "Int32" | "UInt8" | "UInt16" | "UInt32" | "UInt64"
          | "Float" | "Float32" | "String" | "Duration" ;
intType   = "Int8" | "Int16" | "Int32" | "Int" | "UInt8" | "UInt16" | "UInt32" | "UInt64" ;
N         = "0" | nonZeroDigit { digit } ;
```

Single spaces separate every token shown with a space; there are no other spaces, except inside
JSON strings.

### 4.3 Numbering (recursive and cross-package types)

Named types (records, parameterised records, variants, enums) are replaced by `@N`. Numbers are
given by a depth-first **pre-order** walk:

```
next = 0
walk(type):                       # left to right through the type expression
  named type R:
    if R has no number: number[R] = next; next = next + 1; walkBody(R)
  otherwise: walk each sub-type, left to right (map key before value, union base before literals,
             dep branches in order, parameter bindings are not types)
walkBody(record):  for each field (non-input, declaration order): walk(field type)
                   for each $ function (declaration order): walk(its wire type, §4.5)
walkBody(variant): for each case in order: its fields, then its $ functions
walkBody(enum):    nothing
```

The walk starts with the `root` type, then each package `fn` line's type, in order. Blocks are
printed in increasing `N`, after the `root` and `fn` lines. A type met again (recursion, or a second
use) is only referenced. Aliases are expanded first and never numbered. Types of other packages are
numbered like local ones.

### 4.4 Types

| Canon | `type` |
|---|---|
| `Bool`, `Int`, sized integers, `Float`, `Float32`, `String`, `Duration` | the scalar name; refinements dropped (`Int(1..=100_000)` → `Int`, `UInt32(1..)` → `UInt32`) |
| alias | its expansion (`type Penya = Int(0..)` → `Int`; `type Layout = String(/…/)` → `String`) |
| asset | `String` |
| record, variant, enum | `@N` |
| a case of a variant used as a type (`Shape.box`) | `case(@N,"<case wire tag>")`: the variant's number and the case's wire tag (DECISIONS 219) |
| parameterised record applied to arguments | `@N<s1,…>`, one `source` per parameter: `field<path>` when the argument is an earlier field of the enclosing record, `param<i>` when it is the enclosing record's `i`-th parameter (from 0), `key` when it is the key of a dependent map |
| `[T]` | `list(T)` |
| `[T] keyed by f` | `keyed(<wire path of f>,T)` |
| `table T`, `stable table T` | `table(T)` |
| `{K: V}` | `map(K,V)` |
| `{e in c: T(e)}` | `map(ref(<key type of c>),T')` with `T'` bound to `key` |
| `T?` nested in a type | `opt(T)`; a field's own outer `?` is written `opt=1` instead |
| `ref c` | `ref(K)`: `K` is `String` for tables and define tables; for a keyed list, the key field's type (`ref(Int)` for `nodes: [TalentNode] keyed by id` with `id: Int(0..)`). When that key type is an enum, it is walked and numbered like any named type (`ref(@N)`, with its block): its wire values decide how the key decodes |
| `A \| "lit1" \| "lit2"` | `union(A,"lit1","lit2")`, literals in source order |
| dependent type, e.g. `Param(eventType)` | `dep(<source>,<path>,"<m1>"=T1,…)`: `<source>` is what the discriminant is read from (`field["eventType"]`, `param0`, `key`); `<path>` is the wire path from that value to the discriminant (`["param"]`, or `[]` when the source itself is the discriminant); one arm per member of the discriminant's enum in declaration order (retired included), keyed by the member's wire value (`"false"`, `"true"` for a `Bool` discriminant); a `Never` arm is `never` |
| `Never` | `never` |
| `Define` (element of `load.defines`) | none: refused (§8) |

### 4.5 Fields, members and `$` functions

- **Field line**: `path` is the field's wire key path (one element, or several with `@json(path:)`),
  or the word `inline` for a `@json(inline)` field, or `pairs("<k template>","<v template>",N)`
  for a `@json(pairs:)` field (WIRE.md §5.14): the two templates as written, `{i}` included, and
  the number of slots `N`. `type` is its type without the outer `?`; for a `pairs` field it is
  `list(@M)`, and the element record's block is printed as usual (its fields' `unit=` and `enc=`
  decide how slot values are read; their own key names are printed but unused by this field).
- `opt=1` when the field's type is optional (outer `?`), else `0`.
- `none=` is `-` when `opt=0`; else `null`, or the compact JSON of the `@json(none:)` marker
  (WIRE.md §7.4): `none=-1`, `none=""`, `none={}`, `none=0`.
- `unit=` is `-` when the field's type holds no `Duration` outside named types; else the wire unit,
  **explicitly `ms` by default** (so adding `@json(unit: ms)` changes nothing).
- `enc=` is `int` for `@json(int)`, `bits` for `@json(bits)`, else `-`.
- **Enum**: `wire=code` with `@json(codes)`, else `wire=string`; `codes=` the `@codes` integer type
  or `-`; one `member` line per member, in declaration order, retired included: its wire string and
  its code (or `-`).
- **Variant**: `tag=` the tag key (`"kind"` by default); one `case` line per case with its wire name,
  its field lines and `fn` lines indented 4 spaces.
- **`$` functions** (WIRE.md §5.11): one `fn` line per non-translated export function of the record
  or case, in declaration order, after its fields: the JSON key (`"$isStrong"`) and the wire type of
  what is stored: the result type for no parameters, and `map(A1,map(A2,…R))` with one `map` level
  per finite parameter (the parameter's type as the key type) otherwise.
- **Package functions**: when the value's data file carries `$fns`, one `fn` line per package
  export function after the `root` line, keyed by the function's Canon name (the key it has in
  `$fns`), with the same wire type rule.

### 4.6 Hash

`hash = lower-case hex of SHA-256(UTF-8 bytes of the whole text, final LF included)`, truncated to
its first 8 characters. 32 bits are enough: the question a loader asks is "is this the shape I was
built for", not "which of millions of shapes is this".

---

## 5. What changes the hash

| Edit | Hash changes? |
|---|---|
| rename a field that has `@json("…")` | no |
| rename a field without `@json`, or add `@json(case: snake)` to its record | yes (its wire name changes) |
| rename a record, variant or enum | no (the name part of `$schema` changes) |
| reorder fields, add or remove a field, change a field's type | yes |
| `Int(1..=100)` → `Int(0..)`, change a regex or a `where` | no |
| `Int` → `Int32`, `Float` → `Float32` | yes |
| `T` → `T?`, or change `@json(none:)` | yes |
| `@json(unit: s)` → `@json(unit: ms)` | yes |
| add `@json(unit: ms)` to a `Duration` field that had none | no |
| change a default, a doc comment, a check, a view, a translation | no |
| add an enum member or variant case | yes |
| retire an enum member, a case or a table entry; add a table entry | no |
| add an `export fn isX(self) -> Bool` | yes (a new `$` key) |
| add a translated `export fn`, or change any function body | no |
| change a `ref items` into a `ref monsters` (both keyed by strings) | no |
| add an `input` field | no |
| `table` → `stable table`, add `@reload` | no |
| change a `@json(pairs:)` template, or the list's upper length bound (the slot count) | yes |

---

## 6. Versioning (NFR-03)

- The first line names the algorithm: `canon-fp v1`. Any change to this document that changes the
  text for some type, and any change to WIRE.md that changes the data wire for an unchanged shape,
  requires a new version line (`canon-fp v2`). Every hash then changes, so every runtime refuses
  every data file until rebuilt: such a change ships only in a major compiler release.
- A compiler implements exactly one fingerprint version.
- The view model has its own format identifier (`canon-vm/N`, VIEWMODEL.md), unrelated to this one.

---

## 7. Test vectors

Each vector gives the Canon source it comes from, the exact serialization (every line ends with LF,
including the last), its full SHA-256 and the resulting `$schema`. The hashes were computed with
`sha256sum` over the exact bytes shown. Vector 1, reproducible from a shell:

```
printf 'canon-fp v1\nroot keyed(["dwID"],@0)\ntype @0 record params=0\n  field ["dwID"] String opt=0 none=- unit=- enc=-\n  field ["szName"] String opt=0 none=- unit=- enc=-\n  field ["nHeal"] Int opt=0 none=- unit=- enc=-\n  field ["dwCooldownMs"] Duration opt=0 none=- unit=ms enc=-\n  field ["nStack"] Int opt=0 none=- unit=- enc=-\n  fn "$isStrong" Bool\n' | sha256sum
f750790ed1f521f00bf26fa8aee70cba38d2b306e29a6e80f636a0a64a9d01f5  -
```

### Vector 1: Pipeline potions

`let potions: [Potion] keyed by id` in `examples/pipeline/potion.canon`. `id`, `name` and `heal`
lose their refinements; `cooldown` has unit `ms`; `stack`'s default is not covered; `isStrong` is a
`$` key; `healFor` is translated and not covered.

```
canon-fp v1
root keyed(["dwID"],@0)
type @0 record params=0
  field ["dwID"] String opt=0 none=- unit=- enc=-
  field ["szName"] String opt=0 none=- unit=- enc=-
  field ["nHeal"] Int opt=0 none=- unit=- enc=-
  field ["dwCooldownMs"] Duration opt=0 none=- unit=ms enc=-
  field ["nStack"] Int opt=0 none=- unit=- enc=-
  fn "$isStrong" Bool
```

- 342 bytes, SHA-256 `f750790ed1f521f00bf26fa8aee70cba38d2b306e29a6e80f636a0a64a9d01f5`
- `$schema`: `pipeline.Potion@f750790e`

### Vector 2: Teamboard statuses

`let statuses: stable table Status` in `examples/teamboard/taxonomy.canon`. `Tone` comes from
`sovcommon.ui` and is covered by its wire values (`series-1`…); `next` is `[ref Status]`, a list of
string keys; `by: Actor? = none`. The package functions (`canTransition`…) are not in this file's
`$fns` (they go to the first emitted value).

```
canon-fp v1
root table(@0)
type @0 record params=0
  field ["tone"] @1 opt=0 none=- unit=- enc=-
  field ["label"] String opt=0 none=- unit=- enc=-
  field ["terminal"] Bool opt=0 none=- unit=- enc=-
  field ["next"] list(ref(String)) opt=0 none=- unit=- enc=-
  field ["requires"] list(@2) opt=0 none=- unit=- enc=-
  field ["optional"] list(@2) opt=0 none=- unit=- enc=-
  field ["by"] @3 opt=1 none=null unit=- enc=-
type @1 enum wire=string codes=-
  member "warning" code=-
  member "info" code=-
  member "accent" code=-
  member "success" code=-
  member "neutral" code=-
  member "danger" code=-
  member "series-1" code=-
  member "series-2" code=-
  member "series-3" code=-
  member "series-4" code=-
  member "series-5" code=-
  member "series-6" code=-
type @2 enum wire=string codes=-
  member "assignee" code=-
  member "fixed_in" code=-
  member "reason" code=-
  member "duplicate_of" code=-
type @3 enum wire=string codes=-
  member "reporter" code=-
  member "triager" code=-
  member "reporter_or_triager" code=-
```

- 1033 bytes, SHA-256 `4aed3fdee7560d05e1f0dff7159611503c44a3c7451e49cd09e4aca1e7d17c73`
- `$schema`: `teamboard.Status@4aed3fde`

### Vector 3: Event config (inline variant, units, cross-package record)

`let eventConfig: EventConfig` in `examples/resource/events/event.canon`. `kind: EventKind
@json(inline)` with tag `"type"`; `ItemCount` expands to `list(Int)`; `Window`, `Weekday` and
`TimeOfDay` come from `sovcommon.time`; the `ref monsters` / `ref items` define tables are
`ref(String)`.

```
canon-fp v1
root @0
type @0 record params=0
  field ["version"] Int opt=0 none=- unit=- enc=-
  field ["events"] keyed(["id"],@1) opt=0 none=- unit=- enc=-
type @1 record params=0
  field ["id"] String opt=0 none=- unit=- enc=-
  field ["worldId"] UInt32 opt=0 none=- unit=- enc=-
  field ["targetCount"] Int opt=0 none=- unit=- enc=-
  field inline @2 opt=0 none=- unit=- enc=-
  field ["schedule"] list(@4) opt=0 none=- unit=- enc=-
  field ["rollMode"] @7 opt=0 none=- unit=- enc=-
type @2 variant tag="type"
  case "spawn_monster"
    field ["monsterId"] ref(String) opt=0 none=- unit=- enc=-
    field ["spawnRegion"] @3 opt=0 none=- unit=- enc=-
    field ["monsterLifetimeSec"] Duration opt=1 none=null unit=s enc=-
  case "spawn_item"
    field ["itemId"] ref(String) opt=0 none=- unit=- enc=-
    field ["itemCount"] list(Int) opt=0 none=- unit=- enc=-
    field ["spawnRegion"] @3 opt=0 none=- unit=- enc=-
    field ["groundLifetimeSec"] Duration opt=1 none=null unit=s enc=-
  case "monster_drop_inject"
    field ["itemId"] ref(String) opt=0 none=- unit=- enc=-
    field ["itemCount"] list(Int) opt=0 none=- unit=- enc=-
    field ["levelMin"] Int opt=0 none=- unit=- enc=-
    field ["levelMax"] Int opt=0 none=- unit=- enc=-
type @3 record params=0
  field ["left"] Float opt=0 none=- unit=- enc=-
  field ["top"] Float opt=0 none=- unit=- enc=-
  field ["right"] Float opt=0 none=- unit=- enc=-
  field ["bottom"] Float opt=0 none=- unit=- enc=-
type @4 record params=0
  field ["day"] @5 opt=0 none=- unit=- enc=-
  field ["startUtc"] @6 opt=0 none=- unit=- enc=-
  field ["endUtc"] @6 opt=0 none=- unit=- enc=-
type @5 enum wire=string codes=-
  member "Sun" code=-
  member "Mon" code=-
  member "Tue" code=-
  member "Wed" code=-
  member "Thu" code=-
  member "Fri" code=-
  member "Sat" code=-
type @6 record params=0
  field ["hour"] Int opt=0 none=- unit=- enc=-
  field ["minute"] Int opt=0 none=- unit=- enc=-
type @7 enum wire=string codes=-
  member "authoritative" code=-
  member "local_budget" code=-
```

- 2033 bytes, SHA-256 `1ba50ac89d2617aee6f690a29fd1d0ba5a0be88740f8cfc77dd7118771c2d2e4`
- `$schema`: `resource.events.EventConfig@1ba50ac8`

### Vector 4: Heistia (dependent type, `none` marker)

`let heistia: HeistiaConfig` in `examples/resource/heistia/heistia.canon`. `filterParam:
Param(eventType)? = none @json(none: "")`: the branch is chosen by the `param` of the entry that the
earlier field `eventType` refers to; arms follow `ParamKind` in declaration order.

```
canon-fp v1
root @0
type @0 record params=0
  field ["version"] Int opt=0 none=- unit=- enc=-
  field ["tasks"] list(@1) opt=0 none=- unit=- enc=-
  field ["rewardPools"] list(list(@4)) opt=0 none=- unit=- enc=-
  field ["buffPool"] list(@5) opt=0 none=- unit=- enc=-
type @1 record params=0
  field ["eventType"] ref(String) opt=0 none=- unit=- enc=-
  field ["filterParam"] dep(field["eventType"],["param"],"none_"=never,"monster"=ref(String),"item"=ref(String),"dungeon"=ref(String),"element"=@2,"stat"=never,"upgradeType"=ref(String),"gameMode"=String,"questStyle"=@3) opt=1 none="" unit=- enc=-
  field ["targetPerPlayer"] Int opt=0 none=- unit=- enc=-
  field ["maxDurationMin"] Duration opt=0 none=- unit=m enc=-
  field ["description"] String opt=0 none=- unit=- enc=-
type @2 enum wire=string codes=UInt8
  member "FIRE" code=1
  member "WATER" code=2
  member "ELECTRICITY" code=3
  member "WIND" code=4
  member "EARTH" code=5
type @3 enum wire=string codes=UInt8
  member "daily" code=1
  member "weekly" code=2
  member "forever" code=3
  member "season_pass" code=4
  member "pvp" code=5
  member "azure" code=6
type @4 record params=0
  field ["itemId"] ref(String) opt=0 none=- unit=- enc=-
  field ["quantity"] Int opt=0 none=- unit=- enc=-
  field ["weight"] Int opt=0 none=- unit=- enc=-
type @5 record params=0
  field ["buffResId"] Int opt=0 none=- unit=- enc=-
  field ["durationSec"] Duration opt=0 none=- unit=s enc=-
  field ["weight"] Int opt=0 none=- unit=- enc=-
```

- 1491 bytes, SHA-256 `39fd85236e45fc388bc91810ac7921dea1f207503734605e690e97563a90334b`
- `$schema`: `resource.heistia.HeistiaConfig@39fd8523`

### Vector 5: Sweep plan (maps with enum keys, nested lists)

`let plan: Plan` in `examples/balance/parity/sweep_plan.canon`, the value its `emit json` writes.

```
canon-fp v1
root @0
type @0 record params=0
  field ["charName"] String opt=0 none=- unit=- enc=-
  field ["hitsPerBlock"] Int opt=0 none=- unit=- enc=-
  field ["actionDelayMs"] Duration opt=0 none=- unit=ms enc=-
  field ["totemIndex"] map(@1,Int) opt=0 none=- unit=- enc=-
  field ["combos"] list(@2) opt=0 none=- unit=- enc=-
type @1 enum wire=string codes=-
  member "pve" code=-
  member "pvp" code=-
type @2 record params=0
  field ["job"] String opt=0 none=- unit=- enc=-
  field ["jobToken"] String opt=0 none=- unit=- enc=-
  field ["level"] Int opt=0 none=- unit=- enc=-
  field ["weapon"] String opt=0 none=- unit=- enc=-
  field ["weaponId"] Int opt=0 none=- unit=- enc=-
  field ["weaponKind"] String opt=0 none=- unit=- enc=-
  field ["ranged"] Bool opt=0 none=- unit=- enc=-
  field ["profiles"] list(@1) opt=0 none=- unit=- enc=-
  field ["skills"] list(@3) opt=0 none=- unit=- enc=-
type @3 record params=0
  field ["slot"] Int opt=0 none=- unit=- enc=-
  field ["id"] Int opt=0 none=- unit=- enc=-
  field ["mp"] Int opt=0 none=- unit=- enc=-
```

- 1062 bytes, SHA-256 `a2305fd5ae20367a66f10dc291e602891469b8fbfe3be46d0ac4b2912f28a09f`
- `$schema`: `balance.parity.Plan@a2305fd5`

### Vector 6: Legacy encodings (path, marker, codes on the wire, bits, int, union, snake case)

`let skills: [Skill] keyed by id` in the `fpdemo` package of WIRE.md §8.3. `Element` has
`@json(codes)` and a retired member (still covered); `Flag` is a `@codes` enum written as bits;
`Skill` is `@json(case: snake)`, so `castTime` is `"cast_time"`.

```
canon-fp v1
root keyed(["dwID"],@0)
type @0 record params=0
  field ["dwID"] String opt=0 none=- unit=- enc=-
  field ["legacy","reqMp"] Int opt=0 none=- unit=- enc=-
  field ["legacy","reqFp"] Int opt=0 none=- unit=- enc=-
  field ["element"] @1 opt=1 none=0 unit=- enc=-
  field ["flags"] list(@2) opt=0 none=- unit=- enc=bits
  field ["bTwoHanded"] Bool opt=0 none=- unit=- enc=int
  field ["side"] union(@3,"both") opt=0 none=- unit=- enc=-
  field ["cast_time"] Duration opt=1 none=null unit=s enc=-
  field ["weights"] map(@1,Float) opt=0 none=- unit=- enc=-
  fn "$isFree" Bool
type @1 enum wire=code codes=UInt8
  member "FIRE" code=1
  member "WATER" code=2
  member "WIND" code=3
type @2 enum wire=string codes=UInt32
  member "tradable" code=1
  member "droppable" code=2
  member "soulbound" code=4
type @3 enum wire=string codes=-
  member "left" code=-
  member "RIGHT" code=-
```

- 891 bytes, SHA-256 `ae120ca0b24f0327178c8fb3f9120666b416a25c95f7423fd7350b67686011d8`
- `$schema`: `fpdemo.Skill@ae120ca0`

### Vector 7: Teamboard deck (a record value)

`let deck: Deck` in taxonomy.canon; `Layout` is an alias of `String(/^[1-9]:[1-9]$/)`.

```
canon-fp v1
root @0
type @0 record params=0
  field ["layouts"] list(String) opt=0 none=- unit=- enc=-
  field ["maxHidden"] Int opt=0 none=- unit=- enc=-
```

- 155 bytes, SHA-256 `02af81fb280a6be20404e1625adf06181db459193d62eff9151b78b73062c163`
- `$schema`: `teamboard.Deck@02af81fb`

### Vector 8: A scalar value of another package's enum

`let assigneeMinRole: Role` in package `teamboard`; the name part is the enum's own package.
`ordered` is not covered.

```
canon-fp v1
root @0
type @0 enum wire=string codes=-
  member "member" code=-
  member "gm_junior" code=-
  member "gm_senior" code=-
  member "maintainer" code=-
  member "owner" code=-
  member "admin" code=-
```

- 211 bytes, SHA-256 `dc93548589d7cf10a715fa70222d949915644fcf2bc7f026977905e079984b75`
- `$schema`: `sovcommon.roles.Role@dc935485`

### Vector 9: Package functions in `$fns`

`let statuses: stable table Status` in the `flow` package of WIRE.md §8.3, the first emitted value
of its package, so it carries `$fns` with `canTransition`.

```
canon-fp v1
root table(@0)
fn "canTransition" map(ref(String),map(ref(String),Bool))
type @0 record params=0
  field ["label"] String opt=0 none=- unit=- enc=-
  field ["next"] list(ref(String)) opt=0 none=- unit=- enc=-
  fn "$isTerminal" Bool
```

- 245 bytes, SHA-256 `b330a789e4613ff68fbd725b19cc110d4a5d4520e84e95722988f9fa78eeb666`
- `$schema`: `flow.Status@b330a789`

### Vector 10: Adventure quests (parameterised record, dependent map, dependent keys, literal union)

`let adventureQuests: AdventureQuestConfig` in
`examples/resource/adventurequest/adventurequest.canon`. `hourlyTargets: {e in eventTypes:
HourlyTarget(e)}` binds `HourlyTarget`'s parameter to the map `key`; inside it, `SpecificKey(e) =
Param(e) | "default"` reads its discriminant from `param0`. `LevelDiff` keeps camelCase: `case:
snake` does not propagate. `Element` is first met inside the dependent arms.

```
canon-fp v1
root @0
type @0 record params=0
  field ["version"] Int opt=0 none=- unit=- enc=-
  field ["global"] @1 opt=0 none=- unit=- enc=-
  field ["unique_per_batch"] list(ref(String)) opt=0 none=- unit=- enc=-
  field ["styles"] map(@3,@4) opt=0 none=- unit=- enc=-
  field ["target_multipliers"] map(@8,@9) opt=0 none=- unit=- enc=-
  field ["hourly_targets"] map(ref(String),@10<key>) opt=0 none=- unit=- enc=-
  field ["rate_amplifiers"] map(ref(String),list(@12)) opt=0 none=- unit=- enc=-
  field ["task_filters"] map(ref(String),@14) opt=0 none=- unit=- enc=-
type @1 record params=0
  field ["min_level"] Int opt=1 none=null unit=- enc=-
  field ["max_quests_per_style"] Int opt=1 none=null unit=- enc=-
  field ["completion_reroll_cooldown_sec"] Duration opt=0 none=- unit=s enc=-
  field ["levelDiff"] @2 opt=0 none=- unit=- enc=-
type @2 record params=0
  field ["minMob"] Int opt=0 none=- unit=- enc=-
  field ["maxMob"] Int opt=0 none=- unit=- enc=-
  field ["minGiant"] Int opt=0 none=- unit=- enc=-
  field ["maxGiant"] Int opt=0 none=- unit=- enc=-
type @3 enum wire=string codes=UInt8
  member "daily" code=1
  member "weekly" code=2
  member "forever" code=3
  member "season_pass" code=4
  member "pvp" code=5
  member "azure" code=6
type @4 record params=0
  field ["reroll_time_sec"] Duration opt=1 none=null unit=s enc=-
  field ["target_time_sec"] Duration opt=0 none=- unit=s enc=-
  field ["max_target_days"] Int opt=0 none=- unit=- enc=-
  field ["task_slot_weights"] list(Int) opt=0 none=- unit=- enc=-
  field ["task_weights"] map(ref(String),Int) opt=0 none=- unit=- enc=-
  field ["rewards"] @5 opt=1 none={} unit=- enc=-
type @5 record params=0
  field ["type"] @6 opt=0 none=- unit=- enc=-
  field ["amounts"] map(@7,Int) opt=0 none=- unit=- enc=-
type @6 enum wire=string codes=UInt8
  member "activity_points" code=1
  member "season_pass_exp" code=2
type @7 enum wire=string codes=UInt16
  member "Stage_1" code=100
  member "Stage_2" code=101
  member "Stage_3" code=102
type @8 enum wire=string codes=-
  member "2_tasks" code=-
  member "3_tasks" code=-
type @9 record params=0
  field ["default"] Float opt=0 none=- unit=- enc=-
  field ["overrides"] map(ref(String),Float) opt=0 none=- unit=- enc=-
type @10 record params=1
  field ["rates"] map(@7,Int) opt=1 none=null unit=- enc=-
  field ["rates_by_specific"] map(union(dep(param0,["param"],"none_"=never,"monster"=ref(String),"item"=ref(String),"dungeon"=ref(String),"element"=@11,"stat"=never,"upgradeType"=ref(String),"gameMode"=String,"questStyle"=@3),"default"),map(@7,Int)) opt=1 none=null unit=- enc=-
  field ["daily_max_by_specific"] map(union(dep(param0,["param"],"none_"=never,"monster"=ref(String),"item"=ref(String),"dungeon"=ref(String),"element"=@11,"stat"=never,"upgradeType"=ref(String),"gameMode"=String,"questStyle"=@3),"default"),Int) opt=1 none=null unit=- enc=-
  field ["is_daily_flat"] Bool opt=0 none=- unit=- enc=-
type @11 enum wire=string codes=UInt8
  member "FIRE" code=1
  member "WATER" code=2
  member "ELECTRICITY" code=3
  member "WIND" code=4
  member "EARTH" code=5
type @12 record params=0
  field ["condition"] @13 opt=0 none=- unit=- enc=-
  field ["priority"] Int opt=1 none=null unit=- enc=-
  field ["min_value"] Int opt=0 none=- unit=- enc=-
  field ["max_value"] Int opt=0 none=- unit=- enc=-
  field ["bonus_min"] Float opt=0 none=- unit=- enc=-
  field ["bonus_max"] Float opt=0 none=- unit=- enc=-
type @13 enum wire=string codes=-
  member "net_worth" code=-
  member "level" code=-
type @14 record params=0
  field ["level"] @15 opt=0 none=- unit=- enc=-
type @15 record params=0
  field ["min"] Int opt=0 none=- unit=- enc=-
  field ["max"] Int opt=0 none=- unit=- enc=-
```

- 3719 bytes, SHA-256 `6dc7944d6240802b4af81487a4d053bfc59cf9412c7c67e81ed7b33ef9620924`
- `$schema`: `resource.adventurequest.AdventureQuestConfig@6dc7944d`

---

## 8. Diagnostics

This document defines no diagnostic of its own. The fingerprint is always computable for a type
that has a wire form, except `Define` (below); types without one are rejected by WIRE.md (`E8151`,
`E7116`). The `Define` record of `load.defines`, and so a define table, has no form in
`canon-fp v1` (§4.4): a value that holds one is refused wherever a fingerprint is needed, by `emit json`
(`E8151`) and by a `data` or `embedded` code emit (`E8012`, CODEGEN.md §4.4). A `ref` into a define
table is `ref(String)`. A mismatch at run time is reported by the generated loader as an error
value (CODEGEN.md), not as a Canon finding.
