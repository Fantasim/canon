# State — Canon compiler

Updated: 2026-09-23 (night, autonomous session: agent layer set up, ADR-0001)

## Current focus

**Spec v0.1 is being locked.** The consistency pass of `meta/spec-phase/review/CONSISTENCY-TODO.md` is aligning
SPEC.md, CLI.md, `spec/*`, `api/canon.go` and `examples/` with DECISIONS 17–29 (other agents,
same night). **No compiler code exists yet.**

**NEXT: M0 — Contracts** ([plan.md](plan.md) § M0), starting with the M0 audit work of the
IMPLEMENTATION-PLAN addendum (DECISIONS 26–27), once the consistency pass is closed.

## What exists

- The spec: SPEC.md, CLI.md, DECISIONS.md (1–29), `review/`, `spec/` (companion documents,
  ERRORS.md catalogue, IMPLEMENTATION-PLAN.md), AUDIT.md, AUDIT-2.md, MOCKUP-GAPS.md, `mockups/`.
- `go.mod` (`github.com/fantasim/canonlang`, go 1.25) holding only the API stub `api/canon.go`
  (one 1,070-line file, split by concern in M0).
- `examples/` (own module) with `expected/` findings and the illustrative pipeline goldens;
  `examples/_fixtures/` (resource, client).
- `tools/audit/` (own module, the customised sovaudit) and `.sovaudit/` (baseline, state,
  root-allow); the Makefile's `make check` (goldens-check not wired until M1).
- Agent layer (ADR-0001): CLAUDE.md, DOCTRINE.md, `meta/`, `.claude/` (rules, agents,
  settings with write-denies, hooks **not registered**), `.gitignore`.

## Open Louis-calls

1. **Enable the hooks.** `.claude/hooks/` holds four scripts that are deliberately NOT wired in
   `.claude/settings.json` (an orchestrating session was running in this directory; a
   SessionStart/Stop hook could have interfered). The `hooks` block to paste is in
   [../.claude/README.md](../.claude/README.md) § Hooks.
2. **`git init` has not happened** (DECISIONS 28 says `configlang/` is a local repository).
   Until it does, `baseline-guard` and `decision-dropped` have nothing to compare, the
   stop hook has nothing to check, and the audit walks the tree (skipping dot-directories, so
   `.claude/` is not link-checked yet). First commit: everything, `chore: initial import`.
3. **IMPLEMENTATION-PLAN §12.5 is stale**: it says the repository has no `meta/` and that the
   meta rules have nothing to apply to. `meta/` now exists (ADR-0001); §12.5 needs a sentence
   when the spec is next edited (it was being edited tonight by another agent, so not touched).
4. **Absolute-path denies.** The committed `.claude/settings.json` denies writes outside the
   project with project-relative patterns (`/../../Resource/**`); the permission docs do not
   say whether `..` is resolved, so the git-ignored `.claude/settings.local.json` repeats them
   with this machine's absolute paths (ADR-0001). The `guard-outside` hook, once enabled, is the
   name-independent enforcement. A new sibling service must be added to both lists.
5. **`make check-real`** (DECISIONS 29) is not in the Makefile yet: it lands with the real-data
   job (M3). `testdata-real/` is already git-ignored.

## Verify queue

- After `git init`: `make check` with `.claude/` and `meta/` tracked (dead links, root clutter).
- The hooks, once enabled: a session start prints the orientation block; a Write under
  `../../Resource` is refused by `guard-outside.py`.

## What could not be verified

Whether the relative `..` permission patterns match (not testable without writing outside the
project). Everything else in this setup is Markdown, JSON and Python that `make check` or a
manual run covers.
