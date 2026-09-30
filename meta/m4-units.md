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

## Wave 4 (in flight)

- B11 `@json(pairs:)` lists edited whole; nested dependent-keyed map symbol keys in Undo [opus]
- Cleanup-A (semantic: E1903, keyed applied records, P9, the 216 neighbour, U13 leftovers, U5b nits) [opus]
- Cleanup-B (mechanical: Consumes enforcement, FileCache test, B1 nits, "dir", benchgen canonical, dups) [sonnet]
- B12 CI portability: macOS symlinked root, Windows memFS roots, symlink skip, overlay keys [opus]
- B13 UNC completion: project.Join/DirOf in workspace, load, edit, cli (after the edit units) [sonnet]

## Final pass

- Final pass (orchestrator, once): 10-min `FuzzFormat`/`FuzzFormatJSONSource`/`FuzzRewrite`, minimal-write
  fuzz, 60 s stress, NFR-01 bench, `-format.full`, `check-real`, then CI on `claude/m4-ci`.
