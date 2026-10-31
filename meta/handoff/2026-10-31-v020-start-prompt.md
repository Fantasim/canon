# v0.2 session start prompt

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
You are the orchestrator (CLAUDE.md, .claude/rules/orchestration.md). Build and release canon
v0.2.0 exactly as meta/decisions/0023-v02-trust-release.md decides it. main is at or after the
commit that holds that ADR. Go to the end: the tag, the reply handoff, plan and state updated.
Stop only for a philosophy question for Louis: write it to meta/handoff/ and keep going on
everything it does not block.

READ FIRST, in this order, before any action
1. CLAUDE.md, meta/state.md, meta/plan.md ("v0.2: trust").
2. meta/decisions/0023-v02-trust-release.md, WHOLE. Its §0 is your rulebook: read it twice.
   The ADR is decision-complete: build what it says, never redesign. Every Decision binds.
   Every Spec text and ERRORS.md block is applied verbatim.
3. DOCTRINE.md §2-§5, .claude/rules/orchestration.md, .claude/rules/go.md.
4. For the why only: meta/handoff/2026-10-31-canon-roadmap-v02-v04.md §1 and §3.

ORDER
- ADR-0023 §11.2: S1 (spec sync), then S2 (diag), then the units.
- A unit may start once every unit in its "After" column has landed on main.
- Respect the package queues and the caps of §11.1: at most 6 builders, 2 heavy test runs.
- Start DEP, N5-SYNTAX and IR-RANGE first: N5 is the critical path.
- Then §12, the release.

ONE CALL IS LOUIS'S
ADR-0023 §4, Decision 9: Go enums become opaque structs, so Sovereign's `exhaustive` lint goes
blind on them. Write it into the status file before EN-GO starts. If Louis vetoes, EN-GO waits.
Everything else runs.

RULES THAT BITE (all in ADR-0023 §11.1; repeated here because past runs broke them)
- Memory caps:
  - every agent go test or probe:
    systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local go test -timeout 300s ...
  - copy that line into every VERIFY line;
  - your own `make check` runs under MemoryMax=6G.
- Never `git stash`, `git checkout -- <path>` or `git reset`. Units work in worktrees: you commit
  there and cherry-pick onto main, with `make check` before every `cherry-pick --continue`.
- One builder per package at a time. Check ListAgents before telling Louis that units run.
- End every delegation with "send your final report; do not stop without it".
- Review:
  - every non-trivial diff gets spec-reviewer (opus);
  - on a FAIL, resume the SAME builder with the numbered findings, then the SAME reviewer on the
    fixes only;
  - a sonnet builder that fails twice is replaced by an opus one.
- Never accept "it passed" without the log. Read every golden diff, and map each hunk to an ADR
  section; a hunk that no section explains stops the unit.
- Failing test first:
  - a unit whose first test does not fail on unchanged code STOPS and reports (the ADR's reading
    was wrong); you decide under §0's fallback and log it;
  - except a pin: a test the ADR calls a pin passes on unchanged code by design, and its section
    says how to prove it can fail (ADR-0023 §0);
  - prove a test fails by copying files aside and back, never with git stash.
- A builder's better idea is logged as owed in meta/state.md, and the builder builds the ADR's
  design.
- Status for Louis goes in meta/handoff/2026-MM-DD-v020-status.md, updated at each wave
  boundary, with a two-line chat summary.
- Read-only outside this directory (Resource, Source, the services). Never push, never install.
```

What the session should report at the end:
- the tag;
- the reply handoff's path;
- the bench and farm numbers;
- every recorded miss;
- what stays owed;
- what could not be verified (Windows without CI, MSVC).
