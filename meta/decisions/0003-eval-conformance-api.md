# ADR-0003 — The evaluator's conformance API (test calls, vectors, TS mode)

- Date: 2026-09-24
- Status: accepted (orchestrator, DECISIONS 207; reviewed under IMPLEMENTATION-PLAN §4's rule)

## Context

`conform` (CONFORMANCE.md §6) needs two things the frozen evaluator contract (IMPLEMENTATION-PLAN
§4.8) does not offer: the calls a package's tests make to its translated fns, and one vector
evaluated alone, natively or in TS mode, on its own cap, reporting nothing (DECISIONS 204).

## Decision

`internal/eval` gains, additively (nothing existing is renamed or changes meaning):

- `Call{Fn, Recv, Args}`; `(*Evaluator).TestCalls(ctx, pkg, fns, Builder) ([]Call, error)` on a
  fresh evaluator (`ErrNotFresh`, `ErrNoPackage`): the package's tests in declaration order on one
  shared budget, values forced afresh, calls kept up to any stop, no finding reported; the host
  must report into throwaway bags (the build adapter's duty).
- `VectorMode{Steps, TS}` (0 steps means the 1,000,000 vector cap of §6.5), `Outcome{Value, Code,
  Exceeded}`, `Limit` (`NoLimit`, `StepLimit`, `DepthLimit`); `(*Evaluator).Vector(ctx, Call,
  VectorMode) Outcome` on a child of the stage-A evaluator: its own cap and depth, the parent's
  settled values read, a value first forced inside charged to the vector and forgotten, the first
  error in report order returned. Calls are sequential; the parent is not used concurrently.
- Two optional host capabilities, type-asserted beside `eval.Host` (the §3 seam is unchanged): an
  unexported-interface `LoadInto` and `VerifyInto`, each reporting into bags the vector passes and
  then discards. A value first forced in a vector is verified this way (EVALUATION §1); its first
  verification error is the vector's code.
- `Invalid` consults the parent evaluator (only a vector's child has one).

## Consequences

`internal/build` implements `conform.Evaluator` with a thin adapter over `TestCalls` and `Vector`,
and gives `evalHost` `LoadInto`/`VerifyInto`. Once ir fills `ExportFn.Reads`, conform passes them
to `Vector` and eval's own `readsOf` goes (decisions log, "eval: test calls and vectors").
