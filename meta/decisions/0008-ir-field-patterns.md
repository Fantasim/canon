# ADR-0008 — `ir.Field.Patterns`: every pattern of an input's alias chain

- Date: 2026-09-28
- Status: accepted (orchestrator, under IMPLEMENTATION-PLAN §4's review rule: `ir.Field` is a
  frozen contract, §4.5; `spec-reviewer` reviewed the diff as the consumers' proxy — gen/go and
  gen/cpp, the only readers)

## Context

TYPES.md §7.4: a refinement on a named type adds to the named type's own ("both are checked"), and
EVALUATION.md §11.3 step 3 checks an input's own patterns at run time. `ir.Field.Pattern` held one
pattern, the outermost of the chain, so a generated input loader accepted a value verify refuses
(`type Code = String(/^[a-z]+$/)`, `type Short = Code(/^.{1,4}$/)`: `ABC` passed the loader).

## Decision

- `Pattern *regexp.Regexp` becomes `Patterns []*regexp.Regexp`: every distinct pattern of the
  chain (a repeated pattern is kept once), innermost first in alias-chain order (not declaration
  order: an outer alias may be declared above its inner one), as eval checks at storage points. The order is not
  observable in a loader (every pattern failure reads the same line); range and length checks
  run before the patterns.
- The field is replaced, not doubled: a kept `Pattern` would duplicate one element or silently
  bring the bug back for a consumer reading only it.
- Loaders check the patterns in order; the first failure refuses the value with the one line
  naming the variable (`<ENV>: does not match its pattern`), in Go and C++ alike.
- Names: C++ `kPattern`, `kPattern2`, …; Go `input_<T>_<store>_Pattern`, `…_Pattern2`, … — a
  single pattern keeps its name, so no existing golden changes. `ir.PatternNames` makes both.

## Consequences

CODEGEN §5.12 and §7.7 are synced; DECISIONS 225. Any future consumer of `ir.Field`
must walk the list. Stacked patterns in the view model stay the outermost (VIEWMODEL §12.3 has
one `pattern`; the studio's check is advisory, log 2026-09-28 "views V2 (U10 report, calls)").
