# Sovereign -> Canon: C++ types mode should keep the refined bounds (and a look at generated size)

**From:** Sovereign Source (Louis, ADR L-0147 "Campaign-wide scope" item 4, 2026-10-30). **Canon version:** 0.1.13.

**Priority: NOT SCHEDULED.** Written so the need is on record; Louis has not asked Canon to build it yet ("it's not
urgent for now ... I won't implement that feature on Canon yet"). Do not start it until Louis schedules it.

## Why

Sovereign's game servers and client load about 125 Canon-rendered files through the C++ `mode: types` decoders
(ADR L-0138). The Canon gate proves every rendered file before it ships, but the C++ cannot rely on what a
refined type promises, because the decoder drops the bounds. So every loader re-checks them by hand:

- ~310 narrowing guards in ~28 files, over 25+ private copies of the same helper (`Narrow`, `Fit`, `Within`,
  `TakeRange`, `NarrowFloat`...).
- ~25 index-bound checks, ~30 list-capacity checks and ~4 divisor checks.

Of the fields narrowed by hand, ~88 % are fully covered by a refined Canon type, ~8 % in width only (open upper
bound), and ~4 % not at all (plain `Int`; Sovereign fixes those in its own data). Louis wants these duplicate
guards gone; they can go only once the decoder carries the guarantee.

## What Sovereign sees today

`Int(1..=MAX_MOB_KILLS_PER_POINT)` (`Resource/Config/progression/activitypoints/activityPoints.canon`) is emitted as
`int64_t mobKillsPerPoint_` and decoded with `dec.Int(v, "mobKillsPerPoint", ...)`, which is
`AsIntIn(v, key, kIntMin, kIntMax, out)` (`canon_runtime_json.h`, ~line 1025). Refined ints, refined floats and
bounded lists all arrive unchecked:

- **Member types across the generated headers:** 830 `int64_t`, 54 `int32_t`, 51 `uint32_t`, 10 `int16_t`,
  9 `uint64_t`, 4 `uint16_t`, 1 `uint8_t`, 256 `double`, 7 `float`.
- **Already right:** explicit sized types (`UInt32?`) are emitted at width and checked with `AsIntIn(0, 4294967295)`.
- **Lists:** a length bound `[T](1..=8)` is neither enforced nor exported.
- **Constants:** only a user `const X = n` becomes `inline constexpr int64_t`.

## The ask

1. **Checked decode from the refined bounds.** A refined `Int(a..=b)` decodes with `AsIntIn(a, b)` (an
   out-of-range value fails `Parse`, like a wrong type does). The same goes for refined `Float(a..=b)`.
2. **Sized members.** When the range fits a C++ width, emit that member type (`int32_t`, `uint8_t`, ...) instead of
   `int64_t`. Likewise emit `float` when a float range fits a float, and refuse a value past `FLT_MAX`.
3. **List length.** Enforce `[T](a..=b)` in the array decode loop.
4. **Exported bounds.** Emit `inline constexpr` min/max for every refined field and list length (for example
   `k<Record><Field>Min/Max`, `k<Record><Field>LenMin/LenMax`), so a C++ array sized by a Canon bound can
   `static_assert` against it.
5. **Acceptance:** a Sovereign file with one value outside its refined range, or a list outside its length bound,
   fails the generated `Parse` with a key path. The emitted members and constants are as above. Shipped data
   decodes unchanged (Sovereign proves that with its parity tests). Name the rollout if it changes member types
   that existing C++ reads, because Sovereign then updates its loaders in the same pin bump.

## Second, lower priority: generated size

The Source copy of the C++ output is 32 MB / ~819 K lines over 322 files. Most of it is vocab and id tables:

- `text/vocab.gen.cpp` 4.3 MB + `.h` 3.1 MB
- `items/vocab.gen.cpp` 2.5 MB + `.h` 1.8 MB
- `header/ids/ids.gen.cpp` 2.0 MB
- `ai/vocab` 2.2 MB
- `character/vocab` 0.9 MB
- telemetry 1.8 MB

`canon_runtime_json.h` (~57 KB) is written 28 times, once per output directory (1.6 MB in all). Sovereign does
not need a change now; it asks for Canon's view on:

- (a) one shared runtime header per root;
- (b) whether a vocab table's name/wire maps could be emitted more compactly (for example one sorted table
  instead of switch chains, if that is what dominates);
- (c) compile-time numbers on the largest TUs.

Louis's ruling: "optimisation about code generation ... it's canon work."

## Paths

- Source: `Engine/Resource/Generated/**` (the output);
  `meta/decisions/L-0147-prop-structures-over-canon-records.md` ("Campaign-wide scope added", item 4).
- Resource: `Config/**` (the packages).
