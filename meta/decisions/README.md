# Decisions

Two logs, one per phase:

- **The spec phase:** [../../DECISIONS.md](../../DECISIONS.md) at the repository root is the
  decision log of the language and of the rules the implementation obeys (decisions 1–29 and
  on). It wins over every other document (DOCTRINE §2). Implementation agents never edit it; a
  needed change goes to Louis.
- **The implementation phase:** this directory holds the ADRs, one file per durable choice made
  while building the compiler that the spec leaves open (a package split, a harness convention,
  a tooling setup), named `NNNN-slug.md`, numbered in order, never renumbered or reused.

An ADR never overrides the spec; if a choice needs the spec to change, it is a Louis-call in
[../state.md](../state.md) and, once decided, a new entry in DECISIONS.md.

Format: a title `# ADR-NNNN — <subject>`, then Date, Status (`proposed`, `accepted`,
`superseded by NNNN`), and the sections Context, Decision, Consequences. Short; the reasons, not
the narration.

| ADR | Subject |
|---|---|
| [0001](0001-bootstrap.md) | The agent layer: CLAUDE.md, DOCTRINE.md, meta/, .claude/ |
| [0002](0002-api-session.md) | `api/session.go`: the API's glue to `internal/build` |
