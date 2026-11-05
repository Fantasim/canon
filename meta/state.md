# State — Canon compiler

Updated: 2026-11-05 (v0.3 run, §0 units landing). **v0.2.0 released 2026-11-02** (tag on `a0fb77a2`, pushed with `main` and
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

**In flight (2026-11-05): v0.3, lean generated code** ([ADR-0026](decisions/0026-v03-lean-generated-code.md)).
Its 34 units and CLEANUP are on `main`. Its §0 new-bug units are landing, DECISIONS 453-491 so far
(edit/layer correctness, Graph at any depth, `canon check` over every layer: 469-472; copy identity under
layers 486/491; amend keys 487; ref-chain keys 490). The run's state lives in
`/var/tmp/canon-v030/roster.tsv` and `pending-log.md`; fold them into `meta/decisions/log-2026-11-0x.md` at
the end. Then §18: the long gates, the Resource dry run, release notes; Louis pushes. Nothing is pushed yet.
Its §0 flags four calls for Louis, none blocking. v0.4 formulas ([ADR-0025](decisions/0025-v04-formulas-outline.md))
follows; v0.5 binary data (with baked views and presence bits) and v0.6 runtime values are direction
([roadmap](handoff/2026-10-31-canon-roadmap-v02-v04.md)). Louis, 2026-10-31: formulas before binary data;
the on-disk cache (Q3, waiting for v0.3's scale bench) and N1 parked. [Plan](plan.md); [all handoffs](handoff/README.md).
`make realdata-compile` is required before every tag (394); `RD_PATCH` applies a diff first.

**Owed:** ADR-0026 rules every item of v0.2's "What stays owed": in v0.3 (§14-§16), or decided out with
the record corrected (§14.4, §15.9: the define-guard over-wrap is no bug since `3cdc78c9`; dead `ModFloat`
kept on purpose; the `kept` memo measured flat; Resource does use J13/J14 through a ref). After v0.3: its §19.
ADR-0026 §19's items (found while planning, not in v0.3) are in [plan.md](plan.md)'s v0.4, v0.5 and later sections.
Parked: the cache's stat table, `evalAnalysis` → `KeptSelection`; M4 call (b) declined. Still open: the
studio's full reload has no target.
**Documented limits:** `NoHook` is filled for the import closure only (ADR-0021 §2.5); MSVC at scale.
Environment: `internal/workspace` watch tests skip when inotify watches run out (raise
`fs.inotify.max_user_watches`). Cloud: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (caps by hand under `ulimit -v`).

## Milestones

M0-M5, post-M4, M1.5 done. M6: TS, C++ baked done; legacy C++ waits (Louis). M7 not started. v0.2 ticked
in [plan.md](plan.md).

## What exists (committed)

spec + DECISIONS 1-429; `syntax`, `format`, `jsonsrc`, `wire`, `load`, `check`/`types`, `eval`/`eval/std`
+ `value`, `verify`, `lock`, `rules`, `ir`, `gen/json`, `gen/ts`, `gen/go`, `gen/cpp`, `gen/view`, `views`,
`i18n`, `conform`, `build`, `project`, `workspace`, `writelock`, `lsp`, `edit`, `api`, `cli`
(version/init/new/check/build/test/explain/fmt/guide/help/graph), `internal/testkit`, `tools/audit`.
`examples/features/pkgchecks` is the first `tests.txt`. Units: [m4-units.md](m4-units.md).

## Open Louis-calls

None blocking. Decision 9 (Go `exhaustive` blind on opaque enums) was built as written; no veto came.
Q1 (runtime values as schema packages, v0.6) waits for his yes. One ask of the run, for him: a
builder's leftover `sleep` loop (a zsh `until grep … sleep 15`, pid 2792522, watching
`subjcap-all.log`) was still running on this machine on 2026-11-02; the orchestrator does not kill
another process on a builder's behalf. ADR-0026 §0 flags four calls (header weight dropped, baked views to
v0.5, two lints decided out, the scale bench for Q3); its §17.1 puts new constants in `<pkg>/errors.go`.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`; temp dirs in `/var/tmp`.
- Agents share one working tree: never `git stash`; parallel units run in worktrees, cherry-picked
  after `make check`; land only on the run's own `exit=0`, and generator units also run `internal/cli`'s
  goldens and `make diag-check`. Review rules: `.claude/rules/orchestration.md`.
- At most two agents run heavy tests (eval, `-race`, fuzz) at once; fuzz runs at `-parallel 4`.

## What could not be verified

Windows (no CI): junctions, reported names, the refresh stamp (vet and Linux models only). MSVC. Source not
compiled (its 70 `…FileFile` sites unmigrated); wiki not in `realdata-compile`; the services' own CI and
Resource's non-canon gate checks not run. `make bench-edit` still exceeds its old 0.300/0.150 s targets
(on v0.1.13 too). Older caveats: [releases.md](releases.md).

## Verify queue

Re-run `make bench-edit` on a quiet machine or a CI-class runner when one exists. After the push: CI on
the tag, once it returns.
