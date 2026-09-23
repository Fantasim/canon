# ADR-0001 — The agent layer: CLAUDE.md, DOCTRINE.md, meta/, .claude/

- Date: 2026-09-23
- Status: accepted (autonomous night session; Louis reviews)

## Context

The compiler will be written by agents, milestone by milestone (IMPLEMENTATION-PLAN §5–§6).
Louis's other services (the team board, sovcommon) run on one pattern: an orchestrator
session that delegates, reviews and gates; a short CLAUDE.md; a DOCTRINE cited by §; `meta/`
as external memory; `.claude/` with rules, named agents and hooks. This repository had the
spec and the gate (`make check`, `tools/audit`) but no agent layer. It is also a future public
repository, so nothing in it may depend on a sibling service or name a machine path.

## Decision

1. **Adopt the pattern, adapted.** CLAUDE.md and DOCTRINE.md at the root (≤ 80 lines each,
   IMPLEMENTATION-PLAN §12.5), `meta/` (state, plan, decisions, handoff), `.claude/` (two rules,
   three agents: `go-dev`, `spec-reviewer`, `docs-updater`).
2. **The spec is the law, not DOCTRINE.** DOCTRINE points at DECISIONS.md, ACCEPTED-CHOICES
   and the companion documents in that order and restates none of them.
3. **Two decision logs.** DECISIONS.md stays the spec-phase log; implementation ADRs go here.
4. **Left out from the models:** anything web, SQL, auth, taxonomy or deploy; the sovcommon-first
   law (here: standard library first, DECISIONS 25); the fleet's sovaudit hooks (they live in a
   sibling repository; the local gate is `make check`); worktree cherry-pick recipes beyond one
   paragraph (no git history yet).
5. **Writes outside the project are denied three ways:** project-relative deny patterns in the
   committed `.claude/settings.json` (`/../../Resource/**`, every sibling by name), absolute
   ones in the git-ignored `.claude/settings.local.json` (the permission docs do not say
   whether `..` resolves, and a public file must not name a machine path), and the
   `guard-outside.py` hook, which refuses any Edit/Write under the parent tree but outside the
   project whatever its name. The audit baseline and state are denied to Edit/Write too.
6. **Hooks are written but not registered** tonight: an orchestrating session was running in
   this directory and a SessionStart or Stop hook could have interfered. Louis enables them
   (the block to paste is in `.claude/README.md`).
7. **No `git init`** by this setup: DECISIONS 28 decides the repository; creating it while other
   agents edit the tree is Louis's or the orchestrator's step.

## Consequences

- IMPLEMENTATION-PLAN §12.5 ("the repository has no `meta/` directory") is stale and needs one
  sentence at the next spec edit (a Louis-call in `meta/state.md`).
- `DOCTRINE.md` and `meta` are added to `.sovaudit/root-allow.txt`; `CLAUDE.md`, `.claude` and
  `.gitignore` are already on the audit's standard root list.
- Until the hooks are enabled and the repository initialised, the Stop-hook discipline and the
  baseline guard rely on the orchestrator.
