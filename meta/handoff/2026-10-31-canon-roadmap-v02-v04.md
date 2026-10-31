# Canon v0.2 to v0.4: what to build next, after the web-client exploration

2026-10-31, canon 0.1.13. **To:** Louis. **Also for** the Source session: §7 and §8.

**Status: proposal. Nothing is started.** Each version is decided whole in an ADR when it starts,
as v0.1.9 and v0.1.10 were (ADR-0021, ADR-0022), and then run. The technical calls are made here.
Your four calls are in §9. Q2 to Q4 were answered on 2026-10-31; Q1 waits for your yes.

**Version numbers.** "v0.1.2 / 0.1.3 / 0.1.4" are read as **v0.2 / v0.3 / v0.4**. v0.1.13 is out,
and v0.2 is the version [plan.md](../plan.md) already holds.

**Inputs:**
- the [web-client handoff](2026-10-31-sovereign-web-client-direction.md) (exploration, not scheduled);
- the two open C++ asks: [macro collisions](2026-10-29-sovereign-cpp-macro-collisions.md) and
  [checked widths](2026-10-30-sovereign-cpp-checked-widths.md);
- plan.md's v0.2 list (your picks of 2026-10-14);
- the owed lists in [state.md](../state.md) and in the logs of 10-21, 10-22 and 10-29.

Four read-only passes over the spec and the generators checked every ask against the law
([log](../decisions/log-2026-10-31.md)).

## 1. The filter

Sovereign decides the **order**, never the **design** (2026-10-12 §1.1). An item stays only if:
1. it is specified without naming Sovereign or its formats;
2. another plausible user needs it;
3. it adds no knob that only one consumer uses.

Four more rules apply:
- safety by construction comes before detection ("hard to fuck it up, like Rust");
- one form per meaning (DECISIONS 319);
- robustness before speed (log-2026-10-14);
- every ban names its way out (DECISIONS 305).

What the law already says, and this plan keeps:
- **Read-only, always** (DECISIONS 4, yours). Generated code holds "types, values, lookups and getters"
  plus translated `export fn` (SPEC §1.1, §22). Only generated loaders, decoders and make hooks fill
  values (SPEC §15.1).
- **A refinement never changes a generated type** (GEN-02, CODEGEN §4.1). Otherwise a bound edit would
  change a C++ or Go type, and the fingerprint does not cover refinements. A width is declared instead:
  `Int32(1..=n)`, `UInt8(..=8)`.
- **Data matches its fingerprint exactly, or it is refused** (FINGERPRINT §6). No schema version
  exists that someone could forget to bump.
- **Nothing refuses** binary encodings, bit operations, `sqrt`, Float32 arithmetic or a wider
  translated subset. These widen the language and reverse no decision. SPEC §23.3 leaves binary
  layouts open.

## 2. The versions at a glance

| Version | Theme | Size | Why this order |
|---|---|---|---|
| **v0.2** | Trust: tested checks, checked records, every owed bug | ~17 units, about a day | N5 is your first pick. Sovereign's L-0147 wave E is about to build hot paths on generated records, so their guarantees come first |
| **v0.3** | Lean generated code. No language change; every change is checked by goldens | ~15 units, mostly sonnet, about a day | 740k generated lines today, and 5-10x ahead |
| **v0.4** | Formulas: the translated subset grows, exact on every target | ~12 units, mostly opus, about two days | the language work. It serves Sovereign's tooltip formulas (L-0148) and three Go ports that drift today |
| v0.5 | Binary data | direction | the browser's data format and the server boot |
| v0.6 | Runtime values and messages, in schema packages | direction; Q1 waits for your yes | the wire protocol (handoff L3) |

Sizes are scaled from v0.1.9, where 30 units took 18-24 h of agent time.

## 3. v0.2: trust

