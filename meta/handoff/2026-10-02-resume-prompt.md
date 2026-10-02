# Resume prompt (stopped 2026-10-02, mid M6-TypeScript and the M5 gaps)

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
You are the orchestrator (CLAUDE.md, .claude/rules/orchestration.md). Resume exactly where the
2026-10-02 session stopped. Read first: CLAUDE.md, meta/state.md, this file
(meta/handoff/2026-10-02-resume-prompt.md), then meta/decisions/log-2026-10-02.md from
"Louis: after M5" to the end, and DECISIONS 274-278.

STATE
- main (HEAD a7ee52e + this handoff commit) is clean and reviewed: M5 accepted (CI run
  36973712977 on 398d907); since then committed and green: canon guide, releases + install.sh,
  README, features/ts example, the stable-undo lock-fact rule (799c1f3), lock check/--color/--lang
  (5f4360a). Commits after 398d907 have NOT been through CI yet.
- Unreviewed work is on branch wip/2026-10-02, three commits on top of a7ee52e, one per unit:
    a63cadf WIP(check)  G3  @files template names -> E2102
    a3227ce WIP(edit)   M6  append before a trailing comment (fuzz 881024e08ddc6c7d)
    6a1e3a9 WIP(gen/ts) T1  the TypeScript target, round 5 built
  Bring a unit back as uncommitted changes with `git cherry-pick -n <sha>` (one at a time, or
  all three), then continue it. Do not merge the branch; delete it when all three have landed.
- Reviewer probe projects and the fuzz input are kept in /var/tmp/canon-probes-2026-10-02/
  (t1rev/c1..c18 with run.sh and cmp.sh for the TS review; fuzz-881024e08ddc6c7d).
- `npm ci --prefix tools/tsc` is already done on this machine (node_modules is git-ignored); the
  T1 commit makes `make check` export CANON_REQUIRE_TS=1.

UNITS, IN ORDER
1. T1 TypeScript target (opus builder; sonnet failed review twice, so opus only). Round 5 fixed
   review round 4's D1 (bigint map keys, decBigKey), D2 (refs into @ts(bigint) key fields are
   bigint everywhere; additive ir.RefTarget.BigInt; E8101 covers refs), D3 (data values of a
   foreign types-mode record decode via decForeign; refusals renamed ForeignDataRecord). Next:
   spec-reviewer (opus) round 5 on internal/gen/ts, internal/ir (§4.5 owner review: BigInt field),
   internal/build, internal/testkit/{tsc,golden}, tools/tsc, Makefile, .github, the TS goldens,
   internal/cli/testdata/commands/build_ts_emit.txtar; re-run t1rev c1-c18 with run.sh/cmp.sh and
   the corpus sweep (every txtar re-targeted to ts in 4 modes). Log the BigInt IR field as a ruling.
2. G3 @files names (check). Built; still to do: (a) dependent branches as candidates in
   check fieldSets so {spot.x} is checked and recorded (ruled, log "G3"); (b) api
   TestRenameNameCaptureInQuietPlaces: its four unresolved-template cases now expect NotEditable
   reason broken (E32), comment cites DECISIONS 277. Then spec-reviewer.
3. M6 minimal-write bug (edit): AddEntry appended to a table literal whose last line is a comment
   before `}` changed bytes outside its region (vocab.canon). Fix + goldens written; the builder was
   stopped while running -race; it reported the failure it saw also happens at HEAD (likely G3's
   WIP). Finish verification (go test, -race, make fuzz-edit 2m), fix the two comment-adr-narration
   audit findings in internal/edit/applyregion.go:206 and :223, then spec-reviewer. Commit body:
   symptom -> cause -> fix.
4. Queued (not started): G5 translate named-check messages under --lang (I18N.md B5; check/build;
   an example with a French check message); wire bugs (after T1, they touch ir/precompute):
   the emit-json encoder overflows on a stored fn returning self (internal/wire/encode.go), and
   "jsongen: export fn without its stored results" for stored fns of a stored result's receiver,
   each to become a stage-E finding or a fix; G4 cleanup (export check.convertCase and drop edit's
   copy; move the rename test fixture helper into testkit).
5. Then: CI on a claude/* branch (Linux, macOS, Windows; TS jobs included), update meta/state.md
   and plan.md, report to Louis. Not in scope unless Louis asks: M6's legacy C++ struct modes, M7.

RULES THAT BIT THIS SESSION
- Commit only after review PASS and `make check` green in a clean worktree holding exactly the
  unit's files (other units' WIP shares the tree): git worktree add, copy the files, make check,
  commit there, then commit the identical files on main with `git commit -C <sha>`
  (cherry-pick fails while the main tree has those files modified).
- Each review round finds rarer shapes; when a fix is patching shape by shape, stop and rule a
  general rule in the spec (as E23 "an Undo never undoes a lock fact").
- Louis decides only philosophy/scope, dependencies and audit loosening; technical calls are yours,
  logged in meta/decisions/log-<date>.md, lasting ones as DECISIONS items with the spec synced.
```
