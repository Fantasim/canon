# Canon evaluation

Normative companion to [SPEC.md](../SPEC.md) §8, §10, §11, §18 and §19. Version: **0.1 (draft)**.

This document says what the evaluator computes, in which order, what it costs, and which
findings it produces. Two implementations that follow it produce the same values, the same
findings and the same budget verdict. It applies the accepted answers of AUDIT EVL-*, CHK-* and
LAY-* (the non-codegen part of LAY-04) and DECISIONS 17.

Related documents:

| Document | Relied on for |
|---|---|
| [TYPES.md](TYPES.md) | static rules, storage points (§6.2), refinements (§7.4), refs (§10), dependent types (§11) |
| [STDLIB.md](STDLIB.md) | built-in functions and their step costs and errors |
| [WIRE.md](WIRE.md) | decoding loaded files (`E7xxx`, `E3203`, `E3315`–`E3318`) and the JSON pointer format |
| [LOCK.md](LOCK.md) | stable ids (`E6xxx`), layers and the lock |
| [CODEGEN.md](CODEGEN.md), [CONFORMANCE.md](CONFORMANCE.md) | which export fn results are emitted, the domain order of lookup tables, conformance vectors, runtime reading of inputs |
| [API.md](API.md) | value paths (API-02), the finding JSON shape, `Evaluate` for views |

---

## 1. Phases of a build

This replaces SPEC §11.2.

