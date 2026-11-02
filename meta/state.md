# State — Canon compiler

Updated: 2026-11-02. **v0.2.0 released 2026-11-02** (tag on `a0fb77a2`, pushed with `main` and
published as the GitHub release at Louis's request; installed in `~/.local/bin`, 0.1.13 kept as `canon-0.1.13`). It is the trust release: `expect package`, checked decoders, bound constants, opaque enums, macro and
wire-name refusals, DECISIONS 398-429 ([ADR-0023](decisions/0023-v02-trust-release.md),
[log 10-31](decisions/log-2026-10-31.md), [log 11-01](decisions/log-2026-11-01.md),
[reply](handoff/2026-11-02-canon-reply-v020.md), [run status](handoff/2026-10-31-v020-status.md)).
The language stays `canon: "0.1"` (422). v0.1.13 and earlier (pushed, with GitHub releases) and their
headlines: [releases.md](releases.md). `main` is the only branch.

## Current focus

**Next for the Sovereign sessions:** Scripts' canon-bump moves the pin (Resource pre-push and
`lib/canon_gate.sh` still pin 0.1.13, so Resource pushes are refused until then), with the badge patch;
then the reply's migrations. Then the Sovereign sessions apply the reply's migrations (Go: wiki 5, monitoring 3, trello 2
sites; TS: admin 1, monitoring 3; Source: 70 `…FileFile` call sites; Resource: the badge patch, the
`WindowIds.h` defines, `expect package` for its `…Problems` workarounds).

**Next step: v0.3, lean generated code** ([ADR-0024](decisions/0024-v03-lean-generated-code-outline.md),
an outline). Its §0: measure first (§3), then write the decision-complete ADR in the format of
ADR-0021 and ADR-0023 (v0.2 changed the code its items touch), then run it. v0.4 formulas
([ADR-0025](decisions/0025-v04-formulas-outline.md)) follows; v0.5 binary data and v0.6 runtime values
are direction ([roadmap](handoff/2026-10-31-canon-roadmap-v02-v04.md)). Louis, 2026-10-31: formulas
before binary data; the on-disk cache and N1 parked. [Plan](plan.md); [all handoffs](handoff/README.md).
`make realdata-compile` is required before every tag (394); `RD_PATCH` applies a diff first.

**Owed (nothing is a wrong verdict):** the reply's "What stays owed" groups it: generators (E8005 hint
gaps, define-guard over-wrap, dead `ModFloat`, function-size items), checker and eval (the `kept` memo
never pruned, dependent-value residues, `where`-only literals judged free), edit and workspace (typing
spread lines, an Undo marker, `Options.FS` docs, writelock absolute names, Windows spellings), test
tidying, and spec-wording items listed in the two logs. Parked: the cache's stat table,
`evalAnalysis` → `KeptSelection`; M4 call (b) declined. Still open: the studio's full reload has no
target; VIEWMODEL J13/J14 through a ref.
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
another process on a builder's behalf. Owed, technical (v0.3): `internal/ir/constants.go` is at the
audit's 500-line limit; the next constant goes in a split by concern.

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
