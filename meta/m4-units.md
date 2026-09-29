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

## Wave 3b (in flight, launched 2026-09-30)

- B2 one budget counter per invocation (DECISIONS 104; log M4 B1) [opus]
- P13b `lock` merge sorted once [sonnet] · P13c `i18n` bad-node walk not repeated per run [sonnet]
- U7a API.md rule-coverage test and its gaps (acceptance 2) [sonnet]
- U7b stress 8/1/1 (2 s in check, 60 s `make stress`), minimal-write fuzz on every example and the
      benchmark (`make fuzz-edit`), `make bench-edit` NFR-01 gate (acceptance 3, 4, 6) [opus]
- B3 E14 `SetCase` keeps only fields that satisfy the new case whole; edit golden harness fails on an
      unexpected error; M6 one-line form not opt-in (found by U7a) [opus]
- B6 S10: `Project.Revision()` never rolls back [opus] · B7 edit JSON printer writes a dependent
      symbol [opus] · B8 AllowErrors lock ids unique among the post-edit sources' facts [opus]
- B10 (after B3, B7): M6 regions on a JSON last-member modify plus insert; JSON Remove ErrInternal [opus]

## Wave 3c (after 3b)

- P13a `build/hosts.go` listings as maps · P13d `check` `Info.cloned` allocation [opus]
- B4 F1 Path for a finding in a top-level `let` initializer (knownbug test from U7a) [sonnet]
- B5 build manifest computed (WIRE §10, LOD-11; O7's manifest half); the cache stays inert [opus]
- Cleanup unit: duplicates, P9 leftovers, E1903 for a variant case declaring an input field; a
      neighbour's 216 kept-comma line dropped by the M5 settle (M6); U5b nits (`unwritable` gives
      absolute names their own reason, `lockStable` tests the table first); B1 nits (`replayFolds`
      comment cites the real invariant, a free guard ending the lineage when swapped decls fold,
      `holdsCode` reuses `hasCode`); benchgen writes canonical JSON (then drop the fmt step in
      `fuzz-edit`/`bench-edit`); TestDependencyRule also enforces each package's §3 Consumes row
      (log M4 P13c-r: listed or reachable through the row), violations fixed or reported; a table
      test for `check.FileCache` (`Of`, `KeepOnly`)

## Final pass

- Final pass (orchestrator, once): 10-min `FuzzFormat`/`FuzzFormatJSONSource`/`FuzzRewrite`, minimal-write
  fuzz, 60 s stress, NFR-01 bench, `-format.full`, `check-real`, then CI on `claude/m4-ci`.
