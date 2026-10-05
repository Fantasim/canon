# Prompt — compiler and DX issues found by the Emberfall showcase (2026-10-05)

Paste into a Canon orchestrator session. Self-contained.

---

You are the Canon orchestrator (CLAUDE.md, .claude/rules/orchestration.md). Start with the
session read order. Another session may be working on an ADR in meta/decisions/ and
meta/handoff/2026-10-05-design-review.md: check `git status`, and do not touch or commit its files.

**Context.** A demo project, /home/louis/dev/canon-showcase ("Emberfall"), was built on Canon on
2026-10-05:
- `config/` is a game config that exercises most language features.
- `editor/` is a generic web editor on `api/` and the view models.

Read-only for you: everything outside this repository.

**Caution: not all of these are bugs.** A re-check against the spec (2026-10-05) found several
that are deliberate design. Do not "fix" design: for each item, first decide whether it is a bug,
a spec gap, or intended. The re-check's verdict is given next to each item below.

Its findings, each with a minimal repro and the workaround used, are in
/home/louis/dev/canon-showcase/COMPILER-ISSUES.md. Read the whole file first. Every repro there
must become a failing test in this repo before any fix (CLAUDE.md core task 2).

## Wave 1: bugs, one go-dev per owning package, parallel where packages differ

1. **#4: an incomplete new entry poisons its table (highest). Verdict: spec gap plus one bug.**
   - EVALUATION.md §7.1/§7.2 make a whole top-level value the root, and a table is one value,
     so poisoning the whole table follows the letter of the spec.
   - That contradicts VIEWMODEL D3/T3's add-then-complete flow, so this is a ruling: entry-level
     isolation, or a different studio flow.
   - The empty Undo in a stable table contradicts API.md E23 outright: that part is a bug.
   - Symptom: after `addEntry` with `allowErrors` misses a required field, `Value` of every entry
     in the table fails with `ErrNoValue`, and `Remove` of the new entry fails too.
   - In a `stable table`, the edit's `Undo` is empty, although API.md E23 gives a Remove/Retire
     for an AddEntry.
   - The spec says a studio adds, then the findings guide completion: VIEWMODEL.md D3/T3,
     API.md §8.6/§8.7.
   - Expected: the other entries stay readable, the new entry is readable with its findings, and
     both Remove and Undo work.
   - Owner: `eval` and/or `edit`/`api`. Tier: opus (semantics and undo).
2. **#3: `Rename` refuses an entry used as a keyed-list key** (`[S] keyed by m`, `m: ref ms`).
   - **Verdict: follows the letter of the spec; this is a spec gap, not a bug.** API.md §7.2 gives
     a keyed-list key field the reason `key`, and E12 refuses a rename when a reference is not
     editable.
   - Rewriting that key would re-key `z.spawns[wolf]`, which cascades to every ref and path into
     `z.spawns`. That may be a deliberate safety line. Rule on it before implementing a cascade.
   - Symptom: `ErrNotEditable` with reason `key` at `z.spawns[wolf].m`.
   - API.md E11/E12 and §8.4: the key reference is what the rename rewrites. The element keeps
     its place and changes its key.
   - Check what happens to the keyed-list path segment and to the lock.
   - Owner: `edit`. Tier: opus.
3. **#1: `ref in keyedList` is false although the key exists. Verdict: a real bug.**
   - STDLIB.md §5 (Membership): the static type of the operand decides element or key.
   - `.get(r)` works and `String` keys work.
   - Owner: `check` (static choice) and/or `eval`. Tier: opus.
4. **#2: generated TS conformance test disagrees with the generated TS function.**
   - The function checks each parameter in order: E8303 representability, then E3204 range.
   - The vectors expect representability of every input to be checked first (evaluator TS mode).
   - CONFORMANCE.md §2.3 and §4 read both ways. This is a spec gap.
   - The orchestrator rules on it (log-2026-10-05, a DECISIONS item, the spec follows, 207).
   - Then one go-dev on `gen/ts` or `conform`. Tier: sonnet (golden-checked).

Each bug fix must have:
- a commit body in the form symptom → cause → fix;
- a test citing the rule;
- spec-reviewer on the diff (the weekly limit resets on 2026-10-06; if it is still hit, say so
  in state.md as before).

## Wave 2: rulings, then implementation

Re-check verdict:
- E8019 is by definition "a generator cannot produce a construct valid Canon allows": an
  acknowledged gap (DECISIONS 291 calls lifting it a later item), not design.
- E8013 and E8015 are mode design (data files hold data, not functions; data mode loads tables,
  keyed lists and records). Keep both unless there is a strong case.

The generators force the data model to change shape; this cuts against "elegance over legacy".
The showcase hit four restrictions:

| Code | What failed | Workaround used |
|---|---|---|
| E8019 MapField | Go/C++ data mode cannot generate a map field | `[Resistance] keyed by element` instead of `{Element: Int}` |
| E8019 ForeignDataRecord / CrossPackageBakedValue, E8018 | Go/TS cannot generate a record of another package | record moved into its only user |
| E8015 | data mode cannot emit a plain `[ref T]` value | value wrapped in a record |
| E8013 | `text` cannot live in a package with `data` emits | text moved to its own package |

The ruling:
- Decide which of these the language should lift in v0.1. Cross-package records are already on
  the later list ("cross-package decoders §2.2 vs §2.8").
- Log the ruling.
- Plan the implementation units (`gen/go`, `gen/cpp`, `gen/ts`; tier sonnet unless frozen
  contracts are touched).

Do not start a frozen-contract change without §4's review rule.

## Wave 3: small API and DX items

The editor worked around these. Decide each one, then implement or log a refusal.

**API**
- Project facts: name and doc comment. Today the editor parses `project.canon` text. API.md §5.5.
- A per-call language for `Check`, as `EvalRequest.Lang` has (§11). The editor opens one
  `Project` per (layer, language).
- `ViewModel` is recomputed on every call (about 20 ms per package). Decide whether to share or
  cache it per revision (S8 already shares identical concurrent calls).
- `Watch` reports writes made through another `Project` on the same directory as `external`.
  This is by design: each `Project` is its own client (API.md W rules). Nothing to do.

**DX**
- `canon version` prints `0.1.0 (unknown)` under `go install`. Use `debug.ReadBuildInfo` for the
  VCS revision.
- Not issues, by design; nothing to do:
  - `.js` import specifiers in generated TS are the standard ESM/NodeNext convention.
  - Generated Go has no `go.mod` because it lands in the user's module.
  - A view's `preview` asset must exist, which is the checked-reference guarantee.

## Report

Each of the following goes in meta/state.md and the day's decision log:
- what landed (with SHAs);
- the rulings;
- what was refused, and why;
- what could not be verified.

When a fix changes the showcase's workarounds:
- do not edit the showcase;
- write a short meta/handoff note listing which workaround can now be removed.

Louis pushes `main` himself.
