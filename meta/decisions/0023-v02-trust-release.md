# ADR-0023 — v0.2, trust: every decision, and how the run builds it

Date: 2026-10-31. **Status: accepted** (Louis, 2026-10-31: "write the accurate ADR for the v0.2 …
we will start right away afterwards"). Scope: Louis's v0.2 picks of 2026-10-14 (plan.md) and the
[roadmap](../handoff/2026-10-31-canon-roadmap-v02-v04.md) §3, as answered on 2026-10-31
([log-2026-10-31](log-2026-10-31.md)).

Designs are the orchestrator's (technical calls are its own, DECISIONS 207). They come from seven
read-only research passes over `main` at `4dfd414e` (v0.1.13, plus two docs commits), the real
Resource copy in `testdata-real/sov-v0112`, Source's generated C++ and the Sovereign services.

This ADR amends DECISIONS 338, 340, 363, 374, 375, 376, 379, 383, 391 and 393.

The successors are outlines: [ADR-0024](0024-v03-lean-generated-code-outline.md) (v0.3) and
[ADR-0025](0025-v04-formulas-outline.md) (v0.4).

## 0. How to run this ADR

The run is executed by an orchestrator working at medium effort. This section is its rulebook. Read
it twice.

- **The ADR is decision-complete.** The run builds what this ADR says and never redesigns.
  - Each *Decision* binds.
  - Every "Spec text" block is final wording, applied verbatim by the spec sync (§11, S1).
  - Every "ERRORS.md" block is applied verbatim by S2.
  - A builder that proposes another design is thanked. Its proposal is logged as owed in
    `meta/state.md`, and it builds what is written here.
- **DECISIONS numbers.** 398-422 are the next free numbers on `4dfd414e`, where 397 is the last.
  - If one is taken when the run starts, shift every number of this ADR by the same offset and log
    it.
  - Inside the ADR, `D-<tag>` in a quoted block means "the number §1's table gives that tag". The
    spec sync writes the number, never the tag.
- **File:line references** are from `4dfd414e`. They locate; they do not bind. A reference that no
  longer matches means the code moved: find the construct by name.
- **Readings.** Several research passes ran no probe. A "Today" claim marked `[unverified]` is a
  reading of the code.
  - **The failing-test-first rule settles every reading.** Each unit writes its tests first and shows
    that they fail on unchanged production files. Copy the files aside and back; never `git stash`.
  - A test that does not fail means the reading was wrong. **The unit stops and reports**, and the
    orchestrator decides under the fallback below.
- **Fallback, for a small question this ADR does not answer.** A real design question is a gap; see
  the next rule. Otherwise, in order:
  1. A spec document that answers it wins (DOCTRINE §2).
  2. Otherwise take the reading that refuses more, writes less and keeps today's behaviour for
     everything this ADR does not name (strictest option, DECISIONS 25).
  3. Log the question, the readings and the choice in `meta/decisions/log-<date>.md`.
  4. Never widen the scope. A new bug is logged as owed in `meta/state.md`, not fixed, unless it
     blocks an acceptance named here.
- **A real gap** is two readings that change behaviour, neither stricter. It stops that unit only.
  The orchestrator decides it, logs both passages and the choice, and the rest of the run goes on.
- **Philosophy questions only go to Louis**, in a file under `meta/handoff/`. The run continues on
  everything the question does not block.
- **One call is flagged for Louis: Go exhaustiveness (§4, Decision 9).** It does not block the run.
  The default is the design as written. If Louis vetoes before EN-GO starts, EN-GO waits and the
  orchestrator asks him for the alternative in a file.
- **What a medium-effort orchestrator most often gets wrong** (from v0.1.6 to v0.1.13):
  - accepting "it passed" without the log;
  - letting two builders touch one package;
  - regenerating goldens without reading the diff;
  - forgetting the memory cap in a VERIFY line;
  - telling Louis units run without checking `ListAgents`;
  - redesigning when a test is inconvenient.

  §11.1 lists the rule against each.

## 1. Context and scope

- **Scope.** Louis's v0.2 picks (2026-10-14):
  - N5, testable package checks ("very important", first);
  - enums without `@codes` as safe as ids;
  - `E3804` and `E1905`;
  - M4 call (b), only if easy and risk-free.

  Plus the roadmap's v0.2 additions (2026-10-31): checked decode and bound constants (Sovereign's
  L-0147 wave E is about to build hot paths on generated records), the cross-package macro rule,
  wire-name hygiene, and every known bug (the standing rule).
- **Moved out of v0.2** (roadmap §3):
  - `ordered_json`, kind constants and N8 templates go to v0.3 (ADR-0024);
  - N1 is parked (Louis Q4);
  - N2 goes to v0.5;
  - N3 goes to v0.3.
- **Decided out:** M4 call (b), `check.Info` layered maps (§10.9). It is neither easy nor
  risk-free.
- **Release:** `v0.2.0`. The language version stays `canon: "0.1"` (§12, DECISIONS 422).
- **Robustness before speed (Louis, 2026-10-14) holds.** No item may be "improved" with a cache, a
  memo shared across worlds, a parallel stage or a partial re-check.

| # | Item | § | DECISIONS | Units (§11) |
|---|---|---|---|---|
| 1 | N5: `expect package`, testable package checks | 2 | 398 (D-N5) | N5-SYNTAX, N5-RULES, N5-FORMAT, N5-CHECK, N5-EVAL, N5-BUILD, N5-CLI, N5-TOOLS, EXAMPLES, GUIDE |
| 2 | Decoders check range and length refinements | 3.1 | 399 (D-RANGE) | IR-RANGE, CPP-RANGE, GO-RANGE, TS-RANGE, E2E-RANGE |
| 3 | Bound constants | 3.2 | 400 (D-BOUNDS) | (same) |
| 4 | Row addresses written down | 3.3 | 401 (D-ADDR) | (same) |
| 5 | Enums without `@codes` are opaque | 4 | 402 (D-ENUM) | IR-PLANS, EN-GO, EN-CPP, EN-TS |
| 6 | A name another package `#define`s is `E8005` | 5 | 403 (D-MACRO-1) | MACRO-IR, CPP-OWED |
| 7 | Guards skip comments and literals | 5 | 404 (D-MACRO-2) | CPP-OWED |
| 8 | A wire name holds no NUL | 6 | 405 (D-WIRE-1) | CHECK-OWED |
| 9 | Wires unique within an enum and a variant | 6 | 406 (D-WIRE-2) | CHECK-OWED |
| 10 | `<Fn>File` never doubles `File` | 7 | 407 (D-FILE-1) | IR-PLANS |
| 11 | Kept dependent maps (`E3804`) | 8.1 | 408 (D-E3804) | DEP |
| 12 | An amend path enters a dependent map (`E1905`) | 8.2 | 409 (D-E1905) | DEP |
| 13 | A level-1 ref names the entry its field holds (R7) | 8.3 | 410 (D-R7) | EDIT |
| 14 | The "Still open" checker items ruled | 8.4 | 411 (D-OPEN) | DEP, EDIT, S1 |
| 15 | A folded-constant default is checked as its literal | 8.5 | 412 (D-DEFAULT) | CHECK-OWED |
| 16 | `values` never selects table ids | 9.2 | 413 (D-TSIDS) | IR-PLANS, GEN-TS |
| 17 | `E9001`, `E9006`, `E8104` name their way out | 9.8 | 414 (D-WAYOUT) | S2 |
| 18 | The Go and C++ JSON runtimes are their files | 9.10 | 415 (D-RTTEXT) | S1, GEN-GO, CPP-OWED |
| 19 | Owed generator bugs and hygiene | 9 | none (9.1, 9.3-9.7, 9.9, 9.11) | GEN-GO, GEN-TS, IR-PLANS, PROGEN-TIERS, MUST, CHECK-OWED |
| 20 | A failed rollback is a kept journal; display paths | 10.1 | 416 (D-ROLLBACKERR) | EDIT |
| 21 | An edit locks every package whose read set it changes | 10.2 | 417 (D-J7LIST) | WS-LOCKS |
| 22 | A text output is never a source | 10.3 | 418 (D-TEXTSOURCE) | CHECK-OWED, BUILD-LINKS |
| 23 | A Windows junction is a link | 10.4 | 419 (D-JUNCTION) | PROJECT-LINKS, LOAD-LINKS, BUILD-LINKS, WS-LOCKS |
| 24 | Single-file loads follow `load.dir`'s link bounds | 10.5 | 420 (D-SINGLELINK) | LOAD-LINKS |
| 25 | A name reported changed is read again | 10.6 | 421 (D-REPORTED) | WS-REPORTED, LSP-WATCHED |
| 26 | The cache's stat table: plan text only | 10.7 | none | S1 |
| 27 | `evalAnalysis`: stays parked | 10.8 | none | none |
| 28 | M4 call (b): out | 10.9 | none | none |
| 29 | Language version stays 0.1 in compiler 0.2 | 12 | 422 (D-LANGVER) | S1 |
| 30 | Release | 12 | none | orchestrator, RD-PATCH |

## 2. N5: testable package checks, `expect package` (DECISIONS 398)

### Facts

**Tests never run package checks.**
- "Instance checks and package checks do not run on them in `canon test`" (`spec/EVALUATION.md:530-531`).
- "Package checks never run for a subject" (`:547-548`).
- `fails` with only a package check's name is `E5004` (`:564-569`; `SPEC.md:2453-2457`).

**How an `expect` is built today.**
- `execExpect` and `subject` (`internal/eval/tests.go:96-155`) evaluate `x.X` into a capture, then
  call `Builder.Build` (`:16-18`).
- `subjects.Build` (`internal/build/conformer.go:87-107`) verifies the value and runs
  `rules.Instances`, with every bag mapped to the capture.
- `Runner.Package` is called only from `stagesCD` (`internal/build/pipeline.go:318-345`).

**A child evaluator already exists, for conformance vectors.**
- `vectorEvaluator` lives at `eval/vector.go:59-72`.
- `state` returns any settled parent state (`evaluator.go:154-168`).
- The child looks things up through `parent`: invalid marks (`evaluator.go:240-242`), stage-B memos
  (`stageb.go:105-112`), params, origin, history and key indexes.
- Memo tracing is off when `parent != nil` (`memo_trace.go:44`).
- The build host offers `LoadInto` and `VerifyInto` (`build/hosts.go:51-53`, `189-193`).

**Layers.**
- Layers apply after conversion and adoption, in `letValue` (`eval/root.go:11-30`).
- `amendBlock` resolves a layer target (`check/layers.go:33-56`):
  - `E1909` if the target is declared elsewhere;
  - `E1902` for a built-in or a non-let;
  - `E2102` if it is unknown.

  It then records `NameUses` and `dependsOn`.

**Other machinery.**
- **Conformance calls:** `TestCalls` runs tests through the same Builder to record translated calls
  (`eval/testcalls.go:30-57`).
- **Budget:** each evaluator has one `*counter` (`eval/counter.go:12-40`); a vector gets its own.
- **Syntax:**
  - `ExpectStmt{X, Outcome, Message}` (`syntax/ast_stmt.go:133-146`) is a **frozen node**
    (IMPLEMENTATION-PLAN §4.1).
  - `package` is a nameable word (`syntax/tables.go:30-31`), so `expect package passes` is `E2102`
    today `[unverified]`.
- **LSP and rename:** definition, hover and rename read `NameUses` generically (`check/info.go:56-67`,
  `check/occurrences.go:95-104`, `edit/renamename.go:64-80`).
- **Examples' tests are never run:** `TestExampleFindings` (`internal/cli/examples_test.go:155`) runs
  `canon check` only.
- **TextMate:** `expect` and `package` are already keywords (`editors/vscode/syntaxes/canon.tmLanguage.json:563`).
- **Tables:** a table value comes only from a table literal, a load or another table. `table T ≤ [T]`
  works in one direction only (TYPES §6.2), and a spread in a table is `E3320`.

**Resource** (read-only copy):
- 279 top-level `check`/`warn` lines and 59 `fn …Problems`.
- `crafting.canon:100-109`'s check calls `craftingLevelProblems`, and `craftingTests.canon:52-56`
  tests the fn directly. The check's own wiring is never run. `persistedIds.canon:38-48` is the same
  case.

### Problem

A package check cannot be tested:
- it never runs for a subject, and naming it is `E5004`;
- its inputs are top-level values, which a test cannot change.

So projects move each rule into a fn that tests call directly. What the check reads, and where `fail`
points, stay untested.

### Decision

1. **Syntax: one new `expect` subject.**
   - The form is `expect package [{ name: value, … }]`, followed by the unchanged `passes`, `fails …`
     or `warns …` (DECISIONS 319).
   - `package` is reserved. It names what is built, and it gives meaning only to text that is `E2102`
     today.
   - Rejected:
     - a test that names a layer file: one file per scenario, a global name `build --layer` would
       activate, and amend paths that cannot append to a list or drop an entry;
     - path items: a second spelling of a spread, which also could not read test locals;
     - a test-level header;
     - `amend {…}`, which promises layer semantics;
     - `with`, which is not reserved.
2. **Items.**
   - Each item is `IDENT ":" expr`.
   - The braces hold at least one item: `package {}` is `E1116` (omit the braces, 319).
   - `{` must be on the `package` line, and an outcome is required (`E1116`).
3. **Targets.**
   - A target is a top-level `let` of the test's own package, public or `local`. It is resolved as
     `amendBlock` resolves a layer target, and `NameUses` and `dependsOn` are recorded.
   - Anything else is `E5006`: `notLet`, `otherPackage` or `twice`. An unknown name is `E2102`.
   - Consts are excluded, because phase 2 folds them into types. Another package's lets belong to that
     package's tests.
4. **Values.**
   - Each value is checked against the let's type. A static error, including a literal outside a
     refinement (DECISIONS 155), is a compile error that breaks the test (CHK-05).
   - Values are evaluated in the test, left to right, before anything is built, as a value subject is.
     They read the test's values (with active layers, the substituted lets included) and test locals.
   - A hard error or a poisoned read there belongs to the `expect`, and nothing is built.
   - Rejected: evaluating the values in the world. Reading the substituted let there would be a
     cycle, and the test locals are gone.
5. **The world.**
   - Only the test's package is built, as stages A-D of `canon check` build a selected package: every
     top-level const and let afresh, in forced-set order.
   - A substituted let takes its value:
     - copied into world instances (sharing, identities and construction facts kept, no verdicts);
     - converted at the let's storage point and adopted as its collection;
     - with no initializer, no `entry` declarations and no layer.
   - Every other let has its initializer, entries and active layers, so values derived from a
     substituted one follow.
   - Broken declarations stay unevaluated (TYPES §1).
   - Other packages' values are the test's, forced there if needed, with their verdicts. Imports are
     acyclic (`E2002`), so those packages cannot depend on this one, and re-forcing them would
     multiply their budgets (DECISIONS 328).
   - Stage B verifies every world value, stage C runs instance checks, and stage D runs the package's
     package checks.
   - Not run: stage E, emit validation, vectors, the lock comparison, view and unit checks, `W4001`.
     The subject is what `check` says of the package's values and checks, never of its outputs.
6. **Capture.**
   - Every world finding is captured: all bags go to the capture, unlimited and deduplicated, with the
     value paths a build gives them.
   - Reading a poisoned value of another package fails the `expect`, naming that value.
   - `E4401` stops the test and is never captured.
7. **`E5004`.**
   - For a package subject, the name must be a named package check of the test's package, or a check
     reachable from the static type of one of its top-level values (variant `package`).
   - For a value subject whose name is a package check of the test's package, the variant
     `packageCheck` states the way out (DECISIONS 305).
   - Otherwise the variant is `value`, which keeps today's text.
8. **Budget.**
   - The world uses the parent's counter: the values are charged to the test, the world's roots and
     checks as a build charges them.
   - `E4402` counts one stack across the boundary.
9. **Layers.** `--layer` applies in the test and in every world. A substituted value gets no layer: it
   is final.
10. **Isolation.** The world is discarded after its `expect`, and the test's values do not change.
11. **Vectors.** A world records no call, and `TestCalls` skips package subjects.
12. **No memo, no cache.** Nothing of the package is shared between worlds (Louis, 2026-10-14).
13. **Tooling.**
    - The formatter gets one new row.
    - LSP, rename and edit need no code (`NameUses`); API.md E33 gains one phrase.
    - TextMate is unchanged; progen generates the form; the guide gets a recipe.
    - API types are unchanged (B3).

### Code

```go
// internal/syntax/ast_stmt.go — frozen §4.1, review rule
type ExpectStmt struct {
	Bounds
	X       Expr            // nil when Package is set
	Package *PackageSubject // EVALUATION.md §10.5
	Outcome *Ident
	Message NameLit
}
type PackageSubject struct { Bounds; Braces Delims; Items []*Substitution } // Braces zero when bare
type Substitution struct { Bounds; Name *Ident; Value Expr }
// constants.go: KindPackageSubject, KindSubstitution + names; parse_stmt.go: packageSubject()

// internal/check/substitutions.go
func (c *checker) packageSubject(env *env, s *syntax.PackageSubject) // targets as amendBlock; c.expr(env, it.Value, c.letType(o))

// internal/eval/tests.go: execExpect → execPackage(r, x) when x.Package != nil; Builder gains
	BuildPackage(ctx context.Context, w *World, capture *diag.Bag) // EVALUATION.md §10.5
// internal/eval/world.go
func (e *Evaluator) world(pkg string, subs map[check.Object]value.Value, capture *diag.Bag) *World
func (w *World) Evaluator() *Evaluator
func (w *World) Order() []Root // the package's consts and lets, forced-set order
// world_copy.go: sharing-preserving copy; world_host.go: Load via LoadInto, Verify via VerifyInto into the world's bags

// internal/build/world.go: all bags → capture; Force each Order() root; BeginVerification;
// runner.Instances per root; runner.Package(ctx, cp)
func (s subjects) BuildPackage(ctx context.Context, w *eval.World, capture *diag.Bag)
// internal/rules/tests.go: expectNames branches on x.Package; internal/format: packageSubject, substitution + dispatch.go
```

### Spec text

- **EVALUATION.md, new §10.5, after §10.4 (`:578`):**
  > ### 10.5 Package subjects
  > ```
  > expect package passes
  > expect package { levels: levels + [levels[1]] } fails uniqueIndex
  > ```
  > `expect package` builds the test's own package; with `{ name: value, … }`, each `name` holds `value` while it is built. §10.3's forms judge what the build captures.
  > 1. **Substitutions.** `name` is a top-level `let` of the test's package, public or `local` (anything else is `E5006`, an unknown name `E2102`). `value` is checked against the let's type; a static error is a compile error (CHK-05). The values are evaluated in the test, left to right, before the build: they read the test's values (with the active layers), the substituted lets included, and its locals. A hard error or a poisoned read there is the `expect`'s, as for a value subject (§10.2), and nothing is built.
  > 2. **The world.** The package is built in a world of its own, as stages A to D build a selected package (§1, §2.1, §5, §8): every top-level `const` and `let`, in forced-set order. A substituted let takes its value, converted at the let's storage point in the world (§4.3), with no initializer, `entry` declaration or layer; every other let is evaluated from its initializer, entries and active layers, so values derived from a substituted one follow. A broken declaration is not evaluated (TYPES.md §1). Values of other packages are the test's, forced there if needed: none can read this package (TYPES.md §3.1). Conversions, verification and taint (§7.3) are the world's own; the test's verdict on a part of a value is not reused. An entry keeps its identity (TYPES.md §6.3): one of a record field's collection still belongs to the test's instance.
  > 3. **Checks.** Stage B verifies every value of the world, stage C runs the instance checks reachable from them (§8.1), stage D the package's package checks (§8.2). Stage E, emit validation, conformance vectors, the lock comparison and view checks do not run.
  > 4. **Capture.** Every finding the world produces is captured, with the value path a build gives it (§13). Reading a poisoned value of another package makes the `expect` fail, naming it (§10.2). `E4401` in the world stops the test (§10.4) and is not captured.
  > 5. **Cost.** The values are charged to the test, the world as a build is (§12.2), all on the package's counter: each package subject costs what stages A to D of the package cost.
  > 6. **After.** The world is discarded; the test's values do not change. A world records no call for conformance vectors, and computing them builds no package subject (§1).
- **EVALUATION.md, other edits.**
  - `:531`: after "…do not run on them in `canon test`", add "except in the world of a package
    subject (§10.5)".
  - `:548`: replace "Package checks never run for a subject." with "Package checks never run for a
    value subject; `expect package` runs them (§10.5)."
  - `:45`: after "Each package's tests run as `canon test <pkg>` runs them", add ", package subjects
    (§10.5) not built".
  - The §10.3 table gets a new row after `:562`: `| expect package [{ … }] passes / fails … / warns
    … | the same, over the findings of building the package (§10.5) |`.
  - The bullet at `:564-569` is replaced by:
    > - An identifier after `fails`/`warns` matching `^[EW][0-9]{4}$` is a code. Any other names a check: for a value subject, a named check on a type reachable from `v`'s static type (through fields, elements, map keys and values, variant cases and their own checks, and a dependent field's branches); for a package subject (§10.5), a named package check of the test's package or a named check reachable so from one of its top-level values. Otherwise `E5004`, whose message says when the name is a package check, which only `expect package` runs. `E5004` is static: `canon check` and `canon test` report it (log-2026-10-06).
  - The diagnostics table: `:845` becomes `| E5004 | error | expect … fails name (§10.3, §10.5) |`,
    and a new row after `:846`: `| E5006 | error | a package subject's target (§10.5) |`.
- **SPEC.md.**
  - A row after `:2451`: `| expect package passes / expect package { v: e, … } fails … | building
    the package as canon check does, each let v holding e, gives no error / such an error / such a
    warning |`.
  - A bullet after `:2455`:
    > - `expect package { v: e, … }` runs the package's checks, package checks included, over substituted values: each `e` is computed in the test, then the package is evaluated afresh with `v = e`, so values derived from `v` follow. EVALUATION.md §10.5.
  - `:1251`: after "…its imports in scope", add ", and once more in each `expect package` (§18)".
- **GRAMMAR.md.**
  - `:880` is replaced by:
    ```ebnf
    expectStmt      = "expect" ( packageSubject outcome | expr [ outcome ] ) ;
    outcome         = ( "fails" | "warns" ) ( stringLit | IDENT ) | "passes" ;
    packageSubject  = "package" [ BraceList( substitution ) ] ;   (* not empty: E1116 *)
    substitution    = IDENT ":" expr ;
    ```
  - A bullet after `:890`:
    > - After `expect`, `package` always starts a package subject (EVALUATION.md §10.5): its `{` is on that line and it needs an outcome (`E1116`); a data symbol named `package` is written qualified there (`Icon.package`).
- **CLI.md**, after `:347`:
  > `expect package { v: e, … }` runs the package's checks over substituted values (EVALUATION.md §10.5); its failures print as any other, and in JSON each captured finding carries its `path` and `check`.
- **FORMATTER.md.**
  - A row after `:234`: `| package subject | "package", or "package " BL(items), each item name
    rhs(":", value) | per item |`.
  - `:247`: "When the subject is a brace literal" becomes "When the subject is a brace literal or a
    package subject with items".
- **CONFORMANCE.md** `:235-236`: "made while running the package's `test` blocks" becomes "made
  while running the package's `test` blocks, outside the worlds of package subjects, which are not
  built here (EVALUATION.md §10.5)".
- **API.md** E33 (`:1355`): after "active or not;", insert "the targets of `expect package`
  substitutions;".
- **IMPLEMENTATION-PLAN.md.**
  - `:233`: `ExpectStmt` becomes "`ExpectStmt` (with `Package`: `PackageSubject`, `Substitution`)".
  - A row after `:1182`: `| pkgchecks | expect package: substitutions, derived values, a named
    package check, CHK-05 | v0.2 | present |`.

### DECISIONS text

> 398. **Testable package checks: `expect package` (EVALUATION.md §1, §10.1-§10.5; SPEC §10.1, §18; GRAMMAR.md §5.10; CLI.md §3.5; FORMATTER.md §7.2; CONFORMANCE.md §6.1; API.md E33; ERRORS.md E5004, E5006).** `expect package [{ name: value, … }]` with the unchanged outcome forms builds the test's own package as stages A to D of `canon check` do, each named top-level `let` (public or `local`) holding its value. A value is checked against the let's type (a static error is a compile error, CHK-05) and computed in the test, reading its values and locals, before the build. In the world every const and let is evaluated afresh, a substituted one without initializer, entry or layer, so derived values follow; other packages' values are the test's; the world's verdicts are its own; stage E, emits, vectors, the lock comparison and view checks do not run. Every finding is captured; `E4401` stops the test; worlds record no conformance call and are not built for vectors; the test's values do not change. `E5006` refuses another declaration, another package's let or a let given twice; `E5004` checks names against the package's checks. Rejected: a test naming a layer file, amend paths, a test-level header, values evaluated in the world. Reason: Louis's first v0.2 pick (N5): package checks ran in no test (§10.1-§10.2, E5004), so Resource tested none of its package checks and kept its rules in `…Problems` functions.

### ERRORS.md (applied by S2)

- **Codes table:** replace the `E5004` row (`:1077`) and add `E5006` after `:1078`:
  | Code | Severity | Package | Owner | Meaning |
  |---|---|---|---|---|
  | E5004 | error | rules | EVALUATION.md §10.3, §10.5 | `expect … fails name` names no check that runs for its subject |
  | E5006 | error | check | EVALUATION.md §10.5 | a package subject substitutes something other than a top-level let of the test's package, or one let twice |
- **Messages table:** replace the `E5004` row (`:1087`) and add the `E5006` rows after `:1088`:
  | Code | Variant | Args | Template |
  |---|---|---|---|
  | E5004 | value | name:Name, typ:Type | `no check named {name} applies to {typ}` |
  | E5004 | packageCheck | name:Name, typ:Type | `no check named {name} applies to {typ}; {name} is a package check, which only expect package runs` |
  | E5004 | package | name:Name, pkg:Name | `no check named {name} runs for package {pkg}` |
  | E5006 | notLet | name:Name | `{name} is not a let: expect package substitutes top-level lets of its own package` |
  | E5006 | otherPackage | name:Name | `{name} is a let of another package: substitute it in a test of that package` |
  | E5006 | twice | name:Name, first:Loc | `{name} is substituted twice (first at {first})` |

### Tests

