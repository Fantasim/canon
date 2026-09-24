# GEN-01: pipeline golden regeneration, diff and choices

Owner: QA (this unit). IMPLEMENTATION-PLAN §6 M2 acceptance 1, DECISIONS 190: the orchestrator
reviews this diff against sovcommon/Source convention, decides, and M2 continues.

**History:** regenerated with `go test ./internal/testkit/golden -run TestExamples -update` after
wiring the harness to build pipeline for go, cpp and json (view stays unselected: gen/view does
not exist until M3) → refreshed once after loader parity (78b3d7d) and `canon test` (7c00520)
landed → **this revision fixes a review's 4 WARN findings and its NITs** (below); both flagged
items from the first pass (`potion.view.json`, W1701) were already accepted and are unchanged.

## Review fixes (W1–W4)

- **W1 (MANIFEST completeness both ways):** `checkNothingUnlisted` no longer exempts
  `OutputUnchanged`; a new `checkNothingUnwritten` fails when MANIFEST lists a path that is in
  neither `res.Outputs` nor `res.Lock`. `copyProject` replaces `os.CopyFS` and skips every `out/`
  directory while copying `examples/` into the temp project, so a fixture's own `out/` (if ever
  checked in by mistake) can never make a first build report "unchanged" and hide a stale entry.
  Both new failure modes are unit-tested (`TestCheckNothingUnwrittenFailsOnMissing`).
- **W2 (Go-side stale/missing coverage + `-race` in goldens-vet):** `testdata/smoke/pipeline/`
  gained `stale/potions.json` (a fixture with the wrong `$schema`, no real counterpart to copy)
  and `TestStale`, which reloads good data, proves a bad reload is refused naming both
  fingerprints (`potions.PotionsSchema` and the fixture's), proves `Store.Current()` is unchanged
  after both a bad-schema and a missing-file reload. `make goldens-vet` now runs `go test -race`
  (both branches) instead of `go test`.
- **W3 (shared toolchain helper):** new package `internal/testkit/cxx` exports `Toolchain`
  (compiler + nlohmann/json discovery, walked up from the working directory instead of a fixed
  relative depth, so it works from any caller's package directory), `Flags` and `Modes` (data
  only). `internal/gen/cpp/compile_test.go` and its siblings (`calls_test.go`, `imports_test.go`,
  `parity_test.go`) now call `cxx.Toolchain`/`cxx.Flags`/`cxx.Modes`/`cxx.Timeout` from their own
  `buildAndRun`, instead of each keeping its own compiler list, flags and nlohmann search.
  `internal/testkit/golden/cpp_test.go` does the same. **Note:** the actual `exec.CommandContext`
  compile-and-run loop stays duplicated between the two `_test.go` files rather than also moving
  into `cxx` — `internal/gen/cpp/decode.go`-style production code has never shelled out anywhere
  in this repo, and doing so from a non-test file put two gosec findings (G204) and the ratchet's
  empty security/ignore-count baselines in direct conflict with no honest way to fix or suppress
  them; keeping the actual subprocess calls in `_test.go` files (exempt from golangci-lint's
  `tests: false`) avoids the conflict entirely while still sharing the part that was actually
  duplicated (the toolchain search). Flagged for the record, not asked about, since the fix was
  clean and needed no exemption.
- **W4 (both exception modes):** `TestPipelineCppCompiles`'s `buildAndRun` now loops over
  `cxx.Modes` (plain, then `-fno-exceptions`) for every compiler found, as gen/cpp's own tests do.

## NITs

- `canon_runtime.h` **keeps its own `#ifndef CANON_RUNTIME_RT_V1`/`#define` version guard**
  (corrected: only its multi-line comment dropped to the bare marker — the runtime header is
  shared by every output directory using rt_v1, so it cannot switch to `#pragma once`, DECISIONS
  193's reason). `pipeline.gen.h` is the one that switched to `#pragma once`.
- `go/potions.gen.go`'s diff also includes `dir+"/potions.json"` replacing
  `filepath.Join(dir, "potions.json")` in `LoadPipelineSnapshot`, and a `if f.Rows == nil { return
  rt.Missing(...) }` check in `loadPotions` before unmarshalling rows (both loader-parity changes,
  78b3d7d).
- `Makefile`'s `goldens-check` now ties the `potion.view.json` exception to its exact path
  (`examples/pipeline/expected/potion.view.json`), not a bare `-name` filter that would also
  exempt an unrelated file of the same name elsewhere.
- `make goldens-vet` now copies an example's own top-level `expected/*.json` (pipeline's
  `potions.json`) into the smoke module's `data/` itself; the hand-kept copy under
  `testdata/smoke/pipeline/data/` is deleted.
- `cpp_test.go` reads `$schema` out of the golden `potions.json` at test time
  (`potionsSchema`) instead of a hardcoded copy of the fingerprint; only the deliberately-wrong
  `staleSchema` stays a literal (it names no real schema).
- Every compile-and-run driver (`buildAndRun` in both `_test.go` files) now runs the compiled
  binary under a `context.WithTimeout(cxx.Timeout)` too, not only the compile step.
- The dead `exampleSkip` map (empty since the first pass) and the `if exampleSkip[name] {
  continue }` line are removed from `examples_test.go`.
- `buildManifest` now fails (`errDuplicateGolden`) when two display paths would reduce to the
  same golden name, instead of silently letting the second overwrite the first's file
  (`TestBuildManifestFailsOnDuplicateGolden`).

## Files touched by this revision

`internal/testkit/golden/examples_test.go`, `constants.go`, `errors.go`, `cpp_test.go`;
`internal/testkit/cxx/` (new: `doc.go`, `constants.go`, `cxx.go`, `example_test.go`);
`internal/gen/cpp/compile_test.go`, `calls_test.go`, `imports_test.go`, `parity_test.go`
(switched to `cxx`, test files only, no production package touched); `Makefile`;
`internal/testkit/golden/testdata/smoke/pipeline/smoke_test.go`, `stale/potions.json` (new);
`testdata/smoke/pipeline/data/potions.json` deleted (Makefile copies it now).

## Everything from the earlier passes, unchanged

`examples/pipeline/expected/**`'s content diff (headers to DECISIONS 192/193, strict decoding
and `ParseDataFile`/loader parity to 78b3d7d, move-only containers to the gen/cpp round-3
amendment), the `potion.view.json` exclusion (DECISIONS 190/this unit) and the dropped W1701
warning (accepted as a known M3 gap) are all as the previous revision of this document described;
see git history of this file for the full file-by-file breakdown, unchanged by this pass.

