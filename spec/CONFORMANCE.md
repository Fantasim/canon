# Canon conformance tests

Version: **0.1 (draft)**, companion to [SPEC.md](../SPEC.md) §9.4 and §15.6. Normative.

An `export fn` with a runtime input is **translated** into Go, C++ and TypeScript (SPEC §9.4). This
document fixes what a translated function may contain, what the translation must compute (including
every error), how `canon check` and `canon build` pick the input vectors of the generated
conformance tests, and the test files themselves. Names, files and the pure-function shape come from
[CODEGEN.md](CODEGEN.md) §5.10.

"Must" and "is an error" are requirements on the compiler. Codes are listed in
[§8](#8-diagnostics).

---

## Contents

1. [Principle](#1-principle)
2. [What a translated function may contain](#2-what-a-translated-function-may-contain)
3. [Operations and their errors](#3-operations-and-their-errors)
4. [TypeScript integers](#4-typescript-integers)
5. [Float exactness](#5-float-exactness)
6. [Vector selection](#6-vector-selection)
7. [Test files](#7-test-files)
8. [Diagnostics](#8-diagnostics)

---

## 1. Principle

- The Canon evaluator is the reference. A translation is correct when, for every input, it
  returns the value the evaluator returns, or signals the error the evaluator reports, with the
  same code.
- Each Canon operation of the body becomes **exactly one** call of a checked helper (§3), in the
  evaluator's order. So the first error of a translation is the evaluator's first error.
- The translation is emitted once, as a **pure function** of the fields it reads and its
  parameters. The public method and the conformance test both call it (SPEC §9.4, GO-02).
- A test compares codes, never messages.

---

## 2. What a translated function may contain

### 2.1 Signature

| | Allowed | Otherwise |
|---|---|---|
| parameters | `Bool`, every integer type, `Float`, `Float32`, `String`, `Duration`, enums | `E9006` (refs, records, lists, maps, variants, assets) |
| optional parameters | none | `E9003` |
| return type | the parameter types, and `ref T` (returned as its key) | `E9004` |
| refinements of parameters and return | ranges on numbers and durations; sized integer types; `Float32` | `E9007` (`where`, regex, `String` length) |

A method with runtime inputs must be called by at least one `test` of its package, so that the
conformance test has a receiver (`E9008`). A package-level function needs no test.

Only a code emit (`go`, `cpp`, `ts`) translates a function, so vectors, `E9008` and `E9009` apply
only to a package with at least one of them. A package emitted only to `json` or `view` has no
translated function and no conformance file (§7.1).

### 2.2 The portable subset

SPEC §9.4 lists the constructs. Precisely:

- **Statements:** `let` (immutable), `if` / `else if` / `else`, `return`. Every path returns.
- **Expressions:** literals of the allowed types (a duration literal is its millisecond count);
  `if … else …` expressions; `and`, `or`, `not`; `??` on an optional field of `self`.
- **Arithmetic:** `+ - * / %` and unary `-` on integers, `Float` and `Duration` as typed by
  TYPES.md (`Duration * Int`, `Int * Duration`, `Duration / Int`, `Duration / Duration → Float`);
  every integer type computes as `Int` (TYP-03).
- **Comparisons:** `== != < <= > >=` on numbers and durations; `==` and `!=` on `Bool`, `String`
  and enums; `< <= > >=` on `ordered` enums; `is` on a variant-typed path of `self` (the pure
  function receives the case kind).
- **Functions:** `Float(i)`, `Int(f)`, `min`, `max` (two or more arguments), `abs`, `clamp`,
  `floor`, `ceil`, `round`.
- **Templates:** `"…{x}…"` where every `x` is a `String`, an integer or an enum (written by its
  Canon name), without format spec. A `Float` or `Duration` in a template is `E9005`.
- **Reads:** paths of `self` through non-collection fields (`self.startUtc.hour`, `self.bonus?.value
  ?? 0`), enum members.
- **Calls** to other export fns of the same package: a translated one (its pure function is
  called), a precomputed method of `self` (its value is a constant of the receiver, read like a
  field: a vector takes it from the evaluator, as it takes the fields), a lookup function
  (called; it is baked data).

Everything else is `E9001`, naming the construct: loops, `var` and assignment, `match`, lambdas,
comprehensions, collection literals, indexing, every collection, string and graph method,
`String(x)`, `sqrt`, `pow`, ordering on `String`, format specs, calls to non-exported functions,
`load`, `fail`, `warn`, `E.members`, `F(a).members` and `F(a).typeName` (TYPES.md §4.3,
DECISIONS 306). These are exactly the constructs whose behaviour differs across targets
or needs a runtime library.

### 2.3 The pure function

Its parameters, in order: one per distinct `self` path the body reads (in field declaration order,
depth first, named by the path joined with `_`: `heal`, `startUtc_hour`; `self_` is prefixed when
the name equals a declared parameter's), then the declared parameters.
Types: every integer is `int64` / `int64_t` / `number`; `Float` and `Float32` are `float64` /
`double` / `number`; `Duration` is milliseconds in `int64` / `int64_t` / `number`; `String` is
`string` / `std::string_view` / `string`; enums are the enum type. An optional read `T?` is `(x T,
xOk bool)` in Go, `std::optional<T>` in C++, `T | null` in TS.

**On entry**, for each parameter in order:

1. representability: in TypeScript, every integer parameter (declared or read from `self`) must be
   a safe integer (`E8303`, §4); in every target, a `Float` parameter must be finite (`E4104`);
2. the declared parameter's sized type (`E3201`; for a `Duration`, the range ±9 223 372 036 854 ms
   of TYPES.md §7.2, also `E3201`), then its range refinement (`E3204`), the order in which
   EVALUATION.md §4.3 checks a stored value. Values read from `self` are not checked: they were
   validated at build time.

The order is per parameter: one parameter passes steps 1 and 2 before the next is checked. The Go,
C++ and TypeScript translations and the evaluator's TS mode (§4) all follow it (DECISIONS 311).

**On exit**, the result's sized type or `Duration` range (`E3201`) and range refinement (`E3204`)
are checked; a `Float32` result is rounded to nearest-even binary32, and one that overflows is
`E3202`; a sized result is narrowed after the check. The public method converts its arguments to the pure types (sized integers widen, Go `time.Duration`
becomes `rt.DurationToMs(d)`) and converts the result back.

**Evaluation order.** Left to right, operands before their operator; `and`, `or`, `??` and `if`
evaluate only the branch taken. Go and TypeScript evaluate call arguments left to right. C++ does
not, so a C++ helper call with two or more arguments that can signal an error binds those arguments
to `const` locals first, in order:

```cpp
const int64_t t1 = canon::MulInt(a, b);
const int64_t t2 = canon::DivInt(c, d);
return canon::AddInt(t1, t2);
```

### 2.4 Signalling errors

| Target | The translation | The public method |
|---|---|---|
| Go | panics with `*rt.EvalError{Code, Message}` (`rt.Fail`) | lets the panic propagate |
| C++ | calls `canon::OnEvalError(code, message)`; if the handler returns, the helper returns `0` / `0.0` and the function carries on (DECISIONS 19) | returns whatever the function returns |
| TS | throws `CanonEvalError` (`.code`) | lets it propagate |

The default C++ handler prints the error and calls `std::abort()`. A runtime installs its own with
`canon::SetEvalErrorHandler` (for example to log, count, and use the fallback value).

---

## 3. Operations and their errors

The helpers are in `rt` (Go), `canon_runtime.h` (C++) and the TS helper block (CODEGEN.md §6.3,
§7.4, §8.2). The evaluator must implement the same semantics; STDLIB.md and EVALUATION.md are
expected to agree with this table.

| Canon | Result | Error | Go | C++ | TS |
|---|---|---|---|---|---|
| `a + b`, `a - b`, `a * b` (Int, Duration) | exact | `E4101` if outside int64 | `rt.AddInt` `SubInt` `MulInt` | `canon::AddInt` … | `canonAdd` … |
| `a / b` (Int, Duration / Int) | truncated toward zero | `E4102` if `b == 0`; `E4101` for `INT64_MIN / -1` | `rt.DivInt` | `canon::DivInt` | `canonDiv` |
| `a % b` | sign of `a`; `x % -1` is `0` | `E4102` if `b == 0` | `rt.ModInt` | `canon::ModInt` | `canonMod` |
| `-a`, `abs(a)` | | `E4101` for `INT64_MIN` | `rt.NegInt` `AbsInt` | `canon::NegInt` `AbsInt` | `canonNeg` `canonAbs` |
| `min`, `max` (Int, Duration) | | none | builtin `min` `max` | `canon::MinInt` `MaxInt` | `Math.min` `Math.max` |
| `clamp(x, lo, hi)` (Int, Duration) | `min(max(x, lo), hi)` | `E4108` if `lo > hi` | `rt.ClampInt` | `canon::ClampInt` | `canonClamp` |
| `a + b`, `a - b`, `a * b`, `a / b`, `a % b` (Float) | IEEE 754 double, round to nearest; `%` is `fmod` | `E4104` if the result is NaN or infinite (`x / 0.0` included) | `rt.AddFloat` … `ModFloat` | `canon::AddFloat` … | `canonF(a + b)` … |
| `-a`, `abs(a)` (Float) | exact | none | `rt.NegFloat` `AbsFloat` | `canon::NegFloat` `AbsFloat` | `-a`, `Math.abs` |
| `min`, `max` (Float) | `-0.0 < +0.0` | none | `rt.MinFloat` `MaxFloat` | `canon::MinFloat` `MaxFloat` | `canonMinF` `canonMaxF` |
| `clamp` (Float) | `min(max(x, lo), hi)` with the rule above | `E4108` if `lo > hi` | `rt.ClampFloat` | `canon::ClampFloat` | `canonClampF` |
| `Float(i)` | nearest double (ties to even) | none | `rt.IntToFloat` | `canon::IntToFloat` | the number itself |
| `Int(f)` | truncated toward zero | `E4103` unless `-2^63 <= f < 2^63` | `rt.FloatToInt` | `canon::FloatToInt` | `canonToInt` |
| `floor(f)`, `ceil(f)` | | `E4103` as `Int(f)` | `rt.FloorFloat` `CeilFloat` | `canon::FloorFloat` `CeilFloat` | `canonFloor` `canonCeil` |
| `round(f)` | half away from zero | `E4103` as `Int(f)` | `rt.RoundFloat` | `canon::RoundFloat` | `canonRound` |
| `d1 / d2` (Duration) | `Float(d1) / Float(d2)` | `E4102` if `d2 == 0`; `E4104` | `rt.DivDuration` | `canon::DivDuration` | `canonDivDuration` |
| comparisons | IEEE for Float (`-0.0 == 0.0`); enum order by declaration | none | native | native | native (enums through `<E>Index`) |
| `x ?? y`, `and`, `or`, `not`, `if` | short-circuit | none | native | native | native |
| template | concatenation; integers in decimal, enums by Canon name | none | `strconv.FormatInt`, `String()` | `std::to_string`, `ToName` | `String(n)`, `<E>Names` |
| sized type or `Duration` range of a parameter or result | | `E3201` | `rt.CheckIntWidth` | `canon::CheckIntWidth` | `canonCheckWidth` |
| range refinement of a parameter or result | | `E3204` | `rt.CheckIntRange` `CheckFloatRange` | `canon::CheckIntRange` `CheckFloatRange` | `canonCheckRange` |
| store into `Float32` | nearest-even binary32 | `E3202` on overflow | `rt.ToFloat32` | `canon::ToFloat32` | `canonF32` |
| `Float` argument | | `E4104` if not finite | `rt.CheckFloatArg` | `canon::CheckFloatArg` | `canonF` |

Notes:

- The codes of range checks are TYPES.md's: `E3201` for a sized integer type, `E3204` for a range
  refinement, `E3202` for a non-finite or `Float32`-overflowing stored value.
- `min` and `max` on `Float` order `-0.0` before `+0.0`, as STDLIB.md §2.2 states.
- Durations compute in int64 milliseconds (overflow `E4101`, as for `Int`); a stored `Duration`
  is limited to ±9 223 372 036 854 ms (TYPES.md §7.2), which the entry and exit checks enforce with
  `CheckIntWidth` (§2.3).

---

## 4. TypeScript integers

TypeScript computes integers as `number`. A value outside ±(2⁵³−1) cannot be represented exactly,
so the TS translation signals **`E8303`** where the other targets compute or signal `E4101`:

- every integer parameter is checked on entry (§2.3);
- every integer-producing helper checks its result with `Number.isSafeInteger` (`canonInt`);
  `-0` is normalized to `0`;
- `canonDiv` and `canonMod` compute through `BigInt`, so their result is exact;
- `Int(f)`, `floor`, `ceil` and `round` signal `E4103` first when `f` is outside int64, then
  `E8303` when the result is not safe.

Each vector therefore has a **TS expectation**, computed by the evaluator in *TS mode*: the same
evaluation, where every integer parameter, every integer read from `self`, and the exact result of
every integer operation (including those that would overflow int64) is checked against
±(2⁵³−1) right after the operation's own checks, and the first failure is `E8303`. A vector whose
inputs are not safe integers expects `E8303` in TS, unless an earlier parameter fails first (§2.3,
DECISIONS 311). In TS test files, an input outside the safe
range is written as its nearest double, in ECMAScript `Number::toString` form
(`9223372036854776000`, `-9223372036854776000`).

`@ts(bigint)` is not supported on the parameters and result of a translated function (`E9006`).

---

## 5. Float exactness

Results must be **bit-identical** to the evaluator's (CNF-03):

- Comparison: Go `math.Float64bits(got) == math.Float64bits(want)`, C++ `std::memcmp` of the two
  `double`s, TS `Object.is(got, want)`. So `-0.0` and `0.0` differ.
- Every Canon operation is one IEEE operation, rounded to nearest; there is no fused multiply-add
  and no extended precision. C++ runtimes must build generated code with `-ffp-contract=off`
  (GCC, Clang) or `/fp:precise` (MSVC, its default), and with SSE2 arithmetic on 32-bit x86
  (`-msse2 -mfpmath=sse`; MSVC's default since VS 2012). Go and JavaScript already comply.
- Float literals in vectors are written as ECMAScript `Number::toString` gives them (the shortest
  text that reads back exactly), with `.0` appended to an integral value in C++ and Go, and
  `-0.0` for negative zero. All three compilers read decimal literals correctly rounded.
- `Float32` values are rounded to `float` when stored and when returned; C++ literals carry `f`.

---

## 6. Vector selection

For each translated function `F` with parameters `p1 … pn` (CNF-01). All sets are deduplicated
by value (floats by bit pattern).

### 6.1 Receivers (methods only)

The receivers are the `self` values of the calls of `F` made while running the package's `test`
blocks, in the order the calls are evaluated, each **projected** on the paths the body reads
(§2.3). Distinct projections are kept in first-seen order. None is `E9008`. `canon check` and
`canon build` both run the tests to collect these calls, and only for that. Each package's tests
run as `canon test <pkg>` runs them: values forced afresh, never reusing stage A's, per-package
counters of `project.budget` steps (EVALUATION.md §12.2), in declaration order. None of their findings is reported, and the calls made before any stop are kept
(EVALUATION.md §1). A call whose
receiver reads a precomputed method of `self` that fails there (an error code or a limit, on a
receiver built only in a test) gives no receiver and no vector, but still counts as a call for
`E9008`; `canon test` reports the failure.

### 6.2 Candidate values of one parameter, for one receiver `r`

Let `τ` be the parameter's type, `[lo, hi]` its type range (int64 for `Int`, the sized range,
±9 223 372 036 854 for `Duration`, ±1.7976931348623157e308 for `Float`, ±3.4028234663852886e38
for `Float32`) and `R` its refinement range if any (for integers and durations, inclusive bounds:
for `a..b`, the upper bound is `b − 1`). For a float, `pred(b)` and `succ(b)` are the
representable values adjacent to `b` in the parameter's type (binary64 for `Float`, binary32 for
`Float32`: `math.Nextafter`, `math.Nextafter32`).

| `τ` | Candidates |
|---|---|
| integers, `Duration` | the arguments given to this parameter by every test call of `F` ∪ {0, 1, −1, lo, hi} ∪ {b−1, b, b+1 for each finite bound b of R} ∪ {v−1, v, v+1 for each value v of `r` read by the body with the same kind (integer values for integer parameters, durations for duration parameters)} |
| `Float`, `Float32` | test arguments ∪ {0.0, −0.0, 1.0, −1.0, 0.5, −0.5, 1e300, −1e300, lo, hi} ∪ {pred(b), b, succ(b) for each finite bound b of R, the exclusive upper bound of `..b` included} ∪ {v−1, v, v+1 for each `Float` value v of `r` read by the body}, rounded to `float` for `Float32` |
| `Bool` | false, true |
| enum | every member, in declaration order, retired ones included |
| `String` | "" ∪ test arguments |

Then:

1. drop candidates outside `[lo, hi]` and non-finite floats (values outside the refinement stay:
   they expect `E3204`);
2. sort: numbers ascending (−0.0 before 0.0), strings by bytes, enums by declaration order,
   false before true.

### 6.3 Combining parameters

With lists `L1 … Ln` for receiver `r`:

- If `|L1| × … × |Ln| ≤ 256`: the full cartesian product, in lexicographic order (`p1` varies
  slowest).
- Otherwise, pairwise coverage, built deterministically:

```
V = []
covered = {}
for i in 0 … n−1:
  for j in i+1 … n−1:
    for a in 0 … |Li|−1:
      for b in 0 … |Lj|−1:
        if (i, a, j, b) in covered: continue
        v[k] = Lk[len(V) mod |Lk|] for every k
        v[i] = Li[a]; v[j] = Lj[b]
        append v to V
        add every pair (k, index of v[k], l, index of v[l]), k < l, to covered
sort V lexicographically, deduplicate
```

With a single parameter, the product is the list itself.

### 6.4 Order and deduplication

1. **Test vectors**: every test call of `F`, in evaluation order, as (projected receiver,
   arguments).
2. Then, for each receiver in §6.1 order, its generated vectors (§6.3).
3. A vector equal to an earlier one (same projected receiver, same arguments) is dropped.

### 6.5 Expected results

Each vector is evaluated by the evaluator: the result, or the code of the first error. The TS
expectation is computed in TS mode (§4). Both are written into the test files.

**Step cap.** These evaluations never spend the project budget (EVALUATION.md §2.3). Each vector has
its own budget of **1 000 000 steps**, counted as EVALUATION.md §12.1 says, and the call-depth limit
of EVALUATION.md §3.3; a value first forced during a vector's evaluation is charged to that vector.
A vector that exhausts either (unbounded recursion between translated functions) is not an expected
outcome: it is `E9009` at the function, and `canon check` and `canon build` fail, because the
translation would not terminate either. Each vector cut short is its own `E9009`; the function's
other vectors still run, and the function keeps no vectors.

### 6.6 Worked example: `Potion.healFor`

`export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }`, with
the three test calls on `{ heal: 500, … }`:

- receivers: one, `(heal: 500)`;
- candidates of `missingHp`: tests {200, 9000, −5} ∪ {0, 1, −1, INT64_MIN, INT64_MAX} ∪ no
  refinement ∪ {499, 500, 501} (from `heal`); sorted: INT64_MIN, −5, −1, 0, 1, 200, 499, 500, 501,
  9000, INT64_MAX;
- vectors: the three test vectors, then the generated ones without the duplicates.

| # | heal | missingHp | Go, C++ | TS |
|---|---|---|---|---|
| 1 | 500 | 200 | 200 | 200 |
| 2 | 500 | 9000 | 500 | 500 |
| 3 | 500 | −5 | 0 | 0 |
| 4 | 500 | INT64_MIN | 0 | `E8303` |
| 5 | 500 | −1 | 0 | 0 |
| 6 | 500 | 0 | 0 | 0 |
| 7 | 500 | 1 | 1 | 1 |
| 8 | 500 | 499 | 499 | 499 |
| 9 | 500 | 500 | 500 | 500 |
| 10 | 500 | 501 | 500 | 500 |
| 11 | 500 | INT64_MAX | 500 | `E8303` |

With a refined parameter, `export fn damageAt(self, level: Int(1..=150)) -> Int { return heal *
level / 100 }` and one test call `p.damageAt(10)`:

| heal | level | Go, C++ | TS |
|---|---|---|---|
| 500 | 10 | 50 | 50 |
| 500 | INT64_MIN | `E3204` | `E8303` |
| 500 | −1 | `E3204` | `E3204` |
| 500 | 0 | `E3204` | `E3204` |
| 500 | 1 | 5 | 5 |
| 500 | 2 | 10 | 10 |
| 500 | 149 | 745 | 745 |
| 500 | 150 | 750 | 750 |
| 500 | 151 | `E3204` | `E3204` |
| 500 | 499, 500, 501 | `E3204` | `E3204` |
| 500 | INT64_MAX | `E3204` | `E8303` |

---

## 7. Test files

### 7.1 Files and entry points

| Target | File (CODEGEN.md §2.3) | Entry point | Run by |
|---|---|---|---|
| Go | `<gopkg>_conformance_test.go`, in the generated package | `func Test<T><Fn>Conformance(t *testing.T)` per function (`TestPotionHealForConformance`); package-level: `Test<Fn>Conformance` | `go test` |
| C++ | `<last>_conformance.gen.cpp` | `int <namespace>::conformance::Run<P>Conformance()`, declared in `<last>.gen.h`; returns the number of failures and prints each to stderr | the runtime's test suite: `assert(Run<P>Conformance() == 0)` |
| TS | `<last>.conformance.test.ts`, next to the emitted `.ts` | one `test("<T>.<fn> conformance", …)` per function (`node:test`) | `node --test` on the compiled output (imports use `.js` specifiers, CODEGEN.md §2.8) |

They are ordinary generated files: `canon build --check` fails when one is stale.

### 7.2 Content

Each file contains, per function in declaration order, one table of vectors (receiver fields,
arguments, expected value, expected code; the code is empty when a value is expected) and a loop
that calls the **pure** function on each vector:

- a vector passes when the signalled code equals the expected code and, if no code is expected,
  the value is equal (floats bitwise, §5);
- the failure message is `<T>.<fn>(<name>=<value>, …) = <got> [<code>], canon says <want> [<code>]`;
  C++ prefixes it with `<package>: ` (`pipeline: Potion.healFor(…)`), because every package's
  `Run<P>Conformance` prints to the same stderr, while Go and TS report through their test
  runner, which names the package;
- C++ installs a capturing handler with `canon::SetEvalErrorHandler` for the duration of the run
  and restores the previous one; integer limits are written `canon::kIntMin` / `canon::kIntMax`;
- Go catches the panic with a generated `canonCatch` helper; limits are `math.MinInt64` /
  `math.MaxInt64`;
- TS catches `CanonEvalError` and uses the TS expectation.

The Go and C++ files of the pipeline example are goldens:
`examples/pipeline/expected/go/potions_conformance_test.go` and
`examples/pipeline/expected/pipeline_conformance.gen.cpp`. The TS template, for the same package
with the `damageAt` example added (verified: compiled by `tsc` 5.9 with `strict`, run by `node --test` on Node 24):

```ts
// GENERATED by canon from pipeline/. DO NOT EDIT.
// Conformance vectors of package pipeline, computed by the Canon evaluator.
import { test } from "node:test";
import * as assert from "node:assert/strict";
import { $potionDamageAt, $potionHealFor, CanonEvalError } from "./generated.js";

function canonCatch<T>(f: () => T): [T | undefined, string] {
  try {
    return [f(), ""];
  } catch (e) {
    if (e instanceof CanonEvalError) return [undefined, e.code];
    throw e;
  }
}

test("Potion.healFor conformance", () => {
  const vectors: ReadonlyArray<readonly [number, number, number, string]> = [
    [500, 200, 200, ""],
    [500, 9000, 500, ""],
    [500, -5, 0, ""],
    [500, -9223372036854776000, 0, "E8303"],
    [500, -1, 0, ""],
    [500, 0, 0, ""],
    [500, 1, 1, ""],
    [500, 499, 499, ""],
    [500, 500, 500, ""],
    [500, 501, 500, ""],
    [500, 9223372036854776000, 0, "E8303"],
  ];
  for (const [heal, missingHp, want, code] of vectors) {
    const [got, gotCode] = canonCatch(() => $potionHealFor(heal, missingHp));
    assert.equal(gotCode, code, `Potion.healFor(heal=${heal}, missingHp=${missingHp})`);
    if (code === "") assert.ok(Object.is(got, want), `Potion.healFor(heal=${heal}, missingHp=${missingHp}) = ${got}, canon says ${want}`);
  }
});

test("Potion.damageAt conformance", () => {
  const vectors: ReadonlyArray<readonly [number, number, number, string]> = [
    [500, 10, 50, ""],
    [500, -9223372036854776000, 0, "E8303"],
    [500, -1, 0, "E3204"],
    [500, 0, 0, "E3204"],
    [500, 1, 5, ""],
    [500, 2, 10, ""],
    [500, 149, 745, ""],
    [500, 150, 750, ""],
    [500, 151, 0, "E3204"],
    [500, 499, 0, "E3204"],
    [500, 500, 0, "E3204"],
    [500, 501, 0, "E3204"],
    [500, 9223372036854776000, 0, "E8303"],
  ];
  for (const [heal, level, want, code] of vectors) {
    const [got, gotCode] = canonCatch(() => $potionDamageAt(heal, level));
    assert.equal(gotCode, code, `Potion.damageAt(heal=${heal}, level=${level})`);
    if (code === "") assert.ok(Object.is(got, want), `Potion.damageAt(heal=${heal}, level=${level}) = ${got}, canon says ${want}`);
  }
});
```

---

## 8. Diagnostics

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E9001 | error | anything outside §2.2 |
| E9002 | error | finite-input export fn too large (CODEGEN.md §5.10) |
| E9003 | error | optional parameter in an export fn with parameters |
| E9004 | error | §2.1 |
| E9005 | error | Float or Duration in a template |
| E9006 | error | §2.1, `@ts(bigint)` included |
| E9007 | error | `where`, regex or length on a parameter or result |
| E9008 | error | §6.1 |
| E9009 | error | §6.5 step cap |

Codes signalled at runtime by translated code, and checked by the tests: `E3201`, `E3202`,
`E3204` (TYPES.md), `E4101`–`E4104` (EVALUATION.md), `E4108` (STDLIB.md) and `E8303`
(CODEGEN.md §12). The single catalogue is [ERRORS.md](ERRORS.md).
