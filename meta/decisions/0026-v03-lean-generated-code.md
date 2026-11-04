# ADR-0026 — v0.3, lean generated code: every decision, and how the run builds it

Date: 2026-11-02. **Status: accepted** (Louis, 2026-11-02: "Write the ADR for the v0.3.0 … the
orchestrator running it is going to have less effort than you have here"). Scope: Louis's v0.3 of
2026-10-31 ([roadmap](../handoff/2026-10-31-canon-roadmap-v02-v04.md) §4,
[log-2026-10-31](log-2026-10-31.md)), as outlined in [ADR-0024](0024-v03-lean-generated-code-outline.md),
which this ADR supersedes; plus the owed items v0.2's reply lists
([reply](../handoff/2026-11-02-canon-reply-v020.md) "What stays owed").

Designs are the orchestrator's (technical calls are its own, DECISIONS 207). They come from ten
read-only research passes over `main` at `9ed956e6` (v0.2.0 plus docs commits) and over Sovereign's
real output, built for this ADR with the installed canon 0.2.0 into `/var/tmp/canon-030-measure/`
(Resource `5a80f78a`, Source `d01a793e`: 142 packages, 0 findings, 47 s, 2.5 GB peak; 118 `.gen.cpp`
files of 414k lines, 134 `.gen.h` of 307k, 185 `.gen.go` of 809k, 54 TS files of 3.1 MB in five
output directories, 28 copies of `canon_runtime.h`).

This ADR amends DECISIONS 19, 148, 229, 261, 290, 292, 341, 351, 382, 383, 384, 390, 392, 393, 403,
408, 412, 415, 419 and 422, and ADR-0004.

The successor is an outline: [ADR-0025](0025-v04-formulas-outline.md) (v0.4).

## 0. How to run this ADR

The run is executed by an orchestrator working at medium effort. This section is its rulebook. Read
it twice, then §17.1, and keep both open.

- **The ADR is decision-complete.** The run builds what this ADR says and never redesigns.
  - Each *Decision* binds.
  - Every "Spec text" block is final wording, applied verbatim by the spec sync (§17.2, S1), except
    the blocks a test compares byte for byte with code, which land with the unit that changes that
    code (§17.2 lists them).
  - Every "ERRORS.md" block is applied verbatim by S2, or by the orchestrator in the commit of the unit
    §17.2 names for it.
  - A builder that proposes another design is thanked. Its proposal is logged as owed in
    `meta/state.md`, and it builds what is written here.
- **DECISIONS numbers.** 430-452 are the next free numbers on `9ed956e6`, where 429 is the last.
  - If one is taken when the run starts, shift every number of this ADR by the same offset and log
    it.
  - Inside the ADR, `D-<tag>` in a quoted block means "the number §1's table gives that tag". The
    spec sync writes the number, never the tag.
  - A decision the run itself makes (a new bug's ruling, a gap) takes the next free number after
    this ADR's last, as v0.2's 423-429 did.
- **File:line references** are from `9ed956e6`. They locate; they do not bind. A reference that no
  longer matches means the code moved: find the construct by name.
- **Measured numbers** come from the tree above. Release steps re-measure on the day's trees; a
  number that moved since is not a bug, but a number this ADR uses to decide something (§2) is
  re-checked by the unit that builds on it, and a reversal stops that unit.
- **Readings.** Several research passes ran no probe. A "Today" claim marked `[unverified]` is a
  reading of the code.
  - **The failing-test-first rule settles every reading.** Each unit writes its tests first and shows
    that they fail on unchanged production files. Copy the files aside and back; never `git stash`.
  - A test that does not fail means the reading was wrong. **The unit stops and reports**, and the
    orchestrator decides under the fallback below.
  - **Except pins.** A test this ADR calls a pin passes on unchanged files by design; its section says
    how to prove it can fail.
  - **Except equivalence tests.** Most v0.3 items change how generated code is written, not what it
    does. Their tests are "same behaviour, new shape": the old and the new output give identical
    results on the same inputs (run tests, round trips, the differential fuzz of §4). Such a
    test passes before and after by design; what fails first is the shape assertion beside it (a
    size, a table count, a line budget), and the section names it.
- **Fallback, for a small question this ADR does not answer.** A real design question is a gap; see
  the next rule. Otherwise, in order:
  1. A spec document that answers it wins (DOCTRINE §2).
  2. Otherwise take the reading that refuses more, writes less and keeps today's behaviour for
     everything this ADR does not name (strictest option, DECISIONS 25). For generated code, "keeps
     today's behaviour" means the generated program's observable results, its API and its messages
     stay byte-identical.
  3. Log the question, the readings and the choice in `meta/decisions/log-<date>.md`.
  4. Never widen the scope.
- **A real gap** is two readings that change behaviour, neither stricter. It stops that unit only.
  The orchestrator decides it, logs both passages and the choice, and the rest of the run goes on.
- **A new bug found during the run** (v0.2 met fifteen, log-2026-10-31 and log-2026-11-01). Decided
  now, as v0.2 decided it on its second day ("wave G"):
  - A bug a user meets as a wrong verdict, generated code that does not compile or computes a wrong
    value, a refused promised edit, a decoder that accepts what the spec refuses (or the reverse), a
    cross-target difference in a decoder's verdict or bytes, or a compiler crash, is fixed in v0.3:
    a read-only recon locates it, then a new unit fixes it, failing test first, in the package's
    queue. Its ruling takes the next DECISIONS number when it changes the spec.
  - Anything else (a safe over-wrap, a duplicated finding the total order merges, a shape no valid
    program has, memory growth, a message's wording) is logged as owed in `meta/state.md`.
  - Either way, no unit fixes a bug outside its section on its own initiative: it reports, and the
    orchestrator decides.
- **Philosophy questions only go to Louis**, in a file under `meta/handoff/`. The run continues on
  everything the question does not block.
- **Four calls are flagged for Louis.** None blocks the run; each default is the design as written. The
  status file states them until he answers; an answer that changes one stops only the units it names.
  1. **Item 3, header weight, is dropped on its numbers** (§5): the gain is about 1-2% of a consumer
     build. If Louis wants it anyway, a later ADR writes the helper-declaring form; v0.3 does not wait.
  2. **Item 4's views of baked data move to v0.5** (§6): building them now needs DECISIONS 323 reopened
     (Louis accepted it on 2026-10-06). v0.3 builds the sorted lookups.
  3. **`canon lint` ships eight lints, the roadmap's nine and the define lint minus two** (§12): "an integer
     field holding only 0 and 1" (half its Resource findings are counts, and its advice would change a type) and
     "the `{k: true for …}` idiom" (Canon's only membership form) are decided out.
  4. **The scale bench's numbers go to Louis for Q3** (§13), the parked on-disk cache, with the release.
- **What a medium-effort orchestrator most often gets wrong** (v0.1.6 to v0.2):
  - accepting "it passed" without the log, or landing on a log whose last line looks fine but that
    holds `exit=2` (v0.2, EN-TS);
  - chaining the next cherry-pick after a failed guard with `;` (v0.2, OUT-PATH);
  - letting two builders touch one package;
  - regenerating goldens without reading the diff;
  - forgetting the memory cap in a VERIFY line, or fuzzing at the default parallelism (v0.2: 19 of 28
    fuzz targets OOM-killed);
  - telling Louis units run without checking `ListAgents`;
  - redesigning when a test is inconvenient;
  - running a long gate for the first time at release (v0.2: progen-nightly, unrun for a month, found
    15 classes and 2 crashers on the release day).

  §17.1 lists the rule against each.

## 1. Context and scope

- **Scope.** ADR-0024's twelve items (Louis, 2026-10-31: lean generated code; no language change; every
  change checked by goldens and by the compile and run tests of the generated code), each designed here
  from the code of `9ed956e6` and measured on Sovereign's real output. Plus every item v0.2's reply left owed,
  ruled one by one under the standing rule (every known bug is fixed as found), and the spec wording v0.2's
  logs left unwritten.
- **What the measurements changed** (§2): item 3 (header weight) is dropped on its numbers; item 4's views and
  item 12 (presence bits) move to v0.5's record layout, and item 4 keeps its safe half (sorted constant
  lookups); item 5's runtime module is declined on its numbers; item 6 finds a growing function the outline
  missed (Go `decode<T>`); the research found three bugs that predate v0.3 (a C++ data loader accepting
  bytes after a NUL, §4; a long-lived Project growing with every edit, §15.1; a record-literal
  default escaping its check, §14.3).
- **Release:** `v0.3.0`. The language version stays `canon: "0.1"` (§18, 452).
- **Robustness before speed (Louis, 2026-10-14) holds.** v0.3 makes generated code smaller and faster, never
  the compiler: no cache, no memo shared across worlds, no parallel stage, no partial re-check. Every speed
  item is proven equivalent (goldens, run tests, a differential fuzz against v0.2.0's reader).
- **v0.3 is larger than the roadmap's "~15 units, about a day":** 34 units, because the owed list holds eleven
  real fixes and three bugs were found while planning, as v0.2 grew past its estimate for the same reasons.

| # | Item | § | DECISIONS | Units (§17) |
|---|---|---|---|---|
| 1 | An enum's strings are written once (C++, Go) | 3 | 430 (D-ENUMTAB) | ENUM-0, ENUM-CPP, ENUM-GO |
| 2 | The C++ JSON runtime reads a file in one pass; a NUL after a data document is refused | 4 | 431 (D-READER) | READER |
| 3 | Header weight: measured and dropped | 5 | none | none (S1 fixes CODEGEN §7.1) |
| 4 | Baked containers find in sorted constant tables; views of baked data wait for v0.5 | 6 | 432 (D-BAKEDFIND) | BAKED-FIND |
| 5 | Top-level TS values are marked pure; the helper block stays per file | 7 | 433 (D-PURE), 434 (D-TSHELPERS) | TS-PURE |
| 6 | Generated functions stay within 1,000 lines | 8 | 435 (D-SIZE) | FNSIZE, IR-PARTS, GEN-CPP-PARTS, GEN-GO-PARTS, BUDGET |
| 7 | C++ runtime copies carry a content stamp | 9 | 436 (D-RTSTAMP) | CPP-RT |
| 8 | C++ `Decode` takes an `nlohmann::ordered_json` too; a constant of a variant's kind is written | 10 | 437 (D-ORDERED), 438 (D-KINDCONST) | CPP-RT (runtime), CPP-ORDERED, KIND |
| 9 | N8: output templates | 11 | 439 (D-TPL) | TPL-PROJECT, TPL-CHECK, TPL-DOCS |
| 10 | N3: `canon lint` | 12 | 440 (D-LINT) | LINT-CORE, LINT-DATA, LINT-API-CLI |
| 11 | The scale bench; the edit bench re-based | 13 | 441 (D-BENCHEDIT) | SCALE-BENCH |
| 12 | Presence bits: decided out (v0.5) | 6 | none | none |
| 13 | `E8005` names the way out that works | 14.1 | 442 (D-HINTS) | HINTS |
| 14 | Target annotations on a local are `E1118` | 14.2 | 443 (D-LOCALANN) | LOCAL-ANN |
| 15 | A record-literal default is checked | 14.3 | 444 (D-DEFAULTREC) | DEFAULT-FOLD |
| 16 | Owed generator items decided out | 14.4 | none | S1 |
| 17 | A long-lived Project does not grow with its edits | 15.1 | 445 (D-SNAPPIN) | WS-SNAP |
| 18 | Unstored dependent literals judged, once | 15.2 | 446 (D-RECLIT) | EVAL-DEP |
| 19 | Verification's `where` runs are free | 15.3 | 447 (D-WHEREFREE) | S1 |
| 20 | An error outside every `expect` fails its test | 15.4 | 448 (D-TESTSOFT) | TEST-SOFT |
| 21 | `canon test` reports its own warnings | 15.5 | 449 (D-TESTWARN) | TEST-CLI |
| 22 | A String-keyed keyed list's word key starts an amend line | 15.6 | 450 (D-KEYEDLINE) | EDIT-KEYED |
| 23 | writelock names no absolute path | 15.7 | none | WRITELOCK |
| 24 | The first build refuses what the next would | 15.8 | 451 (D-FEEDBACK) | FEEDBACK |
| 25 | Owed core items decided out | 15.9 | none | none |
| 26 | Test tidying | 15.10 | none | TIDY |
| 27 | Spec wording v0.2 left unwritten | 16 | none | S1 |
| 28 | Language version stays 0.1 in compiler 0.3 | 18 | 452 (D-LANGVER) | S1 |
| 29 | Release | 18 | none | orchestrator |

**Not in v0.3** (ADR-0024 §4, unchanged): one shared runtime per root for C++ and Go (each output directory is
self-contained; ADR-0022 §10); lookup versus translation (v0.4: it changes WIRE §5.11's `$` keys, fingerprints and
C++ constexpr, DECISIONS 293); widths from refinements (refused, GEN-02).

## 2. What was measured first (ADR-0024 §3)

Every number is from the measurement tree (`/var/tmp/canon-030-measure/`, canon 0.2.0) unless stated.

| Measurement | Result | What it decided |
|---|---|---|
| Header-use classes of the 225 cross-package `#include`s (item 3) | 194 name only enum types (86%), 13 a record in declarations, 3 constexpr cells, 15 complete records. But generated headers are 21.6% of the generated units' preprocessed input (the standard library 67%, nlohmann 8%); the best forward-header form saves 5.7% of their front-end CPU, about 1-2% of a consumer build | item 3 dropped (§5) |
| Helper bytes repeated per TS file (item 5) | 1.95% of all generated TS; at most 6.3% of one directory; at most 3.2% of a minified, gzipped bundle | the runtime module declined, at a 10% threshold (§7) |
| The PURE marks on a real bundle (item 5) | Vite 7.3 production build reading one constant per module: admin 336 KB → 842 B, monitoring 545 KB → 852 B | the marks, on every call of a chain (§7) |
| Enum-table bytes before and after (item 1) | `.gen.cpp` 16.4 → 7.5 MB (413,577 → 244,567 lines); `text/vocab.gen.cpp` 4.56 → 1.00 MB; Go 40.7 → 31.3 MB; 32,335 macro guard triples leave the `.gen.cpp` files | item 1 as designed (§3) |
| Baked heap copies (item 4) | the 33k define names are a record field built by every mode (DECISIONS 323, 401); the copy costs 27,846 allocations, 2.05 MB, about 1.2 ms at first use | views wait; sorted lookups built (§6) |
| Presence bits (item 12) | about 2.7 MB of loaded data saved, nothing on baked data; a new silent-failure class | decided out (§6) |
| The one-pass reader (item 2) | 234,769 generated inputs, 0 differences from v0.2.0's reader; 1.6-2.4× faster on Sovereign's largest generated reads | item 2 as designed (§4) |
| Generated function sizes (item 6) | Go `build` up to 16,181 lines (17.3 s and 1.5 GB to compile at 10,000 rows; 1.7 s and 0.45 GB cut); Go `decode<T>` 10.9 lines per field; C++ `Decode` at most 773 lines in Sovereign | the 1,000-line budget (§8) |
| The scale bench's calibration (item 11) | 1× = 15,000 benchgen entries matches Resource's source bytes and check memory (1.97 GB); estimates 3× 5.8 GB, 5× 9.6 GB | the bench and its 12G cap; the real numbers come from the run (§13) |

## 3. An enum's strings are written once (DECISIONS 430; amends 383, 393)

Scope: ADR-0024 item 1. Research: one opus pass over `internal/gen/cpp`, `internal/gen/go` and their
name plans in `internal/ir`, Sovereign's 116 C++ pairs and 185 Go files. Its after-numbers come from a
model of the new text that reproduces all 407 of today's C++ helper sets byte for byte; the rewritten
`enum-helpers`, `resource/vocab`, `telemetry` and `shared/core` goldens compile warning-free under
g++ 15.2 and clang++ 21.1, and the real `testdata/main/helpers_main.cpp` prints exactly `helpersWant`
against them.

### Facts

**C++ today.**
- `closedDefinitions` (`internal/gen/cpp/enums_tables.go:107-127`) writes `ToName` (`:115`, function-local
  `kNames`), `ToWire` (`:116`, `kWires`), `IsRetired` (`:117`, `kRetired`), `<E>FromWire` (`:118-123`,
  `kKeys`, a sorted copy of the wires, and `kValues`, the enumerators in that order) and, with `@codes`,
  `<E>FromCode` (`:124-126`). All are function-local `static constexpr` (`text/enum_by_ordinal.txt:2`,
  `text/enum_retired.txt:2`). Variant kind enums take the same path (`enums.go:33-46`, `:75-86`).
- An id enum (`idenums.go:61-81`) has a local `kWires`, and `<Rec>IdFromWire` a local `kKeys` (sorted
  copy) and `kIds`. Branch enums have no helpers (`dependent.go:36-52`).
- Open enums keep one sorted wire table at namespace scope (`openWireTables`, `enums_tables.go:157-169`:
  `detail::k<E>Wires`, `k<E>WireNames`, `k<E>WireCodes`), but their `<E>FromCode` still writes a second
  local string table (`:148-153`).
- `@codes` ordinals: `detail::k<E>Codes`, `k<E>CodeOrdinals` (always `uint32_t`, `enums.go:154`) and the
  `inline constexpr` lower-bound `<E>Ordinal` (`enums.go:144-161`), which returns `k<E>Codes.size()` for
  a non-member (`:157`). Without `@codes` the ordinal is a `static_cast` (`enums_tables.go:110`).
- **"n for a non-member" is relied on only as "≥ n":** `stored.go:110-121` aborts on `iK >= N`;
  `ToName`/`ToWire` test `i >= size`, `IsRetired` `i < size`, `<E>FromCode` `i >= k<E>Members.size()`.
  "Exactly n" is fixed by CODEGEN §5.2 (`spec/CODEGEN.md:1075`) and pinned by
  `testdata/main/helpers_main.cpp:18` and `helpersWant`'s `ordinal=3/0` (`helpers_test.go:107-110`).
- **`kValues` and `kIds` are the only places a `.gen.cpp` names every enumerator,** so DECISIONS 404's
  guard scan wraps every `@cpp(defines:)` enumerator there: Sovereign's `.gen.cpp` files hold 32,335
  push/`#undef`/pop triples (97,005 lines); `text/vocab.gen.cpp` alone 37,077 of its 98,938 lines.
- GCC resolves each scoped-enumerator reference by a linear scan (60,000 references into a 60,000-member
  enum cost +12.2 s); `kValues` and `kIds` are such arrays.

**Go today** (DECISIONS 393). `enum<Role>_<Type>` tables: `enumCodes_` (`@codes`), `enumNames_`,
`enumWires_` only when a wire differs from its name (`internal/gen/go/enums.go:187-191`: Go already
reads the names otherwise), and `writeParse` (`:235-244`) writes `enumKeys_` (a sorted copy of the
names or wires) plus `enumFound_` (the members in that order). Ids and branches get Names, Keys and
Found (`ids.go`). Opened enums (`enums_open.go:8-27`) keep Keys, Names, Numbers, Codes, ByCode and
Members, three `[n]E` tables of the same strings. Plan roles: `internal/ir/goplan_tables.go:16-36`.

**Measured on Sovereign** (the measurement tree):
- C++: 412 enum classes (263 `@codes`, 124 closed without codes, 10 kind, 10 id, 5 branch, 0 opened).
  Wires equal names in 385 of 397 (the outline's 28/31 and 332/340 are now 27/30 in the example
  goldens, 37/47 in `gen/cpp` testdata, 385/397 in Sovereign). Of the 263 `@codes` enums, 217 have
  codes dense in declaration order (2,217 members; the first code is 0 for 115, 1 for 95); 46 are
  sparse (TextId: 12,359 members over a span of 17,891; ItemId 7,328 over 40,042).
- C++ size: the `.gen.cpp` files are 413,577 lines and 16.4 MB as the research's model parses them (`wc -l`
  counts 414,094), of which enum helper functions are
  196,397 lines (47.5%) and 8.2 MB. `text/vocab.gen.cpp` (one sparse `UInt16` enum) is 98,938 lines and
  4.56 MB. Sovereign's real `telemetry.gen.cpp` is 4,234 lines, 4,168 of them enum helpers (the outline's
  "1,016 of 1,734" is the example golden: now 1,098 of 1,734).
- Go: 1,055 enum types (527 distinct); their tables are 211,030 lines and 16.9 MB, 15.5 MB of it in opened
  enums. `enumKeys_` is a sorted copy of the names in 1,001 of 1,001 closed copies; `enumNames_` equals
  the wires in all 54 opened copies.
- **A permutation sorted in constant evaluation does not compile at scale:** a constexpr heapsort of
  60,000 wires hits clang's step limit (2.3 s) and GCC's "operation count exceeds limit of 33554432"
  (24.9 s, 2.48 GB). The generator must write the order as integer literals.

**Plan coupling.** `TestCppNamePlanMatchesGeneratedCpp` (`internal/ir/cppplan_test.go:24`) and
`TestGoNamePlanMatchesGeneratedGo` (`goplan_test.go:24`; also `goplan_modes_test.go:32`) require the
generated files to declare exactly the plan's names, scope by scope. Closed-enum tables are not in
the C++ plan today; opened ones are (`cppplan_lookups.go:64-76`). A plan change lands in the same
commit as its generator change.

### Problem

Each enum string is written three times in C++ (names, wires, a sorted copy) and twice in Go, and C++
names every enumerator once more in `kValues`, which makes DECISIONS 404 wrap tens of thousands of
macros. A dense `@codes` enum binary-searches codes that a subtraction would find.

### Decision

1. **C++: one table of names, one of wires only when they differ, and an order of integers.** For a
   closed enum, a kind enum and a table's id enum, the `.gen.cpp` holds, before the enum's helpers, in
   `namespace detail { namespace {`:
   - `k<E>Names`, the Canon names in declaration order (not for an id enum);
   - `k<E>Wires`, the wires in declaration order, only when one differs from its name (always for an
     id enum);
   - `k<E>ByWire`, the ordinals sorted by wire bytes (stable: the first declared of equal wires first,
     as 383 and 406 require), in the smallest of `uint8_t`, `uint16_t`, `uint32_t` holding every
     ordinal, written by the generator as literals.

   Strings take DECISIONS 392's form (sized views from 1,024 entries). These names are in the C++ name
   plan (`detail` scope), so E8005 covers them.
2. **The helpers:** `ToName` indexes `k<E>Names`; `ToWire` indexes `k<E>Wires`, or `k<E>Names` when the
   wires are the names; `<E>FromWire` and `<Rec>IdFromWire` binary-search `k<E>ByWire`, comparing the
   wire each entry indexes (`text/lower_bound.txt` generalised to take the size, an element accessor
   and the hit; its existing callers stay byte-identical); the hit is `static_cast<E>(k<E>ByWire[lo])`,
   or `k<E>Members[k<E>ByWire[lo]]` with `@codes`. `IsRetired` and a closed `<E>FromCode` are
   byte-identical.
3. **Dense `@codes`:** when the codes are dense in declaration order (one member or more, each code the
   previous plus one, never wrapping), the header's ordinal is
   ```cpp
   namespace detail {
   inline constexpr size_t ElementOrdinal(Element v) {
       const uint8_t code = static_cast<uint8_t>(v);
       const uint64_t i = static_cast<uint64_t>(code) - static_cast<uint64_t>(1);
       return i < 5 ? static_cast<size_t>(i) : 5;
   }
   }  // namespace detail
   ```
   without `k<E>Codes` and `k<E>CodeOrdinals` (the first code written as an integer literal,
   `canon::kIntMin` for the minimum; the two casts make a negative code exact and modular in C++17).
   Sparse `@codes` keep 383's search, with `k<E>CodeOrdinals` in the smallest unsigned type above. An
   empty `@codes` enum keeps today's search.
4. **No direct table over sparse codes.** It would add a third ordinal form whose threshold flips when
   one far code is added, for 3.9% of header lines.
5. **A non-member's ordinal stays the member count** (the dense form clamps), so one contract holds for
   both forms and stays testable by equality.
6. **C++ opened enums:** `k<E>WireNames` is written (and planned) only when a name differs from its
   wire, and `ToName` is then `return ToWire(v);` otherwise; `<E>FromCode` keeps its local sorted codes
   and finds positions in `k<E>Wires` through a local `kWireAt` instead of a second string table.
7. **Go:** for a closed enum, a kind, an id and a branch enum, `enumKeys_` and `enumFound_` give way to
   `var enumByWire_<T> = [...]uintN{…}`, the indexes of the names' order (code order with sparse
   `@codes`) sorted by wire (by key for ids and branches), searched with `slices.BinarySearchFunc` and
   `strings.Compare`; a dense `@codes` enum has no `enumCodes_`, and `String`, `Wire`, `Retired` and
   `<E>FromCode` index by `uint64(int64(self) - F)`. An opened enum keeps `enumKeys_` as its one member
   table; `enumMembers_` and, when its codes are not dense, `enumByCode_` become `[...]uintN` indexes
   into it, and `enumNames_` exists only when a name differs from its wire (`String` returns the wire
   otherwise). `GoTableFound` becomes `GoTableByWire` (`"enumByWire"`).
8. **The API and every answer are unchanged** in both targets. TS is untouched (another section's).

### Code

- **ENUM-0** (`internal/ir`): a new `enum_tables.go` with `DenseCodes(e) (first int64, ok bool)` (at least
  one member, each code the previous plus one, `prev != math.MaxInt64` so it never wraps),
  `EnumWiresAreNames(e)` and `CaseWiresAreNames(v)` (vacuously true with no member), replacing the
  tests at `goplan_tables.go:23` and `goplan_scopes.go:223` (ENUM-GO replaces the one at
  `internal/gen/go/enums.go:187`).
- **ENUM-CPP:** `internal/ir` `cppplan_lookups.go` (`CppEnumHelpers` gains `Names` and `ByWire`; a new
  `declareEnumTables`; `declareOrdinals` only when the codes are not dense; `declareWireTables`'s
  `WireNames` only when needed) and `cppplan_inner.go`; `internal/gen/cpp` `enums_tables.go`, `enums.go`,
  `idenums.go`, `text/lower_bound.txt`, `text/enum_by_ordinal.txt`, a new `text/enum_dense_ordinal.txt`.
  New ir and gen/cpp constants go in each package's `errors.go` (§17.1's room rule); the unit
  deletes the gen/cpp locals (`kNames`, `kWires`, `kValues`, `kIds`) that become unused.
- **ENUM-GO:** `internal/ir` `goplan_tables.go`, `goplan_imports.go` (`goStrings` marked where `slices` is
  for closed enums, variants, dependent types and baked tables, never for opened enums),
  `goplan_dependent.go`, `goplan_scopes.go`, and the `GoTableFound` line of `constants.go` (same line
  count); `internal/gen/go` `enums.go`, `enum_tables.go`, `enums_open.go`, `ids.go`, `text/enums.txt`,
  `members.txt`, `live_members.txt` (an item expression; closed enums pass `m`, byte-identical).

### Spec text

CODEGEN §5.2, the Go paragraph (`spec/CODEGEN.md:1043-1045`) becomes:
> These methods read private package-level tables `enum<Role>_<Type>`, by index or by binary search, so no generated method grows with the member count (DECISIONS 393, D-ENUMTAB). A closed enum's tables are its names (declaration order; code order with `@codes`), its wires only when one differs from its name (`Wire` reads the names otherwise), its members, its codes sorted by value unless they are dense, and `enumByWire_<Type>`, the indexes of the names' order sorted by wire bytes (by key for an id and a branch enum), which `Parse<E>` searches. With codes dense in declaration order (one member or more, each the previous plus one), `String`, `Wire`, `Retired` and `<E>FromCode` index by `code - first`, computed in `uint64`. An opened enum's one member table is `enumKeys_<Type>`, its members sorted by wire, with its codes and, only when a name differs from its wire, its names in that order (`String` returns the wire otherwise); `enumMembers_<Type>` and, with codes that are not dense, `enumByCode_<Type>` beside the sorted codes hold indexes into it. Keys and wires are unique (WIRE.md §4.2, TYPES.md §9.3), so each search finds one member.

CODEGEN §5.2, the C++ sentences from "Each definition reads tables" to "A variant's kind enum (§5.5)
follows the same rules." (`:1066-1076`) become:
> Each definition reads tables, never a chain or a switch over the members, and writes each string once (DECISIONS D-ENUMTAB). Before an enum's helpers the `.gen.cpp` holds, in `namespace detail { namespace {`, `k<E>Names` (the Canon names in declaration order), `k<E>Wires` (the wires in declaration order, only when one differs from its name) and `k<E>ByWire` (the ordinals sorted by wire bytes, the first declared of equal wires first, in the smallest of `uint8_t`, `uint16_t`, `uint32_t` holding every ordinal); strings take DECISIONS 392's form. `ToName` and `ToWire` index `k<E>Names` and `k<E>Wires` (`k<E>Names` when the wires are the names) by the member's ordinal, `IsRetired` an array of flags; `<E>FromWire` binary-searches `k<E>ByWire`, comparing the wires it indexes, as `<Rec>IdFromWire` does (§5.3); `<E>FromCode` goes through the ordinal. A value that is no member gives `""`, `false` or `std::nullopt`. A member's ordinal is its declaration position: `static_cast<std::size_t>(v)` without `@codes`; with them, `detail::<E>Ordinal(v)`, an `inline constexpr` function of the header returning the member count for a value that is no member. With codes dense in declaration order (one member or more, each code the previous plus one) it returns `static_cast<uint64_t>(code) - static_cast<uint64_t>(<first code>)` when that is below the member count, else the member count; for codes that are not dense it searches `detail::k<E>Codes` (the codes, ascending) and returns the matching `detail::k<E>CodeOrdinals` entry (in the smallest unsigned type above). The generator computes every order; none is sorted in constant evaluation. A variant's kind enum (§5.5) follows the same rules.

CODEGEN §5.2, the opened-enum C++ sentence (`:1142-1144`) becomes:
> `ToWire` and the `std::hash` specialisation are inline in the header; `IsKnown`, `IsRetired`, `<E>FromWire` and `ToCode` are defined in the `.gen.cpp` as binary searches over `detail::k<E>Wires`, the member wires sorted by bytes, with the parallel `k<E>WireCodes` and `k<E>WireNames`, the latter only when a name differs from its wire; `ToName` searches it, or returns `ToWire(v)` when every name is its wire. `<E>FromCode` searches the codes, ascending, beside their members' positions in `k<E>Wires` (DECISIONS 383, D-ENUMTAB).

CODEGEN §5.3, in the C++ cell (`:1176`), the substring "`ToWire(StatusId)` (an array by position, in
`.gen.cpp`, DECISIONS 383), `StatusIdFromWire(std::string_view)` (binary search, in `.gen.cpp`)" becomes:
> `ToWire(StatusId)` (`detail::kStatusIdWires` by position, in `.gen.cpp`, DECISIONS 383), `StatusIdFromWire(std::string_view)` (a binary search of `detail::kStatusIdByWire`, the positions sorted by key, in `.gen.cpp`, DECISIONS D-ENUMTAB)

(the rest of the cell, `enum class StatusId…`, `IsRetired` and the deleted pair, is unchanged).

CODEGEN §7.1, the definitions sentence becomes "… enum and id-enum lookups (`ToName`, `ToWire`,
`FromWire`, `FromCode` and an open enum's `IsKnown` and `ToCode`) with the tables they read (§5.2) and
`detail::<P>Access`. Getters, `k<E>Members`, `detail::<E>Ordinal` (with its two arrays when the codes are
not dense), an open enum's `ToWire`, …" (the rest unchanged).

