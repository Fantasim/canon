# canon audit

The project's strict code audit: compiler-style feedback on size, magic values, errors, API
surface, dead code, duplication, comment budget, idioms and the repository's layout. Rules and
their reasons: [rules.md](rules.md). The doctrine contributors follow:
[DOCTRINE-code.md](DOCTRINE-code.md).

A separate Go module (`github.com/fantasim/canonlang/tools/audit`, Go 1.25): nothing imports it,
and the compiler's module does not depend on it. Run it from this directory; from the repository
root, `make audit` and `make check` do it for you.

## Commands

    go run . audit    --repo ../..                     # census: every rule, zeros included
    go run . audit    --repo ../.. --rule fn-length    # one rule, every finding
    go run . check    --repo ../.. [--changed [--base REV]]   # the gate, exit 1 on failure
    go run . baseline --repo ../.. --init | --tighten
    go run . rules                                     # the rulebook

## Modes and the ratchet

Each rule is `enforce`, `ratchet`, `observe` or `off` (rulebook default, per-repo override in
`.sovaudit/state.tsv`). `baseline --init` records every existing finding in
`.sovaudit/baseline.tsv` (and demotes an enforce rule that already has findings to ratchet in
that repo, so a first init is never red). `check` then fails on:

- any finding of an `enforce` rule;
- a `ratchet` finding the baseline does not hold, or one whose measured value grew.

`.sovaudit/root-allow.txt` lists the repository's intentional root entries (one per line) that
the `root-clutter` allowlist does not know. `.sovaudit/imports.tsv` declares import boundaries
(`<from-dir>\t<forbidden-import>`, `path.Match` patterns, `/...` for a subtree).

A finding's identity is `rule | file | symbol | detail`, never a line number, so moving code
does not make it new. `baseline --tighten` lowers the baseline after a cleanup and never raises
it; `baseline-guard` fails a baseline or state that was loosened against the base revision
(git only: outside a git checkout the guard and `decision-dropped` have nothing to compare).

## What is audited

Every tracked Go and Markdown file of the repository, minus `vendor/`, `testdata/`,
`examples/_fixtures/`, every `expected/` tree under `examples/` (generated goldens), nested
modules (this one is audited on its own), and generated Go files (`// Code generated ... DO NOT
EDIT.` or a `.gen.go` name).

## Lanes

| lane | what | engine |
|---|---|---|
| gostyle | size, Go comments, TODOs, doc.go and Example per package | go/ast |
| gorules | magic values, constants, errors, exported-but-local, idioms, stdlib helpers | go/ast + go/types |
| stock | complexity, errors, dead code, dup, fmt, naming, security | golangci-lint v2, deadcode |
| project | dead Markdown links, root clutter | own |
| integrity | baseline/state loosening, dropped decisions, ignore hygiene | own |

The stock lane's tools are pinned in [toolchain/go.mod](toolchain/go.mod) as `tool`
dependencies (golangci-lint v2.12.2 and x/tools deadcode, both building with Go 1.25) and
configured by [toolchain/golangci.yml](toolchain/golangci.yml). The first run builds them
(well under a minute); a tool that cannot be built skips its rules and the report says so: a skipped
rule is never a silent zero.

## The tool gates itself

`make check` at the repository root runs this module's vet and tests, then `check` on its own
code against [.sovaudit/baseline.tsv](.sovaudit/baseline.tsv), then `check` on the repository.
A change to the tool meets the same rules it enforces.

## Adding a rule

Add it to `internal/rules/rules.go` (and a limit to `internal/rules/constants.go`), to
`rules.md` (a test holds the two equal, order included), and to exactly the lane that produces
it.

## Naming

The on-disk format (`.sovaudit/`, `// sovaudit:ignore`, `sovaudit:decision-obsolete`) keeps the
name of the tool this one was derived from, so its baselines and directives read the same.
