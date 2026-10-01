# M4 units (2026-09-29)

Milestone: IMPLEMENTATION-PLAN §6 M4, `plan.md` "M4 — Formatter and the edit API". Calls in
[decisions/log-2026-09-29.md](decisions/log-2026-09-29.md) "M4". Tier in brackets. Tick with the
landing SHA. Long gates (10-min fuzz, 60 s stress, NFR-01 bench, check-real, CI) run once, after
the last wave.

## Architecture (planner, 2026-09-29; orchestrator-accepted)

- `workspace` owns snapshots over a recording snapshot FS (S1, S3, overlays), ≥64 revisions as
  deltas (S4), per-package staleness from `build.Analysis.Reads` (S5), shared reads (S8), the one
  writer (S9–S11), publish/Watch, and the Edit/Evaluate transactions. `build` stays a stateless
  pipeline plus a `build.Cache` type whose lifetime workspace owns. `api` is an adapter.
- NFR-02 memo, all additive, reached through `build`: parse reuse (`project`, one persistent
  FileSet), `check.Session.Recheck` for body-only file changes, `eval.Memo` per entry (steps
  charged replayed), a load memo and record-check memo in `build`/`rules`. Gate: incremental ≡
  cold property test.
- `edit` owns content (Operation, Lit, codec E24–E26, Apply → Plan, minimal writes, Commit,
  Recover, Refs); it takes no lock and never publishes. Workspace runs the transaction: lock →
  S12 → Apply → S5 → M9 → E18 re-check → E20 lock lines (`build.Analysis.LockUpdates`) → E19 →
  Commit → re-stat → publish.
- Measured (this machine, 24 cores, Ryzen AI 9 HX 370): cold `canon check` of the 7,000-entry
  benchmark 3.1 s wall, 1.15 GB peak RSS. CPU: parse 0.68 s, check 0.62 s, eval 0.7 s, rules
  0.57 s, load 0.38 s, verify 0.2 s.

## Wave 1 — on `main` (`76f6158`..`0c80424`)

U1 format §13/§14.2 · U2 `canon fmt` · U3 workspace + api rewire · U4a edit foundations · U9
views/live · U10 check.Session · U11 eval.Memo · U12 parse reuse.

## Wave 2 — on `main` (`853bedc`..`18e5202`)

U4b edit.Apply · U4c Commit/Recover (ADR-0010) · U5 Watch · U5a Evaluate/Refs/Format · U8 build
memo wiring · U13 rules/verify caches + P9 · U9b show templates and methods · two wiring fixups.

## Wave 3 — on `main` (`9d06725`..`bd9da09`)

U1b format Move · U6 cli refs/--watch/explain inputs · U2b fmt --json-sources numbers · P3 load memo and
lineages · U5b api edit surface (E17–E21, O5, drafts, Op JSON, LockCheck) · P12 stage B/C memos on eval
entries · B1 Recheck folds on the program's Info. Memo architecture: ADR-0011. [items] warm p95 275 ms.

## Waves 3b and 3c — on `main` (`79bcc3b`..`aadf1c5`)

P13b lock merge · P13c i18n cache (FileCache in check) · U7a API rule coverage (148 rules) · U7b
stress/fuzz/bench gates · B2 one budget counter · B3 E14 SetCase · B6 Revision never rolls back · B7
dependent symbols in JSON edits · B8 lock uniqueness · B4 F1 finding paths · B5 build manifest · B10 the
gate-found edit bugs · P13a, P13d guard tests. One green `make check`; CI run 36669002215 on
`claude/m4-ci`.

## Wave 4 — on `main` (`9776183`..`b86648c`)

B11 pairs lists written whole, items in their field's wire units, M9 typed numbers per real file (readings on
`build.Analysis`) · B12 Windows/macOS portability, one path rule (`project.Paths`) · Cleanup-A (E1903 cases,
keyed applied records, one P9 rule, per-instance key refs, the 216 comma region) · Cleanup-B (Consumes
enforced, B1 guard, canonical benchgen) · the spec sync (DECISIONS 230–251). One green `make check`;
CI 36674605430 green on all three platforms (wave-4 subset); `claude/m4-ci2` pushed.

## Wave 5 — on `main` (`7ec4bea`..`fb2cc35`)

B13 UNC completion (workspace, load, edit, cli), host volumes in `/` form, one Windows test model
(`testkit/winpaths`, a port of Go's volumeNameLen), `Paths.RootsFromAPI`. CI 36692101649 green on Windows
and macOS; `make check` green.

## Final-pass fixes (all on `main`)

Found by the final pass (NFR-01 bench 2026-09-30: Edit p95 6.25 s, Evaluate 1.50 s; the formatter
fuzzes). Calls: log "Final pass", "P14".."P20", "PA3-r", "PB4-r".

- B14 formatter fuzz findings [opus] `0c59f40`: both were oracle faults (import comment paired by
  attachment, §8.1; Rewrite region keeps a removed item's trailing comment, §13 step 5); review PASS.
- P14 analyses kept across Edit and Evaluate `fb3db6d` · P15 shared parse prefix, inline-table Recheck
  `d7cf6f7` · P16 `load.dir` elements kept apart `3597ca0` · P17 per-file asset-scan memo `b34ebac`
  (the perf wave, `0c59f40..b34ebac`, each after its review PASS).
- Wave A (`7f81490..779e1d7`): PA1 edit-host file index and slot-graph replay copy `7f81490` · PA2 asset
  placements and defines headers `d06e11f` · P19 monster rewrite, region settle and M9 layout memo
  `779e1d7` (ADR-0012). PA3 incremental revision and history, unwatched re-read `d60b09d`.
- Wave B: PB1 stage C `alone()` by read edges `5abe84b` · PB2 lock facts kept per generation `042c953`
  · PB4 M9 verdicts by content, `format.Adopt` `1b7a390` · PB3 files taken by the snapshot's sum
  `d1234e3` (ADR-0011, amended).
- P20 a real edit-fuzz gate `0d43de7` (four review rounds; the fuzz judge fails a vacuous run).
- Fixups: `79e60ab` M6's one-line clause (P20-r2), `c5324f6` TestRealPaths on Windows.

## Final pass (done 2026-10-01)

All six acceptance items proved; see [handoff/2026-10-01-m4-complete.md](handoff/2026-10-01-m4-complete.md).
NFR-01 bench PASS on `f53ba0c` (Edit p95 0.267 s); 10-min format fuzzes, `make stress`, `check-real` on
`f4e4d48`; 10-min `make fuzz-edit` on `260e598`; CI green on `claude/m4-ci8` (see the handoff).

## Ledger closed

M4 commit range `76f6158..c5324f6` on `main`. No M4 unit is open. Later units and the post-M4 spec
sync are listed in the handoff, not here.
