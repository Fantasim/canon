# Canon standard library

Normative companion to [SPEC.md](../SPEC.md) §20 and §2.5. Version: **0.1 (draft)**.

The library is closed and versioned with the language. This document lists every built-in
function, method and member, with its full generic signature, its result on empty input, its
errors, its ordering guarantees and its step cost. It applies the accepted answers of AUDIT
STD-01 to STD-06 and TYP-07.

Related documents: [TYPES.md](TYPES.md) (typing of calls, §12.2; equality, §7.5),
[EVALUATION.md](EVALUATION.md) (step budget, §12; hard errors, §7.1), [WIRE.md](WIRE.md)
(`load`).

---

## 1. Conventions

### 1.1 Signatures

```
receiver.method<T, K: Ord>(param: Type, name: Type = default) -> Result
function<T>(param: Type) -> Result
```

- Type parameters are bound as TYPES.md §12.2 says: from non-lambda arguments first, then
  lambdas. Refinements are dropped when binding; refs bind as refs.
- Parameters may be passed by position or by name, like user functions.
- Every method that takes a function accepts a lambda, a shorthand (`.f`, `.m()`) or a
  top-level `fn` of the right type.

Constraints on type parameters:

| Constraint | Types |
|---|---|
| `Num` | `Int` (any width), `Float` (any width) |
| `NumD` | `Num` or `Duration` |
| `Ord` | `Int`, `Float`, `Duration`, `String`, an `ordered` enum (TYPES.md §7.5) |
| `Eq` | every type except function types (TYPES.md §7.5) |
| `Key` | a valid map key type (TYPES.md §9.2) |

A type argument that breaks its constraint is `E3002` at the argument (or `E3310` for `Ord`).

### 1.2 Receivers

| Name | Types | Elements |
|---|---|---|
| `Seq(T)` | `[T]`, `[T] keyed by f`, `table T` | list elements; table entries in entry order |
| `Keyed(T)` | `[T] keyed by f`, `table T` | as above, with key lookup |
| `Map(K, V)` | `{K: V}`, dependent maps | entries in insertion order |
| `String` | `String` and its refinements | bytes (UTF-8) |
| `Range` | `Range` | integers `start`, `start + 1`, … |

Every method of `Seq(T)` works on keyed lists and tables. Methods that return a list return a
**plain** list `[T]` (not keyed, not a table), in which table and keyed-list elements keep their
identity (TYPES.md §6.3). No method changes its receiver: values are immutable.

### 1.3 Costs

Each call costs one step for the call node (EVALUATION.md §12). The **Cost** column gives the
additional steps. Each invocation of a function argument costs its own steps (invocation plus body
nodes). `n` is the number of elements of the receiver. "Visited" counts elements until the method
stops.

### 1.4 Errors

Errors listed as `E4xxx` are hard errors located at the call expression (EVALUATION.md §7.1).
Static errors are reported by the checker.

---

## 2. Free functions

### 2.1 Conversions

| Signature | Result | Errors | Cost |
|---|---|---|---|
| `Int(x: Float) -> Int` | truncates toward zero | `E4103` if outside the `Int` range | 0 |
| `Int(x: Int) -> Int` | `x` (a sized value as `Int`) | | 0 |
| `Float(x: Int) -> Float` | exact when \|x\| ≤ 2^53, else nearest-even | | 0 |
| `Float(x: Float) -> Float` | `x` | | 0 |
| `String<T: Eq>(x: T) -> String` | the canonical text form (§9) | | 1 per value visited (§9.1) |

There is no conversion between `Int` and `Duration`: write `n * 1s` for `Int → Duration` and
`Int(d / 1ms)` for `Duration → Int` (milliseconds).

### 2.2 Math (STD-05)

All arguments of one call have the same base type (an integer literal is accepted where the
others are `Float`). Mixing types is `E3002`.

