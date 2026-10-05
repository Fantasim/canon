# ADR-0016 — How the 2026-10-05 language and showcase rulings are built

Date: 2026-10-05. Status: accepted (DECISIONS 304-316; branch `feat/past-and-ergonomics`).

## Context

One branch implemented the design review's tier 1 (DECISIONS 304-308) and the Emberfall showcase
findings (309-316). Several choices are durable and not stated by the spec: how `past` is
represented, how a poisoned value is edited, how one collection with two names is compared, and
where the JSON text writer lives. The log is [log-2026-10-05](log-2026-10-05.md).

## Decision

- **`past` is a flag, not a node or a type kind.** Syntax: a `Past` token on the nine type nodes
  that can carry it, read through `syntax.PastOf` (IMPLEMENTATION-PLAN §4.1: the owner adds no
  node). Types: `Refined{Of: X, Past: true}` via `types.Past`, because a `Refined` already never
  changes static identity and every consumer strips it. Added under §4's review rule (§4.2 sketch
  updated). Verification reads it per slot (`verify` walk), never as an inherited scope; inference
  strips it (`check` `inferred`).
- **A poisoned top-level value is edited from source.** `edit` sends Remove, Retire, AddEntry,
  Move, Set/Reset of an entry or its field, and a whole-root Set, to a source-only path
  (`applystatic*.go`) when the root has no value. It refuses with the root's error whatever would
  need a value (a driver field, a spread, an inverse that is not literal-only). Undo is built from
  the text.
- **One collection, two names, compared per instance.** Entries keep the field identity
  `{CollField, owner}`; a let-path ref (`ref z.spawns`) carries no instance; `eval` (`inColl`,
  `letPathMeets`) and `edit` (`target.names`) resolve the let path to its instance where the two
  meet. No change to `internal/value` (§4.3).
- **The JSON text writer is `wire.Text`.** A `@text` result that is not text is written by the
  existing WIRE encoder through `jsonsrc.Format`: one JSON dialect. Stage-E checks on `@text`
  results live in `ir` (`rules_textforms.go`) and skip what stage B verified.
- **Map fields in data loaders** read keys as WIRE §5.8 in file order with identical texts in Go
  and C++; ref keys are checked where ref values are; `ir`'s E8019 rules follow exactly what the
  generators read (`readHolds`, `readableKey`).

## Consequences

`types.Refined` may now carry no written refinement; consumers that list refinements must accept a
past-only layer (audited at the time). The edit API has two paths for the same ops; a fix to one
op's semantics must be applied to both. Generators still refuse a dependent value read through a
ref or an optional in baked mode, and cross-package records in data mode: one generator step
before v0.1 (DECISIONS 312).