| # | Phase | Output |
|---|---|---|
| 1 | Parse every file of the selected packages and of their imports (GRAMMAR.md) | syntax trees |
| 2 | Resolve and type-check, in one bidirectional pass (TYPES.md §1). Every layer file of the loaded packages is checked, active or not (§9.1). Then the static view and translation checks (VIEWMODEL.md §16, I18N.md §12) | static findings, broken declarations |
| 3 | **Stage A, evaluate**: force the values of §2.1, applying active layers to each one right after its base evaluation (§9.3) | values, evaluation findings |
| 4 | **Stage B, verify** every evaluated value (§5), then the view checks that need evaluated values (`E1610` for a `unit` against studio's `units`, VIEWMODEL.md §12.9) | verification findings |
| 5 | **Stage C, instance checks** (§8.1) | check findings |
| 6 | **Stage D, package checks** (§8.5) | check findings |
| 7 | **Stage E, precompute** export fn results (§2.3), then **validate every emit** of the selected packages: build its IR and check every emit rule (CODEGEN.md §12, WIRE.md §8), writing nothing; then compute the conformance vectors of every translated function (CONFORMANCE.md §6) | values for codegen, conformance vectors, findings |
| 8 | Emit (not in this document): code, data and `canon.lock` only if there is no error finding; `emit view` even with errors (VIEWMODEL.md J4, API.md B1) | files |

- `canon check` runs phases 1–7. `canon build` runs 1–8. So `check` and `build` report the same
  findings (CLI.md §3.3), except `E8001` and `E8152`, which need the output files and are reported
  in phase 8 only. `canon test` runs 1–2, then the
  tests (§10).
- `canon check` and `canon build` both compute the conformance vectors in phase 7, so both report
  `E9008` and `E9009`. Computing them runs the tests of a package with a translated function, only
  to collect the calls the vectors need (CONFORMANCE.md §6.1). Each package's tests run as
  `canon test <pkg>` runs them (§10): values forced afresh, never reusing stage A's, one budget of
  `project.budget` steps shared by that package's tests only, in declaration order, not the
  project's budget. So the calls collected, hence the vectors, are what `canon test` sees, unless
  `canon test`'s counter, which also paid phase 2's constant folding (§12.2), runs out first; none
  of the tests' findings is reported, and the calls made before any stop are kept. No other part of
  `check` or `build` runs tests.
- A **broken** declaration (TYPES.md §1) is never evaluated. A value, check or test that is
  broken produces no evaluation finding. Other values are evaluated normally.
- A value forced for the first time after stage B has started (for example by a package check)
  is verified immediately after its evaluation.
- **Names.** These phase numbers and stage names are the reference: every other document says
  "phase 2", "stage B (verify)", "stage E (precompute)" and so on, never another numbering. SPEC §11
  points here.

---

## 2. What is evaluated

### 2.1 The forced set (EVL-01)

`canon check` and `canon build` force, in this order:

1. for each **selected** package, in package order (TYPES.md §3.1): every top-level `const`
   and every top-level `let`, public and `local`, in the order (file path bytes, source
   position);
2. whatever these values read, lazily, including values of imported packages;
3. every constant a phase-2 fold read, directly or through other constants, that 1 and 2 did not
   force, in fold order, charged again on the one counter (§12.2), so stage B verifies it whatever
   the selection (DECISIONS 264, 271).

Values of imported packages that nothing reads are not evaluated; phase 8 evaluates, on demand,
the values a view model reads that phases 3–7 did not (their counts, VIEWMODEL.md C3/J12), and
reports no finding from them; it spends no budget, its stage-B verification of them included
(VIEWMODEL.md J5), so it runs after `E4401` too (§12.2 governs
budgeted runs), and an internal error or an unsupported load met there still fails the build
(DECISIONS 195, 196) — except an unsupported load met while reading a value only to fill a type
function's `drivers` (VIEWMODEL.md §12.3), which contributes no entries, whatever the selection. `canon test` forces only what
the tests read (§10).

### 2.2 Order of evaluation inside an expression

Evaluation is eager and strictly left to right as written:

- operands left to right; `and`, `or`, `??`, `if` and `match` evaluate only what their result
  needs; `?.` skips the rest of its chain on `none` (TYPES.md §6.5);
- call arguments in the order written, then the function body;
- a record or case literal evaluates its spread, then its written fields in the order written,
  then the defaults of the remaining fields in declaration order; one exception: before a
  field whose type reads earlier fields (TYPES.md §11.1) is evaluated, each field it reads that
  has no value yet gets one — a written one by its item (this exception applying to it in
  turn), a defaulted one by its default once every field declared before it has its value —
  then this order resumes (evaluation is pure, so only step charging and which of two errors
  surfaces first can tell);
- list, map and table literals evaluate their items in order;
- a comprehension runs its clauses as nested loops, left to right;
- `for` iterates over a snapshot of its collection taken when the loop starts (values are
  immutable, §4.1): lists in order, tables in entry order (SPEC §4.3), maps in insertion order,
  ranges in ascending order.

### 2.3 Precomputed export fns (stage E)

For each selected package that has at least one `emit` other than `view`, every `export fn`
without runtime inputs is evaluated as CODEGEN.md requires:

- a method without parameters: once per receiver instance, for every instance of its record or
  case reachable from the package's public values (§8.1 traversal order);
- a function or method with only finite parameters: once per cell of its domain, in the domain
  order of CODEGEN.md §5.10 (for each receiver, as above);
- a function without parameters: once.

A method of an imported type is also precomputed on the receivers this package holds, because
their encoded `$` keys need it (WIRE.md §5.11). Every receiver and every cell is evaluated, and
each failure is reported: nothing stops at the first. An evaluation error there is a finding at
the failing expression. Its outermost stack frame names
the fn and its inputs: `while computing canTransition(open, taken)`. `canon check` runs stage E
too, so `check` and `build` report the same findings.

Translated functions (runtime inputs) are evaluated only to compute conformance vectors
(CONFORMANCE.md). Those evaluations never consume the project budget, and an evaluation error
there is an expected outcome of the vector, not a finding. Each vector has its own step cap;
exceeding it is `E9009` (CONFORMANCE.md §6.5).

---

## 3. Forcing, cycles and call depth

### 3.1 Lazy forcing

A top-level value is evaluated the first time it is read, then cached for the rest of the
invocation. Reading a value **forces** it: its initializer is evaluated, then the active layers
are applied (§9.3). A ref is a key, so holding a ref does not force its target collection;
**dereferencing** it does.

### 3.2 Cycles (EVL-06)

Forcing a value that is already being forced is `E4301`. The finding is at the expression that
closed the cycle and lists the cycle with a location per step:

```
error[E4301]  resource/x/x.canon:12:20
  cycle between values: statuses -> firstStatus -> statuses
```

Every value on the cycle is poisoned (§7.2). Cycles are detected at value granularity: a table
entry that reads another entry of the same table while the table is being built is a cycle.
Refs between entries are not (they are keys). Cycles between constants are detected the same
way, at type-checking time.

### 3.3 Call depth

A user function, method or lambda call that would make the call stack deeper than **10,000**
frames is `E4402` at the call. The frames counted are every live user frame of the invocation,
across nested roots (a value forced inside a call, a check run, a `where` re-run, a
precomputation), so chained forcing is bounded too; the finding's `(n more frames)` line
(API.md F13) counts the same frames. The limit keeps deep recursion deterministic and away from
the host stack: every walk over a value (equality, text form, set hashing, binding refs) and
every graph built-in runs iteratively or on an explicit stack, charged per node it visits.

### 3.4 Refs resolved per instance

A `ref T` resolved at level 1 (TYPES.md §10.2) is **bound** when the enclosing record instance is
built: when an instance of a record `R` that has the candidate collection field `F` is
completed, every level-1 ref of that target contained in its field values (including `F`
itself) and not yet bound is bound to that instance's `F`. Dereferencing an unbound level-1 ref
is `E3505`. Verifying one (§5) that is still unbound when its top-level value is complete is
`E3505` too.

---

## 4. Values

### 4.1 Value semantics (EVL-04)

- Every value is immutable. Evaluation order never changes a result.
- `var` bindings are **rebound**, never mutated in place: `xs += [x]` means `xs = xs + [x]`.
  After `var xs = [1]; let ys = xs; xs[0] = 2`, `ys` is still `[1]`.
- Element assignment is allowed through a `var` root at any depth (`m[k][0] = v`). The
  evaluator copies what it changes (copy-on-write); the effect is that of rebuilding the root.
  `xs[i] = v` requires an existing index (negative counts from the end), else `E4002`.
  `m[k] = v` replaces an existing key in place or appends a new key at the end.
- Lambdas capture the values of their free names when they are created.

### 4.2 Identity

Elements of tables and keyed lists carry an identity (collection instance and key) as defined
in TYPES.md §6.3. The evaluator keeps it through bindings, calls, returns and every stdlib
function that returns elements (`filter`, `active`, `first`…). A spread or a literal creates a
value without identity.

Every record or case value is also an **instance**: each evaluation of a literal, each default,
each object decoded from a file creates a new instance. Copies share the instance. Checks run
once per instance (§8.1).

### 4.3 Conversions at storage points

At each storage point (TYPES.md §6.2) the value is converted to the declared type. The
conversion walks the value against the type and, wherever the type carries them:

1. applies the conversions of TYPES.md §6.2 (dereference, entry to ref, wrapping in `?`,
   `Float32` rounding);
2. checks implicit ranges (`E3201`), finiteness (`E3202`), range and length refinements
   (`E3204`), patterns (`E3205`), `where` predicates (`E3206`), asset refinements (§5), entry to
   ref membership (`E3503`), retired members (`E3506`) and keyed-list uniqueness (`E3102`).

These findings are **soft** (§7.1): they are reported at the value's own location (§13), with a
related location at the declared type, and the offending value is marked **invalid**. The
conversion costs no step, except the nodes of `where` predicates (§12).

A value converted several times to the same refined type is checked each time. Identical
findings are reported once (§14).

### 4.4 Equality

Value equality (`==`, `contains`, `unique`, map keys, graph nodes) is defined once, in
TYPES.md §7.5. The evaluator implements exactly that table.

### 4.5 Freezing

With value semantics no program can observe or cause mutation of a shared value. `E4201` is kept
as an internal safety net: an implementation that detects a write to a value that has left its
call reports `E4201` and treats it as a compiler bug (exit code 3, CLI §2.5).

---

## 5. Verification (stage B)

Each evaluated top-level value that is not poisoned (constants only a fold read included, §2.1;
DECISIONS 264) is verified once, after its evaluation and layers, in the order its
evaluation completed. Verification walks the value (not through refs) and checks:

| Check | Code | Located at |
|---|---|---|
| every ref key exists in its target collection (forcing the target) | `E3501` | the ref value |
| a live table entry does not reference a retired entry | `E3502` | the ref value |
| level-1 refs are bound | `E3505` | the ref value |
| table keys and keyed-list keys are unique | `E3101`, `E3102` | the second occurrence; the message names the first |
| dependent types: each dependent value fits the type computed from its arguments; symbolic identifiers resolve | `E3802`, `E3801`, `E3501` | the value |
| assets exist, with an allowed extension and a clean path | `E3701`–`E3703` | the value |
| stable ids, `@codes`, `@stable` values | `E6xxx`, `E3102` | per LOCK.md |

Verification findings are soft. The offending sub-value is marked invalid. Type functions
evaluated here cost steps (§12); an evaluation error inside one is a hard error charged to the
value being verified.

`expect` subjects are verified the same way (§10.2).

---

## 6. Numbers and durations

### 6.1 Integers

- `+`, `-`, `*` and unary `-` that leave the 64-bit signed range are `E4101` (never a wrap).
- `/` truncates toward zero; `%` has the sign of the left operand (`-7 % 2 == -1`). A zero
  divisor is `E4102`. `MIN / -1` is `E4101`; `MIN % -1` is `0`.
- `a..=b` with `b` the largest `Int` is `E4101`.

### 6.2 Floats (EVL-08)

- Operations follow IEEE 754 binary64 with round-to-nearest-even. `-0.0 == 0.0`.
- Any operation whose result is NaN or ±infinity (`x / 0.0`, `0.0 / 0.0`, overflow of `*`,
  `sqrt(-1.0)`, `pow` overflow) is `E4104` at that expression.
- `Float(i)` is exact when `|i| ≤ 2^53`, else rounded to nearest-even.
- `Int(f)` truncates toward zero; a result outside the `Int` range is `E4103`. `floor`, `ceil`
  and `round` likewise (STDLIB.md).
- Storing into a `Float32` rounds to nearest-even binary32; a result that overflows binary32 is
  `E3202`.

### 6.3 Durations (EVL-10)

- A `Duration` is a signed 64-bit count of milliseconds in expressions. Negative durations are
  allowed; storage refinements decide whether they are valid in data. A stored `Duration` is
  limited to ±9,223,372,036,854 ms (TYPES.md §7.2): beyond it, the storage point reports `E3201`,
  as for a sized integer.
- `Duration ± Duration` and `Duration * Int` / `Int * Duration` (they commute): overflow is
  `E4101`.
- `Duration / Int` truncates toward zero at the millisecond (`7ms / 2 == 3ms`); a zero divisor
  is `E4102`.
- `Duration / Duration` is `Float(a) / Float(b)` of the millisecond counts; a zero divisor is
  `E4102`.
- A duration literal that exceeds the range is a static `E3201`.

---

## 7. Errors, poisoning and invalid values

### 7.1 Hard and soft findings

| Kind | Codes | Effect |
|---|---|---|
| **hard** | every `E4xxx` raised at evaluation (including those of STDLIB.md), `E3501` and `E3505` raised by a dereference, load and decode errors (`E7xxx` and the decode codes of WIRE.md), `E1905` | aborts the current root (below) |
| **soft** | `E3101`, `E3102`, `E3201`, `E3202`, `E3204`–`E3206`, `E3322`, `E3501`–`E3503`, `E3505`, `E3506` found by conversion or verification, `E37xx`, `E38xx`, `E6xxx` | marks the offending value invalid; evaluation continues |
| check | `E5xxx`, `W5xxx` | none on evaluation |

A **root** is the unit of evaluation work: a top-level value, one run of one check on one
instance, one package-level check, one test, one precomputation. A hard error found while
evaluating a root is a finding at the failing expression, with the Canon call stack (§13), and
aborts that root:

| Root | After a hard error |
|---|---|
| top-level value | poisoned |
| check run | aborted; the findings it already produced with `fail`/`warn` are discarded |
| test | fails (§10.4) |
| precomputation | that cell or receiver is missing; the error blocks emission |

For a load, the decoder reports every decode error of the files it reads before the value is
poisoned (WIRE.md), so one bad item file does not hide the others.

### 7.2 Poisoning (EVL-05)

- A poisoned value is never verified, amended further or traversed for checks.
- Reading a poisoned value aborts the reader silently: the reader is poisoned (a value) or
  skipped (a check, a precomputation) **without a new finding**. A test that reads one fails,
  naming it.
- A value on a cycle, a value whose initializer raised a hard error, and a value whose
  amendment failed (§9.3) are poisoned.

### 7.3 Invalid values and taint

A value marked invalid by a soft finding stays readable. To keep checks from reporting
consequences of an error already reported:

- An instance check (§8.1) is **not run** on an instance whose subtree contains an invalid
  value. The subtree follows fields, elements, keys and values, never refs. Each value is judged
  the first time stage C's traversal reaches it, and that answer is kept: a mark a later check run
  sets reaches only values not yet asked about (DECISIONS 266).
- Every other root is **tainted** as soon as it reads an invalid value (a name, field, element,
  entry or dereference whose result is marked invalid). A hard error raised in a tainted root is
  **not reported**: the root is aborted silently, as if poisoned.
- A tainted root that completes normally reports its findings normally.

Example: an `Area` whose `group` is an unknown key gets `E3501`. Its own checks are skipped. The
package check that walks `areas.active()` and dereferences `a.group` is tainted, and aborts
silently at the dereference instead of reporting a second error.

---

## 8. Checks

### 8.1 Instance checks (EVL-02)

Record and case checks (`check`/`warn` inside a record or case body) run once per **instance**
(§4.2) reachable from:

- the evaluated, non-poisoned top-level values of the **selected** packages, after
  verification;
- each `expect` subject (§10.2), with findings captured by the test.

Reachability follows record fields, case fields, list and keyed-list elements, map keys and
values, and table entries. It never follows refs (the target is reached from its own
collection). Instances created and dropped inside functions (temporaries) are not checked; their
refinements are (§4.3).

Traversal order (stage C): top-level values in forced-set order (§2.1), each walked depth-first
pre-order: fields in declaration order, elements in order, map entries in insertion order (key,
then value). An instance reached a second time is skipped. On each instance, its checks run in
declaration order: for a case value, the variant-level checks (TYPES.md §12.1), then the case's
checks.

Inside an instance check, the fields are in scope and `self` is the instance (an entry when it is
one).

### 8.2 Package checks

Package-level `check`/`warn` (§8.5) run once, in stage D, for each selected package, in package
order, then file path order, then source order.

### 8.3 Codes and locations (CHK-01, CHK-02)

| Form | Finding | Located at |
|---|---|---|
| `check C else "m"` in a record or case, `C` false | `E5001` error | the instance: see below |
| `warn C else "m"` in a record or case | `W5001` warning | the instance |
| `check C at f else "m"`, `warn C at f else "m"` in a record or case | `E5001` / `W5001` | the value of the field `f` of the instance: the location of that value (§13), and a `path` ending in `.f` (VIEWMODEL.md G19) |
| `check C else "m"` at package level | `E5001` | the `check` keyword of the declaration |
| `warn C else "m"` at package level | `W5001` | the `warn` keyword |
| `fail(at, "m")` in a `check { }` block | `E5002` error | the provenance of `at` (§13) |
| `warn(at, "m")` in a `check { }` block | `W5002` warning | the provenance of `at` |

An instance is located at: the first token of its literal (the type name of `Name { … }`, the
case name of `c { … }`, else `{`); for a table entry, its key token; for a JSON object, its `{`;
for a default-filled instance, the default expression (its `via`, the literal or object that
omitted the field, is shown by `canon explain`). Every check finding has one related location,
at the check declaration, with the note `check <name>`, or `check` for an unnamed check (API.md
F4, F12).

A **named** check (`check name: …`) puts `name` in the finding's `check` field. Block checks
inherit nothing: `fail`/`warn` findings of a named block check carry its name too.

Check names are unique within a record or case (a variant-level check shares every case's names),
and within a package for package checks (`E5003`); a clash is reported at the later of the two in source, once
per case it clashes in.

`fail` and `warn` are allowed only lexically inside `check { }` blocks (`E1105`, GRAMMAR.md).
`at` may be any value, including `none` (located where that `none` was written or defaulted).

### 8.4 Conditions and messages

- A one-line check's condition is evaluated; if it is `false`, the message template is
  evaluated (with the flow facts of TYPES.md §6.6) and the finding is produced.
- A hard error in the condition aborts the check run (§7.1): the error is reported, no `E5001`.
- A hard error in the message template: the finding is still produced, with the template's
  source text uninterpolated as its message, and the error is reported too.
- With `--lang`, the message comes from the translation key of I18N.md (`Type.check.name`,
  `check.name`), evaluated in the same scope. Tests (§10.3) match the **source-language**
  message.

### 8.5 Scope of package checks

A package check sees the package's declarations and imports (TYPES.md §3.3). It runs even if
some values it could read are poisoned or invalid; reading one aborts it silently (§7.2) or
taints it (§7.3).

---

## 9. Layers (LAY-01 to LAY-03)

### 9.1 Finding and checking layers

- A layer file is a file whose first tokens, after comments, are `package p`, a line break and
  `layer name` (GRAMMAR.md §5.2 decides the kind of a file). `--layer x` activates every `layer
  x` file of the loaded packages. A name that no loaded package declares is `E1901` (exit code
  2). Two files of one package declaring the same layer name is `E1906`.
- Layers stack in the order given on the command line (or `Options.Layers`).
- **Every** layer file of the loaded packages is resolved and type-checked in phase 2, whether
  active or not, so renaming a field breaks a stale layer at once.
- A layer never adds or changes types; it only has `amend` blocks.

### 9.2 Amend blocks and paths

```
amend config {
  paths.resourceRoot: "/home/louis/Desktop/Sovereign/Resource"
  server.port: 9000
}
```

- `amend v` names a top-level `let` of the layer's own package, public or `local`. A name of
  another package is `E1909`; a `const`, function or type is `E1902`; an unknown name `E2102`.
- A path is `amendPath` of GRAMMAR.md §5.7, `WORD { "." WORD | "[" expr "]" | "[" "#" INT "]" }`
  (`[` and `#` are two tokens), relative to
  `v`. Segments are resolved against the static type:

| Segment | On | Means |
|---|---|---|
| `.f` | record | field `f` |
| `.f` | variant | field `f` of the current case (checked when applied) |
| `.k` or `[k]` | table, keyed list with a `String` key | entry with key `k` |
| `[k]` | keyed list | entry with key `k` (API-02) |
| `[k]` | map | entry with key `k` (identifiers resolved against the key type) |
| `[i]` | list | element `i` (negative from the end) |
| `[#n]` | list, keyed list, table, map | the element or entry at position `n` (0-based, insertion or entry order), as in API paths (API.md §6.2) |

  A segment that fits none of these statically is `E1905`. A path through an input field is
  `E3313`.
- The value after `:` is checked against the static type at the path (TYPES.md §5.1).
- Within one layer (all its files), the same path twice, or a path that is a prefix of another,
  is `E1908`.

### 9.3 Application (LAY-02)

For a value `v`, its amendments are those of every active layer, in layer order, then in source
order within the layer's file. They are applied **after `v`'s base evaluation and before anyone
else reads `v`**:

1. Evaluate the base value. If it is poisoned, stop.
2. For each amendment in order: evaluate the right-hand side in the package's scope (reading `v`
   itself is `E4301`), convert it at the storage point of the path (§4.3), and replace the
   sub-value at the path.
   - Every intermediate segment must exist at application time: a missing key, an out-of-range
     index, `none` on the way, or a variant whose current case lacks the field is `E1905` (hard: `v`
     is poisoned). The right-hand side is still evaluated, in no field scope (WIRE.md §6.1,
     DECISIONS 272).
   - The **last** segment may name a new map key or a new table key: the entry is added at the
     end (a new table entry gets its identity). Adding to a `stable table` is `E6004`
     (LOCK.md). A keyed list or list never grows.
3. **Derived defaults follow**: when an amendment replaces field `f` of an instance, every
   later field of that instance whose value came from its default (not written in the source or
   file, not amended) is re-evaluated, in declaration order. An applied-record field written in
   the source keeps the arguments it was built with (TYPES.md §11.1): amending the argument it
   depends on leaves it applied to the old one, which verification reports as `E3802`; a
   defaulted one is re-derived and takes the new argument.
4. The amended value then goes through verification and every check, like any value.

Amended sub-values carry layer provenance (§13), which `canon explain` shows. Layers never write
`canon.lock` (LOCK.md).

### 9.4 Outputs (LAY-03)

A build with `--layer` writes the same output paths as a build without. Alternating the two
rewrites the outputs each time. This is by design; the CI build runs without layers.

---

## 10. Tests

### 10.1 Running tests

- `canon test` runs the test blocks of the selected packages whose name matches `--run` (RE2
  search, default: all), in package order, then file path order, then source order.
- Test names are unique within a package (`E5005`).
- Tests are type-checked in every `canon check`, but run only by `canon test`, and silently when
  conformance vectors are computed (§1).
- Top-level values are forced on demand, verified (§5), and shared by all tests of the
  invocation. Instance checks and package checks do not run on them in `canon test`.
- The body is a block. Its statements run in order; `expect` is allowed only in test blocks
  (`E1130`, GRAMMAR.md). The budget is shared by all tests of the invocation: they start on the
  counter phase 2's constant folding spent (§12.2).

### 10.2 Building a subject

`expect v passes`, `fails …` and `warns …` **build** `v`:

1. evaluate `v`;
2. verify it as a value (§5), forcing target collections as needed;
3. run the instance checks of every instance reachable from `v` (§8.1), including instances
   that also belong to top-level values.

Every finding produced by these three steps is **captured** by the `expect`, not reported. So
are soft findings produced while evaluating `v` (refinements at storage points inside
functions). Package checks never run for a subject. Static errors are compile errors, never
"expected" (CHK-05).

A subject that reads a poisoned top-level value makes the `expect` fail, naming that value.

### 10.3 `expect` forms

| Form | Passes when |
|---|---|
| `expect c` | `c ⇐ Bool` evaluates to `true` and no error finding is produced while evaluating it. If `c` is a comparison, a failure reports both operand values |
| `expect v passes` | building `v` captures no **error** finding (warnings are allowed) |
| `expect v fails "text"` | at least one captured error finding's message contains `text` (byte substring, case-sensitive, source-language message) |
| `expect v fails name` | at least one captured error finding has `check` = `name` |
| `expect v fails E3204` | at least one captured error finding has that code |
| `expect v warns …` | the same, over captured **warnings** |

- An identifier after `fails`/`warns` that matches `^[EW][0-9]{4}$` is a code; any other
  identifier is a check name and must be the name of some named check declared on a type
  reachable from `v`'s static type, or in the package (`E5004`).
- Matching counts dynamic `E3xxx`, `E4xxx` and `E5xxx`/`W5xxx` findings (CHK-05).

### 10.4 Failures

- A failing `expect` does not stop the test; later statements still run.
- A hard error outside an `expect` subject (in a `let`, a loop, a call) fails the test and
  stops it.
- A test passes when every `expect` passed and no hard error occurred.
- The output format is CLI §3.5: for `fails`/`warns`, the captured findings are listed.

---

## 11. Runtime inputs (LAY-04, TYP-17)

How generated loaders read inputs is CODEGEN.md. This section defines what an input is.

### 11.1 Declaration

```
apiKey: input String(1..)? from env "RESOURCESTUDIO_GEMINI_KEY"
```

- The type of an input field is `Bool`, an integer type, `Float`, `Float32`, `String`,
  `Duration` or an enum, possibly through an alias, possibly optional, with only these
  refinements: ranges, lengths, and patterns in the portable subset of §11.3 (`E1910`
  otherwise; a `where` is `E1910` because it cannot run at runtime).
- `from env "NAME"` is required on an input and forbidden on any other field (`E1131`,
  GRAMMAR.md). `NAME` matches `^[A-Za-z_][A-Za-z0-9_]*$` (`E1911`).
- An input field has no default (`E1907`). `input T?` means "`none` when unset".
- **Placement** (`E1903`): a record with an input field must be reachable from exactly one
  public top-level `let` of its package, through record fields only (optionals allowed), and
  must not appear anywhere in the package as, or inside, a list, keyed list, map, table,
  variant case or dependent type. One environment variable then maps to one field of one value. A
  variant case may not declare an input field itself either (`E1903`).

### 11.2 Build-time semantics

- Inputs have no value at build time. They are absent from Canon values, literals (`E3312`),
  loaded data (`E3312`), emitted JSON, the text form and `canon explain` (which prints `input
  from env NAME`).
- No expression may read an input (`E3313`): checks, functions, views and other values
  included.

### 11.3 Runtime semantics

When the generated loader reads inputs (CODEGEN.md defines where), for each input field:

1. Read the environment variable. Unset, or set to the empty string, means **absent**:
   `input T?` becomes `none`; a required input fails the load with a message naming the
   variable.
2. Parse the text as the Canon literal of the type:

| Type | Accepted text |
|---|---|
| `Bool` | `true` or `false` |
| integer types | `-?[0-9]+`, decimal only, no separators |
| `Float`, `Float32` | `-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?`; the result must be finite |
| `Duration` | a Canon duration literal (`90s`, `1h30m`), optionally prefixed by `-` |
| `String` | the value byte for byte (no trimming, quotes or escapes); it must be valid UTF-8 |
| enum | a member's wire value, exactly; retired members are refused |

3. Check the field's **own** refinements: the implicit range of a sized type, ranges, lengths
   in bytes, patterns (search semantics). Nothing else runs at runtime: no `where`, no check.

**Portable pattern subset** (`E1904` for anything else), accepted identically by RE2 and
ECMAScript: literal characters and escaped metacharacters, `.`, bracket classes with ranges and
negation, `\d \D \w \W \s \S`, anchors `^` and `$`, groups `( )` and `(?: )`, alternation `|`,
quantifiers `* + ? {n} {n,} {n,m}` and their lazy forms. Not allowed: inline flags (`(?i)`),
named groups, `\p{…}`, `\A`, `\z`, `\b`, `\B`, POSIX classes (`[[:alpha:]]`), backreferences and
lookaround.

---

## 12. Step budget (EVL-03)

### 12.1 What costs a step

The following cost exactly one step each time they are evaluated or executed:

| Unit | Details |
|---|---|
| expression node | literal, name, `.f`, `?.f`, `[i]`, call, unary operator, binary operator (including `and`/`or`/`??`), `!`, `is`, `in`, range, `if` expression, `match` expression, lambda creation, list literal, brace literal (record, case, map, table), typed literal, string template (its parts are separate nodes), comprehension. Parentheses and spreads are not nodes |
| statement | every executed statement, including each `fail`, `warn` and `expect` |
| iteration | each entry into the body of `for` or `while`, and each iteration of a comprehension `for` clause |
| invocation | each entry into the body of a user function, method or lambda (in addition to the call node) |
| built-in | the cost listed in STDLIB.md, in addition to the call node and to the lambdas it invokes |
| type function | each evaluation of a type application in verification (§5), plus its argument and scrutinee paths as nodes: once per record for a type written at a field (an application inside its container type included), whatever its container holds, once per element for one a computed type nests, so a nested empty container costs nothing (DECISIONS 265) |

A shorthand lambda (`.f`, `.m(args)`, `.compared()[k]`) is one lambda-creation node; each
invocation costs one step plus its body's nodes: one for the implicit parameter and one per
postfix step and argument node (`.f` costs 1 + 2). A `where` predicate costs its nodes each time it runs. Default
expressions cost their nodes when evaluated.

**Work proportional to a value** is charged as it is done, so every step buys a bounded amount of
time and memory and `E4401` is the only way evaluation runs long:

- **Equality.** Every equality the evaluator runs (`==`, `!=`, `in`, `contains`, `indexOf`,
  `unique`, map and set lookups) costs one step per pair of values compared, scalar pairs
  included, in addition to the listed cost of the node or built-in: `x in xs` over n elements
  costs n. A map finds a key through a hash index, never a scan, so `m[k]` is charged only for the
  pairs its genuine hash matches compare.
- **Producing a string or a list.** `+` on lists and strings costs one step per element or byte
  of its result; every built-in that produces a string (interpolation, `join`, `replace`,
  `lower`, `upper`, `trim`, `String`, format specs) costs one step per byte of its result,
  charged before building it, and `split` one step per part and per byte of its parts, in
  addition to its listed cost (STDLIB.md §1.3).

**Free:** reading and decoding files, conversions and refinement checks other than `where`,
verification other than type functions, the traversal of instance checks, layer path
resolution, the `@stable` comparisons made while applying an amendment, and formatting findings.
Stage B's re-run of a `where` predicate that the conversion already ran (and paid) is free; such a
re-run is capped at the budget, and past it that re-run alone stops, its value is poisoned, no
`E4401` is reported and evaluation continues.

### 12.2 Budget

- The budget is `project.budget` (default 10⁸) for one `canon check`, `canon build` or `canon
  test` invocation, or for one API re-check. Every budgeted stage (evaluation, verification,
  checks, precomputation, tests) spends from the same counter. Cached results (CLI §2.7) are reused
  only for an identical build manifest, so they reproduce the same verdict.
- One counter per invocation (DECISIONS 104): phase 2's constant folding spends from it first,
  then stages A–E; `canon test`'s tests start on the counter phase 2 left. A constant folded in
  phase 2 and forced again in stage A is evaluated, and charged, twice (TYPES.md §15). An API
  re-check that reuses earlier folds re-spends their steps, so `E4401` lands where a cold run puts
  it. Not on this counter: the tests run to compute conformance vectors (one budget per package,
  §1); view-model rendering and phase 8's verification, which spend nothing (§2.1); and the
  re-run that explains a poisoned value (API.md R6), which starts from a fresh counter, replays
  phase 2's folds and discards its findings, so its causes are those of a cold run.
- Steps are **charged** to the root being evaluated (§7.1). Forcing a top-level value from inside
  another root charges the forced value, not the forcer. All runs of one instance-check
  declaration are charged to that declaration.
- Spending the last step is `E4401` at the expression being evaluated, with its stack and the
  heaviest root (on a tie, the first charged), so a run that costs n steps needs a budget of at
  least n + 1. Its path is the value being evaluated (API.md F1):

```
error[E4401]  balance/parity/sweep_plan.canon:265:15
  plan: evaluation budget of 100000000 steps exhausted
  heaviest: plan (98412330 steps)
```

- After `E4401`, evaluation stops: no further root runs, and the build fails. Findings already
  produced are kept. `E4401` is reported once per invocation; a phase-2 constant fold that fails
  because the budget is spent is still `E3015`, variant `budget` (DECISIONS 150: no declaration
  breaks silently; 263). Stage E still runs after `E4401`, but a fold there that fails on the spent
  counter adds no finding: `E4401` has already failed the build. Because the order of evaluation
  (§2) is fixed, two implementations stop at the same expression.

---

## 13. Provenance (EVL-07)

Every value carries a **provenance**:

| Field | Meaning |
|---|---|
| `origin` | `literal`, `json`, `csv`, `defines`, `text`, `default`, `spread`, `computed` or `layer` (API.md `OriginKind`) |
| `file` | project-relative path, or `@root/…` for a file under a declared root |
| `line`, `col`, `endLine`, `endCol` | 1-based; columns count UTF-8 bytes |
| `pointer` | RFC 6901 JSON pointer for `json`; `/<row>/<column>` for a `csv` cell (1-based, the header counted as row 1; `/<row>` for a whole record) |
| `layer` | the layer name (`layer` only) |
| `stack` | up to the 16 innermost user-function frames `{fn, file, line, col}`, innermost first, plus a count of omitted frames |
| `via` | a second provenance (below; API.md `Origin.Via`) |

Rules:

- A literal in Canon source: the span of the literal. A table entry: from its key token to the
  end of its body. A map key: the key token.
- A value read from a file: the first byte of the value (`{` for an object), with its pointer;
  a JSON object key: the first byte of the key string. CSV: the cell. `load.defines`: the `NAME`
  of the `#define`. `load.text`: line 1, column 1.
- A default-filled field: `default`, the default expression in the record declaration, with
  `via` at the literal or object that omitted the field.
- A spread-copied field: `spread`, at the spread (`...e`), with `via` the original provenance of
  the copied value. Findings about the value are located at that original provenance. The new
  instance has the spread literal's provenance.
- A value computed by an operator, call or literal inside a function: that expression, plus the
  call stack at that moment.
- A value that is only passed along (bound, returned, stored, converted, dereferenced) keeps its
  provenance. Conversions never change it.
- An amended sub-value: the right-hand side in the layer file, with `layer` set.

The **value path** of a finding (SPEC §21.1) is the path from its top-level value to the located
value, in the syntax of CLI §2.6 and API-02 (keyed lists and tables by key). It is omitted when
the value is not reachable from a top-level value (a temporary, a test subject).

---

## 14. Findings

- A finding has: severity, code, location (from the provenance), value path, message, related
  locations, `check` (the check name, if any), `stack` and `layer` (API.md §4 gives the JSON shape,
  DIAG-02, and the text form, F9–F15). A related location is what the value was checked against
  (its declared type, a check, an annotation); a second occurrence (a duplicate key) is named in
  the message.
- **Order** (EVL-09): findings without a file first (sorted by code, then message), then by
  file path bytes, line, column, code and message.
- **Duplicates**: findings with the same severity, code, file, line, column and message are
  reported once. The one kept, with its related locations and stack, is the first in the total
  order of IMPLEMENTATION-PLAN.md §4.4 (every field compared after the order above), so that
  scheduling never chooses it (NFR-05, DECISIONS 105). Duplicates are judged on the source-language
  message, as F7's truncation is, so the finding set does not depend on `Options.Lang` (DECISIONS 281).
- Findings produced inside `expect` subjects are captured by the test, never printed as
  findings.
- Errors block emission (SPEC §10.3); warnings do not.

---

## Diagnostics

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E1901 | error | `--layer` name without a file (exit code 2) |
| E1902 | error | target is a `const`, function or type |
| E1903 | error | §11.1 placement |
| E1904 | error | input refinement pattern (§11.3) |
| E1905 | error | static when the path does not fit the type; hard at application |
| E1906 | error | §9.1 |
| E1907 | error | §11.1 |
| E1908 | error | §9.2 |
| E1909 | error | §9.2 |
| E1910 | error | §11.1 |
| E1911 | error | §11.1 |
| E4001 | error | postfix `!` on `none` (hard) |
| E4002 | error | missing key or index (hard) |
| E4101 | error | §6.1, §6.3 (hard) |
| E4102 | error | §6.1, §6.3 (hard) |
| E4103 | error | `Int(f)`, `floor`, `ceil`, `round` (hard) |
| E4104 | error | NaN or infinity produced (hard) |
| E4201 | error | compiler bug (exit code 3) |
| E4301 | error | §3.2 (hard) |
| E4401 | error | §12.2 |
| E4402 | error | §3.3 (hard) |
| E5001 | error | a one-line `check` is false |
| W5001 | warning | a one-line `warn` is false |
| E5002 | error | `fail(at, message)` |
| W5002 | warning | `warn(at, message)` in a block |
| E5003 | error | §8.3 |
| E5004 | error | `expect … fails name` (§10.3) |
| E5005 | error | §10.1 |

Codes defined elsewhere and used here: `E1105`, `E1130`, `E1131` (GRAMMAR.md); `E2102`, `E3xxx` (TYPES.md,
except `E3203` and `E3315`–`E3318`, WIRE.md); `E4105`–`E4108`, `E45xx` (STDLIB.md); `E6004` and
`E6xxx` (LOCK.md); `E7xxx` (WIRE.md). The single catalogue is [ERRORS.md](ERRORS.md).