| Test | Pkg | Setup → assertion | Rule |
|---|---|---|---|
| `parse/package_subject.txtar` | syntax | bare form, one line, multi-line items; `expect Icon.package == x` stays an expression | GRAMMAR §5.10 |
| `findings/E1116_3.txtar` | syntax | `package {}`, a missing outcome, `a.b: 1` | §5.10 |
| `fmt/package_subject.txtar` | format | fixed point; a wide subject breaks one item per line, verdict after `}` | FORMATTER §7.2 |
| `findings/E5006_1/_2/_3.txtar` | check | `notLet` (const, fn, record, `len`); `otherPackage` (selective import; an unimported package's let); `twice` | §10.5 |
| `E2102_<next>`, `E3002_<next>`, `E3204_<next>.txtar` | check | unknown target; `[Item]` into a `table Item` let; `-1` into `Int(0..)` | CHK-05, DECISIONS 155 |
| `accept/package_subject.txtar` | check | local, public and untyped lets, a table literal, a test local: no finding | §10.5 |
| `findings/E5004_2/_3.txtar` | rules | `package` variant; `packageCheck` variant | §10.3 |
| `TestWorldCopyIsFaithful` | eval | each value kind (defaults, applied records with dependent fields, cases, keyed lists, maps, tables with retired entries, let-path and level-1 refs, one instance held twice): verifying the copy in a world gives the same findings, instance count and `value.Equal` as the original in a fresh evaluator | §10.5.2 |
| `TestVectorsIgnorePackageSubjects` | build | a translated fn called by a package check: vectors are identical with and without an `expect package` | CONFORMANCE §6.1 |
| `commands/test_package_{pass,fails,derived,refs,instances,layer,static,json,budget,imports_shared,poisoned_import,rhs_error,isolation}.txtar` | cli | listed below | CLI §3.5, §10.5 |
| `testdata/package_subject.txtar` | lsp | definition on a target goes to the let; hover shows it | CLI §4 |
| `TestRenameSubstitutionTarget` | api | `RenameName c:n→m` rewrites `expect package { m: 2 } passes`; the tests still pass | E33 |
| a new `lexicalCases` row | vscodegrammar | `package` is scoped as a keyword, `n` is not | §8.4 |
| `TestExampleTests` | cli | for every example with `expected/tests.txt`, `canon test <pkg>` prints exactly that file | §7.9 |

Use the next free number for every `<next>`. If a number this ADR names is taken, take the next free
one and log it.

**What the 13 command goldens assert:**
- `pass`: the bare form and a substituted one pass; `-v` prints their ok lines.
- `fails`: `fails` matches by name, by text and by code; a failing `passes` lists its `got:` lines.
- `derived`: a let that reads the substituted one follows the substitution.
- `refs`:
  - a key removed by the substitution gives `fails E3501` at a ref another let holds;
  - stale-verdict case: a ref that is `E3501` in the test becomes valid once another substitution
    adds the key, and a `warn` that dereferences it fires (`warns` passes).
- `instances`: a record check fires on an element of a substituted list.
- `layer`: under `--layer`, the value reads the layered let; an unsubstituted let is layered in the
  world, a substituted one is not.
- `static`: the test is broken and lists its `E5006` and `E3002`.
- `json`: findings carry `path` and `check`.
- `budget`: `E4401` inside a world stops the test, and the package's later tests do not run.
- `imports_shared`: with budget 100, an imported value of about 60 steps and two worlds give no
  `E4401`.
- `poisoned_import`: the `expect` fails, naming the poisoned value with its cause.
- `rhs_error`: an `E4002` in a value is captured, and no world finding appears.
- `isolation`: after a world, `expect levels.len() == 3` still passes.

**The example.** `examples/features/pkgchecks/pkgchecks.canon`; its `expected/findings.txt` reads
`0 errors…`, and `expected/tests.txt` is written by the tool:
```canon
/// `expect package` (SPEC §18, EVALUATION.md §10.5): a test builds its own package as
/// `canon check` does, over top-level values it substitutes, so package checks have tests.
package features.pkgchecks

/// One crafting level.
record Level {
  /// Level index.
  index: Int(0..=24)
  /// Experience needed to reach it.
  exp: Int(0..)
}

/// The shipped levels.
let levels: [Level] = [
  { index: 0, exp: 0 }
  { index: 1, exp: 100 }
  { index: 2, exp: 250 }
]

/// The highest level a player can reach.
let maxLevel: Int(0..=24) = 2

check uniqueIndex: [l.index for l in levels].isUnique() else "a level index is listed twice"

check {
  for l in levels {
    if l.index > maxLevel {
      fail(l, "level {l.index} is above maxLevel {maxLevel}")
    }
  }
}

test "the shipped levels pass, and still pass without the last one" {
  expect package passes
  expect package { levels: levels[0..2] } passes
}

test "a level listed twice, or above the cap, is refused" {
  expect package { levels: levels + [levels[1]] } fails uniqueIndex
  expect package { maxLevel: 1 } fails "level 2 is above maxLevel 1"
}
```

**The static-error example.** `examples/features/pkgchecks/static/static.canon`. Its `findings.txt`
holds one `E3204` ("-1 is outside 0..", expected by the `cap` declaration). Its `tests.txt` shows the
test as FAIL with that error, `0 passed, 1 failed`.
```canon
/// A substitution is typed like any value: a literal outside the let's refinement is a
/// compile error, never an expected failure (CHK-05, DECISIONS 155).
package features.pkgchecks.static

/// The level cap.
let cap: Int(0..) = 10

check capBound: cap <= 100 else "cap {cap} is above 100"

test "a negative cap does not compile" {
  expect package { cap: -1 } fails E3204
}
```

### Acceptance

- **The new tests prove the change.** The command goldens and the example fail on `4dfd414e` (the
  form is `E2102` there) and pass after the change.
- **The copy is load-bearing.** `TestWorldCopyIsFaithful` and the stale `refs` case fail when the
  copy is disabled. Prove it by copying the file aside, never with `git stash`.
- **Repository checks.** `make check` is green, and `go test -race` passes on every touched package.
- **Resource**, at release §12 step 5, in the scratch farm:
  - rewrite crafting's "listed twice" test and the three persistedIds tests as `expect package`;
  - they pass, and they fail once the check is removed;
  - the whole-snapshot `canon test` gives the same counts as before;
  - report wall time and peak RSS, before and after.

### Units

| Unit | Owns | Tier, reason | Size | After |
|---|---|---|---|---|
| N5-SYNTAX | `internal/syntax` | **opus**: frozen AST (IMPLEMENTATION-PLAN §4.1 review rule) | S | S2 |
| N5-RULES | `internal/rules` | sonnet | S | N5-SYNTAX |
| N5-FORMAT | `internal/format` | sonnet | S | N5-SYNTAX |
| N5-CHECK | `internal/check` | sonnet, opus review (it mirrors `amendBlock`) | S | N5-RULES, DEP |
| N5-EVAL | `internal/eval` | **opus**: state across evaluators, where a slip silences a check | L | N5-CHECK, DEP |
| N5-BUILD | `internal/build` | **opus**: stages over a child evaluator, error routing | M | N5-EVAL, MACRO-IR |
| N5-CLI | `internal/cli` (command goldens, `TestExampleTests`) | sonnet | S | N5-BUILD, N5-FORMAT |
| N5-TOOLS | `internal/lsp` (testdata), `api` (one test), `internal/testkit/vscodegrammar`, `internal/testkit/progen` (generator; may replace N5-CHECK's `gaps.txt` E5006 line with an operator; `coverage.txt` via `-update`) | sonnet | S | N5-CLI, EDIT, PROGEN-TIERS, MUST |
| EXAMPLES | `examples/features/pkgchecks` (+ its granted AST and scope goldens) | sonnet | S | N5-CLI |

Nothing may write the new form before N5-BUILD lands: until then, eval meets a nil `X`.

### Forecast

1. **Frozen AST.**
   - Add only what Code lists, under the review rule: an opus builder, with spec-reviewer as the
     consumers' proxy.
   - Existing AST goldens must not change: nil fields are not dumped (`dump_test.go:84-100`)
     `[unverified]`.
   - Every reader of `ExpectStmt.X` must branch on `Package` first: `eval/tests.go:138-166`,
     `rules/tests.go:41-53`, `check/stmts_more.go:58-67`, `format/stmt.go:151-160`.
2. **`E5004`'s variants rename its constructor** to `AtValue`, `AtPackageCheck` and `AtPackage`.
   S2 regenerates the catalogue and points `rules/tests.go:50` at `AtValue`. `E5004_1.txtar` changes
   on its `global` line only.
3. **Do not reuse `vectorEvaluator` as is.** It shares every package's settled values and verifies at
   once (`vector.go:70`). A world instead:
   - routes its own package to fresh states;
   - routes other packages to the parent, forcing there at the world's depth;
   - starts with `verifying` false and calls `BeginVerification` after stage A.
4. **Parent marks and memos stay correct only because nothing of the test's package reaches a world
   uncopied.**
   - The copier carries `bound`, `written`, `origin`, `history`, `rebuilt` and identities, with
     owners remapped.
   - It never carries `invalid` or `verified`.
   - Comment each value-keyed map of `Evaluator` with its class.
5. **Counter.** Share the parent's counter pointer: never a new counter, never `whole`. An `E4401`
   `heaviest:` line then sums the steps across worlds, which is deterministic.
6. **`E4401` from a world** goes to the test's stop findings (`tests.go:80`), never to the capture.
7. **`TestCalls`.** When `recording` is set, skip the package subject as passed and spend no step.
8. **Loads** are re-decoded per world through `LoadInto` (scratch). The manifest does not change.
9. **Cost and budget.** Each world pays the package's stages A-D, so a loop of `expect package`
   multiplies the cost. No memo or sharing may be added to save it; the way out of an overrun is
   `project.budget`. This means a test of a package as large as Resource's `items` (~46 M steps) runs
   at most a few worlds within the budget. The guide says so.
10. **Tables** accept only table values, and spreads in tables are `E3320`. The guide must say so.
11. **Field-collection entries** inside a value keep their owner from the test, so converting one to
    a ref in the world is `E3503`. The way out is to substitute the record that owns the collection;
    the guide must say so.
12. **Refinement tests.** A literal outside a refinement is a static error. A dynamic violation
    (Resource's `zero() - 1` idiom) is captured at the let's storage point.
13. **Code shape.**
    - New files: `eval/world.go`, `world_copy.go`, `world_host.go`, `check/substitutions.go`,
      `build/world.go`.
    - Respect go.md §2's limits; no repeated literal; no map range reaching an output.
14. **Goldens are written only by `-update`:** `E5004_1`, the progen coverage, the example's AST and
    scope goldens, the cli goldens, the example's findings and tests, and the guide's output.

## 3. Decoders check refinements, bounds become constants, row addresses are written down (DECISIONS 399, 400, 401)

### Facts

All at `4dfd414e`. The probes ran on a scratch `canon` build, under `/var/tmp/range-research/`.

**Decoders today.**
- Decoders already refuse a value outside a sized integer's width, a value outside the Duration range,
  and a fraction for an integer (`spec/CODEGEN.md:1747-1751`).
- `types`-mode decoders accept a value "without checking refinements, refs, keys or checks"
  (`CODEGEN.md:1792-1793`, `WIRE.md:516-518`).
- `data` loaders "never validate" (`CODEGEN.md:1543`; `SPEC.md:575-578`, `:1591-1594`,
  `:2535-2540`). Fixed comment T6 says the same (`CODEGEN.md:321-322`), and so does the runtime's
  `Decoder` comment (`canon_runtime_json.h.txt:954-956`, `CODEGEN.md:3093-3094`).

**The IR.**
- Refinements reach the IR only beside a TypeRef: for inputs (`Field.Range`,
  `internal/ir/fields.go:28-30`, `ir.go:72`) and for translated parameters and results
  (`Param.Range`, `ExportFn.ResultRange`, `fn.go:19-20, 33-37`).
- `ownRefinements` already intersects the ranges of an alias chain (`fields.go:114-168`).
- Bounds are folded to numbers when the checker builds the type (`check/refine.go:55-75`); this
  includes a bound that names another package's const. A `Float32` bound is binary64
  (`refine.go:79-92`).
- `stage.ref` builds every TypeRef (`convert.go:22-32`). `collRef` builds a ref's key with `s.ref`
  (`convert.go:105-123`), so the key would inherit the key field's refinement.

**Canon itself (probes).**
- The evaluator rounds a Float32 to binary32, then checks the rounded value
  (`eval/store.go:57-66, 84-107`): `x: Float32(..=0.1) = 0.1` is `E3204` "0.1 is outside ..=0.1".
- Canon accepts `Int8(0..=300)`, `UInt8(-3..)`, `[Int](-5..=2)`, `String(..0)` and
  `Float32(0.1..=0.1)`.
- A refined fn result is checked (`E3204`).
- **Bug found:** a default that names a constant and violates its refinement (`a: Int(1..) = ZERO`)
  is not reported when no value stores it. It is fixed in §8.5.

**Generators.**
- **C++:** field shortcuts bypass `decodeValue` (`internal/gen/cpp/decode.go:174-224`). `Fail` keeps
  only the first error (runtime `:993-997`). The width message is `expected an integer from <lo> to
  <hi>` (`:94-99`).
- **Go:** every value goes through `readValue`, map keys included (`gen/go/convert.go:28-70`,
  `mapread.go:33-34`). Errors are built with `errAt` (`location.go:67-74`).
- **TS:** `dec()` returns an expression (`gen/ts/decode_value.go:19-42`). The width message is
  `integer out of range` (`decode.ts.txt:91-96`).
- **Bug found:** a TS integer map key at a `number` position skips its width check (`decIntKey`,
  `decode_value.go:245`), against WIRE §5.8. It is fixed by TS-RANGE.

**Traps (probes).**
- GCC with `-Wextra -Werror` (the test flags, `testkit/cxx/constants.go:51`) rejects
  `static_cast<int64_t>(u8) >= 0` and `<= 300` as always true. It accepts every compare that can fail.
- **Go:** an untyped float constant compared with a `float32` is rounded to binary32. So
  `f <= 0.1` is true for `f = float32(0.1)`, while `float64(f) <= typed 0.1` is false.
- **TS:** the helpers below compile under `--strict --noUnusedLocals`. Their UTF-8 byte count matches
  `TextEncoder` on 11 samples, lone surrogates included.

**Cost (probe).** 46 MB of JSON, 20,000 rows × 127 integer fields, nlohmann at `-O2`: decode takes
77.5 ms, and 76.4 ms with a two-sided compare per field (the difference is noise). Parsing takes
365 ms.

**Rows and addresses.**
- `KeyedList` and `FlatMap` wrap `std::vector`s that are built once (`canon_runtime.h.txt:331-362`).
- Table containers are move-only. `KeyedList` itself is copyable.
- Resolved refs point only into top-level values (`gen/cpp/resolve.go:71-86`).
- Baked data is one function-local static (`CODEGEN.md:1534-1537`).

**Contract.** The IR is frozen; changing it needs §4's review rule (`IMPLEMENTATION-PLAN.md:155-160`).
Its consumers are conform, gen/json, gen/go, gen/cpp, gen/ts and gen/view.

### Problem

Every C++, Go and TS decoder drops what a refinement promises, so Sovereign re-checks about 310 bounds
by hand. Refinements are outside the fingerprint, so a data file built under an older, wider
refinement loads silently into a binary that assumes the narrower one. There is no constant to
`static_assert` a hand-sized array against. And nothing in the spec states that row addresses are
stable.

### Decision

1. **IR.**
   - `TypeRef` gains `Range *types.Bound`. `stage.ref` sets it from `ownRefinements(t)` for the base
     kinds Int, Float, Duration, String, List, Map and DepMap, and leaves it nil otherwise.
   - `collRef` sets `key.Range = nil`: refs stay unchecked keys.
   - `Field.Range`, `Param.Range` and `ResultRange` stay; a test pins that each equals its TypeRef's
     `Range`.
   - Neither the fingerprint nor the type mapping reads `Range`.
   - Approval under §4's review rule: this ADR is the approval; spec-reviewer (opus) reviews
     IR-RANGE's diff as the proxy for every consumer.
2. **What is checked.** Every value a decoder reads at a level whose TypeRef has a `Range`:
   - record and case fields;
   - list and keyed-list elements, map keys and map values;
   - a value that is not none;
   - dependent branches;
   - a literal union's base (a literal of the union passes);
   - `$` results and lookup cells;
   - `@json(pairs:)` and `@json(bits)` lists;
   - the rows of a data file's top-level list or keyed list;
   - a `@text` result.

   What is measured, as eval measures it:

   | Kind | Measured |
   |---|---|
   | Int, any width | the integer |
   | Float | the binary64 value read |
   | Float32 | the stored binary32 value, widened to binary64, against the binary64 bound |
   | Duration | milliseconds, after the unit is applied |
   | String | UTF-8 bytes |
   | List, keyed list, map | the element count |

   - Bounds compare as `lo <=`, then `<= hi` for `..=` or `< hi` for `..`.
   - Each check is written as one negated conjunction: `if (!(…)) fail`.
   - Never checked: none (absent, `null` or the `@json(none:)` marker), a default taken for an absent
     key, patterns, `where`, assets, refs, checks.
3. **Implied sides.** An integer-kind side that the stored type already guarantees writes no compare:
   - a width limit (`Basic.Limits()`);
   - ±9,223,372,036,854 ms for a Duration;
   - for a length, `lo ≤ 0`, or an inclusive `hi ≥ 2^63−1`.

   Float sides always compare. This rule is required by GCC's `-Wtype-limits` under `-Werror`
   (DECISIONS 394).
4. **Order.**
   - A refinement is judged only after the value decoded without failure, so type, kind and width
     failures come first.
   - For a collection: its elements and keys first, then its duplicate check, then its count.
   - One failure is reported, the first in decode order (CODEGEN §5.13). So `Int8(0..=5)` reading 200
     keeps the width message, and reading 7 fails the refinement.
5. **Message.**
   - One of `expected a value in <R>`, `expected a byte length in <R>` and
     `expected an element count in <R>`.
   - `<R>` is the folded range in Canon text: `lo`, then `..` or `..=`, then `hi`. Ints are decimal,
     floats use `types.FloatText(f, 64)`, Durations `types.DurationText`, lengths decimal.
   - The bytes are the same in every target and mode, inside the existing envelopes: `<T>: $.a.b`,
     `<file>: rows[0].x`, and `<file>: rows` for a top-level count.
   - Width messages are unchanged.
   - The text is built once, in ir (`ir.RangeFailure`), as `inputReasons` are
     (`ir/constants.go:465-472`).
6. **Where each target puts the check.**
   - **C++.** One helper writes `if (!(<cond>)) dec.Fail(<key>, "<message>");`.
     - Placement: at the end of `decodeValue` for every kind; after a field shortcut, which stays;
       after `decodePairs`; and after `CheckUnique` in a data loader's rows loop, with that loop's
       early-return style.
     - Measures: integers as `static_cast<int64_t>(dst)`, Float as `dst`, Float32 as
       `static_cast<double>(dst)`, Duration as `dst.count()`, strings and vectors as
       `static_cast<int64_t>(dst.size())`, KeyedList and FlatMap as `static_cast<int64_t>(dst.Len())`.
     - The check uses the read's own key expression. A map's count check comes after its `Pop`.
     - The check goes inside the branch that read the value, never after a default.
     - No runtime API change: `rt_v1` stays, and only the `Decoder` comment changes.
   - **Go.** `readValue` binds the read expression to a temp and writes
     `if !(<cond>) { return <errAt(loc, msg)> }`.
     - Measures: `int64` on the integer temp before narrowing, `float64(v)`, `v.Milliseconds()`,
       `int64(len(v))`, `int64(v.Len())`.
     - The same goes for pairs, and for rows after `rt.FirstDup`.
   - **TS.** `dec()` wraps its expression in the helpers below:
     - `decIn`: an open or implied side passes ±Infinity;
     - `decInBig`: `@ts(bigint)` positions, with bigint literals, `null` for an open side;
     - `decStrIn`: the literal-union literals are passed in;
     - `decCountIn`.

     `decMapKey` wraps String and Int keys. Integer bounds are written as JS number literals. Rounding
     to the nearest double is monotonic, and `decInt` refuses unsafe integers first, so the compare is
     exact for every value that reaches it. TS-RANGE also fixes the key bug: a sized integer key
     becomes `decInt(decIntKey(k, p), p, lo, hi)`.
7. **Which decoders.**
   - **Checked:**
     - Go, C++ and TS `data` loaders and `@reload` snapshots;
     - `types` decoders (`Decode`, `Parse`, `decode<T>`, `parse<T>`, `Decode<Fn>File`,
       `parse<Fn>File`);
     - TS `decode<V>` and `parse<V>`;
     - the readers of other packages' classes.
   - **Nothing to check:**
     - baked data (no decoder);
     - Go and C++ `embedded`, which are unbuilt (E8019);
     - TS `embedded`, which is baked.
   - **Unchanged:** make hooks stay unchecked (CODEGEN §5.14), and `LoadInputs` keeps its own checks.
   - **Reason for "every decoder":** one rule everywhere (as DECISIONS 395 reasoned), and it is the
     only protection for old data under a matching fingerprint (point 8).
8. **The fingerprint is unchanged** (FINGERPRINT §5).
   - A file built before a refinement narrowed keeps its `$schema`. Its loader refuses each value
     outside the new refinement, at that value's path, and accepts the rest.
   - That is right: shapes are refused by fingerprint and values by value, and there is no version to
     bump.
   - Widening a refinement keeps every older file valid.
9. **Bound constants (DECISIONS 400).**
   - **Scope:** every field of every public record and variant case that an emit writes, in every
     mode, inputs included. TS has no inputs.
   - **Walk:** the field's type, then list and keyed-list elements (`Elem`), map keys (`Key`) and map
     values (`Value`). An optional and a literal union's base add no step. A named or dependent type
     ends the walk.
   - **Constants:** one per finite side of each level's `Range`: `Min`, `Max` (for `..=`) or `End`
     (for `..`), and for a length `LenMin`, `LenMax`, `LenEnd`. The path words go first, outermost
     first.
   - **Value:** the folded bound as it is, never shifted to an inclusive form, never clamped to the
     width.
   - Names, types and placement follow the §5.15 table below. `<F>` is what follows `Get` in the
     field's C++ getter, the field's Go getter name, or the UpperCamel form of its TS property,
     overrides included.
   - Each constant carries the fixed comment T19.
   - Every name is reserved, written or not: `E8005` `both`, at the field.
   - There are no bound constants for aliases (they have no generated name, CODEGEN §4.4), values,
     consts, fns or dependent branches. Bounds are scalars only, so there are no constexpr tables.
10. **Addresses (DECISIONS 401).** The spec gets the paragraph below, and each target gets one test. Nothing
    in the generated code changes.

### Code

```go
// internal/ir/typeref.go, new last field of TypeRef:
	// Range is this level's range or length refinement, the ranges of its alias chain intersected
	// (TYPES.md §7.4); nil for none and on a ref's key. Decoders check it and bound constants name it
	// (CODEGEN.md §5.15); it changes no generated type (GEN-02) and no fingerprint (FINGERPRINT.md §5).
	Range *types.Bound
// ir.go Field.Range comment becomes: // an input's own range refinement, equal to Type.Range; LoadInputs checks it (CODEGEN.md §5.12)

// internal/ir/bounds.go (new)
type BoundStep uint8 // StepElem, StepKey, StepValue
type BoundSide uint8 // SideMin, SideMax, SideEnd
type BoundConst struct {
	Path   []BoundStep
	Length bool       // a String's bytes or a collection's count
	Side   BoundSide
	Kind   types.Kind // Int, Float or Duration (Int for a length)
	Big    bool       // TS: an Int bound at a @ts(bigint) position
	Limit  types.Limit
	Level  *TypeRef   // for <R>
}
func FieldBounds(f *Field) []BoundConst  // field order, depth first, Key before Value, Min before Max/End
func RangeText(t *TypeRef) string        // "1..=120"
func RangeFailure(t *TypeRef) string     // "expected a value in 1..=120" …
func Implied(t *TypeRef, upper bool) bool // Decision 3
func (pl *CppNamePlan) BoundName(f *Field, b BoundConst) string           // "kTagsElemMax"
func (pl *GoNamePlan) BoundName(owner string, f *Field, b BoundConst) string // "Probe_TagsElemMax"
func TSBoundName(owner string, f *Field, b BoundConst) string
```
Every plan declares these names in its own scope: the C++ class scope (`declareClass`), the Go `top`
scope and the TS module scope. If IR-RANGE needs a helper in `internal/types` (for example
`FloatText`), it uses an existing one or reports. It does not edit `internal/types`, which DEP owns.

**TS helpers**, new units of `runtime/decode.ts.txt`:
```ts
/** A number within its range refinement (CODEGEN.md §5.15): lo inclusive, hi inclusive when `inclusive`, an open end ±Infinity. */
function decIn(v: number, path: string, lo: number, hi: number, inclusive: boolean, range: string): number {
  if (!(v >= lo && (inclusive ? v <= hi : v < hi))) decFail(path, "expected a value in " + range);
  return v;
}
/** decIn for a @ts(bigint) value; null is an open end. */
function decInBig(v: bigint, path: string, lo: bigint | null, hi: bigint | null, inclusive: boolean, range: string): bigint {
  if (!((lo === null || v >= lo) && (hi === null || (inclusive ? v <= hi : v < hi)))) decFail(path, "expected a value in " + range);
  return v;
}
/** A String within its length refinement, in UTF-8 bytes (TYPES.md §7.4): s.length to 3 × s.length bytes, counted only between; a surrogate pair is 4 bytes, a lone surrogate 3 (as TextEncoder writes its replacement). A literal of a union passes. */
function decStrIn(s: string, path: string, lo: number, hi: number, inclusive: boolean, range: string, literals: ReadonlyArray<string> = []): string {
  let n = s.length;
  if ((n >= lo && (inclusive ? 3 * n <= hi : 3 * n < hi)) || literals.includes(s)) return s;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c < 0x80) continue;
    const pair = c >= 0xd800 && c <= 0xdbff && i + 1 < s.length && s.charCodeAt(i + 1) >= 0xdc00 && s.charCodeAt(i + 1) <= 0xdfff;
    n += c < 0x800 ? 1 : 2;
    if (pair) i++;
  }
  if (!(n >= lo && (inclusive ? n <= hi : n < hi))) decFail(path, "expected a byte length in " + range);
  return s;
}
/** A list's, keyed list's or map's element count within its length refinement. */
function decCountIn<T>(v: T, path: string, lo: number, hi: number, inclusive: boolean, range: string): T {
  const n = Array.isArray(v) ? v.length : (v as unknown as ReadonlyMap<unknown, unknown>).size;
  if (!(n >= lo && (inclusive ? n <= hi : n < hi))) decFail(path, "expected an element count in " + range);
  return v;
}
```
CODEGEN §8.2's helper block is compared byte for byte with the code (`TestHelperBlockIsTheSpecs`).
So the §8.2 text for these four helpers is **held back from S1**: the orchestrator writes it into
TS-RANGE's commit (§11, S1).

### Spec text

- **CODEGEN.md, new §5.15, after §5.14:**
  > ### 5.15 Refinements at run time (DECISIONS 399, 400)
  > **Checks.** Every decoder this document defines (`data` loaders and snapshots, §5.9, §5.11; `types`-mode decoders, `Parse`, `parse<T>`, `Decode<Fn>File`, §5.13; TypeScript `decode<V>`/`parse<V>`, §8.1; readers of other packages' classes, §2.8) refuses a value outside the range or length refinement of its type at its position (TYPES.md §7.4, an alias chain's ranges intersected): a record or case field, a list or keyed-list element, a map key or value, a dependent branch, a literal union's base (its literals pass), a stored `$` result or lookup cell, a `@json(pairs:)` or `@json(bits)` list, the rows of a data file's top-level list or keyed list, a `@text` result. It measures an integer of any width; a `Float`; a `Float32` as the binary32 value stored, compared in binary64 with the binary64 bound (TYPES.md §7.3); a `Duration` in milliseconds after its unit; a `String` in UTF-8 bytes; a list, keyed list or map by element count. `none` and a default taken for an absent key are not checked; patterns, `where`, assets, refs and checks never run here. A refinement is judged after the value's own type rules (a width included) and, for a collection, after its elements, keys and duplicates; the first failure in decode order is reported (§5.13): `expected a value in <R>`, `expected a byte length in <R>` or `expected an element count in <R>`, `<R>` the refinement in Canon's text (`1..=120`, `0..1`, `..=8`, `1s..=1h`), the same bytes in every target (`Probe: $.tags[2]: expected a value in 0..=5`; `items.json: rows: expected an element count in ..=500`). An integer, `Duration` or length side the stored type already guarantees writes no comparison. Make hooks check nothing; `LoadInputs` keeps §5.12's checks.
  > **Bound constants.** For every field of a record or variant case an emit writes, and every level of the field's type reached through list and keyed-list elements, map keys and map values (an optional and a literal union's base add no level; a named or dependent type ends the walk), each finite side of the level's refinement is a constant: `Min` (lower bound), `Max` (upper, `..=`), `End` (upper, `..`); a length's are `LenMin`, `LenMax`, `LenEnd`; a level below the field prefixes `Elem`, `Key` or `Value` per step, outermost first. The value is the folded bound, never shifted or clamped.
  >
  > | | Go | C++ | TS |
  > |---|---|---|---|
  > | name | `T_` + `<F>` + suffix (`Probe_LevelMin`) | `static constexpr` member `k` + `<F>` + suffix of class `T` (`Probe::kTagsLenMax`) | `export const T_` + `<F>` + suffix |
  > | Int bound, length | untyped constant | `int64_t` | `number`; at a `@ts(bigint)` position a `bigint`; an Int bound of a `number` position outside ±(2^53 − 1) is not written |
  > | Float bound (also of `Float32`) | `float64` | `double` | `number` |
  > | Duration bound | `N * time.Millisecond` | `std::chrono::milliseconds{N}` | `number` (ms) |
  >
  > `T` is the record's or case type's name, `<F>` the field's getter name (C++: after `Get`; TS: UpperCamel of the property), overrides included. Go writes them in one `const ( … )` block after the type's declaration, C++ at the start of the public members after `Decode`/`Parse`, TS after the interface; in field order, then level order, `Min` first; each with T19. Compare a `Float32` value with its bounds in binary64. The name plan reserves every name, written or not (`E8005`). Aliases (§4.4), values, constants, functions and dependent branches have none.
- **CODEGEN §5.13.**
  - In the list at `:1750-1751`, after "an incomplete or non-contiguous `pairs` slot", insert ", a
    value outside its range or length refinement (§5.15)".
  - `:1792-1793` becomes:
    > It refuses a value outside its range or length refinement (§5.15) and checks no pattern, `where`, ref, key or check: the file is the one `canon check` validated (WIRE.md §5.13).
- **CODEGEN §5.9.**
  - `:1543` becomes:
    > - `data`: loaders read one file, check `$schema`, fill every field and check nothing but the wire and the range and length refinements (§5.15, SPEC §14.5).
  - After the "By mode" table, add:
    > **Addresses** (DECISIONS 401). A generated value is built once and never modified, so every row, element, entry and nested record keeps its address while its owner lives: the program for `baked` data, the `shared_ptr` or snapshot a loader returns, the value a `types` decoder returns. Every lookup of one row (`At`, `All`, `Find`, `Get`, `FindBy<F>`, a resolved getter) gives the same address. C++ containers, `canon::KeyedList` and `canon::FlatMap` keep their rows' addresses when moved (table containers are move-only); a copy of a record or decoded value has its own rows, and the original's stay valid while the original lives. A `@reload` builds a new snapshot; an old row stays valid while its snapshot is held, and code keeps ids across reloads (§5.11, DECISIONS 16). In Go a pointer into a value keeps it alive and never moves; in TypeScript `find(k) === at(i)` for one row.
- **CODEGEN §2.5.**
  - T6 becomes:
    > C++: `` /// Reads <files> from `dir`. Returns nullptr and sets `error` if a file is `` / `/// missing, is not JSON, was built from another schema, or holds a value outside` / `` /// its range or length refinement; `canon build` made every other check. ``
    >
    > Go: `// Load<P>Snapshot reads <files> from dir. It fails if a file is missing, is not` / `// JSON, was built from another schema, or holds a value outside its range or` / `` // length refinement; `canon build` made every other check. ``

    The text templates change to match: `internal/gen/cpp/text/snapshot_load_doc.txt` and
    `internal/gen/go/text/data.txt:108-110`.
  - New row:
    > | T19 | C++, Go, TS | bound constant (§5.15) | C++ `/// <Side> of <subject>, from its refinement <R>.`; Go `// <Name> is the <side> of <subject>, from its refinement <R>.`; TS `/** The <side> of <subject>, from its refinement <R>. */` — `<Side>`/`<side>` `Lower bound`, `Upper bound`, `Exclusive upper bound`; `<subject>` the field's Canon name, `each element of `/`each key of `/`each value of ` per step, prefixed `the byte length of ` or `the element count of ` for a length |

    §4's enum design also adds rows to this table (T20, T21). The spec sync numbers them in this
    order: T19 bound constant, T20 `Position`, T21 deleted hooks.
- **CODEGEN §3.3**, new row:
  > | bound constant of a field (§5.15) | `T_` + UpperCamel(f) + suffix | `k` + UpperCamel(f) + suffix, static member of `T` | `T_` + UpperCamel(f) + suffix |
- **CODEGEN §7.5** (`:3093-3094`), and `canon_runtime_json.h.txt:954-955` in CPP-RANGE's commit:
  > /// Reads JSON values into generated classes. It records the first error with the path of the value; the generated decoders also record a value outside its range or length refinement (CODEGEN.md §5.15) and check nothing else `canon build` checked.

  If §9.10 (DECISIONS 415) has already replaced §7.5's listing, this sentence goes into §7.5's
  contract text instead.
- **WIRE §5.13** (`:517`): "It does not re-validate refinements or refs:" becomes:
  > It checks range and length refinements (CODEGEN.md §5.15), as data loaders do, and no pattern, `where` or ref:
- **SPEC.md §14.5** ("Validation at runtime"):
  > Generated code checks the fingerprint (data files), the wire and every range and length refinement (CODEGEN.md §5.15); patterns, `where`, refs and checks run only in Canon. The runtime trusts the rest because it can only receive data this exact schema has validated.
- **SPEC.md** `:577-578`:
  > At runtime, generated decoders check ranges and lengths (§14.5); runtime inputs also check patterns (§19.3); `where` never runs at runtime.
- **SPEC.md §19.3**, last sentence:
  > Besides it, only the range and length checks of generated decoders (§14.5) run at runtime.
- **TYPES §7.4**, new rule:
  > - Generated decoders check range and length refinements when they read a value (CODEGEN.md §5.15).
- **FINGERPRINT §5**, after the table:
  > A refinement is outside the hash: a file built before a refinement narrowed keeps a matching `$schema`, and its loader refuses each value outside the narrower refinement at its path and accepts the rest (CODEGEN.md §5.15); widening one keeps every older file valid.
- **IMPLEMENTATION-PLAN §4.5**, in the TypeRef sketch, a new line:
  > `Range *types.Bound // this level's range or length refinement; nil on a ref's key (DECISIONS 399, 400; §4 review rule)`

### DECISIONS text

> 399. **Generated decoders refuse a value outside its range or length refinement (CODEGEN.md §5.15, §5.9, §5.13; WIRE.md §5.13; SPEC §14.5, §19.3).** Every Go, C++ and TypeScript loader, snapshot, `types` decoder and foreign reader checks each value it reads against the refinement of its type at that position (fields, elements, keys, values, branches, stored results, rows, `@text` results): integers, `Float`, `Float32` (the stored binary32 value in binary64), `Duration` (ms), `String` (UTF-8 bytes), collections (element count). None and defaults are not checked; patterns, `where`, refs and checks stay build-time. The failure, after the type rules and a collection's elements, reads `expected a value in <R>` (or a byte length, an element count), the same bytes everywhere. `ir.TypeRef` gains `Range` (§4 review rule). Reason: decoders dropped what types promise (Sovereign re-checked 310 bounds by hand), and refinements are outside the fingerprint, so only a check refuses data built before a narrower refinement; the checks come from Canon types, so validation still lives only in Canon (DECISIONS 10).
>
> 400. **Each finite side of a field's range or length refinement is a constant (CODEGEN.md §5.15, §3.3).** C++ `T::k<F><Path><Min|Max|End>` (`Len…` for lengths), Go `T_<F>…`, TS `T_<F>…`; the folded bound unchanged; Int: `int64_t` / untyped / `number` (`bigint` under `@ts(bigint)`; unsafe TS bounds not written); Float: `double` / typed `float64` / `number`; Duration as §5.1. Reserved in each plan (`E8005`). Reason: to `static_assert` hand-sized arrays against Canon bounds; class scope and the `_` separator keep names of different types apart.
>
> 401. **Rows of a generated value never move while their owner lives (CODEGEN.md §5.9).** As §5.9 "Addresses" states, pinned by one test per target. Reason: runtimes keep pointers into tables; the property held but was written nowhere.

No ERRORS.md rows. Decoder texts are not diagnostics, and name collisions use `E8005` `both`.

### Tests

- **IR-RANGE** (`internal/ir`):
  - `TestTypeRefRange` (new `typeref_range_test.go`, built like `types_build_test.go`; cites TYPES
    §7.4) asserts each `Range`:
    - `Int(1..=MAX)`, with `MAX` a const of another package;
    - a stacked alias: `Penya(..=1000)` gives `0..=1000`;
    - a list and its element; a map key and value;
    - an optional field and an optional element;
    - a literal-union base; a dependent branch; a fn result; a top-level let;
    - a ref key is nil;
    - `Float32(..=0.1)` has `Hi.F == 0.1`;
    - a Duration, in ms.
  - `TestRangeMirrorsOwn`: `Field.Range`, `Param.Range` and `ResultRange` deep-equal their TypeRef's
    `Range`.
  - `TestFingerprintIgnoresRange`: the fingerprints are equal with `Range` stripped (FINGERPRINT §5).
  - `TestRangeText`, `TestRangeFailure`: tables, including `0..1`, `1s..=1h` and a negative bound.
  - `TestBoundNames`: the three plans, a case included.
  - `E8005_80`, `E8005_81`, `E8005_82.txtar`: C++ `tags: [Int(0..=5)]` beside `tagsElem: Int(0..=9)`;
    then Go and TS through `@go(name:)` and `@ts(name:)`.
  - Regenerate `testdata/ir/teamboard.txtar` after `dump_test.go`'s `typeText` appends `(<R>)` to a
    level that has a `Range`.
- **CPP-RANGE** (`internal/gen/cpp`):
  - `TestCppRangeChecksRun` (new `range_compile_test.go`; hand-built IR with `Range` set;
    `testdata/main/range_main.cpp`):
    - every position, on both sides of each bound, through `Parse` and `Decode`, prints `ok` or
      `error Probe: $.…: <message>`;
    - cases include Float32 `0.1` against `..=0.1` (refused), `é` and `😀` at a byte bound, none, a
      marker, a union literal, and `UInt8(0..=255)` / `Int8(0..=300)` (no compare written);
    - built with every compiler at `cxx.Flags`.
  - `TestCppRangeDataRun`: `items.json: rows: …` and `rows[1].name: …`.
  - `TestCppBoundConstants`: the header lines, plus `static_assert(Probe::kTagsLenMax == 3)` in the
    driver.
- **GO-RANGE:**
  - `TestGoRangeChecksRun` and `TestGoRangeDataRun`, in a temp module with `go vet` and `go test`.
  - `TestGoBoundConstants`: `var _ [Probe_TagsLenMax]int`, `len(x) <= Probe_TagsLenMax` and
    `float64(f32) <= Probe_F32Max` all compile.
- **TS-RANGE:**
  - `TestTSRangeChecksRun`: both tsc versions, `--strict`, then node.
  - `TestTSBoundConstants`: a bigint bound is written; an unsafe `number` bound is not.
  - `TestTSIntKeyWidth`: `{UInt8: Int}` with key `"300"` gives `integer out of range` (WIRE §5.8).
- **E2E-RANGE** (`internal/ir`, new files only):
  - `TestRangeRoundTrip` (`range_run_test.go`, `testdata/e2e/rangerun/`; modelled on
    `TestRefDependentRoundTrip`; cites CODEGEN §5.15 and FINGERPRINT §5):
    - per case, whether `load` accepts it;
    - every reader's verdict (`ok`, or `error <path after the root>: <message>`) is equal across Go,
      C++ and TS types decoders and data loaders;
    - where `load` refuses, the case wrapped with the true `$schema` (old data under a matching
      fingerprint) must be refused identically.
  - `TestStableAddresses{Cpp,Go,TS}` (`addresses_run_test.go`, `testdata/e2e/addresses/`):
    - **C++ baked:** `Find("open") == &At(0)`; a resolved getter's address equals `Find` on its
      target; a repeated accessor gives the same address;
    - **C++ types:** `Find(1) == &At(0)`; after `Holder m = std::move(*h)`, `m.GetItems().Find(1)` and
      `m.GetW().Find(3)` give the old addresses;
    - **C++ data:** a row pointer held across `Store::Reload` still reads its old value;
    - **Go:** `At(0) == Find(k)`; a copied `KeyedList` gives the same pointer; a resolved getter
      equals `Find`; the pointer survives `Reload`;
    - **TS:** `find(k) === at(0) === all[0]`.

### Acceptance

- Every check test fails on unchanged production files and passes after the change. The address tests
  pass before and after: they pin a guarantee.
- `make check` is green, and `go test -race` passes on each unit's packages.
- The golden diffs hold only these:
  - check lines;
  - bound constants and T19;
  - the runtime comment (21 `internal` copies and 7 example copies of `canon_runtime_json.h`);
  - T6 (the pipeline and constructs C++ headers, the Go data/pipeline and data/shop outputs);
  - TS key widths.
- About 17 of 38 example directories change: pipeline, resource/events, resource/vocab, game/items,
  sovcommon/time, teamboard, telemetry, service/resourcestudio, features/{dependent, embedded,
  gotypes, legacycpp, refdep, textemit, ts}; features/copies and features/shared change for the
  runtime comment only. This is an estimate from reading the sources; the `-update` diff gives the
  exact list.
- With the new compiler, `canon check` of `testdata-real/sov-v0112` gives no new finding from this
  section, and `make realdata-compile` passes (§12).

### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| IR-RANGE | `internal/ir` (TypeRef, convert, new `bounds.go`, the three plans, dump, findings) | **opus**: frozen contract, three name plans | M | S2 |
| CPP-RANGE | `internal/gen/cpp` + `runtime/canon_runtime_json.h.txt` (comment) + `text/snapshot_load_doc.txt` + its C++ example goldens | sonnet, opus review: many paths (shortcut, pairs, union, rows) | M | IR-RANGE, EN-CPP |
| GO-RANGE | `internal/gen/go` + `text/data.txt` + Go example goldens | sonnet, opus review | M | IR-RANGE, GEN-GO |
| TS-RANGE | `internal/gen/ts` + `runtime/decode.ts.txt` + TS example goldens (+ the held-back §8.2 block) | sonnet, opus review | M | IR-RANGE, EN-TS |
| E2E-RANGE | new test files in `internal/ir` | sonnet, opus review | M | CPP-RANGE, GO-RANGE, TS-RANGE, MACRO-IR |

### Forecast

1. **`-Wtype-limits`.** Never compare an implied side. Compare on `static_cast<int64_t>`. The tests
   compile at `-Wextra -Werror`.
2. **Float32 at a bound.** Compare the rounded value in binary64. Never write a `float` bound or
   compare in `float`. Follow eval, not the static literal check.
3. **Go Float bounds** are typed `float64`: an untyped one rounds the bound.
4. **Go integers.** Compare `int64(len(x))`, since a literal above 2^31 would fail on 32-bit. Integer
   constants are untyped.
5. **TS.** `decInt` refuses unsafe integers before the range check, so the bound literals compare
   exactly. `E8101` is untouched: unsafe constants are not written.
6. **-0 and NaN.** -0 is already normalized before the check. NaN never arrives, and the negated form
   would refuse it anyway.
7. **Huge counts.** Cast to `int64`. TS lengths are below 2^32.
8. **Which error wins:** type, then width, then elements and keys, then duplicates, then count, in
   decode order. C++ `Decode` over parsed JSON meets map entries in key byte order, as for every
   other failure (DECISIONS 361).
9. **Defaults are never checked here.** The checker gap (a const-named default that violates its
   refinement passes `canon check`) is fixed in §8.5, not here.
10. **`@json(none:)`.** The check sits inside the non-none branch, so a marker outside the range is
    still none. Resource's `"="` is included.
11. **Maps.** A key is checked at `<map>.<key>`. The count check goes after `Pop`.
12. **Keyed-list keys** are checked as a row field. The list's count comes after the duplicate check.
13. **Ref keys.** `collRef` must clear `Range`. Otherwise C++ `decodeValue`'s Ref recursion and Go's
    `readRef` would check keys.
14. **C++ shortcuts.** Add the check after them, never after an absent-key default.
15. **Literal unions.** Their literals pass, as in eval.
16. **No clang constexpr step limits apply:** constants are scalars and checks are inline.
17. **Legacy data that will fail:** files Canon never loaded, and data built before a narrowing.
    Sovereign's parity tests and Canon's round trip detect them. The reply says so.
18. **Function size.** Each refined field adds 1-3 lines to `Decode`. The realdata gate measures it;
    size budgets come in v0.3 (ADR-0024 §1.6).
19. **Name collisions.** A new `E8005` in the Resource dry run stops IR-RANGE. It is a real gap, for
    the orchestrator to decide.
20. **TS helpers.** Register them in the helper units, or `noUnusedLocals` breaks the build.
21. **Generator fixtures carry no `Range`.** New tests set it explicitly.
22. **Leave `Field.Range`, `Param.Range` and `ResultRange` in place.** Do not migrate their consumers.
23. **TS lone surrogates.** A lone surrogate in a TS `decode<T>` value counts 3 bytes; `parse<T>`
    refuses it earlier.
24. **`KeyedList` is copyable.** The spec promises addresses for moves and lookups, not for copies.
25. **Golden conflicts.** CPP-RANGE, GO-RANGE and TS-RANGE each regenerate example goldens in their
    own worktree. A cherry-pick conflict in `examples/**/expected` is resolved by regenerating with
    `-update` on the merged tree and reading the diff, never by picking a side (§11.3).

## 4. Enums without `@codes` are as safe as ids (DECISIONS 402; amends 379)

Scope: agreed with Louis on 2026-10-14 (ADR-0021 §9, "Enums, v0.2"; the last sentence of DECISIONS
379). Read-only research over `4dfd414e`, Resource, Source and services/{admin, monitoring, trello,
website, wiki}. Behaviour is taken from the committed goldens and from v0.1.9's id tests, which pass
today.

### Facts

**Go.**
- Enums are integers: `type %s %s\n\nconst (` (`internal/gen/go/constants.go:31`;
  `writeEnum`/`writeClosed`, `enums.go:150-194`).
- Kind enums go through the same writer, with `text: true` (`enums.go:84-99`).
- Branch enums are bare constants with no methods (`dependent.go:41-49`).
- Generated Go uses the integer directly:
  - a Canon `const` of an enum becomes `const X = Member` (`consts.go:27-28`);
  - lookups index arrays by the enum itself (`fns.go:243-258`);
  - translated `<` is native (`pure_expr.go:113-131`);
  - placeholders are an untyped `0` (`foreign.go:202`, `:216`; `slot.go:170`; `conformance.go:242`).
- **The precedent is v0.1.9's ids:** `type %s struct{ position %s }`, with members as vars
  `%s = %s{%s}` (`constants.go:39-42`, `ids.go:4-20`, `text/id.txt`).
  - A failed `Parse` returns `T{}` (`enum_tables.go:26-27`).
  - The members are plain package vars. v0.1.9 neither prevents nor discusses reassigning them.
- **Name plan:**
  - `Position` is reserved for ids only (`internal/ir/tables.go:204-209`);
  - branch enums declare only their members (`goplan_dependent.go:38-58`);
  - `writesTextErrors` ignores dependent types (`goplan_imports.go:256-264`).

**C++.**
- Ids get deleted `to_json`/`from_json` templates (`internal/gen/cpp/idenums.go:56-58`,
  `errors.go:243-245`). The names are reserved only for a baked package with a table
  (`internal/ir/cppplan_baked.go:150-163`).
- Plain enums (`enums.go:102-131`) and branch enums (`dependent.go:37-51`) have no deleted pair:
  `testdata/main/baked_json_ok_main.cpp:11-13` asserts that a plain enum converts through nlohmann.
- Open enums are classes with no conversion operator, so nlohmann already refuses them
  (`errors.go:286-293`).
- Source ships nlohmann 3.11.2.

**TypeScript.**
- Every enum exports `<E>Index` (`internal/gen/ts/enums.go:57`).
- Lookups and translated comparisons read it through `indexConst` (`fns.go:162-182`,
  `tr.go:219-233`).
- Ids already build private indexes (`enums.go:238-257`, `internal/ir/tsplan_imports.go:54-72`).
- tsc runs with `--noUnusedLocals`.

**Resource.**
- 547 enums: 341 with `@codes`, 206 without.
- `ordered` enums: `WebsiteRole`, wiki's `Tier` and `Coverage`, and `MemoryMode` (which has
  `@codes`).
- 16 variants and 5 dependent types.
- 14 open enums, all with `@codes`, all opened in Go `types` emits only.

**Consumers.**
- **Go:** closed enums without `@codes`, plus kind enums: admin 34, monitoring 55, trello 20,
  website 48, wiki 81. Branch enums: 2, 3, 0, 2, 2.
- **C++:** Source holds 141 such `enum class`es (126 enums, 10 kind, 5 branch) in 132 Canon headers.
- **TS:** the services' generated files export 433 `<E>Index` tables.
- **Sovereign's lint.** Every Sovereign repo runs `exhaustive` ("A new enum value must break the
  build", `sovcommon/tools/sovaudit/rules.md:254-256`). That linter sees only enums made of
  constants.
- **`make realdata-compile` compiles generated units only.** It compiles no consumer source, so it
  catches none of the breakage below.

### Problem

A declaration position still leaves the generated code as a number:
- **Go:** `Tone(3)`, `int(t)`, `t+1`, `<` on enums that are not `ordered`, `%d`, `database/sql`
  (stored as int64) and `gob`.
- **C++:** nlohmann converts it silently, both ways.
- **TypeScript:** 433 exported `<E>Index` tables, which monitoring already uses to index grids that
  the server builds.

Adding or moving a member renumbers all of these, and no finding fires.

### Decision

1. **Scope: every generated type whose values are declaration positions.**
   - Included:
     - closed enums without `@codes` (not opened by the emit);
     - variant kind enums, which DECISIONS 379 and 395 already treat as enums;
     - dependent branch enums. wiki writes a branch number today (Forecast), so leaving them out would
       leave a known leak; including them is the strictest reading.
   - Unchanged in every target:
     - `@codes` enums: their integer is the declared, locked code (SPEC §12, LOCK.md);
     - open enums: they hold the wire, never a position;
     - string ids in `data`/`types` modes.
2. **The Go shape is copied from ids:** `type E struct{ position uintN }`, with the field unexported
   and `uintN` chosen as today.
   - Members are package-level `var`s with the same names, in declaration order
     (`ToneWarning = Tone{0}`), with their docs kept.
   - Members stay assignable, as ids are: Go has no struct constants, and writing a member is
     deliberate. The tables `enumMembers_E` and `enumFound_E` hold copies taken at package init, so
     `Parse`, `Members` and every lookup stay correct even then.
   - The zero value `E{}` is position 0, the first member, as today and as for ids. A zero value
     meaning "no member" would make `MarshalText` fail on every zero-valued struct.
   - What still compiles: `==`, `!=`, map keys (JSON writes them through `MarshalText`), and `switch`
     with member cases.
   - What no longer compiles: integer conversions either way, untyped constants, `<`, arithmetic,
     `const`, `iota` and array-literal keys.
   - What changes silently: `%d` prints `{2}` `[unverified, from fmt's rules]`.
3. **Methods.** Each family keeps its methods, now reading `self.position`, and gains
   `Position() int` (fixed comment T20).
   - **Closed enum:** `String`, `Wire`, `Parse<E>` (returns `E{}, false` on failure), `<E>Members`,
     `<E>LiveMembers`, `Retired`, `MarshalText`/`UnmarshalText` (texts as in v0.1.9; the error is
     built with `strconv.Itoa(int(self.position))`).
   - **Kind enum:** `String`, `Wire`, `Parse<K>`, `Retired`, `MarshalText`/`UnmarshalText`.
   - **Branch enum** (no methods today): `String()` returns `ir.Branch.Name`, the branch name
     TypeScript uses. It also gets `Parse<B>(name) (B, bool)` and `MarshalText`/`UnmarshalText` over
     that name, with the error `"<B>: no branch "`. It gets no `Wire`, `Members` or `Retired`.
     Without a text method, an opaque type would silently encode as `{}`.
   - The `E(<n>)` fallback stays, because an empty enum's zero value reaches it.
   - `encoding/json`, `xml` and `yaml` write the wire value or the branch name. `database/sql` and
     `gob` refuse the type.
   - Rejected:
     - `type E string`: any string converts, `<` on an `ordered` enum would compile and compare
       bytes, and every array index becomes a search;
     - functions as members: unreadable, as for ids;
     - a `Compare` method: `Position()` already orders `ordered` enums.
4. **Go generated code.**
   - Every array index is `x.Position()`.
   - A Canon `const` of such a type is written `var X = Member`.
   - Every untyped `0` written for such a type becomes `E{}`.
   - Translated `<`, `<=`, `>` and `>=` on an `ordered` enum without `@codes` compare `Position()`s.
     An operand that is not `inert` is written `(x).Position()`.
   - Domain iteration keeps the array literal of members (`readers.go:178-197`).
5. **The Go name plan.**
   - `Position` joins `goTextEnumMethods`.
   - The field `position` is declared in the scope of every such type, as at `goplan_scopes.go:251`.
   - A branch enum declares `Parse<B>` and its Names/Keys/Found tables in the top scope, and `String`,
     `MarshalText`, `UnmarshalText` and `Position` in its own scope.
   - `writesTextErrors` returns true for a `*Dependent`, and `textImports` marks `strconv` and
     `slices` for one.
   - A collision is the existing `E8005`.
6. **C++.**
   - Kept: `enum class`, its underlying integer, `static_cast` both ways, `ToName`, `ToWire`,
     `<E>FromWire`, `IsRetired`, `k<E>Members`, `k<E>LiveMembers` and `-Wswitch` checking.
   - Added: after each type of point 1, the header writes, in the package namespace and with no
     nlohmann include, the T21 comment line, then `template <typename J> void to_json(J&, E) = delete;`
     and `template <typename J> void from_json(const J&, E&) = delete;`.
   - `declareJSONHooks`, in ir and in gen, reserves both names whenever the namespace gets a pair: a
     baked table, or any type of point 1. The names are shared as overloads, as today.
   - Compilers: g++ and clang++ (libstdc++ and libc++) already refuse ids this way
     (`TestIDEnumRefusesNlohmann`). MSVC is `[unverified]`; deleted function templates and expression
     SFINAE are C++11. If a new test row still compiles, the unit stops and reports a bug: the id rows
     already prove the mechanism.
   - Migration: write `j["k"] = std::string(ToWire(v));`, read `<E>FromWire(j.get<std::string>())`.
7. **TypeScript.**
   - No enum exports `<E>Index`, `@codes` and open enums included: their index is a position too, and
     a code lives in `<E>Codes`.
   - The name stays reserved. A file that uses positions declares
     `const <E>Index = canonIndex(<E>Members);` once, after the enum, as `ownIDIndex` does.
   - Another package's enum gets `const <alias>_<E>Index = canonIndex(<alias>.<E>Members);` beside the
     foreign id indexes.
   - The plan declares `TSForeignEnumIndexName(e)` for every enum of another package that has a ts
     emit and that the file reaches: the same safe over-reservation ids use.
   - Unchanged: `<E>Names` (wire to Canon name), `<E>Members`, `<E>LiveMembers` and `<E>Codes`.
8. **Open enums**, decided with the rest: unchanged everywhere except point 7.
9. **Go's `exhaustive` linter no longer sees these types (flagged for Louis; default: accepted).**
   - Go cannot have a constant that no integer converts to, and "positions are not ids" came first
     (Louis, 2026-10-14).
   - Canon's own `match`, C++'s `-Wswitch` and the generated tables stay exhaustive; so does TS
     narrowing over a string union.
   - The cost: Sovereign's fleet rule "a new enum value must break the build" goes blind on Canon
     enums in Go. The reply gives the way out: a `default:` that returns an error, and a test that
     ranges over `<E>Members()` through each switch.
   - The orchestrator puts this point in the status file before EN-GO starts. If Louis vetoes, EN-GO
     waits (§0).

### Code

- **`internal/ir`:**
  - point 5 (`tables.go`, `goplan_scopes.go`, `goplan_dependent.go`, `goplan_imports.go`);
  - `cppplan_baked.go:150-163`;
  - `tsplan_imports.go`, where `TSForeignEnumIndexName` is declared in `declareImports`.

  All three land in IR-PLANS.
- **`internal/gen/go`:**
  - The closed and kind writers take the id's struct/var formats. Branch enums go through `ids.go`
    with branch names, no `Retired`, and T20 instead of T16.
  - `text/enums.txt` and `text/enum_text.txt` read `self.position`.
  - `enum_tables.go:23-36` sets the zero and the value for every opaque type.
  - Call sites: `fns.go:243-258`, `consts.go:27-28`, `pure_expr.go:113-131`.
  - Zero sites: the four above, plus any further `0` site the golden Go compile names.
- **`internal/gen/cpp`:** `enums.go:102-131`, `dependent.go:37-51`, `idenums.go:118-126` and
  `errors.go` (the T21 formats).
- **`internal/gen/ts`:**
  - `enums.go:57`: record the enum's item, so the private const can follow it;
  - `fns.go:176-182`: `indexConst` returns the file's own private name or the foreign one.

### Spec text

- **CODEGEN §5.2, Go:** replace the Go block and the paragraph after it (`spec/CODEGEN.md:1004-1029`)
  with:
  ```go
  type Tone struct{ position uint8 }

  var (
  	ToneWarning = Tone{0}
  	ToneInfo    = Tone{1}
  	ToneSeries1 = Tone{2}
  )

  func (self Tone) String() string        // Canon name: "series_1"
  func (self Tone) Wire() string          // wire value: "series-1"
  func ParseTone(wire string) (Tone, bool) // Tone{} and false for an unknown wire
  func ToneMembers() iter.Seq[Tone]       // declaration order, retired members included
  func ToneLiveMembers() iter.Seq[Tone]   // the members not retired, in ToneMembers order (DECISIONS 395)
  func (self Tone) Retired() bool         // declared retired; false for a value that is no member
  func (self Tone) MarshalText() ([]byte, error)      // the wire value, for encoding/json and others
  func (self *Tone) UnmarshalText(text []byte) error  // a wire value; an unknown one is an error
  func (self Tone) Position() int         // the declaration position, for array indexes in memory

  type Element uint8                      // values are the codes: ElementFire Element = 1
  func (self Element) Code() uint8
  func ElementFromCode(code uint8) (Element, bool)
  ```
  > An enum without `@codes`, a variant's kind enum (§5.5) and a dependent type's branch enum (§5.6) are opaque like a `baked` id (§5.3): no integer converts to or from one, and `<`, arithmetic and `const` do not compile on it. Members are `var`s (Go has no struct constant) and a Canon `const` of such a type is a `var`; `==`, `switch` and map keys work, and the zero value is the first member. Generated code indexes with `Position()`, and an `ordered` enum's members compare by it. Text encoders write the wire value; `database/sql` and `gob` refuse the type. An enum with `@codes` keeps its integer, the declared and locked code, and has no text methods (DECISIONS 379, 402).
- **CODEGEN §5.2, C++:** after `:1045`, insert
  `template <typename J> void to_json(J&, Tone) = delete;` and
  `template <typename J> void from_json(const J&, Tone&) = delete;`. After `:1064`, add:
  > The deleted pair follows every `enum class` whose values are declaration positions (an enum without `@codes`, a kind enum, a branch enum), as it follows an id enum (§5.3); enumerators, their `static_cast`s, `ToName`, `ToWire` and `<E>FromWire` are unchanged, and an `@codes` enum still converts through nlohmann as its code (DECISIONS 402).
- **CODEGEN §5.2, TS:** delete `:1073` (`ToneIndex`). After the block, add:
  > No enum exports `<E>Index`, `@codes` and open ones included: a declaration position is not an id, and an `@codes` member's stable number is `<E>Codes`. A file whose lookups (§5.10) or translated comparisons (CONFORMANCE.md §3) need positions declares `const ToneIndex = canonIndex(ToneMembers);` once, after the enum, or `const <alias>_<E>Index = canonIndex(<alias>.<E>Members);` for another package's enum, beside the private id indexes (§5.3, §8.2). A program that needs the order reads `ToneMembers.indexOf(v)` (DECISIONS 402).
- **CODEGEN §5.2, rules and open enums:**
  - `:1080`: after "the declaration index (`.index`)", add ": C++'s underlying value, Go's
    `Position()`, a TypeScript file's private `<E>Index`".
  - `:1082`: "compare with the native operators" becomes "compare with the native operators, except
    Go without `@codes`, which compares `Position()`s".
  - `:1134`: drop `DstIndex, ` from the comment.
- **CODEGEN §5.5** (`:1259`):
  - The Go cell becomes: "`type EventKindKind struct{ position uint8 }` (opaque, §5.2) with `var`s
    `EventKindKindSpawnMonster…`, `String()`, `Wire()`, `MarshalText()`/`(*EventKindKind).UnmarshalText`
    (the wire name, DECISIONS 379), `ParseEventKindKind`, `Retired()`, `Position()`".
  - The C++ cell gains ", deleted `to_json`/`from_json` (§5.2)".
- **CODEGEN §5.6:** the Go line `type ParamBranch uint8 // …` becomes:
  ```go
  type ParamBranch struct{ position uint8 } // vars ParamBranchMonster, ParamBranchItem, ParamBranchElement, ParamBranchGameMode
  func (self ParamBranch) String() string   // the branch name, as TypeScript writes it: "monster"
  func ParseParamBranch(name string) (ParamBranch, bool)
  func (self ParamBranch) MarshalText() ([]byte, error)      // the branch name
  func (self *ParamBranch) UnmarshalText(text []byte) error  // a branch name; an unknown one is an error
  func (self ParamBranch) Position() int
  ```
  The C++ `ParamBranch` line gains the comment `// and deleted to_json/from_json (§5.2): store the discriminant`.
- **CODEGEN §2.5:**
  - T18's scope becomes "(ids; enums without `@codes`, kind and branch enums)". Append: "; on a
    branch enum, `// MarshalText writes the branch name, never the position.` and
    `// UnmarshalText reads a branch name; an unknown one is an error.`".
  - Two rows, after §3's T19:
    > | T20 | Go | `Position` of an enum, kind or branch enum | `// Position is the member's place in declaration order, for array indexes in memory;` / `// an ordered enum's members compare by it. It is not an id: it changes when members move.` |
    > | T21 | C++ | deleted `to_json`/`from_json` of an enum, kind or branch enum | `/// nlohmann/json would write the position: write ToWire(v), read <E>FromWire(s).`; on a branch enum `/// nlohmann/json would write the position: store the discriminant.` |
- **CODEGEN §2.8:** at `:483`, "an enum's `<E>Index`" becomes "an enum's `<E>Members`"; at `:486`,
  `sovcommon_roles.RoleIndex` becomes `sovcommon_roles.RoleMembers`.
- **CONFORMANCE.md §3** (`:167`): the Go cell becomes "native; an `ordered` enum without `@codes`
  compares `Position()`"; the TS cell becomes "native (enums through the file's private `<E>Index`,
  CODEGEN.md §5.2)".
- **SPEC.md §4.3** (`:494-500`):
  - "Generated id types are opaque" becomes "Generated id types, and the Go types of enums without
    `@codes`, of variant kinds and of dependent branches, are opaque", and its reference becomes
    "(CODEGEN.md §5.2, §5.3)".
  - "`.index` and the number Go and C++ give … through `MarshalText`)" becomes "`.index`, Go's
    `Position()` and a C++ enumerator's value without `@codes` are declaration positions: Go's text
    encoders write the wire value, nlohmann refuses such a C++ enum, and TypeScript exports no
    `<E>Index`".
- **SPEC.md §15.2** (`:1704-1706`) becomes:
  > Enums with `@codes` are typed integers holding the code, with `Code()` and `<Enum>FromCode`; every other closed enum is an opaque struct with `var` members and `Position()` (no integer converts to or from it, CODEGEN.md §5.2). Both have `String()` (the Canon name), `Wire()`, `Parse<Enum>(wire string) (<Enum>, bool)` and `<Enum>Members()` (declaration order, retired members included).
- **SPEC.md §15.3**, after `:1768`: "nlohmann/json refuses an enum without `@codes` (deleted
  `to_json`/`from_json`): store `ToWire(e)`."
