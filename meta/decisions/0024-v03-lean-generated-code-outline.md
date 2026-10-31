# ADR-0024 — v0.3, lean generated code: the outline

Date: 2026-10-31. **Status: outline, proposed.** Scope: Louis, 2026-10-31
([roadmap](../handoff/2026-10-31-canon-roadmap-v02-v04.md) §4, [log-2026-10-31](log-2026-10-31.md)).
The scope and the direction of each item are fixed here. The designs are not.

## 0. How to use this outline

- This outline becomes a decision-complete ADR, in the format of ADR-0021 and ADR-0023, when v0.3
  starts and v0.2 is released. v0.2 changes the code these items touch (§2), so designing now would
  mean designing twice.
- The **directions bind.** The v0.3 start session writes each item's Facts, Decision, Spec text,
  Tests and Unit, re-reading the code of that day. Every file:line below is from `4dfd414e`; these
  references locate, they do not bind.
- **First step: measure (§3).** Then the designs, then the run (ADR-0023 §7's rules apply).
- **No item changes the language.** Every change is checked by goldens and by the compile and run tests
  of the generated code. Generated APIs keep their behaviour, except where an item says its API
  changes (items 4 and 5).

## 1. Scope, in priority order

### 1. Enum tables (C++, Go)
- **Facts.**
  - `internal/gen/cpp/enums_tables.go` `closedDefinitions` (~108-127) writes:
    - `ToName` (`kNames`), `ToWire` (`kWires`) and `IsRetired` (`kRetired`);
    - `FromWire` (`kKeys`, a sorted copy of the wires, plus `kValues`).
  - All of them are function-local `static constexpr`.
  - Open enums already keep one sorted wire table in an unnamed namespace (~130-169).
  - `ToWire`'s table equals `ToName`'s in 28 of 31 golden enums, and in 332 of 340 Sovereign enums.
    Enum helpers are 1,016 of 1,734 lines of `telemetry.gen.cpp`.
  - Ordinals:
    - `detail::k<E>Codes`, `k<E>CodeOrdinals` (always `uint32_t`) and a constexpr `<E>Ordinal`
      lower-bound search (`enums.go` ~134-150);
    - without `@codes`, the ordinal is a `static_cast`;
    - "n for a non-member" is relied on (`stored.go` abort check, `ToName` guards).
  - Spec: CODEGEN §5.2, §5.10, §7.1; DECISIONS 383 ("never a chain or a switch"), 393 (Go).
- **Direction.**
  - `ToWire` reads the name table when the two are equal.
  - `FromWire` searches a permutation index over the one string table.
  - Code to ordinal goes by direct index when the codes are dense in declaration order
    (`code == first + ordinal`); otherwise today's search.
  - Go gets the same dedupe of 393's tables.
  - The API is unchanged.
- **Open at v0.3's start:**
  - a size policy for a direct table over sparse codes;
  - keeping "n for a non-member";
  - clang's constexpr step limit (`sizedTableMin` = 1,024; `TestBigEnumTablesCompile`, 60,000
    members).
- **Size:** C++ M, Go S. Sonnet.

### 2. One-pass C++ JSON reader
- **Facts.** `ParseTextFile` (embedded `canon_runtime_json.h`, ~856-889) runs these passes in order:
  1. BOM;
  2. `SyntaxScan` (~430-565): the first offending byte, for "invalid JSON at byte N";
  3. `TextScan` (~703-828): depth over 512, invalid UTF-8, unpaired surrogates, repeated keys after
     escape decoding, numbers beyond Float64 (refused at the field);
  4. the `StandIn` rewrite;
  5. `StrictCheck` through `sax_parse` (~265-403): duplicate keys and depth again (unreachable on
     this path, but its per-object `std::set` still runs), Float32 rounding-midpoint tokens,
     file-order key lists;
  6. the DOM parse, then `TakeSidecars`, then the `Decode` walk.

  Data loaders use `ParseDataFile` (~901-932). Grammar and UTF-8 are validated three times, and
  nlohmann lexes the text twice.
- **Direction.**
  - A valid file costs one strict SAX pass. It builds the DOM and records what the field decoders
    need: beyond-Float64 numbers, Float32 midpoints, key order.
  - On any failure, today's scanners run and produce the specified first-failure message. Verdicts
    and messages are therefore identical by construction.
  - A differential fuzz against the five-pass pipeline proves it before the old code goes.