CODEGEN §2.7: in item (2), "each closed `@codes` enum followed by its `namespace detail {
k<E>Codes, k<E>CodeOrdinals, <E>Ordinal }` block (§5.2)" becomes "each closed `@codes` enum followed by its
`namespace detail { … <E>Ordinal }` block (§5.2: `k<E>Codes` and `k<E>CodeOrdinals` first, unless the codes
are dense)" ("enums, as above," stays); and in the `.gen.cpp` order, "the enum helper definitions (§5.2; enums
in the order of (2), an opened enum's one sorted wire table in an unnamed namespace before its helpers), each
table's id-enum `ToWire` and `<Rec>IdFromWire` (§5.3)" becomes "the enum helper definitions (§5.2; enums in
the order of (2), each enum's tables, an opened enum's one sorted wire table included, in an unnamed namespace
of `detail` before its helpers), each table's id-enum tables, `ToWire` and `<Rec>IdFromWire` (§5.3)".

CODEGEN §3.5 (`:862`), after "Go's private enum tables `enum<Role>_<Type>` (§5.2)", add: ", C++'s enum
tables `detail::k<E>Names`, `k<E>Wires`, `k<E>ByWire`, `k<E>WireNames`, `k<E>WireCodes` (§5.2)".

### DECISIONS text

> D-ENUMTAB. **An enum's strings are written once (CODEGEN.md §2.7, §3.5, §5.2, §5.3, §7.1; amends 383, 393).** A generated enum holds one table of its Canon names and, only when one differs from its name, one of its wires; every other order is a table of indexes the generator sorts, never constant evaluation (a constexpr sort of 60,000 wires passes neither clang's step limit nor GCC's operation limit). C++: a closed enum, a kind enum and a table's id enum keep `detail::k<E>Names` (not for an id), `k<E>Wires` and `k<E>ByWire` (the ordinals by wire bytes, stable, in the smallest of `uint8_t`, `uint16_t`, `uint32_t` holding them) in an unnamed namespace of the `.gen.cpp`, in the name plan; `ToWire` reads the names when the wires are the names; `<E>FromWire` and `<Rec>IdFromWire` search `k<E>ByWire`. An opened enum writes `k<E>WireNames` only when a name differs from its wire (`ToName` is then `ToWire`), and its `<E>FromCode` finds positions in `k<E>Wires`. A closed `@codes` enum whose codes are dense in declaration order (one member or more, each code the previous plus one) has `detail::<E>Ordinal` compute `code - first` in `uint64_t`, the member count past the last, without `k<E>Codes` and `k<E>CodeOrdinals`; other `@codes` enums keep 383's search, `k<E>CodeOrdinals` in the smallest unsigned type. Go: `Parse<E>` of a closed enum, kind enum, id and branch enum searches `enumByWire_<T>`, indexes into the names' order, instead of `enumKeys_` and `enumFound_`; a dense `@codes` enum indexes by `code - first` in `uint64` and has no `enumCodes_`; an opened enum's one member table is `enumKeys_`, `enumMembers_` and `enumByCode_` index it, and `enumNames_` exists only when a name differs from its wire. Generated APIs and their answers are unchanged; a non-member's ordinal stays the member count. Reason: Sovereign's `.gen.cpp` held each enum string three times and named every enumerator once more (`kValues`), so DECISIONS 404 wrapped 32,335 macros there; after: about 414,000 → 245,000 lines and 16.4 → 7.5 MB of `.gen.cpp`, 40.7 → 31.3 MB of Go, `text/vocab.gen.cpp` 4.56 → 1.00 MB, g++ 3.1 → 1.7 s on it.

"[amended by D-ENUMTAB]" marks go on 383 and 393 (beside their existing 406).

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| ir | `TestDenseCodes` (cites CODEGEN §5.2) | dense for [6000, 6001, 6002], [−128, −127], [MinInt64, MinInt64+1], [MaxInt64−1, MaxInt64] and one member; not dense for none, [1, 3], [2, 1], [MaxInt64, MinInt64] and an enum without `@codes` | fails: no function |
| ir | `findings/E8005_<next>.txtar` | an opened `Stat` beside a closed `StatWire` collides on `kStatWireNames`: the new tables go through the plan | fails first |
| gen/cpp | `TestEnumStringsWrittenOnce` (cites §5.2, §5.3, D-ENUMTAB) | each closed, kind and id wire literal appears once in the `.gen.cpp`; `k<E>ByWire` exists; no local `kNames`/`kWires`/`kKeys`/`kValues`/`kIds`; an opened `FromCode` holds no string table | fails: each literal appears three times |
| gen/cpp | `TestDenseOrdinalHeader` | dense enums (`Int8` from −128, `UInt16` from 6000, `Int` [MaxInt64−1, MaxInt64]) get the `uint64_t` lines and no arrays; sparse [MaxInt64, MinInt64] and an empty `@codes` enum keep the search; `k<E>CodeOrdinals` takes the smallest type | fails first |
| gen/cpp | `TestDenseOrdinalCompileAndRun` (new `testdata/main/dense_main.cpp`, golden dir `enum-dense`) | `static_assert`s: the ordinal equals the position for members and the member count for codes below the first, past the last, and at the type's minimum and maximum; `ToName`, `ToWire`, `IsRetired`, `FromCode`, `FromWire` on members and non-members; a constexpr lookup over the dense enum from a second package; every compiler and mode | a pin (the values equal today's search): prove it can fail by shifting the offset by one in a scratch copy |
| gen/go | `TestDenseCodesIndex` | no `enumCodes_` for a dense enum, kept for a sparse one; smoke on members and non-members | the text part fails first |
| gen/go | `TestOpenEnumTablesOnce` | no `enumNames_` when names equal wires, and `String` returns the wire; `enumMembers_` and `enumByCode_` are `uintN` tables; smoke on `Members` order and `FromCode`, dense and sparse | the text part fails first |
| gen/go | `TestEnumEdgeCases`' text checks, extended | `enumByWire_` present, `enumKeys_`/`enumFound_` absent for closed enums | fails first |
| both | unchanged and green | `TestEnumHelpersCompileAndRun` (`helpersWant` byte-identical), `TestEnumHelpersOnlyDeclaredInHeader`, `TestRetiredLookupsDeclaredAndDefined`, `TestIDEnumHelpersOnlyDeclaredInHeader`, `TestEnumHelpersSize`, `TestBigEnumTablesCompile`, `TestImportedCodedLookupsCompileAndRun`, `TestEnumEdgeCases`, `TestLargeEnumsCompile`, `TestEnumFunctionsDoNotGrow`, the two plan tests | pins |

**Goldens that change** (regenerated, diff read by sampling one closed, one dense, one sparse, one id and
one opened enum, then checking every other hunk is of the same shape): `internal/gen/cpp/testdata`
(23 `.gen.cpp`, the 6 headers that hold `k<E>CodeOrdinals`: dense ones lose both arrays, sparse ones change
its type; types-open); `examples/` C++ (6 `.gen.cpp`, 2 `.gen.h`);
`internal/ir/testdata` cppplan (baked, data, datamore), goplan (data, datamore, entries, enum_tables,
translated, types), planpending (dependent, dependent_collisions); `internal/gen/go/testdata` (about
22); `examples/` Go (11 files and the 2 gotypes ones). Guard lines disappear from `defines/defs.gen.cpp`
and `constructs/shop.gen.cpp`; some files gain `<cstdint>` or the `"strings"` import.

### Acceptance

- The new tests fail on copies of today's files (except the pin) and pass after.
- `make check` green; `TestBigEnumTablesCompile` passes and is faster (the 60,000-member probe: g++
  `-fsyntax-only` 41.8 s and 833 MB → 16.2 s and 431 MB; clang++ 2.1 s → 1.0 s).
- `make realdata-compile` passes; `text/vocab.gen.cpp` and the other vocab units shrink (§18 step 6).

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| ENUM-0 | `internal/ir`: new `enum_tables.go` (+ test), and the predicate calls at `goplan_tables.go:23` and `goplan_scopes.go:223` | sonnet: three pure predicates | XS | S2 |
| ENUM-CPP | `internal/ir` (`cppplan_lookups.go`, `cppplan_inner.go`, the new constants in `errors.go`, the cppplan testdata, `findings/E8005_<next>.txtar`, and `testdata/planpending/dependent.txtar` and `dependent_collisions.txtar`, which dump both plans) and `internal/gen/cpp` | sonnet, opus review (opus if it fails review once): the plan and the generator move together; equivalence is run-checked | M | ENUM-0, HINTS (§14: it chooses `declare`, `declareOwn` or `declareFixed` for the new names) and KIND (gen/cpp) |
| ENUM-GO | `internal/ir` (`goplan_tables.go`, `goplan_imports.go`, `goplan_dependent.go`, `goplan_scopes.go`, the `GoTableFound` line of `constants.go`, the goplan and planpending testdata) and `internal/gen/go` | sonnet: golden- and run-checked | M | ENUM-0, KIND (gen/go) and ENUM-CPP: both rewrite the two `planpending` goldens, so ENUM-GO regenerates them on top of ENUM-CPP's |

The orchestrator applies the spec text in S1 (no test compares it with code).

### Forecast

1. **Name plans** are the most likely review failure: "wires are names" and "dense" must be decided by the
   same `ir` predicates in the plan and the generator, or the plan tests fail.
2. **Keep the sort stable:** fixtures from before DECISIONS 406 (`Dup`, `OpenDup`, `T`) rely on the first
   declared winning.
3. **The Go `strings` import** must be marked exactly where it is used (an unused import does not
   compile).
4. **Do not shadow** `wire`/`key` with the comparison closure's parameter.
5. **Cross-target:** C++ and Go both compute `code - first` modulo 2^64; TS is untouched.
6. **MSVC:** unverified; the new tables are the same class as today's function-local ones (CODEGEN §9
   already marks them unverified at scale).
7. **Sovereign:** 0 consumer sites. The reply states the size drop and that the guard triples leave the
   `.gen.cpp` files (headers keep theirs).
8. The header's `k<E>Members`/`k<E>LiveMembers` keep their GCC cost (about 25 s of the 60,000-member probe
   under g++): that is header weight, dropped (§5).

## 4. The C++ JSON runtime reads a file in one pass (DECISIONS 431; extends 361)

Scope: ADR-0024 item 2. Research: one opus pass over the JSON runtime and nlohmann 3.11.2's source (the
copy the tests use, `Source/External/nlohmann/json.hpp`), with a prototype runtime proven against
v0.2.0's on a deterministic neighbourhood of 234,769 generated inputs
(`/var/tmp/canon-030-research/r-reader/`).

### Facts

- **The pipeline today** (`internal/gen/cpp/runtime/canon_runtime_json.h.txt`, 1,477 lines, unchanged
  since v0.2.0; lines 1-952 unchanged since `4dfd414e`, so the outline's line numbers hold).
  `ParseTextFile` (`:856-889`) runs, in order: the BOM (`:858-859`); `detail::SyntaxScan` (`:430-565`,
  the RFC 8259 grammar, for "invalid JSON at byte N"); `detail::TextScan` (`:703-828`: depth, the 513th
  level failing; per string `Utf8Failure` and `SurrogateFailure`; repeated keys after `DecodeKey`, one
  `std::set<std::string>` per level; number tokens whose `strtod` is infinite, recorded as spans and RFC
  6901 pointers); `detail::StandIn` (`:831-841`, writes `1e308` for each such span); `StrictCheck`
  (`:265-403`) through `Json::sax_parse` (`:872-873`); the DOM parse (`:874`; `:875-881` "Not reached");
  `detail::TakeSidecars` (`:408-420`); empty tokens for the overflows (`:883-887`). Grammar is checked
  three times, UTF-8 twice by TextScan plus twice by nlohmann, and nlohmann lexes the text twice.
- **A beyond-Float64 number is refused at the field, never at the file:** a `Float` field says "number
  out of range" (`Decoder::AsFloat`, `:1057-1063`), an `Int` field "expected an integer…", a `Float32`
  field "expected a number that fits Float32"; an unread key is accepted (CODEGEN §5.13).
- `ParseDataFile` (`:901-932`): a `StrictCheck` SAX pass, then `Json::parse`, any nlohmann refusal
  becoming "`<name>: not a canon data file (invalid JSON)`" (`:910-912`), then the envelope,
  `$schema` and `TakeSidecars`.
- **What nlohmann does** (3.11.2's `json.hpp`): `number_float(number_float_t val, const string_t& s)`
  hands the raw token (`:6638-6644`), so Float32 midpoints can be found in the pass; integers come by
  value and are lossless (a token outside 64 bits arrives as a float with its token); a number beyond
  Float64 is `parse_error` 406 **before any callback** (`:12364-12378`); its UTF-8 ranges equal the
  runtime's `Utf8Step`; lone surrogates are refused (`:7664`, `:7670`, `:7678`); there is no depth limit;
  SAX sees both of two equal keys; **it reads a NUL byte as the end of its input**
  (`:8888-8892`: `case '\0': … return token_type::end_of_input;`, verified); it skips exactly one leading
  BOM (`:8816`). On nlohmann's development branch `json_sax_dom_parser`'s template parameters changed:
  the runtime must never name `nlohmann::detail`.
- **A bug that predates v0.3:** the C++ `ParseDataFile` accepts `{"$schema": "s@1", "rows": []}\0junk`
  (nlohmann stops at the NUL), while Go's `rt.ParseDataFile` refuses it ("not a canon data file
  (invalid JSON)") and WIRE §3.1 calls trailing content a syntax error. `ParseTextFile` is safe
  (SyntaxScan refuses at the NUL).
- **`StrictCheck` is public API** (it lives in `canon::json`, not `detail`; CODEGEN §1.3:58-60): its
  constructors and members stay; additions are allowed.
- **Callers:** `types`-mode `Parse` and `Decode<Fn>File` call `ParseTextFile`, always with orders; `data`
  loaders call `ParseDataFile` (`text/loader_prelude.txt:5`). Sovereign: 844 `ParseTextFile` call sites in
  80 `.gen.cpp` files, 0 `ParseDataFile`; no hand-written code names the scans, `StrictCheck` or
  `TakeSidecars`. The largest file a generated C++ reader parses is `Server/Model/mdlDyna.json`
  (6.1 MB, `ModelFile::Parse`); `Server/Item/propItem.json` (25.2 MB) is read by hand-written nlohmann code
  (`ProjectCmn.cpp:4841-4852`), not by a generated reader.
- **The runtime text checker** (`internal/diag/catalog/runtime.go:37-50`) reads every `Fail("…`,
  `OnEvalError("…` and `canonFail("…` call in a directory named `runtime`; only `make diag-check` runs it
  (v0.2's EMPTY-KEY broke it). Every identifier of the runtime headers is a reserved define name
  (`E8011 reserved`, CODEGEN §7.9).
- **CODEGEN §7.5's second paragraph misreferences** "the byte rules and first-failure messages of WIRE.md
  §8" (WIRE §8 is `emit json`; the byte rules are WIRE §3.1-§3.3).
- **Measured** (best of 9, DOM destruction excluded, g++ 15 -O2, nlohmann 3.11.2, under load):
  `ParseTextFile` on `mdlDyna.json` 85.5-91.3 ms → 52.4-57.3 ms (1.6×); a 3.1 MB `propItem.json` 1.8-1.9×;
  `serverNames.json` 2.2-2.5×; `mdlObj.json`, `EventConfig.json`, `skills.json` about 2×. Peak RSS is
  unchanged (the DOM dominates); the reader TU's `.text` shrinks 14.6% (no `json_sax_dom_parser` is
  instantiated for a file read).

### Problem

A valid file pays two scans, a SAX pass and a DOM parse, so nlohmann lexes it twice and the grammar
and UTF-8 are checked three or four times. The C++ data loader accepts trailing bytes after a NUL.

### Decision

1. **One pass for a valid file.** `StrictCheck` gains an additive constructor
   `StrictCheck(Json& doc, bool keepOrders, bool anyRoot, std::string root)` that builds the document as
   it checks (a `Place` helper, a `node` and `slot` per frame, public nlohmann API only: `emplace`,
   `push_back`, `back`, `object()`, `array()`, all at the 3.9 floor). The existing constructors stay
   check-only and byte-identical.
2. **Repeated keys** are found, in the building pass, by the object's own `emplace` (byte-equal decoded
   keys, the same equality as the `std::set`); the check-only constructors keep the set.
3. **`ParseTextFile`:** when the text holds no NUL byte, one pass over the **whole text, its BOM
   included** (nlohmann skips exactly one; a stripped body would accept a double BOM). On success:
   `doc = std::move(out)`, then `TakeSidecars`, unchanged. On any failure (invalid, or a beyond-Float64
   number, which nlohmann refuses before any callback): today's BOM step, `SyntaxScan`, `TextScan` and
   `StandIn`, unchanged, then the building pass over the stand-in text (one form; `Json::parse` leaves
   file reads). A text holding a NUL skips the pass and goes straight to the scans, which write its
   message.
4. **`ParseDataFile`:** the building pass, then, after a successful pass, a NUL post-check: a NUL anywhere
   is "`<name>: not a canon data file (invalid JSON)`". The post-check keeps document order (a repeated
   key before an inner NUL is still reported first). This is the one verdict change, and it is the bug
   fix above.
5. **Verdicts and messages are identical by construction:** today's verdict on a NUL-free text with no
   beyond-Float64 number is grammar ∧ UTF-8 ∧ surrogates ∧ depth ∧ no repeated key, exactly the
   condition under which the pass succeeds; every other text takes today's scans. Sidecars use today's
   recording and `TakeSidecars` verbatim, so tokens and orders are identical.
6. **A frozen oracle proves it:** v0.2.0's two runtime headers are copied, byte for byte, into
   `internal/gen/cpp/testdata/reader/v0.2.0/` (a directory not named `runtime`, so the text checker does
   not read it), pinned by sha256, and compiled in their own translation unit under
   `#define rt_v1 rt_v1_v020`, linked beside the current runtime. A deterministic enumeration in
   `make check` and a Go fuzz target compare the two readers' verdicts, messages, typed documents (types
   and values, floats as `%a`), tokens and orders.
7. **Not done:** faster sidecars (key orders recorded by node, about 19 ms of 59 on `mdlDyna`): v0.4, with
   its own measurement; a libFuzzer target.
8. **New reserved identifiers:** `Place`, `doc_`, `node`, `slot` (E8011); `is_discarded` disappears. None
   is among Sovereign's 32,398 define names.

### Code (`internal/gen/cpp/runtime`)

`canon_runtime_json.h.txt` only: +59/−22 lines (1,477 → about 1,515), the prototype's patch
(`/var/tmp/canon-030-research/r-reader/a4.diff`) as the starting point, re-applied to the text CPP-RT left.
No generator Go code changes; `constants.go` is untouched. It adds no `Fail("…` literal call.

New and edited tests in `internal/gen/cpp` (granted): `reader_diff_test.go` (about 230 lines),
`testdata/main/reader_diff_main.cpp`, `reader_oracle.cpp`, `reader_subject.cpp` (about 270 lines),
`testdata/reader/v0.2.0/` (the two verbatim copies), `strict_test.go` (+2 cases),
`types_order_test.go` (+4 cases).

### Spec text

CODEGEN §7.5, first paragraph: "Nothing in it throws: parsing uses `allow_exceptions=false` and every
typed read checks the JSON type first, so it also builds with exceptions disabled." becomes:
> Nothing in it throws: a file is read through nlohmann's `sax_parse`, whose failures come back to the handler, any other parse uses `allow_exceptions=false`, and every typed read checks the JSON type first, so it also builds with exceptions disabled.

and, after its `ParseTextFile` sentence:
> Both read a file in one pass that checks the byte rules as it builds the document; a text that pass refuses is read again by the scans that name its first failure, so a verdict and its message never depend on the pass (DECISIONS D-READER). A NUL byte, which nlohmann reads as the end of its input, is a byte like any other: outside a string it is a syntax error, after the document included (WIRE.md §3.1).

CODEGEN §7.5, second paragraph: "the byte rules and first-failure messages of WIRE.md §8" becomes
"the byte rules of WIRE.md §3.1–§3.3 and the first-failure messages of §5.13".

**Held back from S1** to READER's commit, written by the orchestrator: CPP-RT rewrites §7.5's other
sentences first (§9), and `TestSpecNamesTheRuntime` reads §7.5.

### DECISIONS text

