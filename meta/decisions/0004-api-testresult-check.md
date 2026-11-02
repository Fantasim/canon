# ADR-0004 — `api.TestResult` carries the static check findings

- Date: 2026-09-24
- Status: accepted (orchestrator, DECISIONS 207; frozen-contract change under IMPLEMENTATION-PLAN
  §4's review rule: owner builder + spec-reviewer; consumers: internal/cli)

## Context

`canon test` runs phases 1–2, then the tests (EVALUATION §1). The decisions log ("canon test (M2)")
requires every static error of the loaded packages to be printed, in `canon check`'s format, and
to make the run exit 1. API.md §13.2's `TestResult` has no field for findings, and `Project.Check`
runs phases 1–7, which is not what `canon test` runs.

## Decision

`api.TestResult` gains `Check *CheckResult` (as `BuildResult.Check`): the error findings of
phases 1–2 for the loaded packages and project.canon, with their summary. `api.ExpectFailure`
gains `Outcome`, `Op`, `Poisoned` and `Cause`, so the CLI renders its report words from
structured fields (decisions log, "canon test review calls"). An unknown layer is a
`*ProjectError{Err: ErrUnknownLayer}` carrying its `E1901` findings. Nothing else changes. API.md §13.2 gains the matching rule at the next spec sync.

## Consequences

The CLI prints those findings first, then the test report, and exits 1 when any static error or
test failure exists. Other API consumers (workspace, LSP) read the same field.

Amended (v0.3, DECISIONS 449): `canon test` reports its own static findings as `check` does
(CLI.md §3.5; API.md §13.2; amends ADR-0004). It prints the errors of phases 1-2 of every loaded
package (as before) and the warnings of the selected packages, its file-read warnings (`W7101`)
included, before its report, honours `-q` and `--max-warnings` (exit 4), and `TestResult.Check`
carries them. Reason: it printed no warning, `W7101` included, and ignored the global
`--max-warnings`.
