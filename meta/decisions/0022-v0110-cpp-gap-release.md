# ADR-0022 — v0.1.10, the C++ gap release: every decision, and how the run builds it

Date: 2026-10-20. Status: accepted. Scope: Louis, 2026-10-20 ("develop a v0.1.9.1 just for the
gap"; semver has no fourth number, so the tag is `v0.1.10`). Designs: the orchestrator (technical
calls are its own, DECISIONS 207), from read-only research over `main` at `b9e46938` and a probe
compiled with g++ 17. Answers [Sovereign's C++ emit gaps](../handoff/2026-10-20-sovereign-cpp-vocab-and-types-gaps.md);
the asks that need no Canon change are answered in [the reply](../handoff/2026-10-20-canon-reply-cpp-gaps.md).

## 0. How to run this ADR

- **It is decision-complete.** The run builds what it says; it does not redesign. Each *Decision*
  binds; every "Spec text" and "DECISIONS text" block is final wording, applied verbatim by S1 (§8).
- **DECISIONS numbers.** 383-388 are the next free numbers on `b9e46938` (382 is the last). If one
  is taken when the run starts, shift every number of this ADR by the same offset and log it.
- **File:line references** are from `b9e46938`. They locate; they do not bind.
- **Fallback, gaps, questions:** ADR-0021 §0 applies word for word (strictest reading, log it,
  never widen the scope; a real gap stops its unit only; only a philosophy question goes to Louis).
- **Scope guard.** Sovereign's asks 1, 3 (members), 5 (root decoders) and 7 need no Canon change
  (the reply). Nothing here touches Go or TypeScript output, except where a section says so.

## 1. Context and scope

Louis ruled on 2026-10-20 that every Sovereign Canon package emits C++ (Source's L-0138
amendment). Sovereign's probe added `emit cpp` to every package of a scratch `Resource/Config`. It
found 110 refusals in C++ `types` mode. It also found that `items/vocab.gen.cpp` takes 137 s at -O2
and `text/vocab.gen.cpp` longer.

| # | Item | § | DECISIONS | Unit (§8) |
|---|---|---|---|---|
| 1 | Enum helpers are member chains inline in the header | 2 | 383 | HELPERS |
| 2 | C++ `types` decoders refuse maps (94 of the 110) | 3 | 384 | TYPES |
| 3 | C++ refuses lists of optional elements and maps of optional values | 4 | 385 | TYPES |
| 4 | C++ `types` refuses a record or variant default | 5 | 386 | TYPES |
| 5 | C++ refuses a literal union over a ref | 6 | 387 | TYPES |
| 6 | A C++ name that is a standard-library macro breaks the build silently | 7 | 388 | MACRO |
| 7 | Release | 9 | none | orchestrator |

## 2. Enum helpers are defined in the `.gen.cpp`, as tables (DECISIONS 383)

**Facts.**
- Closed enums and variant kind enums share `g.enum` (`internal/gen/cpp/enums.go:88-114`). It
  writes everything inline in the header:
  - `ToName` and `ToWire` as switches (`nameSwitch` :117-127);
  - `<E>FromWire` as an `if (wire == "…")` chain (:129-137);
  - `<E>FromCode` as a switch (:139-149).
- Open enums (`enums_open.go:20-75`) write `ToName` and `ToCode` as `==` chains over the members and
  `IsKnown` through `FromWire`. `<E>FromWire` reuses the closed chain. All of it is inline in the
  header.
- Lookup getters and `baked` constexpr lookups inline a whole `switch` per `@codes` enum parameter
  to compute the ordinal (`stored.go:112-128`, `baked_api.go:130-156`). CODEGEN §5.10 says the
  ordinal is "computed by a `constexpr` function", so the code drifts from the spec.
- Table ids already do the right thing: `<Rec>IdFromWire` is declared in the header and defined in
  the `.gen.cpp` as a binary search (`idenums.go:67-80`, `text/id_from_wire.txt`).
- Nothing needs the helpers inline:
  - no constexpr lookup calls them;
  - `canon_runtime_json.h` takes `&<E>FromWire` as a runtime function pointer;
  - `IsOpenEnum` uses an unevaluated `decltype`;
  - every emit writes a `.gen.cpp` (`gen.go:165`, CODEGEN §2.3);
  - cross-package callers already link the owner's `.gen.cpp` for id enums.
- Resource: `ItemId` is a `@codes(UInt32)` enum of 7,306 members and `TextId` a `@codes(UInt16)`
  enum of 11,351. Both are opened in their Go emits.

**Decision.**
1. **Closed enums and kind enums.**
   - `ToName`, `ToWire`, `<E>FromWire` and `<E>FromCode` are declared in the header and defined in
     the `.gen.cpp`, in the package namespace, after `detail`. The order follows §2.7: each enum's
     helpers in enum order, before the id enums' `FromWire`.
   - Every definition reads tables and never writes a chain or a switch over the members. Its
     tables are `static constexpr` locals of the function, as in `id_from_wire.txt`.
2. **Ordinals.**
   - Without `@codes`, the ordinal is `static_cast<std::size_t>(v)`.
   - With `@codes`, the header defines, in `detail`:
     - `inline constexpr std::array<CodeT, N> k<E>Codes`: the codes, ascending;
     - `inline constexpr std::array<std::uint32_t, N> k<E>CodeOrdinals`: the parallel declaration
       positions;
     - `inline constexpr std::size_t <E>Ordinal(E v)`: a binary search of `k<E>Codes`, returning
       `N` for a value that is no member.
   - Lookup getters and constexpr lookups call `detail::<E>Ordinal`. They abort on `N`, as the
     `default: std::abort()` does today.
   - Only enums used as a lookup parameter or by `<E>FromCode` get these three.
3. **Each helper.**
   - `ToName(v)` and `ToWire(v)`: a `std::array<std::string_view, N>` in declaration order, indexed
     by the ordinal. A value that is no member gives `{}`.
   - `<E>FromWire(wire)`: the wires stable-sorted by their bytes with the parallel enumerators, read
     by `id_from_wire.txt`'s lower-bound search. Equal wires keep declaration order, so the first
     declared wins, as the chain does today.
   - `<E>FromCode(code)`: the same search over `detail::k<E>Codes`, then `k<E>Members`.
4. **Open enums.**
   - `ToWire` (the stored wire) and `std::hash` stay inline.
   - `ToName`, `IsKnown`, `<E>FromWire`, and with `@codes` `ToCode` and `<E>FromCode`, move to the
     `.gen.cpp`. Each is a binary search over the wires sorted by bytes (or over the codes for
     `FromCode`) with parallel arrays of names, codes or wires.
   - `<E>FromWire` returns `E(std::string(wire))` for a known wire. That is equal to the member,
     because `==` compares wires.
5. **Names.** `detail::k<E>Codes`, `detail::k<E>CodeOrdinals` and `detail::<E>Ordinal` go through
   the C++ name plan (`internal/ir`, the `detail` scope), so a clash is `E8005` like every fixed
   name (§3.5).
6. **Empty enums.** An enum with zero members keeps compiling: `std::array<T, 0>` is valid, and the
   search returns before indexing.
7. **Unchanged.** `k<E>Members` stays an `inline constexpr std::array` in the header. Variant
   decoding (`decode_more.go:69-91`) keeps its own tag chain: it is per case, not per member.

**Spec text** (CODEGEN.md §5.2, replacing the paragraph "`ToName`, `ToWire` and `<E>FromWire` are
`inline` in the header. … (this replaces SPEC §15.3's `FromWire`)."):

> `ToName`, `ToWire`, `<E>FromWire` and `<E>FromCode` are declared in the header and defined in the
> `.gen.cpp`, so a header costs the same whatever the size of its enums (DECISIONS 383); a program
> using them links the enum package's `.gen.cpp` (§2.3: always written). `ToName`/`ToWire` are
> overloads in the package namespace; `FromWire` carries the enum name because C++ cannot overload
> on the return type (this replaces SPEC §15.3's `FromWire`). Each definition reads tables, never a
> chain or a switch over the members: `ToName` and `ToWire` index arrays in declaration order by the
> member's ordinal; `<E>FromWire` binary-searches the wires sorted by their bytes (equal wires keep
> declaration order, so the first declared wins), as `<Rec>IdFromWire` does (§5.3); `<E>FromCode`
> binary-searches the codes. A value that is no member gives `""` or `std::nullopt`. A member's
> ordinal is its declaration position: `static_cast<std::size_t>(v)` without `@codes`; with them,
> `detail::<E>Ordinal(v)`, an `inline constexpr` function of the header searching
> `detail::k<E>Codes` (the codes, ascending) and returning the matching
> `detail::k<E>CodeOrdinals` entry, or the member count for a value that is no member. A variant's
> kind enum (§5.5) follows the same rules.

**Spec text** (CODEGEN.md §5.2, open enums, appended after the C++ block of `Dst`):

> `ToWire` and the `std::hash` specialisation are inline in the header; `ToName`, `IsKnown`,
> `<E>FromWire`, `ToCode` and `<E>FromCode` are defined in the `.gen.cpp` as binary searches over the
> member wires sorted by bytes (the codes for `<E>FromCode`), with parallel arrays (DECISIONS 383).

**Spec text** (CODEGEN.md §5.10, in "C++ `constexpr` lookups"):
- Replace "(an enum's ordinal is its declaration position, never its code, computed by a
  `constexpr` function)" with "(an enum's ordinal is its declaration position, never its code:
  `static_cast<std::size_t>(e)` without `@codes`, `detail::<E>Ordinal(e)` with them, §5.2)".
- In the sample, replace `detail::Ordinal(e)` with `static_cast<std::size_t>(e)`.

**Spec text** (CODEGEN.md §7.1, replacing the bullet "The header declares; … are inline in the
header."):

> - The header declares; `.gen.cpp` defines loaders, accessors, lookup tables, baked data, enum and
>   id-enum lookups (`ToName`, `ToWire`, `FromWire`, `FromCode` and an open enum's `IsKnown` and
>   `ToCode`) and `detail::<P>Access`. Getters, `k<E>Members`, `detail::<E>Ordinal` and its two
>   arrays, an open enum's `ToWire`, pure translated functions and `baked` lookups with a constexpr
>   scalar result (§5.10, DECISIONS 293) are inline in the header.

**DECISIONS text:**

> 383. **C++ enum helpers are tables in the `.gen.cpp` (CODEGEN.md §5.2, §5.10, §7.1).**
>      `ToName`, `ToWire`, `<E>FromWire`, `<E>FromCode` and an open enum's `ToName`, `IsKnown` and
>      `ToCode` are declared in the header and defined in the `.gen.cpp`, each reading arrays:
>      names and wires by ordinal, a lower-bound binary search over the wires sorted by bytes
>      (stable, so the first declared of equal wires wins) or over the codes. A `@codes` enum's
>      ordinal is `detail::<E>Ordinal`, an `inline constexpr` binary search of
>      `detail::k<E>Codes`, which lookup getters and constexpr lookups call instead of inlining a
>      switch. Reason: Sovereign's `ItemId` (7,306 members) made a 137 s `.gen.cpp` at -O2 and
>      charged every TU including the header; table ids already worked this way (§5.3).

**Tests.**
- A Go test over a generated header asserts that no per-member function body is in the header: no
  `if (wire ==`, and no `case` inside a helper. The fixture has three enums: plain, `@codes`, and
  open with `@codes`.
- The compile-and-run suite (`cxxbuild_test.go`, every toolchain row) checks every helper:
  - against every member, both ways;
  - on a value that is no member (`static_cast<E>(255)`, an unknown wire, an unknown code);
  - on two members with equal wires (the first declared wins);
  - on a zero-member enum.
- A size test generates a 12,000-member `@codes` enum in a temp dir (never committed). It asserts:
  - the header's helper section stays linear in the member count;
  - one `.gen.cpp` compiles at `-O2` under `cxx.Timeout`. It is skipped under `-short`; it is not a
    timing gate (§9 measures).
- Goldens: regenerate every `internal/gen/cpp/testdata/golden/*` and `examples/**/expected` C++
  directory holding enums. The research counted 16 golden directories and 6 example directories.
  Read the diff.

## 3. C++ `types` decoders read maps (DECISIONS 384)

**Facts.**
- `readsMaps` (`internal/ir/rules_gen_cpp.go:30-33`) is false for C++ `types`, so `unreadMap`
  reports `MapField` at every class of the package and through foreign readers (`judgeMaps`,
  `rules_gen_foreign.go:188-193`). `decode_map.go:13` refuses in the generator too.
- The refusal's reason (nlohmann keeps no key order) was solved by DECISIONS 361. `T::Parse` and
  `Decode<Fn>File` run `ParseTextFile`, which records file order in the `KeyOrders` sidecar. They
  then run the same decode as `T::Decode`. `detail::Members` already iterates that order, as nested
  tables do (`tablefield_compile_test.go:51-69`).
- C++ `data` mode reads maps with `decodeMap`/`mapKey` (`decode_map.go:12-49`) for every key kind of
  WIRE §5.8. Go and TS `types` decoders read maps. This is a C++-only gap.

**Decision.**
- C++ `types` decoders read map fields and maps anywhere in a decoded class with the `data`
  decoder's code (`decodeMap`, `mapKey`). Keys follow WIRE §5.8.
- **Ref keys:** read as keys and never checked. `types` mode checks no ref (§5.13); `walkMap`'s ref
  checks stay `data`-only.
- **Order:** `Parse` and `Decode<Fn>File` keep file order; `Decode` of a parsed `nlohmann::json`
  holds key byte order. This is the nested-table split of DECISIONS 290.
- **IR:** `readsMaps` becomes true for every decoded emit (`data`, and `types` of every target).
  Lifting `judgeMaps` for C++ `types` follows.
- **`@text` results:** a result holding a map inside a record now gets `Decode<Fn>File`
  (`text_decoded.go`).
- **`MapField` retirement:** if no rule reports `MapField` afterwards (the unit proves it with a
  grep and a test run), CLEANUP removes the kind's reporting code and the `MapField` and
  `WayMapField` rows of ERRORS.md §1.4 (§8.2). If a path still reports it, the kind stays and the unit
  reports the path.

**Spec text** (CODEGEN.md §5.9, `data` bullet): replace "; a C++ `types`-mode decoder still refuses
maps (`E8019` `MapField`; §5.13: its JSON keeps no key order)" with "; `types`-mode decoders read
them too, in every target (§5.13, DECISIONS 384)".

**Spec text** (CODEGEN.md §5.13, first bullet): replace "a nested table it decodes holds its entries
in key byte order" with "a nested table or a map it decodes holds its entries in key byte order".

**Spec text** (CODEGEN.md §5.13): delete the sentence "A map inside a record stays `E8019`
`MapField` in C++ `types` mode, and a result holding one gets no decoder." and turn the next
sentence's "Otherwise a runtime" into "A runtime".

**DECISIONS text:**

> 384. **C++ `types` decoders read maps (CODEGEN.md §5.9, §5.13; amends 312, 363).** A map in a
>      class a C++ `types` emit decodes is read as the `data` loader reads it (WIRE.md §5.8 keys),
>      refs unchecked as everywhere in `types` mode; `Parse` and `Decode<Fn>File` keep file order,
>      `Decode` of a parsed `nlohmann::json` holds key byte order, as nested tables (290). A `@text`
>      result holding such a map gets its decoder. `E8019` `MapField` is no longer reported by C++
>      `types`. Reason: 361's `KeyOrders` removed the refusal's cause; Sovereign's every-package C++
>      emit met 94 such refusals.

**Tests.** `E8019_16`, `_46` and `_108` become clean cases. Move each into
`internal/ir/testdata/gensupport/` with the same source. `E8019_99` now reports `OptionalMapValue`
or nothing, per §4. Update `textdecoders_targets/targets.txtar`, which must now list
`DecodeTaggedFile`. Add a compile-and-run case, modelled on `TestTableFieldTypesCompilesAndRuns`,
with:
- every key kind (String, Int, enum, enum with `@json(codes)`, ref, literal union);
- a map of maps, and a map in a list;
- `Decode` byte order against `Parse` file order;
- a non-canonical integer key refused at its path.

## 4. C++ holds optional elements and optional map values (DECISIONS 385)

**Facts.**
- `checkOptionalElements` and `checkOptionalMapValues` (`rules_gen_types.go:59-74`) are in `common`
  (`rules_gen.go:20-24`). Every Go and C++ emit refuses them, in every mode.
- In the generator, `listStorage` and `storage` have no optional case (`ctype.go:61-72, 103-105`).
- TS writes both. WIRE.md: "`none` inside a list or map value (`[T?]`, `{K: V?}`) is always `null`".
- Resource: `character/skills.canon:197` `destData1: [Cell?](..=3)?`, `:296`, and
  `character/troupe.canon:122-128`.

**Decision.**
- **Type.** C++ holds `[T?]` as `std::vector<std::optional<T>>` and `{K: V?}` as
  `canon::FlatMap<K, std::optional<V>>`, whatever `T` is. A record or variant is held by value, as
  in `[R]`. `none` is `std::nullopt`.
- **Every C++ mode:**
  - `baked` writes `std::nullopt` elements;
  - `data` and `types` decode `null` (or the field's `@json(none:)` value where WIRE §5 applies it)
    as `std::nullopt`;
  - a nested optional inside them follows the same rule.
- **IR.** The two rules leave `common` and apply to Go emits only.
- **Unchanged.** Go keeps both refusals (owed, §10).

**Spec text** (CODEGEN.md §4.2, two rows after `[R]`):

> | `[T?]` | `E8019` `OptionalElementList` | `const std::vector<std::optional<T>>&` | `ReadonlyArray<T \| null>` |
> | `{K: V?}` | `E8019` `OptionalMapValue` | `const canon::FlatMap<K, std::optional<V>>&` | `ReadonlyMap<K, V \| null>` |

and after the table's paragraphs:

> An optional list element or map value is `std::optional<T>` in C++ whatever `T` (a record or
> variant by value, as `[R]`'s elements), `none` being `std::nullopt` in every mode (WIRE.md:
> `null` inside a list or map); Go refuses both (`E8019`), TypeScript writes `T | null`
> (DECISIONS 385).

**DECISIONS text:**

> 385. **C++ holds optional list elements and map values (CODEGEN.md §4.2).** `[T?]` is
>      `std::vector<std::optional<T>>` and `{K: V?}` is `canon::FlatMap<K, std::optional<V>>`
>      in every C++ mode: baked literals write `std::nullopt`, decoders read `null` as it.
>      `E8019` `OptionalElementList` and `OptionalMapValue` stay for Go only. Reason: Sovereign's
>      skill tables hold `[Cell?]`; C++ has an exact type for it, Go's parity is owed.

**Tests.** `E8019_15` (C++ `data`) becomes a clean case, and the Go cases stay. A compile-and-run
case covers:
- `[Int?]`, `[String?]`, `[Rec?]`, `{String: Int?}` and `{Enum: Rec?}`;
- baked literals, `data` loading and `types` `Parse`/`Decode`;
- a `null` and a present value at each position.

## 5. C++ `types` builds record and variant defaults (DECISIONS 386)

**Facts.**
- `checkCppDefaults`/`cppDefault` (`rules_gen_cpp.go:52-92`) refuse a default for which
  `holdsRecordValue` is true: a record, a variant, a list or map holding one, or a non-empty table.
  The refusal applies in C++ `types` and in foreign readers (`rules_gen_foreign.go:20, 238-240`).
- `literal()` writes no record (`literal.go:96-98`). The `baked` filler (`baked_fill.go:46-305`)
  builds every value kind with statements, including another package's record through its make
  hook (`fillForeign`).
- Go and TS `types` decoders write such defaults. DECISIONS 360 kept the variant case `E8019`.
- Resource: `ai/monsters/giantSlayer.canon:34` `levelDifference: GiantSlayerLevelDifference = {}`,
  and `content/modes/cardCollection.canon:204-209` (optional fields with record defaults).

**Decision.**
- In a C++ `types` decoder, a default that `holdsRecordValue` is built once, on first use, into a
  function-local `static const` of the field's storage type. The build uses the statements of the
  `baked` filler (`fillValue`), with another package's records built through their make hooks.
- The decoder copies that value into each decoded value whose key is absent. This covers a field,
  and `closePath`'s absent intermediate objects.
- Initialisation of a C++11 function-local static is thread-safe and needs no exceptions.
- Foreign readers lift the same refusal.
- **`RecordDefault` retirement:** handled as `MapField`'s is (§3).

**Spec text** (CODEGEN.md §5.13, the "Defaults must be constant" bullet, appended):

> In C++, a default holding a record, a variant, a non-empty table, or a list or map of them is
> built once, on first use, into a function-local `static const` by the statements a `baked` emit
> fills its data with (another package's record through its make hook, §5.14), and copied into
> each decoded value whose key is absent (DECISIONS 386).

**DECISIONS text:**

> 386. **C++ `types` decoders build record and variant defaults (CODEGEN.md §5.13; amends 360).**
>      A field default holding a record, variant, non-empty table, or list or map of them is built
>      once into a function-local `static const` by the `baked` filler's statements and copied
>      where the key is absent; `E8019` `RecordDefault` is no longer reported. 360's "a
>      `types`-mode C++ default holding a variant stays `E8019`" is withdrawn. Reason: Go and
>      TypeScript already decode them; Sovereign's monster and card tables default records.

**Tests.**
- `e2e/refused/record_default.txtar` and `builds/cpp_table_default_types.txtar` become clean
  cases.
- `types_test.go:36-41` changes accordingly.
- A compile-and-run case covers:
  - a record default (`= {}` with defaulted fields);
  - an optional field with a non-none record default;
  - a variant default;
  - a list-of-records default;
  - a non-empty table default;
  - another package's record default;
  - an absent intermediate object;
  - two decodes sharing the static, the first mutated after copy, the second unchanged.

## 6. C++ `types` holds a literal union over a ref (DECISIONS 387)

**Facts.**
- `checkRefUnions` (`internal/ir/rules_defines.go:22-35`) refuses, for C++, every type site in every
  mode. Its target is a literal union whose element is a ref with a string key.
- In the generator, `refuseUnion` refuses at storage and decode level (`ctype.go:77-95`,
  `decode.go:267-272`).
- CODEGEN §4.1 already maps `A | "lit"` to `const std::string&`.
- Resource: `events/exchange.canon:16` `itemId: ref items | "PENYA"`, and `ui/texts.canon:19`
  `type ResTextKey = ref resTexts | ""` (used at `ui/layout.canon:53, 55, 77, 79`).

**Decision.**
- In a C++ `types` emit, a literal union over a string-keyed ref is a `std::string`, by §4.1's row.
  It is decoded with `AsString`, and neither the ref nor the literal is checked: `types` mode
  checks no ref (§5.13), and the file is the one `canon check` validated.
- The same holds as a map key, through an alias, optional, or in a list.
- **Unchanged:** `baked` and `data` C++ emits keep `RefUnion` (each would need a check), and so
  do Go emits (owed, §10).

**Spec text** (CODEGEN.md §5.13, appended to "Refs are keys; table ids are strings."):

> A literal union over a ref (`ref items | "PENYA"`) is its string in a C++ `types` decoder, read
> unchecked like every ref there, as TypeScript reads it (DECISIONS 387); C++ `baked` and `data`
> emits and Go keep `E8019` `RefUnion`.

**DECISIONS text:**

> 387. **C++ `types` holds a literal union over a ref (CODEGEN.md §4.1, §5.13).** Stored and decoded
>      as its string, unchecked, as every ref in `types` mode; C++ `baked`/`data` and Go keep
>      `E8019` `RefUnion`. Reason: Sovereign's exchange (`ref items | "PENYA"`) and layout
>      (`ref resTexts | ""`) records; TypeScript already reads it so.

**Tests.** `E8019_51` (C++ `types`) becomes a clean case, and `E8019_50` (C++ and Go `data`)
stays. `dependent_refusals_test.go:91-92` changes accordingly. A compile-and-run case covers:
- the ref arm and the literal arm;
- an optional union;
- an alias;
- a list of unions;
- a map keyed by the union.

## 7. A C++ name that is a standard-library macro is an error (DECISIONS 388)

**Facts.**
- `W8006` warns on exact matches of a platform-macro list (`internal/ir/constants.go:403-411`,
  `cppplan.go:57-67`, check `goplan_scopes.go:416-424`).
- Every generated C++ file includes `canon_runtime.h`, which includes `<cstdint>`, `<cstdio>`,
  `<cstdlib>`, `<cmath>`, `<cstddef>`, `<atomic>` and others. JSON modes add
  `canon_runtime_json.h` (`<cctype>`, `<clocale>`, `<cstring>` …), and the header adds what
  `text/std_headers.txt` matches (`<cerrno>`, `<cfloat>` …).
- So a name such as `UINT32_MAX` can never compile. The probe confirmed it: no finding, and g++
  fails.
- Sovereign's `engine` constants `INT32_MAX` and `UINT32_MAX` and `items` `NAME_MAX` were reported.
  `NAME_MAX` is POSIX (`<limits.h>` on glibc), not C++.

**Decision.**
1. **The error.** A generated C++ name equal to a macro that a C++17 header generated code can
   include defines is `E8011` with the new variant `macro`. It applies in every position the C++
   name plan declares, overrides included.
   - A derived name gets `macro`.
   - An override gets the existing `override` variant: the name is not a usable identifier.
2. **The headers.** The set is every macro named in the C++17 synopses of `<atomic>`, `<cerrno>`,
   `<cfloat>`, `<clocale>`, `<cmath>`, `<cstddef>`, `<cstdint>`, `<cstdio>`, `<cstdlib>` and
   `<cstring>`, object-like and function-like alike. Examples: `INT32_MAX`, `UINT32_MAX`,
   `SIZE_MAX`, `INT64_C`, `EOF`, `NULL`, `BUFSIZ`, `SEEK_SET`, `stdin`, `stdout`, `stderr`,
   `EXIT_SUCCESS`, `RAND_MAX`, `HUGE_VAL`, `INFINITY`, `NAN`, `FP_NAN`, `math_errhandling`,
   `errno`, `EDOM`, `ERANGE`, the POSIX `E*` names `<cerrno>` lists, `DBL_MAX`, `FLT_EPSILON`,
   `LC_ALL`, `offsetof`, `ATOMIC_FLAG_INIT`.
3. **The list itself.** MACRO enumerates the set from the standard's synopses into one sorted table
   in `internal/ir/constants.go`. S2 pastes the same list into CODEGEN §3.4, and a test asserts the
   two are equal.
4. **`W8006` additions:**
   - `<climits>`'s names (`CHAR_BIT`, `SCHAR_MIN` … `ULLONG_MAX`, `MB_LEN_MAX`): libraries
     include `<climits>` transitively, but not always;
   - POSIX `NAME_MAX` and `PATH_MAX`;
   - glibc `<cmath>`'s `M_E`, `M_LOG2E`, `M_LOG10E`, `M_LN2`, `M_LN10`, `M_PI`, `M_PI_2`,
     `M_PI_4`, `M_1_PI`, `M_2_PI`, `M_2_SQRTPI`, `M_SQRT2` and `M_SQRT1_2`.
5. **The way out** is `@cpp(name:)`. The Canon name, the wire and the lock are unchanged.

**Spec text** (CODEGEN.md §3.4, appended after "A name starting with `_` followed by an uppercase
letter, or containing `__`, is `E8011` (a reserved name, not a collision)."):

> A C++ name equal to a macro of the C++17 headers generated code includes is `E8011` too, in every
> position, overrides included: those headers are always included (§7.1, §7.4, §7.5), so the name
> can never compile; rename it with `@cpp(name:)` (DECISIONS 388). The names are the macros of the
> C++17 synopses of `<atomic>`, `<cerrno>`, `<cfloat>`, `<clocale>`, `<cmath>`, `<cstddef>`,
> `<cstdint>`, `<cstdio>`, `<cstdlib>` and `<cstring>`: [S2 pastes MACRO's sorted list here].

**Spec text** (CODEGEN.md §3.5, the `W8006` paragraph). After "… `GetFileAttributes GetProp SetProp
CreateWindow CreateFile DeleteFile CopyFile MoveFile LoadImage LoadString DrawText SendMessage
PostMessage RegisterClass`", insert:

> ; `<climits>`'s `CHAR_BIT SCHAR_MIN SCHAR_MAX UCHAR_MAX CHAR_MIN CHAR_MAX MB_LEN_MAX SHRT_MIN
> SHRT_MAX USHRT_MAX INT_MIN INT_MAX UINT_MAX LONG_MIN LONG_MAX ULONG_MAX LLONG_MIN LLONG_MAX
> ULLONG_MAX`, POSIX `NAME_MAX PATH_MAX`, and glibc's `M_E M_LOG2E M_LOG10E M_LN2 M_LN10 M_PI
> M_PI_2 M_PI_4 M_1_PI M_2_PI M_2_SQRTPI M_SQRT2 M_SQRT1_2`

**ERRORS.md** (in MACRO's worktree, §8.2). Add the message row after `E8011 reserved`:

> | E8011 | macro | name:Text, origin:Name | `{name}, the C++ name derived for {origin}, is a macro of the C++ standard library` |

Then update the count sentence to 554 messages. The `E8011` code row is unchanged.

**DECISIONS text:**

> 388. **A C++ name that is a standard-library macro is `E8011` (CODEGEN.md §3.4, §3.5).** A
>      generated C++ name, overrides included, equal to a macro of the C++17 synopses of the
>      headers generated code includes (`<atomic> <cerrno> <cfloat> <clocale> <cmath> <cstddef>
>      <cstdint> <cstdio> <cstdlib> <cstring>`) is `E8011` `macro`; `W8006` gains `<climits>`'s
>      names, POSIX `NAME_MAX`/`PATH_MAX` and glibc's `M_*` constants. Reason: a constant
>      `UINT32_MAX` broke the generated header with no finding (Sovereign, 2026-10-20); a name that
>      can never compile is an error (25), a name that may is a warning.

**Tests.**
- `E8011_<n>.txtar` cases:
  - a constant `UINT32_MAX`;
  - an enum member `EOF`;
  - a getter derived as a macro;
  - `@cpp(name: "SIZE_MAX")` as an override;
  - a renamed `INT32_MAX` (clean).
- `W8006_3.txtar` covers `NAME_MAX`, `INT_MAX` and `M_PI`.
- A Go test asserts that the table equals the spec list once S2 lands. Until then, it asserts the
  table is sorted with no duplicates, and that it contains the examples above.

## 8. The run

### 8.1 Rules

ADR-0021 §7.1 applies word for word: the memory caps, at most 2 heavy test runs at once,
worktrees with cherry-pick, `ListAgents` before saying units run, every prompt ending "send your
final report; do not stop without it", and review by the same builder and reviewer. Status for
Louis goes in `meta/handoff/2026-10-20-v0110-status.md`.

### 8.2 Steps

**S1, spec sync** (orchestrator, before any builder; commit
`docs(spec): v0.1.10 decisions 383-388 (ADR-0022)`):
- the DECISIONS texts 383-388, appended verbatim;
- every "Spec text" block, verbatim, except §7's macro list;
- `amended by 384` on 312 and 363; `amended by 386` on 360.

**ERRORS.md edits.** `internal/diag` is generated from ERRORS.md, so `make check` fails between
an ERRORS.md edit and the regenerated catalogue. Each such edit is committed by the orchestrator
inside the worktree of the unit that needs it, before that builder starts. The builder then
regenerates `internal/diag` and lands both together:
- the `E8011 macro` row and the count sentence go into MACRO's worktree;
- the retired kind rows go into CLEANUP's worktree.

**Wave 1**, three builders in parallel, each in its own worktree:

| Unit | Model | Packages | Builds |
|---|---|---|---|
| HELPERS | opus (generated-C++ performance) | `internal/gen/cpp` (enums, open enums, stored, baked_api, idenums, text templates); `internal/ir` (C++ name plan: the `detail` names only) | §2 |
| TYPES | opus (decoder semantics) | `internal/ir` (`rules_gen*.go`, `rules_defines.go`, `text_decoded.go`); `internal/gen/cpp` (decode*, ctype, types, textdecode, baked_fill reuse) | §3, §4, §5, §6, one commit each, in that order |
| MACRO | sonnet (a table plus txtar, gated by findings tests) | `internal/ir` (constants, cppplan, goname, rules_names); `internal/diag` (regenerated with `go run ./internal/diag/cmd/diaggen` after S1) | §7 |

Each prompt cites its § of this ADR, CODEGEN.md's sections, DOCTRINE §3-§5 and
`.claude/rules/go.md`. VERIFY is `GOTOOLCHAIN=local make check` and the package tests under the
memory cap.

**Integration.** Cherry-pick in this order: MACRO, then HELPERS, then TYPES. A conflict in a golden
is resolved by regenerating with `-update` on the merged tree and reading the diff, never by
picking a side. A conflict in `internal/ir` name-plan code is a ≤ 20-line orchestrator fixup, or
goes back to the later unit. Run `make check` before every `cherry-pick --continue`.

**Review.** One `spec-reviewer` (opus) per unit, against this ADR's § and CODEGEN.md.

**CLEANUP** (sonnet, after the three land):
- remove helpers invented twice (`dup-in-repo`);
- remove the `MapField`/`RecordDefault` reporting code if TYPES proved it unreachable;
- update `canon guide` topics and examples that name the lifted refusals or their ways out (grep
  `MapField`, `WayMapField`, `RecordDefault`, `RefUnion`, `OptionalElementList` under
  `internal/guide` and `examples/guide`).

Then a review.

**S2** (orchestrator):
- the macro list into CODEGEN §3.4;
- `meta/state.md` and `meta/plan.md`.

## 9. Release

Only after CLEANUP is reviewed and S2 is committed. Each step's command, load and result go in
`meta/decisions/log-<date>.md`.

1. **`make check`** under `systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local`, on
   a clean `main`.
2. **Cross-platform:** `GOOS=windows go vet ./...` and `GOOS=darwin go vet ./...`. Then run the C++
   compile suites on every `cxx.Toolchain` row present (g++, clang++, clang++ with libc++).
3. **Candidate:** build it as ADR-0021 §8 step 4 does, with `CompilerVersion=0.1.10`, into
   `/var/tmp/canon-0110/`.
4. **Resource dry run.** Follow ADR-0021 §8 step 4 (read-only on Resource and Source). The outputs
   must equal Resource's committed ones, except Source's generated C++. That code may differ only
   by §2's moves: helpers out of the header, the ordinal function, and the regenerated `.gen.cpp`.
   Read that diff.
5. **The Sovereign C++ farm** (the acceptance of Sovereign's asks 2, 4, 5 and 6; read-only on
   Resource, Source and Scripts):
   - **Setup:** `git -C ../../Resource archive HEAD` into `/var/tmp/canon-0110/cpp/res`, plus the
     dry run's `@source` inputs.
   - **Script:** a script in the scratch directory adds `emit cpp { out: "@cppfarm/<package path>",
     mode: <m> }` to every package without a C++ emit.
     - `<m>` is the mode Source's census (`Source/meta/canon-loaders-census.md`) gives the package.
     - Without a census entry, `types` for a package with runtime data and `baked` otherwise.
     - Vocab packages: `baked`.
     - The script also adds Sovereign's own annotations, the ones the reply tells it to write: the
       reserved members' and macro constants' `@cpp(name:)`. `world.clientworld` goes to `types`.
   - **Pass conditions:**
     - (a) `canon build` reports no `E8019` of kind `MapField`, `RecordDefault`, `RefUnion`, or of
       `OptionalElementList`/`OptionalMapValue` in a C++ emit.
     - (b) Every other C++ finding is listed with its cause. A Canon defect blocks the tag; a
       Sovereign-side cause goes into the reply.
     - (c) Every generated `.gen.cpp` passes `g++ -std=c++17 -fsyntax-only` with nlohmann.
     - (d) `items/vocab.gen.cpp` and `text/vocab.gen.cpp` each compile with `g++ -std=c++17 -O2 -c`
       in under 10 s, in both `baked` and `types` (open: `[ItemId]`/`[TextId]`) farms.
     - (e) A TU including both vocab headers and calling `ItemIdFromWire` and `TextIdFromWire`
       compiles at `-O2` in under 3 s.
     - Timings are the median of 3, with load recorded.
6. **Tag and publish.** Tag `v0.1.10` locally and run `make dist`; push and publish are Louis's. Fill
   the reply's "Coming in v0.1.10" section with the farm's numbers, and update `meta/state.md`.

## 10. Owed, not in v0.1.10

- **Go parity:** `[T?]`, `{K: V?}` and a literal union over a ref in Go `types` (and `data`)
  decoders. Go still refuses them (385, 387).
- **C++ `baked`/`data` literal union over a ref:** it needs a check of the ref arm.
- **`Decode<Fn>File` naming:** a fn named `…File` gives `…FileFile`. Renaming is public API in three
  targets, so it is left for v0.2.
- **An `ordered_json` overload of C++ `Decode`** (DECISIONS 290's later item). Until then `Decode`
  holds maps and nested tables in byte order.
- **Declined:**
  - a C++ name for a table key: the way out is `types` mode;
  - a mangling rule for reserved names: it collides;
  - unscoped constants beside `@cpp(defines:)`: there would be two mechanisms;
  - one shared runtime header: each `out` is self-contained.

## 11. Consequences

- Generated C++ headers stop growing with enum size. A program that uses enum helpers must link the
  enum package's `.gen.cpp`, which every build already did for id enums and decoders.
- C++ `types` reaches parity with Go and TypeScript `types` for maps, record defaults and ref
  unions, and passes them on `[T?]`/`{K: V?}`. Go is now the target that refuses more.
- `E8011` grows a variant. A project with a C++ name that is a standard macro now fails `check`
  instead of failing the C++ compiler.
