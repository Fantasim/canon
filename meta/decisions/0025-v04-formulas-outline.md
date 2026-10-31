# ADR-0025 — v0.4, formulas: the outline

Date: 2026-10-31. **Status: outline, proposed.** Scope: Louis, 2026-10-31
([roadmap](../handoff/2026-10-31-canon-roadmap-v02-v04.md) §5; Q2: formulas before binary data).
The direction of each item is fixed here. The designs are written, decision-complete, when v0.4 starts.

## 0. How to use this outline

- The **directions bind**. Designs, names and syntax are written at v0.4's start, against the code
  of that day. Every file:line below is from `4dfd414e`; these references locate, they do not bind.
- **The rule for every widening:** it is bit-exact on every target by specification, or it is refused,
  with its way out named (DECISIONS 305). A widening that would need a platform library whose results
  differ per target is refused.
- v0.4 reuses v0.2's refinements in the IR (ADR-0023): the ranges let TS prove that an operand fits
  32 bits, and they define finite refined-Int domains.

## 1. Today's subset (CONFORMANCE §2.1-§2.3, §3; SPEC §9.4; CODEGEN §5.10)

- **Parameters:**
  - admitted: Bool, every integer type, Float, Float32, String, Duration, enums;
  - refused: optional (E9003); ref, record, list, map, variant, asset (E9006);
  - results: the same, plus `ref T` (otherwise E9004);
  - refinements: ranges and sized types only (E9007).
- **Control:** `let`, `if`/`else if`/`else`, `return`, `if` expressions, `and`/`or`/`not`, and `??`
  on an optional field of `self`.
- **Operators:** `+ - * / %`, unary `-`, comparisons, and `is` on a variant path of `self`.
- **Builtins:** `Float(i)`, `Int(f)`, `min`, `max`, `abs`, `clamp`, `floor`, `ceil`, `round`.
- **Templates:** String, integer and enum values, with no format spec (E9005; a spec is E9001).
- **Reads:** literals; enum members; `self` paths through non-collection fields; a table entry named
  as a `ref` result. A named `const` is E9001. Calls are allowed to same-package export fns: translated
  ones (self-recursion included), precomputed methods of `self`, and lookup fns.
- **Everything else is E9001:** loops, `var` and assignment, `match`, lambdas, comprehensions,
  collection literals, indexing, every collection, string and graph method, `String(x)`, `sqrt`, `pow`.
- **Lookup fns.**
  - When every parameter is finite (Bool, enums, a ref into a table), the fn becomes a full table:
    `ir/fns.go` ~127-136 decides, and `ir/precompute.go` ~206-240 has the evaluator fill it.
  - The cap is 65,536 cells (E9002). The bodies run in the full evaluator.
  - The tables are stored as `$<fn>` keys in data (WIRE §5.11) and fingerprints. C++ lookups are
    constexpr (DECISIONS 293).
- **TS integers:**
  - they are `number`, within ±(2^53−1) (E8303);
  - `/` and `%` go through `BigInt` per operation;
  - `@ts(bigint)` fields exist (DECISIONS 278, 279), but translated fns refuse them (E9006).
- **Conformance vectors:**
  - test-call inputs come first, then the candidates: 0, 1, −1, limits, refinement bounds ±1, `self`
    values ±1;
  - a full product when it holds at most 256 vectors, pairwise otherwise;
  - the expectations come from the evaluator;
  - each vector is capped at 1,000,000 steps and 10,000 frames (E9009);
  - the generated tests run in each consumer's own suite.
- **The evaluator already has** `var`, `for`, `while`, `break`, `continue`, `match` (statement and
  expression, exhaustive, no literal patterns in v0) and recursion. These are bounded by 10^8 steps
  per package and 10,000 frames. It has `sqrt` and `pow` (STDLIB §2.1-§2.2), but no exp, log or trig.
- **Spec gaps to settle in v0.4's spec sync, if v0.2 has not:** CONFORMANCE §3's Float `%` against
  TYPES §7.1, and E9006's message, which omits Float32.

## 2. Items: direction and open questions

1. **Control.**
   - `match` over enums and variants becomes a `switch` in each target.
   - Local `var` is admitted.
   - `for` is admitted over a range with constant or refined bounds, or over a list parameter, so
     every loop terminates by construction.
   - `while` stays out, because generated code has no step budget; its error names `for` as the way
     out.
   - **Open:**
     - translated recursion is allowed today: verify its run-time bound, then mirror the 10,000-frame
       limit in generated code or refuse recursion;
     - `break` and `continue`;
     - an exhaustive `match` in Go, which has no exhaustiveness check.