| Signature | Result | Errors | Cost |
|---|---|---|---|
| `abs<T: NumD>(x: T) -> T` | absolute value | `E4101` for the smallest `Int` or `Duration` | 0 |
| `min<T: NumD>(a: T, b: T, …: T) -> T` | smallest argument; at least 2 arguments (`E3004`) | | 0 |
| `max<T: NumD>(a: T, b: T, …: T) -> T` | largest argument | | 0 |
| `clamp<T: NumD>(x: T, lo: T, hi: T) -> T` | `min(max(x, lo), hi)` | `E4108` if `lo > hi` | 0 |
| `floor(x: Float) -> Int` | largest integer ≤ `x` | `E4103` | 0 |
| `ceil(x: Float) -> Int` | smallest integer ≥ `x` | `E4103` | 0 |
| `round(x: Float) -> Int` | nearest integer, **half away from zero** (`round(2.5) == 3`, `round(-2.5) == -3`) | `E4103` | 0 |
| `sqrt(x: Float) -> Float` | IEEE square root | `E4104` if `x < 0` | 0 |
| `pow(x: Float, y: Float) -> Float` | IEEE `pow` | `E4104` if the result is NaN or infinite (`pow(0.0, -1.0)`, `pow(-8.0, 0.5)`) | 0 |

`min` and `max` return the first of equal arguments. Floats compare by IEEE value, except that
**`-0.0` orders before `+0.0`** (the `minimum` and `maximum` of IEEE 754-2019): `min(0.0, -0.0)` is
`-0.0` and `max(-0.0, 0.0)` is `0.0`, whatever the argument order. `clamp` is `min(max(x, lo), hi)`
with that rule. The translations compute exactly this (CONFORMANCE.md §3), so the evaluator and
generated code agree bit for bit. These free functions are shadowed by any user name (TYPES.md
§3.3): inside `record LevelRange { min: Int, max: Int }`, `min` is the field.

### 2.3 Graphs (STD-04)

`next` has one of two forms, chosen from the function's result type: `fn(T) -> [T]` (list of
successors, in order) or `fn(T) -> T?` (at most one successor; a function returning `T` also
fits). Nodes are compared with `==` (TYPES.md §7.5): entries by identity. `next` is invoked at
most once per distinct node.

