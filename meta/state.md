# State — Canon compiler

Updated: 2026-09-25 (cloud run). The overnight run resumed in a Claude Code cloud session on
branch `claude/m3-run` (stands in for `main`; Louis fast-forwards `main` from it). Resume plan:
[handoff/2026-09-25-pause.md](handoff/2026-09-25-pause.md); calls of the cloud run:
[decisions/log-2026-09-25.md](decisions/log-2026-09-25.md); earlier calls:
[decisions/log-2026-09-24.md](decisions/log-2026-09-24.md) "Overnight run". Stopped on budget;
report and remaining lists: [handoff/2026-09-25-cloud-run.md](handoff/2026-09-25-cloud-run.md).

## Current focus — overnight run tracker (resume from here)

Ticked means committed on `claude/m3-run` (cloud run) or `main`. Unfinished units sit as unreviewed `wip:` commits on their
worktree branches. They are listed in the pause report with what each still needs.
- [x] A1 loader parity (2eb3294, c9a44cb) · [x] A2 demo bugs (428fbad, 6534f56) · [x] A3 M1.5
  bug-fix wave · [x] A4 M2 accepted (94ddb96).
- [ ] A5: consumer units landed (d5de44d, 5d8232a). ERRORS.md pass: ir group landed (e1eaa0a); check group + check follow-up landed (7d35a39,
  154aa30, a1aa811); load group queued
  (script `load.py`, see pause report). Spec sync #2 and cleanups queued.
- [ ] A6 M1.5 second wave: WIP, review FAILED, branch `claude/wip-a6-progen`.
- [x] W0 gap map (1b0a332).
- [ ] W1: landed: eval layers/provenance (bc99b99, 02f203c), load forms (1a77900), dependent
  types in check (3e46ab3, b2f063c). Gate lift + load into an applied type landed on
  `claude/m3-run` (d5062a3).
- [ ] W2: gen/go inputs + unions landed (8e816d3). Open: gen/cpp inputs round 2 WIP
  (`claude/wip-a17381f528923b9cb`); ir name-plan unit WIP, review FAILED (`claude/wip-ir-names`);
  pattern translator; verify dependent (E3801/E3802);
  view/translation checking; ir `types` mode; dependent types in both generators.
- [ ] W3 · [ ] W4 (see [plan.md](plan.md) "M3 execution").

Milestones: M0, M1, M2 accepted (M1 item 6 deferred to after M7). M1.5 foundation committed
(f498713), not ticked.

## What exists (committed)

spec + DECISIONS 1–219; `syntax`, `format`, `jsonsrc`, `wire`, `load` (every WIRE §6 form),
`check`/`types` (dependent types), `eval`/`eval/std` + `value` (layers, provenance), `verify`,
`lock`, `rules`, `ir` (stage E, fingerprint, Go and C++ name plans), `gen/json`, `gen/go` (baked,
data, stores, translated fns, conformance, runtime inputs, unions), `gen/cpp` (data mode, stores,
runtime, conformance, strict loaders), `conform`, `build`, `project`, `api` (Check/Build/Test),
`cli` (version/init/new/check/build/test), `internal/testkit` (+ `cxx`, `progen`);
`tools/audit`.

## Open Louis-calls

None. There are no direction questions (`handoff/2026-09-24-questions.md` does not exist).

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G` (an uncapped eval
  probe took 24 GB twice). Temp dirs go to `/var/tmp`, never `/tmp` (a 15 GB tmpfs that filled
  twice).
- Agents share one working tree, so **never `git stash`**. Parallel units run in worktrees:
  commit there, cherry-pick, and run `make check` before every continue. A worktree whose base
  is old is rebased onto main by the orchestrator before its review.
- `GOLANGCI_LINT_CACHE` is private per worktree. The audit now fails on an unmeasured lane
  (5b37369).
- Sonnet units that failed review twice moved to opus. Review catches real defects: plan on
  2–3 rounds per unit.

## What could not be verified

Windows and macOS real runs; MSVC 19.2x; GCC 9; Clang 10; nlohmann/json 3.9; CI on a runner.
Only local g++ 15.2 and clang++ 21.1 were used.
