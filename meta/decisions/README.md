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
| [0003](0003-eval-conformance-api.md) | The evaluator's conformance API: test calls, vectors, TS mode |
| [0004](0004-api-testresult-check.md) | `api.TestResult` carries the static check findings |
| [0005](0005-wire-host-dependent-decoding.md) | `wire.Decoder`/`wire.Host` grow for dependent types in loaded data |
| [0006](0006-vm-structs.md) | `api/vm`: view-model structs generated from the schema |
| [0007](0007-api-origin-chain.md) | `api.Origin` gains the amendment chain, its text and cut frames; `ErrInputField` |
| [0008](0008-ir-field-patterns.md) | `ir.Field.Patterns`: every pattern of an input's alias chain |
| [0009](0009-check-info-broken-views.md) | `check.Info.BrokenViews`: which views hold an error |
| [0010](0010-edit-journal-recovery.md) | The edit journal: crash recovery and its threat model |
| [0011](0011-incremental-memo.md) | The incremental memo: one store, epochs, lineages |
| [0012](0012-formatter-region-settle.md) | Rewrite settles one section; the layout verdict is kept per tree |
| [0013](0013-rename-whole-file-layout.md) | A name rename splices tokens and lays the whole file out |
| [0014](0014-cpp-baked-and-text-files.md) | C++ baked data, constexpr lookups and text-file ownership |
| [0015](0015-past-types-name-retired-members.md) | `past E`: a type whose slots may name retired members |
| [0016](0016-past-ergonomics-implementation.md) | How `past`, poisoned-value edits, let-path refs and `@text` JSON are built |