| Signature | Result | Errors | Cost |
|---|---|---|---|
| `reachable<T: Eq>(from: T, next: …) -> [T]` | every node reachable from `from`, including `from`, in **depth-first preorder**: `from` first, then recursively each successor in `next` order, skipping nodes already listed | | 1 per node listed + 1 per successor examined |
| `cycles<T: Eq>(xs: Seq(T), next: …) -> [T]` | the elements `x` of `xs` from which a path of length ≥ 1 leads back to `x` (a member of a strongly connected component of size > 1, or a node with a self-loop), in the order of `xs`, each once | | 1 per node explored + 1 per successor examined |
| `topoSort<T: Eq>(xs: Seq(T), next: …) -> [T]` | the distinct elements of `xs`, each placed **after** every element of `xs` it points to (`next(x)` lists what `x` depends on); successors outside `xs` are ignored. At each step the next element is the **first in `xs` order** whose successors are all placed (Kahn's algorithm with input order as the tie-break) | `E4501` on a cycle | 1 per node + 1 per successor examined |

`E4501` names one cycle: starting from the first unplaced element, follow its first unplaced
successor until an element repeats; the message lists that loop in text form (`open -> taken ->
open`). A self-loop is a cycle.

Examples: `reachable(from: initialStatus, next: .next)` lists every status reachable from the
initial one. `cycles(nodes, next: .parent)` returns the talent nodes on a parent cycle, including
a node that is its own parent.

---

## 3. Members of values

Members are read without parentheses (TYPES.md §3.5). They cost nothing beyond their node.

| Receiver | Member | Type | Meaning |
|---|---|---|---|
| enum value | `.name` | `String` | Canon name of the member |
| | `.index` | `Int` | position from 0 in declaration order, retired members included |
| | `.wire` | `String` | wire value (the name, or the `= "…"` string) |
| | `.code` | `Int` | the code; only with `@codes` |
| variant value | `.kind` | `Kind(V)` | its case (TYPES.md §8.3) |
| table entry, `ref` into a table | `.id` | `String` | the key |
| | `.retired` | `Bool` | whether the entry is retired |
| `Define` entry (`load.defines`) | `.value` | `Int` | the define's value |
| `Range` | `.start` | `Int` | first integer |
| | `.end` | `Int` | end, exclusive; `E4002` on an open range |

A ref reads the members of its entry (implicit dereference).

---

## 4. Sequences: `Seq(T)`

`[T]`, `[T] keyed by f` and `table T`. Element equality is TYPES.md §7.5.

### 4.1 Size and access

| Signature | Result | Empty receiver | Errors | Cost |
|---|---|---|---|---|
| `len() -> Int` | number of elements | `0` | | 1 |
| `isEmpty() -> Bool` | `len() == 0` | `true` | | 1 |
| `first() -> T?` | first element | `none` | | 1 |
| `last() -> T?` | last element | `none` | | 1 |
| `first(pred: fn(T) -> Bool) -> T?` | first element for which `pred` is `true` | `none` | | visited |
| `get(i: Int) -> T?` (lists only) | element `i`; negative counts from the end | `none` | | 1 |
| `contains(x: T) -> Bool` | some element equals `x` | `false` | | visited |
| `indexOf(x: T) -> Int?` | index of the first element equal to `x` | `none` | | visited |

Indexing and slicing are operators (TYPES.md §9.1):

| Form | Result | Errors | Cost |
|---|---|---|---|
| `xs[i]` (list) | element `i`; negative counts from the end | `E4002` unless `-len ≤ i < len` | 0 |
| `xs[a..b]`, `xs[a..]`, `xs[..b]`, `xs[a..=b]`, `xs[r]` | the elements from `a` (default 0) to `b` exclusive (default `len`), after adding `len` to a negative bound | `E4002` unless `0 ≤ a ≤ b ≤ len` | 1 per element of the result |

On keyed lists and tables, `xs[k]` is a key lookup (§5); a `Range` index is always a positional
slice.

### 4.2 Transformations

| Signature | Result | Empty | Errors | Cost |
|---|---|---|---|---|
| `map<U>(f: fn(T) -> U) -> [U]` | `f` of each element, in order | `[]` | | n |
| `filter(pred: fn(T) -> Bool) -> [T]` | elements for which `pred` is `true`, in order | `[]` | | n |
| `flatMap<U>(f: fn(T) -> [U]) -> [U]` | concatenation of `f` of each element | `[]` | | n + length of the result |
| `flatten() -> [U]` (receiver `Seq([U])`) | concatenation of the inner lists | `[]` | `E3002` if the elements are not lists | n + length of the result |
| `reverse() -> [T]` | elements in reverse order | `[]` | | n |
| `sortBy<K: Ord>(f: fn(T) -> K) -> [T]` | elements in ascending order of `f`, **stable** | `[]` | | 1 per key comparison (§4.5) |
| `unique() -> [T]` | first occurrence of each distinct element, in order | `[]` | | n |
| `enumerate() -> [Pair(Int, T)]` | `(0, x0)`, `(1, x1)`, … | `[]` | | n |
| `pairs() -> [Pair(T, T)]` | every `(xs[i], xs[j])` with `i < j`, ordered by `i` then `j` | `[]` | | 1 per pair |
| `zip<U>(other: Seq(U)) -> [Pair(T, U)]` | `(xs[i], other[i])` for each `i` | `[]` | `E4105` if the lengths differ | n |
| `intersect(b: Seq(T)) -> [T]` | elements of the receiver that occur in `b`, in receiver order, duplicates kept | `[]` | | n + len(b) |
| `union(b: Seq(T)) -> [T]` | the receiver, then the elements of `b` that are not in the receiver, in `b` order, each once | `b` without duplicates | | n + len(b) |
| `diff(b: Seq(T)) -> [T]` | elements of the receiver that do not occur in `b`, in receiver order | `[]` | | n + len(b) |
| `groupBy<K: Key>(f: fn(T) -> K) -> {K: [T]}` | elements grouped by `f`; keys in **first-seen** order, elements in receiver order | `{}` | | n |
| `toMap<K: Key, V>(keyF: fn(T) -> K, valF: fn(T) -> V) -> {K: V}` | one entry per element, in order | `{}` | `E4502` on a duplicate key | n |
| `join(sep: String) -> String` (receiver `Seq(String)`) | elements separated by `sep` | `""` | `E3002` on other element types | n |

`xs + ys` concatenates two lists (TYPES.md §7.1); it is an operator node and costs 0 extra.

### 4.3 Predicates and aggregates

| Signature | Result | Empty | Errors | Cost |
|---|---|---|---|---|
| `any(pred: fn(T) -> Bool) -> Bool` | some element satisfies `pred` (stops at the first) | `false` | | visited |
| `all(pred: fn(T) -> Bool) -> Bool` | every element satisfies `pred` (stops at the first failure) | `true` | | visited |
| `count(pred: fn(T) -> Bool) -> Int` | number of elements satisfying `pred` | `0` | | n |
| `isUnique() -> Bool` | no two elements are equal | `true` | | n |
| `sum() -> T` (`T: NumD`) | sum, left to right | `0`, `0.0` or `0s` | `E4101` (Int, Duration), `E4104` (Float); `E3314` if the element type is unknown | n |
| `min() -> T?` (`T: NumD`) | smallest element, first of ties | `none` | | n |
| `max() -> T?` (`T: NumD`) | largest element, first of ties | `none` | | n |
| `minBy<K: Ord>(f: fn(T) -> K) -> T?` | element with the smallest key, first of ties | `none` | | n |
| `maxBy<K: Ord>(f: fn(T) -> K) -> T?` | element with the largest key, first of ties | `none` | | n |

`sum()` of `[Int(0..)]` or any sized integer list is an `Int`. `[].sum()` without an expected
type is `E3314`. `min()` and `max()` on a list of `Float` order `-0.0` before `+0.0`, as the free
functions do (§2.2).

### 4.4 Key functions

`sortBy`, `minBy`, `maxBy` and `groupBy` invoke `f` exactly once per element, in receiver
order, before comparing.

### 4.5 The sort

`sortBy` is this top-down merge sort, so that the number of comparisons (the cost) is the same
everywhere:

```
sort(a):                      // a: list of (key, element)
  if len(a) <= 1: return a
  mid = len(a) / 2            // integer division
  l = sort(a[0:mid]); r = sort(a[mid:])
  merge: while both non-empty:
    compare r[0].key < l[0].key          // one comparison, one step
    if true: take r[0] else take l[0]    // equal keys keep l first: stable
  append the rest of the non-empty side (no comparison)
```

Keys compare with `<` of TYPES.md §7.5 (strings by bytes, floats by IEEE value).

---

## 5. Keyed collections: `Keyed(T)`

Tables and keyed lists have every method of §4, plus key access. The key type `KT` is `String`
for a table and the key field's type for a keyed list. Where a key is expected, a `ref T` into
the same collection is also accepted.

| Signature | Result | Errors | Cost |
|---|---|---|---|
| `xs[k: KT]`, `xs.k` | entry with key `k` (`.k` only for identifier keys) | `E4002` if missing | 0 |
| `get(k: KT) -> T?` | entry with key `k` | | 1 |
| `find(k: KT) -> T?` | same as `get` | | 1 |
| `at(i: Int) -> T` | entry at position `i`; negative counts from the end | `E4002` if out of range | 1 |
| `keys() -> [ref T]` | the keys, in order | | n |
| `values() -> [T]` | the entries, in order | | n |
| `active() -> [T]` (tables only) | entries that are not retired, in order | | n |

`get` on a keyed collection takes a **key**, while `get` on a plain list takes a position.

Membership: `x in xs` and `xs.contains(x)` accept either an element (`T` or `ref T`: true when
an element equals it, entries by identity) or a key (`KT`: true when the key exists). The static
type of `x` decides; a bare identifier that does not resolve in scope is a key (TYPES.md §4.1).

---

## 6. Maps: `Map(K, V)`

| Signature | Result | Empty | Errors | Cost |
|---|---|---|---|---|
| `m[k: K]` | value for `k` | | `E4002` if missing | 0 |
| `len() -> Int` | number of entries | `0` | | 1 |
| `isEmpty() -> Bool` | `len() == 0` | `true` | | 1 |
| `keys() -> [K]` | keys in insertion order | `[]` | | n |
| `values() -> [V]` | values in insertion order | `[]` | | n |
| `get(k: K) -> V?` | value for `k` | `none` | | 1 |
| `contains(k: K) -> Bool` | `k` is a key (same as `k in m`) | `false` | | 1 |
| `map<U>(f: fn(V) -> U) -> {K: U}` | same keys, `f` of each value | `{}` | | n |
| `filter(pred: fn(K, V) -> Bool) -> {K: V}` | entries for which `pred(k, v)` is true, in order | `{}` | | n |
| `any(pred: fn(K, V) -> Bool) -> Bool` | stops at the first `true` | `false` | | visited |
| `all(pred: fn(K, V) -> Bool) -> Bool` | stops at the first `false` | `true` | | visited |
| `count(pred: fn(K, V) -> Bool) -> Int` | number of entries satisfying `pred` | `0` | | n |

Map predicates take two parameters `(k, v)` (STD-01). `for k, v in m` iterates in insertion
order. Key equality is TYPES.md §7.5 (refs by key).

---

## 7. Strings (STD-02)

Lengths and indexes count **bytes** of the UTF-8 encoding. Case mapping is ASCII only.

| Signature | Result | Errors | Cost |
|---|---|---|---|
| `len() -> Int` | number of bytes | | 1 |
| `isEmpty() -> Bool` | `len() == 0` | | 1 |
| `contains(s: String) -> Bool` | `s` occurs (`""` always does) | | 1 |
| `startsWith(s: String) -> Bool` | prefix test | | 1 |
| `endsWith(s: String) -> Bool` | suffix test | | 1 |
| `find(s: String) -> Int?` | byte index of the first occurrence; `"x".find("")` is `0` | | 1 |
| `split(sep: String) -> [String]` | the parts between occurrences of `sep`, left to right: `"a,,b".split(",") == ["a", "", "b"]`, `"".split(",") == [""]` | `E4106` if `sep` is `""` | 1 |
| `trim() -> String` | without leading and trailing ASCII whitespace (space, `\t`, `\n`, `\v`, `\f`, `\r`) | | 1 |
| `lower() -> String` | `A`–`Z` mapped to `a`–`z`; other bytes unchanged | | 1 |
| `upper() -> String` | `a`–`z` mapped to `A`–`Z` | | 1 |
| `replace(a: String, b: String) -> String` | every non-overlapping occurrence of `a`, left to right, replaced by `b` | `E4106` if `a` is `""` | 1 |
| `matches(re) -> Bool` | RE2 search (§8); `re` must be a regex literal | | 1 |
| `s[a..b]` and the other slice forms | bytes `a` to `b` exclusive, with the bounds of §4.1 | `E4002` out of range; `E4107` if a bound is not at a character boundary | 1 |

`+` concatenates. `s[i]` (a single index) is not defined (`E3007`): use a slice. String
comparison with `<` is byte order.

---

## 8. Regular expressions (STD-03)

- Syntax: RE2 (Go `regexp/syntax`, Perl flags), over UTF-8. An invalid pattern is `E1114`
  (GRAMMAR.md), static, at the literal.
- Semantics: **search**. A string matches when the pattern matches any substring (Go
  `regexp.MatchString`). Anchor explicitly for a whole-string match: `/^II_[A-Z0-9_]+$/`.
- This applies to `matches(re)` and to regex refinements (`String(/^IDS_/)`, TYPES.md §7.4).
- Regex literals appear only as the sole argument of a type refinement or of `.matches(`
  (GRAMMAR.md, LEX-01).
- Input patterns (runtime) are restricted further (EVALUATION.md §11.3).
- The view model flags patterns as RE2 (VIEWMODEL.md).

---

## 9. Canonical text form (STD-06)

The canonical text form is used by string interpolation (`"{x}"`), `String(x)`, check
messages, finding messages that quote values, `canon explain`, and the text of a failing
`expect`.

### 9.1 Values

| Value | Text | Example |
|---|---|---|
| `Bool` | `true`, `false` | `true` |
| `Int` (any width) | decimal, `-` for negatives | `-42` |
| `Float` | ECMAScript `Number::toString` (§9.2) | `0.1`, `1`, `1e+21`, `1e-7` |
| `Float32` (static type) | the same algorithm with the shortest digits that round-trip as binary32 | `0.1` |
| `Duration` | canonical duration literal (§9.3) | `1h30m`, `0s`, `-250ms` |
| `String` | the string itself; **quoted** when nested in a composite (§9.4) | `Heal` / `"Heal"` |
| enum member | its Canon name (not the wire value) | `series_1` |
| `Kind` value | the case name | `spawn_item` |
| `ref` | its key: a table key as written, a keyed-list key in its text form without quotes | `open`, `II_POT_HEAL_L`, `3` |
| `none` | `none` | |
| optional holding a value | the value | |
| list, keyed list | `[` elements separated by `, ` `]` | `[1, 2]`, `[]` |
| map | `{` `key: value` separated by `, ` `}`, in insertion order; keys in their text form (strings quoted) | `{Stage_1: 20}`, `{"Cap": [a]}` |
| table | `{` `key: entry` separated by `, ` `}` | `{open: Status{…}}` |
| record | `Name{` `field: value` for every field in declaration order, inputs omitted `}`; `Name` unqualified, without arguments | `TimeOfDay{hour: 8, minute: 30}` |
| case | `case{…}` like a record; a case without fields is its name | `item{define: II_GEN_GOLD, count: 1}`, `nothing` |
| table entry or keyed-list element (a `T` value) | as a record (its identity is not printed) | |
| `Range` | `start..end` (exclusive end), `start..` when open | `0..11` for `0..=10` |
| pair | `(a, b)` | `(0, open)` |
| string-literal union value | the literal, as a string | |
| dependent value | the value of its branch | |
| `Define` entry | `Define{value: 5}` | |

A function value cannot be formatted (`E4503`, static). Formatting costs 1 step per value
visited: a scalar is 1, a composite is 1 plus its components.

### 9.2 Floats

For a finite `x ≠ 0`, let `k`, `n`, `s` be integers with `k ≥ 1`, `10^(k−1) ≤ s < 10^k`,
`s × 10^(n−k) = x` (for binary32: the value rounds to `x` as binary32), and `k` as small as
possible; if several `s` qualify, take the one for which `s × 10^(n−k)` is closest to `x`. With
`d` the `k` digits of `s`:

| Condition | Text |
|---|---|
| `k ≤ n ≤ 21` | `d` followed by `n − k` zeros |
| `0 < n ≤ 21` | the first `n` digits of `d`, `.`, the remaining `k − n` digits |
| `−6 < n ≤ 0` | `0.`, then `−n` zeros, then `d` |
| otherwise, `k = 1` | `d`, `e`, the sign `+` or `-`, then `|n − 1|` |
| otherwise | the first digit, `.`, the other digits, `e`, the sign, then `|n − 1|` |

A negative value is prefixed by `-`. `0` and `-0` both print `0`. The same algorithm writes floats
in emitted JSON (WIRE.md). Examples: `1.0 → 1`, `0.15 → 0.15`, `2.5e6 → 2500000`, `1e21 →
1e+21`, `0.0000001 → 1e-7`, `123e-20 → 1.23e-18`.

### 9.3 Durations

Decompose the absolute number of milliseconds into days (86,400,000 ms), hours, minutes,
seconds and milliseconds. Print each non-zero part with its unit (`d`, `h`, `m`, `s`, `ms`),
largest first, with no spaces. Zero prints `0s`. A negative duration gets a `-` prefix.
Examples: `90s → 1m30s`, `48h → 2d`, `5400000ms → 1h30m`, `1500ms → 1s500ms`.

### 9.4 Strings inside composites

Nested strings (list elements, map keys and values, record fields) are written as Canon string
literals: `"` … `"`, escaping `"` as `\"`, `\` as `\\`, newline as `\n`, tab as `\t`, carriage
return as `\r`, and any other byte below `0x20` and `0x7F` as `\u{X}` (uppercase hex, no leading
zeros). Braces are not escaped. A top-level string (the value interpolated or converted itself)
is written raw.

### 9.5 Format specs

`{x:spec}` in a string template (grammar in GRAMMAR.md, LEX-02):

```
spec = [ "+" ] [ "," ] [ "." digits ]          digits: 0 to 20
```

A spec is allowed only on `Int` (any width) and `Float` (any width) expressions; on any other
type it is `E4503` (static). The spec's syntax, including the limit of 20 digits after `.`, is
GRAMMAR.md's (`E1101`, GRAMMAR.md §2.6). Translated functions allow no spec (SPEC §9.4,
CONFORMANCE.md §2.2). The steps, in order:

1. **`.N`**: an `Int` is printed in decimal followed by `.` and `N` zeros (no `.` when `N = 0`).
   A `Float` is rounded to `N` decimals from its **exact binary value**, ties away from zero, and
   printed in fixed notation with exactly `N` decimals (never an exponent). Without `.N`, the
   canonical form is used (§9.2).
2. **`,`**: groups the digits of the integer part by three from the right with `,`. It has no
   effect on a canonical form that uses an exponent.
3. **`+`**: prefixes `+` to a value that is not negative. A result whose digits are all zero is
   printed without `-` (`-0.001` with `.2` is `0.00`), so `+` gives `+0`, `+0.00`.

| Value | Spec | Text |
|---|---|---|
| `1234567` | `,` | `1,234,567` |
| `-1234.5` | `,.2` | `-1,234.50` |
| `0.125` | `.2` | `0.13` (exact binary value, tie away from zero) |
| `2.675` | `.2` | `2.67` (the binary value is below 2.675) |
| `500` | `.2` | `500.00` |
| `5` | `+` | `+5` |
| `0` | `+` | `+0` |
| `-3` | `+` | `-3` |

---

## 10. Other built-ins

- **`fail(at, message)`, `warn(at, message)`**: statements, only lexically inside `check { }`
  blocks (`E1105`, GRAMMAR.md). `at` is any value, `message` a `String` template.
  EVALUATION.md §8 gives their findings. Cost: the statement step.
- **`load`, `load.dir`, `load.defines`, `load.csv`, `load.text`**: typing in TYPES.md §5.1,
  decoding in WIRE.md. `load.defines` has type `table Define`, `load.text` has type `String`,
  the others take the expected type. Loading costs 0 steps.
- **Ranges**: `r.len() -> Int` (`end − start`; `E4002` if open), `r.isEmpty() -> Bool`,
  `r.contains(x: Int) -> Bool` (same as `x in r`), each cost 1. `for i in a..b` iterates in
  ascending order.

---

## Diagnostics

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E4105 | error | `zip` of sequences of different lengths |
| E4106 | error | `split("")`, `replace("", b)` |
| E4107 | error | string slice not at a character boundary |
| E4108 | error | `clamp(x, lo, hi)` with `lo > hi` |
| E4501 | error | a cycle among the elements of `topoSort` |
| E4502 | error | duplicate key in `toMap` |
| E4503 | error | format spec on a non-numeric type, or a function value in a template or `String(x)` (static) |

Codes defined elsewhere and raised by built-ins: `E4002`, `E4101`, `E4103`, `E4104`
(EVALUATION.md); `E3002`, `E3004`, `E3007`, `E3310`, `E3314` (TYPES.md); `E1101`, `E1114`
(GRAMMAR.md). The single catalogue is [ERRORS.md](ERRORS.md).