> D-READER. **The C++ JSON runtime reads a file in one pass (CODEGEN.md §7.5; WIRE.md §3.1; extends 361).** `ParseTextFile` and `ParseDataFile` read a text with one nlohmann SAX pass (`StrictCheck` given a document) that checks the byte rules, builds the document and records the Float32 tokens and key orders. A text that pass refuses (invalid, or holding a number out of a Float64's range, which nlohmann refuses before any callback) is read again by 361's scans, so every verdict and message is theirs. `StrictCheck`'s other constructors are unchanged (CODEGEN.md §1.3). A NUL byte, which nlohmann reads as the end of its input, never ends a text: `ParseTextFile` sends a text holding one to the scans, and `ParseDataFile` refuses a NUL after the document (`not a canon data file (invalid JSON)`), as the Go and TypeScript loaders did. A differential fuzz holds both readers to v0.2.0's, the NUL refusal its one listed difference. Reason: a valid text was lexed four times (1.6× to 2.4× slower on Sovereign's largest files read by generated C++), and the C++ data loader accepted `{…}\0junk`.

### Tests (`internal/gen/cpp`; the C++ ones skip in `-short`, as their neighbours do)

| Test | Proves | First |
|---|---|---|
| `TestReaderOracleIsV020` | the oracle copies' sha256: `canon_runtime_json.h` `82cf616172e29e52423417a1a7e8dc4e3afd455f77a3d5769484d92751b777b4` (61,061 bytes), `canon_runtime.h` `812e861e62f70c7010013c1a4cae5a9326cf5d4df4e8ef269a7b73b233a6b14b` (13,755 bytes), both from `git show v0.2.0:<path>` | a pin: edit one byte to see it fail |
| `TestReaderMatchesV020` (builds `reader_diff_main.cpp` with the oracle and subject TUs through `buildAndRun`, over every compiler × mode, the libc++ row included; cites CODEGEN §5.13, §7.5, WIRE §3.1-§3.3, D-READER) | enumerate every prefix, every deletion, and, for seeds of at most 512 bytes, every single-byte insertion and substitution from a 41-byte alphabet, over the texts of `parseFiles`, `typesFiles`, `documentOrderFiles` and a `readerExtraSeeds` table (double BOM; BOM before an unread `1e400`; `{…}\0`, `{…} \0junk`, a NUL in a string; `\u0000` in a key and a value; keys `"a/b"`, `"c~d"`, `""` holding midpoints; midpoints `16777217.0`, `1.000000059604644775390625`, `3.4028235677973366e38`, plus `-0.0`, `1e-400`, `-1e400` and 400 nines; integers 2^64−1, 2^64, −2^63, −2^63−1; unsorted keys at three depths; root scalars, arrays, strings; depth 511, 512, 513), text with orders on and off; data with orders on and off. Requires 0 differences; for text at least 5,000 valid inputs, 1,000 with tokens and 1,000 with orders; for data at least 1 NUL refusal, the one listed difference. About 20 s per binary at `-O1` | a pin. Prove it can fail with three mutations of a scratch copy of the `.txt` (copy aside and back): drop the NUL guard (about 11 differences); hand the BOM-stripped body (about 471); make a failed pass return false without the fallback (the `num_*` and BOM-`1e400` seeds differ) |
| `FuzzReaderMatchesV020` | the same comparison on fuzzed bytes, through a persistent `serve` driver built once per content key (`os.TempDir()/canon-reader-diff/<sha256 of compiler, flags, sources, headers>/driver`, atomic rename as `internal/testkit/cxx`'s libc++ wrapper does) while fuzzing, else in `f.TempDir()`; a dead driver is reported as "driver crashed on input", distinct from a difference | new; 3 minutes at `-parallel 4` under 3G at release (§18) |
| `TestLoaderParity`, two cases | `nultrailing` and `nulspace`: Go and C++ both refuse | **fails first** (C++ prints the load today) |
| `TestTypesDecodeCompilesAndRuns`, four `documentOrderFiles` | `ord_nul.json` "invalid JSON at byte 16"; `ord_nulstr.json` "invalid JSON at byte 8"; `bom2.json` "invalid JSON at byte 3"; `num_bom.json` `mode=auto` | pins (verified on both headers); each fails under one mutation above |

**Goldens that change:** the 31 copies of `canon_runtime_json.h` (24 under `internal/gen/cpp/testdata/golden/`,
7 under `examples/**/expected`), each with the same diff as the `.txt`, and, because the stamp moves
(§9), the stamp lines of every `canon_runtime.h` copy. Read one copy's diff; `cmp` the rest
against it.

### Acceptance

- `TestReaderMatchesV020` green with 0 differences on every compiler and mode; its three mutation runs
  shown in the unit's report.
- The two `TestLoaderParity` cases fail on copies of today's files and pass after.
- `make check` green; `make diag-check` green.
- At release, `FuzzReaderMatchesV020` 3 minutes clean (§18 step 2), and `make realdata-compile` passes.

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| READER | `internal/gen/cpp/runtime/canon_runtime_json.h.txt`; granted the new and edited test files listed in Code, in `internal/gen/cpp` | opus: both C++ data paths use this runtime, the equivalence argument is something tests can miss, `StrictCheck` is public API, and the NUL and BOM traps are easy to reintroduce | M | CPP-RT (same file; the stamp) |

### Forecast

1. **The traps** (each pinned): a stripped body accepts a double BOM; no NUL guard accepts `{…}\0`;
   naming `nlohmann::detail::json_sax_dom_parser` breaks on nlohmann 3.12+; building straight into `doc`
   changes what a failed read leaves behind (a failure leaves `doc` untouched, as today); comparing
   documents with `operator==` misses number-type changes (the test compares types).
2. **Review findings to expect, with answers:** "two repeated-key mechanisms in `StrictCheck`" (the
   check-only API keeps its set, CODEGEN §1.3); "a verdict change in data loaders" (the NUL fix,
   D-READER); "a shared temp dir in a fuzz harness" (the libc++ wrapper's precedent).
3. **diag-check:** a `Fail("…` literal added to the `.txt` fails only `make check`. Run `make diag-check`.
4. **Fuzz memory:** the driver is built once, not per worker; `-parallel 4` (v0.2: 19 of 28 targets died
   at 24 workers).
5. **The Go fuzz is blind to C++ coverage;** the deterministic neighbourhood carries most of the proof.
6. **Not verified:** MSVC (no `cl.exe`); nlohmann 3.9, 3.10 and 3.12 (every API used predates 3.9).
7. **Sovereign:** 0 call sites; 0 C++ data loaders, so the NUL fix cannot bite there; the 25 runtime copies
   change at the next build; `ModelFile::Parse` on `mdlDyna.json` parses about 1.6× faster (the
   end-to-end `Parse`, decode and destruction included, was not measured).

## 5. Header weight: measured, and dropped (no DECISIONS item)

Scope: ADR-0024 item 3, "measured first". Its outline said to drop the item if imports that only name
enums were rare. The research measured something better than that proxy: the compile time the item is
for. Research: one opus pass, with a classifier over Sovereign's 116 Canon headers validated by hand and
by compiling three header trees (118/118 `.gen.cpp` and 116/116 standalone headers each, under
`-fsyntax-only -Wall -Wextra`), preprocessed sizes of all 118 generated translation units, and timed
compiles of generated and hand-written ones (clang++ 21.1.8, `-std=c++20`, capped).

### Facts

- **Header assembly:** `header()` (`internal/gen/cpp/gen.go:185-204`); `headerGroups` (`:206-213`)
  writes `<nlohmann/json_fwd.hpp>` (its own group, only when the header declares a decoder), then
  `"canon_runtime.h"` followed by the imported packages' headers. A `.gen.cpp` includes only its own
  header, standard headers and `"canon_runtime_json.h"` (`gen.go:238-246`).
- **Use classes of the 225 cross-package includes:**
  - (a) enum type names only: 194 (86%; 193 of them name only `@codes` enums). `enum class E : T;`
    is a complete type, so these would compile against a forward declaration;
  - (a+) a record named only in declarations: 13 (all `text/text.gen.h`'s `TextEntry`);
  - (b) constexpr lookup cells calling another package's `detail::<E>Ordinal`: 3;
  - (c) complete records (by value, as a base): 15.
  So (a) is not rare.
- **But generated headers are not where the time goes.** The 118 generated translation units
  preprocess to 553.2 MB: the standard library 370.8 MB (67.0%), generated headers 119.6 MB (21.6%),
  nlohmann 45.6 MB (8.2%), the `.gen.cpp` text itself 12.5 MB, the runtime 4.8 MB. A unit holding only
  `canon_runtime_json.h` (nlohmann's `json.hpp`) costs 1.35 s of CPU before its first generated line.
- **The direction as written** (`enum class X : T;` and `class R;` headers) cuts that input by 5.0% and
  the front-end CPU of the generated units by 2.5% (211.6 → 206.3 s): their `.gen.cpp` files call
  `<E>FromWire`/`<E>FromCode`, name enumerators (1,239 references), classes (532) and make hooks (60),
  so they still need the full headers. **A form that also declares those helpers** cuts the input by
  19.1% and the CPU by 5.7% (167.9 → 158.2 s).
- **Hand-written consumers:** 1,231 Source translation units reach a Canon header; 187 would save more
  than 1 MB of input (Game 124, Client 45, Engine 15, Server 3), gaining 7-11% front-end time and about
  5% at `-O3 -g` (`FarmConfig.cpp`: 6.61 → 6.28 s). The whole build would gain about 1-2% (an estimate:
  the compile database lists 1,589 compile instances).
- **What it would cost:** one new output per C++ package (+116 committed files in Source); every golden
  header; a two-level include rule (forward or full, per import, in headers and in `.gen.cpp` files)
  whose every slip fails in a consumer's build; the deleted nlohmann pair of position enums would have
  to move into the forward header (a deleted function is a definition, once per translation unit); and
  16 (the helper form) to 32 (the plain form) Source files to migrate, which used names their includes
  provided transitively (`Engine/World/Environment.cpp`: "no member named 'kLanguageMembers'").
- **Opened enums:** Sovereign has none in C++ (all nine `open:` lists of `Resource/Config/**/emit.canon`
  are on `emit go`); the outline's "60,000 dynamic initialisers" was a hypothetical in
  log-2026-10-21 (`:46-47`). Moving their member definitions into the `.gen.cpp` would lose the
  ordering C++ gives an inline variable over a consumer's later static initialiser
  ([basic.start.dynamic]), so a consumer static copying a member could silently read an empty wire.
- **The roadmap's "256 MB of header closures against 16 MB of own code"** holds in bytes (232.6 MB of
  generated-header closures against 16.4 MB of `.gen.cpp` text); it is not the compile time.
- **A spec contradiction found:** CODEGEN §7.1 (`spec/CODEGEN.md:2031-2034`) puts the imported headers
  before `"canon_runtime.h"` and `json_fwd.hpp` "in JSON modes"; the code and CODEGEN §2.7
  (`:409-424`) put `"canon_runtime.h"` first and `json_fwd.hpp` only when the header declares a decoder.
  The goldens follow §2.7.

### Decision

1. **Item 3 is dropped, both parts.** About 1-2% of a consumer's build, and 2.5-5.7% of the generated
   units' front end, do not pay for a permanent new output kind, a two-level include rule that fails in
   consumers' builds, and a consumer migration. Robustness before speed (Louis, 2026-10-14).
2. **Revisit** when generated text dominates a consumer's preprocessed input, or when nlohmann leaves
   the `.gen.cpp` (item 2's reader does not change that: it is the same `json.hpp`).
3. **The opened-enum part is dropped:** no consumer, and a silent initialisation-order hazard.
4. **CODEGEN §7.1 follows the code and §2.7** (S1, spec only).
5. Louis may overrule Decision 1 (§0): the research holds a decision-complete design of the helper form
   (`/var/tmp/canon-030-research/r-header/`); a later ADR would write it.

### Spec text

CODEGEN §7.1, its first bullet's include sentence becomes:
> `<last>.gen.h` starts with `#pragma once`, as `<last>.defines.gen.h` does (§7.9; recent Source code's form, log-2026-09-24), then standard headers (sorted), then `<nlohmann/json_fwd.hpp>` (its own group) when the header declares a decoder, then `"canon_runtime.h"` followed by the imported packages' headers (sorted), as §2.7 says.

### Forecast

Nothing is built. The numbers go to Louis in the reply (§18 step 11), with the recommendation above.

## 6. Baked containers find their keys in sorted constant tables; views of baked data wait (DECISIONS 432)

Scope: ADR-0024 item 4 ("baked data without heap copies") and item 12 (presence bits, a stretch).
Research: one opus pass over `internal/gen/cpp`, the 12 generated `Build()` functions of the
measurement tree, and Source (read-only), with compile probes under g++ 15.2 and clang++ 21.1.

### Facts

- **One generator function writes every baked `Build`.** `bakedAccess` (`internal/gen/cpp/baked.go:99`)
  with `text/baked_access.txt`: a `struct Data`, `static Data Build()`, and a function-local
  `static const Data data = Build();` in `Get()`. Containers are allocated first, then values, then fn
  cells; long builds split into `Build<N>` parts (`bulk.go:200-214`, DECISIONS 392).
- **What `Build` copies to the heap**, from DECISIONS 392's namespace-scope arrays (which hold):
  - **container rows and keys:** `d.items.rows_ = canon::KeyedList<std::string, Item>::FromRows(std::vector<Item>(8), std::vector<std::string>(std::begin(kBulk0), std::end(kBulk0)));`
    (golden `internal/gen/cpp/testdata/golden/baked-bulk/bulk.gen.cpp:476`; format `keyedFromFormat`,
    `errors.go:234`, written at `baked.go:190`; `fromRowsFormat`, `constants.go:227`, is the data-mode loader's
    and stays), then `KeyedList::FromRows` sorts an index at run time
    (`canon_runtime.h.txt:333-354`, `detail::SortedIndex` `:275`);
  - **a `@stable` index:** `byCode_ = canon::detail::SortedIndex(byCodeKeys_)` (`bulk.gen.cpp:478`);
  - **list and String columns of rows:** `r2.names_ = std::vector<std::string>(kBulk3 + kBulk4[i], kBulk3 + kBulk4[i + 1]);`;
  - **top-level lists and maps, fn cells, resolved ref lists** (heap addresses, never constexpr).
- **Sovereign: 12 `Build()`s** (11 under `Engine/Resource/Generated`, 1 telemetry), all copying on
  first use; 4 build `std::vector<std::string>` from arrays. Together they build 34,313 strings from
  arrays, 33,477 of them in `ids` (97.6%).
- **The outline's motivating case is a record field, not an accessor.** The 33,425 define names are
  `IdHeader::names_` (`Source/Engine/Resource/Generated/header/ids/ids.gen.cpp`: `kBulk3` at line 79,
  copied per row at about `:67045`). Source then reads them through the getter
  `const std::vector<std::string>& GetNames() const` and copies them again into its own maps
  (`Engine/Resource/Defines/GeneratedDefines.cpp:17-30`, `Engine/Service/Fleet/FleetConfig.cpp:101-110`).
- **Sovereign's 15 baked accessors** are 13 containers, `GetRankDailyBoards` (a `FlatMap`) and
  `GetProvenance` (a `std::vector<LakeColumn>`). None is a `[String]` or scalar list, so an
  accessor-only view would change nothing there.
- **A record's storage is the same in every mode.** A `[String]` field is
  `const std::vector<std::string>& GetNames() const { return names_; }` in baked, data and types mode
  alike (`ctype.go:183-205`, `records.go:114-144`), and DECISIONS 323 (Louis, 2026-10-06) lets any
  package build another package's record through make hooks "whatever that package's mode". The
  emberfall fixture does exactly that: `core` is baked and declares `LevelRange.labels: [String]`, and
  `world` in data and types mode decodes `labels` into a `std::vector<std::string>` and calls
  `CoreMake::LevelRange(...)`. DECISIONS 401: "a copy of a record or decoded value has its own rows".
  A view-typed field would have no storage when another package builds the record from a file.
- **The C++ floor is C++17** (CODEGEN §9): `std::span` is not available in generated code;
  `std::string_view` is. `canon_runtime.h.txt` (399 lines) has no view type.
- **The cost of keeping today's copies, measured:** the first `GetIdHeaders()` makes 27,846
  allocations (2.05 MB) in about 1.2 ms (g++ -O2, Linux, three runs).
- **Precedent for generator-sorted constant tables:** DECISIONS 383's enum `FromWire`
  (`enums_tables.go`, `text/lower_bound.txt`), compiled at 60,000 members by
  `TestBigEnumTablesCompile` (`bigenum_test.go:46`). A C++ enum's underlying value is its code under
  `@codes` (CODEGEN §5.2), not its position: a generator-side sort must follow C++'s own order.
- **An address pin exists:** `internal/ir/testdata/e2e/addresses/probe.cpp.txt:30-41` checks
  `baked-find-is-at`, `baked-keyed-find-is-at` and `baked-get-is-find`.
- **Item 12 (presence bits), measured:** 213 classes hold 1,005 optional scalars (the widest,
  `sov::items::Item`, 118). One presence bit per optional would save about 2.73 MB of Sovereign's
  loaded data (items 2.44 MB) and nothing on baked data. Make hooks take `std::optional<T>` parameters
  (CODEGEN §5.14), and Source calls hooks directly at 58 sites in 13 files. The generator files it
  touches are `ctype`, `records`, `stored`, `make`, `decode`, `baked_fill`, `baked_foreign`, `columns`,
  `bulk`, `define_refs`, `walk`, `resolve`, `readers` and `methods` (the outline's list named `inputs`,
  which holds no record member, and missed the last three).

### Problem

- A baked container copies every key into the heap and sorts an index on first use, although the
  generator knows the keys and their order at build time.
- The direction "accessors return views" cannot reach the copies it was written for: they are record
  fields, and a record field's storage is shared with packages that build it from files (DECISIONS
  323, 401). A view there needs either a refusal of those builds (a change to a decision Louis
  accepted) or an arena kept alive by every reader: a record-layout redesign, not a v0.3 item.

### Decision

1. **Baked containers find without copying keys.** In a `baked` emit, a table or keyed-list container
   holds its rows in a `std::vector<T> rows_` that `Build` sizes once (`d.<v>.rows_.resize(N);`) and
   fills in place (`auto& r = d.<v>.rows_[i];`, no `const_cast`). It holds no key vector and no
   run-time index; `FromRows` and `SortedIndex` are not called for it.
2. **`Find` and every `FindBy<F>`** are declared in the header (same doc lines, same parameters) and
   defined in the `.gen.cpp`:
   - a table's `Find` goes through its id enum: `const std::optional<TId> id = TIdFromWire(key);`,
     then `&rows_[static_cast<std::size_t>(*id)]`, or `nullptr`;
   - a keyed list's `Find` and every `FindBy<F>` binary-search two function-local
     `static constexpr std::array`s written by the generator: `kKeys` (the keys, sorted) and `kRows`
     (`uint32_t` row positions), with `text/lower_bound.txt`'s loop; a hit returns `&rows_[kRows[lo]]`.
     String keys are `std::string_view`, sized views from 1,024 entries as enum tables do. (No opened enum
     reaches a baked container: opened enums exist only in `types` emits, and a baked emit using one is
     `E8019` `OpenEnum`.)
   - The definition's parameter is named `key`, so no field name collides with the locals.
3. **The order is C++'s:** strings by bytes (as `std::string_view::compare`); integers and `Duration`s by
   value; an enum by its C++ underlying value (its code under `@codes`, else its position); a table id,
   and a ref held as a table id, by entry position; any other ref by its key's order. Keys are unique (E3102), so the
   order is total. Zero rows give `std::array<K, 0>` and `Find` returns `nullptr`.
4. **Unchanged:** the container's API (`Len`, `At`, `All`, `Get`, `Find`, `FindBy<F>` signatures and
   results), entry addresses (`Find(k) == &At(i)`), `data`-mode containers, `canon::KeyedList`
   fields of records, both runtime headers, every accessor and getter type, Go, TS and JSON.
5. **Views of baked data are decided out of v0.3.** They move to v0.5 (binary data), where the layout
   of loaded records is designed anyway: zero-copy fields need a backing store in every mode, which
   DECISIONS 323 and 401 bind. Until then a `[String]` field stays `const std::vector<std::string>&`.
6. **Presence bits (item 12) are decided out of v0.3**, to the same v0.5 layout design: the gain is
   about 2.7 MB of loaded data and nothing on baked data, and every write path would have to set its
   bit, a new silent-failure class (a missed write turns present data into none, with no compile
   error).

### Code (`internal/gen/cpp`)

- `values.go` (290 lines; the container class, `:157-211`): a baked container declares
  `std::vector<T> rows_;` and only declares `Find`/`FindBy<F>`; `Len`/`At`/`All`/`Get` read `rows_`.
- `baked.go` (283): `allocate` writes `resize` instead of `FromRows`; no `@stable` index.
- `columns.go`, `baked_fill.go`, `bulk.go`: row references and entry addresses (`&d.<v>.rows_[i]`).
- New `baked_find.go` (about 110 lines): the sorted key tables and the definitions.
- `errors.go` (361 lines; holds the baked format block, `:182-252`): about 10 new formats.
  `constants.go` (495) is not touched.

Example (`ids`), header: `const IdHeader* Find(std::string_view file) const { return rows_.Find(file); } … canon::KeyedList<std::string, IdHeader> rows_;`
becomes `const IdHeader* Find(std::string_view file) const; … std::vector<IdHeader> rows_;`. The
`.gen.cpp` gains:
```cpp
const IdHeader* IdHeaders::Find(std::string_view key) const {
    static constexpr std::array<std::string_view, 19> kKeys = {"ContinentDef.h", "WindowIds.h", /* …17 more, sorted by bytes */};
    static constexpr std::array<uint32_t, 19> kRows = {14, 16, /* … */};
    std::size_t lo = 0;
    std::size_t hi = kKeys.size();
    while (lo < hi) { const std::size_t mid = lo + (hi - lo) / 2; if (kKeys[mid] < key) { lo = mid + 1; } else { hi = mid; } }
    if (lo < kKeys.size() && kKeys[lo] == key) return &rows_[kRows[lo]];
    return nullptr;
}
```
(The loop is `text/lower_bound.txt`'s; the unit reuses that template, it does not retype it.)

### Spec text

CODEGEN §5.9, the baked bullet, after "accessors return references into it.":
> A baked table or keyed-list container keeps its rows in a `std::vector` that `Build` sizes once and fills; its `Find` and each `FindBy<F>` are declared in the header and defined in `.gen.cpp`, where they binary-search function-local `static constexpr` arrays of the keys, sorted by the generator, beside the rows' positions, the shape of `<E>FromWire` (§5.2): strings by bytes, integers and `Duration`s by value, an enum by its C++ underlying value (its code under `@codes`, else its position), a table id or a ref held as one by entry position, any other ref by its key's order. A table's `Find` is its id enum's `<Rec>IdFromWire`, then the row at that id. No key is copied and nothing is sorted at run time (DECISIONS D-BAKEDFIND).

CODEGEN §7.3's sample: its container's private members become `std::vector<EventType> rows_;` (the
`codes_` and `byCode_` members go), its Build comment becomes `// fills rows_`, and the paragraph after
it gains:
> A container's `Find` and `FindBy<F>` read their keys from sorted constant arrays of `.gen.cpp` (§5.9), so `Build` copies no key.

### DECISIONS text

> D-BAKEDFIND. **A baked container looks its keys up in constant tables the generator sorts (CODEGEN.md §5.9, §7.3; extends 383, 392).** In a `baked` emit a table or keyed-list container holds its rows in a `std::vector` that `Build` sizes and fills; `Find` and every `FindBy<F>` are defined in the `.gen.cpp` and search function-local `static constexpr` arrays of the keys sorted in C++'s order (bytes for strings, value for integers and durations, underlying value for enums, entry position for table ids and refs held as them, key order for other refs), with the rows' positions beside them, as `<E>FromWire` does; a table's `Find` goes through `<Rec>IdFromWire`. No key is copied into the heap and nothing is sorted at run time; the API is unchanged, and `data`-mode containers and `canon::KeyedList` fields keep their run-time index. Views of baked data and presence bits wait for v0.5's record layout: a record's storage is shared with packages that build it from files (323, 401). Reason: every baked `Build` copied and sorted its container keys on first use, though the generator knows them.

### Tests (`internal/gen/cpp`)

| Test | Proves | First |
|---|---|---|
| `TestBakedContainersHoldNoKeys` (text test on the baked-bulk fixture) | neither generated file of a baked container holds `FromRows(`, `SortedIndex(`, `FindSorted(` or `const_cast<` (today `FindSorted` sits in the header's `FindBy`), and the container declares `std::vector<Item> rows_;` and no `canon::KeyedList<` member | fails on `9ed956e6` (`bulk.gen.cpp:476-478`) |
| `TestBakedFindTables` (new `baked_find_test.go`, compiled and run on every `cxx` compiler) | every key is found at `&At(i)` and an absent key gives `nullptr`, for: String keys `"Z"`, `"_"`, `"a"`, `"ab"`, `"a\0b"`, `"é"` and a prefix pair; Int keys with negatives and ±2^62; an enum whose wire order differs from its positions; a `@codes` enum with codes 9, 1, 4; a ref key into a baked table; a table; `FindBy` over `UInt16` and over `String`; an empty keyed list. Cites CODEGEN §5.9, D-BAKEDFIND, DECISIONS 401 | a pin (today's run-time sort also finds them). Prove it can fail: sort the enum keys by position instead of code in a scratch copy; the `@codes` case fails |
| `TestBigFindTablesCompile` (`bigenum_test.go`) | a 60,000-row keyed list with 40-byte String keys compiles `-fsyntax-only` on every compiler | a pin. Prove it can fail: bypass the sized views; clang hits its constexpr step limit |
| unchanged and green | `baked_compile_test`, the bulk scale test (≤ 1.5 GB at `-O3 -g`, DECISIONS 392), `internal/ir`'s e2e address probe (`baked-find-is-at`) | pins |

**Goldens that change:** about 20 files in 10 trees: `internal/gen/cpp/testdata/golden/` baked,
baked-bulk, baked-nul, baked-tri, emberfall-core, emberfall-world-baked, empty-owner-b; `examples/`
features/shared (core), resource/vocab, telemetry. Only container bodies, `Build` allocations, row
references and addresses, `kBulk` renumbering and the new `Find`/`FindBy` definitions move; any other
hunk stops the unit.

### Acceptance

- `TestBakedContainersHoldNoKeys` fails on copies of today's files and passes after.
- `TestBakedFindTables` and `TestBigFindTablesCompile` pass on g++, clang++ and clang++ with libc++.
- `make check` green; `make realdata-compile` passes (§18 step 6).

### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| BAKED-FIND | `internal/gen/cpp` | sonnet, opus review: golden- and run-checked, but a sort order that must equal C++'s | S-M | ENUM-CPP (§3): it reuses `lower_bound.txt` and the sized views, which ENUM-CPP reshapes |

### Forecast

1. **The order trap** (Decision 3) is the one way to be silently wrong: a `@codes` enum key sorted by
   position finds nothing. `TestBakedFindTables`' `@codes` case exists for it.
2. **Review findings to expect:** declaration and definition parameter names differ (`file` against
   `key`): accepted, the definition's name is fixed; two `Find` shapes (a table through its id enum, a
   keyed list through `kKeys`): accepted; `kBulk` renumbering churn in the goldens; a missing zero-row
   case (it is in the test).
3. **Memory and compile time:** the key tables take the enum tables' proven form. MSVC at that scale
   stays unverified (CODEGEN §9).
4. **Sovereign:** 13 containers, 294 keys and 13 run-time sorts disappear; 0 sites change. The
   `IdHeader::names_` copy (27,846 allocations) stays until v0.5, as the reply says.
5. **Louis may overrule Decision 5** (§0): building views now needs DECISIONS 323 reopened. If he
   does, a new ADR designs it; v0.3 does not wait.

## 7. TypeScript for bundlers: every top-level construction is marked pure (DECISIONS 433, 434)

Scope: ADR-0024 item 5. Research: one pass over `internal/gen/ts`, the 54 TS files of the measurement
tree, and real bundler runs, read-only, from Sovereign's own `node_modules` (Vite 7.3.6 with Rollup
4.63.1, esbuild 0.28.2) and canon-showcase's (Rolldown 1.2.12). Terser and webpack are installed
nowhere on this machine; what this section says of them is from their documentation. A prototype was
built in a scratch copy, and the `gen/ts` suite passed with both TS compilers.

### Facts

**What a generated module builds at load** (`internal/gen/ts`, formats in `constants.go`):

| Construction | Format (`constants.go`) | Written by |
|---|---|---|
| `export const <E>Members` / `<E>LiveMembers` / `<Id>Members` / `<Id>LiveMembers = Object.freeze([...])` | `membersFormat` (:206) | `enums.go:135-136`, `:223-224` |
| `export const <E>Names` / `<E>Codes = Object.freeze({ ... })` | `recordConstFormat` (:207) | `enums.go:146` |
| `const <E>Index = canonIndex(<E>Members)` (private, only where a lookup or comparison needs positions) | `privateIndexFormat` (:208) | `enums.go:244`, `:264` |
| `canonFreeze<T>(...)` | `freezeFormat` (:219) | `values.go:42`, `:135`; `literal.go:196`; `canonlist.go:61` |
| `canonFreeze(canonList<T>(...))` | `callFormat` | `canonlist.go:63` |
| `const X: T = canonFreeze<T>(...)` (precomputed cells) | `frozenConstFormat` (:220) | `fns.go:89` |
| lookup cells (through `cellsFrozen` and `frozenArray`) | `arrayConstFormat` (:202) | `fns.go:114` |
| `export const x: CanonTable<K, T> = canonTable(...)` | `tableFormat` (:222) | `values.go:111` |
| `new CanonMap<K, V>(...)` | `newMapFormat` (:236) | `literal.go:189`; reader defaults |
| `canonList<T>(($r) => [...])` | `canonListFormat` (:237) | `canonlist.go:47` |
| `const b: ReadonlyMap<K, D> = new Map<K, D>(...)` (branch tables) | `branchTableFormat` (:66) | `branches.go:63` |
| `const $union<N> = [...XMembers, "lit"]` (data and types decoders) | `arrayConstFormat` (:202) | `decode_value.go:134` |

- The outline's sites (`literal.go`, `values.go`, `fns.go`, `canonlist.go`, `decode.go`) moved and
  are incomplete: it omits the enum tables, the private indexes and the branch tables, and the
  `decode.go` sites run inside function bodies, not at load.
- **No top-level statement has an effect at load.** An AST scan of every top-level statement of
  Sovereign's 51 non-conformance files, the 86 TS outputs of `internal/gen/ts/testdata/ts/*.txtar` and
  the 14 example goldens found only imports, classes, functions, types, and `const` declarations of
  primitives, literals, or calls of the helpers above, plus `Symbol.for` in the decoder helper text
  (idempotent). No expression statement. `canonFreeze` mutates only its fresh argument; `CanonMap`
  freezes itself. The outline's "the helpers have no side effects" holds.
  `<last>.conformance.test.ts` registers `node:test` tests at load by design; it is never part of a
  program.
- **What bundlers drop** (esbuild, Rollup, Rolldown, all tested): a top-level `const` whose
  initializer is a call or `new` preceded by `/*#__PURE__*/`, when nothing reads it. A mark covers
  its own call, **not its arguments**: `/*#__PURE__*/ f(f(1))` keeps the inner `f(1)`. A mark before a
  property access, an object or array literal, or a template literal does nothing. Unmarked
  `Object.freeze(...)` is dropped by Rollup only. tsc 5.0.4 and 7.0.2 keep the mark through emit;
  `removeComments` deletes it.
- **Sovereign's TS** (51 files, 3,075,370 bytes): 2,135 top-level constructions, of which 1,674
  `Object.freeze` (78%, 538,133 bytes of enum tables), 355 `canonFreeze`, 54 `canonIndex`, 37
  `canonTable`, 13 `new CanonMap`. The five web apps are SvelteKit on Vite 7.3 (Rollup and an esbuild
  minifier), with `verbatimModuleSyntax` and `isolatedModules`.
- **The helper block** (`runtime.go:13-20` embeds `helpers.ts.txt`, 170 lines, CODEGEN §8.2's block,
  and `decode.ts.txt`): each file carries the closure of the helper units it uses
  (`runtime.go:92-182`), pinned byte for byte by `TestHelperBlockIsTheSpecs`
  (`internal/gen/ts/spec_test.go:27`) and `TestHelperBlockIsASubsequenceOfTheSpecs` (`:39`).
- **CODEGEN §2.5** (`spec/CODEGEN.md:307-315`): the explanatory comments of generated code are fixed
  templates, "no other prose is generated", and "a golden that holds other prose is wrong". The
  template table ends at T21 (`:348`).
- `constants.go:130` already names `pureMark = "$"` (a translated fn's pure twin): the new constant
  takes another name.
- **A pre-existing trap, not bytes.** Every generated file declares its own `export class
  CanonEvalError` and `export interface CanonTable` (CODEGEN §8.2). So `export *` over two generated
  modules fails tsc with TS2308, and an error thrown by one file's translated fn is not
  `instanceof` another file's `CanonEvalError`. Sovereign's code references neither name.
- **Measured (§2):**
  - The bundle effect (Vite 7.3.6 production build of a scratch app reading one primitive constant
    from each generated module of a directory): admin 336,404 → 842 bytes, monitoring 545,484 → 852,
    website 35,072 → 845, trello 1,700 → 783. An empty Vite app is about 780 bytes.
  - Marking only the outer `canonFreeze`/`canonTable`/`CanonMap` calls, the outline's list, leaves
    11,755 of 47,552 bytes (esbuild, `services.monitoring.ts`, one constant imported); the full chain
    without `Object.freeze` leaves 2,247; the full chain with it, 38.
  - The helper bytes repeated in a directory: admin 1.55%, monitoring 1.40%, trello 6.27%, website
    5.58%, wiki 0%; 1.95% of all generated TS. One shared copy would save at most 6.07% of a
    directory's gzip, and at most 3.2% of a minified, gzipped bundle holding every module.

### Problem

A bundler keeps a top-level call it cannot prove pure. So a program that imports one constant of a
generated module ships every table of that module: 336 KB for admin's eleven constants, where 842
bytes suffice. The helper block repeated per file costs 2%, which is not the problem.

### Decision

1. **Every top-level construction is marked.** Every `Object.freeze`, `canonFreeze`, `canonList`,
   `canonTable`, `canonIndex`, `new CanonMap` and `new Map` that builds a module-level value is preceded by
   `/*#__PURE__*/ `, each call of a chain separately. The mark is written in the formats that build those
   values (Decision 2), wherever they are used, so a reader's default (`new CanonMap<K, V>([])` inside a
   decoder) carries a harmless mark too. A decoder's returned value (`return canonFreeze(…)` and
   `return canonTable(canonFreeze(…))`, from `publicDecoderFormat`, `tableDecoderFormat` and
   `valueDecoderFormat`, `constants.go:291`, `:295`, `:310`) is not marked: a function body is not a
   top-level statement, and a bundler drops an unused function whole.
2. **One constant**, `pureCall = "/*#__PURE__*/ "` in `constants.go`'s declaration block, joins the
   nine formats of the Facts table (`branchTableFormat`, `membersFormat`, `recordConstFormat`,
   `privateIndexFormat`, `freezeFormat`, `frozenConstFormat`, `tableFormat`, `newMapFormat`,
   `canonListFormat`); `canonlist.go:63` uses a new `freezeBareFormat = pureCall + "%s(%s)"` instead
   of `callFormat`. No other Go change.
3. **Unmarked on purpose:** the fixed helper texts (so CODEGEN §8.2 and its pins do not move),
   `Symbol.for` (every bundler drops it), and `$union<N>` (an array literal with a spread: no mark can
   cover it; Sovereign has none).
4. **The list is closed.** A future construction helper joins the list, the spec text and the pin.
5. **The runtime module per output directory is declined.** It is built only when the helper bytes
   repeated in a directory reach 10% of that directory's generated TS bytes, or one shared copy would
   shrink its gzip by 10%. Measured maxima: 6.27% and 6.07%.
6. **No bundler joins the test toolchain.** esbuild in `tools/tsc` would be a new third-party
   dependency (CLAUDE.md, IMPLEMENTATION-PLAN §11). A text pin stops a forgotten mark; the bundle
   effect is the measurement above, recorded.
7. **The guide** documents the marks, the scoped `"sideEffects": false`, `removeComments`, and the
   per-file `CanonEvalError` (GUIDE, §17.4).
8. **The `CanonEvalError` trap is owed** (§19), for v0.4, where translated fns multiply: either a
   runtime module or a brand check in the class. v0.3 documents it.

### Code (`internal/gen/ts`)

- `constants.go` (385 lines): `pureCall` with its doc line citing CODEGEN §8.1; the nine formats gain
  it; `freezeBareFormat`. About +3 lines.
- `canonlist.go:63`: `freezeBareFormat`.
- `export_test.go`: `const PureCall = pureCall`, so the test repeats no literal.
- New `pure_test.go` (about 75 lines).

Generated text, before and after (from the prototype's real output):
```ts
export const GradeMembers: ReadonlyArray<Grade> = Object.freeze(["normal", "unique", "ultimate"]);
const GradeIndex = canonIndex(GradeMembers);
export const statuses: CanonTable<StatusId, CanonRow<Status, StatusId>> = canonTable(canonFreeze(canonList<CanonRow<Status, StatusId>>(($r) => [
```
```ts
export const GradeMembers: ReadonlyArray<Grade> = /*#__PURE__*/ Object.freeze(["normal", "unique", "ultimate"]);
const GradeIndex = /*#__PURE__*/ canonIndex(GradeMembers);
export const statuses: CanonTable<StatusId, CanonRow<Status, StatusId>> = /*#__PURE__*/ canonTable(/*#__PURE__*/ canonFreeze(/*#__PURE__*/ canonList<CanonRow<Status, StatusId>>(($r) => [
```
Names, exports, types, behaviour, the fixed helper texts, Go, C++ and JSON are byte-identical.

### Spec text

CODEGEN §2.5, a row after T21 (`spec/CODEGEN.md:348`):
> | T22 | TS | before each call or `new` that builds a module-level value, and wherever the formats that write those calls are used (a reader's default), harmlessly (§8.1) | `/*#__PURE__*/ `, immediately before the call or `new`, one space after: a bundler directive, not prose |

CODEGEN §8.1, a bullet after the `canonList` bullet:
> - **Bundlers.** Every `Object.freeze`, `canonFreeze`, `canonList`, `canonTable`, `canonIndex`, `new CanonMap` and `new Map` that builds a module-level value is preceded by `/*#__PURE__*/ ` (T22), each call of a chain separately, since a mark covers its call and not its arguments; the formats that write those calls also mark a reader's default, harmlessly, and a value a decoder returns is not marked (a bundler drops an unused function whole). A bundler that finds a top-level `const` unread then drops its declaration, so a program that imports one constant of a module carries that constant alone. The list is closed: a new construction helper joins it. A top-level statement of a generated module is a declaration, never an expression statement, and builds only immutable data, so a module has no effect at load a program can observe; the `*.conformance.test.ts` file registers tests and is not part of a program. A `$union<N>` constant of a `data` or `types` decoder (an array literal with a spread) is the one top-level value no mark can cover. The helper block stays in each file (DECISIONS D-TSHELPERS).

CODEGEN §5.2's TS sample (`spec/CODEGEN.md:1082-1090`) and §8.3's sample (`:3117`), applied by S1
as a mechanical rule (no test compares them with code):
- each of the four `= Object.freeze(` lines of §5.2 becomes `= /*#__PURE__*/ Object.freeze(`;
- the prose `const ToneIndex = canonIndex(ToneMembers);` becomes
  `const ToneIndex = /*#__PURE__*/ canonIndex(ToneMembers);`, and
  `const <alias>_<E>Index = canonIndex(<alias>.<E>Members);` becomes
  `const <alias>_<E>Index = /*#__PURE__*/ canonIndex(<alias>.<E>Members);`;
- §8.3's `statuses` line becomes
  `export const statuses: CanonTable<StatusId, CanonRow<Status, StatusId>> = /*#__PURE__*/ canonTable(/*#__PURE__*/ canonFreeze(/*#__PURE__*/ canonList<CanonRow<Status, StatusId>>(($r) => [`.

SPEC §15.4, a bullet after "Tables and keyed lists are `CanonTable` values":
> - Every call that builds a top-level table, frozen value or enum table is marked `/*#__PURE__*/`, so a bundler drops what a program never reads (CODEGEN.md §8.1).

### DECISIONS text

> D-PURE. **Top-level TypeScript values are built by calls a bundler can drop (CODEGEN.md §2.5 T22, §5.2, §8.1, §8.3; SPEC §15.4).** Every `Object.freeze`, `canonFreeze`, `canonList`, `canonTable`, `canonIndex`, `new CanonMap` and `new Map` that builds a module-level value is preceded by `/*#__PURE__*/ `, each call of a chain separately (a decoder's returned value is not). A generated module has no top-level expression statement and builds only immutable data. Reason: bundlers keep a top-level call they cannot prove pure, so a program that imported one constant of a generated module shipped every table of it (Sovereign's admin: 336 KB for 842 bytes of use); a mark covers its call and not its arguments, and esbuild and Rolldown do not know `Object.freeze`.

> D-TSHELPERS. **TypeScript keeps its helper block in each file (CODEGEN.md §2.3, §8.1, §8.2).** A runtime module per output directory, as C++'s `canon_runtime.h`, is built only when the helper bytes repeated in a directory reach 10% of its generated TypeScript bytes, or one shared copy would shrink its gzip by 10%. Measured on the real project at v0.2.0: at most 6.3% and 6.1% of the sources, at most 3.2% of a minified, gzipped bundle. Reason: the saving is far below the threshold, and a shared module would end one `.ts` file per emit and change every TypeScript golden.

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| gen/ts | `TestTopLevelConstructionsArePure` (`pure_test.go`; cites CODEGEN §8.1 "Bundlers", §2.5 T22) | over every txtar output and every example golden (conformance files skipped; `spec_test.go`'s helpers): outside the helper block, every match of `Object\.freeze\(\|canonFreeze[<(]\|canonList<\|canonTable\(\|canonIndex\(\|new (?:CanonMap\|Map)<` in a module-level `const` is immediately preceded by `tsgen.PureCall`; it fails when nothing was seen | fails on `9ed956e6` (no module-level construction is marked) |
| gen/ts | unchanged and green | `TestGenerated`, `TestHelperBlockIsTheSpecs`, `TestHelperBlockIsASubsequenceOfTheSpecs`, `TestConformanceFileIsTheSpecsTemplate`, `TestEmbeddedIsBaked`, `TestHeaders`, `TestNoExportedIndex`, the typecheck tests with both compilers (`--strict --noUnusedLocals`), the conformance and driver tests, `TestBigListsTypecheck`, `TestEnumIndexNotExported` | pins |
| cli | `TestGuideBareJSONFencesParse` (GUIDE's unit) | every `json` fence of the guide without a path is valid JSON, and `emit.md` holds the `"sideEffects": false` one | new |

**The review gate for the goldens** is mechanical: for each changed golden,
`git show HEAD:$f | diff -q - <(sed 's|/\*#__PURE__\*/ ||g' $f)` prints nothing. Every hunk is an
inserted mark, or the unit stops.

**Goldens that change** (regenerated, diff read): 49 of 59 `internal/gen/ts/testdata/ts/*.txtar`
cases (`go test ./internal/gen/ts -run 'TestGenerated$' -update`), 12 of the 15 TS files under
`examples/**/expected` (`go test ./internal/testkit/golden -run 'TestExamples$' -update`; no MANIFEST
change), and one `internal/cli` command golden, `build_cross_roots_together` (`-run 'TestCommands$'
-update`). 62 files, 607 marks.

### Acceptance

- `TestTopLevelConstructionsArePure` fails on copies of today's files and passes after.
- The strip-the-mark loop is clean on every changed golden.
- `make check` green (`CANON_REQUIRE_TS=1`: both compilers).
- `make realdata-compile`'s TS leg passes (§18 step 6; the marks are comments).

### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| TS-PURE | `internal/gen/ts` | sonnet: mechanical, golden-checked, with a test that fails first | S | KIND (the gen/ts queue) |

The guide text is GUIDE's (§17.4).

### Forecast

1. **Review findings to expect:** marks in reader bodies (harmless, the rule is per format); "no other
   prose" (T22 answers it); the closed list against a future helper; the `$union` limit. One review
   round is budgeted.
2. **v0.2's TS landing lesson** (log-2026-10-31, EN-TS): `build_cross_roots_together` changes here;
   run `TestCommands` before reporting.
3. **Consumers:** no API change; Sovereign's next `canon build` rewrites 51 files with about 3,570
   marks (+1.6% raw, +0.27% gzip). A consumer that compiles with `removeComments`, or reformats the
   generated files, loses the marks; the guide says so. Sites to migrate: 0.
4. **Not verified:** Terser and webpack (their docs only); SvelteKit's own `$lib` path (the same Vite
   resolver ran); a whole-Sovereign regeneration with the prototype (the Sovereign numbers come from an
   AST model that matches the generator within one mark on the example goldens).
5. **A bundler that ignores annotations** (`esbuild --ignore-annotations`, Terser with side effects
   off) gets today's output, never worse.

## 8. Generated functions stay within 1,000 lines (DECISIONS 435; amends 392)

Scope: ADR-0024 item 6 and the function-size items ADR-0023 §9.5 deferred. Research: one sonnet pass
that measured every generated function of the measurement tree (a brace-counting scanner validated by
hand and, for Go, against `go/parser` on all 19,314 functions), generated synthetic 300-member records
and 10,000-row tables, and compiled hand-cut versions to measure the gain and check equivalence.

### Facts

- **What grows today:**
  - C++ `detail::Decode(<Record>)` (`internal/gen/cpp/decode.go:30` `decoders`, `:53` `decodeRecord`,
    `:64` `decodeBody`, `:104` `decodeField`): one block per field, never split; 4.0 lines per field on
    the synthetic mix (1,211 lines at 300 fields). Sovereign's widest real ones: `TroupeSkill` 773
    lines (`character.gen.cpp:588`), `Item` 632.
  - C++ baked `Build` (`bulk.go:197` `buildParts`, `:215` `splitBody`; `bakedPartLines` = 1000,
    `errors.go:208`): cut once a part **reaches** 1,000 lines, between top-level statements, so parts are
    1,000 lines or a little more (1,000 and 1,003 in `TestBulkBuildIsSplit`'s fixture), and one block is never cut. Sovereign has no `Build<N>` part (its 12 baked
    `Build`s are at most 340 lines).
  - Go `build<Last>` (`internal/gen/go/baked.go:82` `values`, `:126` `allocate`, `:147` `fill`): one
    statement per row, 9 lines per row with 6 scalars, never split. Sovereign: `buildTelemetry` 16,181
    lines (`telcat.gen.go:9309`), `buildFeeds` 9,358, `buildSemantics` 4,305, `buildWiki` 3,761,
    `buildDisplay` 2,780, `buildRecordbook` 1,751. No build function holds a local variable.
  - **Go `decode<T>` grows with fields too** (the outline missed it): `decode.go:120` `decodeBody`,
    `:132` `readFields`, 10.9 lines per field (3,262 at 300). Sovereign: `decodeItem` 1,072 lines (four
    copies). Each field's temporaries are its own, and `err` is redeclared with `:=`.
  - Go `inputEnumBody` (`inputs.go:300`) and `codesIndex` (`fns.go:273`) grow two lines per member (a
    5,000-member enum input gives a 10,015-line `LoadInputs`). `readBits` does **not** grow: `bits`
    needs power-of-two codes, so at most 62 members (WIRE.md:193); the outline was wrong there.
  - **Not growing in practice:** C++ `<P>Make` (members + 3 lines; 131 at most in Sovereign), Go
    `Make_<Record>`, TypeScript (a record's reader is 1.36 lines per field; 10,000 rows are one array
    literal; Sovereign's largest TS function is the 167-line runtime helper `decParse`).
  - **Foreign readers** (C++ `Read_<Pkg>_<T>`, `readers.go:47`; Go `decode_<pkg>_<T>`) hold one local per
    member and make one hook call with every member: 1,513 and 3,671 lines at 300 fields. No Sovereign
    one is near 1,000.
- **What a huge function costs** (measured, capped, under load):
  - a Go `build` of 10,000 rows (90,411 lines): 17.3 s and 1.49 GB; cut into parts of 1,000 lines, 1.72 s
    and 0.45 GB (flat from 100 to 1,000; 3,000 costs +11%);
  - a C++ `Decode` of 3,000 fields: g++ -O2 42.5 s and 1.23 GB, cut at 1,000: 36.5 s and 0.64 GB; clang++
    -O3 -g 33.2 s → 23.6 s;
  - a Go `decode` of 3,000 fields: 12.6 s, cut at 1,000: 1.8 s.
- **Cuts are equivalent:** a 300-field C++ `Wide` cut in two parts and a Go one cut in four give
  byte-identical values and the same first failure on good and bad documents; Sovereign's real
  `Decode(Item)`, `TroupeSkill`, `TargetsSolver`, `Mover` and `EventDef` cut into `<P>Access` members
  compile clean under g++ and clang++ (`-Wall -Wextra -Werror`); its `decodeItem` cut into 43 groups
  passes `go vet`.
- **Tests that bound sizes today:** `TestBulkBuildIsSplit` (`bulk_split_test.go:16`,
  `bulkMaxFnLines` = 2000), `TestBulkCompilesBounded` (`bulk_compile_test.go:64`, -O3 -g ≤ 1.5 GB),
  `TestBigEnumTablesCompile`, `TestEnumHelpersSize`, `TestLargeEnumsCompile`,
  `TestEnumFunctionsDoNotGrow`, `TestBigListsTypecheck`. Nothing bounds `Decode`, Go `build` or Go
  `decode` by width or rows. The largest generator-written function in every golden is 712 lines (Go
  `buildTelemetry` of `examples/telemetry`).
- **The "0.1.11 stack-frame class"** named by the outline is DECISIONS 392's case: `ids.gen.cpp` passed
  15 GB under clang++ -O3 -g because `Build()` held 33,600 inline temporaries in one function (commit
  `4fcb5e0f`), plus make-hook arguments passed directly rather than through locals
  (`baked_foreign.go:17`, `:55-82`).
- **MSVC** documents no line limit (C1053 "function too large" and C4883 carry no number; C4960 names
  65,535 instructions) and no parameter count below 65,535 bytes (C1508): unverified on `cl.exe`.

### Problem

A generator writes functions whose length grows with the data (rows) or the schema (fields), and the
compilers' cost grows faster than linearly in a function's size. Only C++ `Build` is cut today, and its
rule lets a part exceed the budget.

### Decision

1. **One budget: 1,000 lines of body** (the lines strictly between a function's opening and closing
   brace), for C++ and Go. TypeScript is measured against it and has nothing to cut.
2. **The cut:** a body of more than 1,000 lines is cut **between whole units** into parts of **at most**
   1,000 lines, which the function calls in order; a unit alone over 1,000 lines is a part of its own, and
   a function of one statement (a list constant's literal) is as long as that statement. C++ `Build`
   moves from "at least" to "at most" 1,000 (no golden and no Sovereign file has a part today).
3. **What is cut:**
   - C++ `detail::Decode(<Record>)` → `Decode<N>` parts, `static bool Decode<N>(const nlohmann::json& v, canon::json::Decoder& dec, Rec& out);`
     members of `detail::<P>Access` (already a friend of every class, so the header does not change).
     `Decode` keeps the object and key checks, calls each part (`if (!<P>Access::Decode<N>(v, dec, out)) return false;`)
     and ends with `return dec.Ok();`. A part returns `true` at its end and `false` where the original
     returned `false`. The units are one per field (with its `builtDefault` static, path, checks and
     define lookup) and one per stored fn. When any decoder of a file is cut, the `<P>Access` struct, with
     the part declarations first in its body, is written before the cut decoders; a file with no cut is
     byte-identical.
   - Go `build<Last>` → `build<Last>_<N>(d)` parts taking the data pointer (named by the escaped local);
     the units are each allocate statement and each fill entry, in today's order.
   - Go `decode<T>` → `decode<T>_<N>(name, path string, obj map[string]json.RawMessage, out *T) error`
     parts, called as `if err := decode<T>_<N>(…); err != nil { return err }`; the units are one per field
     slot and one per stored fn; the temporary counter keeps counting across parts.
   - Go `inputEnumBody` reads the enum's tables through `Parse<E>` and refuses `Retired()` (the same
     first-declared-wins and the same message); where `Parse<E>` does not exist for an enum kind (the
     builder checks opened and imported enums first), it searches the enum's tables directly. `codesIndex`
     uses the enum's ordinal (§3's dense index or its code search). Both rewrites apply to every enum, whatever
     its size (one form per meaning), so their goldens change: `examples/telemetry`, `internal/gen/go/testdata`'s
     constructs, shop, escapes and geninputs, and the 35 Sovereign Go files that hold such a switch.
4. **Not cut:** variant and dependent tag switches; make hooks; Go list and map constant getters (one
   statement each); the readers of another package's class (a documented limit: a class of more than
   about 200 members read across packages exceeds the budget; cutting them is a redesign of their
   locals, owed in `meta/state.md`).
5. **Names** `Decode<N>` (C++), `build<Last>_<N>` and `decode<T>_<N>` (Go, an interior `_` that
   lowerCamel never writes) are reserved: E8005 through the plan's numbered families.
6. **One constant, shared:** `ir.PartLines = 1000` (exported, in `internal/ir/errors.go`'s const block
   under §17.1's room rule; never a new `constants_*.go` file, which `const-placement` refuses) and a
   pure `ir.PartEnds(lines []int, limit int) []int` (greedy; nil when the units fit). gen/cpp's
   `bakedPartLines` goes. A `partLines` field on each generator, set from `ir.PartLines`, is the test seam
   (`GenerateWithPartLines` in each `export_test.go`).

### Code

- **IR-PARTS** (`internal/ir`): new `parts.go` (`PartEnds`, `CppDecodePart`, `GoPart`); `PartLines` in
  `errors.go`; `cppplan_inner.go` (the `Decode` family reserved in the non-baked branch, where
  `declareDecoders` and `declareAccess` plan the decoders and the access struct, `:35`, `:167`;
  `reserveBulk`, `cppplan_baked.go:216`, runs for baked emits only, which have no decoders); `goplan_scopes.go`
  (a `reserveParts` pass for `build<Last>_<N>` and `decode<T>_<N>`); three findings fixtures.
- **GEN-CPP-PARTS** (`internal/gen/cpp`): new `decode_parts.go`; `decode.go` (each field's text captured
  as a unit); `access.go`; `gen.go` (a deferred section for cut decoders; the `partLines` field);
  `bulk.go` (`splitBody` becomes units plus `ir.PartEnds`, the same text for any `Build` under 1,000
  lines); `errors.go` (−`bakedPartLines`, formats); `export_test.go`.
- **GEN-GO-PARTS** (`internal/gen/go`): new `baked_parts.go` and `decode_parts.go`; `baked.go`; `decode.go`;
  `inputs.go`; `fns.go` (`codesIndex`); `export_test.go`.
- **FNSIZE** (new `internal/testkit/fnsize`): a body measurer for Go (through `go/parser`), C++ and TS
  (table-driven tokenizers that skip comments, strings, raw strings, template literals and preprocessor
  lines): `Go/Cpp/TS(src) []Func{Name, Body, Statements}`; `doc.go`, an `example_test.go`.

### Spec text

CODEGEN, a new §2.11 after §2.10:
> ### 2.11 Function size
>
> A generator never writes a function whose body grows past a line budget because of the data's size. The budget is **1,000 lines of body** (the lines between the opening and the closing brace), the same in every target (DECISIONS D-SIZE).
>
> - **Cut.** A body of more than 1,000 lines is cut between whole statements into parts of at most 1,000 lines, which the function calls in order. A statement is never cut: one longer than 1,000 lines is a part of its own, and a function of a single statement (a list constant's literal) is as long as that statement.
> - **C++.** `<P>Access::Build` is cut into `Build<N>` parts (§7.3); a class's `detail::Decode` is cut into `Decode<N>` parts, static members of `<P>Access` that `Decode` calls in order, each holding whole fields (§7.6). The header is the same with or without the cut.
> - **Go.** `build<Last>` is cut into `build<Last>_<N>` parts that take the baked data (§5.9); a class's `decode<T>` into `decode<T>_<N>` parts that take the object and the value, each holding whole fields.
> - **TypeScript.** Nothing is cut: a record's reader is a line or two per field, and a table of rows is one array literal.
> - **Not cut.** The reader of another package's class (§2.8) holds each member in a local and builds the value through its owner's hook, so it is as long as the class's decoder; a class of more than about 200 members read across packages exceeds the budget. A make hook (a line per member), a variant's or a dependent type's tag switch (a case per case) and a TypeScript reader (about 1.4 lines per field) also follow a type's width, not the data's: they reach the budget only for a type of about 700 members or more, beyond any measured one. Enum helpers (§5.2) and lookups over an enum read the enum's tables, a constant number of lines whatever the members.
> - **Same behaviour.** A cut function reads, fails and reports exactly as the whole did; the first failure is the same.
> - **Names.** `Decode<N>` and `Build<N>` (C++), `build<Last>_<N>` and `decode<T>_<N>` (Go) are generated names (§3.5).
> - **Test.** Generator tests measure every function for a record of 300 members and for a table of 10,000 rows in every target and mode, and for an enum of 5,000 members; none exceeds 1,000 lines unless it is one statement.

CODEGEN §7.3, last paragraph: "and a body longer than about 1,000 lines is cut at top-level statements
into `static void Build<N>(Data& d)` parts that `Build` calls in order." becomes:
> and a body longer than 1,000 lines is cut between top-level statements into `static void Build<N>(Data& d)` parts of at most 1,000 lines that `Build` calls in order (§2.11).

CODEGEN §2.7: after "then the `<P>Access` definition;" insert "(a decoder cut into parts, §2.11, is
defined after the `<P>Access` definition that declares them)". CODEGEN §3.5: after "`<P>Access::Build<N>`
parts (§5.1, §7.3)" insert ", `<P>Access::Decode<N>` parts (§2.11), Go's `build<Last>_<N>` and
`decode<T>_<N>` parts (§2.11)". CODEGEN §5.9: "Go builds them in one unexported function run once through
`sync.OnceValue`" becomes "Go builds them in one unexported function, which calls parts of at most 1,000
lines (§2.11), run once through `sync.OnceValue`". CODEGEN §7.6, after "in field declaration order,":
"a `Decode` of more than 1,000 lines calls `Decode<N>` parts of `detail::<P>Access`, each a run of whole
fields (§2.11),". CODEGEN §8.1: one sentence, "Nothing in a TypeScript file is cut into parts (§2.11).".
CODEGEN §9, the MSVC row's paragraph gains: "A generated function body is at most 1,000 lines (§2.11);
MSVC's reaction to larger ones (C1053, C4883) is not verified on `cl.exe`."

### DECISIONS text

> D-SIZE. **Generated functions stay within a line budget (CODEGEN.md §2.11, §5.9, §7.3, §7.6; amends 392).** No function a generator writes grows past 1,000 lines of body with the data's rows or a record's fields, unless it is one statement or one of §2.11's width-bound shapes (foreign readers, make hooks, tag switches, TypeScript readers). A longer body is cut between whole statements into parts of at most 1,000 lines that it calls in order: C++ `Build` (at most, no longer at least, 1,000) and a class's `detail::Decode` (`Decode<N>`, static members of `<P>Access`, whole fields), Go `build<Last>` and a class's `decode<T>` (`build<Last>_<N>`, `decode<T>_<N>`); TypeScript has nothing to cut. A statement is never cut. The readers of other packages' classes are outside the rule. An enum used as an `input` or as a lookup parameter reads the enum's tables. Reason: a Go `build` of 10,000 rows (90,411 lines) compiled in 17.3 s and 1.5 GB, in 1.7 s and 0.45 GB cut at 1,000; a C++ `Decode` of 3,000 fields took 42 s and 1.2 GB under g++ -O2, 37 s and 0.64 GB cut; a Go `decode` of 3,000 fields 12.6 s, 1.8 s cut.

"[amended by D-SIZE]" goes on 392.

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| testkit/fnsize | `TestGoBodies` (`go/parser`), `TestCppBodies` (braces in strings, raw strings, comments, character literals, digit separators, `#define` lines, inline class members, lambdas counted into their function), `TestTSBodies` (template literals with `${}`; an arrow with a block body counts, an arrow expression does not); cites CODEGEN §2.11 | the measurer | fails: no package |
| ir | `TestPartEnds` (no cut at the limit, an exact fit, a unit over the limit alone, many small units, empty) | the cut | fails: no function |
| ir | `findings/E8005_<next>` ×3: a C++ record named `Decode3` in a `data` emit; a Go `@go(name:)` equal to `build<Last>_0`; one equal to `decode<T>_1` | the families are reserved | fail first |
| build | `TestGeneratedFunctionsStayInTheBudget` (new `budget_test.go`; a pipeline from `.canon` text in a map FS, through `Build`, as `build_test.go` does). Project (a): a 300-member `Wide` (14 type shapes, a wide variant case, a field with a built default) and a 10,000-row table with a nested record, in `types`, `data` and `baked`, emitted for cpp, go and ts; project (b): a 5,000-member `@codes` enum used by a lookup fn, an input field and a list. For every function of every generated file (runtime files and the TS helper block skipped) it asserts `Body <= ir.PartLines \|\| Statements == 1`, logs the maximum per target, mode and construct, and asserts the shape (C++ types `Decode` has at least 2 parts; Go `build` at least 90 parts). Cites CODEGEN §2.11, D-SIZE | **fails** on a `git archive 9ed956e6` scratch tree with `internal/testkit/fnsize` copied in and an `ir.PartLines` shim (the base has no such constant and the test reads it): `Decode(Wide)` 1,211, `Build0` 1,001, Go `decodeWide` 3,262, `buildWbaked` 90,411, `LoadInputs` 10,015, `Weight` 10,004, and the enum helpers until §3 lands. The TS part is a pin: prove it can fail by quadrupling the fixture's members (1,200, about 2,400 reader lines at the measured 2.0 lines per field; the 1,630 forecast assumed 1.36) in a scratch copy |
| gen/cpp | `TestDecodeIsCut` (golden dir `decode-parts`, 5 fields, a limit of 2 through the seam); `TestCutDecodeCompilesAndRuns` (every compiler: a 40-member record with a built default and a variant case, a limit of 60; the program's output equals that of the same package generated uncut); `TestBulkBuildIsSplit`'s bound becomes `ir.PartLines` | the cut and its equivalence | the bound fails first (parts reach 1,003 today) |
| gen/go | `TestBuildIsCut` and `TestDecodeIsCut` (golden dirs `build-parts`, `decode-parts`); `TestCutRuns` (`go vet` and `go test` in a temp module: a 60-row table and a 30-member record, a limit of 100; values and error text equal the uncut generation's); `TestInputEnumUsesTables` (a 2,000-member enum input: `LoadInputs` the same size at 10 and 1,000 members; `inputs_equalwire_test.go` still passes); `TestLookupIndexDoesNotGrow` | the cut, its equivalence, the tables | fail first |
| all | unchanged and green: `TestBulkCompilesBounded`, `TestBigEnumTablesCompile`, `TestLargeEnumsCompile`, every existing golden but Decision 3's lookup and input bodies (no golden holds a function over 1,000 lines) | pins | |

### Acceptance

- `TestGeneratedFunctionsStayInTheBudget` fails on copies of today's files and passes after every unit
  of this section and §3 has landed.
- No existing golden changes but the Go lookup and enum-input bodies of Decision 3 (`examples/telemetry`;
  `internal/gen/go/testdata` constructs, shop, escapes, geninputs); the three new small golden directories appear.
- `make check` green; `make realdata-compile` passes. Sovereign's regeneration changes 10 `.gen.go` files
  (monitoring `feedcat`, `telcat`, `telsem`, `items`; admin, website and wiki `items`; website `display`,
  `recordbook`; wiki `wiki`) by cuts, and the 35 whose lookup or input switches now read the enum's tables (some files are in both: about 41
  distinct files); no C++ and no TS file.

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| FNSIZE | new `internal/testkit/fnsize` | sonnet: a measurer with table-driven tests | S | S2 |
| IR-PARTS | `internal/ir` (`parts.go` new, the `PartLines` line of `errors.go`, `cppplan_inner.go`, `goplan_scopes.go`, three fixtures) | sonnet, opus review | S | HINTS, ENUM-CPP (`cppplan_inner.go`, `errors.go`) and ENUM-GO (`goplan_scopes.go`) |
| GEN-CPP-PARTS | `internal/gen/cpp` | sonnet, opus review: the unit boundaries (a field's static travels with it) | M | IR-PARTS, CPP-ORDERED (the gen/cpp queue) |
| GEN-GO-PARTS | `internal/gen/go` | sonnet, opus review | M | IR-PARTS, ENUM-GO |
| BUDGET | `internal/build/budget_test.go` (new) and its fixture writer | sonnet: shown red on a `git archive 9ed956e6` scratch tree (FNSIZE's package copied in), green on `main` | S-M | FNSIZE, GEN-CPP-PARTS, GEN-GO-PARTS, ENUM-CPP, ENUM-GO |

### Forecast

1. **Go statements that assign `err =`** at function level would not compile in a part; none was seen
   (110 real fields vet clean), but the builder runs `go vet` on every field kind (paths, inline variants,
   pairs, cells, defaults, defines).
2. **C++ units:** a field's `builtDefault` `static const` must travel with its field (the outline's "cut at
   top-level statements" would separate them); define lookups and refinement lines after a field belong
   to it.
3. **Access-struct order** changes only in a file with a cut decoder; data mode's `Load` members call
   `Decode` through its header declaration, so the order is safe (compiled with the struct moved first).
4. **Review findings to expect:** constants in the wrong file (the room rule); one cut loop written twice
   (both generators use `ir.PartEnds`); E8005 coverage of the new families; byte-identity of small
   packages; a missing `Example` in `fnsize`.
5. **Compile-heavy tests:** the cut compile tests stay small (40 members, a small limit through the seam);
   the 300-member and 10,000-row fixtures are generation-only (about 280 MB, 2 s).
6. **Sovereign:** 10 Go files are cut and 35 change their lookup or input bodies (about 41 distinct files); private functions only; `buildTelemetry`'s package gets 3-4 parts
   because its 14,007- and 2,100-line statements stay whole.
7. **Limits logged with their numbers:** foreign readers (1,513 C++ / 3,671 Go lines at 300 fields); a
   `Build` block over 1,000 lines is its own part; Go list constants of 1,329 lines stay one statement.

## 9. Runtime copies of one text agree, by a content stamp (DECISIONS 436; amends 19's CPP-05, and 415)

Scope: ADR-0024 item 7. Research: one opus pass over `internal/gen/cpp/runtime`, the 53 runtime copies
of the measurement tree and Source, with probes compiled and run under g++ 15.2 and clang++ 21.1 for
every option (`/var/tmp/canon-030-research/r-runtime/probe/`).

### Facts

- **The runtime texts are constants of the compiler.** `internal/gen/cpp/runtime/runtime.go:13-18`
  embeds `canon_runtime.h.txt` as `Header` and `canon_runtime_json.h.txt` as `JSON`; `gen.go:168-171`
  writes `canon_runtime.h` into every C++ output directory and `canon_runtime_json.h` unless the mode is
  baked. Nothing in them varies per directory. On the measurement tree all 28 copies of
  `canon_runtime.h` are identical, and so are the 25 of `canon_runtime_json.h` (the real Source has the
  same 28 and 25).
- **Copies from two directories meet in one translation unit.** A `.gen.h` includes `"canon_runtime.h"`
  and then the headers of imported packages (`gen.go:206-213`); on Sovereign, 75 of the 134 `.gen.h`
  include another output directory's header, so two copies routinely meet. CODEGEN §7.1
  (`spec/CODEGEN.md:2034-2036`) designs for it: version guards, "so that identical copies from two
  output directories can meet in one translation unit".
- **Guards and namespace are keyed by version only.** `canon_runtime.h.txt:1-3` (the marker line, then
  `#ifndef CANON_RUNTIME_RT_V1`), `:20-21` (`namespace canon { inline namespace rt_v1 {`);
  `canon_runtime_json.h.txt:2-3` (`CANON_RUNTIME_JSON_RT_V1`) and `:31-33`. The only global state is the
  evaluation-error handler, `inline std::atomic<EvalErrorHandler> g_evalErrorHandler`
  (`canon_runtime.h.txt:43`).
- **The text already changed inside `rt_v1`.** CODEGEN §7.5 says "both are additive to `rt_v1`", but
  `git diff v0.1.13 v0.2.0 -- internal/gen/cpp/runtime/canon_runtime_json.h.txt` (+23/-12) changed
  existing functions: `Decoder::PathOf(std::string_view)` now always writes `<parent>.<key>`, and
  `detail::KeyPath` with it (v0.2's EMPTY-KEY). So a v0.1.13 copy and a v0.2.0 copy in one binary
  violate the one-definition rule today.
- **What happens today, probed** (copy B differs from copy A in an inline body, in `FlatMap`'s layout and
  by one added function; g++ and clang++ agree):
  - (i) one translation unit includes B after A: the guard skips B silently, and code written against
    B's text compiles against A's (a wrong result, or a misleading "not a member" error);
  - (ii) two translation units, one per copy, linked: one body wins by link order;
  - (iii) a value built by code generated against A and read by code generated against B: SIGSEGV.
- **How two texts arise:** `canon build P` writes only the selected packages (CLI.md:74-75); a consumer
  root is built by the consumer's own canon (SPEC.md:360-364); two Canon projects can be linked into one
  program; Sovereign was told on 2026-10-20 that "the first copy included wins and the others compile
  to nothing" (`meta/handoff/2026-10-20-canon-reply-cpp-gaps.md:127-131`).
- **Generated code never names `rt_v1`**: it writes `canon::FlatMap`, `canon::json::Decoder`. Source's
  hand-written code names `canon::FlatMap` at 14 sites in 6 files and never `rt_v1` or
  `CANON_RUNTIME*`. The build recognises a runtime file by its first line only (CODEGEN §2.4,
  `W8030_1.txtar`, `W8030_3.txtar`), so the first line must not change.
- **Pins:** `TestHeaderIsTheSpecs` (`internal/gen/cpp/runtime/spec_test.go:27`) holds CODEGEN §7.4's
  listing (`spec/CODEGEN.md:2136-2536`) equal to `canon_runtime.h.txt` byte for byte (DECISIONS 415);
  `TestSpecNamesTheRuntime` (`:68`) fails when the spec's prose names a `canon::…` the runtime lacks
  (it flags `CODEGEN.md:2131` `canon::rt_v1` and `:2549` `canon::rt_v1::json` once the namespace is
  stamped).
- **Go and TS have no such problem.** Each Go output directory's `rt` is its own package, imported by its
  own path, so two texts are two packages; TS helpers are private to their file.
- `.gitattributes:4` forces LF in every checkout, so a hash of the texts is the same on every OS.

### Problem

The guard says "a copy of `rt_v1` is already here" when it should say "this exact text is already
here". A stale copy silently shadows a newer one in a translation unit, and two different copies in
one binary break the one-definition rule; v0.2.0 already made such a pair possible.

### Decision

1. **A content stamp.** `<stamp>` is the first 12 lowercase hexadecimal digits of the SHA-256 of the two
   embedded texts, `canon_runtime.h.txt`'s bytes then `canon_runtime_json.h.txt`'s. Both texts hold the
   placeholder `@CANON_STAMP@` exactly four times; `runtime.go` writes them with each `@CANON_STAMP@` replaced. One
   compiler writes one stamp everywhere; any change to either text, additive or not, gives another.
   The hash covers both texts because they share one namespace, and a JSON-only change (v0.2.0's) must
   change it.
2. **The check before the unchanged guard, and the stamped namespace.** Each text becomes:
   ```
   // GENERATED by canon: C++ runtime rt_v1. DO NOT EDIT.
   #if defined(CANON_RUNTIME_RT_V1) && !defined(CANON_RUNTIME_RT_V1_@CANON_STAMP@)
   #error "canon: a different canon_runtime.h is already included in this translation unit; regenerate every C++ output with the same canon"
   #endif
   #ifndef CANON_RUNTIME_RT_V1
   #define CANON_RUNTIME_RT_V1
   #define CANON_RUNTIME_RT_V1_@CANON_STAMP@
   …
   namespace canon {
   inline namespace rt_v1_@CANON_STAMP@ {
   …
   }  // namespace rt_v1_@CANON_STAMP@
   }  // namespace canon
   …
   ```
   `canon_runtime_json.h` takes the same lines with `CANON_RUNTIME_JSON_RT_V1` and `canon_runtime_json.h`
   in the message. The first line is unchanged.
3. **What each meeting does** (probed under g++ and clang++, both stop where shown):
   - identical copies in one translation unit: the guard keeps the first, as today;
   - a copy of another stamp after one: the `#error` stops the compile at the include, whatever names
     the translation unit uses, and names the way out (DECISIONS 305);
   - copies of two stamps in translation units that never include both: they share no symbol, so the
     one-definition rule holds; a value cannot pass between them, because any header that declares a
     function over a runtime type includes its own copy, which meets the other copy and stops;
   - **the limit, for copies written before v0.3:** an unstamped copy (canon 0.2.x and earlier) checks
     nothing. Included after a stamped copy, it is skipped by its own guard, as today, and the mix is not
     caught; included before one, the stamped copy's `#error` catches it. A `.gen.h` includes its own
     runtime copy before its imports, so an importer rebuilt with 0.3 over dependencies left at 0.2 is the
     uncaught case. The transition is therefore one step: regenerate every C++ output with 0.3 at once;
     from then on every mismatch fails.
4. **One evaluation-error handler per copy.** `SetEvalErrorHandler` sets the handler of the copy its
   translation unit includes (probed: two stamps, two handlers; identical copies share one). The failure
   mode is the default abort with its message, never a wrong value. The handler is not moved out of the
   stamped namespace: that would reopen a one-definition edge only a test could guard.
5. **No generated API changes.** `canon::FlatMap` still resolves; no `.gen.h` or `.gen.cpp` changes; Go
   and TS are untouched. Mangled names that hold the runtime namespace grow by 14 bytes each. A consumer
   that names `canon::rt_v1::X` explicitly breaks loudly (Sovereign: 0 sites).
6. **`rt_v1` stays the family name.** The "additive" rule of CODEGEN §7.5 stops mattering, since every
   change gives a new stamp. Changing the guard names or the check is `rt_v2`.
7. **Not done:** binding generated headers to their copy (a `static_assert` per header): it rewrites
   every header at every runtime edit and keeps the one-definition violation; whole-project
   `canon build -check` already reports stale outputs in a shared directory.

### Code (`internal/gen/cpp/runtime`)

- `canon_runtime.h.txt` (399 lines → 403) and `canon_runtime_json.h.txt` (1,477 → about 1,481, before
  §10's helpers): the lines of Decision 2.
- `runtime.go` (115 lines):
  ```go
  var (
  	//go:embed canon_runtime.h.txt
  	headerText string
  	//go:embed canon_runtime_json.h.txt
  	jsonText string
  	// Stamp names this compiler's C++ runtime: the first 12 hex digits of the SHA-256 of the two texts, canon_runtime.h's then canon_runtime_json.h's (CODEGEN.md §7.4).
  	Stamp = stampOf(headerText, jsonText)
  	// Header and JSON are canon_runtime.h and canon_runtime_json.h as written: each @CANON_STAMP@ replaced by Stamp.
  	Header = strings.ReplaceAll(headerText, stampMark, Stamp)
  	JSON   = strings.ReplaceAll(jsonText, stampMark, Stamp)
  )
  ```
  with `stampOf` (SHA-256 of the concatenation, hex, cut to `stampLen`).
- `constants.go` (14 lines): `stampMark = "@CANON_STAMP@"`, `stampLen = 12`.
- `spec_test.go`: `TestHeaderIsTheSpecs` compares the spec listing with `headerText` (the text with
  `@CANON_STAMP@`), not `Header`.
- `Identifiers` (read by `internal/ir/tables.go:17` for E8011 `reserved`) scans the written text, so the
  reserved runtime names gain the stamped ones and lose the bare `rt_v1`. Sovereign's 32,398 define
  names meet none of them.
- `gen.go` does not change.

### Spec text

CODEGEN §7.1, the guards sentence (`spec/CODEGEN.md:2034-2036`) becomes:
> Runtime headers use version guards (`CANON_RUNTIME_RT_V1`), never `#pragma once`, so that identical copies from two output directories can meet in one translation unit; a copy of another text stops the compile there (§7.4).

CODEGEN §7.4, the prose before the listing (`:2131-2134`) becomes (no `canon::rt_v1…` token, for
`TestSpecNamesTheRuntime`):
> Written to every C++ output directory. Its names are in the inline namespace `rt_v1_<stamp>` of namespace `canon`, so code uses `canon::FlatMap`. It holds the evaluation-error handler of DECISIONS 19, the checked arithmetic of CONFORMANCE.md §3, the read-only containers and the atomic snapshot pointer.
>
> **Stamp** (DECISIONS D-RTSTAMP). The listing below, like the embedded text of `canon_runtime_json.h` (§7.5), writes `@CANON_STAMP@` where the stamp goes; `canon` writes both with each `@CANON_STAMP@` replaced by the stamp: the first 12 lowercase hexadecimal digits of the SHA-256 of the two texts as embedded, `canon_runtime.h`'s bytes then `canon_runtime_json.h`'s. Every copy one compiler writes holds the same stamp; any change to either text, additive or not, gives another.
> - Identical copies from two output directories meet in one translation unit: the guard keeps the first.
> - A copy included after a copy of another stamp stops the compile with its `#error`, whichever names the translation unit uses. The way out is to regenerate every C++ output directory with the same canon.
> - Copies of two stamps linked into one program, in translation units that never include both, share no symbol, so they cannot break the one-definition rule. Each keeps its own evaluation-error handler: `SetEvalErrorHandler` sets the handler of the copy its translation unit includes.
> - The guard names and the check before each guard are fixed for `rt_v1`; changing them is `rt_v2`.

CODEGEN §7.4's listing: the check's three lines after line 1, `#define CANON_RUNTIME_RT_V1_@CANON_STAMP@` after
the guard's `#define`, and `inline namespace rt_v1_@CANON_STAMP@ {` / `}  // namespace rt_v1_@CANON_STAMP@`. **Held
back from S1:** the listing equals the new `canon_runtime.h.txt` byte for byte, so the orchestrator
copies it from CPP-RT's text in CPP-RT's commit (§17.2), with the two prose blocks above.

CODEGEN §7.5: "Both are additive to `rt_v1`, as are `Decoder::AsOpenEnum` and the `OpenEnum` shortcut,
which read …" becomes:
> `Decoder::AsOpenEnum` and the `OpenEnum` shortcut read an opened enum (§5.2) from any JSON string (DECISIONS 362), and `detail::DecodeOrdered` is `Decode` of an `nlohmann::ordered_json` (§5.13, DECISIONS D-ORDERED).

and "(guard `CANON_RUNTIME_JSON_RT_V1`, namespace `canon::rt_v1::json`), written byte for byte;" becomes:
> (guard `CANON_RUNTIME_JSON_RT_V1`, preceded by §7.4's check on `CANON_RUNTIME_JSON_RT_V1_<stamp>`; namespace `json` of the inline namespace `rt_v1_<stamp>`), written byte for byte but for the stamp (§7.4);

(Also held back to CPP-RT's commit: `TestSpecNamesTheRuntime` reads §7.5 too.)

SPEC §15 (`SPEC.md:1777-1779`), the runtime sentence becomes:
> Both are written once per output directory inside `namespace canon { inline namespace rt_v1_<stamp> { … } }`, `<stamp>` naming their text (CODEGEN.md §7.4), with version guards instead of `#pragma once`: identical copies meet in one translation unit, two different ones there stop the compile, and two different ones linked into one binary share no symbol.

IMPLEMENTATION-PLAN (`:1336`): the C++ cell becomes `namespace canon { inline namespace rt_v1_<stamp> }`
(CODEGEN.md §7.4), and the rule "A breaking change creates `rt_v2`; old and new can link into one
binary." becomes:
> Any change to the C++ runtime's text changes its stamp; two stamps link into one binary and never meet in one translation unit; a change to the guards or the check is `rt_v2`.

### DECISIONS text

> D-RTSTAMP. **C++ runtime copies carry a content stamp (CODEGEN.md §7.1, §7.4, §7.5; SPEC.md §15; amends CPP-05 of 19, and 415).** `canon_runtime.h` and `canon_runtime_json.h` name their inline namespace `rt_v1_<stamp>`, `<stamp>` the first 12 hex digits of the SHA-256 of their two embedded texts, and each copy, before its unchanged version guard, stops the compile with `#error` when a copy of another stamp is already included. Identical copies still meet in one translation unit; copies of two stamps linked into one program share no symbol and keep one evaluation-error handler each. The spec lists the text with `@CANON_STAMP@`. Reason: the guard was keyed by version while the text changed within `rt_v1` (v0.2.0 changed `Decoder::PathOf`), so a stale copy silently shadowed a newer one in a translation unit and two copies in one binary broke the one-definition rule.

"[amended by D-RTSTAMP]" marks go on 19 and 415.

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| gen/cpp/runtime | `TestStamp` (internal test; cites CODEGEN §7.4, D-RTSTAMP) | the embedded texts hold `@CANON_STAMP@` exactly 4 times each; `Stamp` is 12 lowercase hex digits and equals `stampOf(headerText, jsonText)`; `Header` and `JSON` each hold it exactly 4 times, at the four places, and no `@CANON_STAMP@` remains; a one-byte change to either text changes `stampOf` | fails to compile on `9ed956e6` (no `Stamp`) |
| gen/cpp/runtime | `ExampleStamp` | prints `12 true` (the length; `Header` holds `"inline namespace rt_v1_"+Stamp`) | new |
| gen/cpp/runtime | `TestHeaderIsTheSpecs` (pin, now on `headerText`); `TestSpecNamesTheRuntime` | the listing equals the text; the prose names only what exists | pins (shown failing in the scratch run on the code change alone) |
| gen/cpp | `TestRuntimeCopiesMeet` (new `runtime_stamp_compile_test.go`, drivers under `testdata/main/stamp/`, granted to CPP-RT; "another" copy = `Header`/`JSON` with the stamp replaced by `000000000000`) | (a) identical copies of both texts from two directories, using `canon::FlatMap` and `canon::json::Decoder`, compile under `cxx.Flags`; (b) another `canon_runtime.h` after one stops with "a different canon_runtime.h is already included"; (c) the same for `canon_runtime_json.h`; (d) two translation units with identical copies share the handler, two with different stamps do not | fails to compile on `9ed956e6` |

**Golden acceptance:** `find examples internal/gen/cpp/testdata/golden -name canon_runtime.h -exec sha256sum {} + | awk '{print $1}' | sort -u | wc -l`
prints 1, and the same for `canon_runtime_json.h` (READER's frozen v0.2.0 oracle copies, under
`internal/gen/cpp/testdata/reader/`, are outside these two trees on purpose); each copy's diff is the four inserted lines and the
two changed namespace lines (plus §10's helpers in the JSON copies, when CPP-RT carries them).

**Goldens that change:** 76 runtime copies (18 under `examples/`, 58 under
`internal/gen/cpp/testdata/golden/`). No `.gen.h`, `.gen.cpp`, Go or TS file.

### Acceptance

- `TestStamp`, `TestRuntimeCopiesMeet` pass on g++, clang++ and clang++ with libc++.
- The golden sha256 check prints 1 and 1.
- `make check` green, `make diag-check` green (a runtime text changed).
- `make realdata-compile` passes; the C++ units' time and RSS stay within 10% (one Sovereign TU measured
  +0.33-0.39% object size, +14 bytes per symbol, time within noise).

### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| CPP-RT | `internal/gen/cpp/runtime`; granted `internal/gen/cpp/runtime_stamp_compile_test.go` and `internal/gen/cpp/testdata/main/stamp/` (new files), and the golden runtime copies | opus: a runtime contract and C++ semantics | S-M | S2 |

CPP-RT also carries §10's runtime helpers (`RecordOrders`, `DecodeOrdered`), so the runtime text
changes once for both items, and it runs **first** in the runtime package's queue: every later runtime
edit (§4) changes the stamp again in all 76 copies, which is expected and read as such.

### Forecast

1. **The runtime pins fail until the listing lands.** The orchestrator writes §7.4 (listing and prose)
   and §7.5 in CPP-RT's commit, never in S1. Nobody regenerates the spec listing from the code by
   script: it is copied and reviewed.
2. **Every later runtime edit moves the stamp in all 76 copies** (the reader, §4). The golden
   diff of such a unit shows the stamp lines changing in every copy: expected, checked with the sha256
   rule above, never read as noise.
3. **Expected proposals, declined** (Decisions 4 and 7): move the handler out of the stamped namespace;
   stamp the `.gen.h` files.
4. **MSVC** compiles of the `#error` check and the long namespace name are unverified (no CI).
5. **The reply** withdraws the 2026-10-20 sentence "the first copy included wins" and says: regenerate
   every C++ output with 0.3 at once; from then on a mixed tree stops at compile time instead of
   misbehaving (a 0.2.x copy included after a 0.3 one is still skipped: Decision 3's limit).
6. **Sovereign:** 0 sites change. A tree regenerated in part (one package rebuilt with 0.3.0, the
   others left at 0.2.0) is caught only where a 0.2.0 header comes first in a translation unit (Decision
   3's limit); Sovereign regenerates every output at each pin bump, which the reply restates.
7. **New reserved identifiers:** every runtime identifier is a reserved define name (`E8011`). CPP-RT checks
   its new ones (the stamp macros, `RecordOrders`, `DecodeOrdered` and their locals) against Sovereign's
   32,398 define names (`/var/tmp/canon-030-research/r-reader/sovereign-defines.txt` when present, else the
   `*.defines.gen.h` of the measurement tree) and the examples' defines; a collision stops the unit.

## 10. `ordered_json` and kind constants (DECISIONS 437, 438; amend 290, 384, 292)

Scope: ADR-0024 item 8, Louis's v0.2 picks moved here. Research: the R-RUNTIME pass (§9), which
built both in a scratch compiler and ran them under g++ 15.2, clang++ 21.1, Go vet and run, tsc and
node.

### 10.1 A C++ `Decode` of an `nlohmann::ordered_json` (DECISIONS 437)

#### Facts

- DECISIONS 290 (`DECISIONS.md:2731-2737`): "the `types`-mode public `Decode` takes an already-parsed
  `nlohmann::json`, so there a nested table's entries come in key byte order. The public signature keeps
  `nlohmann::json` (an `ordered_json` overload is a later item)." ADR-0022 §10 repeats it; DECISIONS 384
  (ADR-0022 §3) says `Parse` and `Decode<Fn>File` keep file order while `Decode` holds byte order.
- `Decode` is declared by `publicDecodeDecl` (`internal/gen/cpp/types.go:36-42`, format
  `publicDecodeDeclFormat`, `constants.go:176`) and defined from `text/public_decode.txt` (`types.go:44-55`).
  CODEGEN §5.13 (`spec/CODEGEN.md:1751-1754`, `:1760-1765`, `:1847-1852`) states signatures and order.
- **The runtime can already keep order:** `KeyOrders` (`canon_runtime_json.h.txt:257-259`, "an object
  absent here is read in its own order"), `Decoder(name, tokens, orders)` (`:962-969`), `OrderOf`
  (`:1178-1182`), read by `detail::Table` (`:1277`) and the map reader `detail::Members` (`:1312`).
- **Today an `ordered_json` loses its order silently.** nlohmann has converted between `basic_json`
  types since 3.2.0, so `Shelf::Decode(ordered_json)` compiles under `-Werror` and decodes key byte
  order (probe: file order "b a" prints "a b", both compilers). `json_fwd.hpp` declares `ordered_json`
  from 3.9.0, CODEGEN §9's floor; Sovereign vendors 3.11.2.
- **Sovereign:** 767 public `Decode` declarations in 73 headers; hand-written code calls `T::Parse(`
  208 times and `T::Decode(` never; `ordered_json` appears on 26 lines, none passed to a generated
  `Decode`.

#### Decision

1. Every public `Decode` of a C++ `types` emit gains, after the `nlohmann::json` one:
   `static std::optional<T> Decode(const nlohmann::ordered_json& v, std::string& error);`
2. Its definition is one call of a runtime template, so the body is reviewed once, not per class:
   ```cpp
   std::optional<Shelf> Shelf::Decode(const nlohmann::ordered_json& v, std::string& error) {
       return canon::json::detail::DecodeOrdered<Shelf>(v, "Shelf", &detail::Decode, error);
   }
   ```
3. The runtime (`canon_runtime_json.h.txt`, in the `detail` block after `class Decoder`, before
   `detail::Object`) gains `RecordOrders` and `DecodeOrdered`:
   ```cpp
   inline void RecordOrders(const nlohmann::ordered_json& in, const Json& out, KeyOrders& orders) {
       if (in.is_array()) {
           for (size_t i = 0; i < in.size(); ++i) RecordOrders(in[i], out[i], orders);
           return;
       }
       if (!in.is_object()) return;
       std::vector<std::string> keys;
       keys.reserve(in.size());
       for (auto it = in.begin(); it != in.end(); ++it) {
           keys.push_back(it.key());
           RecordOrders(it.value(), *out.find(it.key()), orders);
       }
       if (!std::is_sorted(keys.begin(), keys.end())) orders.emplace(&out, std::move(keys));
   }
   template <class T>
   std::optional<T> DecodeOrdered(const nlohmann::ordered_json& v, const char* name, bool (*decode)(const Json&, Decoder&, T&), std::string& error) {
       const Json doc(v);
       KeyOrders orders;
       RecordOrders(v, doc, orders);
       const Float32Tokens tokens;
       Decoder dec(name, tokens, orders);
       dec.Push("$");
       std::optional<T> out;
       if (decode(doc, dec, out.emplace())) return out;
       error = dec.Error();
       return std::nullopt;
   }
   ```
   CPP-RT writes them (§9), so the runtime text changes once.
4. Maps and nested tables keep the order the value holds, as `Parse` keeps file order. Like `Decode` of
   an `nlohmann::json`, it sees no repeated key and no number token (an `ordered_json` holds neither).
5. A call `Decode({}, e)` becomes ambiguous; Sovereign has none. `= delete` on `ordered_json` was
   declined: it gives no message, and Louis picked the feature.

#### Spec text

CODEGEN §5.13, the signature block gains, after the `json` `Decode`:
> `static std::optional<EventConfig> EventConfig::Decode(const nlohmann::ordered_json& v, std::string& error);`

and the first sentence of its first bullet becomes:
> The C++ `Decode` takes an already-parsed value. Handed an `nlohmann::json`, whose objects are ordered by key, a nested table or a map it decodes holds its entries in key byte order, not file order (the `data`-mode loader, which reads the text, keeps file order; DECISIONS 290). Handed an `nlohmann::ordered_json`, it holds them in the order that value holds, the file's for a value parsed from text, as `Parse` does (DECISIONS D-ORDERED).

In "Paths and order", "key byte order in C++" becomes:
> key byte order for an `nlohmann::json` and the value's own order for an `nlohmann::ordered_json` in C++

(CODEGEN §7.5's sentence naming `detail::DecodeOrdered` is §9's, held back to CPP-RT.)

#### DECISIONS text

> D-ORDERED. **C++ `Decode` takes an `nlohmann::ordered_json` too (CODEGEN.md §5.13, §7.5; amends 290, 384).** Every public `Decode` of a C++ `types` emit gains `static std::optional<T> Decode(const nlohmann::ordered_json& v, std::string& error)`, a call of the runtime's `detail::DecodeOrdered`: maps and nested tables keep the order the value holds, as `Parse` keeps file order; like `Decode` of an `nlohmann::json`, it sees no repeated key and no number token. Reason: 290's later item; without it nlohmann converted an `ordered_json` to `nlohmann::json` silently and the decode held key byte order.

#### Tests (`internal/gen/cpp`)

| Test | Proves | First |
|---|---|---|
| `TestTableFieldTypesCompilesAndRuns` (extended; `tabletypes_main.cpp` gains an `ordered <file>:` line per case) | the ordered decode equals `Parse` for `order.json` and `Decode` for `dup` and `dupextra` | fails on `9ed956e6`: it compiles and prints `a:1,b:2` |
| `TestOrderedDecodeCompilesAndRuns` (new `ordered_compile_test.go`, fixture with a map, a map of maps and a nested table, `testdata/main/ordered_main.cpp`; cites CODEGEN §5.13, D-ORDERED) | a file order unlike byte order is kept; the first failure is the first in file order (`$.power.b`); a repeated key; an empty object | fails on `9ed956e6` |

**Goldens that change:** 24 types-mode files (+1 line per class in `.gen.h`, +4 per class in
`.gen.cpp`). Sovereign's C++ grows by about 3,800 lines.

#### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| CPP-ORDERED | `internal/gen/cpp` (`types.go`, `text/public_decode.txt`, the format line in `errors.go`'s decoder block, tests, drivers) | sonnet: golden- and run-checked | S | BAKED-FIND (the gen/cpp queue), CPP-RT (its runtime helpers) |

### 10.2 Kind constants (DECISIONS 438; amends 292)

#### Facts

- DECISIONS 292 (`DECISIONS.md:2746-2753`): a constant of a variant's kind is `E8019`
  `VariantKindConstant` "until the generators write the kind enum's member"; CODEGEN §4.4 (`:982-983`)
  states it; ERRORS.md rows `VariantKindConstant` (`:207`) and `WayVariantKindConstant` (`:231`).
- Reported by `checkKindConstants` and `holdsVariantKind` (`internal/ir/rules_gen.go:85-98`), registered
  in `common` (`:21`) and `tsCode` (`:35`); its way-out pair is `internal/ir/tables.go:115`.
  Example source: `internal/ir/testdata/findings/E8019_92.txtar:15-18`
  (`const K = Reward.gold.kind`, `const KS = [Reward.gold.kind]`; also `_93`, `_94`, and
  `internal/build/testdata/builds/const_kind_pair.txtar`).
- **The IR drops the variant:** `refOf` (`internal/ir/convert.go:11-19`) has no `VariantKind` entry, so a
  kind's `TypeRef` has a nil `Named`. Only constants carry kind types to the generators (translated
  code uses `IsCase`).
- **The generators already write kind types and literals almost everywhere:** Go writes kind types and
  literals (`gotype.go:26-27`, `literal.go:96-103`) but `consts.go:18-33` fails on a kind; TS writes them
  (`ts/types.go:32-33`, `ts/literal.go:47-48`) but `values.go:26-33` lacks the case; C++ handles a kind
  in none of `ctype.go` storage, `literal.go`, `enumTyped`, `arrayElem` (`bulk.go:47-74`) or
  `literalType` (`baked_fill.go:74-87`), and `gen.go:110`'s comment says a kind "cannot be written".
- Sovereign has no kind constant among its 5,125 consts. A map keyed by kinds cannot be written in
  source (a brace literal keyed by kinds is `E3305`).

#### Decision

1. **The IR keeps the variant:** `refOf[types.VariantKind] = (*stage).kindRef`, a `TypeRef{Kind:
   types.VariantKind, Named: <the variant>}`.
2. **The refusal retires:** `checkKindConstants`, `holdsVariantKind`, their registrations and the
   way-out pair go; the orchestrator deletes the two ERRORS.md rows in KIND's commit (held back from S2,
   which would otherwise drop names the code still uses) and runs diaggen there.
   The refusal is removed in the same unit as the generators' cases, never before, so `check` never
   passes what the build cannot write (DECISIONS 37).
3. **What each target writes** (scratch output):
   - Go: `var K = ShapeKindDot` (the kind enum is opaque, so a const of it is a `var`, CODEGEN §5.2);
     `var Bk = b.RewardKindGold`; a list `func Ks() rt.List[ShapeKind] { return rt.MakeList([]ShapeKind{ShapeKindDot, ShapeKindBox}) }`;
   - C++, after the enums: `inline constexpr ShapeKind K = ShapeKind::dot;`,
     `inline constexpr ::b::RewardKind BK = ::b::RewardKind::gold;`,
     `inline const std::vector<ShapeKind> KS = {ShapeKind::dot, ShapeKind::box};` (8 or more elements
     read a `detail::k…Elems` array, DECISIONS 392);
   - TS: `export const K: ShapeKind = "dot";`, `export const BK: b.RewardKind = "gold";`,
     `export const KS: ReadonlyArray<ShapeKind> = canonFreeze<ReadonlyArray<ShapeKind>>(["dot", "box"]);`
     (with TS-PURE, `= /*#__PURE__*/ canonFreeze…`).
   - A kind of another package's variant needs that package's emit of the target, as any reference does
     (`E8004`).
4. **Code:** `gen/go/consts.go` gains `case types.VariantKind:` (a `var` declaration); `gen/ts/values.go`
   adds `VariantKind` to the `Enum, LitUnion, Ref` case; `gen/cpp` gains the `VariantKind` cases in
   storage, literal (`*value.CaseKind`), `enumTyped`, `constant`, `arrayElem` and `literalType`, through
   two helpers `kindOf` and `kindLit` that call `malformed` when `Named` is not a variant, and fixes
   `gen.go:110`'s comment.

#### Spec text

CODEGEN §4.4: delete "A constant of a variant's kind is `E8019` (DECISIONS 292)."

CODEGEN §5.1's table gains a row:
> | variant kind (`Shape.dot.kind`) | `var X = ShapeKindDot` | `inline constexpr ShapeKind X = ShapeKind::dot;` | `export const X: ShapeKind = "dot";` |

and, after the table:
> A list or map of kinds follows the list or map row (DECISIONS D-KINDCONST).

#### ERRORS.md (applied in KIND's commit by the orchestrator, not by S2)

Delete the rows `VariantKindConstant` (`spec/ERRORS.md:207`) and `WayVariantKindConstant` (`:231`).

#### DECISIONS text

> D-KINDCONST. **A constant of a variant's kind is written (CODEGEN.md §4.4, §5.1; ERRORS.md §1.4; amends 292).** Its value is the kind enum's member: Go a `var` (the kind enum is opaque, §5.2), C++ an `inline constexpr`, TS a typed `export const` of the case's wire; lists and maps of kinds follow the list and map rows. `E8019` `VariantKindConstant` retires. Reason: 292 refused it only "until the generators write the kind enum's member"; they write it everywhere else.

#### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| ir | `E8019_92`, `_93`, `_94.txtar` move to `internal/ir/testdata/gensupport/kind_constant_go.txtar`, `kind_constant_cpp_types.txtar`, `kind_constant_ts.txtar`, wanting 0 errors | the three targets accept kind constants | fail on `9ed956e6` (E8019) |
| ir | new `findings/E8004_<next>.txtar` | a cross-package kind constant without the other package's ts emit: `E8004 b.Reward of package b is used by this ts emit, but b has no ts emit` | fails on `9ed956e6` (E8019) |
| build | `builds/const_kind_pair.txtar` keeps only its 2 `E8012`; new `builds/const_kind.txtar` (packages a and b) builds everything | the build writes them | fail first |
| gen/go | `TestKindConstants` | the generated text, and a run printing `true true 2 8 dot` | fails first |
| gen/cpp | `TestKindConstantsCompile` | the header lines, and `static_assert`s that compile | fails first |
| gen/ts | a toolchain case | tsc then node | fails first |

`make diag-check` must pass after diaggen. No golden changes (no example holds a kind constant).

#### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| KIND | `internal/ir` (`convert.go`, `rules_gen.go`, `tables.go`, testdata); `internal/gen/go/consts.go`; `internal/gen/ts/values.go`; `internal/gen/cpp` (`literal.go`, `ctype.go`, `bulk.go`, `baked_fill.go`, `gen.go`'s comment); the two build txtars | sonnet: small cases in each generator, all compile- or run-checked | S | S2 |

KIND is first in the gen/cpp, gen/go and gen/ts queues, and its `ir` files are disjoint from HINTS',
ENUM-0's and LOCAL-ANN's, which run beside it (§17.2).

#### Forecast

1. Go names `KS` as `Ks` (its naming rule); expected.
2. No new constant in `ir` (500 lines), `gen/go` (496) or `gen/ts` (385); the C++ helpers' formats go in
   `gen/cpp/errors.go` (§17.1's room rule).
3. Sovereign: 0 sites; nothing to migrate.

## 11. N8: output templates (DECISIONS 439; resumes 229's deferral)

Scope: ADR-0024 item 9. Research: one opus pass over the project file's grammar and schema, the three
places that read an emit's `out`, every emit of Sovereign's 433 (`emits.py` over
`/var/tmp/canon-030-measure/proj/Resource/Config`), and a probe built from `HEAD`.

### Facts

- **The project file's grammar is generic** (GRAMMAR.md:1102-1107: `projectItem = WORD (":" pValue |
  BraceList(pEntry))`), so a new key needs no EBNF change; keys are dispatched by `keyRules`
  (`internal/project/constants.go:37-62`) through `schema.go:74-90` (`E1002` unknown, `E1005`
  duplicate). Precedent: DECISIONS 332 (`optional_roots`) and 380 (`tiers`) added keys the same way.
- **Every project-file string is constant, and the parser enforces it for every key**
  (`internal/syntax/parse_amend.go:149-170`: `projectValue` calls `constString`, `E1132`;
  `internal/project/schema.go:294-302`: `isBad` treats any interpolated string as already refused; pinned
  by `internal/project/testdata/findings/interpolation_1.txtar`). So `"internal/gen/{dir}"` is `E1132`
  today.
- **Emits are declared in each package** (`emitDecl`, GRAMMAR.md:612; options are a built-in schema never
  resolved in scope, CODEGEN.md:111-121). `out` must be a constant string, or a list of them except for
  `view`. **Three places read it from syntax:** `internal/check` (`emit.go:153-166` `constOption`,
  `emitout.go:26-101`, `emit.go:101-115` `defaultGoPackage`, `emitpath.go:14-28`), `internal/ir/emits.go:185-196`
  (`outTexts`, with `resolveOut` at `:229-257`) and `internal/build/strays_foreign.go:84-120` (the stray
  check of DECISIONS 356). `ir.Emit.Out` is commented "for messages only" but build re-resolves it
  (`build/emit.go:259`, `crossroots.go:57`, `dircase.go:67`): it must stay a resolvable path.
- DECISIONS 229 (`DECISIONS.md:1991-2003`): "A named group of destinations in `project.canon` is deferred
  until the repetition shows." DECISIONS 215: the default Go package is the last element of `out` as
  declared. DECISIONS 424: `out` segments use only `A-Z a-z 0-9 . _ -`.
- **Canon already has name-only templates:** GRAMMAR §8.2's `template` kind, used by `@json(pairs:)`'s
  `{i}` and `@files`' `{id}`.
- Generated markers name the package directory, not `out` (CODEGEN §2.4), so an expansion equal to a
  literal writes byte-identical files; FINGERPRINT and LOCK never read `out`.
- Two packages a Go emit reaches that share a Go package name are already refused with a way out
  (`E8005 go: vocab is generated for both … give one of their emits a distinct package option`, probed).
- **The repetition, measured** (the outline's "75 of 80" changed): 85 Go emits with 185 `out` entries; 158
  are exactly `@<root>/<that root's layout>/<package path as directories>` and 2 more differ only by a
  lower-cased segment; the layout is per root (`internal/gen` for admin, website, wiki and trello;
  `internal/platform/gen` for monitoring); 72 of 85 emits have every entry on the pattern; all 85 write
  `package:`, whose values follow no one rule (first and last segments 54, last 13, other 18). TS: 48 of
  51 entries follow `@<root>/web/src/lib/generated[/resource]/<dotted>.ts`. View: 109 of 111 follow
  `@resource/Studio/views/<dotted>.view.json`. C++: 21 of 116 fit the package directory (99 group by the
  package's first segment).

### Problem

An emit's destination repeats its root's layout and its own package path in every package, by hand: 158
Go entries, 48 TS and 109 view, each a place where a typo writes a wrong directory silently.

### Decision

1. **A key `out_templates` in `project.canon`:** a map from an emit target to a map from a declared root
   to a path template relative to that root:
   ```
   out_templates {
     go: {
       admin: "internal/gen/{dir}"
       monitoring: "internal/platform/gen/{dir}"
     }
     ts: { admin: "web/src/lib/generated/{dotted}.ts" }
     view: { resource: "Studio/views/{dotted}.view.json" }
   }
   ```
   (Nested maps take `:`; only top-level keys take the block form, `format/file.go:150`.) Keyed by target
   then root, a template can only serve its target and only write under its root, by construction.
2. **Two placeholders:** `{dir}` (the package path with `/`: `items/vocab`, the directory the marker names)
   and `{dotted}` (the package path as written: `items.vocab`). Nothing else: no `{first}`, no case
   transforms, no Go package-name pattern (Go ties the package name to the last import-path element,
   Sovereign's names follow no rule, and E8005 already names the way out); the outline's "Go package-name
   templates" are decided out. The Go package keeps DECISIONS 215's default: the last element of the
   expansion, or `package:`.
3. **An emit names a root instead of writing a path:** `emit go { out: [admin, monitoring], package:
   "itemsvocab", mode: types }`. A bare word in `out` is a root reference, never looked up in scope. Its
   expansion is `@<root>/` followed by the template; from then on the expansion is the entry for every
   rule that judges an `out` (`outChars`, `tsOut`, `outRoot`, `outPackage`, `outForm`, `outConsumer`, `E8004`,
   `E8007`, `E8008`, `E8022`, `E8025`, `E8026`-`E8029`, `E8150`, `E8152`, `W8024`, `W8030`, ownership,
   markers, the default package), reported at the word.
4. **The parser accepts interpolations under `out_templates` only:** it exempts the value of that one
   top-level key from `E1132` and resets after it, so every other project string keeps today's refusal byte
   for byte. The template reader bypasses `isBad`.
5. **Refusals, each with its way out:**
   - `out_templates` not a map of targets to maps of roots to strings: `E1006` `outTemplates`; a repeated
     target or root: `E1005`; in `project.local.canon`: `E1014` (existing);
   - a target outside `go`, `cpp`, `ts`, `json`, `view`, `text`: `E1019` `target`; an undeclared root:
     `E1009` `templateRoot`;
   - a template: an interpolation other than a bare `{dir}` or `{dotted}` is `E1019` `placeholder`; a
     leading `@` is `rooted`; an empty path, a leading or trailing `/`, `//`, or a `.` or `..` segment is
     `path`; a character outside `A-Z a-z 0-9 . _ -` and `/` is `chars` (424, by construction for the
     placeholders); a `ts` template not ending in `.ts` is `ts`;
   - in an emit: a root name with no template for the target is `E8009` `outTemplate`; a quoted string equal
     to the name of a root that has a template for the target is `E8009` `outQuoted` (today that string is a
     directory beside the file, and C++, view, text and json would silently write into the source tree):
     write the name without quotes, or `"./<name>"`.
6. **Not refused:** an unused template (it writes nothing); a literal `out` equal to an expansion (a
   literal pins a path; a template follows the package). A template with no placeholder is allowed (E8008
   and E8152 catch two emits writing one directory or file).
7. **One reader of `out`:** an exported `check.OutTexts(p, pkg, target, value)` replaces the copies in
   `internal/ir/emits.go` and `internal/build/strays_foreign.go`, so the three cannot drift; `ir.Emit.Out`
   holds the expansion (build re-resolves it). That changes the meaning of a frozen field
   (IMPLEMENTATION-PLAN §4.5: "Out as written, for messages only"), so TPL-CHECK makes the change under
   §4's review rule, with this ADR as the approval, and §4.5's comment follows (Spec text).
8. **Interactions:** edit re-checks with the current templates and never writes `project.canon`; rename
   has nothing to rename; the formatter prints strings verbatim; the lock and fingerprints are unaffected;
   `project.canon` is in the read set, and a template change is a full re-open (DECISIONS 380).
9. **Language version:** the key and the bare-word form add syntax and change no existing source's
   meaning (a bare word in `out` is `E8009` today), so `canon: "0.1"` stands (§18).

### Code

- **TPL-PROJECT** (`internal/project`): a new `templates.go` (about 150 lines: the schema rules, sorted
  storage by (target, root), a pure `Expand(target, root, pkg)`); `schema.go`, `project.go`, `constants.go`
  (128 lines). Granted in `internal/syntax`: `parse_amend.go` (the exemption, keyed on the top-level key
  and reset after it), `parser.go` (one field), `constants.go` (485 lines; one key constant shared with
  project).
- **TPL-CHECK** (`internal/check`): `emitout.go` (or a new `emittemplate.go`), `emit.go`; the exported
  `OutTexts`. Granted: `internal/ir/emits.go` and `internal/build/strays_foreign.go` (their readers replaced;
  net about −15 lines each). No new constant in `check` (499 lines) or `ir` (500).
- **TPL-DOCS:** one example package moved to templates with an identical expansion (and the example's
  `project.canon` key), a formatter golden for `out_templates`. The guide recipe is GUIDE's.

### Spec text

GRAMMAR §7 (`spec/GRAMMAR.md:1110`), the strings bullet:
> … Strings are constant strings (`E1132`), except the templates under `out_templates`, whose interpolations are placeholders (§7.1).

GRAMMAR §7.1, a row after `go_module`:
> | `out_templates` | map emit target → map root name → path template | `{}` | where an emit's `out` that names a root writes (CODEGEN.md §2.1, DECISIONS D-TPL): `out_templates { go: { admin: "internal/gen/{dir}" } }`. A target is an emit target word (`go`, `cpp`, `ts`, `json`, `view`, `text`; `E1019` `target`); a root is a declared root written as an identifier (`E1009` `templateRoot`), once per target (`E1005`). A template is a string relative to its root whose only interpolations are the placeholders `{dir}` (the package path with `/`, its directory) and `{dotted}` (the package path as written) (`E1019` `placeholder`); it does not start with `@` (`rooted`); it is a path of non-empty segments, with no leading or trailing `/` and no `.` or `..` segment (`path`); its text uses only `A-Z a-z 0-9 . _ -` and `/` (`chars`, DECISIONS 424); a `ts` template ends in `.ts` (`ts`). Any other value is `E1006` `outTemplates`. A template no emit names writes nothing. Not allowed in `project.local.canon` (`E1014`) |

GRAMMAR §5.3 (`:625-627`), the `out` sentence:
> … `out` is a string or a root name (a word naming a template of `out_templates`, CODEGEN.md §2.1), or, for every target but `view`, a list literal of them (DECISIONS 229, D-TPL).

CODEGEN §2.1, after `:94`:
> - **Templates** (DECISIONS D-TPL). An `out` entry may be a root name instead of a path string: `emit go { out: [admin, wiki] }`. It stands for the template `project.canon`'s `out_templates` gives that root for this target (GRAMMAR.md §7.1), expanded for this package: `@<root>/` followed by the template, `{dir}` replaced by the package path with `.` → `/` (`items/vocab`, the directory §2.4's marker names) and `{dotted}` by the package path (`items.vocab`). The name is never resolved in scope. From then on the expansion is the entry: every rule of this document and of WIRE.md that judges an `out` entry judges it (`outChars`, `tsOut`, `outRoot`, `outPackage`, `outForm`, `outConsumer`, `E8004`, `E8007`, `E8008`, `E8022`, `E8025`, `E8152`, ownership, markers, and the default `package`, the last element of the expansion, DECISIONS 215), at the name. A name that is no root with a template for this target is `E8009` `outTemplate`. A string entry equal to the name of a root that has a template for this target is `E8009` `outQuoted`: write the name without quotes, or `"./<name>"` for the directory beside the file.

and in CODEGEN §2.1's typing bullet (`:114-116`), the words "and `out` a constant string or, except for
`view`, a list of them" become "and `out` a constant string or a root name or, except for `view`, a list of
them", and the words "a non-string element of an `out` list is `E8009` `kind` with `OutPaths`" become "an
element of an `out` list that is neither a string nor a root name is `E8009` `kind` with `OutPaths`". In the
**Copies** bullet (`:100-101`), "`out` is a string or a non-empty list of strings; `view` takes one string"
becomes "`out` is a string or a root name, or a non-empty list of them; `view` takes one string or one root
name". (S1 matches these words across the file's line breaks.)

IMPLEMENTATION-PLAN §4.5 (`:544`), the comment of `Out`: `// Out as written, for messages only` becomes:
> `// Out as written, or a root name's expansion (CODEGEN.md §2.1): a resolvable path, for messages and build's checks`

GRAMMAR §12: a row after `E1018` (`spec/GRAMMAR.md:1398`):
> | E1019 | error | an invalid `out_templates` target or template (§7.1) |

and the `E1009` row (`:1389`) ends "…, or an empty/invalid module path, or an `out_templates` root that is not a
declared root". CODEGEN §12's `E8009` row (`spec/CODEGEN.md:3215`) gains "; an `out` root name with no
template for its target, or a quoted root name (§2.1, DECISIONS D-TPL)".

WIRE §2 (`spec/WIRE.md:45`):
> Applies to every path string in `load`, `load.*`, `emit … { out: … }` (an `out_templates` template's expansion included, CODEGEN.md §2.1), `asset(…)` roots and `@files` templates.

### ERRORS.md (applied by S2)

§1.4 Kinds: a row before `OutPaths`, `| OutPath | a path string or a root name | E8009 |` (view's kind error
uses it instead of `ConstantString`); `OutPaths`' word becomes "a path string, a root name or a list of
them".

Codes: a new row `| E1019 | error | project | GRAMMAR.md §7.1 | an invalid out_templates target or template: not an emit target, a placeholder other than {dir} and {dotted}, a rooted, malformed or non-portable path, or a ts template not ending in .ts (DECISIONS D-TPL) |`; E1009's meaning appends ", or an `out_templates` root is not a declared root"; E8009's meaning appends ", an `out` root name with no template for the target, or a quoted root name (DECISIONS D-TPL)".

Messages:
| Code | Variant | Args | Template |
|---|---|---|---|
| E1006 | outTemplates | - | `"out_templates" must be a map of emit targets to maps of root names to path templates` |
| E1009 | templateRoot | name:Name | `out_templates: "{name}" is not a declared root` |
| E1019 | target | name:Name | `out_templates: "{name}" is not an emit target: expected go, cpp, ts, json, view or text` |
| E1019 | placeholder | text:Expr, target:Name, root:Name | `out_templates: {text} in the {target} template of root {root} is not a placeholder: a template may use only {{dir}} and {{dotted}}` |
| E1019 | rooted | target:Name, root:Name | `out_templates: the {target} template of root {root} starts with @: write it relative to the root, without @{root}/` |
| E1019 | path | target:Name, root:Name | `out_templates: the {target} template of root {root} is not a relative path of non-empty segments: no leading or trailing /, no //, no . or .. segment` |
| E1019 | chars | target:Name, root:Name, char:Rune | `out_templates: the {target} template of root {root} holds {char}: its text may use only A-Z a-z 0-9 . _ - and /` |
| E1019 | ts | root:Name | `out_templates: the ts template of root {root} must end in .ts` |
| E8009 | outTemplate | name:Name, target:Name | `{name} in out of emit {target} names no {target} template of out_templates in project.canon: add one there, or write the path as a string` |
| E8009 | outQuoted | name:Name, target:Name | `out of emit {target} holds the path "{name}", the name of a root with a {target} template: write {name} without quotes to use the template, or "./{name}" for the directory beside this file` |

(The S2 go-dev follows ERRORS.md §1's argument kinds; if one named here does not exist, it uses the
closest existing kind and the orchestrator logs it. `E8009_11` and `E8009_21` change wording with the
`OutPaths` kind: their txtars follow through `-update`, message text only.)

### DECISIONS text

> D-TPL. **Output templates: an emit's `out` may name a root, and `project.canon` gives the path (GRAMMAR.md §7, §7.1, §5.3; CODEGEN.md §2.1; WIRE.md §2; ERRORS.md E1006, E1009, E1019, E8009; resumes 229's deferral; reads 215, 269, 424).** `project.canon` takes `out_templates`, a map from an emit target to a map from a declared root to a path template relative to that root, whose only interpolations are `{dir}` (the package path with `/`) and `{dotted}` (the package path as written); the parser accepts interpolations under this key only. An `out` entry written as a root name stands for that root's template for the emit's target, expanded for the emit's package (`@<root>/…`); the expansion is the entry for every later rule, reported at the name. A name with no template is `E8009` `outTemplate`, a quoted root name that has one `E8009` `outQuoted`. The Go package keeps 215's default, the last element of the expansion. Not taken: a placeholder or pattern for the Go package name (Go ties it to the last element; the one large project's names follow no single rule, and `E8005` already names the way out), `{first}` and case transforms (one consumer's layout), named groups of roots (17 distinct root sets, none dominant), refusing unused templates (a template writes nothing) or literals equal to an expansion (a literal pins a path). Reason: 158 of 185 Go `out` entries, 48 of 51 TS entries and 109 of 111 view entries of Sovereign's Resource repeat `@<root>/<that root's layout>/<package path>`; a typo in one entry wrote a wrong directory silently.

"[amended by D-TPL]" goes on 229.

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| project | `findings/E1019_1..6.txtar`, one per variant: `target`; `placeholder` (with `{dir:5}` and `{f()}`); `rooted`; `path` (with `a//{dir}`, `../{dir}`, a trailing `/`); `chars` (with a raw `r"x/{dir}"`); `ts` | the refusals | fail first (`E1002` today) |
| project | `E1009_4` (`templateRoot`), `E1006_14` (a string value, a list value), `E1005_5` (a repeated target and root), `E1014_7` (in `project.local.canon`), `clean_7` (every target, both placeholders, a cpp template with no placeholder) | the schema | fail first |
| project | `interpolation_1.txtar` unchanged; new `interpolation_2.txtar`: a key after `out_templates` still gets `E1132` (the exemption resets) | the exemption is scoped | `interpolation_1` is a pin (prove it can fail by moving its `{x}` string under `out_templates`); `interpolation_2` fails first (`out_templates` is `E1002` today) |
| syntax | `TestProjectTemplatesInterp` | no `E1132` under `out_templates`, `E1132` under `roots` | fails first |
| check | `E8009` txtars: `outTemplate` (an undeclared root; a root with no template for the target), `outQuoted` (`"./admin"` accepted); judged after expansion: `outRoot` for `[admin, admin]`, `outPackage` for differing templates without `package`, `outChars` for a root path with a space | the emit side | fail first |
| check | `TestOutTemplateExpansion` | `{dir}`, `{dotted}`, both, none, a single-segment package | fails first |
| ir | `TestTemplateOutMatchesLiteral` | `Dir`, `GoImport`, `GoPackage` and `FileName` equal a literal's (no C++ compile) | fails first |
| build | `template_outputs.txtar`; a pin that the same package with literal outs and with templates builds byte-identical files; `strays_template.txtar` (`build --check P` beside Q's templated output gives no `E8029`) | the build | the stray case fails if `strays_foreign.go` keeps its own reader (prove in a scratch copy) |
| format | a golden: `project.canon` with `out_templates` is a fixed point | the formatter | new |
| examples | one example package moved to templates; its `expected/` tree byte-identical | end to end | fails first (a bare word in `out` is `E8009` today); its `expected/` tree does not move |

### Acceptance

- Every new findings case fails on copies of today's files and passes after.
- The converted example's `expected/` tree is byte-identical.
- `make check` green. The Resource dry run is unaffected (Resource does not use the key; the reply offers
  the migration: 72 Go, 33 TS and 109 view emits, byte-identical outputs).

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| TPL-PROJECT | `internal/project`; granted `internal/syntax/parse_amend.go`, `parser.go`, `constants.go` and the syntax test file of `TestProjectTemplatesInterp`, and a mutation operator for `E1019` in `internal/testkit/progen` (`ops_project_test.go`: the coverage ratchet needs one, and `gaps.txt` may only shrink, `coverage_test.go:35`) | sonnet, opus review for the parser exemption | M | S2 |
| TPL-CHECK | `internal/check`; granted `internal/ir/emits.go`, the comment of `Out` at `internal/ir/ir.go:178` (§4.5's text) and the ir test file of `TestTemplateOutMatchesLiteral`, `internal/build/strays_foreign.go` and the two build txtars (`template_outputs`, `strays_template`); a frozen field's meaning (`ir.Emit.Out`) under §4's review rule | opus: `internal/check` and a frozen contract (orchestration.md's opus rule) | M | TPL-PROJECT, DEFAULT-FOLD (the check queue) |
| TPL-DOCS | the converted example package, the examples' `project.canon` key, the formatter golden; granted the VS Code grammar's scope snapshots of the files it changes (`internal/testkit/vscodegrammar/testdata/scopes/`, `TestScopeSnapshots` reads every example `.canon` file, `project.canon` included) | sonnet | S | TPL-CHECK |

### Forecast

1. **Two traps:** the parser exemption must be keyed on the top-level key and reset after it
   (`interpolation_2`); `isBad` marks interpolated strings as refused, so a template reader that does not
   bypass it silently drops every template.
2. **Forgetting the build reader** (`strays_foreign.go`) gives a false `E8029` or `W8030` on a scoped
   `build --check` (`strays_template.txtar`).
3. **Builders may push a Go package pattern or `{first}`:** both are decided out; log them as owed if pressed.
4. **Determinism:** templates sorted by (target, root).
5. **Sovereign:** 0 sites break (the feature is additive). Adoption is optional and byte-identical.

## 12. N3: `canon lint`, advisory lints for agents (DECISIONS 440)

Scope: ADR-0024 item 10, "as specified in the 2026-10-12 roadmap's §3 N3"
([handoff](../handoff/2026-10-12-canon-next-versions.md):158-178), plus its extra lint (legacy member names
that a C++ define also spells). Research: the R-PROJECT pass (§11), with a syntax probe over Sovereign's
8,599 source files and a grep-level estimate of each lint.

### Facts

- **`canon edit` exists** (CLI.md §3.15, `internal/cli/edit.go:16-32`): it reads one request in API.md §8.8's
  JSON form from a file or stdin, prints JSON lines (DECISIONS 274) and prints an undo request. So
  "`--fix` as `canon edit` requests" means printing complete §8.8 request objects.
- API.md S5: any source change makes every other request on the old base stale; S6: a `Base` of `""`
  disables staleness ("for scripts and tests"). API.md E31: a `RenameName` op is alone in its request; E34
  adds `@json("<old wire>")` when a load or a data-mode emit reaches the field's record, **but does not
  count `@text` fn results** (`internal/edit/renamewire.go:148-272`): renaming a field that only a `@text`
  output writes changes that output's key.
- **Lint findings are catalogue codes** (DECISIONS 27, the audit's `diag-message-inline`): codes match
  `^[EW][0-9]{4}$`, the package must be a row of IMPLEMENTATION-PLAN §3 (`internal/diag/catalog/parse.go`),
  each ERRORS.md section title starts with a code range. `W00xx` is free; the sections run from `E10xx`
  (`spec/ERRORS.md:392`), so a `W00xx` section goes first.
- **The precedent for an agent command** is `canon graph` (API.md §5.6, DECISIONS 396, `api/graph.go`):
  additive API under IMPLEMENTATION-PLAN §4's review rule. `build.Analysis` does not expose the stage-E
  IR; v0.2 granted a one-line `Analysis` accessor as "not a frozen contract" (log-2026-11-01).
- **Reusable code:** `ir.CppGuardNames`, `ir.TextFiles`, `ir.PlanCppNames`, the name set `foreignDefine`
  judges (`ir/rules_cppforeign.go`), edit's `@files` expansion (`edit/place.go`, unexported), load's
  WIRE §6.8 `#define` reader (`load/defines.go`, unexported).
- **Guide tests bind the CLI:** `TestGuideCommandTable` (`internal/cli/guide_cli_internal_test.go:107`)
  needs the guide's command table to list exactly the binary's commands.
- **Estimated findings on Sovereign** (grep and AST level; the full analysis was not run):
  - legacy prefix: 11 (`dw` ×8, `szMainProbName`, `lpPoint`, `nLog`); without the type check 15, the 4 extra
    `Float` weights (`wDps` …) being false positives;
  - an integer field holding only 0 and 1: 7, about half of them counts (`aoMin`, `dwItemCountMin`);
  - numbered siblings: 35 same-type groups, all in wire-bound legacy records (excluded by the rule), so
    about 0;
  - a String limited to enum member names: 4, all over several enums;
  - `{k: true for …}`: 29, mostly legitimate lookups;
  - an authoring-note wire name an output writes: 2 (`ai.Mover.comment` `_comment`,
    `items.Item.skillRange` `_dwSkillRange`);
  - canon's marker words in a source string: 6;
  - entry files off their `@files` path: 7,266 of 7,266 `items` entries (one finding per `let`);
  - files over 5,000 lines: 19 (17 mostly one `let`, 2 id enums);
  - C++ names a text output `#define`s with no guard: `ui.WindowId` (1,051 of 1,051) and `ui.ControlId`
    (212 of 212): 2 findings.
- A lint costs the same analysis as `check` (about 47 s and 2.5 GB on Sovereign) plus cheap passes.

### Problem

The legacy smells DECISIONS 3 and 21 forbid in models have no check: the 2026-10-12 Resource review counted
them by hand, and the 2026-10-29 macro clash (its Case 2) had a remedy but no detection.

### Decision

1. **`canon lint [packages…] [--fix] [--layer …]`** reads the analysis `canon check` makes for the same
   selectors and reports lint findings only. It never changes `check`, `build` or their exit codes, so a
   `--max-warnings 0` gate on them is unaffected (precedent DECISIONS 322). Its output is always JSON
   lines, whatever `--format` says (DECISIONS 274): one finding per line, then the summary line. `-q`
   changes nothing. It writes no file and prints no package's source (DECISIONS 188). Exit: 0 when it ran
   (findings or not), 2 for a usage error, 3 for an internal error, 4 when its warnings exceed
   `--max-warnings`; never 1. It does not require a project without findings; a broken declaration
   (TYPES §1) and a value the analysis could not compute are never judged.
2. **Eight lints, as catalogue warnings `W0001`-`W0008`**, package `lint`, owner API.md §5.7:
   - `W0001` a field name that keeps a legacy type prefix;
   - `W0002` numbered sibling fields one list would hold (variants `list`, `pairs`);
   - `W0003` a String limited to enum member names (variants `single`, `several`);
   - `W0004` an authoring-note wire name (starting with `_`) that an output writes;
   - `W0005` a source string holding the words of canon's generated marker;
   - `W0006` entry files off the path their `@files` template gives (one finding per `let`);
   - `W0007` a source file over 5,000 lines held by one `let`;
   - `W0008` a C++ name that a text output or a loaded header `#define`s and no guard covers.
3. **Two of the roadmap's lints are decided out** (a deviation from "as specified", logged):
   - "an integer field whose values are all 0 and 1": on the only real project half its findings are
     counts, and its advice ("type it `Bool`") would lead an agent to change a type silently, the class of
     mistake Canon exists to prevent;
   - "the `{k: true for …}` idiom": a map of `true` is Canon's only constant-time membership (there is no set
     type), and its misuse cannot be told from its shape.
4. **`--fix`** prints, before the findings, one line per finding that one edit request fixes:
   `{"fix":{"code":"W0001","file":…,"line":…,"col":…,"request":{"base":"r1:…","ops":[…]}}}`, `request` a complete
   API §8.8 request based on the revision linted. Only `W0001` has a fix in v0.3: a `renameName` to the
   suggested name, offered only when that name is an identifier, not a reserved word, not already the name
   of another field or method of the record or case, **and the field's record is reached by no `@text` fn
   result** (E34 keeps the old wire name for loads and data emits, not for `@text` outputs, so a fix there
   would silently change an output). A fix is current only at its revision; after one applies, the others
   are stale (API S5): lint again. `Base: ""` is never written (a positional path such as `[3]` could point
   elsewhere after an earlier fix).
5. **API:** `func (p *Project) Lint(ctx context.Context, packages ...string) (*LintResult, error)`, with
   `LintResult{Findings []Finding; Fixes []LintFix; Summary Summary; Revision Revision}` and
   `LintFix{Code string; Span; Edit Edit}`: additive to the frozen `api/canon.go`, under IMPLEMENTATION-PLAN
   §4's review rule with this ADR as the approval, as DECISIONS 396 was.
6. **Package `internal/lint`** (after `edit` in IMPLEMENTATION-PLAN §3; consumes `build`, `edit`, `ir`,
   `load`); `api` consumes `lint`. `lint.Run(ctx, a *build.Analysis, s *edit.Snapshot)` fills per-package
   bags in F2 order (truncated at 1,000 per package) and returns the fixes; one analysis per call.
7. **Determinism:** findings in F2 order; `W0006`'s first file by path bytes; `W0008`'s names sorted; fixes
   in finding order.

### Code

- **LINT-CORE:** the new package (`doc.go`, `constants.go`, the framework, `W0001` with its fix, `W0005`,
  `W0007`, the test harness, an `example_test.go`); granted `internal/build/analysis.go` (+3 lines: an
  `IR() []*ir.Package` accessor). The orchestrator writes, in LINT-CORE's commit: the ERRORS.md `W00xx`
  section, the IMPLEMENTATION-PLAN §3 `lint` row and `api`'s Consumes cell (held back from S1 and S2:
  diaggen requires the package row, and the deps tests require the package), then runs diaggen, sets
  `internal/diag/render_test.go`'s count (583 → 593) and regenerates the messages golden (`-update`). Lint writes
  output, so the same commit adds `lint` to the determinism lane's packages (`maprange`): IMPLEMENTATION-PLAN
  §7.5's list (`:1072-1075`) and `tools/audit/internal/lanes/determinism/constants.go:14` (granted; a tightening).
- **LINT-DATA:** `W0002`, `W0003`, `W0004`, `W0006`, `W0008` in `internal/lint`; granted, with no new
  constant (`edit/constants.go` is full): `internal/edit/renamewire.go` (add an exported reachability that also counts `@text` results, for `W0004` and
  the fix condition; `onWire`, `:148`, and so `canon rename`'s E34, unchanged), `internal/edit/place.go` (export the entry path
  from a record), a new `internal/ir` file exporting the plan-name set `foreignDefine` judges, and
  `internal/load/defines.go` (export the WIRE §6.8 define names).
- **LINT-API-CLI:** `api/lint.go` and its example; `internal/cli/lint.go`, `cli.go` (+1 line),
  `cli/constants.go`; the guide's `cli.md` command row (the guide test fails without it); five command txtars (no deps
  allowlist entry: `consumesAllowlist` only shrinks, and `api` reaches `lint` as IMPLEMENTATION-PLAN §3 says). The orchestrator writes, in its commit, every spec block of this section
  that S1 held back (API.md §5.7, CLI.md §1's row and §3.19, IMPLEMENTATION-PLAN §8.1's row).
- **Rule citations.** `TestEveryAPIRuleHasATest` (`internal/testkit/rules_coverage_test.go`) fails on a
  declared API.md rule no test cites as `API.md <id>`, and on a citation of an undeclared one, and a txtar
  outside the registered harness directories may cite no rule. So until LINT-API-CLI the lint txtars cite
  `API.md §5.7` followed by prose, never by a code word (the citation pattern reads `API.md §5.7 W0001` as a
  citation of a rule `W0001`), and LINT-API-CLI, in the commit that adds §5.7, registers
  `internal/lint/testdata/findings` among the harness directories of `internal/testkit/rules_scan_test.go`
  and rewrites each txtar's citation to its rule: `W0001_*` cites `API.md L2`, `W0002_*` `L3`, `W0003_*` `L4`,
  `W0004_*` `L5`, `W0005_*` `L6`, `W0006_*` `L7`, `W0007_*` `L8`, `W0008_*` `L9`, `fix_1` `L10`, and
  `ExampleProject_Lint` `L1` and `L11`.

### Spec text

**Every block of this section is held back from S1** (§17.2): the API.md and CLI.md blocks and
IMPLEMENTATION-PLAN §8.1's row land in LINT-API-CLI's commit, IMPLEMENTATION-PLAN §3's row in LINT-CORE's.

API.md, a new **§5.7 Lint (DECISIONS D-LINT)** after §5.6:
> ```go
> func (p *Project) Lint(ctx context.Context, packages ...string) (*LintResult, error)
> type LintResult struct { Findings []Finding; Fixes []LintFix; Summary Summary; Revision Revision }
> type LintFix struct { Code string; Span; Edit Edit }   // a complete request, Base = Revision
> ```
> - **L1.** Lint reads the analysis Check reads for the same selectors (S8, R2) and returns the lint findings of the selected packages: warnings of the codes `W0001`-`W0008`, which no other call reports, in F2 order, truncated as F7 says. It does not require a project without findings; a broken declaration (TYPES.md §1) and a value the analysis could not compute are never judged. Layers apply as for every read.
> - **L2. Legacy prefix (`W0001`).** A field of a record or case whose Canon name is one of these prefixes followed by an upper-case letter: `dw`, `sz`, `lp` or `str`, whatever its type; `n`, `w`, `i` or `u` when its type, under `?`, aliases and refinements, is an integer, an enum or a `ref`; `f` for a `Float`; `b` for a `Bool` or an integer; or `m_` followed by a letter. The finding suggests the name without the prefix, its leading capitals lowered, except, in a run of two or more capitals, the last one when a lower-case letter follows (`dwItemCountMax` → `itemCountMax`, `dwID` → `id`, `szURLPath` → `urlPath`).
> - **L3. Numbered siblings (`W0002`).** Two or more fields of one record or case, of one written type, none with `@json(pairs:)`, whose names are one stem followed by distinct decimal numbers (`slot1`, `slot2`): variant `list` when no `load` and no output (L5) reaches the record or case, never through a `ref`; variant `pairs` when exactly two such families of it share the numbers 0 to n-1 (WIRE.md §5.14 maps them onto one list). Other shapes are not reported.
> - **L4. Enum names (`W0003`).** A `String`-based type refined by `it in X` or `X.contains(it)`, where X is, or names a `let` whose value is, built only from enum member names: `[m.name for m in E.members …]`, a `+` of such lists, `.unique()` of one, or `{n: true for n in …}` over one. Variant `single` for one enum, `several` for more.
> - **L5. Authoring notes (`W0004`).** A field whose wire name (WIRE.md §5.5.2) starts with `_`, of a record or case an output reaches, never through a `ref`: the declared type of a value an `emit json`, or a `go`, `cpp` or `ts` emit in mode `embedded`, `data` or `types`, writes, or the result type of a `@text` fn.
> - **L6. Marker words (`W0005`).** A string literal outside `test` declarations whose text holds `GENERATED by canon` or `Code generated by canon`, the words of CODEGEN.md §2.4's markers.
> - **L7. Entry files (`W0006`).** A `let` with `@files` that has at least one `entry` declared outside the `let`'s file at a path other than the one N2-N3 give its evaluated entry; once per `let`, at the annotation, naming how many and the first such file in path order with its path by the template. An entry whose template cannot be expanded (N2's `ErrBadValue`) is not judged.
> - **L8. Big files (`W0007`).** A package source file of more than 5,000 lines whose longest top-level declaration is a `let`.
> - **L9. Text defines (`W0008`).** A name a selected package's C++ emit declares (the names CODEGEN.md §7.9's `foreignDefine` judges) that an object-like `#define` (WIRE.md §6.8's reading; function-like macros skipped) of a `@text` output of the analysis named `*.h`, `*.hh`, `*.hpp` or `*.hxx` spells, or that a `load.defines` table of the analysis reads, and that the package's guard set (§7.9) lacks; once per declaration, naming how many, the file, and a suggested `@cpp(defines:)` prefix: the names' longest common prefix, cut after its last `_`.
> - **L10. Fixes.** A `W0001` finding whose suggested name is an identifier, not a reserved word, named by no other field or method of its record or case, and whose record no `@text` fn result reaches, has a fix: `{Base: Revision, Ops: [RenameName("<pkg>:<Rec>.<field>", "<name>")]}`, or `"<pkg>:<Variant>.<case>.<field>"` for a case's field (E27; §8.9; E34 keeps the old wire name). No other code has a fix in this version. A fix is current only at `Revision`: once one applies the others are stale (S5); lint again.
> - **L11. Order.** Fixes follow Findings; two calls on one snapshot return the same result.

and API S8's list of reads (`API.md:264`) gains `Lint` after `Graph` (the same commit).

CLI.md §1, a row: ``| `canon lint [packages…]` | advisory lints of legacy smells, as JSON lines; `--fix` adds `canon edit` requests |``.

CLI.md, a new **§3.19 `canon lint`** (`canon lint [packages…] [--fix] [--layer …]`):
> Lists the advisory lints of spec/API.md §5.7 for the selected packages, from the analysis `canon check` makes (DECISIONS D-LINT): legacy smells that are valid Canon, as warnings `W0001`-`W0008`, which no other command reports. `canon check` stays the verdict: lint never changes what `check` or `build` report or exit with, so a `--max-warnings 0` gate on them is unaffected. It does not require a project without findings; run `canon check` first when the lints must be of an error-free project. `canon lint` is a command for agents: its output is always JSON lines, whatever `--format` says (DECISIONS 274): one finding per line (§2.4), then the summary line. With `--fix`, the findings are preceded by one line per finding that one edit request fixes: `{"fix":{"code":"W0001","file":…,"line":…,"col":…,"request":{"base":"r1:…","ops":[…]}}}`. `request` is a complete request for `canon edit` (§3.15), based on the revision linted: apply one, then lint again, since the others are stale once a source changes (API.md S5). Lint writes no file and prints no package's source (DECISIONS 188). `-q` changes nothing. Exit: 0, 2, 3, 4.

IMPLEMENTATION-PLAN §8.1, a row: ``| `lint` (always) | with `--fix`, first `{"fix":{"code","file","line","col","request":{…}}}` per fix in finding order; then one finding per line (API.md F5); then check's summary |``.
IMPLEMENTATION-PLAN §3, a row after `edit` (held back to LINT-CORE): ``| `lint` | the advisory lints of `canon lint` and the edit requests that fix them | API.md §5.7, CLI.md §3.19 | build, edit, ir, load |`` (the package name is the first cell's code span, which diaggen and the deps tests read); `api`'s Consumes becomes "workspace, lint".

### ERRORS.md (held back to LINT-CORE's commit, written by the orchestrator)

A new section, before `## E10xx, W10xx` (`spec/ERRORS.md:392`):

> ## W00xx: Lints
>
> Owner: API.md §5.7. Reported by `canon lint` only (CLI.md §3.19, DECISIONS D-LINT).
>
> | Code | Severity | Package | Owner | Meaning |
> |---|---|---|---|---|
> | W0001 | warning | lint | API.md §5.7 | a field name keeps a legacy type prefix |
> | W0002 | warning | lint | API.md §5.7 | numbered sibling fields that one list would hold |
> | W0003 | warning | lint | API.md §5.7 | a String limited to enum member names |
> | W0004 | warning | lint | API.md §5.7 | an authoring-note wire name (starting with `_`) that an output writes |
> | W0005 | warning | lint | API.md §5.7 | a source string holding the words of canon's generated marker |
> | W0006 | warning | lint | API.md §5.7 | entry files off the path their `@files` template gives |
> | W0007 | warning | lint | API.md §5.7 | a source file over 5,000 lines held by one `let` |
> | W0008 | warning | lint | API.md §5.7 | a C++ name that a text output or a loaded header `#define`s and no guard covers |
>
> | Code | Variant | Args | Template |
> |---|---|---|---|
> | W0001 | - | field:Name, prefix:Name, name:Name | `field {field} keeps the legacy prefix {prefix} in its Canon name: rename it {name} (canon rename keeps the old wire name with @json where a load or a data emit reads the field)` |
> | W0002 | list | first:Name, last:Name | `fields {first} to {last} are numbered siblings: make them one list` |
> | W0002 | pairs | first:Name, last:Name, key:Name, value:Name | `fields {first} to {last} are two numbered columns: one list with @json(pairs: ["{key}{{i}}", "{value}{{i}}"]) keeps the wire` |
> | W0003 | single | typ:Name, enum:Name | `{typ} is a String limited to the member names of {enum}: type it {enum}` |
> | W0003 | several | typ:Name, enums:Names | `{typ} is a String limited to the member names of {enums}: type it with the enum, or with a dependent type on the field that decides which enum` |
> | W0004 | - | field:Name, wire:Name, output:Name | `field {field} goes to the wire as {wire}, an authoring note, and {output} writes it: move the note into a /// comment, or give the field a wire name without _` |
> | W0005 | - | - | `this string holds the words of canon's generated marker: an output built from it claims to come from canon's generators; give it a marker of its own` |
> | W0006 | - | let:Name, count:Int, file:Path, want:Path | `{count} entry files of {let} are off the path its @files template gives; first: {file}, which belongs at {want}` |
> | W0007 | - | lines:Int, decl:Name, declLines:Int | `this file has {lines} lines, {declLines} of them in {decl}: hold its rows as entries in files of their own (@files)` |
> | W0008 | - | decl:Name, count:Int, file:Path, prefix:Name | `{count} names of {decl} are #defined by {file}, which no guard covers: add @cpp(defines: "{prefix}") to {decl}, or rename them with @cpp(name:)` |

(diaggen checks section order and the count sentence; if it refuses a `W00xx` section first, the orchestrator
places it where diaggen requires and logs it: diaggen wins. `W00xx` codes get no `gaps.txt` line: the progen
coverage ratchet counts only codes the packages `internal/build` depends on name.)

### DECISIONS text

> D-LINT. **`canon lint`: advisory lints for agents (API.md §5.7; CLI.md §1, §3.19; ERRORS.md W00xx; IMPLEMENTATION-PLAN §3, §8.1; follows 3, 21, 274; precedent 322, 396).** `canon lint [packages…]` reads the analysis `canon check` makes and reports, as warnings `W0001`-`W0008` that no other command reports: a legacy type prefix in a field name, numbered sibling fields a list would hold, a String limited to enum member names, an authoring-note wire name an output writes, canon's marker words in a source string, entry files off their `@files` path, a source file over 5,000 lines held by one `let`, and a C++ name a text output or a loaded header `#define`s with no guard. It never changes `check`, `build` or their exit codes, so a `--max-warnings 0` gate is unaffected. Output is JSON lines whatever `--format` says (274). `--fix` prints, before the findings, one complete `canon edit` request per finding one request fixes (`W0001` only, and only where no `@text` result reaches the field: a `renameName`), based on the revision linted; it never prints or writes a package (188). `Project.Lint` is additive (IMPLEMENTATION-PLAN §4 review rule, as 396). Decided out: an integer field holding only 0 and 1 (half its findings on Resource are counts, and its advice would change a type), and the `{k: true for …}` idiom (a map of `true` is Canon's only constant-time membership). Reason: the 2026-10-12 Resource review counted these smells by hand, DECISIONS 3 and 21 had no check, and the 2026-10-29 macro clash (Case 2) had a remedy but no detection.

### Tests

| Package | Test | Proves | First |
|---|---|---|---|
| lint | `testdata/findings/W0001_1` … `W0008_1.txtar`, one per lint, citing `API.md §5.7` until LINT-API-CLI rewrites it to the rule id | the lints | fail first (no package) |
| lint | negatives: `W0001_2` (the clean name is taken, so no fix; `wDps: Float` is silent; a field reached by a `@text` result has no fix); `W0002_2` (a wire-bound non-pairs group is silent), `W0002_3` (`pairs`); `W0003_2` (`several`); `W0006_2` (entries at their template path are silent); `W0008_2` (guarded names are silent); `clean_1` (a native project gives no finding: a pin, proved by adding a `dwX` field) | no false positive | |
| lint | `fix_1.txtar` with `fixes.txt` | the exact request JSON; lint again after applying it gives none | fails first |
| api | `ExampleProject_Lint` | `Revision` equals the fix's base | new |
| cli | `lint_json` (JSON even with `--format text`), `lint_fix` (fix lines first), `lint_max_warnings` (exit 4), `lint_unknown` (exit 2), `lint_errors` (exit 0, the lints of the sound parts) | the command | fail first |
| testkit | the deps tests (`deps_test.go`, `TestConsumesColumn`) accept the `lint` row | the package graph | |

### Acceptance

- Every lint has its txtar; the fix round-trips through `canon edit`.
- `check` and `build` outputs and exits are unchanged on every example (their goldens do not move).
- `make check` green. In the Resource dry run, `canon lint` is run once, read-only, and its counts go in the
  reply (expected near the estimates above).

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| LINT-CORE | new `internal/lint`; granted `internal/build/analysis.go` | sonnet, opus review | M | S2, KIND (the diaggen order) |
| LINT-DATA | `internal/lint`; granted `internal/edit/renamewire.go`, `internal/edit/place.go`, a new `internal/ir` file, `internal/load/defines.go` | sonnet, opus review | M | LINT-CORE, HINTS (the name set it exports), EDIT-KEYED (`internal/edit`), FEEDBACK (`internal/load`) |
| LINT-API-CLI | `api/lint.go` and its example; `internal/cli` (`lint.go`, `cli.go`, `constants.go`, five txtars); `internal/cli/guide/cli.md` (its row); granted `internal/testkit/rules_scan_test.go` and the citation lines of `internal/lint`'s txtars | opus: an additive frozen contract (orchestration.md's opus rule) | M | LINT-DATA, TEST-CLI (`internal/cli`, `api`) |

### Forecast

1. **The API change is additive to a frozen contract:** opus review under IMPLEMENTATION-PLAN §4's rule.
2. **Every lint must skip broken declarations** (v0.2's MACRO-IR review FAIL repeated the lesson).
3. **The fix loop costs one analysis per applied fix** (about 47 s on Sovereign; the 11 `W0001` fixes about
   9 minutes). Selectors narrow it.
4. **Reviewers may call `W0002`-`W0003` noise:** their definitions are narrow on purpose (wire-bound groups
   excluded; several-enum lists get the softer `several` text).
5. **The E34/`@text` gap** stays owed (§19); the fix condition keeps lint from stepping into it.

## 13. The scale bench, and the edit bench's targets re-based (DECISIONS 441)

Scope: ADR-0024 item 11 (its numbers go to Louis for Q3, the parked on-disk cache) and the owed
`make bench-edit` targets (v0.2's recorded miss). Research: the R-SIZE pass (benchgen calibrated against
Resource's real `canon check`) and the R-OWED-CORE pass (the targets' history).

### Facts

- **benchgen** (`internal/testkit/cmd/benchgen`) has one size knob, `-n` (default 7,000,
  `constants.go:27`), plus `-seed` and `-out`. Its project: `n` item entry files, `n` icon placeholders,
  `n` JSON twins, monsters `n/14`, a define header with `n` `#define`s, three packages, and no emit.
  `make bench` runs `TestCheckCleanDefaultN` under 6G and logs time only; `make bench-edit` measures a cold
  `canon check -q` with peak RSS through the `ru_maxrss` helper (`editbench_rss_test.go`). No scale option
  exists.
- **Resource's own check, measured today** (a scratch copy, `HEAD` compiler): 142 packages, 8,601 `.canon`
  files, 702,763 lines, 21.7 MB of source; 29.9 s in process (30.2 s wall) and 1.97 GB peak RSS.
- **benchgen, measured:** `n` = 1,000: 0.46 s, 151 MB; 7,000: 2.55 s, 955 MB; 14,000: 4.95 s, 1.85 GB
  (linear in that range: 0.343 ms and 127.5 KB per entry). At 15,000 entries it matches Resource's source
  bytes (about 20.6 MB) and its check memory (about 1.98 GB), but its time is 5.8× lower than Resource's
  (3 packages, no emit, light evaluation).
- **Estimates, not run** (a linear fit; the research did not run 3× or 5×): 3× (45,000 entries) about 15.6 s
  and 5.8 GB; 5× (75,000) about 25.9 s and 9.6 GB. Scaled from Resource's own shape: about 91 s and 5.9 GB,
  and 151 s and 9.8 GB.
- Live heap is about 70% of peak; GC settings reach only 1.4 GB at 14,000 entries. `canon check` has no
  on-disk cache (`.canon/` is never created).
- **The edit bench's targets** (`internal/testkit/cmd/benchgen/editbench_test.go:33-34`: `targetEdit` =
  300 ms, `targetEval` = 150 ms) come from IMPLEMENTATION-PLAN §7.6 (`:1097-1098`). Their p95 history: M4
  0.267 s and 15 ms (passed); v0.1.5 and v0.1.6 about 0.32 s and 0.24 s; v0.1.13 against v0.2.0: Edit 0.333
  against 0.343 s, Evaluate 0.268 against 0.264 s, refresh 0.469 against 0.470 s. The Evaluate number
  changed meaning at v0.1.5 ("the first reader of an edited snapshot pays the every-package analysis… the
  cost moved from the untimed Value into Evaluate", ADR-0021 §6; DECISIONS 382 "only the sum is stable").
  `make bench-edit` has exited 2 on every tree since then, so the gate gates nothing.

### Decision

1. **`make bench-scale`**, part of `make bench` (a report, never a gate): a new opt-in test
   `TestBenchScale` (linux) writes the benchmark project at 1×, 3× and 5×, where **1× is 15,000 entries**
   (Resource's source bytes and check memory, the two things Q3 is about; no new benchgen knob, which only
   one consumer would use). For each size, one at a time, in `TMPDIR=/var/tmp`: generate, count the
   `.canon` files, lines and bytes, run one cold `canon check -q` in a fresh process through the existing
   `measured()` helper (wall time and the child's peak RSS), delete the directory. A size whose check exits
   non-zero fails the run ("a project with errors is not measured"). It prints a table (with the machine
   and the load average at the start) and appends one TSV line per size to `-benchgen.scaleout`
   (default `/var/tmp/canon-bench-scale.tsv`, outside the repository). Flags: `-benchgen.scales "1 3 5"`
   (space-separated; **its default is empty, which skips the test**, so `make check`'s `go test ./...` never runs
   it), `-benchgen.unit 15000`, `-benchgen.scaleout`.
2. **One cap for the run, 12G** (`BENCH_SCALE_CAP ?= 12G`), its scope started with `-p OOMPolicy=continue`
   (the user scope's default, `stop`, would kill `go test` with the check): the 5× estimate is 9.6 GB, and
   Louis's machine has 29 GB shared with other sessions. A check killed by a signal is recorded, not failed:
   its TSV row's status is `killed` and its RSS the cap, and the larger sizes are skipped. `measured()` and
   `rssHelper` learn to report a signal death (today `measured()` gives no measurement on a non-zero exit,
   `editbench_rss_test.go:102-105`, and `rssHelper` turns a signal into exit 255). A 5× kill is the 5×
   result ("more than 12 GB"); the run is not repeated with more memory.
3. **Resource's measured row** (29.9 s in process, 1.97 GB today) is re-measured by the orchestrator at
   release and written beside the bench's table in the log and the reply (the test does not hard-code it);
   the bench's time axis is a lower bound for a project of Resource's shape.
4. **The edit bench is re-based, with no performance work:** `targetEdit` = 450 ms (v0.2.0's 0.343 s ×
   1.25, rounded up to 50 ms, DECISIONS 382's rule), and `targetEval` is reported, not gated (only the
   refresh sum is stable, 382). A miss blocks a release only as a B/A regression over 10%, as 382 says.

### Code (`internal/testkit/cmd/benchgen`, the Makefile)

- New `scale_test.go` (about 140 lines: `TestScaleSizes`, pure, and `TestBenchScale`, opt-in);
  `editbench_rss_test.go` (`measured()` and `rssHelper` report a signal death).
- `editbench_test.go:33-34`: the two targets; `editbench_set_test.go:61`'s
  `seconds(p95(s.evals), targetEval, true)` passes `false` (reported, not gated), with its own reason text
  in place of `notGated`'s "log M4 U7b" (`editbench_test.go:43`): "only the refresh sum is gated (DECISIONS
  382, 441)"; the Makefile's `bench-edit` comment (`Makefile:257-260`, "Gated: … Edit, Evaluate and
  refresh p95") drops Evaluate.
- Makefile:
  ```
  BENCH_SCALES    ?= 1 3 5
  BENCH_UNIT_N    ?= 15000
  BENCH_SCALE_CAP ?= 12G
  BENCH_SCALE_OUT ?= /var/tmp/canon-bench-scale.tsv
  .PHONY: bench bench-check bench-scale
  bench: bench-check bench-scale
  bench-check:   # today's bench recipe, unchanged, under 6G
  bench-scale:
  	systemd-run --user --scope -q -p MemoryMax=$(BENCH_SCALE_CAP) -p MemorySwapMax=0 -p OOMPolicy=continue env GOTOOLCHAIN=local TMPDIR=/var/tmp \
  	  go test -count=1 -timeout 0 -run '^TestBenchScale$$' -v ./internal/testkit/cmd/benchgen \
  	  -benchgen.scales '$(BENCH_SCALES)' -benchgen.unit $(BENCH_UNIT_N) -benchgen.scaleout $(BENCH_SCALE_OUT)
  ```
  The `bench` recipe's comment says both targets are opt-in and too heavy for `make check`.

### Spec text

IMPLEMENTATION-PLAN §7.6, after the "Gates" bullet:
> - **Scale bench** (`make bench-scale`, part of `make bench`; a report, never a gate). `benchgen` writes the benchmark project at 1x, 3x and 5x, where 1x is 15,000 entries: the size of Resource's own sources (about 20 MB of `.canon` text) and the memory of its whole `canon check` (about 2 GB). For each size it runs one cold `canon check` in a fresh process and prints and records the wall time and the process's peak RSS, with the machine and the load average at the start. Its time axis is a lower bound for a project of Resource's shape (3 packages, no emit, light evaluation). Its numbers decide whether the on-disk cache leaves its parked state.

IMPLEMENTATION-PLAN §7.6's table (`spec/IMPLEMENTATION-PLAN.md:1096-1098`): the `Edit` row's target cell
"≤ 300 ms" becomes "≤ 450 ms"; the `Evaluate` row's target cell "≤ 150 ms" becomes "reported, not gated (only
the refresh sum is stable, DECISIONS 382)". In the Gates bullet, "gates the `Edit` p95, the `Evaluate` p95,
cold check, cold RSS and the view-model size" becomes "gates the `Edit` p95, cold check, cold RSS and the
view-model size, and reports the `Evaluate` p95"; in the NFR-02 bullet, "re-check a 7,000-entry package in
300 ms" becomes "re-check a 7,000-entry package within the `Edit` target".

### DECISIONS text

> D-BENCHEDIT. **NFR-01's edit targets follow the robustness-bearing code (IMPLEMENTATION-PLAN §7.6; amends 261, 382).** `Edit` p95 ≤ 450 ms (v0.2.0's A/B, 0.343 s, × 1.25 rounded up to 50 ms, 382's rule); `Evaluate` p95 is reported, not gated: whichever call first needs the snapshot's analysis pays it (382). A miss blocks a release only as a regression, as 382 says. Reason: both targets failed on every tree since v0.1.5 (Evaluate's meaning moved when the analysis did, ADR-0021 §6), so the gate gated nothing.

### Tests

| Test | Proves | First |
|---|---|---|
| `TestScaleSizes` (pure) | scales × unit, and the TSV line's format | fails first (no function) |
| `TestBenchScale` (opt-in, the flags above) | the bench | proved by its one real run at release (§18) |
| `TestBenchEdit` | the re-based target and the reported Evaluate line | its constants only; one run on a quiet machine at release |

### Units

| Unit | Exclusive packages and files | Tier, reason | Size | After |
|---|---|---|---|---|
| SCALE-BENCH | `internal/testkit/cmd/benchgen`; the Makefile's bench block | sonnet: a measurement harness, no gate | S | S2 (its real run is a release step) |

### Forecast

1. **The 5× run may be killed at 12G:** that is a result, not a failure of the release.
2. **Disk:** at most 0.9 GB per size, one size at a time, deleted after.
3. **Louis's Q3:** the reading the research gives, for him to weigh, not a decision: memory is structural
   and linear (about 127 KB per entry), and a results cache would address time, not the RSS of a cold
   check.

## 14. Owed generator items (DECISIONS 442, 443, 444)

Scope: the generator-side items v0.2 left owed (its reply's "What stays owed", ADR-0023 §9.5 and §13, the
two v0.2 logs). Research: one sonnet pass with a reproduction for each item and prototypes in a scratch
copy (`/var/tmp/canon-030-research/r-owed-gen/`). Sovereign meets none of them: 0 sites to migrate.

### 14.1 E8005's hint names the way out that works (DECISIONS 442; amends 403)

**Facts.**
- `E8005`'s variants for another package's `#define`s are `foreignDefine` ("rename {a} with
  @cpp(name:), or use a type of that package here"), `foreignId` and `foreignDerived`
  (`spec/ERRORS.md:1300-1307`). The plan marks a name derived only in `shareDerived`/`declareDerived`
  (`internal/ir/cppplan_scopes.go:78`, `:83`: a value's container, `k<V>Schema`, `Data::<v>`, `<v>_ref`,
  the snapshot member, `Load<V>`) and `declareIDEnum` (`cppplan_baked.go:112`); every other declaration
  (about 45 lines in `cppplan_*.go`) is a plain declare, so its finding says "rename X with
  @cpp(name:)", where X can even be a package.
- **The advice is a proven dead end** for names `@cpp(name:)` does not change: an `export fn names()`
  beside a foreign `names` define says "rename b.names with @cpp(name:)", and after adding
  `@cpp(name: "Names2")` the same finding returns (it is the `Data` member, built from the Canon name);
  `title: String @cpp(name: "Heading")` still collides through `title_`. Generator-fixed names (`GetId`,
  `Get`, `Decode`, `Parse`, `Data`, `Build`, `Capture`, `ToName`, `ToWire`, `IsRetired`, `IsKnown`,
  `ToCode`, `to_json`, `from_json`, names built on the package's last segment, namespace segments) cannot
  be renamed at all.
- **Two ways, two builds:** `@cpp(name: "vv") let vv: [Int]` beside a foreign `vv` define gives
  `foreignDefine`; after following it, a second build gives `foreignDerived` for `Data::vv`: the plan
  deduplicates by (name, origin), and the first scope wins (`declaredNames`, `cppforeign.go:150`).
- A container's fixed members (`Len`, `At`, `All`, `Find`, `rows_`, `ToWire`, `Load`, `Value`, `code`,
  `rows`) are `E8011 reserved` as define names; they still reach another package's `E8005` as a
  cascade of that `E8011` (five `E8005` beside one `E8011`).
- Existing fixtures pin wrong hints: `internal/ir/testdata/findings/E8005_85.txtar` (its `BookRow` line
  names `app.kinds`; the row class is built on `lib.Book`; its `kMonstersDefines` line names a local
  `let`). Sovereign's two real cases (the badge enum members) stay `foreignDefine`.

**Decision.**
1. **Four true hints:** `foreignDefine` only where `@cpp(name:)` on `{a}` renames the name; `foreignDerived`
   for every name built on a Canon name that `@cpp(name:)` does not change, now including a field's
   storage `<f>_` and `<f>_ref_`, a stored fn's storage, a define value's member, `FindBy<F>` and its two
   indexes, an input slot, a branch table `k<T><F>Branches`, `Data::<fn>`, `<Class>_<fn>`, a conformance
   vector struct and table, a parameter's vector field, and a dependent type's arm names; `foreignId`
   unchanged; and a new **`foreignFixed`** for a generator-fixed name, whose only way out is a use of a
   type of the defining package. `<Element>Row` names its record, as `<Rec>Id` does.
2. **One finding per way:** the plan keys a declared name by (name, origin, way), so a name met in two ways
   is two findings in one build.
3. **The cascade rule:** a define name that `E8011` refuses is no project define for this rule; its finding
   is the defining package's (DECISIONS 213).
4. **Code** (`internal/ir`, the prototype `proto/item1-hints.diff`): `foreignNote` gains `fixed bool`;
   `declaredKey` becomes {name, origin; id, derived, fixed} and both `declaredNames` and
   `commonDeclared` (`cppplan_everymode.go:26`) use its key (a forgotten second use is a compile error);
   a `pl.shared` entry joins only when no scope declared the same (name, origin) (shared entries mirror
   scope declarations); helpers `declareFixed`, `declareOwn`, `innerFixed`, `innerOwn` at the sites listed
   in the research's classification; `foreignFinding` picks id, then derived, then fixed, else define;
   `addDefine` (`cppforeign.go:108`) skips a name `defineProblem` refuses. No generator byte changes.
5. **The rule for new names:** a C++ plan name added by any later unit chooses `declare`, `declareOwn` or
   `declareFixed`; `TestForeignHintsClear` is the net (below), and the spec-reviewer checks it.

**ERRORS.md (applied by S2).** After the `foreignDerived` row:
> | E8005 | foreignFixed | name:Name, a:Name, b:Name | `{name} is declared by the generator for {a} and written as a #define for {b}, in a package this one does not read, so its headers cannot guard it; no @cpp(name:) renames it, so use a type of that package here` |

**Spec text.** CODEGEN §7.9 (`spec/CODEGEN.md:2867`): the parenthesis after "`E8005` `foreignDefine`", from
"(`foreignId` when" to "the record is named)", becomes:
> (`foreignId` when the name is a table id; `foreignDerived` when it is built on the Canon name of a value, a field, a stored fn or a parameter, which `@cpp(name:)` does not change: a value's container, `k<V>Schema`, `Data::<v>`, `<v>_ref`, the snapshot member `<v>_`, `Load<V>`, `detail::<P>Access::Load<V>` and a define table's `k<T>Defines`; a field's or stored fn's storage `<f>_` and `<f>_ref_`, a define value's member, `FindBy<F>` with its two indexes, an input slot and a branch table `k<T><F>Branches`; a fn's `Data::<fn>`, `<Class>_<fn>`, conformance vector struct and table, a parameter's vector field; a dependent type's arm names; `foreignFixed` when no declaration's name or override changes it: the members every entry, container, class, access struct, snapshot, store and conformance namespace has (`GetId`, `Get`, `Decode`, `Parse`, `Data`, `Build`, `Capture` …), the overloads `ToName`, `ToWire`, `IsRetired`, `IsKnown` and `ToCode`, `to_json` and `from_json`, names built on the package's last segment, and a namespace segment; for `<Rec>Id`, `<Rec>IdFromWire` and `<Element>Row` the record is named)

and after that sentence's end ("…a translation unit that includes that defines header first could not compile
it."), two sentences are added:
> A name met in several of these ways for one declaration is one finding for each way, so that no rename leaves another finding for the same name behind. A define name that `E8011` refuses is no define for this rule: its finding is the defining package's (DECISIONS 213).

and its way-out sentence becomes:
> The way out is the one the finding names: `@cpp(name:)` on the declaration, the Canon rename of a derived name's source, renaming a table id's entry, or, always, a use of a type of the defining package.

**DECISIONS text.**
> D-HINTS. **`E8005` names the way out that works (CODEGEN.md §7.9; ERRORS.md E8005; amends 403).** A name built on the Canon name of a field, a fn or a parameter is `foreignDerived`, as one built on a value's is; a name the generator fixes is the new variant `foreignFixed`, whose way out is a use of a type of the defining package; `<Element>Row` names its record; a name met in several ways is one finding per way; a define that `E8011` refuses is no define here. Reason: "rename X with @cpp(name:)" was followed for `Data::<fn>` and the same finding returned, and a name met two ways needed two builds (v0.2, log-2026-11-01).

**Tests (`internal/ir`; cite CODEGEN §7.9, DECISIONS 403, 213, 305).**
| Test | Proves | First |
|---|---|---|
| `TestForeignHintKinds` (new `cppforeign_kinds_test.go`) | for each family (storage, ref storage, stored-fn storage, `Data::<fn>`, `<Class>_<fn>`, vector names, a parameter, entry members, container `Get`, access members, `Decode`/`Parse`, conformance names, snapshot and store, a dependent arm, a namespace segment, the row class) the variant and exact message, plus unchanged `foreignDefine` and `foreignId` rows | about 25 rows are `foreignDefine` today |
| `TestForeignHintsClear` (same file) | for each variant, applying exactly what the finding says (`@cpp(name:)` on `{a}`; the Canon rename of `{a}`; renaming the entry; `import a { Word }`) gives no `E8005` | fails today on the `Data::<fn>`, storage and vector rows |
| `TestDeclaredNames` (`cppforeign_internal_test.go:135`), `TestForeignDefineHints` (`cppforeign_id_test.go:122`), updated | one (name, origin) in two ways is two entries; a shared name no scope declares is fixed only for a namespace segment | pins updated |
| fixtures: three new `findings/E8005_<next>` (field, fn and parameter derived; fixed; one name two ways in one build); `E8005_85` regenerated (`BookRow` names `lib.Book`; `kMonstersDefines` derived); `cascades/E8011_6.txtar` (a define `Len`: one `E8011`, no `E8005` in the other package; `E8011_1`-`_5` exist) | the rules | fail first |

**Unit.** HINTS: `internal/ir` (the plan files, `rules_cppforeign.go`, tests); sonnet, opus review (mechanical
and golden-checked, with `TestForeignHintsClear` as the net); M; after S2 (which added the `foreignFixed` row,
the count and the messages golden). It starts from the prototype
`/var/tmp/canon-030-research/r-owed-gen/proto/item1-hints.diff` when that file still exists. No `examples/**/expected` change.

### 14.2 `@go`, `@ts` and `@cpp` on a local declaration are `E1118` (DECISIONS 443; amends 390)

**Facts.** `@cpp`, `@go` and `@ts` annotations on a `local` record, enum, variant, const, let, type or fn, or
on a field, member, case or method inside one, are accepted with no finding and no effect, even an invalid
override name (`@go(name: "x y")`): the IR holds no local type (`internal/ir/units.go:109`, `:128`), so the
override checks never see them. `E1118` already refuses `@cpp(defines:)` on a local
(`internal/ir/rules_cppdefines.go:74` `checkLocalDefines`, message "@cpp is not allowed on this local",
position `diag.KindLocal`, fixtures `E1118_1`, `E1118_2`). Sovereign: 439 local declarations, none
annotated; the examples: 63, none. (The same three on a public fn without `export` are also ignored: owed,
§19.)

**Decision.** Any `@go`, `@ts` or `@cpp` annotation on a `local` declaration, or inside one, is `E1118`,
with the existing message and position `local`. `checkLocalAnnotations` replaces `checkLocalDefines`
(`rules_emit.go:30` calls it); `isLocalDecl` (`units.go:128`) gains the const, let and fn cases; the
annotation names it reports are a package-level `var` in `rules_cppdefines.go` (not `tables.go`, which KIND
edits). `E1118_1` and `E1118_2` stay byte-identical.

**Spec text.** GRAMMAR §8.3, the `name` row (`spec/GRAMMAR.md:1262`) ends "…; never on a `local` declaration
or inside one (`E1118`)"; after its combination rules (`:1275`):
> - A `@go`, `@ts` or `@cpp` annotation configures generated code, and a `local` declaration is never generated (TYPES.md §3.7): on one, or on a field, member, case or method inside one, it is `E1118`. Remove the annotation, or drop `local` to emit the declaration.

TYPES §3.7, one sentence: "A `@go`, `@ts` or `@cpp` annotation on a `local` declaration is `E1118`
(GRAMMAR.md §8.3)." GRAMMAR §12's `E1118` row (`spec/GRAMMAR.md:1416`) ends "…; prefix annotation on a
record/enum/variant or field; a `@go`, `@ts` or `@cpp` annotation on a `local` declaration or inside one".

**DECISIONS text.**
> D-LOCALANN. **A target annotation on a local declaration is `E1118` (GRAMMAR.md §8.3; TYPES.md §3.7; amends 390).** A `@go`, `@ts` or `@cpp` annotation on a `local` declaration, or inside one, is refused, as `@cpp(defines:)` is. Reason: nothing generates a local declaration, so the annotation, even an invalid override name, was accepted and ignored; Sovereign has none in 8,600 files.

**Tests.** `internal/ir/testdata/findings/E1118_3.txtar`: every local kind with members, cases and methods,
plus a public control: one finding per annotated site, none on the public control (fails first: 0 today). **Unit.** LOCAL-ANN: `internal/ir`
(`rules_cppdefines.go`, `units.go`, `rules_emit.go`); sonnet; XS; after S2; disjoint from HINTS', KIND's and
ENUM-0's files.

### 14.3 A default that reads a field of a record literal is checked (DECISIONS 444; amends 412)

**Facts.** `constantDefault` (`internal/check/foldeddefault.go:30`) is a whitelist; a typed literal, a
qualified name, a field item and a field selection are not on it, so `record U { x: Int(1..3) = (T {}).a }`
with `record T { a: Int = 5 }` passes `canon check`, and every `types`-mode decoder writes the unchecked 5 for
an absent key (Go `out.viaRecord = 5`, C++ `out.viaRecord_ = 5;`, TS `$r1 === undefined ? 5 : …`): DECISIONS
399 trusts defaults to be checked because 412 checks them. `buildsOf` (`internal/check/foldbuilds.go:30`,
v0.2's CHECK-OWED2) already makes folding a record literal safe. Sovereign: 407 field defaults, none of this
form.

**Decision.** `constantDefault` admits `*syntax.TypedLit`, `*syntax.QualifiedName`, `*syntax.FieldItem`, and a
selection whose `Selections` entry is a field (`SelField`); the receiver is walked next, so a typed literal
passes and an earlier field `y.z` still fails. If the audit's complexity limit trips, the selection test
moves to a helper. Unchanged: record-typed fields are not folded; spreads and entries are not admitted;
cyclic shapes keep today's verdicts (no static `E4301`; `E4402` when a value takes the default, or at check
when an `emit json` evaluates one, so the accept fixture holds no emit).

**Spec text.** TYPES §7.4 (`spec/TYPES.md:763`): "A field's default that is a constant expression (§15), or
that reads a field of a record literal built of constants (`(T {}).a`), is checked against the field's
refinements as the literal it folds to: …"; TYPES §15's bullet (`:1328-1330`) becomes: "A field default (TYP-15) may use constants, **earlier** fields of
the same record, record parameters, built-in functions, and a field of a record literal built of these
(`(T {}).a`); anything else (a package `let`, a user function, a later field) is `E3010`."

**DECISIONS text.**
> D-DEFAULTREC. **A default that reads a field of a record literal is checked as its literal (TYPES.md §7.4, §15; amends 412).** Reason: `x: Int(1..3) = (T {}).a` with `a = 5` passed `canon check`, and every `types`-mode decoder wrote the 5 unchecked for an absent key (399 trusts defaults to be checked).

**Tests.** `TestFoldedDefaultIsItsLiteral` (`internal/check/foldeddefault_test.go:21`) gains rows
(`= (Z {}).k` equals `= 0` in the no-value, one, two and sets-the-field cases; fails first);
`check/testdata/findings/E3204_7.txtar` (two findings); `check/testdata/accept/folded_record_default.txtar` (a
fine case, a forward reference, a self-reading default: no hang, no crash). **Unit.** DEFAULT-FOLD:
`internal/check/foldeddefault.go` and tests; opus (check semantics, the cycle hazard); XS; after S2.

### 14.4 Decided out, with the record corrected

- **The define guard's over-wrap: not a bug any more.** `cppruntime.Scan` has skipped comments, string and
  character literals since `3cdc78c9` (v0.2's CPP-OWED); the `\`-continued `//` case is unreachable from
  Canon source (gen/cpp writes `//` only in fixed lines of identifiers and paths). CODEGEN §7.9 and DECISIONS
  404 already state it. `meta/state.md` already records it (written with this ADR); the reply says the v0.2
  note was stale.
- **The dead `ModFloat` helpers: kept on purpose.** `rt.go.txt:251` and `canon_runtime.h.txt:158` (TS has none,
  only a table entry); Float `%` is `E3007`, so no valid program reaches them, but removing them means guards
  for malformed IR in three generators and 90 goldens, more code for one dead line per output, and they are
  the right implementation if `%` is ever admitted on Float. `meta/state.md` already records it as kept on purpose.
- **MSVC at scale:** CODEGEN §9 already states the limit (`spec/CODEGEN.md:3143`). No action.
- **`replacePlatforms`** (`api/refresh_test.go:24`) and `stampPlatforms` cite each other on purpose: each is a
  test copy, so a broken build tag fails its test instead of skipping it. No action.

## 15. Owed bugs outside the generators (DECISIONS 445, 446, 447, 448, 449, 450, 451)

Scope: the checker, evaluation, edit, workspace and API items v0.2 left owed, ruled one by one (the standing
rule: every known bug is fixed as found). Research: one opus pass with a reproduction for each item where it
was cheap, under the 3G cap (`/var/tmp/canon-030-research/r-owed-core/`). One new bug was found (§.1).

### 15.1 A long-lived Project no longer grows with its edits (DECISIONS 445)

**Facts.**
- `pin` (`internal/workspace/writes.go:22-35`, verified) records a file, a stat and a directory entry for every
  name a writer touches, and `adopt`s them into older snapshots. The edit commit stages each file under a unique
  name, `.<base>.<token>.canon-edit`, its token the journal's first 16 hex characters
  (`internal/edit/journal.go:362-365`, verified; DECISIONS 351). So each new content makes a new staging name,
  and nothing ever drops its entries.
- `refresh` compares every non-overlay entry with the disk, one `Stat` each (`refresh.go:31`, `:41-56`),
  carrying forward an entry whose name no longer exists; `history.push` (`history.go:96-121`) copies its `tip`
  map whole every 16 revisions and keeps 64 records.
- **Measured** (`api` probes): on the OS file system with distinct values, the tip grows by exactly 2 entries per
  edit (607 entries after 300 edits, 2,407 after 1,200) and the heap from 3.6 to 12.1 MB, per-edit time +24%
  over 1,200 edits; on the in-memory file system the heap goes from 1.07 to 17.8 MB over 2,000 edits (77% in
  `maps.clone` under `history.push`) and the time per 500 edits doubles over 4,000. With repeated values the names
  repeat and growth stops; an agent writing new values never repeats.
- Who meets it: every long-lived `api.Project` that calls `Edit` (the studio's server, agent harnesses). The
  language server does not edit; `canon edit` is one process per request.

**Decision.** Entries stay immutable once stored (`snapfs.go:49-51`; `adopt` shares a writer's `*entry` with
older snapshots and `fork` clones pointers, so a mark on the entry would race). Instead each `snapFS` keeps a
set of the names only a writer pinned, carried by `fork`: `pin`, `pinName` and `adopt` add a name, and a
reader's `get` of it removes it from that snapshot's set. `refresh` and `refreshNames` do not carry a name of the
set into the next snapshot when its fresh stat or read says it does not exist; the history delta records it as
gone. Older snapshots keep
their entries (S9); overlays and DECISIONS 421's reported names (reader names) are unaffected; a revision (S4)
covers the static read set, which pinned names are not part of, so revisions are unchanged.

**Spec text.** API.md S4, a sentence:
> A name only a writer pinned, which no read of the program asked for, leaves the next snapshot once it names nothing, so a long-lived Project does not grow with its edits (DECISIONS D-SNAPPIN).

**DECISIONS text.**
> D-SNAPPIN. **A long-lived Project does not grow with its edits (API.md S4; amends 351).** A snapshot entry that only a writer pinned (an edit's staging name), and that no read asked for, is dropped at the next refresh once its name no longer exists. Reason: every edit staged its file under a new unique name (351) and the snapshot kept both entries forever: 2 entries and a full refresh `Stat` per edit, 17.8 MB after 2,000 edits.

**Tests.** `TestLongSessionStaysBounded` (`internal/workspace`): 300 edits with distinct values on one
Project; the entry count and the history tip stay at most (their value after 10 edits) + 8; under `-race` and at
GOMAXPROCS 2 and 4. Fails first (+2 per edit). **Unit.** WS-SNAP: `internal/workspace` (`snapfs.go` is 446
lines: 54 of room); opus (snapshot consistency, S1/S9, concurrency); S; after S2.

### 15.2 Dependent literals: one budget whatever a binder's name; unstored record literals judged (DECISIONS 446)

**Facts.**
- `sameDepMap` (`internal/eval/literals.go:128-131`) is `a == b || types.Identical(a, b) && a.String() == b.String()`,
  and `String()` holds the binder's name, so a pending entry recorded under `{x in c: P(x)}` is never cleared by a
  judgement at `{y in c: P(y)}`, and the residue walk judges it again. Measured: 58 steps against 54 for the same
  program with the other spelling (the finding is not duplicated; only the charge differs, which moves a verdict at
  the budget's edge: DECISIONS 265, "once per element").
- **An unstored dependent record literal is never judged:** `W { n: a, v: 4 } == W { n: a, v: 4 }` and a fn body's
  `W { n: a, v: 3 }` give no finding, while a stored `W { n: a, v: 3 }` gives `E3802 stored.v`. EVALUATION §5
  (`EVALUATION.md:268`) judges kept-map literals stored or not, but says nothing of record literals; so an `==`
  against such a literal is silently always false or always true. DECISIONS 426's reason ("whether a literal is
  stored must not change a verdict") applies.

**Decision.**
1. `sameDepMap(a, b)` becomes `a == b || types.Identical(a, b) && sameUpToBinder(a, b)`: the same `Coll`, then a
   parallel walk of the two value types, comparing each type application's function, argument count, and each
   argument's source, parameter and path, with binders equal after mapping one map's binders to the other's.
   `internal/types` (frozen, IMPLEMENTATION-PLAN §4.2) does not change.
2. A record or case literal whose type has a dependent field (TYPES §11.4) is judged like a kept-map literal:
   eval records it as pending when it builds it (memoised per type), verify's walker clears it when it judges
   that value at its type, and after every root of the flush a still-pending record is verified once as a value
   (`KeptLiteral{Value, Type}` through the host's existing `VerifyLiterals`), its findings carrying the path within
   the literal and no root. Each value is still walked once; a stored record is cleared by its root's walk, so a
   program that stores all its records keeps its budget.

**Spec text.** EVALUATION §5, after `:268`:
> A record or case literal with a dependent field (TYPES.md §11.4) is judged the same way, stored or not: one no verification judged at its type is verified once as a value, charged to the value whose evaluation built it, its findings carrying the path within the literal, with no root (DECISIONS D-RECLIT).

**DECISIONS text.**
> D-RECLIT. **Unstored dependent literals are judged, once, whatever a binder's name (EVALUATION.md §5; TYPES.md §11.6; amends 408).** A record or case literal with a dependent field is judged where it is built when no stored value judges it, as a kept-map literal is; and two dependent-map types that differ only in a binder's name are one type for the residue walk. Reason: `W { n: a, v: 4 } == w` was silently false while the stored literal gave `E3802`, and whether a literal is stored must not change a verdict (426); a binder's spelling changed a budget verdict (265).

**Tests.** `TestKeptLiteralBinderSpelling` (`internal/eval`, the minimum-budget pattern of `enumnamed_test.go:60`):
the `{x…}` and `{y…}` programs have an equal minimum budget (fails first: 54 against 58).
`dependent/unstored_record.txtar` (`internal/eval`): `E3802` at `W { n: a, v: 4 } == …`, at a fn-body literal ("in
f"), and once for a stored record (fails first: no finding); a minimum-budget pin that a stored-only program's
budget does not change. **Unit.** EVAL-DEP: `internal/eval`; granted `internal/verify` and
`internal/build/hosts.go` (and the host's implementing file); opus; M; after S2.

### 15.3 Every `where` run that verification makes is free (DECISIONS 447; amends 148)

**Facts.** Every stage-B `where` run is free (`internal/eval/where.go:12-34` sets `r.free = true`, from
`verify/refine.go:99-110` and `verify/precomputed.go:93-101`), and so is a kept literal it builds
(`literals.go:62-64`). EVALUATION §12.1 (`:708-711`) and DECISIONS 148 make free only "a re-run … that the
conversion already ran (and paid)". The first run on a value an amendment replaced is a case no conversion paid:
measured, an expensive record-level `where` costs 38 steps at base and 39 under `-layer dev` (the +1 is the
amendment's right-hand side), and an amendment to `a: 30` gets `E3206` from stage B.

**Decision.** The spec states the code's rule; no code changes. Charging stage B's first runs would need
per-value "already converted" tracking and would change `E4401` verdicts under layers; the free runs stay bounded
(each capped at the budget; amended values are bounded by layer text).

**Spec text.** EVALUATION §12.1, `:708-711` becomes:
> Every run of a `where` predicate that verification (stage B) makes is free, whether or not a conversion ran it on that value first: a re-run the conversion already paid, and the first run on a value an amendment or a re-derived default replaced (§9.3), whose enclosing refinements no conversion judged whole. A kept literal (§5) that such a run builds is judged free with it. Each run is capped at the budget; past it that run alone stops, its value is poisoned, no `E4401` is reported and evaluation continues.

**DECISIONS text.**
> D-WHEREFREE. **Verification's `where` runs are free (EVALUATION.md §12.1; amends 148).** Every run of a `where` predicate that stage B makes costs nothing, whether or not a conversion ran it on that value first, and a kept literal it builds is judged free; each run is capped at the budget. Reason: 148 freed only re-runs the conversion paid, but the first run on an amended value is also stage B's; charging it would need per-value conversion tracking and would move `E4401` verdicts under layers.

**Unit.** None: S1 only (the code already does it).

### 15.4 An error outside every `expect` fails the test (DECISIONS 448)

**Facts.** `noteStop` (`internal/eval/testlog.go:57-73`) keeps only hard errors of a test's own statements, and
`Test()` (`eval/tests.go:62-85`) sets the test's errors from them. Reproduced: a test with
`let x: Int(0..=5) = nine()` (9) and `let same: Bool = hs[0].by == { n: {} }`, then `expect x == 9`, passes (exit 0,
JSON `"status":"pass"`). EVALUATION §10.4 (`:582-590`): "A test passes when every expect passed and no hard error
occurred"; neither it nor CLI §3.5 says anything of soft findings outside an `expect`. `build.ExpectFailure` with
`Expect: ""` already means "what stopped or broke the test" (`internal/build/tests.go:34-45`).

**Decision.** An error finding produced by the test's own statements outside every `expect` subject and condition
(a refinement at a storage point in a `let` or a call, a kept literal a statement builds) fails the test without
stopping it. Eval notes it like `noteStop`, in a "broke" list (`r.test != nil && r.sink == nil`); the test is
failed, not stopped; duplicates merge by §14. Build maps each to `ExpectFailure{Loc: the finding's span, Expect: "",
Findings: [f]}`. The CLI writes one line per such finding, after the failing `expect`s and before a stopping
error's line, in F2 order: two spaces, `<file>:<line>`, two spaces, `<severity>[<CODE>]  <message>`. A warning
there neither fails the test nor prints.

**Spec text.** EVALUATION §10.4: "A test passes when every `expect` passed and no hard error occurred." becomes:
> An error finding produced outside every `expect` subject and condition (a refinement at a storage point in a `let` or a call, a kept literal a statement builds, §5) fails the test without stopping it; a warning there neither fails nor shows. A test passes when every `expect` passed and no error occurred outside them (DECISIONS D-TESTSOFT).

CLI §3.5, after `CLI.md:382`:
> - An error finding outside every `expect` (EVALUATION.md §10.4), which fails the test without stopping it, is one line, after the failing `expect`s and before a stopping error's line, in F2 order (API.md §4.1): two spaces, `<file>:<line>` of the finding, two spaces, `<severity>[<CODE>]  <message>`; in JSON a failure with `"expect": ""` and the finding in `findings`.

**DECISIONS text.**
> D-TESTSOFT. **An error outside every `expect` fails its test (EVALUATION.md §10.4; CLI.md §3.5).** A soft error finding a test's own statements produce outside every `expect` subject and condition fails the test without stopping it, one failure line per finding; a warning there neither fails nor shows. Reason: a test passed while its own `let` held 9 in `Int(0..=5)`: a false green.

**Tests.** `tests/soft_outside` (an eval or build txtar, and an `internal/cli` command golden): the fixture above
now fails, one line per finding; a warning there passes. Fails first. **Unit.** TEST-SOFT: `internal/eval` (`testlog.go`, `tests.go`, and `litscope.go`, where the kept literals a
statement builds are judged, `:106-125`, today into the test path's throwaway bags); granted
`internal/build/tests.go` and a new `internal/cli/testdata/commands/test_soft_outside.txtar`; opus; S; after
EVAL-DEP (both edit eval).

### 15.5 `canon test` reports its own static findings, warnings included (DECISIONS 449; amends ADR-0004)

**Facts.** `canon test` prints no warning at all: `runTest` (`internal/cli/test.go:15-36`) returns exit 1 or 0
only, prints its check result only when it holds errors (`:39-51`), and never reads the global `--max-warnings`
(`maxWarnings` appears only in `check.go:64` and `build.go`; verified). Reproduced: `canon check` prints `W7101`
and `W1003`, `canon test` neither; `canon test -max-warnings 0` exits 0 where `check -max-warnings 0` exits 4.
`api/build.go:233` says `Check` holds "phases 1-2's error findings (ADR-0004)"; `internal/build/tests.go:20`
already adds `@codes` and refused amendments; API.md §13.2 (`:1820-1832`) still shows the `TestResult` and
`ExpectFailure` of 2026-09-24 (ADR-0004 said "API.md §13.2 gains the matching rule at the next spec sync"; it
never did). Resource's pre-push runs `check -max-warnings 0` before a plain `test`, so nothing there breaks.

**Decision.** The test path computes what it does not compute today: `r.test` (`internal/build/tests.go:57-89`)
runs phase 2 and then the tests, but `W1003` comes only from `checkNames` in `stagesCD`
(`internal/build/pipeline.go:338`) and `W7101` only from `FinishDefines` (`pipeline.go:140`,
`lockcheck.go:53`), whose bags the test host throws away (`tests.go:61`). So `r.test` first fills `r.cps` with the selected packages (as stage A's `forceSelected` does,
`pipeline.go:245-250`; on the test path it is empty, so `checkNames` would find nothing), runs `checkNames`, and
keeps the bags map it creates for the test host (`tests.go:61`, where `FinishDefines` reports through the load
requests, `hosts.go:105-107`) to move its `W7101` findings into the run's real bags. The static result then holds
the errors of every loaded package and of `project.canon`, as today (ADR-0004, `staticErrors`, `tests.go:111-113`),
and the warnings of the packages the run selects, as `canon check` with the same selectors reports them (API R2);
`api.TestResult.Check` carries them with their summary. If the move needs more than `tests.go` and `hosts.go`, the
unit stops and reports. `runTest` prints `Check` whenever it holds any finding (`-q`: errors only), in
`check`'s format, then a blank line, then the report. Exit: 1 if a test failed or `Check` holds an error; else 4
if its warnings exceed `--max-warnings`; else 0. The same unit fixes the `api/build.go:233` comment and writes the
`Options.FS` doc comment of §16.1, both in package `api`, the latter under IMPLEMENTATION-PLAN §4's
review rule (a doc comment of the frozen `api/canon.go`, no signature change).

**Spec text.** CLI §3.5, after `CLI.md:349`:
> Before the report it prints, in `canon check`'s format (§2.4) with its summary line and a blank line after, the errors of phases 1–2 of every loaded package and the warnings of the selected packages (`-q`: errors only), the errors no test can capture (`E3102`, `E6004`, EVALUATION.md §10.5) and the warnings of the files its tests read that no value carries (`W7101`).

and "Exit: 0 when every test passes, 1 otherwise." becomes:
> Exit: 1 when a test fails or one of those findings is an error; else 4 when its warnings exceed `--max-warnings` (§2.5); else 0.

API.md §13.2: §16.2's text.

**DECISIONS text.**
> D-TESTWARN. **`canon test` reports its own static findings as `check` does (CLI.md §3.5; API.md §13.2; amends ADR-0004).** It prints the errors of phases 1-2 of every loaded package (as before) and the warnings of the selected packages, its file-read warnings (`W7101`) included, before its report, honours `-q` and `--max-warnings` (exit 4), and `TestResult.Check` carries them. Reason: it printed no warning, `W7101` included, and ignored the global `--max-warnings`.

ADR-0004 takes an "Amended (v0.3, D-TESTWARN)" paragraph with the same content (the orchestrator writes it in S1).

**Tests.** `internal/cli` command goldens: `canon test` prints `W1003` and `W7101`; `canon test --max-warnings 0`
exits 4; `-q` hides warnings (fail first). An `api` test of `TestResult.Check` cites `API.md B5` (the rule
§16.2 adds, held back to this unit's commit: `TestEveryAPIRuleHasATest` fails on a rule no test cites).
**Unit.** TEST-CLI: `internal/cli` (`test.go`, `test_report.go`, command goldens); granted
`internal/build/tests.go` (the test path and its static result), `internal/build/hosts.go` if the `W7101` move
needs it (free after EVAL-DEP), `api/build.go` (the comment), `api/canon.go` (the `Options.FS` doc comment), an `api`
test file, and `examples/features/pkgchecks/expected/tests.txt`;
opus (the build pipeline's test path); S; after TEST-SOFT.

### 15.6 A String-keyed keyed list's word key starts an amend line (DECISIONS 450)

**Facts.** `amendPathText` (`internal/edit/applylayerline.go:231-246`) and `noAmendForm` (`layerrows.go:179-188`)
require an amend line's first step to be a field, so a `Set` of `items[a].n` under a new layer line is refused
with `layer` (pinned at `edit/apply_parity_test.go:132`). EVALUATION §9.2 accepts `.k` for a keyed list with a
String key, and the hand-written `b.n: 9` parses and applies (`edit/fixture_test.go:203`); API §6.4
(`API.md:812-820`) prints only table entries bare. The refusal names no way out (DECISIONS 305).

**Decision.** `amendPathText` accepts a first key step when its container is a keyed list whose key type is
`String` and the key is a word (`syntax.ValueText`, not a keyword), and writes it bare (`a`, then `.n`);
`noAmendForm` uses the same predicate; any other first key stays `layer`. No new constant (`edit/constants.go` is
full; it reuses the field and bracket constants).

**Spec text.** API §6.4: "a field or a table entry whose key is a word is `.name` (the first bare)" becomes:
> a field or a table entry whose key is a word is `.name` (the first bare); so is the first step of an amend line into a keyed list with a `String` key, when the key is a word that is not a keyword (DECISIONS D-KEYEDLINE); a later `String` key stays a string literal, and a keyword key cannot start a line (`layer`)

**DECISIONS text.**
> D-KEYEDLINE. **A String-keyed keyed list's word key starts an amend line (API.md §6.4; EVALUATION.md §9.2).** Reason: `b.n: 9` parses and applies, but the edit engine refused to write it (`layer`), a ban with no way out (305).

**Tests.** `TestW11NoAmendFormParity`'s row (`apply_parity_test.go:132`) flips to no refusal; an Apply test writes
`a.n: 5` into `amend items { }`, the re-check is clean and the Undo restores; a non-word key (`"a b"`) stays
`layer`. Fails first. Before landing, the 48,550-verdict Editable/Apply parity walk of v0.2's EDIT-OWED runs again
and stays clean. **Unit.** EDIT-KEYED: `internal/edit`; opus; XS-S; after S2.

### 15.7 writelock names no absolute path (no DECISIONS item; implements 201, 416)

**Facts.** ADR-0023 §13's list was partly wrong: `internal/writelock/lock.go:148` leaks (Release wraps a raw
`f.Close()` error), `:191` and `:197` do not (they pass through `relative`, `:176-185`), `:211` is `relative`'s
fallback; missed: `stamp` joins a raw `f.Close()` (`:281`), and `relative` keeps only the first `*fs.PathError` of a
joined error (`:207-209`). Reproduced: Release after a closed handle returns "close /var/tmp/…/.canon/writers/pkg.a:
file already closed".

**Decision.** `Held` keeps its directory and `Release` wraps `relative(h.dir, err)`; `relative` rewrites every
`*fs.PathError` of a joined error tree, keeping order, cause and `errors.Is`, and returns nil for nil; `stamp` joins
`relative(dir, err)` with `relative(dir, f.Close())`.

**Tests.** `TestReleaseNamesNoAbsolutePath` and `TestRelativeJoined` (internal tests; both fail first). **Unit.**
WRITELOCK: `internal/writelock`; sonnet; XS; after S2.

### 15.8 The first build refuses what the next one would (DECISIONS 451; amends 341)

**Facts.** Reproduced: package `a` declares `load.dir("data/*.json")` and package `b` declares
`emit text { out: "@data" }` writing `y.json`. Run 1: `check` and `build` pass, and the build writes
`@data/y.json`. Run 2: both fail with `E8026 … a/data/y.json is read by this load and written by emit text of b`.
Cause: `feedback()` (`internal/build/feedback.go:37-64`) judges each load's names from `load.Names`, which lists
existing matches only, and `overlaps()` (`:78-90`) gates on files actually read. A single-file `load("data/y.json")`
gives `E7004` "cannot read" on both runs: consistent, but the wrong cause.

**Decision.** `load` exports a pure matcher over project-relative paths the caller has already resolved through
the layout (`MatchesPlanned(pattern, display string) bool`, WIRE §6.5's segment rules: `**`, dotfiles,
case-sensitive); `feedback` judges each `load.dir` against every planned output path (lexically, and by real path
where the disk has one). `E8026` is raised whether or not the file exists yet, with today's message; `overlaps()`
also counts loads whose base lies under an emit directory. Every project without such an overlap is
byte-identical. A single-file `load` of a planned output that does not exist yet keeps today's `E7004` "cannot
read" on every run (its build fails, so the file is never written): consistent, and left as it is.

**Spec text.** CODEGEN §2.4, after "(any target, any copy, a skipped output included)":
> , whether or not the file exists yet: a `load.dir` pattern is matched against the paths the build's emits write (the way out: narrow the pattern, or move the emit's `out` outside the load's base),

and CODEGEN §12's `E8026` row (`spec/CODEGEN.md:3228`) gains ", whether or not the file exists yet
(DECISIONS D-FEEDBACK)".

**DECISIONS text.**
> D-FEEDBACK. **A build's outputs are judged as inputs before they exist (CODEGEN.md §2.4; amends 341).** A `load.dir` pattern that matches a path one of the build's emits writes is `E8026`, whether or not that file exists yet; the way out is a narrower pattern, or an `out` outside the load's base. Reason: the first build passed and wrote the file, and the second refused it (`E8026`), so a build's own output broke the next check.

**Tests.** `internal/build/testdata/.../feedback_unwritten.txtar`: the project above gives `E8026` on the first
`check`; a pattern that does not match (`*.csv`) gives no finding. Fails first. **Unit.** FEEDBACK: `internal/build/feedback.go`; granted the matcher export in
`internal/load`; sonnet, opus review; S; after S2. The Resource dry run must show 0 `E8026`.

### 15.9 Decided out, with the record corrected

- **The checker's `kept` memo is not a leak.** Measured: 10,000 Rechecks left every checker and `Info` map at the
  same length, and retained heap matched a control run with no Recheck: a Recheck re-checks values only and cannot
  create a type (`check/doc.go:4-9`; every `DepMapType` comes from a declaration path). The item closes; TIDY adds a
  pin, `TestRecheckLineageKeepsCheckerFlat` (proved able to fail by keying `keptDepMap` on a fresh copy).
- **No related location at an unstored literal:** spec-conformant (EVALUATION §5: "with no root"; API F12 allows
  only expectations in `Related`); the build stays red until the value is fixed. No change.
- **VIEWMODEL J13/J14 through a ref:** studio work, out of v0.3. **The record is corrected:** ADR-0023 §8.4 called
  it "a shape no project uses"; Resource uses it (`world/worldlevel/worldLevel.canon:31-33`,
  `param: EventParamValue(eventType.shape)?` through `eventType: ref rewardEvents`; its view model gives
  `"drivers": {}`, so the studio cannot pick the branch). The lean for that work: `drivers` keyed by the ref's
  target collection, following the path at build time.
- **Typing a line that holds a spread:** the refusal is honest and the same in Apply and Editable. No change.
- **An Operation marked "Undo, skip user rows": decided out for good.** A client-settable marker would bypass the
  `layered` row; the refusal names the blocking layer, and its ways out are making lx the edit layer or stacking
  `[lx, dev]`.
- **Windows spellings** (NTFS case, 8.3 names): until Windows CI returns. The drive-letter case is already
  normalised (`lsp/position.go:109-121`, `project/volume.go:69-73`).
- **`make bench-edit`'s targets:** re-based (§13).

### 15.10 Test tidying (unit TIDY, at the end)

One sonnet delegation after every unit but CLEANUP has landed (no package is then held), test files only, XS each:
- `internal/ir`: a `tsdecode` flag (`range_run_test.go:30`; it must not contain the substring `text`, which the
  parser tests with `strings.Contains`) applied to `testdata/e2e/rangerun/cases.txt:114-115`, so `cpp-decode` joins
  `bits-2p53p1` and `bits-int64-max`; header lines 4-5 describe it. A pin, expected green: if the C++ run disagrees,
  the unit stops and reports (a cross-target difference is a new bug, §0).
- `internal/testkit/progen`: `goDirs` (`ops_emit_dirs_test.go:41-49`) skips an `out` whose value is a list literal;
  one `sharedGoDirOp()` holds the rule string `ops_emit_test.go:37` and `ops_emit_dirs_test.go:54` both repeat.
- `internal/build/drivers_syntax_test.go`: the positive case, a clean table beside the flawed one
  (`let fine: table a.Kind = { c2 { goal: visit, n: 4 } }`), asserting `got["c:fine"]["c2"] == "visit"` (a pin,
  proved able to fail by making a flawed package contribute nothing).
- `internal/check`: `TestRecheckLineageKeepsCheckerFlat` (§.9).

## 16. Spec wording v0.2 left unwritten (no DECISIONS item; S1)

Research swept every "wording owed", "spec wording owed" and "release wording" entry of log-2026-10-31 and
log-2026-11-01 against the spec of today. About 37 entries are synced (each checked; the list is in the research
report). These are not, and S1 writes them verbatim. None changes behaviour: each states a ruling the run already
made and the code already follows.

- **16.1 API §2.2 and `Options.FS`** (log-2026-10-31, WS-INFLIGHT). At the end of API §2.2 (after
  `API.md:186-188`):
  > An FS is called while the Project waits on it: a method must never call back into the Project it serves (such a call waits for the refresh that waits for it, until the caller's context ends). A refresh waits for every read in flight (S1), so a read that hangs, on a network file system say, holds the Project's reads and edits until their contexts end.

  `api/canon.go:33`'s doc comment becomes (TEST-CLI writes it, §15.5; a doc comment of the frozen file,
  under IMPLEMENTATION-PLAN §4's review rule):
  > // FS is the file system of a Project; WriteFile must be atomic, and no method may call back into the Project it serves: a refresh waits for every read in flight (API.md §2.2).
- **16.2 API §13.2** (stale since ADR-0004, 2026-09-24). Line `API.md:1824` becomes the first line below and
  line `:1826` the second; line `:1825` (`type TestCase struct {…}`) stays as it is:
  > `type TestResult struct { Check *CheckResult; Tests []TestCase; Passed, Failed int; Duration time.Duration }`
  > `type ExpectFailure struct { Span; Expect, Outcome, Op, Expected, Got, Poisoned string; Findings, Cause []Finding }`

  and API.md gains a rule B5 after B4 (`API.md:1834`), **held back to TEST-CLI's commit** (a declared API.md rule needs a test citing it,
  `TestEveryAPIRuleHasATest`; TEST-CLI adds that test, and the behaviour B5 describes):
  > - **B5.** `Check` holds the errors of phases 1–2 of every loaded package and of project.canon, and the warnings of the packages the run selects, as `canon check` reports them (R2), and the run's own findings no test captures: `E3102` for a repeated `@codes` code, `E6004` for a refused stable amendment, and `W7101` (EVALUATION.md §10.5), with their summary (ADR-0004, DECISIONS D-TESTWARN). `Expect` is "" for what stopped or broke the test: one finding, or a poisoned read (`Poisoned`, its causes in `Cause`).
- **16.3 CODEGEN §3.5 never received DECISIONS 429.** After `CODEGEN.md:882` ("…is `E8005` (§7.9)."):
  > Under an emit mode `check` refused (`E8009`), every name check of that target (its own plan, the names a shared namespace holds, `#define` duplicates, `foreignDefine` and its variants) judges what the plans of every valid mode of that target have in common (Go: `baked`, `data`, `types`; TypeScript: those and `embedded`; C++: `baked`, `data`, `types`): a collision is reported when it holds in every mode and skipped otherwise. `W8006`, a warning about one plan, is skipped; an unbuilt mode (`E8019`) has no plan. Every operand of an `E8005` names a declaration by its qualified Canon name, never a generated name (DECISIONS 429).
- **16.4 CODEGEN §5.13, the order of checks on an integer map key** (log-2026-10-31, TS-KEYMSG). After the codes-key
  sentence (`CODEGEN.md:1844-1846`):
  > An integer map key is held to WIRE.md §5.8's canonical form first, then to its width (TypeScript: `integer outside the TypeScript safe range`), at the key's path.
- **16.5 WIRE §6.5, an unreadable irregular entry** (log-2026-10-31, PROJECT-LINKS and LOAD-LINKS). After
  "Any other entry is no link and keeps its kind (DECISIONS 419)." (`WIRE.md:671`):
  > An irregular entry whose target `os.Readlink` cannot read for another reason than that it is no link (Go's `EINVAL`, or a missing entry) is an error, a load's `E7004` with the system's cause, never walked as a directory; only a file system that resolves no links decides by its stat alone.

  DECISIONS 419 takes the same text as an in-place note: "(Amended <S1's date>: …)".
- **16.6 WIRE §6.5, a device-path link target** (log-2026-10-31, PROJECT-LINKS). "A mount point whose target is a
  volume (`\\?\Volume{…}\`) has no path the bounds can judge and counts as dangling." becomes:
  > A link whose target is a device path (`\\?\…`, `\\.\…`), a volume mount point's included, has no path the bounds can judge and counts as dangling; a symbolic link whose target text is one does too.
- **16.7 TYPES §7.4, folded defaults** (log-2026-10-31, CHECK-OWED). After §14.3's sentence at
  `TYPES.md:763`, append:
  > A default whose type holds a record or a variant is not folded, nor one met once its package's budget is spent (`E4401`, EVALUATION.md §12.2): storage checks it (EVALUATION.md §4.3). A default whose fold reports a finding breaks its record (§1), as a constant's fold does (DECISIONS 150).

  DECISIONS 412 takes an in-place note with the same content.
- **16.8 TYPES §15, readiness during a fold** (log-2026-11-01, CHECK-OWED2). After `TYPES.md:1325`:
  > - A constant expression may build a record or a case once that body is complete (its fields resolved, its defaults checked); a body still being completed when the fold reaches it (the bound's own record or case) is `E3015` `notConstant`, and one declared later is completed first: declaration order does not matter (§3.2).
- **16.9 API V1, the uncomputed-branch rule binds a Set only** (log-2026-10-31, EDIT). In `API.md:1017`, "A list,
  record or map literal given to a dependent type is typed against the branch its record computes" becomes "A
  list, record or map literal a `Set` gives a dependent type is typed against the branch its record computes", and
  the rule gains:
  > An item an `Insert` or `AddEntry` adds to a dependent container is typed against its branch where that can be computed, else against the declared type, and the re-check judges it (`E3802`).
- **CODEGEN §7.1's include order** (found by §5): its text (§5's Spec text).
- **CODEGEN §7.5's "WIRE.md §8" misreference** (found by §4): READER's commit fixes it.
- **`meta/state.md`** already carries these outcomes (written with this ADR): the define-guard over-wrap is no
  bug, dead `ModFloat` is kept on purpose, the `kept` memo item is closed.
- **Not changed:** ADR-0023 Decision 6's "integer temp before narrowing" (an ADR is history; the log already
  records the reading).

## 17. The run

### 17.1 Rules (ADR-0023 §11.1, with v0.2's lessons)

Every rule below answers something that went wrong in an earlier run; the log entry is named so the
orchestrator can read why.

- **Read first:** CLAUDE.md, `meta/state.md`, `meta/plan.md` ("v0.3: lean generated code"), this ADR
  whole, DOCTRINE §2-§5, `.claude/rules/orchestration.md` and `.claude/rules/go.md`.
- **Base:** `main` at or after the commit holding this ADR.
- **Calls already made are never re-asked:** every decision of this ADR. Technical calls are the
  orchestrator's, decided and logged in `meta/decisions/log-<date>.md`. Only a philosophy question
  goes to Louis, in a file under `meta/handoff/`.
- **Memory caps.**
  - Every agent's `go test` or probe runs as
    `systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local go test -timeout 300s ...`,
    and that exact command is in every VERIFY line. A C++-compiling test that needs more time gets
    `-timeout 1200s`, never more memory.
  - The orchestrator's `make check` runs as
    `systemd-run --user --scope -q -p MemoryMax=6G -p MemorySwapMax=0 env GOTOOLCHAIN=local make check > /var/tmp/canon-v030/mc-<unit>.log 2>&1; echo "exit=$?" >> /var/tmp/canon-v030/mc-<unit>.log`.
  - **Fuzzing always runs with `-parallel 4`** (v0.2: 19 of 28 targets OOM-killed at the default 24
    workers under 3G, log-2026-11-01 "§12 step 2, first pass").
  - Temp dirs go in `/var/tmp`, logs in `/var/tmp/canon-v030/`. At each wave boundary, run
    `df -h /tmp /var/tmp`; when either passes 90%, remove this run's own `cppgen-objects-*`,
    `go-build*` and `canon-*` temp dirs (v0.2: `/tmp` reached 100%, log-2026-11-01).
- **Concurrency.**
  - At most 6 builders at once, on disjoint packages, one builder per package at a time.
  - At most 2 heavy test runs at once (eval, `-race`, fuzz, the C++/Go/TS golden compiles, the
    gen/cpp compile suites): every builder and reviewer wraps them in
    `/var/tmp/canon-v030/heavy.sh <command>` (a copy of `/var/tmp/canon-v020/heavy.sh` with the path
    changed; S0 creates it).
  - `TestBigEnumTablesCompile` and `TestLargeEnumsCompile` are the heaviest tests in the repository
    (v0.2: g++ killed after 702 s under the 6G cap, log-2026-10-31 "OUT-PATH landing"). If either is
    killed under `make check`, re-run `make check` once with no agent running; if it is killed again,
    it runs alone, under 6G, and its own log is the evidence.
- **Working tree.**
  - Never `git stash`, `git checkout --` or `git reset`.
  - Each unit works in a worktree (`isolation: "worktree"`), fast-forwarded to `main` first and
    reporting its base SHA.
  - The orchestrator commits in the worktree and cherry-picks onto `main` (never merges), with
    `make check` before every `cherry-pick --continue`.
  - **Land only on `exit=0`.** The landing step greps the log for the line `exit=0` that the command
    above appends. The last line of the test output, or a background task's own exit status, is not
    evidence (v0.2: EN-TS landed on `exit=2`, `main` red for one commit, log-2026-10-31 "Recorded
    miss").
  - **Never chain a cherry-pick after the guard with `;`.** Each landing is its own command, or
    `&&`-chained (v0.2: OUT-PATH, log-2026-10-31 "Orchestrator slip").
  - Units on disjoint packages that are ready together may be cherry-picked with `-n` and judged by
    one `make check` (about 20 minutes each), then committed one per unit in landing order; a failure
    is bisected by unit (log-2026-10-31 "Integration").
  - Meta commits go on `main` only.
- **Read-only outside this directory** (Resource, Source, Scripts, the services). A need there is a
  handoff.
- **Agents.** Check `ListAgents` before telling Louis that units run. End every delegation prompt
  with "send your final report; do not stop without it". A builder kills only its own processes, by
  pid, never with `pkill -f`, and leaves no polling loop behind (v0.2: a `pkill -f "cat$"`, and a
  `sleep 15` loop, pid 2792522, still running a day later, log-2026-11-01).
- **Generator units also run**, before reporting: `internal/cli`'s `TestCommands` (its command
  goldens hold generated text), `internal/testkit/golden`'s `TestExamples`, and, when they touch a
  runtime text (`internal/gen/cpp/runtime/*.txt`, `internal/gen/go/runtime` text, the TS helper
  units), `make diag-check` (the runtime-text checker reads every `Fail("…` call; v0.2: EMPTY-KEY's
  landing, log-2026-10-31).
- **Cross-target readers.** A unit that changes what a decoder or a reader accepts or writes keeps
  `internal/ir`'s `TestRangeRoundTrip` green (the E2E-RANGE comparison of every reader verdict across
  C++, Go and TS, `internal/ir/range_run_test.go:195`). A new cross-target difference it finds is a
  new bug (§0).
- **ERRORS.md changes.** Every ERRORS.md edit is followed by diaggen, the
  `internal/diag/render_test.go` count, and `go test ./internal/diag -update` for the messages golden,
  in the same commit (v0.2: the OUT-PATH spec commit's first `make check`, log-2026-10-31).
- **New harness directories.** A unit that adds a `testdata` directory that a test reads as goldens
  registers it in `internal/testkit/rules_scan_test.go` (v0.2: SUBJECT-CAPTURE's landing,
  log-2026-11-01).
- **Kept progen counterexamples.** When a new refusal makes a kept archive under
  `internal/testkit/progen/testdata/` report the new code, record it as that archive's leftover
  (`left <code>/<variant>`), as v0.2 did for `E8009/outChars`: a one-line orchestrator fixup.
- **Constants room (the audit, DECISIONS 26).** `internal/gen/cpp/constants.go` is at 495 lines,
  `internal/gen/go/constants.go` at 496, `internal/ir/constants.go` at 500, `internal/check/constants.go`
  at 499 and `internal/edit/constants.go` at 500, against the 500-line `file-length` limit.
  `const-placement` allows a `const` in `<pkg>/constants.go` **or `<pkg>/errors.go`**
  (`tools/audit/internal/lanes/gorules/consts.go:22`), and the generators already use the second:
  `internal/gen/cpp/errors.go` (361 lines) holds the decoder, baked, make-hook and enum-table format
  blocks, `internal/gen/go/errors.go` (130) holds map-field and dependent-type blocks, and
  `internal/ir/errors.go` is 25 lines. A unit whose new constants would push `constants.go` past 500
  lines, in this order:
  1. puts its new block in `<pkg>/errors.go` when that file exists and already holds a `const` block
     (gen/cpp, gen/go, ir, edit), under its own doc comment, the file staying ≤ 500 lines;
  2. otherwise deletes a constant nothing uses, or moves to `<pkg>/constants_tables.go` a package-level
     `var` table some of whose values are named constants (const-placement allows it anywhere; it reads a
     bare identifier key as a field name, so constant keys alone do not count);
  3. otherwise inlines, at its only use, a string constant used exactly once whose value appears
     nowhere else in the repository's non-test Go (DOCTRINE-code §2 forbids a literal twice, not
     once; about half of each package's constants are used once).

  Steps 1 and 3 are technical calls of this ADR, logged: a reviewer does not flag a constant block in
  `errors.go`, or an inlined single-use literal, against the letter of DOCTRINE-code §2 or go.md §2.

  Never raise a limit, never add a `sovaudit:ignore` for these rules, never create an `errors.go` only
  to hold constants, never put a `const` in another file. If 1-3 cannot make room, the unit stops and
  reports.
- **Concurrency tests** also run at `GOMAXPROCS=2` and `4`.
- **Review.**
  - Every non-trivial diff gets `spec-reviewer` (opus), against this ADR's section and the spec it
    cites.
  - On a FAIL, resume the SAME builder with the numbered findings, then the SAME reviewer on the fixes
    only.
  - A sonnet builder that fails review twice is replaced by an opus one.
  - Expect several rounds on opus units: v0.2's longest took seven (EDIT-OWED), six (PROGEN-NIGHTLY)
    and four (DEP). A round is not a failure of the plan.
- **Long gates** run once in the middle (§17.2, S4) and once at the end (§18). Builders run only
  their own package tests.
- **Goldens a unit's change moves are the unit's.** A unit regenerates, with `-update`, every golden its own
  change moves (`examples/**/expected`, `internal/cli`'s command goldens, its packages' testdata, the runtime
  copies under `internal/gen/cpp/testdata/golden/`); that is not an edit outside its packages. A golden that
  moves for a reason its section does not explain stops the unit.
- **An edit outside a unit's packages.** When a unit needs another file outside its packages (a test that
  encoded an old rule, a fixture), it stops and reports. The orchestrator then grants that
  file to the unit (logged) when no other unit writes it, even while another unit holds its package,
  or makes the edit as a ≤ 1-file, ≤ 20-line fixup in the unit's worktree (CLAUDE.md). `main` never
  goes red.
- **Status.** Status for Louis goes in `meta/handoff/<date>-v030-status.md`, updated at each wave
  boundary, with a two-line chat summary. It states the four calls flagged for Louis (§0) until he
  answers them.
- **Against the common mistakes (§0):**
  - Ask every builder for its command log, and read it.
  - Never start a unit whose package another unit holds.
  - Read every golden diff, and classify each hunk by this ADR's section; a hunk no section explains
    stops the unit.
  - Copy the VERIFY line from this section, never retype it.
  - A builder that "improves" a design is answered with §0.

### 17.2 Steps

**S0, preflight.** The orchestrator, before S1; logged in the day's log.
- `git status` is clean and `main` holds this ADR's commit.
- `mkdir -p /var/tmp/canon-v030`, and copy `/var/tmp/canon-v020/heavy.sh` to
  `/var/tmp/canon-v030/heavy.sh` with its two lock paths changed to `/var/tmp/canon-v030/`. If the v0.2
  file is gone, write it again from ADR-0023's rule: two `flock -n` slots, retry every 10 s, the
  command's exit status.
- **The stale worktrees.** `git worktree list` shows about 60 worktrees under `.claude/worktrees/`
  left by v0.2's units. They double every `grep -r` and the session hook warns about them. For each
  one whose branch has no commit missing from `main` by content (`git cherry main <branch>` prints
  no `+` line) and whose tree is clean, run `git worktree remove <path>` (never `--force`; keep the
  branch). Keep and list any other one in the log. Every research grep of this ADR excluded them.
- `df -h /tmp /var/tmp` and `free -g`, logged; `nproc` is 24 on Louis's machine.
- **A baseline `make check`** on the clean `main`, under the 6G command of §17.1, so that a
  failure later is never blamed on a unit when it was already there. Its wall time is the run's
  reference for "about 20 minutes".
- **The toolchains.** `g++ --version`, `clang++ --version`, the libc++ row
  (`internal/testkit/cxx`), `node --version`, the two TS compilers of `tools/tsc`
  (`npm ci --prefix tools/tsc` if missing), nlohmann's version as the C++ tests find it. Logged.
  `make check` sets `CANON_REQUIRE_CXX=1` and `CANON_REQUIRE_TS=1`, so a missing one fails rather
  than skips.
- **The research prototypes** some units start from (`/var/tmp/canon-030-research/r-reader/a4.diff`,
  `r-owed-gen/proto/item1-hints.diff`, `r-ts/pure-prototype.patch`, `r-runtime/scratch-code.patch`) live in
  `/var/tmp`, which ages files out. Their presence is logged; a missing one changes nothing: a unit builds from
  this ADR's text, the prototypes are a convenience and never the source of truth.


**S1, spec sync.** The orchestrator; one commit, `docs(spec): v0.3 decisions 430-452 (ADR-0026)`.
- **Spec texts.** Every "Spec text" block of §3-§16 and §18, verbatim, except the ERRORS.md
  blocks (S2) and the held-back blocks below.
- **DECISIONS.** 430-452 appended in order. "[amended by N]" marks go on 19 and 415 (D-RTSTAMP), 148
  (D-WHEREFREE), 229 (D-TPL), 261 and 382 (D-BENCHEDIT), 290 and 384 (D-ORDERED), 292 (D-KINDCONST), 341
  (D-FEEDBACK), 351 (D-SNAPPIN), 383 and 393 (D-ENUMTAB), 390 (D-LOCALANN), 392 (D-SIZE), 403 (D-HINTS), 408
  (D-RECLIT), 412 (D-DEFAULTREC), 422 (D-LANGVER); in-place "(Amended …)" notes on 412 and 419 (§16.5
  and §16.7).
- **ADR-0004** takes its "Amended (v0.3, D-TESTWARN)" paragraph (§15.5).
- **meta:** `meta/plan.md`'s v0.3 list already points here and `meta/state.md` already carries §16's state
  lines (both written with this ADR). §19's items go to `meta/plan.md`'s v0.4, v0.5 and later sections, with one
  pointer line in `meta/state.md`, which stays at 80 lines or fewer.
- **Held back from S1** (a test compares the block with code, or the block names a package or code that does not
  exist yet). The orchestrator writes each in the commit of the unit named, in that unit's worktree:
  - CODEGEN §7.4's listing and its two prose blocks, and §7.5's two stamp sentences → CPP-RT (`TestHeaderIsTheSpecs`,
    `TestSpecNamesTheRuntime`);
  - CODEGEN §7.5's reader sentences and its misreference fix → READER (after CPP-RT's §7.5);
  - ERRORS.md's two `VariantKindConstant` rows deleted, then diaggen → KIND (the code names them until KIND
    removes the use);
  - ERRORS.md's `W00xx` section, IMPLEMENTATION-PLAN §3's `lint` row and `api`'s Consumes cell, then diaggen →
    LINT-CORE (diaggen needs the package row; the deps tests need the package);
  - API.md §5.7 (its rules L1-L11), CLI.md §1's `canon lint` row and §3.19, IMPLEMENTATION-PLAN §8.1's `lint`
    row → LINT-API-CLI (its rules need their citing tests in the same commit, §12);
  - API.md §13.2's new rule B5 (§16.2) → TEST-CLI (the same reason).

  If S1's `make check` fails on another block, hold that block back the same way, to the unit that changes the
  compared code, and log it.
- **Review and check.** Before the commit, `spec-reviewer` (opus) checks S1's diff against this ADR: every block
  verbatim, nothing added. Then `make check` must be green (print `exit=0`). If `diag-check` fails, or a test
  comparing a document's code table with ERRORS.md fails (none is known today; v0.2's rule), run S2 first and commit
  S1 and S2 together.

**S2, DIAG.** The orchestrator, then one sonnet go-dev in the main tree, with nothing else running. One commit,
`feat(diag): v0.3 codes and messages (DECISIONS 439, 442)`.
- **The orchestrator writes the ERRORS.md blocks** of §11 (E1019 and its six messages, E1006 `outTemplates`,
  E1009 `templateRoot`, E8009 `outTemplate`/`outQuoted`, the `OutPath` kind and `OutPaths`' word, the meaning
  appendices of E1009 and E8009) and §14.1 (E8005 `foreignFixed`), and the count sentence. Expected
  after S2: "The catalogue holds 334 codes: 305 errors, 26 warnings and 3 run-time codes, with 583 messages."
  (after LINT-CORE: 342 codes, 305 errors, 34 warnings, 3 run-time, 593 messages). diaggen checks it; if it
  disagrees, recount (diaggen wins, never edit around it).
- **The go-dev:**
  - runs `go run ./internal/diag/cmd/diaggen -out internal/diag -runtime internal/gen` (`codes.go` and
    `codes_test.go` are regenerated, never hand-edited);
  - updates `internal/diag/render_test.go`'s count and runs `go test ./internal/diag -update` for the messages
    golden (read the diff: the new rows only);
  - follows the wording change of the `OutPaths` kind through `E8009_11` and `E8009_21`'s package `-update`
    (message text only);
  - names no new code in production code, so the `diag-code-untested` ratchet does not grow (the new variants
    stay unreported until their units; `diag-code-unreported` only observes).
- `make check` must be green. No review: no decision is left in S2.

**S3, the units.**
- Start any unit whose "after" units have all landed on `main`, within §17.1's caps.
- Each delegation uses orchestration.md's template:
  - **GOAL:** from the unit's section.
  - **OWNER/SCOPE:** exactly the packages and files below; a granted file is named.
  - **READ FIRST:** this ADR's sections, the spec sections they cite, DOCTRINE §3-§5, `.claude/rules/go.md`,
    and §17.1's constants-room rule.
  - **CONTRACTS:** the frozen interfaces the unit reads. Only LINT-API-CLI (additive `Project.Lint`), TEST-CLI
    (the `Options.FS` doc comment) and TPL-CHECK (`ir.Emit.Out`'s meaning, §11) touch one, under
    IMPLEMENTATION-PLAN §4's review rule.
  - **TESTS:** the section's tests, each citing its rule; every new code or variant has its txtar.
  - **VERIFY:** §17.1's capped `go test` of its packages (`-race` where it touches concurrency), the C++
    compile suites of its package through `heavy.sh`, and, for a generator or runtime unit, `TestCommands`,
    `TestExamples` and `make diag-check`; the log attached. Builders do not run `make check`; the
    orchestrator does, under 6G, before every cherry-pick.
  - **REPORT BACK:** files touched; gaps; any choice not in this ADR (none is expected; an unexpected one is
    logged under §0's fallback); what is unverified.
  - The prompt states the tier and why, and ends with "send your final report; do not stop without it".

| Unit | Packages and files (exclusive; granted files named) | Tier | Size | After | § |
|---|---|---|---|---|---|
| KIND | `internal/ir` (`convert.go`, `rules_gen.go`, `tables.go`, its testdata); `internal/gen/go/consts.go`; `internal/gen/ts/values.go`; `internal/gen/cpp` (`literal.go`, `ctype.go`, `bulk.go`, `baked_fill.go`, `gen.go`'s comment); `internal/build/testdata/builds/const_kind*.txtar`; the orchestrator's ERRORS.md deletion and diaggen | sonnet | S | S2 | 10.2 |
| CPP-RT | `internal/gen/cpp/runtime`; granted `internal/gen/cpp/runtime_stamp_compile_test.go`, `internal/gen/cpp/testdata/main/stamp/` (new files) and the golden runtime copies | opus | S-M | S2 | 9, 10.1 |
| ENUM-0 | `internal/ir` (`enum_tables.go` new, `goplan_tables.go`, `goplan_scopes.go:223`) | sonnet | XS | S2 | 3 |
| HINTS | `internal/ir` (`cppforeign.go`, `rules_cppforeign.go`, `cppplan_*.go`, `cppplan_everymode.go`, tests, findings and cascades fixtures) | sonnet, opus review | M | S2 | 14.1 |
| LOCAL-ANN | `internal/ir` (`rules_cppdefines.go`, `units.go`, `rules_emit.go`, `findings/E1118_3.txtar`) | sonnet | XS | S2 | 14.2 |
| TPL-PROJECT | `internal/project`; granted `internal/syntax/parse_amend.go`, `parser.go`, `constants.go`, its syntax test file, and an `E1019` mutation operator in `internal/testkit/progen` | sonnet, opus review | M | S2 | 11 |
| DEFAULT-FOLD | `internal/check` (`foldeddefault.go`, its tests and fixtures) | opus | XS | S2 | 14.3 |
| EVAL-DEP | `internal/eval`; granted `internal/verify`, `internal/build/hosts.go` (and the host's implementing file) | opus | M | S2 | 15.2 |
| WS-SNAP | `internal/workspace` | opus | S | S2 | 15.1 |
| EDIT-KEYED | `internal/edit` | opus | XS-S | S2 | 15.6 |
| WRITELOCK | `internal/writelock` | sonnet | XS | S2 | 15.7 |
| FEEDBACK | `internal/build/feedback.go` and its txtar; granted the matcher export in `internal/load` | sonnet, opus review | S | S2 | 15.8 |
| FNSIZE | `internal/testkit/fnsize` (new) | sonnet | S | S2 | 8 |
| SCALE-BENCH | `internal/testkit/cmd/benchgen`; the Makefile's bench block | sonnet | S | S2 | 13 |
| LINT-CORE | `internal/lint` (new); granted `internal/build/analysis.go`; the orchestrator's ERRORS.md `W00xx` section, IMPLEMENTATION-PLAN §3 row, diaggen, the `render_test.go` count and the messages golden | sonnet, opus review | M | S2, KIND | 12 |
| TS-PURE | `internal/gen/ts` | sonnet | S | KIND | 7 |
| READER | `internal/gen/cpp/runtime/canon_runtime_json.h.txt`; granted the new test files of §4 and the edited `strict_test.go` and `types_order_test.go` in `internal/gen/cpp`, and the golden runtime copies | opus | M | CPP-RT | 4 |
| ENUM-CPP | `internal/ir` (`cppplan_lookups.go`, `cppplan_inner.go`, new constants in `errors.go`, cppplan testdata, `findings/E8005_<next>.txtar`, the two `planpending` goldens); `internal/gen/cpp` | sonnet, opus review | M | ENUM-0, HINTS, KIND | 3 |
| ENUM-GO | `internal/ir` (`goplan_tables.go`, `goplan_imports.go`, `goplan_dependent.go`, `goplan_scopes.go`, the `GoTableFound` line of `constants.go`, goplan and planpending testdata); `internal/gen/go` | sonnet | M | ENUM-0, KIND, ENUM-CPP | 3 |
| TPL-CHECK | `internal/check`; granted `internal/ir/emits.go`, `ir.go:178`'s comment and its ir test file, `internal/build/strays_foreign.go` and two build txtars; `ir.Emit.Out`'s meaning (§4 review rule) | opus | M | TPL-PROJECT, DEFAULT-FOLD | 11 |
| TEST-SOFT | `internal/eval`; granted `internal/build/tests.go`, a new `internal/cli/testdata/commands/test_soft_outside.txtar` | opus | S | EVAL-DEP | 15.4 |
| LINT-DATA | `internal/lint`; granted `internal/edit/renamewire.go`, `internal/edit/place.go`, a new `internal/ir` file, `internal/load/defines.go` | sonnet, opus review | M | LINT-CORE, HINTS, EDIT-KEYED, FEEDBACK | 12 |
| BAKED-FIND | `internal/gen/cpp` | sonnet, opus review | S-M | ENUM-CPP | 6 |
| IR-PARTS | `internal/ir` (`parts.go` new, the `PartLines` line of `errors.go`, `cppplan_inner.go`, `goplan_scopes.go`, three fixtures) | sonnet, opus review | S | HINTS, ENUM-CPP, ENUM-GO | 8 |
| TEST-CLI | `internal/cli` (`test.go`, `test_report.go`, command goldens); granted `internal/build/tests.go` (and `hosts.go` if needed), `api/build.go`, `api/canon.go` (the doc comment), an `api` test file, `examples/features/pkgchecks/expected/tests.txt` | opus | S | TEST-SOFT | 15.5 |
| TPL-DOCS | one example package, the examples' `project.canon` key, a formatter golden; granted the VS Code grammar's scope snapshots of the files it changes | sonnet | S | TPL-CHECK | 11 |
| CPP-ORDERED | `internal/gen/cpp` | sonnet | S | BAKED-FIND, CPP-RT | 10.1 |
| GEN-GO-PARTS | `internal/gen/go` | sonnet, opus review | M | IR-PARTS, ENUM-GO | 8 |
| LINT-API-CLI | `api/lint.go` and its example; `internal/cli` (`lint.go`, `cli.go`, `constants.go`, five txtars); `internal/cli/guide/cli.md` (its row); granted `internal/testkit/rules_scan_test.go` and the citation lines of `internal/lint`'s txtars | opus | M | LINT-DATA, TEST-CLI | 12 |
| GEN-CPP-PARTS | `internal/gen/cpp` | sonnet, opus review | M | CPP-ORDERED, IR-PARTS | 8 |
| BUDGET | `internal/build/budget_test.go` (new) and its fixture writer | sonnet | S-M | FNSIZE, GEN-CPP-PARTS, GEN-GO-PARTS, ENUM-CPP, ENUM-GO | 8 |
| GUIDE | `internal/cli/guide` and its goldens | sonnet | S | every unit in §17.4's list | 17.4 |
| TIDY | the test files of §15.10 | sonnet | XS | every unit but CLEANUP | 15.10 |
| CLEANUP | as found | sonnet | S | every unit, TIDY included (it is last) | 17.5 |

**`internal/ir` is shared by file, not by package.** KIND, ENUM-0, HINTS and LOCAL-ANN edit disjoint files and may
run at once. ENUM-CPP, ENUM-GO and IR-PARTS run one after the other (they share the two `planpending` goldens,
`errors.go`'s const block and the `E8005` fixture numbers). A unit that needs an `ir` file outside its set stops
and reports (§17.1, "an edit outside a unit's packages"). Every other package has one builder at a time.

**Package queues** (each runs in order, one builder at a time):

| Package | Queue |
|---|---|
| `internal/gen/cpp` | KIND → ENUM-CPP → BAKED-FIND → CPP-ORDERED → GEN-CPP-PARTS (CPP-RT's and READER's granted test files are disjoint from these units' files; both regenerate the runtime goldens) |
| `internal/gen/cpp/runtime` | CPP-RT → READER |
| `internal/gen/go` | KIND → ENUM-GO (after ENUM-CPP) → GEN-GO-PARTS |
| `internal/gen/ts` | KIND → TS-PURE |
| `internal/ir` | {KIND, ENUM-0, HINTS, LOCAL-ANN} → ENUM-CPP → ENUM-GO → IR-PARTS; TPL-CHECK's and LINT-DATA's granted files are disjoint; TIDY last |
| `internal/check` | DEFAULT-FOLD → TPL-CHECK → TIDY (one pin test) |
| `internal/eval` | EVAL-DEP → TEST-SOFT |
| `internal/build` | by file: FEEDBACK (`feedback.go`), EVAL-DEP (`hosts.go`), TPL-CHECK (`strays_foreign.go`), LINT-CORE (`analysis.go`), KIND (two txtars), TEST-SOFT → TEST-CLI (`tests.go`), BUDGET (`budget_test.go`), TIDY (`drivers_syntax_test.go`) |
| `internal/edit` | EDIT-KEYED → LINT-DATA (its two granted files) |
| `internal/load` | FEEDBACK (the matcher) → LINT-DATA (`defines.go`) |
| `internal/cli` | TEST-CLI → LINT-API-CLI → GUIDE |
| `api` | TEST-CLI (`build.go`, `canon.go`'s comment) → LINT-API-CLI (`lint.go`) |
| `internal/lint` | LINT-CORE → LINT-DATA |
| `internal/diag` | S2 → KIND's diaggen → LINT-CORE's diaggen (orchestrator-run) |

**The critical path** runs through ENUM-CPP: S1 → S2 → {KIND, HINTS, ENUM-0} → ENUM-CPP → {BAKED-FIND → CPP-ORDERED,
ENUM-GO → IR-PARTS} → GEN-CPP-PARTS → BUDGET → GUIDE → TIDY → CLEANUP → §18 (the two braced chains are about as
long). Start KIND, HINTS, ENUM-0 and CPP-RT first.

**Wave plan, for the status file** (a forecast; the "after" column binds):

| Wave | Units |
|---|---|
| A | KIND, CPP-RT, HINTS, ENUM-0, LOCAL-ANN, TPL-PROJECT; as slots free: EVAL-DEP, DEFAULT-FOLD, WS-SNAP, EDIT-KEYED, WRITELOCK, FEEDBACK, FNSIZE, SCALE-BENCH, LINT-CORE |
| B | TS-PURE, READER, ENUM-CPP, TPL-CHECK, TEST-SOFT |
| C | ENUM-GO, BAKED-FIND, TEST-CLI, LINT-DATA, TPL-DOCS; then IR-PARTS |
| S4 | the mid-run gates (below) |
| D | CPP-ORDERED, GEN-GO-PARTS, LINT-API-CLI |
| E | GEN-CPP-PARTS |
| F | BUDGET, GUIDE, TIDY, then CLEANUP |

**S4, mid-run gates** (v0.2 ran its long gates first on the release day and found 15 progen classes and 2 crashers).
Once KIND, HINTS, LOCAL-ANN, DEFAULT-FOLD, TPL-CHECK, FEEDBACK and TEST-SOFT have landed (the units that refuse
more), with no heavy unit running:
- `make progen-nightly PROGEN_SEEDS=1` (about 8 minutes);
- `FuzzCheck` and `FuzzEval`, 3 minutes each at `-parallel 4` under 3G.

An archive kept or a crasher found is a new bug (§0): a read-only recon, then a unit in the package's queue. The
release runs all three seeds and every fuzz target again (§18).


### 17.3 Integration

- Cherry-pick each unit's commits onto `main` in landing order, with `make check` before every
  `cherry-pick --continue`, landing only on `exit=0`.
- **A conflict in a golden** (`examples/**/expected`, `testdata/**`) is resolved by regenerating with `-update` on
  the merged tree and reading the diff, never by picking a side. v0.3's generator units all rewrite C++ and Go
  goldens; their queues keep them apart, so a conflict means a unit ran out of order.
- **Runtime copies:** after CPP-RT and READER, every `canon_runtime.h` and `canon_runtime_json.h` copy under
  `examples/` and `internal/gen/cpp/testdata/golden/` must be byte-identical to the embedded text with the stamp
  substituted; the sha256 rule of §9 ("1 and 1") is checked after each runtime landing.
- **A conflict in `internal/testkit/progen/testdata/gaps.txt`** keeps every line, in the file's order.
- **The progen coverage ratchet** (`internal/testkit/progen/coverage_test.go`) fails on a code the compiler can
  report that has neither a mutation operator nor a line in `gaps.txt`. New codes: `E1019` gets a mutation
  operator from TPL-PROJECT (granted in `internal/testkit/progen`), not a `gaps.txt` line: `gaps.txt` may only
  shrink (`coverage_test.go:35`). `W0001`-`W0008` get no line: the ratchet counts only the codes of packages `internal/build` depends
  on, and `internal/lint` is not one (a line for them fails the test). New variants of existing codes need no
  line.
- **A conflict in name-plan code** (`internal/ir/*plan*.go`) is a ≤ 20-line orchestrator fixup, or it goes back to
  the later unit.

### 17.4 GUIDE

GUIDE writes these in `internal/cli/guide/`. Every recipe passes `canon check` in an examples project, every bare
`json` fence parses (`TestGuideBareJSONFencesParse`, new), and the guide's output goldens are regenerated and read.
- **TS in a bundle** (§7): `emit.md`'s section "TypeScript in a bundle" after "Generated enums", with the scoped
  `package.json` (`{ "type": "module", "sideEffects": false }`, a bare `json` fence), the `removeComments`,
  `innerGraph` and conformance-file bullets, and the per-file `CanonEvalError` bullet (tell an evaluation error by
  `e.code` or `e.name`; name a barrel's re-exports instead of `export *` over two generated modules); `index.md`'s
  `emit` row gains ", TypeScript in a bundle".
- **Output templates** (§11): an `emit.md` bullet with the `out_templates` snippet and `out: [admin]`;
  `errors.md` rows for `E1019`, `E8009` `outTemplate` and `outQuoted`.
- **`canon lint`** (§12): `port.md`'s section "6. Lint the port" listing `W0001`-`W0008` and the
  apply-one-then-lint-again loop (`canon lint --fix | jq -c 'select(.fix)|.fix.request' | head -1 | canon edit`).
- **`E8005`'s ways out** (§14.1): `emit.md:275-281` and `:321`, `errors.md:71`, `recipes.md:229` list the four
  ways (rename with `@cpp(name:)`; the Canon rename of a derived name's source; renaming a table id's entry; a use
  of a type of the defining package).
- **Locals** (§14.2): the `E1118` row in `errors.md` names target annotations on locals.
- **Kind constants and `ordered_json`** (§10): one sentence each in `emit.md`'s constants and C++ decoder
  notes.
- **The runtime stamp** (§9): one sentence in `emit.md`'s C++ notes: regenerate every C++ output with one
  canon; a mixed tree stops at compile time with canon's `#error`.
- **Tests** (§15.4, .5): the testing recipe says that an error outside every `expect` fails the test and
  that `canon test` prints its warnings and honours `--max-warnings`.
- **`E8026`** (§15.8): one sentence: a `load.dir` pattern that matches an output is refused before the output exists.

GUIDE runs after every unit that changes user-facing behaviour: TS-PURE, TPL-CHECK, TPL-DOCS, LINT-DATA,
LINT-API-CLI, HINTS, LOCAL-ANN, KIND, CPP-ORDERED, CPP-RT, TEST-SOFT, TEST-CLI, FEEDBACK.

### 17.5 CLEANUP

One delegation after every unit has landed: helpers invented twice (`make audit`, `dup-in-repo`; the likeliest are a
cut loop in gen/cpp and gen/go, which both must use `ir.PartEnds`, and the two `OutTexts` copies TPL-CHECK
replaced), dead code left by the units, then a review. It may not change behaviour; a golden diff in CLEANUP is a
bug.

### 17.6 Estimate

34 delegations (the table's 34 units) plus S2's go-dev, and S1. That is about 40-50 hours of agent time (S1 and S2
about 3 h; the units 30-38 h with review rounds; S4 and the release 7-9 h). Wall time is about 2.5 to 3 days, plus
any rate-limit stop; resume each agent where it stopped.

Calibration: v0.2 planned 36 units and ran about 50 (fifteen were new bugs found by reviews and gates), in about
2.5 days. v0.3's units are mostly sonnet on golden-checked generators, but its gen/cpp queue is five units long, and
new bugs found by reviews will add units, as they did in v0.2: budget for five.

## 18. Release

Release only when every unit has landed, CLEANUP is reviewed, and the spec, DECISIONS and `meta/` are
in sync (`meta/state.md` ≤ 80 lines). Each step's command, machine, load at start and end, and result
go in the final acceptance entry of `meta/decisions/log-<date>.md`. Logs go to `/var/tmp/canon-v030/`.

**The language version stays 0.1 (452).** The spec text goes in S1; SPEC §3.1's sentence
(`SPEC.md:325`) becomes:
> The compiler's version and the language version are separate (DECISIONS 354, 422, D-LANGVER): compilers 0.2 and 0.3 read language 0.1, whose syntax and refusals they extend as the 0.1.x releases did.

> D-LANGVER. **Compiler 0.3 reads language 0.1 (SPEC §3.1; amends 422).** Compiler 0.3.0 accepts every `canon: "0.1"` project; v0.3 changes generated code and adds a project-file key (`out_templates`), a root name in an emit's `out`, a command (`canon lint`) and refusals (`E1019`, `E8009` `outTemplate`/`outQuoted`, `E1118` for a local's target annotations, `E3204` at a record-literal default, `E8026` before the output exists, unstored dependent record literals judged, `E8005` for the new reserved generated names, `E8011` for the new runtime identifiers), and changes no existing source's meaning, so `canon: "0.2"` stays `E1001`. Reason: 422's rule, applied again.

**Steps:**
1. **`make check`** under `systemd-run --user --scope -q -p MemoryMax=6G -p MemorySwapMax=0 env GOTOOLCHAIN=local`,
   on a clean `main`, printing `exit=0`.
2. **Long gates**, one at a time, never together, with no agent running:
   - `make stress` (60 s, 3G, `-race`);
   - `make fuzz-edit` (10 min, 6G). Then `git status`: a crasher lands in
     `internal/edit/testdata/fuzz/FuzzMinimalWriteAll/`. It is a bug, fixed (failing test first)
     before the tag.
   - Every other fuzz target, 3 minutes each, at `-parallel 4`, under 3G (the v0.2 loop with the
     parallelism fixed):
     ```
     for pkg in $(grep -rl --include='*_test.go' '^func Fuzz' internal api | xargs -n1 dirname | sort -u); do
       for f in $(grep -ho '^func Fuzz[A-Za-z0-9_]*' $pkg/*_test.go | sed 's/^func //' | sort -u); do
         [ "$f" = FuzzMinimalWriteAll ] && continue
         systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local TMPDIR=/var/tmp \
           go test -run '^$' -fuzz "^$f\$" -fuzztime 3m -parallel 4 "./$pkg" || echo "FUZZ FAIL $pkg $f"
       done
     done
     ```
     `FuzzReaderMatchesV020` (§4) is in this loop; its driver is built once per content key, so its 3 minutes are fuzzing, not compiling.
     A crasher is a bug, fixed before the tag. An out-of-memory kill with no saved input is re-run
     once alone; a second kill is investigated, never ignored. An input saved by a killed run that passes
     when run alone (`go test -run 'Fuzz<X>/<id>'`) is moved aside to `/var/tmp/canon-v030/fuzz-oom-artifacts/`
     (a dirty `testdata/fuzz` blocks `make tag` and `make dist`), as v0.2 did.
   - `make progen-nightly` (3 seeds at N=10000, about 25 min; it was clean at v0.2.0). An archive
     kept by `-progen.keep` is a bug, triaged and fixed before the tag.
   - **The bench A/B.** Tree A is `git archive v0.2.0` into `/var/tmp/canon-030-ab/a`; tree B is
     `main`. Run `make bench-edit` (6G) as A, B, A, B, and the same for `make bench-lsp`.
     - A refresh p95 over 500 ms (DECISIONS 382), or a B/A regression over 10%, is recorded with its
       numbers. v0.3 changes no compiler hot path on purpose, so any regression over 10% is a bug,
       found by re-running B with one suspect unit reverted in a scratch worktree.
     - Tree A (v0.2.0) exits 2 on its old 0.300/0.150 s targets (v0.2's recorded miss); tree B carries
       SCALE-BENCH's re-based targets (§13): compare B with A, and B with its own targets.
   - **The scale bench** (§13): `make bench-scale` once, alone, with no agent running. Its table and TSV go in
     the log and in the reply, for Louis's Q3; a 5× check killed at 12G is recorded as "more than 12 GB". Resource's
     own `canon check` (wall and peak RSS) is re-measured beside it.
   - **`make bench-edit` on the re-based targets** (§13): `Edit` p95 ≤ 450 ms on B, or within 10% of A.
3. **Cross-platform** (CI is down):
   - `GOOS=windows go vet ./...` and `GOOS=darwin go vet ./...`;
   - the C++ compile suites on every `cxx.Toolchain` row present (g++, clang++, clang++ with libc++):
     `make check` runs them with `CANON_REQUIRE_CXX=1`;
   - `go test -race` over `api`, `internal/workspace`, `internal/edit`, `internal/build`,
     `internal/load`, `internal/lsp`, `internal/writelock` and `internal/cli`, at GOMAXPROCS 2, 4 and
     the default.
4. **The candidate:**
   ```
   CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -ldflags "-s -w -X \
   github.com/fantasim/canonlang/internal/build.CompilerVersion=0.3.0" -o /var/tmp/canon-030-dry/canon ./cmd/canon
   ```
5. **Resource dry run.** Follow ADR-0021 §8 step 4, read-only on Resource and Source. Its paths read
   `canon-030` instead of `canon-019`. **Its pinned binary is canon 0.2.0** (`~/.local/bin/canon`,
   `canon 0.2.0 (a0fb77a2)`), which Resource's own pre-push also pins (`CANON_PIN_VERSION='0.2.0'`, since
   its commit `23d54ac6`). Resource already carries its own `@cpp(name:)` for the two
   badge cases, so no `RD_PATCH` is needed; if the day's Resource fails `check` on 0.2.0, stop: that is
   Sovereign's state, not v0.3's, and the orchestrator patches the farm copy with `RD_PATCH` as v0.2 did,
   logged.
   - **The expected differences** against canon 0.2.0, by section: 
     - enum tables (§3): every `.gen.cpp` with a closed, kind or id enum, every `.gen.h` with a `@codes` enum
       (dense ones lose their two arrays; sparse ones' `k<E>CodeOrdinals` leaves `uint32_t`), and the Go enum
       tables; the guard triples leave the `.gen.cpp` files;
     - the runtime (§9, §4, §10.1): every `canon_runtime.h` and `canon_runtime_json.h` copy;
     - baked containers (§6): the baked packages' container classes, `Build` allocations and new `Find`
       definitions;
     - `ordered_json` (§10.1): every types-mode C++ class (+1 header line, +4 source lines);
     - TS (§7): the 51 non-conformance TS files gain marks only (strip-the-mark check);
     - function parts (§8): 10 Go files cut (monitoring `feedcat`, `telcat`, `telsem`, `items`; admin, website
       and wiki `items`; website `display`, `recordbook`; wiki `wiki`), and every Go lookup over a `@codes` enum and
       every enum input (35 files hold such a switch), whose bodies now read the enum's tables;
     - kind constants, templates and lint: no output change (Resource has none of them).
     - Expected new findings: none from `E1118`, `E3204` defaults or `E8026` (the scans found 0); `E3802` from
       D-RECLIT and failing tests from D-TESTSOFT may appear: each is real Resource data, listed in the reply with
       its count, and the release goes on.
     - `canon test` now prints Resource's warnings (its pre-push runs `check -max-warnings 0` first).
     - `canon lint` is run once on the farm, read-only, and its counts go in the reply.
   - Every other diff is classified by rule, never by judgement: a script classifies every changed
     line of the Source and service outputs by section (v0.2's method: 0 lines unclassified); a
     finding whose code or variant this ADR introduces is real Resource data the new rule catches: an
     upgrade note in the reply, and the release goes on; anything else (another code, a changed
     message, an output change no section explains) is a bug, fixed (failing test first) before the
     tag.
   - Wall time and peak RSS of `check`, `build` and `build -check`, candidate against 0.2.0 (v0.2's
     numbers: `check` 28 s and 1.93 GB, `build` 39.1 s and 2.44 GB).
6. **`make realdata-compile`** must PASS (DECISIONS 394). No tag without it. Compare its table with
   v0.2's (506 units: 118 C++, 282 Go, 106 TS; 405 s; the heaviest, `balance.gen.cpp`, 622 MB and
   17.2 s): the total time, the peak RSS of the five heaviest C++ units, and the sum of C++ unit times
   go in the reply. A C++ unit whose peak RSS or time grows by more than 10% is explained by a section
   or is a bug.
7. **Consumer compile, a one-off** (not a gate): 
   - For each of admin, monitoring, trello, website and wiki: `canon build -only-root <svc>` into a `git archive`
     copy of the service at its day's `HEAD`, then `go vet ./...` and `tsc --noEmit` with the service's own
     tsconfig where it has one. v0.3 changes no Go or TS API, so 0 breakages are expected; any one is a bug or an
     upgrade note, decided by rule as in step 5.
   - Source is not compiled. The reply confirms by scan: no hand-written `canon::rt_v1`, no `Decode({}`, and the 14
     `canon::FlatMap` sites unaffected.
8. **The final acceptance entry** in the log, with every result above.
9. **Tag and dist**, on a clean `main`, as ADR-0021 §8 step 6:
   - copy `dist/` to `/var/tmp/canon-dist-0.2.0/` (`make dist` starts with `rm -rf dist`);
   - `make tag VERSION=v0.3.0` (annotated, local, never pushed);
   - `make dist VERSION=v0.3.0`;
   - `(cd dist && sha256sum -c checksums.txt)`;
   - extract `linux_amd64` and check that `canon version` reads `canon 0.3.0 (<sha>)` with the tag's
     commit, and that its sha256 equals a candidate rebuilt at the tag on the clean tree with step 4's
     command;
   - step 5's results stand when `git diff --stat <dry-run commit> v0.3.0` touches only `meta/`;
     otherwise redo step 5 with the rebuilt candidate;
   - a failure after the tag: `git tag -d v0.3.0` (never pushed), then redo from the failing step.
10. **Never install or push** unless Louis asks in this run (he did for v0.2.0, after the reply). The
    binary goes to `~/.local/bin` only on his word, keeping 0.2.0 as `canon-0.2.0`, as 0.1.13 was kept.
11. **The reply**, committed after the tag: `meta/handoff/<date>-canon-reply-v030.md`, for the
    Sovereign sessions (Resource, Source, Scripts, website, admin, monitoring, wiki, trello,
    config-studio).
    - **What changed**, per item of §1's table, with its DECISIONS number.
    - **Upgrade notes (breaking):** 
      - a mixed C++ tree (outputs of two canon versions) now stops at compile time with canon's `#error`: regenerate
        every C++ output with one canon; this withdraws 2026-10-20's "the first copy included wins";
      - `canon test` prints its warnings and exits 4 past `--max-warnings`; a test with an error outside its `expect`s
        now fails;
      - the refusals of §14 and §15 with their Resource counts from step 5;
      - a C++ data loader refuses a NUL after the document;
      - new reserved names: `E8005` for the generated enum tables and the `Decode<N>`, `build<Last>_<N>` and
        `decode<T>_<N>` parts, `E8011` for the new runtime identifiers (0 collisions on the farm);
      - the language version stays `canon: "0.1"` (D-LANGVER).
    - **New:** `out_templates` (with Resource's optional migration: 72 Go, 33 TS and 109 view emits, byte-identical outputs); `canon lint` (with the farm's counts); C++ `Decode` of an `nlohmann::ordered_json`; kind constants; `E8005`'s working hints; the size and speed numbers of §3, §4, §7 and §8.
    - **Numbers:** the realdata-compile comparison, the dry-run times, the bench A/B, the scale bench
      (§13, for Louis's Q3), the 5 checksum lines and the `linux_amd64` sha256.
    - **What stays owed** (§19).

    Then update `meta/handoff/README.md`, `meta/plan.md` (v0.3 ticked) and `meta/state.md`.
12. **Release notes, drafted before the publish** (v0.1.9 and v0.2.0 shipped with the tag line alone): the
    orchestrator writes `/var/tmp/canon-v030/notes-v0.3.0.md` from the reply, in the GitHub layout of v0.2.0's
    notes (the downloads and `checksums.txt` lines, then What changed, Upgrade notes, Verification; no
    service-internal paths: the repository is public), and shows it to Louis with the reply.
13. **Louis pushes and publishes:** `main` and the tag, then
    `gh release create v0.3.0 dist/*.tar.gz dist/*.zip dist/checksums.txt --title v0.3.0 --notes-file /var/tmp/canon-v030/notes-v0.3.0.md`
    (never `--notes-from-tag`). After it, `gh release view v0.3.0 --json body` must show the notes.
    Scripts' canon-bump moves the pin.

## 19. Found while planning, owed (not in v0.3)

S1 writes these into `meta/plan.md`'s v0.4, v0.5 and later sections, with one pointer line in `meta/state.md`.
None is fixed in v0.3.

- **API E34 ignores `@text` results.** Renaming a field that only a `@text` output writes changes that output's key
  without adding `@json` (`internal/edit/renamewire.go:148-272`). `canon lint`'s fix avoids it (§12); E34 itself
  is a small separate item.
- **`@go`/`@ts`/`@cpp(name:)` on a public fn without `export`** are accepted and ignored (`internal/ir/units.go:93-96`);
  they need their own position word. 0 Sovereign sites.
- **TypeScript's per-file `CanonEvalError` and `CanonTable`:** `export *` over two generated modules fails with
  TS2308, and `instanceof CanonEvalError` is false for an error thrown by another file's translated fn. 0 Sovereign
  references. v0.4 (translated fns multiply): a runtime module or a brand check (`Symbol.hasInstance` by name and
  code). The guide documents it meanwhile (§17.4).
- **Foreign readers over the function budget** (C++ `Read_<Pkg>_<T>` 1,513 and Go `decode_<pkg>_<T>` 3,671 lines at
  300 fields): cutting them needs their locals in a struct. No real record is near.
- **Faster key-order sidecars** in the C++ reader (record orders by node; about 19 ms of 59 on `mdlDyna.json`):
  v0.4, with its own measurement.
- **Views of baked data and presence bits:** v0.5, with the record layout of loaded data (§6).
- **Header weight:** revisit when generated text dominates a consumer's preprocessed input, or nlohmann leaves the
  `.gen.cpp`; the helper-declaring forward-header design is in the research (§5).
- **`E8005`'s plan default:** make an unmarked C++ plan name give the always-true hint (a hardening of
  §14.1); about 30 sites.
- **A libFuzzer target for the reader** (`make fuzz-reader`, coverage-guided): optional, at a milestone's end.
- **VIEWMODEL J13/J14 through a ref:** studio work; Resource uses the shape (§15.9).
- **Windows spellings** (NTFS case, 8.3 names): until Windows CI returns.
- **MSVC at scale:** a documented limit (CODEGEN §9).
- **The on-disk cache:** Louis's Q3, decided from the scale bench's numbers (§13).
- **A leftover process:** a v0.2 builder's `sleep 15` polling loop (pid 2792522, watching
  `/var/tmp/canon-v020/subjcap-all.log`) was still running on 2026-11-02; it is Louis's to stop.

## 20. Consequences

- **Breaking for consumers** (each named in the reply; the scans found almost nothing to migrate):
  - a C++ tree whose outputs come from two canon versions now stops at compile time with canon's `#error`: regenerate
    every C++ output with one canon (it misbehaved silently before);
  - `canon test` prints its warnings and exits 4 past `--max-warnings`; a test with an error outside its `expect`s now
    fails;
  - a C++ data loader refuses a NUL byte after the document (Go and TS already did);
  - `C++` `T::Decode({}, e)` is ambiguous (0 sites);
  - new refusals: `E1118` on a local's target annotations (0 in Resource), `E3204` at a record-literal default (0),
    `E8026` before the output exists (0), unstored dependent record literals judged (`E3802`; the dry run counts
    them);
  - new reserved names: `E8005` for the generated enum tables (`k<E>Names`, `k<E>Wires`, `k<E>ByWire`) and parts
    (`Decode<N>`, `build<Last>_<N>`, `decode<T>_<N>`), `E8011` for the new runtime identifiers (`Place`, `doc_`,
    `node`, `slot`, `RecordOrders`, `DecodeOrdered`, the stamp macros): 0 Sovereign collisions.
- **New surface:** `out_templates` and a root name in `out`; `canon lint` and `Project.Lint`; C++
  `Decode(const nlohmann::ordered_json&, …)`; kind constants in Go, C++ and TS; the codes `E1019` and
  `W0001`-`W0008`; the variants `E8005` `foreignFixed`, `E8009` `outTemplate`/`outQuoted`, the `OutPath` kind; 342
  codes.
- **Frozen contracts.** Three changes go under IMPLEMENTATION-PLAN §4's review rule, with this ADR as the approval:
  `Project.Lint` (additive, as DECISIONS 396's `Graph`); `api/canon.go`'s `Options.FS` doc comment (no signature
  change); `ir.Emit.Out`'s meaning (a root name's expansion, no type change). `check.Info`, `syntax`'s frozen nodes
  and `eval.Host` are untouched.
- **Smaller and faster generated code:** Sovereign's `.gen.cpp` from 16.4 to 7.5 MB, its Go from 40.7 to 31.3 MB,
  32,335 macro guards out of the `.gen.cpp` files; C++ text reads 1.6-2.4× faster; a bundle reading one constant of a
  generated TS module carries that constant alone; no generated function over 1,000 lines but one-statement literals.
- **Robustness gained:** two runtime copies can no longer mix silently; a data loader refuses trailing bytes; a
  long-lived Project stays bounded; a record-literal default and an unstored dependent literal are checked; a test
  cannot pass over its own error; `canon test` cannot hide a warning; a build's output cannot pass its first check
  and fail its second; `E8005`'s advice always works; a target annotation on a local is no longer a silent no-op.
- **Costs:** 14 bytes per runtime symbol (+0.3-0.4% object size); about 3,800 more lines of C++ for `ordered_json`;
  +1.6% raw TS bytes for the marks (+0.27% gzip); one `canon lint` analysis per call.
- **Residual risks, accepted:** MSVC is unverified for the stamp check, the namespace-scope tables and the cut
  functions (no `cl.exe`); the scale bench's 5× may exceed 12 GB; several readings rest on scripted cuts and models
  (the failing-test-first rule turns a wrong reading into a stopped unit, not a wrong release).
- **Unchanged on purpose:** the language version stays 0.1; the compiler's own speed (robustness before speed); one
  self-contained runtime per output directory for C++ and Go; the TS helper block per file; header weight; widths
  from declared types (GEN-02).
