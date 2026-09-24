# State — Canon compiler

Updated: 2026-09-24 (day, autonomous session with Louis intermittently present)

## Current focus

**M1 finishing, M2 started** ([plan.md](plan.md)). M1 items 1–5 met: `canon check teamboard`
equals findings.txt, `canon build --target go,json teamboard sovcommon...` equals the new goldens
(MANIFEST, rebuilt and diffed by `make goldens-check`), canon.lock equals its golden, the generated
module vets. Item 6 postponed (DECISIONS 189). Open before ticking M1: the api/cli review, IR
round 2.
**In flight (uncommitted, one agent each):** M2 `load.dir` review fixes (7 critical); build
API/report (201) fix verification; spec sync to DECISIONS 188–207 under review; M2 `gen/cpp`;
M1.5 generator foundation (206). **Landed today:** `conform` (ddb6fd6; not wired), IR round 2
(5ac1d47; `ir.GoWords` export rides with gen/cpp). **Next:** gen/go data mode + stores + translated fns (after IR round 2),
`conform`, `canon test`, GEN-01 review (handoff), M1.5 program generator (DECISIONS 200).

## What exists

Committed: spec + DECISIONS 1–200; `syntax`, `format`, `jsonsrc`, `wire`, `check`/`types`,
`eval`/`eval/std` + `value` (bounded memory and time, DECISIONS 195/197/199), `verify`, `lock`,
`rules`, `ir` stage E, `gen/json`, `gen/go` baked, `build` (whole pipeline, atomic writes),
`project`, `api` Check/Build, `cli` version/init/new/check/build; `internal/testkit`; `tools/audit`.

## Open Louis-calls

None. Technical gaps and the calls made on them: [decisions/log-2026-09-24.md](decisions/log-2026-09-24.md)
(the orchestrator decides, never asks, DECISIONS 207). Spec sync to DECISIONS 192–207: pending.

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

GCC 9 / Clang 10 / MSVC; `fixturegen` on real data; CI
on a runner; whether relative `..` permission patterns match.
