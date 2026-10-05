# Prompt — issues found by the Emberfall showcase (2026-10-05)

Paste everything below the line into a Canon orchestrator session.

---

You are the Canon orchestrator. Follow CLAUDE.md and .claude/rules/orchestration.md, and start
with the session read order. Another session may be working on an ADR (meta/decisions/,
meta/handoff/2026-10-05-design-review.md). Check `git status`, and never touch or commit its files.

**Context.** /home/louis/dev/canon-showcase ("Emberfall") is a demo built on Canon. It has two
parts:
- `config/`, a game config;
- `editor/`, a generic web editor on `api/` and the view models.

It is read-only for you. Its findings, each with a minimal repro and the workaround used, are in
/home/louis/dev/canon-showcase/COMPILER-ISSUES.md. Read that file whole first.

**Classify before you fix.** A re-check against the spec showed that several findings are
intended design or spec gaps, not bugs. For every item, confirm the verdict below by reading the
cited spec. Then:
- A **bug**: write a failing test first (CLAUDE.md core task 2), then fix it.
- A **spec gap**: you rule on it, log it in meta/decisions/log-2026-10-05.md, and add a DECISIONS
  item; the spec follows (207). Implement only after the ruling.
- **Intended**: write one line in the log saying why, and nothing else.
- If you disagree with a verdict, say so in the log with the passages.

## Wave 1: bugs (one go-dev per owning package; parallel where the packages differ)

1. **`ref in keyedList` returns false when the key exists (COMPILER-ISSUES #1). Verdict: bug.**
   - STDLIB.md §5 (Membership): the operand's static type decides element or key. `ref ms` is the
     key type of `[S] keyed by m` with `m: ref ms`, so the result must be true.
   - `.get(r)` already works, and `String` keys work.
   - Owner: `check` (the static choice) and/or `eval`. Tier: opus.
2. **The Undo of `AddEntry` into a stable table is empty (part of #4). Verdict: bug.**
   - API.md E23 says this inverse is a `Retire` of the new key (DECISIONS 277).
   - Repro: `addEntry` with `allowErrors` and a missing required field, into a `stable table`.
     The result has `undo.ops: []`.
   - Also check whether the empty Undo happens only when the table is poisoned (item 3 below) or
     always.
   - Owner: `edit`/`api`. Tier: opus.

Each fix must have:
- a commit body in the form symptom → cause → fix;
- a test citing the rule;
- spec-reviewer on the diff (its limit resets 2026-10-06; if it is still hit, say so in state.md).

## Wave 2: spec gaps to rule on

3. **An incomplete entry poisons its whole table (#4, main part).**
   - EVALUATION.md §7.1/§7.2: a top-level value is the root, and a table is one value. A
     hard-failed entry therefore poisons the table: every `Value` in it gives `ErrNoValue`, and
     `Remove` of the bad entry fails.
   - That is the letter of the spec. It contradicts VIEWMODEL.md D3/T3 and API.md E19: add with
     `AllowErrors`, then "the missing-field findings guide the user". That flow needs the table
     readable.
   - Rule between two options:
     - (a) entry-level isolation: a bad entry is poisoned or invalid, and its siblings stay
       readable;
     - (b) D3/T3 change: the studio must send complete values. The showcase editor already
       does this as its workaround.
   - Also find which code a missing required field (E3302) raises at evaluation. Is it soft
     (§7.1 table) or hard, and is that intended?
4. **`Rename` is refused for an entry used as a keyed-list key (#3).**
   - Today API.md §7.2 gives a keyed-list key field the reason `key`, and E12 refuses a rename
     when any reference is not editable. So the refusal follows the letter of the spec.
   - Rewriting that ref re-keys `z.spawns[wolf]`, which cascades to every ref and path into
     `z.spawns`. That may be a deliberate safety line.
   - Rule: either keep the refusal (document it under E12 and tell the user why), or define the
     cascade (a nested rename of the keyed-list element), including the lock and the paths.
   - Only then, if needed: go-dev on `edit`, tier opus.
5. **TS conformance order (#2).**
   - The generated TS function checks each parameter in order: E8303 representability, then
     E3204 range.
   - The generated vectors expect representability of every input to be checked first. So
     `mitigated`'s TS conformance test fails on `(-1, -9223372036854776000)`.
   - CONFORMANCE.md §2.3 and §4 read both ways.
   - Rule, then one go-dev on `gen/ts` or `conform`. Tier: sonnet (golden-checked).
6. **Generator gaps (E8019), against "elegance over legacy".** These forced the showcase to
   reshape its data model:
   - `MapField`: Go/C++ `data` mode cannot generate `{Element: Int}`, so the showcase used
     `[Resistance] keyed by element`.
   - `ForeignDataRecord` / `CrossPackageBakedValue` and E8018: a record of another package inside
     a value. The showcase moved the record into its only user.

   E8019 is by definition "a generator cannot produce a construct valid Canon allows": an
   acknowledged gap (DECISIONS 291 calls lifting it a later item, together with "cross-package
   decoders §2.2 vs §2.8").
   - Rule on which gaps v0.1 lifts.
   - Plan the units (`gen/go`, `gen/cpp`, `gen/ts`; tier sonnet unless a frozen contract is
     touched, which needs IMPLEMENTATION-PLAN §4's review rule).

## Intended: log one line each, do nothing else

- E8013: data files cannot hold package functions, so `@text` needs its own package or another mode.
- E8015: `data` mode emits tables, keyed lists and records, not a bare `[ref T]`.
- `Watch` reports writes made through another `Project` as `external`, because each `Project` is
  its own client.
- Generated TS uses `.js` import specifiers, which is the standard ESM/NodeNext convention.
- Generated Go has no `go.mod`, because it lands in the user's module.
- A view's `preview` asset must exist (checked references).

## Wave 3: small items (decide; implement or log a refusal)

- API.md §5.5: expose the project's name and doc comment. Today the editor parses
  `project.canon` text.
- A per-call language for `Check`, as `EvalRequest.Lang` has (§11). It may be deliberate scope;
  decide. Without it, the editor opens one `Project` per (layer, language).
- `ViewModel` costs about 20 ms per package per call. S8 shares only identical concurrent calls.
  Decide whether to keep the result per revision.
- `canon version` prints `0.1.0 (unknown)` under `go install`. Read the VCS revision from
  `debug.ReadBuildInfo`.

## Report

Put each of these in meta/state.md and the day's log:
- what landed (with SHAs);
- each ruling;
- each item logged as intended;
- what could not be verified.

When a fix makes a showcase workaround unnecessary, do not edit the showcase. Instead, add a short
meta/handoff note listing what can now be removed. Louis pushes `main` himself.