| # | Item | Why general | Owner, tier | Size |
|---|---|---|---|---|
| 1 | **N5, testable package checks.** A test runs the package checks over substituted top-level values, on `canon test --layer`'s machinery. Static errors stay uncatchable (CHK-05) | any project with package checks. In Resource, 0 of 97 are tested, and 69 fns work around it | check, eval, cli; opus | L |
| 2 | **Decoders check what refinements promise.** Every generated decoder (C++, Go, TS) refuses, with the key path, a value outside its range refinement and a string, list or map outside its length refinement. Patterns and `where` stay build-time. Refinements enter the IR (today `Field.Range` is set for inputs only; `ir.go` is frozen, so §4's review rule applies). Amends CODEGEN §5.13 and WIRE §5.13 ("accepts without checking refinements") | anyone reading data they did not check themselves. Sovereign re-checks by hand in ~310 guards | ir, then gen/cpp, gen/go, gen/ts; opus for ir, sonnet per target | 3×M |
| 3 | **Bounds as constants:** the min and max of every range and length refinement, in each target (C++ `k<Record><Field>Min`...) | to `static_assert` a hand-sized array against a Canon bound | with 2 | S |
| 4 | **Stable addresses, written down.** Rows of a baked or decoded collection never move while their owner lives. That is true today: they are built once, never mutated, and their containers are move-only. A `@reload` makes a new snapshot (DECISIONS 16). Add a spec sentence and a test per target | anyone keeping pointers into tables | spec, gen/cpp | S |
| 5 | **Opaque enums** (ADR-0021 §9, DECISIONS 379). Go: `struct` plus `Position()`. C++: deleted nlohmann hooks, after a read-only Source scan. TS: `<E>Index` becomes private | positions are not ids (N11) | gen/go, gen/cpp, gen/ts; sonnet | M+S+S |
| 6 | **Macro names across packages** ([ask](2026-10-29-sovereign-cpp-macro-collisions.md)). **Case 1** (a define from a package the enum's package does not read): a read-only Resource scan counts the clashes first. The ADR then picks one of two rules: a check that refuses the clash and names its way out (`@cpp(name:)`, or one enum per concept), or a guard set widened to the whole project. The lean is the refusal, because it keeps a package's output a function of what that package reads. **Case 2** needs no Canon change: render `WindowIds.h` with `@cpp(defines:)` instead of `emit text`. Also owed: guards stop counting names that appear only inside string literals | any C++ codebase with legacy macros | check, gen/cpp; opus decides, sonnet builds | M |
| 7 | **E3804** (dependent types through static typing, owed by DECISIONS 338) **and E1905** (an amend into a dependent map) | correct dependent types | check, types, eval; opus | M |
| 8 | **Wire names** (owed, log-10-21). A NUL in a JSON member name or variant tag is refused at check, instead of being cut by the C++ runtime. Equal wires in one enum (`a = "x", b = "x"`) become an error, unless the scan finds a real use | a round trip of the later member silently returns the first | check; sonnet | S |
| 9 | **Owed bugs, generators.** gen/go's conformance keys refs off `g.isData()`; gen/ts writes an undeclared `<Id>Index`; no C++ test covers two packages sharing one namespace; the C++ macro lists become one embedded data file | the standing rule: every known bug is fixed | gen/*; sonnet | M |
| 10 | **Owed bugs, edit.** `edit`'s rollback error holds an absolute temp path; J7 does not lock readers of a JSON file newly created in a listed directory; `evalAnalysis` never calls `KeptSelection`; a `@text` output named `.canon` or `.json` may overwrite an unmarked file (to verify) | | edit, workspace; opus | M |
| 11 | **Owed bugs, Windows and links** (ADR-0021 §9). `load.dir` skips a junction without `W7115`; single-file `load`, `load.text`, `load.csv` and `load.defines` read through any link; Windows refresh; the cache's stat table trusts size + mtime | | load, workspace; opus | M |
| 12 | **Owed, check and graph** (log-10-29). Entries of an optional collection field (TYPES §10.2); a ref rebound by a spread is listed only as a code ref (R7); progen operators for `tiers` (`E1017`, `E1018`, `E2007`); one `stampMust` helper instead of two | | check, eval, testkit; sonnet | M |
| 13 | **Spec and catalogue hygiene.** E9001, E9006 and E8104 name their way out (DECISIONS 305). CONFORMANCE §3 lists Float `%`, but TYPES §7.1 types `%` on Int only. CODEGEN §6.3 has drifted from the Go runtime by 259 lines | | diag, spec; orchestrator | S |
| 14 | M4 call (b): `check.Info` layered maps, only if easy and with no robustness risk (your 2026-10-14 condition) | | api; opus | M |
| 15 | Spec sync first, then cleanup and the release gates: `make check`, `make realdata-compile` (DECISIONS 394), the Resource dry run, and the deferred long fuzz and progen campaigns, run once | | | |

**Refused from the checked-widths ask:** member widths taken from refinements (GEN-02). Declare the
width instead (`Int32(1..=MAX)`). Agents can do that with `canon edit`, and it settles each getter's
type before wave E.

**Moved out of v0.2:**
- `ordered_json`, kind constants and N8 templates go to v0.3, because they shape output;
- N1, legacy C++ structs: parked (Q4);
- N2, one declaration per id: v0.5;
- N3, `canon lint`: v0.3.

## 4. v0.3: lean generated code

| # | Item | Why general | Owner, tier | Size |
|---|---|---|---|---|
| 1 | **Enum tables.** `ToWire` reads the name table when wires equal names: that is 28 of 31 golden enums and 332 of 340 Sovereign ones. `FromWire` searches a permutation index, not a second sorted copy of the strings. Code to ordinal goes by direct index when the codes are dense. This amends DECISIONS 383's table layout; the API is unchanged. Go gets the same dedupe of DECISIONS 393's tables | size and O(1) lookups. Enum helpers are 1,016 of 1,734 lines of `telemetry.gen.cpp` | gen/cpp, gen/go; sonnet | M+S |
| 2 | **One-pass C++ JSON reader.** A valid file costs one strict SAX pass that builds the DOM. The exact scanners that write the specified first-failure messages run only when that pass fails. Verdicts and messages stay the same, proven by a differential fuzz against today's five passes | boot time of every C++ data consumer. Sovereign's `propItem` (25 MB) moves to the generated decoder soon | gen/cpp runtime; opus | M |
| 3 | **Header weight, measured first.** Measure on Sovereign's tree which imported headers are needed whole. Then give imports that only name enums a forward-declaration header (`enum class X : T;`). An opened enum's per-member `inline const` moves to the `.gen.cpp`; today it costs 60,000 dynamic initialisers in every including TU (log-10-21). Imported records held by value, and constexpr lookups over imported members, still need the whole header, so the gain is measured before anything is built | compile cost of every C++ consumer. Sovereign's header closures are 256 MB, against 16 MB of own code | gen/cpp; sonnet, opus review | L |
| 4 | **Baked data without heap copies.** Views (`std::span`, `std::string_view`) over the constexpr tables replace `Build()` into `std::vector<std::string>`, with a sorted constexpr table for lookup by name | boot heap. Sovereign's 33k id strings are copied twice | gen/cpp; sonnet | M |
| 5 | **TS for bundles.** `/*#__PURE__*/` on every table construction (the helpers have no side effects). The guide documents `"sideEffects": false`. Measure the helper block repeated in every file; if it is large, TS gets one runtime module per output directory, as C++ has | browser and Node bundles drop what they do not import | gen/ts; sonnet | S+M |
| 6 | **A function-size budget** in the generators' tests: C++ `Decode` and `Make` against record width, Go `build<Last>` against row count. Bodies over the budget are split (`Decode(TroupeSkill)` is 770 lines; log-10-21 lists Go functions that keep growing) | the 0.1.11 stack-frame class, at 5-10x | gen/cpp, gen/go; sonnet | M |
| 7 | **Runtime copies of one version must agree.** The C++ guard is keyed by version, not content. A stale copy of the same version can shadow a newer one, and two different copies in one binary break the ODR. A mismatch must fail the build | latent today | gen/cpp runtime; opus | S |
| 8 | `ordered_json` overload (DECISIONS 290) and kind constants (DECISIONS 292) | your v0.2 picks, moved here | gen/*; sonnet | S |
| 9 | **N8:** output-path and Go package-name templates (resumes DECISIONS 229's deferral) | 75 of 80 Sovereign Go emit paths follow one pattern | project, build; sonnet | M |
| 10 | **N3, `canon lint`.** Advisory, with agent-facing JSON; `--fix` emits `canon edit` requests | keeps legacy shapes out of models, such as the legacy member names behind the macro clashes | new package, cli; sonnet | M |
| 11 | **Scale bench.** Synthetic projects at 1x, 3x and 5x Resource's size, with `check` time and peak RSS in `make bench` | the numbers Q3 needs | testkit; sonnet | S |
| 12 | **Stretch: C++ presence bits** instead of one `std::optional` per field, getters unchanged (about 14 generator files) | memory and cache behaviour of wide records | gen/cpp; opus | L |
| 13 | Spec sync, cleanup, gates | | | |

**Not in v0.3: one shared runtime per root for C++ and Go.** Each output directory is self-contained by
design, so it can be moved and vendored (ADR-0022 §10), and the C++ copies cost no compile time behind
their guard. Building it is large: 12 or more files, and a field on the frozen `ir.Emit`. It comes
back only if a consumer measures a real cost.

## 5. v0.4: formulas

Every widening is bit-exact on every target by specification; otherwise it is refused, with its way
out.

| # | Item | Why it is exact, or why general | Owner, tier | Size |
|---|---|---|---|---|
| 1 | **Control:** `match` over enums and variants, local `var`, and `for` over a range with constant or refined bounds or over a list parameter. `while` stays out, because generated code has no step budget; its error names `for` as the way out. Also check that translated recursion, allowed today, is bounded at run time | termination by construction | conform, ir, gen/*; opus | L |
| 2 | **Structured inputs and results:** record, optional and list parameters, several records, and record results | a formula over an item instance. One record per audience plus a projection fn replaces per-audience serializers | check, ir, gen/*, conform; opus | L |
| 3 | **One rule for lookup versus translation.** A fn is a lookup when its domain is finite and holds at most N cells, and a refined `Int(0..=20)` counts as finite. Otherwise it is translated, and its body must be portable (`E9001`). N replaces `E9002`'s 65,536. Same-package baked tables become readable by key | any build-time computation, `pow` included, ships as an exact table. A 14-Bool fn stops being a 16,384-cell literal (292 KB of TS today) | ir, gen/*, conform; opus | M-L |
| 4 | **Math.** `sqrt`, which IEEE 754 rounds correctly, so every target agrees. `pow` with an `Int` exponent, by an algorithm STDLIB fixes and the evaluator also uses. A real exponent stays build-time, with item 3 as the way out | | eval, stdlib, gen/*; opus | M |
| 5 | **Float32 arithmetic.** An operation on two `Float32` values is IEEE binary32, rounded per operation: C++ `float`, Go `float32`, TS `Math.fround` (exact for `+ - * /` and `sqrt` after a double operation). Mixing with `Float` gives `Float`. Amends TYPES §7.3 ("`Float32` is `Float` in expressions") | games, graphics, embedded: any runtime that computes in `float` | types, check, eval, gen/*; opus | L |
| 6 | **Bit operations as builtins:** `bitAnd`, `bitOr`, `bitXor`, `bitNot`, `shiftLeft`, `shiftRight`, on integers, two's complement over 64 bits. TS goes through `BigInt`, or through plain 32-bit operations where ranges prove the operands fit. No operator characters: `|` already means union and line continuation, and C's `&`-below-`==` precedence trap stays out of the language | flags and packing in any binary format | stdlib, eval, gen/*; opus | M |
| 7 | **Floats in translated templates,** with STDLIB §9.2's canonical form and §9.5's `.N` (ties away from zero, as TS's `toFixed` already does below 10^21). Each target gets a runtime helper | any UI text with numbers ("12.5%") | gen/*; sonnet, opus review | M |
| 8 | **`bigint` in translated TS:** `@ts(bigint)` values as parameters and results. They are `E9006` today; fields have had them since DECISIONS 278 | money, ids and timestamps past 2^53 | gen/ts, conform; sonnet | M |
| 9 | **The floating-point contract, verified.** CONFORMANCE §5 states what consumers compile with (no fast-math, no FMA contraction), and the conformance tests catch a violation. Today's `Float` translation is checked the same way | today's bit-exact claim rests on it; not yet verified | spec, conform; opus | S |
| 10 | **Guide: porting a legacy formula.** Captured vectors are written as `test` plus `expect f(x) == v`, and their inputs already join conformance (CONFORMANCE §6). Add a scale probe with thousands of vectors | any port of existing logic | guide, testkit; sonnet | S |
| 11 | Spec sync (CONFORMANCE §2, STDLIB §2, TYPES §7.3), cleanup, gates | | | |

## 6. After v0.4, in order (not scheduled)

**v0.5, binary data** (handoff L1, L5). One binary encoding of Canon values, specified beside the JSON
one (WIRE, FINGERPRINT), and written by `canon build` for `data` mode.
- The fingerprint must match exactly, and a mismatch is refused as for JSON data. No compatibility
  knob.
- C++ reads a loaded or mapped buffer, Go a byte slice, and TS an `ArrayBuffer`, lazily per row.
- The format has a string table, presence bits, varints, bit sets and variant case codes. Its bounds
  come from refinements.
- Tables are chunked, with an index, so a browser fetches a chunk, never a file per entry.
- A copy may choose its own mode: a service keeps `baked` while a browser takes binary `data`
  (amends DECISIONS 229).
- Every output is listed with its SHA-256, as `canon.outputs` already does for text. Content-hashed
  file names stay the deploy step's job.
- **N2, one declaration per id,** lands with it: a table's id type carries its codes, which gives
  id-indexed arrays.

**v0.6, runtime values and messages** (handoff L3, L3b), in Canon, as schema packages (§9, Q1).
- Schema packages hold types, consts and `export fn`, never values. Their records stay immutable.
  They are created by validating builders (ranges and lengths, never named checks), and they get
  encoders for the JSON and binary wires.
- Messages are variants with stable case codes (`canon.lock`). A frame is `[Message]`, and a snapshot
  batch is `[Snapshot]`.
- The schema fingerprint is the protocol version, exchanged at the handshake. Sovereign's version
  gate becomes structural and cannot be forgotten. There is no per-field `since N`.
- Acceptance is on the Sovereign side: the TS headless client.

**Parked:**
- N1, legacy C++ structs (Q4, Louis 2026-10-31).
- N7, native package moves: when the new package families start moving packages.
- M7, `convert`.
- The on-disk cache, per-entry re-check and parallelism: until v0.3's scale bench gives numbers
  (Q3), under your 2026-10-14 rule.

## 7. Every ask of the handoff, judged

- **Keep:** built as asked.
- **Reshape:** the need is kept, but the shape changes.
- **Refuse:** refused by the rule named.
- **Exists:** already possible today.
- **Not Canon:** the owner is named.

| Ask (handoff §) | Verdict | Where |
|---|---|---|
| L1 large TS tables without one giant module (4.1) | Reshape: chunked binary tables with a lazy TS reader. The TS `data` loader (JSON) exists today | v0.5; `/*#__PURE__*/` in v0.3 |
| L1 different shapes per consumer, `E8002` (4.1, 7.4) | Reshape: a copy may choose its mode. No consumer-declared pruning lists | v0.5 |
| L1 binary data (4.1) | Keep | v0.5 |
| L1 game text in TS (4.1) | Exists: give the text packages `emit ts`, and their imports too (`E8004`) | Sovereign |
| L1 runtime inputs in TS, `E8104` (4.1) | Refuse: inputs are environment variables, and a browser has none (SPEC §19). The message names the way out: keep the inputs in their own package | message in v0.2 |
| L2 record, list and optional parameters | Keep | v0.4 #2 |
| L2 reading a table at run time | Reshape: finite domains (a refined Int counts) and same-package baked tables by key | v0.4 #3 |
| L2 `pow`, `sqrt` | Keep `sqrt`, and `pow` with an Int exponent. A real exponent becomes a lookup | v0.4 #3, #4 |
| L2 `match`, `var`, loops | Keep `match`, `var` and bounded `for`. Refuse `while`: there is no run-time budget | v0.4 #1 |
| L2 bitwise operators | Keep, as builtins | v0.4 #6 |
| L2 Float32 arithmetic | Keep | v0.4 #5 |
| L2 a Float in templates | Keep | v0.4 #7 |
| L2 int64 in TS | Exists for fields (`@ts(bigint)`); translated fns get it | v0.4 #8 |
| L2 legacy golden vectors | Exists: `test` plus `expect`. A recipe and a scale probe | v0.4 #10 |
| L2 large finite fns | Keep: one rule for lookup versus translation | v0.4 #3 |
| L3 binary codec, encode and decode | Keep in kind, on v0.5's encoding | v0.6 |
| L3 a byte layout compatible with `CAr` | Refuse: byte-identical legacy writers (ruling B1, DECISIONS 327). Each message switches format under Sovereign's version gate, and both peers ship together | |
| L3 a field present since version N | Reshape: the fingerprint is the version, checked at the handshake | v0.6 |
| L3 a protocol version constant | Exists in kind: the generated `<V>Schema` fingerprints | v0.6, for messages |
| L3 a message id registry | Exists: `stable table`, `@codes`, `canon.lock` | |
| L3 tagged unions, bit fields, varints and bounded arrays in binary | Keep, inside the encoding | v0.5 |
| L3 a stream of tagged records in one frame | Reshape: a list of a variant | v0.6 |
| L3 audience projections | Refuse as a per-field knob. One record per audience, plus a projection fn | v0.4 #2 |
| L3b builders and encoders | Keep, for schema packages (Q1) | v0.6 |
| L3b checked decode of refined bounds | Keep | v0.2 #2 |
| L3b member widths from refinements | Refuse (GEN-02); declare the width | Sovereign |
| L3b exported bounds | Keep | v0.2 #3 |
| L3b open ids in C++ and TS | Exists since v0.1.6 (DECISIONS 362, `types` mode) | |
| L4 kernel constants and golden vectors | Exists: a `const`, a `[Record]` let | |
| L4 generating the kernel | Not Canon (SPEC §22) | Source |
| L5 asset refs with existence checks | Exists | |
| L5 asset hash and size, the CDN manifest | Not Canon: the baker | Source |
| L5 per-entry or per-chunk files | Reshape: chunked tables | v0.5 |
| L5 content-hashed names; a manifest | The names are not Canon (deploy step). The manifest of Canon's outputs: Keep | v0.5 |
| L5 ~2,800 effect definitions; spatial checks | Exists in kind. The budget is measured by the scale bench | v0.3 #11 |
| §7.5 packet ids permuted per release | Not Canon: a seeded remap after generation. Canon keeps the logical id | Source |
| §7.6 messages behind build flags | Refuse types that change with build flags: one schema per project. An exported fn tells both clients whether a message is on | v0.4 |
| §8.1 drop the shims and guards | Exists: `@cpp(defines:)` is opt-in per enum, and without it there is neither shim nor guard (CODEGEN §7.9). The sweep is Sovereign's; the cross-package rule is v0.2 #6 | |
| §8.2 header split | Keep, measured first | v0.3 #3 |
| §8.3 binary columnar data; compact JSON | Binary: v0.5. Refuse compact JSON or an indent option: one form (DECISIONS 327). The `"="` none marker is Resource's own `@json(none: "=")` (161 uses); without it an absent field is left out | |
| §8.4 record layout | Refuse widths (GEN-02). Presence bits are a v0.3 stretch; stable addresses are v0.2 #4 | |
| §8.5 one shared runtime | TS: one module per directory (v0.3 #5). C++ and Go stay per directory, self-contained by design | |
| §8.6 dense lookups; §8.7 name-table dedupe | Keep | v0.3 #1 |
| §8.8 no materialized copies | Keep | v0.3 #4 |
| §8.9 per-consumer pruning | Refuse consumer-declared lists, because they drift. Use finer packages, `values:` and lighter headers | |
| §8.10 TS hygiene | Keep | v0.3 #5; v0.4 #3, #8 |
| §8.11 function-size budget | Keep | v0.3 #6 |
| §8.12 incremental or cached check | Parked until v0.3's scale bench (Q3) | |

## 8. For the Source session

**Answers to the handoff's §10.**
1. **Wire codec.** It is a Canon emit, not a separate generator. One type model gives refinements,
   stable ids and conformance in three languages; a separate generator would map the types a second
   time. It comes in stages: the binary encoding for data (v0.5), then messages (v0.6, after Louis's
   call on runtime values).
2. **Binary data.** Yes: one format for C++, TS and Go under one fingerprint. `data` mode is its home
   (v0.5).
3. **Float32.** An arithmetic rule: two `Float32` operands compute in binary32, and mixing gives
   `Float`. `pow` takes an `Int` exponent by a specified algorithm. A real exponent is a lookup
   computed at build time (v0.4).
4. **Versioning.** Not "since N". The schema fingerprint is the version, checked at the handshake.
   Ids and case codes come from `canon.lock` (v0.6).
5. **Projections.** Several records plus projection fns (v0.4). There is no first-class form.
6. **Builders and encoders.** In Canon, for records of schema packages, which hold types only (§9,
   Q1, waiting for Louis's yes). Configuration records keep DECISIONS 4, and only Canon's loaders and
   decoders fill them.
7. **Pruning and per-entry outputs.** No consumer-declared pruning. Chunked binary tables replace
   per-entry files, and a copy may choose its mode (v0.5).
8. **Scale.** Per-package budgets scale by splitting packages. Whole-project time and memory are
   measured in v0.3. The on-disk cache stays parked until those numbers exist (Louis, Q3).
9. **What §8 costs.**
   - Cheap: `/*#__PURE__*/`, `ToWire` on the name table, dense ordinals, the loader's duplicate key
     set, the function-size test.
   - Medium: the permutation index, the one-pass reader, views of baked data, the macro rule.
   - Large: the header split (its gain is limited to imports that only name enums), presence bits,
     a shared runtime per root, and checked decode in all three targets.

**Facts in the handoff to correct:**
- C++ and TS have had open enums since v0.1.6 (`types` mode, DECISIONS 362).
- Fields already have bigint, through `@ts(bigint)`; only translated fns refuse it.
- Generated headers include only `nlohmann/json_fwd.hpp`. `json.hpp` comes from the runtime, which a
  `.gen.cpp` includes.
- The `"="` marker is Resource's own `@json(none:)` spelling.
- Game text in TS needs only an emit line.
- N4 (more JSON wire forms) was refused on 2026-10-14. The binary encoding is a different item.

**Sovereign's side, whatever Canon schedules:**
- Sweep the legacy macro call sites, then drop `@cpp(defines:)` package by package. The guards
  disappear with the shims.
- Render `Server/UI/WindowIds.h` with `@cpp(defines:)` instead of `emit text`.
- Declare widths where they matter (`Int32(1..=MAX)`) before L-0147 wave E.
- Drop `@json(none: "=")` once `propItem` is on the generated decoder.
- Compile the generated code as one library per language, outside the game's precompiled headers.
- Build every service from one Resource state, or share one generated module. That ends the TS copies
  that drift.
- These stay on the Source side: the id permutation, flag checks in handlers, asset hashing and the
  CDN manifest, the WebAssembly kernel, and the baker.
- Captured legacy vectors can already be written as `test` blocks.

## 9. Louis's calls

**Answered by Louis, 2026-10-31:**
- **Q2, formulas before binary data: yes.** v0.4 is formulas and v0.5 binary data. Sovereign's own
  plan (handoff §9) had binary data first. Formulas are the language work, and they serve Sovereign
  now (L-0148, three drifting Go ports), whatever the web client decides.
- **Q3, the on-disk cache: parked** until v0.3's scale bench gives numbers. A whole check takes 31 s
  and 2 GB today. The safe design is already written (log-2026-10-14).
- **Q4, N1 legacy C++ structs: parked**, because Sovereign chose generated records for every table
  (L-0147). DECISIONS 5 stays, for any other codebase.

**Q1, runtime values (v0.6).** Today every generated value is configuration that Canon checked
(DECISIONS 4, SPEC §15.1). The web client asks Canon to also describe the state and messages that
programs build and send. Louis's follow-up: should this be part of Canon, or another language that
reuses Canon's code?

**Recommendation: part of Canon, with a boundary as strict as a second language would give.**

Why one language:
1. **The type system is the same.** A message is records, variants, enums, refined scalars, lists
   and optionals, which are Canon's types. A second language would either copy that type system,
   leaving two specs to keep identical, or import Canon's, which makes it Canon under another file
   extension.
2. **Messages point into configuration.** A message carries an `ItemId`, an enum member or a ref.
   In one project, ids and case codes come from one `canon.lock`. Two languages would need a
   cross-language import and two locks kept in step.
3. **The formulas put first take runtime state.** L-0148's instance formulas (upgrade, rarity,
   Azure) are functions of an item instance. A translated fn can type that parameter only if the
   state record lives in the same type system.
4. **Agents learn one language.** Two nearly identical languages with different rules are the worst
   case for agent mistakes ("this works in one file kind and not the other").
5. **The code is reused either way.** A second front end over Canon's internal packages would make
   every frozen contract serve two languages, so a change made for one could break the other.

What keeps configuration safe:
- **Schema packages.** Runtime types live in packages that declare only types, consts and
  `export fn`: no `let`, no entries, no layers, no data checks. Nothing configuration-like can live
  there. A package's kind is declared and checked.
- **One import direction.** A schema package may import configuration packages (enums, ids, refs);
  a configuration package never imports a schema package. A message carries an id, never a copy of
  a configuration record. Import tiers already enforce such directions (DECISIONS 380).
- **Read-only, always, still holds** (DECISIONS 4). Records of both kinds have getters and no
  setters. What changes is who creates a value:
  - a configuration record is still filled only by Canon's loaders and decoders;
  - a schema record may also come from a validating builder, which checks ranges and lengths as
    v0.2's decoders do. Named checks never run at run time.

  SPEC §15.1 changes for schema packages only.
- **A companion document of its own** in `spec/`, so the configuration chapters keep their meaning.

A separate tool would be right only to export Canon's types to a foreign ecosystem, such as `.proto`
files for a gRPC peer. That is an export of Canon's types, not a new language, and nothing needs it
now.

**Status:** recommended; waiting for Louis's yes. The v0.6 ADR then fixes the syntax.