- **SPEC.md §15.4** (`:1829`): "`<E>Members`, `<E>Names` and `<E>Index`" becomes "`<E>Members`,
  `<E>LiveMembers` and `<E>Names`", and append "; no `<E>Index` is exported (§4.3)".

### DECISIONS text

> 402. **Enums without `@codes` are as safe as ids (SPEC §4.3, §15.2-§15.4; CODEGEN.md §2.5, §2.8, §5.2, §5.5, §5.6; CONFORMANCE.md §3; amends 379).** Every generated type whose values are declaration positions (a closed enum without `@codes`, a variant's kind enum, a dependent type's branch enum) is built like a `baked` id. Go: `struct{ position uintN }`, `var` members, its family's methods plus `Position() int` (array indexes in memory; an `ordered` enum's members compare by it); a branch enum gains `String`, `Parse<B>`, `MarshalText` and `UnmarshalText` over its branch names; no integer converts to or from one, `<`, arithmetic and `const` do not compile, text encoders write the wire value, `database/sql` and `gob` refuse it; a Canon `const` of one is a `var`; the zero value is the first member. C++ keeps `enum class` and its casts and deletes `to_json`/`from_json` for each. TypeScript exports no `<E>Index` for any enum, `@codes` and open ones included; a file that needs positions builds them privately with `canonIndex`. `@codes` enums keep their integer, the locked code, in Go and C++; open enums keep their wire. Go's `exhaustive` linter no longer sees these types: accepted. Reason: Louis, 2026-10-14 (ADR-0021 §9): a declaration position left Go through conversions, `%d`, `database/sql` and `gob`, C++ through nlohmann and TypeScript through 433 `<E>Index` tables; a read-only scan found four Go and three TypeScript files to migrate and no C++ one.

