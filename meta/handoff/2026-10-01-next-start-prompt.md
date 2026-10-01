# Next session start prompt (after M4)

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
M4 is accepted (2026-10-01, main 338c786+). You are the orchestrator (CLAUDE.md,
.claude/rules/orchestration.md). Start the post-M4 work in this order, running each to the end,
or until you hit a philosophy question for Louis (write it to meta/handoff/ and keep going on
what it does not block).

Read first: CLAUDE.md, meta/state.md, meta/plan.md ("M5, M6, M7"), the M4 report
meta/handoff/2026-10-01-m4-complete.md (whole), then meta/decisions/log-2026-09-29.md from the
entry "U-E22-r" to the end.

1. M4.1: dependent fields in multi-op edits (a known defect, first). A request that changes a
   driver field and then its dependent values can fail, or Undo wrongly (API.md E1, E22, E23).
   Inputs, in meta/handoff/2026-10-01-m41-inputs/:
   - ue22-undo-grouping.patch: failed review, kept for its tests;
   - ue1-forward-typing.patch: applies after ue22, unreviewed; root cause in
     internal/edit/applyset.go `targetType`;
   - probe_ue22_test.go.txt: the reviewer's probes; rename to .go to use.
   Design (log "U-E22-r"):
   - build the inverses as now;
   - verify each Undo by a dry apply against the after state;
   - where it fails, restore the smallest enclosing item whole (E22 over E23);
   - fix the forward typing.
   Opus builder, spec-reviewer. The E1/E22/E23 wording joins the spec sync below.
2. The post-M4 spec sync (DECISIONS 207). Draft DECISIONS items and spec text for every "spec sync
   owed" / "post-M4" item listed in the M4 report, and review them with spec-reviewer.
3. Multi-destination emits (DECISIONS 229): Louis's call, after M4 and before M7's integration;
   meta/plan.md has the steps.
4. M5 (language server), M6 (legacy C++ and TS) and M7 (migration), in parallel waves per
   meta/plan.md and IMPLEMENTATION-PLAN §6. Each needs its feature example first (§7.9: `ts`
   before M6; `pairs` is still owed from M2).

Calls already made (Louis), do not re-ask:
- technical calls are yours (decide, log in meta/decisions/log-<date>.md);
- NFR-01's reference machine is this local machine, but it is usually busy with other projects:
  a perf number counts only with the load recorded, and a busy machine is not a regression
  (compare back to back at equal load);
- M1.5's second wave stays parked; fuzzing runs only the durations an acceptance item states;
- you may push `claude/*` branches and watch CI (`gh run …`); `main` is Louis's to push.

Lessons from M4, apply them:
- run concurrency tests at low GOMAXPROCS (2 and 4) before pushing: a P15 bug failed 23/30 at
  GOMAXPROCS=2 and almost never on this 24-core machine;
- a fuzz or bench gate must fail when it measures nothing (P20); read the exec counts;
- every review round finds something: one builder plus the same reviewer per unit until PASS;
- builders sometimes stall with "waiting on background work": if no process runs, have the
  reviewer verify the working tree, or start a fresh builder in the same worktree;
- meta commits on main only, guarded by `test "$(git -C $M rev-parse --abbrev-ref HEAD)" = main`.
```
