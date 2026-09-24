# State — Canon compiler

Updated: 2026-09-24 (day, autonomous session with Louis intermittently present)

## Current focus

**M1 finishing, M2 started** ([plan.md](plan.md)). M1 items 1–5 met: `canon check teamboard`
equals findings.txt, `canon build --target go,json teamboard sovcommon...` equals the new goldens
(MANIFEST, rebuilt and diffed by `make goldens-check`), canon.lock equals its golden, the generated
module vets. Item 6 postponed (DECISIONS 189). Open before ticking M1: the api/cli review, IR
round 2, Go 1.23 toolchain (not installed here).
**In flight:** IR stage E round 2 (one Go name plan shared by ir and gen/go, E8005/E8011
complete); eval 199 amendment (to commit); M2 `load.dir` of JSON; M2 `gen/cpp` (data mode,
runtime, conformance). **Next:** gen/go data mode + stores + translated fns (after IR round 2),
`conform`, `canon test`, GEN-01 review (handoff), M1.5 program generator (DECISIONS 200).

## What exists

Committed: spec + DECISIONS 1–200; `syntax`, `format`, `jsonsrc`, `wire`, `check`/`types`,
`eval`/`eval/std` + `value` (bounded memory and time, DECISIONS 195/197/199), `verify`, `lock`,
`rules`, `ir` stage E, `gen/json`, `gen/go` baked, `build` (whole pipeline, atomic writes),
`project`, `api` Check/Build, `cli` version/init/new/check/build; `internal/testkit`; `tools/audit`.

## Open Louis-calls

1. **Enable the hooks** (`.claude/README.md` § Hooks).
2. **Review DECISIONS 30–199** made without you; 192 is my reading of your header instruction.
3. **Today's list:** [handoff/2026-09-24-louis-calls.md](handoff/2026-09-24-louis-calls.md):
   spec text to update, rule gaps (free verify walk depth, equality transitivity, double E3501,
   TS reserved words), tooling (pkg-size, citing comments on one line, dead README link).
4. IMPLEMENTATION-PLAN §12.5 is stale (no `meta/`). 5. Absolute-path denies (ADR-0001).
6. `make check-real` lands with M3. 7. Real data: 123 pairs violations, icon letter case.
8. Older calls 6–14 of the previous state (LOD/API, VER, TYP, SYN, LOD decode, FMT, EVL): see
   git history of this file (`git show f89be79:meta/state.md`); none was resolved today.

## Operating notes (today)

- Every agent test runs under `systemd-run --user --scope -p MemoryMax=3G` (an uncapped eval
  probe took 24 GB twice and froze the laptop); a watchdog kills any test process over 5 GB.
- Worktree checks: use a private `GOLANGCI_LINT_CACHE`; the `dead-link` to
  `meta/spec-phase/mockups` is an artifact of untracked files.
- Sonnet units that failed review twice moved to opus (gen/go, ir).

## Size and time (hand-written Go, no testdata/generated/tools)

| Milestone | Budget | Real | Time |
|---|---|---|---|
| M1 remainder | ~5k | +4.6k by 10:29 (69,976 → 74,596), within budget | 08:33 → ~11:00 |

## What could not be verified

Go 1.23 toolchain build of generated Go; GCC 9 / Clang 10 / MSVC; `fixturegen` on real data; CI
on a runner; whether relative `..` permission patterns match.