"[amended by 402]" goes on DECISIONS 379. No ERRORS.md rows: no new code.

### Tests

Each test fails on `4dfd414e`.

| Package | Test | Proves |
|---|---|---|
| ir | existing plan tests gain rows | `Position`, `position`, `Parse<B>` and `enumNames_<B>` are reserved; `to_json` is reserved for a types-mode C++ enum; `<alias>_<E>Index` is declared |
| gen/go | `TestEnumTypeRefusesIntegers` (`opaque_test.go`, using `TestIDTypeRefusesIntegers`'s `bad.go` helper); `opaquePackage` gains `variant Shape { round, flat }` and a dependent type over `Tone` like `dependent_test.go`'s fixture | each of `opaque.Tone(1)`, `var _ opaque.Tone = 1`, `uint8(opaque.ToneWarning)`, `opaque.ToneWarning + 1`, `opaque.ToneWarning < opaque.ToneSeries1`, `const _ = opaque.ToneWarning`, `var _ = [2]int{opaque.ToneWarning: 1}`, `opaque.ShapeKind(0)` and the branch type `(0)` fails to compile; the control row `ToneWarning.Position() < ToneSeries1.Position()` compiles |
| gen/go | smoke `TestIDTypeOpaque` | JSON writes the kind's wire and the branch's name; `database/sql` and `gob` refuse `ToneSeries1`; `Position()` values |
| gen/cpp | `TestIDEnumRefusesNlohmann`, new rows | under g++ and clang++, `json j = b::Tone::loud`, `get<b::Tone>()`, `json j = b::RewardKind::coins` and `json j = demo::PayloadBranch{}` (from `bakedDependentPackage`) fail to compile; the ok driver round-trips `ToWire`/`ToneFromWire`; `b::Element::FIRE` (`@codes`) converts and reads back |
| gen/ts | `TestNoExportedIndex` (replaces `TestNoExportedIdIndex`) | no generated or example file matches `(?m)^export const \w+Index\b` |
| gen/ts | `TestEnumIndexNotExported` (`toolchain_test.go`) | with every compiler, `import { ToneIndex } from "./x.js"` fails with `TS2305`; `ToneMembers.indexOf("warning")` compiles |

**Drivers and pins updated in the same units:**
- **Go drivers:**
  - `smoke/opaque_test.go:55`: delete `Tone(9)`.
  - `smoke/constructs_test.go:19`: `<` becomes a `Position()` comparison.
  - `large/closed_smoke.tmpl:45,49,52,56` and `kinds_smoke.tmpl:25,28,31`: `got != 0` becomes
    `got != T{}`; the out-of-range constructions are deleted; `int(m)` becomes `m.Position()`.
  - `enum_edge_test.go:59-66`: keep `closed.Z{}.String() == "Z(0)"`; delete `E(7)` and `Z(1)`.
- **Go pins:** `golden_test.go:111`, `dependent_test.go:77`.
- **C++ driver:** `cpp/testdata/main/baked_json_ok_main.cpp:11-13`.
- **TS drivers:** `drivers/baked.test.js:67`, `open_types.test.js:4,13`, `imports.test.js:6,37`,
  `drivers-examples/lookup.test.js:3,8,13`, `teamboard.test.js:4-5`. Each moves to
  `<E>Members.indexOf` and asserts `"<E>Index" in m === false`.

**Goldens that change** (regenerated with `-update`, diff read):
- **Go:** about 27 `.gen.go` files.
  - In `examples`: copies ×2, dependent, lookup, refdep, shared/core, teamboard {roles, teamboard,
    ui}, telemetry.
  - In `internal/gen/go/testdata`: constructs, data/{dependentdefine, dependentdefinebaked, shop base,
    shop shop}, emberfall {baked, data} core, escapes, inputs ×2, mapfields, teamboard ×3, translated,
    unions ×2.
  - Plus any file that indexes or zeroes another package's enum.
- **C++:** about 26 headers (6 in examples, 20 in `cpp/testdata`). No `.gen.cpp` changes.
- **TS:** about 28 files lose their exported indexes (9 in examples, 19 txtar); 10 files that read an
  index get a private one.
- Any other golden diff is a bug, explained before the unit lands.

### Acceptance

- The new tests fail against copies of today's production files and pass with the change.
- `make check` is green.
- `TestLargeEnumsCompile` stays under 1 GB.
- `make realdata-compile` passes (§12).

### Units

| Unit | Exclusive packages | Tier, reason | Size | After |
|---|---|---|---|---|
| (in IR-PLANS) | `internal/ir`: the name reservations of Code | sonnet: pinned by the plan tests | XS | IR-RANGE |
| EN-GO | `internal/gen/go` | sonnet, opus review: many emission sites, the golden Go compile is the oracle | M | IR-PLANS; Louis's call (Decision 9) |
| EN-CPP | `internal/gen/cpp` | sonnet: copies the id pair | S | IR-PLANS |
| EN-TS | `internal/gen/ts` | sonnet: copies the id index | S | IR-PLANS, GEN-TS |

**Order in each generator:** EN-x runs first among the v0.2 units that own that generator, so later
units write their tests against opaque enums. `TestExamples -update` rewrites every target's example
goldens; §11.3's golden-conflict rule applies.

### Forecast

1. **Go consumer breakage** (compile errors), with their migrations:
   - wiki `cmd/bundle/main.go:212`: `TierRank: int(s.Tier())` becomes `s.Tier().Position()`. `Tier` is
     `ordered`, so the rank is the same.
   - wiki `cmd/bundle/adapt_kill.go:131`: `strconv.Itoa(int(p.Branch()))` becomes
     `p.Branch().String()`. The value lands in the bundle's `FaeryRewardIn.Param` (`:63`), and a name
     stays stable when arms move.
   - monitoring `internal/platform/lake/internal/feeddefs/feedlegs.go:230`: `return 0, false` (a
     `telsem.LinkKind`) becomes `return telsem.LinkKind{}, false`.
   - monitoring `…/feeddefs/rolelegs_test.go:229,233`: `telcat.SqlType(255)`. Delete both checks: a
     non-member can no longer be built, and `sqlFold.add`'s `default` stays.
   - Not found: `const`, `iota`, `<`/`>`, arithmetic, array indexes, `%d`, SQL arguments, tagged struct
     fields, or reflection on these types.
   - The scan was textual. Release §12 step 7 runs `go vet ./...` of each service on v0.2's output,
     and its output replaces this list in the reply.
2. **TypeScript consumer breakage:**
   - admin `web/src/lib/machine-review.ts:15,70`: `ReviewTagIndex[hit.tag] ?? RANK` becomes
     `ReviewTagMembers.indexOf(hit.tag)`, with -1 mapped to the unknown rank.
   - monitoring `web/src/lib/modules/players/page/prog/systems.ts:19-20,261,282` and
     `sysparts.ts:8,33` use `ProgressTrackIndex` and `AchievementUnlockKindIndex`. Both enums have
     `@codes`, and the server orders those grid rows by `<E>Members()`
     (`internal/platform/lake/playerpage_prog_time.go:18-22`). Migrate to `<E>Members.indexOf(…)`.
3. **C++ consumers in Source: none found.** No hand-written `to_json`/`from_json`,
   `adl_serializer`, `NLOHMANN_JSON_SERIALIZE_ENUM`, `get<E>` or json assignment of such an enum, in
   the 225 files that use nlohmann outside `External/`. Casts stay legal.
4. **Silent Go changes** (0 known sites each):
   - switches with var cases lose duplicate-case errors;
   - `%d` and `%x` change output;
   - `omitempty` no longer drops the first member.
5. **Generator traps.**
   - `TestLargeEnumsCompile` declares 12,000 `var` members per enum. Ids already pass this way; if it
     goes over 1 GB, the unit stops and reports, and no other representation is tried.
   - Write a private TS index only where it is read (`--noUnusedLocals`).
   - An open enum is never indexed, so its private index never appears.
   - Ordered Go enums with `@codes` stay native.
   - A user name such as `game_core_ToneIndex` meeting the new reservation is `E8005`, with its
     existing way out.
6. **realdata-compile does not catch consumer breakage.** It compiles no consumer source; a missed
   Source site fails Sovereign's own MSVC/CMake build. The gate still proves that the new generated
   code compiles at the consumers' flags.

## 5. C++ names that another package's defines file writes (DECISIONS 403, 404; amends 391)

Answers the open ask [2026-10-29-sovereign-cpp-macro-collisions.md](../handoff/2026-10-29-sovereign-cpp-macro-collisions.md).

### Facts

**The scan.** Source's `Engine/{Resource,Telemetry,Service/Notif}/Generated` hold 238 Canon outputs,
12 defines headers and 32,357 names, none written twice.

**The guard.**
- `internal/gen/cpp/defines.go:80-93`, `macroSet`: the package's own defines plus `Imports[].CppDefines`.
- `:96-116`, `macrosIn`: names met in comments and strings count too.
- `:134-153`, `namespaceBody`: the one site for header, source and conformance files.
- `ir/finish.go:86, 92`.

**Defines and declared names.**
- Defines come from declarations alone (`ir/cppdefines.go:18-36`); table keys are never defines
  (DECISIONS 390).
- Declared names are the plan's scopes (`cppplan_scopes.go:9-61`); table ids come from the evaluated
  `v.IDs` (`cppplan_baked.go:98`). They are judged in `checkCppNames` (`rules_names.go:75`).
- `ir.Build` sees only the import closure (`stage.go:158`), while `build` parses every unit
  (`project/sources.go:177`).

**Headers.**
- 32,350 pairs name a define in code. 32,348 are guarded in the header's `push_macro` list.
- **2 are not:** `BADGE_REQ_USE_ITEM` and `BADGE_REQ_FAERY_RANK`, in `progression/quests.gen.h:1782-1785`,
  against `items/badge.defines.gen.h:3-4`.
- 131 more name a define in a comment only (79 guarded, 52 not); these are harmless.
- **Sources:** the same 2 appear in `quests.gen.cpp`; 24 pairs rely on the read rule; 37 are wrapped
  for a string or comment alone.

**Case 1** is one package pair: `progression.quests` <- `items.vocab.badge`. `quests` imports
`items.vocab`, so there is no read edge. It is not one enum declared twice:
- the cases of variant `Badge` (`quests/badge.canon:11-30`; kind enum values 0, 1);
- `BadgeRequirement` (`items/vocab/badge/badge.canon:9`; `@codes(UInt8)` 5, 6;
  `@cpp(defines: "BADGE_REQ_")`).

They share names and differ in values. Hand-written code spells neither `BadgeKind::` nor
`BadgeBadgeReq*`: `Badge.cpp:87` calls `AsBadgeReqUseItem()`.

**Case 2.**
- `Resource/Server/UI/WindowIds.h` has 1,263 defines: 1,051 `APP_*` and 212 `WIDC_*`. They are
  exactly the members of `WindowId` and `ControlId` in `ui.gen.h`: same names, codes and order.
- Resource renders it with `@text("WindowIds.h") export fn windowIdsHeader()`
  (`ui/ids.canon:8-29`).
- `ui` already guards `WndStyle.h` with `@cpp(defines: "")` (`wndStyleData.canon:4-53`).

### Problem

The guard covers a package and what it reads. So a header that names another package's define
compiles in one include order only. `check` never saw it, and `build` wrote it.

### Decision

1. **Refuse; do not widen the guard.** The guard stays own-plus-reads, so a package's output stays a
   function of what it reads.
2. **The finding.**
   - A selected package with a planned cpp emit fails with `E8005` `foreignDefine` for each name its
     C++ name plan declares that is a define of the project and not in `ir.CppGuardNames(p)` (own
     defines plus those of `p.Imports`).
   - The plan's names cover every scope, the shared names, namespace segments, and the members of
     table ids, kind enums and branch enums.
   - It is reported at the declaring item (at the emit when there is none), once per (name, origin).
   - It is `E8005`, not `E8011`: two generated names collide, and the message names both.
   - `macroSet` uses the same function, so the check and the guard cannot drift.
3. **Project defines.**
   - They come from every package whose first `emit cpp` is `baked` (default), `data` or `types`,
     selected or not.
   - They are read from source files and declarations only, never evaluated: non-local enum members
     and consts with `@cpp(defines:)`, by §7.9's prefix rule. Only the declaring side evaluates (table
     ids of a selected package, which stage A already forced).
   - A name keeps its first source in package-name order.
   - Packages under an absent optional root are not scanned, so they are not judged.
4. **Scope.** `ir.Input.Units` carries every scanned package (`nil` means the program's), so
   `check P`, `build P` and a whole run agree on P.
5. **When.** It runs in stage E, beside `checkCppNames`, which owns the plan. Phase 2 has no plan,
   and gen/cpp does not run in `check`.
6. **Case 2 needs no Canon change:** `@cpp(defines: "APP_")` on `WindowId` and `"WIDC_"` on
   `ControlId`. The wrap is by name, so `WindowIds.h` is guarded too. An annotation on `emit text` is
   declined: a second mechanism for one meaning (319).
7. **`macrosIn`** ignores `//` and `/* */` comments and `"..."` and `'...'` literals (DECISIONS 404).
8. **Limit:** a local or a parameter of a translated fn is no plan name. The C++ compiler reports a
   clash there.

### Code

```go
// ir/stage.go: Input gains Units []*project.Unit; build/pipeline.go:349 passes Units: r.s.units
// ir/cppdefines.go: func CppGuardNames(p *Package) []string // CppDefineNames(p) + each p.Imports[i].CppDefines
// ir/cppforeign.go: type defineSource struct{ Pkg, Origin string }; func (s *stage) projectDefines() map[string]defineSource // lazy, once
// ir/rules_cppforeign.go: func (s *stage) checkForeignDefines(u *unit, es *emitSite, pl *CppNamePlan) // end of checkCppNames
// gen/cpp/defines.go: macroSet from ir.CppGuardNames; macrosIn skips comments and literals
```
The names checked are the sorted keys of each `pl.scopes[i].names`, then `pl.shared`, then the
namespace segments. The finding is `diag.E8005.AtForeignDefine(span, name, a, b)`, where `a` is the
item's origin and `b` is `pkg.Enum.member` or `pkg.CONST`.

### Spec text

- **CODEGEN.md §7.9**, after the last bullet:
  > - The wrap covers the defines of the package and of the packages it reads, so that a package's output is a function of what it reads. A name that a package's generated C++ declares (any name of §3.5's plan: enumerators, kind and branch enum members, constants, table ids, members, types, namespace segments) and that a package it does not read writes as a `#define` is `E8005` `foreignDefine`: a translation unit that includes that defines header first could not compile it. The way out is `@cpp(name:)` on the declaration, or a use of a type of the defining package, which makes this one read it. A package writes defines when its first `emit cpp` is `baked`, `data` or `types`. A local or a parameter of a translated fn is no plan name: the C++ compiler reports a clash there.
  > - A name met only in a comment, a string literal or a character literal is not wrapped: a macro is never expanded there.
  > - A header that `emit text` renders is plain text to Canon. A project that includes one beside generated code puts `@cpp(defines: "P")` on the enums or constants whose names it defines: the wrap is by name, so it guards that header's macros as well as the defines header's.
- **CODEGEN.md §3.5**, after "so they never meet." (`:869`):
  > A generated C++ name that a package this one does not read writes as a `#define` is `E8005` (§7.9).

### ERRORS.md (applied by S2)

- The `E8005` codes-row meaning becomes: `two generated names collide in one scope (two packages a Go
  emit reaches included), or a C++ name equals a #define of a package it does not read`.
- A message row after `E8005 | define`:
  `| E8005 | foreignDefine | name:Name, a:Name, b:Name | `{name} is declared for {a} and written as a #define for {b}, in a package this one does not read, so its headers cannot guard it; rename {a} with @cpp(name:), or use a type of that package here` |`

### DECISIONS text

> 403. **A name another package writes as a `#define` is `E8005` (CODEGEN.md §3.5, §7.9; amends 391).** A generated C++ name equal to a define of a package the declaring package does not read is `E8005` `foreignDefine`, judged from the project's declarations alone, in `check` and `build`, whatever the selection. Reason: 391's guard stops at the read edge, so Sovereign's `BadgeKind` compiled in one include order only; widening the guard would make output depend on which packages and roots a machine holds.
>
> 404. **`push_macro` guards names met in code (CODEGEN.md §7.9; amends 391).** A name met only in a comment, a string or a character literal is not wrapped. Reason: 116 pairs of Sovereign's output were wrapped for names no preprocessor expands there.

"[amended by 403, 404]" goes on DECISIONS 391.

### Tests

- **`internal/ir/testdata/findings/E8005_77.txtar`:**
  - `a`: `enum Kind @cpp(defines: "")`;
  - `b`, with a member of that name and no read edge: reports;
  - `c` (a field of type `a.Kind`), `d` (`@cpp(name:)`) and `e` (no cpp emit): do not report.
- **`E8005_78`:** the `Badge` shape (a variant case). **`E8005_79`:** a `const` define against a table
  id.
- **`internal/build/testdata/findings/E8005_foreign_define_scoped.txtar`** (`select b`, `check-only`):
  `b` alone gives `E8005_77`'s finding.
- **`internal/ir/cppforeign_internal_test.go`:**
  - `TestProjectDefines`: prefix, consts, local, `embedded`, emit-less, `Units` nil;
  - `TestCppGuardNames`: against gen/cpp's set.
- **`testdata/e2e/cpprun/defines_unread.txtar`**, `TestCppDefinesUnreadCompileAndRun`: both ways out.
  One TU includes every defines header, then every `.gen.h` and `.gen.cpp`; another the reverse; both
  under `-Werror`.
- **gen/cpp:**
  - the `tokensCases` row "in a string" becomes `nil`; add comment and character rows;
  - the `\b` oracle of `TestMacrosInMatchRegexp` and `FuzzMacrosIn` strips comments and literals
    first;
  - a golden with a macro in a doc comment and in a string constant shows no pragma.

### Acceptance

- The new txtars fail on the unchanged files and pass with the change; `make check` is green.
- Whole-project `check` of the real copy: exactly 2 new findings, time within 2%, RSS +10 MB at most.
- `make realdata-compile` runs on the scratch copy patched with `@cpp(name: "BadgeReqUseItem")` and
  `@cpp(name: "BadgeReqFaeryRank")` on the two cases (§12, RD-PATCH). The accessors keep their names
  (precedent: log-2026-10-22), and the reply tells Sovereign to make the same edit.
- The 116 pairs lose their pragmas; nothing else changes.

**Alternative W (not chosen): the guard set is every define name of the project.**
- Cost: a field on the frozen `ir.Package`, a 32,357-name set per emit, and the same `Units` scan.
- A package's bytes would depend on every scanned package, so a consumer root or a machine lacking an
  optional root (E8023; DECISIONS 332, 343) would write other bytes.
- A hand-written `Enum::NAME` where the macro is live would still fail.

### Units

| Unit | Exclusive packages | Tier | Size | After |
|---|---|---|---|---|
| MACRO-IR | `internal/ir`, plus the one granted line `internal/build/pipeline.go:349` | sonnet, opus review | M (140 lines, 300 test) | IR-PLANS, BUILD-LINKS |
| (in CPP-OWED) | `internal/gen/cpp`: `macrosIn`, and `macroSet` from `ir.CppGuardNames` | sonnet | S (45, 90 test) | MACRO-IR, CPP-RANGE |

### Forecast

1. Reuse `defineName`, `annotation`, `arg`, `constString`, `local` and `sourceFiles`. Extract
   `readOptions`' mode switch into one helper. Write no second prefix rule.
2. `macroSet` calls `CppGuardNames`, and a test asserts that both agree on a package with imports.
3. Sort map keys, and dedupe across scopes and `pl.shared`.
4. The `ir` harness has `Units` nil, which falls back to `Program.Packages`; the scoped case lives in
   `internal/build`. The real-data run must show exactly two findings. Any other is a plan name the
   scan missed: stop and log.
5. **The comment and literal skipper.**
   - A `//` comment ends at the first newline, even after a backslash; over-wrapping is safe.
   - Digit separators and raw strings occur in none of the 238 real files.
   - Write it as a table of byte-class handlers (go.md §2: no bare literals but 0 and 1, functions at
     most 60 lines).
   - Keep the scale test green, and review each golden diff as removed push/undef/pop trios.

## 6. Wire names (DECISIONS 405, 406; amends 383, 393)

### Facts

- **Where wire strings are read** (`internal/check/wire.go`):
  - field `@json("w")`: `:24-27`;
  - `path:`: `:33-46`;
  - `pairs:`: `:240-262`;
  - enum `= "w"` or `@json("w")`: `:415-440`;
  - variant `tag:`: `:460-466`;
  - case: `:469-478`.
- **Nothing compares two members' or two cases' wires.** `completeEnum` (`decls.go:211-232`) checks
  names only (`E2106`). Decoding takes the first match (`wire/decode_scalar.go:171-177`,
  `decode_record.go:354`).
- **NUL.** A raw NUL is `E1124` (`syntax/lexer.go:110-123`), but `\u{0}` passes (`literal.go:52-58`).
  So a NUL reaches a wire only that way. C++ reads names as `const char*`
  (`canon_runtime_json.h.txt:1000, 1011, 1134-1160, 1199, 1324`).
- **The Resource scan:**
  - 544 enums with 38,931 members (retired and the 14 open enums included; 29 with custom wires), and
    16 variants with 67 cases;
  - two members on one wire: **0**; wires equal but for case: **0**;
  - U+0000 or a NUL byte in any `.canon`: **0**.

  No exception is justified.

### Problem

The later of two members on one wire round-trips as the first. A NUL ends a name read as a C string.
Nothing refuses either.

### Decision

1. **No NUL in a wire name.**
   - A wire name written in source holds no U+0000: `E3316` `nul`, at the literal, for every target.
   - This covers a field's `@json("w")`, a `path:` segment, a `pairs:` template, `tag:`, a case wire,
     and an enum's `= "w"` and `@json("w")`.
   - Values, map keys and `none:` markers are not names.
   - No way out is needed: a NUL name has no use.
   - The text stays as written, so no second finding follows.
2. **Unique wires.**
   - Wires are pairwise distinct within an enum (retired members and `@json(codes)` enums included)
     and within a variant: `E3316` `enumWire` or `caseWire`, at the later name.
   - A wire holding a lexer error clashes with nothing (215).
   - No exception.
3. **Where.** `internal/check` only, in the loops that report `E2106`. Generators keep their
   first-wins tie-break, now unreachable through `check`, and goldens do not change. IR-built
   generator fixtures bypass `check`; leave them alone.

### Code (`internal/check`)

```go
// wire.go: func (c *checker) wireText(env *env, v syntax.AnnValue, item string) (string, bool) // annString, then E3316 nul on U+0000
// callers: fieldAnnotations, jsonPath, pairKeys (gains the field), enumMember (both forms), variantAnnotations and caseAnnotations (gain env)
// decls.go: completeEnum keeps wires map[string]*types.Member; completeVariant checks c.cases[v] after its loop
```

### Spec text

- **WIRE.md §4.2**, before the closing `---`:
  > **Wire names of members and cases** [`E3316`]. The wire values of an enum's members (retired included: `= "w"`, `@json("w")` in an enum with `@codes`, else the name) are pairwise distinct, and so are the wire names of a variant's cases; the second is `E3316`, at its name, since decoding answers the first and a retired member's wire stays reserved as its code does (§5.3). A wire name written in source (a field's `@json("w")`, a `path:` segment, a `pairs:` template, a variant's `tag:`, a case's or an enum member's wire) holds no U+0000: generated C++ readers compare names as C strings and would cut it (`E3316`). No legitimate use exists, so no way out is needed; write another character. Values, map keys and `none:` markers may hold U+0000.
- **WIRE.md:**
  - §5.3's first bullet: append "Wire values are unique within an enum (§4.2)."
  - §5.6's second bullet: append "Case wire names are unique within a variant (§4.2)."
- **TYPES.md §8.1**, after the bullet that lists `.wire`:
  > - The wire values of an enum's members are pairwise distinct (WIRE.md §4.2), so `.wire` names one member.
- **CODEGEN.md:**
  - `:1033`: replace "equal keys or wires resolve to the first declared member (DECISIONS 393)" with
    "keys and wires are unique (WIRE.md §4.2, TYPES.md §9.3), so each finds one member (DECISIONS
    393)".
  - `:1057-1058`: replace "(equal wires keep declaration order, so the first declared wins)" with
    "(wires are unique, WIRE.md §4.2)".

### ERRORS.md (applied by S2)

- The `E3316` codes-row meaning ends: `... colliding wire keys, a wire name holding U+0000, or two
  enum members or two cases on one wire`. Its owner gains §4.2, §5.3 and §5.6.
- Messages, after `E3316 | pairsDefault`:
  `| E3316 | nul | item:Name | `a wire name of {item} holds U+0000 (NUL): generated C++ readers compare names as C strings and would cut it; write another character` |`
  `| E3316 | enumWire | wire:Text, member:Name, other:Name | `wire "{wire}" of {member} is also the wire of {other}; give one of them another wire with = "..." (@json("...") in an enum with @codes)` |`
  `| E3316 | caseWire | wire:Text, caseName:Name, other:Name | `wire "{wire}" of case {caseName} is also the wire of case {other}; give one of them another wire with @json("...")` |`

### DECISIONS text

> 405. **A wire name holds no NUL (WIRE.md §4.2; `E3316` `nul`).** A wire name written in source with U+0000 is refused for every target. Reason: C++ readers take names as C strings and cut it (log-2026-10-21, owed); a name with a NUL has no use.
>
> 406. **Wires are unique within an enum and a variant (WIRE.md §4.2, §5.3, §5.6; TYPES.md §8.1; `E3316` `enumWire`, `caseWire`; amends 383, 393; closes the owed items of log-2026-10-21 and -22).** Reason: the later member decoded as the first. Resource has no use: 544 enums, 16 variants.

"[amended by 406]" goes on DECISIONS 383 and 393.

### Tests (`internal/check/testdata/findings/`)

- `E3316_9`: `nul` on a field, a `path:` segment and a `pairs:` template.
- `E3316_10`: `nul` on a plain enum, an `@codes` enum, a `tag:` and a case.
- `E3316_11`, `enumWire`:
  - `a = "x", b = "x"`;
  - a name against another member's wire;
  - a retired duplicate;
  - an `@codes` enum with equal `@json`;
  - an `@json(codes)` enum.
- `E3316_12`: `caseWire`.
- `E3316_13`, zero findings:
  - wires that differ only in case;
  - equal wires in two different enums;
  - U+0000 in a `none:` marker, in a string value and in a map key.
- `internal/ir/testdata/goplan/enum_tables.txtar` is the one fixture that runs equal wires through
  `check` (`Twin`). It becomes `Twin { a = "x", b = "y" }`, with "custom wires" as its first line and
  its golden written by `-update`. IR-PLANS makes this edit ahead of CHECK-OWED (distinct wires are valid today), so CHECK-OWED touches no `internal/ir` file.

### Acceptance

- The new txtars fail before the change and pass after.
- With IR-PLANS' edit landed, `go test ./internal/...` stays green when the check lands; a red `goplan` test means a second fixture holds equal wires: fix the fixture, never the rule.
- `canon check` of the real copy: no new finding.
- `make check` is green.

### Forecast

1. Keep the NUL text as written; do not reset the wire.
2. Skip a member whose `@json` or value holds a lexer error (`holdsLexError`); otherwise it clashes
   with a name.
3. `completeEnum` skips a repeated name (`E2106`): add no wire for it.
4. Findings born in `completeEnum` and `completeVariant` must replay on re-check, as `E2106` does: run
   `go test ./internal/check -run Recheck`.
5. Keep `E3317`. Run `./internal/testkit/...`, since progen may emit wires.

## 7. `Decode<Fn>File` for a fn named `...File` (DECISIONS 407; amends 340, 363)

### Facts

- **Names are formed in `internal/ir`, once per target.** The generators only print them.
  - Go: `goplan_open.go:37-40` (`goExported(fn.Go, fn.Name)` + `File`).
  - C++: `text_plans.go:8-11` (`FnName(fn)` + `File`).
  - TS: `text_plans.go:30-34` (`cppUpperCamel(tsEffective(...))` + `File`).
  - The suffix is at `constants.go:355`.
- **The Resource copy.** 242 `@text` fns, 199 of them ending in `File`. Only a map, list or keyed-list
  result gets a decoder: 72 do, 71 of them ending in `File`, all C++ (`cpp types`; none in Go or TS).
- **Source.** Its headers hold 70 such `...FileFile` names, called by hand in
  `Engine/Resource/Text/TextJson.cpp` (68) and in two files under `Game/Progression/Ranking/`.
- No package has both `x` and `xFile`, and no golden holds `FileFile`.

### Problem

`tipFile` gives `DecodeTipFileFile` (Go, C++), `decodeTipFileFile` and `parseTipFileFile` (TS), and
the error label `TipFileFile`.

### Decision

- `<Fn>` is the UpperCamel of the fn's name in the target, without a final `File`. The names stay
  `Decode<Fn>File` (Go, C++), `decode<Fn>File` and `parse<Fn>File` (TS), with the label `<Fn>File`.
- The test is case-sensitive, on the effective name after `@go`, `@cpp` or `@ts(name:)`.
- `items` and `itemsFile` in one package now collide: `E8005` `both`, with `(name:)` on one fn as the
  way out.
- This breaks the API, with no alias (319; no promise before 1.0). The generators do not change.

### Code (`internal/ir`)

```go
// text_plans.go: func textFileName(name string) string { if strings.HasSuffix(name, goTextFileSuffix) { return name }; return name + goTextFileSuffix }
// CppNamePlan.TextFile: textFileName(pl.FnName(fn)); GoNamePlan.TextFile (goplan_open.go): textFileName(goExported(fn.Go, fn.Name))
// TSTextFunctions: file := textFileName(cppUpperCamel(tsEffective(fn.TS, tsLowerCamel(fn.Name))))
```

### Spec text

CODEGEN.md §5.13 (`:1789-1790`): replace "`<Fn>` the UpperCamel of the fn's name in the target (TS
`items` gives `decodeItemsFile`; DECISIONS 340, 363)" with:
> `<Fn>` the UpperCamel of the fn's name in the target without a final `File` (TS `items` and `itemsFile` both give `decodeItemsFile`: two such fns in one package are `E8005`, the way out `@go(name:)`, `@cpp(name:)` or `@ts(name:)` on one of them; DECISIONS 340, 363, 407). A decoder's errors name `<Fn>File`.

