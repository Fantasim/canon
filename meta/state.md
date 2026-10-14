# State — Canon compiler

Updated: 2026-10-14. **v0.1.8** (performance pass, 371-373) tagged and pushed 2026-10-14; v0.1.7 (TS2590, 370) to v0.1.0 out; `main` is
the only branch. DECISIONS 344-369: [log-2026-10-12](decisions/log-2026-10-12.md),
[log-2026-10-13](decisions/log-2026-10-13.md), [ADR-0020](decisions/0020-api-016-overlay-ctx-call-lang-roots.md)
(earlier: 304-343, ADR-0018/0019, logs 10-06, 10-08, 10-09). M5 accepted (CI run 36973712977).

## Current focus

**v0.1.6 shipped F1-F19** (the Resource review's [next versions](handoff/2026-10-12-canon-next-versions.md)
§2; [reply](handoff/2026-10-13-canon-reply-v016.md)): LSP releases a project on its last close (368);
`ProjectInfo.Roots`, `ValueWith`/`Edit.Lang`, overlays take a ctx (344-346, breaking, ADR-0020); F3 (347-350)
consumer-copy `E8004`, shared string id types, dependent value through a ref; F15, F11 (351-357) unique
staging, per-package writer locks, `compiler:` pin, package-less files, `E8152` in check, stray generated
files, case-only text rename; F7, F8 (360, 361, 364) no repeated `E3305`, every text decoder refuses
repeated keys and names the first failure in document order; F14 (362, 363, 365, 369) C++/TS open enums and
`@text` decoders, exact owner hooks, `E8019`, TS `findBy<F>`; F2 (366) reserved-word members in edits; F18
(358, 359) format specs, `duplicates()`; F4, F19, F6 guide topics; F12, F5; F9, F10: no bug. Deferred: F20
(per-field `@json(codes)`), F21 (warm re-check), F22 (streaming JSON reader). Pre-tag: E8152's json names
planned, not encoded (2.1× edit regression gone), M6 oracle joins, killed-build wedge (336).
**Next: v0.1.9, the robustness release**, decided whole in [ADR-0021](decisions/0021-v019-robustness-release.md) (DECISIONS 374-382, synced first; run as its §7 says), then v0.2 (N5 first). Open too: [Resource structure](handoff/2026-10-12-canon-to-sovereign-resource-structure.md);
[all handoffs](handoff/README.md). Release builds stay local (`make dist`) while CI is down.
**Performance effort (2026-10-14, [log](decisions/log-2026-10-14.md)):** halve `check`/`build` memory and time; token store done (371, −15-17% RSS); lambda captures not copied (−20% allocs per call); Resource `tidCode` handoff sent (−35% `check`); enum by-name lookup, `Prov`, parallel packages next. Robustness first: the stat-trust refresh fix and a safe on-disk cache are documented, not started ([log](decisions/log-2026-10-14.md)). **Owed, performance:** per-entry deps (an `items` edit re-checks 30 importers); stage-E memo; the §2.7
on-disk cache (a `dev` compiler must key by commit); go/cpp/ts emits sharing a dir generate for names; edit time is ops × a full analysis; FMT's whole-file
judgement holds the edit peak; F22 streaming JSON reader for Go loaders (~110 ms/MB); F21 warm re-check
on entry add/remove.
**Owed, small:** transient `ErrJournal` for two processes editing different packages (ADR-0010); `Recover`
at Open takes no writer lock; a case-only rename of an `out` directory is `E8001`; the asset kind test
follows links unbounded (WIRE §6.5); a layer or new entry file cannot name another package's non-nameable
member (`E2102`). Not in v0.1.9 ([ADR-0021](decisions/0021-v019-robustness-release.md) §9): `load.dir`
skips a Windows junction silently (no `W7115`) and does not bound a path through one; single-file `load`, `load.text`, `load.csv`, `load.defines` read through any link; Windows refresh (re-read notified names, drop a `*StaleError`'s entries); the cache's stat table must not trust size + mtime; a `@text` output named `.canon`/`.json` over an unmarked file (unverified); enums opaque in v0.2 (Go, C++ nlohmann, TS `<E>Index`); the studio's full reload has no target; `evalAnalysis` never calls `KeptSelection`.
**Documented limit:** `NoHook` is filled for the import closure only, the only packages read (ADR-0021 §2.5).
**Owed for v0.2:** dependent types kept through static typing (E3804 on dependent-map `union`); an amend
path into a dependent map (E1905); Go and C++ `embedded`; legacy structs (M6); entry isolation and input
defaults (321); `ordered_json`, kind constants. Fuzz/progen campaigns deferred.
Environment: `internal/workspace` OS watch tests skip when inotify watches run out (raise
`fs.inotify.max_user_watches`). Cloud: `apt-get install libc++-18-dev libc++abi-18-dev`, `npm ci --prefix
tools/tsc`; no systemd (memory caps by hand under `ulimit -v`).

## Milestones

M0-M5, post-M4, M1.5 done (3 GB cap unverified). M6: TS, C++ baked done; legacy C++ waits (Louis). M7 not started.

## What exists (committed)

spec + DECISIONS 1-373; `syntax`, `format` (+ §13 `Rewrite`), `jsonsrc` (+ §14.2 edits), `wire`, `load` (every
WIRE §6 form), `check`/`types` (dependent types, views, translations, broken-view tracking), `eval`/`eval/std`
+ `value` (layers, provenance, drivers across the project), `verify`, `lock`, `rules`, `ir` (stage E,
fingerprint, name plans, pattern automaton, `ir.CopyOf`), `gen/json`, `gen/ts` (four modes), `gen/go` (baked,
data, `types`), `gen/cpp` (data, `types`), `views`, `i18n`, `gen/view`, `conform`, `build`, `project`,
`check.Session`, `eval.Memo`, `workspace`, `views/live`, `edit` (+ ops, typing, codec, Refs), `api` over
workspace, `cli` (version/init/new/check/build/test/explain/fmt/guide/help), `internal/testkit`, `tools/audit`.

- Multi-destination emits (ME1, `051c3b7`): `examples/features/copies`. M4.1 (`5541eab`): verified
  multi-op edits and Undos; see [m4-units.md](m4-units.md), [plan.md](plan.md).

## Open Louis-calls
None blocking. [The M4 handoff](handoff/2026-10-01-m4-complete.md)'s (a) O2 and (c) refresh target are in
v0.1.9 (ADR-0021), (b) `check.Info` layered maps in v0.2 (log-2026-10-14). The v0.2 order after N5 is Louis's.

## Operating notes

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G`; temp dirs in `/var/tmp`.
- Agents share one working tree: never `git stash`; parallel units run in worktrees, cherry-picked
  after `make check`. `GOLANGCI_LINT_CACHE` is private per worktree; the audit fails on an
  unmeasured lane. Review rules: `.claude/rules/orchestration.md`.
- At most two agents run heavy tests (eval, `-race`, fuzz) at once (a session-local semaphore did it).

## What could not be verified

NFR-01 measured on this local machine only, not a 4-core CI runner, quiet only on `f53ba0c`; at equal load
on the final code (no regression; Louis accepted it); not re-run after M4.1's Undo verification. macOS/
Windows link tests skip where links cannot be made. Long fuzz/progen campaigns deferred by Louis. The v0.1.2
"one-value edit ~2 s" figure was not re-run. v0.1.3: edit memory measured on synthetic tables only.
v0.1.6: Windows and macOS lock and rename behaviour (vet only); a real editor day on Resource for F1 (the
bound is on a synthetic Resource-shaped project); Go types decoding on Sovereign's real files (synthetic
only); edit p95 0.32 s / evaluate 0.24 s miss 0.30 / 0.15 as v0.1.5 does (load 2-10).
## Verify queue
Re-run `make bench-edit` on a quiet machine or a CI-class runner when one exists.
