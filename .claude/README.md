# .claude — agent layer

Wires the orchestrator model into this repository (ADR
[0001](../meta/decisions/0001-bootstrap.md)). [DOCTRINE.md](../DOCTRINE.md) and the spec are
authoritative; the agents cite them and restate nothing. Entry point:
[../CLAUDE.md](../CLAUDE.md); present state: [../meta/state.md](../meta/state.md).

## Roster

- **Orchestrator:** the main window. Plans, delegates, reviews, gates on `make check`, commits.
- **Dev:** [`go-dev`](agents/go-dev.md) (`sonnet`): one package or plan step per invocation,
  against a named spec section; returns a green `make check`; never commits.
- **Auditor** (read-only): [`spec-reviewer`](agents/spec-reviewer.md) (`opus`): spec
  conformance first, then diagnostics, contracts, determinism, goldens, the code doctrine.
- **Support:** [`docs-updater`](agents/docs-updater.md) (`sonnet`): `meta/` upkeep, never ADRs.
- **Built-ins:** `Explore` (recon), `Plan` (design, returns inline), `general-purpose`
  (one-off work, ADR drafting at a milestone close).

The judge never fixes what it judges: `spec-reviewer` never edits, `go-dev` never grades its own
diff. A new named agent only for a recurring need, recorded as an ADR; never for a one-off.

## Rules

- [rules/orchestration.md](rules/orchestration.md): the operating loop, delegation template,
  model tiers (auto-loaded).
- [rules/go.md](rules/go.md): the Go checklist behind the code doctrine.

## Settings

[settings.json](settings.json) (committed): read-only Bash and the `go`/`make`/`git` commands
the loop needs are allowed; denied are Edit/Write under `../../<Resource, Source, …>` and every
sibling service by name, Edit/Write of the audit baselines and states, `git push` and
`git remote add|set-url`. `settings.local.json` (git-ignored) repeats the directory denies with
absolute paths for this machine, since the docs do not say whether `..` resolves in a pattern.

## Hooks (written, NOT registered yet)

- `hooks/session-start.py`: orientation (branch, dirty count, recent commits, head of state.md).
- `hooks/stop-uncommitted.py`: blocks ending a turn with uncommitted tracked changes.
- `hooks/guard-scratch.py`: durable documents never land in `tmp/`, `scratch/`, `notes/`.
- `hooks/guard-outside.py`: refuses any Edit/Write outside the project, the temp directory and
  `~/.claude`, whatever the target's name.

To enable them (Louis, once no orchestrating session runs here), add to `settings.json`:

```json
"hooks": {
  "PreToolUse": [
    { "matcher": "Edit|Write|MultiEdit|NotebookEdit", "hooks": [
      { "type": "command", "shell": "bash", "command": "python3 \"${CLAUDE_PROJECT_DIR}/.claude/hooks/guard-outside.py\"" },
      { "type": "command", "shell": "bash", "command": "python3 \"${CLAUDE_PROJECT_DIR}/.claude/hooks/guard-scratch.py\"" } ] }
  ],
  "SessionStart": [ { "hooks": [
    { "type": "command", "shell": "bash", "command": "python3 \"${CLAUDE_PROJECT_DIR}/.claude/hooks/session-start.py\"" } ] } ],
  "Stop": [ { "hooks": [
    { "type": "command", "shell": "bash", "command": "python3 \"${CLAUDE_PROJECT_DIR}/.claude/hooks/stop-uncommitted.py\"" } ] } ]
}
```

The fleet's sovaudit hooks are not used: they live in a sibling repository, and this public
repository depends on none. The local gate is `make check`.