### DECISIONS text

> 407. **A decoded `@text` fn's `<Fn>File` never doubles `File` (CODEGEN.md §5.13; amends 340, 363).** `<Fn>` is the target's UpperCamel fn name without a final `File`, so `tipFile` gives `DecodeTipFile`, `decodeTipFile`, `parseTipFile` and the label `TipFile`; two fns reaching one name are `E8005`. Breaking in Go, C++ and TypeScript, with no alias. Reason: `...FileFile`, owed since 0.1.10 (ADR-0022 §10). Sovereign: 70 C++ names in three files, a mechanical `FileFile` to `File`.

"[amended by 407]" goes on DECISIONS 340 and 363.

### Tests (`internal/ir`)

- `testdata/textdecoders_targets/file_suffix.txtar` (golden by `-update`):
  - `@text` fns `itemsFile`, `tipFile` and `counts`, in go, cpp and ts types emits;
  - the plan lines hold `DecodeItemsFile`, `DecodeTipFile`, `DecodeCountsFile` and the ts pairs.
- `testdata/findings/E8005_83.txtar`: `items` and `itemsFile` in one package, with three types emits:
  three findings.
- `testdata/e2e/cpprun/file_suffix.txtar`, `TestCppFileSuffixCompileAndRun`: `DecodeTipFile` compiles
  under `-Werror`, decodes a file, and its error names `TipFile`.

### Acceptance

- `go test ./internal/gen/... ./examples/...` shows no golden diff: the generators print plan names.
- The real copy checks with no new finding, and 71 plan names lose one `File`.
- `make check` is green.

### Unit

In IR-PLANS (`internal/ir`; sonnet; S: about 12 lines and 3 fixtures).

### Forecast

1. One helper serves both the names and the labels. Do not edit the generators.
2. A clash with a record's decoder name (`Decode<T>`) is reported by `declareTextDecoders`. Resource
   has none.
3. Test the text after overrides, not the Canon name.
4. The reply to Sovereign lists the 70 call sites.

## 8. Checker and evaluator gaps (DECISIONS 408-412)

These close Louis's v0.2 picks `E3804` and `E1905`, the "Still open" checker items in DECISIONS
(`DECISIONS.md:3084-3110`), log-2026-10-29's owed R7, and one gap found while planning (§8.5).

The research ran no probe. Every "Today" claim not cited from a committed golden is marked
`[unverified]`, and every unit starts by writing its failing test (§0).

| Item | Verdict | Unit |
|---|---|---|
| E3804: dependent maps kept through static typing (338) | fix | DEP |
| E1905: amend path `dm[k]` | fix | DEP |
| `Target(sub.kind)` with `sub: Sub(goal)`: E3003 | fix | DEP |
| R7: a level-1 ref in a spread-built instance | fix | EDIT |
| An edit-layer amendment whose right-hand side is `load(…)` | fix | EDIT |
| E3806 on a dependent argument | confirmed; spec text and a pin | DEP |
| TYPES §10.2: an optional collection field | spec text | S1 |
| An op's list or brace literal whose branch cannot be computed | spec text and a pin | EDIT |
| E15 and a map with dependent values | pin | EDIT |
| R6: the cause of a fold-only constant | spec text and a pin | EDIT (api test) |
| VIEWMODEL J13/J14 through a ref | stays open | none |
| A folded-constant default (found while planning) | fix | CHECK-OWED |

**Fixture F**, shared by §8.1 and §8.2:
```
-- a/a.canon --
package a

local enum K { num, sub }

local record Node {
  k: K
  max: Int = 5
}

local let nodes: table Node = { a { k: sub }, d { k: sub, max: 7 }, n { k: num } }

local record Sub(e: Node) {
  lim: Int = e.max
}

local type P(e: Node) = match e.k {
  num => Int
  sub => Sub(e)
}
```

### 8.1 Kept dependent maps (DECISIONS 408; amends 338)

**Facts.**
- DECISIONS 338 (`DECISIONS.md:3314-3330`) makes two things `E3804`:
  - `union` on a map whose type mentions a key binder;
  - a literal typed against a static view.

  It records "keeping the dependent type through static typing" as owed for v0.2, and Louis took it
  (log-2026-10-14, "v0.2 picks"). The reviews that failed had rebuilt a binder from a static view
  (log-2026-10-08, "338 narrowed", rounds 3-5).
- **Every read erases a dependent map.** `staticView` turns `{x in c: V}` into `{ref c: V'}`
  (`check/dependent.go:34-36`). Its read call sites are:
  - `check/ident.go:289`, `:295`, `:320`;
  - `chain.go:238`; `qualified.go:104`; `index.go:86`, `:89`;
  - `lists.go:189`, `:191`; `call.go:117`, `:264`; `methods.go:48`, `:52`.
- **After that erasure, nothing binds the binder.** `unboundLiteral` refuses the record values
  (`bind.go:98-106`), and `unionOnKeyDependent` refuses `union` on any `ArgKey` (`keydep.go:12-69`).
- **A literal typed against a `DepMapType` does bind** its binder (check `maps.go:14`; eval
  `lists.go:129-137`). `types.ChainArg` reaches a branch's own parameters through the binding's
  `field` (check `bind.go:73-87`; eval `params.go:201-214`).
- **An open map** is one whose value reads an earlier field or a parameter. Read away from its
  instance, it cannot compute `T(k)`, and verification stops with `ErrUnjudged`
  (`verify/dependent.go:117-133`).
- **Today's output**, from the committed golden `findings/E3804_9.txtar`, for
  `byEv: {x in evs: L(x)}`:
  ```
  error[E3804]  a/a.canon:36:33
    union is not available on a dependent value (L(*))
  error[E3804]  a/a.canon:37:46
    a literal is not available on a dependent value (L(*))
  ```

**Repro:** F plus the lines below (this is also the eval test).
```
local record H {
  by: {x in nodes: P(x)} = {}
}

local let hs: [H] = [{ by: { a: {}, n: 1 } }]
local let merged: {x in nodes: P(x)} = hs[0].by.union({ d: {}, n: 2 })
local let same: [Bool] = [hs[0].by == { a: {}, n: 1 }, hs[0].by != { a: { lim: 4 }, n: 1 }]
local let bad: {x in nodes: P(x)} = hs[0].by.union({ d: 3 })
```
- **Today `[unverified]`**, by the golden's shape: `E3804 union` at `merged` and at `bad`, and
  `E3804 a literal … (Sub(*))` at `same`'s two record literals.
- **Correct:**
  - `merged = {a: Sub{lim: 5}, n: 1, d: Sub{lim: 7}}`;
  - `same = [true, true]`;
  - `E3802` at `bad`'s `3`.

**Problem.** Merging or comparing dependent maps needs a declared field or a `var` loop. All 14
findings of E3804_9, the 3 of E3804_10 and 4 of E3804_11's 6 rows exist only because the read forgot
the type.

**Decision.**
1. **Kept maps.** A dependent map `{k in c: T}` is kept when `T` reads `k` and every type argument in
   `T` is `k` or the binder of a dependent map inside `T`. There were two readings: keep every
   dependent map, or only these. The stricter one wins, because an open map's values cannot be judged
   away from their instance.
2. **Reads keep it whole.** A read keeps a kept map's declared value type. Reads are names, fields,
   qualified lets, elements, map and iteration values, fn results, and the value type of map methods.
   Everything else erases as today (§11.3-11.4).
3. **Literals bind.** A literal typed against a kept map binds `k` to each of its keys. Each value is
   checked against `T`'s branches, and at evaluation against `T(key)` (`E3802`). This covers `==`,
   `!=`, `union`'s argument, `if` branches and list elements, at top level and in fn bodies.
4. **`union`.**
   - On a kept map, `union` checks `b` against the receiver's type and returns that type.
   - On any other map whose type reads a key binder, `union` stays `E3804`.
   - A literal against a static view stays `E3804` "a literal".
5. Other map methods keep their signatures. What they return (`filter`, for example) is not a
   dependent map.
6. Storage points keep today's expected types: record fields, defaults, and the map's own values. A
   kept map converts like its erased view, unless the two types are identical.
7. No new code and no new message.

**Code.**
- **`internal/types`:**
  - `binders.go` gets `func KeptDepMap(d *DepMapType) bool`. It walks through optionals, lists, maps,
    dependent maps, pairs, literal unions and application `Args`, never into a function body.
  - `judge.go`: `assignToMap` and `assignToDepMap` accept a `*DepMapType` source as
    `{ref Coll: Value}`.
- **`check/dependent.go`:**
  - Today's function is renamed `erasedView`.
  - `staticView` sets a new eraser field, `keep`; its `DepMapType` case returns `t, false` when
    `keep && types.KeptDepMap(x)`.
  - Only the read sites in Facts keep `staticView`. Every other caller becomes `erasedView`:
    `braces.go:22`, `records.go:92`, `maps.go:135`, `convert.go:206`, `lists.go:99`,
    `enumreflect.go:75`, `dependent.go:141`, `:144`, `:251`.
- **`check/convert.go`:** `:205-206` becomes `c.convert(erasedView(sb), want)`. `convertDepMap`
  (`:308`) erases a `DepMapType` source first.
- **`join2`** (`check/lists.go:88-100`):
  - the empty-map rows accept kind `DepMap`;
  - two identical kept maps join to the first;
  - anything else is `types.Join` of the erased views.
- **`check/keydep.go`:**
  - `keyApp` returns "" for a kept map.
  - Before `methods.go:23`, a kept receiver goes to a new `keptUnion`, which keeps `misplacedArgs`
    and `union(b:)`'s arity as today, checks the argument against `recv`, records `info.Calls[x]` as
    `applyRows` records the map row (TypeArgs `ref c`, `staticView(d.Value)`), and returns `recv`.
- **`check/maps.go:14`:** for a `DepMapType` `t`, set
  `ve.bind = env.bind.forField(t).withBinder(d.Binder)`.
- **`eval/lists.go:137`** mirrors that change for a `DepMapType` `m.T`. It uses a new
  `func (cx *depCtx) inDepMap(t types.Type, binder string, key value.Value, at syntax.Node) *depCtx`
  in `params.go`: `withBinder` plus `field = t`.
- **`eval/std/aggregate.go:132-144`:** for a `DepMapType`, the key type is `ref Coll` and the value
  type is `AnyType`; the value is judged where it is stored.

**Spec text.**
- **TYPES §11.5** (`spec/TYPES.md:1091`), replacing "On a map whose type mentions a key binder,
  `union` is `E3804` (§11.4, DECISIONS 338).":
  > A dependent map is **kept** when `T(k)` reads `k` and every type argument in it is `k` or the binder of a dependent map inside it. A read of a kept map (a name, a field, an element, a map value, a call's result) has the map's own type, not the static view, and a literal typed against it binds `k` to each of its keys: `m == { a: … }` and `m.union({ a: … })` check each value against `T` applied to its key. `union` on a kept map takes and returns its type; on another map whose type reads a key binder, it is `E3804` (§11.4; DECISIONS 338, 408).
- **TYPES §11.4** (`spec/TYPES.md:1066`): "or a declaration of the dependent type (DECISIONS 338)"
  becomes "a declaration of the dependent type, or a kept dependent map (§11.5) (DECISIONS 338,
  408)".
- **STDLIB §6** (`spec/STDLIB.md:330`): "Not on a map whose value type depends on its key (`E3804`)"
  becomes "On a kept dependent map (TYPES.md §11.5) `b` and the result have the receiver's type; on
  another map whose type reads a key binder, `E3804`".

**DECISIONS text.**
> 408. **Kept dependent maps (TYPES.md §11.4, §11.5; STDLIB.md §6; amends 338).** A dependent map `{k in c: T(k)}` whose value reads `k` and no type argument but `k` or the binder of a dependent map inside it keeps its type at every read, and a literal typed against it binds `k` per key: `==`, `!=` and `union` with a literal check each value against `T(key)`. `union` on it takes and returns its type. A dependent map whose value reads anything else (an earlier field, a parameter) still reads as its static view, and 338's `E3804` stays there: away from its instance its values cannot be judged. Other map methods return no dependent map. Reason: 338's owed item; the reviews' unsoundness came from rebuilding a binder from a static view, and keeping the declared type rebuilds nothing (log-2026-10-08).

"[amended by 408]" goes on DECISIONS 338.

**Tests.**
- **Move to accept:** the sources of `findings/E3804_9` and `E3804_10` move to
  `check/testdata/accept/depmap_kept.txtar` and `depmap_union.txtar`, with headers citing DECISIONS
  408. Both findings files are deleted.
- **Shrink:** `E3804_11` is regenerated. `viaValues` and `viaChecked` stay; the other four rows and
  the `E3305` go; the header is rewritten.
- **New `findings/E3804_13.txtar`.** Its golden is three `E3804`: "a literal", union, union.
  ```
  TYPES.md §11.5, DECISIONS 408: a map reading an earlier field reads as its static view; a value of a kept map reads a free binder: literals and union stay E3804.
  -- a/a.canon --
  package a

  local record Node {
    max: Int = 5
  }

  local let nodes: table Node = { a {}, n {} }

  local record Sub(e: Node, f: Node) {
    lim: Int = e.max + f.max
  }

  local record H {
    like: ref nodes
    bySub: {x in nodes: Sub(x, like)} = {}
    named: {x in nodes: {String: Sub(x, x)}} = {}
  }

  local let hs: [H] = [{ like: a }]
  local let eqOpen: Bool = hs[0].bySub == { a: {} }
  local let unionOpen = hs[0].bySub.union({ a: {} })
  local let unionValue = hs[0].named[a].union({ "k": {} })
  ```
- **New `eval/testdata/dependent/union_kept.txtar`:** the repro above, with a `dump.txt`.

**Acceptance.** `depmap_kept`, `depmap_union` and `union_kept` fail at `4dfd414e` and pass after the
change. `E3804_13` passes both before and after.

### 8.2 An amend path enters a dependent map (DECISIONS 409)

**Facts.**
- EVALUATION §9.2 allows `[k]` and `[#n]` on a map (`spec/EVALUATION.md:476-478`), and TYPES §11.5
  makes `{x in c: T(x)}` a map keyed by `ref c`.
- **Check:** `amendKey` and `elemAtPosition` (`check/layers.go:200-226`) handle only lists, tables and
  maps.
  - A `DepMapType` key is `synth`'d, which gives `E2102` for a bare name.
  - The segment is then `E1905` (`layers.go:139`).
  - `bindCtx.step` (`bind.go:43-51`) binds nothing past a key.
- **Eval:**
  - `locMapKey` binds no binder, and its comment says "an amend path never enters a dependent map"
    (`amendctx.go:68-80`).
  - `setMapKey` stores against `AnyType` unless the type is a `MapType` (`amend.go:122-125`).

**Repro:** F plus the lines below.
```
local record Cfg {
  by: {x in nodes: P(x)} = {}
}

local let cfg: Cfg = { by: { a: {}, n: 1 } }
-- a/dev.layer.canon --
package a
layer dev

amend cfg {
  by[a]: { lim: 3 }
  by["n"]: 2
}
```
- **Today `[unverified]`:** `E1905 amend path by[a]: no such field`, `E2102 unknown name a` and
  `E1905 amend path by["n"]: no such field`.
- **Correct:** no finding. With `dev` active, `cfg = Cfg{by: {a: Sub{lim: 3}, n: 2}}`.

**Decision.**
1. `[k]` and `[#n]` on a dependent map name its entries.
   - `[#n]` warns `W1912`, as it does on any map.
   - `k` is typed against `ref c`, and a bare name resolves contextually.
2. The value at the path is the map's value at `k`. Its literal binds `x` to `k`; it is checked
   against `T`'s branches, and at evaluation against `T(k)` (`E3802`).
3. Later segments follow `T`'s static view.
4. `.k` on a dependent map stays `E1905`: §9.2 gives `.k` only to tables and keyed lists.

**Code.**
- **check:**
  - `amendKey` and `elemAtPosition` gain a `*types.DepMapType` case: the key is
    `&types.RefType{Target: x.Coll}`, and the next type is `x.Value`.
  - For a key or position segment on one, `step` returns `b.forField(t).withBinder(x.Binder)`.
  - `amendpath.go:88`: `keyType` returns `ref Coll`.
- **eval:**
  - For a `DepMapType`, `locMapKey` sets
    `m.dep = m.dep.below().inDepMap(t, d.Binder, key, m.a.Value)`.
  - `setMapKey` takes `vt` from `mapValueType(t)` (`amendctx.go:82`).

