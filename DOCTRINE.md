# DOCTRINE — Canon compiler project law

Cited by § from `CLAUDE.md`, `.claude/` and delegation prompts. It points at the spec, never
restates it. A change here is Louis's call and an ADR in `meta/decisions/`. ≤ 80 lines.

## §1 Identity and scope

Canon is a **generic** configuration language and `canon` its compiler: pure, build-time only,
always finishes (SPEC.md §1). It serves any project; Sovereign is its first user, and the examples
rewrite Sovereign files only because real files make honest tests. No game name, path, root or
convention is hard-coded in the compiler: everything a project needs is declared in its
`project.canon`. The public repository is `github.com/fantasim/canonlang`.

## §2 The spec hierarchy

1. `DECISIONS.md` wins over everything. 2. `meta/spec-phase/review/ACCEPTED-CHOICES.md` wins over every other
document. 3. The companion document in `spec/` that owns the subject (its diagnostics table owns
its codes, `spec/ERRORS.md` lists them all). 4. The summaries in `SPEC.md` and `CLI.md`.
`AUDIT.md` answers are accepted unless a document above overrides them.
`spec/IMPLEMENTATION-PLAN.md` wins on code organisation (layout, packages, owners, milestones)
and loses on behaviour. Code implements a numbered rule and cites it (`// TYPES.md T12`); a test
names the rule it proves. A contradiction, an ambiguity or a missing rule stops the work and goes
to Louis through `meta/state.md`; implementing a guess is a review FAIL.
A ban states its reason and its way out, or why none is needed (DECISIONS 305).

## §3 Code doctrine and strictness

The rules are [tools/audit/DOCTRINE-code.md](tools/audit/DOCTRINE-code.md) and
[tools/audit/rules.md](tools/audit/rules.md), applied as IMPLEMENTATION-PLAN §12 says, gated by
`make check`. DECISIONS 25–27 make them absolute: the strictest technically sound option, no
exemptions, only 0 and 1 bare, table-driven dispatch in the lexer and parser, thresholds as data,
the ignore count ratcheted toward zero, and diagnostics as catalogue data (`diag.E3501.At(...)`,
no message text outside `internal/diag`, every code produced by a test). Standard library first;
a third-party dependency exists only in IMPLEMENTATION-PLAN §11. Package boundaries follow §3's
dependency rule; an agent edits only the packages its task owns (§5.3).

## §4 Testing and goldens

IMPLEMENTATION-PLAN §7 is the test strategy. Goldens (`examples/**/expected/`, `testdata/`
dumps, command outputs) are **written by the compiler** with `-update` and compared byte for
byte; a golden diff is reviewed like code and never typed by hand. The illustrative pipeline
goldens are the one exception until M2 regenerates them (GEN-01). Negative cases are per-code
txtar tests, not dirty fixtures. Committed fixtures are small (`examples/_fixtures/`, ≤ 300 KB);
real data lives only in the git-ignored `testdata-real/` and runs through opt-in targets
(DECISIONS 29). Tests run under `-race`; fuzz targets per §7.7.

## §5 Determinism

No output depends on map order, scheduling, directory listing order, the clock, the environment
or the checkout path (IMPLEMENTATION-PLAN §1.4, §7.5, §10): sorted iteration, `/` paths, `\n`
line ends, byte-order path sorting, case-sensitive names. A `range` over a map in an output
package needs `//canon:unordered` with a reason.

## §6 Performance

The NFR-01 targets and the benchmark project are IMPLEMENTATION-PLAN §7.6; the architecture they
force (per-file and per-entry memoization from M4) is designed in from M1, not retrofitted.

## §7 Versioning

IMPLEMENTATION-PLAN §9: language `MAJOR.MINOR` in `project.canon`, compiler semver `0.x.y`,
`canon-fp v1`, `canon-vm/N`, `canon.lock v1`, runtime helpers `rt_v1`. No API compatibility
promise before 1.0; a breaking change to a versioned format bumps its version, never silently.

## §8 What agents never do

- Edit outside this directory (Resource, Source, sibling services are read-only), push `main`
  or force-push, or add a remote (a cloud session's `claude/*` branch: DECISIONS 28).
- Edit DECISIONS.md, SPEC.md, CLI.md or `spec/` during implementation: a needed change is
  reported to Louis.
- Hand-edit a golden, a generated file, `.sovaudit/baseline.tsv` or `.sovaudit/state.tsv`.
- Commit real game data, a secret, or an absolute machine path.
- Declare done with `make check` red, or grade their own diff.

## §9 Commits, ADRs, state

Conventional commits (`feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `chore:`), one logical
change each, after a green `make check`. Spec-phase decisions live in `DECISIONS.md`;
implementation decisions are ADRs, `meta/decisions/NNNN-slug.md`, never renumbered.
`meta/state.md` (≤ 80 lines) holds the present: focus, next step, open Louis-calls.
