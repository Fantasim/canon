# M4 start prompt (local session)

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
Start M4 — "Formatter and the edit API". You are the orchestrator (CLAUDE.md,
.claude/rules/orchestration.md). Run it locally in this session, end to end, until all six
acceptance items of IMPLEMENTATION-PLAN §6 M4 are proved by tests, or you hit a philosophy
question for Louis (write it to meta/handoff/, keep going on what it does not block).

Read first: CLAUDE.md, meta/state.md, meta/plan.md "M4", IMPLEMENTATION-PLAN §6 M4, §7.4, §7.6,
§7.7, the M3 handoff (meta/handoff/2026-09-29-m3-complete.md "What M4 starts with"). Then read
spec/API.md and spec/FORMATTER.md one section at a time as each unit needs them.

Calls already made (Louis, 2026-09-29) — do not re-ask:
- M4 runs locally, not in cloud sessions.
- DECISIONS 229 (multi-destination emits) is implemented AFTER M4 is accepted.
- NFR-01 reference machine = this local machine; the perf gate is an opt-in make target
  (not part of `make check`), its numbers recorded in meta/.
- M1.5's second wave stays parked. Fuzzing runs only the durations M4 acceptance states
  (10 min formatter fuzz; minimal-write fuzz) — no multi-hour campaigns.
- Technical calls are yours: decide, log in meta/decisions/log-<date>.md, never ask Louis.
- Windows/macOS CI already ran green at the end of M3 (run 36563862341); fix the stale
  "neither has run" line in meta/state.md at the first state update.

Housekeeping before unit 1: the 47 leftover M3 worktrees under .claude/worktrees/ hold only
wip snapshots already landed on main (verified 2026-09-29) — remove them and their local
branches (`git worktree remove --force`, `git branch -D`), then report.

Starting point: `internal/format` is substantial (printer, fuzz, idempotence tests) —
audit it against FORMATTER.md before assuming work; `internal/workspace` is a doc stub;
`internal/edit` has Snapshot/Resolve/editability (M3 U4b).

Suggested units (scope them yourself, one package per go-dev, spec-reviewer on each):
 U1 format: gap audit vs FORMATTER §1–§12, §15; JSON source printer §14; examples fixed points.
 U2 cli: `canon fmt` (+ --check, --json-sources) — CLI.md.
 U3 workspace: per-file parse/type-check cache, per-entry eval memo (§7.6 NFR-02).
 U4 edit: Set/Add/Remove, minimal re-printing (FORMATTER §13, API M6), atomic writes, crash
    safety (acceptance 5).
 U5 api: the rest of API.md — revisions, concurrency, overlays, Evaluate, Refs, Watch.
 U6 cli: explain (input fields, deferred from M3), refs, --watch.
 U7 gates: API.md rule-coverage test (§7.4), minimal-write fuzz, 60 s race stress
    (8 readers/1 editor/1 watcher), NFR-01 bench on benchgen's 7,000-entry project.
Tiers: opus for workspace, edit, api, memo, stress; sonnet for cli and gate plumbing.

Every go test runs under `systemd-run --user --scope -p MemoryMax=3G`; temp dirs in /var/tmp.
Commit granularly after green `GOTOOLCHAIN=local make check`. At the end: meta/state.md,
plan.md ticks, a handoff meta/handoff/<date>-m4-complete.md, and give Louis the push command
(you cannot push).
```