2. **Structured inputs and results.**
   - Records come in as `const&` in C++, by value in Go, as objects in TS.
   - Also admitted: optionals, list parameters, several records, and record results (built through
     the make hooks).
   - **Open:**
     - which list operations are admitted (indexing needs a bounds rule);
     - conformance vectors for records (taken from test calls);
     - whether maps are admitted.
3. **One rule for lookup versus translation.**
   - A fn is a lookup when its domain is finite (refined Int ranges count) and holds at most N cells.
   - Otherwise it is translated, and its body must be portable (E9001).
   - Same-package baked tables become readable by key in translated code.
   - **Open:**
     - the value of N, which replaces E9002's 65,536;
     - fns that move from lookup to translation change WIRE §5.11's `$` keys, the fingerprints and
       C++ constexpr: the reply must say so;
     - how translated code reaches table data in `data` mode.
4. **Math.**
   - `sqrt` is admitted: it is correctly rounded by IEEE 754 in every target.
   - `pow` with an `Int` exponent is admitted, by an algorithm STDLIB specifies: binary
     exponentiation with a fixed multiplication order, and the evaluator uses the same algorithm.
   - A real exponent stays build-time, through item 3.
   - **Open:** a negative exponent; whether build-time `pow` with an Int exponent changes algorithm
     (scan Resource for its uses first).
5. **Float32 arithmetic.**
   - When both operands are `Float32`, the operation is binary32, rounded per operation: C++ `float`,
     Go `float32`, TS `Math.fround` (exact for `+ - * /` and `sqrt`).
   - Mixing with `Float` gives `Float`. Literals adopt a Float32 context.
   - Conversions are explicit: `Float32(x)` rounds, and `Float(x)` is exact.
   - This amends TYPES §7.3 ("`Float32` is `Float` in expressions").
   - **Open:**
     - the static typing changes;
     - existing Float32 arithmetic will change results (scan Resource);
     - x87 and `FLT_EVAL_METHOD`, and item 9's contract.
6. **Bit-operation builtins.**
   - `bitAnd`, `bitOr`, `bitXor`, `bitNot`, `shiftLeft` and `shiftRight` operate on `Int`, two's
     complement over 64 bits.
   - They are builtins, not operators: `|` is already union and line continuation, and C's
     `&`-below-`==` precedence trap stays out of the language.
   - TS uses `BigInt`, or plain 32-bit operations where v0.2's ranges prove that the operands fit.
   - **Open:** shift counts outside 0..63 (an evaluation error, or static when constant); an
     arithmetic versus a logical right shift.
7. **Floats in translated templates.**
   - Floats use STDLIB §9.2's canonical form (ECMAScript `Number::toString`) and §9.5's `.N` (rounded
     from the exact binary value, ties away from zero, N ≤ 20).
   - Go and C++ runtime helpers build the shortest round-trip digits and lay them out as ECMAScript
     does. TS uses `String(x)`, and `toFixed` below 10^21.
   - **Open:** Float32's shortest digits (STDLIB §9.1); Duration.
8. **`bigint` in translated TS.**
   - `@ts(bigint)` values are admitted as parameters and results, with BigInt arithmetic for them.
   - **Open:** mixing `number` and `bigint`; conformance vectors.
9. **The floating-point contract.**
   - CONFORMANCE §5 states the consumer's flags:
     - no `-ffast-math` or `-Ofast`;
     - `-ffp-contract=off` for C++ (GCC's GNU mode fuses to FMA on FMA targets);
     - Go may fuse unless an explicit `float64()` conversion intervenes: check how gen/go writes float
       expressions;
     - JS never fuses.
   - The conformance tests run at the consumer's flags. Verify today's `Float` translation against
     this contract **first**: it may be an existing gap.
10. **A guide recipe: porting a legacy formula.**
    - Captured vectors are written as `test` blocks with `expect f(x) == v`; their inputs already join
      conformance.
    - A scale probe runs thousands of vectors (the budget, and the vector caps).

## 3. Dependencies

- From v0.2: the refinements in the IR (`Field.Range` for every field).
- From v0.3: nothing required.
- Sovereign's L-0148 instance formulas (upgrade, rarity, Azure, level-down) are the acceptance
  vehicle, as a synthetic copy in `examples/`.

## 4. Not in v0.4

- Real-exponent `pow`, exp, log and trig in translated code: no portable definition exists.
- `while`.
- Binary data (v0.5) and runtime values in schema packages (v0.6).

## 5. Acceptance, in outline

- Every new construct's conformance vectors pass in Go, in C++ (g++ and clang++ at `-O2` with the
  stated flags) and in TS (node).
- The tooltip formulas are expressible and translated, in an `examples/` package shaped like
  L-0148's.
- `make realdata-compile` PASS.
