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

## Wave 3 (in flight)

- U5b api edit surface: Edit transaction (E17–E21, S12, W5, W15, X2), Open → Recover (O5), drafts
      (V13/V14), Op JSON, LockCheck (B4), live element naming, wire.Host factory; acceptance 5 via api [opus]
- U6 `cli`: explain input fields, `canon refs`, `--watch` [sonnet]
- U2b `fmt --json-sources` typed number canonicalization (FORMATTER §14.1); `--diff` via go-udiff [opus]
- U1b `format` Move kind; edit's Move keeps comments [opus]
- P12 perf: verify + rules memos, allocation cut; target [items] warm ≤150 ms p95 [opus]
- P3 perf: load memo, one lineage per selection, rare compaction; target [items,twin] ≤300 ms p95 [opus]
- Later: cleanup unit (duplicates, P9 leftovers, E1903 for a variant case declaring an input
      field; a neighbour's 216 kept-comma line dropped by the M5 settle, described in M6) — after P12/P3 land.

## Wave 4 (gates, written now, run long once at the end)

- U7a API.md rule-coverage test (`API.md <ID>` citations) [sonnet]
- U7b stress (8 readers/1 editor/1 watcher; 2 s in make check, 60 s opt-in), minimal-write fuzz,
  `make bench-edit` (NFR-01 p95s, CPU model printed) [opus]
- Final pass (orchestrator, once): 10-min `FuzzFormat`/`FuzzFormatJSONSource`/`FuzzRewrite`, minimal-write
  fuzz, 60 s stress, NFR-01 bench, `-format.full`, `check-real`, then CI on `claude/m4-ci`.
