# Sovereign: `canon test` / `canon check` memory and time profile (2026-10-09)

From the Sovereign `Source` session (ADR L-0119, Resource in Canon). Paste as is. **Informational,
no deadline**: Louis's ruling 2026-10-09 is that Sovereign never has remote CI; the Canon gate
(`check`, `test`, `build --check`) runs only on the developer's laptop before a push, so its peak
memory and wall time are paid on a shared dev box, next to VS Code, a C++ build and other agent
sessions. Canon will optimise later; this brief is the measured baseline and the ranked ideas.

## Evidence that peak memory matters: the box crashed during this audit

During this measurement session (2026-10-09 ~01:45 EEST) the laptop (30 GB RAM, 22 GB swap,
24 threads) crashed and rebooted. Just before, a whole-project `canon test` (7.5 GB RSS, 48K
major page faults, swap at 22.3/22.9 GB after it) had finished, and this audit's runs (under
`MemoryMax=12G`) overlapped with another session's Canon gate run. One whole-project `canon test`
is 7.7 GB; two at once are ~15 GB, on a box where the editor and the agents already hold ~19 GB. After the crash every Canon command of every
session on this box goes through one global `flock` and a `MemoryMax=10G` (this audit) or `8G`
(the gate) scope, one at a time. That serialisation is now the real cost: in this audit a 10 s
package run routinely waited 5-15 min for the lock behind other sessions' gate runs. Lower peaks
would let the lock go.

## Setup

- Project: Sovereign `Resource/Config` (branch `louis/telemetry-v3-on-test`, commit `bfad03e3`),
  75 packages, 7,961 files, 50 MB; `items` alone is 7,186 entry files (6.3 MB of `.canon`),
  `model` 3.3 MB, `ui` 2.1 MB, `text.client` 1.3 MB; 415 test blocks.
- canon 0.1.3 (`b9d5581`, go1.25.14), default `GOGC`, no `GOMEMLIMIT`.
- Every run: `flock <lock> systemd-run --user --scope -p MemoryMax=10G nice -n 15 /usr/bin/time -v
  canon ...` (12G before the crash), read-only commands, a `git archive` snapshot of the commit so a
  concurrent session's edits could not change the input. RSS = `/usr/bin/time` max resident.
  Wall times carry some noise: other sessions' Go compiles ran outside the lock.

## Numbers

### Whole project

| Command | Peak RSS | Wall | CPU (user) | Note |
|---|---:|---:|---:|---|
| `canon check` | 3.29 GB | 25.4 s | 37.7 s | |
| `canon check` with `GOMEMLIMIT=2GiB` | 1.99 GB | 37.4 s | 63.5 s | live heap fits in 2 GB: 1.3 GB of the default peak is GC headroom |
| `canon build --check` | 3.15-3.21 GB | 37-41 s | 53-57 s | |
| `canon test` (box under swap pressure) | 7.53 GB | 156 s | 216 s | 48K major faults |
| `canon test` (rerun, swap empty) | **7.71 GB** | **139.5 s** | 212 s | |
| `canon test` with `GOMEMLIMIT=3GiB` | 4.29 GB | 264.7 s | **2,498 s** | limit not held: live heap > 3 GB; GC death spiral, 12x CPU |

CPU/wall is ~1.5 for every command: the 24-thread box is mostly idle while the gate runs.

### Per package (scoped)

| Package | Tests | `test` RSS | `test` wall | `check` RSS | `check` wall |
|---|---:|---:|---:|---:|---:|
| `header` (3 KB, floor) | 1 | 0.68 GB | 1.1 s | 0.72-0.75 GB | 0.9-1.9 s |
| `items` | 14 | 1.59-1.61 GB | 12.5-17.8 s | 1.44-1.54 GB | 6.5-6.8 s |
| `model` | 4 | 0.98 GB | 4.0 s | 0.99 GB | 4.1 s |
| `ui` | 13 | 0.88 GB | 3.0 s | 0.84 GB | 2.7 s |
| `text.client` | 1 | 0.73 GB | 2.1 s | 0.81 GB | 2.7 s |
| `balance` | 22 | 1.23 GB | 4.4 s | 1.25 GB | 4.8 s |
| `ai` | 7 | 0.95 GB | 4.0 s | 1.41 GB | 5.8 s |
| `telemetry` | 24 | 1.15 GB | 4.5 s | 1.12 GB | 5.3 s |
| `items.catalog` | 54 | 1.27 GB | 4.1 s | 1.34 GB | 3.9 s |
| `character` | 12 | 0.93 GB | 2.4 s | 0.79 GB | 1.8 s |
| `ai.npc` | 18 | 1.10 GB | 2.9 s | 1.23 GB | 3.9 s |
| `ui.clientui` (imports every text table) | 3 | 1.27 GB | 4.8 s | 1.66 GB | 7.9 s |
| `ui.servernames` | 4 | 1.31 GB | 5.6 s | 1.61 GB | 7.8 s |
| `content.modes` | 12 | 1.27 GB | 5.0 s | 1.56 GB | 6.7 s |
| `progression` | 8 | 1.10 GB | 3.8 s | 1.21 GB | 5.0 s |
| `combat` | 20 | 1.19 GB | 4.8 s | 1.27 GB | 6.5 s |
| `content` | 20 | 1.27-1.29 GB | 6.1-7.4 s | 1.53 GB | 5.4 s |
| `events` | 16 | 1.26 GB | 4.2 s | 1.40 GB | 6.4 s |
| `items.upgrade` | 15 | 1.21 GB | 3.6 s | 1.28-1.38 GB | 4.9-5.2 s |

