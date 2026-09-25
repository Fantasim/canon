# ADR-0005 — `wire.Decoder`/`wire.Host` grow for dependent types in loaded data

- Date: 2026-09-25
- Status: accepted (orchestrator, DECISIONS 207; landed with the gate lift, d5062a3; not a
  frozen contract of IMPLEMENTATION-PLAN §4)

## Context

DECISIONS 173 fixes `wire.Decoder{Bag, Pkg, Host, Partial, Coll}` and a `wire.Host` of two
methods, `Default` and `Deref`. The gate lift (loads backed by the evaluator, dependent types in
loaded data: TYPES §11, §15; EVALUATION §3.2, §9.3) needs more from both. A loaded value's type
arguments may name the record whose field the load gives, that record's arguments and the
dependent map binders; decoding an applied record binds its parameters; an entry needed to
decode itself is a cycle; a default must wait only for the fields it reads; and a dependent
field decoded in two passes must undo a failed attempt without a trace (steps, bindings).

## Decision

- `wire.Decoder` gains `Outer` (`Record`, `Params`, `Binders`): what the decoded value's type
  arguments may name around it.
- `wire.Host` gains four methods, all implemented by `eval.Evaluator` (through `load`'s host):
  `Bind(rec, params)` keeps the arguments bound to an applied record instance (TYPES §11.1; it
  replaces the recursive `bindLoaded`, DECISIONS 195); `Cycle(ctx, ref)` reports an entry needed
  to decode itself (EVALUATION §3.2, E4301); `Reads(f, fields)` is the fields a default reads
  (TYPES §15), so a default waits only on a waiting field it reads; `Savepoint()` marks an
  attempt whose `end(true)` takes back its steps and bindings (linear, no trace).
- Everything else in DECISIONS 173 holds: `wire` still imports no `syntax` and evaluates
  nothing itself; files, globs and CSV syntax stay in `load`.

## Consequences

A `wire.Host` implementation outside `eval` (tests) must provide the four methods; a host that
does not bind parameters or detect cycles cannot decode dependent types. DECISIONS 220 amends
173 with this list (the orchestrator's spec sync, DECISIONS 207).
