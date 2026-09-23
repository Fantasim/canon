# ADR-0002 — `api/session.go` holds the API's glue to `internal/build`

- Date: 2026-09-24
- Status: accepted (autonomous night session; Louis reviews)

## Context

IMPLEMENTATION-PLAN §12.4 lists the files of `api/`; none is meant for the code shared by the
calls that M1 implements: making paths absolute, the closed check, the revision cache, the
mapping of `build`'s errors to API.md §15, panic recovery (X2) and the conversion of findings.

## Decision

These helpers live in `api/session.go`, unexported; the exported surface stays in the files
§12.4 names (`canon.go`, `findings.go`, …). No file is renamed.

## Consequences

§12.4's table gains the row at its next edit (Louis). When `workspace` lands (M4), the glue
moves there and `session.go` shrinks to the error and finding conversions.