(Ranges: the same command measured before and after the crash. Some `check` rows after a
concurrent session's edit carried 1-2 errors; their cost matched the clean runs within noise.)

### Cost-structure probes

| Probe | Peak RSS | Wall | Reading |
|---|---:|---:|---|
| `test items --run '^NONE$'` (0 tests) | 0.88 GB | 4.3 s | parse + analysis of `items` and its imports, nothing evaluated |
| `test items --run 'shipped items pass'` | 1.17 GB | 5.3 s | `expect itemCatalog passes`: evaluates the catalog once |
| `test items --run 'whole item table'` | 1.61 GB | 10.8 s | one test = 7 `ItemCatalog { ...c, <one change> }` copies, each fully re-checked |
| `test items --run 'API reads the rows'` | 1.25 GB | 6.9 s | |
| `test items` all 14 | 1.61 GB | 12.5 s | the "whole table" test is ~70 % of the package's test time |
| `test items` minus "whole table" (13 tests) | 1.51 GB | 13.4 s | per-test times are not additive: a shared lazy evaluation is paid once by whichever test touches it first |
| `test items` `GOMEMLIMIT=1GiB` | 1.18 GB | 21.1 s | live heap ~1.1 GB; 4x CPU |
| `test items` `GOGC=50` | 1.48 GB | 20.4 s | barely lower, 2x CPU |
| `test content` text / `--format json` | 1.29 / 1.53 GB | 7.4 / 8.3 s | JSON output +0.25 GB for 20 passing tests (one run each, may be noise) |
| `test items model balance ai telemetry` | 1.86 GB | 28.9 s | ~ the **sum of the five scoped test times** (17+4+4.4+4+4.5) |
| `check items model balance ai telemetry` | 2.01 GB | 17.7 s | below the sum of the scoped checks (~25 s): check shares analyses |

## What dominates (evidence)

1. **Whole-project `test` keeps everything alive: its peak is a SUM, not a MAX.** The heaviest
   single package test is 1.6 GB (`items`); the whole project is 7.7 GB, and its live heap is above
   3 GB (`GOMEMLIMIT=3GiB` could not hold it and spiralled). Whole-project `check` peaks at 3.3 GB
   with a live heap under 2 GB. So `test` holds ~2x what `check` holds for the same 75 packages:
   it seems to keep per-package test state (analyses, evaluated values, copies) for the whole run
   instead of dropping a package's state once its tests are reported.
2. **`test` does not share work across packages the way `check` does.** Five packages: `check`
   costs less than the sum of its scoped runs (17.7 s vs ~25 s), `test` costs the sum (28.9 s).
   Whole project: `check` 25 s, `test` 139 s (5.5x). Since scoped `test X` ~ scoped `check X` for
   almost every package, the gap is not per-test re-analysis inside a package (`items.catalog`, 54
   tests: test 4.1 s = check 3.9 s) but repeated work across packages: each tested package seems
   to re-analyse/re-evaluate its import closure (`items`, `text`, `engine`... are imported by most).
3. **The fixed floor is ~0.7 GB / ~1 s** for any scoped command (a 3 KB package): the whole
   project is parsed (D1 audit: ~0.9 s / 860 MB). Every scoped `canon test X` pays it, so 75
   scoped runs would pay ~75 s and ~50 GB of allocation before any work.
4. **The `items` table (7,186 entry files) drives the heavy package**: 0.88 GB just to analyse it,
   1.44 GB to check it, and one test that spreads the whole catalog 7 times costs 10 s. Packages
   importing `items`/`items.catalog`/every text table (`ui.clientui`, `ui.servernames`,
   `content.modes`, `combat`) all sit at 1.2-1.7 GB scoped.
5. **GC headroom is ~40 % of the check peak**, but `GOMEMLIMIT` alone is not a fix: below the live
   heap it costs 2-12x CPU. The win has to come from a smaller live set.
6. **One core.** CPU/wall ~1.5 on 24 threads for check, build and test.

## Ranked optimisation ideas

Gains are estimates on the numbers above; each has a verification recipe on this project (snapshot
of `bfad03e3`, same wrapper, `/usr/bin/time -v`).

1. **Drop a package's test state after its tests are reported (stream packages).** Analyse a
   package's import closure once (shared, as `check` does), run its tests, print, then release the
   test-only state (spread copies, failure findings, per-test environments) before the next
   package. *Expected*: whole-project `test` peak from 7.7 GB to near the `check` peak plus the
   heaviest package's test delta, ~3.5-4 GB (-50 %). *Verify*: whole `canon test` RSS; and
   `GOMEMLIMIT=3GiB canon test` must finish with CPU/wall ~1.5 instead of 2,498 s CPU.
2. **Share one analysis/evaluation of imported packages across all tested packages** (the `check`
   path already does this). *Expected*: whole `test` wall from 139 s toward `check` 25 s plus
   evaluation of the test bodies, ~40-50 s (-65 %); the 5-package subset from 28.9 s to ~18 s.
   *Verify*: `canon test items model balance ai telemetry` wall <= `canon check` of the same set
   + 10 s; whole-project `test` wall.
3. **Run packages in parallel under a memory budget: `canon test|check|build --jobs N
   --max-memory 6G`** (admission control: start the next package only while the estimated live set
   fits). The box has 24 threads and the gate uses ~1.5. *Expected*: with 1+2 done, `test` to
   ~15-20 s at a bounded peak; and a hard cap means two sessions can gate at once without the global
   lock. *Verify*: `--jobs 8 --max-memory 4G` keeps RSS under 4 GB (MemoryMax=4G scope must not OOM).
4. **On-disk analysis/result cache keyed by package fingerprint** (sources + loaded files + the
   fingerprints of imports + canon version), already noted as owed in the 2026-10-07 reply. A
   package whose fingerprint matches the last green run skips analysis and test evaluation.
   *Expected*: a typical pre-push gate touches 1-5 packages: from 139 s / 7.7 GB to the parse
   floor plus the changed packages and their importers (~5-30 s, ~1.5-2 GB). *Verify*: run `canon
   test` twice; the second run reports N packages cached and takes < 5 s; touch one `items` entry
   file and confirm only `items` and its importers re-run.
5. **`canon test --changed-since <rev>` (or the cache's implicit form)**: test only packages whose
   inputs or import closure changed since the last gated commit. Same gain as 4 without a cache
   store; Sovereign would call it from the pre-push gate with the upstream ref. *Verify*: a commit
   touching only `pet` runs `pet` and its importers, not `items`.
6. **Cheaper parse floor for scoped commands: parse only the import closure of the named packages**
   (not the 7,186 `items` files when testing `header`). *Expected*: `canon test header` from 0.7 GB
   / 1 s to ~50 MB / 0.1 s; every package not importing `items` drops ~0.7 GB. Matters most for the
   agent fix loop and `canon edit`. *Verify*: `canon check header` RSS < 100 MB.
7. **Structural sharing for `{ ...c, field: x }` and incremental re-check of a spread copy.** The
   `items` "whole item table holds" test copies a 7K-entry catalog 7 times and re-checks every
   entry each time (10 s, +0.45 GB over the catalog itself). Re-checking only the checks that read
   the changed field (`reads` is already tracked for findings) would make each `expect ... fails`
   cost proportional to the change. *Expected*: that test from ~6 s of own time to < 1 s; `items`
   test from 12.5 s to ~6 s. *Verify*: `canon test items --run 'whole item table'` wall ~ `canon
   test items --run 'shipped items pass'`.
8. **Compact value representation for large literal tables** (7K `items` entries, `model`'s
   3.3 MB, the text tables: interned strings/field names, packed records, release source text and
   per-token positions after analysis unless a finding needs them). *Expected*: the 0.7 GB floor
   and the 0.9-1.4 GB `items` analysis down 30-50 %; this is the lever for `check` too. *Verify*:
   `canon test items --run '^NONE$'` RSS (today 0.88 GB).
9. **A sane GC default for a batch compiler**: set a soft memory limit from the cgroup/`--max-memory`
   (Go `debug.SetMemoryLimit`) plus `GOGC` around 100-200 only when the limit is far. Alone it
   only trims headroom (check 3.3 -> 2.0 GB at +50 % time); worth doing after 1, when the live set
   is small enough that the limit does not thrash. *Verify*: whole `check` under `MemoryMax=3G`
   with no OOM and wall within +20 %.
10. **`--format json` for passing tests** cost +0.25 GB on `content` in one run; if the JSON path
    buffers all results before writing, stream them. Low priority, measure again first.

## What we did not measure

- Per-test re-analysis inside one package could not be separated from lazy shared evaluation
  (per-test times are not additive in `items`); the cross-package numbers are the evidence above.
- No allocation profile (we do not read Canon's source and canon has no `--cpuprofile/--memprofile`
  flag); a `--profile <dir>` flag writing pprof files would let the next audit point at functions.
- `canon edit` and `canon lsp` memory were not measured.
- Nothing needed more than the 10 GB cap.