- **Open at v0.3's start:** whether nlohmann's SAX callbacks hand over the raw number text that the
  beyond-Float64 and midpoint rules need. v0.2's refinement checks run in `Decode`, after the parse,
  so they are unaffected.
- **Size:** M. Opus (a runtime both C++ data paths use).

### 3. Header weight, measured first
- **Facts.**
  - Header assembly: `gen.go` ~186-205.
  - Includes:
    - the standard headers;
    - `json_fwd.hpp`, only when the header declares a decoder;
    - `canon_runtime.h`;
    - the headers of the imported packages (`imports.go` ~42-60, ~111).

    A `.gen.cpp` includes none of these itself (~70).
  - Inline by CODEGEN §7.1:
    - getters;
    - `k<E>Members` and `k<E>LiveMembers` (public API);
    - `detail` ordinal tables (~20 lines per `@codes` enum);
    - pure translated fns;
    - constexpr lookup cells (a 14-Bool fn is 16,384 header lines).
  - An imported record held by value needs its complete type. The gain is therefore limited to
    imports that only name enums, and to pointer or reference uses.
  - An opened enum defines one `inline const` per member in its header: 60,000 dynamic initialisers
    in every including TU (log-2026-10-21).
- **First:** classify every imported header on Sovereign's tree by use:
  - (a) enum names only;
  - (b) enum members, as in constexpr lookups;
  - (c) records by value.

  Only (a) gains from a forward-declaration header.
- **Direction.**
  - A per-package forward-declaration header (`enum class X : T;`, `class R;`) included instead of
    the whole header where (a) holds.
  - Opened enums' member definitions move to the `.gen.cpp`.
- **Open at v0.3's start:**
  - the measured gain (drop the item if (a) is rare);
  - the new file's place in E8152, stray and ownership rules;
  - `k<E>Members` stays in the header (public, constexpr).
- **Size:** L. Sonnet, opus review.

### 4. Baked data without heap copies (an API change)
- **Facts.**
  - Eleven `Build()` functions copy constexpr tables into heap `std::vector<std::string>` on first
    use.
  - `IdsAccess::Build` copies 33k names, and Sovereign then copies them again into a `std::map`.
  - DECISIONS 392 already puts the arrays at namespace scope.
- **Direction.** Accessors return views (`std::span<const std::string_view>`) over those arrays,
  plus a sorted constexpr name table to look up by name. A consumer holding a
  `const std::vector<std::string>&` migrates; the reply names the change.
- **Open at v0.3's start:** which accessors change (every `[String]` baked value, or bulk arrays
  only). The views point into static storage, so they cannot dangle.
- **Size:** M. Sonnet.

### 5. TS for bundles
- **Facts.**
  - The `canonFreeze`, `new CanonMap` and `canonTable` calls are in `ts/literal.go` ~177-193,
    `values.go` ~42 and ~135, `fns.go` ~89, `canonlist.go` ~59-63, and `decode.go` ~115-180.
  - Their formats are in `ts/constants.go`.
  - The helpers have no side effects: `canonFreeze` freezes its fresh argument, and `CanonMap`
    freezes itself.
  - Each enum has 4 tables, 5 with `@codes` (`ts/enums.go`).
  - Helpers are inlined into every file as a closure of helper units (`ts/runtime.go` ~103-182),
    pinned by `TestHelperBlockIsTheSpecs` (CODEGEN §8.2).
- **Direction.**
  - `/*#__PURE__*/` before every table construction.
  - The guide documents `"sideEffects": false`.
  - Measure the helper bytes repeated per file on Sovereign's TS. If they are large, TS gets one
    runtime module per output directory, as C++ has, and CODEGEN §8.2 is rewritten.
- **Size:** S, then M if the runtime module is built. Sonnet.

### 6. A function-size budget
- **Facts.**
  - `detail::Decode(<Record>)` has one block per field and is never split (`cpp/decode.go` ~30-90).
  - `<P>Make` takes one parameter per member.
  - Baked `Build` parts split at 1,000 lines, but only between top-level statements (`bulk.go`
    ~200-242).
  - Go's `build<Last>` is one function with one statement per row (`go/baked.go` ~82-175).
  - Tests bound the `Build` parts and the compile RSS; nothing bounds `Decode`, `Make`, Go `build`
    or TS by width.
  - Owed (log-2026-10-21):
    - `monitoring`'s nested foreign-record block (~1,700 lines);
    - Go `buildX`, `codesIndex`, `inputEnumBody` and `readBits`, which grow with members.
