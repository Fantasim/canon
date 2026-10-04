# ADR-0014 — C++ baked data, constexpr lookups and text-file ownership

Date: 2026-10-04. Status: accepted (telemetry readiness: units CB, TX; DECISIONS 293-300).

## Context

Telemetry (Source ADR L-0111) needs C++ `baked` output with event metadata usable at compile time
and plain-text outputs (DDL, catalog). CODEGEN specified C++ baked (§5.3, §5.8-§5.10, §7.1-§7.3)
but gen/cpp built only `data` and `types`; there was no text target.

## Decision

- **Baked data is built once, at first use.** `detail::<P>Access` holds a `Data` struct filled by
  `Build()` in two passes (every container's rows first via `KeyedList::FromRows`, then each row),
  reached through a function-local static `Get()`. Pointers into rows survive the return because a
  moved vector keeps its buffer. Values stay out of the header, which carries only declarations.
- **Constexpr only where it pays.** Package-level precomputed and lookup fns with a scalar result
  are `inline constexpr` in the header over a `detail::k<Fn>Cells` array (DECISIONS 293). Each fn
  computes its own ordinals (a cast, or a switch for `@codes` enums whose codes have gaps), so no
  shared `detail::Ordinal` overload is redefined when two packages share a namespace.
- **No check-clean, build-fail input.** Every construct gen/cpp baked cannot write is a stage-E
  finding; the remaining `ErrMalformed` refusals are marked unreachable in a test, and a probe
  harness (`internal/ir/cppbaked_build_test.go`) runs check, build and a syntax compile per case.
- **Text files are owned by a listing, not a marker.** A text output has no room for a marker
  line, so each `emit text` directory carries `.canon-text` (marker + names). Generation lives in
  `build` plus `ir.TextFiles`, with `@text` fns kept in the additive `ir.Package.TextFns` so every
  code generator, conformance and `$fns` drop them in one place (DECISIONS 300).

## Consequences

Large baked tables cost one static construction at first access, not compile-time data. MSVC has
never compiled this output. A `.canon-text` appears in every directory a text emit writes.