**Spec text.**
- **EVALUATION §9.2** (`spec/EVALUATION.md:476`): "| `[k]` | map | entry with key `k` (identifiers
  resolved against the key type) |" becomes "| `[k]` | map, dependent map | entry with key `k`
  (identifiers resolved against the key type: `ref c` for `{x in c: T(x)}`, whose value there is that
  map's value at `k`, TYPES.md §11.5) |".
- **At `:478`:** "list, keyed list, table, map" becomes "list, keyed list, table, map, dependent map".

**DECISIONS text.**
> 409. **An amend path enters a dependent map (EVALUATION.md §9.2; TYPES.md §11.5).** `[k]` and `[#n]` name the entries of `{x in c: T(x)}`, `k` typed against `ref c`; the value at the path is that map's value at `k`: its literal binds `x` to `k` and is judged against `T(k)` at evaluation (`E3802`). Reason: §9.2 allows `[k]` on a map and §11.5 makes a dependent map one; the checker gave `E1905`, and `E2102` for a bare key (log-2026-09-29 "PS2 round 2").

**Tests.**
- `accept/layer_depmap.txtar`: the repro, plus a field `named: {x in nodes: {String: Sub(x)}} = {}`
  amended by `named[a]["k"]: {}`.
- `findings/E1905_4.txtar`: `by[a].nope: 1` gives one `E1905`, at `nope`.
- `findings/W1912_4.txtar`: `by[#0]: {}` gives `W1912` only.
- `eval/testdata/layers/depmap.txtar`, with `-- layers --` set to `dev`: the repro plus `by[d]: 4`.
  Expected: `cfg.by[a]` is `Sub{lim: 3}`, and `E3802` at `4`.

All four tests fail today and pass after the change.

### 8.3 A level-1 ref in a spread-built instance (DECISIONS 410)

**Facts.**
- A spread copies field values as they are (`eval/records.go:96-103`, `165-171`). So `other.mobs`
  holds `arena`'s entries, with `arena`'s identity (TYPES §6.3).
- `boss: 1` is bound to the new instance (`eval/refs.go:160-164`; EVALUATION §3.4).
- Refs matches a level-1 ref only by `(Coll, Owner, Key)` (`edit/refs.go:126-138`), and the graph does
  the same (`graphscan.go:82-96`).
- So `other.boss` matches no target and no node, and the name scan lists its `1` as `code`
  (`refnames.go:73-75`).

**Repro.**
```
package a

local record Mob {
  id: Int
}

local record Arena {
  mobs: [Mob] keyed by id
  boss: ref Mob
}

local let arena: Arena = { mobs: [{ id: 1 }, { id: 2 }], boss: 2 }
local let other: Arena = { ...arena, boss: 1 }
```
- **Today `[unverified]`:** `canon refs a:arena.mobs[1]` lists only a `code` ref at `other`'s `1`, and
  the graph has no edge at `a:other.boss`.
- **Correct:** a `value` ref `other.boss`, and an edge `a:other.boss` → `a:arena.mobs[1]` of kind
  `value`, not computed.

**Decision.** A bound level-1 ref is a ref of the entry its key names, in the collection its
instance's field holds, whatever identity that entry carries. That is the entry the evaluator
dereferences. Evaluation and identity do not change.

**Code** (`internal/edit`).
- A new `heldIdent(x *value.Ref) *value.Identity` in `refs.go`:
  - when `rt.Target.Kind == types.CollField` and `x.Owner != nil`, it returns the `Ident` of the
    element keyed `x.Key` in `x.Owner`'s field `rt.Target.FieldPath[0]`, found with
    `fieldIndex(fieldsOf(…))` (`resolve.go:131-150`);
  - otherwise it returns nil.
- `isValue` also matches when `heldIdent(x)` has the target's `Coll` and `Owner`.
- `refTargets` also adds the node of `heldIdent(x)`.

**Spec text.** API.md R7: a new bullet after "Names in defaults, constants and spreads are `code`
only…" (`spec/API.md:612-615`):
> - A bound level-1 ref (TYPES.md §10.2) is a ref of the entry its key names in the collection its instance's field holds, whatever identity that entry carries: in `{ ...arena, boss: 1 }` the copied `mobs` keep `arena`'s identity (TYPES.md §6.3), so `boss` is a `value` ref of `arena.mobs[1]`, and R11's edge goes there (DECISIONS 410).

**DECISIONS text.**
> 410. **A level-1 ref names the entry its instance's field holds (API.md R7, R11).** `Refs` and `Graph` match a bound level-1 ref against the entry its key names in its instance's field, whatever identity that entry carries; a spread-built instance holds the source's entries, so its refs reach them. Reason: such a ref was listed only as `code` and had no edge (log-2026-10-29 "Owed").

**Tests.**
- `TestRefsSpreadLevelOne` (`edit/refs_field_test.go`):
  - `Refs("a:arena.mobs[1]")` holds the `value` ref `other.boss`, and no `code` ref at that span;
  - `Refs("a:arena.mobs[2]")` holds only `arena.boss`.
- `TestGraphSpreadLevelOne` (`edit/graph_test.go`): has the edge above, and `checkParity` holds.

Both tests fail today.

### 8.4 The rest of "Still open" (DECISIONS 411)

The spec sync strikes every "Still open" checker bullet (`DECISIONS.md:3088-3110`) except J13/J14.

1. **`Target(sub.kind)` with `sub: Sub(goal)`: a bug.**
   - TYPES §11.1 allows a stable path through an earlier field, but `fieldOf`
     (`check/typefunc.go:219-233`) takes only `*types.RecordType`, so this case gives `E3003`. Today
     `[unverified]`: `a.Sub(goal) has no field kind`.
   - Fix: `fieldOf` accepts an `*types.AppliedRecord` and uses its `Rec` (§11.3). `Arg.Path` then
     holds `Sub`'s own fields, so eval, verify, ir and gen read it as they read `ev.kind`.
   - Tests:
     - `accept/typearg_applied.txtar`, all declarations `local`: `enum K { x, y }`,
       `record Goal { k: K }`, `let goals: table Goal`, `record Sub(g: Goal) { kind: K = g.k }`,
       `type Target(k: K) = match k { x => Int  y => String }`, and
       `record Quest { goal: ref goals  sub: Sub(goal)  aim: Target(sub.kind) }`;
     - `eval/testdata/dependent/applied_path.txtar`: the same fixture, with `aim: "s"` for an `x`
       goal, gives `E3802`.
2. **`E3806` on a dependent argument (`aim: Aim(foe)`, `foe: Pick(goal)`): confirmed.**
   - `argFits` (`typefunc.go:281-297`) refuses it, and that is right: no parameter can have a
     dependent type.
   - Pin it with `findings/E3806_12.txtar`.
   - TYPES §11.1, inserted after "…is `E3803` (DECISIONS 307)." (`spec/TYPES.md:1024`):
     > A path may read through a field typed by an applied record (`sub.kind` with `sub: Sub(goal)`, §11.3). An argument whose static type is a dependent value (`F(*)`) fits no parameter: `E3806`; pass the driver it is computed from (`Aim(goal)`).
3. **An optional collection field (TYPES §10.2): no field identity, as built.**
   - §9.4 defines a collection as a table or a keyed list.
   - check (`collectionElem`/`fieldCollection`, `refs.go:293-303` and `411-418`) and eval
     (`collectionOf`, `colls.go:159-166`) both decline `T?`.
   - API R11 (`spec/API.md:708-711`) already makes such entries nodes where they are met, and
     `edit/graph_test.go:284-345` pins `g:slotted.slots[8]`.
   - Giving these entries the field's identity would add level-1 candidates and let paths that no spec
     grants and no user needs (no ref targets Resource's `channelGroups`).
   - `canon graph` and `canon refs` are unchanged. Appended to TYPES §9.4 (`spec/TYPES.md:927`):
     > An optional field or let holding a table or keyed list (`[T] keyed by k?`) holds no collection of `T`: it is no level-1 candidate (§10.2), a let path through it is `E3504`, and its entries take no field identity (§6.3); API.md R11 makes them nodes where they are met.
4. **An op's list or brace literal for a dependent type whose branch cannot be computed.**
   - The edit engine types such a literal only against a computed branch (`edit/littype.go:77-79`,
     `litdependent.go:15-40`).
   - Pin `TestDependentCollectionLiteralUncomputed` in `edit/litdependent_test.go`: a `List` set into
     `slot: Slot(ev)` in the same request that changes `ev` is a `*ValueError`.
   - API.md V1, inserted after "…(TYPES.md §11.4; DECISIONS 267)." (`spec/API.md:1011`):
     > A list, record or map literal given to a dependent type is typed against the branch its record computes in the state the earlier ops left; where that branch cannot be computed, it is a `ValueError` (source text takes its first branch instead, DECISIONS 171).
5. **E15 and a map with dependent values: already as E15 says.**
   - `fieldChecks` judges only optional fields and dependent-keyed maps (`edit/applycascade.go:44-55`,
     `cascadekeys.go:12-15`) `[unverified]`.
   - Pin `TestE15KeepsDependentValuedMap` in `edit/apply_e15_test.go`:
     - a `{String: P(goal)} = {}` field keeps its value and its `E3802`, and is not in `Dropped`;
     - a `{String: P(goal)}?` field becomes `none` and is listed in `Dropped`.
   - If the pin fails, make `fieldChecks` select exactly those two kinds. No spec change.
6. **An edit-layer amendment whose right-hand side is a `load(…)`: a bug.**
   - Under an edit layer, `inAmendment` (`edit/layerpath.go:37-52`) edits the loaded JSON file.
   - In the law fixture, `item.json` is loaded both by the layer and by the base let `holder`
     (`fixture_test.go:140`, `202`, `212`). So a layer edit rewrites a base value, which breaks W11's
     "never to the base". `editable_test.go:290-320` pins that behaviour today.
   - Fix:
     - `FormJSON` there gives `Editability{Reason: ReasonLayer}`;
     - delete `jsonEditability` and `loadDisplay`, which become dead code;
     - the two `a:other.data.n` rows in `editable_test.go` now expect reason `layer`.
   - Appended to API.md W11a (`spec/API.md:935`):
     > An amendment whose right-hand side is a `load(…)` holds no literal: a path below it is `ErrNotEditable` with reason `layer` (its JSON lies outside the layer file, and the base may read it); `Set` of the amended path itself replaces the `load` (W11).
7. **R6, a constant that only a stage-E fold computed: probably closed already.**
   - Stage A now forces every constant (EVALUATION §2.1; DECISIONS 264, 271), and `cause` searches
     every analysis finding inside the declaration (`api/value_cause.go:15-66`) `[unverified]`.
   - Pin `TestValueCauseFoldedConst` in a new `api/value_cause_test.go`:
     - fixture: `local const C: Int = 1 / 0`, read only by `export fn f() -> Int { return C }`;
     - expected: `Value("a:C")` is `ErrNoValue` wrapping that `E4102`.
   - If the pin fails, `cause` adds the fold's error findings for a constant root.
   - API.md R6 (`spec/API.md:544`): "the checker's findings in the root's declaration" becomes "the
     findings in the root's declaration (the checker's, and for a constant its fold's)".
8. **VIEWMODEL J13/J14 through a ref: stays open.**
   - Following `Target(g.goal)` or `Member(column.values)` needs a format choice. Either `drivers` is
     keyed by the root collection and the path is followed at build time, or the studio dereferences
     through `values`.
   - Either way, `viewmodel.schema.json` changes for a shape no project uses. It belongs with the next
     studio work.

> 411. **The checker items of "Still open" ruled (TYPES.md §9.4, §11.1; API.md R6, V1, W11a).** A type argument reads through a field typed by an applied record. A dependent argument is `E3806`: pass its driver. An optional field holding a collection holds none: no level-1 candidate, `E3504` through a let path, no field identity. An op's list or brace literal for a dependent type whose branch cannot be computed is a `ValueError`. E15 drops optional fields and dependent-keyed maps only. Under an `EditLayer`, a path below an amendment whose right-hand side is a `load(…)` is `ErrNotEditable` reason `layer`. R6's cause for a constant includes its fold's findings. J13/J14 through a ref stay open, a view-model format choice. Reason: DECISIONS "Still open" (log-2026-09-29, -10-05).

### 8.5 A default that folds to a constant is checked as its literal (DECISIONS 412)

**Facts.**
- A refined type's literal is checked statically: "a literal checked against a refined type that it
  statically violates is reported at once with the refinement's code" (TYPES §6, around
  `spec/TYPES.md:446`; `check/litcheck.go:157-163`).
- The research probe found that a default naming a constant, `a: Int(1..) = ZERO` with
  `const ZERO: Int = 0`, is not reported when no value stores it. A literal default `= 0` is reported.
- The same probe found that the static literal check compares a `Float32` value unrounded:
  `x: Float32(..=0.1) = 0.1` passes it, and the evaluator then refuses the rounded value at storage
  (`eval/store.go:57-66`, `84-107`). So a `Float32` default `= 0.1` against `..=0.1` that no value
  stores is never reported.
- Bounds are already folded in phase 2, including another package's constants
  (`check/refine.go:55-75`). Constant expressions are TYPES §15's.

**Problem.** A default that violates its own field's refinement passes `canon check` when no value
happens to take it. A later value that takes it fails far from the cause.

**Decision.**
1. **Folded defaults.** A record or case field's default whose expression is a constant expression
   (TYPES §15) is folded in phase 2 and checked against the field's type and refinements exactly as
   the literal it folds to: reported at once, at the default, with the refinement's code, whether or
   not a value takes it.
   - The findings of a folded default equal those of its literal in every case: when no value takes
     it, when one does, and when several do. The default's span is that of its expression.
   - A default that is not a constant expression (it reads an earlier field or a parameter) keeps
     today's check at each storage point.
2. **Float32 rounding.** The static check of a literal or folded constant against a `Float32` type
   compares the value after rounding it to binary32, as storage does. Static and dynamic verdicts then
   agree (TYPES §7.3).
3. No new code: `E3204`, `E3201` and `E3202` as for literals.

**Spec text.**
- **TYPES §7.4**, a new rule after "Bounds are constant expressions…":
  > - A field's default that is a constant expression (§15) is checked against the field's refinements as the literal it folds to: at the default, whether or not a value takes it. A `Float32` literal or constant is compared after rounding to binary32, as stored (§7.3) (DECISIONS 412).

**DECISIONS text.**
> 412. **A default that folds to a constant is checked as its literal (TYPES.md §7.3, §7.4).** A record or case field's default that is a constant expression is folded in phase 2 and judged against the field's refinements exactly as the literal it folds to, at the default, whether or not a value takes it; its findings equal the literal's in every case. The static check of a `Float32` literal or constant compares the binary32 value storage keeps. Reason: `a: Int(1..) = ZERO` passed `canon check` until some value took the default, and `Float32(..=0.1) = 0.1` passed statically and failed only at storage (found while planning v0.2, ADR-0023 §3).

**Tests.**
- `internal/check/testdata/findings/E3204_<next>.txtar`: `const ZERO: Int = 0` (and one from
  another package), `record R { a: Int(1..) = ZERO }`, with no value of `R`: one `E3204` at `ZERO`.
  Today: none.
- `TestFoldedDefaultIsItsLiteral` (`internal/check`, table-driven). For each row, the findings of
  `= ZERO` equal those of `= 0`, modulo the default's text:
  - no value takes the default;
  - one value takes it;
  - two values take it;
  - a value sets the field itself.
- `findings/E3204_<next+1>.txtar`: `x: Float32(..=0.1) = 0.1` in a default and in a `let`: one finding
  each, statically. The eval golden for the `let` is unchanged in text; read the diff.

**Acceptance.** The new tests fail on the unchanged files and pass after the change. `canon check` of
the real copy gives no new finding from this section. If it does, those are real Resource defaults
outside their refinements: an upgrade note, not a bug.

**Unit.** In CHECK-OWED (`internal/check`; sonnet, opus review; S).

**Forecast.**
1. Find how the static literal check keeps a literal default from reporting twice, and route folded
   defaults through the same path. Never add a second reporter.
2. Fold with the phase-2 folder bounds use, never with the evaluator.
3. A default that fails to fold is left to today's check: a fold error is not this check's finding.

### 8.6 Units of §8

| Unit | Exclusive packages | Tier | Size | After |
|---|---|---|---|---|
| DEP: §8.1, §8.2, §8.4 items 1-2 | `internal/types`, `internal/check`, `internal/eval`, `internal/eval/std` | opus | M (~200 lines code, ~450 tests) | S2 |
| EDIT: §8.3, §8.4 items 4-7, §10.1 | `internal/edit` (+ granted: `internal/build/write.go` and its callers, for §10.1; the test files `api/edit_rollback_io_test.go` and `api/value_cause_test.go`) | opus | M | S2 |
| (in CHECK-OWED): §8.5 | `internal/check` | sonnet, opus review | S | N5-CHECK |

**DEP lands as one unit.** A check-only change would accept literals that eval then builds without
their arguments.

### 8.7 Forecast for DEP

1. **The `staticView` split.** Changing `staticView` in place makes `convert.go:205` recurse forever
   and changes joins (`lists.go:88-100`). Split it exactly as §8.1 says.
2. **The binding's `field`, set in check and eval together:** at check `maps.go:14`, eval
   `lists.go:137` and eval `amendctx.go:73`. If only the check half lands, `{ d: {} }` under `P(x)`
   builds `Sub` without its argument, which gives `E3505` or an internal error. The `d: {}` row of
   `union_kept` catches it.
3. **Binders compare by name** (`bind.go:82-83`; eval `params.go:26-34`). A kept map's literal shadows
   an outer binder of the same name, which is the right scope. Leave it.
4. **Frozen contracts keep their shapes:** `types.go`, `check/info.go` and `eval/host.go`.
   `KeptDepMap` is new and additive.
   - The one behaviour change: `Info.Types` for a read of a kept map now holds the `DepMapType`.
   - Consumers:
     - in eval/std, only the map helpers switch on `MapType` for a call result, and §8.1 fixes them;
     - views read declared field types;
     - ir and gen treat `Map` and `DepMap` alike (`gen/go/gotype.go:34`, `gen/ts/fns.go:91`)
       `[unverified for C++]`;
     - LSP hover now prints the kept type, so its goldens are regenerated and reviewed.
5. **Goldens.** E3804_9 and E3804_10 move to accept, and E3804_11 shrinks. No example reads a
   dependent map in an expression, so `examples/` must not change. Any other golden diff is a bug.
6. **Progen** is unaffected: no new code, and the E3804 and E1905 operators still produce their codes.
7. **Real data.** Resource has no dependent map and no layer file.
8. **Performance.** Memoize `KeptDepMap` per `*types.DepMapType` in the checker: it runs at every
   read.

## 9. Owed generator bugs, spec and catalogue hygiene (DECISIONS 413, 414, 415)

The facts are at `4dfd414e`. Probes ran that tree's `canon` (go 1.25, g++ 15, clang 21, tsc 5) in a
scratch directory.

### 9.1 gen/go keys pure refs off the importer's mode (no DECISIONS item)

**Facts.**
- `isData()` (`holders.go:9`) is the mode of the emit being written. `pureKey` (`pure_expr.go:76`;
  used by body literals at `:68` and by vectors at `conformance.go:184`), `verb`
  (`conformance.go:134`) and `zeroOf` (`conformance.go:237`) all test it.
- The ref's own package is `enumIDs(t.Ref.Pkg)` (`convert.go:216`), which `keyLit` (`literal.go:101`)
  already uses.

**Problem**, probed with `go vet`:
- `core` is go baked with `table sizes`; `ui` is go `data` with `export fn sizeFor(n: Int) -> ref sizes`.
  The result is `ui.gen.go:13: cannot convert "large" (untyped string constant) to type core.SizeID`
  (a baked id is an opaque struct).
- Reversed (`ui` baked, `core` data), with a failing vector:
  `cannot use 0 (untyped int constant) as core.SizeID value`.
- `check` and `build` are silent, and the C++ and TS of the same project compile.

**Decision.** Every table-ref choice follows the ref's package.
- Add `func (g *gen) stringID(r *ir.RefTarget) bool { return isTableRef(r) && !g.enumIDs(r.Pkg) }`
  beside `isData`.
- `pureKey`: `stringID` gives the typed conversion, else `keyLit`.
- `zeroOf`: the `enumIDs` case stays first, then `stringID` gives `emptyString`.
- `verb`: `stringID` gives `%q`.
- Own refs are unchanged.

CODEGEN §5.3 and §5.8 already say so: no spec text, no DECISIONS item.

**Tests.** `TestPureRefsFollowTheOwnersMode` (new `internal/gen/go/foreign_pure_test.go`; CODEGEN
§5.3, §5.8), with hand-built IR as in `translated_fixture_test.go`:
- `ui` has the translated `sizeFor(n Int) -> ref core.sizes` (`if n + 1 > 5 { large } else { small }`;
  vectors `7 -> large`, `MaxInt64 -> E4101`). `core` is an `ir.PackageRef` with a go emit, baked or
  data.
- **Owner baked, importer data:** the output holds `core.SizeIDLarge`, never `core.SizeID("`.
- **Owner data, importer baked:** the failing row's want is `""`, and the literal is
  `core.SizeID("large")`.

Both rows fail today.

**Unit:** GEN-GO.

**Forecast.**
- `verb` is cosmetic: claim no compile fix there.
- The second row retypes a baked importer's literal. Read the golden diff; expect none.
- Leave the other `isData()` sites alone: they concern the package's own tables.

### 9.2 `values` never selects table ids (DECISIONS 413)

**Facts.**
- `idTypes` (`gen/ts/enums.go:193`) ranges over `g.emitted`.
- `tableID` (`types.go:116-123`) types a ref as `string` when `tableOf` (`gen.go:204`) finds no
  emitted table.
- `ownIDIndex` (`enums.go:239`) declares `<Id>Index` only for an id it wrote, yet returns the name.
- `foreignHasIDs` (`imports.go:76`) assumes that every non-types emit declares ids.
- The name plan reserves ids for the selected values only (`ir/tsplan.go:292`).
- Go and C++ write every public table's id, whatever `values:` says (probed).

**Problem**, probed with `tsc --strict`:
- Package `a` declares `emit ts { values: [others] }` plus `export fn isTop(t: ref tiers)`. The result
  is `a.ts(97): TS2304 Cannot find name 'TierIdIndex'`, and `t: string`.
- Package `b` imports `tiers`: `TS2551 TierIdMembers`, and `TS2724 no exported member TierId` twice.
- No IR rule refuses this. The logged bug is one case of a wider one.

**Decision.**
- `values:` never selects ids, as in Go and C++, and as `foreignHasIDs` already assumes.
- Every emit but `types` declares, for every public table, `<Id>` plus `<Id>Members` and
  `<Id>LiveMembers` (baked, embedded), or the string `<Id>` (data).
- **gen/ts:** `idTypes` and `tableID` range over the public tables (`g.p.Values` with
  `tableElem != nil`); `tableOf`, `rowSites` and `idTypeOf` keep reading the emitted tables.
- **`ir/tsplan.go`:** move `declareValue`'s id block into `declareTableID`, called for every public
  table outside types mode, so that `E8005` `idType` sees them. A TS-only package with two unselected
  same-record tables is newly `E8005`, as it already is in Go and C++.
- **Order.** The plan part lands first, in IR-PLANS: reserving more names never breaks the generator.
  The generator part follows in GEN-TS. GEN-TS lands before EN-TS, and EN-TS's private index over
  `Members` then relies on every indexed `Members` existing.

**Spec text.**
- **CODEGEN.md**, after line 130 ("…never re-emitted."):
  > `values` never selects table ids: every emit but `types` also declares the id type of every public table of the package, selected or not (§5.3), because an export fn, a field or another package's emit may name it (DECISIONS 413).
- **After the §5.3 table** (line 1165):
  > A table that `values` leaves out still has its id type, and in `baked` and `embedded` mode its members, in every target.

**DECISIONS text.**
> 413. **`values` never selects table ids (CODEGEN.md §2.1, §5.3).** Every emit but `types` declares the id type of every public table of its package, selected or not: in TypeScript `<Id>`, and in `baked` and `embedded` mode `<Id>Members` and `<Id>LiveMembers`. Reason: gen/ts declared ids for emitted tables only, so a lookup over a table `values:` omitted, or another package's TypeScript naming its id, failed `tsc` with no finding; Go and C++ always declared them.

**Tests.**
- New `testdata/ts/ids_unselected.txtar` (`a` and `b` as probed; golden by `-update`).
  `TestCasesTypecheck` fails on it today with the four tsc errors.
- `TestUnselectedTableKeepsItsIds` (no toolchain): `export type TierId`, `TierIdMembers` and
  `const TierIdIndex` come before `isTop`. Plus a data-mode twin.
- `internal/ir/testdata/findings/E8005_76.txtar` (in IR-PLANS): two unselected same-record tables in
  a TS-only package.

**Units:** IR-PLANS (the plan), then GEN-TS (the generator; sonnet, opus review; M).

**Forecast.**
- Types mode declares no ids.
- `ownIDItems` indexes `g.items`, and unselected tables follow `g.p.Values` order. Read every golden
  diff.

### 9.3 Two C++ packages in one namespace (a pin, no DECISIONS item)

**Facts.**
- E8005 across packages is pinned by `E8005_13`, `22`, `25`, `28-31`, `35` and `75`. No archive
  compiles two packages of one namespace.
- The only legal meetings are `to_json`/`from_json` (`cppplan_scopes.go:60`) and `Decode`
  (`cppplan_inner.go:38`), through `meetsOverload`.
- Probed: two baked and two data packages in `sov::gen` build, link and run under g++ and clang++ at
  `-Wall -Wextra -Wpedantic`, and both id enums stay non-constructible into `nlohmann::json`.

There is no bug: the test pins the behaviour.

**Test.** `internal/ir/testdata/e2e/cpprun/shared_namespace.txtar` and
`internal/ir/cpprun_namespace_test.go`, `TestSharedNamespaceCompilesAndRuns` (CODEGEN §3.5, §7.1;
DECISIONS 379):
- **Packages:** `a` and `b` are baked, each with an enum, a record, a table, a lookup over the enum,
  and a translated fn with a `test`. `c` and `d` are data, with a table each and `emit json`. All use
  `emit cpp { namespace: "sov::gen" }`, with disjoint names.
- **Build:** `cppRunBuild` must report no finding.
- **Probe:** `probe.cpp` includes the four headers in one TU. It prints `ToWire` of a's and b's ids,
  both lookups, both translated fns, and c's and d's loaded rows. It also asserts
  `static_assert(!std::is_constructible_v<nlohmann::json, sov::gen::ItemId>)` for both ids.
- **Run:** `cppRunOutputs` runs it under every compiler, with exceptions on and off.

The test passes today. To prove it can fail: copy `cppplan_scopes.go` aside, make the `to_json`
sharing `meetsNever`, expect E8005 at b's emit, then restore the file.

**Unit:** IR-PLANS.

**Forecast.** Keep the four TUs sequential (nlohmann), and keep the names disjoint: a record `Item`
declared twice is the pinned E8005.

### 9.4 The macro lists become one data file (no DECISIONS item)

**Facts.**
- `cppStdMacroNames` (used for E8011: `rules_names.go:178`, `goname.go:96`, `rules_cppdefines.go:65`)
  is a head (`constants.go:474-497`) plus a tail (`constants_macros.go:18-28`), sorted.
- `cppMacroNames` (W8006, `constants_macros.go:9-16`) is unsorted.
- `constants.go` has 497 of its 500 lines.
- CODEGEN §3.4 pastes the std list, and no test compares the two.

**Decision.**
- One file, `internal/ir/text/cpp_macros.txt`: `#` comment lines, headers `[std]` then `[warn]`, one
  name per line, bytewise sorted per section.
- A new `internal/ir/cppmacros.go` holds:
  - `//go:embed text/cpp_macros.txt`;
  - `parseMacroLists(text) (std, warn []string)`: a header switches section, blank and `#` lines are
    skipped, and there is no error path;
  - the two sets.
- Delete `constants_macros.go` and `constants.go:474-497`.
- Generate the file from the current slices with a throwaway test, deleted before the commit. The std
  lines equal the old `cppStdMacroNames`; the warn lines are the old `cppMacroNames`, sorted.

**Tests.**
- `TestCppMacroFile` (replaces `TestCppStdMacroTable`): the lists are sorted and duplicate-free,
  contain the existing examples, and are disjoint.
- `TestCppStdMacrosAreTheSpecs`: the names after "in byte order:" in CODEGEN §3.4 (read as
  `internal/gen/ts/spec_test.go` reads the spec) equal `[std]`.

Both fail first: they do not compile.

**Unit:** IR-PLANS.

**Forecast.** Parse inside a function, so that the sets never read an unparsed string. The size cap
binds `.go` files only. No W8006 or E8011 golden may change.

### 9.5 The function-size items (v0.3) and one correctness bug

Four of the five items belong to v0.3's function-size budget (ADR-0024 §1.6), because they are
correct today:
- `monitoring`'s nested foreign-record block (about 1,700 lines);
- Go `buildX`;
- `codesIndex` (`fns.go:271`);
- `readBits` (`convert.go:345`).

The fifth, **`inputEnumBody` (`inputs.go:300`), is a correctness bug**, fixed here:
- **Problem.** An enum with two members on one wire, used as an `input` field type: the generated
  `switch raw` holds `case "x"` twice. `check` and `build` report 0 findings, and `go vet` says
  `duplicate case "x" (constant of type string) in expression switch`.
- **Decision.** First declared wins (DECISIONS 383, 393), as `Parse<E>` does. Walk the members in
  declaration order with a `seen` set of wires.
  - Each member marks its wire, retired members included.
  - Only a live first holder writes a `case`.
  - A retired first holder leaves its wire refused, as `FromWire` plus `Retired()` already say.

  §6 makes equal wires an error in `check`, but IR-built fixtures bypass `check`, so the fix stays.
- **Test.** `TestInputEnumEqualWires` (new `internal/gen/go/inputs_equalwire_test.go`; DECISIONS 383,
  393; EVALUATION §11.3), over `first="x"`, `second="x"`, `third="y"`, a retired `old="z"`, then
  `new="z"`: each wire appears once, `x` assigns `first`, and `z` is refused. It fails today.
- **Unit:** GEN-GO.

### 9.6 One `must` helper (no DECISIONS item)

**Facts.** `must` (`internal/lsp/latency_test.go:90`, about 80 uses in 8 files) and `stampMust`
(`internal/workspace/stamp_internal_test.go:30`, 15 uses) are the same five lines. The standard
library has none.

**Decision.**
- A new leaf package, `internal/testkit/must`, imports `testing` only (testkit is outside the
  dependency rule).
- It exports `func NoError(tb testing.TB, err error)`, which calls `tb.Helper()` then `tb.Fatal(err)`,
  plus `doc.go` and an `Example` (go.md §2).
- `fixture` is refused as a home: it imports `ir` and `project`.
- Delete both copies in the same commit, and rewrite their call sites.

**Test.** `TestNoError`: a recording `testing.TB`, run in a goroutine. A nil error passes; an error
calls `Fatal`. It is red first: the package does not exist.

**Unit:** MUST.

**Forecast.** Log `replacePlatforms` (`api/refresh_test.go:24`), a third copy of `stampPlatforms`, as
owed. Do not fix it.

### 9.7 MSVC and the array forms: a documented limit

- The only MSVC check is `TestGoldensCompileMSVC` (`internal/testkit/golden/msvc_test.go`). It runs on
  Windows with `cl.exe`, on goldens only, so on small arrays.
- `clang-cl` exists here, but without the MSVC STL or SDK it falls back to libstdc++ and stops.
- Nothing in this repository can verify the bulk array forms at scale.

**Spec text** (S1), CODEGEN.md §9, after the Float-exactness paragraph:
> The MSVC row is verified only by the golden compile on a Windows runner with `cl.exe` (IMPLEMENTATION-PLAN.md §7.8), at the goldens' size. The bulk array forms of §7.3 (DECISIONS 392) are not compiled under MSVC or clang-cl at the scale that motivated them (tens of thousands of elements); their limits there are unverified, and nothing in this repository runs `cl.exe` at that size.

### 9.8 E9001, E9006 and E8104 name their way out (DECISIONS 414; applies 305)

**Facts.**
- DECISIONS 305 asks every ban to name its way out.
- Producers:
  - E9001: `translate.go:225`, one site, any construct;
  - E9006: `translate_sig.go:25` (a declared parameter) and `translate_read.go:234` (`badRead`: a
    `@ts(bigint)` read at `:115`, a ref read at `:216`);
  - E8104: `rules_emit.go:218`.
- E9006's list omits `Float32`, although CONFORMANCE §2.1 admits it.
- SPEC §19.2 gives E8104's reason; CODEGEN §5.12 gives neither a reason nor a way out.

**Decision.**
- **E9001** keeps one message and adds the way out: compute it at build time (an `export fn` over
  finite parameters is a lookup the body may call), or keep the logic out of translated functions.
- **E9006** splits into `param` and `read` (constructors `AtParam` and `AtRead`, as E9007 has them),
  since their ways out differ. Both list `Float32`.
- **E8104** gives its reason and its way out.

**ERRORS.md** (applied by S2; replaces these codes' message rows):
> | E9001 | - | construct:Expr, fn:Name | `{construct} is not allowed in translated function {fn}: it is outside the portable subset; compute it at build time, in an export fn over finite parameters that this function calls, or keep that logic out of translated functions` |
> | E9006 | param | param:Name, fn:Name, typ:Type | `parameter {param} of {fn} has type {typ}: translated functions take Bool, integers, Float, Float32, String, Duration and enums; pass the values the body reads as parameters of those types, or make {fn} a method of the record that holds them` |
> | E9006 | read | param:Name, fn:Name, typ:Type | `{fn} reads {param} of type {typ} through self: translated functions read Bool, integers, Float, Float32, String, Duration and enums from self, never a ref or a @ts(bigint) field; pass the value the body needs as a parameter` |
> | E8104 | - | field:Name | `input field {field} cannot be emitted to TypeScript: TypeScript has no environment to read it from; keep the input fields in a package of their own that has no TypeScript emit` |

**Spec text** (S1):
- **CONFORMANCE.md**, after line 89 (the end of §2.2's E9001 paragraph):
  > The way out is the build: an `export fn` whose parameters are all finite (`Bool`, enums, refs into a table) is computed at build time with the whole language and ships as a lookup table, which a translated body may call (DECISIONS 305, 414). Logic that needs a construct above over runtime values stays outside translated functions.
- **CONFORMANCE.md**, after line 54 (§2.1):
  > `E9006` names its way out: pass the values the body reads as parameters of an allowed type, or make the function a method of the record that holds them, which reads them from `self`; a ref or `@ts(bigint)` field read through `self` is passed as a parameter instead.
- **CODEGEN.md:1717**, replace with:
  > - A TS emit whose package declares an input field is `E8104`: TypeScript has no environment to read an input from (SPEC §19.2). The way out is to keep the input fields in a package of their own that has no TS emit (DECISIONS 305, 414).
- **SPEC.md:1183:** add `Float32` after `Float`.
- **SPEC.md:1199:** after "naming the construct", add "and the way out: compute it at build time in an
  `export fn` over finite parameters, or keep it out of translated functions".

**DECISIONS text.**
> 414. **`E9001`, `E9006` and `E8104` name their way out (CONFORMANCE.md §2.1, §2.2; CODEGEN.md §5.12; ERRORS.md; applies 305).** `E9001` names the build (a lookup over finite parameters) or leaving the logic out of translated functions; `E9006` has two messages, a declared parameter (pass the values, or make the function a method) and a read through `self`, and lists `Float32`; `E8104` gives its reason and the way out, a package with no TypeScript emit. Reason: three bans named none (log-2026-10-31).

**Tests (in S2).**
- New `internal/ir/testdata/findings/E9006_5.txtar`: a record parameter, giving the `param` text.
- `E9006_2` and `E9006_3` become `read`.
- These regenerate with `-update`, and the diff is read for message text only: the 19 `E9001_*`,
  `E9006_1..4`, `E8104_1`, `translate/refusals.txtar`, `e2e/cppbaked/local_table_param.txtar` and
  `internal/diag/testdata/messages.txtar`.
- `internal/diag/render_test.go:12` takes diaggen's message count.
- GUIDE adds `Float32` to `internal/cli/guide/emit.md:115`.

**Forecast.** v0.4 (ADR-0025 §2 item 1) will add a `while` variant to E9001. `Float32` is the `Float`
kind, so no producer changes for it.

### 9.9 Float `%` (spec text and a pin, no DECISIONS item)

**Decision.**
- Probed: `Float % Float` is `E3007` in a `let`, in a non-exported fn and in a translated fn, and
  `Duration % Duration` is too. The evaluator has only `modInt` (`eval/std/arith.go:178`), and gen/ts
  only `canonMod`.
- TYPES §7.1 (696, 713) is the strictest reading and stays. CONFORMANCE.md changes at `:158` and
  `:67-68`.
- The dead `ModFloat` helpers and their table entries (`gen/go/constants.go:425`,
  `gen/ts/constants.go:344`) stay: removing them regenerates every runtime golden. They are logged as
  owed for v0.3.

**Spec text** (S1):
- **CONFORMANCE.md:158**, replace the row:
  > | `a + b`, `a - b`, `a * b`, `a / b` (Float) | IEEE 754 double, round to nearest | `E4104` if the result is NaN or infinite (`x / 0.0` included) | `rt.AddFloat` … `DivFloat` | `canon::AddFloat` … | `canonF(a + b)` … |
- **After the table's notes:**
  > - `%` is defined on integers only (TYPES.md §7.1); on `Float` or `Duration` it is `E3007`, so no translation computes a float remainder.
- **CONFORMANCE.md:67-68:** "`+ - * /` and unary `-` on integers, `Float` and `Duration`, and `%` on
  integers, as typed by TYPES.md (…)".

**Test.** New `internal/check/testdata/findings/E3007_6.txtar`: Float and Duration `%` in a `let` and
in a translated fn (findings by `-update`). It fails first: the file is absent.

**Unit:** CHECK-OWED.

### 9.10 The runtime listings (DECISIONS 415)

**Facts.**
- CODEGEN §6.3 lists 555 lines against the embedded `rt.go.txt`'s 786. There are 13 hunks, and 12
  functions are missing (`checkEnvelope`, `checkObjects`, `DupRow`, `EqualFold`, `FirstDup`,
  `Float32Overflows`, `WireToken`…).
- §7.5 lists 213 of 1,466 lines as if they were the whole file.
- §7.4 equals `canon_runtime.h.txt` byte for byte (399 lines).

**Decision.**
- §6.3 and §7.5 become contract text: the embedded files are the text of record.
- §7.4 is pinned to its file by a test.
- Reason: v0.3 and v0.4 rewrite and extend these runtimes, so a byte pin on 786 and 1,466 lines would
  make every ADR carry them verbatim.

**Edit plan** (S1):
1. Delete the `go` block of §6.3 (about lines 1950-2505) and the `cpp` block of §7.5, and put the
   texts below in their place.
2. §7.4 stays unchanged.

**Spec text** (S1):
- **CODEGEN.md §6.3**, after its first paragraph:
  > Its text is the embedded file `internal/gen/go/runtime/rt.go.txt`, written byte for byte; this section fixes its contract, not a listing (DECISIONS 415). Every `rt.<Name>` this specification names is exported by it. The areas: the version `Version`; read-only collections (`List`, `Map`, `KeyedList` and their `Make…`, `Ptr`); evaluation errors (`EvalError`, `Fail`: translated code panics with `*EvalError`, DECISIONS 19); the checked arithmetic, conversions and range and width checks of CONFORMANCE.md §3; data-file parsing (`ReadDataFile`, `ParseDataFile`, WIRE.md §8); runtime inputs (`Env`, the `Parse…Literal` readers, `InputError`, §5.12).
- **CODEGEN.md §7.5**, in place of its listing:
  > Its text is the embedded file `internal/gen/cpp/runtime/canon_runtime_json.h.txt` (guard `CANON_RUNTIME_JSON_RT_V1`, namespace `canon::rt_v1::json`), written byte for byte; this section fixes its contract, not a listing (DECISIONS 415): the byte rules and first-failure messages of WIRE.md §8 for `ParseDataFile` and `ParseTextFile`, and a `Decoder` whose typed reads check the JSON type first and name the failing key path; the generated decoders also record a value outside its range or length refinement (§5.15) and check nothing else `canon build` checked.

**DECISIONS text.**
> 415. **The Go and C++ JSON runtimes are the embedded files, not a spec listing (CODEGEN.md §6.3, §7.5).** The spec fixes their contract (names, rules, failure texts); `canon_runtime.h` (§7.4) and the TypeScript helper block (§8.2) stay verbatim and are pinned to their files by tests. Reason: §6.3 had drifted by 12 functions and §7.5 held 213 of 1,466 lines; the runtimes grow every version.

**Tests.**
- `TestRuntimeContractNames` (GEN-GO, new `internal/gen/go/runtime_spec_test.go`): every `rt.<Name>`
  in `spec/*.md` is an exported name of `rt.go.txt` (read with go/parser).
- `TestHeaderIsTheSpecs` (CPP-OWED, `internal/gen/cpp/runtime/runtime_test.go`, using
  `cppruntime.Header`): the first `cpp` block after "### 7.4" equals the header, as
  `TestHelperBlockIsTheSpecs` does for TS.
- `TestSpecNamesTheRuntime`: every `canon::<Name>` in `spec/*.md` is in `Identifiers()`.

These are red first only if a name is stale; the pin passes today.

**Forecast.** Leave the TS block alone. Find stale names first
(`grep -o 'rt\.[A-Z][A-Za-z0-9]*' spec/*.md`) and fix the spec, never the test.

### 9.11 Progen operators for `tiers` (no DECISIONS item)

**Facts.**
- `gaps.txt` lists `E1017`, `E1018` and `E2007`.
- The harness strips the examples' `tiers` (`fixture.WithoutTiers`, `harness_test.go:179`), so
  `onKey("tiers", …)` finds nothing.

**Decision.** Build the operators instead of keeping the gaps.
- A new `internal/testkit/progen/ops_tiers_test.go` adds `tiersOperators()` to `catalogue()`.
- `fixture` gains `Tiers(project string) string`, which returns the block `WithoutTiers` removes.
- Each operator inserts the examples' own block, mutated, into the stripped project as one edit. The
  focus region is the whole block, so `also` codes count only inside it.
- The operators:
  - E1017 `name`: quote one tier name (`"shared"`); `also: E1018`;
  - E1017 `duplicate`: repeat the first path of `data` in its own list;
  - E1018 `unplaced`: drop `pipeline` from `data`; `many`;
  - E1018 `unused`: misspell `teamboard` as `teamboards`; `many`;
  - E2007: swap the `ids` and `data` entries; `loose`, `many`. `Site.Packages` is the first clean
    unit importing `features.tiers.ids` (`examples/features/tiers`).
- Delete the three `gaps.txt` lines, and rewrite `coverage.txt` with
  `go test ./internal/testkit/progen -run TestCoverage -update`.

**Tests.** `TestCoverage` fails once the operators exist, because it demands that the gaps lines go.
`TestMutations/E1017_*`, `E1018_*` and `E2007_*` run the operators.

**Unit:** PROGEN-TIERS.

**Forecast.** The first runs decide each `also` and `many`: an unexpected code is reported, never
silenced. If the swap leaves no upward import, pick another pair.

## 10. Owed robustness bugs in edit, workspace and load; two items decided out (DECISIONS 416-421)

The research ran no probe; every "Today" is a reading of the code, and readings that depend on OS
behaviour are marked `[unverified]`. A unit whose first, failing test does not fail on today's code
stops and reports.

### 10.1 A failed rollback is a kept journal, and edit errors name display paths (DECISIONS 416)

**Facts.**
- `tally.add` (`internal/edit/recover.go:58-66`, used by `Recover` and `Settle`) and
  `commit.rollback` (`commit.go:476-477`) put only a `*fault` under `ErrJournal`
  (`journalRefusal`, `journal.go:59-66`). An I/O error stays raw.
- When a restore fails, the OS `WriteFile` error names the absolute temporary file
  `.<base>.<token>.canon-edit.<random>…` (`recover.go:395-406` → `build/osfs.go:127-152`, `180-188`).
- `recover.go:301`, `311`, `480` and `journal.go:306-337` also put absolute names in messages.
- Workspace re-wraps Settle errors under `ErrJournal` (`settler.go:56-60`), but `Open` does not
  (`api/canon.go:95`, `107-112`). So a Recover I/O failure makes `Open` fail with no sentinel. O5
  says a journal that `Open` keeps is `ErrProject`.

**Decision.**
1. **Display paths.** Every error that `Commit`, its rollback, `Recover` and `Settle` return names
   each file or directory they place by `Layout.Display`.
   - An OS error keeps its operation and its cause, and unwraps to the original error.
   - A path recorded in the journal that `resolve` refuses is quoted as recorded: it has no display
     path, and the named journal holds it.
2. **Every rollback failure is a journal error.** A fault or an I/O error is wrapped as
   `fmt.Errorf(fmtBadMember, ErrJournal, err)` in `tally.add` and in `commit.rollback`.
   - `journalRefusal` stays for `vet`: there, an I/O error must not become `ErrChanges`, which the API
     reports as a compiler bug.
   - A failure to list `.canon/journal` stays an I/O error, with its path displayed. Workspace's
     re-wrap stays for that case.
3. **One helper.** build's `displayError` (DECISIONS 201) becomes the exported `build.DisplayError`.

**Code.**
- **`build/write.go`:** `func DisplayError(display string, err error) error`, with every caller in
  `internal/build` renamed. This file is granted to EDIT.
- **`internal/edit`:**
  - `undo` gains `at Layout`, set by `resolve` and by `commit.rollback`.
  - `straight`, `syncDirs`, `journals`, `loadJournal` and `removeJournal` take the Layout.
  - In `commit.go`, `recover.go`, `journal.go` and `settle.go`, every
    `fmt.Errorf(fmtFileErr, <abs | f.Path | t.display>, err)` over a file-system error becomes
    `build.DisplayError(<display>, err)`.
  - The `reasonStray`, `reasonLink` and `reasonChanged` faults name `at.Display(abs)`.

**Spec text.**
- **API.md O5** (`spec/API.md:129-131`), replacing "A journal it must keep (written on another host, a
  file changed since the edit, a journal that fails N11's checks) makes `Open` fail with an error
  wrapping `ErrProject` that names the journal and the files.":
  > A journal it must keep (written on another host, a file changed since the edit, a journal that fails N11's checks, a rollback the file system refuses) makes `Open` fail with an error wrapping `ErrProject` that names the journal and each file by its display path, never by an absolute or temporary name; a file-system cause keeps its operation and stays reachable through `errors.Is` and `errors.As`. A path the journal records that no root places is quoted as recorded (DECISIONS 201, 416).
- **API.md N11** (`:1540-1541`), replacing "If a step after the journal fails, every file already
  changed is restored from the journal and the edit returns the error.":
  > If a step after the journal fails, every file already changed is restored from the journal and the edit returns the error, naming files as O5 does; if that rollback fails too, the error also wraps `ErrProject` and the journal is kept.

**DECISIONS text.**
> 416. **A failed rollback is a kept journal; edit errors name display paths (API.md O5, N11; DECISIONS 201).** Every error an edit's commit, its rollback, `Recover` and `Settle` return names what they place by display path, never an absolute or temporary name; an operating-system error keeps its operation and cause and unwraps to the original. A path a journal records that no root places is quoted as recorded. Any failure to roll back a journal wraps `ErrJournal` (`ErrProject` at the API), so `Open` fails as for any journal it keeps. Reason: such a rollback returned the raw OS error, its text an absolute temporary name with a random part, and `Open` then failed with no sentinel (log-2026-10-14, J-WORKSPACE).

**Tests.**

| Test | Proves | Today |
|---|---|---|
| `TestRollbackIOErrorIsJournal` (edit, new `recover_io_test.go`), with three rows: Recover; Settle (`crashedHolding`); the commit's own rollback (rename 2 fails) | Stage writes during the rollback fail with `&fs.PathError{Op: "open", Path: <stage>+".tmp", Err: fs.ErrPermission}`. The error is `ErrJournal`; `fs.ErrPermission` and `*fs.PathError` stay reachable; the display path is named; no `/p/` appears; the journal is kept | raw error, `/p/` in the text |
| `TestOpenRollbackIOErrorIsProject` (api, new `edit_rollback_io_test.go`, shaped like `TestOpenForeignJournal`) | After a crash, `Open` over an FS that fails stage writes returns `ErrProject`, naming the journal and `a/a.canon`, with no `/law/` | raw error |
| `TestRecoverUserFileInCreatedDir` (`recover_test.go:251`) and the hostile row "created holding others" (`recover_hostile_test.go:85`) | Their expectations become `items/deep/mine.canon` and `pkg/pkg.canon` | absolute names |

**Unit:** EDIT.

**Forecast.**
- Change only those two expectations; any other failing test stops the unit.
- Windows errno texts differ, so assert with `errors.Is`.
- Run `-race` at GOMAXPROCS 2 and 4.

### 10.2 J7 also locks the packages that list a directory an edit changes (DECISIONS 417; amends 375)

**Facts.**
- J7's lock set (`workspace/writers.go:45-54`) adds the readers of written data files through
  `fileReads`, which drops directory listings (`:115-123`).
- The edit's owners (`edit.go:131-152`), computed for its re-check (API.md E17), already include the
  packages whose static read set lists a directory where a name is created or removed (`listings`,
  `:156`), and the packages whose asset root holds such a name (`build/static.go:165-198`).
- N9 re-reads only the files being written (`edit/commit.go:201-221`).

**Problem (a race).**
- Packages `a` and `c` both list `a/items/`, and an edit through each creates `a/items/k.json`.
- Their lock sets, `{a}` and `{c}`, are disjoint. Both N9 checks see the name absent, both renames
  land, the second silently replaces the first, and both edits report applied.
- If the first edit died after its rename, the second meets no journal of its own packages (J6) and
  overwrites the file. The dead journal can then never roll back, and `Open` fails `ErrProject` until
  someone resolves it by hand.

**Decision.**
1. An edit takes exclusive locks on:
   - its owners (E17's packages, before importers);
   - the package of each `canon.lock` it writes;
   - every package whose pre-edit `Analysis.Reads` holds a data file it writes.

   Reason: "every package whose read set the edit's writes change" is the strictest consistent set,
   and E17 already computes it.
2. Unchanged:
   - `Plan.Unowned` stays lockless;
   - locks are still taken all or none, in name order (J4: no deadlock);
   - concurrent edits of different packages still judge E19 each on its own snapshot.

**Code.** At `edit.go:108`, call `out.writers(a, owners)`. `writers` starts from
`slices.Concat(o.Plan.Touched, owners)`.

**Spec text.** CLI.md §3.15 (`CLI.md:645-647`), replacing "An edit takes the writer locks of the
packages of every file it writes, a JSON file's reading packages included (DECISIONS 352, 375), rolls
back under them an unfinished edit of those packages (API.md N11), and runs API.md N9 under them.":
> An edit takes the writer locks of the packages of every file it writes, of each package whose read set holds a file it writes or lists a directory it adds a name to or removes one from, and of each package whose asset root holds such a name (API.md E17; DECISIONS 352, 375, 417), rolls back under them an unfinished edit of those packages (API.md N11), and runs API.md N9 under them.

**DECISIONS text.**
> 417. **An edit locks every package whose read set its writes change (CLI.md §3.15; amends 375).** Beside the packages of the files it writes, each written `canon.lock`'s package and the readers of each data file it writes, an edit takes exclusively every package API.md E17 finds before importers: those whose static read set holds a file it writes or removes or a listing it changes, and those whose asset root holds a name it creates or removes. Reason: two edits through packages listing one directory could create or remove one name under disjoint locks; both passed N9, and the later rename replaced the first or overwrote a dead edit's file, whose journal could then never roll back (log-2026-10-14).

"[amended by 417]" goes on DECISIONS 375.

**Tests.**
- `TestEditLocksListingReaders` (workspace, real disk). Package `a` has an `@files` table in
  `a/items/`, and `c` lists that directory. Subtests "created" and "removed": while `c` is held, an
  `AddEntry` that creates `a/items/pot.json` and a `Remove` that deletes `a/items/x.json` each wait for
  `c`, then apply. Today: no wait.
- `TestEditLocksJSONReaders` and `TestEditsAcrossProcesses` stay unchanged.

**Unit:** WS-LOCKS, which also carries §10.4's `overlay.go` change.

**Forecast.**
- Run `-race` at GOMAXPROCS 2, 4 and the default.
- Only waiting between processes grows, and bench-edit runs in one process.

### 10.3 A text output is never a source (DECISIONS 418)

**Facts.**
- A text output is owned only through its package's `canon.outputs` (or a legacy manifest), never by
  a `.json` file's `$schema` (`build/textemit.go:223-237`). Anything else is `E8001`, and nothing is
  written (`place.go:59-83`). So no build overwrites an unmarked file, `.canon` or `.json`.
- Every `.canon` file under the project root that is not dot-prefixed is a source
  (`project/scan.go:45-51`). A `@text("x.canon")` written into an `out` inside the project therefore
  becomes a source on the next run.
- `E8026` judges loads only, not the scan. A `.json` output that a load of the same build reads is
  already `E8026` (DECISIONS 341).
- `ValidTextName` (`check/text.go:136-148`) has no extension rule, and it also decides which stale
  files a build removes (`build/textstale.go:79`).
- Resource has 243 `@text` names: 229 `.json` and none `.canon`.

**Decision.**
1. A `@text` file name whose extension is exactly `.canon`, compared as the scan compares, is `E8021`
   `source`, found in phase 2, wherever `out` points. Reason: where an output lands depends on the
   machine (`project.local.canon`, `--root`); the name does not. The way out is another extension.
2. `ValidTextName` refuses such a name too. A stale text file with a `.canon` name is never removed;
   it only drops out of the list.
3. Every other name keeps DECISIONS 326 and 341, including `.json`, the designed form of a
   non-`String` result (308).

**Code.**
- In `textFile`, before `ValidTextName`:
  `if path.Ext(file) == project.SourceExt { c.report(s.env, diag.E8021.AtSource(s.env.span(v), writtenText(s.env.file, v))); return "", false }`.
- In `ValidTextName`: `case path.Ext(name) == project.SourceExt: return false`.

**Spec text.**
- CODEGEN.md §2.9 (`spec/CODEGEN.md:582-585`): after "matched ignoring case),", insert " a file name
  whose extension is `.canon` (the scan would read the file as a source, API.md O2; DECISIONS 418),".
- CODEGEN.md §12 (`:3913`): "invalid file name," becomes "invalid file name (a `.canon` one
  included),".

**ERRORS.md** (applied by S2):
- The `E8021` codes row (`spec/ERRORS.md:1249`) becomes: `| E8021 | error | check | CODEGEN.md §2.9 | a misplaced `@text`, an invalid `@text` file name (a `.canon` one included), two `@text` files of one package with one name, or a call to a `@text` fn (DECISIONS 294, 300, 336, 418) |`
- A new message row after `:1335`: `| E8021 | source | value:Text | `invalid @text file name "{value}": a .canon file is read as a source of the project; give it another extension` |`

**DECISIONS text.**
> 418. **A text output is never a source (CODEGEN.md §2.9; ERRORS.md E8021).** A `@text` file name whose extension is `.canon`, compared exactly as the scan compares it, is `E8021` `source`, found in phase 2 wherever `out` points, and a stale text file of such a name is never removed. Every other name, `.json` included, keeps 326's ownership (`E8001` over a file its package's `canon.outputs` does not name) and 341's `E8026`. Reason: written inside the project, such a file was a source of the next run, so a build's output changed the program it came from; `E8026` judges loads, not the scan.

**Tests.**
- `internal/check/testdata/findings/E8021_19.txtar`, with `@text("gen.canon")`. Today: no finding.
- `TestValidTextNameCanon` (check): `x.canon` is refused; `x.CANON`, `x.json` and `x.canon.txt` are
  accepted. Today: `x.canon` is accepted.
- `TestTextStaleKeepsCanonName` (build, in BUILD-LINKS): a marked `canon.outputs` lists
  `@out/x.canon`, which the build no longer writes, and the file stays. Today: it is removed.

**Units:** CHECK-OWED (the check part); BUILD-LINKS (the stale test).

**Forecast.** The golden churn is the new txtar plus S2's `messages.txtar`. The release reply says
that a `.canon` output 0.1.x left inside a project stays a source until deleted by hand.

### 10.4 A Windows junction is a link (DECISIONS 419; amends 376)

**Facts.**
- `project.readLink` (`project/symlink.go:89-102`) treats an entry as a link only when its mode has
  `ModeSymlink`.
- Go's behaviour, read from Go's source and not run `[unverified]`:
  - under `go 1.25.0` (winsymlink=1), Windows reports a junction or mount point as `ModeIrregular`,
    without `ModeDir`, in both `Lstat` and `ReadDir` (`$GOROOT/src/os/types_windows.go:100-124`,
    `177-229`);
  - `os.Readlink` reads mount points (`file_windows.go:475-499`), and returns a volume mount point as
    `\\?\Volume{…}\` (`:432-434`);
  - only Windows and illumos doors ever get `ModeIrregular`.
- These readers test only for `ModeSymlink`: the `load.dir` walk (`load/symlink.go:182`), assets
  (`build/hosts.go:373`), the build's re-read (`reread.go:157`), real paths (`realpaths.go:98`) and
  overlays (`workspace/overlay.go:164`).
- So a junction is skipped with no `W7115`, and `resolveBase` (`load/symlink.go:101-114`) sees no link
  through one.
- Builds already refuse any irregular path component (`linkmodes_windows.go:8`, DECISIONS 342). Edits
  compare resolved paths (`edit/journal.go:322-342`).

**Decision.**
1. **What a link is.** In `readLink`, a link is an entry whose `Lstat` mode is `ModeSymlink`, or
   `ModeIrregular` with a target that `os.Readlink` can read.
   - Other irregular entries (a cloud placeholder, an app-exec link, a door) are not links.
   - The rule is platform-independent: nothing changes where Go never sets `ModeIrregular`.
2. **Device targets.** A link target that is a device path (`//?/…` or `//./…` after slashing, as a
   volume mount point gives) has no path the bounds can judge. It is an error wrapping
   `fs.ErrNotExist` that names no path: the link counts as dangling.
3. **Listing readers.**
   - A `LinkCandidate` entry (`ModeSymlink`, or `ModeIrregular` where the FS resolves links) takes the
     reader's existing link branch, unless it is `Unlinked`: that is, it resolves to its directory's
     real path joined with its own name.
   - An `Unlinked` entry takes today's non-link branch, unchanged, with no added `Stat`.
   - The resolution goes through the reader's own recorded resolver.
4. **Messages are unchanged** and keep the words "symbolic link"; WIRE §6.5 now defines what that
   covers. Edits need no change: `straightOne` refuses a junction once `readLink` resolves it (N11).

**Code.**
```go
// internal/project
func LinkCandidate(fsys FS, t fs.FileMode) bool
func Unlinked(resolve func(string) (string, error), dir, name string) bool // false when resolving name fails
type linkReader interface { lstat(string) (fs.FileMode, error); readlink(string) (string, error) } // '/' names
func evalSymlinksWith(lr linkReader, name string) (string, error) // today's body; evalSymlinksOS passes osLinks{}
func readLink(lr linkReader, child string) (string, bool, error)  // points 1-2
```

**Spec text.** WIRE.md §6.5 (`spec/WIRE.md:654`), after "Only regular files match (after following
links).":
> A link is an entry the file system reads as one: a symbolic link, or a Windows junction or directory mount point (an entry Go lists as irregular whose target `os.Readlink` reads). Every rule of this section, of §6.1 and of TYPES.md §13.4 holds for both, and findings call both "symbolic link". A mount point whose target is a volume (`\\?\Volume{…}\`) has no path the bounds can judge and counts as dangling. Any other entry is no link and keeps its kind (DECISIONS 419).

**DECISIONS text.**
> 419. **A Windows junction is a link (WIRE.md §6.5; TYPES.md §13.4; amends 376).** Every reader of the project's files counts as a link an entry that is a symbolic link, or that Go lists as irregular and whose target `os.Readlink` reads (a junction, a directory mount point): `load.dir`'s walk, base and literal path, single-file loads, asset lookups and real-path comparisons follow it only within the bounds, edits refuse it (API.md N11), and findings report it as a symbolic link. A volume mount point counts as dangling. Other irregular entries are no links and keep their kind; builds already never write through any (342). Reason: on Windows `load.dir` skipped a junction silently, losing every entry under it, and read a base or single file through one with no bound (ADR-0021 §9); 376's "a junction stays a file name" is reversed.

"[amended by 419]" goes on DECISIONS 376.

**Tests.**

| Test | Proves | Today |
|---|---|---|
| `TestEvalSymlinksJunction` (project, internal; a fake `linkReader` with `C:/…` names) | A junction is followed, chains included; a loop is `ErrSymlinkLoop`; a volume target is `fs.ErrNotExist`; an irregular non-link resolves to itself; symlinks are unchanged | junction not followed |
| `TestLinkCandidate`, `TestUnlinked` (project) | the two predicates | new |
| `TestLoadDirJunction` (load; the test memFS models a junction as a `ModeIrregular` entry that its `EvalSymlinks` resolves, and a placeholder as one resolving to itself) | A junction inside the bounds is followed; outside them it is `W7115 outsideRoots`; dangling, `W7115 dangling`. A placeholder matches nothing and reports nothing | skipped silently |
| `TestAssetJunction` (build; memFS) | An asset through a junction inside the bounds is found; outside them, `E3706` | `E3701` |
| `TestEvalSymlinksMklink`, `TestLoadDirMklink` (`//go:build windows`; skip if `mklink /J` fails) | the same behaviour on a real junction | runs when CI returns |

**Units:** PROJECT-LINKS (S, opus) goes first. LOAD-LINKS (§10.5) and BUILD-LINKS (S, opus: `hosts.go`,
`reread.go`, `realpaths.go`, plus §10.3's stale test) follow. WS-LOCKS carries the `overlay.go`
change.

**Forecast.**
- No Windows run is possible before CI returns. `GOOS=windows go vet ./...`, which also vets the
  windows-only tests, is the gate.
- Resolve only through the reader's recorded resolver (`Loader.evalSymlinks`, build's `readLog`),
  never through `project.EvalSymlinks` on the raw FS. Otherwise the load memo (ADR-0011) replays
  without that call.
- With `GODEBUG=winsymlink=0`, junctions come back as `ModeSymlink`, and both paths work.
- Junction cycles are cut by §6.5's walk-path rule. No Linux golden changes.

### 10.5 Single-file loads follow `load.dir`'s link bounds (DECISIONS 420)

**Facts.**
- `statFile` (`load/single.go:45-63`) stats `p.Abs`, so it follows any link. The path itself is
  resolved lexically (`:31-34`).
- It serves `load` (`:95`), `load.text` (`:158`), `load.csv` (`:192`), `load.defines`
  (`defines.go:36`) and `files.go:49`.
- The JSON sources that edits and `canon fmt` write already leave such a file out (`inside`,
  `files.go:90-99`).
- WIRE §6.1 (`spec/WIRE.md:584-585`) has no link rule.
- Resource's 7 `load.text("@source/…")` calls go through a declared root, so they stay within the
  bounds.

**Decision.**
1. After `refuseAbsent`, `statFile` resolves `p.Abs` through `l.evalSymlinks`:
   - a real path other than `p.Abs` that lies outside `l.walkBounds()` is `E7004` `ReadOutsideRoots`;
   - `fs.ErrNotExist` or `ErrSymlinkLoop` is `E7004`, with the cause `causeOf` gives;
   - any other error (an FS without links) leaves the decision to the stat, as today.

   `p.Abs` is still what is stat'd and read.
2. The finding is `E7004`, not `W7115`: a skipped glob leaves a valid empty value, but a single file
   has no such value. It is one more cause kind, not a new code.

**Spec text.** WIRE.md §6.1, appended after "The path must exist and be a readable regular file
[`E7004`]; under an optional root absent on this machine, [`E7009`] (§2.2).":
> The path follows `load.dir`'s link bounds (§6.5): one that is or passes through a link whose target resolves outside the declared roots and the project is `E7004` (`ReadOutsideRoots`), a dangling link or a loop `E7004` (`ReadMissing`, `ReadLinkLoop`); the project root's own ancestors never count as links (DECISIONS 420).

**ERRORS.md** (applied by S2): a new row in the kinds table, between `ReadNotRegular` and
`ReadPermission` (`spec/ERRORS.md:174-175`):
`| `ReadOutsideRoots` | reached through a symbolic link outside the roots and the project | `E7004` |`

**DECISIONS text.**
> 420. **Single-file loads follow `load.dir`'s link bounds (WIRE.md §6.1, §6.5; ERRORS.md E7004).** The path of `load`, `load.text`, `load.csv` and `load.defines` is read through a link only when its real path lies inside the project or a declared root, bounds resolved as `load.dir`'s, the project root's ancestors not counting; otherwise `E7004` `ReadOutsideRoots`. A dangling link or a loop stays `E7004` `ReadMissing`, `ReadLinkLoop`. Reason: a single file was read through a link pointing anywhere on the machine, unlike `load.dir` and assets (376); the JSON sources edits and `canon fmt` write already left it out.

**Tests.**
- `internal/load/testdata/findings/E7004_5.txtar` to `E7004_8.txtar`, in call form: `load`,
  `load.text`, `load.csv` and `load.defines` through `lnk → /outside`. Today: the file is read, with
  no finding.
- `TestSingleFileLinkBounds` (load, real disk; skipped where links cannot be made):
  - links inside the project, into another root, and through an ancestor of the project root are read;
  - a dangling link is `ReadMissing`;
  - a loop is `ReadLinkLoop`.

**Unit:** LOAD-LINKS (M, opus; it also covers §10.4's `classify`). It runs after PROJECT-LINKS and S2.

**Forecast.**
- Each single-file load now costs one `EvalSymlinks`: cached by the snapshot, but resolved again at
  each refresh. Release §12 step 2 runs `make bench-edit`. A refresh p95 over DECISIONS 382's 500 ms,
  or a regression over 10%, is recorded, not optimised away.
- 29 test files create links. A test that reads a single file through a link outside the bounds
  encoded the old rule: update it and log it. Any other failure stops the unit.
- The test memFS's `Stat` does not follow links, so refusals go in txtars and successful reads are
  tested on real disk.

### 10.6 A name reported changed is read again, whatever its stamp (DECISIONS 421; amends 374)

**Facts.**
- On Windows the stamp is size and modification time only (DECISIONS 374).
- The refresh decides by stamp (`workspace/refresh.go:26-33`). An OS event only marks the hub dirty
  (`watch_os.go:88-102`), and `refreshWatched` (`watch_hub.go:372`) then refreshes by stamp.
- The language server ignores the parameters of `didChangeWatchedFiles` (`lsp/sync.go:128-136`).
- After a `*StaleError` (edit N9, workspace J5/S5, build 352), the next refresh is just as blind:
  - every retried `Edit` fails N9 again;
  - `Write`'s single build retry (`writer.go:16-22`) fails too.
- The in-memory test FSes return a nil `Sys()`, so their stamp is exactly Windows' stamp. This can be
  tested on Linux.

**Decision.**
1. **`Project` keeps `suspects`**: absolute names reported as changed.
   - At the next refresh, every file or listing entry named by a suspect, or by its directory, is read
     again whatever its stamp, and compared by hash as today.
   - The same content keeps the snapshot and raises no event.
   - Names the snapshot never read are ignored.
   - `recheck` takes the set under `p.mu`, clears it, and merges it back if the refresh fails.
2. **Who reports names:**
   - the hub's `notified`, with the event's name;
   - the language server, with each `changes[].uri`;
   - `writeAs`, before `end`, for any `*StaleError` its writer returns.

   Reason: these are the only signals of a change that the stamp cannot show.
3. **The rule applies on every platform.** A forced read costs one read and one hash.

**Code.**
- **`workspace`:**
  - `func (p *Project) Suspect(names ...string)`, which takes only `p.mu`;
  - `snapFS.refresh(ctx, forced map[string]bool)`;
  - the file and listing rechecks skip `ifStale` for a forced name;
  - `refreshNames` and `pollOnce` pass nil.
- **`lsp`:**
  - `didChangeWatched` decodes `{"changes":[{"uri":…}]}` with `pathOf`, calls `Suspect` on each open
    project, then touches the projects as today;
  - `config.base` (nil means `build.OS()`) is the test seam that `workspaceProject.open` uses.

**Spec text.**
- **API.md S1** (`spec/API.md:210`), after "changes (DECISIONS 374).":
  > On every platform a name reported changed is read again at the next refresh whatever its stamp, with its directory's listing: a name an OS notification of a `Watch` reports (§12), a file the language server's client reports in `workspace/didChangeWatchedFiles`, and each file a `*StaleError` of an `Edit` or a writing `Build` names (DECISIONS 421).
- **API.md W12** (`:1729`): "any change to the read set that changes a stamp (S1) is seen" becomes
  "any change to the read set that changes a stamp or that a notification reports (S1) is seen".
- **IMPLEMENTATION-PLAN §8.4**, the Sync bullet (`:1276-1279`), appended:
  > A `workspace/didChangeWatchedFiles` notification marks each file it names to be read again at the next pass whatever its stamp (API.md S1).

**DECISIONS text.**
> 421. **A name reported changed is read again whatever its stamp (API.md S1, W12; amends 374).** At the next refresh the project reads again, comparing by hash, each name an OS notification of a `Watch` reports, each file a language server client reports in `workspace/didChangeWatchedFiles` and each file a `*StaleError` of an `Edit` or a writing `Build` names, with its directory's listing, whatever their stamp; the same content keeps the snapshot. Reason: Windows' stamp is size and modification time (374), so a file replaced keeping both stayed stale under a `Watch`, in the language server, and through every retried `Edit` and writing `Build` (ADR-0021 §9).

"[amended by 421]" goes on DECISIONS 374.

**Tests.**

| Test | Proves | Today |
|---|---|---|
| `TestSuspectRereadsKeptStamp` (workspace internal, `writeMapFS`) | New bytes with the same size and ModTime: `Read` misses them, and after `Suspect` it sees the new bytes and one external event. The same bytes keep the same snapshot. A name added under a directory whose stamp is unchanged is listed | new method |
| `TestWatchEventRereadsKeptStamp` (workspace; `WatchOptions{OS: true, notify: fake}` over `writeMapFS`) | The event's file arrives as a `Change` | no Change before the deadline |
| `TestWriteStaleSuspects` (workspace; one row per `*StaleError`: edit, workspace, build) | After `writeAs`, the current snapshot reads the new bytes | old bytes |
| `TestEditStaleRereadsKeptStamp` (workspace) | The first `Edit` is a `*StaleError`; the retry applies | stale again |
| `TestWatchedFilesRereadKeptStamp` (lsp; `config.base` is an OS FS that hides change time and file identity) | Diagnostics follow the change after the notification | old diagnostics |

**Units:** WS-REPORTED (M, opus; `internal/workspace`), after WS-LOCKS. LSP-WATCHED (S, opus;
`internal/lsp`), after WS-REPORTED.

**Forecast.**
- A suspect lost on a cancelled refresh means a silently stale snapshot, so the merge-back must be
  tested.
- Run `-race` at GOMAXPROCS 2 and 4 on workspace and lsp.
- `Suspect` takes only a leaf lock, so the LSP may call it under `s.mu`. Never call it under
  `refreshMu`.
- Real NTFS behaviour (tunnelling, the names ReadDirectoryChangesW reports) waits for CI. The Linux
  tests use Windows' exact stamp.

### 10.7 The cache's stat table: plan text only (S1)

IMPLEMENTATION-PLAN §8.5 (`spec/IMPLEMENTATION-PLAN.md:1309-1312`), replacing "plus a stat table
`(path, size, mtime, sha256)` so an unchanged file is not re-hashed.":
> each with the read set its run consulted (every file with its SHA-256, every stat and listing); an entry is reused only after every one of those reads is repeated and every file re-hashed, and a read it did not record makes it a miss. No stat (size, modification or change time, file identity) stands in for a hash (DECISIONS 374). The cache is parked until v0.3's scale bench (Louis, 2026-10-31); its ADR decides the rest (log-2026-10-14, "On-disk cache").

### 10.8 `evalAnalysis` never calls `KeptSelection`: stays parked

`evalAnalysis` (`api/evaluate.go:142-158`) analyses through `workspace.Analyze`. Calling
`KeptSelection` would only save a second analysis of the touched packages after an edit. That is
performance work, not a correctness issue, so it stays parked: robustness before speed, and reuse
waits for v0.3's scale bench (Louis, 2026-10-31). No unit.

### 10.9 M4 call (b), `check.Info` layered maps: out of v0.2

Louis's condition (2026-10-14) was "only if easy and with no robustness risk (frozen contract)". It
fails both:
- **Not easy.** `check.Info` (`check/info.go:35-53`; IMPLEMENTATION-PLAN §4.7) is 15 exported maps.
  About 150 production files in 13 packages index them directly (check 64, eval 25, views 21, ir 13,
  edit 11, and 16 in the other eight). Layering touches every one of those files, and §4's rule needs
  approval from every consuming package's owner.
- **Not risk-free.** The saving needs one base `Info` shared across snapshots. A write that reaches a
  shared base corrupts every snapshot that reads it: an intermittent race, the kind of work
  log-2026-10-14 parks.
- **Performance only.** It saves about 44-60 MB and 16 ms per warm re-check, and NFR-01 already
  passes without it.

plan.md records it as declined. v0.3's scale bench may reopen it as performance work.

## 11. The run

### 11.1 Rules (ADR-0021 §7.1, with v0.2's lessons)

- **Read first:** CLAUDE.md, `meta/state.md`, `meta/plan.md` ("v0.2: trust"), this ADR whole,
  DOCTRINE §2-§5, `.claude/rules/orchestration.md` and `.claude/rules/go.md`.
- **Base:** `main` at or after the commit holding this ADR.
- **Calls already made are never re-asked:** every decision of this ADR. Technical calls are the
  orchestrator's, decided and logged in `meta/decisions/log-<date>.md`. Only a philosophy question
  goes to Louis, in a file under `meta/handoff/`.
- **Memory caps.**
  - Every agent's `go test` or probe runs as
    `systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local go test -timeout 300s ...`,
    and that exact command is in every VERIFY line.
  - The orchestrator's `make check` runs under `MemoryMax=6G`.
  - Temp dirs go in `/var/tmp`.
- **Concurrency.** At most 6 builders at once, on disjoint packages, one builder per package at a
  time. At most 2 of them run heavy tests at once (eval, `-race`, fuzz, the C++/Go/TS golden
  compiles).
- **Working tree.**
  - Never `git stash`, `git checkout --` or `git reset`.
  - Each unit works in a worktree (`isolation: "worktree"`), fast-forwarded to `main` first and
    reporting its base SHA.
  - The orchestrator commits in the worktree and cherry-picks onto `main` (never merges), with
    `make check` before every `cherry-pick --continue`.
  - Meta commits go on `main` only.
- **Read-only outside this directory** (Resource, Source, Scripts, the services). A need there is a
  handoff.
- **Agents.** Check `ListAgents` before telling Louis that units run. End every delegation prompt
  with "send your final report; do not stop without it".
- **Concurrency tests** also run at `GOMAXPROCS=2` and `4`.
- **Review.**
  - Every non-trivial diff gets `spec-reviewer` (opus), against this ADR's section and the spec it
    cites.
  - On a FAIL, resume the SAME builder with the numbered findings, then the SAME reviewer on the fixes
    only.
  - A sonnet builder that fails review twice is replaced by an opus one.
- **Long gates** run once, at the end (§12). Builders run only their own package tests.
- **Status.** Status for Louis goes in `meta/handoff/<date>-v020-status.md`, updated at each wave
  boundary, with a two-line chat summary.
  - Before EN-GO starts, the status file states §4 Decision 9 (Go exhaustiveness) as the one call
    Louis may veto.
- **Against the common mistakes (§0):**
  - Ask every builder for its command log, and read it.
  - Never start a unit whose package another unit holds.
  - Read every golden diff, and classify each hunk by this ADR's section; a hunk no section explains
    stops the unit.
  - Copy the VERIFY line from this section, never retype it.
  - A builder that "improves" a design is answered with §0.

### 11.2 Steps

**S1, spec sync.** The orchestrator; one commit, `docs(spec): v0.2 decisions 398-422 (ADR-0023)`.
- **Spec texts.** Every "Spec text" block of §2-§10 and §12, verbatim, except:
  - the ERRORS.md blocks (S2);
  - the held-back blocks below.
- **DECISIONS.** 398-422 appended in order.
  - "[amended by N]" marks go on 338 (408), 340 and 363 (407), 374 (421), 375 (417), 376 (419),
    379 (402), 383 and 393 (406), and 391 (403, 404).
  - Strike the "Still open" checker bullets (`DECISIONS.md:3088-3110`) that 408, 409 and 411 close.
    Keep J13/J14, with "(open: a view-model format choice, ADR-0023 §8.4)".
- **IMPLEMENTATION-PLAN:**
  - §4.1, the `ExpectStmt` line (§2);
  - §4.5, the TypeRef `Range` line (§3);
  - §7.9, the `pkgchecks` row (§2);
  - §8.4 (§10.6) and §8.5 (§10.7).
- **meta:**
  - plan.md's v0.2 list already points here;
  - `meta/state.md` moves the owed items this ADR fixes out of "Owed, small" and adds §13's items,
    staying at 80 lines or fewer.
- **Held back from S1.** A spec block that a test compares byte for byte with code lands in the
  commit of the unit that changes that code, written by the orchestrator. Known:
  - CODEGEN §8.2's four new TS helpers (§3), in TS-RANGE's commit, copied from its
    `runtime/decode.ts.txt`, so that `TestHelperBlockIsTheSpecs` passes.

  If S1's `make check` fails on another such test, hold that block back the same way, to the unit
  that changes the compared code, and log it.
- **Review and check.** Before the commit, `spec-reviewer` (opus) checks S1's diff against this ADR:
  every block verbatim, nothing added. Then `make check` must be green. If `diag-check` fails, or a
  test comparing a document's code table with ERRORS.md fails, run S2 first and commit S1 and S2
  together.

**S2, DIAG.** The orchestrator, then one sonnet go-dev in the main tree, with nothing else running.
One commit, `feat(diag): v0.2 codes and messages (DECISIONS 398, 403, 405, 406, 414, 418, 420)`.
- **The orchestrator writes every ERRORS.md block:**
  - §2: E5004's three variants, E5006;
  - §5: E8005's meaning and `foreignDefine`;
  - §6: E3316's meaning, `nul`, `enumWire`, `caseWire`;
  - §9.8: E9001's text, E9006 `param`/`read`, E8104's text;
  - §10.3: E8021's meaning and `source`;
  - §10.5: the `ReadOutsideRoots` kind;
  - the count sentence. The expected text is "The catalogue holds 332 codes: 303 errors, 26 warnings
    and 3 run-time codes, with 567 messages." diaggen checks it; if it disagrees, recount (diaggen
    wins, never edit around it).
- **The go-dev:**
  - runs `go run ./internal/diag/cmd/diaggen -out internal/diag -runtime internal/gen`.
    `codes.go` and `codes_test.go` are regenerated, never hand-edited.
  - fixes only the compile and test failures the regenerated registry forces:
    - `rules/tests.go:50` uses `E5004.AtValue`;
    - `translate_sig.go:25` uses `E9006.AtParam`;
    - `translate_read.go:234` uses `E9006.AtRead`;
    - `internal/diag/testdata/messages.txtar` and every findings txtar whose message text changed
      (§9.8's list) follow through their packages' `-update` (read the diff: message text only);
    - `internal/diag/render_test.go:12` takes diaggen's count;
    - adds `internal/ir/testdata/findings/E9006_5.txtar` (§9.8).
  - names no new code in production code. E5006, `foreignDefine`, the E3316 variants, `source` and
    `ReadOutsideRoots` stay unreported until their units: the `diag-code-untested` ratchet only
    observes.
- `make check` must be green. No review: no decision is left in S2.

**S3, the units.**
- Start any unit whose "after" units have all landed on `main`, within §11.1's caps.
- Each delegation uses orchestration.md's template:
  - **GOAL:** from the unit's section.
  - **OWNER/SCOPE:** exactly the packages below; a granted file is named.
  - **READ FIRST:** this ADR's sections, the spec sections they cite, DOCTRINE §3-§5,
    `.claude/rules/go.md`.
  - **CONTRACTS:** the frozen interfaces the unit reads. Only N5-SYNTAX (`syntax.ExpectStmt`) and
    IR-RANGE (`ir.TypeRef`) change one, under §4's review rule.
  - **TESTS:** the section's tests, each citing its rule; every new code or variant has its txtar.
  - **VERIFY:** §11.1's capped `go test` of its packages, plus `-race` where it touches concurrency,
    with the log attached. Builders do not run `make check`; the orchestrator does, under 6G, before
    every cherry-pick.
  - **REPORT BACK:** files touched; gaps; any choice not in this ADR (none is expected; an unexpected
    one is logged under §0's fallback); what is unverified.
  - The prompt states the tier and why.

| Unit | Packages (exclusive; granted files named) | Tier | Size | After | § |
|---|---|---|---|---|---|
| DEP | `internal/types`, `internal/check`, `internal/eval`, `internal/eval/std` | opus | M | S2 | 8.1, 8.2, 8.4 (1-2) |
| N5-SYNTAX | `internal/syntax` | opus (frozen AST) | S | S2 | 2 |
| IR-RANGE | `internal/ir` | opus (frozen IR) | M | S2 | 3 |
| EDIT | `internal/edit`; granted `internal/build/write.go` and its callers' rename, `api/edit_rollback_io_test.go`, `api/value_cause_test.go` | opus | M | S2 | 10.1, 8.3, 8.4 (4-7) |
| PROJECT-LINKS | `internal/project` | opus | S | S2 | 10.4 |
| PROGEN-TIERS | `internal/testkit/progen`, `internal/testkit/fixture` | sonnet, opus review | M | S2 | 9.11 |
| RD-PATCH | `tools/realdata-compile.sh`, the Makefile comment of `realdata-compile` | sonnet | XS | S2 | 12 |
| N5-RULES | `internal/rules` | sonnet | S | N5-SYNTAX | 2 |
| N5-FORMAT | `internal/format` | sonnet | S | N5-SYNTAX | 2 |
| IR-PLANS | `internal/ir` | sonnet, opus review | M | IR-RANGE | 4 (plans), 7, 9.2 (plan), 9.3, 9.4; and the `Twin` fixture of §6 |
| LOAD-LINKS | `internal/load` | opus | M | PROJECT-LINKS | 10.4, 10.5 |
| WS-LOCKS | `internal/workspace` | opus | S | PROJECT-LINKS | 10.2, 10.4 (`overlay.go`) |
| N5-CHECK | `internal/check`; granted `internal/testkit/progen/testdata/gaps.txt` (its E5006 line) | sonnet, opus review | S | N5-RULES, DEP | 2 |
| N5-EVAL | `internal/eval` | opus | L | N5-CHECK | 2 |
| CHECK-OWED | `internal/check` | sonnet, opus review | M | N5-CHECK, IR-PLANS | 6, 8.5, 9.9, 10.3 (check) |
| GEN-TS | `internal/gen/ts` | sonnet, opus review | M | IR-PLANS | 9.2 |
| EN-GO | `internal/gen/go` | sonnet, opus review | M | IR-PLANS; §4 Decision 9 | 4 |
| EN-CPP | `internal/gen/cpp` | sonnet | S | IR-PLANS | 4 |
| WS-REPORTED | `internal/workspace` | opus | M | WS-LOCKS | 10.6 |
| BUILD-LINKS | `internal/build` | opus | S | EDIT, CHECK-OWED, PROJECT-LINKS | 10.3 (stale), 10.4 |
| MACRO-IR | `internal/ir`; granted `internal/build/pipeline.go:349` | sonnet, opus review | M | IR-PLANS, BUILD-LINKS | 5 |
| EN-TS | `internal/gen/ts` | sonnet | S | GEN-TS | 4 |
| GEN-GO | `internal/gen/go` | sonnet, opus review | S | EN-GO | 9.1, 9.5, 9.10 (Go test) |
| CPP-RANGE | `internal/gen/cpp` (+ `runtime/*.txt` comment, text templates) | sonnet, opus review | M | IR-RANGE, EN-CPP | 3 |
| LSP-WATCHED | `internal/lsp` | opus | S | WS-REPORTED | 10.6 |
| N5-BUILD | `internal/build` | opus | M | N5-EVAL, MACRO-IR | 2 |
| GO-RANGE | `internal/gen/go` | sonnet, opus review | M | GEN-GO, IR-RANGE | 3 |
| TS-RANGE | `internal/gen/ts` (+ the held-back §8.2 block) | sonnet, opus review | M | EN-TS, IR-RANGE | 3 |
| CPP-OWED | `internal/gen/cpp`, `internal/gen/cpp/runtime` | sonnet | S | CPP-RANGE, MACRO-IR | 5 (gen side), 9.10 (C++ pins) |
| N5-CLI | `internal/cli` (not `guide/`) | sonnet | S | N5-BUILD, N5-FORMAT | 2 |
| MUST | `internal/testkit/must` (new); the test files of `internal/lsp` and `internal/workspace` | sonnet | XS | LSP-WATCHED, WS-REPORTED | 9.6 |
| E2E-RANGE | `internal/ir` (new test files only) | sonnet, opus review | M | CPP-RANGE, GO-RANGE, TS-RANGE, MACRO-IR | 3 |
| N5-TOOLS | `internal/lsp` (testdata), `api` (one test), `internal/testkit/vscodegrammar`, `internal/testkit/progen` (the form's generator) | sonnet | S | N5-CLI, EDIT, PROGEN-TIERS, MUST | 2 |
| EXAMPLES | `examples/features/pkgchecks` (+ its granted AST and scope goldens) | sonnet | S | N5-CLI | 2 |
| GUIDE | `internal/cli/guide` and its goldens | sonnet | S | EXAMPLES and every unit in §11.4's list | 11.4 |
| CLEANUP | as found | sonnet | S | every unit | 11.5 |

**Package queues** (each runs in order, one builder at a time):

| Package | Queue |
|---|---|
| `internal/ir` | IR-RANGE → IR-PLANS → MACRO-IR → E2E-RANGE |
| `internal/check` | DEP → N5-CHECK → CHECK-OWED |
| `internal/eval` | DEP → N5-EVAL |
| `internal/build` | EDIT (its granted rename) → BUILD-LINKS → MACRO-IR (its granted line) → N5-BUILD |
| `internal/gen/go` | EN-GO → GEN-GO → GO-RANGE |
| `internal/gen/cpp` | EN-CPP → CPP-RANGE → CPP-OWED |
| `internal/gen/ts` | GEN-TS → EN-TS → TS-RANGE |
| `internal/workspace` | WS-LOCKS → WS-REPORTED → MUST (tests) |
| `internal/lsp` | LSP-WATCHED → MUST (tests) → N5-TOOLS (testdata) |
| `internal/cli` | N5-CLI → GUIDE |
| `api` | EDIT (two test files) → N5-TOOLS (one test) |
| `internal/testkit/progen` | PROGEN-TIERS → N5-CHECK (gaps line) → N5-TOOLS |

**The critical path** is N5: S1 → S2 → DEP → N5-CHECK → N5-EVAL → N5-BUILD → N5-CLI → EXAMPLES →
GUIDE → CLEANUP → §12. Start DEP, N5-SYNTAX and IR-RANGE first.

**Wave plan, for the status file:**

| Wave | Units |
|---|---|
| A | DEP, N5-SYNTAX, IR-RANGE, EDIT, PROJECT-LINKS, PROGEN-TIERS, RD-PATCH (at most 6 at once: RD-PATCH last) |
| B | N5-RULES, N5-FORMAT, IR-PLANS, LOAD-LINKS, WS-LOCKS, N5-CHECK |
| C | N5-EVAL, CHECK-OWED, GEN-TS, EN-GO, EN-CPP, WS-REPORTED, BUILD-LINKS, MACRO-IR |
| D | EN-TS, GEN-GO, CPP-RANGE, LSP-WATCHED, N5-BUILD |
| E | GO-RANGE, TS-RANGE, CPP-OWED, N5-CLI, MUST |
| F | E2E-RANGE, N5-TOOLS, EXAMPLES, GUIDE, CLEANUP |

The waves are a forecast. The "after" column binds.

### 11.3 Integration

- Cherry-pick each unit's commits onto `main` in landing order, with `make check` before every
  `cherry-pick --continue`.
- **A conflict in a golden** (`examples/**/expected`, `testdata/**`) is resolved by regenerating with
  `-update` on the merged tree and reading the diff, never by picking a side (ADR-0022 §8.2).
- **A conflict in `internal/testkit/progen/testdata/gaps.txt`** keeps every line, in the file's order.
- **A conflict in name-plan code** (`internal/ir/*plan*.go`) is a ≤ 20-line orchestrator fixup, or it
  goes back to the later unit.
- **The progen coverage ratchet.** `internal/testkit/progen/coverage_test.go` fails on a code the
  compiler can report that has neither a mutation operator nor a line in `gaps.txt`.
  - The one new code is E5006. N5-CHECK first makes it reportable and adds the line (granted):
    `E5006 a package subject's target is not a top-level let of its own package (EVALUATION.md §10.5, DECISIONS 398); internal/check's E5006_*.txtar produce it`.
  - N5-TOOLS may replace that line with an operator.
  - New variants of existing codes need no line.

### 11.4 GUIDE

GUIDE writes these in `internal/cli/guide/`. Every recipe passes `canon check` in an examples
project, and the guide's output goldens are regenerated and read.
- **`expect package`** (§2): the example of §2; the budget note (Forecast 9); "tables accept table
  values only"; the field-collection way out (Forecast 11). An `errors.md` row for E5006.
- **Checked decode and bound constants** (§3): what is checked; the message forms; a C++
  `static_assert` against `T::k<F>Max`; "declare a width with a sized type" (GEN-02).
- **Opaque enums** (§4): `Position()`; a Canon `const` is a Go `var`; the TS `Members.indexOf` way
  out; Decision 9's note and its way out (a `default:` with an error, plus a test that ranges over
  `Members()`).
- **Wire names** (§6): uniqueness and NUL; `errors.md` rows for the E3316 variants.
- **C++ defines** (§5): E8005 `foreignDefine` and its ways out; the recipe "a header rendered by
  `emit text` that defines enum names: use `@cpp(defines:)` instead".
- **`<Fn>File`** (§7): one sentence in the text-decoder recipe.
- **Ways out** (§9.8): add `Float32` to `emit.md:115`.
- **`.canon` text names** (§10.3): one sentence and an `errors.md` row.
- **Kept dependent maps** (§8.1): `==` and `union` on a kept map.
- **Amend paths into a dependent map** (§8.2): the `by[a]: {…}` form.

GUIDE runs after EXAMPLES and after every unit that changes user-facing behaviour: N5-CLI, EN-GO,
EN-CPP, EN-TS, CPP-RANGE, GO-RANGE, TS-RANGE, CHECK-OWED, MACRO-IR, IR-PLANS, DEP.

### 11.5 CLEANUP

One delegation after every unit has landed: helpers invented twice (`make audit`, `dup-in-repo`),
dead code left by the units, then a review. It may not change behaviour; a golden diff in CLEANUP is
a bug.

### 11.6 Estimate

36 delegations (the table's units and S2's go-dev), plus S1. That is about 35-45 hours of agent time
(S1 and S2 about 3 h; the units 25-32 h with review rounds; acceptance and release 6-8 h). Wall time
is about 2 to 2.5 days, plus any rate-limit stop; resume each agent where it stopped.

Calibration: v0.1.9 took 18-24 h for 30 units. v0.2 is larger because N5 alone is ten units and the
owed bugs grew while planning (seven real bugs found by probes).

## 12. Release

Release only when every unit has landed, CLEANUP is reviewed, and the spec, DECISIONS and `meta/` are
in sync (`meta/state.md` ≤ 80 lines). Each step's command, machine, load at start and end, and result
go in the final acceptance entry of `meta/decisions/log-<date>.md`.

**The language version stays 0.1 (DECISIONS 422).** The spec text goes in S1, appended to SPEC §3.1's
`canon` bullet:
> The compiler's version and the language version are separate (DECISIONS 354, 422): compiler 0.2 reads language 0.1, whose syntax and refusals it extends as the 0.1.x releases did.

> 422. **Compiler 0.2 reads language 0.1 (SPEC §3.1; GRAMMAR.md §7.1).** The language version (`canon: "MAJOR.MINOR"`) moves only when existing source changes meaning; compiler 0.2.0 accepts every `canon: "0.1"` project and adds syntax (`expect package`) and refusals (`E3316` `nul`/`enumWire`/`caseWire`, `E5006`, `E8005` `foreignDefine`, `E8021` `source`, `E7004` `ReadOutsideRoots`) to 0.1, as the 0.1.x releases did; `canon: "0.2"` stays `E1001` until a version changes the meaning of existing source. Reason: v0.2 breaks generated APIs (402, 407) and refuses more, but changes no existing source's meaning; a language bump would make every project edit its header for nothing.

**RD-PATCH (unit, wave A).** `tools/realdata-compile.sh` gains an optional `RD_PATCH`:
- the path of a unified diff (`a/`, `b/` prefixes, paths from the Resource root), applied with
  `patch -p1 --forward --batch -d <scratch>/Resource` after the copy and before `canon build`;
- a patch that does not apply exits 2 with the message `realdata-compile: RD_PATCH does not apply`;
- absent, the behaviour is unchanged;
- the Makefile comment of `realdata-compile` documents it.

Acceptance: the script without `RD_PATCH` behaves byte for byte as before (same table on the same
tree). Reason: a release whose new refusal hits real data must still prove the rest compiles, as
v0.1.12 did by hand (log-2026-10-22).

**Steps:**
1. **`make check`** under `systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local`, on a
   clean `main`.
2. **Long gates**, one at a time, never together, with no agent running:
   - `make stress` (60 s, 3G, `-race`);
   - `make fuzz-edit` (10 min, 6G). Then `git status`: a crasher lands in
     `internal/edit/testdata/fuzz/FuzzMinimalWriteAll/`. It is a bug, fixed (failing test first)
     before the tag.
   - Every other fuzz target, 3 minutes each, under 3G:
     ```
     for pkg in $(grep -rl --include='*_test.go' '^func Fuzz' internal api | xargs -n1 dirname | sort -u); do
       for f in $(grep -ho '^func Fuzz[A-Za-z0-9_]*' $pkg/*_test.go | sed 's/^func //' | sort -u); do
         [ "$f" = FuzzMinimalWriteAll ] && continue
         systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 env GOTOOLCHAIN=local \
           go test -run '^$' -fuzz "^$f\$" -fuzztime 3m "./$pkg" || echo "FUZZ FAIL $pkg $f"
       done
     done
     ```
     A crasher is a bug, fixed before the tag.
   - `make progen-nightly` (3 seeds at N=10000, about 40 min). An archive kept by `-progen.keep` is a
     bug, triaged and fixed before the tag.
   - **The bench A/B.** Tree A is `git archive v0.1.13` into `/var/tmp/canon-020-ab/a`; tree B is
     `main`. Run `make bench-edit` (6G) as A, B, A, B, and the same for `make bench-lsp`.
     - A refresh p95 over 500 ms (DECISIONS 382), or a B/A regression over 10%, is recorded with its
       numbers.
     - If the cause is a robustness change (LOAD-LINKS' `EvalSymlinks`, WS-REPORTED's suspects,
       WS-LOCKS' locks), it is accepted and stated in the reply.
     - Any other cause is a bug, found by re-running B with one suspect unit reverted in a scratch
       worktree, in this order: LOAD-LINKS, WS-REPORTED, WS-LOCKS, DEP, N5-EVAL.
3. **Cross-platform** (CI is down):
   - `GOOS=windows go vet ./...` and `GOOS=darwin go vet ./...`;
   - the C++ compile suites on every `cxx.Toolchain` row present (g++, clang++, clang++ with libc++);
   - `go test -race` over `api`, `internal/workspace`, `internal/edit`, `internal/build`,
     `internal/load`, `internal/lsp`, `internal/writelock` and `internal/cli`, at GOMAXPROCS 2, 4 and
     the default.
4. **The candidate:**
   ```
   CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -ldflags "-s -w -X \
   github.com/fantasim/canonlang/internal/build.CompilerVersion=0.2.0" -o /var/tmp/canon-020-dry/canon ./cmd/canon
   ```
5. **Resource dry run.** Follow ADR-0021 §8 step 4, read-only on Resource and Source, with two
   changes:
   - **The badge patch.** In the farm copy, give the two `Badge` cases that collide (§5: the
     `BADGE_REQ_USE_ITEM` and `BADGE_REQ_FAERY_RANK` kinds in `Config/progression/quests/`)
     `@cpp(name: "BadgeReqUseItem")` and `@cpp(name: "BadgeReqFaeryRank")`.
     - Write that edit as `/var/tmp/canon-020/badge.patch`, a unified diff from the Resource root.
     - Without the patch, the candidate must report exactly those two `E8005` `foreignDefine`, and
       nothing else new.
   - **The expected differences** against the pinned 0.1.13 binary:
     - opaque enums (Go, C++, TS: §4);
     - check lines and bound constants (§3);
     - removed pragma trios (§5, 116 pairs);
     - `FileFile` → `File` (§7, 71 plan names);
     - TS ids of unselected tables (§9.2);
     - T6 and the runtime comment (§3).
   - Every other diff is classified by rule, never by judgement:
     - a finding whose code or variant this ADR introduces is real Resource data that the new rule
       catches: an upgrade note in the reply, and the release goes on;
     - anything else (another code, a changed message, an output change no section explains) is a
       bug, fixed (failing test first) before the tag.
   - **N5 on Resource** (§2 Acceptance):
     - rewrite crafting's "listed twice" test and the three persistedIds tests as `expect package`;
     - they pass, and they fail once the check is removed;
     - `canon test` counts are unchanged otherwise;
     - record wall time and peak RSS.
6. **`RD_PATCH=/var/tmp/canon-020/badge.patch make realdata-compile`** must PASS (DECISIONS 394). No
   tag without it.
7. **Consumer compile, a one-off** (not a gate):
   - For each of admin, monitoring, trello, website and wiki: `canon build -only-root <svc>` (with the
     badge patch) into a `git archive` copy of that service, then `go vet ./...`, and `tsc --noEmit`
     with the service's own tsconfig where it has one.
   - Each breakage goes in the reply with its migration. §4's Forecast lists the expected ones: wiki 2,
     monitoring 2 (Go); admin 1, monitoring 2 (TS).
   - Source is not compiled. The reply lists the 70 `…FileFile` call sites (§7) and confirms no
     nlohmann use of the opaque enums (§4).
8. **The final acceptance entry** in the log, with every result above.
9. **Tag and dist**, on a clean `main`, as ADR-0021 §8 step 6:
   - copy `dist/` to `/var/tmp/canon-dist-0.1.13/` (`make dist` starts with `rm -rf dist`);
   - `make tag VERSION=v0.2.0` (annotated, local, never pushed);
   - `make dist VERSION=v0.2.0`;
   - `(cd dist && sha256sum -c checksums.txt)`;
   - extract `linux_amd64` and check that `canon version` reads `canon 0.2.0 (<sha>)` with the tag's
     commit, and that its sha256 equals a candidate rebuilt at the tag on the clean tree with step 4's
     command;
   - step 5's results stand when `git diff --stat <dry-run commit> v0.2.0` touches only `meta/`;
     otherwise redo step 5 with the rebuilt candidate;
   - a failure after the tag: `git tag -d v0.2.0` (never pushed), then redo from the failing step.
10. **Never install or push.** Never install the binary (`~/.local/bin` is outside this directory)
    and never push `main` or the tag: Louis does both.
11. **The reply**, committed after the tag: `meta/handoff/<date>-canon-reply-v020.md`, for the
    Sovereign sessions (Resource, Source, Scripts, website, admin, monitoring, wiki, trello,
    config-studio).
    - **What changed**, per item of §1's table, with its DECISIONS number.
    - **Upgrade notes (breaking):**
      - opaque enums: the Go migration list (step 7), Decision 9 with its way out, TS
        `Members.indexOf`, and C++ `ToWire`/`FromWire`;
      - decoders refuse values outside refinements: run the parity tests at the pin bump;
      - bound constants replace hand guards; declare widths with sized types;
      - `…FileFile` → `…File`: 70 Source call sites;
      - Resource adds the two `@cpp(name:)` (the badge patch), and moves `Server/UI/WindowIds.h` to
        `@cpp(defines: "APP_")` / `"WIDC_"`;
      - `.canon` `@text` names are refused (none in Resource), and a `.canon` output 0.1.x left
        inside a project stays a source until deleted;
      - single-file loads through links outside the bounds are refused (none in Resource);
      - a TS-only package with two unselected same-record tables is newly `E8005`;
      - folded defaults are now checked;
      - the language version stays `canon: "0.1"` (DECISIONS 422).
    - **New:** `expect package`, with the recipe for Resource's `…Problems` workarounds; kept
      dependent maps; amend paths into dependent maps.
    - **Numbers:** the bench and farm numbers, the 5 checksum lines, and the `linux_amd64` sha256.
    - **What stays owed** (§13).

    Then update `meta/handoff/README.md`, `meta/plan.md` (v0.2 ticked) and `meta/state.md`.
12. **Louis pushes and publishes:** `main` and the tag, then
    `gh release create v0.2.0 dist/*.tar.gz dist/*.zip dist/checksums.txt --title v0.2.0 --notes-from-tag`.
    Scripts' canon-bump moves the pin.

## 13. Found while planning, owed (not in v0.2)

S1 logs these in `meta/state.md` "Owed, small" (or as v0.3/v0.4 items). None is fixed in v0.2.

- **writelock errors.** `writelock`'s I/O errors wrap the OS's absolute names
  (`writelock/lock.go:148, 191, 197, 211`) `[reading]`. Same class as DECISIONS 416; v0.3.
- **A `.json` text output that a glob would match.** A `load.dir` glob that would match a `.json`
  text output not yet written is caught as `E8026` only from the next run `[unverified]`.
- **A third test-helper copy.** `replacePlatforms` (`api/refresh_test.go:24`) is a third copy of
  `stampPlatforms`.
- **Dead `ModFloat` helpers** (`gen/go/constants.go:425`, `gen/ts/constants.go:344`): v0.3, with a
  regeneration of the runtime goldens.
- **The function-size items** of §9.5: v0.3 (ADR-0024 §1.6).
- **MSVC at scale** (§9.7): a documented limit.
- **VIEWMODEL J13/J14 through a ref** (§8.4): the next studio work.
- **E9001's `while` variant:** v0.4 (ADR-0025).
- **The checked-widths ask's "generated size" part:** v0.3 (ADR-0024).
- **M4 call (b):** declined (§10.9). **`evalAnalysis`:** parked (§10.8).

## 14. Consequences

- **Breaking for consumers.** Each is named in the reply, with the scan's counts:
  - Go enums without `@codes`, kind enums and branch enums become opaque structs: four Go files to
    migrate;
  - nlohmann refuses those C++ enums: no Source use;
  - no TS enum exports `<E>Index`: three TS files;
  - decoders refuse values outside refinements: none expected in Canon-rendered data;
  - `Decode<Fn>File` names lose a doubled `File`: 70 Source call sites;
  - `E8005` `foreignDefine`: two Resource names;
  - wire-name, `.canon` text-name and single-file link refusals: 0 in Resource;
  - a TS-only package with two unselected same-record tables: `E8005`;
  - folded defaults checked: none expected;
  - Go's `exhaustive` linter is blind on these enums (Decision 9, Louis's call).
- **New language surface:**
  - the `expect package` form;
  - the code `E5006`;
  - the variants `E5004` `packageCheck`/`package`, `E8005` `foreignDefine`, `E3316`
    `nul`/`enumWire`/`caseWire`, `E8021` `source`, and the kind `E7004` `ReadOutsideRoots`;
  - 332 codes.
- **Frozen contracts.** Both changes go under the review rule, with this ADR as the approval:
  - `syntax.ExpectStmt` gains `Package` (§4.1);
  - `ir.TypeRef` gains `Range` (§4.5).

  `check.Info` and `api/canon.go` are untouched.
- **Robustness gained:**
  - package checks have tests;
  - generated decoders keep what types promise;
  - positions cannot leave Go, C++ or TS as numbers;
  - a C++ header compiles in every include order;
  - wire names round-trip;
  - a failed rollback can no longer hide;
  - two edits cannot overwrite one name under disjoint locks;
  - a build cannot write its own next source;
  - Windows junctions and single-file loads respect the bounds;
  - a reported change is never lost to a stamp.
- **Costs:**
  - one compare per refined field at decode (noise on the probe);
  - one `EvalSymlinks` per single-file load per refresh;
  - one package build per `expect package` world;
  - a few more locks per edit;
  - about 116 fewer pragma trios in Sovereign's C++.
- **Residual risks, accepted:**
  - Windows behaviour (junctions, reported names) is verified by vet and Linux models until CI
    returns;
  - MSVC's handling of the deleted templates is unverified;
  - several readings were not probed (DEP, EDIT, N5). The failing-test-first rule turns a wrong
    reading into a stopped unit, not a wrong release.
- **Unchanged on purpose:**
  - widths come from declared types (GEN-02);
  - performance work stays parked (Louis, 2026-10-14 and 2026-10-31);
  - the language version stays 0.1.
