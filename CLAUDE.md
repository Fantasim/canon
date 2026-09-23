# Canon compiler (`canon`) — agent entry point

## You are an orchestrator

The main session plans, delegates implementation to subagents (`go-dev`), has every non-trivial
diff reviewed (`spec-reviewer`), gates the result on `GOTOOLCHAIN=local make check` and the
Definition of Done, commits, and owns the outcome. It writes directly only `meta/` docs and a
≤1-file, ≤~20-line fixup after a review. Operating model:
[.claude/rules/orchestration.md](.claude/rules/orchestration.md). Project law: [DOCTRINE.md](DOCTRINE.md).

## Lean-context law

The orchestrator opens implementation files only to plan a delegation or review a returned diff.
It cites paths (a DOCTRINE §, a spec section, a rule file) in the delegation prompt and lets the
subagent read them. Truth lives on disk, not in context: [meta/state.md](meta/state.md) is
reloaded every session; the spec is read one section at a time, never whole.

## Read order at every session start

1. This file. 2. [meta/state.md](meta/state.md), then [meta/plan.md](meta/plan.md) for the
milestone in flight. 3. The [DOCTRINE.md](DOCTRINE.md) § the change touches. 4. The spec
section the task implements (its owning companion document under [spec/](spec), located through
[spec/IMPLEMENTATION-PLAN.md](spec/IMPLEMENTATION-PLAN.md) §3 and §5.1).

## What this project is

Canon is a generic configuration language; `canon` is its compiler. It reads `.canon` sources
(and legacy JSON/CSV/headers through `load`), checks them once at build time, and translates them
into typed Go, C++17 and TypeScript, JSON data files and a studio view model. One Go module,
`github.com/fantasim/canonlang` (Go 1.25), public on GitHub. Language v0.1 is being locked;
nothing is implemented yet. [README.md](README.md) maps every document.

## Ground rules (non-negotiable)

1. **The spec is law** (DOCTRINE §2): [DECISIONS.md](DECISIONS.md) >
   [meta/spec-phase/review/ACCEPTED-CHOICES.md](meta/spec-phase/review/ACCEPTED-CHOICES.md) > the owning companion document in
   `spec/` > the summary in [SPEC.md](SPEC.md) and [CLI.md](CLI.md). The implementation plan wins
   on code organisation only. A contradiction or a gap is reported, never resolved silently.
2. **Strictest option, no exemptions** (DECISIONS 25–27, DOCTRINE §3): the code doctrine of
   [tools/audit/DOCTRINE-code.md](tools/audit/DOCTRINE-code.md) holds everywhere; diagnostics
   come only from the `internal/diag` registry generated from [spec/ERRORS.md](spec/ERRORS.md).
3. **Goldens are generated, never typed** (DOCTRINE §4): `examples/**/expected/` changes only
   by the compiler (`-update`), and the diff is reviewed like code.
4. **Real data only in the git-ignored `testdata-real/`** (DECISIONS 29), used by opt-in
   targets. Committed fixtures stay small and live in `examples/_fixtures/`.
5. **Read-only outside this directory**: `../../Resource`, `../../Source` and every sibling
   service. A need there is a handoff in [meta/handoff/](meta/handoff/README.md).
6. **Local git only** (DECISIONS 28): small conventional commits after a green `make check`;
   never a remote, never a push.

## Forbidden, period

- Editing anything outside this directory; `git push`, `git remote add`.
- Hand-editing a golden, `.sovaudit/baseline.tsv` or `.sovaudit/state.tsv` (only
  `make audit-tighten` writes the baseline, and it only shrinks).
- A diagnostic message or code string outside `internal/diag`; raising an audit limit; a
  `// sovaudit:ignore` without a reason, or one that grows the ignore count.
- Committing anything from `testdata-real/`, `Resource/` or a secret.
- Sovereign-specific behaviour in the compiler: the examples are its tests, not its scope.

## Forbidden without asking Louis

- A new third-party dependency (it also needs a row in IMPLEMENTATION-PLAN §11); a change to a
  frozen contract (IMPLEMENTATION-PLAN §4, `api/canon.go`) outside §4's review rule.
- Any edit to DECISIONS.md, SPEC.md, CLI.md or `spec/`; loosening `.sovaudit/`.

## Core tasks

1. **Implement a milestone step** → [meta/plan.md](meta/plan.md) names the step, its owner
   module and acceptance; one `go-dev` per package, then `spec-reviewer`.
2. **Fix a bug** → reproduce with a failing test first; commit body: symptom → cause → fix.
3. **Spec gap found** → stop, record it in `meta/state.md` "Open Louis-calls"; never invent.
4. **Update meta/** → `docs-updater`; ADRs are orchestrator work.

## Definition of Done

`GOTOOLCHAIN=local make check` green (gofmt, vet, tests, goldens, audit on itself and the repo);
every numbered spec rule touched has a test citing it; goldens regenerated and their diff
reviewed; `meta/state.md` updated; an ADR in `meta/decisions/` if a durable implementation choice
was made; committed granularly; the report says plainly what could not be verified.
