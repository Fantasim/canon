# ADR-0007 — `api.Origin` gains the amendment chain, its text and cut frames; `ErrInputField`

- Date: 2026-09-28
- Status: accepted (orchestrator, under IMPLEMENTATION-PLAN §4's review rule: `api` is a frozen
  contract; `spec-reviewer` reviewed the diff as the consumers' proxy; lands with U14,
  `api.Project.Value` and `canon explain`)

## Context

`canon explain` (CLI.md §3.7, M3 acceptance 4) prints each origin of a value "from the one that set
the final value back to the one it replaced", with a detail per origin: a `default`'s canonical
text, a `computed` value's frames and API F13's "(n more frames)" line, and EVALUATION §11.2's
`input from env NAME` for an input field. API.md §5.2's `Origin` had only `Via` (what omitted a
field, or what a spread copied); nothing could carry what an amendment replaced, the text of the
value an origin produced, or how many frames were cut; and an input field had no error of its own.

## Decision

Additive only; nothing existing changes meaning.

- `Origin.Text string`: the canonical text (STDLIB STD-06) of the value this origin produced,
  before any later amendment of it or of its descendants (eval records the pre-amendment copies);
  empty when not known (a `default`'s `Via`).
- `Origin.MoreFrames int`: the number of frames cut from `Stack` (F13), 0 when none.
- `Origin.Replaced *Origin`: the origin of the value an amendment replaced with this one
  (EVALUATION §9.3), and so on back to the base value, newest first; nil when no amendment set it.
  `Via` keeps its meaning.
- `ErrInputField`: a path ending at an `input` field is a `*PathError` wrapping it, `Detail` the
  environment variable (R5); `canon explain` prints `input from env NAME` and exits 0.
- Field order: `Kind, Span, Pointer, Layer, Via, Stack, Text, MoreFrames, Replaced`, so unkeyed
  literals of the old fields still compile.
- Supporting internals (not contracts): `eval.Evaluator.Settled` (a read never evaluates),
  `Produced` (pre-amendment values), the cause log R6 reads (evaluation, verification and, for a
  root the checker broke, check's findings), `edit.PathError.Root`.

## Consequences

API.md §5.2 (Origin, R5, R6, `JSON()` compact) and §15 (`ErrInputField`) are synced; DECISIONS gains
the item. A consumer that switches over `Origin` fields keeps working; one that builds `Origin`
literals positionally must add the new fields. `Value` still re-analyses every package per call
(P6 selection-independence); caching per revision is M4's workspace work.
