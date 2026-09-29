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

## Wave 1

- [ ] U1 `format` + `jsonsrc`: gap audit vs FORMATTER §1–§12, §15; JSON printer §14.1; §13/§14.2
      primitives (`format.Node`, `format.Expr`) for `edit` [opus]
- [ ] U2 `cli`: `canon fmt` (`--check`, `--diff`, `--json-sources` static interim) [sonnet] — PASS, wt commit 24c5e71
- [ ] U3 `workspace` core + `api` rewire (S1–S11, O6, O7, overlays) [opus]
- [ ] U4a `edit` foundations: Operation, Lit, V1–V4, codec E24–E26, Refs R7/R8 [opus]
- [ ] U9 `views/live`: Evaluate content V5–V12 [opus] — in review (show/methods behind a seam → U9b)
- [ ] U10 `check.Session`/`Recheck` [opus]
- [ ] U11 `eval.Memo` [opus]
- [ ] U12 `project` parse reuse [sonnet] — PASS, wt commit a2c3bbc

## Wave 2

- U4b `edit` apply: E1–E16, E22–E23, M1–M9, N1–N8, edit txtar goldens (M6 on every case) [opus]
- U4c `edit` commit/journal/recover: N9–N12, O5, crash test (acceptance 5) [opus]
- U5 watch: `workspace/watch*.go`, api/watch.go, W12–W16 [opus]
- U5a api reads: Evaluate (no draft), Refs, Format/FormatJSONSource (T1, T2) [opus]
- U8 `build` memo integration: `build.Cache`, load memo, LockUpdates, LockCheck (B4), atomic
  fsynced OS WriteFile; incremental ≡ cold property test [opus]
- U13 `rules` record-check memo [opus]
- U9b `views/render` + `eval` + `build` adapter: render `show` templates and view-named methods
      (fills U9's `live.Lines` and `Input.Bound` seams), `index` magic name on search rows, export
      `table.ModeText`/layout showIDs etc., delete U9's copies (+ third `armIndex`) [opus]
- U14 `verify.KeyOf`: literal-union word keys quoted per API.md P9 [sonnet]
- U2b `--json-sources` from build/load's loaded-file list with Canon types (number canonicalization) [sonnet]

## Wave 3

- U5b api edit surface: Edit, Op JSON, LockCheck, Open → Recover, drafts; S12, E17–E21, W5, X2 [opus]
- U6 `cli`: explain input fields, `canon refs`, `--watch` [sonnet]

## Wave 4 (gates, written now, run long once at the end)

- U7a API.md rule-coverage test (`API.md <ID>` citations) [sonnet]
- U7b stress (8 readers/1 editor/1 watcher; 2 s in make check, 60 s opt-in), minimal-write fuzz,
  `make bench-edit` (NFR-01 p95s, CPU model printed) [opus]
