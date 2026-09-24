# Orchestration rules — Canon compiler

Auto-loaded. The operating model: who does what, in what order. What the code must do is in
[DOCTRINE.md](../../DOCTRINE.md) and the spec; the entry point is [CLAUDE.md](../../CLAUDE.md).
≤ 80 lines (IMPLEMENTATION-PLAN §12.5).

## Role

The window is the orchestrator, the single fan-out point: subagents cannot spawn subagents, so
when one reports "this needs X next", the orchestrator spawns it. Direct tool use: `Read` to plan
a delegation or review a diff, `Grep`/`Glob` to scope, `Bash` for git and `make check`, `Agent`
to delegate, `Edit`/`Write` for `meta/` and a ≤ 1-file, ≤ ~20-line fixup after review. It never
implements a package by hand. Reload [meta/state.md](../../meta/state.md) at every session start;
update it at the end whenever focus, milestone or roster changed.

## The loop (every task)

1. **Understand.** Restate the ask; read `meta/state.md`, the plan step, the DOCTRINE § touched.
2. **Plan.** One unit = one package (IMPLEMENTATION-PLAN §5.3: an agent edits only its
   packages). Pick agent and model tier. The built-in `Plan` agent returns plans inline.
3. **Delegate.** A self-contained prompt per unit (template below); independent units run in
   parallel, in one message.
4. **Verify.** Read the returned diff. Every non-trivial diff gets `spec-reviewer`. Run
   `GOTOOLCHAIN=local make check` yourself; never accept "it passed" without the log.
5. **Gate.** The Definition of Done (CLAUDE.md): `make check` green, rule-citing tests, goldens
   regenerated and their diff read, `meta/state.md`, ADR if durable.
6. **Integrate.** After a parallel wave, one cleanup delegation removes cross-package helpers
   invented twice (the audit's `dup-in-repo` finds most of them), then a review.
7. **Commit and report.** Granular conventional commits; lead with the outcome; say plainly
   what could not be verified.

## Delegation prompt template

A subagent starts cold. Give it paths, not pasted rules:

```
GOAL            the observable change, one sentence
OWNER / SCOPE   the plan role (SYN, TYP, ...) and the exact packages it may edit
READ FIRST      the spec section it implements (doc + §/rule ids); DOCTRINE §; .claude/rules/go.md
CONTRACTS       the frozen interfaces it consumes (IMPLEMENTATION-PLAN §4) — read, never edit
TESTS           rule ids each test must cite; goldens to regenerate; txtar cases per new code
VERIFY          GOTOOLCHAIN=local make check (and go test -race ./<pkg>/...), log attached
REPORT BACK     files touched; spec gaps found; any choice not in the prompt; what is unverified
```

## Model selection

Tiers `haiku` < `sonnet` < `opus` < `fable`; every named agent pins one (`inherit` is none).
Use the cheapest tier the gates (goldens, rule tests, `-race`, fuzz, audit, review) fully check.
`haiku`: read-only recon, never code. `sonnet`: `docs-updater`, and `go-dev` on golden-checked or
mechanical work: backends from the IR (`gen/*`), `cli`, `project`, `load` I/O, `lsp` plumbing,
`views`/`i18n` wiring, cleanups, tests. `opus`: `spec-reviewer`, and `go-dev` on semantics tests
can miss: `check`/`types`, `eval`, dependent types, frozen contracts, determinism, the memo, `edit`
minimal writes, generated-C++ performance. Sonnet failing review twice moves to opus. `fable`:
the orchestrator only. State the tier and why in each delegation.

## Auditor discipline

`spec-reviewer` is read-only and reports everything it finds, with severity and confidence.
`go-dev` fixes; the orchestrator re-runs the reviewer. The judge never fixes what it judges. On
a FAIL, resume the SAME builder (`SendMessage`) with the numbered findings, then the SAME
reviewer to verify the fixes only.

## Spec gaps and scope

A contradiction or missing rule stops the unit; the orchestrator decides it (technical calls never
go to Louis) and logs both passages and the choice in `meta/decisions/log-<date>.md`; a lasting
rule becomes a DECISIONS item, the spec follows (207). Builders never edit the spec or DECISIONS.

## Worktrees

An `isolation: "worktree"` delegation first fast-forwards to the main branch and reports its
base SHA; the orchestrator commits inside the worktree and cherry-picks, never merges, and runs
`make check` before every `cherry-pick --continue`.

## Session end

Caps: CLAUDE.md, DOCTRINE.md, `meta/state.md` and every `.claude/` rule or agent ≤ 80 lines;
over a cap, move detail down (CLAUDE.md → DOCTRINE.md → `meta/`). Nothing left uncommitted
without saying why; never commit `testdata-real/`, `.canon/` or build outputs.
