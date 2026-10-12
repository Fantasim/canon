# v0.1.6 session start prompt

Paste the block below into a fresh local Claude Code session at `services/configlang`.

```
You are the orchestrator (CLAUDE.md, .claude/rules/orchestration.md). Build and release canon
v0.1.6: items F1-F19 of meta/handoff/2026-10-12-canon-next-versions.md §2, plus the stretch items
F20-F22 if the wave has room. main is at or after e4a0524. Go to the end: the tag and the reply
handoff. Stop only for a philosophy question for Louis (write it to meta/handoff/ and keep going on
what it does not block).

READ FIRST
- CLAUDE.md, meta/state.md, meta/plan.md ("Post-v0.1.0 hardening").
- meta/handoff/2026-10-12-canon-next-versions.md, whole: §1.1 (the generality rule), §2 (the items),
  §5 (refused, and why). §3-§4 are later versions: do not start them.
- The external briefs behind the items. They are read-only, outside this repo:
  - ../../Scripts/meta/handoffs/2026-10-12-canon-lsp-memory-leak.md (F1);
  - ../config-studio/meta/handoff/2026-10-09-canon-findings.md (F2, F6, F16, F17);
  - for F3: meta/state.md "Owed from 2026-10-08 asks", ../../Resource/meta/decisions/0007-*.md
    (near line 100), ../website/meta/handoff/README.md (H-14 #4).

ORDER
1. Spec first, your own work under DECISIONS 207. For every item that changes behaviour, write
   its DECISIONS item and sync the spec before delegating:
   - F3: the E8005 message names its way out (ERRORS.md first);
   - F11;
   - F13;
   - F15: amends DECISIONS 230 and 140, extends 37 and 320, and adds the toolchain pin to SPEC/CLI;
   - F16 and F17: API.md;
   - F18: STDLIB §9.5 and the new builtin;
   - F20: WIRE.
   F5 is yours too. Log every technical call in meta/decisions/log-<date>.md.
2. Wave 1: parallel units, one per package (IMPLEMENTATION-PLAN §5.3), in worktrees, then
   cherry-picked. Suggested units and tiers; adjust if a package boundary says otherwise:
   - LSP (F1, F12): internal/lsp, plus internal/workspace if the retention lives there. Opus.
   - EDIT (F2): internal/edit. Opus.
   - CHECK (F7; F3's cross-root trap and E8005 cases; F15's E8152 in check): internal/check. Opus.
   - GO (F8 Go side; F9; F3's types-mode table constant and dependent-value decoder):
     internal/gen/go. Sonnet, except the dependent-value decoder, which is opus.
   - CPP (F8 C++ side, F10, F14 C++): internal/gen/cpp. Sonnet.
   - TS (F14 TS, F9 TS side): internal/gen/ts. Sonnet.
   - BUILD (F11; F15's unique staging names, writer lock, unowned marker files): internal/build.
     Opus (concurrency).
   - PROJECT (F15's toolchain pin and package-less files): internal/project and its loader. Sonnet.
   - API (F13, F16, F17): api. Opus, under IMPLEMENTATION-PLAN §4's review rule (frozen
     api/canon.go): spec-reviewer plus an ADR.
   - STD (F18): internal/eval/std. Opus.
   - GUIDE (F4, F19, F6): internal/cli/guide/*.md. Sonnet. Every recipe passes `canon check` in an
     examples/ project.
3. Review. spec-reviewer (opus) on every non-trivial diff. On a FAIL, resume the SAME builder, then
   the SAME reviewer.
4. Integrate.
   - Cherry-pick into main, with make check before every `cherry-pick --continue`.
   - After the wave, one cleanup delegation for helpers invented twice (the audit's dup-in-repo),
     then a review.
5. Stretch wave, only once wave 1 is green:
   - F20: WIRE, then the generators;
   - F21: the incremental checker;
   - F22: the gen/go runtime reader.
6. One acceptance pass at the end:
   - long gates: stress; the NFR edit bench when an item touched edit or lsp; the fuzz durations
     an acceptance names;
   - then CLAUDE.md's Definition of Done.

PER-ITEM ACCEPTANCE (highlights)
- F1: a scripted LSP session on a synthetic project the size of Resource. It runs hundreds of
  didChange/didClose with a full re-check per edit, and RSS must stay bounded within a stated
  multiple of the memory after the first full check.
  - Put heap profiles from before and after in the log.
  - State the measured bound in the release notes and the reply.
  - Optional: run `canon lsp` against Resource, read-only, under
    `flock -x ${XDG_RUNTIME_DIR:-/tmp}/sov-canon-heavy.lock systemd-run --user --scope -q -p MemoryMax=12G nice -n 15`.
- F2: the repros of the config-studio findings §1 become txtar cases. No internal error remains:
  an internal error is a bug, never a finding.
- F3: reproduce website H-14 #4 on a synthetic project, then fix it. Each owed item gets its own
  test.
- F9, F10: if verification finds no bug, the test that proves it is the deliverable.
- F15: a test per failure mode, under -race:
  - two builds of one package;
  - a half-written file in a new directory;
  - a toolchain pin that does not match;
  - two packages claiming one output, reported by `check`.
- F16, F17: API tests and an ADR. No existing caller changes behaviour.
- Every new diagnostic code goes into spec/ERRORS.md first, then the registry, with a txtar per
  code.

RELEASE (only when everything above is green)
- Update meta/state.md (80 lines at most), meta/plan.md and meta/handoff/README.md. DECISIONS and
  the spec must be in sync.
- On a clean main, run `make tag VERSION=v0.1.6` (annotated, never pushed), then
  `make dist VERSION=v0.1.6`. dist/checksums.txt goes into the reply.
- Do not push main or the tag: Louis does.
- Do not replace ~/.local/bin/canon. Sovereign's hook pins the sha256 of 0.1.5, and Scripts'
  canon-bump moves the pin after Louis publishes.
- Write meta/handoff/<date>-canon-reply-v016.md for the Sovereign sessions (Source, Scripts,
  website, config-studio). It covers:
  - what changed, per item;
  - F1's measured bound;
  - upgrade notes: the findings the new check may raise (E8152 in check, package-less files);
  - the checksums;
  - what stays owed.

CALLS ALREADY MADE (do not re-ask)
- Technical calls are yours: decide, and log in meta/decisions/log-<date>.md. Only a philosophy
  question goes to Louis, in a file under meta/handoff/.
- The generality rule (§1.1): Sovereign may decide the order of work, never the design. A change
  that only Sovereign's files would need is refused (§5), and the consumer adapts.
- Long gates run once at the end. Builders run their own package tests. make check runs once per
  wave, after cherry-picking.
- Status for Louis goes in a file, with a two-line chat summary.

RULES AND LESSONS
- Memory caps:
  - every agent go test or probe runs as
    `systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local go test -timeout 300s ...`,
    and that command goes in every VERIFY line;
  - at most 2 agents run heavy tests (eval, -race, fuzz) at once;
  - your own make check runs under MemoryMax=6G;
  - temp dirs go in /var/tmp.
- Shared tree:
  - never git stash, checkout -- or reset;
  - parallel units work in worktrees, each fast-forwarded to main first and reporting its base SHA;
  - meta commits go on main only.
- Read-only outside this directory (Resource, Source, Scripts, services). A need there becomes a
  handoff.
- Before telling Louis units are running, check ListAgents. End every delegation prompt with "send
  your final report; do not stop without it".
- Run concurrency tests at GOMAXPROCS=2 and 4 too.
- Every review round finds something: keep one builder and the same reviewer per unit until PASS.
```
