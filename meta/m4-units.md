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

## Wave 1 (staging branch `m4-wave1`, gate running)

- [x] U1 `format` + `jsonsrc`: §6.3, §13 `Rewrite`/`Node`/`Flat`/`Fresh`, §14.2 JSON edits [opus] — r3 PASS (4205c10)
- [x] U2 `cli`: `canon fmt` (`--check`, `--diff`, `--json-sources` static interim) [sonnet] — PASS (58e6ed4)
- [x] U3 `workspace` core + `api` rewire (S1–S11, O6, O7, overlays) [opus] — r2 PASS (19eab65)
- [x] U4a `edit` foundations: Operation, Lit, V1–V4, codec E24–E26, Refs R7/R8; `wire.Decoder.Keep` [opus] — r3 PASS (f4ebe30)
- [x] U9 `views/live`: Evaluate content V5–V12 [opus] — r4 PASS (6255868)
- [x] U10 `check.Session`/`Recheck`; FileID-free ordering in check and diag [opus] — r2 PASS (c6b543b)
- [x] U11 `eval.Memo` [opus] — r2 PASS (6561ff7)
- [x] U12 `project` parse reuse [sonnet] — PASS (a2c3bbc)

## Wave 2 (file ownership: disjoint; launched after wave 1 reaches `main`)

- U4b `edit` apply — owns edit `apply*.go`, `print*.go`, `splice*.go`, `cascade.go`, `undo.go`,
      `place.go`, `testdata/edits/`, and the codec's Source marshal: E1–E16, E22–E23, M1–M9, N1–N8
      (M6 asserted on every case); `format.Flat` into E26; U4a leftovers (computed-ref span, cover
      guard, History-cost test, a default reading a field `Keep` left nil); M9 on raw bytes (CRLF) [opus]
- U4c `edit` commit — owns edit `journal.go`, `commit.go`, `recover.go`: N9–N12, O5, crash test
      (acceptance 5); journal named by the revision's hex [opus]
- U5 watch — owns `workspace/watch*.go`, `api/watch.go`: W12–W16 [opus]
- U5a api reads — owns `api/evaluate.go`, `api/value.go` (Refs), `api/build.go` (Format,
      FormatJSONSource), `workspace/evaluate.go`: V4a–V12 without drafts, R7/R8 through the api, T1,
      T2; surfaces `Analysis.ViewErr` as X2 [opus]
- U8 `build` memo integration — owns `internal/build/*` except `live.go`, and `internal/diag`
      (`Builder.Detached`, U11 item 7): `build.Cache` (parse reuse + check.Session + eval.Memo under
      the epoch rule), load memo, `findLoad` walk caching, LockUpdates, LockCheck (B4), atomic
      fsynced OS WriteFile, bound the Build pin cost (U3 N1); the incremental ≡ cold property test
      (one edited entry, the rest replayed) [opus]
- U13 `rules` + `verify` — record-check memo; per-file caching of `rules.NewIndex` and
      `verify.NewIndex`; U14's items: `verify.KeyOf` quotes literal-union word keys (P9),
      `verify.substitute` handles `*types.Refined` (TYPES §11.2) [opus]
- U9b `views/render` + `eval` (view-named methods) + `build/live.go`: fill `live.Lines` and
      `Input.Bound`; `index` magic name on search rows; export `table.ModeText`, layout showIDs;
      delete U9's copies and the third `armIndex` [opus]

## Wave 3

- U2b `--json-sources` from build/load's loaded files with their Canon types (number
      canonicalization); `fmt --diff` through `go-udiff` (§11), deleting cli's own LCS [sonnet]
- U5b api edit surface: Edit, Op JSON, LockCheck, Open → Recover (+ OS `Alive`, ErrJournal fails
      Open), drafts (in the shared key); S12, E17–E21, W5, X2; one exported element naming in `live`
      used by `workspace` (plain-list copies → `#<n>`); unify edit/workspace stale sentinels [opus]
- U6 `cli`: explain input fields, `canon refs`, `--watch` [sonnet]

- Cleanup: P9 literal-union keys in `eval`'s paths (`path.go` keyText) and live `colls.go`
      (`verify.MapKey`); one `value.ArmIndex` for live/verify/wire; live's dependent-type resolution vs
      verify's `substitute` (+ DepMapType, AppliedRecord args); live's `layoutOf`/`keyed`/`fieldOf` vs layout/render/encode [sonnet]

## Wave 4 (gates, written now, run long once at the end)

- U7a API.md rule-coverage test (`API.md <ID>` citations) [sonnet]
- U7b stress (8 readers/1 editor/1 watcher; 2 s in make check, 60 s opt-in), minimal-write fuzz,
  `make bench-edit` (NFR-01 p95s, CPU model printed) [opus]
- Final pass (orchestrator, once): 10-min `FuzzFormat`/`FuzzFormatJSONSource`/`FuzzRewrite`, minimal-write
  fuzz, 60 s stress, NFR-01 bench, `-format.full`, `check-real`, then CI on `claude/m4-ci`.
