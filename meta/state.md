# State — Canon compiler

Updated: 2026-11-06 (v0.3 run, §0 units landed). **v0.2.0 released 2026-11-02** (tag on `a0fb77a2`, pushed with `main` and
published as the GitHub release at Louis's request; installed in `~/.local/bin`, 0.1.13 kept as `canon-0.1.13`). It is the trust release: `expect package`, checked decoders, bound constants, opaque enums, macro and
wire-name refusals, DECISIONS 398-429 ([ADR-0023](decisions/0023-v02-trust-release.md),
[log 10-31](decisions/log-2026-10-31.md), [log 11-01](decisions/log-2026-11-01.md),
[reply](handoff/2026-11-02-canon-reply-v020.md), [run status](handoff/2026-10-31-v020-status.md)).
The language stays `canon: "0.1"` (422). v0.1.13 and earlier (pushed, with GitHub releases) and their
headlines: [releases.md](releases.md). `main` is the only branch.

## Current focus

**Next for the Sovereign sessions:** the pin moved (Resource's pre-push and Scripts' `lib/canon_gate.sh` pin
0.2.0; Resource `23d54ac6`); the reply's migrations remain (Go: wiki 5, monitoring 3, trello 2 sites; TS: admin 1, monitoring 3; Source: 70 `…FileFile` call
sites; Resource: the `WindowIds.h` defines, `expect package` for its `…Problems` workarounds).

**In flight (2026-11-06): v0.3, lean generated code** ([ADR-0026](decisions/0026-v03-lean-generated-code.md)).
Its 34 units and CLEANUP were on `main` by 2026-11-04; the §0 new-bug units are now all landed, DECISIONS
453-514 (run log: [log 11-06](decisions/log-2026-11-06.md)). `main` is at `13ff57a3`; nothing is pushed since
v0.2.0. Headlines: edit and layer correctness (486-506); shared values walked once in stages A, B, C and the
wire-form check (507, 509, 511, 513), so deeply shared valid values no longer take exponential time (Louis
asked); identity rules for amended copies and element assignment (510, 512, hard error E4004); a poisoned
`load.dir` table stays repairable (508); a layer naming a broken declaration is broken (514, was an
internal compiler error). Louis, 2026-11-06: "fix the critical ones now"; bar: a wrong result, hang or
crash is fixed in v0.3, the rest goes to v0.4. v0.4 limits recorded in DECISIONS: ENTRYSPAN; VERIFY-SHAREDFOUND
(509), EVAL-WHEREPERPATH and EVAL-STOREFOUND (513), IR-SHAREDWALK (exponential only for `where` leaves and
invalid values); EVAL-LITERALREKEY (512); RECHECKBASE (506); JSONREPAIR limits (508); Refs omitting a read
under a replaced layer entry (506).
**Next:** ADR-0026 §18: the long gates one at a time (the machine is swap-limited); the Resource dry run
(`make realdata-compile`); release notes drafted for Louis via `--notes-file`; Louis pushes and tags.
Then v0.4 formulas ([ADR-0025](decisions/0025-v04-formulas-outline.md)); v0.5 binary data and v0.6
runtime values are direction ([roadmap](handoff/2026-10-31-canon-roadmap-v02-v04.md)). Louis, 2026-10-31: formulas before binary data;
the on-disk cache (Q3) and N1 parked. [Plan](plan.md); [handoffs](handoff/README.md).
`RD_PATCH` applies a diff before `make realdata-compile` (required before every tag, 394).

**Owed:** ADR-0026 rules every item of v0.2's "What stays owed": in v0.3 (§14-§16), or decided out with
the record corrected (§14.4, §15.9: the define-guard over-wrap is no bug since `3cdc78c9`; dead `ModFloat`
kept on purpose; the `kept` memo measured flat; Resource does use J13/J14 through a ref). ADR-0026 §19's
items (found while planning, not in v0.3) are in [plan.md](plan.md)'s v0.4, v0.5 and later sections.
Parked: the cache's stat table, `evalAnalysis` → `KeptSelection`; M4 call (b) declined. Open: studio full reload.
**Documented limits:** `NoHook` is filled for the import closure only (ADR-0021 §2.5); MSVC at scale.
Environment: `internal/workspace` watch tests skip when inotify watches run out. Cloud: `apt-get install
libc++-18-dev libc++abi-18-dev`, `npm ci --prefix tools/tsc`; no systemd (caps by hand under `ulimit -v`).

## Milestones

M0-M5, post-M4, M1.5 done. M6: TS, C++ baked done; legacy C++ waits (Louis). M7 not started.
## What exists (committed)

spec + DECISIONS 1-514; `syntax`, `format`, `jsonsrc`, `wire`, `load`, `check`/`types`, `eval`/`eval/std`
+ `value`, `verify`, `lock`, `rules`, `ir`, `gen/json`, `gen/ts`, `gen/go`, `gen/cpp`, `gen/view`, `views`,
`i18n`, `conform`, `build`, `project`, `workspace`, `writelock`, `lsp`, `edit`, `api`, `cli`
(version/init/new/check/build/test/explain/fmt/guide/help/graph), `internal/testkit`, `tools/audit`.
`examples/features/pkgchecks` is the first `tests.txt`. Units: [m4-units.md](m4-units.md).

## Open Louis-calls

None blocking. Decision 9 (Go `exhaustive` blind on opaque enums) was built as written; no veto came.
Q1 (runtime values as schema packages, v0.6) waits for his yes. A builder's leftover `sleep` loop (pid
2792522, watching `subjcap-all.log`) was still running on 2026-11-02; the orchestrator does not kill it.
ADR-0026 §0 flags four calls (header weight dropped, baked views to v0.5, two lints decided out, the scale
bench for Q3); its §17.1 puts new constants in `<pkg>/errors.go`.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`; temp dirs in `/var/tmp`.
- Agents share one working tree: never `git stash`; parallel units run in worktrees, cherry-picked
  after `make check`; land only on the run's own `exit=0`, and generator units also run `internal/cli`'s
  goldens and `make diag-check`. Review rules: `.claude/rules/orchestration.md`.
- At most two agents run heavy tests at once; fuzz runs at `-parallel 4`.

## What could not be verified

Windows (no CI): junctions, reported names, the refresh stamp (vet and Linux models only). MSVC. Source not
compiled (70 `…FileFile` sites unmigrated); wiki not in `realdata-compile`; services' CI not run. `make bench-edit` still exceeds its old 0.300/0.150 s targets
(on v0.1.13 too). Older: [releases.md](releases.md).

## Verify queue

Re-run `make bench-edit` on a quiet machine or CI-class runner. After the push: CI on the tag.