## Verify (this revision)

- `go build ./...`: clean.
- `systemd-run … go test -race ./internal/testkit/... ./internal/gen/cpp/...`: all packages pass,
  including the new `internal/testkit/cxx` and the extended `smoke/pipeline` (`TestSmoke`,
  `TestStale`) and `internal/testkit/golden` (`TestPipelineCppCompiles` now compiling both
  exception modes, `TestExamples`, the new MANIFEST-symmetry tests).
- `make goldens-vet`: both golden modules vet and `go test -race` clean, including both smoke
  tests, with `data/potions.json` now materialized by the Makefile rather than hand-copied.
- `make goldens-check`: clean.
- `gofmt -l` / `go vet` on the touched packages: clean.
- `tools/audit check --repo ../..`: whole-repo **PASS, 0 new findings** (one stale
  `golangci-lint` cache entry pointing at an unrelated scratchpad path under `/tmp` had to be
  cleared first — `cd tools/audit/toolchain && go tool golangci-lint cache clean` — the known
  remedy already recorded in meta/decisions/log-2026-09-24.md's "Repository and tooling" section;
  not this unit's own finding).

## What could not be verified

MSVC and GCC 9/Clang 10 stay unverifiable here (only current g++/clang++ installed), per
meta/state.md. The tree had other agents' concurrent, uncommitted edits to `internal/ir`
throughout this session (as flagged going in); every verification above was captured at a moment
the tree built cleanly.
