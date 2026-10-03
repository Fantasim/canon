# Cloud prompt: M6 TypeScript + M5 gaps (written 2026-10-03)

Paste the block below into a claude.ai cloud session on repository Fantasim/canon. It is the
cloud version of [2026-10-02-resume-prompt.md](2026-10-02-resume-prompt.md), which stays the
work order; this block only changes what differs in the cloud.

```
You are the orchestrator of the Canon compiler (CLAUDE.md, .claude/rules/orchestration.md),
running in a cloud session. Work autonomously; Louis is not watching and answers no technical
questions (you decide them and log them in meta/decisions/log-<date>.md).

READ FIRST: CLAUDE.md, .claude/rules/orchestration.md, .claude/rules/go.md, meta/state.md, then
meta/handoff/2026-10-02-resume-prompt.md (THE work order: units T1, G3, M6 append bug, the
queue, and "Rules that bit this session"), then meta/decisions/log-2026-10-02.md from
"Louis: after M5" to the end, and DECISIONS 274-278. Everything below overrides the resume
prompt only where it says so.

BRANCHES (DECISIONS 28)
- Create claude/m6-run-1 from origin/main (check it contains e460de0). It stands in for main:
  every unit lands there after review PASS and a green `GOTOOLCHAIN=local make check`. Push it
  after every landed unit. Never push main, never --force, never delete remote branches.
- The unreviewed WIP is on origin/wip/2026-10-02: a63cadf (G3), a3227ce (M6 bug),
  6a1e3a9 (T1), b96efa3 (reviewer probes, never land it). Bring units back with
  `git cherry-pick -n <sha>` as the resume prompt says. Commit discipline: the resume prompt's
  "clean worktree" rule, with claude/m6-run-1 in place of main.

ENVIRONMENT (differs from the local machine)
- Before any make check, install the TypeScript test toolchain (T1 makes make check require
  it): `node --version || sudo apt-get install -y nodejs npm`, then after T1's files are in the
  tree `npm ci --prefix tools/tsc` (in every worktree that runs make check, or symlink
  tools/tsc/node_modules). If the C++ toolchain is reported missing:
  `sudo apt-get install -y nlohmann-json3-dev`.
- The T1 reviewer probes (/var/tmp/canon-probes-2026-10-02 in the resume prompt) are in commit
  b96efa3 under .probes/2026-10-02-t1rev/: `git worktree add /var/tmp/probes b96efa3` and follow
  its README (run.sh / cmp.sh take $CANON). The fuzz input 881024e08ddc6c7d is already in a3227ce.
- No systemd: where a target or memory note says `systemd-run ... MemoryMax`, use
  `ulimit -v 12000000` instead (not under -race). If `make fuzz-edit` fails only for want of
  systemd-run, run its go test command with ulimit by hand and say so in the report.
- At most 2 heavy go test / make check at once, at most 3 builders at once.
- CI: every push to claude/m6-run-1 triggers .github/workflows/check.yml (Linux, macOS,
  Windows). Step 5 of the work order is: the final push's run green; record its run id.

STOP CONDITION: T1, G3, the M6 append bug and the queued M5 gaps (G4, G5, the two wire bugs)
reviewed PASS, landed on claude/m6-run-1, CI green. Stop earlier only on a unit boundary.

BEFORE YOU STOP (always)
- meta/state.md and meta/plan.md updated (SHAs on claude/m6-run-1).
- A report meta/handoff/<date>-cloud-m6-run-1.md: landed (SHAs), CI run id and result, WIP with
  pushed branch and remaining list, decisions, NITs, not verified, stale branches.
- Everything committed and pushed.
```

After the run, locally: fetch, check `claude/m6-run-1` fast-forwards `main`, run `make check`
and `-race`, fast-forward `main`, and delete `wip/2026-10-02` once all three units landed.