- **Direction.**
  - A generator test measures generated function bodies on a wide synthetic record (about 300
    fields) and on 10,000 rows, per target, against a line budget.
  - `Decode` splits into field groups; Go `build` splits into parts, as C++ does.
- **Size:** M. Sonnet.

### 7. Runtime copies of one version must agree
- **Facts.**
  - The guards `CANON_RUNTIME_RT_V1` and `CANON_RUNTIME_JSON_RT_V1`, plus `inline namespace rt_v1`,
    are keyed by version.
  - Additive changes ride on `rt_v1` (CODEGEN §7.5).
  - So a stale copy of the same version shadows a newer one in a TU, and two different copies in one
    binary break the ODR (AUDIT CPP-05).
- **Direction.** A mismatch fails the build. The leading option is a content stamp in the inline
  namespace's name (`rt_v1_<stamp>`): different contents give different symbols, so there is no ODR
  clash, and code generated against two stamps cannot exchange runtime types. Decide the option at
  v0.3's start.
- **Size:** S-M. Opus.

### 8. `ordered_json` and kind constants
- **`ordered_json`:** a C++ overload of `Decode` (DECISIONS 290's "later item"; ADR-0022 §10).
  Without it, maps and nested tables are held in byte order.
- **Kind constants** (DECISIONS 292): CODEGEN §4.4's `E8019 VariantKindConstant` lasts "until the
  generators write the kind enum's member".
- **Size:** S each. Sonnet.

### 9. N8: output-path and Go package-name templates
- DECISIONS 229 deferred "a named group of destinations in `project.canon`" "until the repetition
  shows".
- 75 of 80 Sovereign Go emit paths follow one pattern.
- Emit options are constants today (CODEGEN §2.1, E1132).
- **Direction:** one template form in `project.canon`, with placeholders, used by emits in place of a
  literal `out`.
- **Size:** M. Sonnet.

### 10. N3: `canon lint`
- As specified in the 2026-10-12 roadmap's §3 N3: advisory, agent-facing JSON, outside `check`, with
  `--fix` as `canon edit` requests.
- One more lint: legacy member names that a C++ define also spells (the macro clashes).
- **Size:** M. Sonnet.

### 11. A scale bench
- Synthetic projects at 1x, 3x and 5x Resource's size (`internal/testkit/cmd/benchgen` exists), with
  `check` time and peak RSS in `make bench`.
- Its numbers decide Louis's Q3, the on-disk cache.
- **Size:** S. Sonnet.

### 12. Stretch: C++ presence bits
- One bit per optional scalar replaces one `std::optional` per field, with getters unchanged.
- It touches about 14 generator files: `ctype`, `records`, `stored`, `make`, `decode*`,
  `baked_fill`, `baked_foreign`, `columns`, `define_refs`, `inputs`, `walk`, `resolve`.
- The make hooks change too, but they are reference layout, not API.
- **Size:** L. Opus.

## 2. What v0.2 changes under these items

- **Checked decode** (ADR-0023) adds range and length checks to every decoder. Item 2 keeps them,
  since they run in `Decode`. Item 6's budget counts them.
- **Bound constants** land in the headers, so item 3 measures them too.
- **Opaque enums** change the Go and TS enum API, and the C++ nlohmann hooks. Items 1 and 5 build
  on the new shapes.
- **The cross-package macro rule** changes the guards in headers, which item 3 measures.

## 3. Measure first, at v0.3's start

1. The scale bench (item 11), whose numbers go to Louis.
2. Header-use classes on Sovereign's generated C++ (item 3).
3. Helper bytes repeated per TS file on Sovereign's TS (item 5).
4. Enum-table bytes before and after on `telemetry.gen.cpp` and Sovereign's `text/vocab` (item 1).

## 4. Not in v0.3

- **One shared runtime per root for C++ and Go.** It was declined in ADR-0022 §10, because each
  output directory is self-contained. It would also be large: 12+ files, and a field on the frozen
  `ir.Emit`.
- **Lookup versus translation** moves to v0.4: it changes WIRE §5.11's `$` keys, the fingerprints,
  and C++ constexpr (DECISIONS 293).
- **Widths from refinements** are refused (GEN-02).

## 5. Acceptance, in outline

- Goldens regenerated and read. The C++ compile suites (g++, clang++, libc++), the Go vet and run
  tests, and tsc and node all green.
- `make realdata-compile` PASS, with peak RSS and times compared against v0.2's.
- The reader's differential fuzz clean for its stated duration.
- The scale bench numbers in the log, and to Louis.
