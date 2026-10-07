# ADR-0018 — Frame-owned collections: in-place element assignment without aliasing

Date: 2026-10-07. Status: accepted (EVALUATION.md §4.1, §12; DECISIONS 199; handoff 2026-10-07
A4, C(c)).

## Context

The Sovereign Resource port found two evaluator traps. `m[k] = v` on a `var` map in a loop copied
the whole map on every assignment (quadratic: 6.3 s for 8,000 rows against 0.7 s for
`isUnique()`), and the same held for `xs[i] = v` and `t[k] = rec`. Reading a field through a `ref`
listed the target table's entries on every dereference (about 4x a copy). Both cost real time and
neither is charged in steps, so an author had no way to see them.

EVALUATION.md §4.1 gives values copy semantics ("the effect is that of rebuilding the root") and
IMPLEMENTATION-PLAN §4.3 makes values immutable once they leave the call that built them, with
copy-on-write buffers internal to `eval`. Neither forbids updating a collection no one else can see.

## Decision

- **A frame owns its var's collection** (`internal/eval/owned.go`). An element assignment either
  copies the collection (as before) or, when the frame already owns it, updates its top level in
  place; either way the frame then owns the result (by var and pointer identity). Nested
  collections below the root are always copied.
- **Every read of the var clears ownership, except a peek**: the receiver of an index read `v[k]`
  and of a built-in in `std.Peeks` (`len`, `isEmpty`, `get`, `contains`, `hasKey`), which return a
  scalar, an element or a fresh copy and keep nothing of the receiver. A receiver the checker
  converts is never a peek; lambda captures, call arguments, returns, operands, `for`, `match` and
  literals holding the var all clear it. The peek mark is consumed by the read it marks.
- **Caches follow ownership.** An in-place update drops the per-collection key cache
  (`Evaluator.updated`); every other per-pointer cache is reached only through a read that clears
  ownership first. `value.Map.Set` (new, exported) appends or replaces under the map's lock and
  resets the cached entry sum; its precondition is "the map's sole holder only", and only
  `eval`'s owned path calls it. DECISIONS 199's "computed once per map" now reads "recomputed
  after a `Set`" — unreachable from the language, since hashing needs an escaping read.
- **A `ref` dereference uses the cached key index** and the collection's length, never a list of
  its entries.
- **Steps are unchanged.** The copy path never charged; peeks charge as before. Verified by
  bisecting the minimal passing `project.budget` on old and new binaries over loop, ref and mixed
  probes, and by identical `check`/`explain`/`test` output over aliasing probes (review
  2026-10-07, PASS).

## Consequences

- The loop at 8,000 rows costs 0.6 s (within the `isUnique` time); a ref read costs what a copy
  read costs. `TestCostScale` fails the old code at ~15x and holds the new at ≤8x by allocation.
- The invariant to keep: no value reachable from an owned collection may escape without clearing
  ownership, and a peek may never hand out the container or one of its slices. A new built-in
  joins `std.Peeks` only if it meets that rule (the table is keyed by receiver family).
- `value.Map.Set` widens a frozen §4.3 type additively; reviewed under §4's rule (opus builder,
  spec-reviewer, this ADR).
