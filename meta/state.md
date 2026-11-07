# State — Canon compiler

Updated: 2026-11-07 (v0.3.0 tagged locally, not pushed). **v0.3.0, lean generated code** ([ADR-0026](decisions/0026-v03-lean-generated-code.md)):
every §18 step passed ([final acceptance](decisions/log-2026-11-06.md#final-acceptance-adr-0026-18-step-8-2026-11-07)),
DECISIONS 430-516, reply (`handoff/2026-11-07-canon-reply-v030.md`, written after the tag), release notes for `gh release create --notes-file`
in `handoff/2026-11-07-release-notes-v030.md` (written after the tag). **Louis pushes** `main` and
the tag; nothing is installed (0.2.0 stays in `~/.local/bin`). v0.2.0 (2026-11-02, tag on `a0fb77a2`) was the
trust release, DECISIONS 398-429 ([ADR-0023](decisions/0023-v02-trust-release.md)). The language stays
`canon: "0.1"` (422, 452). Older releases: [releases.md](releases.md). `main` is the only branch.

## Current focus

**Next for the Sovereign sessions:** the pin moved (Resource's pre-push and Scripts' `lib/canon_gate.sh` pin
0.2.0; Resource `23d54ac6`); the reply's migrations remain (Go: wiki 5, monitoring 3, trello 2 sites; TS: admin 1, monitoring 3; Source: 70 `…FileFile` call
sites; Resource: the `WindowIds.h` defines, `expect package` for its `…Problems` workarounds).

**Done (2026-11-07): v0.3.0.** 34 units, CLEANUP and the §0 new-bug units (DECISIONS 453-516; run log
[log 11-06](decisions/log-2026-11-06.md)). Headlines: smaller generated C++ (Resource `.gen.cpp` 16.45 -> 7.57 MB),
`out_templates`, `canon lint`, ordered `Decode`; edit and layer correctness (486-506, 514); shared values walked
once (507, 509, 511, 513); identity rules (510, 512, E4004); narrowing conditions in error (516); `canon test`'s
ref index shared (515). Bar (Louis, 11-06): a wrong result, hang or crash is fixed in v0.3, the rest is v0.4.
v0.4 list (recorded in DECISIONS): ENTRYSPAN; VERIFY-SHAREDFOUND (509), EVAL-WHEREPERPATH and EVAL-STOREFOUND
(513), IR-SHAREDWALK; EVAL-LITERALREKEY (512); RECHECKBASE (506); JSONREPAIR limits (508); Refs omitting a read
under a replaced layer entry (506); BUILD-TESTCHECKREUSE, EVAL-KEPTLITSRELEASE (449: `canon test` 60 s / 3.5 GB
on Resource vs 48 s / 1.5 GB); VALUE-MAPCANDIDATES; E3605's field-read E3003; a layer syntax error outside its
blocks gives an empty R6 cause.
**Next:** Louis pushes and publishes; then Resource and Scripts move their pin to 0.3.0 (the reply's upgrade notes).
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

spec + DECISIONS 1-516; `syntax`, `format`, `jsonsrc`, `wire`, `load`, `check`/`types`, `eval`/`eval/std`
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
compiled (70 `…FileFile` sites unmigrated); wiki not in `realdata-compile`; services' CI not run. Older:
[releases.md](releases.md).

## Verify queue

After the push: CI on the tag (CI is down; macOS and Windows were vetted, not run).
