# Next session start prompt (M5 start)

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
The post-M4 work is done and on main (HEAD 37968e3): the spec syncs (DECISIONS 252-273), PS1-PS3,
multi-destination emits (ME1), DV1 and M4.1. You are the orchestrator (CLAUDE.md,
.claude/rules/orchestration.md). Start M5 (language server), and run M6 (legacy C++ and TS) and
M7 (migration) in parallel waves per meta/plan.md and IMPLEMENTATION-PLAN §6, each to the end, or
until you hit a philosophy question for Louis (write it to meta/handoff/ and keep going on what it
does not block).

Read first: CLAUDE.md, meta/state.md, meta/plan.md ("M5, M6, M7"), the M4 report
meta/handoff/2026-10-01-m4-complete.md (whole, with its "Post-M4 (done)" section), the unit ledger
meta/m4-units.md, then meta/decisions/log-2026-09-29.md from the entry "U-E22-r" to the end.

M5: IMPLEMENTATION-PLAN §8.4 (scope) and §6 M5 (acceptance): JSON-RPC transcripts
internal/lsp/testdata/*.txtar for every CLI.md §4 feature, UTF-16 tests, diagnostics within 500 ms of
the last change, findings in JSON files published without the file open. Packages: `lsp`
(today only doc.go and an example test), `editors/vscode` (absent). Each milestone needs its feature
example first (§7.9: `ts` before M6; `pairs` is still owed from M2).

Calls already made (Louis), do not re-ask:
- technical calls are yours (decide, log in meta/decisions/log-<date>.md);
- NFR-01's reference machine is this local machine, but it is usually busy with other projects:
  a perf number counts only with the load recorded, and a busy machine is not a regression
  (compare back to back at equal load);
- M1.5's second wave stays parked; fuzzing runs only the durations an acceptance item states;
- you may push `claude/*` branches and watch CI (`gh run ...`); `main` is Louis's to push.

Lessons, apply them:
- run concurrency tests at low GOMAXPROCS (2 and 4) before pushing: a P15 bug failed 23/30 at
  GOMAXPROCS=2 and almost never on this 24-core machine;
- a fuzz or bench gate must fail when it measures nothing (P20); read the exec counts;
- every review round finds something: one builder plus the same reviewer per unit until PASS;
- commit every edit shape to a real disk and apply the Undo of the Undo: the overlay and the
  commit disagreed in M4.1 (rounds 5 and 6), and only a disk run showed it;
- builders sometimes stall with "waiting on background work": if no process runs, have the
  reviewer verify the working tree, or start a fresh builder in the same worktree;
- meta commits on main only, guarded by `test "$(git -C $M rev-parse --abbrev-ref HEAD)" = main`.

Logged later items (none blocks M5; each is in meta/decisions/log-2026-09-29.md and, for the
checker gaps, DECISIONS "Still open"):
- E15 drops a map with dependent values to its default;
- an edit-layer amendment whose right-hand side is a `load(...)` (its JSON lies outside the layer file);
- checker gaps: E3003 on `Target(sub.kind)` with `sub: Sub(goal)`; E3806 on `Pick(goal)` (a ref chosen
  by its driver); an amend path into a dependent map (`dm["a"]`, E1905/E2102);
- one shared below-a-field scope helper for eval and edit (owned by TYP);
- format test callers pass `f.FileKind` as the role, and a comment in
  internal/workspace/verdicts_internal_test.go still says "parse kind" (PS3);
- API.md R6 gives no cause for a constant only a stage-E fold evaluated (cold `Cause` is empty);
- E3015 `budget`'s `{what}` is the refinement bound only while check folds nothing else: if check
  ever folds a non-bound position, the folder must be told the kind by an additive §4.7 query under
  §4's review rule (DV1).
```
